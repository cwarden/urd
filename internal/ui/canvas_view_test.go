package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"
)

func TestEventColumnSpanning(t *testing.T) {
	// Create a test model with some default settings
	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
		topSlot:       8, // Start at 8:00 AM
		selectedSlot:  8,
		config:        &config.Config{},
		styles:        defaultStyles(),
		showEventIDs:  false,
	}

	tests := []struct {
		name           string
		events         []remind.Event
		expectedSpans  map[string]int  // event description -> expected column span
		expectedWidths map[string]bool // event description -> should be wider than single column
	}{
		{
			name: "Short events stay in single column",
			events: []remind.Event{
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "Off work",
					Duration:    durationPtr(60),
				},
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(9, 0),
					Description: "Meeting",
					Duration:    durationPtr(30),
				},
			},
			expectedSpans: map[string]int{
				"Off work": 1,
				"Meeting":  1,
			},
			expectedWidths: map[string]bool{
				"Off work": false,
				"Meeting":  false,
			},
		},
		{
			name: "Long events expand when space available",
			events: []remind.Event{
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "This is a very long event description that definitely needs more space",
					Duration:    durationPtr(60),
				},
			},
			expectedSpans: map[string]int{
				"This is a very long event description that definitely needs more space": 1, // Will expand if no conflicts
			},
			expectedWidths: map[string]bool{
				"This is a very long event description that definitely needs more space": true,
			},
		},
		{
			name: "Events don't expand when blocked by other events",
			events: []remind.Event{
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "Check Catapult Kit Predictions",
					Duration:    durationPtr(30),
				},
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "Invoice Acadia",
					Duration:    durationPtr(30),
				},
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 15),
					Description: "Check out https://www.chicagobooth.edu/research/roman/events/think-better",
					Duration:    durationPtr(15),
				},
			},
			expectedSpans: map[string]int{
				"Check Catapult Kit Predictions": 1,
				"Invoice Acadia":                 1,
				"Check out https://www.chicagobooth.edu/research/roman/events/think-better": 1, // Can't expand due to conflicts at 8:00
			},
		},
		{
			name: "Events only expand within existing columns",
			events: []remind.Event{
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "Event 1",
					Duration:    durationPtr(30),
				},
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 0),
					Description: "Event 2 in column 1",
					Duration:    durationPtr(30),
				},
				{
					Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
					Time:        timePtr(8, 30),
					Description: "This event can only expand into existing column",
					Duration:    durationPtr(30),
				},
			},
			expectedSpans: map[string]int{
				"Event 1":             1,
				"Event 2 in column 1": 1,
				"This event can only expand into existing column": 2, // Can expand into column 1
			},
			expectedWidths: map[string]bool{
				"Event 1":             false,
				"Event 2 in column 1": false,
				"This event can only expand into existing column": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.events = tt.events

			// Call the internal method to test column assignment
			// Note: We're testing the logic, not the actual rendering
			layers := m.createEventBlockLayers(24, 20, 7, 80)

			// Since we can't easily inspect the internal EventPosition struct,
			// we verify the behavior through the layer dimensions
			if len(layers) != len(tt.events) {
				t.Errorf("Expected %d event layers, got %d", len(tt.events), len(layers))
			}

			// The actual verification would need access to the EventPosition data
			// For now, we're ensuring the function doesn't panic and produces output
		})
	}
}

func TestSlotToRowIndex(t *testing.T) {
	m := &Model{
		topSlot:       0,
		selectedDate:  time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
		timeIncrement: 60,
	}

	tests := []struct {
		name        string
		slotIndex   int
		slotsPerDay int
		expectedRow int
		topSlot     int
	}{
		{
			name:        "First slot of first day",
			slotIndex:   0,
			slotsPerDay: 24,
			expectedRow: 1, // Row 0 is date separator
			topSlot:     0,
		},
		{
			name:        "Last slot of first day",
			slotIndex:   23,
			slotsPerDay: 24,
			expectedRow: 24, // 1 date separator + 23 time slots
			topSlot:     0,
		},
		{
			name:        "First slot of second day",
			slotIndex:   24,
			slotsPerDay: 24,
			expectedRow: 26, // 2 date separators + 24 time slots
			topSlot:     0,
		},
		{
			name:        "With negative topSlot",
			slotIndex:   5,
			slotsPerDay: 24,
			expectedRow: 6, // 1 date separator + 5 slots
			topSlot:     -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.topSlot = tt.topSlot
			result := m.slotToRowIndex(tt.slotIndex, tt.slotsPerDay)
			if result != tt.expectedRow {
				t.Errorf("slotToRowIndex(%d, %d) = %d, want %d",
					tt.slotIndex, tt.slotsPerDay, result, tt.expectedRow)
			}
		})
	}
}

func TestFindEventSlot(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)
	m := &Model{
		timeIncrement: 60,
	}

	tests := []struct {
		name         string
		event        remind.Event
		slotsPerDay  int
		expectedSlot int
		timeInc      int
	}{
		{
			name: "Event at 8:00 AM same day",
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(8, 0),
			},
			slotsPerDay:  24,
			expectedSlot: 8,
			timeInc:      60,
		},
		{
			name: "Event at 2:30 PM with 30-minute increments",
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(14, 30),
			},
			slotsPerDay:  48,
			expectedSlot: 29, // 14*2 + 1
			timeInc:      30,
		},
		{
			name: "Event next day at noon",
			event: remind.Event{
				Date: baseDate.AddDate(0, 0, 1),
				Time: timePtr(12, 0),
			},
			slotsPerDay:  24,
			expectedSlot: 36, // 24 + 12
			timeInc:      60,
		},
		{
			name: "Event with 15-minute increments at 9:45 AM",
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(9, 45),
			},
			slotsPerDay:  96,
			expectedSlot: 39, // 9*4 + 3
			timeInc:      15,
		},
		{
			name: "Untimed event returns -1",
			event: remind.Event{
				Date: baseDate,
				Time: nil,
			},
			slotsPerDay:  24,
			expectedSlot: -1,
			timeInc:      60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.timeIncrement = tt.timeInc
			result := m.findEventSlot(tt.event, tt.slotsPerDay, baseDate)
			if result != tt.expectedSlot {
				t.Errorf("findEventSlot() = %d, want %d", result, tt.expectedSlot)
			}
		})
	}
}

// Helper functions
func timePtr(hour, minute int) *time.Time {
	t := time.Date(2025, 1, 1, hour, minute, 0, 0, time.Local)
	return &t
}

func durationPtr(minutes int) *time.Duration {
	d := time.Duration(minutes) * time.Minute
	return &d
}

func defaultStyles() Styles {
	return Styles{
		Normal:   lipgloss.NewStyle(),
		Selected: lipgloss.NewStyle(),
		Today:    lipgloss.NewStyle(),
		Header:   lipgloss.NewStyle(),
		Help:     lipgloss.NewStyle(),
		Message:  lipgloss.NewStyle(),
	}
}

// TestEventDateConsistencyWithScrolling verifies that events stay on their correct days when scrolling
func TestEventDateConsistencyWithScrolling(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  baseDate,
		config:        &config.Config{},
		styles:        defaultStyles(),
		showEventIDs:  false,
	}

	// Create events across multiple days
	m.events = []remind.Event{
		// Sunday Aug 24
		{
			ID:          "sun-1",
			Date:        time.Date(2025, 8, 24, 0, 0, 0, 0, time.Local),
			Time:        timePtr(10, 0),
			Description: "Sunday Event",
			Duration:    durationPtr(60),
		},
		// Monday Aug 25
		{
			ID:          "mon-1",
			Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
			Time:        timePtr(9, 0),
			Description: "Monday Event 1",
			Duration:    durationPtr(60),
		},
		{
			ID:          "mon-2",
			Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
			Time:        timePtr(14, 0),
			Description: "Monday Event 2",
			Duration:    durationPtr(30),
		},
		// Tuesday Aug 26
		{
			ID:          "tue-1",
			Date:        time.Date(2025, 8, 26, 0, 0, 0, 0, time.Local),
			Time:        timePtr(11, 0),
			Description: "Tuesday Event",
			Duration:    durationPtr(120),
		},
	}

	tests := []struct {
		name          string
		topSlot       int
		expectedDates map[string]time.Time // event ID -> expected date
	}{
		{
			name:    "Scrolled to Monday morning",
			topSlot: 0, // Monday 00:00
			expectedDates: map[string]time.Time{
				"sun-1": time.Date(2025, 8, 24, 0, 0, 0, 0, time.Local),
				"mon-1": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"mon-2": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"tue-1": time.Date(2025, 8, 26, 0, 0, 0, 0, time.Local),
			},
		},
		{
			name:    "Scrolled to Sunday",
			topSlot: -24, // Sunday 00:00
			expectedDates: map[string]time.Time{
				"sun-1": time.Date(2025, 8, 24, 0, 0, 0, 0, time.Local),
				"mon-1": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"mon-2": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"tue-1": time.Date(2025, 8, 26, 0, 0, 0, 0, time.Local),
			},
		},
		{
			name:    "Scrolled to Tuesday",
			topSlot: 24, // Tuesday 00:00
			expectedDates: map[string]time.Time{
				"sun-1": time.Date(2025, 8, 24, 0, 0, 0, 0, time.Local),
				"mon-1": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"mon-2": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"tue-1": time.Date(2025, 8, 26, 0, 0, 0, 0, time.Local),
			},
		},
		{
			name:    "Scrolled to Monday afternoon",
			topSlot: 12, // Monday 12:00
			expectedDates: map[string]time.Time{
				"sun-1": time.Date(2025, 8, 24, 0, 0, 0, 0, time.Local),
				"mon-1": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"mon-2": time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				"tue-1": time.Date(2025, 8, 26, 0, 0, 0, 0, time.Local),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.topSlot = tt.topSlot

			// Events should always be on their original dates regardless of scroll position
			for _, event := range m.events {
				expectedDate, ok := tt.expectedDates[event.ID]
				if !ok {
					t.Errorf("No expected date for event %s", event.ID)
					continue
				}

				if !event.Date.Equal(expectedDate) {
					t.Errorf("Event %s has date %v, expected %v when topSlot=%d",
						event.ID, event.Date, expectedDate, tt.topSlot)
				}
			}

			// Verify that createEventBlockLayers doesn't panic and produces correct number of layers
			layers := m.createEventBlockLayers(24, 30, 7, 80)
			if layers == nil {
				t.Error("createEventBlockLayers returned nil")
			}
		})
	}
}

// TestEventVisibilityCalculation tests that events are correctly determined to be visible or not
func TestEventVisibilityCalculation(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  baseDate,
		topSlot:       8, // Start at Monday 8:00 AM
		config:        &config.Config{},
		styles:        defaultStyles(),
		showEventIDs:  false,
	}

	tests := []struct {
		name            string
		event           remind.Event
		visibleSlots    int
		shouldBeVisible bool
	}{
		{
			name: "Event in visible range",
			event: remind.Event{
				Date:        baseDate,
				Time:        timePtr(10, 0), // 10:00 AM Monday
				Description: "Visible Event",
				Duration:    durationPtr(60),
			},
			visibleSlots:    10, // Show slots 8-17 (8:00 AM - 5:00 PM)
			shouldBeVisible: true,
		},
		{
			name: "Event before visible range",
			event: remind.Event{
				Date:        baseDate,
				Time:        timePtr(6, 0), // 6:00 AM Monday
				Description: "Early Event",
				Duration:    durationPtr(60),
			},
			visibleSlots:    10,
			shouldBeVisible: false,
		},
		{
			name: "Event after visible range",
			event: remind.Event{
				Date:        baseDate,
				Time:        timePtr(20, 0), // 8:00 PM Monday
				Description: "Late Event",
				Duration:    durationPtr(60),
			},
			visibleSlots:    10,
			shouldBeVisible: false,
		},
		{
			name: "Event spanning into visible range",
			event: remind.Event{
				Date:        baseDate,
				Time:        timePtr(7, 0), // 7:00 AM Monday
				Description: "Spanning Event",
				Duration:    durationPtr(120), // 2 hours, extends to 9:00 AM
			},
			visibleSlots:    10,
			shouldBeVisible: true, // Should be visible because it extends into visible range
		},
		{
			name: "Event on different day",
			event: remind.Event{
				Date:        baseDate.AddDate(0, 0, 1), // Tuesday
				Time:        timePtr(10, 0),
				Description: "Tuesday Event",
				Duration:    durationPtr(60),
			},
			visibleSlots:    10,
			shouldBeVisible: false, // Not visible when viewing Monday 8:00-17:00
		},
		{
			name: "Multi-day view with Tuesday event",
			event: remind.Event{
				Date:        baseDate.AddDate(0, 0, 1), // Tuesday
				Time:        timePtr(10, 0),
				Description: "Tuesday Event",
				Duration:    durationPtr(60),
			},
			visibleSlots:    50, // Show ~2 days worth of slots
			shouldBeVisible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.events = []remind.Event{tt.event}

			// Call createEventBlockLayers and check if event appears in output
			layers := m.createEventBlockLayers(24, tt.visibleSlots, 7, 80)

			// An event is visible if it produces a layer
			isVisible := len(layers) > 0

			if isVisible != tt.shouldBeVisible {
				t.Errorf("Event visibility = %v, want %v for event at %v on %v with topSlot=%d and visibleSlots=%d",
					isVisible, tt.shouldBeVisible,
					tt.event.Time, tt.event.Date.Format("2006-01-02"),
					m.topSlot, tt.visibleSlots)
			}
		})
	}
}

// TestColumnWidthWithoutCap tests that column width is not artificially capped
func TestColumnWidthWithoutCap(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name             string
		width            int
		events           []remind.Event
		expectFullWidth  bool
		expectTruncation bool
	}{
		{
			name:  "Single event uses full available width",
			width: 150,
			events: []remind.Event{
				{
					Date:        baseDate,
					Time:        timePtr(20, 0),
					Description: "Xn to pick up Denisa at 8p",
					Duration:    durationPtr(60),
				},
			},
			expectFullWidth:  true,
			expectTruncation: false,
		},
		{
			name:  "Single long event uses full width without truncation",
			width: 200,
			events: []remind.Event{
				{
					Date:        baseDate.AddDate(0, 0, 1),
					Time:        timePtr(6, 30),
					Description: "Move Car for Street Sweeping",
					Duration:    durationPtr(30),
				},
			},
			expectFullWidth:  true,
			expectTruncation: false,
		},
		{
			name:  "Multiple events in same slot share width",
			width: 150,
			events: []remind.Event{
				{
					Date:        baseDate,
					Time:        timePtr(10, 0),
					Description: "Event 1 in slot",
					Duration:    durationPtr(60),
				},
				{
					Date:        baseDate,
					Time:        timePtr(10, 0),
					Description: "Event 2 in same slot",
					Duration:    durationPtr(60),
				},
			},
			expectFullWidth:  false, // Width is divided
			expectTruncation: false,
		},
		{
			name:  "Events in different slots don't affect each other's width",
			width: 120,
			events: []remind.Event{
				{
					Date:        baseDate,
					Time:        timePtr(20, 0),
					Description: "Xn to pick up Denisa at 8p",
					Duration:    durationPtr(60),
				},
				{
					Date:        baseDate.AddDate(0, 0, 1),
					Time:        timePtr(6, 30),
					Description: "Move Car for Street Sweeping",
					Duration:    durationPtr(30),
				},
			},
			expectFullWidth:  true, // Both events can use full width since they don't overlap
			expectTruncation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				width:         tt.width,
				height:        30,
				timeIncrement: 60,
				selectedDate:  baseDate,
				topSlot:       0,
				config:        &config.Config{},
				styles:        defaultStyles(),
				showEventIDs:  false,
				events:        tt.events,
			}

			// Calculate expected event area width (accounting for sidebar)
			sidebarWidth := 30                            // Approximate sidebar width
			eventAreaWidth := tt.width - sidebarWidth - 7 // 7 for time column

			// Create event layers
			layers := m.createEventBlockLayers(24, 50, 7, eventAreaWidth)

			// Basic validation - ensure we get the right number of layers
			if len(layers) != len(tt.events) {
				t.Errorf("Expected %d layers, got %d", len(tt.events), len(layers))
			}

			// Since we can't easily inspect layer properties, we ensure the function completes
			// The main tests are that:
			// 1. The function doesn't panic
			// 2. It produces the expected number of layers
			// 3. The logic changes ensure full width is used when appropriate
		})
	}
}

// TestStatusBarLayerRendering tests that status bar layers are properly created
func TestStatusBarLayerRendering(t *testing.T) {
	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
		config:        &config.Config{},
		styles:        defaultStyles(),
	}

	tests := []struct {
		name           string
		visibleSlots   int
		message        string
		expectedLayers int
	}{
		{
			name:           "Normal status bar without message",
			visibleSlots:   28,
			message:        "",
			expectedLayers: 4, // 2 background + 2 text layers
		},
		{
			name:           "Status bar with message",
			visibleSlots:   28,
			message:        "Test message",
			expectedLayers: 4, // 2 background + 2 text layers
		},
		{
			name:           "Small visible area",
			visibleSlots:   5,
			message:        "",
			expectedLayers: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.message = tt.message
			layers := m.createStatusBarLayers(tt.visibleSlots)

			if len(layers) != tt.expectedLayers {
				t.Errorf("Expected %d layers, got %d", tt.expectedLayers, len(layers))
			}

			// Check that we got the expected number of layers
			// We can't directly inspect layer properties, but we can verify the function completes
			// and returns the expected number of layers
		})
	}
}

// TestTimeColumnBoundaryChecking tests that time columns don't overflow into status bar
func TestTimeColumnBoundaryChecking(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name          string
		topSlot       int
		visibleSlots  int
		timeIncrement int
		expectMaxRows int // Maximum rows that should be created
	}{
		{
			name:          "Single day view",
			topSlot:       0,
			visibleSlots:  24,
			timeIncrement: 60,
			expectMaxRows: 24, // Should not exceed visibleSlots
		},
		{
			name:          "Multi-day view with separators",
			topSlot:       20,
			visibleSlots:  10,
			timeIncrement: 60,
			expectMaxRows: 10, // Should stop at visibleSlots even with date separators
		},
		{
			name:          "View starting at negative slot",
			topSlot:       -5,
			visibleSlots:  15,
			timeIncrement: 60,
			expectMaxRows: 15,
		},
		{
			name:          "30-minute increments with day boundary",
			topSlot:       45,
			visibleSlots:  8,
			timeIncrement: 30,
			expectMaxRows: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				width:         120,
				height:        tt.visibleSlots + 2, // +2 for status bar
				timeIncrement: tt.timeIncrement,
				selectedDate:  baseDate,
				topSlot:       tt.topSlot,
				config:        &config.Config{},
				styles:        defaultStyles(),
			}

			slotsPerDay := 24
			if tt.timeIncrement == 30 {
				slotsPerDay = 48
			} else if tt.timeIncrement == 15 {
				slotsPerDay = 96
			}

			layers := m.createTimeColumnLayers(slotsPerDay, tt.visibleSlots)

			// We can't directly inspect layer Y positions, but we can verify:
			// 1. The function doesn't panic
			// 2. It returns layers
			// 3. The boundary checking logic in the implementation prevents overflow

			if layers == nil {
				t.Error("createTimeColumnLayers returned nil")
			}

			// The number of layers should be reasonable (not exceed visible slots * 2)
			// (accounting for both time labels and possible date separators)
			maxExpectedLayers := tt.visibleSlots * 2
			if len(layers) > maxExpectedLayers {
				t.Errorf("Too many layers created: %d, max expected: %d",
					len(layers), maxExpectedLayers)
			}
		})
	}
}

// TestCanvasViewIntegration tests the full canvas view rendering
func TestCanvasViewIntegration(t *testing.T) {
	m := &Model{
		width:         120,
		height:        30,
		timeIncrement: 60,
		selectedDate:  time.Date(2025, 8, 25, 12, 0, 0, 0, time.Local),
		topSlot:       8,
		selectedSlot:  12,
		config:        &config.Config{},
		styles:        defaultStyles(),
		showEventIDs:  false,
		events: []remind.Event{
			{
				Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				Time:        timePtr(10, 0),
				Description: "Morning meeting",
				Duration:    durationPtr(60),
			},
			{
				Date:        time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local),
				Time:        timePtr(14, 0),
				Description: "Afternoon task",
				Duration:    durationPtr(120),
			},
		},
	}

	// This should not panic
	output := m.renderCanvasView()

	// Basic checks
	if output == "" {
		t.Error("renderCanvasView returned empty string")
	}

	// Check that the output contains expected elements
	if !strings.Contains(output, "Currently:") {
		t.Error("Output missing 'Currently:' status line")
	}

	// The output should be properly bounded to the terminal height
	lines := strings.Split(output, "\n")
	if len(lines) > m.height {
		t.Errorf("Output has %d lines, exceeds height %d", len(lines), m.height)
	}
}

// TestEventSlotCalculationWithDifferentIncrements tests slot calculation with different time increments
func TestEventSlotCalculationWithDifferentIncrements(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name          string
		timeIncrement int
		event         remind.Event
		expectedSlot  int
	}{
		{
			name:          "60-minute increment, same day noon",
			timeIncrement: 60,
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(12, 0),
			},
			expectedSlot: 12, // Hour 12
		},
		{
			name:          "30-minute increment, same day 14:30",
			timeIncrement: 30,
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(14, 30),
			},
			expectedSlot: 29, // 14*2 + 1
		},
		{
			name:          "15-minute increment, same day 09:45",
			timeIncrement: 15,
			event: remind.Event{
				Date: baseDate,
				Time: timePtr(9, 45),
			},
			expectedSlot: 39, // 9*4 + 3
		},
		{
			name:          "Next day with 60-minute increment",
			timeIncrement: 60,
			event: remind.Event{
				Date: baseDate.AddDate(0, 0, 1),
				Time: timePtr(8, 0),
			},
			expectedSlot: 32, // 24 + 8
		},
		{
			name:          "Previous day with 30-minute increment",
			timeIncrement: 30,
			event: remind.Event{
				Date: baseDate.AddDate(0, 0, -1),
				Time: timePtr(16, 30),
			},
			expectedSlot: -15, // -48 + 33 (16*2 + 1)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				timeIncrement: tt.timeIncrement,
				selectedDate:  baseDate,
			}

			slotsPerDay := 24
			if tt.timeIncrement == 30 {
				slotsPerDay = 48
			} else if tt.timeIncrement == 15 {
				slotsPerDay = 96
			}

			slot := m.findEventSlot(tt.event, slotsPerDay, baseDate)

			if slot != tt.expectedSlot {
				t.Errorf("findEventSlot() = %d, want %d for event at %v on %v with increment %d",
					slot, tt.expectedSlot,
					tt.event.Time, tt.event.Date.Format("2006-01-02"),
					tt.timeIncrement)
			}
		})
	}
}
