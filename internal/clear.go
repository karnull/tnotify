// internal/clear.go

package internal

import (
	"fmt"
	"strconv"
	"strings"
)

const clearUsage = `Usage: tnotify clear [--all | --head <n> | --tail <n> | <number>...] [--author <name>]`

// clearRequest is a parsed "tnotify clear ..." invocation. Author narrows what
// is being cleared; exactly one of the rest says how much of it goes.
//
// Positions are the numbers the side panel draws on its boxes, counting from 1
// with the notification that has been waiting longest.
type clearRequest struct {
	All    bool
	Head   int
	Tail   int
	Author string

	Positions []int
}

//- Private Helpers --------------------------------------------------------------------------------

// Read the count following --head or --tail, which has to be a positive number
// for the flag to mean anything.
func countValue(args []string, i int) (count, next int, err error) {
	flag := args[i]

	value, next := flagValue(args, i)
	if value == "" {
		return 0, next, fmt.Errorf("%s needs a number\n%s", flag, clearUsage)
	}

	count, err = strconv.Atoi(value)
	if err != nil || count < 1 {
		return 0, next, fmt.Errorf("%s needs a positive number, not %q\n%s", flag, value, clearUsage)
	}

	return count, next, nil
}

// Read one position or range of positions — "4" or "4-6" — into the positions
// it names.
func parsePositionRange(token string) ([]int, error) {
	first, last, isRange := strings.Cut(token, "-")

	from, err := strconv.Atoi(strings.TrimSpace(first))
	if err != nil || from < 1 {
		return nil, fmt.Errorf("%q is not a notification number\n%s", token, clearUsage)
	}

	to := from
	if isRange {
		if to, err = strconv.Atoi(strings.TrimSpace(last)); err != nil || to < 1 {
			return nil, fmt.Errorf("%q is not a range of notification numbers\n%s", token, clearUsage)
		}
	}

	// Given backwards, a range still says which notifications are meant.
	if to < from {
		from, to = to, from
	}

	positions := make([]int, 0, to-from+1)
	for position := from; position <= to; position++ {
		positions = append(positions, position)
	}

	return positions, nil
}

// Read an operand into the positions it names. Commas are allowed anywhere
// among the numbers, since "1, 2, 4-5" is how a person writes a list of them
// and the shell hands it over in whatever pieces the spaces left.
func parsePositions(arg string) ([]int, error) {
	positions := []int{}

	for token := range strings.SplitSeq(arg, ",") {
		if token = strings.TrimSpace(token); token == "" {
			continue
		}

		parsed, err := parsePositionRange(token)
		if err != nil {
			return nil, err
		}
		positions = append(positions, parsed...)
	}

	return positions, nil
}

// Check that the request says how much to clear, and says it only once.
func validateClear(req clearRequest) error {
	// Each of these picks a different set, so honouring two of them at once
	// would mean guessing which was meant.
	given := []string{}
	if req.All {
		given = append(given, "--all")
	}
	if req.Head > 0 {
		given = append(given, "--head")
	}
	if req.Tail > 0 {
		given = append(given, "--tail")
	}
	if len(req.Positions) > 0 {
		given = append(given, "notification numbers")
	}

	if len(given) > 1 {
		return fmt.Errorf("%s cannot be given together\n%s", strings.Join(given, " and "), clearUsage)
	}

	// --author on its own clears everything that author sent; with nothing at
	// all there is no telling what was meant, and clearing the lot on a guess
	// throws away notifications nobody has answered.
	if len(given) == 0 && req.Author == "" {
		return fmt.Errorf("nothing to clear: name the notifications, or use --all\n%s", clearUsage)
	}

	// The panel numbers what it is showing, all of it; narrowing that by author
	// first would leave those numbers pointing somewhere else.
	if len(req.Positions) > 0 && req.Author != "" {
		return fmt.Errorf("--author cannot be given with notification numbers, which count from the whole list\n%s", clearUsage)
	}

	return nil
}

// Read a "tnotify clear ..." command line into what it asks to be cleared.
func parseClear(args []string) (clearRequest, error) {
	var req clearRequest
	var err error

	for i := 0; i < len(args); {
		switch arg := args[i]; arg {
		case "--all":
			req.All, i = true, i+1

		case "--author":
			req.Author, i = flagValue(args, i)

		case "--head":
			if req.Head, i, err = countValue(args, i); err != nil {
				return clearRequest{}, err
			}

		case "--tail":
			if req.Tail, i, err = countValue(args, i); err != nil {
				return clearRequest{}, err
			}

		default:
			if isFlag(arg) {
				return clearRequest{}, fmt.Errorf("unknown flag %s\n%s", arg, clearUsage)
			}

			positions, err := parsePositions(arg)
			if err != nil {
				return clearRequest{}, err
			}
			req.Positions, i = append(req.Positions, positions...), i+1
		}
	}

	if err := validateClear(req); err != nil {
		return clearRequest{}, err
	}

	return req, nil
}

// Pick out the notifications a request names, from the whole list in the order
// the panel numbers it.
func selectForClearing(req clearRequest, waiting []storedNotification) ([]storedNotification, error) {
	// Positions count from the whole list, so they are read off before anything
	// narrows it. Out of range is worth saying so rather than quietly clearing
	// less than was asked for.
	if len(req.Positions) > 0 {
		seen := map[int]bool{}
		picked := []storedNotification{}

		for _, position := range req.Positions {
			if position > len(waiting) {
				return nil, fmt.Errorf("there is no notification %d: %d waiting", position, len(waiting))
			}
			// The same notification named twice, by a range and again on its
			// own, is still only cleared once.
			if !seen[position] {
				seen[position] = true
				picked = append(picked, waiting[position-1])
			}
		}

		return picked, nil
	}

	if req.Author != "" {
		matching := []storedNotification{}
		for _, n := range waiting {
			if strings.EqualFold(n.Author, req.Author) {
				matching = append(matching, n)
			}
		}
		waiting = matching
	}

	switch {
	case req.Head > 0:
		waiting = waiting[:min(req.Head, len(waiting))]
	case req.Tail > 0:
		waiting = waiting[max(len(waiting)-req.Tail, 0):]
	}

	return waiting, nil
}

// Dispatch "clear": throw away the notifications the command line names,
// without answering them, and report the status to exit with.
func dispatchClear(args []string) int {
	req, err := parseClear(args)
	if err != nil {
		reportError(err)
		return exitFailure
	}

	// A notification whose pane has closed has no title to give back, and
	// saying so is not worth a complaint on the way past: the notification is
	// cleared either way, and outside tmux there are no panes to ask about.
	reapOrphans()

	waiting, err := allNotifications()
	if err != nil {
		reportError(err)
		return exitFailure
	}

	picked, err := selectForClearing(req, waiting)
	if err != nil {
		reportError(err)
		return exitFailure
	}

	// Nothing to clear is not a failure: the notifications the command line
	// named are gone either way, which is what it asked for.
	if len(picked) == 0 {
		fmt.Println("no notifications to clear")
		return exitSuccess
	}

	ids := make([]int, 0, len(picked))
	for _, n := range picked {
		ids = append(ids, n.ID)
	}

	cleared, err := forgetNotifications(ids)
	if err != nil {
		reportError(err)
		return exitFailure
	}

	fmt.Printf("cleared %d of %d notifications\n", cleared, len(waiting))

	return exitSuccess
}
