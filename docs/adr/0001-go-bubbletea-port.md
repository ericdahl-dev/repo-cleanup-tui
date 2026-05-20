# Go + Bubble Tea port (self-contained binary)

repo-cleanup-tui is being rewritten in Go with the Charm stack (Bubble Tea, Lip Gloss, Bubbles, Huh, Glamour) to match git-green: a single distributable binary, TOML config under `~/.config/repo-cleanup-tui/`, and Goreleaser/Homebrew release plumbing. TypeScript/Ink is removed once the Go MVP ships (in-place rewrite, not a long-lived dual codebase).

Git metadata for each Candidate uses **go-git** in-process so the tool does not depend on a `git` executable on PATH. Ahead/behind counts are best-effort when no upstream is configured (same as today: treat as zero). Local filesystem scanning and `node_modules` deletion stay in Go stdlib.

Alternatives considered: keep Node/Ink (runtime + yarn friction), Rust/Ratatui, shelling out to `git` (simpler but violates self-contained goal). Bubble Tea was chosen for parity with git-green and mature terminal UX primitives.
