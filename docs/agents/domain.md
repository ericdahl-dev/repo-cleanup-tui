# Domain docs

## Layout

This repository uses a **single-context** domain documentation layout.

- Root context file: `CONTEXT.md`
- ADR directory: `docs/adr/`

## Consumer rules for skills

When a skill needs domain or architecture context:

1. Read `CONTEXT.md` first (domain language, goals, constraints).
2. Read ADRs in `docs/adr/` for architectural decisions and rationale.
3. If conflict exists, newer ADRs override older ADRs; unresolved conflict should be surfaced to the user.

## If layout changes later

If the repo moves to multi-context, add `CONTEXT-MAP.md` at root and map each sub-context explicitly.
