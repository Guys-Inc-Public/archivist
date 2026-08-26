# 0008 — `archivist` generates the install snippet

**Status:** Accepted · **Date:** 2026-08-26

## Context

Users of a published repository need a few lines to paste: fetch the key, write
the `sources.list` entry, update, install. In the reference project those lines
are hand-written on a landing page.

That snippet hardcodes four things — the base URL, the keyring filename, the
suite, and the component. All four are values the publishing tool already holds
in its configuration. Anything that restates them by hand drifts, and the
failure mode is a copy-pasteable install command that does not work: the worst
possible defect for a project's adoption path, because it fails for new users
and nobody else, and new users mostly do not report it.

The counter-argument is that presentation is not a packaging tool's job.

## Decision

**`archivist` emits the install snippet, with the fingerprint and URL filled in,
as part of publish output.**

It emits the snippet, not a web page. Where that text is displayed remains the
user's concern; `archivist` publishes a minimal `index.html` containing it so
that a bare bucket is not a dead end, and that page is replaceable.

## Consequences

- Install instructions cannot drift from the repository they describe.
- The fingerprint reaches users' eyes from the same run that signed the
  repository, which supports publishing it in more than one place — see
  [decision 0002](0002-signing-key-handling.md).
- We now have opinions about `apt` client configuration: the snippet uses
  `signed-by=` with a dearmoured key in `/etc/apt/keyrings`, rather than the
  deprecated `apt-key`. That is correct for currently supported Debian and
  Ubuntu releases and wrong for genuinely old ones, which we do not support.
- Generated presentation output is a category we said we did not want. The
  boundary is deliberate and narrow: text, not design.
