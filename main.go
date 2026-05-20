package main

import (
	"fmt"
	"os"
	"strings"
)

const notImplemented = "not implemented yet (see GitHub issue #5–#7)"

func printHelp() {
	fmt.Print(`repo-cleanup-tui — find reclaimable node_modules in local git repos

Usage:
  repo-cleanup-tui [path]          Start TUI (default Workspace: cwd or config)
  repo-cleanup-tui tui [path]      Start TUI explicitly
  repo-cleanup-tui init            Write starter config (~/.config/repo-cleanup-tui/config.toml)
  repo-cleanup-tui scan [--json] [path]   Scan Workspace; --json for machine output

Options:
  -h, --help    Show this help

Environment:
  REPO_CLEANUP_TUI_DEBUG   When set, emit debug logs to stderr

`)
}

func looksLikePath(arg string) bool {
	if arg == "" {
		return false
	}
	if strings.HasPrefix(arg, "-") {
		return false
	}
	switch arg {
	case "init", "scan", "tui", "help":
		return false
	}
	return strings.Contains(arg, "/") || strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "~")
}

func stub(cmd string) {
	fmt.Fprintf(os.Stderr, "repo-cleanup-tui %s: %s\n", cmd, notImplemented)
	os.Exit(2)
}

func runDefault(args []string) {
	if len(args) > 0 && looksLikePath(args[0]) {
		stub("tui")
	}
	stub("tui")
}

func main() {
	args := os.Args[1:]
	for len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		runDefault(nil)
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		return
	case "init":
		stub("init")
	case "scan":
		stub("scan")
	case "tui":
		if len(args) > 1 {
			_ = args[1]
		}
		stub("tui")
	default:
		if looksLikePath(args[0]) {
			runDefault(args)
		}
		fmt.Fprintf(os.Stderr, "repo-cleanup-tui: unknown command %q (try --help)\n", args[0])
		os.Exit(2)
	}
}
