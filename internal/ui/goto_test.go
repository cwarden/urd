package ui

import (
	"fmt"
	"github.com/cwarden/urd/internal/remind"
	"testing"
	"time"
)

func TestGotoDateParsing(t *testing.T) {
	// Set a fixed "now" time for consistent testing
	now := time.Date(2025, 8, 20, 14, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "YYYY-MM-DD format",
			input:    "2020-10-15",
			expected: time.Date(2020, 10, 15, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "MM/DD/YYYY format",
			input:    "12/25/2024",
			expected: time.Date(2024, 12, 25, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "M/D/YYYY format",
			input:    "1/1/2025",
			expected: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "MM/DD format (current year)",
			input:    "12/25",
			expected: time.Date(2025, 12, 25, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "M/D format (current year)",
			input:    "1/1",
			expected: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "Natural language - today",
			input:    "today",
			expected: time.Date(2025, 8, 20, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:     "Natural language - tomorrow",
			input:    "tomorrow",
			expected: time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
			wantErr:  false,
		},
		{
			name:    "Invalid format",
			input:   "not-a-date-at-all-xyz",
			wantErr: true,
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseGotoInput(tt.input, now)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseGotoInput() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseGotoInput() unexpected error: %v", err)
				return
			}

			if !result.Equal(tt.expected) {
				t.Errorf("parseGotoInput() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// parseGotoInput extracts the date parsing logic from handleGotoDateKeys for testing
func parseGotoInput(input string, now time.Time) (time.Time, error) {
	if input == "" {
		return time.Time{}, fmt.Errorf("empty input")
	}

	// Try standard date formats FIRST
	dateFormats := []string{
		"2006-01-02", // YYYY-MM-DD
		"01/02/2006", // MM/DD/YYYY
		"1/2/2006",   // M/D/YYYY
		"01/02",      // MM/DD (current year)
		"1/2",        // M/D (current year)
	}

	for _, format := range dateFormats {
		if pd, err := time.ParseInLocation(format, input, time.Local); err == nil {
			// For MM/DD formats without year, use current year
			if format == "01/02" || format == "1/2" {
				return time.Date(now.Year(), pd.Month(), pd.Day(),
					0, 0, 0, 0, time.Local), nil
			} else {
				// Ensure the date is in local timezone with time at midnight
				return time.Date(pd.Year(), pd.Month(), pd.Day(),
					0, 0, 0, 0, time.Local), nil
			}
		}
	}

	// If standard formats failed, try natural language parsing
	parser := &remind.TimeParser{Now: now, Location: time.Local}
	date, err := parser.ParseDateOnly(input)
	if err == nil {
		return date, nil
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", input)
}
