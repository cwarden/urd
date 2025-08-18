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
		"-s",
		"-q",
		"-g",
		"-b1",
		fmt.Sprintf("-sa%s", start.Format("2006/01/02")),
		fmt.Sprintf("-sb%s", end.Format("2006/01/02")),
	}
	args = append(args, c.Files...)

	cmd := exec.Command(c.RemindPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("remind command failed: %w", err)
	}

	return c.parseRemindOutput(string(output))
}

func (c *Client) GetEventsForDate(date time.Time) ([]Event, error) {
	if len(c.Files) == 0 {
		return nil, fmt.Errorf("no remind files configured")
	}

	args := []string{
		"-s",
		"-q",
		"-g",
		"-b1",
		date.Format("02 Jan 2006"),
	}
	args = append(args, c.Files...)

	cmd := exec.Command(c.RemindPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("remind command failed: %w", err)
	}

	return c.parseRemindOutput(string(output))
}

func (c *Client) parseRemindOutput(output string) ([]Event, error) {
	var events []Event
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Regex patterns for parsing remind output
	dateTimeRe := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}) (\d{2}:\d{2}) (.+)$`)
	dateOnlyRe := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}) \* (.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event Event

		// Try to parse timed events
		if matches := dateTimeRe.FindStringSubmatch(line); matches != nil {
			date, err := time.Parse("2006/01/02", matches[1])
			if err != nil {
				continue
			}

			timeStr := matches[2]
			eventTime, err := time.Parse("15:04", timeStr)
			if err != nil {
				continue
			}

			// Combine date and time
			eventDateTime := time.Date(date.Year(), date.Month(), date.Day(),
				eventTime.Hour(), eventTime.Minute(), 0, 0, c.Timezone)

			event = Event{
				Date:        date,
				Time:        &eventDateTime,
				Description: strings.TrimSpace(matches[3]),
				Type:        EventReminder,
			}
		} else if matches := dateOnlyRe.FindStringSubmatch(line); matches != nil {
			// Parse untimed events
			date, err := time.Parse("2006/01/02", matches[1])
			if err != nil {
				continue
			}

			event = Event{
				Date:        date,
				Description: strings.TrimSpace(matches[2]),
				Type:        EventNote,
			}
		} else {
			continue
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
	cmd := exec.Command(c.RemindPath, "-h")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("remind command not found or not working: %w", err)
	}
	return nil
}
