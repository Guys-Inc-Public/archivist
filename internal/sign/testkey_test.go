package sign_test

import (
	"bytes"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// testKey is a freshly generated key and the two armoured exports a test needs
// from it. Keys are generated per run rather than committed: a private key in a
// repository is a private key on the internet, whatever the comment above it
// says.
type testKey struct {
	entity *openpgp.Entity

	// full is what an unprepared user would paste into a CI secret, private
	// material for the primary included.
	full string

	// subkeyOnly is the shape `gpg --export-secret-subkeys` produces, and the
	// only shape that belongs in CI: no private material for the primary.
	subkeyOnly string

	primaryFingerprint string
	signingFingerprint string
}

func newTestKey(t *testing.T, passphrase string) *testKey {
	t.Helper()

	e, err := openpgp.NewEntity("Example Project APT", "", "apt@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddSigningSubkey(nil); err != nil {
		t.Fatal(err)
	}

	k := &testKey{entity: e, primaryFingerprint: fingerprintOf(e.PrimaryKey)}
	for _, sub := range e.Subkeys {
		if sub.Sig != nil && sub.Sig.FlagSign {
			k.signingFingerprint = fingerprintOf(sub.PublicKey)
		}
	}
	if k.signingFingerprint == "" {
		t.Fatal("generated key has no signing subkey")
	}

	// Serialise the unencrypted subkey-only export before locking the key, so
	// both shapes describe the same key.
	k.subkeyOnly = armour(t, openpgp.PrivateKeyType, subkeyOnlyPackets(t, e))

	if passphrase != "" {
		if err := e.EncryptPrivateKeys([]byte(passphrase), nil); err != nil {
			t.Fatal(err)
		}
		k.subkeyOnly = armour(t, openpgp.PrivateKeyType, subkeyOnlyPackets(t, e))
	}

	var fullRaw bytes.Buffer
	if err := e.SerializePrivateWithoutSigning(&fullRaw, nil); err != nil {
		t.Fatal(err)
	}
	k.full = armour(t, openpgp.PrivateKeyType, fullRaw.Bytes())

	return k
}

// subkeyOnlyPackets writes the primary as a public key and the subkeys as
// private ones. gpg writes a gnu-dummy stub for the primary instead; for the
// property being tested the two are the same thing, which is that no private
// material for a certify-capable key is present.
func subkeyOnlyPackets(t *testing.T, e *openpgp.Entity) []byte {
	t.Helper()
	var raw bytes.Buffer
	if err := e.PrimaryKey.Serialize(&raw); err != nil {
		t.Fatal(err)
	}
	for _, id := range e.Identities {
		if err := id.UserId.Serialize(&raw); err != nil {
			t.Fatal(err)
		}
		if err := id.SelfSignature.Serialize(&raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, sub := range e.Subkeys {
		if sub.PrivateKey == nil {
			continue
		}
		if err := sub.PrivateKey.Serialize(&raw); err != nil {
			t.Fatal(err)
		}
		if err := sub.Sig.Serialize(&raw); err != nil {
			t.Fatal(err)
		}
	}
	return raw.Bytes()
}

func armour(t *testing.T, blockType string, raw []byte) string {
	t.Helper()
	var out bytes.Buffer
	w, err := armor.Encode(&out, blockType, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out.WriteByte('\n')
	return out.String()
}

func fingerprintOf(pk *packet.PublicKey) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(pk.Fingerprint)*2)
	for _, c := range pk.Fingerprint {
		out = append(out, digits[c>>4], digits[c&0x0f])
	}
	return string(out)
}
