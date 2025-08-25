package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"
)

// TestSelectedSlotEventsSorting tests that events in the Selected box are sorted consistently
func TestSelectedSlotEventsSorting(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)
	testTime := time.Date(2025, 8, 25, 10, 0, 0, 0, time.Local)
	laterTime := time.Date(2025, 8, 25, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name          string
		events        []remind.Event
		selectedSlot  int
		expectedOrder []string // Expected order of event descriptions
	}{
		{
			name: "Sort by time first",
			events: []remind.Event{
				{
					ID:          "2",
					Date:        baseDate,
					Time:        &laterTime,
					Description: "Later event",
					Duration:    durationPtr(60),
				},
				{
					ID:          "1",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Earlier event",
					Duration:    durationPtr(90),
				},
			},
			selectedSlot:  10, // 10:00 slot
			expectedOrder: []string{"Earlier event", "Later event"},
		},
		{
			name: "Sort by priority when times are equal",
			events: []remind.Event{
				{
					ID:          "1",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Low priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityNone,
				},
				{
					ID:          "2",
					Date:        baseDate,
					Time:        &testTime,
					Description: "High priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityHigh,
				},
				{
					ID:          "3",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Medium priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityMedium,
				},
			},
			selectedSlot:  10,
			expectedOrder: []string{"High priority", "Medium priority", "Low priority"},
		},
		{
			name: "Sort alphabetically when time and priority are equal",
			events: []remind.Event{
				{
					ID:          "1",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Zebra event",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityNone,
				},
				{
					ID:          "2",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Apple event",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityNone,
				},
				{
					ID:          "3",
					Date:        baseDate,
					Time:        &testTime,
					Description: "Banana event",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityNone,
				},
			},
			selectedSlot:  10,
			expectedOrder: []string{"Apple event", "Banana event", "Zebra event"},
		},
		{
			name: "Complex sorting with multiple criteria",
			events: []remind.Event{
				{
					ID:          "5",
					Date:        baseDate,
					Time:        &laterTime,
					Description: "Z - Later low priority",
					Duration:    durationPtr(30),
					Priority:    remind.PriorityNone,
				},
				{
					ID:          "1",
					Date:        baseDate,
					Time:        &testTime,
					Description: "B - Early high priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityHigh,
				},
				{
					ID:          "2",
					Date:        baseDate,
					Time:        &testTime,
					Description: "A - Early high priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityHigh,
				},
				{
					ID:          "3",
					Date:        baseDate,
					Time:        &testTime,
					Description: "C - Early low priority",
					Duration:    durationPtr(60),
					Priority:    remind.PriorityNone,
				},
				{
					ID:          "4",
					Date:        baseDate,
					Time:        &laterTime,
					Description: "A - Later high priority",
					Duration:    durationPtr(30),
					Priority:    remind.PriorityHigh,
				},
			},
			selectedSlot: 10,
			expectedOrder: []string{
				"A - Early high priority",
				"B - Early high priority",
				"C - Early low priority",
				"A - Later high priority",
				"Z - Later low priority",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				width:         120,
				height:        30,
				timeIncrement: 60,
				selectedDate:  baseDate,
				selectedSlot:  tt.selectedSlot,
				config:        &config.Config{},
				styles:        defaultStyles(),
				events:        tt.events,
			}

			// Render the selected slot events
			output := m.renderSelectedSlotEvents()

			// Check that events appear in the expected order
			for i := 0; i < len(tt.expectedOrder)-1; i++ {
				firstIndex := strings.Index(output, tt.expectedOrder[i])
				secondIndex := strings.Index(output, tt.expectedOrder[i+1])

				if firstIndex == -1 {
					t.Errorf("Expected event '%s' not found in output", tt.expectedOrder[i])
					continue
				}
				if secondIndex == -1 {
					t.Errorf("Expected event '%s' not found in output", tt.expectedOrder[i+1])
					continue
				}

				if firstIndex > secondIndex {
					t.Errorf("Event '%s' should appear before '%s' but doesn't",
						tt.expectedOrder[i], tt.expectedOrder[i+1])
				}
			}
		})
	}
}

// TestSelectedSlotEventsStability tests that sorting is stable across multiple calls
func TestSelectedSlotEventsStability(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)
	testTime := time.Date(2025, 8, 25, 10, 0, 0, 0, time.Local)

	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  baseDate,
		selectedSlot:  10,
		config:        &config.Config{},
		styles:        defaultStyles(),
		events: []remind.Event{
			{
				ID:          "1",
				Date:        baseDate,
				Time:        &testTime,
				Description: "Event A",
				Duration:    durationPtr(60),
			},
			{
				ID:          "2",
				Date:        baseDate,
				Time:        &testTime,
				Description: "Event B",
				Duration:    durationPtr(60),
			},
			{
				ID:          "3",
				Date:        baseDate,
				Time:        &testTime,
				Description: "Event A", // Same description as ID 1
				Duration:    durationPtr(60),
			},
		},
	}

	// Render multiple times and ensure consistency
	output1 := m.renderSelectedSlotEvents()
	output2 := m.renderSelectedSlotEvents()
	output3 := m.renderSelectedSlotEvents()

	if output1 != output2 {
		t.Error("Sorting is not stable: output differs between first and second call")
	}
	if output2 != output3 {
		t.Error("Sorting is not stable: output differs between second and third call")
	}
}
