package lsptest

import (
	"os"
	"path/filepath"
	"testing"
)

// TSConfig is the tsconfig of test projects.
const TSConfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true
  },
  "include": ["src"]
}`

// JSXTypes is a minimal JSX namespace, as `src/jsx.d.ts`.
const JSXTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
}`

// Project writes files into a fresh directory and returns it. A
// `tsconfig.json` and `src/jsx.d.ts` are added unless given; "" as a file's
// text leaves it out.
func Project(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]string{"tsconfig.json": TSConfig, "src/jsx.d.ts": JSXTypes}
	for name, text := range files {
		all[name] = text
	}
	write(t, dir, all)
	return filepath.ToSlash(dir)
}

// Write writes exactly these files into a fresh directory and returns it: a
// project that is given whole, with nothing added.
func Write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, files)
	return filepath.ToSlash(dir)
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		if text == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Core is a stub of @reactogenic/core's types, as files of a project.
var Core = map[string]string{
	"node_modules/@reactogenic/core/package.json": `{ "name": "@reactogenic/core", "types": "index.d.ts" }`,
	"node_modules/@reactogenic/core/index.d.ts": `type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol }; // as React: a function is not a node
type Key = string | number | bigint;
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
export declare const SLOT_KEY: unique symbol;
export type SlotValue<Props, Args = never> = [Args] extends [never] ? Props : Omit<Props, "children"> & { children?: (args: Args) => ReactNode; readonly [SLOT_KEY]?: (args: Args) => Key };
export type Slot<Props, Args = never> = SlotValue<Props, Args> | NotAssigned;
export declare const KEYED: unique symbol;
export type KeyedSlot<Props, Args = never> = { readonly [KEYED]: true } & { readonly [key: string]: SlotValue<Props, Args> };
export type SlotEntry<S> = S extends { readonly [KEYED]: true } & { readonly [key: string]: infer Entry } ? Entry | undefined : S;
export declare function slotEntry<S>(slot: S, key: string | number): SlotEntry<S>;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export type NoArgs = { readonly [arg: string]: never };
export type ArgsOf<S> = S extends { children?: infer Body } ? NonNullable<Body> extends (args: infer Args) => ReactNode ? Args : NoArgs : NoArgs;
export type SlotArgs<S> = S extends readonly unknown[] ? never : ArgsOf<Exclude<SlotEntry<S>, NotAssigned | undefined | null>>;
export declare function slotArgs<S>(slot: S, args: SlotArgs<S>): SlotArgs<S>;
export declare function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined;
export declare function renderSlot<S extends object>(slot: S, args: S extends readonly unknown[] ? never : ArgsOf<S>, fallback?: ReactNode): ReactNode;
export declare function Match(props: { on: unknown; children?: unknown }): never;
export declare function Switch(props: { on: unknown; exhaustive?: true; $Case?: unknown; children?: unknown }): null;
export declare function noMatch(value: never): never;
export declare function Each<T>(props: { items: readonly T[]; children: (params: { item: T; index: number }) => ReactNode }): ReactNode;
export declare function slotEntryName(key: string | number): string;
export declare function slotKeys(slot: unknown): string[];`,
}

// With returns files plus more, for Project.
func With(files map[string]string, more ...map[string]string) map[string]string {
	all := map[string]string{}
	for _, m := range append([]map[string]string{files}, more...) {
		for name, text := range m {
			all[name] = text
		}
	}
	return all
}
