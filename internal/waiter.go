// internal/waiter.go

package internal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/karnull/tnotify/pkg"
)

// How often a process waiting on an answer looks for one. It is idle either
// way; this only decides how soon it notices.
const waitPoll = 250 * time.Millisecond

//- Private Helpers --------------------------------------------------------------------------------

// Create the file a later answer is handed back through — the same shape as the
// one a popup reports its own outcome in — and return the path.
func newReplyFile() (string, error) {
	file, err := os.CreateTemp("", "tnotify-reply-*")
	if err != nil {
		return "", fmt.Errorf("creating reply file: %w", err)
	}
	name := file.Name()

	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("creating reply file: %w", err)
	}

	return name, nil
}

// Whether the process that asked for a notification is still there to take an
// answer. One that has since gone is no reason to hold the answer back; it
// goes to the pane instead.
func waiterAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Signal 0 asks whether the process exists without disturbing it. Being
	// told it is not ours to signal still says that it is there.
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// Hand an outcome back to the process still waiting on this notification, and
// report whether there was one to hand it to. Writing only into a reply file
// that is still empty keeps the disposal that follows an answer from writing
// over the answer that caused it.
func releaseWaiter(n storedNotification, result pkg.NotifyResult) bool {
	if n.Reply == "" || !waiterAlive(n.Waiter) {
		return false
	}

	written, err := os.ReadFile(n.Reply)
	if err != nil {
		// The waiter has taken its answer and cleaned up after itself.
		return false
	}
	if strings.TrimSpace(string(written)) != "" {
		return true
	}

	if err := os.WriteFile(n.Reply, []byte(encodeResult(result)), 0o600); err != nil {
		reportError(fmt.Errorf("handing back the answer: %w", err))
		return false
	}

	return true
}

// Block until the notification this reply file belongs to is answered or thrown
// away — in the side panel, or by "clear" — and report what became of it. A
// zero deadline is no deadline, which is the default: it waits as long as it
// takes.
func waitForAnswer(path string, deadline time.Time) pkg.NotifyResult {
	for {
		data, err := os.ReadFile(path)
		if err != nil {
			// Nothing left to wait on; treat it as thrown away.
			return pkg.NotifyResult{Action: pkg.ActionClear}
		}

		if strings.TrimSpace(string(data)) != "" {
			return decodeResult(string(data))
		}

		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return pkg.NotifyResult{Action: pkg.ActionTimeout}
		}

		time.Sleep(waitPoll)
	}
}
