// Package buildinfo carries version metadata injected at build time.
package buildinfo

var (
	// Version is the release version, set via -ldflags.
	Version = "dev"
	// Commit is the short git commit, set via -ldflags.
	Commit = "none"
	// Date is the build timestamp, set via -ldflags.
	Date = "unknown"
)

// Summary returns "Version (Commit, Date)".
func Summary() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
