package version

import "fmt"

// Set at link time via -ldflags (see .goreleaser.yaml).
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Line returns a single-line version string for --version.
func Line() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	if Commit != "" {
		short := Commit
		if len(short) > 7 {
			short = short[:7]
		}
		return fmt.Sprintf("repo-cleanup-tui %s (%s)", v, short)
	}
	return fmt.Sprintf("repo-cleanup-tui %s", v)
}
