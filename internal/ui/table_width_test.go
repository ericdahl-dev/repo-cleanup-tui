package ui

import "testing"

func TestTableLayoutWidthCapsWideTerminal(t *testing.T) {
	m := model{width: 200, showGitContext: false}
	if got := m.tableLayoutWidth(); got != maxTableWidth {
		t.Fatalf("layout width = %d, want %d", got, maxTableWidth)
	}
	if m.tableRepoWidth() > maxRepoColWidth {
		t.Fatalf("repo col too wide: %d", m.tableRepoWidth())
	}
}
