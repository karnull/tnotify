// resources/resources.go

// Package resources embeds tnotify's static text resources so they're available
// from the compiled binary regardless of the current working directory.
package resources

import (
	_ "embed"

	"github.com/karnull/tnotify"
)

// The release version, stamped in at build time.
var Version = tnotify.Version

//go:embed default.toml
var DefaultConfig string

//go:embed SKILL.md
var Skill string
