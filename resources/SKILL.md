---
name: tnotify
description: Ask the user a question in a tmux popup instead of only in this pane, so it is seen from whichever tmux window they are actually looking at. Use for confirmations, choices and clarifications when the user may not be watching the agent's pane.
---

# tnotify

Your questions only appear in your own pane. `tnotify` draws them as a tmux popup over whatever window the user is looking at, blocks, and prints their answer to stdout.

## When

Use for: irreversible actions (force push, destructive migration, deletion, anything outward-facing); genuine ambiguity you cannot resolve from the repo; a long job finishing with nothing left to do.

Do not use for: anything you can decide yourself, anything trivial, anything already settled this session, or two popups in a row — batch with `--multiple`.

## Preflight

```bash
tnotify --check
```

Exit 0 means this pane is in a tmux session with a client attached to see the popup. Anything else — no tmux, or a session nobody is watching: ask in your own output instead.

## Invocation

Message first, before any flags — `--interactive` consumes every following non-flag word. Only the first operand is the message; later ones are dropped.

```bash
# confirm / choose
a=$(tnotify notify "Force-push to dev? Rewrites 6 commits." --head Confirm --author claude --interactive "yes" "no")

# free text (--custom alone = plain text box; with options = pick or type)
a=$(tnotify notify "Commit subject?" --head Clarify --author claude --interactive "fix: reject unknown flags" --custom)

# several answers at once; space to tick, one answer per line on stdout
a=$(tnotify notify "Which checks before pushing?" --head Clarify --author claude --interactive "tests" "lint" "vet" --multiple)

# information only; still blocks until dismissed, so background it
tnotify notify "Refactor done — 43 tests pass." --head Finished --author claude >/dev/null 2>&1 &
```

Parser rules: `--custom` and `--multiple` each require `--interactive`; `--interactive` requires ≥1 option unless `--custom`.

Always pass `--author`, or export `TNOTIFY_AUTHOR=claude/chatgpt/copilot` once, so your questions can be told apart and cleared as a group.

## Reading the answer

**Always exits 0, even on failure. Branch on empty stdout, never on `$?`.**

| User action                | stdout                | Kept   |
| -------------------------- | --------------------- | ------ |
| Answered, `[enter]`        | answer, one per line  | no     |
| Discarded, `[del]`         | empty                 | no     |
| Ignored, `[esc]`           | empty                 | queued |
| Timed out                  | empty                 | expired |
| No tmux / no client        | empty, stderr error   | no     |

Empty means *no answer*, never *no*. Always branch with a fallback that takes the safe path and states the assumption:

```bash
case "$a" in
  yes) ;;
  no)  ;;
  *)   # unanswered — do not guess at anything destructive
       ;;
esac
```

## Waiting

By default `[esc]` returns empty immediately (notification stays queued). `--wait` holds the call open until the question is answered later via `tnotify show --all` / `show --last`. `--timeout <seconds>` bounds that and implies `--wait`; 0 (default) is unbounded. The timeout covers the whole call; on expiry the question is kept as a grey `timeout` message, clearable only, and stdout is empty.

Do not use `timeout(1)` — absent on macOS, and exit 127 with empty stdout is indistinguishable from a declined answer. Use `--timeout`.

**Claude Code kills a foreground Bash call at 10 minutes**, whatever `--timeout` says. So only two correct uses of `--wait`:

- Background it when the user may take a while; you are re-invoked on exit.
  ```bash
  tnotify notify "…" --author claude --wait --interactive "yes" "no" > /tmp/answer.txt
  ```
  Send with `run_in_background`, read the file when it finishes.
- Stay under the ceiling (`--timeout 540` or less) only when the user is sitting there waiting for the popup right now.

Foreground `--wait` with no timeout = killed at 10 minutes, no answer, question still queued.

## Late answers

An `[esc]`-ignored question is kept and reachable via `tnotify show --all` (side panel of everything waiting) or `show --last`.

- With `--wait`: the answer returns on your still-open stdout.
- Without: your call already returned, so the answer is **typed into your pane's input box** (not executed); the user presses enter and it arrives as an ordinary conversation message. Treat the original call as unanswered, move on, and act on the answer if it turns up.

## Cleanup

```bash
tnotify clear --author claude   # drop only your queued questions
tnotify clear --all             # drop everything waiting
```

## Keys (to relay if asked)

`↑↓`/`j`/`k` move · `space` ticks under `--multiple` · `enter` answers ·
`esc` sets aside · `del` discards.
