package sign_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Guys-Inc-Public/archivist/internal/sign"
)

// The certify guard is tested elsewhere against a key this package builds
// itself. That test can only prove the guard is self-consistent: the fixture
// and the check were written together and could be wrong together.
//
// This one hands the guard what GnuPG actually produces. A subkey export from
// gpg carries a gnu-dummy stub of the primary rather than the public packet the
// Go fixture uses, and it is the stub that the rule in docs/Signing-Keys.md is
// really about. Same discipline as decision 0013 applies to the .deb reader.
func TestGuardAgreesWithRealGpgExports(t *testing.T) {
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Skip("gpg is not installed")
	}

	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GNUPGHOME="+home)

	gpgRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gpg, append([]string{"--batch", "--pinentry-mode", "loopback", "--passphrase", ""}, args...)...)
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("gpg %s: %v", strings.Join(args, " "), err)
		}
		return string(out)
	}

	// A primary that can certify and nothing else, plus a signing subkey: the
	// arrangement docs/Signing-Keys.md tells users to create.
	gpgRun("--quick-generate-key", "Archivist Test <test@example.com>", "rsa2048", "cert", "1y")
	primary := colonField(t, gpgRun("--list-keys", "--with-colons"), "fpr", false)
	gpgRun("--quick-add-key", primary, "rsa2048", "sign", "1y")
	subkey := colonField(t, gpgRun("--list-keys", "--with-colons"), "fpr", true)

	if subkey == "" || subkey == primary {
		t.Fatalf("could not find the signing subkey (primary %s, subkey %q)", primary, subkey)
	}

	full := gpgRun("--armor", "--export-secret-keys", primary)
	// The trailing "!" restricts the export to this subkey rather than the
	// whole keyring, which is the step users most often miss.
	subkeyOnly := gpgRun("--armor", "--export-secret-subkeys", subkey+"!")

	t.Run("a full secret key is refused", func(t *testing.T) {
		_, err := sign.LoadKey(strings.NewReader(full), "", subkey)
		if err == nil {
			t.Fatal("gpg's full secret key export was accepted")
		}
		if !strings.Contains(err.Error(), primary) {
			t.Errorf("error does not name the offending key:\n%v", err)
		}
	})

	t.Run("a subkey export is accepted and signs", func(t *testing.T) {
		key, err := sign.LoadKey(strings.NewReader(subkeyOnly), "", subkey)
		if err != nil {
			t.Fatalf("gpg's subkey export was refused: %v", err)
		}
		if key.Fingerprint() != subkey {
			t.Errorf("selected %s, want %s", key.Fingerprint(), subkey)
		}

		inline, err := sign.Inline(key, []byte(releaseBody))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(home, "InRelease")
		if err := os.WriteFile(path, inline, 0o600); err != nil {
			t.Fatal(err)
		}
		// Verified by the same gpg that made the key, closing the loop.
		gpgRun("--verify", path)
	})
}

// colonField pulls a fingerprint out of gpg's --with-colons output. When sub is
// true it returns the first fingerprint following a "sub" record, which is the
// subkey's; otherwise the first one, which is the primary's.
func colonField(t *testing.T, output, want string, sub bool) string {
	t.Helper()
	seen := !sub
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 10 {
			continue
		}
		if fields[0] == "sub" {
			seen = true
			continue
		}
		if fields[0] == want && seen {
			return fields[9]
		}
	}
	return ""
}
