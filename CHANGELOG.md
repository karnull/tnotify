# Changelog

## added/modified:

* fix/bubbles-v2: built on bubbles, bubbletea and lipgloss v2, which the dependency bump had left unbuildable.
* fix/devcontainer-timezone: the dev container, and with it CI, starts on hosts that have no /etc/timezone.
* chore/setup-skeleton: the project now follows the go template, building, testing and releasing through make and the shared CI, with every commit's tests free of stale merge leftovers.
