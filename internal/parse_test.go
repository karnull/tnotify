// internal/parse_test.go

package internal

import (
	"slices"
	"testing"
)

func TestParseNotify(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantBody   string
		wantHead   string
		wantAuthor string
		wantOpts   []string
		wantCust   bool
		wantMult   bool
	}{
		{
			name:     "body only",
			args:     []string{"hello"},
			wantBody: "hello",
		},
		{
			name:     "body and heading",
			args:     []string{"hello", "--head", "hi"},
			wantBody: "hello",
			wantHead: "hi",
		},
		{
			name:       "explicit author is passed through as given",
			args:       []string{"hello", "--author", "nightly-backup.sh"},
			wantBody:   "hello",
			wantAuthor: "nightly-backup.sh",
		},
		{
			name:       "author can be any caller at all",
			args:       []string{"hello", "--author", "some-ci-runner"},
			wantBody:   "hello",
			wantAuthor: "some-ci-runner",
		},
		{
			name:     "options stop at the next flag",
			args:     []string{"pick", "--interactive", "one", "two", "--head", "hi"},
			wantBody: "pick",
			wantHead: "hi",
			wantOpts: []string{"one", "two"},
		},
		{
			name:     "custom and multiple",
			args:     []string{"pick", "--interactive", "one", "two", "--custom", "--multiple"},
			wantBody: "pick",
			wantOpts: []string{"one", "two"},
			wantCust: true,
			wantMult: true,
		},
		{
			name:     "custom with no options is allowed",
			args:     []string{"pick", "--interactive", "--custom"},
			wantBody: "pick",
			wantCust: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseNotify(test.args)
			if err != nil {
				t.Fatalf("parseNotify(%q) returned error: %v", test.args, err)
			}

			if got.Body != test.wantBody {
				t.Errorf("Body = %q, want %q", got.Body, test.wantBody)
			}
			if got.Head != test.wantHead {
				t.Errorf("Head = %q, want %q", got.Head, test.wantHead)
			}
			if !slices.Equal(got.Interactive, test.wantOpts) {
				t.Errorf("Interactive = %q, want %q", got.Interactive, test.wantOpts)
			}
			if got.Custom != test.wantCust {
				t.Errorf("Custom = %v, want %v", got.Custom, test.wantCust)
			}
			if got.Multiple != test.wantMult {
				t.Errorf("Multiple = %v, want %v", got.Multiple, test.wantMult)
			}
		})
	}
}

<<<<<<<< HEAD:pkg/internal_parse_test.go
========
// A flag show does not know is a slip worth stopping on, since opening the
// default overlay instead looks like the flag did something.
func TestShowModeRejectsUnknownFlags(t *testing.T) {
	for _, args := range [][]string{{"--nonsense"}, {"--everything"}, {"--all", "--nonsense"}} {
		if _, err := showMode(args); err == nil {
			t.Errorf("showMode(%q) succeeded, want an error", args)
		}
	}

	for _, args := range [][]string{nil, {"--all"}, {"--last"}} {
		if _, err := showMode(args); err != nil {
			t.Errorf("showMode(%q) returned error: %v", args, err)
		}
	}
}

// --timeout says how long the caller will hold, and asking for a limit is only
// ever asking to wait up to it.
func TestParseNotifyTimeout(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantWait    bool
		wantTimeout int
	}{
		{"no waiting at all by default", []string{"hi"}, false, 0},
		{"waiting with no limit", []string{"hi", "--wait"}, true, 0},
		{"a limit implies the wait", []string{"hi", "--timeout", "30"}, true, 30},
		{"zero is the no-limit default said out loud", []string{"hi", "--timeout", "0"}, true, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseNotify(test.args)
			if err != nil {
				t.Fatalf("parseNotify(%q) returned error: %v", test.args, err)
			}
			if got.Wait != test.wantWait || got.Timeout != test.wantTimeout {
				t.Errorf("Wait/Timeout = %v/%d, want %v/%d",
					got.Wait, got.Timeout, test.wantWait, test.wantTimeout)
			}
		})
	}
}

>>>>>>>> 4c5d5f9 (fixup! feat: read notify, show and clear command lines):internal/parse_test.go
func TestParseNotifyErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"custom without interactive", []string{"hi", "--custom"}},
		{"multiple without interactive", []string{"hi", "--multiple"}},
		{"interactive with no options", []string{"hi", "--interactive"}},
		{"retired interactive-custom", []string{"hi", "--interactive-custom", "a", "b"}},
		{"unknown flag", []string{"hi", "--nonsense"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseNotify(test.args); err == nil {
				t.Errorf("parseNotify(%q) succeeded, want an error", test.args)
			}
		})
	}
}
