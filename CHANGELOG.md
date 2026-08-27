# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
