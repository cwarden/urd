package remind

import (
	"path/filepath"
	"testing"
	"time"
)

func TestExpandTemplateTimeZone(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	tests := []struct {
		name     string
		timezone *time.Location
		template string
		want     string
	}{
		{
			name:     "%tz% expands to the current zone",
			timezone: chicago,
			template: `REM %monname% %mday% %year% AT %hour%:%min% TZ %tz% MSG`,
			want:     "REM Aug 19 2025 AT 16:30 TZ America/Chicago MSG",
		},
		{
			name:     "template without %tz% is unaffected",
			timezone: chicago,
			template: `REM %monname% %mday% %year% MSG`,
			want:     "REM Aug 19 2025 MSG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient()
			c.Timezone = tt.timezone
			if got := c.expandTemplate(tt.template, "Aug 19 2025", "16:30"); got != tt.want {
				t.Errorf("expandTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExpandTemplateDropsUnnamedTimeZone tests that a TZ clause is omitted
// rather than left empty when the local zone can't be named
func TestExpandTemplateDropsUnnamedTimeZone(t *testing.T) {
	t.Setenv("TZ", "")
	dir := t.TempDir()
	withZoneFiles(t, filepath.Join(dir, "missing"), filepath.Join(dir, "also-missing"))

	c := NewClient()
	c.Timezone = nil

	got := c.expandTemplate(`REM %monname% %mday% %year% AT %hour%:%min% TZ %tz% DURATION 1:00 MSG`, "Aug 19 2025", "16:30")
	want := "REM Aug 19 2025 AT 16:30 DURATION 1:00 MSG"
	if got != want {
		t.Errorf("expandTemplate() = %q, want %q", got, want)
	}
}
