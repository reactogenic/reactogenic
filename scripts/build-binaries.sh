#!/usr/bin/env bash
# Cross-compile the `reactogenic` binary and generate one npm package per
# platform in dist/npm/cli-<platform>-<arch> (RGP1-092). Binaries are never
# committed; this runs at release time.
#
#   scripts/build-binaries.sh [version] [platform-arch ...]
#   scripts/build-binaries.sh 0.1.0 darwin-arm64 linux-x64
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:-0.0.0}"
shift || true
targets=("$@")
[[ ${#targets[@]} -gt 0 ]] || targets=(darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-arm64 win32-x64)

for target in "${targets[@]}"; do
  platform="${target%-*}" arch="${target#*-}"
  goos="$platform"; [[ "$platform" == win32 ]] && goos=windows
  goarch="$arch"; [[ "$arch" == x64 ]] && goarch=amd64
  exe=reactogenic; [[ "$platform" == win32 ]] && exe=reactogenic.exe
  out="$root/dist/npm/cli-$target"
  mkdir -p "$out/bin"
  (cd "$root/go" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$out/bin/$exe" ./cmd/reactogenic)
  # The binary holds tsgo (Apache-2.0) as well as our code (MIT).
  cp "$root/LICENSE" "$out/LICENSE"
  cp "$root/go/third_party/tsgo/LICENSE" "$out/LICENSE-typescript-go"
  cp "$root/go/third_party/tsgo/NOTICE.txt" "$out/NOTICE-typescript-go.txt"
  cat > "$out/package.json" <<JSON
{
  "name": "@reactogenic/cli-$target",
  "version": "$version",
  "description": "The reactogenic binary for $platform $arch",
  "license": "MIT AND Apache-2.0",
  "repository": { "type": "git", "url": "git+https://github.com/reactogenic/reactogenic.git" },
  "os": ["$platform"],
  "cpu": ["$arch"],
  "preferUnplugged": true,
  "files": ["bin", "LICENSE-typescript-go", "NOTICE-typescript-go.txt"],
  "publishConfig": { "access": "public" }
}
JSON
  echo "built $out/bin/$exe"
done

# The stripped sizes: a growth of more than 10% over the previous release
# needs a note in specs/phase01/decisions.md (plan.md, RGP1-113).
for target in "${targets[@]}"; do
  exe=reactogenic; [[ "$target" == win32-* ]] && exe=reactogenic.exe
  printf '%-12s %5.1f MB\n' "$target" "$(echo "$(wc -c < "$root/dist/npm/cli-$target/bin/$exe") / 1048576" | bc -l)"
done
