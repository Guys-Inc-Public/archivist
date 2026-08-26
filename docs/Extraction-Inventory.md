# Extraction Inventory

The Step 1 survey of `github-desktop-linux`, separating its release pipeline
into what could be extracted, what needed parameterising, and what had to stay
behind. Recorded here because it is the evidence the
[kill criteria](Roadmap.md#kill-criteria) get judged against.

Surveyed 2026-08-26 against `.github/workflows/`, `apt/` and `script/`.

## Headline finding

The kickoff framed this as an *extraction*. It is not, and planning it as one
misleads about the effort.

The pipeline splits in two. The **packaging half** — `package-debian.ts`,
`package-redhat.ts`, `dist-info.ts`, the maintainer scripts — is entirely
Electron-shaped, and was already out of scope. The **publishing half** is one
workflow file: about forty substantive lines, of which the `reprepro`
invocation is six.

Everything generic in it is a *fragment* inside a project-specific file — a
`concurrency` block, a `gpg --dearmor`, an attestation step — not a file that
can be moved. **No file extracts unchanged.**

There is therefore no meaningful code reuse available, and that is fine. The
asset is a validated design and a real dogfood target, not the shell.

## Buckets

Three buckets as specified, plus one the material forced: **Split**, for files
where part is tool output and the rest stays behind.

### `.github/workflows/` — 12 files

| File | Bucket | Notes |
|---|---|---|
| `publish-apt.yml` | **Parameterised** | The whole `v0.1` product. Hardcoded and becoming configuration: bucket name, R2 endpoint, `apt/conf/distributions`, key path, `index.html`, suite `stable`, the `*.deb` pattern. Worth keeping: the `concurrency` group — two concurrent publishes to one bucket corrupt each other — and the `gpg --dearmor` keyring step. |
| `ci-linux.yml` | **Split** | Three Electron container builds via pinned packaging forks are project-specific. The `actions/attest-build-provenance` step is generic, already working, and is the model for this project's own release signing. |
| `create-draft-release.yml` | Project | Rebases the fork onto upstream `desktop/desktop` tags. |
| `release-pr.yml` | Project | Inherited `releases/` branch convention and a second GitHub App. |
| `ci.yml`, `codeql.yml`, `sync-with-upstream.yml` | Project | Application build, scanning, upstream sync. |
| `no-response.yml`, `triage-issues.yml`, `close-invalid.yml`, `close-single-word-issues.yml`, `on-issue-close.yml` | Project | Community issue automation. Not pipeline. |

### `apt/` — 3 files, the densest signal in the repository

| File | Bucket | Notes |
|---|---|---|
| `conf/distributions` | **Parameterised** | The configuration schema, already written. Eight fields, every one a config value: `Origin`, `Label`, `Codename`, `Suite`, `Architectures`, `Components`, `Description`, `SignWith`. `archivist.yml` takes its field names from these, because they are the Debian vocabulary and users already know them. |
| `guysinc-apt.asc` | **Split** | The key is project-specific. The mechanic is generic and became tool behaviour: publish the armoured key into the tree, and emit a dearmoured `.gpg` beside it. |
| `index.html` | **Split** | Marketing copy stays behind. The install snippet and fingerprint block became [decision 0008](decisions/0008-tool-owns-install-snippet.md). |

### `script/` — 29 entries, effectively all project

| File | Bucket | Notes |
|---|---|---|
| `package-debian.ts` | Project | `electron-installer-debian`, six icon sizes, three `x-scheme-handler` mime types. |
| `package-redhat.ts` | Project | The same for RPM, and `v0.2` territory regardless. |
| `dist-info.ts` | Project | Electron dist layout, Windows and macOS paths, NuGet deltas, channel sniffing. |
| `generate-release-notes.ts`, `changelog/` | Project | Desktop's changelog format. The changelog assumption the kickoff warned about, and it is load-bearing there. |
| `check-build-secrets.sh` | Project | Asserts no OAuth secret in the bundles. A good script, entirely application-specific. |
| `resources/deb/*`, `resources/rpm/*`, `electron-builder-linux.yml` | Project | Maintainer scripts and AppImage configuration. |
| `build-platforms.ts` | **Generic** | Genuinely generic — and eight lines reading two environment variables. Not extracted: cheaper to retype than to depend on. |

## Hidden couplings

Things that feel generic but are not, in the order they would have bitten.

**1. Three architecture vocabularies.** Debian's `amd64`/`arm64`/`armhf`, RPM's
`x86_64`/`aarch64`/`armv7l`, and Node's `x64`/`arm64`/`armv7l` all live in that
repository, mapped by hand in two `getArchitecture()` functions that disagree by
design. Resolved by [decision 0006](decisions/0006-identity-from-control-stanzas.md).

**2. The filename convention — already decoupled.** `package-debian.ts` renames
its output to `GitHubDesktop-linux-${arch}-${version}.deb`, and it would be easy
to assume the publisher depends on that. It does not: it globs `*.deb` and lets
`reprepro` read control files. The best property of the existing pipeline, and
easy to lose by accident when reimplementing.

**3. The eighteen-file assertion.** `generate-release-notes.ts` hard-fails on
`SUCCESSFUL_RELEASE_FILE_COUNT = 3 * 3 * 2` — build-matrix shape masquerading as
a completeness check. Left behind, but the intent was right: something should
verify the published set is complete. That became
[`archivist verify`](Verification.md), expressed against the repository index
rather than a file count.

**4. Channel sniffing from version strings.** `getChannel()` and
`getChannelFromReleaseBranch()` derive `production`/`beta`/`test` by
substring-matching the version number and branch name. The mechanism that would
naturally have grown into suites, and entirely application-shaped. Resolved by
[decision 0004](decisions/0004-single-suite-in-v0-1.md): never infer a suite.

**5. The package source is a GitHub Release, mid-job.**
`gh release download --pattern '*.deb'` sits inside the publish job, making
GitHub a dependency of publishing. Resolved by
[decision 0005](decisions/0005-cli-first-action-wrapper.md): the CLI takes a
directory; fetching from a release is the Action's job.

## Also found: a live defect in the reference pipeline

The state-pull step ends in `|| true`:

```bash
aws s3 sync "s3://$BUCKET" repo --endpoint-url "…" || true
```

If that sync fails — expired credentials, a transient endpoint error — the
failure is swallowed. `reprepro` then builds a repository containing only the
new package, and the final sync, which has no `--delete`, overwrites `dists/`
with single-package indices. Pool objects survive, so nothing is destroyed; they
simply stop being listed. Every previous version becomes invisible to `apt`, and
the run is green.

It self-heals on the next successful publish, which is what makes it hard to
notice. This is the argument for pulling `verify` forward rather than treating
it as a finishing touch — a post-publish check that the index lists what the
pool contains catches this entire class of failure by construction.

Reported upstream against `github-desktop-linux`.
