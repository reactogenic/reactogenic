#!/usr/bin/env bash
# Regenerate a patch that adds one whole file to the vendored tsgo module
# (e.g. the rtsx bridge) from that file's current content.
#
#   go/scripts/file-patch.sh rtsx/rtsx.go 0001-rtsx-bridge.patch
set -euo pipefail

file="${1:?usage: $0 <path inside third_party/tsgo> <patch name>}"
patch="${2:?usage: $0 <path inside third_party/tsgo> <patch name>}"
go_dir="$(cd "$(dirname "$0")/.." && pwd)"

cd "$go_dir/third_party/tsgo"
{ git diff --no-index --no-prefix /dev/null "$file" || true; } |
  sed -e "1s#.*#diff --git a/$file b/$file#" -e "s#^+++ $file#+++ b/$file#" \
  > "$go_dir/patches/$patch"
