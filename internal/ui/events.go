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
// cached for the current generation are not fetched again. The selected
// month is fetched first and its neighbors wait for it: the built-in remind
// runs one query at a time, so this gets the visible month on screen after
// one query instead of three.
func (m *Model) ensureEventsLoaded() tea.Cmd {
	if m.source == nil {
		return nil
	}
	m.rebuildEvents()
	months := m.visibleMonths()
	selected := monthKeyFor(m.selectedDate)
	first := m.fetchMonthCmd(selected)
	var rest []tea.Cmd
	for _, key := range months {
		if key == selected {
			continue
		}
		if cmd := m.fetchMonthCmd(key); cmd != nil {
			rest = append(rest, cmd)
		}
	}
	if first == nil {
		return tea.Batch(rest...)
	}
	done := make(chan struct{})
	cmds := []tea.Cmd{func() tea.Msg {
		defer close(done)
		return first()
	}}
	for _, cmd := range rest {
		cmds = append(cmds, func() tea.Msg {
			<-done
			return cmd()
		})
	}
	return tea.Batch(cmds...)
}

// reloadEvents marks every cached month stale and fetches the visible months
// again. Results from fetches started before this call are discarded when
// they arrive.
func (m *Model) reloadEvents() tea.Cmd {
	m.cacheGen++
	m.liveGen.Store(int64(m.cacheGen))
	m.cacheTime = time.Now()
	m.sourceStamps = m.statSourceFiles()
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
		m.sourceStamps = m.statSourceFiles()
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
	source, gen, live := m.source, m.cacheGen, &m.liveGen
	return func() tea.Msg {
		if live.Load() != int64(gen) {
			// A reload made this fetch stale before it ran; its result would
			// be discarded, so skip the query.
			return eventLoadedMsg{key: key, gen: gen}
		}
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

// fileStamp is the modification time and size of a source file, or the zero
// value when the file could not be read.
type fileStamp struct {
	modTime time.Time
	size    int64
}

// statSourceFiles returns the current stamp of each source file.
func (m *Model) statSourceFiles() map[string]fileStamp {
	stamps := make(map[string]fileStamp)
	for _, path := range m.sourceFiles() {
		var stamp fileStamp
		if info, err := os.Stat(path); err == nil {
			stamp = fileStamp{modTime: info.ModTime(), size: info.Size()}
		}
		stamps[path] = stamp
	}
	return stamps
}

// cacheIsStale reports whether the periodic refresh should discard the cache:
// a source file's modification time or size differs from when the cache was
// built, or the calendar day has changed since then, which moves reminders
// that depend on today's date. The modification time is compared for
// equality rather than against the cache time because a file replaced by a
// copy that keeps its original time (rsync -t, cp -p, sync clients) can have
// a modification time older than the cache.
func (m *Model) cacheIsStale(now time.Time) bool {
	if m.cacheTime.IsZero() {
		return true
	}
	if now.Year() != m.cacheTime.Year() || now.YearDay() != m.cacheTime.YearDay() {
		return true
	}
	current := m.statSourceFiles()
	if len(current) != len(m.sourceStamps) {
		return true
	}
	for path, stamp := range current {
		saved, ok := m.sourceStamps[path]
		if !ok || !saved.modTime.Equal(stamp.modTime) || saved.size != stamp.size {
			return true
		}
	}
	return false
}

// lineParsedMsg carries the events remind produced for the line an editor
// was opened at, run on its own while the whole file is reloaded. gen is the
// cache generation of that reload.
type lineParsedMsg struct {
	gen    int
	file   string
	line   int
	events []remind.Event
	err    error
}

// reloadAfterEdit reloads every visible month after an editor session and,
// at the same time, runs the line the editor was opened at through remind
// by itself. That line is assumed to be the one that changed. Its events
// are usually ready long before the whole file has been read, so they go on
// screen first; the full reload then replaces them. With the built-in
// remind, runs cannot overlap, so the full reload starts once the line has
// been parsed rather than competing with it for the next run.
func (m *Model) reloadAfterEdit(file string, line int) tea.Cmd {
	reload := m.reloadEvents()
	if m.remindClient == nil || file == "" {
		return reload
	}
	client, gen := m.remindClient, m.cacheGen
	months := m.visibleMonths()
	done := make(chan struct{})
	parse := func() tea.Msg {
		defer close(done)
		events, err := client.LineEvents(file, line, months[0].start(), len(months))
		return lineParsedMsg{gen: gen, file: file, line: line, events: events, err: err}
	}
	if reload != nil && client.SerializesRuns() {
		full := reload
		reload = func() tea.Msg {
			<-done
			return full()
		}
	}
	return tea.Batch(parse, reload)
}

// applyLineParsed puts the events of one parsed line into the cached months
// that the full reload started with it has not replaced yet: those still
// holding an older generation. A month that reload has already refreshed
// holds the events of the whole file, which are at least as accurate, so it
// is left alone. A parse error is ignored; the full reload reports it.
func (m *Model) applyLineParsed(msg lineParsedMsg) {
	if msg.err != nil {
		return
	}
	// The line's events are replaced by line number. The reminder may have
	// grown or shrunk by continuation lines, so both the line the editor
	// opened and the line remind now reports are replaced.
	lines := map[int]bool{}
	if msg.line > 0 {
		lines[msg.line] = true
	}
	byMonth := make(map[monthKey][]remind.Event)
	for _, e := range msg.events {
		lines[e.LineNumber] = true
		key := monthKeyFor(e.Date)
		byMonth[key] = append(byMonth[key], e)
	}
	changed := false
	for _, key := range m.visibleMonths() {
		c, ok := m.eventCache[key]
		if !ok || c.gen >= msg.gen {
			continue
		}
		var kept []remind.Event
		for _, e := range c.events {
			if e.Filename == msg.file && lines[e.LineNumber] {
				continue
			}
			kept = append(kept, e)
		}
		c.events = append(kept, byMonth[key]...)
		m.eventCache[key] = c
		changed = true
	}
	if changed {
		m.rebuildEvents()
	}
}

// removeLineEvents drops the events of the reminder on event's line from
// every cached month and moves the events of later lines in the same file up
// by one line, which is what deleting the line did to the file. The cache
// then matches the file until the next reload replaces it.
func (m *Model) removeLineEvents(event remind.Event) {
	file := event.Filename
	if file == "" {
		if files := m.sourceFiles(); len(files) > 0 {
			file = files[0]
		}
	}
	for key, c := range m.eventCache {
		var kept []remind.Event
		for _, e := range c.events {
			switch {
			case event.LineNumber <= 0:
				if e.ID == event.ID {
					continue
				}
			case e.Filename == file && e.LineNumber == event.LineNumber:
				continue
			case e.Filename == file && e.LineNumber > event.LineNumber:
				e.LineNumber--
				e.ID = remind.EventID(e.Date, e.LineNumber)
			}
			kept = append(kept, e)
		}
		c.events = kept
		m.eventCache[key] = c
	}
	m.rebuildEvents()
}
