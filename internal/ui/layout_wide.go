package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func tablePanelStyle(contentW int) lipgloss.Style {
	if contentW <= 0 {
		return stylePanel
	}
	return stylePanel.MaxWidth(contentW)
}

func detailPanelStyle(sidebarW int) lipgloss.Style {
	if sidebarW <= 0 {
		return stylePanelAccent
	}
	return stylePanelAccent.Width(sidebarW)
}

func joinWideTableDetail(tableBlock, detailBlock string, detailW int) string {
	leftCol := tableBlock
	leftH := lipgloss.Height(leftCol)

	detailBlock = lipgloss.NewStyle().
		Width(detailW).
		Height(leftH).
		AlignVertical(lipgloss.Top).
		AlignHorizontal(lipgloss.Left).
		Render(detailBlock)

	gap := strings.Repeat(" ", wideLayoutGap)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, gap, detailBlock)
}
