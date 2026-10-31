// version.go

// Package tnotify holds the release version, which CI stamps in from the release tag.
package tnotify

// Set with -ldflags -X at build time; anything built outside a release reports dev.
var Version = "dev"
