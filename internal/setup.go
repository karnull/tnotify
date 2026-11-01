// internal/setup.go

package internal

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/karnull/tnotify/resources"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

// The layout a JSON settings file is written back in once the rule is added.
// Keys keep their order; only the spacing is redrawn.
var settingsLayout = &pretty.Options{Width: 80, Indent: "  "}

//- Private Helpers --------------------------------------------------------------------------------

// The agent "tnotify config" knows by the given name.
func agentByID(id string) (agentSetup, bool) {
	for _, agent := range agentSetups() {
		if agent.id == id {
			return agent, true
		}
	}
	return agentSetup{}, false
}

// Every name "tnotify config" accepts, in the order the agents are listed.
func agentIDs() []string {
	ids := []string{}
	for _, agent := range agentSetups() {
		ids = append(ids, agent.id)
	}
	return ids
}

// Turn a path written from the home directory, as the agents' paths are, into
// one that can be opened.
func expandHome(path string) (string, error) {
	rest, fromHome := strings.CutPrefix(path, "~/")
	if !fromHome {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory: %w", err)
	}
	return filepath.Join(home, rest), nil
}

// Read a file that is allowed not to exist yet, keeping the mode it has so a
// rewrite does not loosen it.
func readExisting(path string) (data []byte, mode fs.FileMode, err error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}

	data, err = os.ReadFile(path)
	return data, info.Mode().Perm(), err
}

// Replace a file's contents by renaming a finished copy over it, so an agent
// reading its settings never sees one half written.
func writeAtomically(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tnotify-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}

	return os.Rename(temp.Name(), path)
}

// Write the skill where the agent reads it, reporting whether anything changed.
func installSkill(path string) (bool, error) {
	current, mode, err := readExisting(path)
	if err != nil {
		return false, err
	}
	if bytes.Equal(current, []byte(resources.Skill)) {
		return false, nil
	}

	return true, writeAtomically(path, []byte(resources.Skill), mode)
}

// Add rule to the permissions.allow list of a JSON settings file, creating the
// file or the list when there is none. A file that is not JSON is left alone
// rather than guessed at.
func allowInJSONFile(path, rule string) (bool, error) {
	data, mode, err := readExisting(path)
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	if !gjson.ValidBytes(data) {
		return false, errors.New("not valid JSON, so it was left as it is")
	}

	allow := gjson.GetBytes(data, "permissions.allow")
	if allow.Exists() && !allow.IsArray() {
		return false, errors.New("permissions.allow is not a list, so it was left as it is")
	}
	for _, entry := range allow.Array() {
		if entry.String() == rule {
			return false, nil
		}
	}

	data, err = sjson.SetBytes(data, "permissions.allow.-1", rule)
	if err != nil {
		return false, err
	}

	return true, writeAtomically(path, pretty.PrettyOptions(data, settingsLayout), mode)
}

// Add rule on a line of its own to a rules file, creating it when there is none.
func allowAsLineIn(path, rule string) (bool, error) {
	data, mode, err := readExisting(path)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == rule {
			return false, nil
		}
	}

	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	data = append(data, rule+"\n"...)

	return true, writeAtomically(path, data, mode)
}

// Install the skill for one agent and give it standing permission to run
// tnotify, returning what was done, laid out the way the --skill note is.
func setUpAgent(agent agentSetup) (string, error) {
	name := lipgloss.NewStyle().Foreground(lipgloss.Color(cliAccentColor))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color(cliTermColor))

	indent := strings.Repeat(" ", setupIndent)
	gap := strings.Repeat(" ", setupGap)
	label := func(text string) string {
		return indent + fmt.Sprintf("%-*s", max(len(skillLabel), len(allowLabel)), text) + gap
	}

	report := []string{name.Render(agent.name)}

	skillPath, err := expandHome(agent.skill)
	if err != nil {
		return "", err
	}
	wrote, err := installSkill(skillPath)
	if err != nil {
		return "", fmt.Errorf("writing the skill to %s: %w", agent.skill, err)
	}
	action := "already up to date at "
	if wrote {
		action = "wrote "
	}
	report = append(report, label(skillLabel)+action+value.Render(agent.skill))

	if agent.store == allowByFlag {
		report = append(report, label(allowLabel)+"has no user-wide setting; launch it with "+value.Render(agent.allow))
		return strings.Join(report, "\n") + "\n", nil
	}

	allowPath, err := expandHome(agent.allow)
	if err != nil {
		return "", err
	}
	added := false
	switch agent.store {
	case allowInJSON:
		added, err = allowInJSONFile(allowPath, agent.rule)
	case allowAsLine:
		added, err = allowAsLineIn(allowPath, agent.rule)
	}
	if err != nil {
		return "", fmt.Errorf("allowing tnotify in %s: %w", agent.allow, err)
	}

	action = " already allowed in "
	if added {
		action = " added to "
	}
	report = append(report, label(allowLabel)+value.Render(agent.rule)+action+value.Render(agent.allow))

	return strings.Join(report, "\n") + "\n", nil
}

//- Public Calls -----------------------------------------------------------------------------------

// Dispatch "config": set each named agent up to use tnotify. Every name is
// checked before anything is written, so a typo does not leave half the list
// done.
func dispatchConfig(args []string) int {
	if len(args) == 0 {
		reportError(fmt.Errorf("config needs an agent to set up: %s", strings.Join(agentIDs(), ", ")))
		return exitFailure
	}

	agents := []agentSetup{}
	for _, id := range args {
		agent, known := agentByID(id)
		if !known {
			reportError(fmt.Errorf("unknown agent %q; tnotify can set up %s", id, strings.Join(agentIDs(), ", ")))
			return exitFailure
		}
		if !slices.ContainsFunc(agents, func(seen agentSetup) bool { return seen.id == id }) {
			agents = append(agents, agent)
		}
	}

	status := exitSuccess
	for i, agent := range agents {
		report, err := setUpAgent(agent)
		if err != nil {
			reportError(fmt.Errorf("%s: %w", agent.name, err))
			status = exitFailure
			continue
		}

		if i > 0 {
			fmt.Println()
		}
		fmt.Print(forDestination(os.Stdout, report))
	}

	return status
}
