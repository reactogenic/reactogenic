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
  /**
   * The virtual text the program holds for an `.rtsx` file; undefined when it
   * is not the text of the file's current source (a last good text).
   */
  virtual(fileName: string): Virtual | undefined;
  /** True when the specifier, written in that file, resolves to an `.rtsx` module. */
  importsRtsx(fileName: string, specifier: string): boolean;
  /** For a message: the path as the user knows it. */
  display(fileName: string): string;
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
    const mapped = virtual.map.toSource(span.start, span.length, feature);
    if (!mapped && span.start + span.length === virtual.text.length) {
      // From the first token to the end of the text: the module, as declared
      // by its file — where an aliased specifier (`@/page`) leads. In the
      // source that is the file from its first token, when that one is copied.
      const start = virtual.map.toSource(span.start, 0, 0)?.start ?? 0;
      return { start, length: virtual.source.length - start };
    }
    return mapped;
  };
  /** A declaration around a name: its two ends, whatever was lowered between them. */
  const around = (fileName: string, span: ts.TextSpan | undefined): ts.TextSpan | undefined => {
    if (span === undefined || !isRtsx(fileName)) {
      return span;
    }
    return program.virtual(fileName)?.map.toSourceEnds(span.start, span.length);
  };
  const documentSpan = <T extends ts.DocumentSpan>(location: T, feature: number): T | undefined => {
    if (!isRtsx(location.fileName)) {
      return location;
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

  // ---- workspace-wide answers that the language server gives --------------

  // Symbols declared in `.rtsx` files are listed by `reactogenic lsp` (ide.md,
  // the feature table): listed here too, each would show twice.
  proxy.getNavigateToItems = (...args) => {
    const result = service.getNavigateToItems(...args);
    return program.active() ? result.filter((found) => !isRtsx(found.fileName)) : result;
  };

  // The same division as in the server, from the other side: an edit in an
  // `.rtsx` file, or of an import of an `.rtsx` module, is the server's —
  // made here as well, it would be applied twice.
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
      const text = current?.getSourceFile(change.fileName)?.text;
      const textChanges = change.textChanges.filter((edit) => {
        const specifier = text?.slice(edit.span.start, edit.span.start + edit.span.length);
        return !isRtsx(edit.newText) && !(specifier !== undefined && (isRtsx(specifier) || program.importsRtsx(change.fileName, specifier)));
      });
      return textChanges.length > 0 ? [{ ...change, textChanges }] : [];
    });
  };

  // ---- rename: refused when it reaches an `.rtsx` file ---------------------

  const refusal = (fileName: string): ts.RenameInfoFailure => ({
    canRename: false,
    localizedErrorMessage: `This rename reaches ${program.display(fileName)}: start it from that file, where Reactogenic's language server writes it back.`,
  });
  const reached = (locations: readonly ts.RenameLocation[] | undefined): string | undefined => locations?.find((location) => isRtsx(location.fileName))?.fileName;
  proxy.getRenameInfo = (fileName: string, position: number, ...rest: unknown[]): ts.RenameInfo => {
    const getRenameInfo = service.getRenameInfo as (fileName: string, position: number, ...rest: unknown[]) => ts.RenameInfo;
    if (!program.active()) {
      return getRenameInfo(fileName, position, ...rest);
    }
    if (isRtsx(fileName)) {
      return refusal(fileName);
    }
    const info = getRenameInfo(fileName, position, ...rest);
    if (!info.canRename) {
      return info;
    }
    // What the rename would touch, as tsserver will ask for it next.
    const preferences = (typeof rest[0] === "object" && rest[0] !== null ? rest[0] : {}) as ts.UserPreferences;
    const file = reached(service.findRenameLocations(fileName, position, false, false, preferences));
    return file === undefined ? info : refusal(file);
  };
  // For a caller that does not ask getRenameInfo first: no locations, rather than some.
  proxy.findRenameLocations = ((fileName: string, position: number, ...rest: unknown[]) => {
    const findRenameLocations = service.findRenameLocations as (fileName: string, position: number, ...rest: unknown[]) => readonly ts.RenameLocation[] | undefined;
    if (!program.active()) {
      return findRenameLocations(fileName, position, ...rest);
    }
    if (isRtsx(fileName)) {
      return undefined;
    }
    const locations = findRenameLocations(fileName, position, ...rest);
    return reached(locations) === undefined ? locations : undefined;
  }) as ts.LanguageService["findRenameLocations"];

  // ---- edits: none into an `.rtsx` file -----------------------------------

  const touches = (changes: readonly ts.FileTextChanges[]): string | undefined => changes.find((change) => isRtsx(change.fileName))?.fileName;
  proxy.getCodeFixesAtPosition = (...args) => {
    const result = service.getCodeFixesAtPosition(...args);
    return program.active() ? result.filter((fix) => touches(fix.changes) === undefined) : result;
  };
  proxy.getCombinedCodeFix = (...args) => {
    const result = service.getCombinedCodeFix(...args);
    return program.active() && touches(result.changes) !== undefined ? { changes: [] } : result;
  };
  proxy.getEditsForRefactor = (...args) => {
    const result = service.getEditsForRefactor(...args);
    const file = program.active() && result ? touches(result.edits) : undefined;
    return file === undefined ? result : { edits: [], notApplicableReason: `This refactoring edits ${program.display(file)}, which TypeScript cannot write: make the change in that file.` };
  };
  if (typeof service.getMoveToRefactoringFileSuggestions === "function") {
    proxy.getMoveToRefactoringFileSuggestions = (...args) => {
      const result = service.getMoveToRefactoringFileSuggestions(...args);
      return program.active() ? { ...result, files: result.files.filter((file) => !isRtsx(file)) } : result;
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
  const diagnostics =
    <T extends ts.Diagnostic>(get: (fileName: string) => T[]) =>
    (fileName: string): T[] => {
      if (!program.active()) {
        return get(fileName);
      }
      if (isRtsx(fileName)) {
        return [];
      }
      return get(fileName).map((diagnostic) =>
        diagnostic.relatedInformation?.some((information) => information.file && isRtsx(information.file.fileName))
          ? { ...diagnostic, relatedInformation: diagnostic.relatedInformation.flatMap((information) => related(information) ?? []) }
          : diagnostic,
      );
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
      const mapped = documentSpan(target, Feature.Definition);
      // Without a target the part is plain text: tsserver follows every link.
      return mapped ? { ...part, target: mapped } : ({ text: part.text, kind: "text" } as T);
    });
  const tags = (list: ts.JSDocTagInfo[] | undefined): ts.JSDocTagInfo[] | undefined => list?.map((tag) => (tag.text ? { ...tag, text: parts(tag.text) } : tag));
  at("getQuickInfoAtPosition", Feature.Hover, undefined, (result: ts.QuickInfo | undefined, fileName) => {
    const textSpan = result && exact(fileName, result.textSpan, Feature.Hover);
    return result && textSpan && { ...result, textSpan, documentation: parts(result.documentation), tags: tags(result.tags) };
  });
  proxy.getCompletionEntryDetails = (...args) => {
    const result = service.getCompletionEntryDetails(...args);
    return result && program.active() ? { ...result, documentation: parts(result.documentation), tags: tags(result.tags) } : result;
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
