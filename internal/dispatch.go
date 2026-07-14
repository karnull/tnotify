// internal/dispatch.go

package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/karnull/tnotify/pkg"
)

// Names the file an overlay writes the user's selection to, so the process that
// opened it can print the answer in the original terminal.
const resultEnvVar = "TNOTIFY_RESULT"

// A colour set to this in the config switches that element off instead.
const colorHidden = "<hidden>"

//- Private Helpers --------------------------------------------------------------------------------

// Translate the config's [colors] into the set the TUI draws with, resolving
// "<hidden>" and filling in anything an older config predates.
func notifyColors(cfg Config) pkg.NotifyColors {
	colors := pkg.NotifyColors{
		Border:    cfg.Colors.Border,
		Head:      cfg.Colors.Head,
		Message:   cfg.Colors.Message,
		Author:    cfg.Colors.Author,
		Selection: cfg.Colors.Selection,
		Footer:    cfg.Colors.Footer,
	}

	// Configs written before these existed still get a sensible notification.
	if colors.Selection == "" {
		colors.Selection = colors.Author
	}
	if colors.Footer == "" {
		colors.Footer = colors.Border
	}

	// An empty footer colour is how the TUI is told to leave the footer out.
	if colors.Footer == colorHidden {
		colors.Footer = ""
	}

	return colors
}

// Create the empty file an overlay reports its selection through, returning the
// path and a function that removes it.
func newResultFile() (path string, cleanup func(), err error) {
	file, err := os.CreateTemp("", "tnotify-result-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating result file: %w", err)
	}
	name := file.Name()

	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", nil, fmt.Errorf("creating result file: %w", err)
	}

	return name, func() { os.Remove(name) }, nil
}

// Assemble the notification a parsed request and the config describe.
func buildNotification(req notifyRequest, cfg Config) pkg.Notification {
	// The popup's parent is tmux, so only the outer invocation can tell who
	// actually sent the notification; it passes the answer down explicitly.
	author := req.Author
	if author == "" {
		author = defaultAuthor()
	}

	return pkg.Notification{
		Author:   author,
		Head:     req.Head,
		Body:     req.Body,
		Options:  req.Interactive,
		Custom:   req.Custom,
		Multiple: req.Multiple,
		Colors:   notifyColors(cfg),
	}
}

// Open a popup sized to the notification and return whatever the user picked
// in there, which is empty when they dismissed it.
func openNotifyOverlay(cfg Config, n pkg.Notification, args []string) (string, error) {
	// Size the overlay to the message before opening it, so the TUI inside is
	// handed a popup it already fits.
	overlay, err := notifyOverlay(cfg, n)
	if err != nil {
		return "", err
	}

	// The popup is a terminal of its own, so anything the user picks in there
	// has to be handed back through a file for the calling process — the one
	// still attached to the original terminal — to print.
	resultPath, cleanup, err := newResultFile()
	if err != nil {
		return "", err
	}
	defer cleanup()

	relaunchArgs := append([]string{"--internal", "notify"}, args...)
	relaunchArgs = append(relaunchArgs, "--author", n.Author)

	if err := pkg.TmuxOverlay(overlay, []string{resultEnvVar + "=" + resultPath}, relaunchArgs); err != nil {
		return "", err
	}

	selected, err := os.ReadFile(resultPath)
	if err != nil {
		return "", fmt.Errorf("reading selection: %w", err)
	}

	return string(selected), nil
}

// Show the notification and record what the user chose. Inside a tmux popup the
// selection goes to the file named by the result env var; run directly it goes
// to stdout.
func runNotifyTUI(n pkg.Notification) {
	result, err := pkg.RunNotifyTUI(n)
	if err != nil {
		reportError(err)
		return
	}

	// Ignoring and clearing both dismiss the notification without an answer.
	if result.Action != pkg.ActionSelect || len(result.Selected) == 0 {
		return
	}

	output := strings.Join(result.Selected, "\n") + "\n"

	path := os.Getenv(resultEnvVar)
	if path == "" {
		fmt.Print(output)
		return
	}

	if err := os.WriteFile(path, []byte(output), 0o600); err != nil {
		reportError(fmt.Errorf("writing selection: %w", err))
	}
}

// Dispatch "notify": open a tmux overlay that relaunches tnotify to draw the
// notification, or draw it when this is that relaunched process.
func dispatchNotify(args []string, isInternal bool) {
	req, err := parseNotify(args)
	if err != nil {
		reportError(err)
		return
	}

	cfg, err := LoadConfig()
	if err != nil {
		reportError(err)
		return
	}

	notification := buildNotification(req, cfg)

	if isInternal {
		runNotifyTUI(notification)
		return
	}

	selected, err := openNotifyOverlay(cfg, notification, args)
	if err != nil {
		reportError(err)
		return
	}
	fmt.Print(selected)
}

// Dispatch "show": relaunch tnotify inside the tmux layout the requested mode
// asks for, or print the notifications when this is that relaunched process.
//
//	--all       -> new pane on the side
//	--last      -> new overlay
//	(default)   -> new overlay
func dispatchShow(args []string, isInternal bool) {
	if isInternal {
		fmt.Println(parseShow(args))
		return
	}

	mode, err := showMode(args)
	if err != nil {
		reportError(err)
		return
	}

	cfg, err := LoadConfig()
	if err != nil {
		reportError(err)
		return
	}

	relaunchArgs := append([]string{"--internal", "show"}, args...)

	switch mode {
	case "all":
		err = pkg.TmuxPane(cfg.Sidepanel.Direction, cfg.Sidepanel.Width, relaunchArgs)
	default: // "last" and "default" both use an overlay
		var overlay pkg.Overlay
		if overlay, err = plainOverlay(cfg); err == nil {
			err = pkg.TmuxOverlay(overlay, nil, relaunchArgs)
		}
	}

	if err != nil {
		reportError(err)
	}
}
