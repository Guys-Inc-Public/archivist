package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guys-Inc-Public/archivist/internal/deb"
)

const (
	// SidecarSuffix is appended to a pool object's full name, so that a bucket
	// listing sorts each sidecar immediately after the object it describes.
	SidecarSuffix = ".archivist.json"

	// SidecarSchema is the version written into every sidecar. See
	// docs/decisions/0012-metadata-sidecar-format.md.
	SidecarSchema = 1
)

// Entry is one package in the repository. It is exactly the content of a
// sidecar file: everything needed to write an index entry without reading the
// package back out of the pool.
type Entry struct {
	Schema  int               `json:"schema"`
	Control map[string]string `json:"control"`
	Size    int64             `json:"size"`
	MD5     string            `json:"md5"`
	SHA1    string            `json:"sha1"`
	SHA256  string            `json:"sha256"`

	// parsed is the Control map read back through the same parser that read the
	// package originally, so that a hand-edited sidecar is subject to the same
	// rules a real .deb is.
	parsed *deb.Control
}

// NewEntry records a package that has just been read.
func NewEntry(p *deb.Package) *Entry {
	fields := map[string]string{}
	for _, name := range p.Control.Fields() {
		fields[name] = p.Control.Get(name)
	}
	return &Entry{
		Schema:  SidecarSchema,
		Control: fields,
		Size:    p.Size,
		MD5:     p.MD5,
		SHA1:    p.SHA1,
		SHA256:  p.SHA256,
		parsed:  p.Control,
	}
}

// control returns the entry's stanza, parsing it on first use.
func (e *Entry) control() (*deb.Control, error) {
	if e.parsed != nil {
		return e.parsed, nil
	}
	// Rendering the map and re-parsing it costs a few hundred bytes and buys
	// the required-field checks for free: a sidecar missing Package is rejected
	// by the code that would have rejected the package.
	names := make([]string, 0, len(e.Control))
	for name := range e.Control {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s: %s\n", name, e.Control[name])
	}

	c, err := deb.ParseControl(strings.NewReader(b.String()))
	if err != nil {
		return nil, err
	}
	e.parsed = c
	return c, nil
}

// Validate checks that an entry read from disk is usable.
func (e *Entry) Validate() error {
	if e.Schema != SidecarSchema {
		return fmt.Errorf("sidecar schema %d, want %d", e.Schema, SidecarSchema)
	}
	if _, err := e.control(); err != nil {
		return err
	}
	for _, f := range []struct{ name, value string }{
		{"md5", e.MD5}, {"sha1", e.SHA1}, {"sha256", e.SHA256},
	} {
		if f.value == "" {
			return fmt.Errorf("sidecar is missing %s", f.name)
		}
	}
	if e.Size <= 0 {
		return fmt.Errorf("sidecar records a size of %d", e.Size)
	}
	return nil
}

// Key identifies a package uniquely within a repository. Two packages sharing a
// key are the same package, whatever else differs.
func (e *Entry) Key() (string, error) {
	c, err := e.control()
	if err != nil {
		return "", err
	}
	return c.Package() + "_" + c.Version() + "_" + c.Architecture(), nil
}

// PoolPath returns the entry's path relative to the repository root.
func (e *Entry) PoolPath(component string) (string, error) {
	c, err := e.control()
	if err != nil {
		return "", err
	}
	return c.PoolPath(component), nil
}

// writeSidecar writes the JSON metadata file next to a pool object.
func writeSidecar(objectPath string, e *Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	// Map keys are marshalled in sorted order, so the same entry always
	// produces the same bytes. That is what lets a rebuild be a no-op.
	// #nosec G306 -- the sidecar sits in the published tree beside its object.
	return os.WriteFile(objectPath+SidecarSuffix, append(data, '\n'), publishedFile)
}

// readSidecar reads and validates one sidecar file.
func readSidecar(name string) (*Entry, error) {
	// #nosec G304 -- walking a repository the caller asked us to read.
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var e Entry
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: trailing content after the JSON object", name)
	}
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &e, nil
}

// ScanSidecars reads every sidecar under a repository's pool directory.
//
// This is the merge path: regenerating an index reads sidecars rather than
// packages, which is what makes the cost O(metadata) instead of O(pool). See
// docs/decisions/0001-regenerate-from-scratch.md.
func ScanSidecars(root string) ([]*Entry, error) {
	pool := filepath.Join(root, "pool")
	if _, err := os.Stat(pool); errors.Is(err, fs.ErrNotExist) {
		return nil, nil // nothing published here yet
	}

	var entries []*Entry
	err := filepath.WalkDir(pool, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, SidecarSuffix) {
			return nil
		}
		// A sidecar without its object is a repository defect, not something to
		// quietly carry into a new index.
		object := strings.TrimSuffix(path, SidecarSuffix)
		if _, err := os.Stat(object); err != nil {
			return fmt.Errorf("%s describes %s, which is missing", path, filepath.Base(object))
		}
		e, err := readSidecar(path)
		if err != nil {
			return err
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
