#!/usr/bin/env bash
# `reactogenic content-mapper` against its real host (RGP1-112; ide.md,
# *Stock TypeScript 7.1*): stock `tsc --runExternalCode` from `typescript@next`
# checks a project with .rtsx files through @reactogenic/cli's manifest —
# this checkout's packages/cli, linked, and a binary built from this checkout.
#
# Needs the network (npm) and Node; CI does not run it.
#
#   scripts/e2e-stock-mapper.sh [work-dir]     default: a fresh temporary one
#
#   TYPESCRIPT=typescript@7.1.0-beta   the TypeScript to install (typescript@next)
#   REACTOGENIC_BINARY=/path/to/it     a binary to test instead of building one
#   TS_CONTENT_MAPPER_DEBUG=1          tsc prints the protocol traffic
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="${1:-$(mktemp -d)}"
mkdir -p "$work"
work="$(cd "$work" && pwd)"
project="$work/project"

if [[ -z "${REACTOGENIC_BINARY:-}" ]]; then
  export REACTOGENIC_BINARY="$work/bin/reactogenic"
  (cd "$root/go" && go build -o "$REACTOGENIC_BINARY" ./cmd/reactogenic) || exit 2
fi
echo "binary:     $REACTOGENIC_BINARY ($("$REACTOGENIC_BINARY" --version))"

rm -rf "$project"
mkdir -p "$project/src/widgets"
cd "$project" || exit 2
echo '{ "name": "e2e-stock-mapper", "private": true, "type": "module" }' > package.json
npm install --no-audit --no-fund --loglevel=error \
  "${TYPESCRIPT:-typescript@next}" @reactogenic/core@alpha @types/react react || exit 2
# The package under test: its manifest and its launcher, from this checkout.
ln -s "$root/packages/cli" node_modules/@reactogenic/cli
tsc="$project/node_modules/.bin/tsc"
echo "typescript: $("$tsc" --version)"
echo "manifest:   $(node -p 'JSON.stringify(JSON.parse(fs.readFileSync("node_modules/@reactogenic/cli/package.json", "utf8")).typescript)')"

# No allowImportingTsExtensions: the import a segment root generates for
# intro.tsx would be TS5097 without the mapper's ignore directive.
cat > tsconfig.json <<'EOF'
{
  "compilerOptions": {
    "strict": true, "noEmit": true, "jsx": "react-jsx", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022", "dom"], "skipLibCheck": true,
    "paths": { "@/*": ["./src/*"] }
  },
  "contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }],
  "include": ["src"]
}
EOF
cat > src/button.rtsx <<'EOF'
import type { ComponentProps, ReactNode } from "react";
import type { Slot } from "@reactogenic/core";

type Size = "md" | "lg";

interface ButtonProps {
  $Label?: Slot<{ className?: string; children?: ReactNode }>;
  $Icon?: Slot<ComponentProps<"span">, { size: Size }>;
  size: Size;
}

export function Button({ $Label, $Icon, size }: ButtonProps) {
  return (
    <button data-size={size}>
      <span slot={$Icon} &size />
      <b slot={$Label} className="label">Button</b>
    </button>
  );
}
EOF
cat > src/widgets/index.rtsx <<'EOF'
export function Badge({ text }: { text: string }) {
  return <em>{text}</em>;
}
EOF
cat > src/intro.tsx <<'EOF'
export default function Intro() {
  return <p>A segment written in plain TSX.</p>;
}
EOF
cat > src/outro.rtsx <<'EOF'
export default function Outro() {
  return <p>Bye…</p>;
}
EOF
# Imports without extensions — a sibling, a `paths` alias, a directory's
# index — a shorthand, slots with params, Switch, two segment roots.
cat > src/page.rtsx <<'EOF'
import { Switch } from "@reactogenic/core";
import { Button } from "./button";
import { Button as Aliased } from "@/button";
import { Badge } from "./widgets";

export function Page({ status }: { status: "loading" | "ready" }) {
  const size = "lg";
  return (
    <main>
      <section #intro />
      <Button size>
        <$Icon { size }><i>{size}</i></$Icon>
        <$Label>Save…</$Label>
      </Button>
      <Aliased size="md" />
      <Badge text={status} />
      <Switch on={status} exhaustive>
        <$Case is="loading">Loading…</$Case>
        <$Case is="ready">Ready</$Case>
      </Switch>
      <footer #outro />
    </main>
  );
}
EOF
# A .tsx file names the extension: stock TypeScript does not look for .rtsx.
cat > src/main.tsx <<'EOF'
import { Page } from "./page.rtsx";
export const app = <Page status="ready" />;
EOF
cp -R src "$work/src.clean"

failed=0
# run <title> <expected exit status> [<line the output must contain> ...]
run() {
  local title="$1" status="$2" out code line
  shift 2
  out="$("$tsc" --runExternalCode --noEmit --pretty false -p . 2>&1)"
  code=$?
  echo
  echo "== $title"
  echo "\$ tsc --runExternalCode --noEmit -p .   (exit $code)"
  [[ -n "$out" ]] && echo "$out"
  if [[ "$code" != "$status" ]]; then
    echo "FAIL: exit status $code, expected $status"
    failed=1
  fi
  for line in "$@"; do
    if ! grep -qxF -- "$line" <<<"$out"; then
      echo "FAIL: no line: $line"
      failed=1
    fi
  done
  if [[ $# -eq 0 && -n "$out" ]]; then
    echo "FAIL: output, expected none"
    failed=1
  fi
}
# edit <file> <from> <to>: replace the first occurrence, which must exist.
edit() {
  FILE="$1" FROM="$2" TO="$3" node -e '
    const { FILE, FROM, TO } = process.env, fs = require("fs");
    const text = fs.readFileSync(FILE, "utf8");
    if (!text.includes(FROM)) { console.error(`${FILE}: no ${FROM}`); process.exit(1); }
    fs.writeFileSync(FILE, text.replace(FROM, () => TO));' || exit 2
}
reset() { rm -rf src && cp -R "$work/src.clean" src; }

run "a clean project"  0

edit src/page.rtsx 'const size = "lg";' 'const size: string = "lg";'
run "a type error in copied code: the binding of the shorthand <Button size>"  2 \
  "src/page.rtsx(11,15): error TS2322: Type 'string' is not assignable to type 'Size'."
reset

edit src/page.rtsx '<Badge text={status} />' '<Badge text={status.length} />'
edit src/button.rtsx '<span slot={$Icon} &size />' '<span slot={$Icon} />'
# Line 16 comes after the `…` of line 13: offsets are bytes, and say so.
run "type errors in two .rtsx files: on a copied attribute; in generated code, at the attachment that produced it"  2 \
  "src/page.rtsx(16,14): error TS2322: Type 'number' is not assignable to type 'string'." \
  "src/button.rtsx(15,7): error TS2741: Property 'size' is missing in type '{}' but required in type '{ size: Size; }'."
reset

edit src/page.rtsx '<Badge text={status} />' '<$Hint /><Badge text={status.length} />'
run "a transpiler error: a numeric code, the name in the message; it hides the type error next to it"  2 \
  "src/page.rtsx(16,7): error reactogenic101: orphan-slot: Slot must be immediate child of the component"
reset

edit src/page.rtsx '<Badge text={status} />' '<Badge text={status.} />'
run "a syntax error: once, TypeScript's own"  2 \
  "src/page.rtsx(16,27): error TS1003: Identifier expected."
reset

# The children of a segment root are replaced: the virtual text parses, so
# the error is the mapper's to report, under TypeScript's number.
edit src/page.rtsx '<section #intro />' '<section #intro>{status.}</section>'
run "a syntax error in code the transpiler replaces"  2 \
  "src/page.rtsx(10,31): error reactogenic1003: Identifier expected."
reset

edit src/page.rtsx 'import { Badge } from "./widgets";' 'import { Badge } from "./widgets";
import Intro from "./intro.tsx";'
edit src/page.rtsx '<Badge text={status} />' '<Intro />'
run "the ignore directive is the generated import's only: the author's own ./intro.tsx is TS5097"  2 \
  "src/page.rtsx(5,19): error TS5097: An import path can only end with a '.tsx' extension when 'allowImportingTsExtensions' is enabled."
reset

edit src/main.tsx '"./page.rtsx"' '"./page"'
run "the limit: an extensionless import of an .rtsx file written in a .tsx file"  2 \
  "src/main.tsx(1,22): error TS2307: Cannot find module './page' or its corresponding type declarations."
reset

echo
if [[ "$failed" != 0 ]]; then
  echo "FAILED (project: $project)"
  exit 1
fi
echo "ok (project: $project)"
