# 0003 — Target S3-compatible storage; special-case no provider

**Status:** Accepted · **Date:** 2026-08-26

## Context

The repository has to be published somewhere. Guys Inc already uses Cloudflare
R2, and the reference pipeline talks to R2 directly, constructing the account's
endpoint URL inline.

Writing to R2 specifically would be the shortest path and would serve the first
user perfectly. It would also make every other user a special case.

## Decision

**Target S3-compatible object storage generically.** One code path covers
Cloudflare R2, MinIO, Backblaze B2 and Amazon S3. The endpoint is configuration.
No provider is special-cased, including the one we use.

Credentials are read from the standard AWS environment variables, so anything
already configured for the AWS CLI works unchanged.

## Consequences

- Users are not required to adopt our hosting choices to use the tool.
- Provider-specific features are unavailable. R2's per-object metadata
  behaviours, S3 storage classes, and B2 lifecycle rules are all outside what a
  common code path can express. If one of those turns out to be necessary rather
  than convenient, this decision gets revisited with a concrete case attached.
- `public_url` is configured separately from `endpoint`, because a
  CDN-fronted bucket is served from a different hostname than it is written to.
  That is not a special case for any provider — it is the normal arrangement.
- Testing can run against MinIO in a container, so the publish path is testable
  without a cloud account or a credential.
