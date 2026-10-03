// The two language configurations and the Markdown fence injection.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, test } from 'vitest';
import { packageRoot, SCOPES } from '../grammar/generate.mjs';
import { registry, tokenize } from './textmate.mjs';

const readJson = (file) => JSON.parse(fs.readFileSync(path.join(packageRoot, file), 'utf8'));
const regexp = (p) => (typeof p === 'string' ? new RegExp(p) : new RegExp(p.pattern, p.flags));

describe('language-configuration.json (rtsx)', () => {
  const config = readJson('language-configuration.json');

  test('is TSX\'s, unchanged', () => {
    expect(config).toEqual(readJson('grammar/upstream/vscode/language-configuration.json'));
    expect(config.comments).toEqual({ lineComment: '//', blockComment: ['/*', '*/'] });
  });

  test('the word pattern keeps $ in a word and splits at & and -', () => {
    const words = (s) => s.match(new RegExp(config.wordPattern.pattern, 'g'));
    expect(words('<$IconStart')).toEqual(['$IconStart']);
    expect(words('&&size')).toEqual(['size']);
    expect(words('#about-us')).toEqual(['#about', 'us']);
  });
});

describe('tags-language-configuration.json (rtsx-tags)', () => {
  const config = readJson('tags-language-configuration.json');
  const stock = readJson('grammar/upstream/vscode/tags-language-configuration.json');
  // VS Code matches these against the text of the tag region before and after the cursor.
  const [between, after] = config.onEnterRules;
  const [stockBetween, stockAfter] = stock.onEnterRules;

  test('toggles comments as {/* */}', () => {
    expect(config.comments).toEqual({ blockComment: ['{/*', '*/}'] });
  });

  test('differs from jsx-tags only in the tag-name patterns and the word pattern', () => {
    const strip = (c) => ({ ...c, wordPattern: null, onEnterRules: c.onEnterRules.map((r) => ({ ...r, beforeText: null, afterText: null })) });
    expect(strip(config)).toEqual(strip(stock));
  });

  test.each([
    ['<$IconStart>', true, false],
    ['<$Option key={({ value }) => value} { label }>', true, false],
    ['<$my-slot &size>', true, false],
    ['<span slot={$Badge} &size &&v={1}>', true, true],
    ['<Each items { item, index }>', true, true],
    ['<section #about-us>', true, true],
    ['<Button size>', true, true],
    ['<$Icon />', false, false],
    ['<br>', false, false],
  ])('Enter after %s indents: %s (jsx-tags: %s)', (text, ours, theirs) => {
    expect(regexp(after.beforeText).test(text)).toBe(ours);
    expect(regexp(stockAfter.beforeText).test(text)).toBe(theirs);
  });

  test('Enter between <$Slot> and </$Slot> indents and outdents', () => {
    expect(between.action).toEqual({ indent: 'indentOutdent' });
    expect(regexp(between.beforeText).test('<$IconStart>')).toBe(true);
    expect(regexp(between.afterText).test('</$IconStart>')).toBe(true);
    expect(regexp(stockBetween.afterText).test('</$IconStart>')).toBe(false);
    expect(regexp(between.afterText).test('</Button>')).toBe(true);
    // The closing tag after a lone `>` line (a multi-line opening tag).
    expect(regexp(config.onEnterRules[2].afterText).test('</$IconStart>')).toBe(true);
  });

  test('the word pattern takes $Icon as one word', () => {
    const word = (c, s) => s.match(new RegExp(c.wordPattern.pattern, 'g'));
    expect(word(config, '<$IconStart')).toEqual(['$IconStart']);
    expect(word(stock, '<$IconStart')).toEqual(['IconStart']);
    expect(word(config, 'data-id')).toEqual(['data-id']);
  });
});

describe('```rtsx fences in Markdown', () => {
  // A stand-in for VS Code's Markdown grammar: the injection is all that is under test.
  const markdown = { scopeName: 'text.html.markdown', patterns: [] };

  test('tokenize as .rtsx between the fences, and only there', async () => {
    const r = registry({ grammars: { 'text.html.markdown': markdown }, injectTo: { 'markdown.rtsx.codeblock': ['text.html.markdown'] } });
    const grammar = await r.loadGrammar('text.html.markdown');
    const text = [
      'before &size',
      '```rtsx',
      'const a = <span slot={$Badge} &size #seg { x }>new</span>;',
      '```',
      'after &size',
      '~~~RTSX title="x"',
      'const b = <$Icon />;',
      '~~~',
      'end',
    ].join('\n');
    const { tokens, open } = tokenize(grammar, text);
    const at = (line, tokenText) => tokens.find((t) => t.line === line && t.text === tokenText);
    const embedded = (t) => t.scopes.includes('meta.embedded.block.rtsx');

    expect(at(2, 'rtsx').scopes).toContain('fenced_code.block.language.markdown');
    expect(embedded(at(3, '&'))).toBe(true);
    expect(at(3, '&').scopes).toContain(SCOPES.argSigil);
    expect(at(3, 'size').scopes).toContain(SCOPES.argName);
    expect(at(3, 'seg').scopes).toContain(SCOPES.segment);
    expect(at(3, 'x').scopes).toContain('variable.parameter.tsx');
    expect(at(4, '```').scopes).toContain('punctuation.definition.markdown');
    expect(embedded(at(4, '```'))).toBe(false);
    expect(tokens.filter((t) => t.line === 5).every((t) => t.scopes.length === 1)).toBe(true);
    expect(at(7, '$Icon').scopes).toContain(SCOPES.slotTag);
    expect(tokens.filter((t) => t.line === 9).every((t) => t.scopes.length === 1)).toBe(true);
    expect(tokens.filter((t) => t.line === 1).every((t) => t.scopes.length === 1)).toBe(true);
    expect(open).toEqual(['text.html.markdown']);
  });
});
