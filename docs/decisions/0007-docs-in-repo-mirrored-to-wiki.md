# 0007 — `docs/` is the source of truth; the wiki is generated

**Status:** Accepted · **Date:** 2026-08-26

## Context

The organisation wants a wiki on its repositories, and a uniform documentation
standard across all of them.

A GitHub wiki is a separate git repository with no pull requests, no review, no
required status checks, and no presence in code search. Documentation written
there cannot be updated in the same change as the code it describes, so it
drifts — and drifted documentation is worse than none, because it is trusted.

Documentation in `docs/` has the opposite trade-off: reviewable, versioned,
searchable, and changeable in the same commit as the behaviour it documents —
but it is not a wiki, and it is less inviting to browse.

## Decision

**Write documentation in `docs/`. Generate the wiki from it on every merge to
`main`.**

`.github/workflows/publish-wiki.yml` mirrors `docs/` into the wiki. Filenames
map to page titles; nested directories flatten with a hyphen, since the wiki has
no directory concept. Relative links are rewritten to flat wiki links. Every
generated `Home` page carries a banner saying where the content came from.

## Consequences

- Documentation is reviewed like code and can never contradict the code it ships
  with, because it ships with it.
- The wiki still exists for people who prefer to browse one.
- **Wiki edits are silently destroyed.** This is the sharp edge. It is mitigated
  with the banner on every generated `Home` page and a warning in
  [CONTRIBUTING.md](https://github.com/Guys-Inc-Public/archivist/blob/main/CONTRIBUTING.md),
  but someone will still lose an edit eventually.
- Link rewriting is a translation step that can be wrong in ways that are
  invisible in the repository. CI link-checks `docs/` on every pull request.
- This is the organisation-wide standard, so the workflow is written to be
  copied without modification into any Guys Inc repository.
