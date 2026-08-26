package deb

import (
	"strings"
	"testing"
)

const sampleStanza = `Package: github-desktop
Version: 3.4.9
License: MIT
Vendor: Guys Inc
Architecture: amd64
Maintainer: Guys Inc Team <CJ@guysinc.org>
Installed-Size: 296608
Depends: libcurl3 | libcurl4, libsecret-1-0, gnome-keyring
Section: GNOME;GTK;Development
Priority: extra
Homepage: https://github.com/Guys-Inc-Public/github-desktop-linux
Description: Simple collaboration from your desktop
 This is the unofficial port of GitHub Desktop for Linux
 .
 It is not affiliated with GitHub, Inc.
`

func mustParse(t *testing.T, s string) *Control {
	t.Helper()
	c, err := ParseControl(strings.NewReader(s))
	if err != nil {
		t.Fatalf("ParseControl() error = %v", err)
	}
	return c
}

func TestParseControlFields(t *testing.T) {
	c := mustParse(t, sampleStanza)

	for _, tc := range []struct{ field, want string }{
		{"Package", "github-desktop"},
		{"Version", "3.4.9"},
		{"Architecture", "amd64"},
		{"Installed-Size", "296608"},
		// Field names are case-insensitive.
		{"pAcKaGe", "github-desktop"},
		{"installed-size", "296608"},
	} {
		if got := c.Get(tc.field); got != tc.want {
			t.Errorf("Get(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}

	if got := c.Get("Nonexistent"); got != "" {
		t.Errorf("Get(absent) = %q, want empty", got)
	}
}

func TestParseControlContinuationLines(t *testing.T) {
	c := mustParse(t, sampleStanza)

	desc := c.Get("Description")
	if !strings.HasPrefix(desc, "Simple collaboration from your desktop") {
		t.Errorf("Description lost its synopsis: %q", desc)
	}
	if !strings.Contains(desc, "\n .") {
		t.Error("Description lost the blank-line marker that Debian encodes as ' .'")
	}
	if !strings.Contains(desc, "not affiliated") {
		t.Error("Description lost a continuation line")
	}
}

func TestParseControlStopsAtBlankLine(t *testing.T) {
	two := sampleStanza + "\nPackage: something-else\nVersion: 1.0\nArchitecture: all\n"
	c := mustParse(t, two)
	if got := c.Package(); got != "github-desktop" {
		t.Errorf("parser ran past the stanza boundary: Package = %q", got)
	}
}

func TestParseControlErrors(t *testing.T) {
	tests := map[string]string{
		"missing required field": "Package: foo\nVersion: 1.0\n",
		"no stanza":              "\n\n",
		"malformed line":         "Package: foo\nthis is not a field\n",
		"leading continuation":   " orphaned continuation\nPackage: foo\n",
		"duplicate field":        "Package: foo\nPackage: bar\nVersion: 1\nArchitecture: all\n",
		"empty field name":       "Package: foo\n: value\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseControl(strings.NewReader(input)); err == nil {
				t.Fatal("ParseControl() succeeded, want error")
			}
		})
	}
}

func TestSourceDefaultsToPackage(t *testing.T) {
	c := mustParse(t, sampleStanza)
	if got := c.Source(); got != "github-desktop" {
		t.Errorf("Source() = %q, want the package name when Source is absent", got)
	}

	withSource := "Package: libfoo-bin\nSource: libfoo (1.2-3)\nVersion: 1.2-3\nArchitecture: amd64\n"
	c = mustParse(t, withSource)
	if got := c.Source(); got != "libfoo" {
		t.Errorf("Source() = %q, want %q - the parenthesised version must be stripped", got, "libfoo")
	}
}

func TestPoolPath(t *testing.T) {
	tests := []struct {
		name   string
		stanza string
		want   string
	}{
		{
			name:   "ordinary package uses a single-letter prefix",
			stanza: sampleStanza,
			want:   "pool/main/g/github-desktop/github-desktop_3.4.9_amd64.deb",
		},
		{
			name:   "lib packages use a four-character prefix",
			stanza: "Package: libsecret-1-0\nVersion: 0.20.5-3\nArchitecture: arm64\n",
			want:   "pool/main/libs/libsecret-1-0/libsecret-1-0_0.20.5-3_arm64.deb",
		},
		{
			name:   "prefix follows the source package, not the binary",
			stanza: "Package: libfoo-bin\nSource: zlib\nVersion: 1.0\nArchitecture: all\n",
			want:   "pool/main/z/zlib/libfoo-bin_1.0_all.deb",
		},
		{
			name:   "a short lib-prefixed name is not truncated",
			stanza: "Package: lib\nVersion: 1.0\nArchitecture: all\n",
			want:   "pool/main/l/lib/lib_1.0_all.deb",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := mustParse(t, tc.stanza)
			if got := c.PoolPath("main"); got != tc.want {
				t.Errorf("PoolPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

// An epoch is legal in a version but must never reach a filename - apt cannot
// fetch a path containing a colon.
func TestFilenameStripsEpoch(t *testing.T) {
	c := mustParse(t, "Package: foo\nVersion: 2:1.4.1-1\nArchitecture: armhf\n")
	if got := c.Filename(); got != "foo_1.4.1-1_armhf.deb" {
		t.Errorf("Filename() = %q, want the epoch stripped", got)
	}
}

// The inventory's first hidden coupling: three architecture vocabularies exist
// across a typical build, and the only trustworthy source is the control stanza.
func TestArchitectureComesFromControlNotFilename(t *testing.T) {
	c := mustParse(t, "Package: foo\nVersion: 1.0\nArchitecture: armhf\n")
	if got := c.Architecture(); got != "armhf" {
		t.Errorf("Architecture() = %q, want %q", got, "armhf")
	}
	if got := c.Filename(); !strings.HasSuffix(got, "_armhf.deb") {
		t.Errorf("Filename() = %q, want it derived from the control stanza", got)
	}
}
