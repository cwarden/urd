package remind

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	remindgo "github.com/cwarden/remind/v6"
)

func requireBuiltin(t *testing.T) {
	t.Helper()
	if !remindgo.Builtin {
		t.Skip("remind is not built in on this platform")
	}
}

func writeReminders(t *testing.T, file, body string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func march2024Events(t *testing.T, c *Client) []Event {
	t.Helper()
	events, err := c.GetEvents(
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestBuiltinRunnerGetEvents(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, "REM 15 Mar 2024 MSG Planning meeting\nREM 20 Mar 2024 AT 09:30 MSG Standup\n")
	c := NewClient()
	c.SetFiles([]string{file})
	events := march2024Events(t, c)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(events), events)
	}
	byDesc := map[string]Event{}
	for _, e := range events {
		byDesc[e.Description] = e
	}
	planning, ok := byDesc["Planning meeting"]
	if !ok {
		t.Fatalf("no Planning meeting event in %+v", events)
	}
	if planning.Date.Day() != 15 || planning.Time != nil {
		t.Errorf("Planning meeting: date %v time %v, want the 15th, untimed", planning.Date, planning.Time)
	}
	standup, ok := byDesc["Standup"]
	if !ok {
		t.Fatalf("no Standup event in %+v", events)
	}
	if standup.Time == nil || standup.Time.Hour() != 9 || standup.Time.Minute() != 30 {
		t.Errorf("Standup time %v, want 09:30", standup.Time)
	}
	if standup.Filename != file || standup.LineNumber != 2 {
		t.Errorf("Standup location %s:%d, want %s:2", standup.Filename, standup.LineNumber, file)
	}
}

func TestBuiltinRunnerSeesEdits(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	c := NewClient()
	c.SetFiles([]string{file})
	writeReminders(t, file, "REM 15 Mar 2024 MSG First\n")
	if events := march2024Events(t, c); len(events) != 1 || events[0].Description != "First" {
		t.Fatalf("first read: %+v", events)
	}
	writeReminders(t, file, "REM 15 Mar 2024 MSG Second\nREM 16 Mar 2024 MSG Third\n")
	events := march2024Events(t, c)
	if len(events) != 2 {
		t.Fatalf("after the edit: got %d events, want 2: %+v", len(events), events)
	}
	for _, e := range events {
		if e.Description == "First" {
			t.Errorf("stale event after the edit: %+v", e)
		}
	}
}

func TestBuiltinRunnerReportsSyntaxErrors(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, "REM 15 Mar 2024 AT MSG missing time\n")
	c := NewClient()
	c.SetFiles([]string{file})
	_, err := c.GetEvents(
		time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.March, 31, 0, 0, 0, 0, time.UTC))
	if _, ok := err.(*RemindSyntaxError); !ok {
		t.Errorf("got %v, want a RemindSyntaxError", err)
	}
}

func TestBuiltinRunnerFindNext(t *testing.T) {
	requireBuiltin(t)
	file := filepath.Join(t.TempDir(), "cal.rem")
	writeReminders(t, file, "REM 20 Mar 2024 AT 09:30 MSG Dentist\n")
	c := NewClient()
	c.SetFiles([]string{file})
	event, err := c.FindNext("dentist", time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if event == nil || event.Date.Day() != 20 {
		t.Errorf("got %+v, want the dentist event on the 20th", event)
	}
}

func TestBuiltinRunnerTestConnection(t *testing.T) {
	requireBuiltin(t)
	if err := NewClient().TestConnection(); err != nil {
		t.Error(err)
	}
}

func TestUseCommandRunsExternalProgram(t *testing.T) {
	c := NewClient()
	c.UseCommand(filepath.Join(t.TempDir(), "no-such-remind"))
	if err := c.TestConnection(); err == nil {
		t.Error("TestConnection succeeded with a missing external remind")
	}
}
