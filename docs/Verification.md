# Verification

A `verify` subcommand a user can run against your published repository is worth
more than a paragraph of README claims. This page covers both directions:
verifying a repository `archivist` produced, and verifying `archivist` itself.

## Verifying a published repository

> [!WARNING]
> **Pre-release.** `archivist verify` is not implemented yet. The manual
> equivalents below work today.

```console
$ archivist verify https://apt.example.com --keyring ./public.asc
```

The check walks the whole trust chain rather than sampling it:

| Check | What a failure means |
|---|---|
| `InRelease` signature validates against the keyring | The repository is not signed by the key you expect |
| Signing key is not expired, and not expiring soon | `apt update` is about to start failing for everyone |
| Every `Packages` file matches its checksum in `Release` | The index does not match what was signed |
| Every package in every `Packages` file resolves to a pool object | Packages are listed but not fetchable |
| Every pool object matches its recorded `SHA256` | Content changed after signing |
| Every pool object is referenced by some index | Orphaned objects — usually a partial publish |
| The declared `architectures` all have an index | A build failed and the repository silently lost an architecture |

The last three are the ones that catch real incidents. A publish that fails
halfway leaves a repository that is *signed and internally plausible* but is
missing content, and nothing in `apt`'s own verification notices — `apt` checks
that what it fetched matches what was signed, not that what was signed is
complete.

### Run it in CI, after publishing

```yaml
- name: Verify the published repository
  run: archivist verify "$PUBLIC_URL" --keyring ./public.asc
```

This is the check that catches a swallowed error in the publish step. A publish
job whose failure mode is a green run and a quietly broken repository is worse
than one that fails loudly.

### Doing it by hand

```console
$ curl -fsSLO https://apt.example.com/dists/stable/InRelease
$ gpg --no-default-keyring --keyring ./public.gpg --verify InRelease
$ apt-get --no-download --simulate install your-package    # on a configured host
```

The most honest end-to-end test is still a clean container:

```console
$ docker run --rm -it debian:stable-slim bash -c '
    apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null
    curl -fsSL https://apt.example.com/public.gpg -o /etc/apt/keyrings/example.gpg
    echo "deb [signed-by=/etc/apt/keyrings/example.gpg] https://apt.example.com stable main" \
      > /etc/apt/sources.list.d/example.list
    apt-get update && apt-get install -y your-package'
```

## Verifying an `archivist` release

Every release is signed, carries an SBOM, and records a public build-provenance
attestation. A packaging tool that ships unsigned binaries is a bad joke.

**Provenance** — proves the binary was built by this repository's workflow from a
specific commit. The signing identity is minted per run and discarded, so there
is no key to store or rotate:

```console
$ gh attestation verify archivist_0.1.0_linux_amd64.tar.gz --repo Guys-Inc-Public/archivist
```

**Signature** — `checksums.txt` is signed by the Guys Inc release subkey, and it
names every archive, so verifying it transitively verifies all of them:

```console
$ gpg --verify checksums.txt.asc checksums.txt
$ sha256sum --check --ignore-missing checksums.txt
```

**SBOM** — a CycloneDX SBOM is attached to each release archive, listing every
module compiled in.

Fingerprints are on [Signing Keys](Signing-Keys.md#the-guys-inc-keys).
