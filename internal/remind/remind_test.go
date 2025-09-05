package remind

import (
	"strings"
	"testing"
	"time"
)

func TestParseRemindOutput(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name: "timed and untimed events",
			output: `2024/03/15 * * * 540 09:00 Morning standup
2024/03/15 * * * * All day event
2024/03/15 * * * 870 14:30 Team meeting
2024/03/16 * * * * Weekend task`,
			expected: 4,
		},
		{
			name: "events with priorities",
			output: `2024/03/15 * * * 600 10:00 Regular meeting
2024/03/15 * * * 840 14:00 Important deadline!!
2024/03/15 * * * 960 16:00 Critical issue!!!`,
			expected: 3,
		},
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name: "events with tags",
			output: `2024/03/15 * * * 540 09:00 Review PR @work @code
2024/03/15 * * * 660 11:00 Doctor appointment @personal @health`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := client.parseRemindOutput(tt.output)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(events) != tt.expected {
				t.Errorf("Event count mismatch: got %d, want %d", len(events), tt.expected)
			}
		})
	}
}

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

func TestParseDifferentDateFormats(t *testing.T) {
	client := NewClient()

	output := `2024/03/15 * * * 540 09:00 Morning meeting
2024/03/15 * * * * All day conference
2024/03/16 * * * 870 14:30 Afternoon workshop
2024/03/17 * * * * Weekend project`

	events, err := client.parseRemindOutput(output)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("Expected 4 events, got %d", len(events))
	}

	// Check first timed event
	if events[0].Time == nil {
		t.Error("Expected first event to have time")
	} else if events[0].Time.Hour() != 9 || events[0].Time.Minute() != 0 {
		t.Errorf("Wrong time for first event: %v", events[0].Time)
	}

	// Check first untimed event
	if events[1].Time != nil {
		t.Error("Expected second event to be untimed")
	}

	// Check descriptions
	expectedDescs := []string{
		"Morning meeting",
		"All day conference",
		"Afternoon workshop",
		"Weekend project",
	}

	for i, event := range events {
		if event.Description != expectedDescs[i] {
			t.Errorf("Event %d description mismatch: got %q, want %q",
				i, event.Description, expectedDescs[i])
		}
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
