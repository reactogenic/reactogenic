// No .rtsx source of the repo has an `invalid.*` token, and none ends inside a
// tag: the fixtures, the packages' test apps, and the `// .rtsx` examples of
// specs/phase01/syntax.md (the blocks the conformance suite runs).
import fs from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';
import { isInvalid, repoRoot, rtsx, tokenize, tsx, walk } from './textmate.mjs';

const rel = (file) => path.relative(repoRoot, file).split(path.sep).join('/');

const sources = ['fixtures', 'packages']
  .flatMap((dir) => walk(path.join(repoRoot, dir), (name) => name.endsWith('.rtsx')))
  .map((file) => [rel(file), fs.readFileSync(file, 'utf8')]);
const fileCount = sources.length;

// A ```tsx block whose first line is `// .rtsx`, `// page.rtsx` or `// .rtsx — …`
// (go/internal/conformance/spec.go reads the same header).
const spec = fs.readFileSync(path.join(repoRoot, 'specs/phase01/syntax.md'), 'utf8');
let examples = 0;
for (const m of spec.matchAll(/^```tsx\n(\/\/\s*\S*\.rtsx\b[^\n]*\n[\s\S]*?)^```$/gm)) {
  const line = spec.slice(0, m.index).split('\n').length;
  sources.push([`specs/phase01/syntax.md:${line}`, m[1]]);
  examples++;
}

function problems(grammar, text) {
  const { tokens, open } = tokenize(grammar, text);
  const out = tokens.filter(isInvalid).map((t) => `${t.line}:${t.col + 1} ${JSON.stringify(t.text)} is ${t.scopes.at(-1)}`);
  if (open.some((s) => s.startsWith('meta.tag'))) out.push(`ends inside a tag: ${open.join(' ')}`);
  return out;
}

test('the repo has .rtsx sources to check', () => {
  expect(fileCount).toBeGreaterThanOrEqual(50);
  expect(examples).toBeGreaterThanOrEqual(25);
});

test.each(sources)('%s', (_name, text) => {
  expect(problems(rtsx, text)).toEqual([]);
});

// The same sources under the unmodified TSX grammar: what "open .rtsx as
// TypeScript JSX" would look like. Guards the check above against passing
// because it checks nothing.
test('the unmodified TSX grammar fails on many of them', () => {
  const failing = sources.filter(([, text]) => problems(tsx, text).length > 0);
  expect(failing.length).toBeGreaterThanOrEqual(15);
});
