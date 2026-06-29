// pkg/testexports.go

package pkg

// The tests live in test/ at the project root, which puts them in a package
// of their own and out of reach of everything below. This file is the one
// door through: the models stay unexported and so do their fields, and are
// reached from outside under exported names here.

//- Functions --------------------------------------------------------------------------------------

var (
	SplitFlags = splitFlags
)
