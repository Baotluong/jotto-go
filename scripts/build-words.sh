#!/usr/bin/env bash
# Downloads the word-list sources and regenerates internal/words/data.
# Sources are fetched into a temp dir and never committed.
set -euo pipefail

SCOWL_VERSION=2020.12.07
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL -o "$TMP/scowl.tar.gz" \
  "https://downloads.sourceforge.net/project/wordlist/SCOWL/${SCOWL_VERSION}/scowl-${SCOWL_VERSION}.tar.gz"
tar xzf "$TMP/scowl.tar.gz" -C "$TMP"

# LDNOOBW English list (CC BY 4.0), used only to exclude words.
curl -fsSL -o "$TMP/blocklist.txt" \
  "https://raw.githubusercontent.com/LDNOOBW/List-of-Dirty-Naughty-Obscene-and-Otherwise-Bad-Words/master/en"

cd "$ROOT"
go run ./cmd/wordgen \
  -scowl "$TMP/scowl-${SCOWL_VERSION}/final" \
  -blocklist "$TMP/blocklist.txt" \
  -out internal/words/data
