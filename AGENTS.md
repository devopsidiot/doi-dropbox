# AGENTS.md

This repository keeps a single source of truth for AI agent guidance in
[`CLAUDE.md`](CLAUDE.md). That file is tool-agnostic despite its name — it
contains no Claude-specific syntax.

**If you are an agent operating in this repository, read `CLAUDE.md`.**

`AGENTS.md` exists because the emerging cross-tool convention is for agents to
look for this filename. Rather than maintain two documents that drift apart,
this one delegates.

## The short version

- Module path: `github.com/devopsidiot/doi-dropbox/cli`
- Verify with: `go build ./... && go test ./... -race && go vet ./... && gofmt -l .`
- Four hard invariants (no AWS credentials, no token persistence, no secrets on
  disk or in argv, no direct S3 calls from the CLI) — see `CLAUDE.md` for the
  full statement of each and the reasoning behind them.
- Design decisions are recorded in `docs/adr/`. Changing a decision means adding
  an ADR, not silently reversing it.
