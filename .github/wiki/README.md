# Wiki chrome

Navigation furniture for the generated wiki: `_Sidebar.md` and `_Footer.md`.

These live here rather than in `docs/` because their links are wiki-namespace
links (`[Home](Home)`), which are meaningless inside the repository and would be
flagged by the documentation link check. `docs/` stays purely documentation.

`.github/workflows/publish-wiki.yml` copies this directory into the wiki
verbatim after rendering `docs/`.
