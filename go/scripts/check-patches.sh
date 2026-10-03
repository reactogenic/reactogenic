#!/usr/bin/env bash
# Offline check of the vendored tree against the patches: every file under
# third_party/tsgo is either upstream's, byte for byte (go/patches/
# UPSTREAM.sha256), or listed in exactly one patch — and every upstream file
# is there unless a patch lists it. A fork file in no patch would be lost by
# the next re-vendor; this finds it without fetching anything. (That the
# patches reproduce the tree is CI's `vendor` job.)
set -euo pipefail

go_dir="$(cd "$(dirname "$0")/.." && pwd)"
tsgo="$go_dir/third_party/tsgo"
manifest="$go_dir/patches/UPSTREAM.sha256"

owned="$(sed -n 's#^diff --git a/\(.*\) b/.*#\1#p' "$go_dir"/patches/*.patch | sort)"
twice="$(echo "$owned" | uniq -d)"
[[ -z "$twice" ]] || { echo "listed in more than one patch: $twice" >&2; exit 1; }

status=0
actual="$(cd "$tsgo" && find . -type f | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r f; do printf '%s  %s\n' "$(shasum -a 256 "$f" | cut -d' ' -f1)" "$f"; done)"
# In the tree and not as upstream has it.
while IFS= read -r line; do
  f="${line#*  }"
  grep -qxF "$f" <<< "$owned" && continue
  grep -qxF "$line" "$manifest" && continue
  if grep -q "  $f\$" "$manifest"; then echo "changed, and in no patch: $f" >&2; else echo "not upstream's, and in no patch: $f" >&2; fi
  status=1
done <<< "$actual"
# Upstream's and gone.
while IFS= read -r line; do
  f="${line#*  }"
  [[ -f "$tsgo/$f" ]] || grep -qxF "$f" <<< "$owned" || { echo "missing, and in no patch: $f" >&2; status=1; }
done < "$manifest"
[[ $status -eq 0 ]] || { echo "fix with go/scripts/regen-patch.sh <patch> <file>" >&2; exit 1; }
