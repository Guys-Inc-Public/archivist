package config

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

// Problem is one thing wrong with a configuration file.
type Problem struct {
	// Field is the YAML path, so the message points at a line the author can
	// find: "signing.key_id", "architectures[2]".
	Field   string
	Message string
}

func (p Problem) String() string { return p.Field + ": " + p.Message }

// InvalidError reports every problem found in a configuration file. Problems
// appear in schema order, which is roughly the order they appear in the file.
type InvalidError struct {
	Path     string
	Need     Need
	Problems []Problem
}

func (e *InvalidError) Error() string {
	var b strings.Builder
	noun := "problem"
	if len(e.Problems) != 1 {
		noun = "problems"
	}
	fmt.Fprintf(&b, "%s: %d %s for %q", e.Path, len(e.Problems), noun, e.Need)
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p.String())
	}
	return b.String()
}

// validator accumulates problems instead of returning at the first one.
type validator struct {
	problems []Problem
	need     Need
}

func (v *validator) add(field, format string, args ...any) {
	v.problems = append(v.problems, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
}

var (
	// A Debian architecture name as it appears in binary-<arch> and in a
	// package's Architecture field.
	archPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	// An archive component: main, contrib, non-free-firmware.
	componentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`)
	// A suite or codename, which becomes a path element under dists/.
	suitePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`)
	fingerprint  = regexp.MustCompile(`^[0-9A-F]{40}$`)
)

func validate(name string, raw *file, need Need) (*Config, error) {
	v := &validator{need: need}

	cfg := &Config{
		Origin:      strings.TrimSpace(raw.Origin),
		Label:       strings.TrimSpace(raw.Label),
		Description: strings.TrimSpace(raw.Description),
		Suite:       strings.TrimSpace(raw.Suite),
		Codename:    strings.TrimSpace(raw.Codename),
	}

	// Every one of these is written verbatim into Release, which is a control
	// stanza and then signed. A newline in a value does not corrupt the file;
	// it forges a field, and the forgery is signed along with everything else.
	v.releaseValue("origin", cfg.Origin, true)
	v.releaseValue("label", cfg.Label, true)
	v.releaseValue("description", cfg.Description, false)

	v.releaseValue("suite", cfg.Suite, true)
	if cfg.Suite != "" && !suitePattern.MatchString(cfg.Suite) {
		v.add("suite", "%q is not usable as a path element under dists/", cfg.Suite)
	}
	v.releaseValue("codename", cfg.Codename, true)
	if cfg.Codename != "" && !suitePattern.MatchString(cfg.Codename) {
		v.add("codename", "%q is not usable as a path element under dists/", cfg.Codename)
	}

	cfg.Components = v.list("components", raw.Components, componentPattern,
		"an archive component name such as main")
	cfg.Architectures = v.architectures(raw.Architectures)

	cfg.KeyID = v.keyID(raw.Signing.KeyID)
	cfg.ValidFor = v.validFor(raw.ValidFor)
	cfg.Publish = v.publish(&raw.Publish)

	if len(v.problems) > 0 {
		return nil, &InvalidError{Path: name, Need: need, Problems: v.problems}
	}
	return cfg, nil
}

// releaseValue checks a value destined for a Release field.
func (v *validator) releaseValue(field, value string, required bool) {
	if value == "" {
		if required {
			v.add(field, "required")
		}
		return
	}
	if i := strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }); i >= 0 {
		v.add(field, "contains a control character at byte %d; Release fields are single lines", i)
	}
}

// list validates a required list of path-safe names.
func (v *validator) list(field string, values []string, pattern *regexp.Regexp, want string) []string {
	if len(values) == 0 {
		v.add(field, "required, and must list at least one entry")
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for i, raw := range values {
		at := fmt.Sprintf("%s[%d]", field, i)
		value := strings.TrimSpace(raw)
		switch {
		case value == "":
			v.add(at, "empty entry")
			continue
		case seen[value]:
			v.add(at, "%q is listed more than once", value)
			continue
		case !pattern.MatchString(value):
			v.add(at, "%q is not %s", value, want)
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (v *validator) architectures(values []string) []string {
	out := v.list("architectures", values, archPattern, "a Debian architecture name such as amd64")
	for i, a := range out {
		switch a {
		case "all":
			// Worth spelling out: "all" looks like it belongs here, and the
			// reason it does not is the least obvious rule in the format.
			v.add(fmt.Sprintf("architectures[%d]", i),
				`"all" is not an architecture and must not be listed. `+
					`Architecture: all packages are written into the index of every architecture here, `+
					`because that is where apt looks for them`)
		case "any", "source":
			v.add(fmt.Sprintf("architectures[%d]", i),
				"%q is a wildcard, not an architecture a repository can offer", a)
		}
	}
	return out
}

// keyID normalises a fingerprint. gpg prints fingerprints in spaced groups and
// people paste what gpg printed, so spaces and an 0x prefix are accepted rather
// than treated as a mistake.
func (v *validator) keyID(raw string) string {
	id := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	id = strings.TrimPrefix(id, "0X")

	if id == "" {
		v.add("signing.key_id", "required: the 40-character fingerprint of the signing subkey")
		return ""
	}
	if fingerprint.MatchString(id) {
		return id
	}
	// A short key ID is not merely the wrong length. Short IDs are forgeable by
	// collision, so accepting one would let a repository be signed by a key the
	// author did not mean to trust.
	if len(id) == 8 || len(id) == 16 {
		v.add("signing.key_id",
			"%q is a short key ID; give the full 40-character fingerprint, which cannot be forged by collision", raw)
		return ""
	}
	v.add("signing.key_id", "%q is not a 40-character hex fingerprint", raw)
	return ""
}

func (v *validator) validFor(raw string) time.Duration {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0 // unset, and unset is the documented default
	}
	d, err := parseDuration(s)
	if err != nil {
		v.add("valid_for", "%v", err)
		return 0
	}
	if d <= 0 {
		v.add("valid_for", "%q is not a positive duration; omit the field to publish without Valid-Until", raw)
		return 0
	}
	return d
}

func (v *validator) publish(raw *publishFile) Publish {
	required := v.need >= NeedPublish

	p := Publish{
		Bucket:    strings.TrimSpace(raw.Bucket),
		Region:    strings.TrimSpace(raw.Region),
		Endpoint:  strings.TrimSpace(raw.Endpoint),
		PublicURL: strings.TrimSpace(raw.PublicURL),
	}
	if p.Region == "" {
		p.Region = "auto"
	}

	if p.Bucket == "" && required {
		v.add("publish.bucket", "required to publish")
	}

	// Format is checked whether or not the command needs the field. A typo in a
	// bucket URL should surface when the file is edited, not on the day someone
	// first runs publish.
	if p.PublicURL == "" {
		if required {
			v.add("publish.public_url", "required to publish: the URL users will fetch the repository from")
		}
	} else {
		p.PublicURL = v.httpURL("publish.public_url", p.PublicURL)
	}
	if p.Endpoint != "" {
		p.Endpoint = v.httpURL("publish.endpoint", p.Endpoint)
	}

	p.Prefix = v.prefix(raw.Prefix)
	return p
}

// httpURL checks a URL a client will fetch and returns it without a trailing
// slash, so that joining a path to it is unambiguous.
func (v *validator) httpURL(field, value string) string {
	u, err := url.Parse(value)
	if err != nil {
		v.add(field, "%q is not a valid URL: %v", value, err)
		return value
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		v.add(field, "%q must begin with http:// or https://", value)
	case u.Host == "":
		v.add(field, "%q has no host", value)
	case u.RawQuery != "" || u.Fragment != "":
		v.add(field, "%q must not carry a query string or fragment", value)
	}
	return strings.TrimRight(value, "/")
}

// prefix normalises a key prefix to "" or something ending in "/".
//
// The normalisation matters more than it looks: publishing deletes keys under
// this prefix that the regenerated tree does not contain, so a prefix that
// resolves somewhere other than where it reads is a prefix that deletes
// somebody else's repository.
func (v *validator) prefix(raw string) string {
	p := strings.Trim(strings.TrimSpace(raw), "/")
	if p == "" {
		return ""
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		v.add("publish.prefix", "%q escapes the bucket root", raw)
		return ""
	}
	if cleaned := path.Clean(p); cleaned != p {
		v.add("publish.prefix", "%q is not a plain key prefix; write it as %q", raw, cleaned+"/")
		return ""
	}
	return p + "/"
}
