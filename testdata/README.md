# testdata

Fixture packages for the test suite.

Fixtures are generated, not downloaded, so the tests never depend on a network
or on someone else's release remaining available. Real `.deb` files are
deliberately not committed: a repository whose fixtures are hundred-megabyte
binaries is unpleasant to clone, and a fixture that large rarely tests anything
a small synthetic package does not.

The generator is [`internal/debtest`](../internal/debtest), which builds a
complete `.deb` — `ar` framing, `control.tar` and `data.tar` — in memory. It
covers all four control-archive encodings, so every test run exercises `xz` and
`zstd` regardless of what is installed on the machine running it.

Because a builder and a reader written together can be wrong in the same way,
correctness is established against `dpkg-deb` rather than against the fixtures:
see [decision 0013](../docs/decisions/0013-deb-decompression-support.md).

This directory holds no committed fixtures today. It stays because the
round-trip test that proves M1 writes a generated package set into it.
