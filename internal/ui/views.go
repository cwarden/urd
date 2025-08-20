package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewHelp() string {
	// Create a map of actions to descriptions
	actionDescriptions := map[string]string{
		"scroll_down":   "Next time slot",
		"scroll_up":     "Previous time slot",
		"previous_day":  "Previous day",
		"next_day":      "Next day",
		"previous_week": "Previous week",
		"next_week":     "Next week",
		"home":          "Go to current time",
		"zoom":          "Zoom (change time increment)",
		"edit":          "Edit/create reminder",
		"edit_any":      "Edit reminder file",
		"new_timed":     "Add timed reminder",
		"quick_add":     "Quick add event",
		"refresh":       "Refresh",
		"help":          "Toggle help",
		"quit":          "Quit",
	}

	// Build help text using configured key bindings
	help := []string{
		m.styles.Header.Render("Urd Help"),
		"",
		m.styles.Normal.Render("Navigation:"),
	}

	// Add navigation keys
	navActions := []string{"scroll_down", "scroll_up", "previous_day", "next_day", "previous_week", "next_week", "home"}
	for _, action := range navActions {
		// Find all keys bound to this action
		var keys []string
		for key, boundAction := range m.config.KeyBindings {
			if boundAction == action {
				keys = append(keys, key)
			}
		}
		if len(keys) > 0 {
			// Show first key binding for this action
			help = append(help, m.styles.Help.Render(fmt.Sprintf("  %-10s - %s", keys[0], actionDescriptions[action])))
		}
	}

	help = append(help, "")
	help = append(help, m.styles.Normal.Render("Actions:"))

	// Add action keys
	actionKeys := []string{"quick_add", "new_timed", "edit", "edit_any", "refresh", "zoom", "help", "quit"}
	for _, action := range actionKeys {
		// Find all keys bound to this action
		var keys []string
		for key, boundAction := range m.config.KeyBindings {
			if boundAction == action {
				keys = append(keys, key)
			}
		}
		if len(keys) > 0 {
			// Show first key binding for this action
			help = append(help, m.styles.Help.Render(fmt.Sprintf("  %-10s - %s", keys[0], actionDescriptions[action])))
		}
	}

	// Add hard-coded keys
	help = append(help, m.styles.Help.Render("  i          - Toggle event IDs"))

	help = append(help, "")
	help = append(help, m.styles.Help.Render("Press any key to return..."))

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
