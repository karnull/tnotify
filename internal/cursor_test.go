// internal/cursor_test.go

package internal

import (
	"testing"
)

// A [cursor] section is passed through as written, one marker for picking a
// single option and another for ticking several.
func TestNotifyCursor(t *testing.T) {
	var cfg Config
	cfg.Cursor.Single = "→"
	cfg.Cursor.Multiple = "+"

	cursor := notifyCursor(cfg)

	if cursor.Single != "→" {
		t.Errorf("Single = %q, want →", cursor.Single)
	}
	if cursor.Multiple != "+" {
		t.Errorf("Multiple = %q, want +", cursor.Multiple)
	}
}

// A config written before the section existed gets the markers tnotify ships
// with, which tell picking one option apart from ticking several.
func TestNotifyCursorFillsInOlderConfigs(t *testing.T) {
	shipped := defaultConfig()
	cursor := notifyCursor(Config{})

	if cursor.Single != shipped.Cursor.Single {
		t.Errorf("Single = %q, want the shipped %q", cursor.Single, shipped.Cursor.Single)
	}
	if cursor.Multiple != shipped.Cursor.Multiple {
		t.Errorf("Multiple = %q, want the shipped %q", cursor.Multiple, shipped.Cursor.Multiple)
	}
	if cursor.Single == cursor.Multiple {
		t.Errorf("cursor = %+v, want the two markers to differ", cursor)
	}
}

// "<hidden>" is how a marker is switched off, which the TUI reads as an empty
// one and draws nothing in its place.
func TestNotifyCursorHidden(t *testing.T) {
	var cfg Config
	cfg.Cursor.Single = hiddenSetting
	cfg.Cursor.Multiple = "+"

	cursor := notifyCursor(cfg)

	if cursor.Single != "" {
		t.Errorf("Single = %q, want it emptied to hide the marker", cursor.Single)
	}
	if cursor.Multiple != "+" {
		t.Errorf("Multiple = %q, want hiding one to leave the other alone", cursor.Multiple)
	}
}
