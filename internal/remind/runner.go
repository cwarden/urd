package remind

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// Runner runs the remind program with the given arguments and standard
// input and returns its standard output, standard error and exit status.
// err is non-nil only when remind could not be run at all.
type Runner func(args []string, stdin string) (stdout, stderr []byte, exitCode int, err error)

// run executes remind through c.Runner, or through the external program
// named by c.RemindPath when no runner is set.
func (c *Client) run(args []string, stdin string) (stdout, stderr []byte, exitCode int, err error) {
	if c.Runner != nil {
		return c.Runner(args, stdin)
	}
	return runCommand(c.RemindPath, args, stdin)
}

// runCommand runs an external remind program.
func runCommand(path string, args []string, stdin string) (stdout, stderr []byte, exitCode int, err error) {
	cmd := exec.Command(path, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
		err = nil
	}
	return out.Bytes(), errb.Bytes(), exitCode, err
}
