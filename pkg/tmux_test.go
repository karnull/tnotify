// pkg/tmux_test.go

package pkg

import (
	"slices"
	"strings"
	"testing"
)

//- Tests ------------------------------------------------------------------------------------------

// The side panel runs down one edge of the window, so the only sides it can be
// put on are the two vertical ones.
func TestSplitFlagsTakesTheVerticalSides(t *testing.T) {
	tests := []struct {
		side string
		want []string
	}{
		{"left", []string{"-h", "-b", "-t", "{left}"}},
		{"right", []string{"-h", "-t", "{right}"}},
	}

	for _, test := range tests {
		t.Run(test.side, func(t *testing.T) {
			got, err := splitFlags(test.side)
			if err != nil {
				t.Fatalf("splitFlags(%q) returned error: %v", test.side, err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("splitFlags(%q) = %q, want %q", test.side, got, test.want)
			}
		})
	}
}

// A side the panel cannot be put on is worth saying so about: quietly opening
// it somewhere else would leave the config looking like it had been honoured.
func TestSplitFlagsRejectsAnythingElse(t *testing.T) {
	// Top and bottom are here because they used to work, which is exactly the
	// case a silent fallback would hide.
	for _, side := range []string{"top", "bottom", "", "middle", "Right"} {
		got, err := splitFlags(side)
		if err == nil {
			t.Errorf("splitFlags(%q) = %q, want an error", side, got)
			continue
		}
		if !strings.Contains(err.Error(), "left or right") {
			t.Errorf("splitFlags(%q) error = %q, want it to say which sides there are", side, err)
		}
	}
}
