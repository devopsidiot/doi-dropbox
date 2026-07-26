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
cli/                 The CLI. Its own Go module.
  main.go            Entry point. Deliberately trivial: calls cmd.Execute().
  cmd/root.go        Cobra root command; shared config flags live here.
  cmd/upload.go      The upload subcommand: auth flow + upload logic.
lambda/              The presigned-URL minter. A separate Go module.
  handler.go         API Gateway handler: validates the filename, presigns a PUT.
terraform/           Cognito, API Gateway, Lambda, the uploads bucket, CloudFront.
scripts/             One-time setup helpers (gen-config.sh, setup-mfa.sh).
docs/adr/            Architecture Decision Records. Read before changing design.
```

**There is no Go module at the repo root.** This repo is two independent
modules — `github.com/devopsidiot/doi-dropbox/cli` and
`github.com/devopsidiot/doi-dropbox/lambda` — and every Go command has to run
inside one of them. Running `go build ./...` at the root fails with `directory
prefix . does not contain main module`. Internal imports must match the module
path of whichever module the file lives in.

## Build, test, lint

Use the Makefile — it runs each module in turn, which is the part that is easy
to get wrong by hand:

```bash
make verify           # format check, build, vet, test — what CI runs
```

To run a single check against one module, `cd` into it first. These fail at the
repo root, because no module lives there:

```bash
cd cli                # or: cd lambda
go build ./...        # compiles
go test ./... -race   # tests, with race detector
go vet ./...          # catches suspicious-but-compiling code
gofmt -l .            # lists unformatted files; must output nothing
```

All four must pass, in both modules, before a change is complete. CI runs the
same commands per module, so there is no "works locally, fails in CI" gap by
design.

Note that `go build ./...` inside a main package leaves a binary named after the
directory (`lambda/lambda`). Those are gitignored and `make clean` removes them.

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
