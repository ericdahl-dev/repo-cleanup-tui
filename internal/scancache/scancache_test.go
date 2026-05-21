package scancache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

func writeRepo(t *testing.T, root, name string) string {
	t.Helper()
	repo := filepath.Join(root, name)
	for _, p := range []string{".git", "node_modules"} {
		if err := os.MkdirAll(filepath.Join(repo, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "yarn.lock"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestSaveLoadHit(t *testing.T) {
	root := t.TempDir()
	repo := writeRepo(t, root, "a")
	rows := []scanner.Candidate{{
		RepoPath:        repo,
		NodeModulesPath: filepath.Join(repo, "node_modules"),
		Manager:         scanner.ManagerYarn,
		HasLockfile:     true,
		Bytes:           1024,
	}}
	if err := Save(root, rows); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root, DefaultTTL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].RepoPath != repo {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadMissTTLExpired(t *testing.T) {
	root := t.TempDir()
	repo := writeRepo(t, root, "a")
	rows := []scanner.Candidate{{
		RepoPath:        repo,
		NodeModulesPath: filepath.Join(repo, "node_modules"),
		Manager:         scanner.ManagerYarn,
		HasLockfile:     true,
	}}
	if err := Save(root, rows); err != nil {
		t.Fatal(err)
	}
	path := Path(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatal(err)
	}
	cf.SavedAt = time.Now().UTC().Add(-2 * DefaultTTL)
	data, err = json.MarshalIndent(cf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultTTL); err != ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

func TestLoadMissSignatureChange(t *testing.T) {
	root := t.TempDir()
	repo := writeRepo(t, root, "a")
	rows := []scanner.Candidate{{
		RepoPath:        repo,
		NodeModulesPath: filepath.Join(repo, "node_modules"),
		Manager:         scanner.ManagerYarn,
		HasLockfile:     true,
	}}
	if err := Save(root, rows); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"changed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultTTL); err != ErrSignature {
		t.Fatalf("want ErrSignature, got %v", err)
	}
}

func TestLoadMissCorrupt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(Path(root), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultTTL); err != ErrCorrupt {
		t.Fatalf("want ErrCorrupt, got %v", err)
	}
}

func TestLoadMissUnsupportedVersion(t *testing.T) {
	root := t.TempDir()
	cf := cacheFile{Version: 99, Root: root, SavedAt: time.Now().UTC()}
	data, _ := json.Marshal(cf)
	if err := os.WriteFile(Path(root), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultTTL); err != ErrVersion {
		t.Fatalf("want ErrVersion, got %v", err)
	}
}
