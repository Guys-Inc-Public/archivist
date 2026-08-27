# Repository Layout

What `archivist` publishes, and why each file is there. Everything below follows
the Debian archive format; none of it is invented.

Paths below are relative to the repository root — the bucket root, or the key
prefix within it if you publish into one. A repository is entirely
self-contained under that root, which is what lets several of them share one
bucket.

```
<root>/
├── dists/
│   └── stable/                          <- codename
│       ├── Release                      signed manifest of everything below
│       ├── Release.gpg                  detached signature over Release
│       ├── InRelease                    Release with an inline signature
│       └── main/                        <- component
│           ├── binary-amd64/
│           │   ├── Packages             control stanzas, one per package
│           │   ├── Packages.gz
│           │   └── Release              per-architecture marker
│           ├── binary-arm64/
│           └── binary-armhf/
├── pool/
│   └── main/
│       └── g/github-desktop/
│           ├── github-desktop_3.4.9_amd64.deb
│           └── github-desktop_3.4.9_amd64.deb.archivist.json
├── public.asc                           armoured public key
└── public.gpg                           dearmoured, for /etc/apt/keyrings
```

That is the whole tree. There is no landing page: `apt` never fetches `/`, and a
generated one would be a website to maintain and a file to clobber in a shared
bucket. See [decision 0014](decisions/0014-no-generated-landing-page.md) — where
a bare URL should lead is a hosting question, answered with a redirect.

## The trust chain

This is the whole security model, and it is worth being able to recite:

1. The user installs your **public key** out of band, once.
2. `apt` fetches **`InRelease`** and verifies its signature against that key.
3. `InRelease` contains **checksums of every `Packages` file**.
4. Each `Packages` file contains the **`SHA256` of every `.deb`** it lists.
5. `apt` verifies the downloaded `.deb` against that hash.

Every link is a hash or a signature. Break any one and the chain does not
degrade to "less secure" — it fails closed, which is the correct behaviour and
also why a partially-published repository is worse than no repository.

## `Release` and `InRelease`

`InRelease` is the signed-inline form and is what modern `apt` fetches.
`Release` plus the detached `Release.gpg` is the older arrangement. `archivist`
writes both, because the cost is one extra file and the alternative is breaking
older clients for no gain.

## `pool/` and the prefix directories

The prefix directory (`g/` above) is the first letter of the **source** package
name, except for names starting with `lib`, which use the first four characters
(`libs/libsecret-1-0/`). Debian adopted this because a flat pool with tens of
thousands of library packages is unpleasant for filesystems and mirrors alike.
It is preserved here so that standard mirroring tools work without special
cases.

Pool paths derive from control fields only. A package that arrives named
`GitHubDesktop-linux-amd64-3.4.9.deb` is published as
`github-desktop_3.4.9_amd64.deb`, because the first name encodes one project's
release convention and the second is what the archive format specifies. See
[decision 0006](decisions/0006-identity-from-control-stanzas.md).

## The sidecars

Each pool object has a `.archivist.json` file beside it holding that package's
control stanza and checksums. They are how an index is regenerated without
downloading the pool, and they are not part of the Debian archive format:
standard tools copy them and clients ignore them, so a mirror of an `archivist`
repository is still a valid apt repository. See
[decision 0012](decisions/0012-metadata-sidecar-format.md).

## Publish ordering

Objects are uploaded in dependency order — pool objects, then `Packages`, then
`Release` and its signatures last. A client fetching mid-publish therefore sees
either the previous repository or the new one, never a `Release` promising
content that has not landed.

Deletion is the mirror image: nothing is removed until the new `Release` no
longer references it.

## Architecture-independent packages

A package declaring `Architecture: all` appears in the `Packages` index of every
architecture the repository offers. `apt` fetches
`dists/<suite>/<component>/binary-<arch>/Packages` for the architecture it is
running on and nothing else, so an arch-independent package listed only once
would be invisible to every client. The pool object is stored once; only the
index entry is repeated.

## Suites

The directory under `dists/` is the **codename**, which is Debian's own
convention — `dists/bookworm/`, not `dists/stable/`. `Release` carries both
`Suite:` and `Codename:`, so a client is free to call the archive either name,
but the path it fetches is the codename. For most projects the two are equal and
the distinction never comes up; where they differ, the install snippet
`archivist` generates uses the codename, because that is the one that resolves.

`v0.1` publishes a single suite. `suite` is a required configuration field
regardless, so that adding `testing` alongside `stable` later is a configuration
change rather than a change to the published layout — which would invalidate
every user's `sources.list` line. See
[decision 0004](decisions/0004-single-suite-in-v0-1.md).
