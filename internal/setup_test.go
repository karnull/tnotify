// internal/setup_test.go

package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karnull/tnotify/resources"
	"github.com/tidwall/gjson"
)

//- Private Helpers --------------------------------------------------------------------------------

// Point the home directory at one of this test's own, so setting an agent up
// never touches the user's real settings.
func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// Write a file for a test, failing it if that cannot be done.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// Read a file for a test, failing it if that cannot be done.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

//- Tests ------------------------------------------------------------------------------------------

// An agent with no settings yet gets a file holding just the permission.
func TestAllowInJSONFileCreatesTheSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	added, err := allowInJSONFile(path, "Bash(tnotify:*)")
	if err != nil || !added {
		t.Fatalf("allowInJSONFile() = %v, %v, want it added", added, err)
	}

	allow := gjson.Get(readFile(t, path), "permissions.allow").Array()
	if len(allow) != 1 || allow[0].String() != "Bash(tnotify:*)" {
		t.Errorf("permissions.allow = %v, want just the tnotify rule", allow)
	}
}

// The user's own settings are theirs: everything already there stays, in the
// order it was written, beside the rule tnotify adds.
func TestAllowInJSONFileKeepsWhatIsThere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, `{
  "model": "opus",
  "permissions": {
    "allow": ["Bash(git:*)"],
    "deny": ["Bash(rm:*)"]
  },
  "env": {"A": "1"}
}
`, 0o600)

	if added, err := allowInJSONFile(path, "Bash(tnotify:*)"); err != nil || !added {
		t.Fatalf("allowInJSONFile() = %v, %v, want it added", added, err)
	}

	got := readFile(t, path)
	allow := gjson.Get(got, "permissions.allow").Array()
	if len(allow) != 2 || allow[0].String() != "Bash(git:*)" || allow[1].String() != "Bash(tnotify:*)" {
		t.Errorf("permissions.allow = %v, want the existing rule then tnotify's", allow)
	}
	if gjson.Get(got, "permissions.deny.0").String() != "Bash(rm:*)" || gjson.Get(got, "env.A").String() != "1" {
		t.Errorf("settings lost something while the rule was added:\n%s", got)
	}
	if model, env := strings.Index(got, `"model"`), strings.Index(got, `"env"`); model > env {
		t.Errorf("settings were reordered:\n%s", got)
	}

	// Settings can hold tokens, so a file kept private stays private.
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("settings mode = %v, want it kept at 0600", info.Mode().Perm())
	}
}

// Setting an agent up twice leaves it exactly as setting it up once did.
func TestAllowInJSONFileAddsTheRuleOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := allowInJSONFile(path, "Bash(tnotify:*)"); err != nil {
		t.Fatal(err)
	}
	first := readFile(t, path)

	added, err := allowInJSONFile(path, "Bash(tnotify:*)")
	if err != nil || added {
		t.Fatalf("second allowInJSONFile() = %v, %v, want nothing added", added, err)
	}
	if again := readFile(t, path); again != first {
		t.Errorf("settings changed on the second run:\n%s\nthen\n%s", first, again)
	}
}

// A settings file tnotify cannot read is not one it can safely rewrite.
func TestAllowInJSONFileLeavesBrokenSettingsAlone(t *testing.T) {
	for name, content := range map[string]string{
		"not json":              `{"model": "opus",`,
		"allow that is no list": `{"permissions": {"allow": "Bash(git:*)"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			writeFile(t, path, content, 0o644)

			if _, err := allowInJSONFile(path, "Bash(tnotify:*)"); err == nil {
				t.Error("allowInJSONFile() succeeded, want an error")
			}
			if got := readFile(t, path); got != content {
				t.Errorf("settings were rewritten to %q", got)
			}
		})
	}
}

// A rules file gets the rule on a line of its own, once, whether or not the
// file ended its last line.
func TestAllowAsLineInAppendsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules", "default.rules")
	writeFile(t, path, `prefix_rule(pattern = ["git"], decision = "allow")`, 0o644)
	rule := `prefix_rule(pattern = ["tnotify"], decision = "allow")`

	for run := 1; run <= 2; run++ {
		added, err := allowAsLineIn(path, rule)
		if err != nil {
			t.Fatal(err)
		}
		if added != (run == 1) {
			t.Errorf("run %d added = %v", run, added)
		}
	}

	want := `prefix_rule(pattern = ["git"], decision = "allow")` + "\n" + rule + "\n"
	if got := readFile(t, path); got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}

// The skill written out is the one the binary carries, and writing it again
// changes nothing.
func TestInstallSkillWritesTheEmbeddedSkill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", "tnotify", "SKILL.md")

	if wrote, err := installSkill(path); err != nil || !wrote {
		t.Fatalf("installSkill() = %v, %v, want it written", wrote, err)
	}
	if got := readFile(t, path); got != resources.Skill {
		t.Error("skill on disk is not the embedded one")
	}
	if wrote, err := installSkill(path); err != nil || wrote {
		t.Errorf("second installSkill() = %v, %v, want nothing written", wrote, err)
	}
}

// One command sets an agent up for good: the skill in its skills directory,
// and tnotify in its standing permissions.
func TestDispatchConfigSetsUpEachAgent(t *testing.T) {
	home := tempHome(t)

	if status := dispatchConfig([]string{"claude", "codex", "antigravity"}); status != exitSuccess {
		t.Fatalf("dispatchConfig() = %d, want %d", status, exitSuccess)
	}

	for _, agent := range []string{"claude", "codex", "antigravity"} {
		setup, _ := agentByID(agent)
		if got := readFile(t, filepath.Join(home, strings.TrimPrefix(setup.skill, "~/"))); got != resources.Skill {
			t.Errorf("%s: skill not written to %s", agent, setup.skill)
		}
		if got := readFile(t, filepath.Join(home, strings.TrimPrefix(setup.allow, "~/"))); !strings.Contains(got, setup.rule) {
			t.Errorf("%s: %s does not allow tnotify:\n%s", agent, setup.allow, got)
		}
	}
}

// Copilot has nowhere user-wide to keep the permission, so only its skill is
// written.
func TestDispatchConfigInstallsCopilotsSkill(t *testing.T) {
	home := tempHome(t)

	if status := dispatchConfig([]string{"copilot"}); status != exitSuccess {
		t.Fatalf("dispatchConfig() = %d, want %d", status, exitSuccess)
	}
	if got := readFile(t, filepath.Join(home, ".copilot", "skills", "tnotify", "SKILL.md")); got != resources.Skill {
		t.Error("copilot skill not written")
	}
}

// A name tnotify does not know stops the whole command before anything is
// written, so a typo does not leave the other agents half done.
func TestDispatchConfigRejectsUnknownAgentsFirst(t *testing.T) {
	home := tempHome(t)

	for _, args := range [][]string{nil, {"claude", "cluade"}} {
		if status := dispatchConfig(args); status != exitFailure {
			t.Errorf("dispatchConfig(%q) = %d, want %d", args, status, exitFailure)
		}
	}

	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Errorf("home holds %d entries after a rejected command, want none", len(entries))
	}
}

// Every agent the --skill note describes can also be set up by name.
func TestEveryAgentHasAConfigName(t *testing.T) {
	for _, agent := range agentSetups() {
		if agent.id == "" {
			t.Errorf("%s has no name for tnotify config", agent.name)
		}
		if agent.store != allowByFlag && agent.rule == "" {
			t.Errorf("%s has nowhere to write but no rule to write there", agent.name)
		}
	}
}
