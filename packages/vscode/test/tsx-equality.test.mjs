// Plain TSX tokenizes under `source.tsx.rtsx` exactly as under `source.tsx`:
// same tokens, same positions, same scopes — whitespace included, the root
// scope aside.
import fs from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';
import { repoRoot, rtsx, tokenize, tsx, walk } from './textmate.mjs';

const lines = (grammar, text) =>
  tokenize(grammar, text, { whitespace: true }).tokens.map((t) => `${t.line}:${t.col} ${JSON.stringify(t.text)} ${t.scopes.slice(1).join(' ')}`);

function differences(text) {
  const want = lines(tsx, text);
  const got = lines(rtsx, text);
  const out = [];
  for (let i = 0; i < Math.max(want.length, got.length) && out.length < 5; i++) {
    if (want[i] !== got[i]) out.push(`source.tsx:      ${want[i]}\nsource.tsx.rtsx: ${got[i]}`);
  }
  return out;
}

// Every .tsx file of the repo: the packages' sources and tests, and the
// expected outputs under fixtures/. `output.passN.tsx` is the text after an
// intermediate pass and may still hold .rtsx forms, so it is not plain TSX.
const files = walk(repoRoot, (name) => name.endsWith('.tsx') && !/^output\.pass\d+\.tsx$/.test(name));

test('the repo has .tsx files to compare', () => {
  const rel = files.map((f) => path.relative(repoRoot, f).split(path.sep).join('/'));
  expect(rel.filter((f) => f.startsWith('fixtures/')).length).toBeGreaterThanOrEqual(20);
  expect(rel).toContain('packages/core/src/runtime.test.tsx');
});

test.each(files.map((f) => [path.relative(repoRoot, f), f]))('%s', (_rel, file) => {
  expect(differences(fs.readFileSync(file, 'utf8'))).toEqual([]);
});

// The patched rules are the attribute list and the tag names, so this is where
// a difference could hide. Each snippet is valid TSX.
const SNIPPETS = [
  'const a = <div className="a" id=\'b\' hidden data-x={1} aria-label="l" />;',
  'const a = <div {...props} key="k" {...rest}>text</div>;',
  'const a = <div { ...props } />;',
  'const a = <div {/* c */ ...props} />;',
  'const a = <div {\n  ...props\n} />;',
  'const a = <div {\n\n  ...props\n} id="x" />;',
  'const a = <div {\n  // why\n  ...props\n} id="x" />;',
  'const a = <div { // why\n  ...props\n} id="x" />;',
  'const a = <div {\n  /* why */ ...props\n} />;',
  'const a = <A x={y} z={{ k: 1 }} w= {v} u =  "s" />;',
  'const a = <A x=\n  {y}\n  z=\n  "s" />;',
  'const a = <A x= // it\'s "q"\n  {y} z />;',
  'const a = <A x= // a -> b>\n  {y} z />;',
  'const a = <A x= /* c */ {y} z=/* " */"s" />;',
  'const a = <A x={y}z="s"w={v}>t</A>;',
  'const a = <A\n  // comment\n  x={1} /* c */ y\n  /** doc */\n  z="2"\n/>;',
  'const a = <svg:rect xlink:href="#a" ns:attr={1} />;',
  'const a = <Foo.Bar.Baz a-b-c="d" _x $y={1} />;',
  'const a = <Foo<string, number> x={y as T} />;',
  'const a = <A render={() => <B on={(e) => e > 1 && <C d="e" />} />} />;',
  'const a = <A t={`${x}`} r={/>/} s={a > b ? "<" : ">"} />;',
  'const a = <A style={{ color: "red", ...s }} onClick={() => { go(); }} />;',
  'const a = <>{items.map((i) => <li key={i.id}>{i.name}</li>)}</>;',
  'const a = <p>a &amp; b &lt; c {/* c */} # & {"}"}</p>;',
  'const a = <input disabled value="a>b" title=\'a/>b\' />;',
  'const a = <A b="x"\n  c\n  d={1}\n>\n  text\n</A>;',
  'const a = cond ? <A x /> : <B y="1">t</B>;',
  'const a = <$ns.Comp x="1">t</$ns.Comp>;',
  'function f<T,>(x: T) { return <T,>(y: T) => y; }\nconst g = <T extends object>(x: T) => x;',
  'const x = a < b && c > d; const y = a & b; const z = a && b;',
  'class C { #p = 1; get q() { return this.#p & 1; } static #s = <div id="#p" />; }',
  '/**\n * @param x the x\n * @example <A &b #c { d } />\n */\nexport function f(x: number) { return x; }',
  'const s = `<a &b #c { d }>`; const r = /<a &b>/g; // <a #c>',
  'export default function App() {\n  return (\n    <main>\n      <h1 className="t">Title</h1>\n      <List items={items} renderItem={(i) => <Item {...i} />} />\n    </main>\n  );\n}',
];

test.each(SNIPPETS.map((s) => [s.replace(/\n/g, '⏎'), s]))('%s', (_name, source) => {
  expect(differences(source + '\nconst after = 1;')).toEqual([]);
});

// The one intended difference on valid TSX: an element as an attribute value,
// which the TSX grammar paints as illegal (and, before `>`, never recovers from).
test('an element as an attribute value is where the two differ', () => {
  const source = 'const a = <Card footer=<b>f</b> x />;';
  expect(tokenize(tsx, source).tokens.some((t) => t.scopes.includes('invalid.illegal.attribute.tsx'))).toBe(true);
  expect(tokenize(rtsx, source).tokens.some((t) => t.scopes.some((s) => s.startsWith('invalid.')))).toBe(false);
});
