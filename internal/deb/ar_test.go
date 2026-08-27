package deb

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// arArchive assembles an ar archive from raw member names and bodies, so tests
// can build the forms debtest deliberately cannot.
func arArchive(members ...[2]string) string {
	var b strings.Builder
	b.WriteString(arMagic)
	for _, m := range members {
		name, body := m[0], m[1]
		fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8o%-10d`\n", name, 0, 0, 0, 0o100644, len(body))
		b.WriteString(body)
		if len(body)%2 == 1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func TestARReaderWalksMembers(t *testing.T) {
	// The first body has an odd length, so the second member is only found if
	// the alignment byte is skipped.
	archive := arArchive(
		[2]string{"debian-binary", "2.0\n"},
		[2]string{"control.tar.gz", "odd"},
		[2]string{"data.tar.gz", "even"},
	)

	ar, err := newARReader(strings.NewReader(archive))
	if err != nil {
		t.Fatalf("newARReader() error = %v", err)
	}

	for _, want := range []struct {
		name string
		body string
	}{
		{"debian-binary", "2.0\n"},
		{"control.tar.gz", "odd"},
		{"data.tar.gz", "even"},
	} {
		member, err := ar.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if member.Name != want.name {
			t.Errorf("Next().Name = %q, want %q", member.Name, want.name)
		}
		if member.Size != int64(len(want.body)) {
			t.Errorf("Next().Size = %d, want %d", member.Size, len(want.body))
		}
		body, err := io.ReadAll(ar)
		if err != nil {
			t.Fatalf("reading member %q: %v", want.name, err)
		}
		if string(body) != want.body {
			t.Errorf("member %q body = %q, want %q", want.name, body, want.body)
		}
	}

	if _, err := ar.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next() at end = %v, want io.EOF", err)
	}
}

// TestARReaderSkipsUnreadMembers covers the path the reader actually takes: it
// never reads data.tar, so Next must be able to step over a member whose body
// was ignored.
func TestARReaderSkipsUnreadMembers(t *testing.T) {
	archive := arArchive(
		[2]string{"debian-binary", "2.0\n"},
		[2]string{"control.tar.gz", strings.Repeat("x", 4097)},
		[2]string{"data.tar.gz", "last"},
	)
	ar, err := newARReader(strings.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := ar.Next(); err != nil {
			t.Fatalf("Next() error = %v", err)
		}
	}
	member, err := ar.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if member.Name != "data.tar.gz" {
		t.Errorf("Next().Name = %q, want %q", member.Name, "data.tar.gz")
	}
}

// TestARReaderTrimsGNUNameSuffix covers the trailing slash GNU ar appends to
// member names. dpkg does not, but binutils-produced archives are the reason
// the convention exists and trimming it costs nothing.
func TestARReaderTrimsGNUNameSuffix(t *testing.T) {
	ar, err := newARReader(strings.NewReader(arArchive([2]string{"debian-binary/", "2.0\n"})))
	if err != nil {
		t.Fatal(err)
	}
	member, err := ar.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if member.Name != "debian-binary" {
		t.Errorf("Next().Name = %q, want %q", member.Name, "debian-binary")
	}
}

func TestARReaderRejects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		archive  string
		wantText string
	}{
		{
			name:     "wrong magic",
			archive:  "!<notarch>\nrest",
			wantText: "not an ar archive",
		},
		{
			name:     "empty",
			archive:  "",
			wantText: "reading archive header",
		},
		{
			name:     "BSD extended names",
			archive:  arArchive([2]string{"#1/13", "debian-binary2.0\n"}),
			wantText: "BSD extended member names",
		},
		{
			name:     "GNU name table",
			archive:  arArchive([2]string{"//", "debian-binary/\n"}),
			wantText: "GNU member name table",
		},
		{
			name:     "GNU name reference",
			archive:  arArchive([2]string{"/0", "body"}),
			wantText: "GNU member name table",
		},
		{
			name:     "unreadable size",
			archive:  arMagic + fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10s`\n", "debian-binary", 0, 0, 0, 0o100644, "not-a-num"),
			wantText: "unreadable size",
		},
		{
			name:     "missing entry magic",
			archive:  arMagic + fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d??", "debian-binary", 0, 0, 0, 0o100644, 4),
			wantText: "missing entry magic",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ar, err := newARReader(strings.NewReader(tc.archive))
			if err == nil {
				_, err = ar.Next()
			}
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantText)
			}
		})
	}
}

// TestARReaderTruncatedMember covers a header promising more than the archive
// holds: the shortfall must surface rather than yielding a short read that
// looks like a valid, smaller member.
func TestARReaderTruncatedMember(t *testing.T) {
	archive := arMagic + fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n", "debian-binary", 0, 0, 0, 0o100644, 64) + "2.0\n"

	ar, err := newARReader(strings.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ar.Next(); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if _, err := io.ReadAll(ar); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("reading truncated member = %v, want io.ErrUnexpectedEOF", err)
	}
}
