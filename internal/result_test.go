// internal/result_test.go

package internal

import (
	"slices"
	"testing"

	"github.com/karnull/tnotify/pkg"
)

// The action has to survive the trip out of the popup, since it is what decides
// whether a notification is stored, kept waiting, or thrown away.
func TestResultRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		result pkg.NotifyResult
	}{
		{
			name:   "ignored",
			result: pkg.NotifyResult{Action: pkg.ActionIgnore},
		},
		{
			name:   "cleared",
			result: pkg.NotifyResult{Action: pkg.ActionClear},
		},
		{
			name:   "one option picked",
			result: pkg.NotifyResult{Action: pkg.ActionSelect, Selected: []string{"promote"}},
		},
		{
			name:   "several options picked",
			result: pkg.NotifyResult{Action: pkg.ActionSelect, Selected: []string{"staging", "production"}},
		},
		{
			name:   "an answer with a space in it",
			result: pkg.NotifyResult{Action: pkg.ActionSelect, Selected: []string{"roll back to v1"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := decodeResult(encodeResult(test.result))

			if got.Action != test.result.Action {
				t.Errorf("Action = %q, want %q", got.Action, test.result.Action)
			}
			if !slices.Equal(got.Selected, test.result.Selected) {
				t.Errorf("Selected = %q, want %q", got.Selected, test.result.Selected)
			}
		})
	}
}

// A popup that died before reporting anything leaves the file empty. That has
// to read as ignoring the notification, which keeps it, rather than as anything
// that would throw it away.
func TestDecodeEmptyResultIsIgnored(t *testing.T) {
	if got := decodeResult(""); got.Action != pkg.ActionIgnore {
		t.Errorf("decodeResult(\"\") = %q, want %q", got.Action, pkg.ActionIgnore)
	}
}
