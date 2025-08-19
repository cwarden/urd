package ui

import (
	"fmt"
	"strings"
	"time"

	"urd/internal/remind"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewCalendar() string {
	var sections []string

	// Header
	header := m.renderHeader()
	sections = append(sections, header)

	// Calendar grid
	calendar := m.renderCalendarGrid()

	// Event list for selected date
	eventList := m.renderDayEvents()

	// Layout: calendar on left, events on right
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.Border.Render(calendar),
		"  ",
		m.styles.Border.Render(eventList),
	)
	sections = append(sections, mainContent)

	// Status bar
	status := m.renderStatusBar()
	sections = append(sections, status)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) viewTimedReminders() string {
	var sections []string

	header := m.styles.Header.Render("Timed Reminders")
	sections = append(sections, header)

	// Filter timed events
	var timedEvents []remind.Event
	for _, event := range m.events {
		if event.Time != nil {
			timedEvents = append(timedEvents, event)
		}
	}

	if len(timedEvents) == 0 {
		sections = append(sections, m.styles.Normal.Render("No timed reminders"))
	} else {
		for _, event := range timedEvents {
			line := fmt.Sprintf("%s %s - %s",
				event.Date.Format("Jan 02"),
				event.Time.Format("15:04"),
				event.Description,
			)

			style := m.styles.Event
			if event.Priority > remind.PriorityNone {
				style = m.styles.Priority
			}

			sections = append(sections, style.Render(line))
		}
	}

	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) viewUntimedReminders() string {
	var sections []string

	header := m.styles.Header.Render("Untimed Reminders")
	sections = append(sections, header)

	// Filter untimed events
	var untimedEvents []remind.Event
	for _, event := range m.events {
		if event.Time == nil {
			untimedEvents = append(untimedEvents, event)
		}
	}

	if len(untimedEvents) == 0 {
		sections = append(sections, m.styles.Normal.Render("No untimed reminders"))
	} else {
		for _, event := range untimedEvents {
			line := fmt.Sprintf("%s - %s",
				event.Date.Format("Jan 02"),
				event.Description,
			)

			style := m.styles.Event
			if event.Priority > remind.PriorityNone {
				style = m.styles.Priority
			}

			sections = append(sections, style.Render(line))
		}
	}

	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) viewHelp() string {
	help := []string{
		m.styles.Header.Render("Urd Help"),
		"",
		m.styles.Normal.Render("Navigation:"),
		m.styles.Help.Render("  l/→     - Next day"),
		m.styles.Help.Render("  h/←     - Previous day"),
		m.styles.Help.Render("  j/↓     - Next week/time slot"),
		m.styles.Help.Render("  k/↑     - Previous week/time slot"),
		m.styles.Help.Render("  J       - Jump next week"),
		m.styles.Help.Render("  K       - Jump previous week"),
		m.styles.Help.Render("  >       - Next month"),
		m.styles.Help.Render("  <       - Previous month"),
		m.styles.Help.Render("  t       - Today"),
		"",
		m.styles.Normal.Render("Actions:"),
		m.styles.Help.Render("  n       - New event"),
		m.styles.Help.Render("  r       - Refresh"),
		m.styles.Help.Render("  z       - Zoom (hourly view)"),
		m.styles.Help.Render("  ?       - Toggle help"),
		m.styles.Help.Render("  q       - Quit"),
		"",
		m.styles.Normal.Render("Views:"),
		m.styles.Help.Render("  1       - Calendar view"),
		m.styles.Help.Render("  2       - Hourly schedule"),
		m.styles.Help.Render("  3       - Timed reminders"),
		m.styles.Help.Render("  4       - Untimed reminders"),
		"",
		m.styles.Help.Render("Press any key to return..."),
	}

	return lipgloss.JoinVertical(lipgloss.Left, help...)
}

func (m *Model) viewEventEditor() string {
	var sections []string

	header := m.styles.Header.Render("New Event")
	sections = append(sections, header)
	sections = append(sections, "")

	prompt := m.styles.Normal.Render("Enter event (e.g., 'tomorrow 2pm Meeting with team'):")
	sections = append(sections, prompt)

	// Show input with cursor
	input := m.inputBuffer
	if m.cursorPos < len(input) {
		input = input[:m.cursorPos] + "█" + input[m.cursorPos:]
	} else {
		input = input + "█"
	}

	inputLine := m.styles.Selected.Render(input)
	sections = append(sections, inputLine)
	sections = append(sections, "")

	help := m.styles.Help.Render("Enter to save, Esc to cancel")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderHeader() string {
	monthYear := m.currentDate.Format("January 2006")
	return m.styles.Header.Render(monthYear)
}

func (m *Model) renderCalendarGrid() string {
	var lines []string

	// Day headers
	dayHeaders := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var headerLine []string
	for _, day := range dayHeaders {
		headerLine = append(headerLine, m.styles.Header.Width(4).Render(day))
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, headerLine...))

	// Calculate first day of month and grid
	firstDay := time.Date(m.currentDate.Year(), m.currentDate.Month(), 1, 0, 0, 0, 0, time.Local)
	startOffset := int(firstDay.Weekday())

	// Build calendar grid
	day := firstDay.AddDate(0, 0, -startOffset)
	today := time.Now()

	for week := 0; week < 6; week++ {
		var weekDays []string

		for weekday := 0; weekday < 7; weekday++ {
			dayStr := fmt.Sprintf("%2d", day.Day())

			// Determine style
			style := m.styles.Normal

			if day.Month() != m.currentDate.Month() {
				style = m.styles.Help // Dimmed for other months
			} else if day.Year() == today.Year() && day.YearDay() == today.YearDay() {
				style = m.styles.Today
			} else if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				style = m.styles.Weekend
			}

			if day.Year() == m.selectedDate.Year() && day.YearDay() == m.selectedDate.YearDay() {
				style = m.styles.Selected
			}

			// Check for events
			hasEvent := false
			for _, event := range m.events {
				if event.Date.Year() == day.Year() && event.Date.YearDay() == day.YearDay() {
					hasEvent = true
					break
				}
			}

			if hasEvent && day.Month() == m.currentDate.Month() {
				dayStr = dayStr + "*"
			} else {
				dayStr = dayStr + " "
			}

			weekDays = append(weekDays, style.Width(4).Render(dayStr))
			day = day.AddDate(0, 0, 1)
		}

		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, weekDays...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) renderDayEvents() string {
	var lines []string

	dateStr := m.selectedDate.Format("January 2, 2006")
	lines = append(lines, m.styles.Header.Render(dateStr))
	lines = append(lines, "")

	// Find events for selected date
	var dayEvents []remind.Event
	for _, event := range m.events {
		if event.Date.Year() == m.selectedDate.Year() &&
			event.Date.YearDay() == m.selectedDate.YearDay() {
			dayEvents = append(dayEvents, event)
		}
	}

	if len(dayEvents) == 0 {
		lines = append(lines, m.styles.Help.Render("No events"))
	} else {
		for _, event := range dayEvents {
			timeStr := "All day"
			if event.Time != nil {
				timeStr = event.Time.Format("15:04")
			}

			style := m.styles.Event
			if event.Priority > remind.PriorityNone {
				style = m.styles.Priority
			}

			eventLine := fmt.Sprintf("%s - %s", timeStr, event.Description)
			lines = append(lines, style.Render(eventLine))

			if len(event.Tags) > 0 {
				tagLine := fmt.Sprintf("  Tags: %s", strings.Join(event.Tags, ", "))
				lines = append(lines, m.styles.Help.Render(tagLine))
			}
		}
	}

	// Pad to consistent height
	for len(lines) < 15 {
		lines = append(lines, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) renderStatusBar() string {
	left := fmt.Sprintf(" %s | Events: %d",
		m.selectedDate.Format("Jan 2, 2006"),
		len(m.events))

	right := "? for help | q to quit"

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
