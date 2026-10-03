#!/usr/bin/env bash
# Write a patch from the working tree: the files the patch already lists,
# plus any named here, diffed against pristine upstream. Offline and
# non-destructive — it never re-vendors and never touches third_party/tsgo.
#
#   go/scripts/regen-patch.sh 0002-rtsx-parser.patch                  regenerate
#   go/scripts/regen-patch.sh 0002-rtsx-parser.patch internal/ast/x.go  add a file
#   go/scripts/regen-patch.sh 0004-new.patch internal/module/resolver.go  a new patch
#
# Pristine upstream is rebuilt from HEAD: the committed file minus the
# committed patch. So one patch owns a given upstream file (README.md), and
# HEAD must be a tree the patches reproduce (CI's `vendor` job).
set -euo pipefail

patch="${1:?usage: $0 <patch> [path inside third_party/tsgo]...}"
shift
go_dir="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$go_dir/.." && pwd)"
tsgo="$go_dir/third_party/tsgo"
out="$go_dir/patches/$patch"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

listed() { sed -n 's#^diff --git a/\(.*\) b/.*#\1#p' "$1"; }

# The committed patch, if any: its files are part of the new one.
git -C "$repo_root" show "HEAD:go/patches/$patch" > "$tmp/old.patch" 2>/dev/null || rm -f "$tmp/old.patch"
files="$( { [[ -f "$tmp/old.patch" ]] && listed "$tmp/old.patch"; printf '%s\n' "$@"; } | sed '/^$/d' | sort -u)"
[[ -n "$files" ]] || { echo "$patch: no files" >&2; exit 2; }

# One patch per upstream file.
for other in "$go_dir"/patches/*.patch; do
  [[ "$(basename "$other")" == "$patch" ]] && continue
  clash="$(comm -12 <(listed "$other" | sort) <(echo "$files"))"
  [[ -z "$clash" ]] || { echo "$(basename "$other") already owns: $clash — fold the change into it" >&2; exit 1; }
done

# a/: pristine. HEAD's files, then the committed patch reversed.
mkdir -p "$tmp/a" "$tmp/b"
while IFS= read -r f; do
  mkdir -p "$tmp/a/$(dirname "$f")" "$tmp/b/$(dirname "$f")"
  git -C "$repo_root" show "HEAD:go/third_party/tsgo/$f" > "$tmp/a/$f" 2>/dev/null || rm -f "$tmp/a/$f"
  [[ -f "$tsgo/$f" ]] && cp "$tsgo/$f" "$tmp/b/$f"
done <<< "$files"
if [[ -f "$tmp/old.patch" ]]; then
  (cd "$tmp/a" && git apply -R --unsafe-paths "$tmp/old.patch") ||
    { echo "$patch does not reverse against HEAD: is HEAD a tree the patches reproduce?" >&2; exit 1; }
fi

# b/: the working tree. The patch is the difference.
(cd "$tmp" && git diff --no-index --no-prefix a b || true) |
  perl -pe 's{^diff --git [ab]/(.*) b/\1$}{diff --git a/$1 b/$1}' > "$tmp/new.patch"
[[ -s "$tmp/new.patch" ]] || { echo "$patch: the working tree equals upstream for these files" >&2; exit 1; }

# Prove it: pristine plus the new patch is the working tree.
cp -R "$tmp/a" "$tmp/check"
(cd "$tmp/check" && git apply --unsafe-paths "$tmp/new.patch")
diff -r "$tmp/check" "$tmp/b" > /dev/null || { echo "$patch: generated patch does not reproduce the working tree" >&2; exit 1; }

cp "$tmp/new.patch" "$out"
echo "$patch: $(echo "$files" | wc -l | tr -d ' ') file(s)"
