# 0006 — Package identity comes from control stanzas, never filenames

**Status:** Accepted · **Date:** 2026-08-26

## Context

The [extraction inventory](../Extraction-Inventory.md) found three architecture
vocabularies live simultaneously in the reference project: Debian's
`amd64`/`arm64`/`armhf`, RPM's `x86_64`/`aarch64`/`armv7l`, and Node's
`x64`/`arm64`/`armv7l`. Two near-identical `getArchitecture()` functions map
between them, disagreeing by design.

Release artifacts are also renamed by build tooling into project-specific
conventions — `GitHubDesktop-linux-amd64-3.4.9.deb` in the reference case. A
publisher that reads architecture or version from a filename inherits both the
renaming convention and the architecture vocabulary of whatever produced it.

Notably, the reference pipeline already got this right, probably without
deciding to: it globs `*.deb` and lets `reprepro` read control files. That is
the single best property of the existing implementation, and it is easy to lose
by accident when reimplementing.

## Decision

**A package's identity — name, version, architecture, source — is read from its
control stanza. Filenames are never parsed, and architecture is never accepted
as an input.**

Published filenames are *generated* from control fields in the archive's
canonical form (`package_version_arch.deb`), discarding whatever name the
artifact arrived with.

The `architectures` configuration field declares what the repository advertises.
It is not used to interpret packages: a package whose control architecture is
not in that list is a hard error, not a silent omission.

## Consequences

- Any build system's output works, regardless of naming convention.
- Mislabelled files are impossible: a `.deb` renamed to claim `arm64` is
  published according to what it actually contains.
- The published tree will not match the filenames on a project's GitHub release
  page. This surprises people, and is documented in
  [Repository Layout](../Repository-Layout.md#pool-and-the-prefix-directories).
- Epochs must be stripped when generating filenames — a colon in a path is not
  fetchable by `apt`. There is a test for this.
- We cannot report anything useful about a file that is not a readable `.deb`,
  because we refuse to guess from its name. The error says so plainly.
