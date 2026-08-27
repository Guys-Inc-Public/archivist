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

# How long a Release stays valid. Unset by default; see the note below before
# turning it on.
# valid_for: 7d

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

## What each command requires

"Required" depends on the command, because the commands need different things.

| | Requires |
|---|---|
| `archivist build` | Repository identity and `signing.key_id`. Nothing under `publish`. |
| `archivist publish` | Everything `build` requires, plus `publish.bucket` and `publish.public_url`. |

`build` writes a repository tree to local disk and never contacts object
storage, so it does not ask you to invent a bucket name to exercise a path that
has no bucket in it. Fields you *do* supply are still checked for format
whichever command you run: a malformed `public_url` is a mistake the day it is
written, not the day it is first uploaded.

Validation reports every problem it finds at once. A file with four mistakes
produces four messages, each naming the field, rather than four consecutive
runs.

## Repository identity

| Field | Required | Notes |
|---|---|---|
| `origin` | yes | `Origin:` in `Release`. Who publishes this — usually the project name. |
| `label` | yes | `Label:` in `Release`. A human-readable name for the archive. |
| `description` | no | `Description:` in `Release`. |
| `suite` | yes | `Suite:` — the rolling name users write in `sources.list` (`stable`). |
| `codename` | yes | `Codename:` — the specific name. Equal to `suite` for most projects. |
| `components` | yes | `Components:`. Almost always `[main]`. Incoming packages go into the **first** one; see below. |
| `architectures` | yes | `Architectures:`. Advertised in `Release` so the set stays stable across releases where a build failed. |
| `valid_for` | no | How long after its `Date` a `Release` remains valid, written as `Valid-Until:`. **Unset by default.** |

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

### Components

Every component listed here gets a `Packages` index for every architecture, and
all of them are advertised in `Release`.

**Packages are placed in the first component in the list.** `v0.1` has no way to
route one package to `main` and another to `contrib`, so a second component gets
a valid index that stays empty. Listing more than one is therefore only useful
for advertising a component you intend to fill later — which is a real reason,
since removing a component from `Release` later is a breaking change for anyone
who named it in their `sources.list`.

If you only publish `main`, which is almost everyone, none of this comes up.

### `valid_for`

Written as one or more `<count><unit>` terms, where the units are `w`, `d`,
`h`, `m` and `s` — `7d`, `1w`, `1w12h`. Days are supported because a week is
how maintainers think about repository freshness, and `168h` is an invitation
to get the arithmetic wrong.

Leaving it unset is the default and is the right choice for most projects.
`Valid-Until` defends against a freeze attack, but it also means a repository
that stops publishing **breaks on a timer** — `apt update` fails outright for
every user, and it cannot be fixed through the channel it broke. Turn it on if
you publish on a predictable cadence. See
[decision 0011](decisions/0011-valid-until-is-opt-in.md).

## Signing

| Field | Required | Notes |
|---|---|---|
| `signing.key_id` | yes | Fingerprint of the **signing subkey**. Not the primary. See [Signing Keys](Signing-Keys.md#two-fingerprints-and-why-they-differ). |

`key_id` is the full 40-character fingerprint. Spaces and an `0x` prefix are
accepted, so you can paste what `gpg --fingerprint` printed without reformatting
it. A short 8- or 16-character key ID is rejected rather than padded: short IDs
can be forged by collision, and a repository signed by a key you did not mean to
trust is the failure this tool exists to prevent.

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
| `publish.prefix` | no | Key prefix, for serving a repository from a subdirectory of a shared bucket. Leading and trailing slashes are optional; `foo`, `/foo` and `foo/` all mean the same thing. |

Credentials come from the standard AWS environment variables and are never read
from this file.

## Environment variables

| Variable | Purpose |
|---|---|
| `ARCHIVIST_GPG_KEY` | Armoured signing subkey. |
| `ARCHIVIST_GPG_PASSPHRASE` | Only if the subkey is passphrase-protected. |
| `AWS_ACCESS_KEY_ID` | Object storage access key. |
| `AWS_SECRET_ACCESS_KEY` | Object storage secret key. |
