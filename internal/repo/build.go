package repo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Guys-Inc-Public/archivist/internal/config"
	"github.com/Guys-Inc-Public/archivist/internal/deb"
)

// Options controls repository generation.
type Options struct {
	// Config is the validated configuration. Required.
	Config *config.Config

	// Out is the repository root. It may already hold a repository, in which
	// case the packages already there are kept: a release adds packages, it
	// does not replace the archive.
	Out string

	// Component receives the incoming packages. Defaults to the first
	// configured component.
	Component string

	// Now is the Release timestamp, injected so that a test can assert on the
	// bytes of a signed file.
	Now time.Time

	// Replace permits overwriting a package that is already published under the
	// same name, version and architecture with different content.
	Replace bool
}

// Result summarises what a Generate call did.
type Result struct {
	Added       int
	Replaced    int
	Unchanged   int
	Total       int
	SuiteDir    string   // dists/<codename>, relative to Out
	Indices     []string // index files written, relative to Out
	ReleasePath string   // the file a signature must cover, relative to Out

	// Fingerprint of the key that signed the tree, set by Sign.
	Fingerprint string
}

// A repository tree is world-readable by definition: every file in it is
// published to the internet, and a web server or apt client reading a local
// tree runs as somebody other than whoever built it. These are the modes a
// published archive has, not the modes a secret has, which is why the gosec
// annotations below say no rather than tightening them.
const (
	publishedFile = 0o644
	publishedDir  = 0o755
)

// ErrAlreadyPublished reports a package that exists in the repository under the
// same name, version and architecture but with different content.
var ErrAlreadyPublished = errors.New("already published with different content")

// Generate writes a complete repository tree.
//
// The tree is regenerated in full from metadata every time. There is no
// incremental index state to corrupt, and the sidecars beside each pool object
// mean regenerating costs a read of the metadata rather than a read of the
// pool. See docs/decisions/0001-regenerate-from-scratch.md.
func Generate(packages []*deb.Package, opts Options) (*Result, error) {
	cfg := opts.Config
	if cfg == nil {
		return nil, errors.New("no configuration")
	}
	if opts.Out == "" {
		return nil, errors.New("no output directory")
	}
	component := opts.Component
	if component == "" {
		component = cfg.Components[0]
	}
	if !contains(cfg.Components, component) {
		return nil, fmt.Errorf("component %q is not one of the configured components %v", component, cfg.Components)
	}

	existing, err := ScanSidecars(opts.Out)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*Entry{}
	for _, e := range existing {
		key, err := e.Key()
		if err != nil {
			return nil, err
		}
		byKey[key] = e
	}

	result := &Result{}
	type placement struct {
		source string
		entry  *Entry
	}
	var toPlace []placement
	seen := map[string]string{}

	for _, p := range packages {
		entry := NewEntry(p, component)
		key, err := entry.Key()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Path, err)
		}

		// Two files in one input set claiming to be the same package is a
		// mistake in the input, not something to resolve by picking one.
		if first, dup := seen[key]; dup {
			return nil, fmt.Errorf("%s and %s are both %s", first, p.Path, key)
		}
		seen[key] = p.Path

		switch prior, published := byKey[key]; {
		case !published:
			result.Added++
		case prior.SHA256 == p.SHA256 && prior.Component() == component:
			// Rebuilding the same input must be a no-op, or nobody can run
			// build twice without thinking about it.
			result.Unchanged++
		case !opts.Replace:
			if prior.Component() != component {
				return nil, fmt.Errorf("%s is %w in component %q, not %q",
					key, ErrAlreadyPublished, prior.Component(), component)
			}
			return nil, fmt.Errorf("%s is %w (published %s, incoming %s)",
				key, ErrAlreadyPublished, short(prior.SHA256), short(p.SHA256))
		default:
			result.Replaced++
		}

		byKey[key] = entry
		toPlace = append(toPlace, placement{source: p.Path, entry: entry})
	}

	all := make([]*Entry, 0, len(byKey))
	for _, e := range byKey {
		all = append(all, e)
	}
	if err := sortEntries(all); err != nil {
		return nil, err
	}
	if err := checkArchitectures(all, cfg.Architectures); err != nil {
		return nil, err
	}
	result.Total = len(all)

	for _, p := range toPlace {
		if err := place(opts.Out, p.source, p.entry); err != nil {
			return nil, err
		}
	}

	if err := writeIndices(opts.Out, cfg, all, opts.Now, result); err != nil {
		return nil, err
	}
	return result, nil
}

// place copies a package into the pool and writes its sidecar.
func place(root, source string, e *Entry) error {
	rel, err := e.PoolPath()
	if err != nil {
		return err
	}
	dest := filepath.Join(root, filepath.FromSlash(rel))
	// #nosec G301 -- a published pool directory is world-readable on purpose.
	if err := os.MkdirAll(filepath.Dir(dest), publishedDir); err != nil {
		return err
	}
	if err := copyFile(source, dest); err != nil {
		return err
	}
	return writeSidecar(dest, e)
}

// writeIndices regenerates dists/ from scratch.
func writeIndices(root string, cfg *config.Config, all []*Entry, now time.Time, result *Result) error {
	suiteDir := filepath.Join("dists", cfg.Codename)
	result.SuiteDir = filepath.ToSlash(suiteDir)

	// The suite directory is rebuilt rather than merged into. An index left
	// behind by a previous run with a different architecture list is not
	// referenced by the new Release, but it is still a file apt can fetch, and
	// an unreferenced index is exactly what a downgrade attack serves.
	absSuite := filepath.Join(root, suiteDir)
	if err := os.RemoveAll(absSuite); err != nil {
		return err
	}

	var files []indexFile
	for _, component := range cfg.Components {
		for _, arch := range cfg.Architectures {
			packages, err := packagesIndex(all, component, arch)
			if err != nil {
				return err
			}
			gz, err := gzipDeterministic(packages)
			if err != nil {
				return err
			}
			dir := filepath.Join(component, "binary-"+arch)
			files = append(files,
				indexFile{Path: filepath.ToSlash(filepath.Join(dir, "Packages")), Data: packages},
				indexFile{Path: filepath.ToSlash(filepath.Join(dir, "Packages.gz")), Data: gz},
				indexFile{Path: filepath.ToSlash(filepath.Join(dir, "Release")), Data: archRelease(cfg, component, arch)},
			)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, f := range files {
		dest := filepath.Join(absSuite, filepath.FromSlash(f.Path))
		// #nosec G301 -- a published index directory is world-readable on purpose.
		if err := os.MkdirAll(filepath.Dir(dest), publishedDir); err != nil {
			return err
		}
		// #nosec G306 -- apt must be able to read this.
		if err := os.WriteFile(dest, f.Data, publishedFile); err != nil {
			return err
		}
		result.Indices = append(result.Indices, filepath.ToSlash(filepath.Join(suiteDir, f.Path)))
	}

	releasePath := filepath.Join(suiteDir, "Release")
	// #nosec G306 -- Release is the file every client fetches first.
	if err := os.WriteFile(filepath.Join(root, releasePath), release(cfg, files, now), publishedFile); err != nil {
		return err
	}
	result.ReleasePath = filepath.ToSlash(releasePath)
	return nil
}

func copyFile(source, dest string) error {
	// #nosec G304 -- copying a package the caller pointed us at.
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	// #nosec G302,G304 -- the destination is a pool path this package computed
	// from control fields, and a published pool object is world-readable.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, publishedFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func short(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}
