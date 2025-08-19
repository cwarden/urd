package ui

import (
	"fmt"
	"time"

	"urd/internal/config"
	"urd/internal/parser"
	"urd/internal/remind"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewMode int

const (
	ViewCalendar ViewMode = iota
	ViewHourly
	ViewTimedReminders
	ViewUntimedReminders
	ViewHelp
	ViewEventEditor
)

type Model struct {
	// Core components
	config  *config.Config
	client  *remind.Client
	watcher *remind.FileWatcher
	parser  *parser.TimeParser

	// View state
	mode            ViewMode
	currentDate     time.Time
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

func NewModel(cfg *config.Config, client *remind.Client) *Model {
	now := time.Now()

	m := &Model{
		config:        cfg,
		client:        client,
		parser:        parser.NewTimeParser(),
		mode:          ViewHourly,
		currentDate:   now,
		selectedDate:  now,
		events:        []remind.Event{},
		selectedSlot:  now.Hour()*2 + now.Minute()/30, // Default 30-min slots
		timeIncrement: 30,                             // Default to 30-minute slots
		topSlot:       0,
		styles:        DefaultStyles(),
	}

	// Load initial events for hourly view
	m.loadEventsForSchedule()

	// Set up file watcher
	watcher, err := remind.NewFileWatcher(func(path string) {
		// Trigger refresh when files change
		m.loadEvents()
	})
	if err == nil {
		m.watcher = watcher
		for _, file := range cfg.RemindFiles {
			watcher.AddFile(file)
		}
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
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		// Refresh display periodically
		if m.config.AutoRefresh {
			m.loadEvents()
			return m, m.tickCmd()
		}
		return m, nil

	case eventLoadedMsg:
		m.events = msg.events
		return m, nil

	case messageTimeoutMsg:
		m.message = ""
		return m, nil
	}

	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case ViewCalendar:
		return m.viewCalendar()
	case ViewHourly:
		return m.viewHourlySchedule()
	case ViewTimedReminders:
		return m.viewTimedReminders()
	case ViewUntimedReminders:
		return m.viewUntimedReminders()
	case ViewHelp:
		return m.viewHelp()
	case ViewEventEditor:
		return m.viewEventEditor()
	default:
		return m.viewCalendar()
	}
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.mode != ViewEventEditor {
			return m, tea.Quit
		}

	case "?":
		if m.mode == ViewHelp {
			m.mode = ViewCalendar
		} else {
			m.mode = ViewHelp
		}
		return m, nil

	case "r":
		m.loadEvents()
		now := time.Now()
		currentTimeSlot := now.Hour()
		if m.timeIncrement == 30 {
			currentTimeSlot = now.Hour()*2 + now.Minute()/30
		} else if m.timeIncrement == 15 {
			currentTimeSlot = now.Hour()*4 + now.Minute()/15
		}
		m.showMessage(fmt.Sprintf("Refreshed - Now: %02d:%02d, slot=%d, selected=%d", now.Hour(), now.Minute(), currentTimeSlot, m.selectedSlot))
		return m, nil

	case "i", "I":
		// Toggle showing event IDs
		m.showEventIDs = !m.showEventIDs
		if m.showEventIDs {
			m.showMessage("Showing event IDs")
		} else {
			m.showMessage("Hiding event IDs")
		}
		return m, nil

	}

	// Mode-specific handling
	switch m.mode {
	case ViewCalendar:
		return m.handleCalendarKeys(msg)
	case ViewHourly:
		return m.handleHourlyKeys(msg)
	case ViewEventEditor:
		return m.handleEditorKeys(msg)
	}

	return m, nil
}

func (m *Model) handleHourlyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Calculate slots per day based on increment
	slotsPerDay := 24
	if m.timeIncrement == 30 {
		slotsPerDay = 48
	} else if m.timeIncrement == 15 {
		slotsPerDay = 96
	}

	visibleSlots := m.height - 6
	if visibleSlots < 10 {
		visibleSlots = 10
	}

	switch msg.String() {
	case "j", "down":
		// Move down = next time slot (can roll to next day)
		m.selectedSlot++
		// Scroll if needed
		if m.selectedSlot >= m.topSlot+visibleSlots-1 {
			m.topSlot++
		}

	case "k", "up":
		// Move up = previous time slot (can roll to previous day)
		m.selectedSlot--
		// Scroll if needed
		if m.selectedSlot < m.topSlot+1 {
			m.topSlot--
		}

	case "l", "right", "L":
		// Next day - jump forward by one day
		m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "h", "left", "H":
		// Previous day - jump back by one day
		m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "J":
		// Next week - jump forward by one week
		m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "K":
		// Previous week - jump back by one week
		m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
		if m.needsEventReload() {
			m.loadEventsForSchedule()
		}

	case "o":
		// Go to current time - start fresh
		now := time.Now()
		m.selectedDate = now

		// Calculate current time slot for today (where day 0 = today)
		currentTimeSlot := now.Hour()
		if m.timeIncrement == 30 {
			currentTimeSlot = now.Hour()*2 + now.Minute()/30
		} else if m.timeIncrement == 15 {
			currentTimeSlot = now.Hour()*4 + now.Minute()/15
		}

		// Set slots as if today is day 0 (selectedSlot = 0 means 00:00 today)
		m.selectedSlot = currentTimeSlot
		m.topSlot = currentTimeSlot - visibleSlots/2
		if m.topSlot < 0 {
			m.topSlot = 0
		}

		// Show debug message
		m.showMessage(fmt.Sprintf("Now: %02d:%02d, slot=%d, top=%d", now.Hour(), now.Minute(), m.selectedSlot, m.topSlot))

	case "z":
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

		hour := localSlot
		minute := 0
		if m.timeIncrement == 30 {
			hour = localSlot / 2
			minute = (localSlot % 2) * 30
		} else if m.timeIncrement == 15 {
			hour = localSlot / 4
			minute = (localSlot % 4) * 15
		}

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
		newSlotsPerDay := 24
		if m.timeIncrement == 30 {
			newSlotsPerDay = 48
			localSlot = hour*2 + minute/30
		} else if m.timeIncrement == 15 {
			newSlotsPerDay = 96
			localSlot = hour*4 + minute/15
		} else {
			newSlotsPerDay = 24
			localSlot = hour
		}

		m.selectedSlot = dayOffset*newSlotsPerDay + localSlot

		// Adjust top slot proportionally
		m.topSlot = m.topSlot * newSlotsPerDay / (24 * oldIncrement / 60)

	case "n":
		// New event at selected time
		m.mode = ViewEventEditor
		m.editingEvent = nil

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

		timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
		m.inputBuffer = fmt.Sprintf("%s %s ", selectedDate.Format("2006-01-02"), timeStr)
		m.cursorPos = len(m.inputBuffer)

	case "1":
		m.mode = ViewCalendar
		m.currentDate = m.selectedDate
		m.loadEvents()
	}

	return m, nil
}

func (m *Model) handleCalendarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l", "right":
		// Move right = next day
		m.selectedDate = m.selectedDate.AddDate(0, 0, 1)

	case "h", "left":
		// Move left = previous day
		m.selectedDate = m.selectedDate.AddDate(0, 0, -1)

	case "j", "down":
		// Move down = next week
		m.selectedDate = m.selectedDate.AddDate(0, 0, 7)

	case "k", "up":
		// Move up = previous week
		m.selectedDate = m.selectedDate.AddDate(0, 0, -7)

	case "J":
		m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
		m.currentDate = m.currentDate.AddDate(0, 0, 7)

	case "K":
		m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
		m.currentDate = m.currentDate.AddDate(0, 0, -7)

	case ">":
		m.selectedDate = m.selectedDate.AddDate(0, 1, 0)
		m.currentDate = m.currentDate.AddDate(0, 1, 0)

	case "<":
		m.selectedDate = m.selectedDate.AddDate(0, -1, 0)
		m.currentDate = m.currentDate.AddDate(0, -1, 0)

	case "n":
		m.mode = ViewEventEditor
		m.editingEvent = nil
		m.inputBuffer = ""
		m.cursorPos = 0

	case "1":
		m.mode = ViewCalendar

	case "2":
		m.mode = ViewHourly
		now := time.Now()
		// Calculate current slot based on time increment
		currentSlot := now.Hour()
		if m.timeIncrement == 30 {
			currentSlot = now.Hour()*2 + now.Minute()/30
		} else if m.timeIncrement == 15 {
			currentSlot = now.Hour()*4 + now.Minute()/15
		}
		m.selectedSlot = currentSlot
		m.topSlot = 0

	case "3":
		m.mode = ViewTimedReminders

	case "4":
		m.mode = ViewUntimedReminders
	}

	return m, nil
}

func (m *Model) handleEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = ViewCalendar
		return m, nil

	case tea.KeyEnter:
		// Parse and save event
		if m.inputBuffer != "" {
			parsed, err := m.parser.Parse(m.inputBuffer)
			if err == nil {
				dateStr := parsed.Date.Format("2006/01/02")
				timeStr := ""
				if parsed.HasTime {
					timeStr = parsed.Time.Format("15:04")
				}

				err = m.client.AddEvent(parsed.Text, dateStr, timeStr)
				if err == nil {
					m.showMessage("Event added")
					m.loadEvents()
				} else {
					m.showMessage(fmt.Sprintf("Error: %v", err))
				}
			} else {
				m.showMessage(fmt.Sprintf("Parse error: %v", err))
			}
		}
		m.mode = ViewCalendar
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

	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.inputBuffer = m.inputBuffer[:m.cursorPos] + string(r) + m.inputBuffer[m.cursorPos:]
			m.cursorPos++
		}
	}

	return m, nil
}

func (m *Model) loadEvents() {
	// Get events for the current month view
	start := time.Date(m.currentDate.Year(), m.currentDate.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, -1)

	events, err := m.client.GetEvents(start, end)
	if err == nil {
		m.events = events
	}
}

func (m *Model) loadEventsForSchedule() {
	// Load events for a wider date range for hourly view
	start := m.selectedDate.AddDate(0, 0, -14) // Load 2 weeks before
	end := m.selectedDate.AddDate(0, 0, 14)    // Load 2 weeks after

	events, err := m.client.GetEvents(start, end)
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

func (m *Model) showMessage(msg string) {
	m.message = msg
	if m.messageTimer != nil {
		m.messageTimer.Stop()
	}
	m.messageTimer = time.AfterFunc(3*time.Second, func() {
		m.message = ""
	})
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(m.config.RefreshRate, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Message types
type tickMsg struct{}
type messageTimeoutMsg struct{}
type eventLoadedMsg struct {
	events []remind.Event
}
