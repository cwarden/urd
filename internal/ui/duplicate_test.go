package ui

import (
	"github.com/cwarden/urd/internal/remind"
	"strings"
	"testing"
	"time"
)

func TestMultipleEventsWithSameDescription(t *testing.T) {
	// Test that multiple distinct events with the same description are all shown
	// This can happen when different calendar entries have the same text
	m := &Model{
		selectedDate:  time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
		timeIncrement: 30,
		width:         160,
		height:        40,
		styles:        DefaultStyles(),
		showEventIDs:  false,
	}

	// Create events that appear duplicated in the display
	time0630 := time.Date(2025, 8, 19, 6, 30, 0, 0, time.Local)
	time0645 := time.Date(2025, 8, 19, 6, 45, 0, 0, time.Local)
	time0700 := time.Date(2025, 8, 19, 7, 0, 0, 0, time.Local)
	time1100 := time.Date(2025, 8, 19, 11, 0, 0, 0, time.Local)
	duration30 := 30 * time.Minute

	m.events = []remind.Event{
		{
			ID:          "evt-catapult",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time0630,
			Description: "Schedule Catapult deploy if necessary",
			LineNumber:  1,
		},
		{
			ID:          "evt-backup",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time0630,
			Description: "Backup/restore to chicory.xerus.org (Hetzner)",
			LineNumber:  2,
		},
		{
			ID:          "evt-car1",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time0645,
			Description: "Move Car for Street Sweeping",
			LineNumber:  3,
		},
		{
			ID:          "evt-car2", // Simulate duplicate with different ID
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time0645,
			Description: "Move Car for Street Sweeping",
			LineNumber:  4,
		},
		{
			ID:          "evt-tax",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time0700,
			Description: "Pay 71 E Division Property Tax",
			LineNumber:  5,
		},
		{
			ID:          "evt-meeting1",
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time1100,
			Duration:    &duration30,
			Description: "Salesforce Team Meeting at Microsoft Teams Meeting",
			LineNumber:  6,
		},
		{
			ID:          "evt-meeting2", // Another duplicate
			Date:        time.Date(2025, 8, 19, 0, 0, 0, 0, time.Local),
			Time:        &time1100,
			Duration:    &duration30,
			Description: "Salesforce Team Meeting at Microsoft Teams Meeting",
			LineNumber:  7,
		},
	}

	slotsPerDay := 48
	eventsBySlot, eventColumns := m.buildSimpleEventLayout(slotsPerDay)

	// Check 06:30 slot (slot 13)
	slot0630 := 13
	events0630 := eventsBySlot[slot0630]
	t.Logf("Events at 06:30 (slot %d):", slot0630)
	for _, e := range events0630 {
		t.Logf("  - %s (ID: %s, Column: %d)", e.Description, e.ID, eventColumns[e.ID])
	}

	// Check 06:45 slot (would be between slots in 30-min increments)
	// 06:45 would round to slot 13 or 14 depending on implementation
	slot0645 := 13 // 06:30-07:00 slot
	events0645 := eventsBySlot[slot0645]

	// Count "Move Car" events
	moveCarCount := 0
	for _, e := range events0645 {
		if strings.Contains(e.Description, "Move Car") {
			moveCarCount++
		}
	}

	if moveCarCount != 2 {
		t.Errorf("Found %d 'Move Car' events in slot %d, expected exactly 2 for this test scenario", moveCarCount, slot0645)
	}

	// Check 11:00 slot (slot 22)
	slot1100 := 22
	events1100 := eventsBySlot[slot1100]

	salesforceCount := 0
	for _, e := range events1100 {
		if strings.Contains(e.Description, "Salesforce") {
			salesforceCount++
		}
	}

	if salesforceCount != 2 {
		t.Errorf("Found %d 'Salesforce' events in slot %d, expected exactly 2 for this test scenario", salesforceCount, slot1100)
	}

	// Now test the actual rendering
	output := m.renderSchedule()
	lines := strings.Split(output, "\n")

	// Find the 06:30 line and verify both Move Car events are shown (they have different IDs)
	for _, line := range lines {
		if strings.Contains(line, "06:30") || strings.Contains(line, "06:45") {
			// Check for "Move Car" text (may be truncated when 4 events in one slot)
			moveCarInLine := strings.Count(line, "Move Car")
			// We expect 2 because we have two different events with same description
			if moveCarInLine != 2 {
				t.Errorf("Line should contain 'Move Car' 2 times (different IDs), got %d times: %s", moveCarInLine, line)
			}
		}

		if strings.Contains(line, "11:00") {
			// Check for "Salesforce" text (may be truncated when multiple events in one slot)
			salesforceInLine := strings.Count(line, "Salesforce")
			// We expect 2 because we have two different events with same description
			if salesforceInLine != 2 {
				t.Errorf("Line should contain 'Salesforce' 2 times (different IDs), got %d times: %s", salesforceInLine, line)
			}
		}
	}
}

func TestSlotCalculation(t *testing.T) {
	// Test that 06:45 events go to the right slot
	testCases := []struct {
		time     time.Time
		expected int
		desc     string
	}{
		{
			time:     time.Date(2025, 8, 19, 6, 30, 0, 0, time.Local),
			expected: 13,
			desc:     "06:30 should be slot 13",
		},
		{
			time:     time.Date(2025, 8, 19, 6, 45, 0, 0, time.Local),
			expected: 13, // 06:45 rounds down to 06:30 slot
			desc:     "06:45 should be slot 13",
		},
		{
			time:     time.Date(2025, 8, 19, 7, 0, 0, 0, time.Local),
			expected: 14,
			desc:     "07:00 should be slot 14",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hour := tc.time.Hour()
			minute := tc.time.Minute()
			localSlot := hour*2 + minute/30

			if localSlot != tc.expected {
				t.Errorf("Time %s: expected slot %d, got %d",
					tc.time.Format("15:04"), tc.expected, localSlot)
			}
		})
	}
}
