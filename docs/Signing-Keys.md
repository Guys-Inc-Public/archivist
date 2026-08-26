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

```
primary  9129 8955 2DCD 86E6 9150  D032 753B 218B 25FE 5F74   rsa4096 [C]  expires 2029-08-25
subkey   246F E9B7 D018 E7A8 C906  94AF 1737 19DD ADE2 ACFE   rsa4096 [S]  expires 2028-08-25
```

## Rotation

Rotate the signing subkey **before** it expires, not after. An expired signing
key does not degrade gracefully: `apt update` fails outright for every user, and
they cannot install the fix because the mechanism that would deliver it is the
thing that broke.

Schedule the work at **T-minus 90 days**:

1. Generate a replacement subkey under the same primary.
2. Export it and update the CI secret.
3. Update `signing.key_id` in `archivist.yml`.
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
