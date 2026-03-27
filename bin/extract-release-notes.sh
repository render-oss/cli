#!/bin/sh
# Extracts the release notes for a given version from CHANGELOG.md.
# Usage: bin/extract-release-notes.sh <version> <changelog-file>
# Example: bin/extract-release-notes.sh 2.15.0 CHANGELOG.md
#
# Prints the extracted notes to stdout. Exits non-zero if no entry is found.

set -e

VERSION="${1:?Usage: extract-release-notes.sh <version> <changelog-file>}"
CHANGELOG="${2:?Usage: extract-release-notes.sh <version> <changelog-file>}"

if [ ! -f "$CHANGELOG" ]; then
  echo "Error: file not found: $CHANGELOG" >&2
  exit 1
fi

# Escape dots for regex (2.15.0 -> 2\.15\.0)
VERSION_RE=$(echo "$VERSION" | sed 's/\./\\./g')

# Extract content between this version's heading and the next ## heading,
# stripping leading and trailing blank lines.
NOTES=$(awk "
  /^## \\[${VERSION_RE}\\]/ { found=1; next }
  /^## \\[/ { if (found) exit }
  found { lines[++n] = \$0 }
  END {
    # Trim leading blank lines
    start = 1
    while (start <= n && lines[start] ~ /^[[:space:]]*$/) start++
    # Trim trailing blank lines
    end = n
    while (end >= start && lines[end] ~ /^[[:space:]]*$/) end--
    for (i = start; i <= end; i++) print lines[i]
  }
" "$CHANGELOG")

if [ -z "$(echo "$NOTES" | tr -d '[:space:]')" ]; then
  echo "Error: no CHANGELOG.md entry found for version ${VERSION}" >&2
  exit 1
fi

printf '%s\n' "$NOTES"
