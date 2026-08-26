# 0005 — The CLI is the product; the Action is a thin wrapper

**Status:** Accepted · **Date:** 2026-08-26

## Context

The tool needs to be usable in GitHub Actions, which is where the target users
already publish releases. A composite Action is the obvious distribution shape.

The trap is building the Action *first*. Logic that lives in a workflow file can
only be exercised by pushing a commit and waiting for a runner. That makes every
iteration slow, makes failures hard to reproduce, and quietly ties the tool to
one CI provider.

## Decision

**Build the CLI first. The Action is a thin wrapper over it, containing no
logic.** Both ship, sharing one core.

The CLI is the real product: it means people can run this outside GitHub, and it
means we can test without pushing commits. The Action is the adoption vector —
it is what puts the tool into other people's workflow files.

The dividing line is testability. If a behaviour cannot be exercised by running
a binary locally, it is in the wrong place. Even the Action's installation step
is a script in `action/install.sh` rather than inline YAML, so it can be run
directly.

## Consequences

- Every behaviour is reachable from a terminal, so bug reports are reproducible
  without a CI account.
- Users on GitLab CI, Jenkins, Woodpecker, or a cron job on a server are
  first-class rather than unsupported.
- Two surfaces to document and keep in step. The Action's inputs map one-to-one
  onto CLI flags to keep that cost as low as possible.
- The Action cannot do anything clever that the CLI cannot do. That is the point,
  and it will occasionally be inconvenient.
