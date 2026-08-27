package sign

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Where key material comes from. Neither has a configuration field: a key in a
// config file gets committed, and a key passed as a flag lands in shell history
// and in the process table.
const (
	EnvKey = "ARCHIVIST_GPG_KEY"
	// #nosec G101 -- this is the name of an environment variable, not a secret.
	EnvPassphrase = "ARCHIVIST_GPG_PASSPHRASE"
)

// ErrCertifyCapable reports a key that can certify other keys. Such a key is
// an identity, not a credential, and it does not belong in CI.
var ErrCertifyCapable = errors.New("the key carries private material for a certify-capable key; CI must hold a signing subkey only")

// Key is a signing subkey held in memory.
type Key struct {
	// signing is the entity reduced to the one subkey the configuration names,
	// because the OpenPGP signing calls pick a key from an entity themselves
	// and would otherwise be free to pick a different one.
	signing *openpgp.Entity
	full    *openpgp.Entity
	fp      string
	config  *packet.Config
}

// FromEnvironment loads the signing key named by keyID from ARCHIVIST_GPG_KEY.
func FromEnvironment(keyID string) (*Key, error) {
	armoured := os.Getenv(EnvKey)
	if strings.TrimSpace(armoured) == "" {
		return nil, fmt.Errorf("%s is not set; see docs/Signing-Keys.md", EnvKey)
	}
	return LoadKey(strings.NewReader(armoured), os.Getenv(EnvPassphrase), keyID)
}

// LoadKey reads an armoured private key, decrypts it if necessary, and selects
// the subkey the configuration names.
func LoadKey(r io.Reader, passphrase, keyID string) (*Key, error) {
	entities, err := openpgp.ReadArmoredKeyRing(r)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", EnvKey, err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("%s contains no keys", EnvKey)
	}

	for _, e := range entities {
		if err := refuseCertifyCapable(e); err != nil {
			return nil, err
		}
	}

	want := strings.ToUpper(keyID)
	for _, e := range entities {
		if err := decryptPrivateKeys(e, passphrase); err != nil {
			return nil, err
		}
		signing, fp, err := selectSigningKey(e, want)
		if err != nil {
			return nil, err
		}
		if signing == nil {
			continue
		}
		return &Key{
			signing: signing,
			full:    e,
			fp:      fp,
			// SHA-256 throughout: it is what the Hash header in InRelease will
			// claim, and the two have to agree or verification fails.
			config: &packet.Config{DefaultHash: crypto.SHA256},
		}, nil
	}
	return nil, fmt.Errorf("no signing key with fingerprint %s in %s", keyID, EnvKey)
}

// refuseCertifyCapable enforces the rule in docs/Signing-Keys.md.
//
// Capability alone is not the test. `gpg --export-secret-subkeys` leaves a
// gnu-dummy stub of the primary in the export, so a correctly prepared CI key
// and a full secret key both report a certify-capable primary. What separates
// them is whether the private material is actually there, which is what
// Dummy() answers.
func refuseCertifyCapable(e *openpgp.Entity) error {
	if e.PrivateKey == nil || e.PrivateKey.Dummy() {
		return nil
	}
	sig, _ := e.PrimarySelfSignature()
	// A primary with no key-flags subpacket is certify-capable by default, so
	// the absence of flags is not permission.
	if sig != nil && sig.FlagsValid && !sig.FlagCertify {
		return nil
	}
	return fmt.Errorf("%s: %w", fingerprintOf(e.PrimaryKey), ErrCertifyCapable)
}

func decryptPrivateKeys(e *openpgp.Entity, passphrase string) error {
	keys := []*packet.PrivateKey{e.PrivateKey}
	for i := range e.Subkeys {
		keys = append(keys, e.Subkeys[i].PrivateKey)
	}
	for _, k := range keys {
		if k == nil || k.Dummy() || !k.Encrypted {
			continue
		}
		if passphrase == "" {
			return fmt.Errorf("key %s is passphrase-protected; set %s", fingerprintOf(&k.PublicKey), EnvPassphrase)
		}
		if err := k.Decrypt([]byte(passphrase)); err != nil {
			return fmt.Errorf("decrypting key %s with %s: %w", fingerprintOf(&k.PublicKey), EnvPassphrase, err)
		}
	}
	return nil
}

// selectSigningKey reduces an entity to the subkey the configuration names.
func selectSigningKey(e *openpgp.Entity, want string) (*openpgp.Entity, string, error) {
	for i := range e.Subkeys {
		sub := e.Subkeys[i]
		fp := fingerprintOf(sub.PublicKey)
		if fp != want {
			continue
		}
		if sub.PrivateKey == nil || sub.PrivateKey.Dummy() {
			return nil, "", fmt.Errorf("subkey %s carries no private material", fp)
		}
		reduced := *e
		reduced.Subkeys = []openpgp.Subkey{sub}
		return &reduced, fp, nil
	}

	// The commonest configuration mistake, and the docs say so: users verify
	// the primary fingerprint, so that is the one they have to hand.
	if fingerprintOf(e.PrimaryKey) == want {
		return nil, "", fmt.Errorf(
			"%s is the primary key's fingerprint; signing.key_id must be the signing subkey - see docs/Signing-Keys.md", want)
	}
	return nil, "", nil
}

// Fingerprint returns the 40-character fingerprint of the signing subkey.
func (k *Key) Fingerprint() string { return k.fp }

// PublicKey returns the public key in the two forms an install snippet needs:
// armoured for a .asc file, and binary for /etc/apt/keyrings.
//
// The whole key is exported, primary included. Exporting only the subkey would
// give users something that verifies today's signature and nothing about the
// identity that vouches for it, which is the part that survives rotation.
func (k *Key) PublicKey() (armoured, binaryKey []byte, err error) {
	var raw bytes.Buffer
	if err := k.full.Serialize(&raw); err != nil {
		return nil, nil, err
	}

	var armouredBuf bytes.Buffer
	w, err := armor.Encode(&armouredBuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return nil, nil, err
	}
	if _, err := w.Write(raw.Bytes()); err != nil {
		return nil, nil, err
	}
	if err := w.Close(); err != nil {
		return nil, nil, err
	}
	// A trailing newline, because every other tool that writes a .asc emits one
	// and its absence makes a diff look like a change.
	armouredBuf.WriteByte('\n')

	return armouredBuf.Bytes(), raw.Bytes(), nil
}

func fingerprintOf(pk *packet.PublicKey) string {
	if pk == nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(pk.Fingerprint))
}
