package cleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

func row(overrides func(*scanner.Candidate)) scanner.Candidate {
	repoPath := "/tmp/demo-repo"
	c := scanner.Candidate{
		RepoPath:         repoPath,
		NodeModulesPath:  filepath.Join(repoPath, "node_modules"),
		Manager:          scanner.ManagerYarn,
		HasLockfile:      true,
		InactiveDays:     intPtr(120),
		Bytes:            1234,
		ReinstallCommand: "yarn install --immutable",
		Git:              scanner.GitContext{Branch: "main", Dirty: false},
	}
	if overrides != nil {
		overrides(&c)
	}
	return c
}

func intPtr(v int) *int { return &v }

func TestGuardPassesForRepoNodeModulesWithLockfile(t *testing.T) {
	guard := EvaluateGuards(row(nil))
	if !guard.OK {
		t.Fatalf("expected ok guard, reasons=%v", guard.Reasons)
	}
}

func TestGuardFailsWhenLockfileMissing(t *testing.T) {
	guard := EvaluateGuards(row(func(c *scanner.Candidate) { c.HasLockfile = false }))
	if guard.OK {
		t.Fatal("expected guard failure")
	}
	if !strings.Contains(strings.Join(guard.Reasons, ","), "missing lockfile") {
		t.Fatalf("reasons=%v", guard.Reasons)
	}
}

func TestGuardFailsWhenTargetNotNodeModules(t *testing.T) {
	guard := EvaluateGuards(row(func(c *scanner.Candidate) {
		c.NodeModulesPath = "/tmp/demo-repo/dist"
	}))
	if guard.OK || !strings.Contains(strings.Join(guard.Reasons, ","), "target must be node_modules") {
		t.Fatalf("reasons=%v", guard.Reasons)
	}
}

func TestGuardFailsWhenTargetOutsideRepo(t *testing.T) {
	guard := EvaluateGuards(row(func(c *scanner.Candidate) {
		c.NodeModulesPath = "/tmp/other/node_modules"
	}))
	if guard.OK || !strings.Contains(strings.Join(guard.Reasons, ","), "target must be inside repo") {
		t.Fatalf("reasons=%v", guard.Reasons)
	}
}

func TestGuardFailsWhenTargetNotExactRepoNodeModules(t *testing.T) {
	guard := EvaluateGuards(row(func(c *scanner.Candidate) {
		c.NodeModulesPath = "/tmp/demo-repo/sub/node_modules"
	}))
	if guard.OK || !strings.Contains(strings.Join(guard.Reasons, ","), "target must match repo/node_modules") {
		t.Fatalf("reasons=%v", guard.Reasons)
	}
}

func TestDryRunExecutesWithoutTouchingFS(t *testing.T) {
	result, err := Execute(row(nil), true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("result=%+v", result)
	}
	if result.DeletedPath != "/tmp/demo-repo/node_modules" {
		t.Fatalf("deletedPath=%q", result.DeletedPath)
	}
}

func TestConfirmTokenIsExplicitAndRepoScoped(t *testing.T) {
	got := BuildConfirmToken(row(nil))
	want := "DELETE_NODE_MODULES demo-repo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSafetyBlocksUnknownPackageManager(t *testing.T) {
	assessment, err := AssessSafety(row(func(c *scanner.Candidate) {
		c.Manager = scanner.ManagerUnknown
		c.HasLockfile = false
	}))
	if err != nil {
		t.Fatal(err)
	}
	if assessment.OK || assessment.RiskLevel != RiskHigh {
		t.Fatalf("assessment=%+v", assessment)
	}
	if !strings.Contains(strings.Join(assessment.Reasons, ","), "unknown package manager") {
		t.Fatalf("reasons=%v", assessment.Reasons)
	}
}

func TestSafetyBlocksYarnZeroInstallCache(t *testing.T) {
	tempRoot := t.TempDir()
	repoPath := filepath.Join(tempRoot, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".yarn", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	assessment, err := AssessSafety(row(func(c *scanner.Candidate) {
		c.RepoPath = repoPath
		c.NodeModulesPath = filepath.Join(repoPath, "node_modules")
		c.Manager = scanner.ManagerYarn
		c.HasLockfile = true
	}))
	if err != nil {
		t.Fatal(err)
	}
	if assessment.OK || !strings.Contains(strings.Join(assessment.Reasons, ","), ".yarn/cache") {
		t.Fatalf("assessment=%+v", assessment)
	}
}

func TestSafetyWarnsForRecentlyActiveRepo(t *testing.T) {
	assessment, err := AssessSafety(row(func(c *scanner.Candidate) {
		c.InactiveDays = intPtr(2)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.OK || assessment.RiskLevel != RiskMedium {
		t.Fatalf("assessment=%+v", assessment)
	}
	if !strings.Contains(strings.Join(assessment.Warnings, ","), "recently active") {
		t.Fatalf("warnings=%v", assessment.Warnings)
	}
}
