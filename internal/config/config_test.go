package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Guys-Inc-Public/archivist/internal/config"
)

// valid is a complete, correct configuration. Tests derive from it by
// substitution so that each case shows only what it is actually testing.
const valid = `
origin: Example Project
label: Example Project packages
description: Packages for Example Project
suite: stable
codename: stable
components: [main]
architectures: [amd64, arm64]
signing:
  key_id: 1234567890ABCDEF1234567890ABCDEF12345678
publish:
  bucket: example-apt
  endpoint: https://s3.example.com
  public_url: https://apt.example.com
`

func read(t *testing.T, body string, need config.Need) (*config.Config, error) {
	t.Helper()
	return config.Read("archivist.yml", strings.NewReader(body), need)
}

func mustRead(t *testing.T, body string, need config.Need) *config.Config {
	t.Helper()
	cfg, err := read(t, body, need)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
}

func TestReadValid(t *testing.T) {
	cfg := mustRead(t, valid, config.NeedPublish)

	if cfg.Origin != "Example Project" {
		t.Errorf("Origin = %q", cfg.Origin)
	}
	if cfg.Suite != "stable" || cfg.Codename != "stable" {
		t.Errorf("suite/codename = %q/%q", cfg.Suite, cfg.Codename)
	}
	if got := strings.Join(cfg.Components, ","); got != "main" {
		t.Errorf("Components = %q", got)
	}
	if got := strings.Join(cfg.Architectures, ","); got != "amd64,arm64" {
		t.Errorf("Architectures = %q", got)
	}
	if cfg.KeyID != "1234567890ABCDEF1234567890ABCDEF12345678" {
		t.Errorf("KeyID = %q", cfg.KeyID)
	}
	if cfg.ValidFor != 0 {
		t.Errorf("ValidFor = %v, want unset by default (decision 0011)", cfg.ValidFor)
	}
	if cfg.Publish.Region != "auto" {
		t.Errorf("Region = %q, want the documented default", cfg.Publish.Region)
	}
	if cfg.Publish.Prefix != "" {
		t.Errorf("Prefix = %q", cfg.Publish.Prefix)
	}
}

// The decision this package exists to encode: build writes to local disk and
// must not demand object-storage settings to do it.
func TestBuildDoesNotNeedPublishSettings(t *testing.T) {
	body := strings.Split(valid, "publish:")[0]
	cfg := mustRead(t, body, config.NeedBuild)
	if cfg.Publish.Bucket != "" {
		t.Errorf("Bucket = %q, want empty", cfg.Publish.Bucket)
	}

	_, err := read(t, body, config.NeedPublish)
	if err == nil {
		t.Fatal("publish accepted a configuration with no destination")
	}
	for _, want := range []string{"publish.bucket", "publish.public_url"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s:\n%v", want, err)
		}
	}
}

// A malformed publish URL is a mistake whether or not today's command uploads
// anything, so the format check does not wait for NeedPublish.
func TestPublishURLIsCheckedEvenForBuild(t *testing.T) {
	body := strings.ReplaceAll(valid, "https://apt.example.com", "apt.example.com")
	_, err := read(t, body, config.NeedBuild)
	if err == nil || !strings.Contains(err.Error(), "publish.public_url") {
		t.Fatalf("want a public_url complaint, got %v", err)
	}
}

// The headline property: one pass reports everything, so a five-field file is
// not fixed five times.
func TestReportsEveryProblemAtOnce(t *testing.T) {
	const broken = `
origin: ""
label: ""
suite: stable
codename: stable
components: []
architectures: [amd64, all, amd64]
valid_for: soon
signing:
  key_id: DEADBEEF
publish:
  public_url: ftp://apt.example.com
`
	_, err := read(t, broken, config.NeedBuild)

	var invalid *config.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("want *config.InvalidError, got %T: %v", err, err)
	}

	want := []string{
		"origin", "label", "components",
		"architectures[1]", "architectures[2]",
		"valid_for", "signing.key_id", "publish.public_url",
	}
	got := map[string]bool{}
	for _, p := range invalid.Problems {
		got[p.Field] = true
	}
	for _, field := range want {
		if !got[field] {
			t.Errorf("no problem reported for %s\ngot:\n%v", field, err)
		}
	}
	if len(invalid.Problems) < len(want) {
		t.Errorf("reported %d problems, want at least %d", len(invalid.Problems), len(want))
	}
	if !strings.Contains(err.Error(), "archivist.yml:") {
		t.Errorf("error does not name the file:\n%v", err)
	}
	// The message says which command's requirements were applied, because the
	// same file is valid for one command and not the other.
	if !strings.Contains(err.Error(), `"build"`) {
		t.Errorf("error does not name the command:\n%v", err)
	}
}

func TestArchitectureAllIsRejectedWithAnExplanation(t *testing.T) {
	body := strings.ReplaceAll(valid, "[amd64, arm64]", "[amd64, all]")
	_, err := read(t, body, config.NeedBuild)
	if err == nil {
		t.Fatal(`"all" was accepted as an architecture`)
	}
	// The rule is unintuitive enough that the message has to say why, not just
	// that it is wrong.
	if !strings.Contains(err.Error(), "every architecture") {
		t.Errorf("message does not explain the rule:\n%v", err)
	}
}

func TestKeyIDNormalisation(t *testing.T) {
	// What gpg --fingerprint actually prints, pasted verbatim.
	body := strings.ReplaceAll(valid,
		"1234567890ABCDEF1234567890ABCDEF12345678",
		"1234 5678 90ab cdef 1234  5678 90ab cdef 1234 5678")
	cfg := mustRead(t, body, config.NeedBuild)
	if cfg.KeyID != "1234567890ABCDEF1234567890ABCDEF12345678" {
		t.Errorf("KeyID = %q", cfg.KeyID)
	}
}

func TestShortKeyIDIsRejectedForBeingForgeable(t *testing.T) {
	for _, short := range []string{"90ABCDEF", "1234567890ABCDEF"} {
		body := strings.ReplaceAll(valid, "1234567890ABCDEF1234567890ABCDEF12345678", short)
		_, err := read(t, body, config.NeedBuild)
		if err == nil {
			t.Fatalf("short key ID %q was accepted", short)
		}
		if !strings.Contains(err.Error(), "collision") {
			t.Errorf("%q: message does not say why short IDs are refused:\n%v", short, err)
		}
	}
}

func TestValidFor(t *testing.T) {
	body := valid + "valid_for: 1w\n"
	cfg := mustRead(t, body, config.NeedBuild)
	if cfg.ValidFor != 7*24*time.Hour {
		t.Errorf("ValidFor = %v", cfg.ValidFor)
	}
}

func TestPrefixNormalisation(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"github-desktop", "github-desktop/"},
		{"github-desktop/", "github-desktop/"},
		{"/github-desktop/", "github-desktop/"},
		{"a/b", "a/b/"},
	} {
		cfg := mustRead(t, valid+"  prefix: \""+tc.in+"\"\n", config.NeedPublish)
		if cfg.Publish.Prefix != tc.want {
			t.Errorf("prefix %q -> %q, want %q", tc.in, cfg.Publish.Prefix, tc.want)
		}
	}
}

func TestRejects(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"unknown field", valid + "architecture: [amd64]\n", "architecture"},
		{"duplicate key", valid + "suite: unstable\n", "suite"},
		{"empty file", "\n\n", "empty"},
		{"not a mapping", "- one\n- two\n", "archivist.yml"},
		{"newline in origin", strings.ReplaceAll(valid, "origin: Example Project", "origin: \"a\\nSuite: forged\""), "control character"},
		{"suite with a slash", strings.ReplaceAll(valid, "suite: stable", "suite: sta/ble"), "suite"},
		{"empty components list", strings.ReplaceAll(valid, "components: [main]", "components: []"), "components"},
		{"duplicate architecture", strings.ReplaceAll(valid, "[amd64, arm64]", "[amd64, amd64]"), "more than once"},
		{"wildcard architecture", strings.ReplaceAll(valid, "[amd64, arm64]", "[amd64, any]"), "wildcard"},
		{"unparsable valid_for", valid + "valid_for: 3 fortnights\n", "valid_for"},
		{"zero valid_for", valid + "valid_for: 0d\n", "valid_for"},
		{"prefix with dots", valid + "  prefix: ../other\n", "publish.prefix"},
		{"endpoint without a scheme", strings.ReplaceAll(valid, "https://s3.example.com", "s3.example.com"), "publish.endpoint"},
		{"public_url with a query", strings.ReplaceAll(valid, "https://apt.example.com", "https://apt.example.com/?x=1"), "query"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := read(t, tc.body, config.NeedPublish)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not mention %q:\n%v", tc.want, err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "archivist.yml")
	if err := os.WriteFile(name, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(name, config.NeedPublish); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, err := config.Load(filepath.Join(dir, "absent.yml"), config.NeedBuild); err == nil {
		t.Fatal("Load accepted a missing file")
	}
}
