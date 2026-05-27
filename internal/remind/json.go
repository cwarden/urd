package remind

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RemindJSON represents the JSON output from remind -pppq
type RemindJSON struct {
	MonthName   string        `json:"monthname"`
	Year        int           `json:"year"`
	DaysInMonth int           `json:"daysinmonth"`
	FirstWkDay  int           `json:"firstwkday"`
	MondayFirst int           `json:"mondayfirst"`
	DayNames    []string      `json:"daynames"`
	Entries     []RemindEntry `json:"entries"`
}

// RemindEntry represents a single reminder entry in the JSON
type RemindEntry struct {
	Date          string   `json:"date"`
	Filename      string   `json:"filename"`
	LineNo        int      `json:"lineno"`
	Duration      *int     `json:"duration,omitempty"`
	Time          *int     `json:"time,omitempty"`
	TDelta        *int     `json:"tdelta,omitempty"`
	EventDuration *int     `json:"eventduration,omitempty"`
	EventStart    string   `json:"eventstart,omitempty"`
	Priority      int      `json:"priority"`
	RawBody       string   `json:"rawbody"`
	Body          string   `json:"body"`
	Tags          []string `json:"tags,omitempty"`
	Skip          string   `json:"skip,omitempty"`
	Until         string   `json:"until,omitempty"`
	From          string   `json:"from,omitempty"`
	PassThru      string   `json:"passthru,omitempty"`
}

// extractSubject returns the text between the first pair of %" markers in
// body. Remind uses %"..."%" to delimit a reminder's subject from surrounding
// context; remind's JSON `body` field preserves these markers verbatim, so we
// strip them here for display.
func extractSubject(body string) string {
	start := strings.Index(body, `%"`)
	if start < 0 {
		return body
	}
	rest := body[start+2:]
	end := strings.Index(rest, `%"`)
	if end < 0 {
		return body
	}
	return rest[:end]
}

// ParseRemindJSON parses the JSON output from remind
func ParseRemindJSON(jsonData []byte) ([]RemindJSON, error) {
	var months []RemindJSON
	err := json.Unmarshal(jsonData, &months)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remind JSON: %w", err)
	}
	return months, nil
}

// ConvertJSONToEvents converts RemindJSON entries to Event structs
func ConvertJSONToEvents(entries []RemindEntry, timezone *time.Location) []Event {
	var events []Event

	for _, entry := range entries {
		// Skip SPECIAL SHADE entries - these are for calendar display, not events
		if entry.PassThru == "SHADE" {
			continue
		}

		// Parse date in local timezone
		date, err := time.ParseInLocation("2006-01-02", entry.Date, timezone)
		if err != nil {
			continue
		}

		// Check if this is part of a multi-day event
		var isMultiDayStart bool
		var isMultiDayContinuation bool
		if entry.EventStart != "" && entry.EventDuration != nil {
			if startTime, err := time.ParseInLocation("2006-01-02T15:04", entry.EventStart, timezone); err == nil {
				startDate := startTime.Format("2006-01-02")
				if startDate == entry.Date {
					// This is the starting day of a potentially multi-day event
					isMultiDayStart = true
				} else {
					// This is a continuation day
					isMultiDayContinuation = true
				}
			}
		}

		// Use the current date for the event
		eventDate := date

		// Extract description from Body field, removing time range if present
		description := extractSubject(entry.Body)
		if description == entry.Body && strings.Contains(description, " ") {
			// For multi-day events, Body contains time info like "3:00pm-1:00am+1 Description"
			// Extract just the description part
			parts := strings.Fields(description)
			if len(parts) > 1 && strings.Contains(parts[0], ":") {
				description = strings.Join(parts[1:], " ")
			}
		}

		event := Event{
			ID:          fmt.Sprintf("evt-%s-%d", entry.Date, entry.LineNo),
			Date:        eventDate,
			Description: description,
			Filename:    entry.Filename,
			LineNumber:  entry.LineNo,
			Tags:        entry.Tags,
		}

		// Check if it's a timed event
		if entry.Time != nil {
			// Use the time from this entry
			hours := *entry.Time / 60
			minutes := *entry.Time % 60
			eventTime := time.Date(date.Year(), date.Month(), date.Day(),
				hours, minutes, 0, 0, timezone)
			event.Time = &eventTime
			event.Type = EventReminder

			// For multi-day events, use EventDuration (total duration)
			// For single-day events, use Duration
			if isMultiDayStart && entry.EventDuration != nil {
				duration := time.Duration(*entry.EventDuration) * time.Minute
				event.Duration = &duration
			} else if entry.Duration != nil {
				duration := time.Duration(*entry.Duration) * time.Minute
				event.Duration = &duration
			}
		} else {
			event.Type = EventNote
		}

		// Set priority based on priority value
		// Default remind priority is 5000, treat that as normal
		if entry.Priority > 5000 {
			// Higher values = higher priority
			if entry.Priority >= 7000 {
				event.Priority = PriorityHigh
			} else if entry.Priority >= 6000 {
				event.Priority = PriorityMedium
			} else {
				event.Priority = PriorityLow
			}
		} else {
			event.Priority = PriorityNone
		}

		// For multi-day events, handle the start vs continuation appropriately
		if isMultiDayStart {
			// This is the start of a multi-day event - use full duration from EventDuration
			// The event will naturally extend into the next day due to its duration
			events = append(events, event)
		} else if isMultiDayContinuation {
			// This is a continuation - skip it because the start event already covers it
			// The start event's duration extends into this day
			continue
		} else {
			// Regular single-day event
			events = append(events, event)
		}
	}

	return events
}
