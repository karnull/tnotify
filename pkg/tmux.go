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

// PaneInfo identifies the pane a notification was sent from. ID is tmux's own
// pane identifier ("%3"), which stays with the pane however its window is
// renamed or reordered; the session, window and index are there to describe it
// to a person. Title is the title the pane was wearing before tnotify renamed
// it, kept so it can be put back.
type PaneInfo struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Window  string `json:"window"`
	Index   string `json:"index"`
	Title   string `json:"title"`
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

// Whether a "#{session_attached}" reading says somebody is watching. tmux
// answers with a count, and anything it could not count is taken as nobody
// there rather than as somebody who might be.
func attachedClients(reading string) bool {
	count, err := strconv.Atoi(strings.TrimSpace(reading))
	if err != nil {
		return false
	}
	return count > 0
}

//- Public Calls -----------------------------------------------------------------------------------

// Whether this process is running inside a tmux session. Worth asking before
// anything tmux is merely told about, as opposed to asked for.
func InsideTmux() bool {
	return tmux.IsInsideTmux()
}

// Whether a client is attached to this pane's own session, meaning there is a
// screen for a popup to be drawn on. Being inside a session is not enough on its
// own: a detached session has panes running in it that nobody is looking at, and
// a client attached to some other session is no help to a popup drawn here.
func TmuxHasClient() bool {
	if !tmux.IsInsideTmux() {
		return false
	}

	// Asked of this session rather than the server, which would answer for
	// every session at once and call a detached one watched.
	stdout, _, err := tmux.RunCmd([]string{"display-message", "-p", "#{session_attached}"})
	if err != nil {
		return false
	}

	return attachedClients(stdout)
}

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

// Close any popup open on the client. This is how a notification nobody came to
// is taken off the screen once the caller waiting on it has run out of time.
func TmuxClosePopup() error {
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not inside a tmux session")
	}

	if _, stderr, err := tmux.RunCmd([]string{"display-popup", "-C"}); err != nil {
		return fmt.Errorf("display-popup -C: %v: %s", err, stderr)
	}

	return nil
}

// Set a variable in tmux's global environment, where a status line can read it
// back as "#{NAME}".
func TmuxSetGlobalEnv(name, value string) error {
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not inside a tmux session")
	}

	// The redraw goes across in the same command: a status line is only redrawn
	// on its own interval otherwise, so the value it shows would lag the one it
	// was just given by however long that is.
	args := []string{"set-environment", "-g", name, value, ";", "refresh-client", "-S"}

	if _, stderr, err := tmux.RunCmd(args); err != nil {
		return fmt.Errorf("set-environment: %v: %s", err, stderr)
	}

	return nil
}

// Describe the pane this process is running in. Title is the pane's current
// title, whatever that happens to be.
func TmuxCurrentPane() (PaneInfo, error) {
	if !tmux.IsInsideTmux() {
		return PaneInfo{}, fmt.Errorf("not inside a tmux session")
	}

	// The session name and the title are the two fields that may contain a
	// space, so they go last and are split off one at a time.
	format := "#{pane_id} #{window_index} #{pane_index} #{session_name}"
	stdout, stderr, err := tmux.RunCmd([]string{"display-message", "-p", format})
	if err != nil {
		return PaneInfo{}, fmt.Errorf("display-message: %v: %s", err, stderr)
	}

	fields := strings.SplitN(strings.TrimSpace(stdout), " ", 4)
	if len(fields) < 4 {
		return PaneInfo{}, fmt.Errorf("parsing pane info %q", strings.TrimSpace(stdout))
	}

	pane := PaneInfo{ID: fields[0], Window: fields[1], Index: fields[2], Session: fields[3]}

	// Read the title separately rather than risk it running into the fields
	// above, since a title is free-form text.
	panes, err := TmuxPanes()
	if err != nil {
		return pane, err
	}
	pane.Title = panes[pane.ID]

	return pane, nil
}

// Return every pane open on the tmux server, as pane id -> pane title. Panes
// are listed rather than asked about one at a time so that a pane which has
// been closed simply turns up missing, instead of as an error that has to be
// told apart from tmux itself failing.
func TmuxPanes() (map[string]string, error) {
	if !tmux.IsInsideTmux() {
		return nil, fmt.Errorf("not inside a tmux session")
	}

	stdout, stderr, err := tmux.RunCmd([]string{"list-panes", "-a", "-F", "#{pane_id} #{pane_title}"})
	if err != nil {
		return nil, fmt.Errorf("list-panes: %v: %s", err, stderr)
	}

	panes := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		// A title may contain spaces, and may be empty; the id never is.
		id, title, _ := strings.Cut(line, " ")
		panes[id] = title
	}

	return panes, nil
}

// Retitle a pane, without changing which pane is active. An empty title hands
// the pane back to whatever tmux would name it by default.
func TmuxRenamePane(id, title string) error {
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not inside a tmux session")
	}

	if _, stderr, err := tmux.RunCmd([]string{"select-pane", "-t", id, "-T", title}); err != nil {
		return fmt.Errorf("select-pane: %v: %s", err, stderr)
	}

	return nil
}

// Type text into a pane as though the user had typed it there. Nothing is
// entered afterwards, so the text lands on the pane's command line for whoever
// is at it to use, rather than being run.
func TmuxSendKeys(id, text string) error {
	if !tmux.IsInsideTmux() {
		return fmt.Errorf("not inside a tmux session")
	}

	// -l sends the text literally, so an answer that happens to read like a key
	// name ("Enter", "C-c") is typed rather than pressed.
	if _, stderr, err := tmux.RunCmd([]string{"send-keys", "-t", id, "-l", text}); err != nil {
		return fmt.Errorf("send-keys: %v: %s", err, stderr)
	}

	return nil
}
