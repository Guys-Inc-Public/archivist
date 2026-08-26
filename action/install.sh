#!/usr/bin/env bash
#
# Installs the archivist binary onto the runner and puts it on PATH.
#
# Kept as a script rather than inline YAML so it can be run and tested locally.
set -euo pipefail

version="${ARCHIVIST_VERSION:-latest}"
repo="Guys-Inc-Public/archivist"

case "$(uname -m)" in
  x86_64)          arch="amd64" ;;
  aarch64|arm64)   arch="arm64" ;;
  armv7l)          arch="armv7" ;;
  *) echo "::error::unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

case "$(uname -s)" in
  Linux)  os="linux"  ;;
  Darwin) os="darwin" ;;
  *) echo "::error::unsupported operating system $(uname -s)" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  echo "::warning::Pin 'version' to a release tag. 'latest' makes builds non-reproducible."
  path="latest/download"
else
  path="download/${version}"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

base="https://github.com/${repo}/releases/${path}"
archive="archivist_${version#v}_${os}_${arch}.tar.gz"

echo "Downloading ${archive}"
curl -fsSL --retry 3 -o "${tmp}/archive.tar.gz" "${base}/${archive}"
curl -fsSL --retry 3 -o "${tmp}/checksums.txt" "${base}/checksums.txt"

# Verify before executing. A tool that publishes signed repositories has no
# business installing itself unverified.
( cd "$tmp" && mv archive.tar.gz "$archive" \
  && sha256sum --check --ignore-missing checksums.txt )

tar -xzf "${tmp}/${archive}" -C "$tmp" archivist
install -m 0755 "${tmp}/archivist" "${RUNNER_TEMP:-/tmp}/archivist"
echo "${RUNNER_TEMP:-/tmp}" >> "$GITHUB_PATH"

"${RUNNER_TEMP:-/tmp}/archivist" version
