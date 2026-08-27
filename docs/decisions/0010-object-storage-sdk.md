# 0010 — Use `aws-sdk-go-v2` for object storage

**Status:** Accepted · **Date:** 2026-08-27

## Context

[0003](0003-s3-compatible-storage.md) settled on S3-compatible storage with no
provider special-casing. That leaves the client library: `aws-sdk-go-v2` or
`minio-go`.

`minio-go` is smaller and has a simpler API. `aws-sdk-go-v2` is larger — roughly
ten additional modules — but is the library every R2, B2 and MinIO integration
guide is written against.

## Decision

**`aws-sdk-go-v2`**, with `BaseEndpoint` for the custom endpoint. No bespoke
endpoint resolver.

## Consequences

- The credential behaviour promised in [Configuration](../Configuration.md) —
  standard AWS environment variables — comes from the SDK's default chain, along
  with profiles and instance roles, rather than being hand-wired.
- A user hitting a storage problem can search for it and find answers written
  about the library we actually use.
- The binary grows by several megabytes. For a tool that runs in CI and is
  fetched once, this does not matter; it would matter for something shipped to
  end users, which this is not.
- If the size ever does matter, `minio-go` covers the same operations and the
  publish package is small enough to port.
