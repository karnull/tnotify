// internal/parse.go

package internal

import (
	"fmt"
	"strings"
)

const notifyUsage = `Usage: tnotify notify <body> [--head <heading>] [--author <name>] [--interactive [<option>...] [--custom] [--multiple]]`

// notifyRequest is a parsed "tnotify notify ..." invocation.
type notifyRequest struct {
	Body   string
	Head   string
	Author string

	Options []string

	// Whether --interactive was given at all, which the options alone do not
	// say: a bare --interactive alongside --custom carries none.
	Interactive bool

	Custom   bool
	Multiple bool
}

//- Private Helpers --------------------------------------------------------------------------------

// Whether an argument is a flag rather than a value or an operand.
func isFlag(arg string) bool {
	return strings.HasPrefix(arg, "--")
}

// Read the value following the flag at args[i], along with the index to carry
// on from. A flag left without a value is simply skipped.
func flagValue(args []string, i int) (value string, next int) {
	if i+1 < len(args) && !isFlag(args[i+1]) {
		return args[i+1], i + 2
	}
	return "", i + 1
}

// Collect the run of values following the flag at args[i], along with the index
// to carry on from. The run ends at the next flag or the end of the arguments.
func flagValues(args []string, i int) (values []string, next int) {
	for i++; i < len(args) && !isFlag(args[i]); i++ {
		values = append(values, args[i])
	}
	return values, i
}

// Check that the option-list flags were given a list to work on.
func validateInteractive(req notifyRequest) error {
	// --custom and --multiple only mean anything alongside a list of options.
	if (req.Custom || req.Multiple) && !req.Interactive {
		flag := "--custom"
		if req.Multiple {
			flag = "--multiple"
		}
		return fmt.Errorf("%s requires --interactive\n%s", flag, notifyUsage)
	}

	// A bare --custom needs no options, but a plain --interactive with none has
	// nothing to show.
	if req.Interactive && len(req.Options) == 0 && !req.Custom {
		return fmt.Errorf("--interactive needs at least one option, or --custom to type one\n%s", notifyUsage)
	}

	return nil
}

// Read a "tnotify notify ..." command line into the notification it describes.
func parseNotify(args []string) (notifyRequest, error) {
	if len(args) == 0 {
		return notifyRequest{}, fmt.Errorf("missing notification body\n%s", notifyUsage)
	}

	var req notifyRequest

	for i := 0; i < len(args); {
		switch arg := args[i]; arg {
		case "--head":
			req.Head, i = flagValue(args, i)

		case "--author":
			req.Author, i = flagValue(args, i)

		case "--interactive":
			req.Interactive = true
			req.Options, i = flagValues(args, i)

		case "--custom":
			req.Custom, i = true, i+1

		case "--multiple":
			req.Multiple, i = true, i+1

		case "--interactive-custom":
			// Replaced by "--interactive ... --custom"; say so rather than
			// silently downgrade to a notification with no options.
			return notifyRequest{}, fmt.Errorf("--interactive-custom has been replaced by --interactive with --custom\n%s", notifyUsage)

		default:
			if isFlag(arg) {
				return notifyRequest{}, fmt.Errorf("unknown flag %s\n%s", arg, notifyUsage)
			}
			// The first operand is the message; later ones are ignored.
			if req.Body == "" {
				req.Body = arg
			}
			i++
		}
	}

	if err := validateInteractive(req); err != nil {
		return notifyRequest{}, err
	}

	return req, nil
}

// Describe what a "tnotify --clear ..." command line would clear.
func parseClear(args []string) string {
	if len(args) == 0 {
		return "[CLEAR] Clearing notifications (default range)."
	}

	targets := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--all" {
			arg = "ALL"
		}
		targets = append(targets, arg)
	}

	return fmt.Sprintf("[CLEAR] Cleared target(s): %s.", strings.Join(targets, ", "))
}

// Describe what a "tnotify show ..." command line would display.
func parseShow(args []string) string {
	mode, err := showMode(args)
	if err != nil {
		return fmt.Sprintf("[SHOW] error: %v", err)
	}

	return fmt.Sprintf("[SHOW] Displaying notifications (%s).", mode)
}
