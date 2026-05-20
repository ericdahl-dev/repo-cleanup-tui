package view

import (
	"testing"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

func row(overrides func(*scanner.Candidate)) scanner.Candidate {
	c := scanner.Candidate{
		RepoPath:         "/tmp/repo",
		NodeModulesPath:  "/tmp/repo/node_modules",
		Manager:          scanner.ManagerYarn,
		HasLockfile:      true,
		InactiveDays:     intPtr(120),
		Bytes:            100,
		ReinstallCommand: "yarn install --immutable",
		Git:              scanner.GitContext{Branch: "main", Dirty: false},
	}
	if overrides != nil {
		overrides(&c)
	}
	return c
}

func intPtr(n int) *int { return &n }

func TestFilterAndSortSafetyAndInactivity(t *testing.T) {
	rows := []scanner.Candidate{
		row(func(c *scanner.Candidate) {
			c.RepoPath = "/r1"
			c.HasLockfile = false
			c.InactiveDays = intPtr(400)
			c.Bytes = 500
		}),
		row(func(c *scanner.Candidate) {
			c.RepoPath = "/r2"
			c.InactiveDays = intPtr(20)
			c.Bytes = 400
		}),
		row(func(c *scanner.Candidate) {
			c.RepoPath = "/r3"
			c.InactiveDays = intPtr(300)
			c.Bytes = 300
		}),
	}
	result := FilterAndSort(rows, FilterSortOptions{
		MinInactiveDays: 90,
		ShowOnlySafe:    true,
		SortMode:        SortSize,
	})
	if len(result) != 1 || result[0].RepoPath != "/r3" {
		t.Fatalf("got %+v", result)
	}
}

func TestFilterAndSortInactiveDescendingUnknownLast(t *testing.T) {
	rows := []scanner.Candidate{
		row(func(c *scanner.Candidate) { c.RepoPath = "/r1"; c.InactiveDays = intPtr(10); c.Bytes = 300 }),
		row(func(c *scanner.Candidate) { c.RepoPath = "/r2"; c.InactiveDays = nil; c.Bytes = 999 }),
		row(func(c *scanner.Candidate) { c.RepoPath = "/r3"; c.InactiveDays = intPtr(200); c.Bytes = 100 }),
	}
	result := FilterAndSort(rows, FilterSortOptions{SortMode: SortInactive})
	if len(result) != 3 {
		t.Fatalf("len %d", len(result))
	}
	if result[0].RepoPath != "/r3" || result[1].RepoPath != "/r1" || result[2].RepoPath != "/r2" {
		t.Fatalf("order: %v %v %v", result[0].RepoPath, result[1].RepoPath, result[2].RepoPath)
	}
}

func TestFilterAndSortSearch(t *testing.T) {
	rows := []scanner.Candidate{
		row(func(c *scanner.Candidate) {
			c.RepoPath = "/alpha/service-api"
			c.Git.Branch = "main"
		}),
		row(func(c *scanner.Candidate) {
			c.RepoPath = "/beta/web"
			c.Git.Branch = "feature/cleanup"
		}),
	}
	byPath := FilterAndSort(rows, FilterSortOptions{SortMode: SortSize, SearchQuery: "service"})
	byBranch := FilterAndSort(rows, FilterSortOptions{SortMode: SortSize, SearchQuery: "cleanup"})
	if len(byPath) != 1 || byPath[0].RepoPath != "/alpha/service-api" {
		t.Fatalf("byPath: %+v", byPath)
	}
	if len(byBranch) != 1 || byBranch[0].RepoPath != "/beta/web" {
		t.Fatalf("byBranch: %+v", byBranch)
	}
}

func TestFilterAndSortDirtyOnly(t *testing.T) {
	rows := []scanner.Candidate{
		row(func(c *scanner.Candidate) { c.RepoPath = "/clean"; c.Git.Dirty = false }),
		row(func(c *scanner.Candidate) { c.RepoPath = "/dirty"; c.Git.Dirty = true }),
	}
	result := FilterAndSort(rows, FilterSortOptions{
		ShowOnlyDirty: true,
		SortMode:      SortSize,
	})
	if len(result) != 1 || result[0].RepoPath != "/dirty" {
		t.Fatalf("got %+v", result)
	}
}
