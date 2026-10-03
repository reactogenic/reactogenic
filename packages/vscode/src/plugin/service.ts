// The language service tsserver answers from, with every position in an
// `.rtsx` file mapped (ide.md, *The plugin*, *Answers*): tsserver computes on
// the virtual text; what leaves this file is in source positions, or is not
// there. Positions that come in for an `.rtsx` file are source positions too.
import type * as ts from "typescript";
import { Feature, lineAndCharacter } from "./spans";
import { sourceLineStarts, type Virtual } from "./virtual";

/** What the service needs of the project it serves. */
export interface Program {
  /** Called before every request. */
  request(): void;
  /** True once the project has an `.rtsx` file: until then nothing below costs anything. */
  active(): boolean;
  /** True once any project of this tsserver has one: a file two projects share leads from one into the other. */
  anywhere(): boolean;
  /**
   * The virtual text the program holds for an `.rtsx` file; undefined when it
   * is not the text of the file's current source (a last good text).
   */
  virtual(fileName: string): Virtual | undefined;
  /** The `.rtsx` file that the specifier, written in that file, resolves to. */
  imported(fileName: string, specifier: string): string | undefined;
  /** True when the project's tsconfig lists the `.rtsx` file, as `reactogenic check` reads it: `include` matches `.rtsx`. */
  listed(fileName: string): boolean;
  /** A specifier that ends in `.rtsx`, without the extension when that is the same module from that file. */
  specifier(fileName: string, specifier: string): string;
  /** For a message: the path as the user knows it. */
  display(fileName: string): string;
  /**
   * The other loaded projects that hold the file, each as its language
   * service — TypeScript's own, which answers in virtual positions.
   */
  peers(fileName: string): ts.LanguageService[];
  /** True while `reactogenic lsp` lists the workspace symbols of `.rtsx` files (the extension says so): not listed here too. */
  languageServer(): boolean;
}

export function isRtsx(fileName: string): boolean {
  return fileName.endsWith(".rtsx");
}

export function decorate(service: ts.LanguageService, program: Program): ts.LanguageService {
  // Every method, the internal ones too: tsserver calls some that the types do not list.
  const proxy: ts.LanguageService = Object.create(null);
  for (const key of Object.keys(service) as (keyof ts.LanguageService)[]) {
    const method = service[key] as unknown;
    (proxy as unknown as Record<string, unknown>)[key] = typeof method === "function" ? (...args: unknown[]) => method.apply(service, args) : method;
  }

  // ---- positions out: virtual → source -----------------------------------

  /** An exact span in copied text that answers `feature`; undefined: the location is dropped. */
  const exact = (fileName: string, span: ts.TextSpan, feature: number): ts.TextSpan | undefined => {
    if (!isRtsx(fileName)) {
      return span;
    }
    if (span.start === 0 && span.length === 0) {
      return span; // the file itself: the target of a relative module specifier
    }
    const virtual = program.virtual(fileName);
    if (!virtual) {
      return undefined;
    }
    return virtual.map.toSource(span.start, span.length, feature);
  };
  /** A declaration around a name: its two ends, whatever was lowered between them. */
  const around = (fileName: string, span: ts.TextSpan | undefined): ts.TextSpan | undefined => {
    if (span === undefined || !isRtsx(fileName)) {
      return span;
    }
    return program.virtual(fileName)?.map.toSourceEnds(span.start, span.length);
  };
  /**
   * A whole declaration, for a list of symbols: its two ends — and where its
   * end was lowered (a tag that was rebuilt), as wide as the construct there.
   * None for a declaration that starts in generated text: not the author's.
   */
  const declaration = (fileName: string, span: ts.TextSpan): ts.TextSpan | undefined => {
    const map = program.virtual(fileName)?.map;
    if (!map || span.length === 0 || !map.toSourceEnds(span.start, 1)) {
      return undefined;
    }
    return map.toSourceEnds(span.start, span.length) ?? map.toSourceWide(span.start, span.length);
  };
  const documentSpan = <T extends ts.DocumentSpan>(location: T, feature: number): T | undefined => {
    if (!isRtsx(location.fileName)) {
      return location;
    }
    // A module declared by its file — where a specifier that is not relative
    // (`@/page`) leads — is the text from its first token to its end: the
    // file's start, as for a relative specifier, and as in the server.
    const text = program.virtual(location.fileName)?.text;
    if ((location as { kind?: string }).kind === "module" && text !== undefined && location.textSpan.start + location.textSpan.length === text.length) {
      return { ...location, textSpan: { start: 0, length: 0 }, contextSpan: undefined };
    }
    const textSpan = exact(location.fileName, location.textSpan, feature);
    return textSpan && { ...location, textSpan, contextSpan: around(location.fileName, location.contextSpan) };
  };
  const documentSpans = <T extends ts.DocumentSpan>(locations: readonly T[] | undefined, feature: number): T[] | undefined =>
    locations && unique(locations.flatMap((location) => documentSpan(location, feature) ?? []));

  // ---- positions in: source → virtual ------------------------------------

  /** The position to ask TypeScript at; undefined: it is in no copied text that answers `feature`. */
  const ask = (fileName: string, position: number, feature: number): number | undefined =>
    isRtsx(fileName) ? program.virtual(fileName)?.map.toVirtual(position, feature) : position;

  /** Wraps `(fileName, position, …) => result`: maps the position, and gives `none` where it has no virtual one. */
  const at = <K extends keyof ts.LanguageService>(key: K, feature: number, none: unknown, map: (result: any, fileName: string) => unknown): void => {
    const method = service[key] as unknown as (fileName: string, position: number, ...rest: unknown[]) => unknown;
    (proxy as unknown as Record<string, unknown>)[key] = (fileName: string, position: number, ...rest: unknown[]) => {
      if (!program.active()) {
        return method.call(service, fileName, position, ...rest);
      }
      if (isRtsx(fileName)) {
        service.getProgram(); // the position is of the file as saved: its map must be that text's
      }
      const virtual = ask(fileName, position, feature);
      return virtual === undefined ? none : map(method.call(service, fileName, virtual, ...rest), fileName);
    };
  };

  // ---- definitions, references -------------------------------------------

  at("getDefinitionAtPosition", Feature.Definition, undefined, (result) => documentSpans(result, Feature.Definition));
  at("getDefinitionAndBoundSpan", Feature.Definition, undefined, (result: ts.DefinitionInfoAndBoundSpan | undefined, fileName) => {
    const textSpan = result && exact(fileName, result.textSpan, Feature.Definition);
    return result && textSpan && { definitions: documentSpans(result.definitions, Feature.Definition), textSpan };
  });
  at("getTypeDefinitionAtPosition", Feature.TypeDefinition, undefined, (result) => documentSpans(result, Feature.TypeDefinition));
  at("getImplementationAtPosition", Feature.Implementation, undefined, (result) => documentSpans(result, Feature.Implementation));
  at("getReferencesAtPosition", Feature.References, undefined, (result) => documentSpans(result, Feature.References));
  at("findReferences", Feature.References, undefined, (result: ts.ReferencedSymbol[] | undefined) =>
    result?.flatMap((symbol) => {
      const references = documentSpans(symbol.references, Feature.References)!;
      let definition = documentSpan(symbol.definition, Feature.Definition);
      if (!definition) {
        // Declared in generated code. Kept, on the construct that generated
        // it, only as the heading of references the author wrote.
        const wide = program.virtual(symbol.definition.fileName)?.map.toSourceWide(symbol.definition.textSpan.start, symbol.definition.textSpan.length);
        if (!wide || references.length === 0) {
          return [];
        }
        definition = { ...symbol.definition, textSpan: wide, contextSpan: undefined };
      }
      return [{ definition, references }];
    }),
  );
  at("getDocumentHighlights", Feature.DocumentHighlights, undefined, (result: ts.DocumentHighlights[] | undefined) =>
    result?.flatMap((file) => {
      const highlightSpans = file.highlightSpans.flatMap((span) => {
        const fileName = span.fileName ?? file.fileName;
        const textSpan = exact(fileName, span.textSpan, Feature.DocumentHighlights);
        return textSpan ? [{ ...span, textSpan, contextSpan: around(fileName, span.contextSpan) }] : [];
      });
      return highlightSpans.length > 0 ? [{ ...file, highlightSpans }] : [];
    }),
  );
  proxy.getFileReferences = (fileName) => {
    const result = service.getFileReferences(fileName);
    return program.active() ? documentSpans(result, Feature.References)! : result;
  };

  // ---- call hierarchy -----------------------------------------------------

  const item = (call: ts.CallHierarchyItem): ts.CallHierarchyItem | undefined => {
    if (!isRtsx(call.file)) {
      return call;
    }
    const selectionSpan = exact(call.file, call.selectionSpan, Feature.CallHierarchy);
    return selectionSpan && { ...call, selectionSpan, span: around(call.file, call.span) ?? selectionSpan };
  };
  const spansIn = (fileName: string, spans: readonly ts.TextSpan[]): ts.TextSpan[] => spans.flatMap((span) => exact(fileName, span, Feature.CallHierarchy) ?? []);
  at("prepareCallHierarchy", Feature.CallHierarchy, undefined, (result: ts.CallHierarchyItem | ts.CallHierarchyItem[] | undefined) => {
    if (Array.isArray(result)) {
      return result.flatMap((call) => item(call) ?? []);
    }
    return result && item(result);
  });
  at("provideCallHierarchyIncomingCalls", Feature.CallHierarchy, [], (result: ts.CallHierarchyIncomingCall[]) =>
    result.flatMap((call) => {
      // A call written in generated code — or in a slot's body, a function
      // the transform made — has no caller in the source: not listed.
      const from = item(call.from);
      const fromSpans = from ? spansIn(from.file, call.fromSpans) : [];
      return from && fromSpans.length > 0 ? [{ from, fromSpans }] : [];
    }),
  );
  at("provideCallHierarchyOutgoingCalls", Feature.CallHierarchy, [], (result: ts.CallHierarchyOutgoingCall[], fileName) =>
    result.flatMap((call) => {
      const to = item(call.to);
      const fromSpans = spansIn(fileName, call.fromSpans);
      return to && fromSpans.length > 0 ? [{ to, fromSpans }] : [];
    }),
  );

  // ---- workspace-wide answers ----------------------------------------------

  // Symbols declared in `.rtsx` files are listed by `reactogenic lsp` (ide.md,
  // the feature table): listed here too, each would show twice. While that
  // server has no project — no `.rtsx` document open, a workspace that is not
  // trusted — nobody else lists them: here, at the declaration's place in
  // the source.
  proxy.getNavigateToItems = (...args) => {
    const result = service.getNavigateToItems(...args);
    if (!program.active()) {
      return result;
    }
    const theirs = program.languageServer();
    return result.flatMap((found) => {
      if (!isRtsx(found.fileName)) {
        return [found];
      }
      const textSpan = theirs ? undefined : declaration(found.fileName, found.textSpan);
      return textSpan ? [{ ...found, textSpan }] : [];
    });
  };

  // An edit in an `.rtsx` file is never tsserver's: its text is the saved
  // one, and the language server makes that edit, from the buffer. An import
  // of an `.rtsx` module in a TypeScript file is updated here as any other:
  // the editor asks after the move, when the language server's edit — made
  // before it — is already in the file, and nothing is left to update.
  proxy.getEditsForFileRename = (...args) => {
    const result = service.getEditsForFileRename(...args);
    if (!program.active()) {
      return result;
    }
    const current = service.getProgram();
    return result.flatMap((change) => {
      if (isRtsx(change.fileName)) {
        return [];
      }
      // TypeScript names the file (`./page.rtsx`) where the author wrote
      // `./page`: an importer that moves with its module would have every
      // such import rewritten. Kept as the author wrote it, where that is
      // still the same module.
      const text = current?.getSourceFile(change.fileName)?.text;
      const textChanges = change.textChanges.flatMap((edit) => {
        const written = text?.slice(edit.span.start, edit.span.start + edit.span.length);
        if (!isRtsx(edit.newText) || written === undefined || isRtsx(written)) {
          return [edit];
        }
        const newText = short(change.fileName, edit.newText);
        return newText === written ? [] : [{ ...edit, newText }];
      });
      return textChanges.length > 0 ? [{ ...change, textChanges }] : [];
    });
  };

  // ---- rename: refused when it reaches an `.rtsx` file ---------------------

  const reaches = (fileName: string): string =>
    `This rename reaches ${program.display(fileName)}: start it from that file, where Reactogenic's language server writes it back.`;
  const refusal = (fileName: string): ts.RenameInfoFailure => ({ canRename: false, localizedErrorMessage: reaches(fileName) });
  const reached = (locations: readonly ts.RenameLocation[] | undefined): string | undefined => locations?.find((location) => isRtsx(location.fileName))?.fileName;
  proxy.getRenameInfo = (fileName: string, position: number, ...rest: unknown[]): ts.RenameInfo => {
    const getRenameInfo = service.getRenameInfo as (fileName: string, position: number, ...rest: unknown[]) => ts.RenameInfo;
    if (!program.anywhere()) {
      return getRenameInfo(fileName, position, ...rest);
    }
    if (isRtsx(fileName)) {
      return refusal(fileName);
    }
    const info = getRenameInfo(fileName, position, ...rest);
    if (!info.canRename) {
      return info;
    }
    // What the rename would touch, as tsserver asks for it next: in this
    // project, and in every other project that holds a file it touches — a
    // file two projects share takes the rename into both, whichever was
    // asked. Each project once, from the first place that leads into it.
    const preferences = (typeof rest[0] === "object" && rest[0] !== null ? rest[0] : {}) as ts.UserPreferences;
    const searched = new Set([service]);
    const queue = [{ service, fileName, position }];
    for (let next = queue.shift(); next; next = queue.shift()) {
      const locations = next.service.findRenameLocations(next.fileName, next.position, false, false, preferences);
      const file = reached(locations);
      if (file !== undefined) {
        return refusal(file);
      }
      for (const location of locations ?? []) {
        for (const peer of program.peers(location.fileName)) {
          if (!searched.has(peer)) {
            searched.add(peer);
            queue.push({ service: peer, fileName: location.fileName, position: location.textSpan.start });
          }
        }
      }
    }
    return info;
  };
  // What getRenameInfo could not see — a project tsserver loads for the
  // rename itself (one that references the project asked), a caller that
  // does not ask it first — fails the request: with a project's locations
  // left out, the rename would be a partial one.
  proxy.findRenameLocations = ((fileName: string, position: number, ...rest: unknown[]) => {
    const findRenameLocations = service.findRenameLocations as (fileName: string, position: number, ...rest: unknown[]) => readonly ts.RenameLocation[] | undefined;
    if (isRtsx(fileName)) {
      throw new Error(reaches(fileName));
    }
    const locations = findRenameLocations(fileName, position, ...rest);
    const file = reached(locations);
    if (file !== undefined) {
      throw new Error(reaches(file));
    }
    return locations;
  }) as ts.LanguageService["findRenameLocations"];

  // ---- edits: none into an `.rtsx` file -----------------------------------

  const touches = (changes: readonly ts.FileTextChanges[]): string | undefined => changes.find((change) => isRtsx(change.fileName))?.fileName;
  /**
   * An import TypeScript writes for an `.rtsx` module names the file,
   * `./page.rtsx`: it does not know the extension can go. Written as the
   * convention is — `./page` — where that is the same module (ide.md,
   * *Specifiers the server writes*).
   */
  // The specifier of an import, an export, a `require` or an `import()`: an
  // edit may hold the author's code too (a statement moved to another file),
  // and a string in it that happens to end in `.rtsx` is not one to change.
  const WRITTEN = /(\bfrom\s*|\bimport\s*\(?\s*|\brequire\s*\(\s*)(["'])([^"'\n]+\.rtsx)\2/g;
  /** In a text TypeScript shows: any quoted name. */
  const SHOWN = /()(["'])([^"'\n]+\.rtsx)\2/g;
  /** `./page` for `./page.rtsx`, where that is the same module from the file. Asked once per request: a list of completions names a module many times. */
  const shortened = new Map<string, string>();
  const short = (fileName: string, specifier: string): string => {
    const key = `${fileName}\0${specifier}`;
    let name = shortened.get(key);
    if (name === undefined) {
      name = program.specifier(fileName, specifier);
      shortened.set(key, name);
    }
    return name;
  };
  const specifiers = (fileName: string, text: string, pattern: RegExp): string =>
    text.includes(".rtsx") ? text.replace(pattern, (_, before: string, quote: string, specifier: string) => `${before}${quote}${short(fileName, specifier)}${quote}`) : text;
  const imports = (changes: readonly ts.FileTextChanges[]): ts.FileTextChanges[] =>
    changes.map((change) => ({
      ...change,
      textChanges: change.textChanges.map((edit) => (edit.newText.includes(".rtsx") ? { ...edit, newText: specifiers(change.fileName, edit.newText, WRITTEN) } : edit)),
    }));
  /** An action on the file `fileName`: its edits, and the label that names the module. */
  const action = <T extends ts.CodeAction>(fileName: string, fix: T): T => ({ ...fix, description: specifiers(fileName, fix.description, SHOWN), changes: imports(fix.changes) });
  proxy.getCodeFixesAtPosition = (fileName, ...rest) => {
    const result = service.getCodeFixesAtPosition(fileName, ...rest);
    return program.active() ? result.filter((fix) => touches(fix.changes) === undefined).map((fix) => action(fileName, fix)) : result;
  };
  proxy.getCombinedCodeFix = (...args) => {
    const result = service.getCombinedCodeFix(...args);
    if (!program.active()) {
      return result;
    }
    return touches(result.changes) !== undefined ? { changes: [] } : { ...result, changes: imports(result.changes) };
  };
  proxy.getEditsForRefactor = (...args) => {
    const result = service.getEditsForRefactor(...args);
    if (!program.active() || !result) {
      return result;
    }
    const file = touches(result.edits);
    return file === undefined
      ? { ...result, edits: imports(result.edits) } // a moved statement takes its imports along
      : { edits: [], notApplicableReason: `This refactoring edits ${program.display(file)}, which TypeScript cannot write: make the change in that file.` };
  };
  if (typeof service.getPasteEdits === "function") {
    // Pasted code brings its imports (TypeScript 5.7 and later).
    proxy.getPasteEdits = (...args) => {
      const result = service.getPasteEdits(...args);
      return program.active() && result ? { ...result, edits: imports(result.edits) } : result;
    };
  }
  if (typeof service.getMoveToRefactoringFileSuggestions === "function") {
    // *Move to file* lists the program's files by extension, and fails on
    // one TypeScript does not know ("has unknown extension"): for the length
    // of the call the program has no `.rtsx` files — none is a target anyway.
    proxy.getMoveToRefactoringFileSuggestions = (...args) => {
      const current = program.active() ? service.getProgram() : undefined;
      if (!current) {
        return service.getMoveToRefactoringFileSuggestions(...args);
      }
      const getSourceFiles = current.getSourceFiles;
      current.getSourceFiles = () => getSourceFiles.call(current).filter((file) => !isRtsx(file.fileName));
      try {
        return service.getMoveToRefactoringFileSuggestions(...args);
      } finally {
        current.getSourceFiles = getSourceFiles;
      }
    };
  }

  // ---- diagnostics --------------------------------------------------------

  // An `.rtsx` file's own diagnostics are `reactogenic lsp`'s. In another
  // file, a diagnostic may point into one: placed as `check` places it.
  const stand = new WeakMap<Virtual, ts.SourceFile>();
  const related = (information: ts.DiagnosticRelatedInformation): ts.DiagnosticRelatedInformation | undefined => {
    if (!information.file || !isRtsx(information.file.fileName) || information.start === undefined) {
      return information;
    }
    const virtual = program.virtual(information.file.fileName);
    const range = virtual?.map.toSourceWide(information.start, information.length ?? 0);
    if (!virtual || !range) {
      return undefined;
    }
    // tsserver reads the file's name, and its text for line and column.
    let file = stand.get(virtual);
    if (!file) {
      const starts = sourceLineStarts(virtual);
      file = {
        fileName: information.file.fileName,
        text: virtual.source,
        getLineAndCharacterOfPosition: (position: number) => lineAndCharacter(starts, position),
        getLineStarts: () => starts,
      } as unknown as ts.SourceFile;
      stand.set(virtual, file);
    }
    return { ...information, file, start: range.start, length: range.length };
  };
  // TS6307, "File is not listed within the file list of project": in a
  // composite project every file must be a root, and tsserver's `include`
  // never matches an `.rtsx` file — in `check` it does. The error stands on
  // the import that brought the file in, and stays for a file that `check`
  // does not find listed either.
  const NOT_LISTED = 6307;
  const unlisted = (fileName: string, diagnostic: ts.Diagnostic): boolean => {
    const text = diagnostic.file?.text;
    if (text === undefined || diagnostic.start === undefined || !diagnostic.length) {
      return false;
    }
    const file = program.imported(fileName, text.slice(diagnostic.start + 1, diagnostic.start + diagnostic.length - 1));
    return file !== undefined && program.listed(file);
  };
  const diagnostics =
    <T extends ts.Diagnostic>(get: (fileName: string) => T[]) =>
    (fileName: string): T[] => {
      if (!program.active()) {
        return get(fileName);
      }
      if (isRtsx(fileName)) {
        return [];
      }
      return get(fileName).flatMap((diagnostic) => {
        if (diagnostic.code === NOT_LISTED && unlisted(fileName, diagnostic)) {
          return [];
        }
        return diagnostic.relatedInformation?.some((information) => information.file && isRtsx(information.file.fileName))
          ? [{ ...diagnostic, relatedInformation: diagnostic.relatedInformation.flatMap((information) => related(information) ?? []) }]
          : [diagnostic];
      });
    };
  proxy.getSemanticDiagnostics = diagnostics((fileName) => service.getSemanticDiagnostics(fileName));
  proxy.getSyntacticDiagnostics = diagnostics((fileName) => service.getSyntacticDiagnostics(fileName));
  proxy.getSuggestionDiagnostics = diagnostics((fileName) => service.getSuggestionDiagnostics(fileName));

  // ---- text in an answer: `{@link}` targets --------------------------------

  const parts = <T extends ts.SymbolDisplayPart>(list: T[] | undefined): T[] | undefined =>
    list?.map((part) => {
      const target = (part as { target?: ts.DocumentSpan }).target;
      if (!target || !isRtsx(target.fileName)) {
        return part;
      }
      // The target is a whole declaration, whatever was lowered inside it.
      const textSpan = around(target.fileName, target.textSpan);
      // Without a target the part is plain text: tsserver follows every link.
      return textSpan ? { ...part, target: { ...target, textSpan, contextSpan: around(target.fileName, target.contextSpan) } } : ({ text: part.text, kind: "text" } as T);
    });
  const tags = (list: ts.JSDocTagInfo[] | undefined): ts.JSDocTagInfo[] | undefined => list?.map((tag) => (tag.text ? { ...tag, text: parts(tag.text) } : tag));
  at("getQuickInfoAtPosition", Feature.Hover, undefined, (result: ts.QuickInfo | undefined, fileName) => {
    const textSpan = result && exact(fileName, result.textSpan, Feature.Hover);
    return result && textSpan && { ...result, textSpan, documentation: parts(result.documentation), tags: tags(result.tags) };
  });
  // A completion that imports: the module is named in the list (`source`,
  // `sourceDisplay`, the label's description), and for an import statement
  // being typed the entry's text is the statement itself — inserted as it
  // is, without a request for its details.
  const shown = <T extends ts.SymbolDisplayPart>(fileName: string, list: T[] | undefined): T[] | undefined =>
    list?.map((part) => (isRtsx(part.text) ? { ...part, text: short(fileName, part.text) } : part));
  proxy.getCompletionsAtPosition = (fileName, ...rest) => {
    const result = service.getCompletionsAtPosition(fileName, ...rest);
    if (!result || !program.active()) {
      return result;
    }
    return {
      ...result,
      entries: result.entries.map((entry) => {
        if (!(entry.source && isRtsx(entry.source)) && !entry.insertText?.includes(".rtsx")) {
          return entry;
        }
        return {
          ...entry,
          // Where it is the specifier TypeScript resolved: the entry is then
          // found again by its `data`. Else it is the module's path, by
          // which TypeScript finds the entry again: as it is.
          source: entry.source && entry.source === entry.data?.moduleSpecifier ? short(fileName, entry.source) : entry.source,
          sourceDisplay: shown(fileName, entry.sourceDisplay),
          labelDetails: entry.labelDetails?.description && isRtsx(entry.labelDetails.description) ? { ...entry.labelDetails, description: short(fileName, entry.labelDetails.description) } : entry.labelDetails,
          insertText: entry.insertText && specifiers(fileName, entry.insertText, WRITTEN),
        };
      }),
    };
  };
  proxy.getCompletionEntryDetails = (fileName, ...rest) => {
    const result = service.getCompletionEntryDetails(fileName, ...rest);
    if (!result || !program.active()) {
      return result;
    }
    // An auto-import: its edits are in the file the completion is in.
    const codeActions = result.codeActions?.map((fix) => action(fileName, fix));
    return { ...result, codeActions, sourceDisplay: shown(fileName, result.sourceDisplay), documentation: parts(result.documentation), tags: tags(result.tags) };
  };
  proxy.getSignatureHelpItems = (...args) => {
    const result = service.getSignatureHelpItems(...args);
    if (!result || !program.active()) {
      return result;
    }
    return {
      ...result,
      items: result.items.map((signature) => ({
        ...signature,
        documentation: parts(signature.documentation)!,
        tags: tags(signature.tags)!,
        parameters: signature.parameters.map((parameter) => ({ ...parameter, documentation: parts(parameter.documentation)! })),
      })),
    };
  };

  // ---- line and column ----------------------------------------------------

  // tsserver turns the offsets above into lines with the program's file, whose
  // text is virtual: for an `.rtsx` file the lines are the source's.
  proxy.toLineColumnOffset = (fileName, position) => {
    const virtual = isRtsx(fileName) ? program.virtual(fileName) : undefined;
    if (!virtual || position === 0) {
      return service.toLineColumnOffset!(fileName, position);
    }
    return lineAndCharacter(sourceLineStarts(virtual), position);
  };

  // Every request passes by the project first.
  const methods = proxy as unknown as Record<string, unknown>;
  for (const key of Object.keys(methods)) {
    const method = methods[key];
    if (typeof method === "function") {
      methods[key] = (...args: unknown[]) => {
        program.request();
        shortened.clear();
        return method(...args);
      };
    }
  }
  return proxy;
}

/** One location once: two copies of a token are one place in the source. */
function unique<T extends ts.DocumentSpan>(locations: T[]): T[] {
  const seen = new Set<string>();
  return locations.filter((location) => {
    if (!isRtsx(location.fileName)) {
      return true;
    }
    const key = `${location.fileName}\0${location.textSpan.start}\0${location.textSpan.length}`;
    return !seen.has(key) && seen.add(key);
  });
}
