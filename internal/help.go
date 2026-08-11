// internal/help.go

package internal

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/karnull/tnotify/resources"
)

// helpEntry is one row of a section: a term — a command, a flag, an environment
// variable — and what it does. A term with no description stands on its own.
type helpEntry struct {
	term string
	desc string

	// Set for a flag belonging to the command above it, which is indented
	// under that command rather than listed in its own right.
	under bool
}

// helpSection is one titled block of the help.
type helpSection struct {
	title   string
	prose   string
	entries []helpEntry

	// Set where the entries are groups in their own right — commands and the
	// flags beneath them — and want air between them.
	spaced bool
}

const (
	// The help is wrapped to the terminal within these bounds: narrower and the
	// descriptions become a column of single words, wider and the eye loses the
	// line it is reading.
	helpMinWidth = 56
	helpMaxWidth = 96

	// Columns a term is indented by, and again for a flag beneath a command.
	helpIndent    = 2
	helpSubIndent = 4

	// Blank columns between the longest term and the descriptions.
	helpGap = 2

	// A term longer than this gets its description on the next line rather than
	// pushing every description on the page to the right.
	helpTermMax = 26
)

//- Private Helpers --------------------------------------------------------------------------------

// Everything the help has to say, in the order it says it.
func helpContent() (usage []string, sections []helpSection) {
	// Written as one line each; how they break up is the terminal's business.
	usage = []string{
		"tnotify [-h | -v | --check | --skill | --config | --defaults]",
		"tnotify notify <body> [--head <heading>] [--author <name>] [--wait] [--timeout <seconds>] [--interactive [<option>...] [--custom] [--multiple]]",
		"tnotify show [--all | --last]",
		"tnotify clear [--all | --head <n> | --tail <n> | <number>...] [--author <name>]",
	}

	sections = []helpSection{
		{title: "OPTIONS", entries: []helpEntry{
			{term: "-h, --help", desc: "Print this help and exit"},
			{term: "-v, --version", desc: "Print version information and exit"},
			{term: "--check", desc: "Print nothing; exit 0 when a client is attached to this session to see a popup"},
			{term: "--skill", desc: "Print the agent skill that teaches an agent to use tnotify"},
			{term: "--config", desc: "Print the config path, creating it from the defaults when there is none"},
			{term: "--defaults", desc: "Back up the current config and write the defaults"},
		}},

		{title: "COMMANDS", spaced: true, entries: []helpEntry{
			{term: "notify <body>", desc: "Send a notification, <body> being its message"},
			{term: "--head <heading>", under: true, desc: "Heading shown above the message"},
			{term: "--author <name>", under: true, desc: "Who sent it; defaults to $TNOTIFY_AUTHOR, else the calling script"},
			{term: "--interactive [<option>...]", under: true, desc: "Offer options to pick from"},
			{term: "--custom", under: true, desc: "Add a text box to type an answer in; requires --interactive"},
			{term: "--multiple", under: true, desc: "Select several options with space; requires --interactive"},
			{term: "--wait", under: true, desc: "Hold for an answer given later, however long that takes"},
			{term: "--timeout <seconds>", under: true, desc: "Give up waiting after this long; implies --wait. Zero, the default, is no limit"},

			{term: "show", desc: "Display the notification history"},
			{term: "--all", under: true, desc: "List every waiting notification in a side panel to answer or clear"},
			{term: "--last", under: true, desc: "Raise the most recently ignored notification to answer now"},

			{term: "clear [<number>...]", desc: "Throw notifications away without answering them, by number or range (e.g. 1 2 4-5)"},
			{term: "--all", under: true, desc: "Clear every waiting notification"},
			{term: "--head <n>", under: true, desc: "Clear the <n> that have been waiting longest"},
			{term: "--tail <n>", under: true, desc: "Clear the <n> that arrived most recently"},
			{term: "--author <name>", under: true, desc: "Clear only what this author sent; cannot be given alongside numbers"},
		}},

		{title: "ENVIRONMENT", entries: []helpEntry{
			{term: "TNOTIFY_AUTHOR", desc: "Name recorded as the sender when --author is not given"},
			{term: "TNOTIFY_COUNT", desc: "Set by tnotify in tmux's global environment: how many notifications are waiting. Read it in a status line as #{TNOTIFY_COUNT}"},
			{term: "XDG_STATE_HOME", desc: "Where ignored notifications are kept; defaults to ~/.local/state"},
		}},
	}

	return usage, sections
}

// The width the help should be laid out for: the terminal it is going to,
// held within the bounds that keep it readable.
func helpWidth(file *os.File) int {
	width, _, err := term.GetSize(file.Fd())
	if err != nil || width < 1 {
		// Not a terminal, so nobody is reading it at a width worth measuring.
		return helpMaxWidth
	}

	return min(max(width, helpMinWidth), helpMaxWidth)
}

// Break text into lines no wider than width, on spaces. A word longer than the
// width goes on a line of its own rather than being cut in half.
func wrapText(text string, width int) []string {
	lines := []string{}
	line := ""

	for word := range strings.FieldsSeq(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return lines
}

// Where the descriptions start: past the longest term that is not so long it
// would push them off the page on its own.
func helpDescColumn(sections []helpSection) int {
	longest := 0

	for _, section := range sections {
		for _, entry := range section.entries {
			if entry.desc == "" {
				continue
			}

			indent := helpIndent
			if entry.under {
				indent = helpSubIndent
			}

			if width := indent + len(entry.term); width <= helpTermMax && width > longest {
				longest = width
			}
		}
	}

	return longest + helpGap
}

// Draw one line of the synopsis: the command picked out, with its flags carried
// onto further lines indented clear of it when they will not all fit.
func helpUsageRow(line string, width int, termStyle, descStyle lipgloss.Style) []string {
	command, rest, _ := strings.Cut(line, " ")

	// A subcommand reads as part of the command; a list of flags does not.
	if word, tail, found := strings.Cut(rest, " "); found && !strings.HasPrefix(word, "[") && !strings.HasPrefix(word, "-") {
		command, rest = command+" "+word, tail
	}

	head := strings.Repeat(" ", helpIndent) + command
	if rest == "" {
		return []string{termStyle.Render(head)}
	}

	hang := len(head) + 1
	rows := []string{}

	for i, part := range wrapText(rest, max(width-hang, 20)) {
		if i == 0 {
			rows = append(rows, termStyle.Render(head)+" "+descStyle.Render(part))
			continue
		}
		rows = append(rows, strings.Repeat(" ", hang)+descStyle.Render(part))
	}

	return rows
}

// Draw one term and its description as a two-column row, giving the term a line
// of its own when it is too long to share.
func helpRow(entry helpEntry, descCol, width int, termStyle, descStyle lipgloss.Style) []string {
	indent := helpIndent
	if entry.under {
		indent = helpSubIndent
	}

	label := strings.Repeat(" ", indent) + entry.term
	if entry.desc == "" {
		return []string{termStyle.Render(label)}
	}

	rows := []string{}
	pad := descCol

	if len(label)+helpGap > descCol {
		rows = append(rows, termStyle.Render(label))
	} else {
		pad = descCol - len(label)
	}

	for i, line := range wrapText(entry.desc, max(width-descCol, 20)) {
		// The first line shares the term's row, unless the term took one of its
		// own above; everything after it lines up underneath.
		gap := strings.Repeat(" ", descCol)
		if i == 0 && pad != descCol {
			gap = strings.Repeat(" ", pad)
			rows = append(rows, termStyle.Render(label)+gap+descStyle.Render(line))
			continue
		}
		rows = append(rows, gap+descStyle.Render(line))
	}

	return rows
}

// Render the whole help, wrapped to the terminal it is going to.
func renderHelp(file *os.File) string {
	// The renderer is bound to the file the help is written to, so colour is
	// dropped when that is redirected even though the other stream is still a
	// terminal — and dropped altogether when NO_COLOR is set.
	return renderHelpAt(lipgloss.NewRenderer(file), helpWidth(file))
}

// Render the help into the given width, with the given renderer deciding how
// much colour the destination can take.
func renderHelpAt(renderer *lipgloss.Renderer, width int) string {
	usage, sections := helpContent()
	descCol := helpDescColumn(sections)

	name := renderer.NewStyle().Foreground(lipgloss.Color(cliAccentColor)).Bold(true)
	version := renderer.NewStyle().Foreground(lipgloss.Color(cliTermColor))
	heading := renderer.NewStyle().Foreground(lipgloss.Color(cliAccentColor))
	termStyle := renderer.NewStyle().Foreground(lipgloss.Color(cliTermColor))

	// Prose is left in whatever the terminal writes in, which is the one colour
	// certain to be readable against its background.
	descStyle := renderer.NewStyle()

	out := []string{
		name.Render("tnotify") + " " + version.Render(strings.TrimSpace(resources.Version)) +
			descStyle.Render("  —  tmux notification manager"),
		"",
		heading.Render("USAGE"),
	}

	for _, line := range usage {
		out = append(out, helpUsageRow(line, width, termStyle, descStyle)...)
	}

	for _, section := range sections {
		out = append(out, "", heading.Render(section.title))

		if section.prose != "" {
			for _, line := range wrapText(section.prose, width-helpIndent) {
				out = append(out, strings.Repeat(" ", helpIndent)+descStyle.Render(line))
			}
			continue
		}

		for i, entry := range section.entries {
			// Commands are groups in their own right, so they get air above
			// them; the flags beneath one stay tight against it.
			if section.spaced && !entry.under && i > 0 {
				out = append(out, "")
			}
			out = append(out, helpRow(entry, descCol, width, termStyle, descStyle)...)
		}
	}

	return strings.Join(out, "\n") + "\n"
}
