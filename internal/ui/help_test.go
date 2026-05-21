package ui

import (
	"strings"
	"testing"
)

func TestRenderHelpDocumentsBrowseAndCleanupKeys(t *testing.T) {
	out := RenderHelp(100)
	for _, needle := range []string{
		"repo-cleanup-tui",
		"x",
		"cleanup",
		"/",
		"search",
		"w",
		"workspace",
		"r",
		"rescan",
	} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(needle)) {
			t.Fatalf("help missing %q:\n%s", needle, out)
		}
	}
}
