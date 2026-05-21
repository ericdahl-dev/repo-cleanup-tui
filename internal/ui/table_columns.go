package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	colMarker = 2
	colSize   = 8
	colIdle   = 7
	colMgr    = 8
	colBranch = 12
	colDirty  = 5
)

func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := runewidth.StringWidth(s)
	if w > width {
		return runewidth.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", width-w)
}

func joinCells(cells ...string) string {
	return strings.TrimRight(strings.Join(cells, " "), " ")
}
