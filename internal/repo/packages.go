package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sort"
	"strings"
)

// The order fields appear in a Packages stanza. apt does not care, but a
// deterministic order is what makes two builds of the same input produce
// identical bytes, and matching dpkg's conventional order makes the output
// readable next to a real Debian index. Anything not listed here follows,
// sorted, before the archive fields.
var packagesFieldOrder = []string{
	"Package", "Source", "Version", "Architecture", "Essential",
	"Priority", "Section", "Origin", "Maintainer", "Original-Maintainer",
	"Bugs", "Installed-Size", "Provides", "Pre-Depends", "Depends",
	"Recommends", "Suggests", "Conflicts", "Breaks", "Replaces", "Enhances",
	"Built-Using", "Multi-Arch", "Homepage", "Description", "Description-md5",
	"Tag",
}

// Fields the repository computes and a package must never supply. A .deb whose
// control stanza carries a Filename would otherwise choose where apt believes
// it lives, which is a path traversal with a signature over it.
var archiveFields = map[string]bool{
	"Filename": true, "Size": true,
	"MD5sum": true, "SHA1": true, "SHA256": true, "SHA512": true,
}

// stanza renders one entry as it appears in a Packages index.
func stanza(e *Entry, component string) (string, error) {
	c, err := e.control()
	if err != nil {
		return "", err
	}
	poolPath, err := e.PoolPath(component)
	if err != nil {
		return "", err
	}

	written := map[string]bool{}
	var b strings.Builder

	write := func(name string) {
		if written[name] || archiveFields[name] {
			return
		}
		value := c.Get(name)
		if value == "" {
			return
		}
		written[name] = true
		fmt.Fprintf(&b, "%s: %s\n", name, value)
	}

	for _, name := range packagesFieldOrder {
		write(name)
	}
	// Whatever the package declared that we have no opinion about. Dropping it
	// would silently discard fields apt or a user's tooling may rely on.
	for _, name := range c.Fields() {
		write(name)
	}

	fmt.Fprintf(&b, "Filename: %s\n", poolPath)
	fmt.Fprintf(&b, "Size: %d\n", e.Size)
	fmt.Fprintf(&b, "MD5sum: %s\n", e.MD5)
	fmt.Fprintf(&b, "SHA1: %s\n", e.SHA1)
	fmt.Fprintf(&b, "SHA256: %s\n", e.SHA256)

	return b.String(), nil
}

// packagesIndex renders the Packages file for one component and architecture.
//
// An entry declaring Architecture: all is written into every architecture's
// index, because apt fetches only binary-<its own arch>/Packages and would
// never see a package listed once. The pool object is stored once; only the
// index entry repeats.
func packagesIndex(entries []*Entry, component, arch string) ([]byte, error) {
	selected, err := selectForArch(entries, arch)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	for i, e := range selected {
		if i > 0 {
			b.WriteByte('\n')
		}
		s, err := stanza(e, component)
		if err != nil {
			return nil, err
		}
		b.WriteString(s)
	}
	return b.Bytes(), nil
}

func selectForArch(entries []*Entry, arch string) ([]*Entry, error) {
	var out []*Entry
	for _, e := range entries {
		c, err := e.control()
		if err != nil {
			return nil, err
		}
		if c.Architecture() == arch || c.Architecture() == "all" {
			out = append(out, e)
		}
	}
	if err := sortEntries(out); err != nil {
		return nil, err
	}
	return out, nil
}

// sortEntries orders an index deterministically: by package name, then version,
// then architecture. Input order is whatever a directory listing happened to
// return, which is not a property to publish.
func sortEntries(entries []*Entry) error {
	keys := make(map[*Entry]string, len(entries))
	for _, e := range entries {
		k, err := e.Key()
		if err != nil {
			return err
		}
		keys[e] = k
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return keys[entries[i]] < keys[entries[j]]
	})
	return nil
}

// gzipDeterministic compresses data reproducibly. The gzip header carries a
// modification time and a name; left at their defaults they would make two
// builds of identical input differ, which defeats the point of being able to
// diff a regenerated repository against the one it replaces.
func gzipDeterministic(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	w.Header = gzip.Header{OS: 255} // 255: unknown, and no name or timestamp
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// checkArchitectures rejects a package whose architecture the repository does
// not advertise. A silent omission would produce a repository that builds
// cleanly and is missing a package, which is the worst of both outcomes.
func checkArchitectures(entries []*Entry, declared []string) error {
	offered := map[string]bool{"all": true}
	for _, a := range declared {
		offered[a] = true
	}
	var bad []string
	for _, e := range entries {
		c, err := e.control()
		if err != nil {
			return err
		}
		if !offered[c.Architecture()] {
			bad = append(bad, fmt.Sprintf("%s declares %s", c.Filename(), c.Architecture()))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("package architecture is not in the configured architectures %v:\n  %s",
			declared, strings.Join(bad, "\n  "))
	}
	return nil
}
