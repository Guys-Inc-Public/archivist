# 0011 — `Valid-Until` is opt-in and unset by default

**Status:** Accepted · **Date:** 2026-08-27

## Context

`Release` may carry a `Valid-Until` field. When present, `apt` refuses an index
older than that date. It defends against a freeze attack, where someone serves
an old but validly signed index to hide a security update — the signature checks
out, because it genuinely was signed by the right key, just not recently.

The cost is that a repository which stops publishing **breaks on a timer**. Not
degrades: `apt update` fails outright for every user.

For the projects this tool targets — small ones, publishing when there is
something to publish — pauses between releases are normal rather than a signal
that something is wrong. A default that converts a quiet repository into a
broken one punishes exactly the behaviour we should expect.

## Decision

Support `Valid-Until` as a configuration field. **Leave it unset by default.**

## Consequences

- A repository that goes quiet keeps working. Nobody is broken by inactivity.
- Repositories that want freeze protection can have it, and projects publishing
  on a predictable cadence should turn it on.
- The trade is documented next to subkey expiry in
  [Signing Keys](../Signing-Keys.md), because they are the same class of
  problem: a timer that is invisible until it fires, and whose failure cannot be
  fixed through the channel it breaks.
- Without it, an attacker who can serve stale content can withhold updates. That
  is a real weakness and it is accepted knowingly. Detecting it is one of the
  things `verify` can do out of band, which is a better place for the check than
  a deadline baked into every client.
