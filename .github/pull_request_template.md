## What changed

<!-- One or two sentences. The diff shows what; this should say what it means. -->

## Why

<!-- The part reviewers can't reconstruct from the code. What problem does this
     solve, or what were you unable to do before? -->

## How it was verified

<!-- Beyond "CI passed." What did you actually run or observe? -->

- [ ] `make verify` passes locally
- [ ] Tests added or updated for changed logic
- [ ] Manually exercised the affected path

## Checklist

- [ ] Commit messages follow Conventional Commits (`feat:`, `fix:`, `docs:`, …)
- [ ] No new dependency, **or** it's justified in this description
- [ ] No change to the auth flow, presigned URL handling, or IAM scope,
      **or** the security implications are described below
- [ ] Design decisions that a future reader would question are recorded in
      `docs/adr/`

## Security notes

<!-- Delete if not applicable. Required if this touches authentication, token
     handling, presigned URLs, or IAM permissions — describe what changes about
     the tool's security properties. -->
