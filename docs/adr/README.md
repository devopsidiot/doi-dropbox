# Architecture Decision Records

An ADR captures a decision that was genuinely contested — where a reasonable
engineer could have chosen differently — along with the reasoning and what it
cost us.

The point is not ceremony. It's that six months from now, someone (possibly the
author) will look at a design choice, assume it was an accident, and "fix" it.
An ADR is the note that says: this was on purpose, here's what we traded away,
and here's what would have to change for the decision to be worth revisiting.

## What earns an ADR

Write one when the answer to "why is it like this?" is not obvious from reading
the code, **and** a different choice was defensible. Examples: choosing between
two auth models, deliberately *not* building something users will ask for,
accepting a known limitation.

Don't write one for: naming, formatting, obvious choices, or anything a comment
in the code covers better.

## Status values

- **Accepted** — in effect now.
- **Superseded by ADR-NNNN** — replaced. Leave the original in place; the
  history is the value.
- **Proposed** — under discussion, not yet decided.

## Format

Keep it short. If it's longer than a page, it's probably two decisions.

```markdown
# ADR-NNNN: Short title in the imperative

**Status:** Accepted
**Date:** YYYY-MM-DD

## Context
What situation forced a decision? What constraints were real?

## Decision
What we chose, stated plainly.

## Consequences
What this costs us, what it buys us, and what we gave up. Be honest about the
downsides — an ADR with no downsides listed is a sales pitch, not a record.

## Alternatives considered
What else was on the table and why it lost.
```

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-presigned-urls-over-temporary-credentials.md) | Presigned URLs over temporary credentials | Accepted |
| [0002](0002-no-token-caching.md) | No token caching between invocations | Accepted |
| [0003](0003-cobra-project-layout.md) | Cobra multi-file layout for a small CLI | Accepted |
