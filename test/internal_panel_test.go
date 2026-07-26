// test/internal_panel_test.go

package test

import (
	"strings"
	"testing"

	"github.com/karnull/tnotify/internal"
)

//- Tests ------------------------------------------------------------------------------------------

// The panel lists every notification still waiting, in the order they arrived,
// so the oldest is at the top where it can be dealt with first.
func TestAllNotificationsListsThemOldestFirst(t *testing.T) {
	tempStore(t)

	remember(t, "first")
	remember(t, "second")
	remember(t, "third")

	all, err := internal.AllNotifications()
	if err != nil {
		t.Fatalf("allNotifications() returned error: %v", err)
	}

	bodies := make([]string, 0, len(all))
	for _, n := range all {
		bodies = append(bodies, n.Body)
	}

	if got, want := strings.Join(bodies, ","), "first,second,third"; got != want {
		t.Errorf("allNotifications() = %q, want %q", got, want)
	}
}

// An empty store is what the panel opens onto most of the time, and is not an
// error to read.
func TestAllNotificationsOnEmptyStore(t *testing.T) {
	tempStore(t)

	all, err := internal.AllNotifications()
	if err != nil {
		t.Fatalf("allNotifications() returned error: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("allNotifications() = %d notifications, want none", len(all))
	}
}

// The panel knows its notifications by id, and has to be able to get back to
// the pane a notification came from when the user answers it.
func TestNotificationByID(t *testing.T) {
	tempStore(t)

	remember(t, "first")
	remember(t, "second")

	wanted := last(t)

	got, found, err := internal.NotificationByID(wanted.ID)
	if err != nil {
		t.Fatalf("notificationByID() returned error: %v", err)
	}
	if !found {
		t.Fatalf("notificationByID(%d) found nothing", wanted.ID)
	}
	if got.Body != "second" {
		t.Errorf("notificationByID(%d) = %q, want %q", wanted.ID, got.Body, "second")
	}

	// Answering one that has since been dealt with elsewhere is how a stale
	// panel ends up here, and is not an error.
	if _, found, err := internal.NotificationByID(999); err != nil || found {
		t.Errorf("notificationByID(999) = found %v, error %v; want not found and no error", found, err)
	}
}

// A notification whose pane has closed has nowhere to send an answer, so it is
// left in the store to be read and thrown away rather than quietly consumed.
func TestAnsweringAnOrphanKeepsIt(t *testing.T) {
	tempStore(t)

	orphan := internal.StoredNotification{Body: "promote?", Orphaned: true}
	orphan.Pane.ID = "%3"

	if _, err := internal.RememberNotification(orphan); err != nil {
		t.Fatalf("rememberNotification() returned error: %v", err)
	}
	stored := last(t)

	err := internal.AnswerNotification(stored.ID, []string{"yes"})
	if err == nil {
		t.Fatal("answerNotification() on an orphan returned no error")
	}
	if !strings.Contains(err.Error(), "%3") {
		t.Errorf("answerNotification() error = %q, want it to name the pane", err)
	}

	if got := last(t); got.ID != stored.ID {
		t.Error("answerNotification() dropped a notification it could not deliver")
	}
}

// A notification answered from somewhere else between the panel drawing it and
// the user picking has nothing left to deliver, and nothing has been lost.
func TestAnsweringOneThatHasGone(t *testing.T) {
	tempStore(t)

	if err := internal.AnswerNotification(999, []string{"yes"}); err != nil {
		t.Errorf("answerNotification(999) returned error: %v", err)
	}
}
