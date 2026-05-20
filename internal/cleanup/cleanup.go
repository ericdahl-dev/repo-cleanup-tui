package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

type Guard struct {
	OK      bool
	Reasons []string
}

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type Assessment struct {
	OK         bool
	Blocked    bool
	Reasons    []string
	Warnings   []string
	RiskLevel  RiskLevel
	Confidence Confidence
}

type Result struct {
	OK              bool
	DryRun          bool
	DeletedPath     string
	RestoreCommand  string
	Reasons         []string
	RiskLevel       RiskLevel
	Confidence      Confidence
}

func BuildConfirmToken(row scanner.Candidate) string {
	return fmt.Sprintf("DELETE_NODE_MODULES %s", filepath.Base(row.RepoPath))
}

func EvaluateGuards(row scanner.Candidate) Guard {
	var reasons []string
	normalizedRepo, err := filepath.Abs(row.RepoPath)
	if err != nil {
		normalizedRepo = row.RepoPath
	}
	normalizedNodeModules, err := filepath.Abs(row.NodeModulesPath)
	if err != nil {
		normalizedNodeModules = row.NodeModulesPath
	}
	allowed := filepath.Join(normalizedRepo, "node_modules")
	inRepo := strings.HasPrefix(normalizedNodeModules, normalizedRepo+string(filepath.Separator))

	if !row.HasLockfile {
		reasons = append(reasons, "missing lockfile")
	}
	if filepath.Base(normalizedNodeModules) != "node_modules" {
		reasons = append(reasons, "target must be node_modules")
	}
	if !inRepo {
		reasons = append(reasons, "target must be inside repo")
	}
	if normalizedNodeModules != allowed {
		reasons = append(reasons, "target must match repo/node_modules")
	}

	return Guard{OK: len(reasons) == 0, Reasons: reasons}
}

func AssessSafety(row scanner.Candidate) (Assessment, error) {
	guard := EvaluateGuards(row)
	reasons := append([]string{}, guard.Reasons...)
	warnings := []string{}
	blocked := !guard.OK

	if row.Manager == scanner.ManagerUnknown {
		reasons = append(reasons, "unknown package manager")
		blocked = true
	}

	if row.Manager == scanner.ManagerYarn {
		yarnCache := filepath.Join(row.RepoPath, ".yarn", "cache")
		if _, err := os.Stat(yarnCache); err == nil {
			reasons = append(reasons, "yarn zero-install cache detected (.yarn/cache)")
			blocked = true
		}
	}

	if !row.HasLockfile {
		reasons = append(reasons, "missing lockfile")
		blocked = true
	}

	if row.InactiveDays != nil && *row.InactiveDays < 7 {
		warnings = append(warnings, "recently active repo (<7d)")
	}

	risk := riskLevel(blocked, warnings)
	conf := confidence(row, blocked)

	return Assessment{
		OK:         !blocked,
		Blocked:    blocked,
		Reasons:    uniqueStrings(reasons),
		Warnings:   warnings,
		RiskLevel:  risk,
		Confidence: conf,
	}, nil
}

func Execute(row scanner.Candidate, dryRun bool) (Result, error) {
	assessment, err := AssessSafety(row)
	if err != nil {
		return Result{}, err
	}
	if !assessment.OK {
		return Result{
			OK:             false,
			DryRun:         dryRun,
			DeletedPath:    row.NodeModulesPath,
			RestoreCommand: row.ReinstallCommand,
			Reasons:        assessment.Reasons,
			RiskLevel:      assessment.RiskLevel,
			Confidence:     assessment.Confidence,
		}, nil
	}

	if !dryRun {
		if err := os.RemoveAll(row.NodeModulesPath); err != nil {
			return Result{}, err
		}
	}

	return Result{
		OK:             true,
		DryRun:         dryRun,
		DeletedPath:    row.NodeModulesPath,
		RestoreCommand: row.ReinstallCommand,
		Reasons:        nil,
		RiskLevel:      assessment.RiskLevel,
		Confidence:     assessment.Confidence,
	}, nil
}

func riskLevel(blocked bool, warnings []string) RiskLevel {
	if blocked {
		return RiskHigh
	}
	if len(warnings) > 0 {
		return RiskMedium
	}
	return RiskLow
}

func confidence(row scanner.Candidate, blocked bool) Confidence {
	if blocked {
		return ConfidenceHigh
	}
	if row.Manager == scanner.ManagerUnknown {
		return ConfidenceLow
	}
	if !row.HasLockfile {
		return ConfidenceMedium
	}
	return ConfidenceHigh
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
