package sign_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Guys-Inc-Public/archivist/internal/sign"
)

// A Release file shaped like the ones internal/repo writes, including the
// indented checksum lines that make it a control stanza rather than a blob.
const releaseBody = `Origin: Example Project
Label: Example Project packages
Suite: stable
Codename: stable
Date: Thu, 27 Aug 2026 12:00:00 UTC
Architectures: amd64 arm64
Components: main
Acquire-By-Hash: no
SHA256:
 e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 0 main/binary-amd64/Packages
`

func loadSubkey(t *testing.T, k *testKey, passphrase string) *sign.Key {
	t.Helper()
	key, err := sign.LoadKey(strings.NewReader(k.subkeyOnly), passphrase, k.signingFingerprint)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	return key
}

// The test that matters: apt does not read our signatures, gpgv does. Anything
// this file asserts about structure is a proxy for this.
func TestSignaturesVerifyWithGpgv(t *testing.T) {
	gpgv, err := exec.LookPath("gpgv")
	if err != nil {
		t.Skip("gpgv is not installed")
	}

	k := newTestKey(t, "")
	key := loadSubkey(t, k, "")

	_, binaryKey, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	keyring := write("keyring.gpg", binaryKey)
	release := write("Release", []byte(releaseBody))

	detached, err := sign.Detached(key, []byte(releaseBody))
	if err != nil {
		t.Fatal(err)
	}
	inline, err := sign.Inline(key, []byte(releaseBody))
	if err != nil {
		t.Fatal(err)
	}
	releaseGPG := write("Release.gpg", detached)
	inRelease := write("InRelease", inline)

	t.Run("Release.gpg", func(t *testing.T) {
		run(t, gpgv, "--keyring", keyring, releaseGPG, release)
	})
	t.Run("InRelease", func(t *testing.T) {
		run(t, gpgv, "--keyring", keyring, inRelease)
	})
}

// gpg is the tool that exposed the armour defect decision 0009 records: it
// prints "Good signature" and then exits 2 when the block carries no CRC24.
// The exit status is the assertion.
func TestInReleaseSurvivesGpgAndItsExitCode(t *testing.T) {
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Skip("gpg is not installed")
	}

	k := newTestKey(t, "")
	key := loadSubkey(t, k, "")
	inline, err := sign.Inline(key, []byte(releaseBody))
	if err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	armouredKey, _, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(home, "public.asc")
	if err := os.WriteFile(keyFile, armouredKey, 0o600); err != nil {
		t.Fatal(err)
	}
	inRelease := filepath.Join(home, "InRelease")
	if err := os.WriteFile(inRelease, inline, 0o600); err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(), "GNUPGHOME="+home)
	runEnv(t, env, gpg, "--batch", "--import", keyFile)
	runEnv(t, env, gpg, "--batch", "--verify", inRelease)
}

// A direct guard on the specific regression: armour with no CRC24 line is what
// GnuPG 2.2 refuses, and a library upgrade could quietly reintroduce it.
func TestSignatureArmourCarriesItsChecksum(t *testing.T) {
	k := newTestKey(t, "")
	key := loadSubkey(t, k, "")

	for name, produce := range map[string]func() ([]byte, error){
		"Release.gpg": func() ([]byte, error) { return sign.Detached(key, []byte(releaseBody)) },
		"InRelease":   func() ([]byte, error) { return sign.Inline(key, []byte(releaseBody)) },
	} {
		body, err := produce()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
		var checksum string
		for i, line := range lines {
			if line == "-----END PGP SIGNATURE-----" && i > 0 {
				checksum = lines[i-1]
			}
		}
		// A CRC24 line is "=" followed by four base64 characters.
		if len(checksum) != 5 || checksum[0] != '=' {
			t.Errorf("%s: line before END PGP SIGNATURE is %q, want a CRC24 checksum", name, checksum)
		}
	}
}

func TestInReleaseContainsTheReleaseItSigns(t *testing.T) {
	k := newTestKey(t, "")
	key := loadSubkey(t, k, "")
	inline, err := sign.Inline(key, []byte(releaseBody))
	if err != nil {
		t.Fatal(err)
	}
	body := string(inline)

	if !strings.HasPrefix(body, "-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA256\n\n") {
		t.Errorf("InRelease does not open with the cleartext header:\n%.120s", body)
	}
	for _, want := range []string{"Origin: Example Project", "Suite: stable", " e3b0c442"} {
		if !strings.Contains(body, want) {
			t.Errorf("InRelease lost %q", want)
		}
	}
	if !strings.Contains(body, "-----BEGIN PGP SIGNATURE-----") {
		t.Error("InRelease carries no signature block")
	}
}

func TestCertifyCapableKeyIsRefused(t *testing.T) {
	k := newTestKey(t, "")

	_, err := sign.LoadKey(strings.NewReader(k.full), "", k.signingFingerprint)
	if !errors.Is(err, sign.ErrCertifyCapable) {
		t.Fatalf("a full secret key was accepted, or failed for the wrong reason: %v", err)
	}
	// The message has to name the key, because a CI log showing only "refused"
	// leaves the operator guessing which secret is wrong.
	if !strings.Contains(err.Error(), k.primaryFingerprint) {
		t.Errorf("error does not name the offending key:\n%v", err)
	}

	// And the correctly prepared export of the same key is accepted, which is
	// the half that a capability-only check gets wrong.
	if _, err := sign.LoadKey(strings.NewReader(k.subkeyOnly), "", k.signingFingerprint); err != nil {
		t.Fatalf("the subkey-only export was refused: %v", err)
	}
}

func TestKeySelection(t *testing.T) {
	k := newTestKey(t, "")

	t.Run("selects the configured subkey", func(t *testing.T) {
		key := loadSubkey(t, k, "")
		if key.Fingerprint() != k.signingFingerprint {
			t.Errorf("signed with %s, want %s", key.Fingerprint(), k.signingFingerprint)
		}
	})

	t.Run("the primary fingerprint gets a useful error", func(t *testing.T) {
		_, err := sign.LoadKey(strings.NewReader(k.subkeyOnly), "", k.primaryFingerprint)
		if err == nil {
			t.Fatal("the primary fingerprint was accepted as signing.key_id")
		}
		if !strings.Contains(err.Error(), "Signing-Keys.md") {
			t.Errorf("error does not point at the documentation:\n%v", err)
		}
	})

	t.Run("an unknown fingerprint is refused", func(t *testing.T) {
		_, err := sign.LoadKey(strings.NewReader(k.subkeyOnly), "",
			"1234567890ABCDEF1234567890ABCDEF12345678")
		if err == nil || !strings.Contains(err.Error(), "1234567890ABCDEF") {
			t.Fatalf("want a complaint naming the fingerprint, got %v", err)
		}
	})
}

func TestPassphrase(t *testing.T) {
	k := newTestKey(t, "correct horse")

	_, err := sign.LoadKey(strings.NewReader(k.subkeyOnly), "", k.signingFingerprint)
	if err == nil || !strings.Contains(err.Error(), sign.EnvPassphrase) {
		t.Fatalf("want an error naming %s, got %v", sign.EnvPassphrase, err)
	}

	_, err = sign.LoadKey(strings.NewReader(k.subkeyOnly), "wrong", k.signingFingerprint)
	if err == nil {
		t.Fatal("the wrong passphrase was accepted")
	}

	key := loadSubkey(t, k, "correct horse")
	if _, err := sign.Detached(key, []byte(releaseBody)); err != nil {
		t.Fatalf("signing with a decrypted key: %v", err)
	}
}

func TestFromEnvironment(t *testing.T) {
	k := newTestKey(t, "")

	if _, err := sign.FromEnvironment(k.signingFingerprint); err == nil {
		t.Fatal("an unset key was accepted")
	} else if !strings.Contains(err.Error(), sign.EnvKey) {
		t.Errorf("error does not name %s:\n%v", sign.EnvKey, err)
	}

	t.Setenv(sign.EnvKey, k.subkeyOnly)
	key, err := sign.FromEnvironment(k.signingFingerprint)
	if err != nil {
		t.Fatalf("FromEnvironment: %v", err)
	}
	if key.Fingerprint() != k.signingFingerprint {
		t.Errorf("Fingerprint = %s", key.Fingerprint())
	}
}

// The public key is what users install once and trust thereafter, so both forms
// have to parse and both have to describe the same key.
func TestPublicKeyIsUsableInBothForms(t *testing.T) {
	k := newTestKey(t, "")
	key := loadSubkey(t, k, "")

	armoured, binaryKey, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	fromArmour, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(armoured))
	if err != nil {
		t.Fatalf("armoured public key does not parse: %v", err)
	}
	fromBinary, err := openpgp.ReadKeyRing(bytes.NewReader(binaryKey))
	if err != nil {
		t.Fatalf("binary public key does not parse: %v", err)
	}

	for name, ring := range map[string]openpgp.EntityList{"armoured": fromArmour, "binary": fromBinary} {
		if len(ring) != 1 {
			t.Fatalf("%s: %d entities, want 1", name, len(ring))
		}
		e := ring[0]
		if got := fingerprintOf(e.PrimaryKey); got != k.primaryFingerprint {
			t.Errorf("%s: primary is %s, want %s", name, got, k.primaryFingerprint)
		}
		// The primary alone would verify nothing: the binding signature over
		// the signing subkey is what makes today's Release checkable.
		found := false
		for _, sub := range e.Subkeys {
			if fingerprintOf(sub.PublicKey) == k.signingFingerprint {
				found = true
			}
			if sub.PrivateKey != nil {
				t.Errorf("%s: the exported public key carries private material", name)
			}
		}
		if !found {
			t.Errorf("%s: the signing subkey is missing", name)
		}
	}
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	runEnv(t, nil, name, args...)
}

func runEnv(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", filepath.Base(name), strings.Join(args, " "), err, out)
	}
}
