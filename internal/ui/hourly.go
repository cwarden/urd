package ui

import (
	"fmt"
	"strings"
	"time"

	"urd/internal/remind"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// viewHourlySchedule renders the hourly schedule view
func (m *Model) viewHourlySchedule() string {
	var sections []string

	// Top section: Schedule on left, Calendar + Selected Events on right
	scheduleView := m.renderSchedule()
	calendarView := m.renderMiniCalendar()
	selectedEventsView := m.renderSelectedSlotEvents()

	// Right side: calendar above, selected events below
	rightSide := lipgloss.JoinVertical(
		lipgloss.Left,
		calendarView,
		"",
		selectedEventsView,
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

	slotIndex := 0 // Track actual slot index separately from line count
	for i := 0; i < visibleSlots && slotIndex < visibleSlots; i++ {
		globalSlot := m.topSlot + slotIndex
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
			// Don't increment slotIndex for the date separator
			continue
		}

		slotIndex++ // Only increment for actual time slots

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
			// Build the line with events in proper columns
			// Start with the time
			line = timeStr + "  "

			// Create a map of column to event for this slot
			columnEvents := make(map[int]string)
			maxColumn := 0

			// First pass: figure out how many columns we have
			for _, event := range events {
				column := eventColumns[event.ID]
				if column > maxColumn {
					maxColumn = column
				}

				eventStr := event.Description
				if m.showEventIDs {
					// Show event ID for debugging
					eventStr = fmt.Sprintf("[%s] %s", event.ID, event.Description)
				}

				// Only store the first event for each column (shouldn't have duplicates)
				if _, exists := columnEvents[column]; !exists {
					columnEvents[column] = eventStr
				}
			}

			// Calculate available width for event text
			scheduleWidth := m.width * 2 / 3
			timeWidth := 7 // "HH:MM  "
			availableWidth := scheduleWidth - timeWidth

			// Calculate space per column based on actual number of columns
			numColumns := maxColumn + 1
			padding := 2 // Space between columns
			columnWidth := availableWidth / numColumns
			if numColumns > 1 {
				columnWidth = (availableWidth - (padding * (numColumns - 1))) / numColumns
			}

			// Only truncate if necessary
			maxLen := columnWidth
			if maxLen < 10 {
				maxLen = 10 // Minimum readable width
			}

			// Now truncate events that are too long for their column
			for col, eventStr := range columnEvents {
				if len(eventStr) > maxLen {
					columnEvents[col] = eventStr[:maxLen-3] + "..."
				}
			}

			// Build the line with proper column spacing
			currentPos := len(timeStr) + 2
			for col := 0; col <= maxColumn; col++ {
				if eventStr, exists := columnEvents[col]; exists {
					// Calculate where this column should start
					targetPos := len(timeStr) + 2 + (col * (maxLen + padding))
					// Add padding to reach the target position
					if targetPos > currentPos {
						line += strings.Repeat(" ", targetPos-currentPos)
						currentPos = targetPos
					}
					line += eventStr
					currentPos += len(eventStr)
				}
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

	// Deduplicate events before processing
	seen := make(map[string]bool)
	var uniqueEvents []remind.Event

	for _, event := range m.events {
		// Use just the event ID which should be unique
		key := event.ID

		if !seen[key] {
			seen[key] = true
			uniqueEvents = append(uniqueEvents, event)
		}
	}

	for _, event := range uniqueEvents {
		if event.Time != nil {
			// Calculate day offset from base date
			// Use calendar days, not 24-hour periods to avoid timezone issues
			baseDate := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), 0, 0, 0, 0, m.selectedDate.Location())
			eventDate := time.Date(event.Date.Year(), event.Date.Month(), event.Date.Day(), 0, 0, 0, 0, event.Date.Location())
			dayDiff := int(eventDate.Sub(baseDate).Hours() / 24)

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
			// Check if this event is already in this slot (shouldn't happen)
			alreadyInSlot := false
			for _, existing := range eventsBySlot[globalSlot] {
				if existing.ID == event.ID {
					alreadyInSlot = true
					break
				}
			}
			if !alreadyInSlot {
				eventsBySlot[globalSlot] = append(eventsBySlot[globalSlot], event)
			}
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
	timeHeader := fmt.Sprintf("Selected: %s at %02d:%02d",
		selectedDate.Format("Mon Jan 2, 2006"),
		hour, minute)
	lines = append(lines, m.styles.Header.Render(timeHeader))

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
				if hours > 0 {
					eventTime += fmt.Sprintf(" (%dh %dm)", hours, minutes)
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

// renderScheduleStatusBar renders the status bar for schedule view
func (m *Model) renderScheduleStatusBar() string {
	dateStr := m.selectedDate.Format("Monday, January 2 at 15:04")

	left := fmt.Sprintf(" Currently: %s", dateStr)

	right := "j/k:slot  H/L:day  J/K:week  z:zoom  o:today  i:IDs  n:new  ?:help  q:quit"

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
