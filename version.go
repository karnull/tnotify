// version.go

// Package tnotify holds the release version from VERSION, kept here because
// go:embed only reaches files at or below the embedding package.
package tnotify

import _ "embed"

//go:embed VERSION
var Version string
