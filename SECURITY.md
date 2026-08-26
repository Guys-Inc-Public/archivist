# Security Policy

The [organisation security policy](https://github.com/Guys-Inc-Public/.github/blob/main/SECURITY.md)
applies. This file adds what is specific to `archivist`.

## Reporting a vulnerability

Use [private vulnerability reporting](https://github.com/Guys-Inc-Public/archivist/security/advisories/new)
on this repository, or email **CJ@guysinc.org**. Please do not open a public issue.

## Why this project warrants extra care

`archivist` handles GPG signing material and produces the metadata that `apt`
trusts to decide whether a package is authentic. A defect here can cause a user's
package manager to accept something it should have rejected. Reports in the
following areas are especially welcome:

- **Signature generation** — anything that produces a `Release`, `Release.gpg` or
  `InRelease` that `apt` accepts but that does not actually attest to the
  package contents.
- **Checksum handling** — a `Packages` index whose hashes do not bind the pool
  objects they name.
- **Key handling** — key material reaching disk, logs, process arguments, error
  output, or the published repository tree.
- **Publish path** — a partial or failed upload that leaves a published
  repository in a state `apt` will still accept.

## What we consider out of scope

- Vulnerabilities in `apt`, `gnupg`, or object storage providers themselves.
- Attacks that require an attacker to already hold the signing key.
- Missing hardening that has no demonstrated impact.

## Signing keys

`archivist` releases are signed. The key policy, fingerprints, and rotation
procedure are documented in [Signing Keys](./docs/Signing-Keys.md). If a Guys Inc
signing key is believed to be compromised, report it through the channels above
and we will publish a revocation and a rotation notice.
