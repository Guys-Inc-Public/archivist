// Package deb parses Debian binary package metadata.
//
// The single rule this package exists to enforce: a package's identity comes
// from its control stanza, never from its filename. Release artifacts get
// renamed by build tooling in ways that encode a project's own conventions, and
// a repository that trusts those names inherits every one of them.
package deb

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Control is a parsed Debian control stanza: a set of case-insensitive fields
// in the order they appeared.
type Control struct {
	fields map[string]string
	order  []string
}

// Required fields, per Debian Policy §5.3, for a binary package control file.
var requiredFields = []string{"Package", "Version", "Architecture"}

// ParseControl reads a single control stanza.
//
// Continuation lines (those beginning with a space or tab) are folded into the
// preceding field, preserving the leading whitespace that Debian's own format
// uses to mark blank lines inside a long description. Parsing stops at the
// first empty line, so a Packages index can be read one stanza at a time.
func ParseControl(r io.Reader) (*Control, error) {
	c := &Control{fields: map[string]string{}}

	sc := bufio.NewScanner(r)
	// Descriptions and Checksums fields comfortably exceed the 64KiB default.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var current string
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")

		if strings.TrimSpace(line) == "" {
			if len(c.order) == 0 {
				continue // leading blank lines before the stanza
			}
			break // end of stanza
		}

		if line[0] == '#' {
			continue
		}

		if line[0] == ' ' || line[0] == '\t' {
			if current == "" {
				return nil, fmt.Errorf("continuation line with no preceding field: %q", line)
			}
			c.fields[current] += "\n" + line
			continue
		}

		name, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("malformed line, expected %q: %q", "Field: value", line)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("empty field name: %q", line)
		}

		key := canonical(name)
		if _, dup := c.fields[key]; dup {
			return nil, fmt.Errorf("duplicate field %q", name)
		}
		c.fields[key] = strings.TrimSpace(value)
		c.order = append(c.order, key)
		current = key
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading control data: %w", err)
	}
	if len(c.order) == 0 {
		return nil, fmt.Errorf("no control stanza found")
	}

	var missing []string
	for _, f := range requiredFields {
		if c.Get(f) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("control stanza is missing required field(s): %s", strings.Join(missing, ", "))
	}
	return c, nil
}

// canonical normalises a field name for lookup. Debian field names are
// case-insensitive; their canonical form capitalises each hyphen-separated word.
func canonical(name string) string {
	parts := strings.Split(strings.ToLower(name), "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

// Get returns a field value, or "" if the field is absent.
func (c *Control) Get(name string) string { return c.fields[canonical(name)] }

// Fields returns the field names present, sorted.
func (c *Control) Fields() []string {
	out := make([]string, 0, len(c.order))
	out = append(out, c.order...)
	sort.Strings(out)
	return out
}

func (c *Control) Package() string      { return c.Get("Package") }
func (c *Control) Version() string      { return c.Get("Version") }
func (c *Control) Architecture() string { return c.Get("Architecture") }

// Source returns the source package name, which defaults to the binary package
// name when no Source field is present. A Source field may carry a version in
// parentheses ("foo (1.2-3)"); only the name is returned.
func (c *Control) Source() string {
	src := c.Get("Source")
	if src == "" {
		return c.Package()
	}
	if name, _, found := strings.Cut(src, " "); found {
		return name
	}
	return src
}

// PoolPath returns the path a package occupies within the archive pool, using
// Debian's conventional layout:
//
//	pool/<component>/<prefix>/<source>/<package>_<version>_<arch>.deb
//
// The prefix is the source package's first letter, except for names beginning
// "lib", which use the first four characters so that the enormous number of
// library packages spread across directories instead of piling into "l".
func (c *Control) PoolPath(component string) string {
	src := c.Source()

	prefix := ""
	switch {
	case strings.HasPrefix(src, "lib") && len(src) > 3:
		prefix = src[:4]
	case src != "":
		prefix = src[:1]
	}

	return fmt.Sprintf("pool/%s/%s/%s/%s", component, prefix, src, c.Filename())
}

// Filename returns the canonical archive filename for the package. It is
// derived entirely from control fields; any name the artifact arrived with is
// deliberately discarded.
func (c *Control) Filename() string {
	// An epoch is legal in a version but must not appear in a filename.
	version := c.Version()
	if _, after, found := strings.Cut(version, ":"); found {
		version = after
	}
	return fmt.Sprintf("%s_%s_%s.deb", c.Package(), version, c.Architecture())
}
