#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

# BashUnit supports installing its standalone executable into a project:
# https://bashunit.com/installation#install-sh
bashunit_version="0.50.1"
bashunit_checksum="18d83d590c5304f1853dd4fe4fec4ec6effbd9fe5a21831fe9f66f70afe17d93"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bashunit_path="$script_dir/bashunit"

checksum() {
  shasum -a 256 "$1" | cut -d ' ' -f 1
}

if [[ ! -s "$bashunit_path" || "$(checksum "$bashunit_path")" != "$bashunit_checksum" ]]; then
  curl --silent --show-error --fail --location \
    "https://github.com/TypedDevs/bashunit/releases/download/${bashunit_version}/bashunit" \
    --output "$bashunit_path"

  if [[ "$(checksum "$bashunit_path")" != "$bashunit_checksum" ]]; then
    rm -f "$bashunit_path"
    echo "BashUnit checksum verification failed" >&2
    exit 1
  fi

  chmod +x "$bashunit_path"
fi

"$bashunit_path" "$script_dir" "$@"
