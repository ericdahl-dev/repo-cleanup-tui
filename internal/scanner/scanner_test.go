package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
}

func writeRepo(t *testing.T, root, name string, lockfiles []string, commitDate string) string {
	t.Helper()
	repo := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(repo, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, lf := range lockfiles {
		if err := os.WriteFile(filepath.Join(repo, lf), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "node_modules", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	cmd := exec.Command("git", "commit", "-m", "init")
	cmd.Dir = repo
	if commitDate != "" {
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+commitDate, "GIT_COMMITTER_DATE="+commitDate)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s %v", out, err)
	}
	return repo
}

func TestDetectManagerLockfilePriority(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "multi", []string{"yarn.lock", "package-lock.json"}, "")

	rows, err := Scan(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows: %d", len(rows))
	}
	if rows[0].Manager != ManagerYarn || !rows[0].HasLockfile {
		t.Fatalf("manager: %+v", rows[0])
	}
	if rows[0].ReinstallCommand != "yarn install --immutable" {
		t.Fatalf("restore: %q", rows[0].ReinstallCommand)
	}
	if rows[0].Git.Branch == "" {
		t.Fatal("expected branch name")
	}
	if rows[0].Git.Ahead != 0 || rows[0].Git.Behind != 0 {
		t.Fatalf("ahead/behind: %+v", rows[0].Git)
	}
	if rows[0].Bytes <= 0 {
		t.Fatal("expected bytes > 0")
	}
}

func TestInactiveDaysFromCommit(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "old", []string{"pnpm-lock.yaml"}, "2020-01-01T00:00:00Z")

	rows, err := Scan(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows: %d", len(rows))
	}
	if rows[0].Manager != ManagerPnpm {
		t.Fatalf("manager: %q", rows[0].Manager)
	}
	if rows[0].InactiveDays == nil || *rows[0].InactiveDays < 365 {
		t.Fatalf("inactiveDays: %v", rows[0].InactiveDays)
	}
}

func TestScanProgress(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, "prog", []string{"yarn.lock"}, "")

	var events int
	_, err := Scan(root, Options{
		OnProgress: func(p Progress) {
			if p.DirectoriesScanned > 0 {
				events++
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if events == 0 {
		t.Fatal("expected progress events")
	}
}

func TestScanFoundAndSizedCallbacks(t *testing.T) {
	root := t.TempDir()
	repoA := writeRepo(t, root, "a", []string{"yarn.lock"}, "")
	repoB := writeRepo(t, root, "b", []string{"pnpm-lock.yaml"}, "")

	var (
		mu    sync.Mutex
		found []string
		sized []int64
	)
	rows, err := Scan(root, Options{
		OnFound: func(c Candidate) {
			mu.Lock()
			found = append(found, c.RepoPath)
			mu.Unlock()
		},
		OnSized: func(c Candidate) {
			mu.Lock()
			sized = append(sized, c.Bytes)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(found) != 2 {
		t.Fatalf("rows=%d found=%d", len(rows), len(found))
	}
	if !contains(found, repoA) || !contains(found, repoB) {
		t.Fatalf("found: %v", found)
	}
	if len(sized) < 2 {
		t.Fatalf("sized: %v", sized)
	}
	for _, b := range sized {
		if b <= 0 {
			t.Fatal("expected positive size")
		}
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
