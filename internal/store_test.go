// internal/store_test.go

package internal

import (
	"slices"
	"testing"
)

//- Private Helpers --------------------------------------------------------------------------------

// stubAnnounceCount catches the counts the store would publish to tmux, so a
// test never reaches the session it is being run in. The returned function puts
// the real announcer back.
//
// The dedupe is process-wide, so it starts over both ways round: a later test
// would otherwise see nothing published for a count an earlier one used.
func stubAnnounceCount(announce func(waiting int) error) func() {
	announceCount, publishedCount = announce, -1

	return func() { announceCount, publishedCount = tmuxAnnounceCount, -1 }
}

// Point the store at a directory of this test's own, so tests never touch the
// notifications the user is actually keeping.
func tempStore(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
<<<<<<<< HEAD:pkg/internal_store_test.go
========

	published := []int{}

	t.Cleanup(stubAnnounceCount(func(waiting int) error {
		published = append(published, waiting)
		return nil
	}))

	return &published
>>>>>>>> 64dd78e (fixup! feat: keep ignored notifications and answer them later):internal/store_test.go
}

// Keep a notification with the given body, failing the test if it cannot be.
func remember(t *testing.T, body string) {
	t.Helper()
<<<<<<< HEAD:internal/store_test.go
<<<<<<<< HEAD:pkg/internal_store_test.go
	if err := internal.RememberNotification(internal.StoredNotification{Body: body}); err != nil {
========
	if _, err := rememberNotification(storedNotification{Body: body}); err != nil {
>>>>>>>> 64dd78e (fixup! feat: keep ignored notifications and answer them later):internal/store_test.go
=======
	if _, err := internal.RememberNotification(internal.StoredNotification{Body: body}); err != nil {
>>>>>>> c02fdfb (feat: hold the caller on the line for a later answer):test/internal_store_test.go
		t.Fatalf("rememberNotification(%q) returned error: %v", body, err)
	}
}

// The last stored notification, failing the test if there is none.
func last(t *testing.T) storedNotification {
	t.Helper()

	stored, found, err := lastNotification()
	if err != nil {
		t.Fatalf("lastNotification() returned error: %v", err)
	}
	if !found {
		t.Fatal("lastNotification() found nothing, want a stored notification")
	}

	return stored
}

//- Tests ------------------------------------------------------------------------------------------

// An empty store is not an error: there simply is nothing waiting.
func TestLastNotificationOnEmptyStore(t *testing.T) {
	tempStore(t)

	_, found, err := lastNotification()
	if err != nil {
		t.Fatalf("lastNotification() returned error: %v", err)
	}
	if found {
		t.Error("lastNotification() found something in an empty store")
	}
}

// Everything a notification was sent with survives a trip through the store, so
// it can be shown again exactly as it first arrived.
func TestNotificationSurvivesTheStore(t *testing.T) {
	tempStore(t)

	sent := storedNotification{
		Author:      "deploy.sh",
		TrueAuthor:  "zsh",
		Head:        "Deploy finished",
		Body:        "staging is up",
		Options:     []string{"promote", "roll back"},
		Interactive: true,
		Custom:      true,
		Multiple:    true,
	}
	sent.Pane.ID = "%7"

<<<<<<< HEAD:internal/store_test.go
<<<<<<<< HEAD:pkg/internal_store_test.go
	if err := internal.RememberNotification(sent); err != nil {
========
	if _, err := rememberNotification(sent); err != nil {
>>>>>>>> 64dd78e (fixup! feat: keep ignored notifications and answer them later):internal/store_test.go
=======
	if _, err := internal.RememberNotification(sent); err != nil {
>>>>>>> c02fdfb (feat: hold the caller on the line for a later answer):test/internal_store_test.go
		t.Fatalf("rememberNotification() returned error: %v", err)
	}

	got := last(t)

	if got.Author != sent.Author || got.TrueAuthor != sent.TrueAuthor {
		t.Errorf("author = %q/%q, want %q/%q", got.Author, got.TrueAuthor, sent.Author, sent.TrueAuthor)
	}
	if got.Head != sent.Head || got.Body != sent.Body {
		t.Errorf("head/body = %q/%q, want %q/%q", got.Head, got.Body, sent.Head, sent.Body)
	}
	if !slices.Equal(got.Options, sent.Options) {
		t.Errorf("Options = %q, want %q", got.Options, sent.Options)
	}
	if !got.Interactive || !got.Custom || !got.Multiple {
		t.Errorf("interactive/custom/multiple = %v/%v/%v, want all true",
			got.Interactive, got.Custom, got.Multiple)
	}
	if got.Pane.ID != sent.Pane.ID {
		t.Errorf("Pane.ID = %q, want %q", got.Pane.ID, sent.Pane.ID)
	}
}

// "The last one" is the one ignored most recently, and forgetting it uncovers
// the one before rather than emptying the store.
func TestForgetNotificationUncoversThePrevious(t *testing.T) {
	tempStore(t)

	remember(t, "first")
	remember(t, "second")

	newest := last(t)
	if newest.Body != "second" {
		t.Fatalf("lastNotification() = %q, want the most recently ignored %q", newest.Body, "second")
	}

	if err := forgetNotification(newest.ID); err != nil {
		t.Fatalf("forgetNotification() returned error: %v", err)
	}

	if got := last(t); got.Body != "first" {
		t.Errorf("lastNotification() = %q, want %q once the newer one is dealt with", got.Body, "first")
	}
}

// Forgetting a notification that is not there is how answering one twice ends
// up, and is not worth an error.
func TestForgetUnknownNotification(t *testing.T) {
	tempStore(t)

	remember(t, "still waiting")

	if err := forgetNotification(999); err != nil {
		t.Fatalf("forgetNotification(999) returned error: %v", err)
	}

	if got := last(t); got.Body != "still waiting" {
		t.Errorf("lastNotification() = %q, want the store left alone", got.Body)
	}
}

<<<<<<<< HEAD:pkg/internal_store_test.go
========
// The count is what a tmux status line shows, so it has to follow the store up
// as notifications arrive and back down as they are dealt with.
func TestWaitingCountIsPublished(t *testing.T) {
	published := tempStore(t)

	remember(t, "first")
	remember(t, "second")

	newest := last(t)
	if err := forgetNotification(newest.ID); err != nil {
		t.Fatalf("forgetNotification() returned error: %v", err)
	}

	if want := []int{1, 2, 1}; !slices.Equal(*published, want) {
		t.Errorf("published counts = %v, want %v", *published, want)
	}
}

// Reading the store says nothing new, and the side panel reads it several times
// a second, so the same count must not be handed to tmux over and over.
func TestUnchangedCountIsNotRepublished(t *testing.T) {
	published := tempStore(t)

	remember(t, "waiting")

	for range 3 {
		if _, err := allNotifications(); err != nil {
			t.Fatalf("allNotifications() returned error: %v", err)
		}
	}

	if want := []int{1}; !slices.Equal(*published, want) {
		t.Errorf("published counts = %v, want %v", *published, want)
	}
}

// A tmux server that has been restarted has forgotten the count while the store
// still has it, so the first store access of a process publishes regardless.
func TestFirstStoreAccessPublishesTheCount(t *testing.T) {
	published := tempStore(t)

	if _, err := allNotifications(); err != nil {
		t.Fatalf("allNotifications() returned error: %v", err)
	}

	if want := []int{0}; !slices.Equal(*published, want) {
		t.Errorf("published counts = %v, want %v", *published, want)
	}
}

>>>>>>>> 64dd78e (fixup! feat: keep ignored notifications and answer them later):internal/store_test.go
// Ids are what a notification is removed by, so one that has been dealt with
// must not have its id handed to the next notification along.
func TestNotificationIDsAreNotReused(t *testing.T) {
	tempStore(t)

	remember(t, "first")
	first := last(t)

	if err := forgetNotification(first.ID); err != nil {
		t.Fatalf("forgetNotification() returned error: %v", err)
	}

	remember(t, "second")

	if second := last(t); second.ID == first.ID {
		t.Errorf("second notification reused id %d", second.ID)
	}
}
