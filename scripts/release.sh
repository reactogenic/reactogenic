#!/usr/bin/env bash
# Pack and publish the npm packages (RGP1-093) and the VS Code extension
# (RGP1-113). A release, in this order: the version in packages/{cli,core,
# vite}/package.json (cli's optionalDependencies too) and CHANGELOG.md; then
#
#   scripts/release.sh pack <version>      build and pack all nine into dist/release/
#   scripts/release.sh publish <otp>       publish dist/release/*.tgz, binaries first
#   scripts/release.sh latest <otp>        while there is no stable release: `latest` follows the prerelease
#   pnpm install                           the lockfile: cli's platform packages exist only now. Wait for
#                                          `npm view` to show all six: one that lags is left out silently
#   scripts/release.sh vsix                the seven .vsix in dist/vsix/, from pack's binaries
#   scripts/release.sh publish-vsix [marketplace|openvsx]
#                                          dist/vsix/*.vsix to the Marketplace ($VSCE_PAT) and Open VSX ($OVSX_PAT);
#                                          the tokens may be in .env.release, which git ignores
#
# pack checks that every package.json carries <version>; publish uploads the
# tarballs pack made, so what was tested is what is published. The dist-tag
# is the prerelease id (`0.1.0-alpha.0` → alpha), else latest.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
release="$root/dist/release"
platforms=(darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-arm64 win32-x64)

case "${1:-}" in
pack)
  version="${2:?version}"
  for pkg in cli core vite; do
    got="$(node -p "require('$root/packages/$pkg/package.json').version")"
    [[ "$got" == "$version" ]] || { echo "packages/$pkg is $got, not $version" >&2; exit 1; }
  done
  rm -rf "$release" "$root/dist/npm"
  mkdir -p "$release"
  "$root/scripts/build-binaries.sh" "$version" "${platforms[@]}"
  for target in "${platforms[@]}"; do
    (cd "$root/dist/npm/cli-$target" && npm pack --silent --pack-destination "$release" >/dev/null)
  done
  for pkg in core vite; do
    (cd "$root/packages/$pkg" && rm -rf dist && pnpm -s build)
  done
  for pkg in cli core vite; do
    cp "$root/LICENSE" "$root/packages/$pkg/LICENSE"
    (cd "$root/packages/$pkg" && pnpm pack --pack-destination "$release" >/dev/null)
  done
  ls -1 "$release"
  ;;
publish)
  otp="${2:?one-time password}"
  version="$(node -p "require('$root/packages/cli/package.json').version")"
  tag=latest
  [[ "$version" == *-* ]] && tag="$(sed -E 's/^[^-]+-([A-Za-z]+).*/\1/' <<<"$version")"
  order=()
  for target in "${platforms[@]}"; do order+=("reactogenic-cli-$target-$version.tgz"); done
  order+=("reactogenic-cli-$version.tgz" "reactogenic-core-$version.tgz" "reactogenic-vite-$version.tgz")
  for tgz in "${order[@]}"; do
    [[ -f "$release/$tgz" ]] || { echo "missing $tgz: run pack first" >&2; exit 1; }
  done
  # A one-time password lasts about 30 seconds: a rerun with a fresh one
  # skips what is already on the registry. The public view of a new package
  # lags behind its publish, so a name the org owns with no public versions
  # yet counts as just published; the registry's refusal is the last check.
  owned="$(npm access list packages @reactogenic 2>/dev/null || true)"
  for tgz in "${order[@]}"; do
    name="$(tar xzf "$release/$tgz" -O package/package.json | node -p 'JSON.parse(require("fs").readFileSync(0)).name')"
    published="$(npm view "$name@$version" version 2>/dev/null || true)"
    if [[ "$published" == "$version" ]] || { grep -q "^$name:" <<<"$owned" && [[ -z "$(npm view "$name" versions 2>/dev/null || true)" ]]; }; then
      echo "skip $name@$version: already published"
      continue
    fi
    if out="$(npm publish "$release/$tgz" --tag "$tag" --access public --otp "$otp" 2>&1)"; then
      grep '^+ ' <<<"$out"
    elif grep -q "cannot publish over the previously published version" <<<"$out"; then
      echo "skip $tgz: already published"
    else
      grep -v '^npm notice' <<<"$out" >&2
      exit 1
    fi
  done
  ;;
latest)
  # `npm install @reactogenic/cli` without a tag takes `latest`, which the
  # first publish set and a tagged one does not move.
  otp="${2:?one-time password}"
  version="$(node -p "require('$root/packages/cli/package.json').version")"
  for name in "${platforms[@]/#/cli-}" cli core vite; do
    npm dist-tag add "@reactogenic/$name@$version" latest --otp "$otp"
  done
  ;;
vsix)
  # The binaries pack built — the ones on npm — never rebuilt.
  version="$(node -p "require('$root/packages/cli/package.json').version")"
  for target in "${platforms[@]}"; do
    [[ "$(node -p "require('$root/dist/npm/cli-$target/package.json').version" 2>/dev/null)" == "$version" ]] ||
      { echo "dist/npm/cli-$target is not $version: run pack first" >&2; exit 1; }
  done
  rm -rf "$root/dist/vsix"
  (cd "$root/packages/vscode" && node scripts/package.mjs --pre-release && node scripts/smoke-vsix.mjs "$root"/dist/vsix/*.vsix)
  ;;
publish-vsix)
  # The tokens: from the environment, else from .env.release (gitignored).
  if [[ -f "$root/.env.release" ]]; then set -a; . "$root/.env.release"; set +a; fi
  where="${2:-both}"
  [[ "$where" == both || "$where" == marketplace || "$where" == openvsx ]] || { echo "publish-vsix [marketplace|openvsx]" >&2; exit 2; }
  version="$(node -p "require('$root/packages/vscode/package.json').version")"
  files=()
  for target in "${platforms[@]}" universal; do
    [[ -f "$root/dist/vsix/rtsx-$target-$version.vsix" ]] || { echo "missing rtsx-$target-$version.vsix: run vsix first" >&2; exit 1; }
    files+=("$root/dist/vsix/rtsx-$target-$version.vsix")
  done
  # 0.1.x is a pre-release line: the Marketplace has no pre-release tags, a
  # .vsix is one or is not (package.mjs --pre-release) — both stores read it
  # from the file.
  if [[ "$where" != openvsx ]]; then
    # Without a token (it needs an Azure subscription): upload the seven at
    # marketplace.visualstudio.com/manage — the universal one first.
    : "${VSCE_PAT:?the Marketplace token of the publisher reactogenic}"
    (cd "$root/packages/vscode" && pnpm exec vsce publish --pre-release --packagePath "${files[@]}")
  fi
  if [[ "$where" != marketplace ]]; then
    : "${OVSX_PAT:?the Open VSX token of the namespace reactogenic}"
    export OVSX_PAT
    for file in "${files[@]}"; do
      pnpm dlx ovsx publish "$file"
    done
  fi
  ;;
*)
  sed -n '2,17p' "$0" >&2
  exit 2
  ;;
esac
