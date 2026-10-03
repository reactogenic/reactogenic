// Package checktest holds the projects behind `reactogenic check`'s golden
// output (internal/check/testdata/golden), as data: check's tests print
// them, and the language server's tests pull the diagnostics of each .rtsx
// document of the same projects — the editor shows what check prints
// (specs/phase01/ide.md, *Diagnostics*).
package checktest

import (
	"maps"
	"strings"
)

// Project is one project of the goldens.
type Project struct {
	Name  string
	Files map[string]string // by path relative to the project's directory
	// Config is the tsconfig `check` is run on; "" is tsconfig.json.
	Config string
	// SyntaxError: a file of the project does not parse. There the hosts
	// differ by design — the builds are strict, the editor tolerant
	// (ide.md, *Tolerance*) — and in tsc's steps (*Diagnostics*).
	SyntaxError bool
	// Unlisted are the .rtsx files that are in no program of the project:
	// no `include` matches them and no import reaches them. `check` does
	// not report them; an editor that opens one checks it on its own.
	Unlisted []string
}

// TSConfig is the tsconfig of most projects.
const TSConfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true
  },
  "include": ["src"]
}`

// Options is the compilerOptions object of TSConfig, for projects that
// write their own.
const Options = `"strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true`

// JSXTypes is a minimal JSX namespace, as `src/jsx.d.ts`.
const JSXTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
}`

// Core is a stand-in for the installed @reactogenic/core.
var Core = map[string]string{
	"node_modules/@reactogenic/core/package.json": `{ "name": "@reactogenic/core", "types": "index.d.ts" }`,
	"node_modules/@reactogenic/core/index.d.ts": `type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol }; // as React: a function is not a node
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
type Key = string | number | bigint;
export declare const SLOT_KEY: unique symbol;
export type SlotValue<Props, Args = never> = [Args] extends [never] ? Props : Omit<Props, "children"> & { children?: (args: Args) => ReactNode; readonly [SLOT_KEY]?: (args: Args) => Key };
export type SlotArgs<S> = S extends readonly unknown[] ? never : ArgsOf<Exclude<SlotEntry<S>, NotAssigned | undefined | null>>;
export declare function slotArgs<S>(slot: S, args: SlotArgs<S>): SlotArgs<S>;
export declare function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined;
export type Slot<Props, Args = never> = SlotValue<Props, Args> | NotAssigned;
export declare const KEYED: unique symbol;
export type KeyedSlot<Props, Args = never> = { readonly [KEYED]: true } & { readonly [key: string]: SlotValue<Props, Args> };
export type SlotEntry<S> = S extends { readonly [KEYED]: true } & { readonly [key: string]: infer Entry } ? Entry | undefined : S;
export declare function slotEntry<S>(slot: S, key: string | number): SlotEntry<S>;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export type NoArgs = { readonly [arg: string]: never };
export type ArgsOf<S> = S extends { children?: infer Body } ? NonNullable<Body> extends (args: infer Args) => ReactNode ? Args : NoArgs : NoArgs;
export declare function renderSlot<S extends object>(slot: S, args: S extends readonly unknown[] ? never : ArgsOf<S>, fallback?: ReactNode): ReactNode;
export declare function Match(props: { on: unknown; children?: unknown }): never;`,
}

// With adds the core stand-in and, unless the project has its own, the JSX
// types to a project's files.
func With(files map[string]string) map[string]string {
	maps.Copy(files, Core)
	if _, ok := files["src/jsx.d.ts"]; !ok {
		files["src/jsx.d.ts"] = JSXTypes
	}
	return files
}

// simple is a project of TSConfig, the JSX types and the core stand-in.
func simple(name string, files map[string]string) Project {
	files["tsconfig.json"] = TSConfig
	files["src/jsx.d.ts"] = JSXTypes
	return Project{Name: name, Files: With(files)}
}

// Get is the project of that name.
func Get(name string) Project {
	for _, p := range Projects {
		if p.Name == name {
			return p
		}
	}
	panic("checktest: no project " + name)
}

// Projects are the projects of the goldens, but for the Vite test app
// (packages/vite/test/render), which is on disk.
var Projects = []Project{
	// Shorthand, then a type error on a copied attribute: mapped exactly.
	{Name: "check", Files: map[string]string{
		"tsconfig.json": TSConfig,
		"src/jsx.d.ts":  JSXTypes,
		"src/b.rtsx":    "import { C } from \"./c\";\nconst label = 1;\nexport const b = <C label />;\nexport const o = <div><$Title>t</$Title></div>;\n",
		"src/c.tsx":     "export function C(p: { label: string }) {\n  return <div>{p.label}</div>;\n}\nexport const n: number = \"x\";\n",
	}},

	// syntax.md, *Slots → Attachment*: args follow function-call rules.
	simple("slot-args", map[string]string{"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
interface ButtonProps {
  size: string;
  $Icon?: Slot<{ className?: string }, { size: string }>;
  $Label?: Slot<{ title?: string; children?: string }>;
  $List?: Slot<{ children?: string }>[];
}
export function Button({ size, $Icon, $Label, $List }: ButtonProps) {
  const tone = "muted";
  return (
    <button>
      <i slot={$Icon} &size />
      <i slot={$Icon} />
      <span slot={$Label} />
      <span slot={$Label} &tone />
      <span slot={$List} />
    </button>
  );
}
`}),

	// `&&name` is an arg and a prop: one source text, emitted three times.
	simple("arg-and-prop", map[string]string{"src/rows.rtsx": `import type { Slot } from "@reactogenic/core";
type P = { $Row: Slot<{ className?: string }, { className: string }>; row: { id: string } };
export function A({ $Row, row }: P) {
  return <tr slot={$Row} &&className={row.nope} />;
}
export function B({ $Row }: P) {
  return <tr slot={$Row} &&className />;
}
export function C({ $Row, row }: P) {
  return <tr slot={$Row} &&className={row.nope}>fallback</tr>;
}
export function D({ $Row }: P) {
  return <tr slot={$Row} &&className>fallback</tr>;
}
`}),

	// The attachment's children are the fallback; recursive slots and
	// last-wins type-check clean.
	simple("slots-type-check", map[string]string{
		"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export interface ButtonProps {
  variant?: string;
  $IconStart?: Slot<{ children?: JSX.Element }>;
  children?: string;
}
export function Button({ $IconStart, children }: ButtonProps) {
  return <button><span slot={$IconStart}>+</span>{children}</button>;
}
`,
		"src/dialog.rtsx": `import type { Slot } from "@reactogenic/core";
import { Button, type ButtonProps } from "./button";
export function Dialog({ $Action }: { $Action?: Slot<ButtonProps> }) {
  return <dialog><Button slot={$Action} /></dialog>;
}
export const page = (
  <Dialog>
    <$Action variant="ghost">Cancel</$Action>
    <$Action variant="solid">
      <$IconStart><b /></$IconStart>
      Close
    </$Action>
  </Dialog>
);
`,
	}),

	// Foo.tsx next to Foo.rtsx (vite.md, *Module resolution*).
	simple("ambiguous-module", map[string]string{
		"src/card.rtsx": "export const a = <p>rtsx</p>;\n",
		"src/card.tsx":  "export const a = <p>tsx</p>;\n",
	}),
	simple("ambiguous-module-is-checked", map[string]string{
		"src/card.rtsx": "export const a: number = <p>rtsx</p>;\n",
		"src/card.tsx":  "export const a = <p>tsx</p>;\n",
		"src/main.tsx":  "import { a } from \"./card\";\nimport { a as b } from \"./card.rtsx\";\nexport const s: string[] = [a, b];\n",
	}),

	// Keyed slots type-check: entries as slot values, attached by key.
	simple("keyed-slots", map[string]string{"src/table.rtsx": `import type { KeyedSlot } from "@reactogenic/core";
interface TableProps {
  columns: { name: string; label: string }[];
  $Column?: KeyedSlot<{ width?: number; children?: string }>;
}
export function Table({ columns, $Column }: TableProps) {
  return <tr>{columns.map((col) => <th key={col.name} slot={$Column}>{col.label}</th>)}</tr>;
}
export const ok = (
  <Table columns={[]}>
    <$Column key="email" width={2}>Email</$Column>
    <$Column key="name" />
  </Table>
);
export const typo = (
  <Table columns={[]}>
    <$Column key="email" widht={2} />
  </Table>
);
`}),

	// syntax.md, *Conditional slots*.
	simple("slot-conditional", map[string]string{
		"src/card.rtsx": `import type { Slot } from "@reactogenic/core";
export interface CardProps {
  $Title: Slot<{ children?: string }>;
  $Footer: Slot<{ children?: string }>;
  $Badge?: Slot<{ children?: string }>;
}
export function Card({ $Title, $Footer, $Badge }: CardProps) {
  return (
    <article>
      <h2 slot={$Title} />
      <footer slot={$Footer}>Default</footer>
      <i slot={$Badge} />
    </article>
  );
}
`,
		"src/page.rtsx": `import { Match } from "@reactogenic/core";
import { Card } from "./card";
declare const c: boolean;
export const bad = (
  <Card>
    <Match on={c}><$Title>Hi</$Title></Match>
    <$Footer />
  </Card>
);
export const fine = (
  <Card>
    <$Title>Always</$Title>
    <Match on={c}><$Title>Sometimes</$Title></Match>
    <Match on={c}><$Footer>Sometimes</$Footer></Match>
    <Match on={c}><$Badge>New</$Badge></Match>
  </Card>
);
export const later = (
  <Card>
    <Match on={c}><$Title>Sometimes</$Title></Match>
    <$Title>Always</$Title>
    <$Footer />
  </Card>
);
`,
	}),

	// syntax.md, *Key functions*.
	simple("key-functions", map[string]string{"src/select.rtsx": `import type { Slot } from "@reactogenic/core";
interface SelectProps {
  options: { value: string; label: string }[];
  $Option?: Slot<{ value?: string; children?: string }, { value: string; label: string }>;
  $Hint?: Slot<{ children?: string }>;
}
export function Select({ options, $Option, $Hint }: SelectProps) {
  return (
    <select>
      {options.map((option, i) => <option key={i} slot={$Option} &&value={option.value} &label={option.label} />)}
      <small slot={$Hint} />
    </select>
  );
}
declare function getKey(args: { value: string }): string;
export const ok = (
  <Select options={[]}>
    <$Option key={({ value }) => value} { label }>{label}</$Option>
  </Select>
);
export const noArgs = (
  <Select options={[]}>
    <$Hint key={() => "h"}>Hint</$Hint>
  </Select>
);
export const reference = (
  <Select options={[]}>
    <$Option key={getKey} { label }>{label}</$Option>
  </Select>
);
`}),

	// syntax.md, *Segment files*: the lookup order, and the import names
	// the file found.
	simple("segment-lookup", map[string]string{
		"src/page.rtsx":   "export default function Page() {\n  return <main><section #intro /><section #faq /><section #broken /></main>;\n}\n",
		"src/intro.rtsx":  "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/intro.ts":    "const notAComponent = 1;\nexport default notAComponent;\n",
		"src/faq.tsx":     "export default function Faq() {\n  return <dl />;\n}\n",
		"src/faq.ts":      "const notAComponent = 1;\nexport default notAComponent;\n",
		"src/broken.rtsx": "const notAComponent = 1;\nexport default notAComponent;\n",
	}),

	// Each mistake once: the three projects whose output the move to the
	// mapped program changed.
	simple("segment", map[string]string{
		"src/page.rtsx":  "export const page = <main><section #intro /></main>;\n",
		"src/intro.rtsx": "export default function Intro() {\n  const n: number = \"x\";\n  return <p>{n}</p>;\n}\n",
	}),
	simple("explicit-import", map[string]string{
		"src/main.tsx": "import { b } from \"./b.rtsx\";\nexport const a = b;\n",
		"src/b.rtsx":   "export const b: number = \"x\";\n",
	}),
	simple("attachment", map[string]string{
		"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export function Button({ $Label }: { $Label?: Slot<{ title?: string; children?: string }> }) {
  return <b slot={$Label} title={$Label.nope}>Button</b>;
}
`,
	}),

	// RGP1-073: every rewrite of diagnostics.md, *Rewrites*, on one project.
	rewrites(),

	// A project with nothing but .rtsx files has inputs: no TS18003.
	{Name: "rtsx-only", Files: map[string]string{
		"tsconfig.json": TSConfig,
		"src/a.rtsx":    "import { b } from \"./b\";\nexport const a: string = b;\n",
		"src/b.rtsx":    "export const b = 1;\n",
	}},

	// Vite's template: the root tsconfig has no files, only references.
	{Name: "vite-template", Files: With(map[string]string{
		"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }] }`,
		"tsconfig.app.json":  `{ "compilerOptions": { ` + Options + ` }, "include": ["src"] }`,
		"tsconfig.node.json": `{ "compilerOptions": { ` + Options + ` }, "include": ["vite.config.ts"] }`,
		"vite.config.ts":     "export default { port: 1 };\n",
		"src/main.tsx":       "import { Page } from \"./page\";\nexport const app = <Page title={1} />;\n",
		"src/page.rtsx":      "export function Page({ title }: { title: string }) {\n  const n: number = title;\n  return <h1>{n}</h1>;\n}\n",
	})},

	// A file of two referenced projects is reported once, by the first.
	{Name: "references-shared-file", Files: With(map[string]string{
		"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.test.json" }] }`,
		"tsconfig.app.json":  `{ "compilerOptions": { ` + Options + ` }, "include": ["src"] }`,
		"tsconfig.test.json": `{ "compilerOptions": { ` + Options + ` }, "include": ["src", "test"] }`,
		"src/page.rtsx":      "export const page: number = <p><$Title /></p>;\n",
		"test/page.test.tsx": "import { page } from \"../src/page\";\nexport const s: string = page;\n",
	})},

	// An .rtsx module of a referenced project is read from its source; its
	// files are its own to report, with its own options: lib is not strict.
	{Name: "cross-project", Files: With(map[string]string{
		"tsconfig.json":       `{ "files": [], "references": [{ "path": "./lib" }, { "path": "./app" }] }`,
		"lib/tsconfig.json":   `{ "compilerOptions": { ` + loose + `, "composite": true, "noEmit": false, "emitDeclarationOnly": true, "outDir": "out" }, "include": ["src", "../src/jsx.d.ts"] }`,
		"lib/src/button.rtsx": "export function Button({ size }: { size: number }) {\n  const label: string = size;\n  return <button>{label}</button>;\n}\n",
		"lib/src/util.ts":     "export const twice = (n: number) => n * 2;\nexport const pick = (o, k) => o[k];\n",
		"app/tsconfig.json":   `{ "compilerOptions": { ` + Options + ` }, "include": ["src", "../src/jsx.d.ts"], "references": [{ "path": "../lib" }] }`,
		"app/src/page.rtsx":   "import { Button } from \"../../lib/src/button\";\nimport { twice } from \"../../lib/src/util\";\nexport const page = <Button size=\"lg\">{twice(1)}</Button>;\n",
		"app/src/main.tsx":    "import { Button } from \"../../lib/src/button.rtsx\";\nexport const app = <Button size={2} />;\n",
	})},

	// Who reports a file does not depend on the order of `references`: the
	// project that lists it does. The app is strict, its tests are not: the
	// untyped parameters are the app's errors, whichever comes first.
	referencesOrder("references-order", `{ "path": "./tsconfig.test.json" }, { "path": "./tsconfig.app.json" }`, loose, Options),
	referencesOrder("references-order/app-first", `{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.test.json" }`, loose, Options),
	// The tests are strict, the app is not: they are nobody's.
	referencesOrder("references-order/strict-tests", `{ "path": "./tsconfig.test.json" }, { "path": "./tsconfig.app.json" }`, Options, loose),

	// A reference to a project that is not there is one error, the
	// referencing project's (TS6053).
	{Name: "reference-missing", Files: With(map[string]string{
		"tsconfig.json": `{ "compilerOptions": { ` + Options + ` }, "include": ["src"], "references": [{ "path": "./gone" }] }`,
		"src/a.rtsx":    "export const el = <div><$T>t</$T></div>;\n",
	})},

	// diagnostics.md, step 1: `include` matches .rtsx as it matches .tsx.
	includeExtensions("include-extensions/directory", `"src"`, nil),
	includeExtensions("include-extensions", `"src/**/*.ts", "src/**/*.tsx", "src/**/*.rtsx"`, nil),
	// No .rtsx file is listed: page.rtsx is reached by an import, and
	// unreached.rtsx by nothing.
	includeExtensions("include-extensions/ts-only", `"src/**/*.ts", "src/**/*.tsx"`, []string{"src/unreached.rtsx"}),

	// A construct that is an error where it stands and stays as written
	// leaves text that is not TSX (ide.md, *Tolerance*, rule 4).
	{Name: "unlowered", Files: With(map[string]string{
		"tsconfig.json":   TSConfig,
		"src/args.rtsx":   "declare const size: number;\ndeclare const v: number;\nexport const a = <option &size />;\nexport const b = <option slot=\"header\" &&value={v} />;\nexport const n: number = \"x\";\n",
		"src/params.rtsx": "declare const size: number;\nexport const x = <div { size }>body</div>;\n",
		"src/main.tsx":    "import { n } from \"./args\";\nexport const s: string = n;\n",
	})},

	// What an emit of declarations would report (TS4094), in a composite
	// project.
	{Name: "declarations", Files: With(map[string]string{
		"tsconfig.json": `{ "compilerOptions": { ` + strings.Replace(Options, `"noEmit": true`, `"composite": true, "emitDeclarationOnly": true, "outDir": "out"`, 1) + ` }, "include": ["src"] }`,
		"src/a.rtsx":    "export const C = class { private x = 1 };\nexport const el = <div />;\n",
	})},

	// A `paths` alias finds an .rtsx module, with and without the extension.
	{Name: "paths-alias", Files: With(map[string]string{
		"tsconfig.json":   `{ "compilerOptions": { ` + Options + `, "paths": { "@/*": ["./src/*"], "@button": ["./src/button"] } }, "include": ["src"] }`,
		"src/button.rtsx": "export function Button({ size }: { size: number }) {\n  return <button>{size}</button>;\n}\n",
		"src/page.rtsx":   "import { Button } from \"@/button\";\nimport { Button as Exact } from \"@button\";\nimport { Button as Explicit } from \"@/button.rtsx\";\nexport const page = <main><Button size=\"a\" /><Exact size=\"b\" /><Explicit size=\"c\" /></main>;\n",
		"src/main.tsx":    "import { page } from \"@/page\";\nexport const app: string = page;\n",
	})},

	// A tsconfig that lists the stock content mapper (ide.md, *Stock
	// TypeScript 7.1*) is checked as one without the entry.
	contentMappersEntry("content-mappers-entry", `, "contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]`),
	contentMappersEntry("content-mappers-entry/without", ""),

	// syntax.md, *Segment files*: the generated import names the .rtsx
	// file, which node16 / nodenext resolution finds as bundler's does.
	segmentUnder("node16"),
	segmentUnder("nodenext"),

	// A segment that mounts itself, directly and through another file.
	{Name: "segment-self", Files: With(map[string]string{
		"tsconfig.json":  TSConfig,
		"src/page.rtsx":  "export default function Page() {\n  return <main><section #intro /></main>;\n}\n",
		"src/intro.rtsx": "export default function Intro() {\n  return <div><section #outro /></div>;\n}\n",
		"src/outro.rtsx": "export default function Outro() {\n  return <div><section #intro /><section #outro /></div>;\n}\n",
	})},

	// A syntax error fails the check: the file reports its syntax errors
	// and nothing else, and the rest of the program is still checked.
	{Name: "syntax-error", SyntaxError: true, Files: With(map[string]string{
		"tsconfig.json":   TSConfig,
		"src/broken.rtsx": "export const n: number = \"x\";\nexport function Broken() {\n  return <div><$Title>t</$Title><span></div>;\n}\n",
		"src/main.tsx":    "import { Broken, n } from \"./broken\";\nexport const s: string = n;\nexport const app = <Broken />;\nexport const m: number = \"y\";\n",
	})},

	// A syntax error in a .ts / .tsx file is TypeScript's: as in tsc, no
	// types are checked while it stands.
	{Name: "tsx-syntax-error", SyntaxError: true, Files: With(map[string]string{
		"tsconfig.json": TSConfig,
		"src/page.rtsx": "export const n: number = \"x\";\nexport const page = <div><$Title>t</$Title></div>;\n",
		"src/main.tsx":  "export const app = <div><span></div>;\n",
	})},

	// A warning is printed and does not fail the check.
	{Name: "warning", Files: With(map[string]string{
		"tsconfig.json":  TSConfig,
		"src/page.rtsx":  "export const page = <section #intro>old</section>;\n",
		"src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
	})},
}

// loose is Options without `strict`.
var loose = strings.Replace(Options, `"strict": true`, `"strict": false`, 1)

func rewrites() Project {
	files := With(map[string]string{
		"tsconfig.json": TSConfig,
		"src/jsx.d.ts":  JSXTypes,
		"src/lib.tsx": `import type { Slot } from "@reactogenic/core";
type ReactNode = string | JSX.Element | undefined;
export function Card(p: { $Title?: Slot<{ tone?: string; children?: ReactNode }>; $Box?: Slot<{ color: string }>; $Label?: Slot<{ children: ReactNode }>; $Icon?: Slot<{ id?: string }, { size: string }> }) { return <div />; }
export function Must(p: { $Title: Slot<{ children?: ReactNode }> }) { return <div />; }
export function Plain(p: { className?: string }) { return <div />; }
export function Input(p: { value: string }) { return <input />; }
`,
		"src/num.rtsx":   "export default 42;\n",
		"src/props.rtsx": "export default function P(p: { x: string }) { return <div />; }\n",
		"src/ok.rtsx":    "export default function Ok() { return <div />; }\n",
		"src/page.rtsx": `import { Card, Must, Plain, Input } from "./lib";
import { Switch } from "@reactogenic/core";
declare function getStatus(): "a" | "b" | "c";
export const p1 = <Card><$Nope>x</$Nope></Card>;
export const p2 = <Must />;
export const p3 = <Card><$Title { x }>t</$Title></Card>;
export const p4 = <Card><$Box color="r">Hi</$Box></Card>;
export const p5 = <Card><$Label /></Card>;
export const p6 = <Card><$Icon>text</$Icon></Card>;
export const p7 = <Switch on={getStatus()} exhaustive><$Case is="a">A</$Case></Switch>;
export const p8 = <div #num />;
export const p9 = <div #props />;
export const p10 = <Plain #ok />;
export const p11 = <Input value />;
`,
	})
	files["node_modules/@reactogenic/core/index.d.ts"] += "\nexport declare function Switch(p: { on: unknown; exhaustive?: true; $Case?: unknown }): null;\nexport declare function noMatch(value: never): never;\n"
	return Project{Name: "rewrites", Files: files}
}

func referencesOrder(name, references, test, app string) Project {
	return Project{Name: name, Files: With(map[string]string{
		"tsconfig.json":      `{ "files": [], "references": [` + references + `] }`,
		"tsconfig.test.json": `{ "compilerOptions": { ` + test + ` }, "include": ["test", "src/jsx.d.ts"] }`,
		"tsconfig.app.json":  `{ "compilerOptions": { ` + app + ` }, "include": ["src"] }`,
		"src/page.rtsx":      "export function Page({ items }) {\n  return <ul>{items.map((i) => <li>{i}</li>)}</ul>;\n}\n",
		"src/util.ts":        "export const pick = (o, k) => o[k];\n",
		"test/page.test.tsx": "import { Page } from \"../src/page\";\nimport { pick } from \"../src/util\";\nexport const t = [<Page items={[]} />, pick];\n",
	})}
}

func includeExtensions(name, include string, unlisted []string) Project {
	return Project{Name: name, Unlisted: unlisted, Files: With(map[string]string{
		"tsconfig.json":      `{ "compilerOptions": { ` + Options + ` }, "include": [` + include + `] }`,
		"src/main.tsx":       "import { page } from \"./page\";\nexport const app = page;\n",
		"src/page.rtsx":      "export const page: number = \"x\";\n",
		"src/unreached.rtsx": "export const u: number = \"x\";\nexport const el = <div><$T>t</$T></div>;\n",
	})}
}

func contentMappersEntry(name, entry string) Project {
	return Project{Name: name, Files: With(map[string]string{
		"tsconfig.json": `{ "compilerOptions": { ` + Options + ` }, "include": ["src"]` + entry + ` }`,
		"src/page.rtsx": "import { Card } from \"./card\";\nexport const page = <Card><$Nope>x</$Nope></Card>;\n",
		"src/card.rtsx": "import type { Slot } from \"@reactogenic/core\";\nexport function Card({ $Title }: { $Title?: Slot<{ children?: string }> }) {\n  return <h2 slot={$Title} />;\n}\n",
		"src/main.tsx":  "import { page } from \"./page.rtsx\";\nexport const app: number = page;\n",
	})}
}

func segmentUnder(resolution string) Project {
	return Project{Name: "segment-" + resolution, Files: With(map[string]string{
		"package.json":   `{ "name": "app", "type": "module" }`,
		"tsconfig.json":  `{ "compilerOptions": { ` + strings.Replace(Options, `"module": "esnext", "moduleResolution": "bundler"`, `"module": "`+resolution+`", "moduleResolution": "`+resolution+`"`, 1) + ` }, "include": ["src"] }`,
		"src/page.rtsx":  "export default function Page() {\n  return <main><section #intro /><section #faq /></main>;\n}\n",
		"src/intro.rtsx": "export default function Intro({ title }: { title: string }) {\n  return <p>{title}</p>;\n}\n",
		"src/faq.tsx":    "export default function Faq() {\n  return <dl />;\n}\n",
	})}
}
