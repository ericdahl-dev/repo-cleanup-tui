package ui

const (
	wideLayoutMinWidth = 100
	wideLayoutGap      = 2
	minTableAreaWidth  = 56
)

type wideColumns struct {
	tableW  int
	detailW int
}

func (m model) useWideDetailPanel(filteredCount int) bool {
	if m.width < wideLayoutMinWidth {
		return false
	}
	if m.showHelp || m.mode != modeBrowse {
		return false
	}
	return filteredCount > 0 && m.selected < filteredCount
}

func (m model) wideColumns(filteredCount int) (wideColumns, bool) {
	if !m.useWideDetailPanel(filteredCount) {
		return wideColumns{}, false
	}
	detailW := min(50, max(36, (m.width-wideLayoutGap)/3))
	tableW := m.width - detailW - wideLayoutGap
	if tableW < minTableAreaWidth {
		return wideColumns{}, false
	}
	return wideColumns{tableW: tableW, detailW: detailW}, true
}
