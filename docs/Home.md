# archivist

**Point it at a directory of `.deb` files and get a signed apt repository.
Publish that repository to any S3-compatible bucket, on your own domain.**

> [!WARNING]
> **Pre-release.** `build`, `inspect` and `version` work. `publish` and `verify`
> describe the design target and exit `2`. See [Roadmap](Roadmap.md) for what is
> actually built. Nothing is stable until `v0.1.0` is tagged.

## Start here

| Page | What it covers |
|---|---|
| [Getting Started](Getting-Started.md) | Install, build a first repository, point `apt` at it |
| [Configuration](Configuration.md) | Every field of `archivist.yml` |
| [Signing Keys](Signing-Keys.md) | Key generation, CI handling, rotation |
| [Repository Layout](Repository-Layout.md) | What gets published, and where |
| [Verification](Verification.md) | Proving a published repository is intact |
| [Decisions](decisions/README.md) | Why the design is the way it is |
| [Roadmap](Roadmap.md) | Milestones and scope boundaries |
| [Extraction Inventory](Extraction-Inventory.md) | What this was extracted from, and what was left behind |

## The problem

Publishing Debian packages properly means learning `reprepro`, GPG key handling
in CI, repository metadata layout, and multi-architecture packaging. Most small
projects skip it and drop loose `.deb` files on a releases page, so their users
download binaries over HTTPS and hope. The hosted services that solve this
charge for it and hold your repository on their domain.

`archivist` is a single static binary. It runs in your CI, signs with your key,
and publishes to storage you control.

## What it will not do

Build packages. Host anything. Provide a web UI. Promote packages between
channels. See [scope discipline](Roadmap.md#scope-discipline) — the list is
deliberate, and it is the main thing keeping this project finishable.
