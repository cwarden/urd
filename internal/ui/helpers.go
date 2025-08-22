package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/cwarden/urd/internal/remind"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"
)

// getEventBackgroundColor returns a background color based on event properties
func (m *Model) getEventBackgroundColor(event remind.Event) lipgloss.ANSIColor {
	// P2 tasks get different colors than remind events
	if len(event.ID) >= 3 && event.ID[:3] == "p2-" {
		// P2 task colors based on duration
		if event.Duration != nil {
			duration := event.Duration.Hours()
			if duration >= 4 {
				return lipgloss.ANSIColor(88) // Dark red for long tasks (4+ hours)
			} else if duration >= 2 {
				return lipgloss.ANSIColor(208) // Orange for medium tasks (2-4 hours)
			} else if duration >= 1 {
				return lipgloss.ANSIColor(220) // Yellow for short tasks (1-2 hours)
			} else {
				return lipgloss.ANSIColor(48) // Light green for very short tasks (<1 hour)
			}
		}
		// Default for P2 tasks without duration
		return lipgloss.ANSIColor(24) // Blue for P2 tasks
	}

	// Remind events get different colors
	if event.Duration != nil {
		duration := event.Duration.Hours()
		if duration >= 4 {
			return lipgloss.ANSIColor(52) // Dark purple for long events
		} else if duration >= 2 {
			return lipgloss.ANSIColor(63) // Medium purple for medium events
		} else if duration >= 1 {
			return lipgloss.ANSIColor(99) // Light purple for short events
		} else {
			return lipgloss.ANSIColor(105) // Very light purple for brief events
		}
	}

	// Priority-based colors for events without duration
	switch event.Priority {
	case remind.PriorityHigh:
		return lipgloss.ANSIColor(196) // Bright red
	case remind.PriorityMedium:
		return lipgloss.ANSIColor(214) // Orange-yellow
	case remind.PriorityLow:
		return lipgloss.ANSIColor(228) // Light yellow
	default:
		return lipgloss.ANSIColor(240) // Gray for normal remind events
	}
}

// renderMiniCalendar renders a small calendar for navigation
func (m *Model) renderMiniCalendar() string {
	var lines []string

	// Month/Year header
	monthYear := m.selectedDate.Format("January 2006")
	lines = append(lines, m.styles.Header.Render(monthYear))

	// Day headers
	lines = append(lines, "Mo Tu We Th Fr Sa Su")

	// Calculate first day of month
	firstDay := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), 1, 0, 0, 0, 0, time.Local)
	startOffset := int(firstDay.Weekday())
	if startOffset == 0 {
		startOffset = 7 // Sunday -> 7
	}
	startOffset-- // Monday = 0

	// Build calendar grid
	day := firstDay.AddDate(0, 0, -startOffset)
	today := time.Now()

	var weekLines []string
	weekDays := ""
	for week := 0; week < 6; week++ {
		for weekday := 0; weekday < 7; weekday++ {
			dayStr := fmt.Sprintf("%2d", day.Day())

			// Apply styling
			if day.Month() != m.selectedDate.Month() {
				dayStr = m.styles.Help.Render(dayStr) // Dimmed
			} else if day.Year() == today.Year() && day.YearDay() == today.YearDay() {
				dayStr = m.styles.Today.Render(dayStr)
			} else if day.Year() == m.selectedDate.Year() && day.YearDay() == m.selectedDate.YearDay() {
				dayStr = m.styles.Selected.Render(dayStr)
			} else if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				dayStr = m.styles.Weekend.Render(dayStr)
			} else {
				dayStr = m.styles.Normal.Render(dayStr)
			}

			weekDays += dayStr
			if weekday < 6 {
				weekDays += " "
			}

			day = day.AddDate(0, 0, 1)
		}
		weekLines = append(weekLines, weekDays)
		weekDays = ""

		// Stop if we've shown all days of the month
		if day.Month() != m.selectedDate.Month() && week > 3 {
			break
		}
	}

	lines = append(lines, weekLines...)

	// Add border
	bordered := m.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return bordered
}

// renderSelectedSlotEvents renders all events for the selected time slot
func (m *Model) renderSelectedSlotEvents() string {
	// Find event at selected slot
	slotsPerDay := 24
	if m.timeIncrement == 30 {
		slotsPerDay = 48
	} else if m.timeIncrement == 15 {
		slotsPerDay = 96
	}

	dayOffset := m.selectedSlot / slotsPerDay
	localSlot := m.selectedSlot % slotsPerDay

	// Handle negative slots
	if m.selectedSlot < 0 {
		dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
		localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
		if localSlot == slotsPerDay {
			localSlot = 0
			dayOffset++
		}
	}

	selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

	hour := localSlot
	minute := 0
	if m.timeIncrement == 30 {
		hour = localSlot / 2
		minute = (localSlot % 2) * 30
	} else if m.timeIncrement == 15 {
		hour = localSlot / 4
		minute = (localSlot % 4) * 15
	}

	// Find events active during this time slot
	var selectedEvents []remind.Event
	for _, event := range m.events {
		if event.Time != nil &&
			event.Date.Year() == selectedDate.Year() &&
			event.Date.YearDay() == selectedDate.YearDay() {

			// Calculate event start and end times
			eventStart := *event.Time

			// Calculate slot start and end times
			slotStart := time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day(),
				hour, minute, 0, 0, selectedDate.Location())
			slotEnd := slotStart
			if m.timeIncrement == 60 {
				slotEnd = slotStart.Add(time.Hour)
			} else if m.timeIncrement == 30 {
				slotEnd = slotStart.Add(30 * time.Minute)
			} else if m.timeIncrement == 15 {
				slotEnd = slotStart.Add(15 * time.Minute)
			}

			// Check if event overlaps with the selected time slot
			if event.Duration != nil {
				// For events with duration, check overlap
				eventEnd := eventStart.Add(*event.Duration)
				// Event is active if it starts before slot ends AND ends after slot starts
				if eventStart.Before(slotEnd) && eventEnd.After(slotStart) {
					selectedEvents = append(selectedEvents, event)
				}
			} else {
				// For events without duration, only show if they start within this slot
				if !eventStart.Before(slotStart) && eventStart.Before(slotEnd) {
					selectedEvents = append(selectedEvents, event)
				}
			}
		}
	}

	var lines []string

	// Calculate available width for the box
	// The schedule takes 2/3 of width, so we have 1/3 for the right side
	scheduleWidth := m.width * 2 / 3
	if scheduleWidth < 40 {
		scheduleWidth = 40
	}
	// Right side width minus padding and borders
	boxWidth := m.width - scheduleWidth - 4
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Header with selected time
	timeHeader := fmt.Sprintf("%s at %02d:%02d",
		selectedDate.Format("Mon Jan 2, 2006"),
		hour, minute)
	// Wrap the header to fit within the box width
	wrappedHeader := wordwrap.String(timeHeader, boxWidth-2)
	lines = append(lines, m.styles.Header.Render(wrappedHeader))

	// Show all events for this slot
	if len(selectedEvents) == 0 {
		lines = append(lines, "")
		lines = append(lines, m.styles.Help.Render("(no reminders at this time)"))
	} else {
		lines = append(lines, "")
		for i, event := range selectedEvents {
			if i > 0 {
				lines = append(lines, "") // Separator between events
			}

			// Event time and duration
			eventTime := fmt.Sprintf("%02d:%02d", event.Time.Hour(), event.Time.Minute())
			if event.Duration != nil {
				// Format duration without seconds
				hours := int(event.Duration.Hours())
				minutes := int(event.Duration.Minutes()) % 60
				if hours > 0 && minutes > 0 {
					eventTime += fmt.Sprintf(" (%dh %dm)", hours, minutes)
				} else if hours > 0 {
					eventTime += fmt.Sprintf(" (%dh)", hours)
				} else {
					eventTime += fmt.Sprintf(" (%dm)", minutes)
				}
			}
			lines = append(lines, m.styles.Event.Render(eventTime))

			// Event description
			desc := event.Description
			if m.showEventIDs {
				// Show ID for debugging
				lines = append(lines, m.styles.Help.Render(fmt.Sprintf("ID: %s", event.ID)))
			}
			// Wrap long descriptions using wordwrap to avoid breaking words/URLs
			maxWidth := boxWidth - 4 // Account for padding
			if maxWidth < 20 {
				maxWidth = 20 // Minimum width to avoid too narrow wrapping
			}
			wrapped := wordwrap.String(desc, maxWidth)
			for _, line := range strings.Split(wrapped, "\n") {
				if line != "" {
					lines = append(lines, line)
				}
			}

			// Tags if any
			if len(event.Tags) > 0 {
				tagStr := "Tags: " + strings.Join(event.Tags, ", ")
				lines = append(lines, m.styles.Help.Render(tagStr))
			}

			// Priority indicator
			if event.Priority > remind.PriorityNone {
				priorityStr := "Priority: "
				switch event.Priority {
				case remind.PriorityLow:
					priorityStr += "!"
				case remind.PriorityMedium:
					priorityStr += "!!"
				case remind.PriorityHigh:
					priorityStr += "!!!"
				}
				lines = append(lines, m.styles.Priority.Render(priorityStr))
			}
		}
	}

	// Add border with calculated width
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxStyle := m.styles.Border.Copy().Width(boxWidth)
	return boxStyle.Render(content)
}
