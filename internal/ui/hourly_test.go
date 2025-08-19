package ui

import (
	"testing"
	"time"
	"urd/internal/remind"
)

func TestBuildSimpleEventLayout(t *testing.T) {
	// Create a test model
	m := &Model{
		selectedDate:  time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
		timeIncrement: 30, // 30-minute slots
		width:         100,
		styles:        DefaultStyles(),
		showEventIDs:  false,
	}

	// Test case 1: No duplicate events should appear in the same slot
	t.Run("NoDuplicatesInSlot", func(t *testing.T) {
		// Create test events - two events at the same time
		eventTime := time.Date(2025, 8, 19, 6, 30, 0, 0, time.Local)
		m.events = []remind.Event{
			{
				ID:          "evt-1",
				Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
				Time:        &eventTime,
				Description: "Move Car for Street Sweeping",
				LineNumber:  100,
			},
			{
				ID:          "evt-2", // Different ID
				Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
				Time:        &eventTime,
				Description: "Another Event",
				LineNumber:  101,
			},
		}

		slotsPerDay := 48 // 30-minute slots
		eventsBySlot, eventColumns := m.buildSimpleEventLayout(slotsPerDay)

		// Check that slot 13 (6:30 AM) has both events
		slot13 := 13 // 6:30 AM in 30-minute slots
		if len(eventsBySlot[slot13]) != 2 {
			t.Errorf("Expected 2 events in slot 13, got %d", len(eventsBySlot[slot13]))
		}

		// Check that each event appears only once
		eventCount := make(map[string]int)
		for _, event := range eventsBySlot[slot13] {
			eventCount[event.ID]++
		}

		for id, count := range eventCount {
			if count > 1 {
				t.Errorf("Event %s appears %d times in the same slot (should be 1)", id, count)
			}
		}

		// Check that events are assigned to different columns
		if eventColumns["evt-1"] == eventColumns["evt-2"] {
			t.Errorf("Two simultaneous events assigned to same column: both in column %d", eventColumns["evt-1"])
		}
	})

	// Test case 2: Same event should not appear multiple times
	t.Run("SameEventNotDuplicated", func(t *testing.T) {
		eventTime := time.Date(2025, 8, 19, 6, 30, 0, 0, time.Local)
		event := remind.Event{
			ID:          "evt-1",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &eventTime,
			Description: "Move Car for Street Sweeping",
			LineNumber:  100,
		}

		// Add the same event twice to m.events (simulating the bug)
		m.events = []remind.Event{event, event}

		slotsPerDay := 48
		eventsBySlot, _ := m.buildSimpleEventLayout(slotsPerDay)

		// Check that slot 13 has only 1 event (deduplication should work)
		slot13 := 13
		if len(eventsBySlot[slot13]) != 1 {
			t.Errorf("Expected 1 event in slot 13 after deduplication, got %d", len(eventsBySlot[slot13]))
			for i, e := range eventsBySlot[slot13] {
				t.Logf("  Event %d: ID=%s, Desc=%s", i, e.ID, e.Description)
			}
		}
	})

	// Test case 3: Events with duration should occupy multiple slots for collision detection
	t.Run("EventDurationHandling", func(t *testing.T) {
		eventTime1 := time.Date(2025, 8, 19, 9, 0, 0, 0, time.Local)
		duration1 := 2 * time.Hour
		eventTime2 := time.Date(2025, 8, 19, 10, 0, 0, 0, time.Local)

		m.events = []remind.Event{
			{
				ID:          "evt-long",
				Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
				Time:        &eventTime1,
				Duration:    &duration1, // 9:00-11:00
				Description: "Long Meeting",
				LineNumber:  100,
			},
			{
				ID:          "evt-overlap",
				Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
				Time:        &eventTime2, // 10:00 - overlaps with long meeting
				Description: "Overlapping Event",
				LineNumber:  101,
			},
		}

		slotsPerDay := 48
		_, eventColumns := m.buildSimpleEventLayout(slotsPerDay)

		// The overlapping event should be in a different column
		if eventColumns["evt-long"] == eventColumns["evt-overlap"] {
			t.Errorf("Overlapping events assigned to same column: both in column %d", eventColumns["evt-long"])
		}
	})
}

func TestEventRendering(t *testing.T) {
	// Test that events are rendered correctly without duplication
	t.Run("RenderWithoutDuplication", func(t *testing.T) {
		m := &Model{
			selectedDate:  time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			timeIncrement: 30,
			width:         120,
			height:        40,
			styles:        DefaultStyles(),
			showEventIDs:  false,
		}

		eventTime := time.Date(2025, 8, 19, 6, 30, 0, 0, time.Local)
		m.events = []remind.Event{
			{
				ID:          "evt-1",
				Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
				Time:        &eventTime,
				Description: "Move Car for Street Sweeping",
				LineNumber:  100,
			},
		}

		// Render the schedule
		output := m.renderSchedule()

		// Count how many times the event appears in the output
		eventCount := 0
		lines := splitLines(output)
		for _, line := range lines {
			if contains(line, "Move Car for Street Sweeping") {
				eventCount++
				t.Logf("Found event in line: %s", line)
			}
		}

		if eventCount > 1 {
			t.Errorf("Event appears %d times in output (should be 1)", eventCount)
		}
	})
}

// Helper functions
func splitLines(s string) []string {
	// Simple line splitter that handles ANSI codes
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func contains(s, substr string) bool {
	// Simple substring search that ignores ANSI codes
	// This is a simplified version - in production you'd strip ANSI properly
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
