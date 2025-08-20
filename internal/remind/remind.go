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

// AddEventFromTemplate creates a new reminder using the provided template
// and appends it to the remind file
func (c *Client) AddEventFromTemplate(template, dateStr, timeStr string) (int, error) {
	if len(c.Files) == 0 {
		return 0, fmt.Errorf("no remind files configured")
	}

	// Use first file for new events
	file := c.Files[0]

	// Get current line count to know where we're adding the new entry
	existingContent, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to read remind file: %w", err)
	}
	lineNumber := strings.Count(string(existingContent), "\n") + 1

	// Build the remind line
	remindLine := c.expandTemplate(template, dateStr, timeStr)
	if !strings.HasSuffix(remindLine, "\n") {
		remindLine = remindLine + "\n"
	}

	// Append to file
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open remind file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(remindLine)
	if err != nil {
		return 0, fmt.Errorf("failed to write to remind file: %w", err)
	}

	return lineNumber, nil
}

// AddTimedEventFromTemplate creates a new timed reminder using the provided template
// and appends it to the remind file at the current time slot
func (c *Client) AddTimedEventFromTemplate(template, dateStr, timeStr string) (int, error) {
	if len(c.Files) == 0 {
		return 0, fmt.Errorf("no remind files configured")
	}

	// Use first file for new events
	file := c.Files[0]

	// Get current line count to know where we're adding the new entry
	existingContent, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to read remind file: %w", err)
	}
	lineNumber := strings.Count(string(existingContent), "\n") + 1

	// Build the remind line
	remindLine := c.expandTemplate(template, dateStr, timeStr)
	if remindLine == "" {
		// Fallback to simple format
		remindLine = fmt.Sprintf("REM %s AT %s MSG New reminder\n", dateStr, timeStr)
	} else if !strings.HasSuffix(remindLine, "\n") {
		remindLine = remindLine + "\n"
	}

	// Append to file
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open remind file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(remindLine)
	if err != nil {
		return 0, fmt.Errorf("failed to write to remind file: %w", err)
	}

	return lineNumber, nil
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

// EditEvent opens the remind file for editing at a specific line number
func (c *Client) EditEvent(event Event, editCommand string) error {
	if editCommand == "" {
		return fmt.Errorf("no edit command specified")
	}

	// Find which file contains this event
	file, err := c.findEventFile(event)
	if err != nil {
		return fmt.Errorf("failed to find event file: %w", err)
	}

	return c.executeEditCommand(editCommand, file, event.LineNumber)
}

// EditFile opens a remind file for editing (for new events)
func (c *Client) EditFile(filePath string, editCommand string) error {
	if editCommand == "" {
		return fmt.Errorf("no edit command specified")
	}

	return c.executeEditCommand(editCommand, filePath, 0)
}

// findEventFile attempts to locate which remind file contains the given event
func (c *Client) findEventFile(event Event) (string, error) {
	if len(c.Files) == 0 {
		return "", fmt.Errorf("no remind files configured")
	}

	// For now, use the first file as default
	// In a more sophisticated implementation, we could parse the event ID
	// or search through files to find the exact match
	return c.Files[0], nil
}

// executeEditCommand runs the editor command with proper variable substitution
func (c *Client) executeEditCommand(command, filePath string, lineNumber int) error {
	// Expand variables in the command
	expandedCommand := c.expandCommandVariables(command, filePath, lineNumber)

	// Parse the command into program and arguments
	parts, err := c.parseCommand(expandedCommand)
	if err != nil {
		return fmt.Errorf("failed to parse edit command: %w", err)
	}

	if len(parts) == 0 {
		return fmt.Errorf("empty edit command")
	}

	// Execute the editor
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the editor and wait for it to complete
	err = cmd.Run()

	// Give the terminal a moment to settle after editor exit
	// This helps with screen redraw issues
	if err == nil {
		// Send a clear screen sequence to help with redraw
		fmt.Print("\033[2J\033[H")
	}

	return err
}

// expandCommandVariables replaces template variables in the command string
func (c *Client) expandCommandVariables(command, filePath string, lineNumber int) string {
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
func (c *Client) parseCommand(command string) ([]string, error) {
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

// expandTemplate replaces template placeholders with actual values
func (c *Client) expandTemplate(template, dateStr, timeStr string) string {
	if template == "" {
		return ""
	}

	// Parse the date and time to get components
	// dateStr is like "Aug 19 2025", timeStr is like "16:30"
	var monthName, dayStr, yearStr, monthNum string
	var hourStr, minStr string
	var weekdayName string

	if dateStr != "" {
		parts := strings.Fields(dateStr)
		if len(parts) >= 3 {
			monthName = parts[0] // "Aug"
			dayStr = parts[1]    // "19"
			yearStr = parts[2]   // "2025"

			// Parse to get numeric month and weekday
			if t, err := time.Parse("Jan 2 2006", dateStr); err == nil {
				monthNum = fmt.Sprintf("%d", int(t.Month()))
				weekdayName = t.Format("Mon")
			}
		}
	}

	if timeStr != "" {
		timeParts := strings.Split(timeStr, ":")
		if len(timeParts) >= 2 {
			hourStr = timeParts[0] // "16"
			minStr = timeParts[1]  // "30"
		}
	}

	// Replace template placeholders
	remindLine := template
	remindLine = strings.ReplaceAll(remindLine, "%monname%", monthName)
	remindLine = strings.ReplaceAll(remindLine, "%mon%", monthNum)
	remindLine = strings.ReplaceAll(remindLine, "%mday%", dayStr)
	remindLine = strings.ReplaceAll(remindLine, "%year%", yearStr)
	remindLine = strings.ReplaceAll(remindLine, "%hour%", hourStr)
	remindLine = strings.ReplaceAll(remindLine, "%min%", minStr)
	remindLine = strings.ReplaceAll(remindLine, "%wdayname%", weekdayName)
	remindLine = strings.ReplaceAll(remindLine, "%wday%", fmt.Sprintf("%d", getWeekdayNum(weekdayName)))
	remindLine = strings.ReplaceAll(remindLine, "%dura%", "1") // Default 1 hour duration

	// Remove the trailing % if present
	if strings.HasSuffix(remindLine, "%") {
		remindLine = remindLine[:len(remindLine)-1]
	}

	return remindLine
}

// getWeekdayNum returns the weekday number (0=Sunday, 6=Saturday)
func getWeekdayNum(weekdayName string) int {
	switch weekdayName {
	case "Sun":
		return 0
	case "Mon":
		return 1
	case "Tue":
		return 2
	case "Wed":
		return 3
	case "Thu":
		return 4
	case "Fri":
		return 5
	case "Sat":
		return 6
	default:
		return 0
	}
}

// AddEventStruct adds a remind.Event to the remind file and returns the line number
func (c *Client) AddEventStruct(event Event) (int, error) {
	if len(c.Files) == 0 {
		return 0, fmt.Errorf("no remind files configured")
	}

	// Use first file for new events
	file := c.Files[0]

	// Get current line count to know where we're adding the new entry
	existingContent, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to read remind file: %w", err)
	}
	lineNumber := strings.Count(string(existingContent), "\n") + 1

	// Format the remind line based on the event
	var remindLine string
	dateStr := event.Date.Format("Jan 2 2006")

	if event.Time != nil {
		timeStr := event.Time.Format("15:04")
		remindLine = fmt.Sprintf("REM %s AT %s MSG %s\n", dateStr, timeStr, event.Description)
	} else {
		remindLine = fmt.Sprintf("REM %s MSG %s\n", dateStr, event.Description)
	}

	// Append to file
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open remind file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(remindLine)
	if err != nil {
		return 0, fmt.Errorf("failed to write to remind file: %w", err)
	}

	return lineNumber, nil
}

// RemoveEvent removes an event from the remind file
// This is a simplified implementation that removes by matching description and date
func (c *Client) RemoveEvent(event Event) error {
	if len(c.Files) == 0 {
		return fmt.Errorf("no remind files configured")
	}

	// If we have a line number, use it directly
	if event.LineNumber > 0 {
		// If we have a filename, use it; otherwise use the first file
		file := event.Filename
		if file == "" && len(c.Files) > 0 {
			file = c.Files[0]
		}

		// Read the file
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read remind file: %w", err)
		}

		lines := strings.Split(string(content), "\n")

		// Check if line number is valid
		if event.LineNumber > len(lines) {
			return fmt.Errorf("line number %d exceeds file length", event.LineNumber)
		}

		// Remove the line at the specified line number (1-indexed)
		var newLines []string
		for i, line := range lines {
			if i != event.LineNumber-1 { // Line numbers are 1-indexed
				newLines = append(newLines, line)
			}
		}

		// Write the updated content back to file
		newContent := strings.Join(newLines, "\n")
		err = os.WriteFile(file, []byte(newContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write updated remind file: %w", err)
		}

		return nil
	}

	// Fallback to pattern matching if no line number
	// Use first file as default
	file := c.Files[0]

	// Read the file
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read remind file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	// Create patterns to match the event - be more flexible with date formats
	descPattern := regexp.QuoteMeta(event.Description)

	var linePattern *regexp.Regexp
	if event.Time != nil {
		timeStr := event.Time.Format("15:04")
		// Pattern for timed events with flexible date format
		// Match lines like: REM 20 AT 09:30 DURATION 1:00 MSG wat
		// or: REM Jan 20 2025 AT 09:30 MSG description
		linePattern = regexp.MustCompile(fmt.Sprintf(`^REM\s+.*AT\s+%s.*MSG\s+.*%s.*$`,
			regexp.QuoteMeta(timeStr), descPattern))
	} else {
		// Pattern for untimed events with flexible date format
		linePattern = regexp.MustCompile(fmt.Sprintf(`^REM\s+.*MSG\s+.*%s.*$`, descPattern))
	}

	// Filter out the matching line (remove first match only)
	removed := false
	for _, line := range lines {
		if !removed && linePattern.MatchString(line) {
			removed = true
			continue // Skip this line (remove it)
		}
		newLines = append(newLines, line)
	}

	if !removed {
		return fmt.Errorf("event not found in remind file")
	}

	// Write the updated content back to file
	newContent := strings.Join(newLines, "\n")
	err = os.WriteFile(file, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated remind file: %w", err)
	}

	return nil
}

// AddQuickEvent parses natural language event description and adds it to remind file
func (c *Client) AddQuickEvent(eventDesc string) (int, error) {
	if len(c.Files) == 0 {
		return 0, fmt.Errorf("no remind files configured")
	}

	// Parse the natural language description using the time parser
	parser := &TimeParser{Now: time.Now(), Location: time.Local}
	parsed, err := parser.Parse(eventDesc)
	if err != nil {
		return 0, fmt.Errorf("failed to parse event description: %w", err)
	}

	// Use first file for new events
	file := c.Files[0]

	// Get current line count to know where we are adding the new entry
	existingContent, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to read remind file: %w", err)
	}
	lineNumber := strings.Count(string(existingContent), "\n") + 1

	// Format the remind line based on parsing results
	var remindLine string
	dateStr := parsed.Date.Format("Jan 2 2006")
	description := strings.TrimSpace(parsed.Text)
	if description == "" {
		description = "New reminder"
	}

	if parsed.HasTime {
		timeStr := parsed.Time.Format("15:04")
		if parsed.Duration > 0 {
			// Calculate duration in hours and minutes
			totalMin := int(parsed.Duration.Minutes())
			hours := totalMin / 60
			minutes := totalMin % 60
			remindLine = fmt.Sprintf("REM %s AT %s DURATION %d:%.2d MSG %s\n",
				dateStr, timeStr, hours, minutes, description)
		} else {
			remindLine = fmt.Sprintf("REM %s AT %s MSG %s\n", dateStr, timeStr, description)
		}
	} else {
		remindLine = fmt.Sprintf("REM %s MSG %s\n", dateStr, description)
	}

	// Append to file
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open remind file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(remindLine)
	if err != nil {
		return 0, fmt.Errorf("failed to write to remind file: %w", err)
	}

	return lineNumber, nil
}
