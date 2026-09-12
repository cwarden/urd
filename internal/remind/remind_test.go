package remind

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseEventDetails(t *testing.T) {
	client := NewClient()

	tests := []struct {
		desc             string
		expectedDesc     string
		expectedPriority Priority
		expectedTags     []string
	}{
		{
			desc:             "Simple event",
			expectedDesc:     "Simple event",
			expectedPriority: PriorityNone,
			expectedTags:     []string{},
		},
		{
			desc:             "High priority event!!!",
			expectedDesc:     "High priority event",
			expectedPriority: PriorityHigh,
			expectedTags:     []string{},
		},
		{
			desc:             "Medium priority!!",
			expectedDesc:     "Medium priority",
			expectedPriority: PriorityMedium,
			expectedTags:     []string{},
		},
		{
			desc:             "Low priority task!",
			expectedDesc:     "Low priority task",
			expectedPriority: PriorityLow,
			expectedTags:     []string{},
		},
		{
			desc:             "Meeting @work @important",
			expectedDesc:     "Meeting",
			expectedPriority: PriorityNone,
			expectedTags:     []string{"work", "important"},
		},
		{
			desc:             "Urgent task!! @home @chores",
			expectedDesc:     "Urgent task",
			expectedPriority: PriorityMedium,
			expectedTags:     []string{"home", "chores"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			desc, priority, tags := client.parseEventDetails(tt.desc)

			if desc != tt.expectedDesc {
				t.Errorf("Description mismatch: got %q, want %q", desc, tt.expectedDesc)
			}

			if priority != tt.expectedPriority {
				t.Errorf("Priority mismatch: got %v, want %v", priority, tt.expectedPriority)
			}

			if len(tags) != len(tt.expectedTags) {
				t.Errorf("Tag count mismatch: got %d, want %d", len(tags), len(tt.expectedTags))
			}

			for i, tag := range tags {
				if i < len(tt.expectedTags) && tag != tt.expectedTags[i] {
					t.Errorf("Tag mismatch at index %d: got %q, want %q", i, tag, tt.expectedTags[i])
				}
			}
		})
	}
}

func TestGenerateEventID(t *testing.T) {
	client := NewClient()

	event1 := Event{
		Date:        time.Date(2024, 3, 15, 0, 0, 0, 0, time.Local),
		Description: "Test event",
	}

	event2 := Event{
		Date:        time.Date(2024, 3, 15, 0, 0, 0, 0, time.Local),
		Description: "Test event",
	}

	event3 := Event{
		Date:        time.Date(2024, 3, 16, 0, 0, 0, 0, time.Local),
		Description: "Different event",
	}

	id1 := client.generateEventID(event1)
	id2 := client.generateEventID(event2)
	id3 := client.generateEventID(event3)

	// Same events should generate same ID
	if id1 != id2 {
		t.Errorf("Same events generated different IDs: %s vs %s", id1, id2)
	}

	// Different events should generate different IDs
	if id1 == id3 {
		t.Errorf("Different events generated same ID: %s", id1)
	}

	// IDs should have expected format
	if !strings.HasPrefix(id1, "evt-") {
		t.Errorf("ID doesn't have expected prefix: %s", id1)
	}
}

func TestParseRemindNextOutput(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name     string
		output   string
		expected []Event
	}{
		{
			name: "mixed timed and untimed events",
			output: `2025/12/24 Christmas Eve
2025/12/25 10:00 Christmas Brunch
2025/12/25 Christmas Day
2026/01/01 New Year's Day`,
			expected: []Event{
				{
					Date:        time.Date(2025, 12, 24, 0, 0, 0, 0, time.Local),
					Description: "Christmas Eve",
				},
				{
					Date:        time.Date(2025, 12, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(time.Date(2025, 12, 25, 10, 0, 0, 0, time.Local)),
					Description: "Christmas Brunch",
				},
				{
					Date:        time.Date(2025, 12, 25, 0, 0, 0, 0, time.Local),
					Description: "Christmas Day",
				},
				{
					Date:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
					Description: "New Year's Day",
				},
			},
		},
		{
			name: "events with priorities and tags",
			output: `2025/08/29 18:00 Dinner @home
2025/08/30 09:00 Important Meeting!! @work
2025/08/31 All day task!!! @urgent`,
			expected: []Event{
				{
					Date:        time.Date(2025, 8, 29, 0, 0, 0, 0, time.Local),
					Time:        timePtr(time.Date(2025, 8, 29, 18, 0, 0, 0, time.Local)),
					Description: "Dinner",
					Tags:        []string{"home"},
				},
				{
					Date:        time.Date(2025, 8, 30, 0, 0, 0, 0, time.Local),
					Time:        timePtr(time.Date(2025, 8, 30, 9, 0, 0, 0, time.Local)),
					Description: "Important Meeting",
					Priority:    PriorityMedium,
					Tags:        []string{"work"},
				},
				{
					Date:        time.Date(2025, 8, 31, 0, 0, 0, 0, time.Local),
					Description: "All day task",
					Priority:    PriorityHigh,
					Tags:        []string{"urgent"},
				},
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: []Event{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := client.parseRemindNextOutput(tt.output)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(events) != len(tt.expected) {
				t.Fatalf("Event count mismatch: got %d, want %d", len(events), len(tt.expected))
			}

			for i, event := range events {
				expected := tt.expected[i]

				if !event.Date.Equal(expected.Date) {
					t.Errorf("Event %d: Date mismatch: got %v, want %v", i, event.Date, expected.Date)
				}

				if (event.Time == nil) != (expected.Time == nil) {
					t.Errorf("Event %d: Time nil mismatch", i)
				} else if event.Time != nil && !event.Time.Equal(*expected.Time) {
					t.Errorf("Event %d: Time mismatch: got %v, want %v", i, event.Time, expected.Time)
				}

				if event.Description != expected.Description {
					t.Errorf("Event %d: Description mismatch: got %q, want %q", i, event.Description, expected.Description)
				}

				if event.Priority != expected.Priority {
					t.Errorf("Event %d: Priority mismatch: got %v, want %v", i, event.Priority, expected.Priority)
				}

				if !slicesEqual(event.Tags, expected.Tags) {
					t.Errorf("Event %d: Tags mismatch: got %v, want %v", i, event.Tags, expected.Tags)
				}
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseRemindError(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name         string
		output       string
		expectError  bool
		expectedFile string
		expectedLine int
		expectedMsg  string
	}{
		{
			name:         "undefined function error",
			output:       "reminders.rem(6): Undefined function: `ack'",
			expectError:  true,
			expectedFile: "reminders.rem",
			expectedLine: 6,
			expectedMsg:  "Undefined function: `ack'",
		},
		{
			name:         "expecting valid expression",
			output:       "test.rem(10): Expecting valid expression",
			expectError:  true,
			expectedFile: "test.rem",
			expectedLine: 10,
			expectedMsg:  "Expecting valid expression",
		},
		{
			name:         "parse error with path",
			output:       "/home/user/.reminders(42): Parse error",
			expectError:  true,
			expectedFile: "/home/user/.reminders",
			expectedLine: 42,
			expectedMsg:  "Parse error",
		},
		{
			name:         "multiple lines with error",
			output:       "Some other output\nreminders.rem(3): Unknown keyword\nMore output",
			expectError:  true,
			expectedFile: "reminders.rem",
			expectedLine: 3,
			expectedMsg:  "Unknown keyword",
		},
		{
			name:        "no error in output",
			output:      "Regular remind output without errors",
			expectError: false,
		},
		{
			name:         "error keyword without proper format",
			output:       "An error occurred but not in remind format",
			expectError:  true,
			expectedFile: "",
			expectedLine: 0,
			expectedMsg:  "An error occurred but not in remind format",
		},
		{
			name:        "empty output",
			output:      "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.parseRemindError(tt.output)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectError && err != nil {
				syntaxErr, ok := err.(*RemindSyntaxError)
				if !ok {
					t.Errorf("Expected RemindSyntaxError, got %T", err)
					return
				}

				if syntaxErr.File != tt.expectedFile {
					t.Errorf("File mismatch: got %q, want %q", syntaxErr.File, tt.expectedFile)
				}

				if syntaxErr.Line != tt.expectedLine {
					t.Errorf("Line mismatch: got %d, want %d", syntaxErr.Line, tt.expectedLine)
				}

				if syntaxErr.Message != tt.expectedMsg {
					t.Errorf("Message mismatch: got %q, want %q", syntaxErr.Message, tt.expectedMsg)
				}
			}
		})
	}
}

func TestRemindSyntaxErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      RemindSyntaxError
		expected string
	}{
		{
			name: "with file and line",
			err: RemindSyntaxError{
				File:    "test.rem",
				Line:    42,
				Message: "Undefined function",
			},
			expected: "test.rem:42: Undefined function",
		},
		{
			name: "without line number",
			err: RemindSyntaxError{
				File:    "test.rem",
				Line:    0,
				Message: "General error",
			},
			expected: "test.rem: General error",
		},
		{
			name: "without file",
			err: RemindSyntaxError{
				File:    "",
				Line:    0,
				Message: "Unknown error",
			},
			expected: ": Unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error string mismatch: got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestConvertJSONMultiDayEvent(t *testing.T) {
	// Test JSON entries for a 10-hour event split across two days
	entries := []RemindEntry{
		{
			Date:          "2025-09-21",
			Filename:      "/tmp/test.rem",
			LineNo:        1,
			Duration:      intPtr(540), // 9 hours for day 1 (15:00 to 00:00)
			Time:          intPtr(900), // 15:00
			EventDuration: intPtr(600), // Total event duration (10 hours)
			EventStart:    "2025-09-21T15:00",
			Priority:      5000,
			RawBody:       "Multi-day meeting",
			Body:          "3:00pm-1:00am+1 Multi-day meeting",
		},
		{
			Date:          "2025-09-22",
			Filename:      "/tmp/test.rem",
			LineNo:        1,
			Duration:      intPtr(60),  // 1 hour for day 2 (00:00 to 01:00)
			Time:          intPtr(0),   // 00:00
			EventDuration: intPtr(600), // Total event duration (10 hours)
			EventStart:    "2025-09-21T15:00",
			Priority:      5000,
			RawBody:       "Multi-day meeting",
			Body:          "12:00-1:00am Multi-day meeting",
		},
	}

	events := ConvertJSONToEvents(entries, time.Local)

	// Should only get 1 event (the continuation should be skipped)
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d events", len(events))
		for i, e := range events {
			durStr := "nil"
			if e.Duration != nil {
				durStr = e.Duration.String()
			}
			timeStr := "nil"
			if e.Time != nil {
				timeStr = e.Time.Format("15:04")
			}
			t.Logf("Event %d: %s on %s at %s, duration: %s",
				i+1, e.Description, e.Date.Format("2006-01-02"), timeStr, durStr)
		}
	}

	if len(events) > 0 {
		event := events[0]
		// Check the event details - should extract description from Body
		if !strings.Contains(event.Description, "Multi-day meeting") {
			t.Errorf("Wrong description: %q", event.Description)
		}

		// Should start at 3pm on Sep 21
		if event.Time == nil {
			t.Error("Event should have a time")
		} else if event.Time.Hour() != 15 || event.Time.Minute() != 0 {
			t.Errorf("Wrong start time: %v", event.Time)
		}

		// Should have 10 hours total duration (from EventDuration)
		if event.Duration == nil {
			t.Error("Event should have duration")
		} else if event.Duration.Hours() != 10 {
			t.Errorf("Wrong duration: got %v, want 10 hours", event.Duration)
		}
	}
}

// Helper function for tests
func intPtr(i int) *int {
	return &i
}

func TestConvertJSONStripsBodyMarkers(t *testing.T) {
	entries := []RemindEntry{
		{
			Date:     "2025-09-21",
			Filename: "/tmp/test.rem",
			LineNo:   1,
			Time:     intPtr(540),
			Priority: 5000,
			RawBody:  `%"Check restic%"%`,
			Body:     `9:00am %"Check restic%"`,
		},
		{
			Date:     "2025-09-21",
			Filename: "/tmp/test.rem",
			LineNo:   2,
			Priority: 5000,
			RawBody:  `%"Renew passport online%"%`,
			Body:     `%"Renew passport online%"`,
		},
		{
			Date:     "2025-09-21",
			Filename: "/tmp/test.rem",
			LineNo:   3,
			Time:     intPtr(1140),
			Priority: 5000,
			RawBody:  `%"Record Time in Zeph: Off work%" [t()]%`,
			Body:     `7:00pm %"Record Time in Zeph: Off work%" today at 7:00pm`,
		},
		{
			Date:     "2025-09-21",
			Filename: "/tmp/test.rem",
			LineNo:   4,
			Time:     intPtr(1080),
			Priority: 5000,
			RawBody:  `%"Dinner%" [t()]%`,
			Body:     `6:00pm %"Dinner%" today at 6:00pm`,
		},
	}

	events := ConvertJSONToEvents(entries, time.Local)

	if len(events) != 4 {
		t.Fatalf("Expected 4 events, got %d", len(events))
	}

	expected := []string{
		"Check restic",
		"Renew passport online",
		"Record Time in Zeph: Off work",
		"Dinner",
	}
	for i, want := range expected {
		if events[i].Description != want {
			t.Errorf("Event %d: expected description %q, got %q", i, want, events[i].Description)
		}
	}
}

func TestAddEventStructWithDuration(t *testing.T) {
	// Create a temporary file for testing
	tmpFile := t.TempDir() + "/test.rem"

	client := NewClient()
	client.Files = []string{tmpFile}

	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name: "event with time and duration",
			event: Event{
				Date:        time.Date(2025, 9, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 9, 21, 12, 0, 0, 0, time.Local)),
				Duration:    durationPtr(90 * time.Minute),
				Description: "Meeting with duration",
			},
			expected: "REM Sep 21 2025 AT 12:00 DURATION 1:30 MSG Meeting with duration\n",
		},
		{
			name: "event with time but no duration",
			event: Event{
				Date:        time.Date(2025, 9, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 9, 21, 14, 30, 0, 0, time.Local)),
				Description: "Quick check-in",
			},
			expected: "REM Sep 21 2025 AT 14:30 MSG Quick check-in\n",
		},
		{
			name: "untimed event",
			event: Event{
				Date:        time.Date(2025, 9, 22, 0, 0, 0, 0, time.Local),
				Description: "All day event",
			},
			expected: "REM Sep 22 2025 MSG All day event\n",
		},
		{
			name: "event with 2 hour duration",
			event: Event{
				Date:        time.Date(2025, 10, 1, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 10, 1, 9, 0, 0, 0, time.Local)),
				Duration:    durationPtr(2 * time.Hour),
				Description: "Workshop",
			},
			expected: "REM Oct 1 2025 AT 09:00 DURATION 2:00 MSG Workshop\n",
		},
		{
			name: "event with 45 minute duration",
			event: Event{
				Date:        time.Date(2025, 10, 15, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 10, 15, 15, 15, 0, 0, time.Local)),
				Duration:    durationPtr(45 * time.Minute),
				Description: "Stand-up",
			},
			expected: "REM Oct 15 2025 AT 15:15 DURATION 0:45 MSG Stand-up\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the file
			_ = os.WriteFile(tmpFile, []byte(""), 0644)

			// Add the event
			lineNum, err := client.AddEventStruct(tt.event)
			if err != nil {
				t.Fatalf("AddEventStruct failed: %v", err)
			}

			if lineNum != 1 {
				t.Errorf("Expected line number 1, got %d", lineNum)
			}

			// Read the file and check contents
			content, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			if string(content) != tt.expected {
				t.Errorf("File content mismatch:\ngot:  %q\nwant: %q", string(content), tt.expected)
			}
		})
	}
}
