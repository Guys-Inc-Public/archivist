# 0009 — Sign with a Go OpenPGP library, not by invoking `gpg`

**Status:** Accepted · **Date:** 2026-08-27

## Context

Something has to produce the signatures on `Release` and `InRelease`. Two
options: shell out to `gpg`, which is what `reprepro` does, or use a Go OpenPGP
implementation.

Shelling out is the safer-looking choice — it is unquestionably compatible,
because it is the same program `apt` verifies against. But it makes the binary a
liar. The kickoff calls a single static binary "close to mandatory" for a CI
tool, and a tool that requires GnuPG, a `GNUPGHOME`, and an agent that behaves
differently in every container is not that. It also pushes the failure onto
users: the tool works on the maintainer's laptop and fails in someone else's
minimal image.

The standard library's `x/crypto/openpgp` is frozen and deprecated, so the
maintained path is `github.com/ProtonMail/go-crypto`.

## Decision

**Use `github.com/ProtonMail/go-crypto/openpgp`.** Keep signing behind the
`Signer` interface promised by [0002](0002-signing-key-handling.md), with a
`gpg`-invoking implementation left as a documented escape hatch, so this is
reversible without touching repository generation.

This was validated before being adopted, against GnuPG 2.2.40 — Debian
bookworm's version, and the `gpgv` `apt` actually runs.

## Consequences

- The binary stays self-contained. No GnuPG at runtime, on any machine.
- **`clearsign.Encode` cannot be used.** It calls
  `armor.EncodeWithChecksumOption(..., false)` with the flag hardcoded, so its
  output carries no CRC24. GnuPG 2.2 rejects such a block: it prints
  `Good signature` and then exits 2 with `no valid OpenPGP data found`. The
  signature is fine; the armour is not, and there is no configuration knob.
  `InRelease` is therefore assembled directly, using `ArmoredDetachSignText` for
  the signature block, which does emit the checksum.
- Two canonicalisation rules must be implemented rather than assumed, because
  getting either wrong produces `BAD signature` — a failure that reads like a
  key problem and is not one:
  - the signature is **text mode** (sigclass `0x01`), not a binary document
    signature;
  - per RFC 4880 §7.1 the signed bytes are not the document bytes — trailing
    whitespace is stripped from every line and the final line terminator is
    excluded.
- Certify-capability detection works on the parsed key material via
  `PrivateKey.Dummy()`, which correctly accepts an export whose primary is a
  stub and refuses one carrying real primary material. Confirmed to hold when
  the primary lives on a smartcard, which still exports as a `gnu-dummy` stub
  rather than a card stub.
- One dependency, plus its transitive `circl` and `x/crypto`.
