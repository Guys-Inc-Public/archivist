# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `internal/repo`: generate the published tree. Pool placement from control
  fields, `.archivist.json` sidecars, per-architecture `Packages` and
  `Packages.gz`, per-architecture `Release` markers, and the signed-manifest
  `Release` with its MD5Sum, SHA1 and SHA256 blocks. Output is byte-for-byte
  deterministic, so a regenerated repository can be diffed against the one it
  replaces.
- A package belongs to exactly one component, and a second configured component
  gets a valid but empty index. `ScanSidecars` also rejects a pool object that
  is not where its own control stanza puts it, which catches a renamed file or
  a sidecar sitting beside the wrong object.
- Building into a directory that already holds a repository merges rather than
  replaces: existing packages are kept, rebuilding the same input is a no-op,
  and republishing a version with different content is refused unless it is
  explicitly allowed.
- `internal/config`: load and validate `archivist.yml`. Unknown and duplicate
  keys are rejected, every problem in a file is reported at once rather than
  one per run, and which fields are required depends on the command — `build`
  writes to local disk and does not ask for object-storage settings.
- `valid_for` is documented and parsed, closing the gap decision 0011 left: it
  accepts week and day units, and stays unset by default.
- Read package metadata directly from `.deb` files: `ar` framing, and control
  archives compressed with gzip, xz or zstd.
- `archivist inspect` accepts a `.deb` as well as a bare control file, and
  reports the size and SHA256 an index would record for it. Which kind of file
  it was given is determined by content, never by its name.
- Repository scaffolding, governance, and CI/CD workflows.
- Architecture decision records covering repository state, signing, storage,
  distribution shape, and documentation.
- Extraction inventory recording what was and was not carried over from
  `github-desktop-linux`.

[Unreleased]: https://github.com/Guys-Inc-Public/archivist/commits/main
