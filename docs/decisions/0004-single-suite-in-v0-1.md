# 0004 — One suite in `v0.1`, but `suite` is a required field

**Status:** Accepted · **Date:** 2026-08-26

## Context

Debian repositories can carry multiple suites — `stable`, `testing`,
`unstable` — and users reasonably expect a repository tool to support
pre-release channels.

Against that: the reference pipeline declares exactly one codename, and the only
channel logic anywhere in that project derives `production`, `beta` and `test`
by substring-matching the application's version string and branch name. That is
Electron-application-shaped, not repository-shaped, and there is no working
multi-suite implementation to extract.

Supporting multiple suites also has a cost beyond the code: it introduces
promotion between channels, which `v0.1` explicitly excludes, and a promotion
model chosen without a real use case is a model chosen wrong.

## Decision

**Publish a single suite in `v0.1`.** But make `suite` a **required**
configuration field from the first release, rather than defaulting it or
omitting it.

## Consequences

- The published layout is already correct for multiple suites. Adding `testing`
  alongside `stable` in `v0.2` is a configuration change, not a change to the
  path structure.
- This matters more than it sounds: the suite name appears in every user's
  `sources.list` line. A layout change after users have configured `apt` would
  break every installation, and the fix cannot be delivered through the
  mechanism that broke.
- Users who want channels now cannot have them, and will say so. That feedback
  is more useful than a guess, and it arrives attached to a real use case.
- Requiring an explicit `suite` means nobody gets a repository whose suite is
  whatever the default happened to be. Never infer a suite — see the version
  string parsing in the [extraction inventory](../Extraction-Inventory.md).
