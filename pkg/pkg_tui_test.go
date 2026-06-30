// test/pkg_tui_test.go

package test

import (
	"testing"

	"github.com/karnull/tnotify/pkg"
)

// Hiding the footer takes rows off the minimum a notification can be drawn in,
// so the popup can be sized tighter around the message.
func TestMinHeightDropsWithHiddenFooter(t *testing.T) {
	withFooter := pkg.Notification{Colors: pkg.NotifyColors{Footer: "#ffffff"}}
	hidden := pkg.Notification{}

	if withFooter.MinHeight() <= hidden.MinHeight() {
		t.Errorf("MinHeight with footer = %d, want more than without (%d)",
			withFooter.MinHeight(), hidden.MinHeight())
	}
}

// The measured height must match, since the popup is sized from it before the
// TUI ever runs.
func TestHeightAccountsForHiddenFooter(t *testing.T) {
	body := "a short message"
	withFooter := pkg.Notification{Body: body, Colors: pkg.NotifyColors{Footer: "#ffffff"}}
	hidden := pkg.Notification{Body: body}

	// The blank row between the message and the footer, plus the footer itself.
	if got, want := withFooter.Height(40)-hidden.Height(40), 2; got != want {
		t.Errorf("footer costs %d rows, want %d", got, want)
	}
}
