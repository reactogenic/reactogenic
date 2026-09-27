#!/usr/bin/env bash
# Regenerate a patch from the current state of the vendored files it touches,
# diffed against pristine upstream (the tree synced without that patch).
#
#   go/scripts/regen-patch.sh 0002-rtsx-parser.patch internal/parser/parser.go internal/parser/rtsx.go
set -euo pipefail

patch="${1:?usage: $0 <patch> <path inside third_party/tsgo>...}"
shift
go_dir="$(cd "$(dirname "$0")/.." && pwd)"
tsgo="$go_dir/third_party/tsgo"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

for f in "$@"; do mkdir -p "$tmp/b/$(dirname "$f")"; cp "$tsgo/$f" "$tmp/b/$f"; done
mv "$go_dir/patches/$patch" "$tmp/$patch"
"$go_dir/scripts/sync-tsgo.sh" >/dev/null || { mv "$tmp/$patch" "$go_dir/patches/$patch"; exit 1; }
for f in "$@"; do
  mkdir -p "$tmp/a/$(dirname "$f")"
  if [[ -f "$tsgo/$f" ]]; then cp "$tsgo/$f" "$tmp/a/$f"; fi
done
: > "$go_dir/patches/$patch"
for f in "$@"; do
  a="$tmp/a/$f"; [[ -f "$a" ]] || a=/dev/null
  (cd "$tmp" && git diff --no-index --no-prefix "${a#"$tmp/"}" "b/$f" || true) |
    sed -e "1s#.*#diff --git a/$f b/$f#" -e "s#^--- a/$f#--- a/$f#" -e "s#^+++ b/$f#+++ b/$f#" \
        -e "s#^--- /dev/null#--- /dev/null#" \
    >> "$go_dir/patches/$patch"
done
for f in "$@"; do cp "$tmp/b/$f" "$tsgo/$f"; done
"$go_dir/scripts/sync-tsgo.sh" >/dev/null   # prove the patches rebuild the tree
