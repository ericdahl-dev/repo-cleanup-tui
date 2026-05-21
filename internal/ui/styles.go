package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

// Palette: industrial ops console — charcoal base, amber reclaim, teal safe, coral risk.
var (
	colorInk      = lipgloss.Color("252")
	colorMuted    = lipgloss.Color("245")
	colorDim      = lipgloss.Color("238")
	colorAmber    = lipgloss.Color("220")
	colorTeal     = lipgloss.Color("86")
	colorCoral    = lipgloss.Color("203")
	colorSlate    = lipgloss.Color("239")
	colorSelected = lipgloss.Color("235")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAmber).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	styleStatLabel = lipgloss.NewStyle().Foreground(colorDim)
	styleStatValue = lipgloss.NewStyle().Bold(true).Foreground(colorInk)
	styleStatReclaim = lipgloss.NewStyle().Bold(true).Foreground(colorAmber)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSlate).
			Padding(0, 1)

	stylePanelAccent = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAmber).
				Padding(0, 1)

	styleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("237")).
				Padding(0, 1)

	styleRow = lipgloss.NewStyle().Foreground(colorInk)
	styleRowSel = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAmber).
			Background(colorSelected).
			Padding(0, 1)

	styleRowMarker = lipgloss.NewStyle().Foreground(colorTeal)
	styleRowMarkerSel = lipgloss.NewStyle().Bold(true).Foreground(colorAmber)

	styleKeyHint = lipgloss.NewStyle().Foreground(colorDim)
	styleKey = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)

	styleChipOn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(colorTeal).
			Padding(0, 1)

	styleChipOff = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(lipgloss.Color("237")).
			Padding(0, 1)

	styleChipWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(colorCoral).
			Padding(0, 1)

	styleProgress = lipgloss.NewStyle().Foreground(colorTeal)

	styleDetailLabel = lipgloss.NewStyle().Foreground(colorDim)
	styleDetailValue = lipgloss.NewStyle().Foreground(colorInk)

	styleRiskLow = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	styleRiskMed = lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	styleRiskHigh = lipgloss.NewStyle().Foreground(colorCoral).Bold(true)

	styleHelpBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			BorderForeground(colorAmber)
)

func managerStyle(m scanner.Manager) lipgloss.Style {
	switch m {
	case scanner.ManagerYarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	case scanner.ManagerNpm:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	case scanner.ManagerPnpm:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	case scanner.ManagerBun:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("222"))
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

func riskStyle(level string) lipgloss.Style {
	switch strings.ToLower(level) {
	case "low":
		return styleRiskLow
	case "high":
		return styleRiskHigh
	default:
		return styleRiskMed
	}
}

func filterChip(label string, on bool) string {
	if on {
		return styleChipOn.Render(label)
	}
	return styleChipOff.Render(label)
}

func truncatePath(s string, max int) string {
	if max < 8 || len(s) <= max {
		return s
	}
	return "…" + s[len(s)-(max-1):]
}
