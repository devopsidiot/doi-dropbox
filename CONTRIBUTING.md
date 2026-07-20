# Contributing

Thanks for taking a look. This is a small, focused tool, and the goal is to keep
it that way — so this document covers both *how* to contribute and *what belongs
here* in the first place.

## Scope

`doi-dropbox` uploads files to a private S3 bucket using short-lived presigned
URLs, authenticated by Cognito with MFA. It is deliberately narrow.

**In scope:** upload reliability, auth-flow correctness, cross-platform support,
error messages, tests, documentation.

**Out of scope** (these have been considered and declined — see `docs/adr/`):
credential caching, storing AWS keys, a daemon or background sync, a GUI,
support for storage backends other than S3.

If you want something out of scope, open an issue proposing an ADR before
writing code. It's much cheaper to disagree about a design in an issue than in a
pull request.

## Getting set up

You need Go (the version in `go.mod`) and nothing else to build.

```bash
git clone https://github.com/devopsidiot/doi-dropbox.git
cd doi-dropbox/cli
go mod download
go build ./...
```

To actually *run* it against real infrastructure you also need the deployed AWS
side (Cognito user pool, API Gateway, Lambda, S3). Those live in the companion
infrastructure repo. You do not need them to build, test, or lint.

## The development loop

```bash
make verify     # runs everything CI runs — do this before pushing
```

Or individually:

```bash
gofmt -w .            # format (not optional; CI fails on unformatted code)
go build ./...        # compile
go vet ./...          # catch suspicious constructs
go test ./... -race   # tests with the race detector
```

CI runs exactly these commands. If `make verify` passes locally, CI should pass
too — if it doesn't, that's a bug in our setup and worth reporting.

## Testing expectations

- New logic needs a test. "Logic" means branching, parsing, validation, or
  formatting — not a one-line pass-through to the AWS SDK.
- Prefer table-driven tests. They're the Go convention and they make adding the
  next case a one-line diff. See `handler_test.go` in the Lambda for the style.
- Don't write tests that require live AWS. Anything needing real credentials or
  network calls to AWS is an integration test, and we don't run those in CI.
  Extract the pure logic and test that instead.

## Commit messages

Conventional Commits, because the release changelog is generated from them:

```
feat: add --dry-run flag to upload
fix: handle empty MFA input without panicking
docs: clarify module path in README
chore: bump golangci-lint
test: cover filename validation edge cases
```

The prefix matters (`feat`/`fix` appear in the changelog; `docs`/`chore`/`test`
are filtered out). The rest is just a clear sentence in the imperative mood.

## Pull requests

Keep them small and single-purpose. A PR that fixes a bug *and* refactors
*and* adds a feature is three PRs wearing a trenchcoat, and it's much harder to
review or revert.

Include in the description:
- What changed and **why** — the why is the part reviewers can't reconstruct.
- How you verified it (beyond "CI passed").
- Any new dependency, with justification. Adding a dependency to this project
  should feel slightly uncomfortable; the standard library covers most of what a
  tool this size needs.

## Security

Do not open a public issue for a security problem. See
[`SECURITY.md`](SECURITY.md).

Be especially careful with anything touching the authentication flow, presigned
URL handling, or IAM scope. Those aren't "just code" — a mistake there is a
security regression. When in doubt, ask in an issue first.
