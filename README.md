# tnotify

A small command line tool that puts a notification in a tmux popup, over
whatever window is on screen.

Anything that runs in a pane nobody is watching — a long build, a script that
needs a decision, a coding agent waiting on an answer — can only speak in its
own pane, and goes unread until someone comes back to it. `tnotify` draws the
message as a popup instead, waits for an answer, and prints it to stdout.

## What it does

- Shows a message in a tmux popup, sized and placed around the text.
- Asks questions: pick one option, tick several, or type a free text answer.
- Blocks until answered, so the caller can act on the reply.
- Keeps anything set aside, to be answered later rather than lost.
- Lists everything waiting in a side panel, where it can be answered or cleared.
- Publishes the number of waiting notifications to tmux, for a status line.
- Colours, popup position, side panel and cursor markers all come from a TOML config.

## Using it

Send a notification and carry on:

    tnotify notify "Build finished, 43 tests pass." --head Done

Ask a question and read the answer off stdout:

    answer=$(tnotify notify "Deploy to production?" --head Confirm --interactive "yes" "no")

Deal with what is waiting:

    tnotify show --all      # side panel of everything queued
    tnotify show --last     # raise the one set aside most recently
    tnotify clear --all     # throw the queue away unanswered

[ScriptingGuide.md](ScriptingGuide.md) covers the rest: gating on whether there
is anywhere to draw, what stdout says about how a question ended, waiting for
late answers, and reading the waiting count into a status line.

## Agent support

`tnotify` ships with a skill that teaches a coding agent to ask through the
popup rather than in its own pane, which is where its questions normally go
unread.

    tnotify --skill          # where the skill file goes, and how to allow it
    tnotify --skill-export   # the skill itself, to write into a skills directory

Questions carry an author, so several agents can be running at once and still be
told apart, and one of them cleared without touching the rest.

[AgentGuide.md](AgentGuide.md) covers installing the skill for Claude Code,
Codex, Copilot CLI and Antigravity, and what an agent has to be told about
answers it never receives.

## Configuration

    tnotify --config     # print the config path, creating it if there is none
    tnotify --defaults   # back up the current config and write the defaults

## Keys

`↑ ↓` or `j k` move · `space` ticks an option under `--multiple` · `enter`
answers · `esc` sets aside for later · `del` discards.
