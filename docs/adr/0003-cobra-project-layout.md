# ADR-0003: Cobra multi-file layout for a small CLI

**Status:** Accepted
**Date:** 2026-07-04

## Context

This tool has exactly one subcommand. It could be a single `main.go` of roughly
two hundred lines using the standard library's `flag` package, with no
dependencies at all.

Instead it uses Cobra and splits across `main.go`, `cmd/root.go`, and
`cmd/upload.go` — three files and a dependency for one command.

That's a real cost and worth justifying rather than absorbing silently.

## Decision

Use Cobra with the conventional multi-file layout, despite the current size.

## Consequences

**What this buys us:**

- Help text, flag parsing, argument validation (`cobra.MinimumNArgs`), and
  correct exit-code handling via `RunE` all come for free and are consistent
  with the CLIs users already know.
- Adding a second subcommand (`list`, `download`) is a new file, not a
  restructure. Those are plausible near-term additions, so the layout is not
  speculative.
- The structure is immediately legible to anyone who has read a Go CLI before —
  which matters more for a repository meant to be shared than saving two files
  does.

**What it costs us:**

- A dependency, and its transitive tree, for a program that could have none.
- Three files where one would do, which is genuine indirection for a reader
  tracing a single code path.
- Cobra's `init()`-based flag registration is implicit — flags appear in
  `root.go` and are consumed in `upload.go` via package-level variables, which
  is harder to follow than passing them explicitly.

## Alternatives considered

**Standard library `flag` in a single file** — rejected. Zero dependencies and
genuinely simpler today, but every subcommand added later would be hand-rolled
dispatch, and the help output would be worse than what users expect. The
tipping point is the second subcommand, which is close enough to be worth
building for.

**Cobra in a single file** — rejected. It works, but it puts this repo at odds
with every other Cobra project a reader has seen, which undercuts the main
reason for choosing Cobra in the first place.
