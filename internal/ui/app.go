package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/cleanup"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/view"
)

const pageSize = 20

type scanProgressMsg struct {
	dirs int
}

type scanFoundMsg struct {
	candidate scanner.Candidate
}

type scanSizedMsg struct {
	candidate scanner.Candidate
}

type scanDoneMsg struct {
	err error
}

type tickMsg struct{}

type model struct {
	root            string
	ignore          []string
	rows            []scanner.Candidate
	loading         bool
	scanCh          chan scanEvent
	dirsScanned     int
	reposDiscovered int
	reposSized      int
	spinnerFrame    int
	sortMode        view.SortMode
	minInactiveDays int
	showOnlySafe    bool
	showOnlyDirty   bool
	showHelp        bool
	selected        int
	width           int
	mode            uiMode
	confirmInput    string
	assessment      *cleanup.Assessment
	assessmentBusy  bool
	cleanupBusy     bool
	auditLog        []string
}

type scanEvent struct {
	progress *scanner.Progress
	found    *scanner.Candidate
	sized    *scanner.Candidate
	done     bool
	err      error
}

// Run starts the browse TUI for the given workspace root.
func Run(root string, ignore []string) error {
	m := newModel(root, ignore)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(root string, ignore []string) model {
	return model{
		root:         root,
		ignore:       ignore,
		loading:      true,
		sortMode:     view.SortSize,
		minInactiveDays: 90,
		showOnlySafe: true,
		scanCh:       make(chan scanEvent, 64),
		width:        80,
	}
}

func (m model) Init() tea.Cmd {
	go m.runScan()
	return tea.Batch(m.waitScan(), tickCmd())
}

func (m model) runScan() {
	defer close(m.scanCh)
	_, err := scanner.Scan(m.root, scanner.Options{
		IgnoreDirs: m.ignore,
		OnProgress: func(p scanner.Progress) {
			m.scanCh <- scanEvent{progress: &p}
		},
		OnFound: func(c scanner.Candidate) {
			cp := c
			m.scanCh <- scanEvent{found: &cp}
		},
		OnSized: func(c scanner.Candidate) {
			cp := c
			m.scanCh <- scanEvent{sized: &cp}
		},
	})
	m.scanCh <- scanEvent{done: true, err: err}
}

func (m model) waitScan() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.scanCh
		if !ok {
			return scanDoneMsg{}
		}
		if ev.err != nil {
			return scanDoneMsg{err: ev.err}
		}
		if ev.done {
			return scanDoneMsg{}
		}
		if ev.progress != nil {
			return scanProgressMsg{dirs: ev.progress.DirectoriesScanned}
		}
		if ev.found != nil {
			return scanFoundMsg{candidate: *ev.found}
		}
		if ev.sized != nil {
			return scanSizedMsg{candidate: *ev.sized}
		}
		return scanDoneMsg{}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) filtered() []scanner.Candidate {
	return view.FilterAndSort(m.rows, view.FilterSortOptions{
		MinInactiveDays: m.minInactiveDays,
		ShowOnlySafe:    m.showOnlySafe,
		ShowOnlyDirty:   m.showOnlyDirty,
		SortMode:        m.sortMode,
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tickMsg:
		if m.loading || m.cleanupBusy {
			m.spinnerFrame++
		}
		return m, tickCmd()

	case assessmentMsg:
		m.assessmentBusy = false
		if msg.err != nil {
			m.appendAudit(fmt.Sprintf("assessment failed reason=%v", msg.err))
			return m, nil
		}
		a := msg.assessment
		m.assessment = &a
		return m, nil

	case cleanupDoneMsg:
		cmd := m.handleCleanupDone(msg)
		return m, cmd

	case scanProgressMsg:
		m.dirsScanned = msg.dirs
		return m, m.waitScan()

	case scanFoundMsg:
		if !m.hasRow(msg.candidate.RepoPath) {
			m.rows = append(m.rows, msg.candidate)
			m.reposDiscovered++
		}
		m.clampSelection(len(m.filtered()))
		return m, m.waitScan()

	case scanSizedMsg:
		if msg.candidate.Bytes > 0 {
			m.upsertRow(msg.candidate)
			m.reposSized = m.countSized()
		}
		return m, m.waitScan()

	case scanDoneMsg:
		m.loading = false
		if msg.err != nil {
			return m, tea.Quit
		}
		m.reposDiscovered = len(m.rows)
		m.reposSized = m.countSized()
		return m, nil

	case tea.KeyMsg:
		var extra tea.Cmd
		if quit, cmd := m.handleKey(msg); quit {
			return m, tea.Quit
		} else if cmd != nil {
			extra = cmd
		}
		if extra != nil {
			return m, extra
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (quit bool, cmd tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" || key == "esc" {
		if m.showHelp {
			m.showHelp = false
			return false, nil
		}
		if m.mode != modeBrowse {
			m.mode = modeBrowse
			m.confirmInput = ""
			m.assessment = nil
			m.assessmentBusy = false
			return false, nil
		}
		return true, nil
	}

	if m.mode == modePreview {
		if quit, cmd := m.handlePreviewKey(msg); quit || cmd != nil {
			return quit, cmd
		}
		return false, nil
	}
	if m.mode == modeConfirm {
		if quit, cmd := m.handleConfirmKey(msg); quit || cmd != nil {
			return quit, cmd
		}
		return false, nil
	}

	filtered := m.filtered()
	switch key {
	case "x":
		if row, ok := m.activeRow(); ok {
			m.mode = modePreview
			m.confirmInput = ""
			m.assessment = nil
			m.assessmentBusy = true
			return false, assessCmd(row)
		}
	case "r":
		cmd = m.startRescan()
	case "s":
		if m.sortMode == view.SortSize {
			m.sortMode = view.SortInactive
		} else {
			m.sortMode = view.SortSize
		}
		m.clampSelection(len(m.filtered()))
	case "f":
		switch m.minInactiveDays {
		case 0:
			m.minInactiveDays = 30
		case 30:
			m.minInactiveDays = 90
		case 90:
			m.minInactiveDays = 180
		default:
			m.minInactiveDays = 0
		}
		m.clampSelection(len(m.filtered()))
	case "k":
		m.showOnlySafe = !m.showOnlySafe
		m.clampSelection(len(m.filtered()))
	case "d":
		m.showOnlyDirty = !m.showOnlyDirty
		m.clampSelection(len(m.filtered()))
	case "?":
		m.showHelp = !m.showHelp
	case "down", "j":
		if m.selected < len(filtered)-1 {
			m.selected++
		}
	case "up", "u":
		if m.selected > 0 {
			m.selected--
		}
	}
	m.clampSelection(len(m.filtered()))
	return false, cmd
}

func (m *model) startRescan() tea.Cmd {
	m.loading = true
	m.rows = nil
	m.dirsScanned = 0
	m.reposDiscovered = 0
	m.reposSized = 0
	m.selected = 0
	m.scanCh = make(chan scanEvent, 64)
	go m.runScan()
	return tea.Batch(m.waitScan(), tickCmd())
}

func (m *model) hasRow(path string) bool {
	for _, r := range m.rows {
		if r.RepoPath == path {
			return true
		}
	}
	return false
}

func (m *model) upsertRow(c scanner.Candidate) {
	for i := range m.rows {
		if m.rows[i].RepoPath == c.RepoPath {
			m.rows[i] = c
			return
		}
	}
	m.rows = append(m.rows, c)
}

func (m *model) countSized() int {
	n := 0
	for _, r := range m.rows {
		if r.Bytes > 0 {
			n++
		}
	}
	return n
}

func (m *model) clampSelection(n int) {
	if n == 0 {
		m.selected = 0
		return
	}
	if m.selected >= n {
		m.selected = n - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m model) View() string {
	var b strings.Builder
	filtered := m.filtered()

	totalReclaim := int64(0)
	for _, row := range filtered {
		totalReclaim += row.Bytes
	}

	titleStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(titleStyle.Render("Repo Cleanup TUI (browse)") + "\n")
	fmt.Fprintf(&b, "Found %d repos | visible %d | reclaimable %s\n",
		len(m.rows), len(filtered), formatBytes(totalReclaim))
	fmt.Fprintf(&b, "Workspace: %s | sort: %s | selected %d/%d\n",
		m.root, m.sortMode, selectionLabel(len(filtered), m.selected), len(filtered))

	if m.loading {
		spin := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		fmt.Fprintf(&b, "%s scanning %d dirs in %s\n", spin, m.dirsScanned, m.root)
		fmt.Fprintf(&b, "Discovery %s %d dirs | repos found %d\n",
			indeterminateBar(m.spinnerFrame, discoveryBarWidth), m.dirsScanned, m.reposDiscovered)
	} else {
		fmt.Fprintf(&b, "Scan complete in %s\n", m.root)
	}

	if m.reposDiscovered > 0 {
		fmt.Fprintf(&b, "Sizing %s %d/%d\n",
			ratioBar(m.reposSized, m.reposDiscovered, sizingBarWidth), m.reposSized, m.reposDiscovered)
	}

	inactiveLabel := "all"
	if m.minInactiveDays > 0 {
		inactiveLabel = fmt.Sprintf(">=%dd", m.minInactiveDays)
	}
	b.WriteString("Keys: q quit | ? help | r rescan | x cleanup | j/u move | s sort | f inactive | k safe | d dirty\n")
	fmt.Fprintf(&b, "Filters: inactive(%s) safe-only(%s) dirty-only(%s)\n",
		inactiveLabel, onOff(m.showOnlySafe), onOff(m.showOnlyDirty))

	if m.showHelp {
		b.WriteString("\nHelp:\n")
		b.WriteString("  x  cleanup preview | p dry-run | y confirm | n cancel\n")
		b.WriteString("  s  toggle sort (size / inactive)\n")
		b.WriteString("  f  cycle inactivity threshold (all, 30d, 90d, 180d)\n")
		b.WriteString("  k  toggle safe-only (lockfile required)\n")
		b.WriteString("  d  toggle dirty-only\n")
		b.WriteString("  r  full rescan (no cache)\n")
	}

	b.WriteString("\n      size | inactive | repo\n")
	pageStart := pageWindowStart(m.selected, len(filtered))
	visible := filtered[pageStart:min(pageStart+pageSize, len(filtered))]
	for i, row := range visible {
		abs := pageStart + i
		prefix := " "
		if abs == m.selected {
			prefix = ">"
		}
		rel := row.RepoPath
		if r, err := filepath.Rel(m.root, row.RepoPath); err == nil && r != "" {
			rel = r
		}
		inactive := "unknown"
		if row.InactiveDays != nil {
			inactive = fmt.Sprintf("%dd", *row.InactiveDays)
		}
		fmt.Fprintf(&b, "%s %8s | %7s | %s\n", prefix, formatBytes(row.Bytes), inactive, rel)
	}

	if m.loading && len(filtered) == 0 {
		b.WriteString("\nScanning… list fills as candidates are sized.\n")
	} else if !m.loading && len(filtered) == 0 {
		b.WriteString("\nNo rows match filters.\n")
	} else if len(filtered) > pageSize {
		end := min(pageStart+pageSize, len(filtered))
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", pageStart+1, end, len(filtered))
	}

	if len(filtered) > 0 && m.selected < len(filtered) {
		row := filtered[m.selected]
		b.WriteString("\nSelected\n")
		fmt.Fprintf(&b, "  Repo: %s\n", row.RepoPath)
		fmt.Fprintf(&b, "  Manager: %s | lockfile: %s | dirty: %s\n",
			row.Manager, yesNo(row.HasLockfile), yesNo(row.Git.Dirty))
		fmt.Fprintf(&b, "  node_modules: %s\n", row.NodeModulesPath)
		fmt.Fprintf(&b, "  Restore: (cd %s && %s)\n", row.RepoPath, row.ReinstallCommand)
		if m.mode == modeBrowse {
			b.WriteString("  Next: press x for preview, then p (dry-run) or y (confirm).\n")
		}
	} else if m.loading {
		b.WriteString("\nWaiting for first match…\n")
	}

	if m.mode == modePreview && len(filtered) > 0 && m.selected < len(filtered) {
		row := filtered[m.selected]
		b.WriteString("\nPreview (dry-run)\n")
		fmt.Fprintf(&b, "  Target: %s\n", row.NodeModulesPath)
		if m.assessmentBusy {
			b.WriteString("  Assessing risk…\n")
		} else if m.assessment != nil {
			fmt.Fprintf(&b, "  Risk: %s | confidence: %s | guards: %s\n",
				m.assessment.RiskLevel, m.assessment.Confidence,
				assessmentGuardLabel(m.assessment))
			if len(m.assessment.Warnings) > 0 {
				fmt.Fprintf(&b, "  Warnings: %s\n", strings.Join(m.assessment.Warnings, ", "))
			}
		}
		b.WriteString("  Keys: p run dry-run | y continue to confirm | n cancel | esc back\n")
	}

	if m.mode == modeConfirm && len(filtered) > 0 && m.selected < len(filtered) {
		row := filtered[m.selected]
		b.WriteString("\nConfirm cleanup: remove node_modules only (repository is not deleted).\n")
		fmt.Fprintf(&b, "  Action: delete %s\n", row.NodeModulesPath)
		b.WriteString("  Type the confirmation token exactly, then press Enter.\n")
		fmt.Fprintf(&b, "  Token: %s\n", cleanup.BuildConfirmToken(row))
		fmt.Fprintf(&b, "  Input: %s\n", m.confirmInput)
		if m.cleanupBusy {
			spin := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
			fmt.Fprintf(&b, "  %s deleting node_modules…\n", spin)
		}
	}

	if len(m.auditLog) > 0 {
		b.WriteString("\nAudit log (also on stderr)\n")
		for _, line := range m.auditLog {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}

	return b.String()
}

func assessmentGuardLabel(a *cleanup.Assessment) string {
	if a.OK {
		return "ok"
	}
	return strings.Join(a.Reasons, ", ")
}

func pageWindowStart(selected, total int) int {
	if total <= pageSize {
		return 0
	}
	start := selected - pageSize/2
	if start < 0 {
		return 0
	}
	maxStart := total - pageSize
	if start > maxStart {
		return maxStart
	}
	return start
}

func selectionLabel(total, selected int) int {
	if total == 0 {
		return 0
	}
	return selected + 1
}

func formatBytes(bytes int64) string {
	const gb = 1024 * 1024 * 1024
	if bytes >= gb {
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("%d MB", bytes/mb)
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
