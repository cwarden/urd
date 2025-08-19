package remind

import (
	"encoding/json"
	"fmt"
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
		// Parse date in local timezone
		date, err := time.ParseInLocation("2006-01-02", entry.Date, timezone)
		if err != nil {
			continue
		}

		event := Event{
			ID:          fmt.Sprintf("evt-%s-%d", entry.Date, entry.LineNo),
			Date:        date,
			Description: entry.Body,
			Filename:    entry.Filename,
			LineNumber:  entry.LineNo,
			Tags:        entry.Tags,
		}

		// Check if it's a timed event
		if entry.Time != nil {
			hours := *entry.Time / 60
			minutes := *entry.Time % 60
			eventTime := time.Date(date.Year(), date.Month(), date.Day(),
				hours, minutes, 0, 0, timezone)
			event.Time = &eventTime
			event.Type = EventReminder

			if entry.Duration != nil {
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

		events = append(events, event)
	}

	return events
}
