package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"

	tea "charm.land/bubbletea/v2"
)

func lineEvent(file string, line int, date time.Time, desc string) remind.Event {
	return remind.Event{
		ID:          remind.EventID(date, line),
		Date:        date,
		Description: desc,
		Filename:    file,
		LineNumber:  line,
	}
}

func descriptions(events []remind.Event) string {
	var out []string
	for _, e := range events {
		out = append(out, fmt.Sprintf("%s@%d", e.Description, e.LineNumber))
	}
	return strings.Join(out, " ")
}

func hasDescription(events []remind.Event, desc string) bool {
	for _, e := range events {
		if e.Description == desc {
			return true
		}
	}
	return false
}

// cachedModel returns a model whose three visible months are cached at
// generation 0 with events from file: "Old" on line 2 of the selected month
// and "Keep" on line 1 of every month.
func cachedModel(file string, sep time.Time) *Model {
	m := newCacheTestModel(&fakeSource{}, sep)
	m.eventCache = make(map[monthKey]cachedMonth)
	for _, key := range m.visibleMonths() {
		day := key.start().AddDate(0, 0, 9)
		events := []remind.Event{lineEvent(file, 1, day, "Keep")}
		if key == monthKeyFor(sep) {
			events = append(events, lineEvent(file, 2, sep, "Old"))
		}
		m.eventCache[key] = cachedMonth{events: events, gen: 0}
	}
	m.rebuildEvents()
	return m
}

func TestApplyLineParsedReplacesTheLineInMonthsTheReloadHasNotRefreshed(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := "/tmp/cal.rem"
	m := cachedModel(file, sep)
	m.cacheGen = 1 // a reload is in flight

	oct := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.Local)
	m.Update(lineParsedMsg{gen: 1, file: file, line: 2, events: []remind.Event{
		lineEvent(file, 2, sep, "New"),
		lineEvent(file, 2, oct, "New"),
	}})

	if hasDescription(m.events, "Old") {
		t.Errorf("the old events of line 2 are still shown: %s", descriptions(m.events))
	}
	if n := strings.Count(descriptions(m.events), "New@2"); n != 2 {
		t.Errorf("got %d New events, want 2: %s", n, descriptions(m.events))
	}
	if n := strings.Count(descriptions(m.events), "Keep@1"); n != 3 {
		t.Errorf("got %d Keep events, want 3: %s", n, descriptions(m.events))
	}
}

func TestApplyLineParsedLeavesMonthsTheReloadAlreadyRefreshed(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := "/tmp/cal.rem"
	m := cachedModel(file, sep)
	m.cacheGen = 1
	// The full reload finished the selected month first, with the file's
	// real contents for line 2.
	m.eventCache[monthKeyFor(sep)] = cachedMonth{events: []remind.Event{lineEvent(file, 2, sep, "Fresh")}, gen: 1}
	m.rebuildEvents()

	oct := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.Local)
	m.Update(lineParsedMsg{gen: 1, file: file, line: 2, events: []remind.Event{
		lineEvent(file, 2, sep, "New"),
		lineEvent(file, 2, oct, "New"),
	}})

	if got := descriptions(m.eventCache[monthKeyFor(sep)].events); got != "Fresh@2" {
		t.Errorf("the refreshed month changed: %s", got)
	}
	if got := descriptions(m.eventCache[monthKeyFor(oct)].events); got != "Keep@1 New@2" {
		t.Errorf("the stale month was not patched: %s", got)
	}
}

func TestApplyLineParsedIgnoresErrorsAndOlderGenerations(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := "/tmp/cal.rem"
	m := cachedModel(file, sep)
	m.cacheGen = 1
	before := descriptions(m.events)

	m.Update(lineParsedMsg{gen: 1, file: file, line: 2, err: errors.New("bad line")})
	if got := descriptions(m.events); got != before {
		t.Errorf("an error changed the events: %s", got)
	}
	m.Update(lineParsedMsg{gen: 0, file: file, line: 2, events: []remind.Event{lineEvent(file, 2, sep, "New")}})
	if got := descriptions(m.events); got != before {
		t.Errorf("a parse from the cache's own generation changed the events: %s", got)
	}
}

func TestRemoveLineEventsDropsTheLineAndShiftsLaterLines(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := "/tmp/cal.rem"
	m := cachedModel(file, sep)
	key := monthKeyFor(sep)
	c := m.eventCache[key]
	c.events = append(c.events,
		lineEvent(file, 3, sep, "Later"),
		lineEvent("/tmp/other.rem", 3, sep, "OtherFile"))
	m.eventCache[key] = c
	m.rebuildEvents()

	m.removeLineEvents(lineEvent(file, 2, sep, "Old"))

	got := descriptions(m.events)
	if strings.Contains(got, "Old") {
		t.Errorf("the cut line is still shown: %s", got)
	}
	if !strings.Contains(got, "Later@2") {
		t.Errorf("the event after the cut line did not move up: %s", got)
	}
	if !strings.Contains(got, "OtherFile@3") {
		t.Errorf("an event in another file moved: %s", got)
	}
	for _, e := range m.events {
		if e.Description == "Later" && e.ID != remind.EventID(sep, 2) {
			t.Errorf("shifted event has ID %s, want %s", e.ID, remind.EventID(sep, 2))
		}
	}
}

func TestCutEventRemovesTheLineFromTheDisplayAtOnce(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := filepath.Join(t.TempDir(), "cal.rem")
	if err := os.WriteFile(file, []byte("REM 9 Sep 2026 MSG Keep\nREM 15 Sep 2026 MSG Old\nREM 16 Sep 2026 MSG Later\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := cachedModel(file, sep)
	client := remind.NewClient()
	client.SetFiles([]string{file})
	client.Runner = func([]string, string) ([]byte, []byte, int, error) {
		t.Error("remind ran before the cut was shown")
		return nil, nil, 0, nil
	}
	m.remindClient = client

	cmd := m.cutEvent(lineEvent(file, 2, sep, "Old"))

	if cmd == nil {
		t.Error("cut returned no reload command")
	}
	if hasDescription(m.events, "Old") {
		t.Errorf("the cut event is still shown: %s", descriptions(m.events))
	}
	if m.clipboardEvent == nil || m.clipboardEvent.Description != "Old" || !m.clipboardCut {
		t.Errorf("clipboard %+v cut=%v", m.clipboardEvent, m.clipboardCut)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "Old") {
		t.Errorf("the line is still in the file:\n%s", content)
	}
}

// editTestClient returns a client whose runner answers a run of one line
// on standard input with "New" on line 2 and a run over the file with
// "Full" on line 2, recording the order of the runs.
func editTestClient(file string, sep time.Time) (*remind.Client, func() []string) {
	var mu sync.Mutex
	var runs []string
	c := remind.NewClient()
	c.SetFiles([]string{file})
	c.Runner = func(args []string, stdin string) ([]byte, []byte, int, error) {
		mu.Lock()
		defer mu.Unlock()
		kind, body, name := "full", "Full", file
		if stdin != "" {
			kind, body, name = "line", "New", "-stdin-"
		}
		runs = append(runs, kind)
		month, _ := time.ParseInLocation("Jan 2 2006", strings.Join(args[len(args)-3:], " "), time.Local)
		date := time.Date(month.Year(), month.Month(), sep.Day(), 0, 0, 0, 0, time.Local)
		out := fmt.Sprintf(`[{"monthname":"%s","year":%d,"entries":[{"date":%q,"filename":%q,"lineno":2,"priority":5000,"body":%q}]}]`,
			month.Month(), month.Year(), date.Format("2006-01-02"), name, body)
		return []byte(out), nil, 0, nil
	}
	return c, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), runs...)
	}
}

func TestReloadAfterEditShowsTheEditedLineBeforeTheFullReload(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	file := filepath.Join(t.TempDir(), "cal.rem")
	if err := os.WriteFile(file, []byte("REM 9 Sep 2026 MSG Keep\nREM 15 Sep 2026 MSG New\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := cachedModel(file, sep)
	client, runs := editTestClient(file, sep)
	m.remindClient = client
	m.source = client
	m.config = &config.Config{RemindFiles: []string{file}}

	_, cmd := m.Update(editorFinishedMsg{file: file, line: 2})
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("got %T with %d commands, want a batch of the line parse and the reload", cmd(), len(batch))
	}

	// The line parse finishes first and goes on screen while the whole
	// file is still being read.
	m.Update(batch[0]())
	if got := descriptions(m.events); !strings.Contains(got, "New@2") || strings.Contains(got, "Old") {
		t.Errorf("after the line parse: %s", got)
	}
	if got := runs(); len(got) != 1 || got[0] != "line" {
		t.Errorf("runs before the reload: %v", got)
	}

	// The full reload then replaces every month.
	runCmd(t, m, batch[1])
	if got := descriptions(m.events); !strings.Contains(got, "Full@2") || strings.Contains(got, "New") {
		t.Errorf("after the full reload: %s", got)
	}
	if got := runs(); got[0] != "line" || len(got) != 4 {
		t.Errorf("runs: %v, want the line first and then three months", got)
	}
}

func TestReloadAfterEditWithoutAFileOnlyReloads(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	m := cachedModel("/tmp/cal.rem", sep)
	client, runs := editTestClient("/tmp/cal.rem", sep)
	m.remindClient = client
	m.source = client

	_, cmd := m.Update(editorFinishedMsg{})
	runCmd(t, m, cmd)
	for _, r := range runs() {
		if r == "line" {
			t.Error("a line was parsed although no file was opened")
		}
	}
}
