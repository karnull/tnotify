// resources/resources.go

// Package resources embeds tnotify's static text resources so they're available
// from the compiled binary regardless of the current working directory.
package resources

import _ "embed"

//go:embed version.txt
var Version string

//go:embed default.toml
var DefaultConfig string

//go:embed SKILL.md
var Skill string
