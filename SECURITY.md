# Security Policy

## Reporting a vulnerability

**Do not open a public issue.** Use GitHub's private vulnerability reporting
(Security → Report a vulnerability) or email `dan@devopsidiot.com`.

Please include what you did, what happened, and why you think it's exploitable.
A proof of concept helps enormously. I'll acknowledge within a few days — this
is a personal project, not a staffed product, so please calibrate expectations
accordingly.

## What this tool guarantees

These are the security properties the design is built around. A break in any of
them is a vulnerability, not a feature request:

- **No AWS credentials on the client.** The CLI holds a Cognito ID token and,
  briefly, presigned URLs. It never obtains, stores, or transmits AWS access
  keys or session credentials.
- **Nothing sensitive at rest.** No token, password, or credential is written to
  disk, ever. Every invocation authenticates fresh.
- **Passwords never appear in argv or the environment.** They are read with echo
  disabled and held only in memory. There is no `--password` flag, by design.
- **MFA is mandatory.** The Cognito user pool requires TOTP; there is no
  code path that skips it.
- **Least-privilege server side.** The Lambda that mints presigned URLs holds an
  IAM policy scoped to one bucket and the minimum actions needed.

## Known limitations

Stated plainly, because a security document that only lists strengths isn't one:

- **Presigned URLs are bearer tokens.** Anyone holding one can use it, for one
  object, until it expires (five minutes). We accept this exposure.
- **Not hardened against a compromised local machine.** If an attacker controls
  the machine while you type your password and MFA code, they can act as you.
  No client-side design prevents this.
- **Single-user by design.** There is no multi-tenancy, no per-user isolation,
  and no authorization model beyond "is this the one valid Cognito user." Do not
  deploy this for a team and expect users to be isolated from each other.
- **No integrity verification of uploads.** The tool does not checksum files
  before or after upload beyond what S3 does natively.

## Scope

In scope: this CLI and the auth flow it participates in.

Out of scope: vulnerabilities in AWS services themselves (report those to AWS),
and issues that require an already-compromised local machine.
