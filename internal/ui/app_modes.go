package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) handleSearchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeBrowse
		m.clampSelection(len(m.filtered()))
		return false, nil
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
		m.clampSelection(len(m.filtered()))
		return false, nil
	default:
		if len(msg.Runes) > 0 {
			m.searchQuery += string(msg.Runes)
			m.clampSelection(len(m.filtered()))
		}
	}
	return false, nil
}

func (m *model) handleWorkspaceKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return false, m.applyWorkspace(m.workspaceInput)
	case "backspace":
		if len(m.workspaceInput) > 0 {
			m.workspaceInput = m.workspaceInput[:len(m.workspaceInput)-1]
		}
		return false, nil
	default:
		if len(msg.Runes) > 0 {
			m.workspaceInput += string(msg.Runes)
		}
	}
	return false, nil
}

func (m *model) applyWorkspace(input string) tea.Cmd {
	target := strings.TrimSpace(input)
	if target == "" {
		target = m.root
	}
	target, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		m.appendAudit("blocked workspace change reason=invalid-path")
		m.mode = modeBrowse
		return nil
	}
	updated := m.cfg.UpsertWorkspace(target)
	m.cfg = updated
	if err := m.cfg.Save(); err != nil {
		m.appendAudit("failed save config reason=" + err.Error())
		m.mode = modeBrowse
		return nil
	}
	m.root = target
	m.ignore = m.cfg.IgnoreForActive()
	m.mode = modeBrowse
	m.workspaceInput = ""
	m.searchQuery = ""
	m.confirmInput = ""
	m.assessment = nil
	return m.startRescan()
}
