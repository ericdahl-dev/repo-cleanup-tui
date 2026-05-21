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
