package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Guys-Inc-Public/archivist/internal/config"
	"github.com/Guys-Inc-Public/archivist/internal/deb"
	"github.com/Guys-Inc-Public/archivist/internal/repo"
	"github.com/Guys-Inc-Public/archivist/internal/sign"
)

func build(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "archivist.yml", "path to the configuration file")
	out := fs.String("out", "./repo", "directory to write the repository into")
	force := fs.Bool("force", false, "republish a version whose content has changed")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "Usage: archivist build [flags] <packages-dir>\n\n"+
			"Reads every .deb under the given directory and writes a signed repository\n"+
			"tree. Packages are recognised by content, not by filename, and their\n"+
			"identity comes from their control stanza.\n\n"+
			"The signing key is read from "+sign.EnvKey+", and its passphrase, if it has\n"+
			"one, from "+sign.EnvPassphrase+". Neither has a flag: a key on a command line\n"+
			"lands in shell history and in the process table.\n\n")
		fs.PrintDefaults()
	}
	operands, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(operands) != 1 {
		fs.Usage()
		return errors.New("expected exactly one packages directory")
	}
	source := operands[0]

	cfg, err := config.Load(*configPath, config.NeedBuild)
	if err != nil {
		return err
	}

	packages, scanned, err := readPackages(source)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no .deb packages found in %s (examined %d files)", source, scanned)
	}

	// The key is loaded before anything is written. Discovering that the key is
	// wrong after generating a tree leaves an unsigned repository on disk,
	// which is the one thing worse than no repository.
	signer, err := sign.FromEnvironment(cfg.KeyID)
	if err != nil {
		return err
	}

	result, err := repo.Generate(packages, repo.Options{
		Config:  cfg,
		Out:     *out,
		Now:     time.Now(),
		Replace: *force,
	})
	if err != nil {
		if errors.Is(err, repo.ErrAlreadyPublished) {
			return fmt.Errorf("%w\nPass --force if the change is deliberate", err)
		}
		return err
	}
	if err := repo.Sign(*out, result, signer); err != nil {
		return err
	}

	return report(os.Stdout, cfg, result, source, *out, len(packages))
}

// readPackages finds the packages under a directory.
//
// Files are identified by their leading bytes rather than their extension. A
// release directory holds checksums, archives and signatures alongside the
// packages, and decision 0006's rule - that a package's identity is in its
// control stanza, never its name - applies just as much to deciding whether a
// file is a package at all.
func readPackages(root string) (packages []*deb.Package, scanned int, err error) {
	// #nosec G703,G122 -- the root is the directory the user named on the
	// command line, and this runs with the user's own privileges over their own
	// build output. There is no boundary here for a symlink to cross: anything
	// the walk can reach, the invoking shell could already read.
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		scanned++

		// #nosec G304 -- walking the directory the caller named.
		f, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer func() { _ = f.Close() }()

		magic := make([]byte, len(debMagic))
		if _, readErr := io.ReadFull(f, magic); readErr != nil {
			return nil // too short to be a package
		}
		if !bytes.Equal(magic, []byte(debMagic)) {
			return nil
		}
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return seekErr
		}

		p, readErr := deb.Read(path, f)
		if readErr != nil {
			return readErr
		}
		packages = append(packages, p)
		return nil
	})
	if err != nil {
		return nil, scanned, err
	}
	// Walk order is filesystem order. Sorting here keeps the "Read N packages"
	// listing stable between runs on different machines.
	sort.Slice(packages, func(i, j int) bool { return packages[i].Path < packages[j].Path })
	return packages, scanned, nil
}

// report prints what was built and the one command a user needs to check it.
func report(w io.Writer, cfg *config.Config, result *repo.Result, source, out string, read int) error {
	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Read %d packages from %s\n", read, source)
	fmt.Fprintf(&b, "Wrote %s (%s)\n\n", out, describe(result))

	fmt.Fprintf(&b, "  Repository  %s\n", cfg.Origin)
	fmt.Fprintf(&b, "  Suite       %s (%s)\n", cfg.Codename, strings.Join(cfg.Components, ", "))
	fmt.Fprintf(&b, "  Packages    %d across %s\n", result.Total, strings.Join(cfg.Architectures, ", "))
	fmt.Fprintf(&b, "  Signed by   %s\n", result.Fingerprint)
	if cfg.ValidFor > 0 {
		fmt.Fprintf(&b, "  Valid for   %s\n", cfg.ValidFor)
	}

	// A local repository is a real repository, and this is the whole reason the
	// tool has a CLI and not only an Action: the thing you are about to publish
	// can be installed from first.
	b.WriteString("\nTry it before publishing:\n\n")
	fmt.Fprintf(&b, "  echo \"deb [signed-by=%s/%s] file://%s %s %s\" \\\n",
		absOut, repo.PublicASCName, absOut, cfg.Codename, cfg.Components[0])
	b.WriteString("    | sudo tee /etc/apt/sources.list.d/archivist-local.list\n")
	b.WriteString("  sudo apt-get update\n")

	_, err = io.WriteString(w, b.String())
	return err
}

func describe(r *repo.Result) string {
	var parts []string
	if r.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", r.Added))
	}
	if r.Replaced > 0 {
		parts = append(parts, fmt.Sprintf("%d replaced", r.Replaced))
	}
	if r.Unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", r.Unchanged))
	}
	if len(parts) == 0 {
		return "no packages"
	}
	return strings.Join(parts, ", ")
}
