# Contributing to archivist

Organisation-wide policy — conduct, PR etiquette, commit style — lives in the
[Guys Inc contribution guide](https://github.com/Guys-Inc-Public/.github/blob/main/CONTRIBUTING.md).
This file covers what is specific to *this* repository.

## Where things go

| You have | Open |
|---|---|
| A reproducible defect | an [Issue](https://github.com/Guys-Inc-Public/archivist/issues) |
| A question, or an idea that isn't a defect | a [Discussion](https://github.com/Guys-Inc-Public/archivist/discussions) |
| A security vulnerability | a private advisory — see [SECURITY.md](./SECURITY.md) |
| A change you've already written | a Pull Request |

## Development

```console
$ git clone https://github.com/Guys-Inc-Public/archivist.git
$ cd archivist
$ make build        # ./bin/archivist
$ make test
$ make lint
```

Requirements: Go (the version in [`go.mod`](./go.mod) or newer). `gpg` and
`apt-utils` are needed for the integration tests only.

## Repository layout

```
cmd/archivist/    CLI entrypoint — flag parsing and output, no logic
internal/deb/     control file parsing, Packages index generation
internal/repo/    dists/ layout, Release and InRelease generation
internal/sign/    GPG, isolated behind an interface
internal/publish/ S3-compatible upload
action/           composite Action wrapping the CLI
docs/             documentation, mirrored to the wiki on merge
testdata/         fixture packages
```

The CLI is the product; the Action is a thin wrapper over it. Logic that lands in
`action/` instead of `internal/` cannot be tested without pushing a commit, so it
does not land in `action/`.

## Pull requests

- One logical change per PR. If the title needs an "and", split it.
- The PR title becomes the squash commit subject — write it in the imperative
  (`Parse Multi-Arch field from control stanzas`).
- Update [`CHANGELOG.md`](./CHANGELOG.md) under `## [Unreleased]` for anything a
  user would notice.
- New behaviour needs a test. Bug fixes need a test that fails without the fix.
- CI must be green. Maintainers will not merge a red PR to save a round trip.

## Documentation

Documentation is written in [`docs/`](./docs) and published to the wiki
automatically. **Do not edit the wiki directly** — it is overwritten on every
merge to `main`, and your edit will be lost. Edit `docs/` and open a PR.

Filenames in `docs/` map to wiki page titles, so `Signing-Keys.md` becomes the
*Signing Keys* page. Use `Title-Case-With-Hyphens.md`.

## Decisions

Choices that are expensive to reverse are recorded as ADRs in
[`docs/decisions/`](./docs/decisions). If a PR changes one of those choices, it
supersedes the ADR rather than editing it — see the
[ADR index](./docs/decisions/README.md).
