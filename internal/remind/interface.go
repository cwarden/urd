package remind

import (
	"time"
)

// ReminderSource is an interface for sources that can provide events/reminders
type ReminderSource interface {
	// GetEvents returns events between start and end times
	GetEvents(start, end time.Time) ([]Event, error)
	// SetFiles sets the source files (for remind) or configuration (for other sources)
	SetFiles(files []string)
	// WatchFiles returns a channel that sends updates when source files change
	// Returns nil if watching is not supported
	WatchFiles() (<-chan FileChangeEvent, error)
	// StopWatching stops any file watching
	StopWatching() error
}

// FileChangeEvent represents a change to a source file
type FileChangeEvent struct {
	Path      string
	Timestamp time.Time
}
