// Package version holds the single source of truth for the goIsekai release
// number. It is a var rather than a const so a release build can stamp it:
//
//	go build -ldflags "-X goisekai/internal/version.Version=$(git describe --tags)"
//
// Without that it falls back to the value below, which is what a plain
// `go build` from source produces.
package version

// Version is the release number, without a leading "v".
var Version = "0.1.0"

// String returns the version as shown to users and printed by -version.
func String() string { return "v" + Version }
