package ui

import (
	"fmt"
	"strings"
	"time"

	"urd/internal/remind"

	"github.com/charmbracelet/lipgloss"
)

// viewHourlySchedule renders the hourly schedule view
func (m *Model) viewHourlySchedule() string {
	var sections []string

	// Top section: Schedule on left, Calendar + Untimed on right
	scheduleView := m.renderSchedule()
	calendarView := m.renderMiniCalendar()
	untimedView := m.renderUntimedList()

	// Right side: calendar above, untimed below
	rightSide := lipgloss.JoinVertical(
		lipgloss.Left,
		calendarView,
		"",
		untimedView,
	)

	// Main content: schedule left, calendar/untimed right
	scheduleWidth := m.width * 2 / 3
	if scheduleWidth < 40 {
		scheduleWidth = 40
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(scheduleWidth).Render(scheduleView),
		" ",
		rightSide,
	)
	sections = append(sections, mainContent)

	// Description section at the bottom
	description := m.renderEventDescription()
	sections = append(sections, description)

	// Status bar
	status := m.renderScheduleStatusBar()
	sections = append(sections, status)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderScheduleSimple renders the time slot schedule with simple indentation for overlaps
func (m *Model) renderSchedule() string {
	var lines []string

	// Calculate slots per day based on time increment
	slotsPerDay := 24
	if m.timeIncrement == 30 {
		slotsPerDay = 48
	} else if m.timeIncrement == 15 {
		slotsPerDay = 96
	}

	// Calculate visible slots
	visibleSlots := m.height - 6 // Leave room for description and status
	if visibleSlots < 10 {
		visibleSlots = 10
	}

	// Events should already be loaded - don't reload on every render

	// Build event map and track overlaps
	eventsBySlot, eventColumns := m.buildSimpleEventLayout(slotsPerDay)

	// Render time slots
	now := time.Now()
	prevDay := -999

	for i := 0; i < visibleSlots; i++ {
		globalSlot := m.topSlot + i
		dayOffset := globalSlot / slotsPerDay
		localSlot := globalSlot % slotsPerDay

		// Handle negative slots (previous days)
		if globalSlot < 0 {
			dayOffset = -1 + (globalSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (globalSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		currentDate := m.selectedDate.AddDate(0, 0, dayOffset)

		// Add date separator when day changes
		if dayOffset != prevDay {
			dateLine := currentDate.Format("─Mon Jan 02")
			lines = append(lines, m.styles.Header.Render(dateLine))
			prevDay = dayOffset
			if i > 0 { // Don't double count first line
				i++
				if i >= visibleSlots {
					break
				}
			}
		}

		hour := localSlot
		minute := 0
		if m.timeIncrement == 30 {
			hour = localSlot / 2
			minute = (localSlot % 2) * 30
		} else if m.timeIncrement == 15 {
			hour = localSlot / 4
			minute = (localSlot % 4) * 15
		}

		timeStr := fmt.Sprintf("%02d:%02d", hour, minute)

		// Build the line with events
		line := timeStr
		if events, ok := eventsBySlot[globalSlot]; ok && len(events) > 0 {
			// Add events with indentation based on column assignment
			eventParts := []string{}
			for _, event := range events {
				column := eventColumns[event.ID]
				indent := strings.Repeat("  ", column) // 2 spaces per indentation level

				eventStr := event.Description
				// Truncate to fit
				maxLen := (m.width * 2 / 3) - 7 - (column * 2)
				if maxLen < 20 {
					maxLen = 20
				}
				if len(eventStr) > maxLen {
					eventStr = eventStr[:maxLen-3] + "..."
				}

				eventParts = append(eventParts, indent+eventStr)
			}

			// Join all events for this slot
			if len(eventParts) > 0 {
				line = fmt.Sprintf("%s  %s", timeStr, strings.Join(eventParts, " "))
			}
		}

		// Apply styling
		style := m.styles.Normal

		// Highlight current time
		isCurrentTime := currentDate.Year() == now.Year() &&
			currentDate.YearDay() == now.YearDay() &&
			hour == now.Hour()
		if isCurrentTime {
			if m.timeIncrement == 60 ||
				(m.timeIncrement == 30 && minute <= now.Minute() && now.Minute() < minute+30) ||
				(m.timeIncrement == 15 && minute <= now.Minute() && now.Minute() < minute+15) {
				// Use a blue background for current time
				style = m.styles.Today.Background(lipgloss.Color("4"))
			}
		}

		// Highlight selected slot
		if globalSlot == m.selectedSlot {
			style = m.styles.Selected
		}

		// Check for priority events
		if events, ok := eventsBySlot[globalSlot]; ok {
			for _, event := range events {
				if event.Priority > remind.PriorityNone {
					if globalSlot != m.selectedSlot { // Don't override selection
						style = m.styles.Priority
					}
					break
				}
			}
		}

		lines = append(lines, style.Render(line))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// buildSimpleEventLayout creates a map of events and assigns them to columns
func (m *Model) buildSimpleEventLayout(slotsPerDay int) (map[int][]remind.Event, map[string]int) {
	eventsBySlot := make(map[int][]remind.Event)
	eventColumns := make(map[string]int)

	// Track which columns are busy at each time slot
	columnBusy := make(map[int][]string) // slot -> list of event IDs occupying columns

	for _, event := range m.events {
		if event.Time != nil {
			// Calculate day offset from base date
			dayDiff := int(event.Date.Sub(m.selectedDate).Hours() / 24)

			hour := event.Time.Hour()
			minute := event.Time.Minute()
			localSlot := hour

			if m.timeIncrement == 30 {
				localSlot = hour*2 + minute/30
			} else if m.timeIncrement == 15 {
				localSlot = hour*4 + minute/15
			}

			globalSlot := dayDiff*slotsPerDay + localSlot

			// Calculate duration in slots
			eventSlots := 1
			if event.Duration != nil {
				durationMinutes := int(event.Duration.Minutes())
				if m.timeIncrement == 30 {
					eventSlots = (durationMinutes + 29) / 30
				} else if m.timeIncrement == 15 {
					eventSlots = (durationMinutes + 14) / 15
				} else {
					eventSlots = (durationMinutes + 59) / 60
				}
			}

			// Find available column for this event
			column := 0
			for {
				isAvailable := true
				// Check if column is free for entire duration
				for s := globalSlot; s < globalSlot+eventSlots; s++ {
					if busy, ok := columnBusy[s]; ok {
						// Check if this column is occupied
						if column < len(busy) && busy[column] != "" {
							isAvailable = false
							break
						}
					}
				}

				if isAvailable {
					break
				}
				column++
				if column > 3 { // Limit to 4 columns
					column = 0
					break
				}
			}

			// Assign column to event
			eventColumns[event.ID] = column

			// Mark column as busy for duration
			for s := globalSlot; s < globalSlot+eventSlots; s++ {
				if columnBusy[s] == nil {
					columnBusy[s] = make([]string, column+1)
				}
				for len(columnBusy[s]) <= column {
					columnBusy[s] = append(columnBusy[s], "")
				}
				columnBusy[s][column] = event.ID
			}

			// Add event to its starting slot only
			eventsBySlot[globalSlot] = append(eventsBySlot[globalSlot], event)
		}
	}

	return eventsBySlot, eventColumns
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

// renderUntimedList renders the untimed reminders for the day
func (m *Model) renderUntimedList() string {
	var lines []string

	lines = append(lines, m.styles.Header.Render("Untimed Reminders"))

	// Find untimed events for the selected day
	var untimedEvents []remind.Event
	for _, event := range m.events {
		if event.Time == nil &&
			event.Date.Year() == m.selectedDate.Year() &&
			event.Date.YearDay() == m.selectedDate.YearDay() {
			untimedEvents = append(untimedEvents, event)
		}
	}

	if len(untimedEvents) == 0 {
		lines = append(lines, m.styles.Help.Render("(no untimed reminders)"))
	} else {
		for i, event := range untimedEvents {
			if i >= 5 { // Limit display
				lines = append(lines, m.styles.Help.Render(fmt.Sprintf("... and %d more", len(untimedEvents)-5)))
				break
			}
			style := m.styles.Event
			if event.Priority > remind.PriorityNone {
				style = m.styles.Priority
			}
			line := "• " + event.Description
			if len(line) > 30 {
				line = line[:27] + "..."
			}
			lines = append(lines, style.Render(line))
		}
	}

	// Add border
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.styles.Border.Render(content)
}

// renderEventDescription renders the selected event details
func (m *Model) renderEventDescription() string {
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

	// Find events at this time
	var selectedEvents []remind.Event
	for _, event := range m.events {
		if event.Time != nil &&
			event.Date.Year() == selectedDate.Year() &&
			event.Date.YearDay() == selectedDate.YearDay() &&
			event.Time.Hour() == hour {
			// Check minute match
			eventMin := event.Time.Minute()
			if m.timeIncrement == 60 ||
				(m.timeIncrement == 30 && eventMin >= minute && eventMin < minute+30) ||
				(m.timeIncrement == 15 && eventMin >= minute && eventMin < minute+15) {
				selectedEvents = append(selectedEvents, event)
			}
		}
	}

	// Build description
	if len(selectedEvents) == 0 {
		return m.styles.Help.Render("(no reminder selected)")
	}

	event := selectedEvents[0]
	desc := fmt.Sprintf("%s at %02d:%02d",
		selectedDate.Format("Monday, January 2, 2006"),
		hour, minute)

	if event.Duration != nil {
		desc += fmt.Sprintf(" (Duration: %v)", *event.Duration)
	}

	desc += "\n" + event.Description

	if len(event.Tags) > 0 {
		desc += "\nTags: " + strings.Join(event.Tags, ", ")
	}

	// Add border
	return m.styles.Border.Render(desc)
}

// renderScheduleStatusBar renders the status bar for schedule view
func (m *Model) renderScheduleStatusBar() string {
	dateStr := m.selectedDate.Format("Monday, January 2 at 15:04")

	left := fmt.Sprintf(" Currently: %s", dateStr)

	right := "j/k:slot  H/L:day  J/K:week  z:zoom  o:today  n:new  ?:help  q:quit"

	if m.message != "" {
		right = m.styles.Message.Render(m.message)
	}

	width := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if width < 0 {
		width = 0
	}

	middle := strings.Repeat(" ", width)

	return m.styles.Help.Render(left + middle + right)
}
