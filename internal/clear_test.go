// internal/clear_test.go

package internal

import (
	"slices"
	"testing"
)

//- Private Helpers --------------------------------------------------------------------------------

// Keep a notification from a named author, failing the test if it cannot be.
func rememberFrom(t *testing.T, author, body string) {
	t.Helper()
<<<<<<<< HEAD:pkg/internal_clear_test.go
	if err := internal.RememberNotification(internal.StoredNotification{Author: author, Body: body}); err != nil {
========
	if _, err := rememberNotification(storedNotification{Author: author, Body: body}); err != nil {
>>>>>>>> b72b730 (fixup! feat: clear notifications by author, position or range):internal/clear_test.go
		t.Fatalf("rememberNotification(%q) returned error: %v", body, err)
	}
}

// The bodies of what a request would clear, in the order they were picked.
func clearing(t *testing.T, req clearRequest, waiting []storedNotification) []string {
	t.Helper()

	picked, err := selectForClearing(req, waiting)
	if err != nil {
		t.Fatalf("selectForClearing() returned error: %v", err)
	}

	bodies := make([]string, 0, len(picked))
	for _, n := range picked {
		bodies = append(bodies, n.Body)
	}

	return bodies
}

// Five waiting notifications from three authors, numbered 1 to 5 the way the
// panel would draw them.
func waitingNotifications() []storedNotification {
	return []storedNotification{
		{ID: 1, Author: "claude", Body: "one"},
		{ID: 2, Author: "deploy.sh", Body: "two"},
		{ID: 3, Author: "claude", Body: "three"},
		{ID: 4, Author: "backup", Body: "four"},
		{ID: 5, Author: "claude", Body: "five"},
	}
}

//- Tests ------------------------------------------------------------------------------------------

// The command line says which notifications go, in whichever of the several
// ways a person might reach for.
func TestParseClear(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantAll       bool
		wantAuthor    string
		wantHead      int
		wantTail      int
		wantPositions []int
	}{
		{name: "all", args: []string{"--all"}, wantAll: true},
		{name: "author", args: []string{"--author", "claude"}, wantAuthor: "claude"},
		{name: "head", args: []string{"--head", "10"}, wantHead: 10},
		{name: "tail", args: []string{"--tail", "10"}, wantTail: 10},
		{
			name:          "loose numbers and a range",
			args:          []string{"1", "2", "4-5", "10"},
			wantPositions: []int{1, 2, 4, 5, 10},
		},
		{
			// What the shell hands over when the numbers were written
			// "1, 2, 4-5, 10" with spaces after the commas.
			name:          "numbers written with commas",
			args:          []string{"1,", "2,", "4-5,", "10"},
			wantPositions: []int{1, 2, 4, 5, 10},
		},
		{
			name:          "numbers written with no spaces at all",
			args:          []string{"1,2,4-5,10"},
			wantPositions: []int{1, 2, 4, 5, 10},
		},
		{
			name:          "a range given backwards still names its numbers",
			args:          []string{"5-3"},
			wantPositions: []int{3, 4, 5},
		},
		{
			name:       "an author's oldest few",
			args:       []string{"--author", "claude", "--head", "2"},
			wantAuthor: "claude",
			wantHead:   2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseClear(test.args)
			if err != nil {
				t.Fatalf("parseClear(%q) returned error: %v", test.args, err)
			}

			if got.All != test.wantAll {
				t.Errorf("All = %v, want %v", got.All, test.wantAll)
			}
			if got.Author != test.wantAuthor {
				t.Errorf("Author = %q, want %q", got.Author, test.wantAuthor)
			}
			if got.Head != test.wantHead || got.Tail != test.wantTail {
				t.Errorf("Head/Tail = %d/%d, want %d/%d", got.Head, got.Tail, test.wantHead, test.wantTail)
			}
			if !slices.Equal(got.Positions, test.wantPositions) {
				t.Errorf("Positions = %v, want %v", got.Positions, test.wantPositions)
			}
		})
	}
}

// Clearing throws away notifications nobody has answered, so a command line
// that does not plainly say what to throw away must not be guessed at.
func TestParseClearErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"nothing at all", nil},
		{"all and a count together", []string{"--all", "--head", "2"}},
		{"two counts together", []string{"--head", "2", "--tail", "2"}},
		{"a count and numbers together", []string{"--tail", "2", "3"}},
		{"author narrows numbers that count from the whole list", []string{"--author", "claude", "3"}},
		{"a count of none", []string{"--head", "0"}},
		{"a count that is not a number", []string{"--head", "lots"}},
		{"a count left off", []string{"--tail"}},
		{"a number that is not one", []string{"first"}},
		{"a number counted from zero", []string{"0"}},
		{"an unknown flag", []string{"--newest"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseClear(test.args); err == nil {
				t.Errorf("parseClear(%q) succeeded, want an error", test.args)
			}
		})
	}
}

// The numbers name the notifications the panel drew under them.
func TestSelectForClearingByPosition(t *testing.T) {
	waiting := waitingNotifications()

	req, err := parseClear([]string{"1,", "3-4"})
	if err != nil {
		t.Fatalf("parseClear() returned error: %v", err)
	}

	if got, want := clearing(t, req, waiting), []string{"one", "three", "four"}; !slices.Equal(got, want) {
		t.Errorf("clearing = %q, want %q", got, want)
	}
}

// A notification named twice, by a range and again on its own, is one
// notification and is cleared once.
func TestSelectForClearingIgnoresRepeats(t *testing.T) {
	waiting := waitingNotifications()

	req, err := parseClear([]string{"2-3", "3", "2"})
	if err != nil {
		t.Fatalf("parseClear() returned error: %v", err)
	}

	if got, want := clearing(t, req, waiting), []string{"two", "three"}; !slices.Equal(got, want) {
		t.Errorf("clearing = %q, want %q", got, want)
	}
}

// Naming a notification that is not there means the list being read from was
// not the list as it stands, which is worth stopping over rather than clearing
// whichever of the numbers happened to land.
func TestSelectForClearingRejectsAPositionPastTheEnd(t *testing.T) {
	req, err := parseClear([]string{"2", "9"})
	if err != nil {
		t.Fatalf("parseClear() returned error: %v", err)
	}

	if _, err := selectForClearing(req, waitingNotifications()); err == nil {
		t.Error("selectForClearing() accepted a notification number past the end")
	}
}

// An author's notifications go together, however they capitalised their name.
func TestSelectForClearingByAuthor(t *testing.T) {
	waiting := waitingNotifications()

	req, err := parseClear([]string{"--author", "CLAUDE"})
	if err != nil {
		t.Fatalf("parseClear() returned error: %v", err)
	}

	if got, want := clearing(t, req, waiting), []string{"one", "three", "five"}; !slices.Equal(got, want) {
		t.Errorf("clearing = %q, want %q", got, want)
	}
}

// --head takes them from the end that has been waiting longest, --tail from the
// end that just arrived.
func TestSelectForClearingByCount(t *testing.T) {
	waiting := waitingNotifications()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"the oldest two", []string{"--head", "2"}, []string{"one", "two"}},
		{"the newest two", []string{"--tail", "2"}, []string{"four", "five"}},
		{"more than there are", []string{"--head", "50"}, []string{"one", "two", "three", "four", "five"}},
		{"an author's newest two", []string{"--author", "claude", "--tail", "2"}, []string{"three", "five"}},
		{"everything", []string{"--all"}, []string{"one", "two", "three", "four", "five"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := parseClear(test.args)
			if err != nil {
				t.Fatalf("parseClear(%q) returned error: %v", test.args, err)
			}

			if got := clearing(t, req, waiting); !slices.Equal(got, test.want) {
				t.Errorf("clearing = %q, want %q", got, test.want)
			}
		})
	}
}

// An author nobody has heard from has nothing waiting, which is not an error.
func TestSelectForClearingAnAuthorWithNothingWaiting(t *testing.T) {
	req, err := parseClear([]string{"--author", "nobody"})
	if err != nil {
		t.Fatalf("parseClear() returned error: %v", err)
	}

	if got := clearing(t, req, waitingNotifications()); len(got) != 0 {
		t.Errorf("clearing = %q, want nothing", got)
	}
}

// Several notifications go in one pass over the store, and it reports how many
// it actually took away.
func TestForgetNotifications(t *testing.T) {
	tempStore(t)

	rememberFrom(t, "claude", "one")
	rememberFrom(t, "deploy.sh", "two")
	rememberFrom(t, "claude", "three")

	all, err := allNotifications()
	if err != nil {
		t.Fatalf("allNotifications() returned error: %v", err)
	}

	// The first and last, plus one that was already dealt with.
	cleared, err := forgetNotifications([]int{all[0].ID, all[2].ID, 999})
	if err != nil {
		t.Fatalf("forgetNotifications() returned error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("forgetNotifications() cleared %d, want 2", cleared)
	}

	left, err := allNotifications()
	if err != nil {
		t.Fatalf("allNotifications() returned error: %v", err)
	}
	if len(left) != 1 || left[0].Body != "two" {
		t.Errorf("store holds %d notifications, want only %q", len(left), "two")
	}
}
