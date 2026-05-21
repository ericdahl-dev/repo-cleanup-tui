# repo-cleanup-tui

![repo-cleanup-tui app rendering](docs/repo-cleanup-tui-render.svg)

TUI scanner for reclaimable Node disk usage in repo folders.

## Goal

Find `node_modules` in git repos, sort by reclaimable size, filter by inactivity, and only suggest cleanup that package managers can safely restore.

## Install

### Homebrew

```bash
brew install ericdahl-dev/tap/repo-cleanup-tui
```

Formula updates on each `v*` tag via GoReleaser ([homebrew-tap](https://github.com/ericdahl-dev/homebrew-tap)).

### Go

Requires [Go 1.24+](https://go.dev/dl/). If your installed `go` is older, the toolchain will auto-download a newer one (that message is normal).

```bash
go install github.com/ericdahl-dev/repo-cleanup-tui@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH` (e.g. `~/go/bin`).

Prefer `@latest` or `v0.1.2+`. Do not use `v0.1.0` for `go install` — that tag was published while the repo was private and remains broken in the public Go module proxy.

### From source

```bash
git clone https://github.com/ericdahl-dev/repo-cleanup-tui.git
cd repo-cleanup-tui
go build -o repo-cleanup-tui .
```

## Run

```bash
repo-cleanup-tui                  # TUI for cwd or configured workspace
repo-cleanup-tui /path/to/repos   # TUI for a specific root
repo-cleanup-tui init             # write ~/.config/repo-cleanup-tui/config.toml
repo-cleanup-tui scan --json .    # machine-readable scan
```

## Commands

| Command | Description |
|---------|-------------|
| `repo-cleanup-tui [path]` | Start TUI (default workspace: cwd or config) |
| `repo-cleanup-tui tui [path]` | Start TUI explicitly |
| `repo-cleanup-tui init` | Interactive config wizard |
| `repo-cleanup-tui init --force` | Overwrite existing config |
| `repo-cleanup-tui scan [--json] [path]` | Scan workspace; `--json` prints candidates |

## Controls

- `q` / `esc`: quit (or back from search/workspace/preview)
- `?`: toggle help overlay
- `s`: toggle sort (`size` / `inactive`)
- `f`: inactivity filter (`all` → `>=30d` → `>=90d` → `>=180d`)
- `k`: safe-only toggle (lockfile required)
- `d`: dirty-only toggle
- `g`: toggle git context columns
- `j` / `u` (or arrows): move selection
- `r`: full rescan
- `/`: search repo path or branch; `c` clear search
- `w`: switch workspace (saved to config, then rescan)
- `x`: cleanup preview → `p` dry-run, `y` confirm, token + Enter to delete `node_modules`

## Development

```bash
go test -race ./...
go build -o repo-cleanup-tui .
golangci-lint run   # optional; matches CI
```

## Release

Tag a version to build cross-platform archives, checksums, GitHub release assets, and update the Homebrew formula:

```bash
git tag v0.1.2
git push origin v0.1.2
```

The [release workflow](.github/workflows/release.yml) runs GoReleaser. Set repository secret `HOMEBREW_TAP_GITHUB_TOKEN` (PAT with push access to `ericdahl-dev/homebrew-tap`) so the tap formula updates automatically.

## Safety behavior

- Scans only repos that contain `.git`, `package.json`, and `node_modules`.
- Detects package manager via lockfile.
- Shows restore command (`yarn install --immutable`, `pnpm install --frozen-lockfile`, `npm ci`, `bun install --frozen-lockfile`).
- Cleanup is gated behind `x` → preview → typed confirmation token.
- Preview supports dry-run before deletion.
- Guards block deletion unless lockfile exists and target is exact `repo/node_modules`.
- Manager-aware risk checks block unsafe cleanup (unknown manager, Yarn zero-install cache, missing lockfile).
- Audit log prints each dry-run/delete/block event to stderr.

## Trust posture

- local-first and no telemetry
- explicit user-triggered actions only
- safety gates before deletion
