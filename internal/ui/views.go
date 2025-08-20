package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewHelp() string {
	// Create a map of actions to descriptions
	actionDescriptions := map[string]string{
		// Navigation
		"scroll_down":    "Next time slot",
		"scroll_up":      "Previous time slot",
		"previous_day":   "Previous day",
		"next_day":       "Next day",
		"previous_week":  "Previous week",
		"next_week":      "Next week",
		"previous_month": "Previous month",
		"next_month":     "Next month",
		"home":           "Go to current time",
		"goto":           "Go to specific date",
		"zoom":           "Zoom (change time increment)",
		// Basic actions
		"edit":        "Edit/create reminder",
		"edit_any":    "Edit reminder file",
		"new_timed":   "Add timed reminder",
		"new_untimed": "Add untimed reminder",
		"quick_add":   "Quick add event",
		// Templates
		"new_template0":        "Weekly recurring reminder",
		"new_template1":        "Weekly untimed reminder",
		"new_template2":        "Monthly recurring reminder",
		"new_template3":        "Monthly untimed reminder",
		"new_template4_dialog": "Todo (floating date)",
		"new_template5":        "Instantaneous reminder",
		"new_template6_dialog": "Goal with due date",
		"new_template7":        "Floating date reminder",
		"new_template8":        "Weekday floating reminder",
		"new_untimed_dialog":   "Untimed reminder (dialog)",
		// Clipboard
		"copy":  "Copy reminder",
		"cut":   "Cut reminder",
		"paste": "Paste reminder",
		// Search
		"begin_search": "Begin search",
		"search_next":  "Search next",
		// View modes
		"view_week":   "Week view",
		"view_month":  "Month view",
		"view_remind": "Remind output",
		// General
		"refresh": "Refresh",
		"help":    "Toggle help",
		"quit":    "Quit",
	}

	// Build help text using configured key bindings
	help := []string{
		m.styles.Header.Render("Urd Help"),
		"",
		m.styles.Normal.Render("Navigation:"),
	}

	// Helper function to add bound keys for a list of actions
	addBoundActions := func(actions []string) {
		for _, action := range actions {
			// Find all keys bound to this action
			var keys []string
			for key, boundAction := range m.config.KeyBindings {
				if boundAction == action {
					keys = append(keys, key)
				}
			}
			if len(keys) > 0 {
				// Sort keys for consistent display
				sort.Strings(keys)

				// Clean up keys for better display
				var displayKeys []string
				for _, key := range keys {
					// Convert control sequences to more readable format
					if strings.HasPrefix(key, "\\\\C") && len(key) == 4 {
						// Convert \\Cl to Ctrl+L, \\Ca to Ctrl+A, etc.
						ctrlKey := strings.ToUpper(string(key[3]))
						displayKey := "Ctrl+" + ctrlKey
						if !contains(displayKeys, displayKey) {
							displayKeys = append(displayKeys, displayKey)
						}
					} else {
						displayKeys = append(displayKeys, key)
					}
				}

				// Show all keys for this action (up to 3)
				keyStr := displayKeys[0]
				if len(displayKeys) > 1 {
					for i := 1; i < len(displayKeys) && i < 3; i++ {
						keyStr += "/" + displayKeys[i]
					}
					if len(displayKeys) > 3 {
						keyStr += "..."
					}
				}
				if desc, ok := actionDescriptions[action]; ok {
					help = append(help, m.styles.Help.Render(fmt.Sprintf("  %-12s - %s", keyStr, desc)))
				}
			}
		}
	}

	// Navigation section
	navActions := []string{"scroll_down", "scroll_up", "previous_day", "next_day",
		"previous_week", "next_week", "previous_month", "next_month", "home", "goto", "zoom"}
	addBoundActions(navActions)

	help = append(help, "")
	help = append(help, m.styles.Normal.Render("Actions:"))

	// Basic actions
	basicActions := []string{"edit", "edit_any", "quick_add", "new_timed", "new_untimed", "refresh"}
	addBoundActions(basicActions)

	// Templates section
	templateActions := []string{"new_template0", "new_template1", "new_template2", "new_template3",
		"new_template4_dialog", "new_template5", "new_template6_dialog", "new_template7", "new_template8",
		"new_untimed_dialog"}
	// Check if any templates are bound
	hasTemplates := false
	for _, action := range templateActions {
		for _, boundAction := range m.config.KeyBindings {
			if boundAction == action {
				hasTemplates = true
				break
			}
		}
		if hasTemplates {
			break
		}
	}
	if hasTemplates {
		help = append(help, "")
		help = append(help, m.styles.Normal.Render("Templates:"))
		addBoundActions(templateActions)
	}

	// Clipboard section (if bound)
	clipboardActions := []string{"copy", "cut", "paste"}
	hasClipboard := false
	for _, action := range clipboardActions {
		for _, boundAction := range m.config.KeyBindings {
			if boundAction == action {
				hasClipboard = true
				break
			}
		}
	}
	if hasClipboard {
		help = append(help, "")
		help = append(help, m.styles.Normal.Render("Clipboard:"))
		addBoundActions(clipboardActions)
	}

	// Search section (if bound)
	searchActions := []string{"begin_search", "search_next"}
	hasSearch := false
	for _, action := range searchActions {
		for _, boundAction := range m.config.KeyBindings {
			if boundAction == action {
				hasSearch = true
				break
			}
		}
	}
	if hasSearch {
		help = append(help, "")
		help = append(help, m.styles.Normal.Render("Search:"))
		addBoundActions(searchActions)
	}

	// General
	help = append(help, "")
	help = append(help, m.styles.Normal.Render("General:"))
	generalActions := []string{"help", "quit"}
	addBoundActions(generalActions)

	// Add hard-coded keys (only if not bound to something else)
	if _, bound := m.config.KeyBindings["i"]; !bound {
		help = append(help, "")
		help = append(help, m.styles.Normal.Render("Special:"))
		help = append(help, m.styles.Help.Render("  i            - Toggle event IDs"))
	}

	help = append(help, "")
	// Show which keys actually exit help based on configuration
	helpKey := "?"
	for key, action := range m.config.KeyBindings {
		if action == "help" {
			helpKey = key
			break
		}
	}
	help = append(help, m.styles.Help.Render(fmt.Sprintf("Press %s, Esc, or q to return", helpKey)))

	return lipgloss.JoinVertical(lipgloss.Left, help...)
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
