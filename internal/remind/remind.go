package remind

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	RemindPath string
	Files      []string
	Timezone   *time.Location
}

func NewClient() *Client {
	return &Client{
		RemindPath: "remind",
		Files:      []string{},
		Timezone:   time.Local,
	}
}

func (c *Client) SetFiles(files []string) {
	c.Files = files
}

func (c *Client) GetEvents(start, end time.Time) ([]Event, error) {
	if len(c.Files) == 0 {
		return nil, fmt.Errorf("no remind files configured")
	}

	// Simply call getEventsForMonth for a single month if the date range is within one month
	// This avoids duplicates from calling remind multiple times
	if start.Month() == end.Month() && start.Year() == end.Year() {
		// Single month request
		monthStart := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		events, err := c.getEventsForMonth(monthStart)
		if err != nil {
			return nil, err
		}

		// Filter to date range
		var filtered []Event
		for _, event := range events {
			if !event.Date.Before(start) && !event.Date.After(end) {
				filtered = append(filtered, event)
			}
		}
		return filtered, nil
	}

	// Use a map to deduplicate events for multi-month spans
	eventMap := make(map[string]Event)

	// Get events month by month
	// Start from the first day of the month containing 'start'
	currentMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())

	for currentMonth.Before(end) || currentMonth.Equal(end) {
		events, err := c.getEventsForMonth(currentMonth)
		if err != nil {
			return nil, fmt.Errorf("failed to get events for %s: %w", currentMonth.Format("Jan 2006"), err)
		}

		// Filter events to the requested date range and deduplicate
		for _, event := range events {
			if !event.Date.Before(start) && !event.Date.After(end) {
				// Use the event ID as the deduplication key
				// The ID already includes date and line number which makes it unique
				if _, exists := eventMap[event.ID]; !exists {
					eventMap[event.ID] = event
				}
			}
		}

		// Move to next month
		currentMonth = currentMonth.AddDate(0, 1, 0)

		// Safety check to avoid infinite loop
		if currentMonth.After(end.AddDate(0, 12, 0)) {
			break
		}
	}

	// Convert map back to slice
	var allEvents []Event
	for _, event := range eventMap {
		allEvents = append(allEvents, event)
	}

	return allEvents, nil
}

// getEventsForMonth gets events for a specific month
func (c *Client) getEventsForMonth(monthStart time.Time) ([]Event, error) {
	args := []string{
		"-pppq", // rem2ps format with preprocessing, quiet
		"-l",    // include file and line number
		"-g",    // sort output
		"-b2",   // no time format in output
	}

	// Add remind files
	args = append(args, c.Files...)

	// Add date arguments for the first day of the month
	args = append(args,
		monthName(monthStart.Month()),
		fmt.Sprintf("%d", monthStart.Day()),
		fmt.Sprintf("%d", monthStart.Year()))

	cmd := exec.Command(c.RemindPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if we got JSON output despite error
		if len(output) == 0 {
			return nil, fmt.Errorf("remind command failed: %w", err)
		}
	}

	// Parse JSON output
	months, parseErr := ParseRemindJSON(output)
	if parseErr != nil {
		// Fall back to text parsing if JSON fails
		return c.parseRemindOutput(string(output))
	}

	// Convert JSON entries to events
	var events []Event
	for _, month := range months {
		monthEvents := ConvertJSONToEvents(month.Entries, c.Timezone)
		events = append(events, monthEvents...)
	}

	return events, nil
}

func monthName(m time.Month) string {
	return []string{
		"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}[m]
}

func (c *Client) GetEventsForDate(date time.Time) ([]Event, error) {
	// Get events for the entire day
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
	return c.GetEvents(start, end)
}

func (c *Client) parseRemindOutput(output string) ([]Event, error) {
	var events []Event
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Regex for remind -s output format:
	// date * * duration_mins start_mins time_range description
	// or: date * * * start_mins time description (no duration)
	// or: date * * * * description (untimed)
	lineRe := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2})\s+\*\s+\*\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.Contains(line, "Unknown option") {
			continue
		}

		matches := lineRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		date, err := time.Parse("2006/01/02", matches[1])
		if err != nil {
			continue
		}

		remainder := matches[2]
		parts := strings.Fields(remainder)
		if len(parts) < 2 {
			continue
		}

		var event Event
		event.Date = date
		event.Type = EventNote

		// Parse remind -s format: duration start_time time_str description
		// * * means untimed, * 540 means timed
		idx := 0

		if parts[idx] == "*" {
			idx++
			if idx < len(parts) && parts[idx] == "*" {
				// Untimed event - rest is description
				event.Description = strings.Join(parts[idx+1:], " ")
				event.Type = EventNote
			} else if idx < len(parts) {
				// Timed event - next is start time in minutes
				idx++ // Skip start time

				// Look for time string
				if idx < len(parts) && strings.Contains(parts[idx], ":") {
					t, err := time.Parse("15:04", parts[idx])
					if err == nil {
						eventTime := time.Date(date.Year(), date.Month(), date.Day(),
							t.Hour(), t.Minute(), 0, 0, c.Timezone)
						event.Time = &eventTime
						event.Type = EventReminder
					}
					idx++
				}

				// Rest is description
				if idx < len(parts) {
					event.Description = strings.Join(parts[idx:], " ")
				}
			}
		} else {
			// Has duration - skip it
			idx++
			if idx < len(parts) {
				// Skip start time
				idx++
			}
			if idx < len(parts) && strings.Contains(parts[idx], ":") {
				t, err := time.Parse("15:04", parts[idx])
				if err == nil {
					eventTime := time.Date(date.Year(), date.Month(), date.Day(),
						t.Hour(), t.Minute(), 0, 0, c.Timezone)
					event.Time = &eventTime
					event.Type = EventReminder
				}
				idx++
			}
			// Rest is description
			if idx < len(parts) {
				event.Description = strings.Join(parts[idx:], " ")
			}
		}

		// Parse priority and tags from description
		event.Description, event.Priority, event.Tags = c.parseEventDetails(event.Description)
		event.ID = c.generateEventID(event)

		events = append(events, event)
	}

	return events, scanner.Err()
}

func (c *Client) parseEventDetails(desc string) (string, Priority, []string) {
	priority := PriorityNone
	tags := []string{}

	// Look for priority indicators
	if strings.Contains(desc, "!!!") {
		priority = PriorityHigh
		desc = strings.ReplaceAll(desc, "!!!", "")
	} else if strings.Contains(desc, "!!") {
		priority = PriorityMedium
		desc = strings.ReplaceAll(desc, "!!", "")
	} else if strings.Contains(desc, "!") {
		priority = PriorityLow
		desc = strings.ReplaceAll(desc, "!", "")
	}

	// Extract tags (words starting with @)
	tagRe := regexp.MustCompile(`@\w+`)
	tagMatches := tagRe.FindAllString(desc, -1)
	for _, tag := range tagMatches {
		tags = append(tags, tag[1:]) // Remove @ prefix
	}
	desc = tagRe.ReplaceAllString(desc, "")

	return strings.TrimSpace(desc), priority, tags
}

func (c *Client) generateEventID(event Event) string {
	hash := fmt.Sprintf("%s-%s-%d",
		event.Date.Format("2006-01-02"),
		event.Description,
		len(event.Description))

	// Simple hash for ID
	sum := 0
	for _, b := range hash {
		sum += int(b)
	}

	return fmt.Sprintf("evt-%d", sum)
}

func (c *Client) AddEvent(desc, dateStr, timeStr string) error {
	if len(c.Files) == 0 {
		return fmt.Errorf("no remind files configured")
	}

	// Use first file for new events
	file := c.Files[0]

	// Format remind entry
	var remindLine string
	if timeStr != "" {
		remindLine = fmt.Sprintf("REM %s AT %s MSG %s\n", dateStr, timeStr, desc)
	} else {
		remindLine = fmt.Sprintf("REM %s MSG %s\n", dateStr, desc)
	}

	// Append to file
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open remind file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(remindLine)
	if err != nil {
		return fmt.Errorf("failed to write to remind file: %w", err)
	}

	return nil
}

func (c *Client) TestConnection() error {
	// Test with a simple remind command that should always work
	cmd := exec.Command(c.RemindPath, "-n")
	cmd.Stdin = strings.NewReader("REM MSG test\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if we at least got remind output (it may exit 1 but still work)
		if len(output) > 0 && (strings.Contains(string(output), "No reminders") || strings.Contains(string(output), "REM")) {
			return nil
		}
		return fmt.Errorf("remind command not found or not working: %w", err)
	}
	return nil
}
