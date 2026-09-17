package remind

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReminderText(t *testing.T) {
	content := "REM 1 Mar 2024 MSG One\nREM 2 Mar 2024 \\\n MSG Two \\\n continued\nREM 3 Mar 2024 MSG Three\n\n"
	tests := []struct {
		name     string
		line     int
		wantText string
		wantLast int
		wantErr  bool
	}{
		{name: "single line", line: 1, wantText: "REM 1 Mar 2024 MSG One\n", wantLast: 1},
		{name: "first line of a continued reminder", line: 2, wantText: "REM 2 Mar 2024 \\\n MSG Two \\\n continued\n", wantLast: 4},
		{name: "middle line of a continued reminder", line: 3, wantText: "REM 2 Mar 2024 \\\n MSG Two \\\n continued\n", wantLast: 4},
		{name: "last line of a continued reminder", line: 4, wantText: "REM 2 Mar 2024 \\\n MSG Two \\\n continued\n", wantLast: 4},
		{name: "zero selects the last non-blank line", line: 0, wantText: "REM 3 Mar 2024 MSG Three\n", wantLast: 5},
		{name: "blank trailing line", line: 6, wantText: "\n", wantLast: 6},
		{name: "beyond the end", line: 7, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, last, err := reminderText(content, tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want an error", text)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if text != tt.wantText || last != tt.wantLast {
				t.Errorf("got %q line %d, want %q line %d", text, last, tt.wantText, tt.wantLast)
			}
		})
	}
	if _, _, err := reminderText("\n\n", 0); err == nil {
		t.Error("blank file: got no error")
	}
}

func TestReminderTextKeepsTheDirectivesBeforeTheReminder(t *testing.T) {
	content := strings.Join([]string{
		"# comment",
		"SET day 15",
		"  fset t() \\",
		"     \"x\"",
		"REM 1 Mar 2024 MSG Earlier reminder",
		"IF day > 10",
		"  OMIT 4 Jul",
		"  REM 4 Jul MSG Skipped",
		"ENDIF",
		"REM Mar [day] 2024 MSG Target",
		"SET later 1",
		"",
	}, "\n")
	want := strings.Join([]string{
		"SET day 15",
		"  fset t() \\",
		"     \"x\"",
		"IF day > 10",
		"  OMIT 4 Jul",
		"ENDIF",
		"REM Mar [day] 2024 MSG Target",
		"",
	}, "\n")
	text, last, err := reminderText(content, 10)
	if err != nil {
		t.Fatal(err)
	}
	if text != want {
		t.Errorf("got:\n%s\nwant:\n%s", text, want)
	}
	if last != 10 {
		t.Errorf("last line %d, want 10", last)
	}
}

func TestLineEventsRunsTheLineAloneOnStandardInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, "SET x 1\nREM 15 Mar 2024 AT 10:00 MSG Standup\nREM 20 Mar 2024 MSG Other\n")
	var gotArgs []string
	var gotStdin string
	c := NewClient()
	c.Runner = func(args []string, stdin string) ([]byte, []byte, int, error) {
		gotArgs, gotStdin = args, stdin
		out := `[{"monthname":"February","year":2024,"entries":[]},` +
			`{"monthname":"March","year":2024,"entries":[` +
			`{"date":"2024-03-15","filename":"-stdin-","lineno":1,"time":600,"priority":5000,"body":"Standup"}]},` +
			`{"monthname":"April","year":2024,"entries":[]}]`
		return []byte(out), nil, 0, nil
	}

	events, err := c.LineEvents(file, 2, time.Date(2024, time.February, 1, 0, 0, 0, 0, time.Local), 3)
	if err != nil {
		t.Fatal(err)
	}
	if gotStdin != "SET x 1\nREM 15 Mar 2024 AT 10:00 MSG Standup\n" {
		t.Errorf("stdin %q, want the SET and line 2", gotStdin)
	}
	wantArgs := []string{"-pppq3", "-l", "-g", "-b2", "-", "Feb", "1", "2024"}
	if strings.Join(gotArgs, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("args %q, want %q", gotArgs, wantArgs)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(events), events)
	}
	e := events[0]
	if e.Filename != file || e.LineNumber != 2 {
		t.Errorf("location %s:%d, want %s:2", e.Filename, e.LineNumber, file)
	}
	if e.ID != EventID(e.Date, 2) {
		t.Errorf("ID %q, want %q", e.ID, EventID(e.Date, 2))
	}
	if e.Time == nil || e.Time.Hour() != 10 {
		t.Errorf("time %v, want 10:00", e.Time)
	}
}

func TestLineEventsReportsRemindErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, "REM 15 Mar 2024 AT MSG missing time\n")
	c := NewClient()
	c.Runner = func(args []string, stdin string) ([]byte, []byte, int, error) {
		return nil, []byte("-stdin-(1): Expecting time\n"), 0, nil
	}
	if _, err := c.LineEvents(file, 1, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.Local), 1); err == nil {
		t.Error("got no error for a line remind rejects")
	}
	if _, err := c.LineEvents(file, 9, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.Local), 1); err == nil {
		t.Error("got no error for a line past the end of the file")
	}
}

// TestBuiltinLineEventsMatchFullRun checks that a line parsed by itself
// yields the events the whole file yields for that line, with the same
// IDs and locations, so the two can replace one another in a cache.
func TestBuiltinLineEventsMatchFullRun(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, strings.Join([]string{
		"REM 15 Mar 2024 MSG Planning meeting",
		"REM Mon AT 09:30 DURATION 0:30 \\",
		"    MSG Standup",
		"REM 2 Apr 2024 AT 14:00 MSG Review",
		"",
	}, "\n"))
	c := NewClient()
	c.SetFiles([]string{file})
	from := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.Local)
	full, err := c.GetEvents(from, time.Date(2024, time.April, 30, 23, 59, 59, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(full) == 0 {
		t.Fatal("the full run produced no events")
	}
	describe := func(e Event) string {
		tm := "untimed"
		if e.Time != nil {
			tm = e.Time.Format("15:04")
		}
		dur := ""
		if e.Duration != nil {
			dur = e.Duration.String()
		}
		return fmt.Sprintf("%s %s %s %s %s:%d %s", e.ID, e.Date.Format("2006-01-02"), tm, dur, e.Filename, e.LineNumber, e.Description)
	}
	for _, line := range []int{1, 2, 3, 4} {
		got, err := c.LineEvents(file, line, from, 3)
		if err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		reported := line
		if line == 2 {
			// remind reports a continued reminder at its last line.
			reported = 3
		}
		want := map[string]bool{}
		for _, e := range full {
			if e.LineNumber == reported {
				want[describe(e)] = true
			}
		}
		if len(want) == 0 {
			t.Fatalf("line %d: the full run has no events at line %d", line, reported)
		}
		gotSet := map[string]bool{}
		for _, e := range got {
			gotSet[describe(e)] = true
		}
		for d := range want {
			if !gotSet[d] {
				t.Errorf("line %d: missing %s", line, d)
			}
		}
		for d := range gotSet {
			if !want[d] {
				t.Errorf("line %d: unexpected %s", line, d)
			}
		}
	}
}

func TestBuiltinLineEventsUsesDefinitionsBeforeTheLine(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, strings.Join([]string{
		"SET day 15",
		`FSET tag() "[tagged]"`,
		"REM Mar 1 2024 MSG Other",
		"REM Mar [day] 2024 MSG Uses [tag()]",
		"SET day 20",
		"",
	}, "\n"))
	c := NewClient()
	c.SetFiles([]string{file})
	march := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.Local)
	events, err := c.LineEvents(file, 4, march, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Date.Day() != 15 || events[0].Description != "Uses [tagged]" {
		t.Errorf("got %+v, want one event on the 15th described by the function", events)
	}
	if _, err := c.LineEvents(file, 3, march, 1); err != nil {
		t.Errorf("line 3: %v", err)
	}
	writeReminders(t, file, "REM Mar [day] 2024 MSG Uses a variable\nSET day 15\n")
	if _, err := c.LineEvents(file, 1, march, 1); err == nil {
		t.Error("got no error for a line that reads a variable set after it")
	}
}

func TestSerializesRuns(t *testing.T) {
	c := NewClient()
	c.Runner = BuiltinRunner
	if !c.SerializesRuns() {
		t.Error("a client with a runner does not report serialized runs")
	}
	c.UseCommand("remind")
	if c.SerializesRuns() {
		t.Error("a client running the external command reports serialized runs")
	}
}
