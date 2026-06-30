// pkg/tui_test.go

package pkg

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

)

//- Tests ------------------------------------------------------------------------------------------

// Hiding the footer takes rows off the minimum a notification can be drawn in,
// so the popup can be sized tighter around the message.
func TestMinHeightDropsWithHiddenFooter(t *testing.T) {
	withFooter := Notification{Colors: NotifyColors{Footer: "#ffffff"}}
	hidden := Notification{}

	if withFooter.MinHeight() <= hidden.MinHeight() {
		t.Errorf("MinHeight with footer = %d, want more than without (%d)",
			withFooter.MinHeight(), hidden.MinHeight())
	}
}

// The measured height must match, since the popup is sized from it before the
// TUI ever runs.
func TestHeightAccountsForHiddenFooter(t *testing.T) {
	body := "a short message"
	withFooter := Notification{Body: body, Colors: NotifyColors{Footer: "#ffffff"}}
	hidden := Notification{Body: body}

	// The blank row between the message and the footer, plus the footer itself.
	if got, want := withFooter.Height(40)-hidden.Height(40), 2; got != want {
		t.Errorf("footer costs %d rows, want %d", got, want)
	}
}

// The marker against the option under the cursor comes from the config, and a
// notification that ticks several options can be marked differently from one
// that picks a single answer.
func TestOptionRowsMarkTheCursor(t *testing.T) {
	tests := []struct {
		name     string
		multiple bool
		want     string
	}{
		{"one option picked", false, "> "},
		{"several ticked", true, "+ "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := newNotifyModel(Notification{
				Options:  []string{"yes", "no"},
				Multiple: test.multiple,
				Cursor:   NotifyCursor{Single: ">", Multiple: "+"},
			})

			rows := m.optionRows(40)

			if !strings.HasPrefix(rows[0], test.want) {
				t.Errorf("row under the cursor = %q, want it marked with %q", rows[0], test.want)
			}
			if !strings.HasPrefix(rows[1], strings.Repeat(" ", len(test.want))) {
				t.Errorf("row below the cursor = %q, want it indented clear of the marker", rows[1])
			}
		})
	}
}

// Whatever the marker is, the rows have to line up under one another: the blank
// standing in for it is measured rather than assumed, so a marker that takes
// two cells is stood in for by two spaces.
func TestOptionRowsLineUpUnderAnyMarker(t *testing.T) {
	for _, mark := range []string{"", ">", "❯", "🔥"} {
		m := newNotifyModel(Notification{
			Options: []string{"yes", "no"},
			Cursor:  NotifyCursor{Single: mark},
		})

		rows := m.optionRows(40)

		if got, want := lipgloss.Width(rows[1]), lipgloss.Width(rows[0]); got != want {
			t.Errorf("with marker %q, row below the cursor is %d wide, want %d: %q",
				mark, got, want, rows[1])
		}
	}
}

// A notification with no marker configured is drawn without one, leaving the
// row under the cursor to its colour alone.
func TestOptionRowsWithoutAMarker(t *testing.T) {
	m := newNotifyModel(Notification{Options: []string{"yes", "no"}})

	if rows := m.optionRows(40); !strings.HasPrefix(rows[0], "1.") {
		t.Errorf("row under the cursor = %q, want it to start with its number", rows[0])
	}
}
