// Command archivist builds, publishes, and verifies signed apt repositories.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Guys-Inc-Public/archivist/internal/deb"
)

// Overridden at link time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// errNotImplemented is returned by commands whose behaviour is designed but not
// yet built. Returning a distinct exit code keeps a script that calls archivist
// from mistaking "not built yet" for "nothing to do".
var errNotImplemented = errors.New("not implemented in this pre-release")

const usage = `archivist - signed apt repositories from a directory of .deb files

Usage:
  archivist <command> [flags]

Commands:
  build      Generate a signed repository tree from a directory of packages
  publish    Upload a repository tree to S3-compatible object storage
  verify     Check that a published repository is internally consistent
  inspect    Print the parsed control stanza of a .deb or control file
  version    Print version information

Run "archivist <command> -h" for the flags a command accepts.
Documentation: https://github.com/Guys-Inc-Public/archivist/tree/main/docs
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "archivist: %v\n", err)
		if errors.Is(err, errNotImplemented) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return errors.New("no command given")
	}

	switch cmd := args[0]; cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version":
		return printVersion()
	case "inspect":
		return inspect(args[1:])
	case "build":
		return build(args[1:])
	case "publish", "verify":
		return fmt.Errorf("%q: %w - see the roadmap for milestone status", cmd, errNotImplemented)
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func printVersion() error {
	v, c, d := version, commit, date
	// A `go install`ed binary has no ldflags, but the module system still knows
	// what it built from.
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	_, err := fmt.Printf("archivist %s (commit %s, built %s)\n", v, c, d)
	return err
}

// inspect prints the parsed control stanza of a package. It exists so the
// control-stanza parser - the component everything else depends on - can be
// exercised directly against a real package's metadata.
func inspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	component := fs.String("component", "main", "archive component, for the computed pool path")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "Usage: archivist inspect [flags] <file>\n\n"+
			"Reads a .deb package or a bare Debian control file, or standard input\n"+
			"when the path is \"-\". Which one it is is determined by content, never\n"+
			"by the file's name.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("expected exactly one file")
	}

	name := fs.Arg(0)
	var in io.Reader = os.Stdin
	if name != "-" {
		// #nosec G304 -- reading a caller-supplied path is this command's entire job.
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		in = f
	}

	control, pkg, err := readForInspection(name, in)
	if err != nil {
		return err
	}

	fields := control.Fields()
	width := 0
	for _, f := range fields {
		if len(f) > width {
			width = len(f)
		}
	}
	for _, f := range fields {
		// Continuation lines are re-indented so the output stays readable.
		value := strings.ReplaceAll(control.Get(f), "\n", "\n"+strings.Repeat(" ", width+2))
		fmt.Printf("%-*s  %s\n", width, f, value)
	}
	fmt.Printf("\n%-*s  %s\n", width, "(pool path)", control.PoolPath(*component))
	if pkg != nil {
		fmt.Printf("%-*s  %d\n", width, "(size)", pkg.Size)
		fmt.Printf("%-*s  %s\n", width, "(sha256)", pkg.SHA256)
	}
	return nil
}

// readForInspection decides whether the input is a .deb or a bare control file
// by looking at what it starts with. Decision 0006 forbids trusting a package's
// name for its identity; choosing a parser by extension would be the same
// mistake in a smaller place, and would break "-" besides.
func readForInspection(name string, r io.Reader) (*deb.Control, *deb.Package, error) {
	buf := bufio.NewReader(r)

	magic, err := buf.Peek(len(debMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, nil, err
	}
	if string(magic) != debMagic {
		control, err := deb.ParseControl(buf)
		return control, nil, err
	}

	pkg, err := deb.Read(name, buf)
	if err != nil {
		return nil, nil, err
	}
	return pkg.Control, pkg, nil
}

// debMagic is the ar archive header every .deb file begins with.
const debMagic = "!<arch>\n"

// parseFlags parses args, allowing flags and positional arguments to be
// interleaved.
//
// The standard flag package stops at the first non-flag word, so
// "build ./dist --out ./repo" would treat --out as another operand. That is the
// order the documentation uses and the order people type, and being right about
// argument order is not a thing a user should have to be.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var operands []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return operands, nil
		}
		operands = append(operands, fs.Arg(0))
		rest = fs.Args()[1:]
	}
}
