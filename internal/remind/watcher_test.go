package remind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestWatcher returns a watcher on path and a channel receiving the paths
// it reports.
func newTestWatcher(t *testing.T, path string) (*FileWatcher, <-chan string) {
	t.Helper()
	changes := make(chan string, 10)
	fw, err := NewFileWatcher(func(p string) { changes <- p })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fw.Close() })
	if err := fw.AddFile(path); err != nil {
		t.Fatal(err)
	}
	return fw, changes
}

func expectChange(t *testing.T, changes <-chan string, want string) {
	t.Helper()
	select {
	case got := <-changes:
		if got != want {
			t.Errorf("reported %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no change reported for %s", want)
	}
}

func expectNoChange(t *testing.T, changes <-chan string) {
	t.Helper()
	select {
	case got := <-changes:
		t.Errorf("unexpected change reported for %q", got)
	case <-time.After(300 * time.Millisecond):
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// replaceFile writes content to a temporary file and renames it over path,
// the way sed -i and many editors save.
func replaceFile(t *testing.T, path, content string) {
	t.Helper()
	tmp := path + ".tmp"
	writeFile(t, tmp, content)
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func TestFileWatcherReportsInPlaceWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminders.rem")
	writeFile(t, path, "REM 1 Jan MSG a\n")
	_, changes := newTestWatcher(t, path)

	writeFile(t, path, "REM 1 Jan MSG b\n")
	expectChange(t, changes, path)
}

func TestFileWatcherKeepsReportingAfterRenameReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminders.rem")
	writeFile(t, path, "REM 1 Jan MSG a\n")
	_, changes := newTestWatcher(t, path)

	replaceFile(t, path, "REM 1 Jan MSG b\n")
	expectChange(t, changes, path)

	writeFile(t, path, "REM 1 Jan MSG c\n")
	expectChange(t, changes, path)

	replaceFile(t, path, "REM 1 Jan MSG d\n")
	expectChange(t, changes, path)
}

func TestFileWatcherIgnoresOtherFilesInTheDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reminders.rem")
	writeFile(t, path, "REM 1 Jan MSG a\n")
	_, changes := newTestWatcher(t, path)

	writeFile(t, filepath.Join(dir, ".reminders.rem.swp"), "swap")
	expectNoChange(t, changes)
}

func TestFileWatcherReportsChangesToASymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "reminders.rem")
	writeFile(t, target, "REM 1 Jan MSG a\n")
	link := filepath.Join(dir, "link.rem")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, changes := newTestWatcher(t, link)

	replaceFile(t, target, "REM 1 Jan MSG b\n")
	expectChange(t, changes, link)
}

func TestFileWatcherStopsReportingRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reminders.rem")
	other := filepath.Join(dir, "other.rem")
	writeFile(t, path, "REM 1 Jan MSG a\n")
	writeFile(t, other, "REM 1 Jan MSG a\n")
	fw, changes := newTestWatcher(t, path)
	if err := fw.AddFile(other); err != nil {
		t.Fatal(err)
	}

	if err := fw.RemoveFile(path); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "REM 1 Jan MSG b\n")
	expectNoChange(t, changes)

	writeFile(t, other, "REM 1 Jan MSG b\n")
	expectChange(t, changes, other)
}
