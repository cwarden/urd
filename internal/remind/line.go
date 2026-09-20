package remind

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// SerializesRuns reports whether the client's runs of remind cannot overlap.
// The built-in remind is not reentrant, so a run waits for the one before it
// to finish; the external command runs as a separate process each time.
func (c *Client) SerializesRuns() bool {
	return c.Runner != nil
}

// LineEvents runs the reminder on line lineNumber of path through remind
// without the other reminders in the file, and returns the events it
// produces in the months months starting with the one containing
// monthStart. The reminder is run after the directives that precede it in
// the file and that a reminder can depend on: SET, FSET, OMIT, INCLUDE and
// IF blocks (see contextDirectives). Those are few next to the reminders,
// so the run takes a fraction of the time of a run over the whole file. A
// reminder continued over several lines with trailing backslashes is read
// whole whichever of its lines is named. lineNumber 0 names the last
// non-blank line of the file, which is where an editor opened at the end of
// the file adds a reminder.
//
// The events carry path and the line number of the reminder's last line, as
// a run over the whole file reports them, so they replace that run's events
// for the same line. A reminder that still needs something else from the
// file fails to parse; the caller then waits for the run over the whole
// file.
func (c *Client) LineEvents(path string, lineNumber int, monthStart time.Time, months int) ([]Event, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text, last, err := reminderText(string(content), lineNumber)
	if err != nil {
		return nil, err
	}
	if months < 1 {
		months = 1
	}

	args := []string{
		fmt.Sprintf("-pppq%d", months), // JSON output for this many months
		"-l",
		"-g",
		"-b2",
		"-", // read the reminder from standard input
		monthName(monthStart.Month()),
		"1",
		fmt.Sprintf("%d", monthStart.Year()),
	}
	stdout, stderr, exitCode, err := c.run(args, text)
	if err != nil {
		return nil, fmt.Errorf("remind command failed: %w", err)
	}
	if len(stderr) > 0 {
		if syntaxErr := c.parseRemindError(string(stderr)); syntaxErr != nil {
			return nil, syntaxErr
		}
	}
	if exitCode != 0 && len(stdout) == 0 {
		return nil, fmt.Errorf("remind exited with status %d: %s", exitCode, strings.TrimSpace(string(stderr)))
	}

	parsed, err := ParseRemindJSON(stdout)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, month := range parsed {
		for i := range month.Entries {
			month.Entries[i].Filename = path
			month.Entries[i].LineNo = last
		}
		events = append(events, ConvertJSONToEvents(month.Entries, c.location())...)
	}
	return events, nil
}

// contextDirectives are the directives a reminder can depend on: they
// define variables and functions, set the omitted days, include other
// files, or enclose those in conditional blocks. Every other line of the
// file is a reminder or a comment.
var contextDirectives = map[string]bool{
	"SET": true, "FSET": true, "UNSET": true,
	"INCLUDE": true, "INCLUDER": true, "INCLUDECMD": true,
	"OMIT": true, "PUSH-OMIT-CONTEXT": true, "POP-OMIT-CONTEXT": true, "CLEAR-OMIT-CONTEXT": true,
	"IF": true, "IFDEF": true, "IFNDEF": true, "ELSE": true, "ENDIF": true,
	"PRESERVE": true,
}

// isContextDirective reports whether line starts with one of
// contextDirectives.
func isContextDirective(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && contextDirectives[strings.ToUpper(fields[0])]
}

// reminderText returns the text to run for the reminder that includes line
// lineNumber of content: the context directives before it in the file,
// each extended over the lines joined to it by trailing backslashes, then
// the reminder itself, extended the same way. It also returns the number
// of the reminder's last line. lineNumber 0 selects the last non-blank
// line. Only the directives before the reminder are kept because remind
// reads a file in order, so a definition after a reminder does not apply
// to it.
func reminderText(content string, lineNumber int) (text string, last int, err error) {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if lineNumber == 0 {
		lineNumber = len(lines)
		for lineNumber > 0 && strings.TrimSpace(lines[lineNumber-1]) == "" {
			lineNumber--
		}
		if lineNumber == 0 {
			return "", 0, fmt.Errorf("file has no reminder lines")
		}
	}
	if lineNumber < 0 || lineNumber > len(lines) {
		return "", 0, fmt.Errorf("line %d is beyond the end of the file (%d lines)", lineNumber, len(lines))
	}
	first := lineNumber
	for first > 1 && continues(lines[first-2]) {
		first--
	}
	last = lineNumber
	for last < len(lines) && continues(lines[last-1]) {
		last++
	}

	var b strings.Builder
	for i := 0; i < first-1; {
		// lines[i:j] is one logical line. It ends before the reminder,
		// which starts at the first line not continued from an earlier one.
		j := i + 1
		for j < first-1 && continues(lines[j-1]) {
			j++
		}
		if isContextDirective(lines[i]) {
			b.WriteString(strings.Join(lines[i:j], "\n"))
			b.WriteByte('\n')
		}
		i = j
	}
	b.WriteString(strings.Join(lines[first-1:last], "\n"))
	b.WriteByte('\n')
	return b.String(), last, nil
}

// continues reports whether remind joins the next line to line: a trailing
// backslash continues a reminder.
func continues(line string) bool {
	return strings.HasSuffix(strings.TrimRight(line, "\r"), "\\")
}
