# tnotify Structure

## Overview

tnotify is a single Go binary. Every command line goes through `cmd`, is parsed
and dispatched in `internal`, and is drawn through tmux and bubbletea in `pkg`.
To show a notification, tnotify relaunches itself inside a tmux popup, pane or
window with the `--internal` marker, and the copy outside waits for the answer.

## Top-Level Layout

| Path                  | Holds                                                      |
|-----------------------|------------------------------------------------------------|
| `cmd/main.go`         | Entry point: strips `--internal` and hands over to `internal` |
| `internal/`           | Command line parsing, dispatch, config, the notification store, waiting callers and help |
| `pkg/`                | tmux control and the bubbletea models for notifications and the side panel |
| `resources/`          | Embedded default config and agent skill                    |
| `version.go`, `VERSION` | The release version, embedded from `VERSION`             |
| `scripts/release.sh`  | Cross-compiles the release targets into `dist/`            |
| `.github/`            | Shared CI, and the workflow that keeps the Homebrew formula current |
| `AgentGuide.md`, `ScriptingGuide.md` | Guides for agents and for scripts           |

## Runtime Package

`internal` is the part of tnotify that knows what a command means:

| File         | Responsibility                                                    |
|--------------|-------------------------------------------------------------------|
| `args.go`    | Routes the command line to commands and flags                     |
| `parse.go`, `clear.go` | Parse `notify`, `show` and `clear` invocations          |
| `dispatch.go`| Opens popups and panes and hands the answer back to the caller    |
| `config.go`  | Reads `~/.config/tnotify/config.toml` over the embedded default   |
| `store.go`   | Keeps notifications set aside to be answered later                |
| `waiter.go`  | Holds a caller on the line for a later answer                     |
| `author.go`  | Names the caller that sent a notification                         |
| `overlay.go`, `panel.go` | Size and place popups, and drive the side panel       |
| `help.go`, `skill.go` | The help text and the agent skill instructions           |

`pkg` knows nothing about the command line: `tmux.go` drives tmux, and `tui.go`
and `panel.go` are the bubbletea models it draws.
