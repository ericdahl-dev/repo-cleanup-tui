package ui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/config"
)

func (m *model) ensureConfig() *config.Config {
	if m.cfg != nil {
		return m.cfg
	}
	m.cfg = config.Default(m.root)
	return m.cfg
}

func (m *model) activeWorkspaceIndex() int {
	cfg := m.ensureConfig()
	for i, ws := range cfg.Workspaces {
		if ws.Path == cfg.ActiveWorkspace {
			return i
		}
	}
	return 0
}

func (m *model) openWorkspaceManager() {
	m.ensureConfig()
	m.mode = modeWorkspaceManager
	m.wsMgrPane = wsMgrList
	m.wsMgrSel = m.activeWorkspaceIndex()
	m.wsMgrInput = ""
	m.clampWorkspaceSelection()
}

func (m *model) clampWorkspaceSelection() {
	cfg := m.ensureConfig()
	n := len(cfg.Workspaces)
	if n == 0 {
		m.wsMgrSel = 0
		return
	}
	if m.wsMgrSel >= n {
		m.wsMgrSel = n - 1
	}
	if m.wsMgrSel < 0 {
		m.wsMgrSel = 0
	}
}

func (m *model) saveConfig(cfg *config.Config) error {
	m.cfg = cfg
	if err := cfg.Save(); err != nil {
		return err
	}
	if cfg.ActiveWorkspace == m.root {
		m.ignore = cfg.IgnoreForActive()
	}
	return nil
}

func (m *model) leaveWorkspaceManager() tea.Cmd {
	m.mode = modeBrowse
	m.wsMgrPane = wsMgrList
	m.wsMgrInput = ""
	if m.wsPendingRescan {
		m.wsPendingRescan = false
		return m.startRescan()
	}
	return nil
}

func (m *model) selectedWorkspace() (config.Workspace, bool) {
	cfg := m.ensureConfig()
	if m.wsMgrSel < 0 || m.wsMgrSel >= len(cfg.Workspaces) {
		return config.Workspace{}, false
	}
	return cfg.Workspaces[m.wsMgrSel], true
}

func (m *model) markActiveWorkspaceChanged() {
	m.wsPendingRescan = true
}

func (m *model) activateWorkspacePath(path string) error {
	updated, err := m.ensureConfig().SetActive(path)
	if err != nil {
		return err
	}
	if updated.ActiveWorkspace == m.root {
		if err := m.saveConfig(updated); err != nil {
			return err
		}
		return nil
	}
	if err := m.saveConfig(updated); err != nil {
		return err
	}
	m.root = updated.ActiveWorkspace
	m.ignore = updated.IgnoreForActive()
	m.markActiveWorkspaceChanged()
	return nil
}

func (m *model) handleWorkspaceManagerKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch m.wsMgrPane {
	case wsMgrAddPath:
		return m.handleWorkspaceManagerAddKey(msg)
	case wsMgrEditIgnores:
		return m.handleWorkspaceManagerEditKey(msg)
	default:
		return m.handleWorkspaceManagerListKey(msg)
	}
}

func (m *model) handleWorkspaceManagerListKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	cfg := m.ensureConfig()
	switch msg.String() {
	case "down", "j":
		if m.wsMgrSel < len(cfg.Workspaces)-1 {
			m.wsMgrSel++
		}
	case "up", "u":
		if m.wsMgrSel > 0 {
			m.wsMgrSel--
		}
	case "enter":
		ws, ok := m.selectedWorkspace()
		if !ok {
			return false, nil
		}
		if err := m.activateWorkspacePath(ws.Path); err != nil {
			m.appendAudit("workspace activate failed reason=" + err.Error())
			return false, nil
		}
		return false, m.leaveWorkspaceManager()
	case "a":
		m.wsMgrPane = wsMgrAddPath
		m.wsMgrInput = ""
	case "i":
		ws, ok := m.selectedWorkspace()
		if !ok {
			return false, nil
		}
		m.wsMgrPane = wsMgrEditIgnores
		m.wsMgrInput = config.FormatIgnoreList(ws.Ignore)
	case "delete", "backspace":
		ws, ok := m.selectedWorkspace()
		if !ok {
			return false, nil
		}
		updated, err := cfg.RemoveWorkspace(ws.Path)
		if err != nil {
			if errors.Is(err, config.ErrLastWorkspace) {
				m.appendAudit("workspace remove blocked reason=last-entry")
			} else {
				m.appendAudit("workspace remove failed reason=" + err.Error())
			}
			return false, nil
		}
		if ws.Path == m.root || updated.ActiveWorkspace != m.root {
			m.root = updated.ActiveWorkspace
			m.ignore = updated.IgnoreForActive()
			m.markActiveWorkspaceChanged()
		}
		if err := m.saveConfig(updated); err != nil {
			m.appendAudit("failed save config reason=" + err.Error())
			return false, nil
		}
		m.clampWorkspaceSelection()
	default:
		if len(msg.Runes) > 0 {
			return false, nil
		}
	}
	return false, nil
}

func (m *model) handleWorkspaceManagerAddKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter":
		prev := len(m.ensureConfig().Workspaces)
		updated, err := m.ensureConfig().AddWorkspace(m.wsMgrInput)
		if err != nil {
			m.appendAudit("workspace add failed reason=" + err.Error())
			m.wsMgrPane = wsMgrList
			m.wsMgrInput = ""
			return false, nil
		}
		if err := m.saveConfig(updated); err != nil {
			m.appendAudit("failed save config reason=" + err.Error())
			m.wsMgrPane = wsMgrList
			return false, nil
		}
		if len(updated.Workspaces) > prev {
			m.wsMgrSel = len(updated.Workspaces) - 1
		}
		m.wsMgrPane = wsMgrList
		m.wsMgrInput = ""
		m.clampWorkspaceSelection()
	case "backspace":
		if len(m.wsMgrInput) > 0 {
			m.wsMgrInput = m.wsMgrInput[:len(m.wsMgrInput)-1]
		}
	default:
		if len(msg.Runes) > 0 {
			m.wsMgrInput += string(msg.Runes)
		}
	}
	return false, nil
}

func (m *model) handleWorkspaceManagerEditKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	ws, ok := m.selectedWorkspace()
	if !ok {
		m.wsMgrPane = wsMgrList
		return false, nil
	}
	switch msg.String() {
	case "enter":
		ignore := config.ParseIgnoreList(m.wsMgrInput)
		updated, err := m.ensureConfig().SetWorkspaceIgnores(ws.Path, ignore)
		if err != nil {
			m.appendAudit("workspace ignores failed reason=" + err.Error())
			m.wsMgrPane = wsMgrList
			m.wsMgrInput = ""
			return false, nil
		}
		if err := m.saveConfig(updated); err != nil {
			m.appendAudit("failed save config reason=" + err.Error())
			m.wsMgrPane = wsMgrList
			return false, nil
		}
		m.wsMgrPane = wsMgrList
		m.wsMgrInput = ""
	case "backspace":
		if len(m.wsMgrInput) > 0 {
			m.wsMgrInput = m.wsMgrInput[:len(m.wsMgrInput)-1]
		}
	default:
		if len(msg.Runes) > 0 {
			m.wsMgrInput += string(msg.Runes)
		}
	}
	return false, nil
}
