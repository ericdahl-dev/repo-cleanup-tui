package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/config"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/ui"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/version"
	"github.com/ericdahl-dev/repo-cleanup-tui/internal/wizard"
)

func printHelp() {
	fmt.Print(`repo-cleanup-tui — find reclaimable node_modules in local git repos

Usage:
  repo-cleanup-tui [path]          Start TUI (default Workspace: cwd or config)
  repo-cleanup-tui tui [path]      Start TUI explicitly
  repo-cleanup-tui init            Write starter config (~/.config/repo-cleanup-tui/config.toml)
  repo-cleanup-tui scan [--json] [path]   Scan Workspace; --json for machine output

Options:
  -h, --help       Show this help
  --version        Print version and exit

Environment:
  REPO_CLEANUP_TUI_DEBUG   When set, emit debug logs to stderr

`)
}

func isVersionFlag(arg string) bool {
	return arg == "--version" || arg == "-version"
}

func printVersion() {
	fmt.Println(version.Line())
}

func looksLikePath(arg string) bool {
	if arg == "" || strings.HasPrefix(arg, "-") {
		return false
	}
	switch arg {
	case "init", "scan", "tui", "help":
		return false
	}
	return strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "~")
}

func expandPath(p string) (string, error) {
	s := strings.TrimSpace(p)
	if strings.HasPrefix(s, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		s = strings.Replace(s, "~", home, 1)
	}
	return filepath.Clean(s), nil
}

func resolveWorkspacePath(args []string, cwd string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return expandPath(args[0])
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return "", err
	}
	if cfg.ActiveWorkspace != "" {
		return cfg.ActiveWorkspace, nil
	}
	return cwd, nil
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "overwrite existing config")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	path := config.ConfigPath()
	if err := wizard.RunInteractive(path, *force); err != nil {
		if errors.Is(err, wizard.ErrUserAborted) {
			return 1
		}
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui init: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
	return 0
}

func runScan(args []string) int {
	asJSON := false
	var pathArgs []string
	for _, a := range args {
		if a == "--json" {
			asJSON = true
			continue
		}
		pathArgs = append(pathArgs, a)
	}
	cwd, _ := os.Getwd()
	root, err := resolveWorkspacePath(pathArgs, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui scan: %v\n", err)
		return 1
	}

	cfg, _ := config.Load(cwd)
	ignore := config.DefaultIgnore
	if cfg != nil {
		ignore = cfg.IgnoreForActive()
	}

	rows, err := scanner.Scan(root, scanner.Options{IgnoreDirs: ignore})
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui scan: %v\n", err)
		return 1
	}

	if asJSON {
		payload := struct {
			RootPath string              `json:"rootPath"`
			Count    int                 `json:"count"`
			Rows     []scanner.Candidate `json:"rows"`
		}{RootPath: root, Count: len(rows), Rows: rows}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "repo-cleanup-tui scan: %v\n", err)
			return 1
		}
		return 0
	}

	for _, row := range rows {
		inactive := "unknown"
		if row.InactiveDays != nil {
			inactive = fmt.Sprintf("%d", *row.InactiveDays)
		}
		fmt.Printf("%s\t%d\t%s\t%s\n", row.RepoPath, row.Bytes, row.Manager, inactive)
	}
	return 0
}

func runTUI(pathArgs []string) int {
	cwd, _ := os.Getwd()
	root, err := resolveWorkspacePath(pathArgs, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui: %v\n", err)
		return 1
	}
	cfg, _ := config.Load(cwd)
	ignore := config.DefaultIgnore
	if cfg != nil {
		ignore = cfg.IgnoreForActive()
	}
	if err := ui.Run(root, ignore, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	args := os.Args[1:]
	for len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		os.Exit(runTUI(nil))
	}

	if len(args) == 1 && isVersionFlag(args[0]) {
		printVersion()
		return
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		return
	case "init":
		os.Exit(runInit(args[1:]))
	case "scan":
		os.Exit(runScan(args[1:]))
	case "tui":
		os.Exit(runTUI(args[1:]))
	default:
		if looksLikePath(args[0]) {
			os.Exit(runTUI(args))
		}
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui: unknown command %q (try --help)\n", args[0])
		os.Exit(2)
	}
}
