package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/cleanup"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

func (m model) renderHeader(totalReclaim int64, visible int) string {
	title := styleTitle.Render("◆ repo-cleanup-tui")
	sub := styleSubtitle.Render("reclaim node_modules · safe restore paths")
	line1 := lipgloss.JoinHorizontal(lipgloss.Top,
		styleStatLabel.Render("repos "),
		styleStatValue.Render(fmt.Sprintf("%d", len(m.rows))),
		styleStatLabel.Render("  visible "),
		styleStatValue.Render(fmt.Sprintf("%d", visible)),
		styleStatLabel.Render("  reclaimable "),
		styleStatReclaim.Render(formatBytes(totalReclaim)),
	)
	line2 := lipgloss.JoinHorizontal(lipgloss.Top,
		styleStatLabel.Render("workspace "),
		styleDetailValue.Render(truncatePath(m.root, max(20, m.width/3))),
		styleStatLabel.Render("  sort "),
		styleStatValue.Render(string(m.sortMode)),
		styleStatLabel.Render("  cursor "),
		styleStatValue.Render(fmt.Sprintf("%d/%d", selectionLabel(visible, m.selected), visible)),
	)
	body := lipgloss.JoinVertical(lipgloss.Left, title, sub, line1, line2)
	return stylePanel.Render(body)
}

func (m model) renderStatus() string {
	if m.loading {
		spin := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		spinS := styleProgress.Render(spin)
		disc := styleProgress.Render(indeterminateBarStyled(m.spinnerFrame, discoveryBarWidth))
		line1 := fmt.Sprintf("%s scanning %s", spinS, styleDetailValue.Render(truncatePath(m.root, m.width-20)))
		line2 := fmt.Sprintf("discovery %s %d dirs · found %d repos",
			disc, m.dirsScanned, m.reposDiscovered)
		var line3 string
		if m.reposDiscovered > 0 {
			sizeBar := styleProgress.Render(ratioBarStyled(m.reposSized, m.reposDiscovered, sizingBarWidth))
			line3 = fmt.Sprintf("sizing    %s %d/%d", sizeBar, m.reposSized, m.reposDiscovered)
		}
		return stylePanelAccent.Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3))
	}
	if m.reposDiscovered > 0 && m.reposSized < m.reposDiscovered {
		sizeBar := styleProgress.Render(ratioBarStyled(m.reposSized, m.reposDiscovered, sizingBarWidth))
		return styleStatLabel.Render(fmt.Sprintf("sizing %s %d/%d", sizeBar, m.reposSized, m.reposDiscovered))
	}
	return styleStatLabel.Render(fmt.Sprintf(
		"scan complete · %d dirs walked · %d repos with node_modules",
		m.dirsScanned, len(m.rows),
	))
}

func (m model) renderFilterBar() string {
	inactiveLabel := "inactive:all"
	if m.minInactiveDays > 0 {
		inactiveLabel = fmt.Sprintf("inactive:≥%dd", m.minInactiveDays)
	}
	chips := []string{
		filterChip(inactiveLabel, m.minInactiveDays > 0),
		filterChip("safe-only", m.showOnlySafe),
		filterChip("dirty-only", m.showOnlyDirty),
	}
	if q := strings.TrimSpace(m.searchQuery); q != "" {
		chips = append(chips, styleChipWarn.Render("search:"+q))
	}
	if m.showGitContext {
		chips = append(chips, styleChipOn.Render("git cols"))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, chips...)
	keys := styleKeyHint.Render("keys ") +
		styleKey.Render("?") + styleKeyHint.Render(" help ") +
		styleKey.Render("/") + styleKeyHint.Render(" find ") +
		styleKey.Render("w") + styleKeyHint.Render(" root ") +
		styleKey.Render("r") + styleKeyHint.Render(" scan ") +
		styleKey.Render("x") + styleKeyHint.Render(" clean ") +
		styleKey.Render("q") + styleKeyHint.Render(" quit")
	return lipgloss.JoinVertical(lipgloss.Left, row, keys)
}

func (m model) renderTable(filtered []scanner.Candidate) string {
	if len(filtered) == 0 {
		return stylePanel.Render(m.renderEmptyListMessage())
	}

	pageStart := pageWindowStart(m.selected, len(filtered))
	visible := filtered[pageStart:min(pageStart+pageSize, len(filtered))]

	repoW := 28
	if m.width > 100 {
		repoW = m.width - 58
	}
	if m.showGitContext && repoW > 20 {
		repoW -= 18
	}
	if repoW < 12 {
		repoW = 12
	}

	var header string
	if m.showGitContext {
		header = styleTableHeader.Render(
			fmt.Sprintf("%-2s %-10s %-5s %8s %7s %-8s %s",
				"", "branch", "dirty", "size", "idle", "mgr", "repo"))
	} else {
		header = styleTableHeader.Render(
			fmt.Sprintf("%-2s %8s %7s %-8s %s", "", "size", "idle", "mgr", "repo"))
	}

	var rows []string
	for i, row := range visible {
		abs := pageStart + i
		selected := abs == m.selected
		rel := row.RepoPath
		if r, err := filepath.Rel(m.root, row.RepoPath); err == nil && r != "" {
			rel = r
		}
		rel = truncatePath(rel, repoW)
		inactive := styleStatLabel.Render("?")
		if row.InactiveDays != nil {
			inactive = styleStatValue.Render(fmt.Sprintf("%dd", *row.InactiveDays))
		}
		size := styleStatReclaim.Render(formatBytes(row.Bytes))
		if row.Bytes == 0 {
			size = styleStatLabel.Render("…")
		}
		mgr := managerStyle(row.Manager).Render(string(row.Manager))
		marker := " "
		if selected {
			marker = styleRowMarkerSel.Render("▸")
		} else {
			marker = styleRowMarker.Render(" ")
		}
		var line string
		if m.showGitContext {
			branch := row.Git.Branch
			if branch == "" {
				branch = "-"
			}
			branch = truncatePath(branch, 10)
			dirty := styleStatLabel.Render("no")
			if row.Git.Dirty {
				dirty = styleChipWarn.Render("yes")
			}
			line = fmt.Sprintf("%s %-10s %-5s %8s %7s %-8s %s",
				marker, branch, dirty, size, inactive, mgr, rel)
		} else {
			line = fmt.Sprintf("%s %8s %7s %-8s %s", marker, size, inactive, mgr, rel)
		}
		if selected {
			rows = append(rows, styleRowSel.Render(line))
		} else {
			rows = append(rows, styleRow.Render(line))
		}
	}

	body := strings.Join(append([]string{header}, rows...), "\n")
	panel := stylePanel.Render(body)
	if len(filtered) > pageSize {
		end := min(pageStart+pageSize, len(filtered))
		pager := styleStatLabel.Render(fmt.Sprintf("  showing %d–%d of %d", pageStart+1, end, len(filtered)))
		return lipgloss.JoinVertical(lipgloss.Left, panel, pager)
	}
	return panel
}

func (m model) renderSelectionDetail(row scanner.Candidate) string {
	title := styleSubtitle.Render("selected")
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			styleDetailLabel.Render("repo "), styleDetailValue.Render(truncatePath(row.RepoPath, m.width-8))),
		lipgloss.JoinHorizontal(lipgloss.Top,
			styleDetailLabel.Render("manager "),
			managerStyle(row.Manager).Render(string(row.Manager)),
			styleDetailLabel.Render(" · lockfile "),
			lockfileStyle(row.HasLockfile).Render(yesNo(row.HasLockfile)),
			styleDetailLabel.Render(" · dirty "),
			dirtyStyle(row.Git.Dirty).Render(yesNo(row.Git.Dirty)),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			styleDetailLabel.Render("node_modules "),
			styleStatReclaim.Render(formatBytes(row.Bytes)),
		),
		styleDetailLabel.Render("restore") + " " +
			styleDetailValue.Render(fmt.Sprintf("(cd %s && %s)", truncatePath(row.RepoPath, 40), row.ReinstallCommand)),
	}
	if m.showGitContext && row.Git.Branch != "" {
		lines = append(lines, styleDetailLabel.Render("branch ")+styleDetailValue.Render(row.Git.Branch))
	}
	if m.mode == modeBrowse {
		lines = append(lines, styleKeyHint.Render("press x → preview cleanup"))
	}
	return stylePanelAccent.Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title}, lines...)...))
}

func lockfileStyle(ok bool) lipgloss.Style {
	if ok {
		return styleRiskLow
	}
	return styleRiskHigh
}

func dirtyStyle(dirty bool) lipgloss.Style {
	if dirty {
		return styleChipWarn
	}
	return styleStatLabel
}

func (m model) renderModeBanner() string {
	switch m.mode {
	case modeSearch:
		return stylePanelAccent.Render(
			styleKey.Render("search") + " " + styleDetailValue.Render(m.searchQuery) +
				styleKeyHint.Render(" · enter apply · esc cancel"))
	case modeWorkspace:
		return stylePanelAccent.Render(
			styleKey.Render("workspace") + " " + styleDetailValue.Render(m.workspaceInput) +
				styleKeyHint.Render(" · enter save+rescan · esc cancel"))
	case modePreview:
		return m.renderPreviewPanel()
	case modeConfirm:
		return m.renderConfirmPanel()
	default:
		return ""
	}
}

func (m model) renderPreviewPanel() string {
	filtered := m.filtered()
	if len(filtered) == 0 || m.selected >= len(filtered) {
		return ""
	}
	row := filtered[m.selected]
	lines := []string{
		styleSubtitle.Render("cleanup preview"),
		styleDetailLabel.Render("target ") + styleDetailValue.Render(row.NodeModulesPath),
	}
	if m.assessmentBusy {
		lines = append(lines, styleProgress.Render("  ◌ assessing risk…"))
	} else if m.assessment != nil {
		lines = append(lines,
			lipgloss.JoinHorizontal(lipgloss.Top,
				styleDetailLabel.Render("risk "),
				riskStyle(string(m.assessment.RiskLevel)).Render(string(m.assessment.RiskLevel)),
				styleDetailLabel.Render(" · confidence "),
				styleStatValue.Render(string(m.assessment.Confidence)),
				styleDetailLabel.Render(" · guards "),
				styleStatValue.Render(assessmentGuardLabel(m.assessment)),
			))
		if len(m.assessment.Warnings) > 0 {
			lines = append(lines, styleChipWarn.Render("warn: "+strings.Join(m.assessment.Warnings, ", ")))
		}
	}
	lines = append(lines, styleKeyHint.Render("p dry-run · y confirm · n cancel · esc back"))
	return stylePanel.Render(strings.Join(lines, "\n"))
}

func (m model) renderConfirmPanel() string {
	filtered := m.filtered()
	if len(filtered) == 0 || m.selected >= len(filtered) {
		return ""
	}
	row := filtered[m.selected]
	token := cleanup.BuildConfirmToken(row)
	lines := []string{
		styleChipWarn.Render("confirm delete — repo is not removed"),
		styleDetailLabel.Render("delete ") + styleDetailValue.Render(row.NodeModulesPath),
		styleDetailLabel.Render("type token ") + styleStatReclaim.Render(token),
		styleDetailLabel.Render("input ") + styleDetailValue.Render(m.confirmInput),
	}
	if m.cleanupBusy {
		spin := styleProgress.Render(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
		lines = append(lines, spin+" deleting node_modules…")
	}
	lines = append(lines, styleKeyHint.Render("enter submit · esc cancel"))
	return stylePanelAccent.Render(strings.Join(lines, "\n"))
}

func (m model) renderAudit() string {
	if len(m.auditLog) == 0 {
		return ""
	}
	lines := []string{styleSubtitle.Render("audit (stderr)")}
	for _, line := range m.auditLog {
		lines = append(lines, styleStatLabel.Render("  "+line))
	}
	return strings.Join(lines, "\n")
}

func indeterminateBarStyled(frame, width int) string {
	if width <= 0 {
		return "░░░░"
	}
	filled := frame % width
	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == filled:
			b.WriteString("█")
		case i == filled-1 || i == filled+1:
			b.WriteString("▓")
		default:
			b.WriteString("░")
		}
	}
	return b.String()
}

func ratioBarStyled(done, total, width int) string {
	if width <= 0 || total <= 0 {
		return strings.Repeat("░", max(width, 4))
	}
	clamped := done
	if clamped < 0 {
		clamped = 0
	}
	if clamped > total {
		clamped = total
	}
	filled := (clamped * width) / total
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			b.WriteString("█")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) renderEmptyListMessage() string {
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left,
			styleSubtitle.Render("scanning"),
			styleStatLabel.Render("rows appear as repos are discovered and sized"),
		)
	}
	if len(m.rows) == 0 {
		hint := parentWorkspaceHint(m.root)
		lines := []string{
			styleSubtitle.Render("no node_modules repos here"),
			styleStatLabel.Render(
				"this workspace has no git repos with both package.json and node_modules",
			),
		}
		if hint != "" {
			lines = append(lines,
				styleKeyHint.Render("try ")+
					styleKey.Render("w")+styleKeyHint.Render(" workspace → ")+
					styleDetailValue.Render(hint)+
					styleKeyHint.Render(" then ")+
					styleKey.Render("r")+styleKeyHint.Render(" rescan"),
			)
		} else {
			lines = append(lines,
				styleKeyHint.Render("press ")+styleKey.Render("w")+styleKeyHint.Render(" to choose a folder with JS repos, then ")+styleKey.Render("r")+styleKeyHint.Render(" rescan"),
			)
		}
		lines = append(lines,
			styleKeyHint.Render("or run: ")+styleDetailValue.Render("repo-cleanup-tui ~/Documents/GitHub"),
		)
		return strings.Join(lines, "\n")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		styleSubtitle.Render("filters hide every repo"),
		styleStatLabel.Render(fmt.Sprintf(
			"%d repos found under workspace; none match inactive/safe/dirty/search filters",
			len(m.rows),
		)),
		styleKeyHint.Render("press ")+styleKey.Render("k")+styleKeyHint.Render(" safe-only · ")+
			styleKey.Render("f")+styleKeyHint.Render(" inactive · ")+
			styleKey.Render("d")+styleKeyHint.Render(" dirty · ")+
			styleKey.Render("c")+styleKeyHint.Render(" clear search"),
	)
}

// parentWorkspaceHint suggests scanning the parent when cwd looks like a single repo.
func parentWorkspaceHint(root string) string {
	root = filepath.Clean(root)
	parent := filepath.Dir(root)
	if parent == root || parent == "/" || parent == "." {
		return ""
	}
	base := filepath.Base(root)
	// Common: launched inside one repo; parent often holds many siblings.
	if pathExists(filepath.Join(root, ".git")) || pathExists(filepath.Join(root, "package.json")) {
		return parent
	}
	if strings.EqualFold(base, "repo-cleanup-tui") {
		return parent
	}
	return ""
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
