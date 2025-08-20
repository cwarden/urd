package remind

import (
	"testing"
	"time"
)

func TestTimeParser_Parse(t *testing.T) {
	// Fixed time for testing
	fixedTime := time.Date(2024, time.January, 15, 10, 0, 0, 0, time.Local) // Monday, Jan 15, 2024, 10:00 AM

	parser := &TimeParser{
		Now:      fixedTime,
		Location: time.Local,
	}

	tests := []struct {
		name        string
		input       string
		wantDate    time.Time
		wantHasTime bool
		wantHour    int
		wantMinute  int
		wantText    string
		wantErr     bool
	}{
		// Test cases from the examples
		{
			name:        "do something at 2pm",
			input:       "do something at 2pm",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "do something",
		},
		{
			name:        "do something at 2pm tomorrow",
			input:       "do something at 2pm tomorrow",
			wantDate:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local), // tomorrow
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "do something",
		},
		{
			name:        "do something tomorrow",
			input:       "do something tomorrow",
			wantDate:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local), // tomorrow
			wantHasTime: false,
			wantText:    "do something",
		},
		{
			name:        "do something next monday",
			input:       "do something next monday",
			wantDate:    time.Date(2024, time.January, 22, 0, 0, 0, 0, time.Local), // next Monday
			wantHasTime: false,
			wantText:    "do something",
		},
		{
			name:        "do something at 2pm next monday",
			input:       "do something at 2pm next monday",
			wantDate:    time.Date(2024, time.January, 22, 0, 0, 0, 0, time.Local), // next Monday
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "do something",
		},
		// Additional test cases
		{
			name:        "meeting with tim",
			input:       "meeting with tim",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today (default)
			wantHasTime: false,
			wantText:    "meeting with tim",
		},
		{
			name:        "call john at 3:30pm",
			input:       "call john at 3:30pm",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    15,
			wantMinute:  30,
			wantText:    "call john",
		},
		{
			name:        "lunch tomorrow at 12:30pm",
			input:       "lunch tomorrow at 12:30pm",
			wantDate:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local), // tomorrow
			wantHasTime: true,
			wantHour:    12,
			wantMinute:  30,
			wantText:    "lunch",
		},
		{
			name:        "dentist appointment at 10am",
			input:       "dentist appointment at 10am",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    10,
			wantMinute:  0,
			wantText:    "dentist appointment",
		},
		{
			name:        "review code this friday",
			input:       "review code this friday",
			wantDate:    time.Date(2024, time.January, 19, 0, 0, 0, 0, time.Local), // this Friday
			wantHasTime: false,
			wantText:    "review code",
		},
		{
			name:        "submit report next tuesday at 5pm",
			input:       "submit report next tuesday at 5pm",
			wantDate:    time.Date(2024, time.January, 23, 0, 0, 0, 0, time.Local), // next Tuesday
			wantHasTime: true,
			wantHour:    17,
			wantMinute:  0,
			wantText:    "submit report",
		},
		{
			name:        "buy groceries today",
			input:       "buy groceries today",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: false,
			wantText:    "buy groceries",
		},
		{
			name:        "exercise at 6am tomorrow",
			input:       "exercise at 6am tomorrow",
			wantDate:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local), // tomorrow
			wantHasTime: true,
			wantHour:    6,
			wantMinute:  0,
			wantText:    "exercise",
		},
		{
			name:        "team standup at 9:15am",
			input:       "team standup at 9:15am",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    9,
			wantMinute:  15,
			wantText:    "team standup",
		},
		{
			name:        "dinner with family at 7pm this saturday",
			input:       "dinner with family at 7pm this saturday",
			wantDate:    time.Date(2024, time.January, 20, 0, 0, 0, 0, time.Local), // this Saturday
			wantHasTime: true,
			wantHour:    19,
			wantMinute:  0,
			wantText:    "dinner with family",
		},
		{
			name:        "project deadline next friday",
			input:       "project deadline next friday",
			wantDate:    time.Date(2024, time.January, 26, 0, 0, 0, 0, time.Local), // next Friday
			wantHasTime: false,
			wantText:    "project deadline",
		},
		{
			name:        "2pm meeting",
			input:       "2pm meeting",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "meeting",
		},
		{
			name:        "meeting 2pm",
			input:       "meeting 2pm",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "meeting",
		},
		{
			name:        "just a plain reminder",
			input:       "just a plain reminder",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: false,
			wantText:    "just a plain reminder",
		},
		{
			name:        "do something on monday",
			input:       "do something on monday",
			wantDate:    time.Date(2024, time.January, 22, 0, 0, 0, 0, time.Local), // next Monday (Jan 15 is Monday, so next occurrence is Jan 22)
			wantHasTime: false,
			wantText:    "do something",
		},
		{
			name:        "meeting on tuesday at 2pm",
			input:       "meeting on tuesday at 2pm",
			wantDate:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local), // Tuesday Jan 16
			wantHasTime: true,
			wantHour:    14,
			wantMinute:  0,
			wantText:    "meeting",
		},
		{
			name:        "call john on friday",
			input:       "call john on friday",
			wantDate:    time.Date(2024, time.January, 19, 0, 0, 0, 0, time.Local), // Friday Jan 19
			wantHasTime: false,
			wantText:    "call john",
		},
		{
			name:        "early morning call at 3am",
			input:       "early morning call at 3am",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    3, // 3am stays as 3am
			wantMinute:  0,
			wantText:    "early morning call",
		},
		{
			name:        "late night work at 11pm",
			input:       "late night work at 11pm",
			wantDate:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local), // today
			wantHasTime: true,
			wantHour:    23,
			wantMinute:  0,
			wantText:    "late night work",
		},
		// Test edge cases
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error = %v", err)
				return
			}

			// Check date
			if !result.Date.Equal(tt.wantDate) {
				t.Errorf("Parse() Date = %v, want %v", result.Date, tt.wantDate)
			}

			// Check if has time
			if result.HasTime != tt.wantHasTime {
				t.Errorf("Parse() HasTime = %v, want %v", result.HasTime, tt.wantHasTime)
			}

			// Check time if applicable
			if tt.wantHasTime {
				if result.Time.Hour() != tt.wantHour {
					t.Errorf("Parse() Hour = %v, want %v", result.Time.Hour(), tt.wantHour)
				}
				if result.Time.Minute() != tt.wantMinute {
					t.Errorf("Parse() Minute = %v, want %v", result.Time.Minute(), tt.wantMinute)
				}
			}

			// Check description text
			if result.Text != tt.wantText {
				t.Errorf("Parse() Text = %q, want %q", result.Text, tt.wantText)
			}
		})
	}
}

func TestTimeParser_ExtractTime(t *testing.T) {
	parser := &TimeParser{
		Now:      time.Now(),
		Location: time.Local,
	}

	tests := []struct {
		name          string
		input         string
		wantFound     bool
		wantHour      int
		wantMinute    int
		wantRemaining string
	}{
		{
			name:          "at 2pm",
			input:         "meeting at 2pm tomorrow",
			wantFound:     true,
			wantHour:      14,
			wantMinute:    0,
			wantRemaining: "meeting tomorrow",
		},
		{
			name:          "at 3:30pm",
			input:         "call john at 3:30pm",
			wantFound:     true,
			wantHour:      15,
			wantMinute:    30,
			wantRemaining: "call john",
		},
		{
			name:          "just 2pm",
			input:         "2pm meeting",
			wantFound:     true,
			wantHour:      14,
			wantMinute:    0,
			wantRemaining: "meeting",
		},
		{
			name:          "24 hour format",
			input:         "appointment at 14:30",
			wantFound:     true,
			wantHour:      14,
			wantMinute:    30,
			wantRemaining: "appointment",
		},
		{
			name:          "no time",
			input:         "do something tomorrow",
			wantFound:     false,
			wantRemaining: "do something tomorrow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, hour, minute, remaining := parser.extractTime(tt.input)

			if found != tt.wantFound {
				t.Errorf("extractTime() found = %v, want %v", found, tt.wantFound)
			}
			if found {
				if hour != tt.wantHour {
					t.Errorf("extractTime() hour = %v, want %v", hour, tt.wantHour)
				}
				if minute != tt.wantMinute {
					t.Errorf("extractTime() minute = %v, want %v", minute, tt.wantMinute)
				}
			}
			if remaining != tt.wantRemaining {
				t.Errorf("extractTime() remaining = %q, want %q", remaining, tt.wantRemaining)
			}
		})
	}
}

func TestTimeParser_ExtractDate(t *testing.T) {
	fixedTime := time.Date(2024, time.January, 15, 10, 0, 0, 0, time.Local) // Monday

	parser := &TimeParser{
		Now:      fixedTime,
		Location: time.Local,
	}

	tests := []struct {
		name          string
		input         string
		wantFound     bool
		wantDate      time.Time
		wantRemaining string
	}{
		{
			name:          "tomorrow",
			input:         "do something tomorrow",
			wantFound:     true,
			wantDate:      time.Date(2024, time.January, 16, 0, 0, 0, 0, time.Local),
			wantRemaining: "do something",
		},
		{
			name:          "today",
			input:         "meeting today at 2pm",
			wantFound:     true,
			wantDate:      time.Date(2024, time.January, 15, 0, 0, 0, 0, time.Local),
			wantRemaining: "meeting at 2pm",
		},
		{
			name:          "next monday",
			input:         "submit report next monday",
			wantFound:     true,
			wantDate:      time.Date(2024, time.January, 22, 0, 0, 0, 0, time.Local),
			wantRemaining: "submit report",
		},
		{
			name:          "this friday",
			input:         "deadline this friday",
			wantFound:     true,
			wantDate:      time.Date(2024, time.January, 19, 0, 0, 0, 0, time.Local),
			wantRemaining: "deadline",
		},
		{
			name:          "no date",
			input:         "just a reminder",
			wantFound:     false,
			wantRemaining: "just a reminder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, date, remaining := parser.ExtractDate(tt.input)

			if found != tt.wantFound {
				t.Errorf("extractDate() found = %v, want %v", found, tt.wantFound)
			}
			if found && !date.Equal(tt.wantDate) {
				t.Errorf("extractDate() date = %v, want %v", date, tt.wantDate)
			}
			if remaining != tt.wantRemaining {
				t.Errorf("extractDate() remaining = %q, want %q", remaining, tt.wantRemaining)
			}
		})
	}
}
