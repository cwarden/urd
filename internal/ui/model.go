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
	mode         ViewMode
	currentDate  time.Time
	selectedDate time.Time
	events       []remind.Event

	// UI state
	width        int
	height       int
	helpVisible  bool
	message      string
	messageTimer *time.Timer

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
		config:       cfg,
		client:       client,
		parser:       parser.NewTimeParser(),
		mode:         ViewCalendar,
		currentDate:  now,
		selectedDate: now,
		events:       []remind.Event{},
		styles:       DefaultStyles(),
	}

	// Load initial events
	m.loadEvents()

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
		m.showMessage("Refreshed")
		return m, nil

	case "t":
		m.selectedDate = time.Now()
		m.currentDate = time.Now()
		return m, nil
	}

	// Mode-specific handling
	switch m.mode {
	case ViewCalendar:
		return m.handleCalendarKeys(msg)
	case ViewEventEditor:
		return m.handleEditorKeys(msg)
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
		m.mode = ViewTimedReminders

	case "3":
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
