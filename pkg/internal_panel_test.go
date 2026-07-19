// internal/panel_test.go

package internal

import (
	"strings"
	"testing"
<<<<<<<< HEAD:pkg/internal_panel_test.go

	"github.com/karnull/tnotify/internal"
========
	"time"
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):internal/panel_test.go
)

//- Tests ------------------------------------------------------------------------------------------

// The panel lists every notification still waiting, in the order they arrived,
// so the oldest is at the top where it can be dealt with first.
func TestAllNotificationsListsThemOldestFirst(t *testing.T) {
	tempStore(t)

	remember(t, "first")
	remember(t, "second")
	remember(t, "third")

	all, err := allNotifications()
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

	all, err := allNotifications()
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

	got, found, err := notificationByID(wanted.ID)
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
	if _, found, err := notificationByID(999); err != nil || found {
		t.Errorf("notificationByID(999) = found %v, error %v; want not found and no error", found, err)
	}
}

// A notification whose pane has closed has nowhere to send an answer, so it is
// left in the store to be read and thrown away rather than quietly consumed.
func TestAnsweringAnOrphanKeepsIt(t *testing.T) {
	tempStore(t)

	orphan := storedNotification{Body: "promote?", Orphaned: true}
	orphan.Pane.ID = "%3"

<<<<<<<< HEAD:pkg/internal_panel_test.go
	if err := internal.RememberNotification(orphan); err != nil {
========
	if _, err := rememberNotification(orphan); err != nil {
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):internal/panel_test.go
		t.Fatalf("rememberNotification() returned error: %v", err)
	}
	stored := last(t)

	err := answerNotification(stored.ID, []string{"yes"})
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

	if err := answerNotification(999, []string{"yes"}); err != nil {
		t.Errorf("answerNotification(999) returned error: %v", err)
	}
}
<<<<<<<< HEAD:pkg/internal_panel_test.go
========

// The clock and date the config names have to reach the panel as the layouts it
// draws with, whether they were given as a convention or written out in full.
func TestPanelClockResolvesTheConfig(t *testing.T) {
	tests := []struct {
		name               string
		clock, date        string
		wantTime, wantDate string
	}{
		{name: "the shipped default", clock: "24h", date: "dmy", wantTime: "15:04", wantDate: "02/01/2006"},
		{name: "a twelve hour clock", clock: "12h", date: "mdy", wantTime: "3:04 PM", wantDate: "01/02/2006"},
		{name: "named however it is spelt", clock: "24H", date: "ISO", wantTime: "15:04", wantDate: "2006-01-02"},

		// A config predating these settings is not one asking to go without.
		{name: "a config that says nothing", wantTime: "15:04", wantDate: "02/01/2006"},

		// Anything the presets do not cover is a layout in its own right, which
		// is what makes a format they do not offer possible at all.
		{name: "a layout of the user's own", clock: "15:04:05", date: "Mon 2 Jan", wantTime: "15:04:05", wantDate: "Mon 2 Jan"},

		{name: "the date switched off", clock: "24h", date: hiddenSetting, wantTime: "15:04"},
		{name: "the time switched off", clock: hiddenSetting, date: "dmy", wantDate: "02/01/2006"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cfg Config
			cfg.Sidepanel.Clock, cfg.Sidepanel.Date = test.clock, test.date

			got := panelClock(cfg)
			if got.Time != test.wantTime || got.Date != test.wantDate {
				t.Errorf("panelClock(clock %q, date %q) = %q/%q, want %q/%q",
					test.clock, test.date, got.Time, got.Date, test.wantTime, test.wantDate)
			}
		})
	}
}

// The panel draws the time a notification arrived, so the store has to hand it
// over along with everything else.
func TestPanelLoaderCarriesTheArrivalTime(t *testing.T) {
	tempStore(t)

	sent := time.Date(2026, time.August, 17, 9, 41, 0, 0, time.Local)
	if _, err := rememberNotification(storedNotification{Body: "staging is up", Sent: sent}); err != nil {
		t.Fatalf("rememberNotification() returned error: %v", err)
	}

	items, err := panelLoader(defaultConfig())()
	if err != nil {
		t.Fatalf("panel loader returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("panel loader returned %d notifications, want 1", len(items))
	}

	if !items[0].Sent.Equal(sent) {
		t.Errorf("panel item sent at %v, want %v", items[0].Sent, sent)
	}
}
>>>>>>>> 5d0a602 (fixup! feat: list waiting notifications in a side panel):internal/panel_test.go
