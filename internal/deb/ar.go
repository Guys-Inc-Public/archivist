package deb

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// A .deb is an ar archive holding exactly three members, in order:
// debian-binary, control.tar.*, and data.tar.*. Go has no ar reader in its
// standard library, and the full format is not worth implementing: the variants
// below exist for C toolchains producing object archives with long member
// names, which dpkg never emits. They are rejected by name rather than ignored,
// so a file using them fails with an explanation instead of a confusing miss.
const (
	arMagic      = "!<arch>\n"
	arHeaderSize = 60
	arEntryMagic = "`\n"
)

// arMember is the header of one archive member.
type arMember struct {
	Name string
	Size int64
}

// arReader walks an ar archive's members in order. The member returned by Next
// is read from the arReader itself, which reports io.EOF at that member's end.
type arReader struct {
	r    io.Reader
	data int64 // unread bytes of the current member
	pad  int64 // alignment bytes following it, skipped before the next header
}

func newARReader(r io.Reader) (*arReader, error) {
	var magic [len(arMagic)]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("reading archive header: %w", err)
	}
	if string(magic[:]) != arMagic {
		return nil, errors.New("not an ar archive")
	}
	return &arReader{r: r}, nil
}

// Next advances to the following member, discarding any unread bytes of the
// current one. It returns io.EOF when the archive is exhausted.
func (a *arReader) Next() (*arMember, error) {
	if skip := a.data + a.pad; skip > 0 {
		if _, err := io.CopyN(io.Discard, a.r, skip); err != nil {
			return nil, fmt.Errorf("seeking to next archive member: %w", err)
		}
		a.data, a.pad = 0, 0
	}

	var hdr [arHeaderSize]byte
	if _, err := io.ReadFull(a.r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF // a clean end, on a member boundary
		}
		return nil, fmt.Errorf("reading member header: %w", err)
	}
	if string(hdr[58:60]) != arEntryMagic {
		return nil, errors.New("corrupt member header: missing entry magic")
	}

	name := strings.TrimRight(string(hdr[0:16]), " ")
	if err := checkMemberName(name); err != nil {
		return nil, err
	}
	name = strings.TrimSuffix(name, "/")

	size, err := strconv.ParseInt(strings.TrimSpace(string(hdr[48:58])), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("corrupt member header: unreadable size for %q", name)
	}
	if size < 0 {
		return nil, fmt.Errorf("corrupt member header: negative size for %q", name)
	}

	a.data = size
	a.pad = size % 2 // members are padded to an even offset
	return &arMember{Name: name, Size: size}, nil
}

// checkMemberName rejects the two extended-name encodings. Both store the real
// name outside the header, and neither is produced by dpkg.
func checkMemberName(name string) error {
	if strings.HasPrefix(name, "#1/") {
		return fmt.Errorf("archive uses BSD extended member names (%q), which dpkg does not produce", name)
	}
	if name == "//" || (len(name) > 1 && name[0] == '/' && isDigits(name[1:])) {
		return fmt.Errorf("archive uses a GNU member name table (%q), which dpkg does not produce", name)
	}
	if name == "" {
		return errors.New("corrupt member header: empty member name")
	}
	return nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Read reads the current member's data. It reports io.EOF at the member's end,
// never at the archive's.
func (a *arReader) Read(p []byte) (int, error) {
	if a.data <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > a.data {
		p = p[:a.data]
	}
	n, err := a.r.Read(p)
	a.data -= int64(n)
	if errors.Is(err, io.EOF) && a.data > 0 {
		// The header promised more than the archive holds.
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
