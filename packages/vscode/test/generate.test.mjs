// The generated files are what the generator writes; the generator refuses an
// upstream grammar whose patched rules changed shape; the manifest carries
// what VS Code's own TSX entry carries.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, test } from 'vitest';
import { generate, packageRoot, patchGrammar, patchTagsConfiguration, SCOPE_NAME, stale } from '../grammar/generate.mjs';

const read = (file) => fs.readFileSync(path.join(packageRoot, file), 'utf8');
const readJson = (file) => JSON.parse(read(file));
const upstreamGrammar = () => readJson('grammar/upstream/TypeScriptReact.tmLanguage.json');

describe('generated files', () => {
  const files = generate();

  test('regenerating changes nothing', () => {
    expect(stale(files)).toEqual([]);
  });

  test.each(Object.keys(files))('%s is committed as generated', (file) => {
    expect(read(file)).toBe(files[file]);
  });

  test.each(Object.keys(files).filter((f) => f.endsWith('.json')))('%s is valid JSON', (file) => {
    expect(() => JSON.parse(files[file])).not.toThrow();
  });

  test('the generator does not modify its input and is deterministic', () => {
    const input = upstreamGrammar();
    const before = JSON.stringify(input);
    const once = JSON.stringify(patchGrammar(input));
    expect(JSON.stringify(input)).toBe(before);
    expect(JSON.stringify(patchGrammar(input))).toBe(once);
  });

  test('the grammar adds rules and changes three upstream ones, nothing else', () => {
    const up = upstreamGrammar();
    const ours = readJson('syntaxes/rtsx.tmLanguage.json');
    expect(ours.scopeName).toBe(SCOPE_NAME);
    expect(ours.patterns).toEqual(up.patterns);
    const names = Object.keys(ours.repository);
    const added = names.filter((n) => !(n in up.repository));
    expect(added.sort()).toEqual(
      ['rtsx-attribute-value', 'rtsx-segment-root', 'rtsx-sigil-illegal', 'rtsx-slot-arg', 'rtsx-slot-params', 'rtsx-slot-params-or-spread'].sort(),
    );
    expect(Object.keys(up.repository).filter((n) => !(n in ours.repository))).toEqual([]);
    const changed = Object.keys(up.repository).filter((n) => JSON.stringify(up.repository[n]) !== JSON.stringify(ours.repository[n]));
    expect(changed.sort()).toEqual(['jsx-tag', 'jsx-tag-attributes', 'jsx-tag-without-attributes']);
  });

  test('the notices carry every upstream licence', () => {
    const notices = read('ThirdPartyNotices.txt');
    for (const file of ['LICENSE.txt', 'ThirdPartyNotices.txt', 'vscode/LICENSE.txt']) {
      expect(notices).toContain(read(`grammar/upstream/${file}`).trim());
    }
  });
});

describe('an upstream change fails the generator', () => {
  const mutations = {
    'the attribute list gains a pattern': (g) => g.repository['jsx-tag-attributes'].patterns.push({ include: '#comment' }),
    'the attribute list ends differently': (g) => (g.repository['jsx-tag-attributes'].end = '(?=>)'),
    'the assignment rule accepts something new': (g) => (g.repository['jsx-tag-attribute-assignment'].match += '?'),
    'the attribute name ends differently': (g) => (g.repository['jsx-tag-attribute-name'].match += '|$'),
    'the catch-all changes': (g) => (g.repository['jsx-tag-attributes-illegal'].match = '[^\\s>]+'),
    'the {…} rule changes its braces': (g) => (g.repository['jsx-evaluated-code'].begin = '\\{\\{'),
    'a string rule changes its quote': (g) => (g.repository['jsx-string-single-quoted'].end = '`'),
    'the line comment consumes its newline': (g) => (g.repository.comment.patterns.at(-1).end = '$'),
    'the tag name pattern changes in an opening tag': (g) => {
      const open = g.repository['jsx-tag'].patterns[0];
      open.begin = open.begin.replace('[a-z][a-z0-9]*', '[a-z][a-z0-9_]*');
    },
    'the tag name pattern changes in a closing tag': (g) => {
      const tag = g.repository['jsx-tag'];
      tag.end = tag.end.replace('[a-z][a-z0-9]*', '[a-z][a-z0-9_]*');
    },
    'a tag capture is renumbered': (g) => {
      const c = g.repository['jsx-tag-without-attributes'].beginCaptures;
      [c[4], c[5]] = [c[5], c[4]];
    },
    'a tag look-ahead changes': (g) => {
      const r = g.repository['jsx-tag-in-expression'];
      r.end = r.end.replace('[a-z][a-z0-9]*', '[a-z][a-z0-9_]*');
    },
    'a new rule uses the tag name pattern': (g) => (g.repository['jsx-new'] = { match: g.repository['jsx-tag'].begin }),
    'a rule we include is renamed': (g) => {
      g.repository['parameter-object-binding'] = g.repository['parameter-object-binding-element'];
      delete g.repository['parameter-object-binding-element'];
    },
    'upstream takes one of our rule names': (g) => (g.repository['rtsx-slot-arg'] = { match: 'x' }),
    'it is another grammar': (g) => (g.scopeName = 'source.ts'),
    'the version is gone': (g) => delete g.version,
  };

  test('the unmodified grammar passes', () => {
    expect(() => patchGrammar(upstreamGrammar())).not.toThrow();
  });

  test.each(Object.entries(mutations))('%s', (_name, mutate) => {
    const g = upstreamGrammar();
    mutate(g);
    expect(() => patchGrammar(g)).toThrow();
  });

  test('so does a change to the jsx-tags configuration', () => {
    const tags = () => readJson('grammar/upstream/vscode/tags-language-configuration.json');
    expect(() => patchTagsConfiguration(tags())).not.toThrow();
    const fewer = tags();
    fewer.onEnterRules.pop();
    fewer.onEnterRules.pop();
    expect(() => patchTagsConfiguration(fewer)).toThrow();
    const words = tags();
    words.wordPattern.pattern = words.wordPattern.pattern.replace('\\$', '');
    expect(() => patchTagsConfiguration(words)).toThrow();
  });
});

describe('the manifest', () => {
  const manifest = readJson('package.json');
  const { contributes } = manifest;
  const basics = readJson('grammar/upstream/vscode/typescript-basics.package.json').contributes;
  const tsxGrammar = basics.grammars.find((g) => g.scopeName === 'source.tsx');
  const grammar = contributes.grammars.find((g) => g.language === 'rtsx');

  test('is the extension reactogenic.rtsx', () => {
    expect(`${manifest.publisher}.${manifest.name}`).toBe('reactogenic.rtsx');
    expect(manifest.engines.vscode).toBe('^1.91.0');
    expect(manifest.private).toBe(true);
  });

  test('declares the languages rtsx and rtsx-tags', () => {
    const [rtsx, tags] = contributes.languages;
    expect(rtsx).toMatchObject({ id: 'rtsx', extensions: ['.rtsx'], configuration: './language-configuration.json' });
    expect(tags).toMatchObject({ id: 'rtsx-tags', configuration: './tags-language-configuration.json' });
    expect(tags.extensions).toBeUndefined(); // only ever an embedded language
  });

  test('the grammar entry carries what the TSX entry carries', () => {
    expect(grammar.scopeName).toBe(SCOPE_NAME);
    expect(grammar.unbalancedBracketScopes).toEqual(tsxGrammar.unbalancedBracketScopes);
    expect(grammar.tokenTypes).toEqual(tsxGrammar.tokenTypes);
    // The same scopes, mapped to our two languages instead of TSX's two.
    const ours = { 'jsx-tags': 'rtsx-tags', typescriptreact: 'rtsx' };
    const expected = Object.fromEntries(Object.entries(tsxGrammar.embeddedLanguages).map(([scope, lang]) => [scope, ours[lang]]));
    expect(grammar.embeddedLanguages).toEqual(expected);
    expect(Object.values(expected).every(Boolean)).toBe(true);
  });

  test('semantic token scopes are the TSX ones', () => {
    const tsx = basics.semanticTokenScopes.find((s) => s.language === 'typescriptreact');
    expect(contributes.semanticTokenScopes).toEqual([{ language: 'rtsx', scopes: tsx.scopes }]);
  });

  test('snippets are the TSX ones', () => {
    const tsx = basics.snippets.find((s) => s.language === 'typescriptreact');
    expect(contributes.snippets).toEqual([{ language: 'rtsx', path: tsx.path }]);
    expect(Object.keys(readJson(tsx.path)).length).toBeGreaterThan(20);
  });

  test('the Markdown fence injection, breakpoints and Emmet', () => {
    const md = contributes.grammars.find((g) => g.scopeName === 'markdown.rtsx.codeblock');
    expect(md.injectTo).toEqual(['text.html.markdown']);
    expect(md.embeddedLanguages).toEqual({ 'meta.embedded.block.rtsx': 'rtsx' });
    expect(readJson(md.path).scopeName).toBe(md.scopeName);
    expect(contributes.breakpoints).toEqual([{ language: 'rtsx' }]);
    expect(contributes.configurationDefaults['emmet.includeLanguages']).toEqual({ rtsx: 'typescriptreact' });
  });

  test('every contributed path exists and scope names agree with the files', () => {
    const paths = [
      ...contributes.languages.map((l) => l.configuration),
      ...contributes.grammars.map((g) => g.path),
      ...contributes.snippets.map((s) => s.path),
    ];
    for (const p of paths) expect(fs.existsSync(path.join(packageRoot, p)), p).toBe(true);
    for (const g of contributes.grammars) expect(readJson(g.path).scopeName).toBe(g.scopeName);
    // Every scope an embedded-language entry names is one the grammar produces.
    const text = read(grammar.path);
    for (const scope of Object.keys(grammar.embeddedLanguages)) expect(text).toContain(JSON.stringify(scope));
  });
});
