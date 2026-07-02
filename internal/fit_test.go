// internal/fit_test.go

package internal

import (
	"strings"
	"testing"

	"github.com/karnull/tnotify/pkg"
)

// The shortest popup a notification with a footer can be drawn in.
var heightFloor = pkg.Notification{}.MinHeight() + overlayBorder

func testNotification(body string) pkg.Notification {
	return pkg.Notification{
		Head:   "Heading",
		Body:   body,
		Colors: pkg.NotifyColors{Footer: "#ffffff"},
	}
}

// Width is spent before height: a message only widens the popup until it runs
// out of max_width, and only then makes it taller.
func TestFitOverlayWidensBeforeGrowingTaller(t *testing.T) {
	const (
		minWidth  = 20
		maxWidth  = 60
		minHeight = 7
		maxHeight = 40
	)

	short := testNotification("short")
	long := testNotification(strings.Repeat("word ", 60))

	shortWidth, shortHeight := fitOverlay(short, minWidth, maxWidth, minHeight, maxHeight)
	longWidth, longHeight := fitOverlay(long, minWidth, maxWidth, minHeight, maxHeight)

	// Both end up as short as their width budget allows — that is what makes
	// this width-first rather than height-first.
	for _, test := range []struct {
		name   string
		n      pkg.Notification
		height int
	}{
		{"short", short, shortHeight},
		{"long", long, longHeight},
	} {
		if want := test.n.Height(maxWidth-overlayBorder) + overlayBorder; test.height != want {
			t.Errorf("%s message height = %d, want the shortest possible %d", test.name, test.height, want)
		}
	}

	// A message that needs little room does not spend the width it is allowed.
	if shortWidth >= maxWidth {
		t.Errorf("short message widened to %d, want well under max_width %d", shortWidth, maxWidth)
	}

	// A long one spends more width, and only then ends up taller.
	if longWidth <= shortWidth {
		t.Errorf("long message width %d, want wider than the short message's %d", longWidth, shortWidth)
	}
	if longHeight <= shortHeight {
		t.Errorf("long message height %d, want taller than the short message's %d", longHeight, shortHeight)
	}
}

// Between those extremes the popup takes the narrowest width that keeps the
// message within min_height, rather than jumping straight to max_width.
func TestFitOverlayTakesNarrowestWidthThatFits(t *testing.T) {
	const (
		minWidth  = 20
		maxWidth  = 100
		minHeight = 9
		maxHeight = 40
	)

	n := testNotification("a message that needs a couple of lines to sit in")

	width, height := fitOverlay(n, minWidth, maxWidth, minHeight, maxHeight)

	if width == maxWidth {
		t.Fatalf("width went straight to max_width %d", maxWidth)
	}
	if height != minHeight {
		t.Errorf("height = %d, want min_height %d", height, minHeight)
	}

	// It fits at the chosen width...
	if got := n.Height(width-overlayBorder) + overlayBorder; got > minHeight {
		t.Errorf("message needs %d rows at width %d, over min_height %d", got, width, minHeight)
	}
	// ...and does not at anything narrower.
	if got := n.Height(width-1-overlayBorder) + overlayBorder; got <= minHeight {
		t.Errorf("message also fits at width %d, so %d is not the narrowest", width-1, width)
	}
}

// A max_height that cannot hold the message caps the popup rather than letting
// it run off the screen.
func TestFitOverlayRespectsMaxHeight(t *testing.T) {
	const maxHeight = 12

	n := testNotification(strings.Repeat("word ", 200))

	width, height := fitOverlay(n, 20, 40, 7, maxHeight)

	if height != maxHeight {
		t.Errorf("height = %d, want it capped at max_height %d", height, maxHeight)
	}
	if width != 40 {
		t.Errorf("width = %d, want max_width 40", width)
	}
}

// Bounds left at zero fall back to the smallest drawable popup and the size of
// the terminal.
func TestOverlayBoundsDefaults(t *testing.T) {
	var cfg Config

	minWidth, maxWidth, minHeight, maxHeight := overlayBounds(cfg, 100, 30, heightFloor)

	if minWidth != pkg.MinWidth+overlayBorder {
		t.Errorf("minWidth = %d, want %d", minWidth, pkg.MinWidth+overlayBorder)
	}
	if maxWidth != 100 {
		t.Errorf("maxWidth = %d, want the terminal width 100", maxWidth)
	}
	if minHeight != heightFloor {
		t.Errorf("minHeight = %d, want %d", minHeight, heightFloor)
	}
	if maxHeight != 30 {
		t.Errorf("maxHeight = %d, want the terminal height 30", maxHeight)
	}
}

// The terminal is the hard limit, whatever the config asks for.
func TestOverlayBoundsClampToTerminal(t *testing.T) {
	var cfg Config
	cfg.Overlay.MinWidth, cfg.Overlay.MaxWidth = 500, 400
	cfg.Overlay.MinHeight, cfg.Overlay.MaxHeight = 200, 100

	minWidth, maxWidth, minHeight, maxHeight := overlayBounds(cfg, 100, 30, heightFloor)

	if maxWidth != 100 || minWidth != 100 {
		t.Errorf("width bounds = %d..%d, want both clamped to 100", minWidth, maxWidth)
	}
	if maxHeight != 30 || minHeight != 30 {
		t.Errorf("height bounds = %d..%d, want both clamped to 30", minHeight, maxHeight)
	}
}
