package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpsertWorkspacePrependsAndSetsActive(t *testing.T) {
	cfg := Default("/tmp/one")
	updated := cfg.UpsertWorkspace("/tmp/two")
	if updated.ActiveWorkspace != "/tmp/two" {
		t.Fatalf("active: got %q", updated.ActiveWorkspace)
	}
	if updated.Workspaces[0].Path != "/tmp/two" {
		t.Fatalf("first workspace: got %q", updated.Workspaces[0].Path)
	}
	found := false
	for _, ws := range updated.Workspaces {
		if ws.Path == "/tmp/one" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected /tmp/one in workspaces")
	}
}

func TestMigrateLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	jsonPath := filepath.Join(dir, "config.json")
	preferred := "/tmp/preferred"
	legacy := map[string]any{
		"roots":         []string{"/tmp/a", "/tmp/b"},
		"ignore":        []string{"node_modules", ".git"},
		"preferredRoot": preferred,
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	// migrateLegacyJSON reads fixed legacy path; test via copy helper
	cfg, err := migrateFromJSONBytes(tomlPath, raw, "/tmp/cwd")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != preferred {
		t.Fatalf("active: got %q", cfg.ActiveWorkspace)
	}
	if len(cfg.Workspaces) != 2 {
		t.Fatalf("workspaces: got %d", len(cfg.Workspaces))
	}
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Default("/scan/root")
	cfg.ActiveWorkspace = "/scan/root"
	if err := cfg.SaveAt(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ActiveWorkspace != "/scan/root" {
		t.Fatalf("active: got %q", loaded.ActiveWorkspace)
	}
	if len(loaded.Workspaces) != 1 || loaded.Workspaces[0].Path != "/scan/root" {
		t.Fatalf("workspaces: %+v", loaded.Workspaces)
	}
}
