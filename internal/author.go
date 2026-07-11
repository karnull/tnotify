// internal/author.go

package internal

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Lets any caller name itself without passing --author on every call, for
// programs and scripts that wrap tnotify.
const authorEnvVar = "TNOTIFY_AUTHOR"

// Shells are reported as the parent of anything they run, so a script invoking
// tnotify would otherwise be credited to its interpreter.
var shells = map[string]bool{
	"bash": true, "csh": true, "dash": true, "fish": true, "ksh": true,
	"sh": true, "tcsh": true, "zsh": true,
}

//- Private Helpers --------------------------------------------------------------------------------

// Read one ps field for a pid, or "" if ps cannot report it.
func psField(format, pid string) string {
	out, err := exec.Command("ps", "-o", format, "-p", pid).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Pick the script out of a shell's command line — the first argument that is
// not a flag — or "" when the shell is not running one.
func scriptArg(args string) string {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		return ""
	}

	for _, arg := range fields[1:] {
		if strings.HasPrefix(arg, "-") {
			// Everything after -c is inline code, not a script to name.
			if strings.HasPrefix(arg, "-c") {
				return ""
			}
			continue
		}
		return filepath.Base(arg)
	}

	return ""
}

// The name of whatever started this process, or "" if it cannot be read. A
// script is named in preference to the shell interpreting it.
func parentName() string {
	ppid := strconv.Itoa(os.Getppid())

	command := psField("comm=", ppid)
	if command == "" {
		return ""
	}

	// ps reports the full path, and marks login shells with a leading hyphen.
	command = filepath.Base(strings.TrimPrefix(command, "-"))

	if shells[command] {
		if script := scriptArg(psField("args=", ppid)); script != "" {
			return script
		}
	}

	return command
}

// Work out who is sending the notification when --author was not given.
//
// Nothing here knows about any particular tool: it reports whatever ran
// tnotify, whether that is a program, a script, or a person at a shell. A
// caller that wants to be named something else sets TNOTIFY_AUTHOR or passes
// --author. This is only meaningful in the outer invocation — inside the tmux
// popup the parent is tmux — so the outer process resolves it and passes it down.
func defaultAuthor() string {
	if name := strings.TrimSpace(os.Getenv(authorEnvVar)); name != "" {
		return name
	}

	if name := parentName(); name != "" {
		return name
	}

	// Nothing useful to name the caller by, so fall back to whose session it is.
	if current, err := user.Current(); err == nil && current.Username != "" {
		return current.Username
	}

	return "unknown"
}
