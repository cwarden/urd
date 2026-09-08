package ui

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cwarden/urd/internal/config"
	"github.com/cwarden/urd/internal/remind"

	tea "charm.land/bubbletea/v2"
)

// fakeSource records which months are requested and answers from a fixed map.
type fakeSource struct {
	mu     sync.Mutex
	calls  []monthKey
	events map[monthKey][]remind.Event
	err    error
}

func (f *fakeSource) GetEvents(start, end time.Time) ([]remind.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := monthKeyFor(start)
	f.calls = append(f.calls, key)
	if f.err != nil {
		return nil, f.err
	}
	return f.events[key], nil
}

func (f *fakeSource) SetFiles([]string) {}

func (f *fakeSource) WatchFiles() (<-chan remind.FileChangeEvent, error) { return nil, nil }

func (f *fakeSource) StopWatching() error { return nil }

func (f *fakeSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeSource) calledMonths() map[monthKey]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := make(map[monthKey]int)
	for _, k := range f.calls {
		counts[k]++
	}
	return counts
}

func untimedEvent(id string, date time.Time) remind.Event {
	return remind.Event{ID: id, Date: date, Description: id}
}

func newCacheTestModel(source remind.ReminderSource, selected time.Time) *Model {
	return &Model{
		config:        &config.Config{AutoRefresh: true, RefreshRate: time.Hour},
		source:        source,
		selectedDate:  selected,
		timeIncrement: 30,
		mode:          ViewHourly,
		cacheTime:     time.Now(),
	}
}

// runCmd executes cmd synchronously and feeds every resulting message back
// into the model, expanding batches. Timer commands are not run.
func runCmd(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	switch msg := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range msg {
			runCmd(t, m, c)
		}
	default:
		m.Update(msg)
	}
}

func eventIDs(events []remind.Event) map[string]bool {
	ids := make(map[string]bool)
	for _, e := range events {
		ids[e.ID] = true
	}
	return ids
}

func TestEnsureEventsLoadedFetchesSelectedAndAdjacentMonthsOnce(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	src := &fakeSource{events: map[monthKey][]remind.Event{
		monthKeyFor(sep.AddDate(0, -1, 0)): {untimedEvent("aug", time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local))},
		monthKeyFor(sep):                   {untimedEvent("sep", sep)},
		monthKeyFor(sep.AddDate(0, 1, 0)):  {untimedEvent("oct", time.Date(2026, 10, 3, 0, 0, 0, 0, time.Local))},
	}}
	m := newCacheTestModel(src, sep)

	runCmd(t, m, m.ensureEventsLoaded())

	want := map[monthKey]int{
		{2026, time.August}:    1,
		{2026, time.September}: 1,
		{2026, time.October}:   1,
	}
	got := src.calledMonths()
	for k, n := range want {
		if got[k] != n {
			t.Errorf("month %v fetched %d times, want %d", k, got[k], n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("fetched months %v, want %v", got, want)
	}

	ids := eventIDs(m.events)
	for _, id := range []string{"aug", "sep", "oct"} {
		if !ids[id] {
			t.Errorf("m.events missing %q after load: %v", id, ids)
		}
	}

	// A second call finds everything cached and issues no command.
	if cmd := m.ensureEventsLoaded(); cmd != nil {
		t.Fatalf("expected no fetch when all visible months are cached")
	}
	if src.callCount() != 3 {
		t.Errorf("source called %d times, want 3", src.callCount())
	}
}

func TestMovingIntoPrefetchedMonthFetchesOnlyTheNewNeighbor(t *testing.T) {
	sep := time.Date(2026, time.September, 28, 0, 0, 0, 0, time.Local)
	src := &fakeSource{events: map[monthKey][]remind.Event{}}
	m := newCacheTestModel(src, sep)
	m.config.KeyBindings = map[string]string{"n": "next_month", "p": "previous_month"}
	runCmd(t, m, m.ensureEventsLoaded())

	_, cmd := m.handleHourlyKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.selectedDate.Month() != time.October {
		t.Fatalf("selectedDate = %v, want October", m.selectedDate)
	}
	runCmd(t, m, cmd)

	got := src.calledMonths()
	if got[monthKey{2026, time.November}] != 1 {
		t.Errorf("November fetched %d times, want 1", got[monthKey{2026, time.November}])
	}
	if got[monthKey{2026, time.October}] != 1 || got[monthKey{2026, time.September}] != 1 {
		t.Errorf("already cached months were fetched again: %v", got)
	}
	if src.callCount() != 4 {
		t.Errorf("source called %d times, want 4", src.callCount())
	}

	// Moving back re-uses the cache entirely.
	_, cmd = m.handleHourlyKeys(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd != nil {
		t.Errorf("expected no fetch when moving back into cached months")
	}
}

func TestDayNavigationInsideCachedMonthsDoesNotFetch(t *testing.T) {
	sep := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local)
	src := &fakeSource{events: map[monthKey][]remind.Event{}}
	m := newCacheTestModel(src, sep)
	m.config.KeyBindings = map[string]string{"l": "next_day", "L": "next_week"}
	runCmd(t, m, m.ensureEventsLoaded())
	before := src.callCount()

	for i := 0; i < 20; i++ {
		_, cmd := m.handleHourlyKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
		runCmd(t, m, cmd)
	}
	_, cmd := m.handleHourlyKeys(tea.KeyPressMsg{Code: 'L', Text: "L"})
	runCmd(t, m, cmd)

	if src.callCount() != before {
		t.Errorf("day and week navigation inside cached months called the source %d extra times", src.callCount()-before)
	}
}

func TestResultFromOlderGenerationIsDiscarded(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	key := monthKeyFor(sep)
	src := &fakeSource{events: map[monthKey][]remind.Event{key: {untimedEvent("old", sep)}}}
	m := newCacheTestModel(src, sep)

	stale := m.fetchMonthCmd(key)
	m.reloadEvents() // invalidates before the stale fetch completes

	m.applyEventLoaded(stale().(eventLoadedMsg))

	if m.isMonthCached(key) {
		t.Errorf("stale result was stored in the cache")
	}
	if eventIDs(m.events)["old"] {
		t.Errorf("stale result was applied to m.events")
	}
}

func TestReloadEventsFetchesEveryVisibleMonthAgain(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	key := monthKeyFor(sep)
	src := &fakeSource{events: map[monthKey][]remind.Event{key: {untimedEvent("v1", sep)}}}
	m := newCacheTestModel(src, sep)
	runCmd(t, m, m.ensureEventsLoaded())

	src.mu.Lock()
	src.events[key] = []remind.Event{untimedEvent("v2", sep)}
	src.mu.Unlock()

	runCmd(t, m, m.reloadEvents())

	if src.callCount() != 6 {
		t.Errorf("source called %d times, want 6", src.callCount())
	}
	ids := eventIDs(m.events)
	if !ids["v2"] || ids["v1"] {
		t.Errorf("events after reload = %v, want only v2", ids)
	}
}

func TestFetchErrorIsNotRetriedUntilReload(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	src := &fakeSource{err: errors.New("remind exploded")}
	m := newCacheTestModel(src, sep)

	runCmd(t, m, m.ensureEventsLoaded())
	if src.callCount() != 3 {
		t.Fatalf("source called %d times, want 3", src.callCount())
	}
	if cmd := m.ensureEventsLoaded(); cmd != nil {
		t.Errorf("failed months were fetched again without invalidation")
	}

	src.mu.Lock()
	src.err = nil
	src.mu.Unlock()
	runCmd(t, m, m.reloadEvents())
	if src.callCount() != 6 {
		t.Errorf("source called %d times after reload, want 6", src.callCount())
	}
	for _, key := range m.visibleMonths() {
		if !m.isMonthCached(key) {
			t.Errorf("month %v not cached after successful reload", key)
		}
	}
}

func TestSyntaxErrorIsKeptUntilASuccessfulFetch(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	src := &fakeSource{err: &remind.RemindSyntaxError{File: "x.rem", Line: 3, Message: "bad"}}
	m := newCacheTestModel(src, sep)

	runCmd(t, m, m.ensureEventsLoaded())
	if m.syntaxError == nil {
		t.Fatalf("syntax error was not recorded")
	}

	src.mu.Lock()
	src.err = nil
	src.mu.Unlock()
	runCmd(t, m, m.reloadEvents())
	if m.syntaxError != nil {
		t.Errorf("syntax error still set after successful reload: %v", m.syntaxError)
	}
}

func TestFileChangedMsgReloadsAndKeepsWatching(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	src := &fakeSource{events: map[monthKey][]remind.Event{}}
	m := newCacheTestModel(src, sep)
	runCmd(t, m, m.ensureEventsLoaded())

	ch := make(chan remind.FileChangeEvent)
	m.watchChan = ch
	_, cmd := m.Update(fileChangedMsg{path: "x.rem"})

	// Close the channel so the re-issued watch command returns nil instead
	// of blocking; the remaining commands are the month fetches.
	close(ch)
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch of fetches and a watch command, got %T", msg)
	}
	fetches := 0
	for _, c := range batch {
		inner := c()
		if inner == nil {
			continue
		}
		if b, ok := inner.(tea.BatchMsg); ok {
			for _, fc := range b {
				m.Update(fc())
				fetches++
			}
			continue
		}
		if _, ok := inner.(eventLoadedMsg); ok {
			m.Update(inner)
			fetches++
			continue
		}
		t.Fatalf("unexpected message %T", inner)
	}
	_ = fetches
	if src.callCount() != 6 {
		t.Errorf("source called %d times after file change, want 6", src.callCount())
	}
}

func TestTickWithFreshCacheDoesNotFetch(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	src := &fakeSource{events: map[monthKey][]remind.Event{}}
	m := newCacheTestModel(src, sep)
	runCmd(t, m, m.ensureEventsLoaded())
	before := src.callCount()

	m.Update(tickMsg{})

	if len(m.pendingFetch) != 0 || src.callCount() != before {
		t.Errorf("tick with an unchanged source started fetches: pending=%v calls=%d", m.pendingFetch, src.callCount()-before)
	}
}

func TestCacheIsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reminders.rem")
	if err := os.WriteFile(path, []byte("REM 1 Jan MSG x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	m := &Model{config: &config.Config{RemindFiles: []string{path}}}
	now := time.Now()

	m.cacheTime = now.Add(-time.Minute)
	if m.cacheIsStale(now) {
		t.Errorf("unchanged file and same day reported stale")
	}

	m.cacheTime = now.Add(-2 * time.Hour)
	if !m.cacheIsStale(now) {
		t.Errorf("file modified after cacheTime not reported stale")
	}

	m.cacheTime = now.Add(-time.Minute)
	if !m.cacheIsStale(now.AddDate(0, 0, 1)) {
		t.Errorf("day rollover not reported stale")
	}

	m.cacheTime = time.Time{}
	if !m.cacheIsStale(now) {
		t.Errorf("zero cacheTime not reported stale")
	}
}

func TestSourceFilesPrefersRemindClient(t *testing.T) {
	m := &Model{
		config:       &config.Config{RemindFiles: []string{"cfg.rem"}},
		remindClient: &remind.Client{Files: []string{"client.rem"}},
	}
	if got := m.sourceFiles(); len(got) != 1 || got[0] != "client.rem" {
		t.Errorf("sourceFiles = %v, want [client.rem]", got)
	}
	m.remindClient = nil
	if got := m.sourceFiles(); len(got) != 1 || got[0] != "cfg.rem" {
		t.Errorf("sourceFiles = %v, want [cfg.rem]", got)
	}
}

func TestModelWithoutSourceLeavesEventsAlone(t *testing.T) {
	sep := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.Local)
	m := &Model{selectedDate: sep, events: []remind.Event{untimedEvent("kept", sep)}}
	if cmd := m.ensureEventsLoaded(); cmd != nil {
		t.Errorf("expected no command without a source")
	}
	if !eventIDs(m.events)["kept"] {
		t.Errorf("events were cleared without a source")
	}
}

func TestMonthKeyRange(t *testing.T) {
	k := monthKey{2026, time.February}
	if got := k.start(); got != time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local) {
		t.Errorf("start = %v", got)
	}
	if got := k.end(); got.Month() != time.February || got.Day() != 28 || got.Hour() != 23 {
		t.Errorf("end = %v, want last nanosecond of February", got)
	}
	if got := k.add(11); got != (monthKey{2027, time.January}) {
		t.Errorf("add(11) = %v", got)
	}
	if got := k.add(-2); got != (monthKey{2025, time.December}) {
		t.Errorf("add(-2) = %v", got)
	}
}

func TestSelectedMonthIsFetchedBeforeItsNeighbors(t *testing.T) {
	source := &fakeSource{events: map[monthKey][]remind.Event{}}
	selected := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	m := newCacheTestModel(source, selected)
	runCmd(t, m, m.ensureEventsLoaded())
	if source.callCount() != 3 {
		t.Fatalf("fetched %d months, want 3", source.callCount())
	}
	source.mu.Lock()
	first := source.calls[0]
	source.mu.Unlock()
	if first != monthKeyFor(selected) {
		t.Errorf("first fetch was %v, want the selected month %v", first, monthKeyFor(selected))
	}
}

func TestFetchStartedBeforeAReloadSkipsTheQuery(t *testing.T) {
	source := &fakeSource{events: map[monthKey][]remind.Event{}}
	selected := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	m := newCacheTestModel(source, selected)
	stale := m.fetchMonthCmd(monthKeyFor(selected))
	m.reloadEvents()
	msg, ok := stale().(eventLoadedMsg)
	if !ok {
		t.Fatalf("stale fetch returned %T", msg)
	}
	if source.callCount() != 0 {
		t.Errorf("stale fetch queried the source %d times", source.callCount())
	}
	if msg.gen == m.cacheGen {
		t.Errorf("stale fetch reported the current generation %d", msg.gen)
	}
}
