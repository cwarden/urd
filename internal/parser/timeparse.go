package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedTime struct {
	Date     time.Time
	HasTime  bool
	Time     time.Time
	Duration time.Duration
	Text     string // Remaining text after parsing time
}

type TimeParser struct {
	now      time.Time
	location *time.Location
}

func NewTimeParser() *TimeParser {
	return &TimeParser{
		now:      time.Now(),
		location: time.Local,
	}
}

func (p *TimeParser) SetNow(now time.Time) {
	p.now = now
}

func (p *TimeParser) Parse(input string) (*ParsedTime, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	result := &ParsedTime{}

	// Try various parsing strategies
	remaining := input

	// Parse relative dates first
	if date, text, ok := p.parseRelativeDate(remaining); ok {
		result.Date = date
		remaining = text
	} else if date, text, ok := p.parseAbsoluteDate(remaining); ok {
		result.Date = date
		remaining = text
	} else {
		// Default to today if no date specified
		result.Date = p.today()
	}

	// Parse time
	if t, duration, text, ok := p.parseTime(remaining); ok {
		result.HasTime = true
		result.Time = t
		result.Duration = duration
		remaining = text
	}

	result.Text = strings.TrimSpace(remaining)
	return result, nil
}

func (p *TimeParser) parseRelativeDate(input string) (time.Time, string, bool) {
	lower := strings.ToLower(input)

	// Today
	if strings.HasPrefix(lower, "today") {
		return p.today(), strings.TrimSpace(input[5:]), true
	}

	// Tomorrow
	if strings.HasPrefix(lower, "tomorrow") || strings.HasPrefix(lower, "tmrw") {
		prefixLen := 8
		if strings.HasPrefix(lower, "tmrw") {
			prefixLen = 4
		}
		return p.today().AddDate(0, 0, 1), strings.TrimSpace(input[prefixLen:]), true
	}

	// Yesterday
	if strings.HasPrefix(lower, "yesterday") {
		return p.today().AddDate(0, 0, -1), strings.TrimSpace(input[9:]), true
	}

	// Next/this weekday
	weekdayRe := regexp.MustCompile(`^(next|this)\s+(mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday|sun|sunday)\b`)
	if matches := weekdayRe.FindStringSubmatch(lower); matches != nil {
		isNext := matches[1] == "next"
		weekday := p.parseWeekday(matches[2])
		date := p.findNextWeekday(weekday, isNext)
		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	// In N days/weeks/months
	inRe := regexp.MustCompile(`^in\s+(\d+)\s+(day|days|week|weeks|month|months)\b`)
	if matches := inRe.FindStringSubmatch(lower); matches != nil {
		n, _ := strconv.Atoi(matches[1])
		unit := matches[2]
		date := p.today()

		switch {
		case strings.HasPrefix(unit, "day"):
			date = date.AddDate(0, 0, n)
		case strings.HasPrefix(unit, "week"):
			date = date.AddDate(0, 0, n*7)
		case strings.HasPrefix(unit, "month"):
			date = date.AddDate(0, n, 0)
		}

		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	// N days/weeks from now
	fromNowRe := regexp.MustCompile(`^(\d+)\s+(day|days|week|weeks|month|months)\s+from\s+(now|today)`)
	if matches := fromNowRe.FindStringSubmatch(lower); matches != nil {
		n, _ := strconv.Atoi(matches[1])
		unit := matches[2]
		date := p.today()

		switch {
		case strings.HasPrefix(unit, "day"):
			date = date.AddDate(0, 0, n)
		case strings.HasPrefix(unit, "week"):
			date = date.AddDate(0, 0, n*7)
		case strings.HasPrefix(unit, "month"):
			date = date.AddDate(0, n, 0)
		}

		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	return time.Time{}, input, false
}

func (p *TimeParser) parseAbsoluteDate(input string) (time.Time, string, bool) {
	// MM/DD/YYYY or MM-DD-YYYY
	dateRe := regexp.MustCompile(`^(\d{1,2})[/-](\d{1,2})[/-](\d{4})`)
	if matches := dateRe.FindStringSubmatch(input); matches != nil {
		month, _ := strconv.Atoi(matches[1])
		day, _ := strconv.Atoi(matches[2])
		year, _ := strconv.Atoi(matches[3])

		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, p.location)
		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	// MM/DD or MM-DD (assume current year)
	shortDateRe := regexp.MustCompile(`^(\d{1,2})[/-](\d{1,2})`)
	if matches := shortDateRe.FindStringSubmatch(input); matches != nil {
		month, _ := strconv.Atoi(matches[1])
		day, _ := strconv.Atoi(matches[2])
		year := p.now.Year()

		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, p.location)
		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	// Month DD, YYYY or Month DD
	monthNameRe := regexp.MustCompile(`^(jan|january|feb|february|mar|march|apr|april|may|jun|june|jul|july|aug|august|sep|september|oct|october|nov|november|dec|december)\s+(\d{1,2})(?:,?\s+(\d{4}))?`)
	if matches := monthNameRe.FindStringSubmatch(strings.ToLower(input)); matches != nil {
		month := p.parseMonth(matches[1])
		day, _ := strconv.Atoi(matches[2])
		year := p.now.Year()
		if matches[3] != "" {
			year, _ = strconv.Atoi(matches[3])
		}

		date := time.Date(year, month, day, 0, 0, 0, 0, p.location)
		remaining := input[len(matches[0]):]
		return date, strings.TrimSpace(remaining), true
	}

	return time.Time{}, input, false
}

func (p *TimeParser) parseTime(input string) (time.Time, time.Duration, string, bool) {
	lower := strings.ToLower(input)

	// Handle "at" prefix
	if strings.HasPrefix(lower, "at ") {
		lower = lower[3:]
		input = input[3:]
	}

	// Time range (e.g., "2pm-4pm" or "14:00-16:00")
	rangeRe := regexp.MustCompile(`^(\d{1,2}):?(\d{2})?\s*(am|pm)?\s*-\s*(\d{1,2}):?(\d{2})?\s*(am|pm)?`)
	if matches := rangeRe.FindStringSubmatch(lower); matches != nil {
		startHour, _ := strconv.Atoi(matches[1])
		startMin := 0
		if matches[2] != "" {
			startMin, _ = strconv.Atoi(matches[2])
		}
		if matches[3] == "pm" && startHour < 12 {
			startHour += 12
		}

		endHour, _ := strconv.Atoi(matches[4])
		endMin := 0
		if matches[5] != "" {
			endMin, _ = strconv.Atoi(matches[5])
		}
		if matches[6] == "pm" && endHour < 12 {
			endHour += 12
		}

		startTime := time.Date(p.now.Year(), p.now.Month(), p.now.Day(), startHour, startMin, 0, 0, p.location)
		endTime := time.Date(p.now.Year(), p.now.Month(), p.now.Day(), endHour, endMin, 0, 0, p.location)
		duration := endTime.Sub(startTime)

		remaining := input[len(matches[0]):]
		return startTime, duration, strings.TrimSpace(remaining), true
	}

	// Single time (e.g., "2pm", "14:00", "2:30pm")
	timeRe := regexp.MustCompile(`^(\d{1,2}):?(\d{2})?\s*(am|pm)?`)
	if matches := timeRe.FindStringSubmatch(lower); matches != nil {
		hour, _ := strconv.Atoi(matches[1])
		min := 0
		if matches[2] != "" {
			min, _ = strconv.Atoi(matches[2])
		}

		// Handle AM/PM
		if matches[3] == "pm" && hour < 12 {
			hour += 12
		} else if matches[3] == "am" && hour == 12 {
			hour = 0
		}

		t := time.Date(p.now.Year(), p.now.Month(), p.now.Day(), hour, min, 0, 0, p.location)
		remaining := input[len(matches[0]):]
		return t, 0, strings.TrimSpace(remaining), true
	}

	// Named times
	namedTimes := map[string]int{
		"noon":      12,
		"midnight":  0,
		"morning":   9,
		"afternoon": 14,
		"evening":   18,
		"night":     21,
	}

	for name, hour := range namedTimes {
		if strings.HasPrefix(lower, name) {
			t := time.Date(p.now.Year(), p.now.Month(), p.now.Day(), hour, 0, 0, 0, p.location)
			remaining := input[len(name):]
			return t, 0, strings.TrimSpace(remaining), true
		}
	}

	return time.Time{}, 0, input, false
}

func (p *TimeParser) parseWeekday(s string) time.Weekday {
	switch strings.ToLower(s) {
	case "sun", "sunday":
		return time.Sunday
	case "mon", "monday":
		return time.Monday
	case "tue", "tuesday":
		return time.Tuesday
	case "wed", "wednesday":
		return time.Wednesday
	case "thu", "thursday":
		return time.Thursday
	case "fri", "friday":
		return time.Friday
	case "sat", "saturday":
		return time.Saturday
	default:
		return time.Sunday
	}
}

func (p *TimeParser) parseMonth(s string) time.Month {
	switch strings.ToLower(s) {
	case "jan", "january":
		return time.January
	case "feb", "february":
		return time.February
	case "mar", "march":
		return time.March
	case "apr", "april":
		return time.April
	case "may":
		return time.May
	case "jun", "june":
		return time.June
	case "jul", "july":
		return time.July
	case "aug", "august":
		return time.August
	case "sep", "september":
		return time.September
	case "oct", "october":
		return time.October
	case "nov", "november":
		return time.November
	case "dec", "december":
		return time.December
	default:
		return time.January
	}
}

func (p *TimeParser) findNextWeekday(target time.Weekday, skipThisWeek bool) time.Time {
	date := p.today()
	daysUntilTarget := int(target - date.Weekday())

	if daysUntilTarget <= 0 || skipThisWeek {
		daysUntilTarget += 7
	}

	return date.AddDate(0, 0, daysUntilTarget)
}

func (p *TimeParser) today() time.Time {
	y, m, d := p.now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, p.location)
}
