#!/bin/sh
# Tests for bin/extract-release-notes.sh
set -e

SCRIPT="$(dirname "$0")/extract-release-notes.sh"
PASS=0
FAIL=0

assert_eq() {
  test_name="$1"
  expected="$2"
  actual="$3"
  if [ "$expected" = "$actual" ]; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $test_name" >&2
    echo "  expected: $(echo "$expected" | head -3)" >&2
    echo "  actual:   $(echo "$actual" | head -3)" >&2
  fi
}

assert_fails() {
  test_name="$1"
  shift
  if "$@" > /dev/null 2>&1; then
    FAIL=$((FAIL + 1))
    echo "FAIL: $test_name (expected non-zero exit)" >&2
  else
    PASS=$((PASS + 1))
  fi
}

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# --- Test: extracts correct version section ---
cat > "$TMPDIR/changelog.md" << 'EOF'
# Changelog

## [2.15.0] - 2026-03-23

### Added

- Feature A
- Feature B

### Fixed

- Bug fix C

## [2.14.0] - 2026-03-13

### Added

- Feature D
EOF

RESULT=$(sh "$SCRIPT" "2.15.0" "$TMPDIR/changelog.md")
assert_eq "extracts correct version" "### Added

- Feature A
- Feature B

### Fixed

- Bug fix C
" "$RESULT
"

# --- Test: extracts last version (no trailing heading) ---
RESULT=$(sh "$SCRIPT" "2.14.0" "$TMPDIR/changelog.md")
assert_eq "extracts last version" "### Added

- Feature D" "$RESULT"

# --- Test: fails on missing version ---
assert_fails "missing version exits non-zero" sh "$SCRIPT" "9.9.9" "$TMPDIR/changelog.md"

# --- Test: fails on empty changelog ---
cat > "$TMPDIR/empty.md" << 'EOF'
# Changelog
EOF
assert_fails "empty changelog exits non-zero" sh "$SCRIPT" "1.0.0" "$TMPDIR/empty.md"

# --- Test: fails on missing file ---
assert_fails "missing file exits non-zero" sh "$SCRIPT" "1.0.0" "$TMPDIR/nonexistent.md"

# --- Test: dots are literal, not regex wildcards ---
cat > "$TMPDIR/tricky.md" << 'EOF'
## [2X15Y0] - 2026-01-01

### Added

- Should not match

## [2.15.0] - 2026-03-23

### Added

- Correct match
EOF

RESULT=$(sh "$SCRIPT" "2.15.0" "$TMPDIR/tricky.md")
assert_eq "dots are literal" "### Added

- Correct match" "$RESULT"

# --- Test: no args ---
assert_fails "no args exits non-zero" sh "$SCRIPT"

# --- Summary ---
echo "${PASS} passed, ${FAIL} failed"
[ "$FAIL" -eq 0 ]
