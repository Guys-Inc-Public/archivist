package deb_test

import (
	"crypto/md5"  // #nosec G501 -- verifying the digest the format mandates
	"crypto/sha1" // #nosec G505 -- verifying the digest the format mandates
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Guys-Inc-Public/archivist/internal/deb"
	"github.com/Guys-Inc-Public/archivist/internal/debtest"
)

// writeFixture builds a package in a temporary directory and returns its path.
func writeFixture(t *testing.T, name string, opts debtest.Options) string {
	t.Helper()
	path, err := debtest.Write(t.TempDir(), name, opts)
	if err != nil {
		t.Fatalf("building fixture: %v", err)
	}
	return path
}

// TestOpenEveryCompression is the reason this package took a dependency on two
// compressors: Debian's dpkg-deb emits control.tar.xz, Ubuntu's emits
// control.tar.zst, and older packages carry control.tar.gz. All three are
// packages a user will hand us. See decision 0013.
func TestOpenEveryCompression(t *testing.T) {
	for _, c := range []debtest.Compression{debtest.None, debtest.Gzip, debtest.Xz, debtest.Zstd} {
		t.Run(string(c), func(t *testing.T) {
			path := writeFixture(t, "fixture.deb", debtest.Options{
				Control:     debtest.Control("archivist-fixture", "1.2.3", "amd64"),
				Compression: c,
				Data:        map[string]string{"./usr/bin/fixture": "#!/bin/sh\nexit 0\n"},
			})

			pkg, err := deb.Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if got, want := pkg.Control.Package(), "archivist-fixture"; got != want {
				t.Errorf("Package() = %q, want %q", got, want)
			}
			if got, want := pkg.Control.Version(), "1.2.3"; got != want {
				t.Errorf("Version() = %q, want %q", got, want)
			}
			if got, want := pkg.Control.Architecture(), "amd64"; got != want {
				t.Errorf("Architecture() = %q, want %q", got, want)
			}
		})
	}
}

// TestOpenHashesWholeFile guards the one-pass read: the control archive is the
// second of three members, so a reader that stops once it has the stanza would
// hash only the beginning of the file and produce an index that fails apt's
// verification at install time.
func TestOpenHashesWholeFile(t *testing.T) {
	path := writeFixture(t, "fixture.deb", debtest.Options{
		Control: debtest.Control("archivist-fixture", "1.2.3", "amd64"),
		// Large enough that a truncated read cannot coincidentally match.
		Data: map[string]string{"./usr/share/fixture/payload": strings.Repeat("payload\n", 4096)},
	})

	body, err := os.ReadFile(path) // #nosec G304 -- a path this test just wrote
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := deb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if got, want := pkg.Size, int64(len(body)); got != want {
		t.Errorf("Size = %d, want %d", got, want)
	}
	md5Sum := md5.Sum(body)   // #nosec G401 -- format-mandated digest
	sha1Sum := sha1.Sum(body) // #nosec G401 -- format-mandated digest
	sha256Sum := sha256.Sum256(body)
	for _, tc := range []struct{ name, got, want string }{
		{"MD5", pkg.MD5, hex.EncodeToString(md5Sum[:])},
		{"SHA1", pkg.SHA1, hex.EncodeToString(sha1Sum[:])},
		{"SHA256", pkg.SHA256, hex.EncodeToString(sha256Sum[:])},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestOpenIgnoresFilename is decision 0006 as a test. The fixture is named the
// way GitHub Desktop's release tooling names its artifacts; everything the
// repository publishes must come from the stanza instead.
func TestOpenIgnoresFilename(t *testing.T) {
	path := writeFixture(t, "GitHubDesktop-linux-arm64-3.4.9.deb", debtest.Options{
		Control: debtest.Control("github-desktop", "3.4.9", "amd64"),
	})

	pkg, err := deb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := pkg.Control.Architecture(), "amd64"; got != want {
		t.Errorf("Architecture() = %q, want %q - the filename must not be believed", got, want)
	}
	if got, want := pkg.Control.Filename(), "github-desktop_3.4.9_amd64.deb"; got != want {
		t.Errorf("Filename() = %q, want %q", got, want)
	}
	if got, want := pkg.Control.PoolPath("main"), "pool/main/g/github-desktop/github-desktop_3.4.9_amd64.deb"; got != want {
		t.Errorf("PoolPath() = %q, want %q", got, want)
	}
}

// TestOpenEpochInVersion covers the filename rule an epoch breaks: a colon is
// legal in a version and not fetchable in a URL.
func TestOpenEpochInVersion(t *testing.T) {
	path := writeFixture(t, "fixture.deb", debtest.Options{
		Control: debtest.Control("archivist-fixture", "2:1.2.3-1", "amd64"),
	})

	pkg, err := deb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := pkg.Control.Version(), "2:1.2.3-1"; got != want {
		t.Errorf("Version() = %q, want %q - the epoch belongs in the index", got, want)
	}
	if got, want := pkg.Control.Filename(), "archivist-fixture_1.2.3-1_amd64.deb"; got != want {
		t.Errorf("Filename() = %q, want %q - the epoch must not reach a path", got, want)
	}
}

// TestOpenSourceDiffersFromPackage covers the pool prefix, which is taken from
// the source package rather than the binary one.
func TestOpenSourceDiffersFromPackage(t *testing.T) {
	for _, tc := range []struct {
		name, control, wantPool string
	}{
		{
			name:     "source field",
			control:  debtest.Control("libsecret-1-0", "0.21.4-1", "amd64", "Source: libsecret"),
			wantPool: "pool/main/libs/libsecret/libsecret-1-0_0.21.4-1_amd64.deb",
		},
		{
			name:     "source with version",
			control:  debtest.Control("github-desktop", "3.4.9", "amd64", "Source: desktop (3.4.9)"),
			wantPool: "pool/main/d/desktop/github-desktop_3.4.9_amd64.deb",
		},
		{
			name:     "lib prefix from package name",
			control:  debtest.Control("libfixture", "1.0", "amd64"),
			wantPool: "pool/main/libf/libfixture/libfixture_1.0_amd64.deb",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFixture(t, "fixture.deb", debtest.Options{Control: tc.control})
			pkg, err := deb.Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if got := pkg.Control.PoolPath("main"); got != tc.wantPool {
				t.Errorf("PoolPath() = %q, want %q", got, tc.wantPool)
			}
		})
	}
}

// TestOpenArchitectureAll records that "all" reaches the reader untouched. The
// fan-out into every declared architecture's index belongs to repository
// generation, not here.
func TestOpenArchitectureAll(t *testing.T) {
	path := writeFixture(t, "fixture.deb", debtest.Options{
		Control: debtest.Control("archivist-docs", "1.0", "all"),
	})
	pkg, err := deb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := pkg.Control.Architecture(), "all"; got != want {
		t.Errorf("Architecture() = %q, want %q", got, want)
	}
	if got, want := pkg.Control.Filename(), "archivist-docs_1.0_all.deb"; got != want {
		t.Errorf("Filename() = %q, want %q", got, want)
	}
}

// TestOpenControlWithoutLeadingDot covers builders that write "control" rather
// than dpkg's "./control".
func TestOpenControlWithoutLeadingDot(t *testing.T) {
	path := writeFixture(t, "fixture.deb", debtest.Options{
		Control:     debtest.Control("fixture", "1.0", "amd64"),
		ControlPath: "control",
	})

	pkg, err := deb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := pkg.Control.Package(), "fixture"; got != want {
		t.Errorf("Package() = %q, want %q", got, want)
	}
}

// TestOpenRejects covers every way a file can fail to be a package we can
// describe. Decision 0006 forbids guessing from the name, so each of these must
// say what is actually wrong.
func TestOpenRejects(t *testing.T) {
	valid := debtest.Options{Control: debtest.Control("fixture", "1.0", "amd64")}

	for _, tc := range []struct {
		name     string
		build    func(t *testing.T) string
		wantText string
	}{
		{
			name: "not an archive",
			build: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "notes.deb")
				if err := os.WriteFile(path, []byte("this is not a package\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantText: "not an ar archive",
		},
		{
			name: "empty file",
			build: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "empty.deb")
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantText: "archive header",
		},
		{
			name: "wrong format version",
			build: func(t *testing.T) string {
				opts := valid
				opts.FormatVersion = "3.0\n"
				return writeFixture(t, "future.deb", opts)
			},
			wantText: "unsupported package format version",
		},
		{
			name: "unsupported compression",
			build: func(t *testing.T) string {
				opts := valid
				opts.ControlMemberName = "control.tar.bz2"
				return writeFixture(t, "bzipped.deb", opts)
			},
			wantText: "unsupported control archive compression",
		},
		{
			name: "no control file",
			build: func(t *testing.T) string {
				opts := valid
				opts.OmitControlFile = true
				return writeFixture(t, "hollow.deb", opts)
			},
			wantText: "no control file",
		},
		{
			name: "control stanza missing required fields",
			build: func(t *testing.T) string {
				opts := valid
				opts.Control = "Package: fixture\n"
				return writeFixture(t, "incomplete.deb", opts)
			},
			wantText: "missing required field",
		},
		{
			name: "truncated mid-archive",
			build: func(t *testing.T) string {
				full := writeFixture(t, "truncated.deb", valid)
				body, err := os.ReadFile(full) // #nosec G304 -- a path this test just wrote
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, body[:len(body)/2], 0o600); err != nil {
					t.Fatal(err)
				}
				return full
			},
			wantText: "", // any error will do; the point is that it is not silent
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.build(t)
			pkg, err := deb.Open(path)
			if err == nil {
				t.Fatalf("Open() succeeded, want an error (got package %q)", pkg.Control.Package())
			}
			if tc.wantText != "" && !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("Open() error = %q, want it to mention %q", err, tc.wantText)
			}
			if !strings.Contains(err.Error(), filepath.Base(path)) {
				t.Errorf("Open() error = %q, want it to name the file", err)
			}
		})
	}
}

// TestOpenMissingFile keeps the not-found path distinguishable from the
// not-a-package one.
func TestOpenMissingFile(t *testing.T) {
	_, err := deb.Open(filepath.Join(t.TempDir(), "absent.deb"))
	if !os.IsNotExist(err) {
		t.Errorf("Open(absent) error = %v, want a not-exist error", err)
	}
}
