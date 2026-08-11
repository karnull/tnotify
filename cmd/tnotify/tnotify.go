// cmd/tnotify/tnotify.go

package main

import (
	"os"

	"github.com/karnull/tnotify/internal"
)

//- Private Helpers --------------------------------------------------------------------------------

// Strip the leading "--internal" marker, reporting whether it was there. It is
// set when tnotify relaunches itself inside a tmux pane, overlay or window.
func splitInternal(args []string) (rest []string, isInternal bool) {
	if len(args) > 0 && args[0] == "--internal" {
		return args[1:], true
	}
	return args, false
}

//- Public Calls -----------------------------------------------------------------------------------

// Entry point: hand the command line to the CLI, and exit with whatever it
// makes of it.
func main() {
	args, isInternal := splitInternal(os.Args[1:])
	os.Exit(internal.ProcessArgs(args, isInternal))
}
