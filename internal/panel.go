// internal/panel.go

package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/karnull/tnotify/pkg"
)

// Working out which panes have closed means asking tmux, so it is done on a
// slower cadence than the store reads that carry it.
const panelReapEvery = 5 * time.Second

// The clock and date settings the config names, and the Go time layouts they
// stand for. A setting that is none of these is taken to be a layout already,
// so a format the presets do not cover can still be had.
var (
	clockPresets = map[string]string{
		"24h": "15:04",
		"12h": "3:04 PM",
	}

	datePresets = map[string]string{
		"dmy": "02/01/2006",
		"mdy": "01/02/2006",
		"ymd": "2006-01-02",
		"iso": "2006-01-02",
	}
)

// What the panel falls back to when the config says nothing: a 24-hour clock,
// and a day-first date on anything that did not arrive today.
const (
	defaultClock = "24h"
	defaultDate  = "dmy"
)

//- Private Helpers --------------------------------------------------------------------------------

// Resolve one time setting to the Go layout it means. An empty setting is a
// config written before it existed, which gets the default rather than no time
// at all; "<hidden>" is how that part is asked for and left off.
func timeLayout(setting, fallback string, presets map[string]string) string {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		setting = fallback
	}

	if setting == hiddenSetting {
		return ""
	}

	if layout, ok := presets[strings.ToLower(setting)]; ok {
		return layout
	}

	return setting
}

// Translate the config's [sidepanel] clock and date into the layouts the panel
// draws each notification's arrival time with.
func panelClock(cfg Config) pkg.PanelClock {
	return pkg.PanelClock{
		Time: timeLayout(cfg.Sidepanel.Clock, defaultClock, clockPresets),
		Date: timeLayout(cfg.Sidepanel.Date, defaultDate, datePresets),
	}
}

// Build the call the panel reads notifications through, which also keeps the
// store's record of which panes have closed from going stale while the panel
// sits open.
func panelLoader(cfg Config) func() ([]pkg.PanelItem, error) {
	var reaped time.Time

	return func() ([]pkg.PanelItem, error) {
		if time.Since(reaped) >= panelReapEvery {
			reaped = time.Now()

			// Reaping only keeps a flag up to date for other commands to read:
			// an answer given here checks the pane itself on the way out, so a
			// reap that fails costs the panel nothing worth interrupting it
			// over — and there is nowhere to report it that is not the panel.
			reapOrphans()
		}

		stored, err := allNotifications()
		if err != nil {
			return nil, err
		}

		items := make([]pkg.PanelItem, 0, len(stored))
		for _, n := range stored {
			items = append(items, pkg.PanelItem{
				ID:           n.ID,
				Notification: storedNotificationView(n, cfg),
				Sent:         n.Sent,
			})
		}

		return items, nil
	}
}

// Deliver an answer the panel collected and let the notification go, leaving it
// in the store when the answer had nowhere to land.
func answerNotification(id int, selected []string) error {
	stored, found, err := notificationByID(id)
	if err != nil {
		return err
	}
	if !found {
		// Dealt with elsewhere between the panel drawing it and the user
		// picking; there is nothing left to answer, and nothing lost either.
		return nil
	}

	// Something still waiting on this notification is owed the answer before
	// anyone else: it asked for it and has not given up.
	if releaseWaiter(stored, pkg.NotifyResult{Action: pkg.ActionSelect, Selected: selected}) {
		return forgetNotification(id)
	}

	// An orphaned notification has no pane left to answer into. Trying anyway
	// would report the same thing in less familiar words.
	if stored.Orphaned {
		return fmt.Errorf("pane %s has closed", stored.Pane.ID)
	}

	if err := replyToPane(stored, selected); err != nil {
		return err
	}

	return forgetNotification(id)
}

// Draw the side panel, listing every waiting notification for the user to
// answer or throw away.
func runPanelTUI(cfg Config) {
	source := pkg.PanelSource{
		Load:   panelLoader(cfg),
		Answer: answerNotification,
		Delete: forgetNotification,
	}

	if err := pkg.RunPanelTUI(notifyColors(cfg), panelClock(cfg), source); err != nil {
		reportError(err)
	}
}
