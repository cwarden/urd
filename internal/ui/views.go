package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewHelp() string {
	help := []string{
		m.styles.Header.Render("Urd Help"),
		"",
		m.styles.Normal.Render("Navigation:"),
		m.styles.Help.Render("  j/↓     - Next time slot"),
		m.styles.Help.Render("  k/↑     - Previous time slot"),
		m.styles.Help.Render("  h/H/←   - Previous day"),
		m.styles.Help.Render("  l/L/→   - Next day"),
		m.styles.Help.Render("  K       - Previous week"),
		m.styles.Help.Render("  J       - Next week"),
		m.styles.Help.Render("  o       - Go to current time"),
		"",
		m.styles.Normal.Render("Actions:"),
		m.styles.Help.Render("  n       - New event"),
		m.styles.Help.Render("  t       - Add timed reminder at cursor"),
		m.styles.Help.Render("  e       - Edit event at cursor"),
		m.styles.Help.Render("  r       - Refresh"),
		m.styles.Help.Render("  z       - Zoom (change time increment)"),
		m.styles.Help.Render("  i       - Toggle event IDs"),
		m.styles.Help.Render("  ?       - Toggle help"),
		m.styles.Help.Render("  q       - Quit"),
		"",
		m.styles.Help.Render("Press any key to return..."),
	}

	return lipgloss.JoinVertical(lipgloss.Left, help...)
}

func (m *Model) viewEventSelector() string {
	var sections []string

	header := m.styles.Header.Render("Select Event to Edit")
	sections = append(sections, header)
	sections = append(sections, "")

	if len(m.eventChoices) == 0 {
		sections = append(sections, m.styles.Help.Render("No events to select"))
	} else {
		for i, event := range m.eventChoices {
			prefix := fmt.Sprintf("%d. ", i+1)

			// Format the event description
			var eventStr string
			if event.Time != nil {
				eventStr = fmt.Sprintf("%s %s - %s",
					event.Time.Format("15:04"),
					event.Description,
					event.Date.Format("Jan 2"))
			} else {
				eventStr = fmt.Sprintf("%s - %s",
					event.Description,
					event.Date.Format("Jan 2"))
			}

			// Highlight the selected item
			if i == m.selectedEventIndex {
				sections = append(sections, m.styles.Selected.Render(prefix+eventStr))
			} else {
				sections = append(sections, m.styles.Normal.Render(prefix+eventStr))
			}
		}
	}

	sections = append(sections, "")
	sections = append(sections, m.styles.Help.Render("Enter/1-9: Select  j/k: Navigate  Esc: Cancel"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
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
