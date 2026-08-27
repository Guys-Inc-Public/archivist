<div align="center">

# archivist

**Point it at a directory of `.deb` files and get a signed apt repository.**
**Publish that repository to any S3-compatible bucket, on your own domain.**

[![CI](https://github.com/Guys-Inc-Public/archivist/actions/workflows/ci.yml/badge.svg)](https://github.com/Guys-Inc-Public/archivist/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Guys-Inc-Public/archivist/actions/workflows/codeql.yml/badge.svg)](https://github.com/Guys-Inc-Public/archivist/actions/workflows/codeql.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-2f80c4.svg)](./LICENSE)

</div>

---

> [!WARNING]
> **Pre-release.** The CLI surface below is the design target, not yet a working
> implementation. Track progress against [the roadmap](./docs/Roadmap.md).
> Nothing here is stable until `v0.1.0` is tagged.

## Why

Most small projects publish loose `.deb` files on a GitHub Releases page, because
doing it properly means learning `reprepro`, GPG key handling in CI, repository
metadata layout, and multi-architecture cross-compilation. The hosted options
solve this as SaaS. `archivist` solves it as a single static binary you run in
your own CI, publishing to storage you own.

### Where it came from

This is not a design sketched in the abstract. It is the publishing half of
[GitHub Desktop for Linux](https://github.com/Guys-Inc-Public/github-desktop-linux),
which serves signed `.deb` packages for three architectures from
[apt.guysinc.pub](https://apt.guysinc.pub) and has already been through the things
that go wrong: a passphrase-protected key that cannot sign in CI, a state pull that
swallowed its own failure and quietly unlisted every previous version, a signing key
rotation, and an armour checksum that GnuPG rejects but the Go library omits.

What carried over is the design, not the code — a survey of the original pipeline
found no file worth extracting unchanged, so `archivist` is a clean-room rewrite
([ADR&nbsp;0001](./docs/decisions/0001-regenerate-from-scratch.md), and
[Extraction Inventory](./docs/Extraction-Inventory.md) for what was surveyed). The
original repository still publishes with `reprepro` directly; it moves to `archivist`
once `v0.1.0` ships.

## Example

```console
$ archivist build ./dist --config archivist.yml --out ./repo
  reading 3 packages from ./dist
    github-desktop  3.4.9  amd64
    github-desktop  3.4.9  arm64
    github-desktop  3.4.9  armhf
  writing dists/stable/main/binary-{amd64,arm64,armhf}/Packages
  signing dists/stable/Release  ->  Release.gpg, InRelease
  ok  ./repo

$ archivist publish ./repo --bucket my-apt-repo --endpoint https://s3.example.com
  uploaded 3 packages, 11 metadata files
  ok  https://apt.example.com/

$ archivist verify https://apt.example.com --keyring ./public.asc
  InRelease signature      ok  (1234567890AB…12345678, expires 2028-01-01)
  Packages checksums       ok  (3/3)
  pool objects reachable   ok  (3/3)
  ok  repository is internally consistent
```

Add it to a workflow:

```yaml
- uses: Guys-Inc-Public/archivist@v1
  with:
    packages: ./dist
    config: archivist.yml
    bucket: my-apt-repo
  env:
    ARCHIVIST_GPG_KEY: ${{ secrets.APT_GPG_PRIVATE_KEY }}
```

## Documentation

Docs live in [`docs/`](./docs) and are mirrored to the
[wiki](https://github.com/Guys-Inc-Public/archivist/wiki) on every merge to `main`.

| | |
|---|---|
| [Getting Started](./docs/Getting-Started.md) | Install, first repository, pointing `apt` at it |
| [Configuration](./docs/Configuration.md) | Every field in `archivist.yml` |
| [Signing Keys](./docs/Signing-Keys.md) | Key generation, CI handling, and rotation |
| [Repository Layout](./docs/Repository-Layout.md) | What gets published, and where |
| [Verification](./docs/Verification.md) | Proving a published repository is intact |
| [Decisions](./docs/decisions) | Architecture decision records |
| [Roadmap](./docs/Roadmap.md) | Milestones and scope boundaries |

## Scope

`v0.1` does exactly one thing: **a directory of `.deb` files → a signed apt
repository → published to object storage.**

Deliberately out of scope: building packages, RPM repositories (`v0.2`), a
hosting service, a web UI, package promotion between channels, and anything
Kubernetes. See [Roadmap](./docs/Roadmap.md#scope-discipline).

## Contributing

Start with [CONTRIBUTING.md](./CONTRIBUTING.md) for the development setup, and the
[organisation contribution guide](https://github.com/Guys-Inc-Public/.github/blob/main/CONTRIBUTING.md)
for the policy that applies across all Guys Inc repositories.

Questions and ideas belong in
[Discussions](https://github.com/Guys-Inc-Public/archivist/discussions);
reproducible defects belong in
[Issues](https://github.com/Guys-Inc-Public/archivist/issues).

## Security

Vulnerabilities: see [SECURITY.md](./SECURITY.md). Do not open a public issue.

## License

[MIT](./LICENSE) © Guys Inc
