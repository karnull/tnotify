// internal/args.go

package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/karnull/tnotify/pkg"
	"github.com/karnull/tnotify/resources"
)

// What tnotify writes about itself — the help, and the note that comes with the
// skill — is coloured to these rather than to the config, so that it reads the
// same whatever the notifications are painted in.
const (
	cliAccentColor = "#874BFD"
	cliTermColor   = "#04B575"
)

// What tnotify exits with. "notify" stands outside this and always succeeds: an
// unanswered question is not a failure, and a caller reading the answer off
// stdout under "set -e" should not be killed by one.
const (
	exitSuccess = 0
	exitFailure = 1
)

//- Private Helpers --------------------------------------------------------------------------------

// Report a failure to the user in the CLI's standard form.
func reportError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}

// Print the help documentation to stderr, laid out for the terminal it lands in.
func helpText() {
	fmt.Fprint(os.Stderr, renderHelp(os.Stderr))
}

// Say what the skill is and where it belongs. It goes to stderr so that the
// redirection it describes leaves the file with nothing but the skill in it,
// and leaves this on the screen where it can be read.
func skillNote(file *os.File) string {
	renderer := lipgloss.NewRenderer(file)
	command := renderer.NewStyle().Foreground(lipgloss.Color(cliTermColor))
	path := renderer.NewStyle().Foreground(lipgloss.Color(cliAccentColor))

	return "Run " + command.Render("tnotify --skill > SKILL.md") + " in your agent's skills directory,\n" +
		"e.g. " + path.Render("~/.claude/skills/tnotify/SKILL.md") + " for Claude Code.\n"
}

// Implements --skill: write the skill to stdout, where it can be redirected
// into a file, and how to install it to stderr, where it cannot.
func skillText() {
	fmt.Fprint(os.Stderr, skillNote(os.Stderr))
	fmt.Print(resources.Skill)
}

// Implements --check: report through the exit status alone whether tnotify has
// anywhere to draw, so a caller can gate on it without reading any output.
func checkUsable() int {
	if !pkg.TmuxHasClient() {
		return exitFailure
	}

	return exitSuccess
}

// Determine which show mode was requested, or "default" when none was given.
func showMode(args []string) (string, error) {
	mode := "default"

	// --all and --last each pick a different tmux layout, so only one of them
	// can be honoured.
	for _, arg := range args {
		switch arg {
		case "--all", "--last":
			flag := strings.TrimPrefix(arg, "--")
			if mode != "default" && mode != flag {
				return "", fmt.Errorf("--all and --last are mutually exclusive")
			}
			mode = flag

		default:
			// Quietly showing something else instead is worse than saying so,
			// most of all for a flag that used to work.
			if isFlag(arg) {
				return "", fmt.Errorf("unknown flag %s\nUsage: tnotify show [--all | --last]", arg)
			}
		}
	}

	return mode, nil
}

//- Public Calls -----------------------------------------------------------------------------------

// Route a command line to the command or flag it names, returning the status to
// exit with. isInternal is true when this process was relaunched by tnotify
// itself to do the work inside tmux.
func ProcessArgs(args []string, isInternal bool) int {
	if len(args) == 0 {
		helpText()
		return exitSuccess
	}

	switch cmd := args[0]; cmd {
	case "--help", "-h":
		helpText()
	case "--version", "-v":
		fmt.Println(strings.TrimSpace(resources.Version))
	case "--check":
		return checkUsable()
	case "--skill":
		skillText()
	case "--config":
		showConfig()
	case "--defaults":
		resetConfig()
	case "notify":
		// Always a success, whatever became of the question: see above.
		dispatchNotify(args[1:], isInternal)
	case "show":
		return dispatchShow(args[1:], isInternal)
	case "clear":
		return dispatchClear(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command or flag: %s\n", cmd)
		helpText()

		// A command line that named nothing tnotify has did not do what it was
		// asked, whatever the help printed after it says.
		return exitFailure
	}

	return exitSuccess
}
