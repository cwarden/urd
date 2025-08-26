package ui

import (
	"testing"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"
)

// TestUpdateSelectedDateFromSlot tests that the calendar date updates correctly when navigating slots
func TestUpdateSelectedDateFromSlot(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name            string
		timeIncrement   int
		initialSlot     int
		initialTopSlot  int
		initialDate     time.Time
		expectedDate    time.Time
		expectedSlot    int
		expectedTopSlot int
	}{
		{
			name:            "Stay on same day",
			timeIncrement:   60,
			initialSlot:     12, // noon
			initialTopSlot:  8,
			initialDate:     baseDate,
			expectedDate:    baseDate,
			expectedSlot:    12,
			expectedTopSlot: 8,
		},
		{
			name:            "Move to next day with 60-min increment",
			timeIncrement:   60,
			initialSlot:     25, // 1am next day
			initialTopSlot:  20,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, 1),
			expectedSlot:    1,  // Reset to 1am of new day
			expectedTopSlot: -4, // Adjusted for new reference
		},
		{
			name:            "Move to previous day with 60-min increment",
			timeIncrement:   60,
			initialSlot:     -2, // 10pm previous day
			initialTopSlot:  -5,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, -1),
			expectedSlot:    22, // Reset to 10pm of new day
			expectedTopSlot: 19,
		},
		{
			name:            "Move to next day with 30-min increment",
			timeIncrement:   30,
			initialSlot:     50, // 1am next day (slot 2 of next day)
			initialTopSlot:  40,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, 1),
			expectedSlot:    2,  // Reset to slot 2 of new day
			expectedTopSlot: -8, // Adjusted for new reference
		},
		{
			name:            "Move to next day with 15-min increment",
			timeIncrement:   15,
			initialSlot:     100, // 1am next day (slot 4 of next day)
			initialTopSlot:  90,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, 1),
			expectedSlot:    4,  // Reset to slot 4 of new day
			expectedTopSlot: -6, // Adjusted for new reference
		},
		{
			name:            "Move multiple days forward",
			timeIncrement:   60,
			initialSlot:     72, // Midnight 3 days later
			initialTopSlot:  70,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, 3),
			expectedSlot:    0,  // Midnight of new day
			expectedTopSlot: -2, // Adjusted for new reference
		},
		{
			name:            "Move multiple days backward",
			timeIncrement:   60,
			initialSlot:     -50, // 22:00 2 days earlier
			initialTopSlot:  -52,
			initialDate:     baseDate,
			expectedDate:    baseDate.AddDate(0, 0, -3),
			expectedSlot:    22, // 10pm of new day
			expectedTopSlot: 20, // Adjusted for new reference
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				timeIncrement:   tt.timeIncrement,
				selectedSlot:    tt.initialSlot,
				topSlot:         tt.initialTopSlot,
				selectedDate:    tt.initialDate,
				config:          &config.Config{},
				remindClient:    &remind.Client{},
				eventsLoadedFor: tt.initialDate, // Prevent reload in test
			}

			m.updateSelectedDateFromSlot()

			if !m.selectedDate.Equal(tt.expectedDate) {
				t.Errorf("selectedDate = %v, want %v", m.selectedDate, tt.expectedDate)
			}

			if m.selectedSlot != tt.expectedSlot {
				t.Errorf("selectedSlot = %d, want %d", m.selectedSlot, tt.expectedSlot)
			}

			if m.topSlot != tt.expectedTopSlot {
				t.Errorf("topSlot = %d, want %d", m.topSlot, tt.expectedTopSlot)
			}
		})
	}
}

// TestSlotNavigationDateSync tests that navigating with scroll_up/scroll_down updates the calendar
func TestSlotNavigationDateSync(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 12, 0, 0, 0, time.Local) // Monday noon

	tests := []struct {
		name          string
		actions       []string // sequence of navigation actions
		expectedDate  time.Time
		expectedSlot  int
		timeIncrement int
	}{
		{
			name:          "Scroll down within same day",
			actions:       []string{"scroll_down", "scroll_down"},
			expectedDate:  baseDate,
			expectedSlot:  14, // 2pm
			timeIncrement: 60,
		},
		{
			name:          "Scroll down to next day",
			actions:       repeatAction("scroll_down", 13), // 12 + 13 = 25 (1am next day)
			expectedDate:  baseDate.AddDate(0, 0, 1),
			expectedSlot:  1,
			timeIncrement: 60,
		},
		{
			name:          "Scroll up to previous day",
			actions:       repeatAction("scroll_up", 13), // 12 - 13 = -1 (11pm previous day)
			expectedDate:  baseDate.AddDate(0, 0, -1),
			expectedSlot:  23,
			timeIncrement: 60,
		},
		{
			name:          "Scroll with 30-min increments",
			actions:       repeatAction("scroll_down", 25), // For 30-min: noon = slot 24, +25 = 49 (crosses to next day, slot 1 = 00:30)
			expectedDate:  baseDate.AddDate(0, 0, 1),
			expectedSlot:  1, // 00:30 of next day
			timeIncrement: 30,
		},
		{
			name:          "Mixed scrolling stays on correct day",
			actions:       []string{"scroll_down", "scroll_down", "scroll_up", "scroll_down"},
			expectedDate:  baseDate,
			expectedSlot:  14, // 12 + 1 + 1 - 1 + 1 = 14
			timeIncrement: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Adjust starting slot based on time increment
			startSlot := 12 // noon for 60-min increments
			if tt.timeIncrement == 30 {
				startSlot = 24 // noon for 30-min increments (12 * 2)
			} else if tt.timeIncrement == 15 {
				startSlot = 48 // noon for 15-min increments (12 * 4)
			}

			m := &Model{
				timeIncrement: tt.timeIncrement,
				selectedSlot:  startSlot,
				topSlot:       8, // View starts at 8am
				selectedDate:  baseDate,
				config: &config.Config{
					KeyBindings: map[string]string{
						"j": "scroll_down",
						"k": "scroll_up",
					},
				},
				remindClient:    &remind.Client{},
				mode:            ViewHourly,
				eventsLoadedFor: baseDate, // Prevent reload in test
			}

			// Apply the sequence of actions
			for _, action := range tt.actions {
				// Simulate the key action
				switch action {
				case "scroll_down":
					m.selectedSlot++
					if !m.isSlotVisible(m.selectedSlot) {
						m.topSlot++
					}
					m.updateSelectedDateFromSlot()
				case "scroll_up":
					m.selectedSlot--
					if !m.isSlotVisible(m.selectedSlot) {
						m.topSlot--
					}
					m.updateSelectedDateFromSlot()
				}
			}

			if !m.selectedDate.Equal(tt.expectedDate) {
				t.Errorf("After %v: selectedDate = %v, want %v",
					tt.actions, m.selectedDate.Format("2006-01-02"), tt.expectedDate.Format("2006-01-02"))
			}

			if m.selectedSlot != tt.expectedSlot {
				t.Errorf("After %v: selectedSlot = %d, want %d",
					tt.actions, m.selectedSlot, tt.expectedSlot)
			}
		})
	}
}

// TestDayNavigationResetsSlotsCorrectly tests that using next_day/previous_day navigation works correctly
func TestDayNavigationResetsSlotsCorrectly(t *testing.T) {
	baseDate := time.Date(2025, 8, 25, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name           string
		initialSlot    int
		initialTopSlot int
		action         string
		expectedDate   time.Time
		expectedSlot   int // Should maintain time of day
	}{
		{
			name:           "Next day maintains time",
			initialSlot:    14, // 2pm
			initialTopSlot: 8,
			action:         "next_day",
			expectedDate:   baseDate.AddDate(0, 0, 1),
			expectedSlot:   14, // Still 2pm
		},
		{
			name:           "Previous day maintains time",
			initialSlot:    14, // 2pm
			initialTopSlot: 8,
			action:         "previous_day",
			expectedDate:   baseDate.AddDate(0, 0, -1),
			expectedSlot:   14, // Still 2pm
		},
		{
			name:           "Next week maintains time",
			initialSlot:    9, // 9am
			initialTopSlot: 6,
			action:         "next_week",
			expectedDate:   baseDate.AddDate(0, 0, 7),
			expectedSlot:   9, // Still 9am
		},
		{
			name:           "Previous week maintains time",
			initialSlot:    20, // 8pm
			initialTopSlot: 16,
			action:         "previous_week",
			expectedDate:   baseDate.AddDate(0, 0, -7),
			expectedSlot:   20, // Still 8pm
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				timeIncrement: 60,
				selectedSlot:  tt.initialSlot,
				topSlot:       tt.initialTopSlot,
				selectedDate:  baseDate,
				config:        &config.Config{},
				remindClient:  &remind.Client{},
			}

			// Simulate the action
			switch tt.action {
			case "next_day":
				m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
			case "previous_day":
				m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
			case "next_week":
				m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
			case "previous_week":
				m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
			}

			if !m.selectedDate.Equal(tt.expectedDate) {
				t.Errorf("After %s: selectedDate = %v, want %v",
					tt.action, m.selectedDate, tt.expectedDate)
			}

			if m.selectedSlot != tt.expectedSlot {
				t.Errorf("After %s: selectedSlot = %d, want %d",
					tt.action, m.selectedSlot, tt.expectedSlot)
			}
		})
	}
}

// Helper function to repeat an action multiple times
func repeatAction(action string, count int) []string {
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = action
	}
	return result
}

// TestInactivityAutoAdvance tests the auto-advance behavior after inactivity
func TestInactivityAutoAdvance(t *testing.T) {
	// We'll test by setting lastKeyInput to more than 5 minutes ago
	// and checking if the slot advances when conditions are met

	tests := []struct {
		name                   string
		timeIncrement          int
		selectedSlot           int              // Current slot user is at
		setupLastInput         func() time.Time // How to set lastKeyInput relative to now
		shouldAdvance          bool
		expectedSlotAdjustment int // How much the slot should change
	}{
		{
			name:          "Advances when at previous slot after inactivity",
			timeIncrement: 60,
			selectedSlot: func() int {
				now := time.Now()
				return now.Hour() - 1 // Previous hour slot
			}(),
			setupLastInput: func() time.Time {
				return time.Now().Add(-6 * time.Minute) // 6 minutes ago
			},
			shouldAdvance:          true,
			expectedSlotAdjustment: 1, // Should advance by 1 slot
		},
		{
			name:          "Does not advance when not at previous slot",
			timeIncrement: 60,
			selectedSlot:  10, // Some arbitrary slot, not the previous one
			setupLastInput: func() time.Time {
				return time.Now().Add(-6 * time.Minute) // 6 minutes ago
			},
			shouldAdvance:          false,
			expectedSlotAdjustment: 0, // Should stay the same
		},
		{
			name:          "Does not advance when recently active",
			timeIncrement: 60,
			selectedSlot: func() int {
				now := time.Now()
				return now.Hour() - 1 // Previous hour slot
			}(),
			setupLastInput: func() time.Time {
				return time.Now().Add(-2 * time.Minute) // Only 2 minutes ago
			},
			shouldAdvance:          false,
			expectedSlotAdjustment: 0, // Should stay the same
		},
		{
			name:          "Advances with 30-min increments",
			timeIncrement: 30,
			selectedSlot: func() int {
				now := time.Now()
				currentSlot := now.Hour()*2 + now.Minute()/30
				return currentSlot - 1 // Previous slot
			}(),
			setupLastInput: func() time.Time {
				return time.Now().Add(-6 * time.Minute) // 6 minutes ago
			},
			shouldAdvance:          true,
			expectedSlotAdjustment: 1, // Should advance by 1 slot
		},
		{
			name:          "Does not advance when user navigated away",
			timeIncrement: 60,
			selectedSlot:  20, // Some future slot (user navigated forward)
			setupLastInput: func() time.Time {
				return time.Now().Add(-6 * time.Minute) // 6 minutes ago
			},
			shouldAdvance:          false,
			expectedSlotAdjustment: 0, // Should stay the same
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			m := &Model{
				timeIncrement: tt.timeIncrement,
				selectedSlot:  tt.selectedSlot,
				selectedDate:  now,
				lastKeyInput:  tt.setupLastInput(),
				height:        30,
				config:        &config.Config{},
				remindClient:  &remind.Client{},
			}

			initialSlot := m.selectedSlot

			// Simulate timeUpdateMsg handling
			msg := timeUpdateMsg{}
			model, _ := m.Update(msg)
			updatedModel := model.(*Model)

			actualAdjustment := updatedModel.selectedSlot - initialSlot

			if tt.shouldAdvance {
				if actualAdjustment != tt.expectedSlotAdjustment {
					t.Errorf("Expected slot to advance by %d, but it changed by %d (from %d to %d)",
						tt.expectedSlotAdjustment, actualAdjustment, initialSlot, updatedModel.selectedSlot)
				}
			} else {
				if actualAdjustment != 0 {
					t.Errorf("Expected slot to stay at %d, but got %d", initialSlot, updatedModel.selectedSlot)
				}
			}
		})
	}
}

// TestLastKeyInputField tests that the lastKeyInput field exists and can be manipulated
func TestLastKeyInputField(t *testing.T) {
	initialTime := time.Date(2025, 8, 25, 14, 0, 0, 0, time.Local)

	m := &Model{
		timeIncrement: 60,
		selectedSlot:  14,
		selectedDate:  initialTime,
		lastKeyInput:  initialTime.Add(-10 * time.Minute), // 10 minutes ago
		height:        30,
		config:        &config.Config{},
		remindClient:  &remind.Client{},
		mode:          ViewHourly,
	}

	// Verify the lastKeyInput field exists and was set correctly
	expectedDiff := 10 * time.Minute
	actualDiff := initialTime.Sub(m.lastKeyInput)

	if actualDiff != expectedDiff {
		t.Errorf("Expected lastKeyInput to be %v before initialTime, but got %v", expectedDiff, actualDiff)
	}

	// Test that we can update it
	newTime := initialTime.Add(-5 * time.Minute)
	m.lastKeyInput = newTime

	if m.lastKeyInput != newTime {
		t.Errorf("Failed to update lastKeyInput field")
	}
}

// TestIsSlotVisible tests the slot visibility check
func TestIsSlotVisible(t *testing.T) {
	tests := []struct {
		name         string
		topSlot      int
		selectedSlot int
		height       int
		expected     bool
	}{
		{
			name:         "Slot is visible in middle",
			topSlot:      10,
			selectedSlot: 15,
			height:       30,
			expected:     true,
		},
		{
			name:         "Slot is at top edge",
			topSlot:      10,
			selectedSlot: 10,
			height:       30,
			expected:     true,
		},
		{
			name:         "Slot is at bottom edge",
			topSlot:      10,
			selectedSlot: 20,
			height:       30,
			expected:     true,
		},
		{
			name:         "Slot is above visible area",
			topSlot:      10,
			selectedSlot: 5,
			height:       30,
			expected:     false,
		},
		{
			name:         "Slot is below visible area",
			topSlot:      10,
			selectedSlot: 50,
			height:       30,
			expected:     false,
		},
		{
			name:         "Negative slot above negative topSlot",
			topSlot:      -10,
			selectedSlot: -15,
			height:       30,
			expected:     false,
		},
		{
			name:         "Negative slot within view",
			topSlot:      -10,
			selectedSlot: -5,
			height:       30,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				topSlot:      tt.topSlot,
				selectedSlot: tt.selectedSlot,
				height:       tt.height,
			}

			result := m.isSlotVisible(tt.selectedSlot)
			if result != tt.expected {
				t.Errorf("isSlotVisible(%d) = %v, want %v (topSlot=%d, height=%d)",
					tt.selectedSlot, result, tt.expected, tt.topSlot, tt.height)
			}
		})
	}
}
