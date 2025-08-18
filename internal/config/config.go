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
	QuickTemplate  string
	TimedTemplate  string
	AllDayTemplate string
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
			"quit":         "q",
			"help":         "?",
			"today":        "t",
			"refresh":      "r",
			"new_event":    "n",
			"edit_event":   "e",
			"delete_event": "d",
			"next_day":     "j",
			"prev_day":     "k",
			"next_week":    "J",
			"prev_week":    "K",
			"next_month":   ">",
			"prev_month":   "<",
			"goto_date":    "g",
		},

		StartupView:   "month",
		AutoRefresh:   true,
		RefreshRate:   30 * time.Second,
		ConfirmDelete: true,
		WrapText:      true,

		QuickTemplate:  "REM %s MSG %s",
		TimedTemplate:  "REM %s AT %s MSG %s",
		AllDayTemplate: "REM %s MSG %s",
	}
}

func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Try multiple config file locations
	configPaths := []string{
		os.Getenv("WYRD_CONFIG"),
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
	// Handle set commands: set variable value
	setRe := regexp.MustCompile(`^set\s+(\w+)\s+(.+)$`)
	if matches := setRe.FindStringSubmatch(line); matches != nil {
		return c.setVariable(matches[1], matches[2])
	}

	// Handle bind commands: bind key action
	bindRe := regexp.MustCompile(`^bind\s+(\S+)\s+(\S+)$`)
	if matches := bindRe.FindStringSubmatch(line); matches != nil {
		c.KeyBindings[matches[2]] = matches[1]
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
	// Remove quotes if present
	value = strings.Trim(value, `"'`)

	switch name {
	case "remind_file", "remind_files":
		// Handle multiple files separated by commas
		files := strings.Split(value, ",")
		for i, file := range files {
			files[i] = strings.TrimSpace(file)
			// Expand ~ to home directory
			if strings.HasPrefix(files[i], "~/") {
				home, _ := os.UserHomeDir()
				files[i] = filepath.Join(home, files[i][2:])
			}
		}
		c.RemindFiles = files

	case "remind_command":
		c.RemindCommand = value

	case "editor":
		c.Editor = value

	case "week_start_day":
		switch strings.ToLower(value) {
		case "sunday", "sun", "0":
			c.WeekStartDay = time.Sunday
		case "monday", "mon", "1":
			c.WeekStartDay = time.Monday
		default:
			return fmt.Errorf("invalid week_start_day: %s", value)
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

	default:
		return fmt.Errorf("unknown config variable: %s", name)
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
