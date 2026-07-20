# ADR-0002: No token caching between invocations

**Status:** Accepted
**Date:** 2026-07-04

## Context

Cognito issues an ID token valid for one hour, plus a refresh token valid for
thirty days. A CLI could cache either one on disk and skip re-authenticating on
subsequent runs.

Without caching, every invocation prompts for a password and an MFA code. For a
batch of files that's fine — they share one login — but running the command
three times in a row means three MFA prompts.

## Decision

Do not persist tokens. Every invocation authenticates from scratch, MFA
included. Nothing is written to disk.

## Consequences

**What this buys us:**

- There is no credential material at rest, anywhere, ever. No cache file to
  chmod correctly, no keyring integration to get subtly wrong on one of three
  operating systems, no stale token to invalidate when something goes wrong.
- Moving to a new machine requires nothing but the binary and four non-secret
  environment variables. This is the property that makes the tool genuinely
  portable.
- The threat model stays trivially explainable: if someone has your password and
  your phone, they can upload. Otherwise they cannot. A cache file would add a
  third path that has to be reasoned about.

**What it costs us:**

- Repeated invocations are annoying. Uploading three files in three separate
  commands means three MFA prompts.
- Unattended use (cron, CI, a scheduled backup) is effectively impossible
  without a person present to type a code. This is a real limitation, not a
  hypothetical one.

**Mitigation:** the `upload` subcommand accepts multiple files and authenticates
once for the whole batch. `doi-dropbox upload a b c` is the intended usage;
three separate commands is not.

## Alternatives considered

**Cache the refresh token in a file with 0600 permissions** — rejected. It
mostly works, but "mostly" is doing heavy lifting: file permissions behave
differently across platforms, and a plaintext refresh token grants thirty days
of access without MFA. That's a meaningfully worse worst case than what we have
now.

**OS keychain integration** — rejected for scope, not principle. It's the
*correct* way to do this, and it's three different implementations (macOS
Keychain, Windows Credential Manager, libsecret on Linux) plus a dependency
tree. Not justified for a single-user tool where the annoyance is a few extra
seconds.

**Revisit if:** unattended/scheduled uploads become a real requirement. That use
case can't be served by this decision, and no amount of ergonomics work will
change that — it would need a genuinely different auth path (a dedicated
machine identity), which would be its own ADR.
