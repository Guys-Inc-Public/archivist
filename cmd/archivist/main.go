// Command archivist builds, publishes, and verifies signed apt repositories.
package main

import (
	"errors"
	"flag"
	"fmt"
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
  inspect    Print the parsed control stanza of a Debian control file
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
	case "build", "publish", "verify":
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

// inspect parses a Debian control file and prints its fields. It exists so the
// control-stanza parser - the component everything else depends on - can be
// exercised directly against a real package's metadata.
func inspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	component := fs.String("component", "main", "archive component, for the computed pool path")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: archivist inspect [flags] <control-file>")
		fmt.Fprintln(fs.Output(), "\nReads a Debian control file, or standard input when the path is \"-\".")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("expected exactly one control file")
	}

	in := os.Stdin
	if path := fs.Arg(0); path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}

	control, err := deb.ParseControl(in)
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
	return nil
}
