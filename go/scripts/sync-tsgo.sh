#!/usr/bin/env bash
# Vendor the tsc/ Go module of microsoft/TypeScript into go/third_party/tsgo
# at a pinned commit, then apply go/patches/*.patch in order.
#
#   go/scripts/sync-tsgo.sh <commit>    re-vendor at <commit>
#   go/scripts/sync-tsgo.sh             re-vendor at the commit in UPSTREAM
set -euo pipefail

go_dir="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$go_dir/.." && pwd)"
dest="$go_dir/third_party/tsgo"
commit="${1:-$(sed -n 's/^commit: //p' "$dest/UPSTREAM" 2>/dev/null || true)}"
[[ -n "$commit" ]] || { echo "usage: $0 <commit>" >&2; exit 2; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Fetch only the pinned commit, only tsc/: no history, no other blobs.
git init -q "$tmp/ts"
git -C "$tmp/ts" remote add origin https://github.com/microsoft/TypeScript.git
git -C "$tmp/ts" sparse-checkout set tsc
git -C "$tmp/ts" fetch -q --depth 1 --filter=blob:none origin "$commit"
git -C "$tmp/ts" checkout -q FETCH_HEAD

rm -rf "$dest"
mkdir -p "$dest"
rsync -a --exclude-from="$go_dir/scripts/tsgo-exclude.txt" "$tmp/ts/tsc/" "$dest/"
cp "$tmp/ts/NOTICE.txt" "$dest/NOTICE.txt"
{
  echo "repository: https://github.com/microsoft/TypeScript"
  echo "path: tsc"
  echo "commit: $(git -C "$tmp/ts" rev-parse HEAD)"
  echo "date: $(git -C "$tmp/ts" log -1 --format=%cs)"
} > "$dest/UPSTREAM"

# The manifest of pristine upstream: what regen-patch.sh and check-patches.sh
# compare the tree with, offline.
(cd "$dest" && find . -type f | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r f; do
  printf '%s  %s\n' "$(shasum -a 256 "$f" | cut -d' ' -f1)" "$f"
done) > "$go_dir/patches/UPSTREAM.sha256"

shopt -s nullglob
for patch in "$go_dir"/patches/*.patch; do
  echo "applying $(basename "$patch")"
  git -C "$repo_root" apply --directory=go/third_party/tsgo "$patch"
done
