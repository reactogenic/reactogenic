// Module resolution for `.rtsx` (ide.md, *The plugin*, *Resolution*):
// TypeScript's own first; on failure, `x.rtsx`. Built-in extensions win, and
// `paths`, `index.rtsx` and an explicit `./x.rtsx` behave as in the server.
import type * as ts from "typescript";

export interface Found {
  /** The `.rtsx` file the name resolves to. */
  file?: string;
  external?: boolean;
  /** The `.rtsx` files that would have resolved the name, had they existed. */
  missing: string[];
}

/** What the lookup reads of the file system. */
export interface Files {
  fileExists(path: string): boolean;
  readFile(path: string): string | undefined;
  directoryExists?(path: string): boolean;
  getDirectories?(path: string): string[];
  realpath?(path: string): string;
  getCurrentDirectory(): string;
  useCaseSensitiveFileNames: boolean;
}

/**
 * Called once TypeScript has not resolved `name`: its own algorithm again,
 * with `x.rtsx` seen as `x.tsx` — so `.rtsx` comes after every built-in
 * extension, a file before a directory's `index.rtsx`, and `paths`, `baseUrl`
 * and packages are followed as TypeScript follows them. An explicit
 * `./x.rtsx` is tried as `x.rtsx.tsx`, as TypeScript tries `x.rtsx.ts`.
 */
export function resolveRtsx(
  typescript: typeof ts,
  files: Files,
  name: string,
  containingFile: string,
  options: ts.CompilerOptions,
  redirectedReference: ts.ResolvedProjectReference | undefined,
  mode: ts.ResolutionMode,
  cache?: ts.ModuleResolutionCache,
): Found {
  // A relative specifier that names the file is the file, in every resolution mode.
  if (name.endsWith(".rtsx") && (name.startsWith("./") || name.startsWith("../"))) {
    const file = typescript.server.toNormalizedPath(`${containingFile.slice(0, containingFile.lastIndexOf("/") + 1)}${name}`);
    return files.fileExists(file) ? { file, missing: [] } : { missing: [file] };
  }
  /** The `.rtsx` file that stands behind a `.tsx` name TypeScript asks for. */
  const behind = (asked: string): string | undefined => {
    if (!asked.endsWith(".tsx")) {
      return undefined;
    }
    const base = asked.slice(0, -".tsx".length);
    return base.endsWith(".rtsx") ? base : `${base}.rtsx`;
  };
  const standsIn = (asked: string): string | undefined => {
    const file = behind(asked);
    return file !== undefined && files.fileExists(file) ? file : undefined;
  };
  const host: ts.ModuleResolutionHost = {
    fileExists: (path) => files.fileExists(path) || standsIn(path) !== undefined,
    readFile: (path) => files.readFile(path),
    directoryExists: files.directoryExists && ((path) => files.directoryExists!(path)),
    getDirectories: files.getDirectories && ((path) => files.getDirectories!(path)),
    getCurrentDirectory: () => files.getCurrentDirectory(),
    useCaseSensitiveFileNames: files.useCaseSensitiveFileNames,
    realpath:
      files.realpath &&
      ((path) => {
        const file = standsIn(path);
        if (file === undefined) {
          return files.realpath!(path);
        }
        const real = files.realpath!(file);
        return real.endsWith(".rtsx") ? `${real}.tsx` : path;
      }),
  };
  const result = typescript.resolveModuleName(name, containingFile, options, host, cache, redirectedReference, mode);
  const resolved = result.resolvedModule;
  const file = resolved && standsIn(resolved.resolvedFileName);
  if (file !== undefined) {
    return { file, external: resolved?.isExternalLibraryImport, missing: [] };
  }
  // Not exported by TypeScript's types; present in every version this runs in.
  const failed: readonly string[] = (result as { failedLookupLocations?: readonly string[] }).failedLookupLocations ?? [];
  return { missing: failed.flatMap((location) => behind(location) ?? []) };
}
