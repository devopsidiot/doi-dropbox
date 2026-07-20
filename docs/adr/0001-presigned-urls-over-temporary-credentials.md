# ADR-0001: Presigned URLs over temporary credentials

**Status:** Accepted
**Date:** 2026-07-04

## Context

The tool needs to put files into a private S3 bucket from machines that should
not hold AWS credentials. Authentication is Cognito with mandatory TOTP MFA.

Once a user is authenticated, there are two established ways to let them write
to S3:

1. **Cognito Identity Pool → temporary AWS credentials.** Exchange the Cognito
   login for short-lived AWS credentials scoped to an IAM role, then call S3
   directly with the AWS SDK.
2. **A backend endpoint that mints presigned URLs.** The client sends its
   Cognito token to an API; a Lambda (running as its own tightly-scoped role)
   returns a presigned URL for one object; the client PUTs to that URL.

Both avoid long-lived keys. The difference is what the client ends up holding.

## Decision

Use presigned URLs minted by a Lambda behind API Gateway. The client never
receives an AWS credential of any kind — only a URL valid for one object for
five minutes.

## Consequences

**What this buys us:**

- The client's maximum authority is "upload one named object, for five minutes."
  A compromised client session cannot enumerate the bucket, read other files, or
  reach any other AWS service, regardless of what the IAM role permits.
- Server-side validation is possible and enforced. Filenames are checked and the
  object key is chosen by the Lambda, so the client cannot control where its
  bytes land.
- The credential-issuing surface is one Lambda with one narrow IAM policy, which
  is a much smaller thing to audit than a role assumable by any authenticated
  identity-pool user.

**What it costs us:**

- More infrastructure: API Gateway, a Lambda, and their IAM plumbing exist
  solely to hand out URLs. Option 1 needs none of that.
- An extra network round trip per file — the client asks for a URL before it can
  upload.
- Multipart uploads for very large files are not free. A single presigned PUT
  caps at 5 GB; supporting more means minting a set of URLs, which is real
  additional work. Option 1 would have gotten multipart from the SDK for free.
- The presigned URL is a bearer token. If it leaks within its five-minute
  window, someone else can write that one object. We accept this; the exposure
  is small and time-boxed.

## Alternatives considered

**Cognito Identity Pool with temporary credentials** — rejected. Simpler and it
would have given us multipart uploads for free, but it hands the client a real
AWS credential. For an internet-facing tool on a public domain, the tighter
posture was worth the extra plumbing. This was close; if the multipart
limitation becomes painful, it is the first thing to revisit.

**Long-lived IAM user with an access key** — rejected outright. It's the problem
this project exists to avoid.
