// Package sign produces the detached and inline OpenPGP signatures that apt
// requires: Release.gpg alongside Release, and InRelease containing both.
//
// Signing is isolated behind an interface so that the default implementation -
// a signing subkey held in a CI secret - can be replaced by an external signer
// without touching the repository generation code. See
// docs/decisions/0002-signing-key-handling.md.
package sign
