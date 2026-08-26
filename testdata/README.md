# testdata

Fixture packages for the test suite.

Fixtures are generated, not downloaded, so the tests never depend on a network
or on someone else's release remaining available. Real `.deb` files are
deliberately not committed: a repository whose fixtures are hundred-megabyte
binaries is unpleasant to clone, and a fixture that large rarely tests anything
a small synthetic package does not.

> [!NOTE]
> Fixture generation lands with M1, when there is a package reader to feed. The
> current tests exercise control-stanza parsing against inline fixtures in
> `internal/deb/control_test.go`.
