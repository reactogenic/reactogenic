// Scope assertions for the .rtsx forms (ide.md, "Syntax highlighting").
//
// Every case is followed by the line `const after = 1;`, and every case
// asserts that this line is code again: the failure this grammar exists to
// prevent is a form that swallows the tag end and repaints the rest of the
// file as attributes.
import { describe, expect, test } from 'vitest';
import { SCOPES } from '../grammar/generate.mjs';
import { isInvalid, registry, rtsx, RTSX, tokenize, tsx } from './textmate.mjs';

const SLOT = SCOPES.slotTag;
const SIGIL = SCOPES.argSigil;
const ARG = SCOPES.argName;
const PARAMS = SCOPES.params;
const SEGMENT = SCOPES.segment;
const PARAM = 'variable.parameter.tsx';
const ATTR = 'entity.other.attribute-name.tsx';
const EXPR = 'meta.embedded.expression.tsx';
const COMPONENT = 'support.class.component.tsx';
const TAG = 'entity.name.tag.tsx';
const CHILDREN = 'meta.jsx.children.tsx';
const READ = 'variable.other.readwrite.tsx';

/**
 * A check is [token text, occurrence (1-based), scope it must have, scope it
 * must not have]. `invalid` lists the texts of the `invalid.*` tokens a case
 * expects (none unless given).
 */
function run(grammar, source, checks = [], invalid = []) {
  const { tokens, open } = tokenize(grammar, source + '\nconst after = 1;');
  const problems = [];
  for (const [text, nth, must, mustNot] of checks) {
    const t = tokens.filter((t) => t.text === text)[nth - 1];
    if (!t) problems.push(`no token ${JSON.stringify(text)} #${nth}`);
    else if (!t.scopes.includes(must)) problems.push(`${JSON.stringify(text)} #${nth}: want ${must}, got ${t.scopes.join(' ')}`);
    else if (mustNot && t.scopes.includes(mustNot)) problems.push(`${JSON.stringify(text)} #${nth}: must not have ${mustNot}`);
  }
  const after = tokens.filter((t) => t.text === 'after').at(-1);
  if (!after) problems.push('the line after is not tokenized as code: no token "after"');
  else if (!after.scopes.includes('variable.other.constant.tsx') || after.scopes.some((s) => s.startsWith('meta.tag')))
    problems.push(`the line after is derailed: "after" is ${after.scopes.join(' ')}`);
  if (open.length !== 1) problems.push(`rules left open at the end: ${open.join(' ')}`);
  const got = tokens.filter(isInvalid).map((t) => t.text);
  if (JSON.stringify(got) !== JSON.stringify(invalid)) problems.push(`invalid tokens: want ${JSON.stringify(invalid)}, got ${JSON.stringify(got)}`);
  return problems;
}

function cases(table) {
  test.each(table)('%s', (_name, source, checks, invalid) => {
    expect(run(rtsx, source, checks, invalid)).toEqual([]);
  });
}

describe('slot tags', () => {
  cases([
    ['open and close, with attributes', 'const a = <B><$Icon x="1">t</$Icon></B>;', [
      ['$Icon', 1, SLOT, COMPONENT], ['$Icon', 2, SLOT, COMPONENT], ['$Icon', 1, TAG], ['x', 1, ATTR], ['B', 1, COMPONENT, SLOT]]],
    ['without attributes', 'const a = <B><$Label>Save</$Label></B>;', [['$Label', 1, SLOT], ['$Label', 2, SLOT], ['Save', 1, CHILDREN]]],
    ['self-closing, directly before />', 'const a = <B><$Icon/><$Icon /></B>;', [['$Icon', 1, SLOT], ['$Icon', 2, SLOT]]],
    ['with a hyphen', 'const a = <B><$my-slot /></B>;', [['$my-slot', 1, SLOT]]],
    ['bare $ and $$x', 'const a = <B><$ /><$$x>t</$$x></B>;', [['$', 1, SLOT], ['$$x', 1, SLOT], ['$$x', 2, SLOT]]],
    ['unicode name', 'const a = <B><$Größe>t</$Größe></B>;', [['$Größe', 1, SLOT], ['$Größe', 2, SLOT]]],
    ['with type arguments', 'const a = <B><$I<string> { x }>y</$I></B>;', [['$I', 1, SLOT], ['x', 1, PARAM], ['$I', 2, SLOT]]],
    ['closing tag with spaces', 'const a = <B><$I x >t</ $I ></B>;', [['$I', 2, SLOT]]],
    ['multi-line', 'const a = (\n  <B>\n    <$Option\n      key={({ value }) => value}\n      { label, value }\n    >\n      {label}\n    </$Option>\n  </B>\n);', [
      ['$Option', 1, SLOT], ['label', 1, PARAM], ['label', 2, READ], ['$Option', 2, SLOT]]],
    ['after return, without parentheses', 'function f() {\n  return <$I { x }>y</$I>;\n}', [['$I', 1, SLOT], ['x', 1, PARAM]]],
    ['$Case of a Switch', 'const a = <Switch on={s} exhaustive { value }><$Case is="a">A</$Case><$Case default>{value}</$Case></Switch>;', [
      ['Switch', 1, COMPONENT, SLOT], ['$Case', 1, SLOT], ['is', 1, ATTR], ['default', 1, ATTR], ['value', 1, PARAM]]],
    // Not slot tags: the transpiler reads these as a member expression and a namespaced name.
    ['$ns.Comp is a component', 'const a = <$ns.Comp x />;', [['$ns.Comp', 1, COMPONENT, SLOT]]],
    ['$a:b is a namespaced tag', 'const a = <B><$a:b /></B>;', [['$a', 1, 'entity.name.tag.namespace.tsx', SLOT], ['b', 1, TAG, SLOT]]],
    ['lower-case and component tags are untouched', 'const a = <div><Button>t</Button></div>;', [
      ['div', 1, TAG, SLOT], ['div', 1, TAG, COMPONENT], ['Button', 1, COMPONENT, SLOT], ['Button', 2, COMPONENT, SLOT]]],
  ]);
});

describe('slot args', () => {
  cases([
    ['& and &&, bare and valued', 'const a = <span slot={$X} &size &&v={o.v} &l={o.l} &&sel={s === o.v}>n</span>;', [
      ['&', 1, SIGIL], ['size', 1, ARG], ['&&', 1, SIGIL], ['v', 1, ARG], ['&', 2, SIGIL], ['l', 1, ARG], ['&&', 2, SIGIL], ['sel', 1, ARG],
      ['===', 1, 'keyword.operator.comparison.tsx'], ['=', 2, 'keyword.operator.assignment.tsx'], ['n', 1, CHILDREN],
      ['slot', 1, ATTR, ARG], ['$X', 1, READ]]],
    ['bare arg directly before >', 'const a = <b slot={$X} &size>n</b>;', [['&', 1, SIGIL], ['size', 1, ARG], ['n', 1, CHILDREN]]],
    ['bare arg directly before />', 'const a = <b slot={$X} &size/>;', [['&', 1, SIGIL], ['size', 1, ARG]]],
    ['&& arg directly before > and />', 'const a = <i><b slot={$X} &&size>n</b><b slot={$X} &&size/></i>;', [
      ['&&', 1, SIGIL], ['size', 1, ARG], ['&&', 2, SIGIL], ['size', 2, ARG], ['n', 1, CHILDREN]]],
    ['valued arg directly before > and />', 'const a = <i><b slot={$X} &&sel={a === b}>n</b><b slot={$X} &v="s"/></i>;', [
      ['sel', 1, ARG], ['v', 1, ARG], ['s', 1, 'string.quoted.double.tsx'], ['n', 1, CHILDREN]]],
    ['hyphenated name', 'const a = <b slot={$X} &data-id={1} />;', [['data-id', 1, ARG]]],
    ['names that are keywords', 'const a = <b slot={$X} &class &&default={1} &in />;', [['class', 1, ARG], ['default', 1, ARG], ['in', 1, ARG]]],
    ['valued by a string and by an element', 'const a = <b slot={$X} &t="s" &&u=<i/> v />;', [['t', 1, ARG], ['u', 1, ARG], ['i', 1, TAG], ['v', 1, ATTR, ARG]]],
    ['right after a value, no space', 'const a = <b slot={$X}&size>t</b>;', [['size', 1, ARG]]],
    ['after a multi-line value, one per line', 'const a = <b slot={$X} x={\n  y\n} &z\n  &&w={1}\n>t</b>;', [['z', 1, ARG], ['w', 1, ARG]]],
    ['in both arms of a conditional', 'const a = c ? <b slot={$X} &x/> : d && <i slot={$Y} &&y={1}>t</i>;', [
      ['x', 1, ARG], ['y', 1, ARG], ['&&', 1, 'keyword.operator.logical.tsx'], ['&&', 2, SIGIL]]],
    ['unicode name', 'const a = <b slot={$X} &größe />;', [['größe', 1, ARG]]],
  ]);
});

describe('slot params', () => {
  cases([
    ['on a slot element', 'const a = <B><$I c="i" { size }>x</$I></B>;', [
      ['size', 1, PARAM, EXPR], ['size', 1, PARAMS], ['{', 1, PARAMS], ['}', 1, PARAMS],
      ['{', 1, 'punctuation.definition.binding-pattern.object.tsx']]],
    ['rename, default, nested, rest', 'const a = <B><$I { row: { id }, size = "md", ...rest }>x</$I></B>;', [
      ['row', 1, 'variable.object.property.tsx'], [':', 1, 'punctuation.destructuring.tsx'], ['id', 1, PARAM], ['size', 1, PARAM],
      ['md', 1, 'string.quoted.double.tsx'], ['...', 1, 'keyword.operator.rest.tsx'], ['rest', 1, PARAM]]],
    ['empty', 'const a = <B><$I {}>x</$I></B>;', [['{', 1, PARAMS], ['}', 1, PARAMS]]],
    ['directly before >', 'const a = <B><$I { size }>x</$I></B>;', [['size', 1, PARAM], ['x', 1, CHILDREN]]],
    ['directly before />', 'const a = <Each items { item }/>;', [['item', 1, PARAM]]],
    ['empty, directly before > and />', 'const a = <B><$I {}>x</$I><$J {}/></B>;', [['{', 1, PARAMS], ['{', 2, PARAMS], ['x', 1, CHILDREN]]],
    ['on a component, after a bare attribute', 'const a = <Each items { item, index }>x</Each>;', [['items', 1, ATTR], ['item', 1, PARAM], ['index', 1, PARAM]]],
    ['right after a value, no space', 'const a = <Each items={xs}{ item } />;', [['xs', 1, EXPR], ['item', 1, PARAM]]],
    ['after a key function', 'const a = <B><$O key={({ value }) => value} { label }>{label}</$O></B>;', [
      ['=>', 1, 'storage.type.function.arrow.tsx'], ['value', 1, PARAM], ['label', 1, PARAM], ['label', 2, READ]]],
    ['on a generic component', 'const a = <Foo<string> { a } x={y as T} />;', [['a', 2, PARAM]]],
    ['then an arg and a bare attribute', 'const a = <b slot={$X} value { x } &y disabled />;', [
      ['value', 1, ATTR], ['x', 1, PARAM], ['y', 1, ARG], ['disabled', 1, ATTR]]],
    ['default: an object literal', 'const a = <B><$I { o = { k: 1 } }>x</$I></B>;', [['k', 1, 'meta.object-literal.key.tsx']]],
    ['default: an arrow with a block body', 'const a = <B><$I { f = () => { return 1; }, g }>x</$I></B>;', [['g', 1, PARAM]]],
    ['default: a JSX element', 'const a = <B><$I { icon = <i className="x" />, g }>x</$I></B>;', [['g', 1, PARAM], ['i', 1, TAG]]],
    ['default: a comparison with >', 'const a = <B><$I { n = a > b ? 1 : 2, g }>x</$I></B>;', [['g', 1, PARAM]]],
    ['default: a string holding }', 'const a = <B><$I { s = "}", g }>x</$I></B>;', [['g', 1, PARAM]]],
    ['computed and string keys', 'const a = <B><$I { [k]: v, "data-x": d }>x</$I></B>;', [['v', 1, PARAM], ['d', 1, PARAM]]],
    ['a nested array pattern', 'const a = <B><$I { pair: [x, y] }>t</$I></B>;', [['x', 1, PARAM], ['y', 1, PARAM]]],
    ['comments inside', 'const a = <B><$I { /* c */ size, // d\n  other }>t</$I></B>;', [['size', 1, PARAM], ['other', 1, PARAM]]],
    ['multi-line', 'const a = <Each items {\n  item,\n  index,\n}>x</Each>;', [['item', 1, PARAM], ['index', 1, PARAM], ['item', 1, PARAMS]]],
    ['multi-line, a default that holds a spread', 'const a = <Each items {\n  item = { ...d },\n  index,\n}>x</Each>;', [['item', 1, PARAM], ['index', 1, PARAM]]],
    ['multi-line, a comment line first', 'const a = <Each items {\n  // the item\n  item,\n}>x</Each>;', [
      [' the item', 1, 'comment.line.double-slash.tsx'], ['item', 1, PARAM]]],
    ['multi-line, a comment after the brace', 'const a = <Each items { // the item\n  item,\n}>x</Each>;', [['item', 1, PARAM]]],
    ['multi-line, empty', 'const a = <B><$I {\n}>x</$I></B>;', [['x', 1, CHILDREN]]],
    // The decision waits for the line the comment closes on, as for a spread.
    ['multi-line, a block comment left open after the brace', 'const a = <Each items {/* the\n item */ item }>x</Each>;', [
      ['item', 1, PARAM], ['item', 1, PARAMS], [' item ', 1, 'comment.block.tsx']]],
  ]);

  // A `{` that ends its line is TSX's `{…}` until a later line decides, so the
  // braces of multi-line params keep TSX's scopes; only what is inside is params.
  test('multi-line params keep the brace scopes of TSX', () => {
    const brace = (source, text) => tokenize(rtsx, source).tokens.find((t) => t.text === text).scopes;
    const multi = 'const a = <Each items {\n  item,\n}>x</Each>;';
    for (const [text, scope] of [['{', 'punctuation.section.embedded.begin.tsx'], ['}', 'punctuation.section.embedded.end.tsx']]) {
      expect(brace(multi, text)).toContain(scope);
      expect(brace(multi, text)).not.toContain(PARAMS);
    }
    expect(brace(multi, 'item')).toEqual(expect.arrayContaining([EXPR, PARAMS, PARAM]));
    const single = 'const a = <Each items { item }>x</Each>;';
    for (const text of ['{', '}']) {
      expect(brace(single, text)).toEqual(expect.arrayContaining([PARAMS, 'punctuation.definition.binding-pattern.object.tsx']));
    }
    expect(brace(single, 'item')).not.toContain(EXPR);
  });
});

// No whitespace is needed between two forms — the transpiler reads `items{ item }`
// and `value&size` as two — and while an attribute, an arg or a segment root is
// typed in front of what the tag already holds, there is none.
describe('a name directly before {, & or #', () => {
  cases([
    ['a bare attribute before an arg', 'const a = <b slot={$X} value&size>t</b>;', [['value', 1, ATTR], ['&', 1, SIGIL], ['size', 1, ARG], ['t', 1, CHILDREN]]],
    ['a bare attribute before a segment root', 'const a = <section hidden#seg>t</section>;', [['hidden', 1, ATTR], ['seg', 1, SEGMENT], ['t', 1, CHILDREN]]],
    ['an arg before an arg', 'const a = <b slot={$X} &size&&v={1}&w/>;', [['size', 1, ARG], ['&&', 1, SIGIL], ['v', 1, ARG], ['w', 1, ARG]]],
    ['a segment root and an arg, both orders', 'const a = <i><b slot={$X} #seg&x>t</b><b slot={$X} &x#seg /></i>;', [
      ['seg', 1, SEGMENT], ['x', 1, ARG], ['x', 2, ARG], ['seg', 2, SEGMENT], ['t', 1, CHILDREN]]],
    ['a bare attribute before params', 'const a = <Each items{ item }>x</Each>;', [['items', 1, ATTR], ['item', 1, PARAM], ['x', 1, CHILDREN]]],
    ['a namespaced attribute before params', 'const a = <A ns:x{ item }>t</A>;', [
      ['ns', 1, 'entity.other.attribute-name.namespace.tsx'], ['x', 1, ATTR], ['item', 1, PARAM]]],
    ['a bare attribute before multi-line params', 'const a = <Each items{\n  item\n}>x</Each>;', [['items', 1, ATTR], ['item', 1, PARAM]]],
    ['an arg before params', 'const a = <b slot={$X} &size{ x }>t</b>;', [['&', 1, SIGIL], ['size', 1, ARG], ['x', 1, PARAM], ['t', 1, CHILDREN]]],
    ['an && arg before params', 'const a = <b slot={$X} &&size{ x }>t</b>;', [['&&', 1, SIGIL], ['size', 1, ARG], ['x', 1, PARAM]]],
    ['a segment root before params', 'const a = <S #seg{ x }>t</S>;', [['seg', 1, SEGMENT], ['x', 1, PARAM], ['t', 1, CHILDREN]]],
    ['& without a name before params', 'const a = <Each items &{ item }>x</Each>;', [['&', 1, SIGIL], ['item', 1, PARAM], ['x', 1, CHILDREN]]],
    ['&& without a name before params', 'const a = <Each items &&{ item }>x</Each>;', [['&&', 1, SIGIL], ['item', 1, PARAM]]],
    ['# without a name before params', 'const a = <Each items #{ item }>x</Each>;', [['#', 1, SEGMENT], ['item', 1, PARAM], ['x', 1, CHILDREN]]],
    // Valid TSX that the TSX grammar derails on: see tsx-equality.test.mjs.
    ['a bare attribute before a spread', 'const a = <A x{...p}>t</A>;', [['x', 1, ATTR], ['p', 1, EXPR, PARAMS], ['t', 1, CHILDREN]]],
    ['an arg, and a sigil without a name, before a spread', 'const a = <i><b slot={$X} &size{...p}>t</b><b slot={$X} &{...p} /></i>;', [
      ['size', 1, ARG], ['p', 1, EXPR, PARAMS], ['&', 2, SIGIL], ['p', 2, EXPR, PARAMS], ['t', 1, CHILDREN]]],
    ['a segment root before a spread', 'const a = <S #seg{...p} />;', [['seg', 1, SEGMENT], ['p', 1, EXPR, PARAMS]]],
  ]);

  // Not valid .rtsx: the sigil's part is illegal, the params are still params.
  cases([
    ['a namespaced arg before params', 'const a = <b slot={$X} &a:b{ x }>t</b>;', [['x', 1, PARAM], ['t', 1, CHILDREN]], ['&a:b']],
    ['a segment that starts with a digit, before params', 'const a = <b #404{ x }>t</b>;', [['x', 1, PARAM], ['t', 1, CHILDREN]], ['#404']],
    // A sigil needs its name before the next sigil: these stay illegal as a whole.
    ['a sigil directly before a sigil', 'const a = <i><b slot={$X} &#seg>t</b><b ##x /><b &&&x /></i>;', [['t', 1, CHILDREN]], ['&#seg', '##x', '&&&x']],
  ]);

  // Every keystroke of typing a form in front of what the tag already holds.
  const hosts = {
    params: 'const a = <b slot={$X} ▮{ x }>t</b>;',
    'a spread': 'const a = <b slot={$X} ▮{...rest}>t</b>;',
    'a string-valued attribute': 'const a = <b slot={$X} ▮className="c">t</b>;',
    'an arg': 'const a = <b slot={$X} ▮&&v={1}>t</b>;',
    'a segment root': 'const a = <b slot={$X} ▮#about-us>t</b>;',
  };
  const typed = ['&size', '&&size', '#seg', 'value'];
  const table = Object.entries(hosts).flatMap(([host, source]) => typed.map((text) => [text, host, source]));
  test.each(table)('typing %s in front of %s never derails', (text, _host, source) => {
    for (let i = 1; i <= text.length; i++) {
      const state = source.replace('▮', text.slice(0, i));
      const { tokens } = tokenize(rtsx, state);
      const problems = run(rtsx, state, [['t', 1, CHILDREN]], tokens.filter(isInvalid).map((t) => t.text));
      expect(problems, state).toEqual([]);
    }
  });
});

describe('segment roots', () => {
  cases([
    ['with other attributes', 'const a = <main><section #about-us className="b" /></main>;', [
      ['#', 1, SCOPES.segmentSigil], ['#', 1, SEGMENT], ['about-us', 1, SEGMENT], ['about-us', 1, SCOPES.segmentRoot], ['className', 1, ATTR, SEGMENT]]],
    ['directly before >', 'const a = <p #x>t</p>;', [['x', 1, SEGMENT], ['t', 1, CHILDREN]]],
    ['directly before />', 'const a = <S #intro/>;', [['intro', 1, SEGMENT], ['S', 1, COMPONENT]]],
    ['right after a string, no space', 'const a = <b c="d"#seg />;', [['seg', 1, SEGMENT]]],
    ['unicode name', 'const a = <b #über-uns/>;', [['über-uns', 1, SEGMENT]]],
    // A segment root takes no value; the transpiler reports it. Here it must only not derail.
    ['with a value', 'const a = <section #about="x">t</section>;', [['about', 1, SEGMENT], ['x', 1, 'string.quoted.double.tsx'], ['t', 1, CHILDREN]]],
  ]);
});

// The state one keystroke after typing the sigil: no name yet, often right in
// front of an existing `>`.
describe('a sigil without a name', () => {
  cases([
    ['& before >', 'const a = <b slot={$X} &>t</b>;', [['&', 1, SIGIL], ['t', 1, CHILDREN]]],
    ['& before />', 'const a = <b slot={$X} &/>;', [['&', 1, SIGIL]]],
    ['&& before >', 'const a = <b slot={$X} &&>t</b>;', [['&&', 1, SIGIL], ['t', 1, CHILDREN]]],
    ['&& before />', 'const a = <b slot={$X} &&/>;', [['&&', 1, SIGIL]]],
    ['& before a space', 'const a = <b slot={$X} & size>t</b>;', [['&', 1, SIGIL], ['size', 1, ATTR, ARG], ['t', 1, CHILDREN]]],
    ['& at the end of a line', 'const a = <b slot={$X} &\n>t</b>;', [['&', 1, SIGIL], ['t', 1, CHILDREN]]],
    ['& before =', 'const a = <b slot={$X} &={x}>t</b>;', [['&', 1, SIGIL], ['x', 1, EXPR], ['t', 1, CHILDREN]]],
    ['# before >', 'const a = <section #>t</section>;', [['#', 1, SEGMENT], ['t', 1, CHILDREN]]],
    ['# before />', 'const a = <section #/>;', [['#', 1, SEGMENT]]],
    ['# before a space', 'const a = <section # id="x">t</section>;', [['#', 1, SEGMENT], ['id', 1, ATTR], ['t', 1, CHILDREN]]],
    ['# at the end of a line', 'const a = <section #\n/>;', [['#', 1, SEGMENT]]],
  ]);
});

// Not valid .rtsx. Marked invalid, as TSX's catch-all would, but the tag still ends.
describe('a sigil followed by something that is not a name', () => {
  cases([
    ['a namespaced arg before >', 'const a = <b slot={$X} &a:b>t</b>;', [['t', 1, CHILDREN]], ['&a:b']],
    ['a segment that starts with a digit, before >', 'const a = <section #404>t</section>;', [['t', 1, CHILDREN]], ['#404']],
    ['three ampersands before />', 'const a = <b slot={$X} &&&x/>;', [], ['&&&x']],
    ['a quote after the sigil', 'const a = <b slot={$X} &"x">t</b>;', [['t', 1, CHILDREN]], ['&"x"']],
  ]);
});

describe('what stays TSX', () => {
  cases([
    ['a spread attribute', 'const a = <B {...rest} { ...more } />;', [['rest', 1, EXPR, PARAMS], ['more', 1, EXPR, PARAMS], ['rest', 1, READ]]],
    ['a spread after a block comment', 'const a = <div {/* c */ ...props} />;', [['props', 1, EXPR, PARAMS], [' c ', 1, 'comment.block.tsx']]],
    ['a spread directly before > and />', 'const a = <i><B {...rest}>t</B><B {...rest}/></i>;', [['rest', 1, EXPR], ['rest', 2, EXPR], ['t', 1, CHILDREN]]],
    ['a multi-line spread', 'const a = <B {\n  ...rest\n} />;', [['rest', 1, READ, PARAM], ['rest', 1, EXPR]]],
    ['a multi-line spread after a blank line', 'const a = <B {\n\n  ...rest\n} c />;', [['rest', 1, READ, PARAM], ['c', 1, ATTR]]],
    ['a multi-line spread after a comment line', 'const a = <B {\n  // c\n  ...rest\n} />;', [['rest', 1, READ, PARAM], [' c', 1, 'comment.line.double-slash.tsx']]],
    ['a multi-line spread, a comment after the brace', 'const a = <B { // c\n  ...rest\n} />;', [['rest', 1, READ, PARAM]]],
    ['an attribute value is not params', 'const a = <B x={y} z={{ k: 1 }} w= {v} />;', [
      ['y', 1, EXPR, PARAMS], ['k', 1, 'meta.object-literal.key.tsx', PARAMS], ['v', 1, EXPR, PARAMS]]],
    ['a value on the next line', 'const a = <B x=\n  {y} />;', [['y', 1, EXPR, PARAMS]]],
    ['a value after two spaces', 'const a = <B x=  {y} />;', [['y', 1, EXPR, PARAMS]]],
    ['a value after a line comment that ends in a quote', 'const a = <X a= // it\'s "q"\n  {y} b />;', [['y', 1, EXPR, PARAMS], ['b', 1, ATTR]]],
    ['a value after a line comment that ends in >', 'const a = <X a= // a -> b>\n  {y} b />;', [['y', 1, EXPR, PARAMS], ['b', 1, ATTR]]],
    ['a value after a comment line that ends in a quote', 'const a = <X a=\n  // it\'s "q"\n  {y} b />;', [['y', 1, EXPR, PARAMS], ['b', 1, ATTR]]],
    ['a value after two comment lines, } and >', 'const a = <X a= // {c}\n\t// a -> b>\n\t{y} b />;', [['y', 1, EXPR, PARAMS], ['b', 1, ATTR]]],
    ['a spread after a block comment left open on the line of {', 'const a = <div {/* a\n b */ ...props} c />;', [
      ['props', 1, EXPR, PARAMS], ['props', 1, READ, PARAM], ['c', 1, ATTR]]],
    ['a value after a block comment', 'const a = <X a=/* " */{y} b />;', [['y', 1, EXPR, PARAMS], ['b', 1, ATTR]]],
    ['&& in an expression is the logical operator', 'const a = <b x={p &&q} y={p&&q}>{p &&q}</b>;', [
      ['&&', 1, 'keyword.operator.logical.tsx'], ['&&', 2, 'keyword.operator.logical.tsx'], ['&&', 3, 'keyword.operator.logical.tsx']]],
    ['& and # in strings and text', 'const a = <b t="a &b c" u=\'#x \'>R&amp;D #x &y {e} $F</b>;', [
      ['a &b c', 1, 'string.quoted.double.tsx'], ['#x ', 1, 'string.quoted.single.tsx'], ['amp', 1, 'constant.character.entity.tsx'], ['e', 1, READ]]],
    ['a private name', 'class C { #x = 1; m() { return <b y={this.#x} />; } }', [['#x', 1, 'meta.field.declaration.tsx', SCOPES.segmentRoot]]],
    ['a generic arrow function', 'const f = <$T,>(x: $T) => x;', [['$T', 1, 'entity.name.type.tsx', TAG]]],
    ['less-than before a $ name', 'const a = <B>{n <$max ? 1 : 2}</B>;\nconst b = n <$max ;', [['$max', 1, READ, TAG], ['$max', 2, READ, TAG]]],
    ['rtsx-looking text in a template, a regex and a string', 'const s = `<b &x #y { z }>`; const r = /<b &x>/; const q = "<b #y>";', []],
    ['types: generics and intersections', 'type A<T> = B<T> & C; const f = <T,>(x: T & { a: 1 }) => x as T & U;', []],
    ['slot={$X} is an ordinary attribute', 'const a = <span slot={$Badge} className="d">f</span>;', [['slot', 1, ATTR], ['$Badge', 1, READ, SLOT]]],
    ['a comment between attributes', 'const a = <b /* &x #y { z } */ &k // &m\n  #n />;', [['k', 1, ARG], ['n', 1, SEGMENT], [' &m', 1, 'comment.line.double-slash.tsx']]],
  ]);
});

// Valid JSX that the TSX grammar marks illegal; syntax.md uses it (`footer=<Match …>`).
describe('an element as an attribute value', () => {
  cases([
    ['an element, then a self-closing element', 'const a = <Card footer=<Match on={more}>More</Match> render=<b /> x />;', [
      ['Match', 1, COMPONENT], ['More', 1, CHILDREN], ['b', 1, TAG], ['x', 1, ATTR]]],
    ['directly before >', 'const a = <Card footer=<b>f</b>>t</Card>;', [['f', 1, CHILDREN], ['t', 1, CHILDREN]]],
    ['a fragment', 'const a = <A b=<><i/></> c />;', [['c', 1, ATTR]]],
    ['whose text holds >', 'const a = <A b=<i>1 > 0</i> c />;', [['c', 1, ATTR]]],
    ['followed by params', 'const a = <A b=<C d={e} { f } /> { g } h />;', [['f', 1, PARAM], ['g', 1, PARAM], ['h', 1, ATTR]]],
  ]);
});

describe('nesting', () => {
  cases([
    ['a tag inside an attribute expression', 'const a = <T r={(r) => <td slot={$C} &r #s { x }>{x}</td>} />;', [
      ['r', 3, ARG], ['s', 1, SEGMENT], ['x', 1, PARAM], ['x', 2, READ]]],
    ['four levels deep', 'const a = <A a={<B b={<C c={<D d={<E slot={$X} &e { p } />} />} />} />} />;', [['e', 1, ARG], ['p', 1, PARAM]]],
    ['a slot element inside a slot element', 'const a = <B><$A { x }><C><$D &&y={x}>{x}</$D></C></$A></B>;', [
      ['$A', 1, SLOT], ['$D', 1, SLOT], ['y', 1, ARG], ['$D', 2, SLOT], ['$A', 2, SLOT]]],
  ]);
});

describe('the unmodified TSX grammar', () => {
  // The reason for this grammar (ide.md): under `source.tsx` an arg or a
  // segment root in front of `>` repaints everything after it. If a new
  // upstream grammar ever handles these, the fork may no longer be needed.
  test.each([
    ['an arg before >', 'const a = <b slot={$X} &size>n</b>;'],
    ['a segment root before />', 'const a = <section #about-us/>;'],
  ])('derails on %s', (_name, source) => {
    expect(run(tsx, source).join('\n')).toMatch(/the line after is derailed/);
    expect(run(rtsx, source)).toEqual([]);
  });
});

describe('the root scope', () => {
  test('is source.tsx.rtsx on every token', () => {
    const { tokens } = tokenize(rtsx, 'const a = <B><$I { x }>y</$I></B>;');
    expect(tokens.length).toBeGreaterThan(10);
    for (const t of tokens) expect(t.scopes[0]).toBe('source.tsx.rtsx');
  });

  test('new scopes end in .rtsx, inherited ones keep .tsx', () => {
    const { tokens } = tokenize(rtsx, 'const a = <B><$I { x } &a #s>y</$I></B>;');
    const scopes = new Set(tokens.flatMap((t) => t.scopes.slice(1)));
    const ours = new Set([...scopes].filter((s) => s.endsWith('.rtsx')));
    expect(ours).toEqual(new Set([SLOT, SIGIL, ARG, PARAMS, SEGMENT, SCOPES.segmentSigil, SCOPES.segmentRoot]));
    for (const s of scopes) expect(s).toMatch(/\.(tsx|rtsx)$/);
  });

  // Why not `source.rtsx`: an extension that injects into `source.tsx`
  // (styled-components, GraphQL templates, VS Code's own JSDoc) reaches
  // `source.tsx.rtsx` through the prefix walk, with its selector unchanged.
  test('receives grammars injected into source.tsx', async () => {
    const injection = {
      scopeName: 'thirdparty.injection',
      injectionSelector: 'L:source.tsx -comment -string',
      patterns: [{ match: '\\bMAGIC\\b', name: 'keyword.other.thirdparty' }],
    };
    const r = registry({ grammars: { 'thirdparty.injection': injection }, injectTo: { 'thirdparty.injection': ['source.tsx'] } });
    const magic = (grammar) => tokenize(grammar, 'const a = MAGIC; // MAGIC').tokens.filter((t) => t.text.includes('MAGIC'));
    for (const scope of [RTSX, 'source.tsx']) {
      const [code, comment] = magic(await r.loadGrammar(scope));
      expect(code.scopes).toContain('keyword.other.thirdparty');
      expect(comment.scopes).not.toContain('keyword.other.thirdparty');
    }
  });
});
