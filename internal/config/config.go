package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var DefaultIgnore = []string{
	"node_modules", ".git", ".next", ".nuxt",
	"dist", "build", "coverage",
}

type Workspace struct {
	Path   string   `toml:"path"`
	Ignore []string `toml:"ignore,omitempty"`
}

type Config struct {
	ActiveWorkspace string      `toml:"active_workspace"`
	Workspaces      []Workspace `toml:"workspaces"`
	path            string
}

func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "repo-cleanup-tui", "config.toml")
	}
	return filepath.Join(dir, "repo-cleanup-tui", "config.toml")
}

func legacyJSONPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "repo-cleanup-tui", "config.json")
	}
	return filepath.Join(dir, "repo-cleanup-tui", "config.json")
}

type legacyJSON struct {
	Roots         []string `json:"roots"`
	Ignore        []string `json:"ignore"`
	PreferredRoot *string  `json:"preferredRoot"`
}

func Default(cwd string) *Config {
	resolved := filepath.Clean(cwd)
	return &Config{
		ActiveWorkspace: resolved,
		Workspaces: []Workspace{{
			Path:   resolved,
			Ignore: append([]string(nil), DefaultIgnore...),
		}},
	}
}

func Load(cwd string) (*Config, error) {
	path := ConfigPath()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if migrated, err := migrateLegacyJSON(path, cwd); err != nil {
				return nil, err
			} else if migrated != nil {
				return migrated, nil
			}
			return Default(cwd), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	return loadFile(path)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	normalize(&cfg)
	cfg.path = path
	return &cfg, nil
}

func normalize(cfg *Config) {
	for i := range cfg.Workspaces {
		cfg.Workspaces[i].Path = filepath.Clean(cfg.Workspaces[i].Path)
		if len(cfg.Workspaces[i].Ignore) == 0 {
			cfg.Workspaces[i].Ignore = append([]string(nil), DefaultIgnore...)
		}
	}
	if cfg.ActiveWorkspace != "" {
		cfg.ActiveWorkspace = filepath.Clean(cfg.ActiveWorkspace)
	}
	if cfg.ActiveWorkspace == "" && len(cfg.Workspaces) > 0 {
		cfg.ActiveWorkspace = cfg.Workspaces[0].Path
	}
}

func migrateLegacyJSON(tomlPath, cwd string) (*Config, error) {
	data, err := os.ReadFile(legacyJSONPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
	return migrateFromJSONBytes(tomlPath, data, cwd)
}

func migrateFromJSONBytes(tomlPath string, data []byte, cwd string) (*Config, error) {
	var legacy legacyJSON
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parse legacy config: %w", err)
	}
	roots := legacy.Roots
	if len(roots) == 0 {
		roots = []string{cwd}
	}
	cfg := &Config{Workspaces: make([]Workspace, 0, len(roots))}
	for _, r := range roots {
		cfg.Workspaces = append(cfg.Workspaces, Workspace{
			Path:   filepath.Clean(r),
			Ignore: append([]string(nil), coalesceIgnore(legacy.Ignore)...),
		})
	}
	if legacy.PreferredRoot != nil && *legacy.PreferredRoot != "" {
		cfg.ActiveWorkspace = filepath.Clean(*legacy.PreferredRoot)
	} else if len(cfg.Workspaces) > 0 {
		cfg.ActiveWorkspace = cfg.Workspaces[0].Path
	}
	if err := os.MkdirAll(filepath.Dir(tomlPath), 0750); err != nil {
		return nil, err
	}
	if err := cfg.SaveAt(tomlPath); err != nil {
		return nil, err
	}
	cfg.path = tomlPath
	return cfg, nil
}

func coalesceIgnore(ignore []string) []string {
	if len(ignore) == 0 {
		return DefaultIgnore
	}
	return ignore
}

func (c *Config) Path() string { return c.path }

func (c *Config) Save() error {
	path := c.path
	if path == "" {
		path = ConfigPath()
	}
	return c.SaveAt(path)
}

func (c *Config) SaveAt(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	normalize(c)
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	c.path = path
	return nil
}

func (c *Config) UpsertWorkspace(rootPath string) *Config {
	resolved := filepath.Clean(rootPath)
	out := *c
	out.Workspaces = []Workspace{{Path: resolved, Ignore: append([]string(nil), DefaultIgnore...)}}
	for _, ws := range c.Workspaces {
		if ws.Path == resolved {
			continue
		}
		out.Workspaces = append(out.Workspaces, ws)
	}
	out.ActiveWorkspace = resolved
	return &out
}

func (c *Config) IgnoreForActive() []string {
	for _, ws := range c.Workspaces {
		if ws.Path == c.ActiveWorkspace {
			if len(ws.Ignore) > 0 {
				return ws.Ignore
			}
			break
		}
	}
	return DefaultIgnore
}

// WriteStarter writes a minimal config with one Workspace.
func WriteStarter(path, workspacePath string) error {
	cfg := Default(workspacePath)
	return cfg.SaveAt(path)
}
