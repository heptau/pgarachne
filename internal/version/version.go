// Package version exposes the build-time version metadata. The values are
// injected at link time by goreleaser and the Makefile via:
//
//	-ldflags "-X github.com/heptau/pgarachne/internal/version.Version=<v>
//	          -X github.com/heptau/pgarachne/internal/version.Commit=<sha>
//	          -X github.com/heptau/pgarachne/internal/version.BuildDate=<rfc3339>"
//
// Any package (including the MCP handler) can import this without depending
// on cmd/pgarachne.
package version

// Version, Commit and BuildDate are overridden at link time. The defaults
// make the binary self-describing in a "go run ./cmd/pgarachne" context
// where no ldflags are passed: the user sees "dev" plus the host's local
// time, which is the closest thing to a useful answer when nothing else
// is available.
var (
	Version   = "0.0.0-dev"
	Commit    = "dev"
	BuildDate = "unknown"
)

// Full returns a single-line, human-readable summary suitable for
// `-version` style CLI output. Format is stable; downstream tooling may
// parse it.
func Full() string {
	return Version + " (" + Commit + ", built " + BuildDate + ")"
}
