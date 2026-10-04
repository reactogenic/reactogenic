// The TS server plugin `reactogenic-typescript-plugin` (ide.md, *The `.ts`
// side*): tsserver — VS Code's built-in TypeScript, TS 5 and 6 — learns that
// `import { Page } from "./page"` is `page.rtsx`, a module whose text is the
// emitted TSX. It is loaded into every TypeScript project: it runs nothing
// until an import resolves to an `.rtsx` file, and until then does one thing —
// an import TypeScript does not resolve is looked up once more, once per
// folder.
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
/** An import found its `.rtsx` file and no text for it: a binary that is configured later needs the projects loaded again. */
let starved = false;
/** The projects — by directory — whose imports found no binary at all: one that appears needs them loaded again. */
const without = new Set<string>();
let reloading = false;

/** What the plugin keeps of each project it serves. */
interface Instance {
  /** The language service as TypeScript made it: its answers are in virtual positions. */
  service: ts.LanguageService;
  /** The configuration changed. */
  reconfigured(): void;
}
const instances = new Map<ts.server.Project, Instance>();

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

  function create(info: ts.server.PluginCreateInfo): ts.LanguageService {
    const host = info.languageServiceHost;
    const project = info.project;
    const projects = project.projectService;
    const log = (message: string) => projects.logger.info(`reactogenic: ${message}`);
    // tsserver enables a project's plugins again each time it loads the
    // project's configuration (Reload Project, an edit of tsconfig.json), and
    // hands over the host and the language service as they are: wrapped by
    // this plugin already. Wrapped twice, a text would be transformed twice
    // and a position mapped twice.
    if (instances.has(project)) {
      return info.languageService;
    }
    const resolveModuleNameLiterals = host.resolveModuleNameLiterals?.bind(host);
    if (!resolveModuleNameLiterals) {
      log(`TypeScript ${typescript.version} is older than 5.0: .rtsx imports are not resolved`);
      return info.languageService;
    }
    // VS Code runs a second tsserver that answers while the first one loads
    // the projects (`--serverMode partialSemantic`): a process run from it
    // would make it wait too, and make every text twice.
    const mode = (projects as { serverMode?: ts.LanguageServiceMode }).serverMode;
    if (mode !== undefined && mode !== typescript.LanguageServiceMode.Semantic) {
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
    /** Per file: the specifiers written in it that resolve to an `.rtsx` module, each with its file. */
    const imports = new Map<string, Map<string, string>>();
    let generation = files.generation;
    /** Since when a last good text waits to be made again, in this project. */
    let waiting: number | undefined;
    /** A failure in here is the plugin's, and must not be the request's: said once per place. */
    const said = new Set<string>();
    const failed = (where: string, error: unknown) => {
      if (!said.has(where)) {
        said.add(where);
        log(`${where} failed, TypeScript's own answer stands: ${error instanceof Error ? (error.stack ?? error.message) : String(error)}`);
      }
    };

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
    /**
     * The failed resolutions that were looked up a second time, and found no
     * `.rtsx` file either. TypeScript keeps one object per name and folder —
     * for a package, one for all the folders below where it looked last — and
     * makes a new one when a file appears at a place it had looked at: the
     * second lookup is made once per object and folder, not once per importer.
     */
    const missed = new WeakMap<object, { folders: Set<string>; watched: Set<string> }>();
    /** How many second lookups were made, and how many of them the log has. */
    let looked = 0;
    let reported = 0;

    host.resolveModuleNameLiterals = (literals, containingFile, redirectedReference, options, containingSourceFile, reusedNames) => {
      const resolved = resolveModuleNameLiterals(literals, containingFile, redirectedReference, options, containingSourceFile, reusedNames);
      try {
        return withRtsx(resolved, literals, containingFile, redirectedReference, options, containingSourceFile);
      } catch (error) {
        failed("the lookup of .rtsx modules", error);
        return resolved;
      }
    };

    function withRtsx(
      resolved: readonly ts.ResolvedModuleWithFailedLookupLocations[],
      literals: readonly ts.StringLiteralLike[],
      containingFile: string,
      redirectedReference: ts.ResolvedProjectReference | undefined,
      options: ts.CompilerOptions,
      containingSourceFile: ts.SourceFile,
    ): readonly ts.ResolvedModuleWithFailedLookupLocations[] {
      const found: { index: number; file: string; external?: boolean }[] = [];
      let names = imports.get(containingFile);
      let folder: string | undefined;
      resolved.forEach((own, index) => {
        const name = literals[index].text;
        names?.delete(name);
        if (own.resolvedModule) {
          return; // built-in extensions win
        }
        folder ??= containingFile.slice(0, containingFile.lastIndexOf("/"));
        let miss = missed.get(own);
        if (miss?.folders.has(folder)) {
          return;
        }
        looked++;
        const mode = typescript.getModeForUsageLocation(containingSourceFile, literals[index], options);
        const result = resolveRtsx(typescript, info.serverHost, name, containingFile, options, redirectedReference, mode, lookups(options));
        if (result.file !== undefined) {
          found.push({ index, file: result.file, external: result.external });
          return;
        }
        if (!miss) {
          miss = { folders: new Set(), watched: new Set() };
          missed.set(own, miss);
        }
        miss.folders.add(folder);
        watch(own, result.missing, miss.watched);
      });
      if (found.length === 0) {
        return resolved;
      }
      // The files this importer needs, in one process. Without their text —
      // no binary, or one that fails — the modules stay unresolved, as they
      // are without the plugin.
      const binary = files.ensure(
        directory,
        found.map((module) => module.file),
        read,
      );
      if (!binary) {
        without.add(directory);
      } else if (without.delete(directory)) {
        // The imports that found no binary are unresolved in TypeScript's own cache.
        reload(projects, log, "a binary has appeared");
      }
      const result = resolved.slice();
      for (const { index, file, external } of found) {
        if (!files.has(file)) {
          starved = true;
          continue;
        }
        if (!names) {
          names = new Map();
          imports.set(containingFile, names);
        }
        names.set(literals[index].text, file);
        result[index] = {
          resolvedModule: { resolvedFileName: file, extension: typescript.Extension.Tsx, isExternalLibraryImport: external ?? false, resolvedUsingTsExtension: false },
        };
      }
      return result;
    }

    /**
     * TypeScript resolves a failed import again when a file appears at one of
     * the places it looked. The `.rtsx` files the plugin looked for are added
     * to them, so that creating `page.rtsx` clears the importer's TS2307
     * without an edit to it. (They are also how TypeScript finds an import to
     * update after a folder with `.rtsx` files was moved.)
     */
    function watch(own: ts.ResolvedModuleWithFailedLookupLocations, missing: readonly string[], watched: Set<string>): void {
      if (missing.length === 0) {
        return;
      }
      const lookedAt = own as { failedLookupLocations?: string[] };
      if (!Array.isArray(lookedAt.failedLookupLocations)) {
        return; // a shape this version does not have: the import is resolved at the importer's next edit
      }
      for (const file of missing) {
        if (!watched.has(file)) {
          watched.add(file);
          lookedAt.failedLookupLocations.push(file);
        }
      }
    }

    // ---- text ---------------------------------------------------------------

    const getScriptSnapshot = host.getScriptSnapshot.bind(host);
    const getScriptVersion = host.getScriptVersion.bind(host);
    const getScriptKind = host.getScriptKind?.bind(host);

    host.getScriptKind = (fileName) => (isRtsx(fileName) ? typescript.ScriptKind.TSX : (getScriptKind?.(fileName) ?? typescript.ScriptKind.Unknown));
    // A text changes with its source, with the binary that makes it, and when
    // a last good one is made again.
    host.getScriptVersion = (fileName) => {
      const version = getScriptVersion(fileName);
      if (!isRtsx(fileName)) {
        return version;
      }
      try {
        return `${version}:${files.generation}:${files.binaryId(directory)}:${held.get(fileName)?.attempt ?? 0}`;
      } catch (error) {
        failed("the version of an .rtsx file", error);
        return version;
      }
    };
    host.getScriptSnapshot = (fileName) => {
      // tsserver's own snapshot is the saved file: it reads it, watches it,
      // and keeps it for the lines and columns of what is answered.
      const source = getScriptSnapshot(fileName);
      if (!source || !isRtsx(fileName)) {
        return source;
      }
      try {
        return virtualSnapshot(fileName, source);
      } catch (error) {
        // The program must be built all the same: the text it had, else the source.
        failed("the text of an .rtsx file", error);
        const was = held.get(fileName);
        if (was) {
          was.current = false;
        }
        return was?.snapshot ?? source;
      }
    };

    function virtualSnapshot(fileName: string, source: ts.IScriptSnapshot): ts.IScriptSnapshot {
      const was = held.get(fileName);
      if (was && was.source === source && was.generation === files.generation && (was.current || was.asked === was.attempt)) {
        return was.snapshot;
      }
      const code = source.getText(0, source.getLength());
      // After a change of binary every text of the project is due: all of
      // them with the first one asked for. A file that yields nothing keeps
      // its last good text; one that never had any stands in as its own (its
      // declarations are TSX all the same).
      const others = was && was.generation !== files.generation ? [...held.keys()] : [];
      const virtual = files.get(directory, fileName, code, others, read) ?? { source: code, text: code, map: SpanMap.identity(code.length), binary: "" };
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
    }

    /**
     * tsserver asks for a text again only when the project has changed. A
     * last good text has to be asked for without that: before a request, now
     * and then, the files that have one get a new version and the project is
     * marked as changed.
     */
    const again = () => {
      if (looked !== reported) {
        // What the plugin costs a project that has no `.rtsx`: in the log, so that it can be counted.
        log(`${looked - reported} unresolved import(s) looked up as .rtsx`);
        reported = looked;
      }
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

    // ---- the project's file list -------------------------------------------

    /**
     * The `.rtsx` files the project's tsconfig lists — `files`, and `include`
     * with `.rtsx` among the extensions it matches, as in `reactogenic check`
     * — for one version of the project. Read when a composite project asks
     * (TS6307), not before: it walks the project's folders.
     */
    let roots: { version: string | undefined; files: Set<string> } | undefined;
    const listed = (fileName: string): boolean => {
      const version = host.getProjectVersion?.();
      if (!roots || version === undefined || roots.version !== version) {
        roots = { version, files: new Set() };
        const config = project.getCompilerOptions().configFilePath;
        if (typeof config === "string") {
          const system = info.serverHost;
          const parsed = typescript.getParsedCommandLineOfConfigFile(
            config,
            undefined,
            {
              useCaseSensitiveFileNames: system.useCaseSensitiveFileNames,
              readDirectory: (...args) => system.readDirectory(...args),
              fileExists: (file) => system.fileExists(file),
              readFile: (file) => system.readFile(file),
              getCurrentDirectory: () => directory,
              onUnRecoverableConfigFileDiagnostic: () => {},
            },
            undefined,
            undefined,
            // Deferred: the one kind TypeScript adds to the extensions `include` matches.
            [{ extension: ".rtsx", isMixedContent: false, scriptKind: typescript.ScriptKind.Deferred }],
          );
          for (const file of parsed?.fileNames ?? []) {
            if (isRtsx(file)) {
              roots.files.add(typescript.server.toNormalizedPath(file));
            }
          }
        }
      }
      return roots.files.has(fileName);
    };

    // ---- configuration ----------------------------------------------------

    const reconfigured = () => {
      if (generation === files.generation) {
        return;
      }
      generation = files.generation;
      if (starved && files.binary(directory) !== undefined) {
        // Imports that found no text are unresolved in TypeScript's own
        // cache: only loading the projects again resolves them.
        reload(projects, log, "a binary is configured");
      } else if (held.size > 0) {
        changed(); // the texts are made again at the next request
        project.refreshDiagnostics();
      }
    };
    for (const known of instances.keys()) {
      if ((known as { isClosed?(): boolean }).isClosed?.()) {
        instances.delete(known);
      }
    }
    instances.set(project, { service: info.languageService, reconfigured });

    return decorate(info.languageService, {
      request: again,
      active: () => held.size > 0,
      anywhere: () => files.has(),
      virtual: (fileName) => {
        const now = held.get(fileName);
        return now?.current ? now.virtual : undefined;
      },
      imported: (fileName, specifier) => imports.get(fileName)?.get(specifier),
      listed,
      specifier: (fileName, specifier) => {
        // `./page` for `./page.rtsx` when the shorter name is this file too:
        // no built-in sibling, which the lookup would find first.
        const bare = specifier.slice(0, -".rtsx".length);
        const options = project.getCompilationSettings();
        const mode = info.languageService.getProgram()?.getSourceFile(fileName)?.impliedNodeFormat;
        const named = resolveRtsx(typescript, info.serverHost, specifier, fileName, options, undefined, mode).file;
        return named !== undefined && resolveRtsx(typescript, info.serverHost, bare, fileName, options, undefined, mode).file === named ? bare : specifier;
      },
      display: (fileName) => {
        const prefix = `${directory}/`;
        return fileName.startsWith(prefix) ? fileName.slice(prefix.length) : fileName;
      },
      peers: (fileName) => {
        const script = projects.getScriptInfo(fileName);
        return (script?.containingProjects ?? []).flatMap((other) => {
          const instance = other === project ? undefined : instances.get(other);
          if (!instance || (other as { isClosed?(): boolean }).isClosed?.()) {
            return []; // a project the plugin does not serve has no `.rtsx` file
          }
          other.getLanguageService(); // brings its program up to date, as tsserver does before it asks
          return [instance.service];
        });
      },
      languageServer: () => files.languageServer,
    });
  }

  return {
    create,
    onConfigurationChanged(config: Config) {
      transformer?.configure(config);
      // tsserver tells, for each project, the module it made last — which,
      // once a project was loaded again, is not the one that serves it.
      // Every project is told here; one that has heard does nothing.
      for (const instance of [...instances.values()]) {
        instance.reconfigured();
      }
    },
  };
}

/** Loads every project again, once for all that ask in the same turn: the one way to have TypeScript resolve again what it has as unresolved. */
function reload(projects: ts.server.ProjectService, log: (message: string) => void, why: string): void {
  if (reloading) {
    return;
  }
  reloading = true;
  setImmediate(() => {
    reloading = false;
    starved = false;
    without.clear(); // what still has none says so again, while it loads
    log(`${why}: reloading the projects`);
    projects.reloadProjects();
  });
}

// tsserver calls what the module exports: the build makes this the module's
// `module.exports` (scripts/build.mjs).
export default init;
