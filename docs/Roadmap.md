# Roadmap

## Scope discipline

The single biggest risk to this project is scope.

**`v0.1` does exactly one thing:** take a directory of `.deb` files, produce a
signed apt repository, publish it to object storage.

**Explicitly out of scope for `v0.1`:**

- RPM repositories — `v0.2`
- Building packages — `archivist` consumes artifacts, it does not produce them
- A hosting service, or SaaS anything
- A web UI
- Package promotion between channels
- Anything Kubernetes

If `v0.1` cannot be explained in a two-sentence README, it is too big.

## Milestones

| | Milestone | Done when |
|---|---|---|
| **M0** | Scaffolding | Repository, governance, CI/CD, and decisions recorded. **Complete.** |
| **M1** | Local round trip | The CLI turns a directory of `.deb` files into a valid signed repository tree on local disk, verified by pointing a real `apt` at it over `file://`. No cloud involved. |
| **M2** | Publish | The same tree uploads to S3-compatible storage, verified by installing from a live URL in a clean container. |
| **M3** | Dogfood | `github-desktop-linux` uses `archivist` for its actual releases. **This is the milestone that matters** — until it happens, this is a toy. |
| **M4** | Action and docs | Composite Action published, copy-pasteable workflow in the README, signing-key documentation finished, repository public. |
| **M5** | First external user | Not something we control. But the "how do I use this" path should be frictionless before we go looking. |

## Current state

M0 is complete. What exists today:

- `archivist version` and `archivist inspect` work.
- `internal/deb` parses control stanzas and computes pool paths, under test.
- `build`, `publish` and `verify` parse their arguments and exit `2`.

## Open questions

Tracked in [Discussions](https://github.com/Guys-Inc-Public/archivist/discussions),
not here, so that answers arrive with their reasoning attached. The ones that
were open at kickoff and are now settled have become
[decisions](decisions/README.md).

Still genuinely open:

- Does `verify` belong to `archivist`, or is a standalone verifier that has no
  publishing code — and therefore no key handling — a stronger claim?
- Is regenerate-from-scratch still right when a project's package set has
  hundreds of versions? The
  [sidecar approach](decisions/0001-regenerate-from-scratch.md) makes the
  arithmetic work, but it has not been tested at that size.

## Kill criteria

Worth stating honestly: if by M3 the extraction is fighting us — if too much of
`github-desktop-linux`'s pipeline turns out to be Electron-shaped — the right
answer is to stop, keep the scripts project-local, and spend the time elsewhere.
Extracting a generic tool from one concrete case is a known trap.

The [extraction inventory](Extraction-Inventory.md) is the evidence this gets
judged against. Its finding: the packaging half is entirely Electron-shaped and
was already out of scope, and the publishing half is about forty substantive
lines of shell. The risk is not that the work is hard. It is that one data point
is a poor basis for a configuration schema — so **reach M3 early**, and find a
second consumer before the config format is frozen at `v0.1.0`.
