package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// File settings
	RemindFiles   []string
	RemindCommand string
	Editor        string

	// Display settings
	WeekStartDay   time.Weekday
	TimeFormat     string
	DateFormat     string
	CalendarWidth  int
	CalendarHeight int

	// UI settings
	Colors      map[string]string
	KeyBindings map[string]string
	StartupView string

	// Behavior settings
	AutoRefresh   bool
	RefreshRate   time.Duration
	ConfirmDelete bool
	WrapText      bool

	// Templates
	QuickTemplate   string
	TimedTemplate   string
	AllDayTemplate  string
	UntimedTemplate string
	// Numbered templates (0-9)
	Templates [10]string

	// Editor commands
	EditOldCommand string // Edit existing reminder at specific line
	EditNewCommand string // Edit file for new reminder (go to end)
	EditAnyCommand string // Edit file without specific position
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()

	return &Config{
		RemindFiles:   []string{filepath.Join(home, ".reminders")},
		RemindCommand: "remind",
		Editor:        getDefaultEditor(),

		WeekStartDay:   time.Monday,
		TimeFormat:     "15:04",
		DateFormat:     "Jan 2, 2006",
		CalendarWidth:  80,
		CalendarHeight: 24,

		Colors: map[string]string{
			"normal":   "default",
			"today":    "yellow",
			"selected": "reverse",
			"weekend":  "blue",
			"event":    "green",
			"priority": "red",
			"header":   "bold",
		},

		KeyBindings: map[string]string{
			// Navigation (Hourly View)
			"j":      "scroll_down",
			"k":      "scroll_up",
			"<down>": "scroll_down",
			"<up>":   "scroll_up",
			"H":      "previous_day",
			"L":      "next_day",
			"K":      "previous_week",
			"J":      "next_week",
			"<":      "previous_month",
			">":      "next_month",
			"o":      "home",
			"g":      "goto",
			"/":      "begin_search",
			"n":      "search_next",
			"N":      "search_previous",
			"z":      "zoom",

			// Actions
			"<enter>": "edit",
			"t":       "new_timed",
			"u":       "new_untimed",
			"a":       "quick_add",
			"e":       "edit_any",
			"X":       "cut",
			"y":       "copy",
			"p":       "paste",
			"\\Cl":    "refresh",
			"?":       "help",
			"Q":       "quit",
			"i":       "toggle_ids",
			"\\Cb":    "open_url",

			// Template-Based Creation
			"w": "new_template0",
			"m": "new_template2",
			"M": "new_template3",
			"I": "new_template4",
			"U": "new_untimed_dialog",

			// Other
			"<tab>": "next_area",
		},

		StartupView:   "month",
		AutoRefresh:   true,
		RefreshRate:   30 * time.Second,
		ConfirmDelete: true,
		WrapText:      true,

		QuickTemplate:   `REM %monname% %mday% %year% MSG %"<++>%"%`,
		TimedTemplate:   `REM %monname% %mday% %year% <++>AT %hour%:%min% +%dura%<++> DURATION %dura%:00<++> MSG %"<++>%"%`,
		AllDayTemplate:  `REM %monname% %mday% %year% MSG %"<++>%"%`,
		UntimedTemplate: `REM %monname% %mday% %year% <++>MSG %"<++>%"%`,
		Templates: [10]string{
			`REM %wdayname% AT %hour%:%min% DURATION 1:00 MSG`, // template0 - weekly recurrence
			`REM %wdayname% MSG`,                              // template1 - weekly untimed
			`REM %mday% AT %hour%:%min% DURATION 1:00 MSG`,    // template2 - monthly recurrence
			`REM %mday% MSG`,                                  // template3 - monthly untimed
			`REM %monname% %mday% %year% AT %hour%:%min% MSG`, // template4 - instantaneous
			``, // template5 - unused
			``, // template6 - unused
			``, // template7 - unused
			``, // template8 - unused
			``, // template9 - unused
		},

		// Default editor commands - use vim with line numbers
		EditOldCommand: "vim +%line% %file%",
		EditNewCommand: "vim +999999 %file%",
		EditAnyCommand: "vim %file%",
	}
}

func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Try multiple config file locations
	configPaths := []string{
		os.Getenv("URD_CONFIG"),
		filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "urd", "urdrc"),
		filepath.Join(os.Getenv("HOME"), ".config", "urd", "urdrc"),
		filepath.Join(os.Getenv("HOME"), ".urdrc"),
	}

	for _, path := range configPaths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			if err := config.loadFromFile(path); err != nil {
				return nil, fmt.Errorf("error loading config from %s: %w", path, err)
			}
			break
		}
	}

	return config, nil
}

func (c *Config) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := c.parseLine(line); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	return scanner.Err()
}

func (c *Config) parseLine(line string) error {
	// Skip comments and empty lines
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	// Handle set commands: set variable value or set variable="value"
	setRe := regexp.MustCompile(`^set\s+(\w+)\s*=?\s*(.+)$`)
	if matches := setRe.FindStringSubmatch(line); matches != nil {
		return c.setVariable(matches[1], matches[2])
	}

	// Handle bind commands: bind key action
	// Keys can be quoted like "<down>" or unquoted like j
	bindRe := regexp.MustCompile(`^bind\s+("[^"]+"|\S+)\s+(\S+)$`)
	if matches := bindRe.FindStringSubmatch(line); matches != nil {
		key := matches[1]
		// Remove quotes if present
		if strings.HasPrefix(key, `"`) && strings.HasSuffix(key, `"`) {
			key = key[1 : len(key)-1]
		}
		action := matches[2]
		// Store as key -> action mapping
		c.KeyBindings[key] = action
		return nil
	}

	// Handle color commands: color element color_spec
	colorRe := regexp.MustCompile(`^color\s+(\w+)\s+(.+)$`)
	if matches := colorRe.FindStringSubmatch(line); matches != nil {
		c.Colors[matches[1]] = matches[2]
		return nil
	}

	return fmt.Errorf("unknown config line: %s", line)
}

func (c *Config) setVariable(name, value string) error {
	// Handle quoted strings - remove quotes and unescape
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
		// Remove the surrounding quotes
		value = value[1 : len(value)-1]
		// Unescape any escaped quotes inside
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\'`, `'`)
	}

	switch name {
	case "remind_file", "remind_files", "reminders_file":
		// Handle multiple files separated by commas
		files := strings.Split(value, ",")
		for i, file := range files {
			files[i] = strings.TrimSpace(file)
			// Expand ~ to home directory
			if strings.HasPrefix(files[i], "~/") {
				home, _ := os.UserHomeDir()
				files[i] = filepath.Join(home, files[i][2:])
			}
			// Expand $HOME
			if strings.HasPrefix(files[i], "$HOME/") {
				home, _ := os.UserHomeDir()
				files[i] = filepath.Join(home, files[i][6:])
			}
		}
		c.RemindFiles = files

	case "remind_command":
		c.RemindCommand = value

	case "editor":
		c.Editor = value

	case "week_start_day", "week_starts_monday":
		if name == "week_starts_monday" {
			// Handle boolean format
			if strings.ToLower(value) == "true" || value == "1" {
				c.WeekStartDay = time.Monday
			} else {
				c.WeekStartDay = time.Sunday
			}
		} else {
			switch strings.ToLower(value) {
			case "sunday", "sun", "0":
				c.WeekStartDay = time.Sunday
			case "monday", "mon", "1":
				c.WeekStartDay = time.Monday
			default:
				return fmt.Errorf("invalid week_start_day: %s", value)
			}
		}

	case "time_format":
		c.TimeFormat = value

	case "date_format":
		c.DateFormat = value

	case "calendar_width":
		width, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid calendar_width: %s", value)
		}
		c.CalendarWidth = width

	case "calendar_height":
		height, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid calendar_height: %s", value)
		}
		c.CalendarHeight = height

	case "startup_view":
		c.StartupView = value

	case "auto_refresh":
		c.AutoRefresh = strings.ToLower(value) == "true" || value == "1"

	case "refresh_rate":
		rate, err := time.ParseDuration(value)
		if err != nil {
			// Try parsing as seconds
			if seconds, err2 := strconv.Atoi(value); err2 == nil {
				rate = time.Duration(seconds) * time.Second
			} else {
				return fmt.Errorf("invalid refresh_rate: %s", value)
			}
		}
		c.RefreshRate = rate

	case "confirm_delete":
		c.ConfirmDelete = strings.ToLower(value) == "true" || value == "1"

	case "wrap_text":
		c.WrapText = strings.ToLower(value) == "true" || value == "1"

	case "quick_template":
		c.QuickTemplate = value

	case "timed_template":
		c.TimedTemplate = value

	case "allday_template":
		c.AllDayTemplate = value
	case "edit_old_command":
		c.EditOldCommand = value
	case "edit_new_command":
		c.EditNewCommand = value
	case "edit_any_command":
		c.EditAnyCommand = value

	case "untimed_template":
		c.UntimedTemplate = value

	case "template0":
		c.Templates[0] = value
	case "template1":
		c.Templates[1] = value
	case "template2":
		c.Templates[2] = value
	case "template3":
		c.Templates[3] = value
	case "template4":
		c.Templates[4] = value
	case "template5":
		c.Templates[5] = value
	case "template6":
		c.Templates[6] = value
	case "template7":
		c.Templates[7] = value
	case "template8":
		c.Templates[8] = value
	case "template9":
		c.Templates[9] = value

	case "timed_bold", "untimed_bold", "description_first", "schedule_12_hour", "busy_algorithm", "goto_big_endian", "untimed_duration", "status_12_hour", "center_cursor":
		// TODO: Implement additional display options

	case "busy_level1", "busy_level2", "busy_level3", "busy_level4":
		// TODO: Implement busy level colors

	case "selection_12_hour", "description_12_hour", "quick_date_US", "number_weeks", "home_sticky", "advance_warning", "untimed_window_width":
		// TODO: Implement additional display and behavior options

	default:
		// Return error for unknown config variables
		return fmt.Errorf("unknown configuration variable: %s", name)
	}

	return nil
}

func getDefaultEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	return "vi"
}
