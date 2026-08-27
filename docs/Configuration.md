# Configuration

> [!WARNING]
> **Pre-release.** The schema below is the design target for `v0.1` and is open
> for comment until it ships. Once `v0.1.0` is tagged, changes here are breaking
> changes.

`archivist.yml` describes the repository you are publishing. Field names follow
Debian's own vocabulary wherever one exists, so that what you write here matches
what appears in `Release` and what the `reprepro` documentation calls it.

## Example

```yaml
# --- Repository identity -----------------------------------------------------
origin: Example Project
label: Example Project packages
description: Packages for Example Project
suite: stable
codename: stable
components: [main]
architectures: [amd64, arm64, armhf]

# --- Signing -----------------------------------------------------------------
signing:
  key_id: 1234567890ABCDEF1234567890ABCDEF12345678
  # The key itself comes from $ARCHIVIST_GPG_KEY. There is no field for it.

# --- Publishing --------------------------------------------------------------
publish:
  bucket: example-apt
  endpoint: https://s3.example.com
  region: auto
  public_url: https://apt.example.com
  prefix: ""
```

## Repository identity

| Field | Required | Notes |
|---|---|---|
| `origin` | yes | `Origin:` in `Release`. Who publishes this — usually the project name. |
| `label` | yes | `Label:` in `Release`. A human-readable name for the archive. |
| `description` | no | `Description:` in `Release`. |
| `suite` | yes | `Suite:` — the rolling name users write in `sources.list` (`stable`). |
| `codename` | yes | `Codename:` — the specific name. Equal to `suite` for most projects. |
| `components` | yes | `Components:`. Almost always `[main]`. |
| `architectures` | yes | `Architectures:`. Advertised in `Release` so the set stays stable across releases where a build failed. |

`architectures` declares what the repository *offers*. It is not used to
interpret packages: a package's architecture is read from its control stanza,
and a package whose architecture is not in this list is a hard error rather than
a silent omission.

**`Architecture: all` is the exception, and it is not optional.** An
architecture-independent package declares `all`, which is not an architecture
and never appears in this list. Such a package is listed in the `Packages` index
of *every* declared architecture, because that is where `apt` looks for it —
there is no `binary-all` index in the paths a client fetches. Treating `all` as
"not in `architectures`" would reject every arch-independent package, which is
why it is called out here rather than left to the reader.

## Signing

| Field | Required | Notes |
|---|---|---|
| `signing.key_id` | yes | Fingerprint of the **signing subkey**. Not the primary. See [Signing Keys](Signing-Keys.md#two-fingerprints-and-why-they-differ). |

The private key is read from `ARCHIVIST_GPG_KEY` and has no configuration field
by design. A key in a config file gets committed; a key passed as a flag lands
in shell history and in the process table.

## Publishing

| Field | Required | Notes |
|---|---|---|
| `publish.bucket` | yes | Destination bucket. |
| `publish.endpoint` | no | S3-compatible endpoint. Omit for Amazon S3. |
| `publish.region` | no | Defaults to `auto`, which suits R2 and MinIO. |
| `publish.public_url` | yes | The URL users will reach the repository at. Used for the generated install snippet — it can differ from the endpoint, and for a CDN-fronted bucket it always does. |
| `publish.prefix` | no | Key prefix, for serving a repository from a subdirectory. |

Credentials come from the standard AWS environment variables and are never read
from this file.

## Environment variables

| Variable | Purpose |
|---|---|
| `ARCHIVIST_GPG_KEY` | Armoured signing subkey. |
| `ARCHIVIST_GPG_PASSPHRASE` | Only if the subkey is passphrase-protected. |
| `AWS_ACCESS_KEY_ID` | Object storage access key. |
| `AWS_SECRET_ACCESS_KEY` | Object storage secret key. |
