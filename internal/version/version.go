// Package version holds the single source of truth for the goIsekai release
// number. It is a var rather than a const so a release build can stamp it:
//
//	go build -ldflags "-X goisekai/internal/version.Version=$(git describe --tags)"
//
// Without that it falls back to the value below, which is what a plain
// `go build` from source produces.
package version

import "strings"

// Version is the release number. A leading "v" is optional so a build can stamp
// it straight from `git describe --tags`, which already includes one.
var Version = "0.1.0"

// String returns the version with exactly one leading "v", as shown to users and
// printed by -version. It normalises rather than blindly prepending, so stamping
// "v0.1.0" from git describe and "0.1.0" by hand both render as "v0.1.0".
func String() string { return "v" + strings.TrimPrefix(Version, "v") }
