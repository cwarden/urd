package remind

import (
	"testing"
	"time"
)

func TestExtractSubject(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "no markers passes through",
			body: "plain reminder text",
			want: "plain reminder text",
		},
		{
			name: "subject only",
			body: `%"subject%"`,
			want: "subject",
		},
		{
			name: "subject with leading time and trailing context",
			body: `6:30am %"the subject%" today at 6:30am`,
			want: "the subject",
		},
		{
			name: "subject with leading time prefix only",
			body: `3:15pm %"the subject%"`,
			want: "the subject",
		},
		{
			name: "unclosed marker passes through",
			body: `%"never closed`,
			want: `%"never closed`,
		},
		{
			name: "single marker without pair passes through",
			body: `prefix %" suffix`,
			want: `prefix %" suffix`,
		},
		{
			name: "empty body",
			body: "",
			want: "",
		},
		{
			name: "empty subject",
			body: `prefix %"%" suffix`,
			want: "",
		},
		{
			name: "only the first pair is extracted",
			body: `%"first%" middle %"second%"`,
			want: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSubject(tt.body); got != tt.want {
				t.Errorf("extractSubject(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestConvertJSONToEventsDescription(t *testing.T) {
	tz := time.UTC
	timed := func(mins int) *int { return &mins }

	tests := []struct {
		name    string
		entry   RemindEntry
		wantDsc string
	}{
		{
			name: "subject extracted from %\" markers",
			entry: RemindEntry{
				Date:   "2026-04-28",
				LineNo: 1,
				Time:   timed(12 * 60),
				Body:   `12:00pm %"timed subject%"`,
			},
			wantDsc: "timed subject",
		},
		{
			name: "subject extracted with trailing context",
			entry: RemindEntry{
				Date:   "2026-04-28",
				LineNo: 2,
				Time:   timed(6*60 + 30),
				Body:   `6:30am %"subject with context%" today at 6:30am`,
			},
			wantDsc: "subject with context",
		},
		{
			name: "no markers — leading time token still stripped",
			entry: RemindEntry{
				Date:   "2026-04-28",
				LineNo: 3,
				Time:   timed(15 * 60),
				Body:   "3:00pm-4:00pm body without markers",
			},
			wantDsc: "body without markers",
		},
		{
			name: "no markers, no leading time — body passes through",
			entry: RemindEntry{
				Date:   "2026-04-28",
				LineNo: 4,
				Body:   "untimed body",
			},
			wantDsc: "untimed body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := ConvertJSONToEvents([]RemindEntry{tt.entry}, tz)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			if got := events[0].Description; got != tt.wantDsc {
				t.Errorf("Description = %q, want %q", got, tt.wantDsc)
			}
		})
	}
}

func TestConvertJSONToEventsTimeZone(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	timed := func(mins int) *int { return &mins }

	tests := []struct {
		name       string
		entry      RemindEntry
		wantZone   string
		wantInZone string // "" means TimeInZone should be nil
	}{
		{
			name: "TZ clause records the zone and the time in it",
			entry: RemindEntry{
				Date:     "2026-09-16",
				LineNo:   1,
				Time:     timed(9*60 + 45),
				TimeInTZ: timed(7*60 + 45),
				TZ:       "America/Los_Angeles",
				Body:     `9:45-10:15am %"Coffee%"`,
			},
			wantZone:   "America/Los_Angeles",
			wantInZone: "2026-09-16 07:45",
		},
		{
			name: "TZ clause naming the local zone still records it",
			entry: RemindEntry{
				Date:     "2026-09-16",
				LineNo:   2,
				Time:     timed(9 * 60),
				TimeInTZ: timed(9 * 60),
				TZ:       "America/Chicago",
				Body:     `9:00am %"Local%"`,
			},
			wantZone:   "America/Chicago",
			wantInZone: "2026-09-16 09:00",
		},
		{
			name: "unknown zone falls back to time_in_tz",
			entry: RemindEntry{
				Date:     "2026-09-16",
				LineNo:   3,
				Time:     timed(9*60 + 45),
				TimeInTZ: timed(7*60 + 45),
				TZ:       "Mars/Olympus_Mons",
				Body:     `9:45am %"Rover check%"`,
			},
			wantZone:   "Mars/Olympus_Mons",
			wantInZone: "2026-09-16 07:45",
		},
		{
			name: "no TZ clause leaves the zone unset",
			entry: RemindEntry{
				Date:   "2026-09-16",
				LineNo: 4,
				Time:   timed(9 * 60),
				Body:   `9:00am %"Local%"`,
			},
		},
		{
			name: "untimed reminder with a TZ clause has no time in zone",
			entry: RemindEntry{
				Date:   "2026-09-16",
				LineNo: 5,
				TZ:     "America/Los_Angeles",
				Body:   `%"All day%"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := ConvertJSONToEvents([]RemindEntry{tt.entry}, chicago)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			event := events[0]
			if event.TimeZone != tt.wantZone {
				t.Errorf("TimeZone = %q, want %q", event.TimeZone, tt.wantZone)
			}
			if tt.wantInZone == "" {
				if event.TimeInZone != nil {
					t.Errorf("TimeInZone = %v, want nil", event.TimeInZone)
				}
				return
			}
			if event.TimeInZone == nil {
				t.Fatalf("TimeInZone = nil, want %s", tt.wantInZone)
			}
			if got := event.TimeInZone.Format("2006-01-02 15:04"); got != tt.wantInZone {
				t.Errorf("TimeInZone = %s, want %s", got, tt.wantInZone)
			}
		})
	}
}
