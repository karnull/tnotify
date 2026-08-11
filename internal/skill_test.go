// internal/skill_test.go

package internal

import (
	"io"
	"strings"
	"testing"
)

//- Tests ------------------------------------------------------------------------------------------

// The note is the only place an agent's setup is written down, so every agent
// tnotify knows about has to turn up in it whole.
func TestSkillNoteCoversEveryAgent(t *testing.T) {
	note := skillNote(io.Discard)

	for _, agent := range agentSetups() {
		for _, want := range []string{agent.name, agent.skill, agent.allow, agent.entry} {
			// Copilot has no file to edit, so it has no entry to look for.
			if want == "" {
				continue
			}
			if !strings.Contains(note, want) {
				t.Errorf("skill note does not mention %q", want)
			}
		}
	}
}

// The note describes a redirection, and has to name the flag that actually
// writes the skill rather than the one that prints the note.
func TestSkillNoteSendsPeopleToTheExport(t *testing.T) {
	note := skillNote(io.Discard)

	if !strings.Contains(note, "tnotify --skill-export > SKILL.md") {
		t.Error("skill note does not say how to write the skill out")
	}
}

// Colour belongs on a terminal. Written anywhere else the note has to come out
// as plain text, so it can be piped and read.
func TestSkillNoteIsPlainWhenItIsNotGoingToATerminal(t *testing.T) {
	if note := skillNote(io.Discard); strings.Contains(note, "\x1b") {
		t.Error("skill note written somewhere that is not a terminal still carries escape sequences")
	}
}

// Each agent's paths are read as a column, so the labels they hang off have to
// leave them starting in the same place.
func TestSkillNoteLinesThePathsUp(t *testing.T) {
	note := skillNote(io.Discard)

	column := -1
	for line := range strings.SplitSeq(note, "\n") {
		label := ""
		switch {
		case strings.Contains(line, skillLabel+" "):
			label = skillLabel
		case strings.Contains(line, allowLabel+" "):
			label = allowLabel
		default:
			continue
		}

		at := strings.Index(line, label) + len(label) + setupGap
		if column < 0 {
			column = at
		}
		if at != column {
			t.Errorf("path on %q starts at column %d, want %d", line, at, column)
		}
	}

	if column < 0 {
		t.Fatal("skill note carries no labelled paths at all")
	}
}
