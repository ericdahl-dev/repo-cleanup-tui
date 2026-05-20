package view

import (
	"strings"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

type SortMode string

const (
	SortSize     SortMode = "size"
	SortInactive SortMode = "inactive"
)

type FilterSortOptions struct {
	MinInactiveDays int
	ShowOnlySafe    bool
	ShowOnlyDirty   bool
	SortMode        SortMode
	SearchQuery     string
}

func FilterAndSort(rows []scanner.Candidate, opts FilterSortOptions) []scanner.Candidate {
	normalizedQuery := strings.TrimSpace(strings.ToLower(opts.SearchQuery))

	var base []scanner.Candidate
	for _, row := range rows {
		inactiveOk := opts.MinInactiveDays == 0 ||
			(row.InactiveDays != nil && *row.InactiveDays >= opts.MinInactiveDays)
		safeOk := !opts.ShowOnlySafe || row.HasLockfile
		dirtyOk := !opts.ShowOnlyDirty || row.Git.Dirty
		searchOk := normalizedQuery == "" ||
			strings.Contains(strings.ToLower(row.RepoPath), normalizedQuery) ||
			strings.Contains(strings.ToLower(row.NodeModulesPath), normalizedQuery) ||
			strings.Contains(strings.ToLower(row.Git.Branch), normalizedQuery)
		if inactiveOk && safeOk && dirtyOk && searchOk {
			base = append(base, row)
		}
	}

	out := append([]scanner.Candidate(nil), base...)
	sortRows(out, opts.SortMode)
	return out
}

func sortRows(rows []scanner.Candidate, mode SortMode) {
	if len(rows) < 2 {
		return
	}
	if mode == SortInactive {
		for i := 0; i < len(rows); i++ {
			for j := i + 1; j < len(rows); j++ {
				if inactiveLess(rows[j], rows[i]) {
					rows[i], rows[j] = rows[j], rows[i]
				}
			}
		}
		return
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Bytes > rows[i].Bytes {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}

func inactiveLess(a, b scanner.Candidate) bool {
	av := inactiveKey(a.InactiveDays)
	bv := inactiveKey(b.InactiveDays)
	return av > bv
}

func inactiveKey(days *int) int {
	if days == nil {
		return -1
	}
	return *days
}
