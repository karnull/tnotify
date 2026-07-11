// internal/author_test.go

package internal

import (
	"strings"
	"testing"
)

// A caller that names itself is taken at its word, whoever it is.
func TestDefaultAuthorFromEnvironment(t *testing.T) {
	for _, name := range []string{"deploy-bot", "nightly-backup.sh", "some-tool", "a name with spaces"} {
		t.Setenv(authorEnvVar, name)

		if got := defaultAuthor(); got != name {
			t.Errorf("defaultAuthor() = %q, want %q", got, name)
		}
	}
}

// With nothing set it still names somebody rather than coming back empty.
func TestDefaultAuthorFallsBack(t *testing.T) {
	t.Setenv(authorEnvVar, "")

	if got := defaultAuthor(); got == "" {
		t.Error("defaultAuthor() is empty, want the calling process or user")
	}
}

// Whitespace-only is as good as unset.
func TestDefaultAuthorIgnoresBlankEnvironment(t *testing.T) {
	t.Setenv(authorEnvVar, "   ")

	if got := defaultAuthor(); got == "" || got == "   " {
		t.Errorf("defaultAuthor() = %q, want it to ignore a blank value", got)
	}
}

// A shell running a script is credited to the script, since that is the real
// caller; a shell doing anything else is credited to itself.
func TestScriptArg(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{"/bin/bash /Users/someone/nightly-backup.sh", "nightly-backup.sh"},
		{"bash ./deploy.sh --force", "deploy.sh"},
		{"/bin/sh -e /opt/ci/run-tests", "run-tests"},

		// Not a script: an interactive shell, or inline code.
		{"-zsh", ""},
		{"/bin/zsh", ""},
		{"bash -c tnotify notify hi", ""},
	}

	for _, test := range tests {
		if got := scriptArg(test.args); got != test.want {
			t.Errorf("scriptArg(%q) = %q, want %q", test.args, got, test.want)
		}
	}
}

// Whatever the parent turns out to be, it is reported as a bare command name
// rather than a path or a padded string.
func TestParentName(t *testing.T) {
	got := parentName()
	if got == "" {
		t.Skip("ps cannot be run here, so there is no parent to name")
	}

	if strings.ContainsAny(got, "/ \t\n") {
		t.Errorf("parentName() = %q, want a bare command name", got)
	}
}
