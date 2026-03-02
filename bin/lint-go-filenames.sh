#!/usr/bin/env bash
#
# Checks that the given files conform to our Go file naming conventions.
# Accepts filenames. Directories are recursively traversed for files.
# If no arguments are given, the script defaults to the repo root, i.e.
# checks all files in the repo.
#
# Filenames which do not conform to our conventions are printed.
# Exits with status 1 if any files do not conform; otherwise, 0.

set -o errexit -o nounset -o pipefail
shopt -s extglob

# Go filenames should be lowercase without underscores except for some exceptions
files=("$@")
if [[ ${#files[@]} -eq 0 ]]; then
  files=("$(git rev-parse --show-toplevel)")
fi

# these directories are excepted from linting
dir_exceptions=(
  third_party # we don't control 3rd party
)

check_go_file_has_no_invalid_chars() {
  local filename=$1
  if [[ $filename != *.go ]]; then
    return
  fi
  for dir in "${dir_exceptions[@]}"; do
    if [[ $filename =~ $dir/* ]]; then
      return
    fi
  done
  local basename
  basename=$(basename "$filename" .go)
  basename=${basename%%_@(test|internal_test|integration_test|gen|generated.deepcopy|rbac|darwin|linux|windows|amd64|arm64)} # remove accepted suffixes
  if [[ $basename == *[^a-z0-9]* ]]; then
    echo "$filename"
    return 1
  fi
}

exit_code=0
while read -r filename; do
  if ! check_go_file_has_no_invalid_chars "$filename"; then
    exit_code=1
  fi
done < <(git ls-files -c -m "${files[@]}")

exit $exit_code
