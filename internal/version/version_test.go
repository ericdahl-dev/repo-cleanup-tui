package version

import (
	"strings"
	"testing"
)

func TestLineDevDefault(t *testing.T) {
	oldV, oldC := Version, Commit
	t.Cleanup(func() {
		Version, Commit = oldV, oldC
	})
	Version, Commit = "dev", ""
	got := Line()
	if !strings.Contains(got, "repo-cleanup-tui dev") {
		t.Fatalf("Line() = %q, want dev build line", got)
	}
}

func TestLineRelease(t *testing.T) {
	oldV, oldC := Version, Commit
	t.Cleanup(func() {
		Version, Commit = oldV, oldC
	})
	Version, Commit = "0.1.3", "abc1234567890"
	got := Line()
	want := "repo-cleanup-tui 0.1.3 (abc1234)"
	if got != want {
		t.Fatalf("Line() = %q, want %q", got, want)
	}
}
