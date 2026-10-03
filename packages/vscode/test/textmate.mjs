// A vscode-textmate registry wired the way VS Code wires its own: grammars by
// scope name, and injections found by walking the dot-prefixes of the root
// scope (TMGrammarFactory.getInjections), which is why `source.tsx.rtsx`
// receives what is injected into `source.tsx`.
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { packageRoot, SCOPE_NAME } from '../grammar/generate.mjs';

const require = createRequire(import.meta.url);
const vsctm = require('vscode-textmate');
const oniguruma = require('vscode-oniguruma');

export const RTSX = SCOPE_NAME;
export const TSX = 'source.tsx';
export const MARKDOWN = 'text.html.markdown';
export const repoRoot = path.resolve(packageRoot, '../..');

const onigLib = oniguruma
  .loadWASM(fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm')))
  .then(() => ({
    createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
    createOnigString: (s) => new oniguruma.OnigString(s),
  }));

const FILES = {
  [RTSX]: 'syntaxes/rtsx.tmLanguage.json',
  [TSX]: 'grammar/upstream/TypeScriptReact.tmLanguage.json', // the unmodified TSX grammar
  'markdown.rtsx.codeblock': 'syntaxes/rtsx.markdown.tmLanguage.json',
  [MARKDOWN]: 'grammar/upstream/markdown/markdown.tmLanguage.json', // VS Code's, unmodified
};

/**
 * `grammars`: extra raw grammars by scope name. `injectTo`: scope name of an
 * injection grammar → the scopes it is injected into (a manifest's `injectTo`).
 */
export function registry({ grammars = {}, injectTo = {} } = {}) {
  return new vsctm.Registry({
    onigLib,
    loadGrammar: async (scopeName) => {
      if (grammars[scopeName]) return structuredClone(grammars[scopeName]);
      const file = FILES[scopeName];
      if (!file) return null;
      return vsctm.parseRawGrammar(fs.readFileSync(path.join(packageRoot, file), 'utf8'), file);
    },
    getInjections: (scopeName) => {
      const parts = scopeName.split('.');
      const out = [];
      for (let i = 1; i <= parts.length; i++) {
        const prefix = parts.slice(0, i).join('.');
        for (const [injection, targets] of Object.entries(injectTo)) if (targets.includes(prefix)) out.push(injection);
      }
      return out;
    },
  });
}

const shared = registry();
export const rtsx = await shared.loadGrammar(RTSX);
export const tsx = await shared.loadGrammar(TSX);

/**
 * Tokenizes a whole text. Returns the tokens as `{ line, col, text, scopes }`
 * (line 1-based) — without the whitespace-only ones unless `whitespace` is
 * set — and `open`: the scopes still open after the last line.
 */
export function tokenize(grammar, text, { whitespace = false } = {}) {
  const tokens = [];
  let stack = vsctm.INITIAL;
  text
    .replace(/\n$/, '')
    .split('\n')
    .forEach((line, i) => {
      const r = grammar.tokenizeLine(line, stack);
      for (const t of r.tokens) {
        const s = line.slice(t.startIndex, t.endIndex);
        if (whitespace || s.trim()) tokens.push({ line: i + 1, col: t.startIndex, text: s, scopes: t.scopes });
      }
      stack = r.ruleStack;
    });
  // What an empty next line would be scoped as: the rules the text left open.
  const open = grammar.tokenizeLine('', stack).tokens[0]?.scopes ?? [];
  return { tokens, open };
}

export const isInvalid = (token) => token.scopes.some((s) => s.startsWith('invalid.'));

/** Files under `dir` (absolute paths) whose name `keep` accepts; skips build and vendor trees. */
export function walk(dir, keep, out = []) {
  const SKIP = new Set(['node_modules', '.git', 'dist', 'third_party', '.claude']);
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (SKIP.has(e.name)) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, keep, out);
    else if (keep(e.name)) out.push(p);
  }
  return out.sort();
}
