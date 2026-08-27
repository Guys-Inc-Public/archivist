package repo_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Guys-Inc-Public/archivist/internal/config"
	"github.com/Guys-Inc-Public/archivist/internal/deb"
	"github.com/Guys-Inc-Public/archivist/internal/debtest"
	"github.com/Guys-Inc-Public/archivist/internal/repo"
)

// A fixed timestamp: Release carries a Date, and a test that asserts on bytes
// cannot have one that moves.
var buildTime = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

const configBody = `
origin: Example Project
label: Example Project packages
description: Packages for Example Project
suite: stable
codename: stable
components: [main]
architectures: [amd64, arm64]
signing:
  key_id: 1234567890ABCDEF1234567890ABCDEF12345678
`

func testConfig(t *testing.T, body string) *config.Config {
	t.Helper()
	cfg, err := config.Read("archivist.yml", strings.NewReader(body), config.NeedBuild)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return cfg
}

// pkg writes a fixture package and reads it back the way the CLI will.
func pkg(t *testing.T, dir, name, version, arch string, extra ...string) *deb.Package {
	t.Helper()
	path, err := debtest.Write(dir, name+"_"+version+"_"+arch+".deb", debtest.Options{
		Control: debtest.Control(name, version, arch, extra...),
		Data:    map[string]string{"usr/bin/" + name: "#!/bin/sh\necho " + name + "\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := deb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func generate(t *testing.T, out string, packages []*deb.Package, cfg *config.Config, opts ...func(*repo.Options)) *repo.Result {
	t.Helper()
	o := repo.Options{Config: cfg, Out: out, Now: buildTime}
	for _, f := range opts {
		f(&o)
	}
	result, err := repo.Generate(packages, o)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return result
}

func read(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGenerateWritesTheDocumentedLayout(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := testConfig(t, configBody)
	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, cfg)

	for _, want := range []string{
		"dists/stable/Release",
		"dists/stable/main/binary-amd64/Packages",
		"dists/stable/main/binary-amd64/Packages.gz",
		"dists/stable/main/binary-amd64/Release",
		"dists/stable/main/binary-arm64/Packages",
		"pool/main/w/widget/widget_1.0_amd64.deb",
		"pool/main/w/widget/widget_1.0_amd64.deb.archivist.json",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(want))); err != nil {
			t.Errorf("missing %s", want)
		}
	}

	// An architecture with no packages still gets an index. An absent Packages
	// file is a 404 for every client running that architecture.
	if body := read(t, out, "dists/stable/main/binary-arm64/Packages"); body != "" {
		t.Errorf("arm64 index is not empty:\n%s", body)
	}
}

// v0.1 has no way to route one package to main and another to contrib, so a
// second component gets a valid index that stays empty. That is a documented
// limitation rather than an accident, and an empty index is not the same thing
// as a missing one: a missing Packages file is a 404 on every apt update.
func TestASecondComponentGetsAnEmptyButValidIndex(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := testConfig(t, strings.ReplaceAll(configBody, "components: [main]", "components: [main, contrib]"))
	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, cfg)

	if body := read(t, out, "dists/stable/contrib/binary-amd64/Packages"); body != "" {
		t.Errorf("contrib is not empty:\n%s", body)
	}
	if !strings.Contains(read(t, out, "dists/stable/main/binary-amd64/Packages"), "Package: widget") {
		t.Error("the package did not go into the first component")
	}

	release := read(t, out, "dists/stable/Release")
	if !strings.Contains(release, "Components: main contrib") {
		t.Errorf("Release does not advertise both components:\n%s", release)
	}
	// Release must still bind the empty index, or apt has an unlisted file.
	if !strings.Contains(release, "contrib/binary-amd64/Packages") {
		t.Errorf("Release does not list the empty component's index:\n%s", release)
	}
}

func TestPackagesEntryDescribesThePoolObject(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	p := pkg(t, in, "widget", "1.0", "amd64")
	generate(t, out, []*deb.Package{p}, testConfig(t, configBody))

	index := read(t, out, "dists/stable/main/binary-amd64/Packages")
	for _, want := range []string{
		"Package: widget",
		"Version: 1.0",
		"Architecture: amd64",
		"Filename: pool/main/w/widget/widget_1.0_amd64.deb",
		"Size: " + itoa(p.Size),
		"MD5sum: " + p.MD5,
		"SHA1: " + p.SHA1,
		"SHA256: " + p.SHA256,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index is missing %q:\n%s", want, index)
		}
	}
	// Package must come first: an index whose stanzas start elsewhere is legal
	// but every tool that reads one by eye expects this.
	if !strings.HasPrefix(index, "Package: widget\n") {
		t.Errorf("stanza does not begin with Package:\n%s", index)
	}
}

// Architecture: all is the least intuitive rule in the format, so it gets the
// most explicit test.
func TestArchitectureAllAppearsInEveryIndex(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	generate(t, out, []*deb.Package{pkg(t, in, "docs", "2.0", "all")}, testConfig(t, configBody))

	for _, arch := range []string{"amd64", "arm64"} {
		index := read(t, out, "dists/stable/main/binary-"+arch+"/Packages")
		if !strings.Contains(index, "Package: docs") {
			t.Errorf("binary-%s does not list the Architecture: all package:\n%s", arch, index)
		}
	}
	// Stored once, listed twice.
	matches, _ := filepath.Glob(filepath.Join(out, "pool", "main", "d", "docs", "*.deb"))
	if len(matches) != 1 {
		t.Errorf("pool holds %d objects, want 1: %v", len(matches), matches)
	}
}

// Two builds of the same input must produce identical bytes, or a regenerated
// repository cannot be diffed against the one it replaces.
func TestOutputIsByteForByteDeterministic(t *testing.T) {
	in := t.TempDir()
	packages := []*deb.Package{
		pkg(t, in, "widget", "1.0", "amd64"),
		pkg(t, in, "docs", "2.0", "all"),
		pkg(t, in, "libthing", "3.0", "arm64"),
	}
	cfg := testConfig(t, configBody)

	first, second := t.TempDir(), t.TempDir()
	generate(t, first, packages, cfg)
	// Reversed, because input order is a directory listing and not a property
	// worth publishing.
	reversed := []*deb.Package{packages[2], packages[1], packages[0]}
	generate(t, second, reversed, cfg)

	compareTrees(t, first, second)
}

func TestMergeKeepsWhatIsAlreadyPublished(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := testConfig(t, configBody)

	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, cfg)
	result := generate(t, out, []*deb.Package{pkg(t, in, "widget", "2.0", "amd64")}, cfg)

	if result.Added != 1 || result.Total != 2 {
		t.Errorf("Added=%d Total=%d, want 1 and 2", result.Added, result.Total)
	}
	index := read(t, out, "dists/stable/main/binary-amd64/Packages")
	for _, want := range []string{"Version: 1.0", "Version: 2.0"} {
		if !strings.Contains(index, want) {
			t.Errorf("index lost %q after the second build:\n%s", want, index)
		}
	}
}

func TestRebuildingTheSameInputChangesNothing(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := testConfig(t, configBody)
	p := pkg(t, in, "widget", "1.0", "amd64")

	generate(t, out, []*deb.Package{p}, cfg)
	before := snapshot(t, out)

	result := generate(t, out, []*deb.Package{p}, cfg)
	if result.Unchanged != 1 || result.Added != 0 {
		t.Errorf("Unchanged=%d Added=%d, want 1 and 0", result.Unchanged, result.Added)
	}
	if after := snapshot(t, out); !equalSnapshots(before, after) {
		t.Error("rebuilding the same input changed the tree")
	}
}

func TestRepublishingDifferentContentNeedsPermission(t *testing.T) {
	first, second, out := t.TempDir(), t.TempDir(), t.TempDir()
	cfg := testConfig(t, configBody)

	generate(t, out, []*deb.Package{pkg(t, first, "widget", "1.0", "amd64")}, cfg)

	// Same name, version and architecture; different payload.
	path, err := debtest.Write(second, "widget_1.0_amd64.deb", debtest.Options{
		Control: debtest.Control("widget", "1.0", "amd64"),
		Data:    map[string]string{"usr/bin/widget": "#!/bin/sh\necho something else\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := deb.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Generate([]*deb.Package{changed}, repo.Options{Config: cfg, Out: out, Now: buildTime})
	if !errors.Is(err, repo.ErrAlreadyPublished) {
		t.Fatalf("want ErrAlreadyPublished, got %v", err)
	}

	result := generate(t, out, []*deb.Package{changed}, cfg, func(o *repo.Options) { o.Replace = true })
	if result.Replaced != 1 {
		t.Errorf("Replaced=%d, want 1", result.Replaced)
	}
	if got := read(t, out, "dists/stable/main/binary-amd64/Packages"); !strings.Contains(got, changed.SHA256) {
		t.Error("index still records the superseded checksum")
	}
}

func TestRejects(t *testing.T) {
	cfg := testConfig(t, configBody)

	t.Run("undeclared architecture", func(t *testing.T) {
		in, out := t.TempDir(), t.TempDir()
		_, err := repo.Generate([]*deb.Package{pkg(t, in, "widget", "1.0", "riscv64")},
			repo.Options{Config: cfg, Out: out, Now: buildTime})
		if err == nil || !strings.Contains(err.Error(), "riscv64") {
			t.Fatalf("want a complaint naming riscv64, got %v", err)
		}
	})

	t.Run("duplicate in one input set", func(t *testing.T) {
		a, b, out := t.TempDir(), t.TempDir(), t.TempDir()
		_, err := repo.Generate([]*deb.Package{
			pkg(t, a, "widget", "1.0", "amd64"),
			pkg(t, b, "widget", "1.0", "amd64"),
		}, repo.Options{Config: cfg, Out: out, Now: buildTime})
		if err == nil || !strings.Contains(err.Error(), "widget_1.0_amd64") {
			t.Fatalf("want a duplicate complaint, got %v", err)
		}
	})

	t.Run("unconfigured component", func(t *testing.T) {
		out := t.TempDir()
		_, err := repo.Generate(nil, repo.Options{Config: cfg, Out: out, Component: "contrib", Now: buildTime})
		if err == nil || !strings.Contains(err.Error(), "contrib") {
			t.Fatalf("want a component complaint, got %v", err)
		}
	})
}

// A package does not get to say where apt believes it lives. Filename is
// computed; a stanza carrying one is a path traversal with a signature over it.
func TestPackageCannotChooseItsOwnFilename(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	path, err := debtest.Write(in, "widget.deb", debtest.Options{
		Control: debtest.Control("widget", "1.0", "amd64",
			"Filename: ../../../etc/cron.d/pwn",
			"SHA256: 0000000000000000000000000000000000000000000000000000000000000000"),
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := deb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	generate(t, out, []*deb.Package{p}, testConfig(t, configBody))

	index := read(t, out, "dists/stable/main/binary-amd64/Packages")
	if strings.Contains(index, "cron.d") {
		t.Errorf("the package chose its own Filename:\n%s", index)
	}
	if !strings.Contains(index, "Filename: pool/main/w/widget/widget_1.0_amd64.deb") {
		t.Errorf("computed Filename is missing:\n%s", index)
	}
	if strings.Contains(index, "SHA256: 00000000") {
		t.Errorf("the package supplied its own SHA256:\n%s", index)
	}
	if !strings.Contains(index, "SHA256: "+p.SHA256) {
		t.Errorf("index does not carry the real SHA256:\n%s", index)
	}
}

func TestReleaseBindsEveryIndex(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, testConfig(t, configBody))

	release := read(t, out, "dists/stable/Release")
	for _, want := range []string{
		"Origin: Example Project",
		"Suite: stable",
		"Codename: stable",
		"Architectures: amd64 arm64",
		"Components: main",
		"Date: Thu, 27 Aug 2026 12:00:00 UTC",
		"MD5Sum:", "SHA1:", "SHA256:",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("Release is missing %q:\n%s", want, release)
		}
	}
	if strings.Contains(release, "Valid-Until") {
		t.Error("Valid-Until is present without valid_for; decision 0011 says it is opt-in")
	}

	// Every checksum in the SHA256 block must match the file it names, or the
	// trust chain is decorative.
	for _, line := range strings.Split(sectionOf(release, "SHA256:"), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		digest, name := fields[0], fields[2]
		got := sha256File(t, filepath.Join(out, "dists", "stable", filepath.FromSlash(name)))
		if got != digest {
			t.Errorf("%s: Release says %s, file is %s", name, digest, got)
		}
	}
}

func TestValidUntilIsWrittenWhenConfigured(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := testConfig(t, configBody+"valid_for: 7d\n")
	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, cfg)

	release := read(t, out, "dists/stable/Release")
	if !strings.Contains(release, "Valid-Until: Thu, 03 Sep 2026 12:00:00 UTC") {
		t.Errorf("Valid-Until is wrong:\n%s", release)
	}
}

func TestScanSidecarsRoundTrips(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	generate(t, out, []*deb.Package{
		pkg(t, in, "widget", "1.0", "amd64"),
		pkg(t, in, "docs", "2.0", "all"),
	}, testConfig(t, configBody))

	entries, err := repo.ScanSidecars(out)
	if err != nil {
		t.Fatalf("ScanSidecars: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("read %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Schema != repo.SidecarSchema {
			t.Errorf("schema = %d", e.Schema)
		}
		// The whole stanza is kept, not a subset: an index needs fields we do
		// not currently use, and re-reading the pool to recover one would
		// defeat the point of the sidecar.
		if e.Control["Maintainer"] == "" || e.Control["Description"] == "" {
			t.Errorf("sidecar dropped control fields: %v", e.Control)
		}
	}

	// A sidecar whose object is gone is a defect, not something to carry into
	// a new index.
	object := filepath.Join(out, "pool", "main", "w", "widget", "widget_1.0_amd64.deb")
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ScanSidecars(out); err == nil {
		t.Error("ScanSidecars accepted a sidecar with no object")
	}
}

// A pool object's path is computed from its control stanza, so an object that
// is not where its own stanza puts it has been renamed or the sidecar was
// copied next to the wrong file. Carrying either into an index publishes a
// Filename that resolves to nothing.
func TestScanSidecarsRejectsAMisplacedObject(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	generate(t, out, []*deb.Package{pkg(t, in, "widget", "1.0", "amd64")}, testConfig(t, configBody))

	dir := filepath.Join(out, "pool", "main", "w", "widget")
	for _, name := range []string{"widget_1.0_amd64.deb", "widget_1.0_amd64.deb" + repo.SidecarSuffix} {
		renamed := strings.Replace(name, "widget_1.0_amd64.deb", "WidgetInstaller-1.0.deb", 1)
		if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, renamed)); err != nil {
			t.Fatal(err)
		}
	}

	_, err := repo.ScanSidecars(out)
	if err == nil {
		t.Fatal("ScanSidecars accepted an object that is not where its stanza puts it")
	}
	if !strings.Contains(err.Error(), "widget_1.0_amd64.deb") {
		t.Errorf("error does not say where it should have been:\n%v", err)
	}
}

// The index is generated from sidecars on a rebuild and from packages on a
// first build. If those two paths disagreed, a repository would change shape
// the second time it was written.
func TestSidecarPathAndPackagePathAgree(t *testing.T) {
	in := t.TempDir()
	cfg := testConfig(t, configBody)
	packages := []*deb.Package{pkg(t, in, "widget", "1.0", "amd64"), pkg(t, in, "docs", "2.0", "all")}

	fromPackages := t.TempDir()
	generate(t, fromPackages, packages, cfg)

	fromSidecars := t.TempDir()
	generate(t, fromSidecars, packages, cfg)
	generate(t, fromSidecars, nil, cfg) // regenerate reading only sidecars

	compareTrees(t, fromPackages, fromSidecars)
}

// Validating the generator against Debian's own indexer, for the same reason
// decision 0013 validates the reader against dpkg-deb: a generator and a test
// written together can agree with each other and still be wrong.
func TestIndexAgreesWithAptFtparchive(t *testing.T) {
	tool, err := exec.LookPath("apt-ftparchive")
	if err != nil {
		t.Skip("apt-ftparchive is not installed")
	}

	in, out := t.TempDir(), t.TempDir()
	packages := []*deb.Package{
		pkg(t, in, "widget", "1.0", "amd64"),
		pkg(t, in, "libthing", "2.1-3", "amd64"),
	}
	generate(t, out, packages, testConfig(t, configBody))

	cmd := exec.Command(tool, "packages", "pool")
	cmd.Dir = out
	reference, err := cmd.Output()
	if err != nil {
		t.Fatalf("apt-ftparchive: %v", err)
	}

	ours := fieldsByPackage(t, read(t, out, "dists/stable/main/binary-amd64/Packages"))
	theirs := fieldsByPackage(t, string(reference))

	for name, want := range theirs {
		got, ok := ours[name]
		if !ok {
			t.Errorf("apt-ftparchive indexed %s and we did not", name)
			continue
		}
		for _, field := range []string{"Version", "Architecture", "Filename", "Size", "MD5sum", "SHA1", "SHA256"} {
			if want[field] != "" && got[field] != want[field] {
				t.Errorf("%s %s: apt-ftparchive says %q, we say %q", name, field, want[field], got[field])
			}
		}
	}
	if len(ours) != len(theirs) {
		t.Errorf("we indexed %d packages, apt-ftparchive indexed %d", len(ours), len(theirs))
	}
}

func fieldsByPackage(t *testing.T, index string) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, block := range strings.Split(strings.TrimSpace(index), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, " ") {
				continue
			}
			if name, value, found := strings.Cut(line, ":"); found {
				fields[name] = strings.TrimSpace(value)
			}
		}
		if name := fields["Package"]; name != "" {
			out[name] = fields
		}
	}
	return out
}
