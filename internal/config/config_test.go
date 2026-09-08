package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwarden/urd/internal/remind"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RemindCommand != "remind" {
		t.Errorf("Wrong default remind command: %s", cfg.RemindCommand)
	}
	if cfg.RemindCommandSet {
		t.Error("RemindCommandSet is true by default")
	}

	if cfg.WeekStartDay != time.Monday {
		t.Errorf("Wrong default week start day: %v", cfg.WeekStartDay)
	}

	if cfg.TimeFormat != "15:04" {
		t.Errorf("Wrong default time format: %s", cfg.TimeFormat)
	}

	if cfg.DateFormat != "Jan 2, 2006" {
		t.Errorf("Wrong default date format: %s", cfg.DateFormat)
	}

	if !cfg.AutoRefresh {
		t.Error("Auto refresh should be enabled by default")
	}

	if cfg.RefreshRate != 30*time.Second {
		t.Errorf("Wrong default refresh rate: %v", cfg.RefreshRate)
	}

	if len(cfg.KeyBindings) == 0 {
		t.Error("Default key bindings should not be empty")
	}

	if cfg.KeyBindings["Q"] != "quit" {
		t.Errorf("Wrong quit key binding: %s", cfg.KeyBindings["Q"])
	}
}

func TestParseLine(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		line     string
		check    func(*Config) bool
		expected bool
		hasError bool
	}{
		{
			line: "set remind_command /usr/bin/remind",
			check: func(c *Config) bool {
				return c.RemindCommand == "/usr/bin/remind"
			},
			expected: true,
			hasError: false,
		},
		{
			line: "set week_start_day sunday",
			check: func(c *Config) bool {
				return c.WeekStartDay == time.Sunday
			},
			expected: true,
			hasError: false,
		},
		{
			line: "set auto_refresh false",
			check: func(c *Config) bool {
				return !c.AutoRefresh
			},
			expected: true,
			hasError: false,
		},
		{
			line: "set refresh_rate 60",
			check: func(c *Config) bool {
				return c.RefreshRate == 60*time.Second
			},
			expected: true,
			hasError: false,
		},
		{
			line: "bind j next_day",
			check: func(c *Config) bool {
				return c.KeyBindings["j"] == "next_day"
			},
			expected: true,
			hasError: false,
		},
		{
			line: "color today yellow",
			check: func(c *Config) bool {
				return c.Colors["today"] == "yellow"
			},
			expected: true,
			hasError: false,
		},
		{
			line:     "invalid command",
			hasError: true,
		},
		{
			line:     "# comment line",
			hasError: false,
		},
		{
			line:     "",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			err := cfg.parseLine(tt.line)

			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.check != nil {
				result := tt.check(cfg)
				if result != tt.expected {
					t.Errorf("Check failed for line: %s", tt.line)
				}
			}
		})
	}
}

func TestSetVariable(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		value    string
		check    func(*Config) bool
		hasError bool
	}{
		{
			name:  "remind_files",
			value: "~/reminders.rem,/tmp/test.rem",
			check: func(c *Config) bool {
				return len(c.RemindFiles) == 2 && strings.HasSuffix(c.RemindFiles[1], "test.rem")
			},
			hasError: false,
		},
		{
			name:  "editor",
			value: "vim",
			check: func(c *Config) bool {
				return c.Editor == "vim"
			},
			hasError: false,
		},
		{
			name:  "calendar_width",
			value: "100",
			check: func(c *Config) bool {
				return c.CalendarWidth == 100
			},
			hasError: false,
		},
		{
			name:     "calendar_width",
			value:    "invalid",
			hasError: true,
		},
		{
			name:  "startup_view",
			value: "week",
			check: func(c *Config) bool {
				return c.StartupView == "week"
			},
			hasError: false,
		},
		{
			name:  "confirm_delete",
			value: "true",
			check: func(c *Config) bool {
				return c.ConfirmDelete
			},
			hasError: false,
		},
		{
			name:  "refresh_rate",
			value: "5m",
			check: func(c *Config) bool {
				return c.RefreshRate == 5*time.Minute
			},
			hasError: false,
		},
		{
			name:     "unknown_variable",
			value:    "something",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.setVariable(tt.name, tt.value)

			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.check != nil && !tt.check(cfg) {
				t.Errorf("Check failed for %s = %s", tt.name, tt.value)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_urdrc")

	content := `# Test config file
set remind_command /usr/local/bin/remind
set remind_files ~/calendar.rem,~/work.rem
set editor emacs
set week_start_day sunday
set time_format 12:00
set auto_refresh false
set refresh_rate 120

bind q quit
bind n new_event
bind ? help

color today cyan
color selected reverse
`

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg := DefaultConfig()
	err = cfg.loadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	// Verify loaded values
	if cfg.RemindCommand != "/usr/local/bin/remind" {
		t.Errorf("Wrong remind command: %s", cfg.RemindCommand)
	}
	if !cfg.RemindCommandSet {
		t.Error("RemindCommandSet is false after remind_command was set")
	}

	if len(cfg.RemindFiles) != 2 {
		t.Errorf("Wrong number of remind files: %d", len(cfg.RemindFiles))
	}

	if cfg.Editor != "emacs" {
		t.Errorf("Wrong editor: %s", cfg.Editor)
	}

	if cfg.WeekStartDay != time.Sunday {
		t.Errorf("Wrong week start day: %v", cfg.WeekStartDay)
	}

	if cfg.AutoRefresh {
		t.Error("Auto refresh should be disabled")
	}

	if cfg.RefreshRate != 120*time.Second {
		t.Errorf("Wrong refresh rate: %v", cfg.RefreshRate)
	}

	if cfg.KeyBindings["q"] != "quit" {
		t.Errorf("Wrong quit binding: %s", cfg.KeyBindings["q"])
	}

	if cfg.Colors["today"] != "cyan" {
		t.Errorf("Wrong today color: %s", cfg.Colors["today"])
	}
}

func TestGetDefaultEditor(t *testing.T) {
	// Save original env vars
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")
	defer func() {
		os.Setenv("EDITOR", origEditor)
		os.Setenv("VISUAL", origVisual)
	}()

	// Test EDITOR env var
	os.Setenv("EDITOR", "nano")
	os.Setenv("VISUAL", "")
	editor := getDefaultEditor()
	if editor != "nano" {
		t.Errorf("Expected nano, got %s", editor)
	}

	// Test VISUAL env var
	os.Setenv("EDITOR", "")
	os.Setenv("VISUAL", "code")
	editor = getDefaultEditor()
	if editor != "code" {
		t.Errorf("Expected code, got %s", editor)
	}

	// Test default
	os.Setenv("EDITOR", "")
	os.Setenv("VISUAL", "")
	editor = getDefaultEditor()
	if editor != "vi" {
		t.Errorf("Expected vi, got %s", editor)
	}
}

// TestDefaultTemplatesIncludeTimeZone tests that the default templates with an
// AT clause pin the reminder to the timezone it was created in, and that the
// lines they produce are valid remind syntax
func TestDefaultTemplatesIncludeTimeZone(t *testing.T) {
	if _, err := exec.LookPath("remind"); err != nil {
		t.Skip("remind not installed")
	}
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	cfg := DefaultConfig()
	templates := map[string]string{
		"timed_template":   cfg.TimedTemplate,
		"quick_template":   cfg.QuickTemplate,
		"allday_template":  cfg.AllDayTemplate,
		"untimed_template": cfg.UntimedTemplate,
	}
	for i, template := range cfg.Templates {
		if template != "" {
			templates[fmt.Sprintf("template%d", i)] = template
		}
	}

	for name, template := range templates {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "test.rem")
			client := remind.NewClient()
			client.SetFiles([]string{file})
			client.Timezone = chicago

			if _, err := client.AddEventFromTemplate(template, "Aug 19 2025", "16:30"); err != nil {
				t.Fatalf("AddEventFromTemplate: %v", err)
			}

			written, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			line := strings.TrimSpace(string(written))

			// A TZ clause requires an AT clause, so only timed templates carry one
			wantTZ := strings.Contains(template, "AT ")
			if hasTZ := strings.Contains(line, "TZ America/Chicago"); hasTZ != wantTZ {
				t.Errorf("TZ clause present = %v, want %v, in %q", hasTZ, wantTZ, line)
			}

			// Templates leave <++> cursor markers for the user to fill in, and
			// end at MSG; complete the reminder before handing it to remind.
			line = strings.ReplaceAll(line, "<++>", "")
			if strings.HasSuffix(line, "MSG") {
				line += " Test reminder"
			}
			if err := os.WriteFile(file, []byte(line+"\n"), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			var stderr bytes.Buffer
			cmd := exec.Command("remind", "-n", "-q", file, "Aug", "19", "2025")
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("remind failed on %q: %v\n%s", line, err, stderr.String())
			}
			if stderr.Len() > 0 {
				t.Errorf("remind complained about %q:\n%s", line, stderr.String())
			}
		})
	}
}
