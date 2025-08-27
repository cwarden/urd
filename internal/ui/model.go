package ui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/parser"
	"github.com/cwarden/urd/internal/remind"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

type ViewMode int

const (
	ViewHourly ViewMode = iota
	ViewHelp
	ViewEventEditor
	ViewEventSelector     // For choosing between multiple events
	ViewGotoDate          // For entering a date to jump to
	ViewSearch            // For entering search terms
	ViewClipboardSelector // For choosing which event to cut/copy
)

type Model struct {
	// Core components
	config       *config.Config
	source       remind.ReminderSource
	remindClient *remind.Client // Keep reference for remind-specific operations
	parser       *parser.TimeParser

	// View state
	mode            ViewMode
	selectedDate    time.Time
	events          []remind.Event
	eventsLoadedFor time.Time // Track when we last loaded events

	// Hourly view state
	selectedSlot  int // Selected time slot index (can span multiple days)
	timeIncrement int // Minutes per slot (15, 30, or 60)
	topSlot       int // First visible slot in the schedule

	// UI state
	width        int
	height       int
	helpVisible  bool
	message      string
	messageTimer *time.Timer
	showEventIDs bool

	// Editor state
	editingEvent *remind.Event
	inputBuffer  string
	cursorPos    int

	// Event selection state
	eventChoices       []remind.Event
	selectedEventIndex int

	// Clipboard state
	clipboardEvent     *remind.Event
	clipboardCut       bool   // true if event was cut (should be removed on paste)
	clipboardOperation string // "cut" or "copy" - which operation is pending

	// Untimed reminders state
	focusUntimed         bool // true when focused on untimed reminders box
	selectedUntimedIndex int  // index of selected untimed reminder

	// Search state
	searchTerm       string         // current search term
	searchResults    []remind.Event // events matching search
	currentSearchHit int            // index in searchResults
	lastSearchDate   time.Time      // when we last searched (for cache invalidation)

	// Activity tracking
	lastKeyInput time.Time // last time a key was pressed

	// Styles
	styles Styles
}

type Styles struct {
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Today    lipgloss.Style
	Weekend  lipgloss.Style
	Header   lipgloss.Style
	Event    lipgloss.Style
	Priority lipgloss.Style
	Help     lipgloss.Style
	Message  lipgloss.Style
	Border   lipgloss.Style
}

func NewModelWithRemind(cfg *config.Config, source remind.ReminderSource, remindClient *remind.Client) *Model {
	now := time.Now()

	m := &Model{
		config:        cfg,
		source:        source,
		remindClient:  remindClient,
		parser:        parser.NewTimeParser(),
		mode:          ViewHourly,
		selectedDate:  now,
		events:        []remind.Event{},
		selectedSlot:  now.Hour()*2 + now.Minute()/30, // Default 30-min slots (can't use timeToSlot yet as timeIncrement not set)
		timeIncrement: 30,                             // Default to 30-minute slots
		topSlot:       0,
		lastKeyInput:  now, // Initialize to current time
		styles:        DefaultStyles(),
	}

	// Load initial events for hourly view
	m.loadEventsForSchedule()

	// Set up file watcher using the source's watch capability
	if watchChan, err := source.WatchFiles(); err == nil && watchChan != nil {
		// Start a goroutine to handle file change events
		go func() {
			for range watchChan {
				// Trigger refresh when files change
				m.loadEvents()
			}
		}()
	}

	return m
}

func DefaultStyles() Styles {
	return Styles{
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(lipgloss.Color("220")).
			Bold(true),
		Today: lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true),
		Weekend: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Underline(true),
		Event: lipgloss.NewStyle().
			Foreground(lipgloss.Color("40")),
		Priority: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		Message: lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Background(lipgloss.Color("235")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.tickCmd(),
		m.timeUpdateCmd(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		m.lastKeyInput = time.Now()
		return m.handleKeyPress(msg)

	case tickMsg:
		// Refresh display periodically
		if m.config.AutoRefresh {
			m.loadEvents()
			return m, m.tickCmd()
		}
		return m, nil

	case timeUpdateMsg:
		// Update current time display every minute and handle auto-advance
		m.handleInactivityAutoAdvance()
		return m, m.timeUpdateCmd()

	case eventLoadedMsg:
		m.events = msg.events
		return m, nil

	case messageTimeoutMsg:
		m.message = ""
		return m, nil

	case editorFinishedMsg:
		if msg.err != nil {
			m.showMessage(fmt.Sprintf("Editor failed: %v", msg.err))
		} else {
			m.showMessage("Editor session completed")
		}
		// Reload events after editing
		m.loadEvents()
		return m, nil
	}

	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case ViewHourly:
		return m.renderCanvasView()
	case ViewHelp:
		return m.viewHelp()
	case ViewEventEditor:
		return m.viewEventEditor()
	case ViewEventSelector:
		return m.viewEventSelector()
	case ViewGotoDate:
		return m.viewGotoDate()
	case ViewSearch:
		return m.viewSearch()
	case ViewClipboardSelector:
		return m.viewClipboardSelector()
	default:
		panic("unhandled mode")
	}
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Check configured key bindings
	key := msg.String()

	// Handle special key representations
	switch key {
	case "up":
		key = "<up>"
	case "down":
		key = "<down>"
	case "left":
		key = "<left>"
	case "right":
		key = "<right>"
	case "enter":
		key = "<enter>"
	case "tab":
		key = "<tab>"
	case "backspace":
		key = "<backspace>"
	case "esc":
		key = "<esc>"
	case "pgup":
		key = "<pageup>"
	case "pgdown":
		key = "<pagedown>"
	case "home":
		key = "<home>"
	case "ctrl+l":
		key = "\\Cl"
	}

	// Look up the action for this key
	action := m.getActionForKey(key)

	// If there's a configured action for this key, handle it
	if action != "" {
		// Global keys that work in all modes
		switch action {
		case "quit":
			if m.mode != ViewEventEditor {
				return m, tea.Quit
			}
		case "help":
			if m.mode == ViewHelp {
				m.mode = ViewHourly
			} else {
				m.mode = ViewHelp
			}
			return m, nil
		case "refresh":
			m.loadEvents()
			now := time.Now()
			currentTimeSlot := m.getCurrentTimeSlot()
			m.showMessage(fmt.Sprintf("Refreshed - Now: %02d:%02d, slot=%d, selected=%d", now.Hour(), now.Minute(), currentTimeSlot, m.selectedSlot))
			return m, nil
		}
	} else {
		// No configured binding - check for hard-coded keys
		switch key {
		case "ctrl+c":
			if m.mode != ViewEventEditor {
				return m, tea.Quit
			}
		case "i":
			// Toggle showing event IDs (only if not in input modes)
			if m.mode != ViewEventEditor && m.mode != ViewSearch && m.mode != ViewGotoDate {
				m.showEventIDs = !m.showEventIDs
				if m.showEventIDs {
					m.showMessage("Showing event IDs")
				} else {
					m.showMessage("Hiding event IDs")
				}
				return m, nil
			}
		}
	}

	// Mode-specific handling
	switch m.mode {
	case ViewHelp:
		// In help mode, only respond to keys that exit help
		switch key {
		case "?", "<esc>", "q":
			m.mode = ViewHourly
			return m, nil
		}
		// Ignore all other keys in help mode
		return m, nil
	case ViewHourly:
		return m.handleHourlyKeys(msg)
	case ViewEventEditor:
		return m.handleEditorKeys(msg)
	case ViewEventSelector:
		return m.handleEventSelectorKeys(msg)
	case ViewGotoDate:
		return m.handleGotoDateKeys(msg)
	case ViewSearch:
		return m.handleSearchKeys(msg)
	case ViewClipboardSelector:
		return m.handleClipboardSelectorKeys(msg)
	}

	return m, nil
}

// handleInactivityAutoAdvance advances the selected slot to the current time
// if the user has been inactive for more than 5 minutes and is currently at
// the slot immediately before the current time slot.
func (m *Model) handleInactivityAutoAdvance() {
	// Only auto-advance after 5 minutes of inactivity
	if time.Since(m.lastKeyInput) <= 5*time.Minute {
		return
	}

	now := time.Now()

	// Calculate the current slot based on current time increment
	slotsPerDay := m.getSlotsPerDay()
	currentTimeSlot := m.timeToSlot(now.Hour(), now.Minute())

	// Calculate the day offset from the base date (selectedDate at 00:00)
	baseDate := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), 0, 0, 0, 0, m.selectedDate.Location())
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayOffset := int(todayDate.Sub(baseDate).Hours() / 24)

	// Calculate what the current time slot is relative to our base date
	targetSlot := dayOffset*slotsPerDay + currentTimeSlot

	// Only auto-advance if user is at the previous time slot (the slot immediately before current time)
	// This means they were at "now" when they stopped interacting, and time has moved forward by one slot
	if m.selectedSlot != targetSlot-1 {
		return
	}

	// Advance to current time slot
	m.selectedSlot = targetSlot

	// Update selected date to today if we've moved to a different day
	if dayOffset != 0 {
		m.selectedDate = now
		// Reset slot to be relative to today
		m.selectedSlot = currentTimeSlot
	}

	// Ensure the selected slot is visible
	m.centerSelectedSlot()
}

func (m *Model) handleHourlyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Calculate slots per day based on increment
	slotsPerDay := m.getSlotsPerDay()

	visibleSlots := m.getVisibleSlots()

	// Get the key string and action
	key := msg.String()
	// Handle special key representations
	switch key {
	case "up":
		key = "<up>"
	case "down":
		key = "<down>"
	case "left":
		key = "<left>"
	case "right":
		key = "<right>"
	case "enter":
		key = "<enter>"
	case "tab":
		key = "<tab>"
	case "pgup":
		key = "<pageup>"
	case "pgdown":
		key = "<pagedown>"
	case "home":
		key = "<home>"
	}

	action := m.getActionForKey(key)

	switch action {
	case "scroll_down":
		// If focused on untimed reminders, this is handled later
		if m.focusUntimed {
			break
		}
		// Move down = next time slot (can roll to next day)
		m.selectedSlot++
		// Check if selected slot is still visible
		if !m.isSlotVisible(m.selectedSlot) {
			m.topSlot++
		}
		// Update selectedDate to match the day of the selected slot
		m.updateSelectedDateFromSlot()

	case "scroll_up":
		// If focused on untimed reminders, this is handled later
		if m.focusUntimed {
			break
		}
		// Move up = previous time slot (can roll to previous day)
		m.selectedSlot--
		// Check if selected slot is still visible
		if !m.isSlotVisible(m.selectedSlot) {
			m.topSlot--
		}
		// Update selectedDate to match the day of the selected slot
		m.updateSelectedDateFromSlot()

	case "next_day":
		// Next day - jump forward by one day
		m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "previous_day":
		// Previous day - jump back by one day
		m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "next_week":
		// Next week - jump forward by one week
		m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "previous_week":
		// Previous week - jump back by one week
		m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "next_month":
		// Next month - jump forward by one month
		m.selectedDate = m.selectedDate.AddDate(0, 1, 0)
		// Always reload events when changing months
		m.loadEventsForSchedule()

	case "previous_month":
		// Previous month - jump back by one month
		m.selectedDate = m.selectedDate.AddDate(0, -1, 0)
		// Always reload events when changing months
		m.loadEventsForSchedule()

	case "home":
		// Go to current time - start fresh
		now := time.Now()
		m.selectedDate = now

		// Calculate current time slot for today (where day 0 = today)
		currentTimeSlot := m.getCurrentTimeSlot()

		// Set slots as if today is day 0 (selectedSlot = 0 means 00:00 today)
		m.selectedSlot = currentTimeSlot
		m.topSlot = currentTimeSlot - visibleSlots/2
		if m.topSlot < 0 {
			m.topSlot = 0
		}

		// Always load events for the current date (force reload)
		m.loadEventsForSchedule()
		// Show debug message
		m.showMessage(fmt.Sprintf("Now: %02d:%02d, slot=%d, top=%d", now.Hour(), now.Minute(), m.selectedSlot, m.topSlot))

	case "zoom":
		// Zoom - cycle through time increments
		// Convert current slot to time
		dayOffset := m.selectedSlot / slotsPerDay
		localSlot := m.selectedSlot % slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		hour, minute := m.slotToTime(localSlot)

		// Change increment
		oldIncrement := m.timeIncrement
		switch m.timeIncrement {
		case 60:
			m.timeIncrement = 30
		case 30:
			m.timeIncrement = 15
		case 15:
			m.timeIncrement = 60
		}

		// Recalculate slot position with new increment
		newSlotsPerDay := m.getSlotsPerDay()
		localSlot = m.timeToSlot(hour, minute)

		m.selectedSlot = dayOffset*newSlotsPerDay + localSlot

		// Adjust top slot proportionally
		m.topSlot = m.topSlot * newSlotsPerDay / (24 * oldIncrement / 60)

		// Ensure selected slot is visible after zoom
		m.ensureSelectedSlotVisible()

	case "goto":
		// Go to specific date
		m.mode = ViewGotoDate
		m.inputBuffer = ""
		m.cursorPos = 0
		// Don't show a message here since the dialog will show instructions
		return m, nil

	case "begin_search":
		// Start search
		m.mode = ViewSearch
		m.inputBuffer = ""
		m.cursorPos = 0
		return m, nil

	case "search_next":
		// Find next search result
		if m.searchTerm != "" {
			found := m.findNextSearchResult()
			if !found {
				m.showMessage("No more search results found.")
			}
		} else {
			m.showMessage("No active search. Press / to search.")
		}
		return m, nil

	case "quick_add":
		// Quick add event using natural language parsing
		m.mode = ViewEventEditor
		m.editingEvent = nil

		// Clear input buffer for natural language input
		m.inputBuffer = ""
		m.cursorPos = 0

	case "edit_any":
		// If focused on untimed reminders, edit the selected untimed reminder
		if m.focusUntimed {
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

			// Get sorted untimed events and find the selected one
			untimedEvents := m.getSortedUntimedEvents(selectedDate)
			if m.selectedUntimedIndex < len(untimedEvents) {
				event := untimedEvents[m.selectedUntimedIndex]
				// Edit this event
				file, err := m.findEventFile(event)
				if err != nil {
					m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
				} else {
					m.showMessage("Launching editor for untimed reminder...")
					return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
				}
			}
			return m, nil
		}

		// Otherwise, edit event at selected time slot
		event := m.getEventAtSlot(m.selectedSlot)
		if event != nil {
			// Find which file contains this event
			file, err := m.findEventFile(*event)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
			} else {
				m.showMessage("Launching editor...")
				return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
			}
		} else {
			// No event at this slot - edit file for new event
			if len(m.config.RemindFiles) > 0 {
				m.showMessage("Launching editor for new event...")
				return m, m.editCmd(m.config.EditNewCommand, m.config.RemindFiles[0], 0)
			} else {
				m.showMessage("No remind files configured")
			}
		}

	case "new_timed":
		// Add new timed reminder at selected time slot using template
		// Calculate time from selected slot
		dayOffset := m.selectedSlot / slotsPerDay
		localSlot := m.selectedSlot % slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)
		hour, minute := m.slotToTime(localSlot)

		// Format date and time for remind format (e.g., "Aug 19 2025")
		dateStr := fmt.Sprintf("%s %02d %d", monthName(selectedDate.Month()), selectedDate.Day(), selectedDate.Year())
		timeStr := fmt.Sprintf("%02d:%02d", hour, minute)

		// Add the timed event using the template and get the line number
		if m.remindClient == nil {
			m.showMessage("Cannot add events: remind client not available")
			return m, nil
		}
		lineNumber, err := m.remindClient.AddTimedEventFromTemplate(m.config.TimedTemplate, dateStr, timeStr)
		if err != nil {
			m.showMessage(fmt.Sprintf("Failed to add reminder: %v", err))
			return m, nil
		}

		// Launch editor at the new line
		if len(m.config.RemindFiles) > 0 {
			m.showMessage("Launching editor for new timed reminder...")
			return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
		}

	case "new_untimed":
		// Add new untimed reminder at selected date using template
		dayOffset := m.selectedSlot / slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)
		dateStr := fmt.Sprintf("%s %02d %d", monthName(selectedDate.Month()), selectedDate.Day(), selectedDate.Year())

		// Add the untimed event using the template
		if m.remindClient == nil {
			m.showMessage("Cannot add events: remind client not available")
			return m, nil
		}
		lineNumber, err := m.remindClient.AddEventFromTemplate(m.config.UntimedTemplate, dateStr, "")
		if err != nil {
			m.showMessage(fmt.Sprintf("Failed to add untimed reminder: %v", err))
			return m, nil
		}

		// Launch editor at the new line
		if len(m.config.RemindFiles) > 0 {
			m.showMessage("Launching editor for new untimed reminder...")
			return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
		}
		return m, nil

	case "new_template0", "new_template1", "new_template2", "new_template3", "new_template4", "new_template5", "new_template6", "new_template7", "new_template8", "new_template9":
		// Get template number from action name
		templateNum := -1
		if len(action) > 12 { // "new_template" is 12 chars
			templateNum = int(action[12] - '0')
		}
		if templateNum < 0 || templateNum > 9 {
			m.showMessage("Invalid template number")
			return m, nil
		}

		template := m.config.Templates[templateNum]
		if template == "" {
			m.showMessage(fmt.Sprintf("Template %d not configured", templateNum))
			return m, nil
		}

		// Calculate date and time from selected slot
		dayOffset := m.selectedSlot / slotsPerDay
		localSlot := m.selectedSlot % slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)
		hour, minute := m.slotToTime(localSlot)

		dateStr := fmt.Sprintf("%s %02d %d", monthName(selectedDate.Month()), selectedDate.Day(), selectedDate.Year())
		timeStr := fmt.Sprintf("%02d:%02d", hour, minute)

		// Some templates don't use time (untimed ones)
		if strings.Contains(template, "%hour%") || strings.Contains(template, "AT ") {
			// Template uses time
			if m.remindClient == nil {
				m.showMessage("Cannot add events: remind client not available")
				return m, nil
			}
			lineNumber, err := m.remindClient.AddEventFromTemplate(template, dateStr, timeStr)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to add from template: %v", err))
				return m, nil
			}
			if len(m.config.RemindFiles) > 0 {
				m.showMessage(fmt.Sprintf("Created from template %d...", templateNum))
				return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
			}
		} else {
			// Untimed template
			if m.remindClient == nil {
				m.showMessage("Cannot add events: remind client not available")
				return m, nil
			}
			lineNumber, err := m.remindClient.AddEventFromTemplate(template, dateStr, "")
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to add from template: %v", err))
				return m, nil
			}
			if len(m.config.RemindFiles) > 0 {
				m.showMessage(fmt.Sprintf("Created from template %d...", templateNum))
				return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
			}
		}
		return m, nil

	case "edit", "entry_complete":
		// If focused on untimed reminders, edit the selected untimed reminder
		if m.focusUntimed {
			// Calculate the selected date based on the selected slot
			dayOffset := m.selectedSlot / slotsPerDay
			if m.selectedSlot < 0 {
				dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			}

			selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

			// Get sorted untimed events and find the selected one
			untimedEvents := m.getSortedUntimedEvents(selectedDate)
			if m.selectedUntimedIndex < len(untimedEvents) {
				event := untimedEvents[m.selectedUntimedIndex]
				// Edit this event
				file, err := m.findEventFile(event)
				if err != nil {
					m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
				} else {
					m.showMessage("Launching editor for untimed reminder...")
					return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
				}
			}
			return m, nil
		}

		// Edit existing reminder or create new one
		events := m.getEventsAtSlot(m.selectedSlot)

		if len(events) == 0 {
			// No events - create a new timed reminder
			// Calculate time from selected slot
			dayOffset := m.selectedSlot / slotsPerDay
			localSlot := m.selectedSlot % slotsPerDay
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

			// Format date and time for remind format
			dateStr := fmt.Sprintf("%s %02d %d", monthName(selectedDate.Month()), selectedDate.Day(), selectedDate.Year())
			timeStr := fmt.Sprintf("%02d:%02d", hour, minute)

			// Add the timed event using the template and get the line number
			if m.remindClient == nil {
				m.showMessage("Cannot add events: remind client not available")
				return m, nil
			}
			lineNumber, err := m.remindClient.AddTimedEventFromTemplate(m.config.TimedTemplate, dateStr, timeStr)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to add reminder: %v", err))
				return m, nil
			}

			// Launch editor at the new line
			if len(m.config.RemindFiles) > 0 {
				m.showMessage("Creating new timed reminder...")
				return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
			}

		} else if len(events) == 1 {
			// Single event - check if it's a P2 task
			event := events[0]
			if strings.HasPrefix(event.ID, "p2-") {
				// P2 task - do nothing for now
				m.showMessage("P2 tasks cannot be edited from here")
				return m, nil
			}
			// Regular event - edit it directly
			file, err := m.findEventFile(event)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
			} else {
				m.showMessage("Launching editor...")
				return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
			}

		} else {
			// Multiple events - filter out P2 tasks before showing selector
			var editableEvents []remind.Event
			for _, event := range events {
				if !strings.HasPrefix(event.ID, "p2-") {
					editableEvents = append(editableEvents, event)
				}
			}

			if len(editableEvents) == 0 {
				// All events are P2 tasks
				m.showMessage("P2 tasks cannot be edited from here")
				return m, nil
			} else if len(editableEvents) == 1 {
				// Single editable event - edit it directly
				event := editableEvents[0]
				file, err := m.findEventFile(event)
				if err != nil {
					m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
				} else {
					m.showMessage("Launching editor...")
					return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
				}
			} else {
				// Multiple editable events - show selector
				m.eventChoices = editableEvents
				m.selectedEventIndex = 0
				m.mode = ViewEventSelector
			}
			return m, nil
		}
		return m, nil

	case "new_untimed_dialog", "new_template4_dialog", "new_template6_dialog":
		// For dialog versions, we'll use the same logic as non-dialog for now
		// In the future, these could show a prompt for additional input
		var templateNum int
		var template string

		switch action {
		case "new_untimed_dialog":
			template = m.config.UntimedTemplate
		case "new_template4_dialog":
			template = m.config.Templates[4]
			templateNum = 4
		case "new_template6_dialog":
			template = m.config.Templates[6]
			templateNum = 6
		}

		if template == "" {
			if action == "new_untimed_dialog" {
				m.showMessage("Untimed template not configured")
			} else {
				m.showMessage(fmt.Sprintf("Template %d not configured", templateNum))
			}
			return m, nil
		}

		// Calculate date from selected slot
		dayOffset := m.selectedSlot / slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)
		dateStr := fmt.Sprintf("%s %02d %d", monthName(selectedDate.Month()), selectedDate.Day(), selectedDate.Year())

		// These are typically untimed templates
		if m.remindClient == nil {
			m.showMessage("Cannot add events: remind client not available")
			return m, nil
		}
		lineNumber, err := m.remindClient.AddEventFromTemplate(template, dateStr, "")
		if err != nil {
			m.showMessage(fmt.Sprintf("Failed to add from template: %v", err))
			return m, nil
		}

		if len(m.config.RemindFiles) > 0 {
			m.showMessage("Launching editor...")
			return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
		}
		return m, nil

	case "copy":
		// If focused on untimed reminders, copy the selected untimed reminder
		if m.focusUntimed {
			// Calculate the selected date based on the selected slot
			dayOffset := m.selectedSlot / slotsPerDay
			if m.selectedSlot < 0 {
				dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			}

			selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

			// Find the selected untimed event
			eventIndex := 0
			for i := range m.events {
				if m.events[i].Time == nil &&
					m.events[i].Date.Year() == selectedDate.Year() &&
					m.events[i].Date.YearDay() == selectedDate.YearDay() {
					if eventIndex == m.selectedUntimedIndex {
						m.clipboardEvent = &m.events[i]
						m.clipboardCut = false
						m.showMessage("Event copied to clipboard")
						break
					}
					eventIndex++
				}
			}
		} else {
			// Get all events at the selected time slot
			events := m.getEventsAtSlot(m.selectedSlot)
			if len(events) == 0 {
				m.showMessage("No event at current time to copy")
			} else if len(events) == 1 {
				// Single event - copy directly
				m.clipboardEvent = &events[0]
				m.clipboardCut = false
				m.showMessage("Event copied to clipboard")
			} else {
				// Multiple events - show selector
				m.eventChoices = events
				m.selectedEventIndex = 0
				m.clipboardOperation = "copy"
				m.mode = ViewClipboardSelector
			}
		}
		return m, nil

	case "cut":
		// If focused on untimed reminders, cut the selected untimed reminder
		if m.focusUntimed {
			// Calculate the selected date based on the selected slot
			dayOffset := m.selectedSlot / slotsPerDay
			if m.selectedSlot < 0 {
				dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			}

			selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

			// Find the selected untimed event
			eventIndex := 0
			for i := range m.events {
				if m.events[i].Time == nil &&
					m.events[i].Date.Year() == selectedDate.Year() &&
					m.events[i].Date.YearDay() == selectedDate.YearDay() {
					if eventIndex == m.selectedUntimedIndex {
						// Store in clipboard
						m.clipboardEvent = &m.events[i]
						m.clipboardCut = true

						// Immediately remove from file
						if m.remindClient == nil {
							m.showMessage("Cannot remove events: remind client not available")
							return m, nil
						}
						if err := m.remindClient.RemoveEvent(m.events[i]); err != nil {
							m.showMessage(fmt.Sprintf("Failed to cut event: %v", err))
							m.clipboardEvent = nil
							m.clipboardCut = false
						} else {
							m.showMessage("Event cut to clipboard")
							// Reload events to show the change
							m.loadEvents()
						}
						break
					}
					eventIndex++
				}
			}
		} else {
			// Get all events at the selected time slot
			events := m.getEventsAtSlot(m.selectedSlot)
			if len(events) == 0 {
				m.showMessage("No event at current time to cut")
			} else if len(events) == 1 {
				// Single event - cut directly
				m.clipboardEvent = &events[0]
				m.clipboardCut = true

				// Immediately remove from file
				if m.remindClient == nil {
					m.showMessage("Cannot remove events: remind client not available")
					return m, nil
				}
				if err := m.remindClient.RemoveEvent(events[0]); err != nil {
					m.showMessage(fmt.Sprintf("Failed to cut event: %v", err))
					m.clipboardEvent = nil
					m.clipboardCut = false
				} else {
					m.showMessage("Event cut to clipboard")
					// Reload events to show the change
					m.loadEvents()
				}
			} else {
				// Multiple events - show selector
				m.eventChoices = events
				m.selectedEventIndex = 0
				m.clipboardOperation = "cut"
				m.mode = ViewClipboardSelector
			}
		}
		return m, nil

	case "paste":
		// Paste the clipboard event at the selected time slot or as untimed
		if m.clipboardEvent == nil {
			m.showMessage("No event in clipboard")
			return m, nil
		}

		// Calculate the target date from selected slot
		dayOffset := m.selectedSlot / slotsPerDay
		localSlot := m.selectedSlot % slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

		// Create a new event based on the clipboard event
		newEvent := *m.clipboardEvent
		newEvent.Date = selectedDate

		if m.focusUntimed {
			// Pasting into untimed section - remove time
			newEvent.Time = nil
			newEvent.Duration = nil
		} else {
			// Pasting into timed section - set or update time
			hour := localSlot
			minute := 0
			if m.timeIncrement == 30 {
				hour = localSlot / 2
				minute = (localSlot % 2) * 30
			} else if m.timeIncrement == 15 {
				hour = localSlot / 4
				minute = (localSlot % 4) * 15
			}

			newTime := time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day(),
				hour, minute, 0, 0, selectedDate.Location())
			newEvent.Time = &newTime
			// Keep duration if original event had one, otherwise leave nil
		}

		// Add the event to the remind file
		if m.remindClient == nil {
			m.showMessage("Cannot add events: remind client not available")
			return m, nil
		}
		lineNumber, err := m.remindClient.AddEventStruct(newEvent)
		if err != nil {
			m.showMessage(fmt.Sprintf("Failed to paste event: %v", err))
			return m, nil
		}

		// If it was cut, the original was already removed, so just clear clipboard
		if m.clipboardCut {
			m.showMessage("Event moved - launching editor...")
			m.clipboardEvent = nil
			m.clipboardCut = false
		} else {
			m.showMessage("Event pasted - launching editor...")
		}

		// Launch editor for the newly pasted event
		if len(m.config.RemindFiles) > 0 {
			return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
		}
		return m, nil

	case "paste_dialog":
		// Same as paste for now - could add confirmation dialog later
		if m.clipboardEvent == nil {
			m.showMessage("No event in clipboard")
			return m, nil
		}

		// Calculate the target date from selected slot
		dayOffset := m.selectedSlot / slotsPerDay
		localSlot := m.selectedSlot % slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
			localSlot = slotsPerDay + (m.selectedSlot % slotsPerDay)
			if localSlot == slotsPerDay {
				localSlot = 0
				dayOffset++
			}
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

		// Create a new event based on the clipboard event
		newEvent := *m.clipboardEvent
		newEvent.Date = selectedDate

		if m.focusUntimed {
			// Pasting into untimed section - remove time
			newEvent.Time = nil
			newEvent.Duration = nil
		} else {
			// Pasting into timed section - set or update time
			hour := localSlot
			minute := 0
			if m.timeIncrement == 30 {
				hour = localSlot / 2
				minute = (localSlot % 2) * 30
			} else if m.timeIncrement == 15 {
				hour = localSlot / 4
				minute = (localSlot % 4) * 15
			}

			newTime := time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day(),
				hour, minute, 0, 0, selectedDate.Location())
			newEvent.Time = &newTime
			// Keep duration if original event had one, otherwise leave nil
		}

		// Add the event to the remind file
		if m.remindClient == nil {
			m.showMessage("Cannot add events: remind client not available")
			return m, nil
		}
		lineNumber, err := m.remindClient.AddEventStruct(newEvent)
		if err != nil {
			m.showMessage(fmt.Sprintf("Failed to paste event: %v", err))
			return m, nil
		}

		// If it was cut, the original was already removed, so just clear clipboard
		if m.clipboardCut {
			m.showMessage("Event moved - launching editor...")
			m.clipboardEvent = nil
			m.clipboardCut = false
		} else {
			m.showMessage("Event pasted - launching editor...")
		}

		// Launch editor for the newly pasted event
		if len(m.config.RemindFiles) > 0 {
			return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
		}
		return m, nil
	}

	// Handle tab key for switching focus between timed and untimed reminders
	if key == "tab" || key == "<tab>" || action == "next_area" {
		// Toggle focus between timed slots and untimed reminders
		m.focusUntimed = !m.focusUntimed
		if m.focusUntimed {
			// Reset untimed selection index when switching to untimed
			m.selectedUntimedIndex = 0
			m.showMessage("Focused on untimed reminders")
		} else {
			m.showMessage("Focused on timed slots")
		}
		return m, nil
	}

	// Handle navigation within untimed reminders when focused
	if m.focusUntimed {
		// Count untimed events for selected day
		slotsPerDay := m.getSlotsPerDay()

		dayOffset := m.selectedSlot / slotsPerDay
		if m.selectedSlot < 0 {
			dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
		}

		selectedDate := m.selectedDate.AddDate(0, 0, dayOffset)

		// Get sorted untimed events for this day
		untimedEvents := m.getSortedUntimedEvents(selectedDate)
		untimedCount := len(untimedEvents)

		// Handle navigation actions when focused on untimed reminders
		switch action {
		case "scroll_down":
			if m.selectedUntimedIndex < untimedCount-1 {
				m.selectedUntimedIndex++
			}
			return m, nil
		case "scroll_up":
			if m.selectedUntimedIndex > 0 {
				m.selectedUntimedIndex--
			}
			return m, nil
		}

		// Also handle raw keys for navigation (but not enter - that's handled by edit_any)
		switch key {
		case "j", "<down>":
			if m.selectedUntimedIndex < untimedCount-1 {
				m.selectedUntimedIndex++
			}
			return m, nil
		case "k", "<up>":
			if m.selectedUntimedIndex > 0 {
				m.selectedUntimedIndex--
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) handleEventSelectorKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Get the key string and action
	key := msg.String()
	// Handle special key representations
	switch key {
	case "up":
		key = "<up>"
	case "down":
		key = "<down>"
	case "enter":
		key = "<enter>"
	case "esc":
		key = "<esc>"
	}

	action := m.getActionForKey(key)

	// Also check the raw key for actions
	switch action {
	case "entry_cancel":
		// Cancel selection and return to hourly view
		m.mode = ViewHourly
		m.eventChoices = nil
		m.selectedEventIndex = 0
		return m, nil

	case "scroll_down":
		// Move down in the list
		if m.selectedEventIndex < len(m.eventChoices)-1 {
			m.selectedEventIndex++
		}
		return m, nil

	case "scroll_up":
		// Move up in the list
		if m.selectedEventIndex > 0 {
			m.selectedEventIndex--
		}
		return m, nil

	case "entry_complete", "edit":
		// Select the current event and edit it
		if m.selectedEventIndex < len(m.eventChoices) {
			event := m.eventChoices[m.selectedEventIndex]
			// Check if it's a P2 task (shouldn't happen since we filter them, but just in case)
			if strings.HasPrefix(event.ID, "p2-") {
				m.showMessage("P2 tasks cannot be edited from here")
				m.mode = ViewHourly
				m.eventChoices = nil
				m.selectedEventIndex = 0
				return m, nil
			}
			file, err := m.findEventFile(event)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
				m.mode = ViewHourly
			} else {
				m.showMessage("Launching editor...")
				m.mode = ViewHourly
				m.eventChoices = nil
				m.selectedEventIndex = 0
				return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
			}
		}
		return m, nil
	}

	// Handle special cases
	switch key {
	case "<esc>", "q":
		// Cancel selection and return to hourly view
		m.mode = ViewHourly
		m.eventChoices = nil
		m.selectedEventIndex = 0
		return m, nil

	case "j", "<down>":
		// Move down in the list
		if m.selectedEventIndex < len(m.eventChoices)-1 {
			m.selectedEventIndex++
		}
		return m, nil

	case "k", "<up>":
		// Move up in the list
		if m.selectedEventIndex > 0 {
			m.selectedEventIndex--
		}
		return m, nil

	// Number keys for quick selection
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		index := int(msg.String()[0] - '1')
		if index < len(m.eventChoices) {
			event := m.eventChoices[index]
			file, err := m.findEventFile(event)
			if err != nil {
				m.showMessage(fmt.Sprintf("Failed to find event file: %v", err))
				m.mode = ViewHourly
			} else {
				m.showMessage("Launching editor...")
				m.mode = ViewHourly
				m.eventChoices = nil
				m.selectedEventIndex = 0
				return m, m.editCmd(m.config.EditOldCommand, file, event.LineNumber)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleEditorKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.mode = ViewHourly
		return m, nil

	case tea.KeyEnter:
		// Parse and save event using natural language processing
		if m.inputBuffer != "" {
			// Use the new quick event method with natural language parsing
			if m.remindClient == nil {
				m.showMessage("Cannot add events: remind client not available")
				return m, nil
			}
			lineNumber, err := m.remindClient.AddQuickEvent(m.inputBuffer)
			if err == nil {
				m.showMessage("Event added - launching editor...")
				m.mode = ViewHourly
				m.loadEvents()

				// Launch editor for the newly created event
				if len(m.config.RemindFiles) > 0 {
					return m, m.editCmd(m.config.EditOldCommand, m.config.RemindFiles[0], lineNumber)
				}
			} else {
				m.showMessage(fmt.Sprintf("Error: %v", err))
			}
		}
		m.mode = ViewHourly
		return m, nil

	case tea.KeyBackspace:
		if m.cursorPos > 0 {
			m.inputBuffer = m.inputBuffer[:m.cursorPos-1] + m.inputBuffer[m.cursorPos:]
			m.cursorPos--
		}

	case tea.KeyLeft:
		if m.cursorPos > 0 {
			m.cursorPos--
		}

	case tea.KeyRight:
		if m.cursorPos < len(m.inputBuffer) {
			m.cursorPos++
		}

	case tea.KeySpace:
		// Handle space explicitly
		m.inputBuffer = m.inputBuffer[:m.cursorPos] + " " + m.inputBuffer[m.cursorPos:]
		m.cursorPos++

	default:
		for _, r := range msg.Text {
			m.inputBuffer = m.inputBuffer[:m.cursorPos] + string(r) + m.inputBuffer[m.cursorPos:]
			m.cursorPos++
		}
	}

	return m, nil
}

func (m *Model) handleGotoDateKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.mode = ViewHourly
		return m, nil
	case tea.KeyEnter:
		// Parse the date input
		if m.inputBuffer != "" {
			// Try standard date formats FIRST
			dateFormats := []string{
				"2006-01-02", // YYYY-MM-DD
				"01/02/2006", // MM/DD/YYYY
				"1/2/2006",   // M/D/YYYY
				"01/02",      // MM/DD (current year)
				"1/2",        // M/D (current year)
			}

			var parsedDate time.Time
			var parseSuccess bool

			for _, format := range dateFormats {
				if pd, err := time.ParseInLocation(format, m.inputBuffer, time.Local); err == nil {
					// For MM/DD formats without year, use current year
					if format == "01/02" || format == "1/2" {
						parsedDate = time.Date(time.Now().Year(), pd.Month(), pd.Day(),
							0, 0, 0, 0, time.Local)
					} else {
						// Ensure the date is in local timezone with time at midnight
						parsedDate = time.Date(pd.Year(), pd.Month(), pd.Day(),
							0, 0, 0, 0, time.Local)
					}
					parseSuccess = true
					break
				}
			}

			// If standard formats failed, try natural language parsing
			if !parseSuccess {
				parser := &remind.TimeParser{Now: time.Now(), Location: time.Local}
				date, err := parser.ParseDateOnly(m.inputBuffer)
				if err == nil {
					parsedDate = date
					parseSuccess = true
				}
			}

			if parseSuccess {
				// Jump to the parsed date
				m.selectedDate = parsedDate

				// Reset the time slot to noon of the selected day
				m.selectedSlot = m.getNoonSlot()

				// Adjust top slot to center the selected slot
				m.centerSelectedSlot()

				// Load events for the new date
				m.loadEventsForSchedule()
				m.showMessage(fmt.Sprintf("Jumped to %s (slot %d)", m.selectedDate.Format("Monday, Jan 2, 2006"), m.selectedSlot))
				// Clear input buffer
				m.inputBuffer = ""
				m.cursorPos = 0
			} else {
				m.showMessage(fmt.Sprintf("Invalid date format: %s", m.inputBuffer))
			}
		}
		m.mode = ViewHourly
		return m, nil
	case tea.KeyBackspace:
		if m.cursorPos > 0 {
			m.inputBuffer = m.inputBuffer[:m.cursorPos-1] + m.inputBuffer[m.cursorPos:]
			m.cursorPos--
		}
	case tea.KeyLeft:
		if m.cursorPos > 0 {
			m.cursorPos--
		}
	case tea.KeyRight:
		if m.cursorPos < len(m.inputBuffer) {
			m.cursorPos++
		}
	case tea.KeySpace:
		// Handle space explicitly
		m.inputBuffer = m.inputBuffer[:m.cursorPos] + " " + m.inputBuffer[m.cursorPos:]
		m.cursorPos++
	default:
		for _, r := range msg.Text {
			m.inputBuffer = m.inputBuffer[:m.cursorPos] + string(r) + m.inputBuffer[m.cursorPos:]
			m.cursorPos++
		}
	}
	return m, nil
}

func (m *Model) handleSearchKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.mode = ViewHourly
		return m, nil
	case tea.KeyEnter:
		// Perform search
		if m.inputBuffer != "" {
			m.searchTerm = m.inputBuffer
			// Search forward from current position
			found := m.findNextSearchResult()
			if found {
				m.showMessage("Press 'n' to find next occurrence.")
			} else {
				m.showMessage("No results found.")
			}
		}
		m.mode = ViewHourly
		return m, nil
	case tea.KeyBackspace:
		if m.cursorPos > 0 {
			m.inputBuffer = m.inputBuffer[:m.cursorPos-1] + m.inputBuffer[m.cursorPos:]
			m.cursorPos--
		}
	case tea.KeyLeft:
		if m.cursorPos > 0 {
			m.cursorPos--
		}
	case tea.KeyRight:
		if m.cursorPos < len(m.inputBuffer) {
			m.cursorPos++
		}
	case tea.KeySpace:
		// Handle space explicitly
		m.inputBuffer = m.inputBuffer[:m.cursorPos] + " " + m.inputBuffer[m.cursorPos:]
		m.cursorPos++
	default:
		for _, r := range msg.Text {
			m.inputBuffer = m.inputBuffer[:m.cursorPos] + string(r) + m.inputBuffer[m.cursorPos:]
			m.cursorPos++
		}
	}

	// Handle 'n' key even in search mode for next result
	if msg.String() == "n" && m.searchTerm != "" {
		found := m.findNextSearchResult()
		if !found {
			m.showMessage("No more search results found.")
		}
	}

	return m, nil
}

func (m *Model) handleClipboardSelectorKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Get the key string and action
	key := msg.String()
	// Handle special key representations
	switch key {
	case "up":
		key = "<up>"
	case "down":
		key = "<down>"
	case "enter":
		key = "<enter>"
	case "esc":
		key = "<esc>"
	}

	action := m.getActionForKey(key)

	// Also check the raw key for actions
	switch action {
	case "entry_cancel":
		// Cancel selection and return to hourly view
		m.mode = ViewHourly
		m.eventChoices = nil
		m.selectedEventIndex = 0
		m.clipboardOperation = ""
		return m, nil

	case "scroll_down":
		// Move down in the list
		if m.selectedEventIndex < len(m.eventChoices)-1 {
			m.selectedEventIndex++
		}
		return m, nil

	case "scroll_up":
		// Move up in the list
		if m.selectedEventIndex > 0 {
			m.selectedEventIndex--
		}
		return m, nil

	case "entry_complete":
		// Select the current event for clipboard operation
		if m.selectedEventIndex < len(m.eventChoices) {
			event := m.eventChoices[m.selectedEventIndex]

			if m.clipboardOperation == "copy" {
				// Copy the selected event
				m.clipboardEvent = &event
				m.clipboardCut = false
				m.showMessage("Event copied to clipboard")
			} else if m.clipboardOperation == "cut" {
				// Cut the selected event
				m.clipboardEvent = &event
				m.clipboardCut = true

				// Immediately remove from file
				if m.remindClient == nil {
					m.showMessage("Cannot remove events: remind client not available")
				} else if err := m.remindClient.RemoveEvent(event); err != nil {
					m.showMessage(fmt.Sprintf("Failed to cut event: %v", err))
					m.clipboardEvent = nil
					m.clipboardCut = false
				} else {
					m.showMessage("Event cut to clipboard")
					// Reload events to show the change
					m.loadEvents()
				}
			}

			// Return to hourly view
			m.mode = ViewHourly
			m.eventChoices = nil
			m.selectedEventIndex = 0
			m.clipboardOperation = ""
		}
		return m, nil
	}

	// Handle numeric keys for quick selection (1-9)
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		index := int(key[0] - '1')
		if index < len(m.eventChoices) {
			m.selectedEventIndex = index
			// Automatically select if numeric key pressed
			event := m.eventChoices[m.selectedEventIndex]

			if m.clipboardOperation == "copy" {
				// Copy the selected event
				m.clipboardEvent = &event
				m.clipboardCut = false
				m.showMessage("Event copied to clipboard")
			} else if m.clipboardOperation == "cut" {
				// Cut the selected event
				m.clipboardEvent = &event
				m.clipboardCut = true

				// Immediately remove from file
				if m.remindClient == nil {
					m.showMessage("Cannot remove events: remind client not available")
				} else if err := m.remindClient.RemoveEvent(event); err != nil {
					m.showMessage(fmt.Sprintf("Failed to cut event: %v", err))
					m.clipboardEvent = nil
					m.clipboardCut = false
				} else {
					m.showMessage("Event cut to clipboard")
					// Reload events to show the change
					m.loadEvents()
				}
			}

			// Return to hourly view
			m.mode = ViewHourly
			m.eventChoices = nil
			m.selectedEventIndex = 0
			m.clipboardOperation = ""
			return m, nil
		}
	}

	return m, nil
}

// findNextSearchResult searches forward from current position for next matching event
func (m *Model) findNextSearchResult() bool {
	if m.searchTerm == "" {
		return false
	}

	searchLower := strings.ToLower(m.searchTerm)

	// Get current position
	currentDate := m.selectedDate
	currentSlot := m.selectedSlot

	// Calculate current time for timed events
	slotsPerDay := m.getSlotsPerDay()

	// Search forward through events starting from current position
	// First, check if we need to expand our event range
	endDate := m.selectedDate.AddDate(0, 1, 0) // Search up to 1 month ahead

	// Load events for extended range if needed
	events, err := m.source.GetEvents(m.selectedDate, endDate)
	if err != nil {
		return false
	}

	// Helper function to check if event matches search
	eventMatches := func(event remind.Event) bool {
		// Search in description
		if strings.Contains(strings.ToLower(event.Description), searchLower) {
			return true
		}
		// Search in tags
		for _, tag := range event.Tags {
			if strings.Contains(strings.ToLower(tag), searchLower) {
				return true
			}
		}
		return false
	}

	// Helper function to compare event position with current position
	isAfterCurrent := func(event remind.Event) bool {
		// If event is on a later date, it's after current
		if event.Date.After(currentDate) {
			return true
		}

		// If event is on same date
		if event.Date.Year() == currentDate.Year() && event.Date.YearDay() == currentDate.YearDay() {
			// If it's an untimed event and we're focused on untimed, check index
			if event.Time == nil {
				if m.focusUntimed {
					// Find index of this untimed event
					untimedIndex := 0
					for _, e := range events {
						if e.Time == nil &&
							e.Date.Year() == currentDate.Year() &&
							e.Date.YearDay() == currentDate.YearDay() {
							if e.ID == event.ID {
								return untimedIndex > m.selectedUntimedIndex
							}
							untimedIndex++
						}
					}
				}
				// If we're not focused on untimed, untimed events come after timed
				return !m.focusUntimed
			}

			// For timed events, compare time slots
			if event.Time != nil {
				hour := event.Time.Hour()
				minute := event.Time.Minute()
				localSlot := m.timeToSlot(hour, minute)

				// Calculate day offset
				baseDate := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, currentDate.Location())
				eventDate := time.Date(event.Date.Year(), event.Date.Month(), event.Date.Day(), 0, 0, 0, 0, event.Date.Location())
				dayDiff := int(eventDate.Sub(baseDate).Hours() / 24)
				eventSlot := dayDiff*slotsPerDay + localSlot

				return eventSlot > currentSlot
			}
		}

		return false
	}

	// Find the next matching event
	var nextEvent *remind.Event
	for _, event := range events {
		if eventMatches(event) && isAfterCurrent(event) {
			if nextEvent == nil {
				nextEvent = &event
			} else {
				// Choose the earlier of the two events
				if event.Date.Before(nextEvent.Date) {
					nextEvent = &event
				} else if event.Date.Equal(nextEvent.Date) {
					// Same date - compare times
					if event.Time != nil && nextEvent.Time != nil {
						if event.Time.Before(*nextEvent.Time) {
							nextEvent = &event
						}
					} else if event.Time != nil && nextEvent.Time == nil {
						// Timed event comes before untimed
						nextEvent = &event
					}
				}
			}
		}
	}

	if nextEvent != nil {
		// Jump to the found event
		m.selectedDate = nextEvent.Date

		// Load events for the new date if needed
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

		// Set position based on event type
		if nextEvent.Time != nil {
			// Timed event - jump to its time slot
			hour := nextEvent.Time.Hour()
			minute := nextEvent.Time.Minute()
			localSlot := hour
			if m.timeIncrement == 30 {
				localSlot = hour*2 + minute/30
			} else if m.timeIncrement == 15 {
				localSlot = hour*4 + minute/15
			}

			baseDate := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), 0, 0, 0, 0, m.selectedDate.Location())
			eventDate := time.Date(nextEvent.Date.Year(), nextEvent.Date.Month(), nextEvent.Date.Day(), 0, 0, 0, 0, nextEvent.Date.Location())
			dayDiff := int(eventDate.Sub(baseDate).Hours() / 24)
			m.selectedSlot = dayDiff*slotsPerDay + localSlot

			// Adjust view to show the selected slot
			m.centerSelectedSlot()

			m.focusUntimed = false
		} else {
			// Untimed event - focus on untimed section
			m.focusUntimed = true
			// Find the index of this event in untimed events
			untimedIndex := 0
			for _, e := range m.events {
				if e.Time == nil &&
					e.Date.Year() == nextEvent.Date.Year() &&
					e.Date.YearDay() == nextEvent.Date.YearDay() {
					if e.ID == nextEvent.ID {
						m.selectedUntimedIndex = untimedIndex
						break
					}
					untimedIndex++
				}
			}
		}

		m.showMessage(fmt.Sprintf("Found: %s", nextEvent.Description))
		return true
	}

	return false
}

func (m *Model) loadEvents() {
	// Get events for the selected month in hourly view
	start := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, -1)

	events, err := m.source.GetEvents(start, end)
	if err == nil {
		m.events = events
	}
}

func (m *Model) loadEventsForSchedule() {
	// Load events for a wider date range for hourly view
	start := m.selectedDate.AddDate(0, 0, -14) // Load 2 weeks before
	end := m.selectedDate.AddDate(0, 0, 14)    // Load 2 weeks after

	events, err := m.source.GetEvents(start, end)
	if err == nil {
		m.events = events
		m.eventsLoadedFor = m.selectedDate // Track when we last loaded events
	} else {
		// Show error message for debugging
		m.showMessage(fmt.Sprintf("Error loading events: %v", err))
	}
}

// needsEventReload checks if we need to reload events based on current selected date
func (m *Model) needsEventReload() bool {
	if m.eventsLoadedFor.IsZero() {
		return true // Never loaded
	}

	// Reload if we've moved more than 1 week from when we last loaded
	daysSinceLoad := int(m.selectedDate.Sub(m.eventsLoadedFor).Hours() / 24)
	if daysSinceLoad < -7 || daysSinceLoad > 7 {
		return true
	}

	return false
}

// updateSelectedDateFromSlot updates the selectedDate when the selected slot crosses day boundaries
// This keeps the calendar in sync with the hourly view
func (m *Model) updateSelectedDateFromSlot() {
	// In this codebase, selectedDate acts as a reference date where slot 0 = midnight of selectedDate
	// When we scroll through slots, we need to check if we've moved to a different day
	// and update selectedDate accordingly to keep the calendar synchronized

	// Calculate slots per day based on time increment
	slotsPerDay := m.getSlotsPerDay()

	// Calculate which day the selected slot falls on
	dayOffset := m.selectedSlot / slotsPerDay
	if m.selectedSlot < 0 {
		// Handle negative slots correctly
		dayOffset = -1 + (m.selectedSlot+1)/slotsPerDay
	}

	// If we've moved to a different day, update selectedDate and reset slot numbering
	if dayOffset != 0 {
		// Update the reference date to the new day
		m.selectedDate = m.selectedDate.AddDate(0, 0, dayOffset)

		// Adjust slot numbers to be relative to the new reference date
		m.selectedSlot = m.selectedSlot - (dayOffset * slotsPerDay)
		m.topSlot = m.topSlot - (dayOffset * slotsPerDay)

		// Check if we need to reload events for the new date range
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}
	}
}

func (m *Model) getEventAtSlot(slot int) *remind.Event {
	// Calculate slots per day based on increment
	slotsPerDay := m.getSlotsPerDay()

	// Calculate day offset and local slot
	dayOffset := slot / slotsPerDay
	localSlot := slot % slotsPerDay
	if slot < 0 {
		dayOffset = -1 + (slot+1)/slotsPerDay
		localSlot = slotsPerDay + (slot % slotsPerDay)
		if localSlot == slotsPerDay {
			localSlot = 0
			dayOffset++
		}
	}

	// Calculate the target date
	targetDate := m.selectedDate.AddDate(0, 0, dayOffset)

	// Find an event at this time slot
	for _, event := range m.events {
		// Check if event is on the target date
		if event.Date.Year() != targetDate.Year() ||
			event.Date.Month() != targetDate.Month() ||
			event.Date.Day() != targetDate.Day() {
			continue
		}

		// For timed events, check if it matches the time slot
		if event.Time != nil {
			eventHour := event.Time.Hour()
			eventMinute := event.Time.Minute()

			// Calculate which slot this event should be in
			eventSlot := m.timeToSlot(eventHour, eventMinute)

			if eventSlot == localSlot {
				return &event
			}
		} else {
			// For untimed events, return the first one on this day
			return &event
		}
	}

	return nil
}

// getSortedUntimedEvents returns untimed events for the given date, sorted consistently
func (m *Model) getSortedUntimedEvents(date time.Time) []remind.Event {
	var untimedEvents []remind.Event
	for _, event := range m.events {
		if event.Time == nil &&
			event.Date.Year() == date.Year() &&
			event.Date.Month() == date.Month() &&
			event.Date.Day() == date.Day() {
			untimedEvents = append(untimedEvents, event)
		}
	}

	// Sort for consistent ordering
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

	return untimedEvents
}

// getEventsAtSlot returns all events at the specified time slot
func (m *Model) getEventsAtSlot(slot int) []remind.Event {
	var events []remind.Event

	// Calculate slots per day based on increment
	slotsPerDay := m.getSlotsPerDay()

	// Calculate day offset and local slot
	dayOffset := slot / slotsPerDay
	localSlot := slot % slotsPerDay
	if slot < 0 {
		dayOffset = -1 + (slot+1)/slotsPerDay
		localSlot = slotsPerDay + (slot % slotsPerDay)
		if localSlot == slotsPerDay {
			localSlot = 0
			dayOffset++
		}
	}

	// Calculate the target date
	targetDate := m.selectedDate.AddDate(0, 0, dayOffset)

	// Find all events at this time slot
	for _, event := range m.events {
		// Check if event is on the target date
		if event.Date.Year() != targetDate.Year() ||
			event.Date.Month() != targetDate.Month() ||
			event.Date.Day() != targetDate.Day() {
			continue
		}

		// For timed events, check if it matches the time slot
		if event.Time != nil {
			eventHour := event.Time.Hour()
			eventMinute := event.Time.Minute()

			// Calculate which slot this event starts in
			eventStartSlot := m.timeToSlot(eventHour, eventMinute)

			// Calculate how many slots this event spans
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

			// Check if the current slot falls within the event's time range
			if localSlot >= eventStartSlot && localSlot < eventStartSlot+eventSlots {
				events = append(events, event)
			}
		}
		// Don't include untimed events - they're not "at" a time slot
	}

	return events
}

// findEventAtSlot returns the first event at the specified time slot (for copy/cut operations)
func (m *Model) findEventAtSlot(slot int) *remind.Event {
	events := m.getEventsAtSlot(slot)
	if len(events) > 0 {
		return &events[0]
	}
	return nil
}

// findEventFile attempts to locate which remind file contains the given event
func (m *Model) findEventFile(event remind.Event) (string, error) {
	if len(m.config.RemindFiles) == 0 {
		return "", fmt.Errorf("no remind files configured")
	}

	// For now, use the first file as default
	// In a more sophisticated implementation, we could parse the event ID
	// or search through files to find the exact match
	return m.config.RemindFiles[0], nil
}

// monthName returns the three-letter month name for remind format
func monthName(m time.Month) string {
	return []string{
		"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}[m]
}

// getActionForKey returns the action associated with a key binding
func (m *Model) getActionForKey(key string) string {
	// Check if there's a configured binding for this key
	if action, ok := m.config.KeyBindings[key]; ok {
		return action
	}
	return ""
}

func (m *Model) showMessage(msg string) {
	m.message = msg
	if m.messageTimer != nil {
		m.messageTimer.Stop()
	}
	m.messageTimer = time.AfterFunc(3*time.Second, func() {
		m.message = ""
	})
}

// getVisibleSlots returns the number of slots that can be displayed
func (m *Model) getVisibleSlots() int {
	// Reserve 2 lines for status bar (current time and help)
	statusBarHeight := 2
	visibleSlots := m.height - statusBarHeight
	if visibleSlots < 10 {
		visibleSlots = 10
	}
	return visibleSlots
}

// centerSelectedSlot adjusts topSlot to center the selected slot in the view
func (m *Model) centerSelectedSlot() {
	visibleSlots := m.getVisibleSlots()
	m.topSlot = m.selectedSlot - visibleSlots/2

	// Ensure topSlot doesn't go negative
	if m.topSlot < 0 {
		m.topSlot = 0
	}
}

// ensureSelectedSlotVisible adjusts topSlot to make the selected slot visible,
// only scrolling if necessary (minimal scroll)
func (m *Model) ensureSelectedSlotVisible() {
	visibleSlots := m.getVisibleSlots()

	if m.selectedSlot < m.topSlot {
		// Slot is above visible area, scroll up
		m.topSlot = m.selectedSlot
	} else if m.selectedSlot >= m.topSlot+visibleSlots {
		// Slot is below visible area, scroll down to center it
		m.topSlot = m.selectedSlot - visibleSlots/2

		// Ensure topSlot doesn't go negative
		if m.topSlot < 0 {
			m.topSlot = 0
		}
	}
	// If slot is already visible, no need to adjust
}

// getSlotsPerDay returns the number of slots per day based on the time increment
func (m *Model) getSlotsPerDay() int {
	switch m.timeIncrement {
	case 15:
		return 96 // 24 hours * 4 slots per hour
	case 30:
		return 48 // 24 hours * 2 slots per hour
	case 60:
		return 24 // 24 hours * 1 slot per hour
	default:
		return 24 // Default to 60-minute slots
	}
}

// getCurrentTimeSlot returns the slot index for the current time
func (m *Model) getCurrentTimeSlot() int {
	now := time.Now()
	return m.timeToSlot(now.Hour(), now.Minute())
}

// timeToSlot converts hour and minute to a slot index
func (m *Model) timeToSlot(hour, minute int) int {
	switch m.timeIncrement {
	case 15:
		return hour*4 + minute/15
	case 30:
		return hour*2 + minute/30
	case 60:
		return hour
	default:
		return hour // Default to 60-minute slots
	}
}

// slotToTime converts a slot index to hour and minute
func (m *Model) slotToTime(slot int) (hour, minute int) {
	switch m.timeIncrement {
	case 15:
		hour = slot / 4
		minute = (slot % 4) * 15
	case 30:
		hour = slot / 2
		minute = (slot % 2) * 30
	case 60:
		hour = slot
		minute = 0
	default:
		hour = slot // Default to 60-minute slots
		minute = 0
	}
	return hour, minute
}

// getNoonSlot returns the slot index for noon (12:00)
func (m *Model) getNoonSlot() int {
	return m.timeToSlot(12, 0)
}

// isSlotVisible checks if a given slot is actually visible on screen
func (m *Model) isSlotVisible(slot int) bool {
	// Calculate slots per day based on time increment
	slotsPerDay := m.getSlotsPerDay()

	// Calculate visible slots
	visibleSlots := m.getVisibleSlots()

	// Simulate the same rendering logic to count actual visible slots
	prevDay := -999
	actualSlotsRendered := 0

	for i := 0; i < visibleSlots && actualSlotsRendered < visibleSlots; i++ {
		globalSlot := m.topSlot + actualSlotsRendered
		dayOffset := globalSlot / slotsPerDay

		// Handle negative slots
		if globalSlot < 0 {
			dayOffset = -1 + (globalSlot+1)/slotsPerDay
		}

		// Check if this is the slot we're looking for
		if globalSlot == slot {
			return true // Found it within the visible range
		}

		// Check for day change (which adds a separator line)
		if dayOffset != prevDay {
			prevDay = dayOffset
			// Day separator doesn't count as a slot
			continue
		}

		actualSlotsRendered++
	}

	return false // Slot is not visible
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(m.config.RefreshRate, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *Model) timeUpdateCmd() tea.Cmd {
	return tea.Every(time.Minute, func(time.Time) tea.Msg {
		return timeUpdateMsg{}
	})
}

// editCmd launches an external editor using tea.ExecProcess for proper terminal handling
func (m *Model) editCmd(command, filePath string, lineNumber int) tea.Cmd {
	// Expand variables in the command
	expandedCommand := m.expandCommandVariables(command, filePath, lineNumber)

	// Parse the command into program and arguments
	parts, err := m.parseCommand(expandedCommand)
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("failed to parse edit command: %w", err)}
		}
	}

	if len(parts) == 0 {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("empty edit command")}
		}
	}

	// Create the command
	cmd := exec.Command(parts[0], parts[1:]...)

	// Use tea.ExecProcess for proper terminal handling
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// expandCommandVariables replaces template variables in the command string
func (m *Model) expandCommandVariables(command, filePath string, lineNumber int) string {
	result := command
	result = strings.ReplaceAll(result, "%file%", filePath)
	if lineNumber > 0 {
		result = strings.ReplaceAll(result, "%line%", fmt.Sprintf("%d", lineNumber))
	} else {
		// For new events, go to end of file
		result = strings.ReplaceAll(result, "%line%", "999999")
	}
	return result
}

// parseCommand splits a command string into program and arguments
// Handles quoted arguments properly
func (m *Model) parseCommand(command string) ([]string, error) {
	var parts []string
	var current string
	var inQuotes bool
	var quoteChar rune

	for _, r := range command {
		switch {
		case !inQuotes && (r == '"' || r == '\''):
			inQuotes = true
			quoteChar = r
		case inQuotes && r == quoteChar:
			inQuotes = false
		case !inQuotes && r == ' ':
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	if inQuotes {
		return nil, fmt.Errorf("unclosed quote in command: %s", command)
	}

	return parts, nil
}

// Message types
type tickMsg struct{}
type timeUpdateMsg struct{}
type messageTimeoutMsg struct{}
type eventLoadedMsg struct {
	events []remind.Event
}
type editorFinishedMsg struct {
	err error
}
