// internal/help_test.go

package internal

import (
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

)

//- Private Helpers --------------------------------------------------------------------------------

// The help as it would be drawn into a terminal of the given width. Rendered to
// a writer that is no terminal at all, so nothing is coloured and the text can
// be measured as it stands.
func helpAt(width int) string {
	return renderHelpAt(lipgloss.NewRenderer(io.Discard), width)
}

//- Tests ------------------------------------------------------------------------------------------

// Text is wrapped by hand rather than by lipgloss, since the descriptions hang
// off a column the terms share.
func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"short enough to stay put", "one two", 20, []string{"one two"}},
		{"broken on spaces", "one two three four", 9, []string{"one two", "three", "four"}},
		{"exactly the width", "abc def", 7, []string{"abc def"}},
		{"nothing at all", "   ", 10, []string{}},
		{
			// Cutting it in half would be worse than letting it stick out.
			name:  "a word wider than the column",
			text:  "a supercalifragilistic word",
			width: 6,
			want:  []string{"a", "supercalifragilistic", "word"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := wrapText(test.text, test.width)

			if len(got) != len(test.want) {
				t.Fatalf("wrapText(%q, %d) = %q, want %q", test.text, test.width, got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

// The help is laid out for the terminal it lands in, so nothing in it may run
// past the edge and wrap somewhere the indentation did not plan for.
func TestHelpFitsTheTerminal(t *testing.T) {
	for _, width := range []int{helpMinWidth, 64, 80, helpMaxWidth} {
		for i, line := range strings.Split(helpAt(width), "\n") {
			// A single word longer than the column it sits in is allowed to
			// stick out; there is nowhere else for it to go.
			if lipgloss.Width(line) > width && len(strings.Fields(line)) > 1 {
				t.Errorf("at %d columns, line %d is %d wide: %q", width, i, lipgloss.Width(line), line)
			}
		}
	}
}

// Descriptions line up in a column, which is the whole point of measuring the
// terms before drawing any of them.
func TestHelpAlignsDescriptions(t *testing.T) {
	help := helpAt(helpMaxWidth)

	// A row carrying a flag and its description, with the column the
	// description starts at as the capture.
	row := regexp.MustCompile(`^ +--?\S.*?\s{2,}(\S)`)

	column := -1
	for _, line := range strings.Split(help, "\n") {
		match := row.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}

		at := match[2]
		if column == -1 {
			column = at
		} else if at != column {
			t.Errorf("description starts at column %d, want %d: %q", at, column, line)
		}
	}

	if column == -1 {
		t.Fatal("found no rows with a description to line up")
	}
}

// Every command and flag the CLI takes has to be in the help, or it may as well
// not exist.
func TestHelpDocumentsEveryCommandAndFlag(t *testing.T) {
	help := helpAt(helpMaxWidth)

	for _, want := range []string{
		"--help", "--version", "--skill", "--config", "--defaults",
		"notify", "--head", "--author", "--interactive", "--custom", "--multiple",
		"show", "--all", "--last",
		"clear", "--tail",
		"TNOTIFY_AUTHOR", "TNOTIFY_COUNT", "XDG_STATE_HOME",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}

// Colour belongs on a terminal. Redirected to anything else the help has to
// come out as plain text, so it can be piped and read.
func TestHelpIsPlainWhenItIsNotGoingToATerminal(t *testing.T) {
	if help := helpAt(helpMaxWidth); strings.Contains(help, "\x1b") {
		t.Error("help written somewhere that is not a terminal still carries escape sequences")
	}
}
