package parser

import (
	"testing"
	"time"
)

func TestParseRelativeDates(t *testing.T) {
	parser := NewTimeParser()
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.Local)
	parser.SetNow(now)

	tests := []struct {
		input        string
		expectedDate time.Time
		expectedText string
		hasTime      bool
	}{
		{
			input:        "today meeting with team",
			expectedDate: time.Date(2024, 3, 15, 0, 0, 0, 0, time.Local),
			expectedText: "meeting with team",
			hasTime:      false,
		},
		{
			input:        "tomorrow 2pm dentist appointment",
			expectedDate: time.Date(2024, 3, 16, 0, 0, 0, 0, time.Local),
			expectedText: "dentist appointment",
			hasTime:      true,
		},
		{
			input:        "next monday submit report",
			expectedDate: time.Date(2024, 3, 18, 0, 0, 0, 0, time.Local),
			expectedText: "submit report",
			hasTime:      false,
		},
		{
			input:        "in 3 days project deadline",
			expectedDate: time.Date(2024, 3, 18, 0, 0, 0, 0, time.Local),
			expectedText: "project deadline",
			hasTime:      false,
		},
		{
			input:        "2 weeks from now vacation starts",
			expectedDate: time.Date(2024, 3, 29, 0, 0, 0, 0, time.Local),
			expectedText: "vacation starts",
			hasTime:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !sameDate(result.Date, tt.expectedDate) {
				t.Errorf("Date mismatch: got %v, want %v", result.Date, tt.expectedDate)
			}

			if result.Text != tt.expectedText {
				t.Errorf("Text mismatch: got %q, want %q", result.Text, tt.expectedText)
			}

			if result.HasTime != tt.hasTime {
				t.Errorf("HasTime mismatch: got %v, want %v", result.HasTime, tt.hasTime)
			}
		})
	}
}

func TestParseAbsoluteDates(t *testing.T) {
	parser := NewTimeParser()
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.Local)
	parser.SetNow(now)

	tests := []struct {
		input        string
		expectedDate time.Time
		expectedText string
	}{
		{
			input:        "3/25/2024 birthday party",
			expectedDate: time.Date(2024, 3, 25, 0, 0, 0, 0, time.Local),
			expectedText: "birthday party",
		},
		{
			input:        "12-31-2024 new year's eve",
			expectedDate: time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local),
			expectedText: "new year's eve",
		},
		{
			input:        "4/1 april fools",
			expectedDate: time.Date(2024, 4, 1, 0, 0, 0, 0, time.Local),
			expectedText: "april fools",
		},
		{
			input:        "May 15, 2024 conference",
			expectedDate: time.Date(2024, 5, 15, 0, 0, 0, 0, time.Local),
			expectedText: "conference",
		},
		{
			input:        "december 25 christmas",
			expectedDate: time.Date(2024, 12, 25, 0, 0, 0, 0, time.Local),
			expectedText: "christmas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !sameDate(result.Date, tt.expectedDate) {
				t.Errorf("Date mismatch: got %v, want %v", result.Date, tt.expectedDate)
			}

			if result.Text != tt.expectedText {
				t.Errorf("Text mismatch: got %q, want %q", result.Text, tt.expectedText)
			}
		})
	}
}

func TestParseTimes(t *testing.T) {
	parser := NewTimeParser()
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.Local)
	parser.SetNow(now)

	tests := []struct {
		input        string
		expectedHour int
		expectedMin  int
		expectedText string
	}{
		{
			input:        "2pm meeting",
			expectedHour: 14,
			expectedMin:  0,
			expectedText: "meeting",
		},
		{
			input:        "14:30 conference call",
			expectedHour: 14,
			expectedMin:  30,
			expectedText: "conference call",
		},
		{
			input:        "at 9am standup",
			expectedHour: 9,
			expectedMin:  0,
			expectedText: "standup",
		},
		{
			input:        "noon lunch",
			expectedHour: 12,
			expectedMin:  0,
			expectedText: "lunch",
		},
		{
			input:        "midnight deadline",
			expectedHour: 0,
			expectedMin:  0,
			expectedText: "deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !result.HasTime {
				t.Fatal("Expected time to be parsed")
			}

			if result.Time.Hour() != tt.expectedHour {
				t.Errorf("Hour mismatch: got %d, want %d", result.Time.Hour(), tt.expectedHour)
			}

			if result.Time.Minute() != tt.expectedMin {
				t.Errorf("Minute mismatch: got %d, want %d", result.Time.Minute(), tt.expectedMin)
			}

			if result.Text != tt.expectedText {
				t.Errorf("Text mismatch: got %q, want %q", result.Text, tt.expectedText)
			}
		})
	}
}

func TestParseTimeRanges(t *testing.T) {
	parser := NewTimeParser()
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.Local)
	parser.SetNow(now)

	tests := []struct {
		input            string
		expectedHour     int
		expectedDuration time.Duration
		expectedText     string
	}{
		{
			input:            "2pm-4pm workshop",
			expectedHour:     14,
			expectedDuration: 2 * time.Hour,
			expectedText:     "workshop",
		},
		{
			input:            "9:00-10:30 meeting",
			expectedHour:     9,
			expectedDuration: 90 * time.Minute,
			expectedText:     "meeting",
		},
		{
			input:            "1pm-2pm lunch break",
			expectedHour:     13,
			expectedDuration: 1 * time.Hour,
			expectedText:     "lunch break",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !result.HasTime {
				t.Fatal("Expected time to be parsed")
			}

			if result.Time.Hour() != tt.expectedHour {
				t.Errorf("Hour mismatch: got %d, want %d", result.Time.Hour(), tt.expectedHour)
			}

			if result.Duration != tt.expectedDuration {
				t.Errorf("Duration mismatch: got %v, want %v", result.Duration, tt.expectedDuration)
			}

			if result.Text != tt.expectedText {
				t.Errorf("Text mismatch: got %q, want %q", result.Text, tt.expectedText)
			}
		})
	}
}

func TestParseCombinations(t *testing.T) {
	parser := NewTimeParser()
	now := time.Date(2024, 3, 15, 10, 0, 0, 0, time.Local)
	parser.SetNow(now)

	tests := []struct {
		input        string
		expectedDate time.Time
		expectedHour int
		expectedText string
	}{
		{
			input:        "tomorrow at 3pm doctor appointment",
			expectedDate: time.Date(2024, 3, 16, 0, 0, 0, 0, time.Local),
			expectedHour: 15,
			expectedText: "doctor appointment",
		},
		{
			input:        "next friday 2:30pm team meeting",
			expectedDate: time.Date(2024, 3, 22, 0, 0, 0, 0, time.Local),
			expectedHour: 14,
			expectedText: "team meeting",
		},
		{
			input:        "May 20, 2024 at noon graduation",
			expectedDate: time.Date(2024, 5, 20, 0, 0, 0, 0, time.Local),
			expectedHour: 12,
			expectedText: "graduation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parser.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !sameDate(result.Date, tt.expectedDate) {
				t.Errorf("Date mismatch: got %v, want %v", result.Date, tt.expectedDate)
			}

			if !result.HasTime {
				t.Fatal("Expected time to be parsed")
			}

			if result.Time.Hour() != tt.expectedHour {
				t.Errorf("Hour mismatch: got %d, want %d", result.Time.Hour(), tt.expectedHour)
			}

			if result.Text != tt.expectedText {
				t.Errorf("Text mismatch: got %q, want %q", result.Text, tt.expectedText)
			}
		})
	}
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}