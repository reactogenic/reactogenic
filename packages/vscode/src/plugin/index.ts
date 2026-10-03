// The TS server plugin `reactogenic-typescript-plugin` (ide.md, *The `.ts`
// side*): tsserver — VS Code's built-in TypeScript, TS 5 and 6 — learns that
// `import { Page } from "./page"` is `page.rtsx`, a module whose text is the
// emitted TSX. It is loaded into every TypeScript project, so it does nothing,
// and runs nothing, until an import resolves to an `.rtsx` file.
//
// Bundled into one CommonJS file without dependencies (scripts/build.mjs);
// in the .vsix it is node_modules/reactogenic-typescript-plugin, where
// tsserver finds it by name.
import type * as ts from "typescript";
import { resolveRtsx } from "./resolution";
import { decorate, isRtsx } from "./service";
import { SpanMap } from "./spans";
import { type Config, RETRY_AFTER, Transformer, type Virtual } from "./virtual";

/** One per tsserver process: a file is transformed once for all its projects. */
let transformer: Transformer | undefined;
/** An import found its `.rtsx` file and no binary to read it with: a binary that comes later needs the projects loaded again. */
let starved = false;
let reloading = false;

/** The text of one `.rtsx` file in one project's program. */
interface Held {
  /** tsserver's snapshot of the source, and the generation, it was made for. */
  source: ts.IScriptSnapshot;
  generation: number;
  virtual: Virtual;
  /** False for a last good text: the source has changed since, and no position maps. */
  current: boolean;
  snapshot: ts.IScriptSnapshot;
  /** A last good text is asked for again now and then: how often it is due, and was asked for. */
  attempt: number;
  asked: number;
}

function init(modules: { typescript: typeof ts }): ts.server.PluginModule {
  const typescript = modules.typescript;
  let reconfigured: (() => void) | undefined;

  function create(info: ts.server.PluginCreateInfo): ts.LanguageService {
    const host = info.languageServiceHost;
    const project = info.project;
    const projects = project.projectService;
    const log = (message: string) => projects.logger.info(`reactogenic: ${message}`);
    const resolveModuleNameLiterals = host.resolveModuleNameLiterals?.bind(host);
    if (!resolveModuleNameLiterals) {
      log(`TypeScript ${typescript.version} is older than 5.0: .rtsx imports are not resolved`);
      return info.languageService;
    }
    transformer ??= new Transformer(log);
    const files = transformer;
    // The configuration is the client's (`configurePlugin`), never the
    // project's: a tsconfig entry of this plugin's name must not choose the
    // binary. A client that configured before this project was created left it here.
    const overrides = (projects as unknown as { currentPluginConfigOverrides?: Map<string, Config> }).currentPluginConfigOverrides;
    const configured = overrides?.get(info.config?.name);
    if (configured) {
      files.configure(configured);
    }

    const directory = project.getCurrentDirectory();
    const read = (file: string) => info.serverHost.readFile(file);
    const held = new Map<string, Held>();
    /** Per file: the specifiers written in it that resolve to an `.rtsx` module. */
    const imports = new Map<string, Set<string>>();
    let generation = files.generation;
    /** Since when a last good text waits to be made again, in this project. */
    let waiting: number | undefined;

    // ---- resolution -------------------------------------------------------

    // What the second lookup finds out about folders and packages holds for
    // one build of the program: a project whose imports mostly fail (a clone
    // before its install) asks the same questions from every file.
    let cache: { version: string | undefined; lookups: ts.ModuleResolutionCache } | undefined;
    const lookups = (options: ts.CompilerOptions): ts.ModuleResolutionCache => {
      const version = host.getProjectVersion?.();
      if (!cache || version === undefined || cache.version !== version) {
        const name = info.serverHost.useCaseSensitiveFileNames ? (file: string) => file : (file: string) => file.toLowerCase();
        cache = { version, lookups: typescript.createModuleResolutionCache(directory, name, options) };
      }
      return cache.lookups;
    };

    host.resolveModuleNameLiterals = (literals, containingFile, redirectedReference, options, containingSourceFile, reusedNames) => {
      const resolved = resolveModuleNameLiterals(literals, containingFile, redirectedReference, options, containingSourceFile, reusedNames);
      const found: { index: number; file: string; external?: boolean }[] = [];
      let names = imports.get(containingFile);
      resolved.forEach((own, index) => {
        const name = literals[index].text;
        names?.delete(name);
        if (own.resolvedModule) {
          return; // built-in extensions win
        }
        const mode = typescript.getModeForUsageLocation(containingSourceFile, literals[index], options);
        const result = resolveRtsx(typescript, info.serverHost, name, containingFile, options, redirectedReference, mode, lookups(options));
        if (result.file !== undefined) {
          found.push({ index, file: result.file, external: result.external });
        } else {
          watch(own, result.missing);
        }
      });
      if (found.length === 0) {
        return resolved;
      }
      // The files this importer needs, in one process. Without their text —
      // no binary, or one that fails — the modules stay unresolved, as they
      // are without the plugin.
      files.ensure(
        directory,
        found.map((module) => module.file),
        read,
      );
      const result = resolved.slice();
      for (const { index, file, external } of found) {
        if (!files.has(file)) {
          starved = true;
          continue;
        }
        if (!names) {
          names = new Set();
          imports.set(containingFile, names);
        }
        names.add(literals[index].text);
        result[index] = {
          resolvedModule: { resolvedFileName: file, extension: typescript.Extension.Tsx, isExternalLibraryImport: external ?? false, resolvedUsingTsExtension: false },
        };
      }
      return result;
    };

    /**
     * TypeScript resolves a failed import again when a file appears at one of
     * the places it looked. The `.rtsx` files the plugin looked for are added
     * to them, so that creating `page.rtsx` clears the importer's TS2307
     * without an edit to it.
     */
    function watch(own: ts.ResolvedModuleWithFailedLookupLocations, missing: readonly string[]): void {
      if (missing.length === 0) {
        return;
      }
      const failed = own as { failedLookupLocations?: string[] };
      if (!Array.isArray(failed.failedLookupLocations)) {
        return; // a shape this version does not have: the import is resolved at the importer's next edit
      }
      for (const file of missing) {
        if (!failed.failedLookupLocations.includes(file)) {
          failed.failedLookupLocations.push(file);
        }
      }
    }

    // ---- text ---------------------------------------------------------------

    const getScriptSnapshot = host.getScriptSnapshot.bind(host);
    const getScriptVersion = host.getScriptVersion.bind(host);
    const getScriptKind = host.getScriptKind?.bind(host);

    host.getScriptKind = (fileName) => (isRtsx(fileName) ? typescript.ScriptKind.TSX : (getScriptKind?.(fileName) ?? typescript.ScriptKind.Unknown));
    // A text changes with its source, and with the binary that makes it.
    // A text changes with its source, with the binary that makes it, and when
    // a last good one is made again.
    host.getScriptVersion = (fileName) => (isRtsx(fileName) ? `${getScriptVersion(fileName)}:${files.generation}:${held.get(fileName)?.attempt ?? 0}` : getScriptVersion(fileName));
    host.getScriptSnapshot = (fileName) => {
      // tsserver's own snapshot is the saved file: it reads it, watches it,
      // and keeps it for the lines and columns of what is answered.
      const source = getScriptSnapshot(fileName);
      if (!source || !isRtsx(fileName)) {
        return source;
      }
      const was = held.get(fileName);
      if (was && was.source === source && was.generation === files.generation && (was.current || was.asked === was.attempt)) {
        return was.snapshot;
      }
      const code = source.getText(0, source.getLength());
      // A file that yields nothing keeps its last good text; one that never
      // had any stands in as its own (its declarations are TSX all the same).
      const virtual = files.get(directory, fileName, code, [], read) ?? { source: code, text: code, map: SpanMap.identity(code.length), binary: "" };
      const now: Held = {
        source,
        generation: files.generation,
        virtual,
        current: virtual.source === code,
        snapshot: was?.virtual === virtual ? was.snapshot : typescript.ScriptSnapshot.fromString(virtual.text),
        attempt: was?.attempt ?? 0,
        asked: was?.attempt ?? 0,
      };
      held.set(fileName, now);
      if (!now.current) {
        waiting = Date.now();
      }
      return now.snapshot;
    };

    /**
     * tsserver asks for a text again only when the project has changed. A
     * last good text has to be asked for without that: before a request, now
     * and then, the files that have one get a new version and the project is
     * marked as changed.
     */
    const again = () => {
      if (waiting === undefined || Date.now() - waiting < RETRY_AFTER) {
        return;
      }
      waiting = undefined;
      for (const now of held.values()) {
        if (!now.current) {
          now.attempt++;
          waiting = Date.now();
        }
      }
      if (waiting !== undefined) {
        changed();
      }
    };
    /** The program is out of date though no file changed: tsserver builds it again at the next request. */
    const changed = () => (project as unknown as { markAsDirty?(): void }).markAsDirty?.();

    // ---- configuration ----------------------------------------------------

    reconfigured = () => {
      if (generation === files.generation) {
        return;
      }
      generation = files.generation;
      if (starved && files.binary(directory) !== undefined) {
        // Imports that found no binary are unresolved in TypeScript's own
        // cache: only loading the projects again resolves them.
        if (!reloading) {
          reloading = true;
          setImmediate(() => {
            reloading = false;
            starved = false;
            log("a binary is configured: reloading the projects");
            projects.reloadProjects();
          });
        }
      } else if (held.size > 0) {
        changed(); // the texts are made again at the next request
        project.refreshDiagnostics();
      }
    };

    return decorate(info.languageService, {
      request: again,
      active: () => held.size > 0,
      virtual: (fileName) => {
        const now = held.get(fileName);
        return now?.current ? now.virtual : undefined;
      },
      importsRtsx: (fileName, specifier) => imports.get(fileName)?.has(specifier) ?? false,
      display: (fileName) => {
        const prefix = `${directory}/`;
        return fileName.startsWith(prefix) ? fileName.slice(prefix.length) : fileName;
      },
    });
  }

  return {
    create,
    onConfigurationChanged(config: Config) {
      transformer?.configure(config);
      reconfigured?.();
    },
  };
}

// tsserver calls what the module exports: the build makes this the module's
// `module.exports` (scripts/build.mjs).
export default init;
