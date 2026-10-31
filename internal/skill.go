// internal/skill.go

package internal

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/karnull/tnotify/resources"
)

// agentSetup is what one agent has to be told: where it keeps its skills, and
// where its standing permission to run tnotify unprompted is written. entry is
// what goes in that file, and is empty where allow is a command rather than a
// file to edit. id is the name "tnotify config" knows it by, and rule is the
// one permission entry that config writes, stored the way store says.
type agentSetup struct {
	id    string
	name  string
	skill string
	allow string
	entry string
	rule  string
	store allowStore
}

// How an agent keeps its standing permissions.
type allowStore int

const (
	// A JSON settings file, with the rule in its permissions.allow list.
	allowInJSON allowStore = iota

	// A rules file, with the rule on a line of its own.
	allowAsLine

	// Nowhere tnotify can write, so the agent has to be launched with a flag.
	allowByFlag
)

// The labels each agent's paths are written against. Both are laid out to the
// longer of the two, so the paths line up under each other.
const (
	skillLabel = "skill"
	allowLabel = "allow"
)

// Columns the labels are indented by, and the blank between a label and the
// path beside it.
const (
	setupIndent = 4
	setupGap    = 2
)

//- Private Helpers --------------------------------------------------------------------------------

// Where each agent looks for skills, and how each is told it may run tnotify
// without stopping to ask. Only the user-wide locations are given: an agent
// asked about once is being set up for good, not for one repository.
func agentSetups() []agentSetup {
	return []agentSetup{
		{
			id:    "claude",
			name:  "Claude Code",
			skill: "~/.claude/skills/tnotify/SKILL.md",
			allow: "~/.claude/settings.json",
			entry: `"permissions": { "allow": ["Bash(tnotify:*)"] }`,
			rule:  "Bash(tnotify:*)",
			store: allowInJSON,
		},
		{
			id:    "codex",
			name:  "Codex",
			skill: "~/.codex/skills/tnotify/SKILL.md",
			allow: "~/.codex/rules/default.rules",
			entry: `prefix_rule(pattern = ["tnotify"], decision = "allow")`,
			rule:  `prefix_rule(pattern = ["tnotify"], decision = "allow")`,
			store: allowAsLine,
		},
		{
			// Copilot only saves approvals per repository, in a file it rewrites
			// itself, so the flag is the one user-wide allowance there is.
			id:    "copilot",
			name:  "Copilot CLI",
			skill: "~/.copilot/skills/tnotify/SKILL.md",
			allow: `copilot --allow-tool 'shell(tnotify)'`,
			store: allowByFlag,
		},
		{
			id:    "antigravity",
			name:  "Antigravity",
			skill: "~/.gemini/config/skills/tnotify/SKILL.md",
			allow: "~/.gemini/antigravity-cli/settings.json",
			entry: `"permissions": { "allow": ["command(tnotify)"] }`,
			rule:  "command(tnotify)",
			store: allowInJSON,
		},
	}
}

// Say where the skill belongs and what each agent has to be told to use it,
// laid out for the terminal it is written to so that colour is dropped when it
// is redirected somewhere that cannot show it.
func skillNote(out io.Writer) string {
	// Coloured the way the help is: the agent's name reads as a heading, what
	// has to be typed or opened stands out under it, and the labels are left in
	// the terminal's own colour.
	name := lipgloss.NewStyle().Foreground(lipgloss.Color(cliAccentColor))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color(cliTermColor))
	plain := lipgloss.NewStyle()

	// Where the paths start, and so where an entry with no label of its own is
	// carried across to sit under them.
	gutter := strings.Repeat(" ", setupIndent+max(len(skillLabel), len(allowLabel))+setupGap)
	indent := strings.Repeat(" ", setupIndent)
	gap := strings.Repeat(" ", setupGap)

	note := []string{
		"Run " + value.Render("tnotify config <agent>") + " to have this done for you, with <agent> one of",
		strings.Join(agentIDs(), ", ") + ".",
		"",
		"By hand: run " + value.Render("tnotify --skill-export > SKILL.md") + " in your agent's skills directory,",
		"then let the agent run tnotify without asking each time:",
	}

	for _, agent := range agentSetups() {
		note = append(note,
			"",
			"  "+name.Render(agent.name),
			indent+plain.Render(skillLabel)+gap+value.Render(agent.skill),
			indent+plain.Render(allowLabel)+gap+value.Render(agent.allow),
		)

		if agent.entry != "" {
			note = append(note, gutter+value.Render(agent.entry))
		}
	}

	return forDestination(out, strings.Join(note, "\n")+"\n")
}

// Implements --skill: say where the skill goes and how to let an agent run
// tnotify unprompted. It prints nothing but that, so --skill-export is what a
// redirection into a file should be pointed at.
func skillText() {
	fmt.Print(skillNote(os.Stdout))
}

// Implements --skill-export: write the skill itself to stdout, where it can be
// redirected into the file the agent will read it from.
func skillExport() {
	fmt.Print(resources.Skill)
}
