package remind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalZoneName(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	t.Run("named location is used directly", func(t *testing.T) {
		if got := LocalZoneName(chicago); got != "America/Chicago" {
			t.Errorf("LocalZoneName() = %q, want America/Chicago", got)
		}
	})

	t.Run("UTC is a usable zone name", func(t *testing.T) {
		if got := LocalZoneName(time.UTC); got != "UTC" {
			t.Errorf("LocalZoneName() = %q, want UTC", got)
		}
	})

	t.Run("TZ environment variable resolves an unnamed location", func(t *testing.T) {
		t.Setenv("TZ", ":America/Denver")
		if got := LocalZoneName(nil); got != "America/Denver" {
			t.Errorf("LocalZoneName() = %q, want America/Denver", got)
		}
	})

	t.Run("localtime symlink resolves an unnamed location", func(t *testing.T) {
		t.Setenv("TZ", "")
		dir := t.TempDir()
		link := filepath.Join(dir, "localtime")
		if err := os.Symlink("/usr/share/zoneinfo/Europe/Amsterdam", link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		withZoneFiles(t, link, filepath.Join(dir, "missing"))

		if got := LocalZoneName(nil); got != "Europe/Amsterdam" {
			t.Errorf("LocalZoneName() = %q, want Europe/Amsterdam", got)
		}
	})

	t.Run("timezone file resolves an unnamed location", func(t *testing.T) {
		t.Setenv("TZ", "")
		dir := t.TempDir()
		file := filepath.Join(dir, "timezone")
		if err := os.WriteFile(file, []byte("Asia/Tokyo\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		withZoneFiles(t, filepath.Join(dir, "missing"), file)

		if got := LocalZoneName(nil); got != "Asia/Tokyo" {
			t.Errorf("LocalZoneName() = %q, want Asia/Tokyo", got)
		}
	})

	t.Run("no system configuration yields no name", func(t *testing.T) {
		t.Setenv("TZ", "")
		dir := t.TempDir()
		withZoneFiles(t, filepath.Join(dir, "missing"), filepath.Join(dir, "also-missing"))

		if got := LocalZoneName(nil); got != "" {
			t.Errorf("LocalZoneName() = %q, want empty", got)
		}
	})
}

// withZoneFiles points the system timezone lookups at test fixtures for the
// duration of the test.
func withZoneFiles(t *testing.T, link, file string) {
	t.Helper()
	origLink, origFile := localZoneLink, localZoneFile
	localZoneLink, localZoneFile = link, file
	t.Cleanup(func() {
		localZoneLink, localZoneFile = origLink, origFile
	})
}

// useLocaltimeLink points RefreshLocalZone at a symlink in a temporary
// directory, with TZ unset and no file recorded yet, and restores time.Local
// afterwards. It returns a function that makes the link refer to a zone.
func useLocaltimeLink(t *testing.T) func(zone string) {
	t.Helper()
	const zoneinfo = "/usr/share/zoneinfo/"
	if _, err := os.Stat(zoneinfo + "Asia/Tokyo"); err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	link := filepath.Join(t.TempDir(), "localtime")
	oldLink, oldLocal := localZoneLink, time.Local
	localZoneLink = link
	localZoneInfo = nil
	t.Cleanup(func() {
		localZoneLink = oldLink
		localZoneInfo = nil
		time.Local = oldLocal
	})

	if tz, set := os.LookupEnv("TZ"); set {
		os.Unsetenv("TZ")
		t.Cleanup(func() { os.Setenv("TZ", tz) })
	}

	return func(zone string) {
		t.Helper()
		os.Remove(link)
		if err := os.Symlink(zoneinfo+zone, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}
}

func TestRefreshLocalZoneFollowsAReplacedLocaltime(t *testing.T) {
	setZone := useLocaltimeLink(t)

	setZone("UTC")
	if RefreshLocalZone() {
		t.Error("the first call reported a change")
	}
	if RefreshLocalZone() {
		t.Error("an unchanged /etc/localtime reported a change")
	}

	setZone("Asia/Tokyo")
	if !RefreshLocalZone() {
		t.Fatal("a replaced /etc/localtime reported no change")
	}
	// Asia/Tokyo is 9 hours ahead of UTC all year.
	if _, offset := time.Now().Zone(); offset != 9*60*60 {
		t.Errorf("time.Now() offset = %d seconds, want %d", offset, 9*60*60)
	}
	if got := LocalZoneName(time.Local); got != "Asia/Tokyo" {
		t.Errorf("LocalZoneName(time.Local) = %q, want Asia/Tokyo", got)
	}
	if RefreshLocalZone() {
		t.Error("the call after a change reported another change")
	}
}

func TestRefreshLocalZoneLeavesAZoneFromTZAlone(t *testing.T) {
	setZone := useLocaltimeLink(t)
	setZone("UTC")
	RefreshLocalZone()

	t.Setenv("TZ", "America/Denver")
	before := time.Local
	setZone("Asia/Tokyo")
	if RefreshLocalZone() {
		t.Error("a replaced /etc/localtime reported a change while TZ is set")
	}
	if time.Local != before {
		t.Error("time.Local was replaced while TZ is set")
	}
}

func TestClientWithoutTimezoneReadsDatesInTheCurrentLocalZone(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	oldLocal := time.Local
	t.Cleanup(func() { time.Local = oldLocal })

	c := NewClient()
	time.Local = tokyo
	if got := c.location(); got != tokyo {
		t.Errorf("location() = %v, want the zone time.Local was replaced with", got)
	}

	c.Timezone = time.UTC
	if got := c.location(); got != time.UTC {
		t.Errorf("location() = %v, want the configured Timezone", got)
	}
}
