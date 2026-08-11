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

// A setting given this value in the config switches that element off instead.
const hiddenSetting = "<hidden>"

// What a timed-out notification is drawn in when the config predates having a
// colour for it: grey enough to read as spent without disappearing.
const defaultExpiredColor = "#6C6C6C"

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
		Expired:   cfg.Colors.Expired,
	}

	// Configs written before these existed still get a sensible notification.
	if colors.Selection == "" {
		colors.Selection = colors.Author
	}
	if colors.Footer == "" {
		colors.Footer = colors.Border
	}
	if colors.Expired == "" {
		colors.Expired = defaultExpiredColor
	}

	// An empty footer colour is how the TUI is told to leave the footer out.
	if colors.Footer == hiddenSetting {
		colors.Footer = ""
	}

	return colors
}

// Resolve one configured marker: what the config asks for, the shipped default
// when it asks for nothing, and nothing at all when it asks for "<hidden>".
func cursorMark(set, shipped string) string {
	switch set {
	case "":
		return shipped
	case hiddenSetting:
		return ""
	}
	return set
}

// Translate the config's [cursor] into the markers the TUI draws the option
// under the cursor with. A config written before the section existed falls back
// to the shipped markers rather than one mark for both, so picking one option
// still looks different from ticking several.
func notifyCursor(cfg Config) pkg.NotifyCursor {
	shipped := defaultConfig()

	return pkg.NotifyCursor{
		Single:   cursorMark(cfg.Cursor.Single, shipped.Cursor.Single),
		Multiple: cursorMark(cfg.Cursor.Multiple, shipped.Cursor.Multiple),
	}
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
		Cursor:   notifyCursor(cfg),
	}
}

// Restore a stored notification to the form the TUI draws. Its colours come
// from the config as it stands now, not as it stood when it was sent.
func storedNotificationView(stored storedNotification, cfg Config) pkg.Notification {
	// An expired notification keeps everything it was sent with; the TUI is
	// what decides that there is no longer anything to pick from it.
	return pkg.Notification{
		Author:   stored.Author,
		Head:     stored.Head,
		Body:     stored.Body,
		Options:  stored.Options,
		Custom:   stored.Custom,
		Multiple: stored.Multiple,
		Expired:  stored.Expired,
		Colors:   notifyColors(cfg),
		Cursor:   notifyCursor(cfg),
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
func storeIgnored(req notifyRequest, n pkg.Notification, sent time.Time, reply string, expired bool) int {
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
		Expired:     expired,
		Pane:        pane,
	}

	// A caller that is staying on the line says where to reach it, so an answer
	// given days later still gets back to the thing that asked.
	if reply != "" {
		stored.Reply, stored.Waiter = reply, os.Getpid()
	}

	id, err := rememberNotification(stored)
	if err != nil {
		reportError(err)
	}

	return id
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

	// A zero timeout is no timeout, and is the default: the caller has not put
	// a limit on how long it will hold.
	var deadline time.Time
	if req.Timeout > 0 {
		deadline = sent.Add(time.Duration(req.Timeout) * time.Second)
	}

	result, err := answerFromOverlay(cfg, notification, args, deadline)
	if err != nil {
		reportError(err)
		return
	}

	switch result.Action {
	case pkg.ActionSelect:
		fmt.Println(strings.Join(result.Selected, "\n"))

	case pkg.ActionTimeout:
		// Nobody came to the popup in the time the caller had. What it asked is
		// kept, as a plain message to be read and thrown away.
		storeIgnored(req, notification, sent, "", true)

	case pkg.ActionIgnore:
		// An ignored notification is kept so it can be picked up later; a
		// cleared one was dismissed on purpose, and is not.
		if !req.Wait {
			storeIgnored(req, notification, sent, "", false)
			return
		}
		waitForLateAnswer(req, notification, sent, deadline)
	}
}

// Open the popup and report what came of it. Past the deadline the popup is
// taken off the screen and the notification reported as timed out, since a
// caller that named a limit is not held past it.
func answerFromOverlay(cfg Config, n pkg.Notification, args []string, deadline time.Time) (pkg.NotifyResult, error) {
	type outcome struct {
		result pkg.NotifyResult
		err    error
	}

	done := make(chan outcome, 1)
	go func() {
		result, err := openNotifyOverlay(cfg, n, args)
		done <- outcome{result, err}
	}()

	if deadline.IsZero() {
		got := <-done
		return got.result, got.err
	}

	select {
	case got := <-done:
		return got.result, got.err

	case <-time.After(time.Until(deadline)):
		// Closing the popup is what lets the process drawing it exit, so the
		// call above can be collected rather than left running.
		if err := pkg.TmuxClosePopup(); err != nil {
			reportError(err)
		}

		// Whatever it reports now is the popup being taken away, not something
		// the user did with it.
		<-done
		return pkg.NotifyResult{Action: pkg.ActionTimeout}, nil
	}
}

// Stay on the line for a notification the user set aside, until it is answered
// or thrown away wherever they get to it, or until the caller's time runs out.
func waitForLateAnswer(req notifyRequest, n pkg.Notification, sent, deadline time.Time) {
	reply, err := newReplyFile()
	if err != nil {
		reportError(err)
		storeIgnored(req, n, sent, "", false)
		return
	}
	defer os.Remove(reply)

	id := storeIgnored(req, n, sent, reply, false)

	switch answered := waitForAnswer(reply, deadline); answered.Action {
	case pkg.ActionSelect:
		if len(answered.Selected) > 0 {
			fmt.Println(strings.Join(answered.Selected, "\n"))
		}

	case pkg.ActionTimeout:
		// The notification outlives the caller that was waiting on it, as a
		// plain message nobody owes an answer to.
		if err := expireNotification(id); err != nil {
			reportError(err)
		}
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
		// Something still waiting on this notification is owed the answer
		// before anyone else: it asked for it and has not given up.
		delivered := releaseWaiter(stored, result)

		// An orphaned notification has no pane left to answer into; anything
		// else is worth trying, and may still turn out to have lost its pane.
		if !delivered && !stored.Orphaned {
			if err := replyToPane(stored, result.Selected); err != nil {
				reportError(err)
			} else {
				delivered = true
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
// Reports the status to exit with.
//
//	--all       -> new pane on the side, listing every waiting notification
//	--last      -> the stored notification, in its own overlay
//	(default)   -> new overlay
func dispatchShow(args []string, isInternal bool) int {
	mode, err := showMode(args)
	if err != nil {
		reportError(err)
		return exitFailure
	}

	cfg, err := LoadConfig()
	if err != nil {
		reportError(err)
		return exitFailure
	}

	// The tmux layout has been opened by now, and this is the process running
	// inside it, so what was asked for is drawn here rather than relaunched.
	if isInternal {
		if mode == "all" {
			runPanelTUI(cfg)
			return exitSuccess
		}

		fmt.Println(parseShow(args))
		return exitSuccess
	}

	// --last reopens a stored notification rather than listing anything, so it
	// goes through the notification overlay, which sizes itself to the message.
	if mode == "last" {
		showLast(cfg)
		return exitSuccess
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
		return exitFailure
	}

	return exitSuccess
}
