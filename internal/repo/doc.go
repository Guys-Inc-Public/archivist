// Package repo generates the published repository tree: the dists/ layout, the
// per-component Packages indices, and the Release file that binds them.
//
// The repository is regenerated in full on every publish. There is no database
// and no incremental state to corrupt; the package set in object storage is the
// only source of truth. See docs/decisions/0001-regenerate-from-scratch.md for
// why, and for the constraint that keeps it affordable.
package repo
