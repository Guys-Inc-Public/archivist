package repo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// sectionOf returns the indented lines following a header in a control file.
func sectionOf(body, header string) string {
	_, after, found := strings.Cut(body, header+"\n")
	if !found {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(after, "\n") {
		if !strings.HasPrefix(line, " ") {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func sha256File(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// snapshot records every file in a tree by path and content hash.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = sha256File(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// compareTrees fails with the specific difference rather than "trees differ",
// because "trees differ" is the least useful thing a determinism test can say.
func compareTrees(t *testing.T, want, got string) {
	t.Helper()
	a, b := snapshot(t, want), snapshot(t, got)

	names := map[string]bool{}
	for k := range a {
		names[k] = true
	}
	for k := range b {
		names[k] = true
	}
	sorted := make([]string, 0, len(names))
	for k := range names {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, name := range sorted {
		switch {
		case a[name] == "":
			t.Errorf("%s: present in the second tree only", name)
		case b[name] == "":
			t.Errorf("%s: present in the first tree only", name)
		case a[name] != b[name]:
			t.Errorf("%s: differs between builds", name)
		}
	}
}
