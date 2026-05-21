package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/cleanup"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

type uiMode int

const (
	modeBrowse uiMode = iota
	modePreview
	modeConfirm
	modeSearch
	modeWorkspace
	modeWorkspaceManager
)

type wsMgrPane int

const (
	wsMgrList wsMgrPane = iota
	wsMgrAddPath
	wsMgrEditIgnores
)

type assessmentMsg struct {
	assessment cleanup.Assessment
	err        error
}

type cleanupDoneMsg struct {
	rowPath string
	result  cleanup.Result
	err     error
	dryRun  bool
}

func (m *model) appendAudit(entry string) {
	line := fmt.Sprintf("%s %s", time.Now().Format(time.RFC3339), entry)
	fmt.Fprintln(os.Stderr, line)
	m.auditLog = append([]string{line}, m.auditLog...)
	if len(m.auditLog) > 5 {
		m.auditLog = m.auditLog[:5]
	}
}

func assessCmd(row scanner.Candidate) tea.Cmd {
	return func() tea.Msg {
		a, err := cleanup.AssessSafety(row)
		return assessmentMsg{assessment: a, err: err}
	}
}

func cleanupCmd(row scanner.Candidate, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		r, err := cleanup.Execute(row, dryRun)
		return cleanupDoneMsg{rowPath: row.RepoPath, result: r, err: err, dryRun: dryRun}
	}
}

func (m *model) activeRow() (scanner.Candidate, bool) {
	filtered := m.filtered()
	if len(filtered) == 0 || m.selected >= len(filtered) {
		return scanner.Candidate{}, false
	}
	return filtered[m.selected], true
}

func (m *model) handlePreviewKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	row, ok := m.activeRow()
	if !ok {
		return false, nil
	}
	switch msg.String() {
	case "n":
		m.mode = modeBrowse
		m.confirmInput = ""
		return false, nil
	case "p":
		return false, cleanupCmd(row, true)
	case "y":
		if m.assessment == nil {
			m.appendAudit(fmt.Sprintf("blocked cleanup %s reason=assessment-pending", row.RepoPath))
			return false, nil
		}
		if !m.assessment.OK {
			m.appendAudit(fmt.Sprintf("blocked cleanup %s reason=%s", row.RepoPath, strings.Join(m.assessment.Reasons, ",")))
			return false, nil
		}
		m.mode = modeConfirm
		m.confirmInput = ""
		return false, nil
	}
	return false, nil
}

func (m *model) handleConfirmKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if m.cleanupBusy {
		return false, nil
	}
	row, ok := m.activeRow()
	if !ok {
		return false, nil
	}
	token := cleanup.BuildConfirmToken(row)
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.confirmInput) != token {
			m.appendAudit(fmt.Sprintf("blocked cleanup %s reason=bad-confirm-token", row.RepoPath))
			return false, nil
		}
		m.cleanupBusy = true
		return false, cleanupCmd(row, false)
	case "backspace":
		if len(m.confirmInput) > 0 {
			m.confirmInput = m.confirmInput[:len(m.confirmInput)-1]
		}
		return false, nil
	case "delete":
		m.confirmInput = ""
		return false, nil
	default:
		if len(msg.Runes) > 0 {
			m.confirmInput += string(msg.Runes)
		}
	}
	return false, nil
}

func (m *model) handleCleanupDone(msg cleanupDoneMsg) tea.Cmd {
	m.cleanupBusy = false
	prefix := "cleanup"
	if msg.dryRun {
		prefix = "dry-run"
	}
	if msg.err != nil {
		m.appendAudit(fmt.Sprintf("failed %s %s reason=%v", prefix, msg.rowPath, msg.err))
		return nil
	}
	if !msg.result.OK {
		m.appendAudit(fmt.Sprintf("blocked %s %s reason=%s", prefix, msg.rowPath, strings.Join(msg.result.Reasons, ",")))
		return nil
	}
	m.appendAudit(fmt.Sprintf("%s %s restore=\"%s\"", prefix, msg.result.DeletedPath, msg.result.RestoreCommand))
	if !msg.dryRun {
		m.removeRow(msg.rowPath)
		m.mode = modeBrowse
		m.confirmInput = ""
		m.assessment = nil
		m.clampSelection(len(m.filtered()))
	}
	return nil
}

func (m *model) removeRow(repoPath string) {
	out := m.rows[:0]
	for _, r := range m.rows {
		if r.RepoPath != repoPath {
			out = append(out, r)
		}
	}
	m.rows = out
}
