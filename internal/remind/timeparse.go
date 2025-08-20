package remind

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeParser is a simplified version for remind client use
type TimeParser struct {
	Now      time.Time
	Location *time.Location
}

type ParsedEvent struct {
	Date     time.Time
	HasTime  bool
	Time     time.Time
	Duration time.Duration
	Text     string // Description text
}

func (p *TimeParser) Parse(input string) (*ParsedEvent, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	result := &ParsedEvent{}

	// Extract time first (can appear anywhere)
	hasTime, hour, minute, remaining := p.extractTime(input)

	// Extract date (can appear anywhere in remaining text)
	hasDate, date, description := p.ExtractDate(remaining)

	// Set the date
	if hasDate {
		result.Date = date
	} else {
		// Default to today if no date specified
		result.Date = time.Date(p.Now.Year(), p.Now.Month(), p.Now.Day(), 0, 0, 0, 0, p.Location)
	}

	// Set the time if found
	if hasTime {
		result.HasTime = true
		result.Time = time.Date(result.Date.Year(), result.Date.Month(), result.Date.Day(),
			hour, minute, 0, 0, p.Location)
	}

	// Clean up the description
	result.Text = strings.TrimSpace(description)
	if result.Text == "" {
		result.Text = "New reminder"
	}

	return result, nil
}

// ParseDateOnly parses input that is expected to be primarily a date
// Returns an error if no recognizable date pattern is found
func (p *TimeParser) ParseDateOnly(input string) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, fmt.Errorf("empty input")
	}

	// Extract time first (can appear anywhere) but ignore it for date-only parsing
	_, _, _, remaining := p.extractTime(input)

	// Extract date (can appear anywhere in remaining text)
	hasDate, date, _ := p.ExtractDate(remaining)

	if !hasDate {
		return time.Time{}, fmt.Errorf("no valid date found in input: %s", input)
	}

	return date, nil
}

// extractTime looks for time patterns anywhere in the input and returns the time and remaining text
func (p *TimeParser) extractTime(input string) (found bool, hour int, minute int, remaining string) {
	// Look for patterns like "at 2pm", "at 14:30", "at 2:30pm", "2pm", "14:30"
	// Try "at TIME" pattern first
	atTimeRe := regexp.MustCompile(`\bat\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?\b`)
	matches := atTimeRe.FindStringSubmatch(strings.ToLower(input))

	if matches == nil {
		// Try just TIME pattern without "at"
		timeRe := regexp.MustCompile(`\b(\d{1,2}):(\d{2})\s*(am|pm)?\b|\b(\d{1,2})\s*(am|pm)\b`)
		matches = timeRe.FindStringSubmatch(strings.ToLower(input))
		if matches != nil {
			// Adjust match indices for different regex groups
			if matches[1] != "" {
				// Format: HH:MM [am/pm]
				hour, _ = strconv.Atoi(matches[1])
				minute, _ = strconv.Atoi(matches[2])
				if matches[3] != "" {
					if matches[3] == "pm" && hour < 12 {
						hour += 12
					} else if matches[3] == "am" && hour == 12 {
						hour = 0
					}
				}
			} else if matches[4] != "" {
				// Format: H [am/pm]
				hour, _ = strconv.Atoi(matches[4])
				minute = 0
				if matches[5] == "pm" && hour < 12 {
					hour += 12
				} else if matches[5] == "am" && hour == 12 {
					hour = 0
				}
			}

			// Remove the matched time from input and clean up extra spaces
			remaining = timeRe.ReplaceAllString(input, " ")
			remaining = regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " ")
			remaining = strings.TrimSpace(remaining)
			return true, hour, minute, remaining
		}
	} else {
		// Parse the "at TIME" match
		hour, _ = strconv.Atoi(matches[1])
		minute = 0
		if matches[2] != "" {
			minute, _ = strconv.Atoi(matches[2])
		}

		// Handle AM/PM
		if matches[3] == "pm" && hour < 12 {
			hour += 12
		} else if matches[3] == "am" && hour == 12 {
			hour = 0
		}

		// If no AM/PM specified and hour < 8, assume PM for convenience
		if matches[3] == "" && hour < 8 && hour != 0 {
			hour += 12
		}

		// Remove the matched time from input and clean up extra spaces
		remaining = atTimeRe.ReplaceAllString(input, " ")
		remaining = regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " ")
		remaining = strings.TrimSpace(remaining)
		return true, hour, minute, remaining
	}

	return false, 0, 0, input
}

// ExtractDate looks for date patterns anywhere in the input and returns the date and remaining text
func (p *TimeParser) ExtractDate(input string) (found bool, date time.Time, remaining string) {
	today := time.Date(p.Now.Year(), p.Now.Month(), p.Now.Day(), 0, 0, 0, 0, p.Location)

	// Try each date pattern and find which one matches
	patterns := []struct {
		regex   *regexp.Regexp
		handler func([]string) time.Time
	}{
		{
			// "tomorrow"
			regex: regexp.MustCompile(`(?i)\btomorrow\b`),
			handler: func(m []string) time.Time {
				return today.AddDate(0, 0, 1)
			},
		},
		{
			// "today"
			regex: regexp.MustCompile(`(?i)\btoday\b`),
			handler: func(m []string) time.Time {
				return today
			},
		},
		{
			// "yesterday"
			regex: regexp.MustCompile(`(?i)\byesterday\b`),
			handler: func(m []string) time.Time {
				return today.AddDate(0, 0, -1)
			},
		},
		{
			// "next monday", "this friday", etc
			regex: regexp.MustCompile(`(?i)\b(next|this)\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`),
			handler: func(m []string) time.Time {
				isNext := strings.ToLower(m[1]) == "next"
				weekday := p.parseWeekday(m[2])
				return p.findNextWeekday(today, weekday, isNext)
			},
		},
		{
			// Just weekday names with optional "on" prefix (treat as "this" - next occurrence)
			regex: regexp.MustCompile(`(?i)\b(?:on\s+)?(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`),
			handler: func(m []string) time.Time {
				weekday := p.parseWeekday(m[1])
				return p.findNextWeekday(today, weekday, false)
			},
		},
		{
			// YYYY-MM-DD format
			regex: regexp.MustCompile(`\b(\d{4})-(\d{1,2})-(\d{1,2})\b`),
			handler: func(m []string) time.Time {
				year, _ := strconv.Atoi(m[1])
				month, _ := strconv.Atoi(m[2])
				day, _ := strconv.Atoi(m[3])
				return time.Date(year, time.Month(month), day, 0, 0, 0, 0, p.Location)
			},
		},
		{
			// MM/DD/YYYY format
			regex: regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4})\b`),
			handler: func(m []string) time.Time {
				month, _ := strconv.Atoi(m[1])
				day, _ := strconv.Atoi(m[2])
				year, _ := strconv.Atoi(m[3])
				return time.Date(year, time.Month(month), day, 0, 0, 0, 0, p.Location)
			},
		},
		{
			// MM/DD format (current year)
			regex: regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})\b`),
			handler: func(m []string) time.Time {
				month, _ := strconv.Atoi(m[1])
				day, _ := strconv.Atoi(m[2])
				return time.Date(p.Now.Year(), time.Month(month), day, 0, 0, 0, 0, p.Location)
			},
		},
	}

	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(input); matches != nil {
			date = pattern.handler(matches)
			// Remove the matched date from input and clean up extra spaces
			remaining = pattern.regex.ReplaceAllString(input, " ")
			remaining = regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " ")
			remaining = strings.TrimSpace(remaining)
			return true, date, remaining
		}
	}

	// No date found
	return false, time.Time{}, input
}

func (p *TimeParser) parseWeekday(weekdayStr string) time.Weekday {
	switch strings.ToLower(weekdayStr) {
	case "sun", "sunday":
		return time.Sunday
	case "mon", "monday":
		return time.Monday
	case "tue", "tues", "tuesday":
		return time.Tuesday
	case "wed", "wednesday":
		return time.Wednesday
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday
	case "fri", "friday":
		return time.Friday
	case "sat", "saturday":
		return time.Saturday
	default:
		return time.Monday
	}
}

func (p *TimeParser) findNextWeekday(from time.Time, targetWeekday time.Weekday, forceNext bool) time.Time {
	currentWeekday := from.Weekday()
	daysAhead := int(targetWeekday - currentWeekday)

	if forceNext {
		// "next monday" means the monday of next week (always add 7 days to get to next week)
		// Even if today is Monday and they say "next Monday", they mean a week from now
		if daysAhead < 0 {
			daysAhead += 7
		}
		// Always add 7 more days to get to NEXT week's occurrence
		daysAhead += 7
	} else {
		// "this monday" or just "monday" means the next occurrence within this week
		if daysAhead <= 0 {
			// If it's today or in the past, jump to next week
			daysAhead += 7
		}
	}

	return from.AddDate(0, 0, daysAhead)
}
