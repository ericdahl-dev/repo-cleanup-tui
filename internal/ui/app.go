package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/cleanup"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/config"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scancache"
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
	showGitContext  bool
	searchQuery     string
	workspaceInput  string
	cfg             *config.Config
	selected        int
	width           int
	mode            uiMode
	confirmInput    string
	assessment      *cleanup.Assessment
	assessmentBusy  bool
	cleanupBusy     bool
	auditLog        []string
	fromCache       bool
}

type scanEvent struct {
	progress *scanner.Progress
	found    *scanner.Candidate
	sized    *scanner.Candidate
	done     bool
	err      error
}

// Run starts the browse TUI for the given workspace root.
func Run(root string, ignore []string, cfg *config.Config) error {
	m := newModel(root, ignore, cfg)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(root string, ignore []string, cfg *config.Config) model {
	m := model{
		root:            root,
		ignore:          ignore,
		cfg:             cfg,
		workspaceInput:  root,
		loading:         true,
		sortMode:        view.SortSize,
		minInactiveDays: 90,
		showOnlySafe:    true,
		scanCh:          make(chan scanEvent, 64),
		width:           80,
	}
	if rows, err := scancache.Load(root, scancache.DefaultTTL); err == nil {
		m.rows = rows
		m.loading = false
		m.fromCache = true
		m.reposDiscovered = len(rows)
		m.reposSized = countSizedRows(rows)
	}
	return m
}

func countSizedRows(rows []scanner.Candidate) int {
	n := 0
	for _, r := range rows {
		if r.Bytes > 0 {
			n++
		}
	}
	return n
}

func (m model) Init() tea.Cmd {
	if !m.loading {
		return tickCmd()
	}
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
		SearchQuery:     m.searchQuery,
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
		_ = scancache.Save(m.root, m.rows)
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
		if m.mode == modeSearch || m.mode == modeWorkspace {
			m.mode = modeBrowse
			m.workspaceInput = ""
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

	if m.mode == modeSearch {
		return m.handleSearchKey(msg)
	}
	if m.mode == modeWorkspace {
		return m.handleWorkspaceKey(msg)
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
	case "/":
		m.mode = modeSearch
	case "c":
		m.searchQuery = ""
		m.clampSelection(len(m.filtered()))
	case "w":
		m.mode = modeWorkspace
		m.workspaceInput = m.root
	case "g":
		m.showGitContext = !m.showGitContext
	case "down", "j":
		if m.selected < len(filtered)-1 {
			m.selected++
		}
	case "up", "u":
		if m.selected > 0 {
			m.selected--
		}
	case "]":
		m.selected = pageJumpIndex(m.selected, len(filtered), pageSize)
	case "[":
		m.selected = pageJumpIndex(m.selected, len(filtered), -pageSize)
	}
	m.clampSelection(len(m.filtered()))
	return false, cmd
}

func (m *model) startRescan() tea.Cmd {
	m.loading = true
	m.fromCache = false
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
	filtered := m.filtered()
	totalReclaim := int64(0)
	for _, row := range filtered {
		totalReclaim += row.Bytes
	}

	parts := []string{
		m.renderHeader(totalReclaim, len(filtered)),
		m.renderStatus(),
		m.renderFilterBar(),
	}
	if m.showHelp {
		parts = append(parts, RenderHelp(m.width))
	}
	parts = append(parts, m.renderTable(filtered))

	var detail string
	if len(filtered) > 0 && m.selected < len(filtered) {
		detail = m.renderSelectionDetail(filtered[m.selected])
	} else if m.loading {
		detail = styleStatLabel.Render("◌ waiting for first match…")
	}

	wide := m.useWideDetailPanel(len(filtered)) && detail != ""
	if wide {
		leftW := m.width - detailPanelWidth - 2
		if leftW < 40 {
			leftW = 40
		}
		left := lipgloss.NewStyle().Width(leftW).Render(strings.Join(parts, "\n"))
		out := lipgloss.JoinHorizontal(lipgloss.Top, left, detail)
		if banner := m.renderModeBanner(); banner != "" {
			out += "\n" + banner
		}
		if audit := m.renderAudit(); audit != "" {
			out += "\n" + audit
		}
		return out
	}

	if detail != "" {
		parts = append(parts, detail)
	}
	if banner := m.renderModeBanner(); banner != "" {
		parts = append(parts, banner)
	}
	if audit := m.renderAudit(); audit != "" {
		parts = append(parts, audit)
	}
	return strings.Join(parts, "\n")
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
