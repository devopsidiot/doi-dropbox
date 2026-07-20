# doi-dropbox

[![CI](https://github.com/devopsidiot/doi-dropbox/actions/workflows/ci.yml/badge.svg)](https://github.com/devopsidiot/doi-dropbox/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/devopsidiot/doi-dropbox)](https://goreportcard.com/report/github.com/devopsidiot/doi-dropbox)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A single-user CLI for uploading files to a private S3 bucket — authenticated
with a password and TOTP MFA, and **without any AWS credentials on the machine
you're uploading from.**

```console
$ doi-dropbox upload notes.md screenshot.png
Logging in as dan...
Password:
MFA code: 123456
  uploaded: notes.md
  uploaded: screenshot.png
All uploads finished.
```

## Why it works this way

The usual way to script an S3 upload is to put an IAM access key on the machine.
That key is long-lived, has to be rotated, and is exactly the thing you don't
want sitting on a laptop you travel with.

This tool takes a different path:

```
  you ──password + TOTP──► Cognito ──► short-lived ID token
                                            │
                                            ▼
                              API Gateway (verifies the token)
                                            │
                                            ▼
                                  Lambda mints a presigned URL
                                            │
                                            ▼
                       CLI PUTs the file directly to S3 with that URL
```

The CLI never holds an AWS credential — only a token that expires in an hour and
a presigned URL that's good for one object for five minutes. Nothing is written
to disk. Every run re-authenticates from scratch.

That last point is a deliberate tradeoff, not a missing feature. See
[ADR-0002](docs/adr/0002-no-token-caching.md).

## Install

**From a release** (no Go toolchain needed):

Download the archive for your platform from the
[releases page](https://github.com/devopsidiot/doi-dropbox/releases), extract it,
and put `doi-dropbox` somewhere on your `PATH`.

**From source:**

```bash
go install github.com/devopsidiot/doi-dropbox/cli@latest
```

## Configure

The CLI needs four values. None of them are secrets — they're public
identifiers for your own deployment. Set them as environment variables:

```bash
export COGNITO_REGION=us-west-2
export COGNITO_CLIENT_ID=<your cognito app client id>
export API_BASE_URL=https://<id>.execute-api.<region>.amazonaws.com
export DROPBOX_USERNAME=<your username>
```

Or pass them per-invocation as flags (`--region`, `--client-id`, `--api-url`,
`--username`). Flags win over environment variables.

Your **password** is never configured — it's prompted for, with echo disabled,
on every run.

## Usage

```bash
doi-dropbox upload <file> [more files...]
```

Multiple files in one invocation share a single login, so prefer
`doi-dropbox upload a.txt b.txt` over two separate commands.

If one file fails, the rest still upload and the command exits non-zero — which
means it composes correctly in scripts and cron jobs.

```bash
doi-dropbox --help          # all commands
doi-dropbox upload --help   # flags for this subcommand
```

## Using it from another machine

There's nothing machine-specific to copy. Install the binary, set the four
non-secret environment variables, and run it — your password is typed in and
your MFA code comes from your phone, neither of which is tied to a workstation.

## Development

See [`CONTRIBUTING.md`](CONTRIBUTING.md). The short version:

```bash
make verify   # format check, build, vet, test — the same thing CI runs
```

AI agents working in this repo: see [`CLAUDE.md`](CLAUDE.md).

## Design decisions

Non-obvious choices are recorded as ADRs in [`docs/adr/`](docs/adr/). If you're
wondering "why on earth did they do it *that* way," that's where the answer
should be. If it isn't, that's a documentation bug worth filing.

## License

Apache 2.0 — see [`LICENSE`](LICENSE).
