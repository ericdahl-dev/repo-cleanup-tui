package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestPadRight(t *testing.T) {
	if got := padRight("yarn", 8); got != "yarn    " {
		t.Fatalf("got %q", got)
	}
	trunc := padRight("very-long-repo-name", 8)
	if runewidth.StringWidth(trunc) != 8 {
		t.Fatalf("truncate width: %q", trunc)
	}
}

func TestJoinCellsFixedColumns(t *testing.T) {
	c1 := padRight("656 MB", colSize)
	c2 := padRight("460d", colIdle)
	if runewidth.StringWidth(c1) != colSize || runewidth.StringWidth(c2) != colIdle {
		t.Fatalf("column widths: %q %q", c1, c2)
	}
	line := joinCells(padRight("▸", colMarker), c1, c2, padRight("yarn", colMgr))
	if !strings.Contains(line, "656 MB") {
		t.Fatalf("line: %q", line)
	}
}
