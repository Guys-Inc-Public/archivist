// Package debtest builds synthetic .deb files for tests.
//
// Fixtures are generated rather than committed. A real package is tens of
// megabytes of Electron, which makes a repository unpleasant to clone and
// exercises nothing a small synthetic package does not: every property this
// project cares about lives in the control stanza and the archive framing.
//
// Building the fixtures here also means the reader is tested against all four
// control-archive encodings on every run, including the ones whose toolchain
// is not installed on the machine running the tests.
package debtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Compression selects the encoding of the control and data archives.
type Compression string

// The encodings dpkg-deb produces, and the ones the reader must therefore
// handle. See decision 0013.
const (
	None Compression = "none"
	Gzip Compression = "gzip"
	Xz   Compression = "xz"
	Zstd Compression = "zstd"
)

// extension returns the suffix dpkg gives an archive member under this
// compression.
func (c Compression) extension() (string, error) {
	switch c {
	case None:
		return "", nil
	case Gzip:
		return ".gz", nil
	case Xz:
		return ".xz", nil
	case Zstd:
		return ".zst", nil
	default:
		return "", fmt.Errorf("unknown compression %q", c)
	}
}

// Options describes a package to build.
type Options struct {
	// Control is the control stanza, verbatim. Tests that need a malformed
	// stanza pass one.
	Control string

	// Compression selects the control and data archive encoding. The zero
	// value means Gzip.
	Compression Compression

	// Data holds the files placed in data.tar, keyed by path. A package with
	// no payload is still a valid package, so this may be empty.
	Data map[string]string

	// FormatVersion overrides the debian-binary member. The zero value means
	// "2.0\n".
	FormatVersion string

	// OmitControlFile builds a control archive with no control file in it,
	// for the error path.
	OmitControlFile bool

	// ControlPath overrides the path of the control file inside the control
	// archive. The zero value is dpkg's "./control"; some builders write a
	// bare "control", and the reader accepts both.
	ControlPath string

	// ControlMemberName overrides the name of the control archive member. The
	// zero value derives it from Compression, which is what dpkg does; tests
	// covering unreadable packages set it to something the reader rejects.
	ControlMemberName string
}

// Control builds a well-formed control stanza. Callers override individual
// fields by writing their own; this exists so the common case is one line.
func Control(pkg, version, arch string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Package: %s\n", pkg)
	fmt.Fprintf(&b, "Version: %s\n", version)
	fmt.Fprintf(&b, "Architecture: %s\n", arch)
	b.WriteString("Maintainer: Guys Inc Team <CJ@guysinc.org>\n")
	b.WriteString("Priority: optional\n")
	b.WriteString("Section: devel\n")
	for _, line := range extra {
		b.WriteString(strings.TrimRight(line, "\n") + "\n")
	}
	b.WriteString("Description: A synthetic package\n")
	b.WriteString(" Built by internal/debtest. Not a real program.\n")
	return b.String()
}

// Build returns the bytes of a .deb file.
func Build(opts Options) ([]byte, error) {
	if opts.Compression == "" {
		opts.Compression = Gzip
	}
	ext, err := opts.Compression.extension()
	if err != nil {
		return nil, err
	}
	format := opts.FormatVersion
	if format == "" {
		format = "2.0\n"
	}

	controlPath := opts.ControlPath
	if controlPath == "" {
		controlPath = "./control"
	}
	controlFiles := map[string]string{}
	if !opts.OmitControlFile {
		controlFiles[controlPath] = opts.Control
	}
	controlName := opts.ControlMemberName
	if controlName == "" {
		controlName = "control.tar" + ext
	}
	controlTar, err := buildTar(controlFiles, opts.Compression)
	if err != nil {
		return nil, fmt.Errorf("building control archive: %w", err)
	}
	dataTar, err := buildTar(opts.Data, opts.Compression)
	if err != nil {
		return nil, fmt.Errorf("building data archive: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	for _, m := range []struct {
		name string
		body []byte
	}{
		{"debian-binary", []byte(format)},
		{controlName, controlTar},
		{"data.tar" + ext, dataTar},
	} {
		if err := writeARMember(&out, m.name, m.body); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// Write builds a package and writes it to dir under the given name. The name is
// the caller's choice precisely so tests can use one that contradicts the
// control stanza - see decision 0006.
func Write(dir, name string, opts Options) (string, error) {
	body, err := Build(opts)
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, body, 0o600); err != nil {
		return "", err
	}
	return full, nil
}

func writeARMember(w io.Writer, name string, body []byte) error {
	// mtime, uid and gid are fixed so that building the same fixture twice
	// produces identical bytes.
	header := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n", name, 0, 0, 0, 0o100644, len(body))
	if len(header) != 60 {
		return fmt.Errorf("member header for %q is %d bytes, want 60", name, len(header))
	}
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if len(body)%2 == 1 {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func buildTar(files map[string]string, c Compression) ([]byte, error) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	// Sorted, so a fixture is byte-identical between runs.
	for _, name := range sortedKeys(files) {
		body := files[name]
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(tw, body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return compress(raw.Bytes(), c)
}

func compress(body []byte, c Compression) ([]byte, error) {
	if c == None {
		return body, nil
	}

	var out bytes.Buffer
	var w io.WriteCloser
	var err error

	switch c {
	case Gzip:
		w = gzip.NewWriter(&out)
	case Xz:
		w, err = xz.NewWriter(&out)
	case Zstd:
		w, err = zstd.NewWriter(&out)
	default:
		return nil, fmt.Errorf("unknown compression %q", c)
	}
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(body); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
