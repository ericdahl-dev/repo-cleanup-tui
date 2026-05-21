# repo-cleanup-tui

Terminal tool to find reclaimable `node_modules` disk in local git repos, review risk, and delete only when package managers can restore dependencies.

## Language

**Workspace**
The filesystem directory tree the tool scans (e.g. a folder of cloned repos).
_Avoid_: root path, scan path, cwd (when you mean the configured scan tree)

**Candidate**
A git repository under a Workspace that has `package.json` and a `node_modules` directory worth listing.
_Avoid_: repo row, hit, match

**Reclaimable footprint**
Disk bytes used by a Candidate's `node_modules` folder.
_Avoid_: size, weight, space (without "reclaimable")

**Manager**
The package manager inferred from lockfiles (`yarn`, `pnpm`, `npm`, `bun`, or `unknown`).
_Avoid_: package manager (spell out only in docs), PM

**Restore command**
The install command that can recreate `node_modules` after cleanup (e.g. `yarn install --immutable`).
_Avoid_: reinstall script, install hint

**Safe-only filter**
UI filter that hides Candidates without a lockfile (cleanup not considered safe).
_Avoid_: lockfile mode, k toggle name in UI

**Dirty-only filter**
**Browse view** filter that hides **Candidates** whose working tree is clean (shows only repos with uncommitted or unstaged changes); does not gate **Cleanup**. Off by default when **Browse view** opens.
_Avoid_: dirty-only mode, d toggle name in UI

**Cleanup**
Deleting a Candidate's `node_modules` after gates pass, with a logged **Restore command**.
_Avoid_: purge, wipe, uninstall

**Cleanup preview**
Screen flow after choosing a Candidate: shows risk, restore command, and optional dry-run before confirm.
_Avoid_: modal, dialog

**Confirm token**
Exact phrase the user must type to authorize **Cleanup** (includes the repo folder name).
_Avoid_: confirmation string, delete password

**Dry-run**
Cleanup path that records what would happen without deleting `node_modules`.
_Avoid_: simulate, preview-only (prefer **Cleanup preview** for UI step)

**Config file**
User-managed file at `~/.config/repo-cleanup-tui/config.toml` listing **Workspace** paths, ignore dirs, and the active **Workspace** for the TUI.
_Avoid_: config.json, settings file, preferences

**Scan**
A full pass over a **Workspace** that finds **Candidates** and measures each **Reclaimable footprint** (no reuse of prior scan results in MVP).
_Avoid_: refresh, rescan (use only for user-triggered repeat **Scan**)

**Discovery phase**
First part of a **Scan**: walk the **Workspace** tree and identify **Candidates** before sizes are known.
_Avoid_: crawl, walk

**Sizing phase**
Second part of a **Scan**: measure **Reclaimable footprint** for each discovered **Candidate**.
_Avoid_: stat phase, du phase

**Browse view**
Primary TUI screen: **Candidate** list, filters, and **Scan** progress.
_Avoid_: main screen, dashboard (reserved for CI tools like git-green)

**Workspace manager**
Dedicated TUI screen to add, edit ignores, remove, and activate **Workspace** entries in the **Config file** (`m` from **Browse view**); changes persist immediately.
_Avoid_: settings screen, config editor

**Help overlay**
Toggleable keybinding reference (`?`), separate from **Browse view** and **Cleanup preview**.
_Avoid_: help screen, docs mode

**Git context**
Per-**Candidate** branch name, whether the working tree is **dirty**, and ahead/behind vs upstream (read from the local repo via embedded git library, not GitHub API).
_Avoid_: git status, repo status, shell git

**Dirty**
Working tree has uncommitted or unstaged changes; does not include unpushed commits or ahead/behind alone.
_Avoid_: modified, WIP (without defining worktree), unpushed

## Relationships

- A **Workspace** contains zero or more **Candidates**
- Each **Candidate** has exactly one **Manager**, one **Reclaimable footprint**, and one **Restore command**
- **Cleanup** applies to at most one **Candidate** at a time and requires passing safety gates and **Confirm token**
- **Dry-run** is optional within **Cleanup preview** before real **Cleanup**
- **Dirty-only filter** applies only in **Browse view**; **Cleanup** ignores **Dirty**
- **Dirty-only filter** does not toggle git column visibility (branch/dirty columns remain user-controlled)

## Example dialogue

> **Dev:** "Can I delete this repo's deps?"
> **User:** "Only if it's a **Candidate** with a lockfile, you pass **Safe-only**, and you type the **Confirm token** after **Cleanup preview**."

> **Dev:** "What does the scanner show?"
> **User:** "Every **Candidate** under the **Workspace**, sorted by **Reclaimable footprint** or inactivity, with **Manager** and **Restore command**."

## Flagged ambiguities

- "repo" — in UI/logs often means **Candidate** (one git repo), not the whole **Workspace**.
- Go migration scope — resolved: **MVP first** (scan, list, filters, cleanup gates, `scan --json`, `init`); full TS parity later.
- Config format — resolved: **TOML** at `~/.config/repo-cleanup-tui/config.toml`; optional one-time read of legacy JSON.
- Codebase strategy — resolved: **Go-only** (`main.go`, `internal/`); TypeScript/Ink stack removed after MVP port.
- Distribution — resolved: match **git-green** (Goreleaser, golangci, release workflow, `ericdahl-dev/tap`, `go install`).
- Scan cache — resolved: **none in MVP**; every **Scan** is full; `r` forces repeat **Scan**. Disk cache later if needed.
- Scan progress UX — resolved: **two progress bars** (**Discovery phase** + **Sizing phase**); stream **Candidates** into list as sizing completes.
- TUI modes (MVP) — resolved: **Browse view**, **Cleanup preview**, confirm token, path search, **Workspace** switch, **Help overlay**.
- Go module — resolved: `github.com/ericdahl-dev/repo-cleanup-tui`.
- `init` — resolved: **Huh** wizard writes starter **Config file**; `--force` overwrite; MVP one **Workspace** + default ignores.
- **Git context** — resolved: **`go-git`** in-process (self-contained binary; no `git` on PATH required). No upstream → ahead/behind zero. Inactivity from last commit on default branch/HEAD.
- `internal/` layout — resolved: `config`, `scanner`, `cleanup`, `view`, `ui`, `wizard`, `logx` (optional debug).
- Tests (MVP) — resolved: port core table tests (cleanup, view, scanner); TDD during build; fixture repos may use shell `git` only in tests.
