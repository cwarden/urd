package remind

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// P2WorkPeriod represents a work period from p2 work --json output
type P2WorkPeriod struct {
	TaskID     string    `json:"task_id"`
	TaskName   string    `json:"task_name"`
	User       string    `json:"user"`
	PackageID  string    `json:"package_id"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Hours      float64   `json:"hours"`
	IsComplete bool      `json:"is_complete"`
	TotalHours float64   `json:"total_hours"`
}

// P2Client is a ReminderSource that reads work periods from p2
type P2Client struct {
	P2Path    string // Path to p2 binary
	TasksFile string // Path to tasks.rec file
	ShowAll   bool   // Show all periods (not currently used with work command)
	watcher   *FileWatcher
	eventChan chan FileChangeEvent
}

// NewP2Client creates a new P2 client
func NewP2Client() *P2Client {
	return &P2Client{
		P2Path:    "p2",
		TasksFile: "tasks.rec",
		ShowAll:   false,
	}
}

// SetFiles sets the tasks file to use (implements ReminderSource)
func (c *P2Client) SetFiles(files []string) {
	if len(files) > 0 {
		c.TasksFile = files[0]
	}
}

// GetEvents implements ReminderSource - returns p2 work periods as events
func (c *P2Client) GetEvents(start, end time.Time) ([]Event, error) {
	// Build command - use 'work' instead of 'tasks list'
	args := []string{"work", "--json"}
	if c.TasksFile != "" && c.TasksFile != "tasks.rec" {
		args = append(args, c.TasksFile)
	}

	cmd := exec.Command(c.P2Path, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start p2: %w", err)
	}

	var events []Event
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		var period P2WorkPeriod
		if err := json.Unmarshal(scanner.Bytes(), &period); err != nil {
			continue // Skip malformed lines
		}

		// Convert P2 work period to Event
		event := c.workPeriodToEvent(period)

		// Filter by date range
		// Check if the work period's start date falls within the requested date range
		periodDate := time.Date(period.Start.Year(), period.Start.Month(), period.Start.Day(), 0, 0, 0, 0, period.Start.Location())

		// Skip if the period's date is outside the requested range
		if periodDate.Before(start) || periodDate.After(end) {
			continue
		}

		events = append(events, event)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("p2 command failed: %w", err)
	}

	return events, scanner.Err()
}

// workPeriodToEvent converts a P2WorkPeriod to a remind Event
func (c *P2Client) workPeriodToEvent(period P2WorkPeriod) Event {
	// Create a unique ID for this work period
	periodID := fmt.Sprintf("p2-%s-%s", period.TaskID, period.Start.Format("20060102-150405"))

	// Build description with partial indicator if needed
	description := period.TaskName
	if !period.IsComplete && period.TotalHours > 0 {
		description = fmt.Sprintf("%s (%.1f/%.1fh)", period.TaskName, period.Hours, period.TotalHours)
	}

	event := Event{
		ID:          periodID,
		Description: description,
		Body:        "", // Work periods don't have descriptions
		Type:        EventTodo,
		Priority:    PriorityNone,
		Tags:        []string{},
		// Set the date to midnight of the work period's day
		Date: time.Date(period.Start.Year(), period.Start.Month(), period.Start.Day(), 0, 0, 0, 0, period.Start.Location()),
		// Set the actual start time
		Time: &period.Start,
	}

	// Calculate duration from start to end
	duration := period.End.Sub(period.Start)
	event.Duration = &duration

	// Add package as a tag
	if period.PackageID != "" && period.PackageID != "default" {
		event.Tags = append(event.Tags, period.PackageID)
	}

	// Add user as a tag if present
	if period.User != "" {
		event.Tags = append(event.Tags, fmt.Sprintf("@%s", period.User))
	}

	// Add partial tag if this is a partial work period
	if !period.IsComplete && period.TotalHours > 0 {
		event.Tags = append(event.Tags, "PARTIAL")
	}

	return event
}

// WatchFiles implements ReminderSource - watches tasks.rec for changes
func (c *P2Client) WatchFiles() (<-chan FileChangeEvent, error) {
	if c.watcher != nil {
		return c.eventChan, nil // Already watching
	}

	c.eventChan = make(chan FileChangeEvent, 10)

	watcher, err := NewFileWatcher(func(path string) {
		select {
		case c.eventChan <- FileChangeEvent{Path: path, Timestamp: time.Now()}:
		default:
			// Channel full, drop event
		}
	})
	if err != nil {
		return nil, err
	}

	c.watcher = watcher

	// Watch the tasks file
	if c.TasksFile != "" {
		if err := c.watcher.AddFile(c.TasksFile); err != nil {
			// Non-fatal, continue without watching
		}
	}

	return c.eventChan, nil
}

// StopWatching implements ReminderSource - stops file watching
func (c *P2Client) StopWatching() error {
	if c.watcher == nil {
		return nil
	}

	err := c.watcher.Close()
	c.watcher = nil

	if c.eventChan != nil {
		close(c.eventChan)
		c.eventChan = nil
	}

	return err
}
