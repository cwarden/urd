package ui

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cwarden/urd/internal/remind"

	tea "charm.land/bubbletea/v2"
)

// monthKey identifies one calendar month in the event cache.
type monthKey struct {
	year  int
	month time.Month
}

func monthKeyFor(t time.Time) monthKey {
	return monthKey{year: t.Year(), month: t.Month()}
}

func (k monthKey) start() time.Time {
	return time.Date(k.year, k.month, 1, 0, 0, 0, 0, time.Local)
}

func (k monthKey) end() time.Time {
	return k.start().AddDate(0, 1, 0).Add(-time.Nanosecond)
}

func (k monthKey) add(months int) monthKey {
	return monthKeyFor(k.start().AddDate(0, months, 0))
}

func (k monthKey) String() string {
	return fmt.Sprintf("%04d-%02d", k.year, int(k.month))
}

// cachedMonth holds the result of one fetch. A month whose fetch failed is
// cached with err set so the same failure is not retried until the cache is
// invalidated.
type cachedMonth struct {
	events []remind.Event
	err    error
	gen    int
}

// eventLoadedMsg carries the result of a month fetch back to Update.
type eventLoadedMsg struct {
	key    monthKey
	gen    int
	events []remind.Event
	err    error
}

// fileChangedMsg is sent when a watched source file changes.
type fileChangedMsg struct {
	path string
}

// visibleMonths returns the months whose events the schedule view draws: the
// selected month and the months on either side of it. Fetching the neighbors
// means moving into an adjacent month never waits on remind.
func (m *Model) visibleMonths() []monthKey {
	cur := monthKeyFor(m.selectedDate)
	return []monthKey{cur.add(-1), cur, cur.add(1)}
}

// ensureEventsLoaded rebuilds m.events from the cache and returns a command
// that fetches every visible month that is missing or stale. Months already
// cached for the current generation are not fetched again.
func (m *Model) ensureEventsLoaded() tea.Cmd {
	if m.source == nil {
		return nil
	}
	m.rebuildEvents()
	var cmds []tea.Cmd
	for _, key := range m.visibleMonths() {
		if cmd := m.fetchMonthCmd(key); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// reloadEvents marks every cached month stale and fetches the visible months
// again. Results from fetches started before this call are discarded when
// they arrive.
func (m *Model) reloadEvents() tea.Cmd {
	m.cacheGen++
	m.cacheTime = time.Now()
	return m.ensureEventsLoaded()
}

// loadMonthNow fetches one month synchronously. It is used once at startup so
// the first frame already shows events.
func (m *Model) loadMonthNow(key monthKey) {
	if m.source == nil {
		return
	}
	if m.cacheTime.IsZero() {
		m.cacheTime = time.Now()
	}
	m.applyEventLoaded(fetchMonth(m.source, key, m.cacheGen))
}

// fetchMonthCmd returns a command that fetches key, or nil when the month is
// already cached or a fetch for it is in flight.
func (m *Model) fetchMonthCmd(key monthKey) tea.Cmd {
	if m.source == nil {
		return nil
	}
	if c, ok := m.eventCache[key]; ok && c.gen == m.cacheGen {
		return nil
	}
	if g, ok := m.pendingFetch[key]; ok && g == m.cacheGen {
		return nil
	}
	if m.pendingFetch == nil {
		m.pendingFetch = make(map[monthKey]int)
	}
	m.pendingFetch[key] = m.cacheGen
	source, gen := m.source, m.cacheGen
	return func() tea.Msg {
		return fetchMonth(source, key, gen)
	}
}

// fetchMonth runs the source query for one month. It runs in a command
// goroutine, so it must not touch the model.
func fetchMonth(source remind.ReminderSource, key monthKey, gen int) eventLoadedMsg {
	events, err := source.GetEvents(key.start(), key.end())
	return eventLoadedMsg{key: key, gen: gen, events: events, err: err}
}

// applyEventLoaded stores a fetch result in the cache and refreshes m.events.
// Results from a previous cache generation are dropped.
func (m *Model) applyEventLoaded(msg eventLoadedMsg) {
	if msg.gen != m.cacheGen {
		return
	}
	delete(m.pendingFetch, msg.key)
	if m.eventCache == nil {
		m.eventCache = make(map[monthKey]cachedMonth)
	}
	m.eventCache[msg.key] = cachedMonth{events: msg.events, err: msg.err, gen: msg.gen}

	if msg.err != nil {
		var syntaxErr *remind.RemindSyntaxError
		if errors.As(msg.err, &syntaxErr) {
			m.syntaxError = msg.err
		} else {
			m.showMessage(fmt.Sprintf("Error loading events: %v", msg.err))
		}
		return
	}
	m.syntaxError = nil
	m.rebuildEvents()
}

// rebuildEvents sets m.events to the cached events of the visible months.
func (m *Model) rebuildEvents() {
	var events []remind.Event
	for _, key := range m.visibleMonths() {
		if c, ok := m.eventCache[key]; ok {
			events = append(events, c.events...)
		}
	}
	m.events = events
}

// isMonthCached reports whether key holds events for the current generation.
func (m *Model) isMonthCached(key monthKey) bool {
	c, ok := m.eventCache[key]
	return ok && c.gen == m.cacheGen && c.err == nil
}

// waitForFileChange returns a command that blocks until the source reports a
// file change. Update re-issues it after each message so the watch continues.
func waitForFileChange(ch <-chan remind.FileChangeEvent) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return fileChangedMsg{path: ev.Path}
	}
}

// sourceFiles returns the reminder files whose modification times the periodic
// refresh checks.
func (m *Model) sourceFiles() []string {
	if m.remindClient != nil && len(m.remindClient.Files) > 0 {
		return m.remindClient.Files
	}
	if m.config != nil {
		return m.config.RemindFiles
	}
	return nil
}

// cacheIsStale reports whether the periodic refresh should discard the cache:
// a source file was modified after the cache was built, or the calendar day
// has changed since then, which moves reminders that depend on today's date.
func (m *Model) cacheIsStale(now time.Time) bool {
	if m.cacheTime.IsZero() {
		return true
	}
	if now.Year() != m.cacheTime.Year() || now.YearDay() != m.cacheTime.YearDay() {
		return true
	}
	for _, path := range m.sourceFiles() {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(m.cacheTime) {
			return true
		}
	}
	return false
}
