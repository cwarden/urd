package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/cwarden/urd/internal/remind"
)

// renderCanvasView renders the entire screen using a lipgloss Canvas
func (m *Model) renderCanvasView() string {
	// Calculate basic dimensions
	scheduleWidth := m.width * 2 / 3
	if scheduleWidth < 40 {
		scheduleWidth = 40
	}

	// Calculate time configuration
	slotsPerDay := 24
	if m.timeIncrement == 30 {
		slotsPerDay = 48
	} else if m.timeIncrement == 15 {
		slotsPerDay = 96
	}

	// Reserve space for status bar (2 lines at bottom)
	visibleSlots := m.height - 2
	if visibleSlots < 1 {
		visibleSlots = 1
	}

	var layers []*lipgloss.Layer

	// Create time column layers (individual layers for each time slot)
	timeLayers := m.createTimeColumnLayers(slotsPerDay, visibleSlots)
	layers = append(layers, timeLayers...)

	// Create event block layers
	timeWidth := 7 // "HH:MM  "
	eventAreaWidth := scheduleWidth - timeWidth
	eventLayers := m.createEventBlockLayers(slotsPerDay, visibleSlots, timeWidth, eventAreaWidth)
	layers = append(layers, eventLayers...)

	// Create sidebar layer with 1 column spacing
	sidebarWidth := m.width - scheduleWidth - 1
	if sidebarWidth > 0 {
		sidebarLayer := m.createSidebarLayer(scheduleWidth+1, sidebarWidth)
		layers = append(layers, sidebarLayer)
	}

	// Add status bar layers at the bottom
	statusLayers := m.createStatusBarLayers(visibleSlots)
	layers = append(layers, statusLayers...)

	// Render the canvas
	canvas := lipgloss.NewCanvas(layers...)
	canvasOutput := canvas.Render()

	// Return the Canvas output
	return canvasOutput
}

// createTimeColumnLayers creates individual layers for each time label and date separator
func (m *Model) createTimeColumnLayers(slotsPerDay, visibleSlots int) []*lipgloss.Layer {
	var layers []*lipgloss.Layer
	now := time.Now()
	prevDay := -999
	rowIndex := 0

	for i := 0; i < visibleSlots; i++ {
		globalSlot := m.topSlot + i

		// Calculate day offset
		dayOffset := globalSlot / slotsPerDay
		if globalSlot < 0 {
			dayOffset = -1 + (globalSlot+1)/slotsPerDay
		}

		// Add date separator when day changes
		if dayOffset != prevDay {
			currentDate := m.selectedDate.AddDate(0, 0, dayOffset)
			dateLine := currentDate.Format("─Mon Jan 02")
			dateLayer := lipgloss.NewLayer(m.styles.Header.Render(dateLine)).X(0).Y(rowIndex).Z(0)
			layers = append(layers, dateLayer)
			prevDay = dayOffset
			rowIndex++
		}

		// Calculate time for this slot
		slotInDay := globalSlot % slotsPerDay
		if globalSlot < 0 {
			slotInDay = slotsPerDay + (globalSlot % slotsPerDay)
			if slotInDay == slotsPerDay {
				slotInDay = 0
			}
		}

		hour := slotInDay
		minute := 0

		if m.timeIncrement == 30 {
			hour = slotInDay / 2
			minute = (slotInDay % 2) * 30
		} else if m.timeIncrement == 15 {
			hour = slotInDay / 4
			minute = (slotInDay % 4) * 15
		}

		timeLabel := fmt.Sprintf("%02d:%02d", hour, minute)

		// Calculate current date for this slot
		currentDate := m.selectedDate.AddDate(0, 0, dayOffset)
		slotTime := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(),
			hour, minute, 0, 0, currentDate.Location())

		// Apply styling
		style := m.styles.Normal

		// Highlight current time
		if slotTime.Year() == now.Year() &&
			slotTime.YearDay() == now.YearDay() &&
			slotTime.Hour() == now.Hour() {
			if m.timeIncrement == 60 ||
				(m.timeIncrement == 30 && minute <= now.Minute() && now.Minute() < minute+30) ||
				(m.timeIncrement == 15 && minute <= now.Minute() && now.Minute() < minute+15) {
				style = m.styles.Today
			}
		}

		// Highlight selected slot
		if globalSlot == m.selectedSlot {
			style = m.styles.Selected
		}

		// Create time layer
		timeLayer := lipgloss.NewLayer(style.Render(timeLabel)).X(0).Y(rowIndex).Z(0)
		layers = append(layers, timeLayer)
		rowIndex++
	}

	return layers
}

// createEventBlockLayers creates individual layers for each event block
func (m *Model) createEventBlockLayers(slotsPerDay, visibleSlots, timeWidth, eventAreaWidth int) []*lipgloss.Layer {
	var layers []*lipgloss.Layer

	// Use m.selectedDate as the base reference point for all calculations
	// This ensures events stay on their correct days regardless of scrolling
	baseDate := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), 0, 0, 0, 0, m.selectedDate.Location())

	// Map events to visible slots and assign columns
	type EventPosition struct {
		Event        remind.Event
		StartRow     int // Row index in visible area (accounting for date separators)
		SpanRows     int // Number of rows to span
		Column       int // Column assignment
		ColumnSpan   int // Number of columns to span
		ClippedStart int // For tracking slot occupancy
		ClippedEnd   int // For tracking slot occupancy
	}

	var eventPositions []EventPosition
	slotOccupancy := make(map[int]map[int]bool) // slot -> column -> occupied

	// Sort events by time, then by description for consistent ordering
	sortedEvents := make([]remind.Event, len(m.events))
	copy(sortedEvents, m.events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		// Untimed events go last
		if sortedEvents[i].Time == nil && sortedEvents[j].Time != nil {
			return false
		}
		if sortedEvents[i].Time != nil && sortedEvents[j].Time == nil {
			return true
		}
		if sortedEvents[i].Time == nil && sortedEvents[j].Time == nil {
			// Sort untimed events by priority, then description, then ID
			if sortedEvents[i].Priority != sortedEvents[j].Priority {
				return sortedEvents[i].Priority > sortedEvents[j].Priority
			}
			if sortedEvents[i].Description != sortedEvents[j].Description {
				return sortedEvents[i].Description < sortedEvents[j].Description
			}
			return sortedEvents[i].ID < sortedEvents[j].ID
		}

		// Both have times - sort by date first
		if !sortedEvents[i].Date.Equal(sortedEvents[j].Date) {
			return sortedEvents[i].Date.Before(sortedEvents[j].Date)
		}

		// Same date - sort by time
		iTime := sortedEvents[i].Time.Hour()*60 + sortedEvents[i].Time.Minute()
		jTime := sortedEvents[j].Time.Hour()*60 + sortedEvents[j].Time.Minute()
		if iTime != jTime {
			return iTime < jTime
		}

		// Same time - sort by priority (higher priority first)
		if sortedEvents[i].Priority != sortedEvents[j].Priority {
			return sortedEvents[i].Priority > sortedEvents[j].Priority
		}

		// Sort by description
		if sortedEvents[i].Description != sortedEvents[j].Description {
			return sortedEvents[i].Description < sortedEvents[j].Description
		}

		// Finally, sort by ID for absolute stability
		return sortedEvents[i].ID < sortedEvents[j].ID
	})

	// Calculate positions for each event
	for _, event := range sortedEvents {
		if event.Time == nil {
			continue
		}

		// Calculate event's slot position
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

		eventSlot := dayDiff*slotsPerDay + localSlot

		// Check if event is in visible range
		visibleStart := eventSlot - m.topSlot
		if visibleStart >= visibleSlots {
			continue // Event is after visible area
		}

		// Calculate duration in slots
		slotSpan := 1
		if event.Duration != nil {
			durationMinutes := int(event.Duration.Minutes())
			if m.timeIncrement == 30 {
				slotSpan = (durationMinutes + 29) / 30
			} else if m.timeIncrement == 15 {
				slotSpan = (durationMinutes + 14) / 15
			} else {
				slotSpan = (durationMinutes + 59) / 60
			}
		}

		visibleEnd := visibleStart + slotSpan
		if visibleEnd <= 0 {
			continue // Event is before visible area
		}

		// Clip to visible area
		clippedStart := visibleStart
		if clippedStart < 0 {
			clippedStart = 0
		}
		clippedEnd := visibleEnd
		if clippedEnd > visibleSlots {
			clippedEnd = visibleSlots
		}
		clippedSpan := clippedEnd - clippedStart

		if clippedSpan <= 0 {
			continue
		}

		// Convert slot indices to row indices (accounting for date separators)
		startRow := m.slotToRowIndex(clippedStart, slotsPerDay)
		spanRows := clippedSpan // Simplified: assume 1 slot = 1 row for now

		// Find available column
		column := 0
		for {
			available := true
			for slot := clippedStart; slot < clippedEnd; slot++ {
				if slotOccupancy[slot] == nil {
					slotOccupancy[slot] = make(map[int]bool)
				}
				if slotOccupancy[slot][column] {
					available = false
					break
				}
			}

			if available {
				// Mark slots as occupied
				for slot := clippedStart; slot < clippedEnd; slot++ {
					slotOccupancy[slot][column] = true
				}
				break
			}

			column++
			if column > 10 { // Safety limit
				column = 0
				break
			}
		}

		eventPositions = append(eventPositions, EventPosition{
			Event:        event,
			StartRow:     startRow,
			SpanRows:     spanRows,
			Column:       column,
			ColumnSpan:   1, // Start with single column
			ClippedStart: clippedStart,
			ClippedEnd:   clippedEnd,
		})
	}

	// Calculate initial column width to determine if expansion is needed
	maxColumn := 0
	for _, pos := range eventPositions {
		if pos.Column > maxColumn {
			maxColumn = pos.Column
		}
	}

	initialNumColumns := maxColumn + 1
	if initialNumColumns == 0 {
		return layers // No events
	}

	padding := 2
	initialColumnWidth := eventAreaWidth / initialNumColumns
	if initialNumColumns > 1 {
		initialColumnWidth = (eventAreaWidth - padding*(initialNumColumns-1)) / initialNumColumns
	}

	// After initial column assignment, try to expand events that need more space
	for i := range eventPositions {
		pos := &eventPositions[i]

		// Calculate the text length for this event
		textLen := len(pos.Event.Description)
		if m.showEventIDs {
			textLen += len(pos.Event.ID) + 3 // "[ID] "
		}

		// Only try to expand if the text doesn't fit comfortably in one column
		if textLen <= initialColumnWidth-3 {
			continue // Text fits fine in single column
		}

		// Try to expand rightward
		for nextCol := pos.Column + 1; ; nextCol++ {
			// Check if this column is free for all slots this event occupies
			canExpand := true
			for slot := pos.ClippedStart; slot < pos.ClippedEnd; slot++ {
				if slotOccupancy[slot] != nil && slotOccupancy[slot][nextCol] {
					canExpand = false
					break
				}
			}

			if !canExpand {
				break
			}

			// Mark the new column as occupied
			for slot := pos.ClippedStart; slot < pos.ClippedEnd; slot++ {
				if slotOccupancy[slot] == nil {
					slotOccupancy[slot] = make(map[int]bool)
				}
				slotOccupancy[slot][nextCol] = true
			}

			// Expand the column span
			pos.ColumnSpan++

			// Check if we now have enough space for the text
			currentWidth := initialColumnWidth*pos.ColumnSpan + padding*(pos.ColumnSpan-1)
			if textLen <= currentWidth-3 {
				break // We have enough space now
			}

			// Don't expand too much - leave some space for readability
			if pos.ColumnSpan >= 3 {
				break
			}
		}
	}

	// Recalculate column width based on maximum columns actually used (after expansion)
	maxColumn = 0
	for _, pos := range eventPositions {
		endColumn := pos.Column + pos.ColumnSpan - 1
		if endColumn > maxColumn {
			maxColumn = endColumn
		}
	}

	numColumns := maxColumn + 1
	if numColumns == 0 {
		return layers // No events
	}

	// Recalculate column width based on actual columns used
	columnWidth := eventAreaWidth / numColumns
	if numColumns > 1 {
		columnWidth = (eventAreaWidth - padding*(numColumns-1)) / numColumns
	}
	if columnWidth < 10 {
		columnWidth = 10
	}
	if columnWidth > 60 {
		columnWidth = 60
	}

	// Create layer for each event
	for i, pos := range eventPositions {
		// Calculate the width for this event based on its column span
		eventWidth := columnWidth*pos.ColumnSpan + padding*(pos.ColumnSpan-1)

		// Create event text (only show text if event starts in visible area)
		text := ""
		if visibleStart := pos.Event.Time; visibleStart != nil {
			// Check if this is the start of the event
			eventSlot := m.findEventSlot(pos.Event, slotsPerDay, baseDate)
			visibleEventStart := eventSlot - m.topSlot
			if visibleEventStart >= 0 {
				text = pos.Event.Description
				if m.showEventIDs {
					text = fmt.Sprintf("[%s] %s", pos.Event.ID, text)
				}
				// Use the calculated event width for truncation
				if len(text) > eventWidth {
					text = text[:eventWidth-3] + "..."
				}
			}
		}

		// Get event colors
		bgColor := m.getEventBackgroundColor(pos.Event)
		textColor := m.getEventTextColor(bgColor)

		// Create styled block with calculated width
		block := lipgloss.NewStyle().
			Background(bgColor).
			Foreground(textColor).
			Width(eventWidth).
			Height(pos.SpanRows).
			Render(text)

		// Position the layer
		xPos := timeWidth + pos.Column*(columnWidth+padding)
		yPos := pos.StartRow

		layer := lipgloss.NewLayer(block).
			X(xPos).
			Y(yPos).
			Z(i + 1) // Events have Z > 0, time column is Z = 0

		layers = append(layers, layer)
	}

	return layers
}

// slotToRowIndex converts a slot index to a row index, accounting for date separators
func (m *Model) slotToRowIndex(slotIndex, slotsPerDay int) int {
	// Count exactly how many date separators appear before this slot
	rowIndex := 0
	prevDay := -999

	for i := 0; i <= slotIndex; i++ {
		globalSlot := m.topSlot + i

		// Calculate day offset
		dayOffset := globalSlot / slotsPerDay
		if globalSlot < 0 {
			dayOffset = -1 + (globalSlot+1)/slotsPerDay
		}

		// Add 1 for date separator when day changes
		if dayOffset != prevDay {
			prevDay = dayOffset
			rowIndex++ // Date separator row
		}

		if i == slotIndex {
			return rowIndex
		}

		rowIndex++ // Time slot row
	}

	return rowIndex
}

// findEventSlot finds the slot index for an event
func (m *Model) findEventSlot(event remind.Event, slotsPerDay int, baseDate time.Time) int {
	if event.Time == nil {
		return -1
	}

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

// createSidebarLayer creates the sidebar with calendar and untimed reminders
func (m *Model) createSidebarLayer(xOffset, width int) *lipgloss.Layer {
	var lines []string

	// Add calendar
	lines = append(lines, m.styles.Header.Render("Calendar"))
	calendarContent := m.renderMiniCalendar()
	lines = append(lines, calendarContent)

	// Add spacing
	lines = append(lines, "")

	// Add current slot info
	lines = append(lines, m.styles.Header.Render("Selected"))
	selectedContent := m.renderSelectedSlotEvents()
	lines = append(lines, selectedContent)

	// Add spacing
	lines = append(lines, "")

	// Add untimed reminders for the selected date
	headerText := "Untimed Reminders"
	if m.focusUntimed {
		headerText = "▶ " + headerText
	}
	lines = append(lines, m.styles.Header.Render(headerText))

	// Collect untimed events for the selected date
	var untimedEvents []remind.Event
	for _, event := range m.events {
		// Only show untimed events for the selected date (compare date only, not time)
		if event.Time == nil &&
			event.Date.Year() == m.selectedDate.Year() &&
			event.Date.Month() == m.selectedDate.Month() &&
			event.Date.Day() == m.selectedDate.Day() {
			untimedEvents = append(untimedEvents, event)
		}
	}

	// Sort untimed events for consistent ordering
	sort.Slice(untimedEvents, func(i, j int) bool {
		// Sort by priority first (higher priority first)
		if untimedEvents[i].Priority != untimedEvents[j].Priority {
			return untimedEvents[i].Priority > untimedEvents[j].Priority
		}
		// Then by description alphabetically
		if untimedEvents[i].Description != untimedEvents[j].Description {
			return untimedEvents[i].Description < untimedEvents[j].Description
		}
		// Finally by ID for absolute stability
		return untimedEvents[i].ID < untimedEvents[j].ID
	})

	// Display sorted untimed events
	hasUntimed := len(untimedEvents) > 0
	for untimedIndex, event := range untimedEvents {
		line := event.Description
		if event.Priority > remind.PriorityNone {
			line = strings.Repeat("!", int(event.Priority)) + " " + line
		}
		// Truncate if too long for sidebar
		if len(line) > width-2 {
			line = line[:width-5] + "..."
		}

		// Highlight selected untimed reminder when focused
		if m.focusUntimed && untimedIndex == m.selectedUntimedIndex {
			line = m.styles.Selected.Render(line)
		} else {
			line = m.styles.Normal.Render(line)
		}

		lines = append(lines, line)
	}

	if !hasUntimed {
		lines = append(lines, "(no untimed reminders)")
	}

	sidebarContent := strings.Join(lines, "\n")

	return lipgloss.NewLayer(sidebarContent).
		X(xOffset).
		Y(0).
		Z(1000) // High Z to ensure sidebar is on top
}

// createStatusBarLayers creates layers for the status bar at the bottom of the screen
func (m *Model) createStatusBarLayers(visibleSlots int) []*lipgloss.Layer {
	var layers []*lipgloss.Layer
	now := time.Now()

	// First line: Current time
	dateStr := now.Format("Monday, January 2 at 15:04")
	currentTime := fmt.Sprintf(" Currently: %s", dateStr)
	timeLayer := lipgloss.NewLayer(m.styles.Help.Render(currentTime)).
		X(0).
		Y(visibleSlots).
		Z(2000) // High Z to ensure status bar is on top
	layers = append(layers, timeLayer)

	// Second line: Help shortcuts (or message if present)
	var helpText string
	if m.message != "" {
		helpText = m.message
		helpLayer := lipgloss.NewLayer(m.styles.Message.Render(helpText)).
			X(0).
			Y(visibleSlots + 1).
			Z(2000)
		layers = append(layers, helpLayer)
	} else {
		helpText = "j/k:slot  H/L:day  J/K:week  {/}:month  g:goto  /:search  n:next  z:zoom  o:today  ?:help  q:quit"
		// Right-align the help text
		rightAlignedHelp := m.styles.Help.Copy().Width(m.width).Align(lipgloss.Right).Render(helpText)
		helpLayer := lipgloss.NewLayer(rightAlignedHelp).
			X(0).
			Y(visibleSlots + 1).
			Z(2000)
		layers = append(layers, helpLayer)
	}

	return layers
}
