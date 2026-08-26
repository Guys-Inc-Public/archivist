# 0001 — Regenerate repository metadata from scratch

**Status:** Accepted · **Date:** 2026-08-26

## Context

Repository metadata tooling presented three options:

| Option | Trade-off |
|---|---|
| `reprepro` | Mature and well understood, opinionated about layout, awkward in ephemeral CI because it wants a durable database directory |
| `aptly` | More flexible, snapshot model, heavier |
| Hand-rolled | `apt-ftparchive` plus our own `Release` generation — full control, more surface to get wrong |

The real decision driver is CI ephemerality. `reprepro` expects a database that
persists between runs. In a GitHub Actions job there is no such thing, so either
the existing repository state is pulled down each run, or the index is generated
from scratch every time.

The reference pipeline in `github-desktop-linux` chose to pull state down, with
`aws s3 sync s3://$BUCKET repo` — the entire bucket, package pool included — at
the start of every publish. Measured against its actual artifacts, one release
is three `.deb` files totalling roughly 312 MB. At one release that is free. At
twenty releases it is about 6 GB pulled down and pushed back on every publish,
in order to regenerate an index that is currently 929 bytes.

That pipeline also demonstrated the failure mode of stateful publishing: its
state-pull step ends in `|| true`, so a failed pull produces a repository
containing only the newest package, and the subsequent sync overwrites `dists/`
with single-package indices. The pool objects survive; every previous version
simply stops being listed, and the run is green.

## Decision

**Regenerate the repository metadata in full on every publish.** There is no
database and no incremental state. The set of packages in object storage is the
only source of truth.

To keep that affordable, **write a metadata sidecar next to each pool object at
publish time**, holding that package's control stanza and checksums.
Regenerating an index means reading the sidecars, not the packages. The cost is
proportional to the metadata, not to the pool.

## Consequences

- No database to corrupt, and no state that can drift from the packages.
- A publish is idempotent: running it twice produces the same repository.
- The metadata sidecar is a new invariant that must be maintained. A pool object
  without a sidecar is a repository defect, so `verify` checks for exactly that.
- Removing a package means removing its object and its sidecar. Simple, but two
  operations rather than one.
- Sidecars are an implementation detail of this tool and are not part of the
  Debian archive format. They are additive files that standard mirroring tools
  ignore, so a mirror of an `archivist` repository stays valid.
- **Untested at scale.** The arithmetic works for hundreds of packages. It has
  not been exercised against thousands, and that remains an open question on the
  [roadmap](../Roadmap.md#open-questions).
