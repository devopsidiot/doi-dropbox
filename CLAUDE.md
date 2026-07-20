# CLAUDE.md

Guidance for AI coding agents working in this repository. Humans should read
[`CONTRIBUTING.md`](CONTRIBUTING.md) first — this file assumes that context and
adds only what an agent needs to avoid predictable mistakes here.

## What this is

`doi-dropbox` is a single-user CLI that uploads files to a private S3 bucket via
short-lived presigned URLs. It authenticates against Amazon Cognito with
password + TOTP MFA. **It never handles AWS credentials** — that property is the
whole point of the design, not an implementation detail.

## Repository layout

```
main.go              Entry point. Deliberately trivial: calls cmd.Execute().
cmd/root.go          Cobra root command; shared config flags live here.
cmd/upload.go        The upload subcommand: auth flow + upload logic.
docs/adr/            Architecture Decision Records. Read before changing design.
```

Module path is `github.com/devopsidiot/doi-dropbox/cli`. Internal imports must
match this exactly.

## Build, test, lint

```bash
go build ./...        # compiles
go test ./... -race   # tests, with race detector
go vet ./...          # catches suspicious-but-compiling code
gofmt -l .            # lists unformatted files; must output nothing
```

All four must pass before a change is complete. CI runs the same commands, so
there is no "works locally, fails in CI" gap by design.

## Invariants — do not violate without an ADR

These are load-bearing. If a change requires breaking one, stop and raise it
rather than working around it:

1. **No AWS credentials, ever.** No `~/.aws/credentials` reads for
   authorization, no long-lived keys, no IAM users. Cognito issues a short-lived
   ID token; that token is the only authority the CLI holds. The AWS SDK appears
   only to call Cognito's *public* auth API, which requires no credentials.
2. **No token persistence.** Every invocation re-authenticates, MFA included.
   This is a deliberate tradeoff (see `docs/adr/0002-no-token-caching.md`), not
   an unfinished feature. Do not add a token cache, keyring integration, or
   session file.
3. **Secrets never touch disk or argv.** The password is read via
   `golang.org/x/term` (no echo) and held only in memory. Never add a
   `--password` flag, never log it, never write it to a config file.
4. **The CLI is not in the upload data path decision-maker.** It asks the API
   for a presigned URL and PUTs to it. Do not add direct `s3:PutObject` calls.

## Conventions

- **Error handling:** wrap with context using `%w` —
  `fmt.Errorf("opening %s: %w", path, err)`. Never discard an error with `_`
  unless the failure is genuinely unactionable (e.g. a closing read at a
  keyboard prompt); if you do, that deserves a comment saying why.
- **Exit codes:** command functions use Cobra's `RunE` and return errors.
  `main.go` converts a non-nil error to exit code 1. Do not call `os.Exit`
  inside command logic — it bypasses deferred cleanup.
- **Comments:** this codebase is intentionally over-commented for readers new to
  Go. Preserve that style when editing; explain *why*, not just *what*. Do not
  strip existing explanatory comments to "clean up" a file.
- **Batch behavior:** a failed file must not abort the remaining uploads. Report
  it, continue, and return a non-nil error at the end so the exit code is right.

## Common mistakes in this repo

- Using `filename.Base(...)` instead of `filepath.Base(...)` — the package is
  `path/filepath`.
- Passing `cip.RespondToAuthChallenge{}` instead of
  `cip.RespondToAuthChallengeInput{}`. The AWS SDK method and its input struct
  differ by the `Input` suffix.
- Writing `if err := f(): err != nil` — the separator in an if-with-setup is a
  **semicolon**, not a colon.
- Adding a dependency to solve something the standard library already does.
  Justify new dependencies in the PR description.

## When you're unsure

Prefer asking over guessing on anything touching authentication, the presigned
URL flow, or IAM scope. A wrong guess in those areas is a security regression,
not a bug. Everything else — formatting, refactors, test coverage, docs — go
ahead.
