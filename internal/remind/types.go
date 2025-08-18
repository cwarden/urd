package remind

import (
	"time"
)

type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
)

type EventType int

const (
	EventReminder EventType = iota
	EventNote
	EventTodo
)

type Event struct {
	ID          string
	Date        time.Time
	Time        *time.Time // nil for untimed events
	Duration    *time.Duration
	Description string
	Body        string
	Priority    Priority
	Type        EventType
	Filename    string
	LineNumber  int
	Tags        []string
	IsRepeating bool
	RepeatSpec  string
}

type Calendar struct {
	Events []Event
	Date   time.Time
}

type DisplayMode int

const (
	DisplayWeek DisplayMode = iota
	DisplayMonth
	DisplayYear
)

type ViewState struct {
	CurrentDate  time.Time
	DisplayMode  DisplayMode
	SelectedDate time.Time
	WindowWidth  int
	WindowHeight int
}

type RemindFile struct {
	Path     string
	ModTime  time.Time
	Events   []Event
	HasError bool
	Error    string
}
