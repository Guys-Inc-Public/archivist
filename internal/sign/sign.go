// Package sign produces the detached and inline OpenPGP signatures that apt
// requires: Release.gpg alongside Release, and InRelease containing both.
//
// Signing is isolated behind an interface so that the default implementation -
// a signing subkey held in a CI secret - can be replaced by an external signer
// without touching the repository generation code. See
// docs/decisions/0002-signing-key-handling.md.
package sign

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Signer produces the signatures over a Release file.
type Signer interface {
	// SignBinary returns an armoured detached signature over exactly these
	// bytes, for Release.gpg.
	SignBinary(data []byte) ([]byte, error)

	// SignText returns an armoured detached signature made in text mode, for
	// the cleartext framework used by InRelease.
	SignText(data []byte) ([]byte, error)

	// Fingerprint identifies the key that signs.
	Fingerprint() string

	// PublicKey returns the public key armoured and in binary form.
	PublicKey() (armoured, binary []byte, err error)
}

var _ Signer = (*Key)(nil)

// SignBinary signs the exact bytes of a file. Release.gpg is a signature over
// the file as it sits on disk, so nothing is canonicalised here.
func (k *Key) SignBinary(data []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&out, k.signing, bytes.NewReader(data), k.config); err != nil {
		return nil, fmt.Errorf("signing with %s: %w", k.fp, err)
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// SignText signs in text mode, sigclass 0x01. The library converts line endings
// to CRLF for hashing; the rest of the canonicalisation is the caller's job,
// because the library does not do it and a signature over the wrong bytes
// verifies as BAD - a failure that reads like a key problem and is not one.
func (k *Key) SignText(data []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := openpgp.ArmoredDetachSignText(&out, k.signing, bytes.NewReader(data), k.config); err != nil {
		return nil, fmt.Errorf("signing with %s: %w", k.fp, err)
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// Detached returns the content of Release.gpg.
func Detached(s Signer, release []byte) ([]byte, error) {
	return s.SignBinary(release)
}

// Inline returns the content of InRelease: the Release file and its signature
// in one document, which is what modern apt fetches.
//
// This is assembled here rather than with clearsign.Encode, which hardcodes
// armour carrying no CRC24 checksum. GnuPG 2.2 rejects such a block - it prints
// "Good signature" and then exits 2 with "no valid OpenPGP data found". The
// signature is fine; the armour is not, and there is no knob for it. See
// docs/decisions/0009-openpgp-library-not-gpg.md.
func Inline(s Signer, release []byte) ([]byte, error) {
	signed := canonicalText(release)

	signature, err := s.SignText([]byte(signed))
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteString("-----BEGIN PGP SIGNED MESSAGE-----\n")
	// The declared hash has to match the one the signature was made with, or a
	// verifier hashes the document the wrong way and reports BAD signature.
	out.WriteString("Hash: SHA256\n\n")
	out.WriteString(dashEscape(signed))
	// The line terminator before the armour belongs to the armour, not to the
	// signed text: RFC 4880 section 7.1 excludes it from what was hashed.
	out.WriteString("\n")
	out.Write(signature)
	return out.Bytes(), nil
}

// canonicalText applies RFC 4880 section 7.1: trailing whitespace is removed
// from every line, and the final line terminator is not part of the signed
// text. The bytes on disk and the bytes that were signed are therefore not the
// same bytes, which is the detail that makes hand-assembly necessary.
func canonicalText(data []byte) string {
	body := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	// Split leaves an empty final element for a body ending in a newline. That
	// element is the excluded terminator.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// dashEscape protects lines that would otherwise be mistaken for the armour
// header. A Release file has no such lines today, which is exactly why leaving
// it out would go unnoticed until a description began with a dash.
func dashEscape(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "-") {
			lines[i] = "- " + line
		}
	}
	return strings.Join(lines, "\n")
}
