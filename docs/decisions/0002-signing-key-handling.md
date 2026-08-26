# 0002 — CI holds a signing subkey; the primary stays offline

**Status:** Accepted · **Date:** 2026-08-26

## Context

This is the part of the project that will embarrass us if we get it wrong, and
it is the part users are trusting us on.

A CI secret is reachable by every workflow in the repository, every Action those
workflows depend on, and everyone with write access. A primary OpenPGP key in
that position is not a signing key that happens to live in CI — it is the
project's identity, and anything that can read it can issue keys that users will
trust.

## Decision

**CI holds a signing subkey and nothing else.** The primary key is
certify-capable only, is generated off the build machine, and stays offline.

This is enforced rather than recommended: on import, `archivist` inspects the
key's capabilities and refuses a key that can certify. The release workflow
performs the same check on its own signing key.

Additionally:

- The signing subkey carries an expiry, and the rotation procedure is documented
  before launch rather than after the first expiry surprises everyone.
- Signing sits behind an interface, so an external signer or HSM can replace the
  default without touching repository generation. `v0.1` ships the CI-secret
  implementation and leaves the seam.
- The primary fingerprint is published in more than one place.

## Consequences

- A compromised CI secret costs a subkey rotation, which is invisible to users,
  rather than a primary rotation, which requires every user to install a new key.
- Users verify a fingerprint that is not the one that signs. This is
  conventional and correct, and it confuses people, so it is stated explicitly
  in [Signing Keys](../Signing-Keys.md#two-fingerprints-and-why-they-differ).
- Setup is more work than "generate a key, paste it into a secret". The
  documentation carries that cost with copy-pasteable commands.
- Refusing a certify-capable key will reject a working configuration for
  somebody who deliberately chose otherwise. We accept that: the failure is
  loud, immediate, and explains itself, which is preferable to silently
  accepting a key that should not be there.
