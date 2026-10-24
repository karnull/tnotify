// internal/replay_test.go

package internal

import (
	"slices"
	"testing"
)

// A stored notification is shown again by rebuilding the command line it came
// from, so that command line has to parse back into the request that produced
// it. Anything lost here is a notification that comes back changed.
func TestStoredNotificationArgsRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		stored storedNotification
	}{
		{
			name:   "plain message",
			stored: storedNotification{Body: "build finished"},
		},
		{
			name:   "message with a heading",
			stored: storedNotification{Body: "staging is up", Head: "Deploy"},
		},
		{
			name: "options to pick from",
			stored: storedNotification{
				Body:        "promote the build?",
				Head:        "Deploy",
				Options:     []string{"yes", "no"},
				Interactive: true,
			},
		},
		{
			name: "several options and a text box",
			stored: storedNotification{
				Body:        "which environments?",
				Options:     []string{"staging", "production"},
				Interactive: true,
				Custom:      true,
				Multiple:    true,
			},
		},
		{
			name: "a text box and nothing else",
			stored: storedNotification{
				Body:        "name the release",
				Interactive: true,
				Custom:      true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := storedNotificationArgs(test.stored)

			req, err := parseNotify(args)
			if err != nil {
				t.Fatalf("parseNotify(%q) returned error: %v", args, err)
			}

			if req.Body != test.stored.Body {
				t.Errorf("Body = %q, want %q", req.Body, test.stored.Body)
			}
			if req.Head != test.stored.Head {
				t.Errorf("Head = %q, want %q", req.Head, test.stored.Head)
			}
			if !slices.Equal(req.Options, test.stored.Options) {
				t.Errorf("Options = %q, want %q", req.Options, test.stored.Options)
			}
			if req.Interactive != test.stored.Interactive {
				t.Errorf("Interactive = %v, want %v", req.Interactive, test.stored.Interactive)
			}
			if req.Custom != test.stored.Custom {
				t.Errorf("Custom = %v, want %v", req.Custom, test.stored.Custom)
			}
			if req.Multiple != test.stored.Multiple {
				t.Errorf("Multiple = %v, want %v", req.Multiple, test.stored.Multiple)
			}
		})
	}
}
