#!/usr/bin/env python3
"""Render docs/ into a flat set of GitHub wiki pages.

The wiki has no directory concept and no relative-path resolution, so every
link has to be rewritten. Doing that with a handful of sed expressions looks
adequate and silently misses forms it was not written for, so links are
resolved properly instead: each one is interpreted relative to its source file,
matched against the known page set, and rewritten to the flat page name.

A link that cannot be resolved is an error. A broken link in a wiki is
invisible until a reader hits it, which is exactly the failure this replaces.

Usage: render.py <docs-dir> <output-dir> <repo-slug>
"""

from __future__ import annotations

import posixpath
import re
import sys
from pathlib import Path

LINK = re.compile(r"(?<!\!)\[([^\]]*)\]\(([^)\s]+?)(#[^)\s]*)?\)")
EXTERNAL = ("http://", "https://", "mailto:", "#")


def page_name(rel: Path) -> str:
    """docs/decisions/0001-x.md -> decisions-0001-x"""
    return str(rel.with_suffix("")).replace("/", "-")


def main(docs: Path, out: Path, slug: str) -> int:
    sources = sorted(p for p in docs.rglob("*.md"))
    if not sources:
        print(f"error: no markdown found under {docs}", file=sys.stderr)
        return 1

    pages = {p.relative_to(docs).as_posix(): page_name(p.relative_to(docs)) for p in sources}
    repo_root = docs.parent
    errors: list[str] = []

    for src in sources:
        rel = src.relative_to(docs)
        here = rel.parent.as_posix()

        def rewrite(m: re.Match[str]) -> str:
            text, target, frag = m.group(1), m.group(2), m.group(3) or ""
            if target.startswith(EXTERNAL):
                return m.group(0)

            resolved = posixpath.normpath(posixpath.join(here, target))

            # A link within docs/ becomes a flat wiki page link.
            if resolved in pages:
                return f"[{text}]({pages[resolved]}{frag})"

            # docs/ links are conventionally written without the .md suffix in
            # some places; accept that too.
            if f"{resolved}.md" in pages:
                return f"[{text}]({pages[resolved + '.md']}{frag})"

            # A link to something else in the repository has to become absolute,
            # because the wiki is a different repository entirely.
            outside = posixpath.normpath(posixpath.join("docs", here, target))
            if (repo_root / outside).exists():
                return f"[{text}](https://github.com/{slug}/blob/main/{outside}{frag})"

            errors.append(f"{rel}: cannot resolve link {target!r}")
            return m.group(0)

        body = LINK.sub(rewrite, src.read_text(encoding="utf-8"))
        dest = out / f"{pages[rel.as_posix()]}.md"
        dest.write_text(body, encoding="utf-8")
        print(f"  {rel}  ->  {dest.name}")

    if errors:
        print("\nUnresolvable links:", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        return 1

    print(f"\nRendered {len(sources)} pages.")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 4:
        print(__doc__, file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main(Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]))
