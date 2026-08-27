package repo

import (
	"os"
	"path/filepath"

	"github.com/Guys-Inc-Public/archivist/internal/sign"
)

// Names of the signature and key files, relative to the repository root. They
// are constants because a user's sources.list line names public.asc and a
// rename would break every installation that already exists.
const (
	ReleaseGPGName = "Release.gpg"
	InReleaseName  = "InRelease"
	PublicASCName  = "public.asc"
	PublicGPGName  = "public.gpg"
)

// Sign writes the signatures and the public key that make a generated tree a
// repository apt will accept.
//
// Order matters. InRelease and Release.gpg are written last because they are
// the files a client fetches first: until they exist, a partially written tree
// is simply a tree apt refuses, rather than one it half trusts.
func Sign(root string, result *Result, signer sign.Signer) error {
	releasePath := filepath.Join(root, filepath.FromSlash(result.ReleasePath))
	// #nosec G304 -- the path was computed by this package.
	release, err := os.ReadFile(releasePath)
	if err != nil {
		return err
	}

	armoured, binaryKey, err := signer.PublicKey()
	if err != nil {
		return err
	}
	// The key goes down before anything that can be checked against it, so a
	// tree is never briefly signed by a key it does not carry.
	for _, f := range []struct {
		name string
		data []byte
	}{
		{PublicASCName, armoured},
		{PublicGPGName, binaryKey},
	} {
		// #nosec G306 -- users fetch this key over HTTP.
		if err := os.WriteFile(filepath.Join(root, f.name), f.data, publishedFile); err != nil {
			return err
		}
	}

	detached, err := sign.Detached(signer, release)
	if err != nil {
		return err
	}
	inline, err := sign.Inline(signer, release)
	if err != nil {
		return err
	}

	suite := filepath.Join(root, filepath.FromSlash(result.SuiteDir))
	for _, f := range []struct {
		name string
		data []byte
	}{
		{ReleaseGPGName, detached},
		{InReleaseName, inline},
	} {
		// #nosec G306 -- apt fetches this.
		if err := os.WriteFile(filepath.Join(suite, f.name), f.data, publishedFile); err != nil {
			return err
		}
		result.Indices = append(result.Indices, filepath.ToSlash(filepath.Join(result.SuiteDir, f.name)))
	}

	result.Fingerprint = signer.Fingerprint()
	return nil
}
