---
name: release-cli
description: Cut a tagged release of the doi-dropbox CLI. Use when the user asks to release, cut a version, publish a new version, ship a release, or tag a version of this CLI. Covers pre-release verification, semantic version selection, changelog conventions, tagging, and post-release checks.
---

# Releasing the doi-dropbox CLI

Releases are tag-driven: pushing a `v*` tag triggers
`.github/workflows/release.yml`, which runs GoReleaser to build every platform,
generate a changelog, and publish a GitHub Release.

This means **the tag is the release trigger** — pushing a wrong tag publishes a
wrong release. Verify before tagging, not after.

## 1. Verify the tree is releasable

Run from the repo root. All of these must pass:

```bash
make verify
```

Confirm separately that:

- You are on `main` and up to date (`git pull --ff-only`).
- The working tree is clean (`git status --porcelain` outputs nothing).
- CI is green on the commit you're about to tag.

If any of these fail, stop. Do not tag a dirty or unverified tree.

## 2. Choose the version

Semantic versioning, `vMAJOR.MINOR.PATCH`:

- **PATCH** (`v0.2.1`) — bug fixes only, no interface change.
- **MINOR** (`v0.3.0`) — new flags, new subcommands, new behavior that doesn't
  break existing usage.
- **MAJOR** (`v1.0.0`) — a breaking change to flags, command names, config
  variable names, or output format that scripts might parse.

While the project is pre-1.0, breaking changes go in MINOR bumps, but say so
prominently in the release notes.

Check what changed since the last tag before deciding:

```bash
git describe --tags --abbrev=0          # the previous tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

**Renaming or removing a flag is a breaking change** even if the code compiles.
Someone's script depends on it.

## 3. Confirm the changelog will read well

GoReleaser builds release notes from commit subjects, excluding `docs:`,
`test:`, and `chore:` prefixes (see `.goreleaser.yaml`). Preview what users will
see:

```bash
git log $(git describe --tags --abbrev=0)..HEAD --oneline \
  | grep -Ev '^[a-f0-9]+ (docs|test|chore):'
```

If that list is unhelpful — vague subjects, missing context — the fix is a
better release description, not rewriting history on `main`.

## 4. Dry-run the build

Build every platform locally without publishing:

```bash
goreleaser build --snapshot --clean
```

This catches cross-compilation failures before they become a failed release run.
Inspect `dist/` to confirm binaries were produced for each target.

## 5. Tag and push

```bash
git tag -a v0.3.0 -m "v0.3.0"
git push origin v0.3.0
```

Use an annotated tag (`-a`). Lightweight tags lack the metadata GoReleaser and
`git describe` rely on.

## 6. Verify the release landed

- Watch the Release workflow to completion in GitHub Actions.
- Open the published release: confirm binaries for linux/darwin/windows on the
  expected architectures, plus `checksums.txt`.
- Download one archive and run it:

```bash
tar xzf doi-dropbox_*_linux_amd64.tar.gz
./doi-dropbox --version    # must report the version you just tagged, not "dev"
```

If `--version` reports `dev`, the `ldflags` stamping in `.goreleaser.yaml` is
not reaching `main.version`. Fix that before announcing the release.

## If a release goes out wrong

Do not delete and re-push the tag — anyone who already pulled it gets a
mismatched artifact, and the failure mode is confusing.

Instead: mark the GitHub Release as a pre-release or delete the release (leaving
the tag), fix forward, and cut the next patch version. A skipped version number
costs nothing; a mutated tag costs someone an afternoon.
