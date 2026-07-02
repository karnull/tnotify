// internal/overlay_test.go

package internal

import (
	"testing"
)

// A popup of this size in an 80x24 terminal leaves 24 spare columns and 15
// spare rows to distribute, which the expectations below are written against.
const (
	testWidth      = 56
	testHeight     = 9
	testTermWidth  = 80
	testTermHeight = 24
)

func TestPlaceOverlay(t *testing.T) {
	// Y is tmux's convention: the row one past the popup's bottom.
	tests := []struct {
		location string
		wantX    int
		wantY    int
	}{
		{"top-left", 0, 9},
		{"top-center", 12, 9},
		{"top-right", 24, 9},
		{"middle-left", 0, 16},
		{"middle-center", 12, 16},
		{"middle-right", 24, 16},
		{"bottom-left", 0, 24},
		{"bottom-center", 12, 24},
		{"bottom-right", 24, 24},

		// Aliases, spelling, casing and axis order.
		{"center", 12, 16},
		{"centre", 12, 16},
		{"top", 12, 9},
		{"left", 0, 16},
		{"BOTTOM-Right", 24, 24},
		{" top-left ", 0, 9},
		{"left-top", 0, 9},
		{"centre-top", 12, 9},
	}

	for _, test := range tests {
		got, err := placeOverlay(test.location, testWidth, testHeight, testTermWidth, testTermHeight)
		if err != nil {
			t.Errorf("placeOverlay(%q) returned error: %v", test.location, err)
			continue
		}

		if got.X != test.wantX || got.Y != test.wantY {
			t.Errorf("placeOverlay(%q) = (x=%d, y=%d), want (x=%d, y=%d)",
				test.location, got.X, got.Y, test.wantX, test.wantY)
		}

		if got.Width != testWidth || got.Height != testHeight {
			t.Errorf("placeOverlay(%q) resized the popup to %dx%d", test.location, got.Width, got.Height)
		}
	}
}

// The popup must stay entirely on screen, whichever corner it is placed in.
func TestPlaceOverlayStaysOnScreen(t *testing.T) {
	for _, location := range []string{"top-left", "top-right", "bottom-left", "bottom-right", "middle-center"} {
		got, err := placeOverlay(location, testWidth, testHeight, testTermWidth, testTermHeight)
		if err != nil {
			t.Fatalf("placeOverlay(%q) returned error: %v", location, err)
		}

		if got.X < 0 || got.X+got.Width > testTermWidth {
			t.Errorf("placeOverlay(%q) spans columns %d..%d, outside 0..%d",
				location, got.X, got.X+got.Width, testTermWidth)
		}

		// Y is one past the bottom, so the popup's top row is Y-Height.
		if got.Y-got.Height < 0 || got.Y > testTermHeight {
			t.Errorf("placeOverlay(%q) spans rows %d..%d, outside 0..%d",
				location, got.Y-got.Height, got.Y, testTermHeight)
		}
	}
}

func TestParseLocationInvalid(t *testing.T) {
	locations := []string{
		// Not positions at all.
		"upper-left", "centred", "nonsense", "top-middel",

		// One axis named twice — nearly always the axes written back to front,
		// which must not be silently resolved to one of them.
		"middle-top", "top-bottom", "left-right", "bottom-middle",
	}

	for _, location := range locations {
		if _, _, err := parseLocation(location); err == nil {
			t.Errorf("parseLocation(%q) succeeded, want an error", location)
		}
	}
}
