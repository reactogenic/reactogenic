#!/usr/bin/env bash
# Write a patch from the working tree: the files the patch lists (committed
# or not), plus any named here, diffed against pristine upstream. Offline and
# non-destructive — it never re-vendors and never touches third_party/tsgo.
#
#   go/scripts/regen-patch.sh 0002-rtsx-parser.patch                  regenerate
#   go/scripts/regen-patch.sh 0002-rtsx-parser.patch internal/ast/x.go  add a file
#   go/scripts/regen-patch.sh 0004-new.patch internal/module/resolver.go  a new patch
#
# Pristine upstream: a file that is not in go/patches/UPSTREAM.sha256 (the
# manifest sync-tsgo.sh writes) is ours, and its pristine side is empty; a
# file that is, is HEAD's copy minus the committed patch, and must then hash
# as the manifest says. One patch owns a given upstream file (README.md).
# It ends by checking the whole tree (check-patches.sh).
set -euo pipefail

patch="${1:?usage: $0 <patch> [path inside third_party/tsgo]...}"
shift
go_dir="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$go_dir/.." && pwd)"
tsgo="$go_dir/third_party/tsgo"
manifest="$go_dir/patches/UPSTREAM.sha256"
out="$go_dir/patches/$patch"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

listed() { sed -n 's#^diff --git a/\(.*\) b/.*#\1#p' "$1"; }
upstream() { grep -q "  $1\$" "$manifest"; }
hash_of() { shasum -a 256 "$1" | cut -d' ' -f1; }

# The patch as committed and as it is in the working tree: both name files.
git -C "$repo_root" show "HEAD:go/patches/$patch" > "$tmp/old.patch" 2>/dev/null || rm -f "$tmp/old.patch"
files="$( { [[ -f "$tmp/old.patch" ]] && listed "$tmp/old.patch"; [[ -f "$out" ]] && listed "$out"; printf '%s\n' "$@"; } | sed '/^$/d' | sort -u)"
[[ -n "$files" ]] || { echo "$patch: no files" >&2; exit 2; }

# One patch per file.
for other in "$go_dir"/patches/*.patch; do
  [[ "$(basename "$other")" == "$patch" ]] && continue
  clash="$(comm -12 <(listed "$other" | sort) <(echo "$files"))"
  [[ -z "$clash" ]] || { echo "$(basename "$other") already owns: $clash — fold the change into it" >&2; exit 1; }
done

# a/: pristine. b/: the working tree.
mkdir -p "$tmp/a" "$tmp/b" "$tmp/head"
while IFS= read -r f; do
  if [[ ! -f "$tsgo/$f" ]] && ! upstream "$f"; then
    echo "$patch: $f is neither in the working tree nor upstream (a mistyped path?)" >&2; exit 1
  fi
  mkdir -p "$tmp/a/$(dirname "$f")" "$tmp/b/$(dirname "$f")" "$tmp/head/$(dirname "$f")"
  [[ -f "$tsgo/$f" ]] && cp "$tsgo/$f" "$tmp/b/$f"
  git -C "$repo_root" show "HEAD:go/third_party/tsgo/$f" > "$tmp/head/$f" 2>/dev/null || rm -f "$tmp/head/$f"
done <<< "$files"
# HEAD's copies minus the committed patch: the pristine side of upstream files.
if [[ -f "$tmp/old.patch" ]]; then
  for f in $(listed "$tmp/old.patch"); do [[ -f "$tmp/head/$f" ]] || { mkdir -p "$tmp/head/$(dirname "$f")"; git -C "$repo_root" show "HEAD:go/third_party/tsgo/$f" > "$tmp/head/$f" 2>/dev/null || rm -f "$tmp/head/$f"; }; done
  (cd "$tmp/head" && git apply -R --unsafe-paths "$tmp/old.patch") ||
    { echo "$patch does not reverse against HEAD: is HEAD a tree the patches reproduce?" >&2; exit 1; }
fi
while IFS= read -r f; do
  upstream "$f" || continue # ours: pristine is empty
  [[ -f "$tmp/head/$f" ]] || { echo "$patch: no pristine copy of $f at HEAD" >&2; exit 1; }
  want="$(grep "  $f\$" "$manifest" | cut -d' ' -f1)"
  [[ "$(hash_of "$tmp/head/$f")" == "$want" ]] ||
    { echo "$patch: $f at HEAD, minus the committed patch, is not upstream's file (another uncommitted or unowned change?)" >&2; exit 1; }
  cp "$tmp/head/$f" "$tmp/a/$f"
done <<< "$files"

(cd "$tmp" && git diff --no-index --no-prefix a b || true) |
  perl -pe 's{^diff --git [ab]/(.*) b/\1$}{diff --git a/$1 b/$1}' > "$tmp/new.patch"
[[ -s "$tmp/new.patch" ]] || { echo "$patch: the working tree equals upstream for these files" >&2; exit 1; }
missing="$(comm -23 <(echo "$files") <(listed "$tmp/new.patch" | sort))"
[[ -z "$missing" ]] || { echo "$patch: no difference from upstream in: $missing" >&2; exit 1; }

# Prove it: pristine plus the new patch is the working tree.
cp -R "$tmp/a" "$tmp/check"
(cd "$tmp/check" && git apply --unsafe-paths "$tmp/new.patch")
diff -r "$tmp/check" "$tmp/b" > /dev/null || { echo "$patch: generated patch does not reproduce the working tree" >&2; exit 1; }

cp "$tmp/new.patch" "$out"
echo "$patch: $(listed "$out" | wc -l | tr -d ' ') file(s)"
"$go_dir/scripts/check-patches.sh"
