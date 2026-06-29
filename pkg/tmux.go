// pkg/tmux.go

package pkg

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tmux "github.com/jubnzv/go-tmux"
)

// Overlay is the position and size of a tmux popup, in terminal cells. X and Y
// follow tmux's own convention: X is the popup's left column, and Y is the row
// one past its bottom, so the popup covers rows Y-Height .. Y-1. Width and
// Height are the popup's full size, border included.
type Overlay struct {
	X      int
	Y      int
	Width  int
	Height int
}

//- Private Helpers --------------------------------------------------------------------------------

// Return the path to the currently running tnotify binary, so a new tmux
// pane or overlay can relaunch it instead of running a separate binary.
func selfBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determining executable path: %w", err)
	}
	return exe, nil
}

// Run a tmux command that opens somewhere new and starts this binary with args
// inside it. tmuxArgs is the tmux command and its own flags.
func runSelfInTmux(tmuxArgs, args []string) error {
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not inside a tmux session")
	}

	exe, err := selfBinary()
	if err != nil {
		return err
	}

	cmdArgs := make([]string, 0, len(tmuxArgs)+len(args)+1)
	cmdArgs = append(cmdArgs, tmuxArgs...)
	cmdArgs = append(cmdArgs, exe)
	cmdArgs = append(cmdArgs, args...)

	if _, stderr, err := tmux.RunCmd(cmdArgs); err != nil {
		return fmt.Errorf("%s: %v: %s", tmuxArgs[0], err, stderr)
	}

	return nil
}

// The tmux split-window flags that pin a new pane to the extreme edge given by
// side. The panel lists notifications one under another, so it is only ever put
// down one side of the window.
func splitFlags(side string) ([]string, error) {
	switch side {
	case "left":
		return []string{"-h", "-b", "-t", "{left}"}, nil
	case "right":
		return []string{"-h", "-t", "{right}"}, nil
	default:
		return nil, fmt.Errorf("invalid side %q: must be left or right", side)
	}
}

//- Public Calls -----------------------------------------------------------------------------------

// Return the size of the tmux client displaying the current pane, in cells.
// This is the whole terminal, status line included, which is the coordinate
// space popups are positioned in.
func TmuxClientSize() (width, height int, err error) {
	if !tmux.IsInsideTmux() {
		return 0, 0, fmt.Errorf("not inside a tmux session")
	}

	stdout, stderr, err := tmux.RunCmd([]string{"display-message", "-p", "#{client_width} #{client_height}"})
	if err != nil {
		return 0, 0, fmt.Errorf("display-message: %v: %s", err, stderr)
	}

	if _, err := fmt.Sscanf(strings.TrimSpace(stdout), "%d %d", &width, &height); err != nil {
		return 0, 0, fmt.Errorf("parsing client size %q: %w", strings.TrimSpace(stdout), err)
	}

	return width, height, nil
}

// Split the current window, pinning a new pane of sizePercent to the given
// side, and run the tnotify binary with args inside it.
func TmuxPane(side string, sizePercent int, args []string) error {
	flags, err := splitFlags(side)
	if err != nil {
		return err
	}

	tmuxArgs := append([]string{"split-window"}, flags...)
	tmuxArgs = append(tmuxArgs, "-l", fmt.Sprintf("%d%%", sizePercent))

	return runSelfInTmux(tmuxArgs, args)
}

// Open a popup at the given position and size, and run the tnotify binary with
// args inside it. Each entry of env is a "NAME=value" pair set in the popup's
// environment, and the call blocks until the popup closes.
func TmuxOverlay(overlay Overlay, env []string, args []string) error {
	tmuxArgs := []string{
		"display-popup",
		"-x", strconv.Itoa(overlay.X),
		"-y", strconv.Itoa(overlay.Y),
		"-w", strconv.Itoa(overlay.Width),
		"-h", strconv.Itoa(overlay.Height),
	}
	for _, pair := range env {
		tmuxArgs = append(tmuxArgs, "-e", pair)
	}
	tmuxArgs = append(tmuxArgs, "-E")

	return runSelfInTmux(tmuxArgs, args)
}
