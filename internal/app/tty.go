package app

import "os"

// isTerminal reports whether f is an interactive terminal. Uses the file's
// stat mode rather than syscalls so behaviour is consistent with stdin checks
// elsewhere in the project.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func stdoutIsTTY() bool { return isTerminal(os.Stdout) }
func stderrIsTTY() bool { return isTerminal(os.Stderr) }
