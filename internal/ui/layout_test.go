package ui

import "testing"

func TestUseWideDetailPanel(t *testing.T) {
	m := model{width: 120, mode: modeBrowse, selected: 0}
	if !m.useWideDetailPanel(3) {
		t.Fatal("expected wide layout")
	}
	m.showHelp = true
	if m.useWideDetailPanel(3) {
		t.Fatal("help should disable wide layout")
	}
	m.showHelp = false
	m.width = 80
	if m.useWideDetailPanel(3) {
		t.Fatal("narrow width should stack vertically")
	}
}

func TestWideColumns(t *testing.T) {
	m := model{width: 120, mode: modeBrowse, selected: 0}
	cols, ok := m.wideColumns(3)
	if !ok {
		t.Fatal("expected wide columns")
	}
	if cols.tableW+cols.detailW+wideLayoutGap != m.width {
		t.Fatalf("columns should span width: table=%d detail=%d gap=%d width=%d",
			cols.tableW, cols.detailW, wideLayoutGap, m.width)
	}
	if cols.tableW < minTableAreaWidth {
		t.Fatalf("table area too narrow: %d", cols.tableW)
	}
}

func TestTableRepoWidthForNarrowContent(t *testing.T) {
	m := model{width: 120, showGitContext: true}
	full := m.tableRepoWidth()
	split := m.tableRepoWidthFor(68)
	if split >= full {
		t.Fatalf("split table should be narrower than full width: split=%d full=%d", split, full)
	}
}
