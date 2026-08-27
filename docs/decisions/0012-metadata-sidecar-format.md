# 0012 — The metadata sidecar format

**Status:** Accepted · **Date:** 2026-08-27

## Context

[0001](0001-regenerate-from-scratch.md) invented the metadata sidecar — a file
written next to each pool object holding that package's control stanza and
checksums — so that regenerating an index costs `O(metadata)` rather than
`O(pool)`. It did not specify what the file is called or what is in it.

That needs pinning before anything writes one. A sidecar is an on-disk format
living in someone else's bucket; once published it carries a compatibility
burden, and changing it later means reading two shapes forever.

## Decision

One JSON file per pool object, named by appending `.archivist.json` to the
object's full name:

```
pool/main/g/github-desktop/github-desktop_3.4.9_amd64.deb
pool/main/g/github-desktop/github-desktop_3.4.9_amd64.deb.archivist.json
```

```json
{
  "schema": 1,
  "control": { "Package": "github-desktop", "Version": "3.4.9", "...": "..." },
  "size": 129609048,
  "md5": "…", "sha1": "…", "sha256": "…"
}
```

Three properties are deliberate:

- **The suffix appends rather than replaces**, so a bucket listing sorts each
  sidecar immediately after the object it describes. A listing that interleaves
  them arbitrarily is unreadable at scale, which is precisely when you need it.
- **`schema` is present from the first write.** A version 2 is then possible.
  Adding a version field later means guessing at files that predate it.
- **`control` is the stanza as parsed**, not a subset. The index needs fields we
  do not currently use, and re-reading the pool to recover one would defeat the
  purpose of the sidecar.

## Consequences

- Regenerating an index means listing and reading sidecars. The pool is never
  downloaded.
- A pool object without a sidecar, or a sidecar without its object, is a
  repository defect. `verify` reports both.
- Sidecars are additive files that are not part of the Debian archive format.
  Standard mirroring tools copy them and clients ignore them, so a mirror of an
  `archivist` repository remains a valid apt repository.
- Removing a package is two deletions rather than one.
- The format is public whether or not we intend it to be, since it sits in a
  world-readable bucket. It is documented rather than treated as an internal
  detail.
