// internal/dispatch.go

package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

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
		Options:  req.Options,
		Custom:   req.Custom,
		Multiple: req.Multiple,
		Colors:   notifyColors(cfg),
	}
}

// Restore a stored notification to the form the TUI draws. Its colours come
// from the config as it stands now, not as it stood when it was sent.
func storedNotificationView(stored storedNotification, cfg Config) pkg.Notification {
	return pkg.Notification{
		Author:   stored.Author,
		Head:     stored.Head,
		Body:     stored.Body,
		Options:  stored.Options,
		Custom:   stored.Custom,
		Multiple: stored.Multiple,
		Colors:   notifyColors(cfg),
	}
}

// Rebuild the "notify" command line a stored notification came from, so the
// popup can be handed the same arguments the original one was. --author is left
// out because the overlay appends it itself.
func storedNotificationArgs(stored storedNotification) []string {
	args := []string{stored.Body}

	if stored.Head != "" {
		args = append(args, "--head", stored.Head)
	}
	if stored.Interactive {
		args = append(args, "--interactive")
		args = append(args, stored.Options...)
	}
	if stored.Custom {
		args = append(args, "--custom")
	}
	if stored.Multiple {
		args = append(args, "--multiple")
	}

	return args
}

// Everything worth keeping about a notification the user ignored, so that it
// can be raised again and answered later.
func storeIgnored(req notifyRequest, n pkg.Notification, sent time.Time) {
	// A pane that cannot be identified is not worth dropping the notification
	// over: it can still be shown again, only its answer has nowhere to go.
	pane, err := pkg.TmuxCurrentPane()
	if err != nil {
		reportError(fmt.Errorf("recording pane: %w", err))
	}

	stored := storedNotification{
		Sent:        sent,
		Author:      n.Author,
		TrueAuthor:  trueAuthor(),
		Head:        req.Head,
		Body:        req.Body,
		Options:     req.Options,
		Interactive: req.Interactive,
		Custom:      req.Custom,
		Multiple:    req.Multiple,
		Pane:        pane,
	}

	if err := rememberNotification(stored); err != nil {
		reportError(err)
	}
}

// Deliver an answer to the pane the notification was sent from, typing it in
// without entering it, so it lands on that pane's command line rather than
// running there.
func replyToPane(stored storedNotification, selected []string) error {
	if stored.Pane.ID == "" {
		return fmt.Errorf("notification %d has no pane to answer", stored.ID)
	}

	panes, err := pkg.TmuxPanes()
	if err != nil {
		return err
	}

	// The pane has to still be the one tnotify marked: pane ids start over when
	// the tmux server restarts, and typing an answer into a stranger's shell is
	// worse than not delivering it at all.
	title, open := panes[stored.Pane.ID]
	switch {
	case !open:
		return fmt.Errorf("pane %s has closed", stored.Pane.ID)
	case !markedPane(title):
		return fmt.Errorf("pane %s is no longer waiting on a notification", stored.Pane.ID)
	}

	// A newline would be typed as enter, running whatever came before it, so
	// several answers go across on one line.
	return pkg.TmuxSendKeys(stored.Pane.ID, strings.Join(selected, " "))
}

// Write an overlay's outcome in the form the process that opened it reads back:
// the action on the first line, and anything the user picked on the lines after.
func encodeResult(result pkg.NotifyResult) string {
	return strings.Join(append([]string{result.Action}, result.Selected...), "\n") + "\n"
}

// Read back what an overlay reported. An overlay that never got as far as
// writing anything is taken to have been ignored, which is the outcome that
// loses the least.
func decodeResult(data string) pkg.NotifyResult {
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	if lines[0] == "" {
		return pkg.NotifyResult{Action: pkg.ActionIgnore}
	}

	return pkg.NotifyResult{Action: lines[0], Selected: lines[1:]}
}

// Open a popup sized to the notification and report what the user did with it.
func openNotifyOverlay(cfg Config, n pkg.Notification, args []string) (pkg.NotifyResult, error) {
	// Size the overlay to the message before opening it, so the TUI inside is
	// handed a popup it already fits.
	overlay, err := notifyOverlay(cfg, n)
	if err != nil {
		return pkg.NotifyResult{}, err
	}

	// The popup is a terminal of its own, so what happens in there has to be
	// handed back through a file for the calling process — the one still
	// attached to the original terminal — to act on.
	resultPath, cleanup, err := newResultFile()
	if err != nil {
		return pkg.NotifyResult{}, err
	}
	defer cleanup()

	relaunchArgs := append([]string{"--internal", "notify"}, args...)
	relaunchArgs = append(relaunchArgs, "--author", n.Author)

	if err := pkg.TmuxOverlay(overlay, []string{resultEnvVar + "=" + resultPath}, relaunchArgs); err != nil {
		return pkg.NotifyResult{}, err
	}

	reported, err := os.ReadFile(resultPath)
	if err != nil {
		return pkg.NotifyResult{}, fmt.Errorf("reading selection: %w", err)
	}

	return decodeResult(string(reported)), nil
}

// Show the notification and report what the user did. Inside a tmux popup the
// outcome goes to the file named by the result env var; run directly it goes to
// stdout.
func runNotifyTUI(n pkg.Notification) {
	result, err := pkg.RunNotifyTUI(n)
	if err != nil {
		reportError(err)
		return
	}

	path := os.Getenv(resultEnvVar)

	// Run directly there is nobody waiting on the outcome, so only an actual
	// answer is worth printing.
	if path == "" {
		if result.Action == pkg.ActionSelect && len(result.Selected) > 0 {
			fmt.Println(strings.Join(result.Selected, "\n"))
		}
		return
	}

	// The process that opened the popup needs the action as well as the answer:
	// an ignored notification is the one worth keeping, and a cleared one the
	// one worth throwing away.
	if err := os.WriteFile(path, []byte(encodeResult(result)), 0o600); err != nil {
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

	// Taken before the popup opens, since this is when the notification was
	// sent rather than whenever the user got round to dismissing it.
	sent := time.Now()

	result, err := openNotifyOverlay(cfg, notification, args)
	if err != nil {
		reportError(err)
		return
	}

	switch result.Action {
	case pkg.ActionSelect:
		fmt.Println(strings.Join(result.Selected, "\n"))

	case pkg.ActionIgnore:
		// An ignored notification is kept so it can be picked up later; a
		// cleared one was dismissed on purpose, and is not.
		storeIgnored(req, notification, sent)
	}
}

// Raise the most recently ignored notification again, exactly as it first
// arrived, and deal with whatever the user does with it this time.
func showLast(cfg Config) {
	// A notification whose pane has gone can still be answered, but the answer
	// will have nowhere to go; find that out before showing it rather than after.
	if err := reapOrphans(); err != nil {
		reportError(err)
	}

	stored, found, err := lastNotification()
	if err != nil {
		reportError(err)
		return
	}
	if !found {
		fmt.Println("no ignored notifications")
		return
	}

	result, err := openNotifyOverlay(cfg, storedNotificationView(stored, cfg), storedNotificationArgs(stored))
	if err != nil {
		reportError(err)
		return
	}

	// Ignored a second time, it stays exactly where it was, still waiting.
	if result.Action == pkg.ActionIgnore {
		return
	}

	if result.Action == pkg.ActionSelect && len(result.Selected) > 0 {
		// An orphaned notification has no pane left to answer into; anything
		// else is worth trying, and may still turn out to have lost its pane.
		delivered := !stored.Orphaned
		if delivered {
			if err := replyToPane(stored, result.Selected); err != nil {
				reportError(err)
				delivered = false
			}
		}

		// The answer is still worth having when the pane it was owed to is not
		// there to take it, so it falls back to the terminal that asked for it.
		if !delivered {
			fmt.Println(strings.Join(result.Selected, "\n"))
		}
	}

	// Answered or cleared, it has been dealt with either way.
	if err := forgetNotification(stored.ID); err != nil {
		reportError(err)
	}
}

// Dispatch "show": relaunch tnotify inside the tmux layout the requested mode
// asks for, or print the notifications when this is that relaunched process.
//
//	--all       -> new pane on the side
//	--last      -> the stored notification, in its own overlay
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

	// --last reopens a stored notification rather than listing anything, so it
	// goes through the notification overlay, which sizes itself to the message.
	if mode == "last" {
		showLast(cfg)
		return
	}

	relaunchArgs := append([]string{"--internal", "show"}, args...)

	switch mode {
	case "all":
		err = pkg.TmuxPane(cfg.Sidepanel.Direction, cfg.Sidepanel.Width, relaunchArgs)
	default:
		var overlay pkg.Overlay
		if overlay, err = plainOverlay(cfg); err == nil {
			err = pkg.TmuxOverlay(overlay, nil, relaunchArgs)
		}
	}

	if err != nil {
		reportError(err)
	}
}
