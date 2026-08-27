package deb

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"  // #nosec G501 -- Packages indices record MD5sum; the format requires it
	"crypto/sha1" // #nosec G505 -- Packages indices record SHA1; the format requires it
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Package is a .deb file read from disk: the identity its control stanza
// declares, and the size and checksums an archive index records for it.
type Package struct {
	// Path is where the file was read from. It is used for error messages and
	// for copying the file into a pool; nothing is ever inferred from it.
	Path string

	Control *Control

	Size   int64
	MD5    string
	SHA1   string
	SHA256 string
}

// Limits on what a control archive may expand to. A control stanza runs to a
// few kilobytes; anything approaching these numbers is malformed or hostile,
// and refusing to decompress it is cheaper than discovering why.
const (
	maxControlArchive = 32 << 20 // 32 MiB of decompressed control.tar
	maxControlFile    = 4 << 20  // 4 MiB for the control file itself
)

// Open reads a .deb file from disk.
func Open(name string) (*Package, error) {
	// #nosec G304 -- opening a caller-supplied package path is the point.
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return Read(name, f)
}

// Read parses a .deb from an arbitrary stream. The name is used for the
// resulting Path and for error messages; nothing is ever inferred from it.
//
// The stream is read once: the checksums cover every byte, including the parts
// after the control archive, so a package the size of an Electron application
// is not read twice to be described.
func Read(name string, r io.Reader) (*Package, error) {
	h := newHashes()
	tee := io.TeeReader(r, h)

	control, err := readControl(tee)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	// Drain whatever follows the control archive so the hashes describe the
	// whole file rather than its first half.
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return &Package{
		Path:    name,
		Control: control,
		Size:    h.n,
		MD5:     hex.EncodeToString(h.md5.Sum(nil)),
		SHA1:    hex.EncodeToString(h.sha1.Sum(nil)),
		SHA256:  hex.EncodeToString(h.sha256.Sum(nil)),
	}, nil
}

// readControl walks the ar members far enough to find and parse the control
// stanza, leaving the rest of the stream unread.
func readControl(r io.Reader) (*Control, error) {
	ar, err := newARReader(r)
	if err != nil {
		return nil, err
	}

	first, err := ar.Next()
	if err != nil {
		return nil, fmt.Errorf("reading first archive member: %w", err)
	}
	if first.Name != "debian-binary" {
		return nil, fmt.Errorf("not a Debian package: first member is %q, want %q", first.Name, "debian-binary")
	}
	format, err := io.ReadAll(io.LimitReader(ar, 64))
	if err != nil {
		return nil, fmt.Errorf("reading package format version: %w", err)
	}
	// Only the major version constrains the layout. dpkg has emitted 2.0 since
	// 1996; a 3.x would be free to rearrange the members this code walks.
	if v := strings.TrimSpace(string(format)); !strings.HasPrefix(v, "2.") {
		return nil, fmt.Errorf("unsupported package format version %q, want 2.x", v)
	}

	for {
		member, err := ar.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("not a Debian package: no control archive member")
		}
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(member.Name, "control.tar") {
			continue
		}

		decompressed, closeFn, err := decompress(member.Name, ar)
		if err != nil {
			return nil, err
		}
		defer func() { _ = closeFn() }()

		control, err := controlFromTar(io.LimitReader(decompressed, maxControlArchive))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", member.Name, err)
		}
		return control, nil
	}
}

// decompress wraps a control archive in the reader its extension calls for.
// The returned close function releases the decompressor and is always non-nil.
func decompress(name string, r io.Reader) (io.Reader, func() error, error) {
	noClose := func() error { return nil }

	switch ext := path.Ext(name); ext {
	case ".tar":
		return r, noClose, nil
	case ".gz":
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		return zr, zr.Close, nil
	case ".xz":
		zr, err := xz.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		return zr, noClose, nil
	case ".zst":
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		return zr, func() error { zr.Close(); return nil }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported control archive compression: %q", name)
	}
}

// controlFromTar finds the control file inside a decompressed control archive.
func controlFromTar(r io.Reader) (*Control, error) {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("control archive contains no control file")
		}
		if err != nil {
			return nil, err
		}
		// dpkg writes "./control"; some builders write "control".
		if strings.TrimPrefix(path.Clean(hdr.Name), "./") != "control" {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("control is not a regular file (type %q)", hdr.Typeflag)
		}
		return ParseControl(io.LimitReader(tr, maxControlFile))
	}
}

// hashes computes every digest an archive index records, in one pass, and
// counts the bytes it saw.
type hashes struct {
	md5    hash.Hash
	sha1   hash.Hash
	sha256 hash.Hash
	n      int64
}

func newHashes() *hashes {
	return &hashes{md5: md5.New(), sha1: sha1.New(), sha256: sha256.New()} // #nosec G401 -- format-mandated digests
}

func (h *hashes) Write(p []byte) (int, error) {
	h.md5.Write(p)
	h.sha1.Write(p)
	h.sha256.Write(p)
	h.n += int64(len(p))
	return len(p), nil
}
