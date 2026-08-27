# Signing Keys

This is the part users are trusting you on, so it is written down before launch
rather than after the first expiry surprises everyone.

## The rule

**The primary key never enters CI.** CI holds a signing subkey and nothing else.
If your CI secret contains a key that can certify, you have put your identity in
a place that every workflow, every Action you depend on, and every person with
write access can reach.

`archivist` enforces this rather than merely recommending it: on import it
inspects the key's capabilities and refuses a key with the certify capability.

## Generating a key

Generate the primary key on a machine that is not a build server, and keep it
offline afterwards.

```console
$ gpg --quick-generate-key "Example Project APT <apt@example.com>" rsa4096 cert 3y
```

`cert` gives the primary key the certify capability *only* — it cannot sign
data. That is deliberate: it means the primary's job is to vouch for subkeys and
nothing else.

Add a signing subkey with a shorter life than the primary:

```console
$ gpg --quick-add-key <PRIMARY_FINGERPRINT> rsa4096 sign 2y
```

Export the subkey — note the trailing `!`, which is what restricts the export to
that specific subkey rather than the whole keyring:

```console
$ gpg --armor --export-secret-subkeys <SUBKEY_FINGERPRINT>! > signing-subkey.asc
```

Put the contents of `signing-subkey.asc` in your CI secret. Then back up the
primary key somewhere offline, delete it from the build machine, and generate a
revocation certificate before you need one:

```console
$ gpg --output revoke.asc --gen-revoke <PRIMARY_FINGERPRINT>
```

## Two fingerprints, and why they differ

This confuses people, so state it plainly in your own install docs:

| Key | Fingerprint | Role |
|---|---|---|
| Primary `[C]` | what users verify | Certifies subkeys. Offline. |
| Subkey `[S]` | what appears in `SignWith` / `signing.key_id` | Signs `Release`. Lives in CI. |

Users verify the **primary** fingerprint, because that is the stable identity.
The key that actually signs is the **subkey**, and it changes on rotation. A
rotation that replaces the subkey requires no action from your users; that is
the entire benefit of the arrangement.

Publish the primary fingerprint in more than one place — the repository index
page, your project README, and the release notes at minimum. A fingerprint that
exists in exactly one location is a fingerprint an attacker only has to replace
once.

## The Guys Inc keys

One primary certifies **two** signing subkeys. Users verify the primary
fingerprint and nothing else; which subkey signed a given artifact is an
implementation detail they never need to care about.

```
primary     F45BB6D34D82EF56BB97FBE0F305FB33592B46C8  rsa4096 [C]  expires 2029-08-26
 |
 +- sign    63029D0DD76E40614D44631FB7CAA8B2B498960D  rsa4096 [S]  archivist releases
 +- sign    65B6F5564EAF3BC46D38867B10239ACC5F23DEEE  rsa4096 [S]  github-desktop apt repository
```

Two subkeys rather than one, because a single subkey shared between two CI
systems means a compromise of either repository can sign for both. Splitting
them costs users nothing — the fingerprint they check is the primary's either
way — and it means each repository's Actions secret can only sign that
repository's artifacts.

## Custody

Where each piece lives, and why there. The rule underneath is that the thing
which is hard to replace should be hard to reach.

| Artifact | Lives at | Reached |
|---|---|---|
| Primary `[C]` | YubiKey 5 | A few times a year, to certify a subkey |
| Primary backup, encrypted | 1Password, plus an offline copy elsewhere | Only if the YubiKey is lost |
| Revocation certificate | 1Password, plus the same offline copy | Only on compromise |
| Signing subkeys | Each repository's Actions secret | Every release |
| Subkey exports | 1Password | Re-provisioning, since Actions secrets cannot be read back |
| Public key | This repository and the published repository | Constantly, by everyone |

The YubiKey is deliberately **not** in the signing path — no hosted runner can
reach a USB device. It holds the primary, whose only job is certification. What
signs releases is a software key in a CI secret, and that is the arrangement
working as intended: a leaked subkey costs a rotation nobody notices, a leaked
primary costs every user a manual reinstall.

Generate on an air-gapped live system — the value is that the OS is RAM-only and
leaves nothing behind, not that the hardware is special. Write the revocation
certificate and the encrypted backup **before** moving the primary to the
YubiKey: `keytocard` moves the key and leaves a stub, so a missing backup means
the only copy is on hardware that can be lost.

## Rotation

Rotate the signing subkey **before** it expires, not after. An expired signing
key does not degrade gracefully: `apt update` fails outright for every user, and
they cannot install the fix because the mechanism that would deliver it is the
thing that broke.

Schedule the work at **T-minus 90 days**:

1. Generate a replacement subkey under the same primary — plug in the YubiKey;
   no air-gapped session is needed, because the primary is on the card.
2. Export it and update that repository's CI secret. Rotating one subkey does
   not touch the other repository.
3. Update `signing.key_id` in `archivist.yml`, and the
   `RELEASE_GPG_FINGERPRINT` Actions **variable**, which GoReleaser reads.
4. Publish a repository with the new subkey. The public key block that ships in
   the repository contains both subkeys, so clients that have not refreshed
   still validate.
5. Verify from a clean container before announcing anything —
   see [Verification](Verification.md).
6. After a full release cycle, remove the old subkey from the published key.

Rotating the **primary** is a different and much larger job: every user must
install a new key. Do it only on compromise, and expect to need an announcement
on every channel you have.

## On compromise

1. Publish the revocation certificate to the repository and to any keyserver
   where the key appeared.
2. Rotate immediately, and treat every package signed after the suspected
   compromise as untrusted until re-signed.
3. Announce it. A signing key compromise that users learn about from someone
   else costs more trust than the compromise itself.

## Why not Sigstore / keyless

`apt` verifies OpenPGP signatures on `Release`. That is not a design choice
`archivist` gets to make — it is the format the client implements, so the
repository signature is OpenPGP.

`archivist`'s *own releases* are a different matter, and they carry a Sigstore
build-provenance attestation in addition to a GPG signature over the checksums.
See [Verification](Verification.md#verifying-an-archivist-release).
