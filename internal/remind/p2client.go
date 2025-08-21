package remind

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// P2Task represents a task from p2 list --json output
type P2Task struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	PackageID      string     `json:"package_id"`
	User           string     `json:"user"`
	EstimateLow    float64    `json:"estimate_low"`
	EstimateHigh   float64    `json:"estimate_high"`
	Done           bool       `json:"done"`
	OnHold         bool       `json:"on_hold"`
	ScheduledStart *time.Time `json:"scheduled_start"`
	ExpectedEnd    *time.Time `json:"expected_end"`
}

// P2Client is a ReminderSource that reads tasks from p2
type P2Client struct {
	P2Path    string // Path to p2 binary
	TasksFile string // Path to tasks.rec file
	ShowAll   bool   // Show completed tasks
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

// GetEvents implements ReminderSource - returns p2 tasks as events
func (c *P2Client) GetEvents(start, end time.Time) ([]Event, error) {
	// Build command
	args := []string{"tasks", "list", "--json"}
	if c.ShowAll {
		args = append(args, "--all")
	}
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
		var task P2Task
		if err := json.Unmarshal(scanner.Bytes(), &task); err != nil {
			continue // Skip malformed lines
		}

		// Skip completed tasks unless ShowAll is true
		if task.Done && !c.ShowAll {
			continue
		}

		// Skip tasks on hold
		if task.OnHold {
			continue
		}

		// Convert P2 task to Event
		event := c.taskToEvent(task)

		// Filter by date range (comparing dates only, not times)
		// Normalize dates to midnight in local time
		eventDate := time.Date(event.Date.Year(), event.Date.Month(), event.Date.Day(), 0, 0, 0, 0, event.Date.Location())
		startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		endDate := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())

		if eventDate.Before(startDate) || eventDate.After(endDate) {
			continue
		}

		events = append(events, event)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("p2 command failed: %w", err)
	}

	return events, scanner.Err()
}

// taskToEvent converts a P2Task to a remind Event
func (c *P2Client) taskToEvent(task P2Task) Event {
	event := Event{
		ID:          fmt.Sprintf("p2-%s", task.ID),
		Description: task.Name,
		Body:        task.Description,
		Type:        EventTodo,
		Priority:    PriorityNone,
		Tags:        []string{},
	}

	// Use scheduled start date if available, otherwise use today
	if task.ScheduledStart != nil {
		// Set date to midnight of the scheduled day in local time
		t := *task.ScheduledStart
		event.Date = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		// Extract time if it's not midnight
		if t.Hour() != 0 || t.Minute() != 0 {
			eventTime := *task.ScheduledStart
			event.Time = &eventTime
		}
	} else {
		now := time.Now()
		event.Date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// Calculate duration from estimates
	if task.EstimateHigh > 0 {
		// Use the average of low and high estimates
		avgHours := (task.EstimateLow + task.EstimateHigh) / 2
		duration := time.Duration(avgHours * float64(time.Hour))
		event.Duration = &duration
	}

	// Add package as a tag
	if task.PackageID != "" && task.PackageID != "default" {
		event.Tags = append(event.Tags, task.PackageID)
	}

	// Add user as a tag if present
	if task.User != "" {
		event.Tags = append(event.Tags, fmt.Sprintf("@%s", task.User))
	}

	// Mark as completed if done
	if task.Done {
		event.Tags = append(event.Tags, "DONE")
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
