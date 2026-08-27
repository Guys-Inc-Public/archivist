// Package config loads and validates archivist.yml.
//
// Two properties shape this package. The first is that a configuration file is
// wrong in more than one way at a time, so validation reports every problem it
// finds rather than the first - a tool that makes you fix a five-field file
// five times is a tool nobody enjoys.
//
// The second is that what a command requires depends on the command. "build"
// writes a repository tree to local disk and has no use for a bucket name;
// demanding one would mean inventing object-storage settings to exercise a
// path that never touches object storage.
package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// Need names the set of fields a command requires. The sets are cumulative:
// publishing regenerates the tree it uploads, so it needs everything building
// needs and the destination as well.
type Need int

const (
	// NeedBuild requires the fields that describe the repository and sign it.
	NeedBuild Need = iota
	// NeedPublish additionally requires the object-storage destination.
	NeedPublish
)

func (n Need) String() string {
	if n == NeedPublish {
		return "publish"
	}
	return "build"
}

// Config is a validated configuration. Every field has been checked; nothing
// here needs re-parsing by a caller.
type Config struct {
	Origin        string
	Label         string
	Description   string
	Suite         string
	Codename      string
	Components    []string
	Architectures []string

	// KeyID is the signing subkey fingerprint, normalised to 40 uppercase hex
	// characters. The key itself is never configured - see docs/Configuration.md.
	KeyID string

	// ValidFor is how long after its Date a Release stays valid. Zero means no
	// Valid-Until field is written, which is the default: see
	// docs/decisions/0011-valid-until-is-opt-in.md.
	ValidFor time.Duration

	Publish Publish
}

// Publish is the object-storage destination.
type Publish struct {
	Bucket    string
	Endpoint  string
	Region    string
	PublicURL string
	// Prefix is normalised to either "" or a value ending in "/", so callers
	// can concatenate without deciding whose job the separator is.
	Prefix string
}

// file is the YAML wire shape, kept separate from Config so that every field
// arrives as a plain scalar. That separation is what makes reporting all
// problems at once possible: a custom unmarshaler aborts decoding at the first
// failure, so no field may validate itself on the way in.
type file struct {
	Origin        string   `yaml:"origin"`
	Label         string   `yaml:"label"`
	Description   string   `yaml:"description"`
	Suite         string   `yaml:"suite"`
	Codename      string   `yaml:"codename"`
	Components    []string `yaml:"components"`
	Architectures []string `yaml:"architectures"`
	ValidFor      string   `yaml:"valid_for"`

	Signing signingFile `yaml:"signing"`
	Publish publishFile `yaml:"publish"`
}

type signingFile struct {
	KeyID string `yaml:"key_id"`
}

type publishFile struct {
	Bucket    string `yaml:"bucket"`
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	PublicURL string `yaml:"public_url"`
	Prefix    string `yaml:"prefix"`
}

// maxConfigSize bounds what Load will read. A configuration file is a few
// hundred bytes; anything approaching this is not one.
const maxConfigSize = 1 << 20 // 1 MiB

// Load reads and validates a configuration file.
func Load(name string, need Need) (*Config, error) {
	// #nosec G304 -- reading a caller-supplied config path is the point.
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Read(name, f, need)
}

// Read validates configuration from r. The name is used only in error
// messages, so a caller reading from somewhere other than a file can still
// produce a message that says where the problem is.
func Read(name string, r io.Reader, need Need) (*Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if int64(len(data)) > maxConfigSize {
		return nil, fmt.Errorf("%s: configuration file is larger than %d bytes", name, maxConfigSize)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("%s: file is empty", name)
	}

	var raw file
	// Strict decoding rejects unknown keys. A misspelled field that is silently
	// ignored is worse here than elsewhere: the repository still builds, and the
	// setting the author believed they had applied is simply absent.
	if err := yaml.UnmarshalWithOptions(data, &raw, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return validate(name, &raw, need)
}
