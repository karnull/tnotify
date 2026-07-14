// internal/colors_test.go

package internal

import (
	"testing"
)

// A fully specified [colors] section is passed through as written.
func TestNotifyColors(t *testing.T) {
	var cfg Config
	cfg.Colors.Border = "#111111"
	cfg.Colors.Head = "#222222"
	cfg.Colors.Message = "#333333"
	cfg.Colors.Author = "#444444"
	cfg.Colors.Selection = "#555555"
	cfg.Colors.Footer = "#666666"

	colors := notifyColors(cfg)

	if colors.Selection != "#555555" {
		t.Errorf("Selection = %q, want #555555", colors.Selection)
	}
	if colors.Footer != "#666666" {
		t.Errorf("Footer = %q, want #666666", colors.Footer)
	}
	if colors.Author != "#444444" {
		t.Errorf("Author = %q, want #444444", colors.Author)
	}
}

// "<hidden>" is how the footer is switched off, which the TUI reads as an
// empty footer colour.
func TestNotifyColorsHiddenFooter(t *testing.T) {
	var cfg Config
	cfg.Colors.Border = "#111111"
	cfg.Colors.Footer = hiddenSetting

	if colors := notifyColors(cfg); colors.Footer != "" {
		t.Errorf("Footer = %q, want it emptied to hide the footer", colors.Footer)
	}
}

// A config written before selection/footer existed still draws sensibly.
func TestNotifyColorsFillsInOlderConfigs(t *testing.T) {
	var cfg Config
	cfg.Colors.Border = "#111111"
	cfg.Colors.Author = "#444444"

	colors := notifyColors(cfg)

	if colors.Selection != cfg.Colors.Author {
		t.Errorf("Selection = %q, want it to fall back to the author colour %q", colors.Selection, cfg.Colors.Author)
	}
	if colors.Footer != cfg.Colors.Border {
		t.Errorf("Footer = %q, want it to fall back to the border colour %q", colors.Footer, cfg.Colors.Border)
	}
}
