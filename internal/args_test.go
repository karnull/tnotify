// internal/args_test.go

package internal

import (
	"testing"
)

//- Tests ------------------------------------------------------------------------------------------

// A command line naming something tnotify does not have has not been carried
// out, and says so in the only way a script reads.
func TestProcessArgsReportsUnknownCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"nothing at all", []string{}, exitSuccess},
		{"the help", []string{"--help"}, exitSuccess},
		{"a flag that does not exist", []string{"--nonesuch"}, exitFailure},
		{"a command that does not exist", []string{"nonesuch"}, exitFailure},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ProcessArgs(test.args, false); got != test.want {
				t.Errorf("ProcessArgs(%q) = %d, want %d", test.args, got, test.want)
			}
		})
	}
}

// A command line that could not be read has not been carried out either, and a
// script gating on "clear" has nothing but the status to tell it so.
func TestDispatchClearReportsUnreadableCommandLines(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"a flag that does not exist", []string{"--nonesuch"}, exitFailure},
		{"nothing to clear", []string{}, exitFailure},
		{"two ways to pick at once", []string{"--all", "--head", "2"}, exitFailure},
		{"a number that is not one", []string{"tuesday"}, exitFailure},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dispatchClear(test.args); got != test.want {
				t.Errorf("dispatchClear(%q) = %d, want %d", test.args, got, test.want)
			}
		})
	}
}

// "show" is told apart the same way, before it opens anything.
func TestDispatchShowReportsUnreadableCommandLines(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"a flag that does not exist", []string{"--nonesuch"}, exitFailure},
		{"two layouts at once", []string{"--all", "--last"}, exitFailure},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dispatchShow(test.args, false); got != test.want {
				t.Errorf("dispatchShow(%q) = %d, want %d", test.args, got, test.want)
			}
		})
	}
}
