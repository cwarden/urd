package remind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestP2ClientTaskParsing(t *testing.T) {
	client := NewP2Client()

	testCases := []struct {
		name     string
		task     P2Task
		expected Event
	}{
		{
			name: "basic task with scheduled time",
			task: P2Task{
				ID:             "123",
				Name:           "Test Task",
				Description:    "Test description",
				PackageID:      "test-package",
				User:           "testuser",
				EstimateLow:    2,
				EstimateHigh:   4,
				Done:           false,
				OnHold:         false,
				ScheduledStart: timePtr(time.Date(2025, 8, 21, 10, 0, 0, 0, time.Local)),
				ExpectedEnd:    timePtr(time.Date(2025, 8, 21, 14, 0, 0, 0, time.Local)),
			},
			expected: Event{
				ID:          "p2-123",
				Description: "Test Task",
				Body:        "Test description",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 8, 21, 10, 0, 0, 0, time.Local)),
				Duration:    durationPtr(3 * time.Hour),
				Tags:        []string{"test-package", "@testuser"},
			},
		},
		{
			name: "task without user",
			task: P2Task{
				ID:             "456",
				Name:           "No User Task",
				PackageID:      "default",
				EstimateLow:    1,
				EstimateHigh:   1,
				ScheduledStart: timePtr(time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local)),
			},
			expected: Event{
				ID:          "p2-456",
				Description: "No User Task",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Duration:    durationPtr(1 * time.Hour),
				Tags:        []string{},
			},
		},
		{
			name: "completed task",
			task: P2Task{
				ID:             "789",
				Name:           "Done Task",
				PackageID:      "test",
				Done:           true,
				ScheduledStart: timePtr(time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local)),
			},
			expected: Event{
				ID:          "p2-789",
				Description: "Done Task",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Tags:        []string{"test", "DONE"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.taskToEvent(tc.task)

			if result.ID != tc.expected.ID {
				t.Errorf("ID mismatch: got %s, want %s", result.ID, tc.expected.ID)
			}
			if result.Description != tc.expected.Description {
				t.Errorf("Description mismatch: got %s, want %s", result.Description, tc.expected.Description)
			}
			if result.Body != tc.expected.Body {
				t.Errorf("Body mismatch: got %s, want %s", result.Body, tc.expected.Body)
			}
			if result.Type != tc.expected.Type {
				t.Errorf("Type mismatch: got %v, want %v", result.Type, tc.expected.Type)
			}
			if !result.Date.Equal(tc.expected.Date) {
				t.Errorf("Date mismatch: got %v, want %v", result.Date, tc.expected.Date)
			}

			// Check Time
			if tc.expected.Time == nil && result.Time != nil {
				t.Errorf("Expected nil Time, got %v", result.Time)
			} else if tc.expected.Time != nil && result.Time == nil {
				t.Errorf("Expected Time %v, got nil", tc.expected.Time)
			} else if tc.expected.Time != nil && result.Time != nil {
				if !tc.expected.Time.Equal(*result.Time) {
					t.Errorf("Time mismatch: got %v, want %v", result.Time, tc.expected.Time)
				}
			}

			// Check Duration
			if tc.expected.Duration == nil && result.Duration != nil {
				t.Errorf("Expected nil Duration, got %v", result.Duration)
			} else if tc.expected.Duration != nil && result.Duration == nil {
				t.Errorf("Expected Duration %v, got nil", tc.expected.Duration)
			} else if tc.expected.Duration != nil && result.Duration != nil {
				if *tc.expected.Duration != *result.Duration {
					t.Errorf("Duration mismatch: got %v, want %v", *result.Duration, *tc.expected.Duration)
				}
			}

			// Check Tags
			if len(result.Tags) != len(tc.expected.Tags) {
				t.Errorf("Tags length mismatch: got %d, want %d", len(result.Tags), len(tc.expected.Tags))
			} else {
				for i, tag := range tc.expected.Tags {
					if i >= len(result.Tags) || result.Tags[i] != tag {
						t.Errorf("Tag mismatch at index %d: got %v, want %s", i, result.Tags, tag)
					}
				}
			}
		})
	}
}

func TestP2ClientWithMockCommand(t *testing.T) {
	// Create a mock p2 command that outputs test data
	mockScript := filepath.Join(t.TempDir(), "mock_p2")
	mockContent := `#!/bin/sh
cat <<EOF
{"id":"1","name":"Test Task 1","package_id":"test","estimate_low":1,"estimate_high":2,"done":false,"on_hold":false,"scheduled_start":"2025-08-21T10:00:00-05:00"}
{"id":"2","name":"Test Task 2","package_id":"test","user":"testuser","estimate_low":2,"estimate_high":4,"done":false,"on_hold":false,"scheduled_start":"2025-08-21T14:00:00-05:00"}
{"id":"3","name":"Done Task","package_id":"test","estimate_low":1,"estimate_high":1,"done":true,"on_hold":false}
{"id":"4","name":"On Hold Task","package_id":"test","estimate_low":1,"estimate_high":1,"done":false,"on_hold":true,"scheduled_start":"2025-08-21T10:00:00-05:00"}
{"id":"5","name":"Future Task","package_id":"test","estimate_low":1,"estimate_high":1,"done":false,"on_hold":false,"scheduled_start":"2025-08-22T10:00:00-05:00"}
EOF
`
	if err := os.WriteFile(mockScript, []byte(mockContent), 0755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	// Create P2 client with mock
	client := NewP2Client()
	client.P2Path = mockScript
	client.SetFiles([]string{"dummy.rec"})

	// Get events for a specific date
	start := time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 8, 21, 23, 59, 59, 0, time.Local)

	events, err := client.GetEvents(start, end)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should get 2 events (not the done one, on hold one, or future one)
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
		for i, e := range events {
			t.Logf("Event %d: %s (done=%v, onhold=%v, date=%s)",
				i, e.Description,
				containsTag(e.Tags, "DONE"),
				false, // We don't track on_hold in tags
				e.Date.Format("2006-01-02"))
		}
	}

	// Check first event
	if len(events) > 0 {
		event := events[0]
		if event.Description != "Test Task 1" {
			t.Errorf("Expected description 'Test Task 1', got '%s'", event.Description)
		}
		if event.ID != "p2-1" {
			t.Errorf("Expected ID 'p2-1', got '%s'", event.ID)
		}
		if event.Type != EventTodo {
			t.Errorf("Expected type EventTodo, got %v", event.Type)
		}

		// Check tags
		if !containsTag(event.Tags, "test") {
			t.Errorf("Expected package tag 'test' in tags %v", event.Tags)
		}
	}

	// Check second event has user tag
	if len(events) > 1 {
		event := events[1]
		if !containsTag(event.Tags, "@testuser") {
			t.Errorf("Expected user tag '@testuser' in tags %v", event.Tags)
		}
	}
}

func TestP2ClientJSONParsing(t *testing.T) {
	// Test parsing of JSON lines
	jsonLines := []string{
		`{"id":"1","name":"Task 1","package_id":"pkg1","estimate_low":1,"estimate_high":2,"done":false,"on_hold":false}`,
		`{"id":"2","name":"Task 2","package_id":"pkg2","user":"user1","estimate_low":3,"estimate_high":5,"done":false,"on_hold":false,"scheduled_start":"2025-08-21T10:00:00Z"}`,
		`{"invalid json`,
		`{"id":"3","name":"Task 3","package_id":"pkg3","done":true,"on_hold":false}`,
	}

	validTasks := 0
	for _, line := range jsonLines {
		var task P2Task
		if err := json.Unmarshal([]byte(line), &task); err == nil {
			validTasks++
		}
	}

	if validTasks != 3 {
		t.Errorf("Expected 3 valid tasks, got %d", validTasks)
	}
}

func TestCompositeSource(t *testing.T) {
	// Create mock sources
	source1 := &mockSource{
		events: []Event{
			{ID: "evt-1", Description: "Event 1", Date: time.Now()},
			{ID: "evt-2", Description: "Event 2", Date: time.Now()},
		},
	}

	source2 := &mockSource{
		events: []Event{
			{ID: "p2-1", Description: "Task 1", Date: time.Now()},
			{ID: "p2-2", Description: "Task 2", Date: time.Now()},
		},
	}

	// Create composite
	composite := NewCompositeSource(source1, source2)

	// Get events
	start := time.Now().AddDate(0, 0, -1)
	end := time.Now().AddDate(0, 0, 1)

	events, err := composite.GetEvents(start, end)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should get all 4 events
	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}

	// Check for duplicates
	idMap := make(map[string]bool)
	for _, event := range events {
		if idMap[event.ID] {
			t.Errorf("Duplicate event ID found: %s", event.ID)
		}
		idMap[event.ID] = true
	}
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Mock source for testing composite
type mockSource struct {
	events []Event
}

func (m *mockSource) GetEvents(start, end time.Time) ([]Event, error) {
	var result []Event
	for _, e := range m.events {
		if !e.Date.Before(start) && !e.Date.After(end) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockSource) SetFiles(files []string) {}

func (m *mockSource) WatchFiles() (<-chan FileChangeEvent, error) {
	return nil, nil
}

func (m *mockSource) StopWatching() error {
	return nil
}
