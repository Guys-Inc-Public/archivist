# 0014 — `archivist` does not generate `index.html`

**Status:** Accepted · **Date:** 2026-08-27

**Amends:** [0008](0008-tool-owns-install-snippet.md), which said the tool would
publish "a minimal `index.html` so that a bare bucket is not a dead end". The
rest of 0008 stands: `archivist` still emits the install snippet as text.

## Context

`apt` never fetches `/`. A client is given a base URL and goes straight to
`dists/<codename>/InRelease`, so the landing page has no part in the trust chain
and no part in installation. It exists for a person who pastes the bucket URL
into a browser.

Two things about the reference deployment make generating one a mistake rather
than a nicety.

**The bucket is shared.** `apt.guysinc.pub` is a hub: each project publishes
under its own key prefix, and the bucket root holds a page and a key that belong
to all of them. A tool that writes `index.html` writes it at the root of what it
was pointed at. Pointed at a prefix it produces a page nobody visits; pointed at
the root it overwrites a page it did not author and cannot reconstruct.

**A landing page is a website.** The moment we generate one, its wording,
styling and accessibility are our problem, and changing them means a release of
a packaging tool. That is a support burden accepted in exchange for a file that
`apt` never reads.

The counter-argument is the one 0008 made: a bare bucket URL returning nothing
is an unfriendly dead end for someone who found the URL in a `sources.list` line
and wondered what it was. That is real, but it is a hosting concern with a
hosting answer.

## Decision

**`archivist` never generates `index.html`.** The published tree contains
`dists/`, `pool/`, `public.asc` and `public.gpg`, and nothing else.

Where a bare URL should lead is left to whoever owns the domain. For the
reference deployment that is a redirect rule at the edge, sending
`https://apt.guysinc.pub/` to the project's own install page, scoped to the
root path so that `dists/` and `pool/` are untouched.

## Consequences

- The tool writes only files the archive format defines, plus the public key
  that the format expects to be distributed out of band. Everything it publishes
  has a reader.
- A repository can be published into a prefix of a bucket it shares with others
  without any risk of clobbering what is at the root. That is what makes the
  multi-tenant layout safe, and it becomes load-bearing when `publish` starts
  removing keys the new tree no longer contains.
- Users who want a landing page write one. It cannot drift out of date with the
  repository, because the thing that would drift — the install snippet — is
  printed by the tool on every run, per 0008.
- A bare bucket URL returns whatever the host is configured to return. Where
  that is a 404, it is a worse first impression than a generated page would
  have been. Accepted: the audience for that URL is small, and the fix belongs
  to the person who owns the domain rather than to every user of the tool.
