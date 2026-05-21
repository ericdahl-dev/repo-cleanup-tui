package ui

const (
	wideLayoutMinWidth = 100
	detailPanelWidth   = 44
)

func (m model) useWideDetailPanel(filteredCount int) bool {
	if m.width < wideLayoutMinWidth {
		return false
	}
	if m.showHelp || m.mode != modeBrowse {
		return false
	}
	return filteredCount > 0 && m.selected < filteredCount
}
