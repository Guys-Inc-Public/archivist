# Decisions

Choices that are expensive to reverse, with the reasoning that produced them.
Recording the reasoning matters more than recording the choice: a decision whose
justification is written down can be re-examined when the circumstances change,
and one whose justification is lost can only be defended or abandoned.

## Format

Each record states the **context** that forced a choice, the **decision**, and
its **consequences** — including the ones we would rather not have. Records are
immutable once merged. A decision that changes is *superseded* by a new record
rather than edited, so the history of the design stays legible.

## Index

| # | Decision | Status |
|---|---|---|
| [0001](0001-regenerate-from-scratch.md) | Regenerate repository metadata from scratch, with a metadata sidecar | Accepted |
| [0002](0002-signing-key-handling.md) | CI holds a signing subkey; the primary stays offline | Accepted |
| [0003](0003-s3-compatible-storage.md) | Target S3-compatible storage; special-case no provider | Accepted |
| [0004](0004-single-suite-in-v0-1.md) | One suite in `v0.1`, but `suite` is a required field | Accepted |
| [0005](0005-cli-first-action-wrapper.md) | The CLI is the product; the Action is a thin wrapper | Accepted |
| [0006](0006-identity-from-control-stanzas.md) | Package identity comes from control stanzas, never filenames | Accepted |
| [0007](0007-docs-in-repo-mirrored-to-wiki.md) | `docs/` is the source of truth; the wiki is generated | Accepted |
| [0008](0008-tool-owns-install-snippet.md) | `archivist` generates the install snippet | Accepted |
| [0009](0009-openpgp-library-not-gpg.md) | Sign with a Go OpenPGP library, not by invoking `gpg` | Accepted |
| [0010](0010-object-storage-sdk.md) | Use `aws-sdk-go-v2` for object storage | Accepted |
| [0011](0011-valid-until-is-opt-in.md) | `Valid-Until` is opt-in and unset by default | Accepted |
| [0012](0012-metadata-sidecar-format.md) | The metadata sidecar format | Accepted |
