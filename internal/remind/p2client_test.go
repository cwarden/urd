package remind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestP2ClientWorkPeriodParsing(t *testing.T) {
	client := NewP2Client()

	testCases := []struct {
		name     string
		period   P2WorkPeriod
		expected Event
	}{
		{
			name: "basic work period",
			period: P2WorkPeriod{
				TaskID:     "123",
				TaskName:   "Test Task",
				PackageID:  "test-package",
				User:       "testuser",
				Start:      time.Date(2025, 8, 21, 10, 0, 0, 0, time.Local),
				End:        time.Date(2025, 8, 21, 13, 0, 0, 0, time.Local),
				Hours:      3.0,
				IsComplete: false,
				TotalHours: 5.0,
			},
			expected: Event{
				ID:          "p2-123-20250821-100000",
				Description: "Test Task (3.0/5.0h)",
				Body:        "",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 8, 21, 10, 0, 0, 0, time.Local)),
				Duration:    durationPtr(3 * time.Hour),
				Tags:        []string{"test-package", "@testuser", "PARTIAL"},
			},
		},
		{
			name: "complete work period",
			period: P2WorkPeriod{
				TaskID:     "456",
				TaskName:   "Complete Task",
				PackageID:  "default",
				Start:      time.Date(2025, 8, 21, 14, 0, 0, 0, time.Local),
				End:        time.Date(2025, 8, 21, 16, 0, 0, 0, time.Local),
				Hours:      2.0,
				IsComplete: true,
				TotalHours: 2.0,
			},
			expected: Event{
				ID:          "p2-456-20250821-140000",
				Description: "Complete Task",
				Body:        "",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 8, 21, 14, 0, 0, 0, time.Local)),
				Duration:    durationPtr(2 * time.Hour),
				Tags:        []string{}, // default package is not added as tag
			},
		},
		{
			name: "work period without user",
			period: P2WorkPeriod{
				TaskID:     "789",
				TaskName:   "No User Task",
				PackageID:  "backend",
				User:       "",
				Start:      time.Date(2025, 8, 21, 9, 0, 0, 0, time.Local),
				End:        time.Date(2025, 8, 21, 10, 30, 0, 0, time.Local),
				Hours:      1.5,
				IsComplete: false,
				TotalHours: 0, // No total means it's not partial
			},
			expected: Event{
				ID:          "p2-789-20250821-090000",
				Description: "No User Task",
				Body:        "",
				Type:        EventTodo,
				Priority:    PriorityNone,
				Date:        time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local),
				Time:        timePtr(time.Date(2025, 8, 21, 9, 0, 0, 0, time.Local)),
				Duration:    durationPtr(90 * time.Minute),
				Tags:        []string{"backend"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.workPeriodToEvent(tc.period)

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
				t.Errorf("Tags length mismatch: got %d (%v), want %d (%v)",
					len(result.Tags), result.Tags, len(tc.expected.Tags), tc.expected.Tags)
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
	// Create a mock p2 command that outputs test work period data
	mockScript := filepath.Join(t.TempDir(), "mock_p2")
	mockContent := `#!/bin/sh
cat <<EOF
{"task_id":"1","task_name":"Test Task 1","package_id":"test","user":"user1","start":"2025-08-21T10:00:00-05:00","end":"2025-08-21T12:00:00-05:00","hours":2.0,"is_complete":false,"total_hours":4.0}
{"task_id":"2","task_name":"Test Task 2","package_id":"test","user":"user2","start":"2025-08-21T14:00:00-05:00","end":"2025-08-21T16:30:00-05:00","hours":2.5,"is_complete":true,"total_hours":2.5}
{"task_id":"3","task_name":"Test Task 3","package_id":"backend","start":"2025-08-22T10:00:00-05:00","end":"2025-08-22T11:00:00-05:00","hours":1.0,"is_complete":false,"total_hours":3.0}
{"task_id":"4","task_name":"Test Task 4","package_id":"frontend","user":"user3","start":"2025-08-21T08:00:00-05:00","end":"2025-08-21T09:30:00-05:00","hours":1.5,"is_complete":false,"total_hours":0}
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

	// Should get 3 events for Aug 21 (not the future one on Aug 22)
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
		for i, e := range events {
			t.Logf("Event %d: %s (date=%s, time=%v)",
				i, e.Description, e.Date.Format("2006-01-02"),
				e.Time)
		}
	}

	// Check that events have correct structure
	foundPartial := false
	foundComplete := false
	for _, event := range events {
		if containsTag(event.Tags, "PARTIAL") {
			foundPartial = true
			// Check that partial task has hours indicator in description
			if event.Description == "Test Task 1 (2.0/4.0h)" {
				// Expected format
			} else {
				t.Errorf("Partial task has incorrect description: %s", event.Description)
			}
		}
		if event.Description == "Test Task 2" && !containsTag(event.Tags, "PARTIAL") {
			foundComplete = true
		}
	}

	if !foundPartial {
		t.Error("Expected to find a partial work period")
	}
	if !foundComplete {
		t.Error("Expected to find a complete work period")
	}
}

func TestP2ClientJSONParsing(t *testing.T) {
	// Test parsing of JSON lines for work periods
	jsonLines := []string{
		`{"task_id":"1","task_name":"Task 1","package_id":"pkg1","user":"user1","start":"2025-08-21T10:00:00Z","end":"2025-08-21T12:00:00Z","hours":2.0,"is_complete":false,"total_hours":5.0}`,
		`{"task_id":"2","task_name":"Task 2","package_id":"pkg2","start":"2025-08-21T14:00:00Z","end":"2025-08-21T15:30:00Z","hours":1.5,"is_complete":true,"total_hours":1.5}`,
		`{"invalid json`,
		`{"task_id":"3","task_name":"Task 3","package_id":"pkg3","user":"user2","start":"2025-08-21T16:00:00Z","end":"2025-08-21T17:00:00Z","hours":1.0,"is_complete":false,"total_hours":0}`,
	}

	validPeriods := 0
	for _, line := range jsonLines {
		var period P2WorkPeriod
		if err := json.Unmarshal([]byte(line), &period); err == nil {
			validPeriods++
		}
	}

	if validPeriods != 3 {
		t.Errorf("Expected 3 valid work periods, got %d", validPeriods)
	}
}

func TestP2ClientDateRangeFiltering(t *testing.T) {
	// Create a mock p2 command with work periods spanning multiple days
	mockScript := filepath.Join(t.TempDir(), "mock_p2")
	mockContent := `#!/bin/sh
cat <<EOF
{"task_id":"1","task_name":"Day 1 Morning","package_id":"test","start":"2025-08-20T09:00:00-05:00","end":"2025-08-20T11:00:00-05:00","hours":2.0,"is_complete":true,"total_hours":2.0}
{"task_id":"2","task_name":"Day 1 Afternoon","package_id":"test","start":"2025-08-20T14:00:00-05:00","end":"2025-08-20T17:00:00-05:00","hours":3.0,"is_complete":true,"total_hours":3.0}
{"task_id":"3","task_name":"Day 2 All Day","package_id":"test","start":"2025-08-21T08:00:00-05:00","end":"2025-08-21T17:00:00-05:00","hours":9.0,"is_complete":true,"total_hours":9.0}
{"task_id":"4","task_name":"Day 3 Morning","package_id":"test","start":"2025-08-22T10:00:00-05:00","end":"2025-08-22T12:00:00-05:00","hours":2.0,"is_complete":false,"total_hours":4.0}
EOF
`
	if err := os.WriteFile(mockScript, []byte(mockContent), 0755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	client := NewP2Client()
	client.P2Path = mockScript
	client.SetFiles([]string{"dummy.rec"})

	// Test: Get only Day 2 events
	start := time.Date(2025, 8, 21, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 8, 21, 23, 59, 59, 0, time.Local)

	events, err := client.GetEvents(start, end)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for Day 2, got %d", len(events))
		for _, e := range events {
			t.Logf("Event: %s on %s", e.Description, e.Date.Format("2006-01-02"))
		}
	}

	if len(events) > 0 && events[0].Description != "Day 2 All Day" {
		t.Errorf("Expected 'Day 2 All Day', got '%s'", events[0].Description)
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

func TestCompositeSourceDeduplication(t *testing.T) {
	// Create mock sources with overlapping IDs
	source1 := &mockSource{
		events: []Event{
			{ID: "shared-1", Description: "Event from Source 1", Date: time.Now()},
			{ID: "unique-1", Description: "Unique to Source 1", Date: time.Now()},
		},
	}

	source2 := &mockSource{
		events: []Event{
			{ID: "shared-1", Description: "Event from Source 2", Date: time.Now()}, // Same ID
			{ID: "unique-2", Description: "Unique to Source 2", Date: time.Now()},
		},
	}

	composite := NewCompositeSource(source1, source2)

	start := time.Now().AddDate(0, 0, -1)
	end := time.Now().AddDate(0, 0, 1)

	events, err := composite.GetEvents(start, end)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should get 3 events (one duplicate removed)
	if len(events) != 3 {
		t.Errorf("Expected 3 events after deduplication, got %d", len(events))
	}

	// Verify no duplicate IDs
	idMap := make(map[string]bool)
	for _, event := range events {
		if idMap[event.ID] {
			t.Errorf("Duplicate event ID found after deduplication: %s", event.ID)
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
