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
