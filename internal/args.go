// internal/args.go

package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/karnull/tnotify/resources"
)

// What tnotify writes about itself — the help, and the note that comes with the
// skill — is coloured to these rather than to the config, so that it reads the
// same whatever the notifications are painted in.
const (
	cliAccentColor = "#874BFD"
	cliTermColor   = "#04B575"
)

//- Private Helpers --------------------------------------------------------------------------------

// Report a failure to the user in the CLI's standard form.
func reportError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}

// Print the help documentation (resources/help.txt) to stderr.
func helpText() {
	fmt.Fprint(os.Stderr, resources.HelpText)
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

// Route a command line to the command or flag it names. isInternal is true when
// this process was relaunched by tnotify itself to do the work inside tmux.
func ProcessArgs(args []string, isInternal bool) {
	if len(args) == 0 {
		helpText()
		return
	}

	switch cmd := args[0]; cmd {
	case "--help", "-h":
		helpText()
	case "--version", "-v":
		fmt.Println(strings.TrimSpace(resources.Version))
	case "--skill":
		skillText()
	case "--config":
		showConfig()
	case "--defaults":
		resetConfig()
	case "notify":
		dispatchNotify(args[1:], isInternal)
	case "show":
		dispatchShow(args[1:], isInternal)
	case "clear":
		dispatchClear(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command or flag: %s\n", cmd)
		helpText()
	}
}
