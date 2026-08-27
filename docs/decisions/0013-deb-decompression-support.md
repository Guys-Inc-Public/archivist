# 0013 — Decompress control archives in-process, covering gzip, xz and zstd

**Status:** Accepted · **Date:** 2026-08-27

## Context

A `.deb` is an `ar` archive of three members: `debian-binary`, `control.tar.*`
and `data.tar.*`. Reading a package's identity means reading the control member,
and the compression that member uses is whatever `dpkg-deb` chose on the machine
that built it.

Go's standard library covers `gzip` and nothing else relevant here. The formats
in circulation are:

| Encoding | Produced by |
|---|---|
| `control.tar.xz` | `dpkg-deb` on Debian 12 and current Ubuntu — the overwhelming default |
| `control.tar.zst` | Ubuntu 21.10 and later, natively built packages |
| `control.tar.gz` | Older packages, and some third-party builders |
| `control.tar` | Legal, occasionally emitted with compression disabled |

Measured rather than assumed: of the 676 real packages cached on the
development host, 674 are `control.tar.xz` and 2 are `control.tar.gz`. A reader
supporting only the standard library's `gzip` would fail on 99.7% of them.

Three options:

| Option | Trade-off |
|---|---|
| Shell out to `dpkg-deb` | Unquestionably compatible, and unavailable on macOS, in `scratch` containers, and anywhere `dpkg` is not installed |
| `gzip` only | No new dependencies, and cannot read a current Debian package |
| Pure-Go decompressors | Two dependencies, and the binary stays self-contained |

## Decision

**Decompress in-process, supporting `gzip`, `xz`, `zstd` and uncompressed
`tar`.** `gzip` comes from the standard library, `xz` from
`github.com/ulikunitz/xz`, and `zstd` from `github.com/klauspost/compress`. Both
are pure Go and build with `CGO_ENABLED=0`.

This follows [0009](0009-openpgp-library-not-gpg.md) rather than deciding
anything new: that record rejected shelling out to `gpg` because a CI tool
requiring an external program on every machine is not a single static binary.
Requiring `dpkg-deb` would be the same defect, and worse — it would put a Debian
tool on the critical path of a binary we cross-compile for macOS.

Any other encoding is refused by name. `bzip2` and `lzma` appear in the
historical record; neither has been emitted by `dpkg-deb` this decade, and
guessing is worse than an error that says which format was found.

## Consequences

- The reader handles every package a mainstream tool produces, and needs nothing
  installed to do it.
- **Two dependencies, plus a language-version bump.**
  `github.com/klauspost/compress` requires Go 1.24, so the `go` directive moves
  from 1.23 to 1.24. CI reads the version from `go.mod` and follows
  automatically; contributors on 1.23 will need to upgrade.
- Validated in both directions rather than against itself: the reader agrees
  with `dpkg-deb -f` on `Package`, `Version` and `Architecture` for all 676
  cached packages, and `dpkg-deb` reads all four encodings the fixture builder
  produces. A reader tested only against its own writer proves nothing, because
  both can be wrong in the same way.
- Decompression is bounded — 32 MiB for the control archive and 4 MiB for the
  control file. A control stanza is a few kilobytes; those ceilings exist so a
  hostile package cannot turn a metadata read into an out-of-memory failure.
- `data.tar` is never decompressed. Nothing in the archive format needs its
  contents, and reading it would make the cost of indexing proportional to the
  size of the pool rather than to its metadata.
