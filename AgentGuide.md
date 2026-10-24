# Agent guide

A coding agent asks its questions in its own pane, which is the one pane the
user is least likely to be looking at. `tnotify` moves the question to a popup
over whatever window is on screen, blocks, and hands the answer back on stdout.

This guide is about setting an agent up and about the behaviour an agent has to
account for. For the shell-level contract — exit statuses, the meaning of an
empty answer, waiting for late answers — see
[ScriptingGuide.md](ScriptingGuide.md).

## The skill

The skill is the agent-facing half of `tnotify`: it tells an agent when to reach
for the popup, how to phrase the call, how to read an answer back, and what an
empty answer means, so silence is not taken for consent.

    tnotify --skill-export > SKILL.md

writes it. `tnotify --skill` prints the same table of paths given below, for
whichever agents are installed.

## Where it goes

Each agent reads skills from its own directory, and each needs standing
permission before it will run `tnotify` without stopping to ask every time. The
user-wide locations are the ones worth setting: an agent set up once is set up
for good, not for one repository.

| Agent | Skill | Permission |
| --- | --- | --- |
| Claude Code | `~/.claude/skills/tnotify/SKILL.md` | `~/.claude/settings.json` |
| Codex | `~/.codex/skills/tnotify/SKILL.md` | `~/.codex/rules/default.rules` |
| Copilot CLI | `~/.copilot/skills/tnotify/SKILL.md` | passed as a flag |
| Antigravity | `~/.gemini/config/skills/tnotify/SKILL.md` | `~/.gemini/antigravity-cli/settings.json` |

What each permission file wants:

    # Claude Code — ~/.claude/settings.json
    "permissions": { "allow": ["Bash(tnotify:*)"] }

    # Codex — ~/.codex/rules/default.rules
    prefix_rule(pattern = ["tnotify"], decision = "allow")

    # Antigravity — ~/.gemini/antigravity-cli/settings.json
    "permissions": { "allow": ["command(tnotify)"] }

Copilot CLI writes its own permissions file and has no documented syntax for
editing by hand, so the allowance is given at launch:

    copilot --allow-tool 'shell(tnotify)'

Without the permission the popup still works, but every question costs a
confirmation prompt in the pane the popup exists to avoid.

## Telling agents apart

Every notification records an author. Set it once per agent and its questions
can be picked out of a busy queue, and cleared as a group:

    export TNOTIFY_AUTHOR=claude

or per call, with `--author claude`. Left unset, the author falls back to the
name of the calling script. The author is drawn on the popup and in the side
panel, so a queue built up by two agents and a build script still says who is
asking for what.

    tnotify clear --author claude    # drop one agent's questions, leave the rest

## Checking there is an audience

    tnotify --check

exits 0 when this pane is inside tmux with a client attached — that is, when a
popup would actually be seen. Anything else means the question would be drawn to
nobody, and the agent should fall back to asking in its own output. An agent
should run this before its first question, not after.

## What an agent has to account for

**An unanswered question is not a "no".** `notify` always exits 0, and reports
what became of the question through stdout instead: an answer, or nothing at
all. Nothing at all covers a discarded popup, one set aside for later, and one
that timed out. An agent that reads empty as refusal, or as consent, is wrong in
both directions; the safe reading is *no answer yet*, and the safe action is the
one that can be undone.

**Questions set aside come back later.** `esc` keeps the question in the queue
rather than answering it. What happens when it is finally answered depends on
how it was sent:

- Sent with `--wait`, the answer arrives on the still-open stdout of the
  original call.
- Sent without, that call has already returned. The answer is typed into the
  agent pane's input box for the user to send, so it arrives as an ordinary
  message in the conversation, out of step with whatever the agent is doing by
  then. The original question should be treated as unanswered until it turns up.

**Agents have their own timeouts.** A foreground command is usually killed after
a fixed period — ten minutes in Claude Code — whichever `--timeout` was asked
for. So a long `--wait` belongs in a background call whose output is redirected
to a file, and a foreground `--wait` should carry a timeout under the agent's
own ceiling. `--timeout` is also the only timeout worth using: `timeout(1)` is
absent on macOS, and its exit status cannot be told apart from a declined
answer.

**One popup at a time.** Two questions fired in a row leave a queue for the user
to work through. Where an agent has several things to ask, `--multiple` puts
them in a single popup and prints one answer per line.
