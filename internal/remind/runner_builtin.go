package remind

import (
	remindgo "github.com/cwarden/remind/v6"
)

// BuiltinRunner runs the remind program compiled into this binary. It is
// only usable where remindgo.Builtin is true.
func BuiltinRunner(args []string, stdin string) (stdout, stderr []byte, exitCode int, err error) {
	res, err := remindgo.Run(args, []byte(stdin))
	if err != nil {
		return nil, nil, 0, err
	}
	return res.Stdout, res.Stderr, res.ExitCode, nil
}

// defaultRunner returns the built-in runner where the remind program is
// compiled in, and nil elsewhere, which makes the client run the external
// command named by Client.RemindPath.
func defaultRunner() Runner {
	if remindgo.Builtin {
		return BuiltinRunner
	}
	return nil
}
