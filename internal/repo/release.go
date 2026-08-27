package repo

import (
	"crypto/md5"  // #nosec G501 -- the Release format mandates an MD5Sum block.
	"crypto/sha1" // #nosec G505 -- the Release format mandates a SHA1 block.
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strings"
	"time"

	"github.com/Guys-Inc-Public/archivist/internal/config"
)

// dateFormat is the form apt expects in Date and Valid-Until: RFC 1123 with a
// literal UTC rather than a numeric offset, which is what Debian's own archives
// publish.
const dateFormat = "Mon, 02 Jan 2006 15:04:05 UTC"

// indexFile is one file listed in a Release checksum block.
type indexFile struct {
	// Path is relative to the suite directory, which is how apt resolves it.
	Path string
	Data []byte
}

// release renders the top-level Release file.
//
// Everything a client trusts hangs off this file: it is the only thing signed,
// and it is only worth signing because it carries a checksum of every index,
// each of which carries a checksum of every package.
func release(cfg *config.Config, files []indexFile, now time.Time) []byte {
	var b strings.Builder

	field := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s: %s\n", name, value)
		}
	}

	field("Origin", cfg.Origin)
	field("Label", cfg.Label)
	field("Suite", cfg.Suite)
	field("Codename", cfg.Codename)
	field("Date", now.UTC().Format(dateFormat))
	if cfg.ValidFor > 0 {
		field("Valid-Until", now.UTC().Add(cfg.ValidFor).Format(dateFormat))
	}
	field("Architectures", strings.Join(cfg.Architectures, " "))
	field("Components", strings.Join(cfg.Components, " "))
	field("Description", cfg.Description)

	// By-hash index paths are not published, and saying so explicitly stops a
	// client from trying them and taking a 404 on every update.
	field("Acquire-By-Hash", "no")

	sorted := make([]indexFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	// All three blocks are written. MD5Sum and SHA1 are weak and are here
	// because the format specifies them and old clients read them; SHA256 is
	// the one that carries the security claim.
	for _, block := range []struct {
		name string
		new  func() hash.Hash
	}{
		{"MD5Sum", func() hash.Hash { return md5.New() }}, // #nosec G401
		{"SHA1", func() hash.Hash { return sha1.New() }},  // #nosec G401
		{"SHA256", func() hash.Hash { return sha256.New() }},
	} {
		fmt.Fprintf(&b, "%s:\n", block.name)
		for _, f := range sorted {
			h := block.new()
			h.Write(f.Data)
			// Debian pads the size column; apt parses whitespace-separated
			// fields, so a single space is valid and stays stable as sizes grow.
			fmt.Fprintf(&b, " %s %d %s\n", hex.EncodeToString(h.Sum(nil)), len(f.Data), f.Path)
		}
	}

	return []byte(b.String())
}

// archRelease renders the per-architecture marker file.
//
// apt does not require it, but it is what tells a tool inspecting one index
// directory which archive it came from, and its absence is the kind of thing
// mirror checkers complain about.
func archRelease(cfg *config.Config, component, arch string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "Archive: %s\n", cfg.Suite)
	if cfg.Codename != cfg.Suite {
		fmt.Fprintf(&b, "Codename: %s\n", cfg.Codename)
	}
	if cfg.Origin != "" {
		fmt.Fprintf(&b, "Origin: %s\n", cfg.Origin)
	}
	if cfg.Label != "" {
		fmt.Fprintf(&b, "Label: %s\n", cfg.Label)
	}
	fmt.Fprintf(&b, "Component: %s\n", component)
	fmt.Fprintf(&b, "Architecture: %s\n", arch)
	return []byte(b.String())
}
