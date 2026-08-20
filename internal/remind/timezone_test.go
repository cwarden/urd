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
