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

	args := []string{
		"-pppq", // rem2ps format with preprocessing, quiet
		"-l",    // include file and line number
		"-g",    // sort output
		"-b2",   // no time format in output
	}

	// Add remind files
	args = append(args, c.Files...)

	// Add date arguments (as separate args, not a single string)
	args = append(args,
		monthName(start.Month()),
		fmt.Sprintf("%d", start.Day()),
		fmt.Sprintf("%d", start.Year()))

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
		for _, event := range monthEvents {
			// Filter by date range
			if !event.Date.Before(start) && !event.Date.After(end) {
				events = append(events, event)
			}
		}
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

		// Check if it's a timed event
		if parts[0] != "*" {
			// Has duration and/or time
			idx := 1
			if parts[0] != "*" && len(parts) > idx {
				// Duration in minutes (skip for now)
				idx++
			}

			if idx < len(parts) && parts[idx] != "*" {
				// Start time in minutes from midnight (skip)
				idx++
			}

			// Look for time range (HH:MM-HH:MM or HH:MM)
			if idx < len(parts) {
				timeStr := parts[idx]
				if strings.Contains(timeStr, ":") {
					// Parse time
					timeParts := strings.Split(timeStr, "-")
					if len(timeParts) > 0 {
						t, err := time.Parse("15:04", timeParts[0])
						if err == nil {
							eventTime := time.Date(date.Year(), date.Month(), date.Day(),
								t.Hour(), t.Minute(), 0, 0, c.Timezone)
							event.Time = &eventTime
							event.Type = EventReminder
						}
					}
					idx++
				}

				// Rest is description
				if idx < len(parts) {
					event.Description = strings.Join(parts[idx:], " ")
				}
			}
		} else {
			// Untimed event - everything after the stars is description
			if len(parts) > 1 && parts[1] == "*" {
				event.Description = strings.Join(parts[2:], " ")
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
