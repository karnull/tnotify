// internal/waiter_test.go

package internal

import (
	"os"
	"testing"
	"time"

	"github.com/karnull/tnotify/pkg"
)

//- Private Helpers --------------------------------------------------------------------------------

// A reply file of this test's own, cleaned up with it.
func replyFile(t *testing.T) string {
	t.Helper()

	path, err := newReplyFile()
	if err != nil {
		t.Fatalf("newReplyFile() returned error: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	return path
}

//- Tests ------------------------------------------------------------------------------------------

// The point of --wait: setting a notification aside is not answering it, and
// the answer given later reaches whoever asked rather than their pane.
func TestWaiterIsGivenALateAnswer(t *testing.T) {
	tempStore(t)
	reply := replyFile(t)

	id, err := rememberNotification(storedNotification{
		Body: "promote?", Reply: reply, Waiter: os.Getpid(),
	})
	if err != nil {
		t.Fatalf("rememberNotification() returned error: %v", err)
	}

	got := make(chan pkg.NotifyResult, 1)
	go func() { got <- waitForAnswer(reply, time.Time{}) }()

	// Nothing has happened yet, so it must still be waiting.
	select {
	case answered := <-got:
		t.Fatalf("waiter returned %q before anyone answered", answered.Action)
	case <-time.After(500 * time.Millisecond):
	}

	if err := answerNotification(id, []string{"yes"}); err != nil {
		t.Fatalf("answerNotification() returned error: %v", err)
	}

	select {
	case answered := <-got:
		if answered.Action != pkg.ActionSelect || len(answered.Selected) != 1 || answered.Selected[0] != "yes" {
			t.Errorf("waiter got %q %q, want a select of [yes]", answered.Action, answered.Selected)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter never woke")
	}

	if all, _ := allNotifications(); len(all) != 0 {
		t.Errorf("%d notifications left, want the answered one gone", len(all))
	}
}

// Throwing a notification away has to let go of anything waiting on it, or the
// caller would sit there on an answer that is never coming.
func TestClearingReleasesTheWaiter(t *testing.T) {
	tempStore(t)
	reply := replyFile(t)

	id, err := rememberNotification(storedNotification{Body: "promote?", Reply: reply, Waiter: os.Getpid()})
	if err != nil {
		t.Fatalf("rememberNotification() returned error: %v", err)
	}

	got := make(chan pkg.NotifyResult, 1)
	go func() { got <- waitForAnswer(reply, time.Time{}) }()

	if _, err := forgetNotifications([]int{id}); err != nil {
		t.Fatalf("forgetNotifications() returned error: %v", err)
	}

	select {
	case answered := <-got:
		if answered.Action != pkg.ActionClear {
			t.Errorf("waiter got %q, want %q", answered.Action, pkg.ActionClear)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter stranded after the notification was cleared")
	}
}

// An answer must not be handed to a process that has gone; it goes to the pane
// the notification came from instead.
func TestAnswerIsNotHandedToADeadWaiter(t *testing.T) {
	reply := replyFile(t)

	// A pid past the system maximum belongs to nothing.
	n := storedNotification{Reply: reply, Waiter: 4194303}
	if releaseWaiter(n, pkg.NotifyResult{Action: pkg.ActionSelect, Selected: []string{"yes"}}) {
		t.Error("released an answer to a process that is not there")
	}
}

// A caller that named a limit is let go when it runs out, and what it asked is
// left behind as a plain message with nobody owing it an answer.
func TestWaitingGivesUpAtTheDeadline(t *testing.T) {
	tempStore(t)
	reply := replyFile(t)

	id, err := rememberNotification(storedNotification{
		Body: "promote?", Options: []string{"yes", "no"}, Interactive: true,
		Reply: reply, Waiter: os.Getpid(),
	})
	if err != nil {
		t.Fatalf("rememberNotification() returned error: %v", err)
	}

	start := time.Now()
	if answered := waitForAnswer(reply, start.Add(500*time.Millisecond)); answered.Action != pkg.ActionTimeout {
		t.Fatalf("action = %q, want %q", answered.Action, pkg.ActionTimeout)
	}
	if waited := time.Since(start); waited < 400*time.Millisecond {
		t.Errorf("gave up after %v, want it to have waited the deadline out", waited)
	}

	if err := expireNotification(id); err != nil {
		t.Fatalf("expireNotification() returned error: %v", err)
	}

	stored, found, err := notificationByID(id)
	if err != nil || !found {
		t.Fatalf("notificationByID(%d) = found %v, error %v", id, found, err)
	}
	if !stored.Expired {
		t.Error("notification was not marked as timed out")
	}
	// Nothing should try to hand an answer to the caller that has gone.
	if stored.Reply != "" || stored.Waiter != 0 {
		t.Errorf("expired notification still points at %q/%d", stored.Reply, stored.Waiter)
	}
}

// A deadline of zero is no deadline, and is what a caller gets by default.
func TestZeroDeadlineWaitsIndefinitely(t *testing.T) {
	reply := replyFile(t)

	got := make(chan pkg.NotifyResult, 1)
	go func() { got <- waitForAnswer(reply, time.Time{}) }()

	select {
	case answered := <-got:
		t.Errorf("gave up with %q despite having no deadline", answered.Action)
	case <-time.After(900 * time.Millisecond):
	}
}
