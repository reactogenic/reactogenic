// Generates every derived file of this package from the unmodified copies in
// grammar/upstream (specs/phase01/ide.md, "Syntax highlighting"):
//
//   syntaxes/rtsx.tmLanguage.json           VS Code's TSX grammar + the .rtsx forms
//   syntaxes/rtsx.markdown.tmLanguage.json  ```rtsx fences: VS Code's ```tsx fence rule
//   language-configuration.json             TSX's, unchanged
//   tags-language-configuration.json        jsx-tags', with `$` in tag names and words
//   snippets/typescript.code-snippets       TSX's snippets, unchanged
//   ThirdPartyNotices.txt                   the licences of all of the above
//
//   node grammar/generate.mjs           write them
//   node grammar/generate.mjs --check   exit 1 if any is out of date
//
// Every patch asserts the shape of the upstream rule it changes, so an
// upstream update that touches one of them fails here instead of producing a
// grammar that is quietly wrong.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const packageRoot = path.resolve(here, '..');
const upstream = (file) => fs.readFileSync(path.join(here, 'upstream', file), 'utf8');

export const SCOPE_NAME = 'source.tsx.rtsx';

/** The scope table of ide.md, "Syntax highlighting". */
export const SCOPES = {
  slotTag: 'entity.name.function.slot.rtsx', // `$Icon` in `<$Icon>` and `</$Icon>`
  argSigil: 'storage.modifier.slot-arg.rtsx', // `&`, `&&`
  argName: 'entity.other.attribute-name.slot-arg.rtsx',
  params: 'meta.slot-params.rtsx', // `{ size }` in attribute position
  segment: 'support.class.component.segment.rtsx', // `#about-us`
  segmentSigil: 'punctuation.definition.segment.rtsx',
  segmentRoot: 'meta.segment-root.rtsx',
};

// A JSX identifier, as upstream's attribute-name rule spells it, and what must
// follow an attribute name there. An arg or a segment root ends the same way,
// which is what lets `&size>` and `#about-us/>` leave the tag end alone.
const IDENT = '[_$[:alpha:]][-_$[:alnum:]]*';
const UPSTREAM_NAME_END = '(?=\\s|=|/?>|/\\*|//)';
// Here a name also ends before `{`, `&` and `#`: no whitespace is needed
// before params, a spread, an arg or a segment root (`items{ item }`,
// `x{...p}`, `value&size`), and a name typed in front of one of them has none
// until the space is typed. The attribute name gets this end too. A sigil
// without a name ends before `{` only: `&&&x` and `&#x` stay illegal.
const NAME_END = '(?=\\s|=|/?>|/\\*|//|[{&#])';
const SIGIL_END = '(?=\\s|=|/?>|/\\*|//|\\{)';
// Block comments between `{` and what follows it, on one line.
const COMMENTS = '(?:/\\*.*?\\*/\\s*)*';
// A block comment that its line leaves open.
const OPEN_COMMENT = '/\\*(?:(?!\\*/).)*';
// The whitespace that leads a comment line, as upstream's line comment has it.
const LINE_INDENT = '^[ \\t]+';

const EMBEDDED_BEGIN = { 0: { name: 'punctuation.section.embedded.begin.tsx' } };
const EMBEDDED_END = { 0: { name: 'punctuation.section.embedded.end.tsx' } };
const BINDING_BRACE = { 0: { name: 'punctuation.definition.binding-pattern.object.tsx' } };

/** The rules .rtsx adds. Keys are repository names; every one starts with `rtsx-`. */
function rtsxRules() {
  return {
    // `&size`, `&&value={x}` — syntax.md, Slots → Attachment. The name is
    // optional: `&` right after it is typed, in front of an existing `>`,
    // would otherwise fall to upstream's catch-all `\S+`, which swallows the
    // `>` and repaints the rest of the file as attributes.
    'rtsx-slot-arg': {
      match: `(&&?)(?:(${IDENT})${NAME_END}|${SIGIL_END})`,
      captures: {
        1: { name: SCOPES.argSigil },
        2: { name: SCOPES.argName },
      },
    },
    // `#about-us` — syntax.md, Segment roots. The name is optional, as above.
    'rtsx-segment-root': {
      name: SCOPES.segmentRoot,
      match: `(#)(?:${IDENT}${NAME_END}|${SIGIL_END})`,
      captures: {
        0: { name: SCOPES.segment },
        1: { name: SCOPES.segmentSigil },
      },
    },
    // Anything else that starts with one of our sigils (`&a:b`, `#404`) is
    // illegal as upstream's catch-all would have it, but stops before the tag
    // end and before a `{`, whose `}` the catch-all would take with the tag
    // end. `&` and `#` never start an attribute in TSX, so no TSX file sees
    // this rule.
    'rtsx-sigil-illegal': {
      name: 'invalid.illegal.attribute.tsx',
      match: '[&#](?:(?!/?>|\\{)\\S)*',
    },
    // `{ size }`, `{ label: text, value = 0, ...rest }`, `{}` — an object
    // binding pattern in attribute position. `{` followed by `...` (after any
    // block comments) stays a spread attribute, the TSX rule.
    'rtsx-slot-params': {
      name: SCOPES.params,
      begin: `\\{(?!\\s*${COMMENTS}\\.\\.\\.)`,
      beginCaptures: BINDING_BRACE,
      end: '\\}',
      endCaptures: BINDING_BRACE,
      patterns: [{ include: '#parameter-object-binding-element' }],
    },
    // `{` with nothing but comments after it on its line — the last of them
    // may be a block comment the line leaves open. A regex cannot look at the
    // next line, so the first thing inside that is not a comment decides:
    // `...` makes it a spread, anything else params. Until then the rule is
    // upstream's `{…}` (jsx-evaluated-code), scopes included, so a multi-line
    // spread tokenizes exactly as in TSX; multi-line params keep that rule's
    // brace scopes and carry its content scope under their own.
    'rtsx-slot-params-or-spread': {
      begin: `\\{(?=\\s*${COMMENTS}(?://.*|${OPEN_COMMENT})?$)`,
      beginCaptures: EMBEDDED_BEGIN,
      end: '\\}',
      endCaptures: EMBEDDED_END,
      contentName: 'meta.embedded.expression.tsx',
      patterns: [
        { include: '#comment' },
        {
          begin: '(?=\\.\\.\\.)',
          end: '(?=\\})',
          patterns: [{ include: '#expression' }],
        },
        {
          contentName: SCOPES.params,
          begin: '(?=[^\\s}])',
          end: '(?=\\})',
          patterns: [{ include: '#parameter-object-binding-element' }],
        },
      ],
    },
  };
}

function rule(grammar, name) {
  const r = grammar.repository[name];
  assert.ok(r, `upstream rule ${name} is gone`);
  return r;
}

/**
 * Attribute position. Upstream lists `=` and the value as siblings of the
 * attribute name, so a value's `{` and a `{` that starts an attribute are the
 * same rule to it. Params need to tell them apart: `=` now opens a rule that
 * owns its value, and a `{` seen directly in the attribute list is params
 * unless `...` follows.
 */
function patchAttributes(g) {
  assert.deepEqual(
    rule(g, 'jsx-tag-attributes'),
    {
      name: 'meta.tag.attributes.tsx',
      begin: '\\s+',
      end: '(?=[/]?>)',
      patterns: [
        { include: '#comment' },
        { include: '#jsx-tag-attribute-name' },
        { include: '#jsx-tag-attribute-assignment' },
        { include: '#jsx-string-double-quoted' },
        { include: '#jsx-string-single-quoted' },
        { include: '#jsx-evaluated-code' },
        { include: '#jsx-tag-attributes-illegal' },
      ],
    },
    'upstream jsx-tag-attributes changed',
  );
  const assignment = rule(g, 'jsx-tag-attribute-assignment');
  assert.deepEqual(
    assignment,
    { name: 'keyword.operator.assignment.tsx', match: '=(?=\\s*(?:\'|"|{|/\\*|//|\\n))' },
    'upstream jsx-tag-attribute-assignment changed',
  );
  // Our rules end a name the way upstream's attribute name does, plus `{`, `&`
  // and `#` — which the attribute name gets as well: the fourth upstream rule
  // we change. In TSX only `{` can follow a name directly (`x{...p}`).
  const attributeName = rule(g, 'jsx-tag-attribute-name');
  assert.ok(attributeName.match.endsWith(`(${IDENT})\n  ${UPSTREAM_NAME_END}`), 'upstream jsx-tag-attribute-name changed');
  attributeName.match = attributeName.match.slice(0, -UPSTREAM_NAME_END.length) + NAME_END;
  assert.deepEqual(
    rule(g, 'jsx-tag-attributes-illegal'),
    { name: 'invalid.illegal.attribute.tsx', match: '\\S+' },
    'upstream jsx-tag-attributes-illegal changed',
  );
  // The value rule's end looks back at what closed the value.
  const code = rule(g, 'jsx-evaluated-code');
  assert.deepEqual(
    { begin: code.begin, end: code.end, contentName: code.contentName, beginCaptures: code.beginCaptures, endCaptures: code.endCaptures },
    { begin: '\\{', end: '\\}', contentName: 'meta.embedded.expression.tsx', beginCaptures: EMBEDDED_BEGIN, endCaptures: EMBEDDED_END },
    'upstream jsx-evaluated-code changed',
  );
  for (const quote of ['double', 'single']) {
    const s = rule(g, `jsx-string-${quote}-quoted`);
    const q = quote === 'double' ? '"' : "'";
    assert.deepEqual([s.begin, s.end], [q, q], `upstream jsx-string-${quote}-quoted changed`);
  }
  // A line comment ends before the newline, and begins with the whitespace
  // that leads its line: see the wrapper below.
  const lineComment = rule(g, 'comment').patterns.at(-1);
  assert.equal(lineComment.end, '(?=$)', 'upstream line comment changed');
  assert.ok(lineComment.begin.startsWith(`(${LINE_INDENT})?((//)`), 'upstream line comment changed');

  g.repository['rtsx-attribute-value'] = {
    // + `<`: an element as the value (`footer=<Match …>…</Match>`), valid JSX
    // that upstream marks illegal.
    begin: assignment.match.replace('{|', '{|<|'),
    beginCaptures: { 0: { name: assignment.name } },
    // Ends right after the value: a string, `{…}` or an element.
    end: '(?<=[}\'">])|(?=/?>)',
    patterns: [
      // `a= // it's "q"` + newline + `{y}`: the comment's last character must
      // not read as the end of a value, so the comment takes its newline along.
      // On a line of its own the comment starts at its indentation, where
      // upstream's rule starts: this one has to start there too to come first.
      { begin: `(?=(?:${LINE_INDENT})?//)`, end: '\\n', patterns: [{ include: '#comment' }] },
      { include: '#comment' },
      { include: '#jsx-string-double-quoted' },
      { include: '#jsx-string-single-quoted' },
      { include: '#jsx-evaluated-code' },
      { include: '#jsx-tag-without-attributes' },
      { include: '#jsx-tag' },
    ],
  };
  assert.notEqual(g.repository['rtsx-attribute-value'].begin, assignment.match);

  g.repository['jsx-tag-attributes'].patterns = [
    { include: '#comment' },
    { include: '#rtsx-slot-arg' },
    { include: '#rtsx-segment-root' },
    { include: '#rtsx-sigil-illegal' },
    { include: '#jsx-tag-attribute-name' },
    { include: '#rtsx-attribute-value' },
    { include: '#jsx-string-double-quoted' },
    { include: '#jsx-string-single-quoted' },
    { include: '#rtsx-slot-params-or-spread' },
    { include: '#rtsx-slot-params' },
    { include: '#jsx-evaluated-code' },
    { include: '#jsx-tag-attributes-illegal' },
  ];
}

// Slot tags: a third alternative in the tag-name group, captured on its own.
//   upstream  ((?:[a-z][a-z0-9]*|(COMPONENT))(?<!\.|-))           tag, component
//   patched   ((?:[a-z][a-z0-9]*|(SLOT)|(COMPONENT))(?<!\.|-))    tag, slot, component
// A slot tag is a plain identifier that starts with `$`: `<$ns.Comp>` and
// `<$a:b>` fall through to the component and namespace alternatives, as the
// transpiler reads them.
const TAG_NAME = '((?:[a-z][a-z0-9]*|([_$[:alpha:]][-_$[:alnum:].]*))(?<!\\.|-))';
const TAG_NAME_SLOT =
  '((?:[a-z][a-z0-9]*|(\\$[-_$[:alnum:]]*(?![-_$[:alnum:].:]))|([_$[:alpha:]][-_$[:alnum:].]*))(?<!\\.|-))';

/** `tagGroup` is the number of the tag-name group in `r[key]`. */
function patchTagName(r, key, capturesKey, tagGroup, where) {
  assert.equal(typeof r[key], 'string', `${where}: no ${key}`);
  assert.equal(r[key].split(TAG_NAME).length, 2, `${where}: the tag-name pattern must occur exactly once`);
  const captures = r[capturesKey];
  assert.equal(captures?.[tagGroup]?.name, 'entity.name.tag.tsx', `${where}: capture ${tagGroup} changed`);
  assert.equal(captures[tagGroup + 1]?.name, 'support.class.component.tsx', `${where}: capture ${tagGroup + 1} changed`);
  r[key] = r[key].replace(TAG_NAME, TAG_NAME_SLOT);
  const shifted = {};
  for (const [k, v] of Object.entries(captures)) shifted[Number(k) > tagGroup ? Number(k) + 1 : Number(k)] = v;
  shifted[tagGroup + 1] = { name: SCOPES.slotTag };
  r[capturesKey] = shifted;
}

function patchTagNames(g) {
  const bare = rule(g, 'jsx-tag-without-attributes');
  patchTagName(bare, 'begin', 'beginCaptures', 4, 'jsx-tag-without-attributes begin');
  patchTagName(bare, 'end', 'endCaptures', 4, 'jsx-tag-without-attributes end');
  const tag = rule(g, 'jsx-tag');
  patchTagName(tag, 'end', 'endCaptures', 5, 'jsx-tag end');
  assert.ok(Array.isArray(tag.patterns) && tag.patterns[0]?.begin, 'jsx-tag: no opening-tag pattern');
  patchTagName(tag.patterns[0], 'begin', 'beginCaptures', 4, 'jsx-tag opening tag');
  // Four more rules repeat the pattern inside look-aheads. They capture
  // nothing, and `$name` matches their component alternative, so they stay.
  const lookaheads = [
    tag.begin,
    ...['jsx-tag-in-expression', 'jsx-tag-without-attributes-in-expression'].flatMap((n) => [rule(g, n).begin, rule(g, n).end]),
  ];
  for (const re of lookaheads) assert.equal(re.split(TAG_NAME).length, 2, 'a tag look-ahead changed');
  const occurrences = JSON.stringify(g).split(JSON.stringify(TAG_NAME).slice(1, -1)).length - 1;
  assert.equal(occurrences, lookaheads.length, 'upstream uses the tag-name pattern somewhere new');
}

function includesOf(node, out = []) {
  if (Array.isArray(node)) node.forEach((n) => includesOf(n, out));
  else if (node && typeof node === 'object') {
    if (typeof node.include === 'string') out.push(node.include);
    Object.values(node).forEach((n) => includesOf(n, out));
  }
  return out;
}

/** The rtsx grammar, from the parsed upstream TSX grammar (which it does not modify). */
export function patchGrammar(upstreamGrammar) {
  const g = structuredClone(upstreamGrammar);
  assert.equal(g.scopeName, 'source.tsx', 'not the TSX grammar');
  assert.match(g.version ?? '', /TypeScript-TmLanguage\/commit\/[0-9a-f]{40}$/, 'upstream grammar has no version');
  const taken = Object.keys(g.repository).filter((k) => k.startsWith('rtsx-'));
  assert.deepEqual(taken, [], 'upstream has rules named rtsx-*');

  const added = rtsxRules();
  Object.assign(g.repository, added);
  patchAttributes(g);
  patchTagNames(g);
  // Every rule we add is reachable by name, and names only what exists.
  for (const [name, r] of Object.entries(g.repository)) {
    if (!name.startsWith('rtsx-')) continue;
    for (const inc of includesOf(r)) assert.ok(g.repository[inc.slice(1)], `${name} includes ${inc}, which does not exist`);
  }

  // Root scope `source.tsx.rtsx`: editors find injections by scope prefix, so
  // everything injected into `source.tsx` reaches .rtsx. Inherited scopes keep
  // their `.tsx` suffix, so themes and semantic-token fallbacks written for
  // TSX apply unchanged.
  return {
    information_for_contributors: [
      'GENERATED by packages/vscode/grammar/generate.mjs. Do not edit.',
      'Derived from TypeScriptReact.tmLanguage of https://github.com/microsoft/TypeScript-TmLanguage (MIT), as shipped by VS Code: see grammar/upstream/UPSTREAM.',
    ],
    version: g.version,
    name: 'rtsx',
    scopeName: SCOPE_NAME,
    patterns: g.patterns,
    repository: g.repository,
  };
}

/**
 * ```rtsx fences in Markdown: the rule VS Code's Markdown grammar has for
 * ```tsx fences, with our language in the three places that name TSX. It is
 * injected, since that grammar's list of fence languages is closed.
 */
export function markdownGrammar(upstreamMarkdown) {
  assert.equal(upstreamMarkdown.scopeName, 'text.html.markdown', 'not the Markdown grammar');
  assert.match(upstreamMarkdown.version ?? '', /vscode-markdown-tm-grammar\/commit\/[0-9a-f]{40}$/, 'upstream Markdown grammar has no version');
  const fence = structuredClone(upstreamMarkdown.repository?.fenced_code_block_tsx);
  assert.ok(fence, 'upstream rule fenced_code_block_tsx is gone');
  // The identifiers after the fence: only `tsx`, so ours is only `rtsx`.
  const IDENTIFIERS = '(?i:(tsx)(';
  assert.equal(fence.begin?.split(IDENTIFIERS).length, 2, 'upstream fenced_code_block_tsx: the identifiers changed');
  fence.begin = fence.begin.replace(IDENTIFIERS, '(?i:(rtsx)(');
  // One body, which is TSX and nothing more.
  assert.equal(fence.patterns?.length, 1, 'upstream fenced_code_block_tsx: the body changed');
  const [body] = fence.patterns;
  assert.equal(body.contentName, 'meta.embedded.block.typescriptreact', 'upstream fenced_code_block_tsx: the content scope changed');
  assert.deepEqual(body.patterns, [{ include: 'source.tsx' }], 'upstream fenced_code_block_tsx: the body changed');
  body.contentName = 'meta.embedded.block.rtsx'; // package.json maps this scope to the language
  body.patterns = [{ include: SCOPE_NAME }];
  // Nothing else in the rule names a language.
  const rest = JSON.stringify(fence).replaceAll(SCOPE_NAME, '').replaceAll('rtsx', '');
  assert.ok(!/tsx|typescript/i.test(rest), 'upstream fenced_code_block_tsx names TSX somewhere new');
  // The scope the manifest injects into, and the rule's own.
  assert.equal(fence.name, 'markup.fenced_code.block.markdown', 'upstream fenced_code_block_tsx: the scope changed');
  return {
    information_for_contributors: [
      'GENERATED by packages/vscode/grammar/generate.mjs. Do not edit.',
      'Derived from the rule fenced_code_block_tsx of https://github.com/microsoft/vscode-markdown-tm-grammar (MIT), as shipped by VS Code: see grammar/upstream/UPSTREAM.',
    ],
    version: upstreamMarkdown.version,
    scopeName: 'markdown.rtsx.codeblock',
    injectionSelector: 'L:text.html.markdown',
    patterns: [{ include: '#rtsx-code-block' }],
    repository: { 'rtsx-code-block': fence },
  };
}

/** jsx-tags' configuration with `$` allowed in tag names and in words. */
export function patchTagsConfiguration(upstreamConfig) {
  const c = structuredClone(upstreamConfig);
  // On-enter: indent after `<$Slot>`, and between `<$Slot>` and `</$Slot>`.
  const NAMES = [
    ['([_:\\w][_:\\w\\-.\\d]*)', '([_:$\\w][_:$\\w\\-.\\d]*)'], // opening tag
    ['([_:\\w][_:\\w-.\\d]*)', '([_:$\\w][_:$\\w-.\\d]*)'], // closing tag
  ];
  let edits = 0;
  for (const r of c.onEnterRules) {
    for (const side of ['beforeText', 'afterText']) {
      const p = r[side]?.pattern;
      if (!p) continue;
      for (const [from, to] of NAMES) {
        if (!p.includes(from)) continue;
        assert.equal(p.split(from).length, 2, 'jsx-tags: a tag name occurs twice in one pattern');
        r[side].pattern = p.replace(from, to);
        edits++;
      }
    }
  }
  assert.equal(edits, 4, 'jsx-tags: expected four tag-name patterns in onEnterRules');
  // Word pattern: `$Icon` is one word, so completing `<$Ic` replaces the `$`.
  const EXCLUDED = '\\@\\$\\^';
  assert.equal(c.wordPattern.pattern.split(EXCLUDED).length, 2, 'jsx-tags: wordPattern changed');
  c.wordPattern.pattern = c.wordPattern.pattern.replace(EXCLUDED, '\\@\\^');
  assert.ok(!c.wordPattern.pattern.includes('$'), 'jsx-tags: wordPattern still mentions $');
  return c;
}

function notices() {
  const rule = '='.repeat(72);
  return [
    'THIRD-PARTY SOFTWARE NOTICES AND INFORMATION',
    '',
    'GENERATED by grammar/generate.mjs from grammar/upstream. Do not edit.',
    '',
    'This extension includes material from the projects below.',
    '',
    '1. TypeScript-TmLanguage (https://github.com/microsoft/TypeScript-TmLanguage)',
    '   syntaxes/rtsx.tmLanguage.json is derived from its TSX grammar.',
    '2. Visual Studio Code (https://github.com/microsoft/vscode)',
    '   language-configuration.json, tags-language-configuration.json,',
    '   snippets/typescript.code-snippets and the grammar entries of package.json',
    '   are derived from its typescript-basics and javascript extensions.',
    '3. vscode-markdown-tm-grammar (https://github.com/microsoft/vscode-markdown-tm-grammar)',
    '   syntaxes/rtsx.markdown.tmLanguage.json is derived from the rule its',
    '   Markdown grammar has for tsx code fences.',
    '',
    rule,
    '1. TypeScript-TmLanguage',
    rule,
    '',
    upstream('LICENSE.txt').trim(),
    '',
    upstream('ThirdPartyNotices.txt').trim(),
    '',
    rule,
    '2. Visual Studio Code',
    rule,
    '',
    upstream('vscode/LICENSE.txt').trim(),
    '',
    rule,
    '3. vscode-markdown-tm-grammar',
    rule,
    '',
    upstream('markdown/LICENSE.txt').trim(),
    '',
  ].join('\n');
}

const json = (value) => JSON.stringify(value, null, '\t') + '\n';

/** Every generated file: path relative to the package root → content. */
export function generate() {
  const configuration = JSON.parse(upstream('vscode/language-configuration.json'));
  // TSX's own word pattern already takes `$` (and `#`) as word characters.
  assert.ok(!configuration.wordPattern.pattern.includes('$'), 'language-configuration: wordPattern excludes $');
  return {
    'syntaxes/rtsx.tmLanguage.json': json(patchGrammar(JSON.parse(upstream('TypeScriptReact.tmLanguage.json')))),
    'syntaxes/rtsx.markdown.tmLanguage.json': json(markdownGrammar(JSON.parse(upstream('markdown/markdown.tmLanguage.json')))),
    'language-configuration.json': json(configuration),
    'tags-language-configuration.json': json(patchTagsConfiguration(JSON.parse(upstream('vscode/tags-language-configuration.json')))),
    'snippets/typescript.code-snippets': upstream('vscode/typescript.code-snippets'),
    'ThirdPartyNotices.txt': notices(),
  };
}

/** The generated files that differ from what is on disk. */
export function stale(files = generate()) {
  return Object.entries(files)
    .filter(([file, content]) => {
      const target = path.join(packageRoot, file);
      return !fs.existsSync(target) || fs.readFileSync(target, 'utf8') !== content;
    })
    .map(([file]) => file);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const files = generate();
  const changed = stale(files);
  if (process.argv.includes('--check')) {
    if (changed.length) {
      console.error(`out of date — run "pnpm generate" in packages/vscode:\n${changed.map((f) => '  ' + f).join('\n')}`);
      process.exit(1);
    }
    console.log(`${Object.keys(files).length} generated files are up to date`);
  } else {
    for (const file of changed) {
      const target = path.join(packageRoot, file);
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, files[file]);
    }
    console.log(changed.length ? `wrote ${changed.join(', ')}` : 'nothing to write');
  }
}
