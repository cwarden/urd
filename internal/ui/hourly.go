package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cwarden/urd/internal/remind"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"
)

// viewHourlySchedule renders the hourly schedule view
func (m *Model) viewHourlySchedule() string {
	var sections []string

	// Top section: Schedule on left, Calendar + Selected Events on right
	scheduleView := m.renderSchedule()
	calendarView := m.renderMiniCalendar()
	selectedEventsView := m.renderSelectedSlotEvents()
	untimedEventsView := m.renderUntimedEvents()

	// Right side: calendar above, selected events below, untimed at bottom
	rightSide := lipgloss.JoinVertical(
		lipgloss.Left,
		calendarView,
		"",
		selectedEventsView,
		"",
		untimedEventsView,
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
	visibleSlots := m.getVisibleSlots()

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

			// Create a map of column to event data for this slot
			type columnEvent struct {
				text  string
				event remind.Event
			}
			columnEvents := make(map[int]columnEvent)
			maxColumn := 0

			// First pass: figure out how many columns we have and determine event display
			for _, event := range events {
				column := eventColumns[event.ID]
				if column > maxColumn {
					maxColumn = column
				}

				// Determine if this is the start of the event or a continuation
				eventStartSlot := m.findEventStartSlot(event, slotsPerDay)
				isEventStart := globalSlot == eventStartSlot

				var eventStr string
				if isEventStart {
					// Show full event name at the start
					eventStr = event.Description
					if m.showEventIDs {
						// Show event ID for debugging
						eventStr = fmt.Sprintf("[%s] %s", event.ID, event.Description)
					}
				} else {
					// Show continuation indicator
					eventStr = "┃" // Vertical line to show continuation
				}

				// Only store the first event for each column (shouldn't have duplicates)
				if _, exists := columnEvents[column]; !exists {
					columnEvents[column] = columnEvent{
						text:  eventStr,
						event: event,
					}
				}
			}

			// Calculate available width for the schedule
			scheduleWidth := m.width * 2 / 3
			if scheduleWidth < 40 {
				scheduleWidth = 40
			}

			// Calculate column properties
			timeWidth := 7 // "HH:MM  "
			availableWidth := scheduleWidth - timeWidth
			numColumns := maxColumn + 1

			if numColumns > 0 {
				// Build the complete line first, then render with Canvas
				line = timeStr + "  "

				// Calculate column width based on available space
				padding := 2 // Space between columns
				totalPadding := padding * (numColumns - 1)
				columnWidth := (availableWidth - totalPadding) / numColumns

				// Ensure columns fit within available space
				// Don't allow columns to be wider than available space permits
				if columnWidth < 10 {
					columnWidth = 10 // Absolute minimum
					// May need to show fewer columns
				}
				// Cap maximum width for readability
				if columnWidth > 60 {
					columnWidth = 60
				}

				// Ensure we don't overflow the schedule width
				totalNeededWidth := (columnWidth * numColumns) + totalPadding
				if totalNeededWidth > availableWidth {
					// Recalculate to fit
					columnWidth = (availableWidth - totalPadding) / numColumns
					if columnWidth < 10 {
						columnWidth = 10
					}
				}

				// Build a single line with all events properly spaced
				var eventBlocks []string
				for col := 0; col <= maxColumn; col++ {
					if colEvent, exists := columnEvents[col]; exists {
						// Prepare event text
						eventText := colEvent.text

						// Truncate if too long (don't pad - let lipgloss handle width)
						if len(eventText) > columnWidth {
							eventText = eventText[:columnWidth-3] + "..."
						}

						// Create styled block with fixed width
						bgColor := m.getEventBackgroundColor(colEvent.event)
						styledBlock := lipgloss.NewStyle().
							Background(bgColor).
							Foreground(lipgloss.ANSIColor(15)).
							Width(columnWidth). // Use Width to create fixed-width blocks
							Render(eventText)

						eventBlocks = append(eventBlocks, styledBlock)
					}
				}

				// Join all event blocks with proper spacing
				if len(eventBlocks) > 0 {
					eventsLine := strings.Join(eventBlocks, strings.Repeat(" ", padding))
					// Ensure the entire line fits within schedule width
					line += lipgloss.NewStyle().
						MaxWidth(availableWidth).
						Render(eventsLine)
				}
			} else {
				// No events, just the time
				line = timeStr + "  "
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
				style = m.styles.Today.Background(lipgloss.ANSIColor(4))
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

// findEventStartSlot calculates the global slot where an event starts
func (m *Model) findEventStartSlot(event remind.Event, slotsPerDay int) int {
	if event.Time == nil {
		return -1 // Untimed events don't have slots
	}

	// Calculate day offset from base date
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

	return dayDiff*slotsPerDay + localSlot
}

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

	// Sort events by time, then by ID for consistent column assignment
	sort.Slice(uniqueEvents, func(i, j int) bool {
		// First sort by date
		if !uniqueEvents[i].Date.Equal(uniqueEvents[j].Date) {
			return uniqueEvents[i].Date.Before(uniqueEvents[j].Date)
		}
		// Then by time (nil times come last)
		if uniqueEvents[i].Time == nil && uniqueEvents[j].Time == nil {
			return uniqueEvents[i].ID < uniqueEvents[j].ID
		}
		if uniqueEvents[i].Time == nil {
			return false
		}
		if uniqueEvents[j].Time == nil {
			return true
		}
		if !uniqueEvents[i].Time.Equal(*uniqueEvents[j].Time) {
			return uniqueEvents[i].Time.Before(*uniqueEvents[j].Time)
		}
		// Finally by ID for consistent ordering
		return uniqueEvents[i].ID < uniqueEvents[j].ID
	})

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

			// Add event to all slots it spans
			for s := globalSlot; s < globalSlot+eventSlots; s++ {
				// Check if this event is already in this slot (shouldn't happen)
				alreadyInSlot := false
				for _, existing := range eventsBySlot[s] {
					if existing.ID == event.ID {
						alreadyInSlot = true
						break
					}
				}
				if !alreadyInSlot {
					eventsBySlot[s] = append(eventsBySlot[s], event)
				}
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

	// Find events active during this time slot
	var selectedEvents []remind.Event
	for _, event := range m.events {
		if event.Time != nil &&
			event.Date.Year() == selectedDate.Year() &&
			event.Date.YearDay() == selectedDate.YearDay() {

			// Calculate event start and end times
			eventStart := *event.Time
			eventEnd := eventStart
			if event.Duration != nil {
				eventEnd = eventStart.Add(*event.Duration)
			} else {
				// Default to 1 hour if no duration specified
				eventEnd = eventStart.Add(time.Hour)
			}

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
			// Event is active if it starts before slot ends AND ends after slot starts
			if eventStart.Before(slotEnd) && eventEnd.After(slotStart) {
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

// renderUntimedEvents renders untimed reminders for the selected day
func (m *Model) renderUntimedEvents() string {
	// Calculate the selected date based on the selected slot
	slotsPerDay := 24
	if m.timeIncrement == 30 {
		slotsPerDay = 48
	} else if m.timeIncrement == 15 {
		slotsPerDay = 96
	}

	dayOffset := m.selectedSlot / slotsPerDay
	if m.selectedSlot < 0 {
		dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
	}

	selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

	// Find untimed events for the selected day
	var untimedEvents []remind.Event
	selectedIndex := -1
	eventIndex := 0

	for _, event := range m.events {
		if event.Time == nil &&
			event.Date.Year() == selectedDate.Year() &&
			event.Date.YearDay() == selectedDate.YearDay() {
			if m.focusUntimed && eventIndex == m.selectedUntimedIndex {
				selectedIndex = len(untimedEvents)
			}
			untimedEvents = append(untimedEvents, event)
			eventIndex++
		}
	}

	var lines []string

	// Calculate available width for the box
	scheduleWidth := m.width * 2 / 3
	if scheduleWidth < 40 {
		scheduleWidth = 40
	}
	boxWidth := m.width - scheduleWidth - 4
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Header
	headerText := "Untimed Reminders"
	if m.focusUntimed {
		headerText = "▶ " + headerText
	}
	lines = append(lines, m.styles.Header.Render(headerText))

	if len(untimedEvents) == 0 {
		lines = append(lines, "")
		lines = append(lines, m.styles.Help.Render("(no untimed reminders)"))
	} else {
		lines = append(lines, "")

		for i, event := range untimedEvents {
			if i > 0 {
				lines = append(lines, "") // Separator between events
			}

			// Apply selection style if this event is selected
			isSelected := m.focusUntimed && i == selectedIndex

			// Event description
			desc := event.Description
			if m.showEventIDs {
				// Show ID for debugging
				idStr := fmt.Sprintf("ID: %s", event.ID)
				if isSelected {
					lines = append(lines, m.styles.Selected.Render(idStr))
				} else {
					lines = append(lines, m.styles.Help.Render(idStr))
				}
			}

			// Format as bullet point
			bulletPoint := "• " + desc

			// Wrap long descriptions using wordwrap to avoid breaking words/URLs
			maxWidth := boxWidth - 6 // Account for padding and bullet
			if maxWidth < 20 {
				maxWidth = 20 // Minimum width to avoid too narrow wrapping
			}
			wrapped := wordwrap.String(bulletPoint, maxWidth)
			for j, line := range strings.Split(wrapped, "\n") {
				if line != "" {
					// Apply style
					var style lipgloss.Style
					if isSelected {
						style = m.styles.Selected
					} else if event.Priority > remind.PriorityNone {
						style = m.styles.Priority
					} else {
						style = m.styles.Event
					}

					if j == 0 {
						// First line has the bullet
						lines = append(lines, style.Render(line))
					} else {
						// Indent continuation lines
						lines = append(lines, style.Render("  "+line))
					}
				}
			}

			// Tags if any
			if len(event.Tags) > 0 {
				tagStr := "  Tags: " + strings.Join(event.Tags, ", ")
				if isSelected {
					lines = append(lines, m.styles.Selected.Render(tagStr))
				} else {
					lines = append(lines, m.styles.Help.Render(tagStr))
				}
			}

			// Priority indicator
			if event.Priority > remind.PriorityNone {
				priorityStr := "  Priority: "
				switch event.Priority {
				case remind.PriorityLow:
					priorityStr += "!"
				case remind.PriorityMedium:
					priorityStr += "!!"
				case remind.PriorityHigh:
					priorityStr += "!!!"
				}
				if isSelected {
					lines = append(lines, m.styles.Selected.Render(priorityStr))
				} else {
					lines = append(lines, m.styles.Priority.Render(priorityStr))
				}
			}
		}
	}

	// Add border with calculated width
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxStyle := m.styles.Border.Copy().Width(boxWidth)
	return boxStyle.Render(content)
}

// getStatusBarHeight returns the actual height of the rendered status bar
func (m *Model) getStatusBarHeight() int {
	statusBar := m.renderScheduleStatusBar()
	return strings.Count(statusBar, "\n") + 1
}

// renderScheduleStatusBar renders the status bar for schedule view
func (m *Model) renderScheduleStatusBar() string {
	now := time.Now()
	dateStr := now.Format("Monday, January 2 at 15:04")

	// First line: Current time
	currentTime := fmt.Sprintf(" Currently: %s", dateStr)

	// Second line: Help shortcuts (or message if present)
	var helpLine string
	if m.message != "" {
		helpLine = m.styles.Message.Render(m.message)
	} else {
		helpText := "j/k:slot  H/L:day  J/K:week  {/}:month  g:goto  /:search  n:next  z:zoom  o:today  ?:help  q:quit"
		// Right-align the help text using lipgloss
		helpLine = m.styles.Help.Copy().Width(m.width).Align(lipgloss.Right).Render(helpText)
	}

	// Combine the two lines
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.Help.Render(currentTime),
		helpLine,
	)
}
