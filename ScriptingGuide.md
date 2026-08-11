# Scripting guide

`tnotify` is built to sit inside other things: a build script that wants a
decision, a git hook that wants a subject line, a status line that wants a
count. This guide covers what a caller can rely on.

For setting a coding agent up against it, see [AgentGuide.md](AgentGuide.md).

## Check before sending

    if tnotify --check; then
        answer=$(tnotify notify "Deploy to production?" --head Confirm --interactive "yes" "no")
    else
        read -r -p "Deploy to production? [y/N] " answer
    fi

`--check` prints nothing and reports through its exit status alone: 0 when this
pane is inside tmux with a client attached to see a popup, non-zero otherwise. A
script that might run outside tmux — over ssh, from cron, in CI — should gate on
it and keep a fallback for the times there is no audience.

## Asking

The message comes first, before any flags: `--interactive` swallows every
non-flag word that follows it, so a message written after it becomes an option.

    # a plain notification
    tnotify notify "Build finished, 43 tests pass." --head Done

    # pick one option
    answer=$(tnotify notify "Deploy to production?" --head Confirm --interactive "yes" "no")

    # options plus a text box for anything not on the list
    subject=$(tnotify notify "Commit subject?" --interactive "fix: reject unknown flags" --custom)

    # a text box on its own
    subject=$(tnotify notify "Commit subject?" --interactive --custom)

    # several answers at once, one per line on stdout
    checks=$(tnotify notify "Which checks before pushing?" --interactive "tests" "lint" "vet" --multiple)

`--custom` and `--multiple` each require `--interactive`, and `--interactive`
requires at least one option unless `--custom` is given.

Multiple answers come back newline-separated, so they read straight into a loop:

    while IFS= read -r check; do
        run_check "$check"
    done <<< "$checks"

## Reading the answer

**`notify` always exits 0.** An unanswered question is not a failure, and a
caller reading the answer under `set -e` should not be killed by one. What
became of the question is on stdout:

| What happened | stdout | Kept in the queue |
| --- | --- | --- |
| Answered with `enter` | the answer, one per line | no |
| Discarded with `del` | empty | no |
| Set aside with `esc` | empty | yes |
| Timed out | empty | yes, as expired |
| No tmux, or no client attached | empty, error on stderr | no |

So branch on the text, not on the status, and treat empty as *no answer* rather
than as "no":

    case "$answer" in
        yes) deploy ;;
        no)  echo "Skipping deploy." ;;
        *)   echo "No answer — skipping deploy." ;;
    esac

The fallback should always take the path that can be undone.

## Waiting for a late answer

By default `esc` returns immediately with empty stdout and leaves the question
in the queue. `--wait` holds the call open instead, until the question is
answered later through `tnotify show --all` or `tnotify show --last`, and the
answer arrives on that still-open stdout.

    branch=$(tnotify notify "Which branch?" --interactive "dev" "main" --wait --timeout 300)

`--timeout <seconds>` bounds the wait and implies `--wait`; zero, the default,
is no limit. The timeout covers the whole call. On expiry the question stays in
the queue as an expired one — greyed out, clearable but no longer answerable —
and stdout is empty.

Use `--timeout` rather than wrapping the call in `timeout(1)`, which is not
installed on macOS and whose exit status cannot be told apart from a declined
answer.

For a wait that may run long, put the call in the background and collect the
answer from a file when it finishes:

    tnotify notify "Which branch?" --interactive "dev" "main" --wait > /tmp/branch &

## Working through the queue

    tnotify show --all      # side panel of everything waiting, to answer or clear
    tnotify show --last     # raise the one set aside most recently

    tnotify clear --all             # everything
    tnotify clear 1 2 4-5           # by number, ranges allowed
    tnotify clear --head 3          # the three that have waited longest
    tnotify clear --tail 3          # the three that arrived most recently
    tnotify clear --author deploy   # only what one sender sent

`--author` cannot be combined with numbers: one names a group, the other names
particular notifications.

## A status line count

Every store access publishes the number of waiting notifications into tmux's
global environment as `TNOTIFY_COUNT`, so a status line can draw it:

    set -g status-right "#{?#{!=:#{TNOTIFY_COUNT},0},#[fg=red] #{TNOTIFY_COUNT} waiting ,}%H:%M"

The count is only ever there to be read off a status line; a failure to publish
it never interrupts the command that caused it.

## Environment

| Variable | What it does |
| --- | --- |
| `TNOTIFY_AUTHOR` | Recorded as the sender when `--author` is not given. Falls back to the name of the calling script. |
| `TNOTIFY_COUNT` | Set by `tnotify` in tmux's global environment: how many notifications are waiting. |
| `XDG_STATE_HOME` | Where the queue is kept; defaults to `~/.local/state`. |
| `NO_COLOR` | Drops colour from what `tnotify` prints about itself. |

Naming the sender is worth doing in anything that sends more than one
notification, since it is what makes a queue readable and what `clear --author`
selects on:

    export TNOTIFY_AUTHOR=deploy

## Configuration

    tnotify --config     # print the config path, creating it from the defaults if there is none
    tnotify --defaults   # back up the current config and write the defaults

The config is TOML, and covers colours, the cursor markers, where the popup is
drawn and how large it may grow, and the side panel's side, width and clock.
`--config` prints a path, so it can be opened straight from a script:

    $EDITOR "$(tnotify --config)"
