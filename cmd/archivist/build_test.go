package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Guys-Inc-Public/archivist/internal/debtest"
	"github.com/Guys-Inc-Public/archivist/internal/sign"
)

const testConfig = `
origin: Example Project
label: Example Project packages
description: Packages for Example Project
suite: stable
codename: stable
components: [main]
architectures: [amd64, arm64]
signing:
  key_id: %KEY%
`

// buildFixture writes packages, a configuration file and a signing key, then
// runs the build command exactly as a user would.
type fixture struct {
	source, out, configPath string
	fingerprint             string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Skip("gpg is not installed")
	}

	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GNUPGHOME="+home)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gpg, append([]string{"--batch", "--pinentry-mode", "loopback", "--passphrase", ""}, args...)...)
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("gpg %s: %v", strings.Join(args, " "), err)
		}
		return string(out)
	}

	run("--quick-generate-key", "Archivist Build Test <test@example.com>", "rsa2048", "cert", "1y")
	primary := firstFingerprint(run("--list-keys", "--with-colons"), false)
	run("--quick-add-key", primary, "rsa2048", "sign", "1y")
	subkey := firstFingerprint(run("--list-keys", "--with-colons"), true)
	t.Setenv(sign.EnvKey, run("--armor", "--export-secret-subkeys", subkey+"!"))

	f := &fixture{
		source:      t.TempDir(),
		out:         filepath.Join(t.TempDir(), "repo"),
		configPath:  filepath.Join(t.TempDir(), "archivist.yml"),
		fingerprint: subkey,
	}
	body := strings.ReplaceAll(testConfig, "%KEY%", subkey)
	if err := os.WriteFile(f.configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, p := range []struct{ name, version, arch string }{
		{"widget", "1.0", "amd64"},
		{"widget-doc", "1.0", "all"},
		{"gadget", "2.1-3", "arm64"},
	} {
		if _, err := debtest.Write(f.source, p.name+"_"+p.version+"_"+p.arch+".deb", debtest.Options{
			Control: debtest.Control(p.name, p.version, p.arch),
			Data:    map[string]string{"usr/share/doc/" + p.name + "/README": "synthetic\n"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A release directory holds more than packages. These must be ignored
	// rather than misread, and they must not be identified by extension.
	for name, body := range map[string]string{
		"checksums.txt":        "abc123  widget_1.0_amd64.deb\n",
		"archivist.tar.gz":     "not really a tarball",
		"notes.deb":            "this file is named .deb and is not one",
		"widget_1.0_amd64.sig": "signature",
	} {
		if err := os.WriteFile(filepath.Join(f.source, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) run(t *testing.T, extra ...string) {
	t.Helper()
	args := append([]string{"--config", f.configPath, "--out", f.out}, extra...)
	if err := build(append(args, f.source)); err != nil {
		t.Fatalf("build: %v", err)
	}
}

func TestBuildProducesASignedTree(t *testing.T) {
	f := newFixture(t)
	f.run(t)

	for _, want := range []string{
		"dists/stable/Release",
		"dists/stable/Release.gpg",
		"dists/stable/InRelease",
		"dists/stable/main/binary-amd64/Packages",
		"dists/stable/main/binary-arm64/Packages",
		"public.asc",
		"public.gpg",
		"pool/main/w/widget/widget_1.0_amd64.deb",
		"pool/main/g/gadget/gadget_2.1-3_arm64.deb",
	} {
		if _, err := os.Stat(filepath.Join(f.out, filepath.FromSlash(want))); err != nil {
			t.Errorf("missing %s", want)
		}
	}

	// The files that are not packages must not have become packages.
	index := readFile(t, filepath.Join(f.out, "dists/stable/main/binary-amd64/Packages"))
	if strings.Contains(index, "notes") || strings.Contains(index, "checksums") {
		t.Errorf("a non-package was indexed:\n%s", index)
	}
	// An epoch-free version with a Debian revision still round-trips.
	arm := readFile(t, filepath.Join(f.out, "dists/stable/main/binary-arm64/Packages"))
	if !strings.Contains(arm, "Filename: pool/main/g/gadget/gadget_2.1-3_arm64.deb") {
		t.Errorf("arm64 index is wrong:\n%s", arm)
	}
	// Architecture: all belongs in both.
	for _, arch := range []string{"amd64", "arm64"} {
		body := readFile(t, filepath.Join(f.out, "dists/stable/main/binary-"+arch+"/Packages"))
		if !strings.Contains(body, "Package: widget-doc") {
			t.Errorf("binary-%s is missing the Architecture: all package", arch)
		}
	}
}

func TestBuildIsRerunnable(t *testing.T) {
	f := newFixture(t)
	f.run(t)
	f.run(t) // the second run must not need --force
}

// The M1 acceptance test: a real apt, reading a repository over file://,
// verifying the signature against the key the build emitted.
//
// apt runs against a temporary state directory so that nothing on the machine
// running the tests is touched.
func TestAptAcceptsTheRepository(t *testing.T) {
	aptGet, err := exec.LookPath("apt-get")
	if err != nil {
		t.Skip("apt-get is not installed")
	}
	aptCache, err := exec.LookPath("apt-cache")
	if err != nil {
		t.Skip("apt-cache is not installed")
	}

	f := newFixture(t)
	f.run(t)

	state := t.TempDir()
	for _, dir := range []string{"lists/partial", "cache/archives/partial", "etc/preferences.d"} {
		if err := os.MkdirAll(filepath.Join(state, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sources := filepath.Join(state, "sources.list")
	line := "deb [signed-by=" + filepath.Join(f.out, "public.gpg") + "] file://" + f.out + " stable main\n"
	if err := os.WriteFile(sources, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	options := []string{
		"-o", "Dir::Etc::sourcelist=" + sources,
		"-o", "Dir::Etc::sourceparts=/dev/null",
		"-o", "Dir::Etc::preferencesparts=" + filepath.Join(state, "etc/preferences.d"),
		"-o", "Dir::State::lists=" + filepath.Join(state, "lists"),
		"-o", "Dir::Cache=" + filepath.Join(state, "cache"),
		"-o", "APT::Get::List-Cleanup=0",
		"-o", "APT::Sandbox::User=root",
	}

	update := exec.Command(aptGet, append(options, "update")...)
	out, err := update.CombinedOutput()
	if err != nil {
		t.Fatalf("apt-get update: %v\n%s", err, out)
	}
	// apt does not fail on an unverifiable repository, it warns and ignores it.
	// The warning is the failure.
	for _, bad := range []string{"NO_PUBKEY", "not signed", "BADSIG", "is not signed"} {
		if strings.Contains(string(out), bad) {
			t.Fatalf("apt rejected the signature (%s):\n%s", bad, out)
		}
	}

	// widget is amd64 and widget-doc is Architecture: all, so both are visible
	// here. gadget is arm64 and must not be: apt fetches only its own
	// architecture's index, which is the whole reason an Architecture: all
	// package has to be written into every one of them.
	for _, name := range []string{"widget", "widget-doc"} {
		if !aptSees(t, aptCache, options, name) {
			t.Errorf("apt cannot see %s", name)
		}
	}
	if aptSees(t, aptCache, options, "gadget") {
		t.Error("apt sees an arm64 package on an amd64 host; the per-architecture indices are not separate")
	}

	// A repository whose signature is decorative would pass everything above.
	// Corrupt an index and apt must refuse it: the Release that was signed
	// carries that index's checksum, so the chain has to break here.
	t.Run("tampering is detected", func(t *testing.T) {
		index := filepath.Join(f.out, "dists/stable/main/binary-amd64/Packages")
		body := readFile(t, index)
		if err := os.WriteFile(index, []byte(body+"\nPackage: smuggled\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		fresh := t.TempDir()
		for _, dir := range []string{"lists/partial", "cache/archives/partial"} {
			if err := os.MkdirAll(filepath.Join(fresh, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		tampered := append([]string{}, options...)
		for i := range tampered {
			if strings.HasPrefix(tampered[i], "Dir::State::lists=") {
				tampered[i] = "Dir::State::lists=" + filepath.Join(fresh, "lists")
			}
			if strings.HasPrefix(tampered[i], "Dir::Cache=") {
				tampered[i] = "Dir::Cache=" + filepath.Join(fresh, "cache")
			}
		}

		out, err := exec.Command(aptGet, append(tampered, "update")...).CombinedOutput()
		if err == nil && !strings.Contains(string(out), "Hash Sum mismatch") {
			t.Errorf("apt accepted a tampered index:\n%s", out)
		}
	})
}

func aptSees(t *testing.T, aptCache string, options []string, name string) bool {
	t.Helper()
	out, err := exec.Command(aptCache, append(append([]string{}, options...), "policy", name)...).CombinedOutput()
	if err != nil {
		t.Fatalf("apt-cache policy %s: %v\n%s", name, err, out)
	}
	return strings.Contains(string(out), "Candidate:") && !strings.Contains(string(out), "Candidate: (none)")
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func firstFingerprint(colons string, afterSub bool) string {
	seen := !afterSub
	for _, line := range strings.Split(colons, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 10 {
			continue
		}
		if fields[0] == "sub" {
			seen = true
			continue
		}
		if fields[0] == "fpr" && seen {
			return fields[9]
		}
	}
	return ""
}
