// The span map of an `.rtsx` file's virtual text, in JavaScript: the tuples
// of `reactogenic serve`'s `virtual` request (ide.md, *Span map*), with
// UTF-16 offsets. Positions are mapped both ways here; nothing else in the
// plugin does arithmetic on them.

/** [virtualStart, virtualLength, originalStart, originalLength, kind, features] */
export type Tuple = readonly [number, number, number, number, number, number];

export const VERBATIM = 0;
export const ATOM = 1;

/** The feature bits of go/internal/emit/features.go, in its order (test/spans.test.ts compares them). */
export const Feature = {
  Hover: 1 << 0,
  SignatureHelp: 1 << 1,
  Completion: 1 << 2,
  Definition: 1 << 3,
  TypeDefinition: 1 << 4,
  Implementation: 1 << 5,
  References: 1 << 6,
  DocumentHighlights: 1 << 7,
  Rename: 1 << 8,
  CallHierarchy: 1 << 9,
  CodeActions: 1 << 10,
  Formatting: 1 << 11,
  InlayHints: 1 << 12,
  SemanticTokens: 1 << 13,
  FoldingRanges: 1 << 14,
  SelectionRanges: 1 << 15,
  LinkedEditing: 1 << 16,
  AutoInsert: 1 << 17,
  DocumentSymbols: 1 << 18,
  CodeLens: 1 << 19,
} as const;

export interface Range {
  start: number;
  length: number;
}

export class SpanMap {
  /** Sorted by virtual start, covering the virtual text without gaps — as the transform writes them. */
  constructor(readonly spans: readonly Tuple[]) {}

  /** The map of a text that is its own virtual text. */
  static identity(length: number): SpanMap {
    const all = Object.values(Feature).reduce((a, b) => a | b, 0) & ~Feature.Formatting;
    return new SpanMap(length > 0 ? [[0, length, 0, length, VERBATIM, all]] : []);
  }

  /** The index of the span that holds the virtual offset; -1 outside the text. */
  private indexAt(offset: number): number {
    let low = 0;
    let high = this.spans.length - 1;
    while (low <= high) {
      const mid = (low + high) >> 1;
      const span = this.spans[mid];
      if (span[0] + span[1] <= offset) {
        low = mid + 1;
      } else if (span[0] > offset) {
        high = mid - 1;
      } else {
        return mid;
      }
    }
    return -1;
  }

  /**
   * The source range of a virtual range that lies in copied text answering
   * `feature`: the exact one, or none. A range may run through several
   * copies when they are one run of the source. An empty range belongs to the
   * copy it starts in, else to the one that ends there.
   */
  toSource(start: number, length: number, feature: number): Range | undefined {
    if (length === 0) {
      const at = this.indexAt(start);
      const here = at >= 0 ? this.spans[at] : undefined;
      if (here && answers(here, feature)) {
        return { start: here[2] + start - here[0], length: 0 };
      }
      const before = this.spans[at >= 0 ? at - 1 : this.spans.length - 1];
      return before && before[0] + before[1] === start && answers(before, feature) ? { start: before[2] + before[3], length: 0 } : undefined;
    }
    let at = this.indexAt(start);
    if (at < 0 || !answers(this.spans[at], feature)) {
      return undefined;
    }
    const first = this.spans[at];
    const end = start + length;
    for (let span = first; span[0] + span[1] < end; ) {
      const next = this.spans[++at];
      if (!next || !answers(next, feature) || next[2] !== span[2] + span[3]) {
        return undefined;
      }
      span = next;
    }
    return { start: first[2] + start - first[0], length };
  }

  /**
   * The source range between two virtual offsets that are each in copied
   * text, whatever lies between them: a declaration with lowered constructs
   * inside. None when an end is in generated text, or the two are out of order.
   */
  toSourceEnds(start: number, length: number): Range | undefined {
    if (length === 0) {
      return this.toSource(start, 0, 0);
    }
    const first = this.spans[this.indexAt(start)];
    const last = this.spans[this.indexAt(start + length - 1)];
    if (!first || !last || first[4] !== VERBATIM || last[4] !== VERBATIM) {
      return undefined;
    }
    const from = first[2] + start - first[0];
    const to = last[2] + start + length - last[0];
    return from <= to ? { start: from, length: to - from } : undefined;
  }

  /**
   * As a diagnostic is placed (emit.Map.Source): exact inside copied text; an
   * end that falls in generated text widens to the construct that generated it.
   */
  toSourceWide(start: number, length: number): Range | undefined {
    if (this.spans.length === 0) {
      return undefined;
    }
    // An offset at the end of the text is in the last span.
    const clamp = (offset: number) => {
      const at = this.indexAt(offset);
      return this.spans[at >= 0 ? at : this.spans.length - 1];
    };
    const first = clamp(start);
    let from = first[4] === VERBATIM ? first[2] + Math.min(start - first[0], first[3]) : first[2];
    let to = first[4] === VERBATIM ? from : first[2] + first[3];
    if (length > 0) {
      const last = clamp(start + length - 1);
      to = last[4] === VERBATIM ? last[2] + start + length - last[0] : last[2] + last[3];
    }
    if (from > to) {
      [from, to] = [to, from];
    }
    return { start: from, length: to - from };
  }

  /**
   * The virtual offset of a source offset: in the first copy that answers
   * `feature`; the end of such a copy when the offset is in none.
   */
  toVirtual(offset: number, feature: number): number | undefined {
    let atEnd: number | undefined;
    for (const span of this.spans) {
      if (!answers(span, feature)) {
        continue;
      }
      if (span[2] <= offset && offset < span[2] + span[3]) {
        return span[0] + offset - span[2];
      }
      if (atEnd === undefined && offset === span[2] + span[3]) {
        atEnd = span[0] + span[1];
      }
    }
    return atEnd;
  }
}

function answers(span: Tuple, feature: number): boolean {
  return span[4] === VERBATIM && (span[5] & feature) === feature;
}

/** The offsets at which the lines of `text` start, as TypeScript counts lines. */
export function lineStarts(text: string): number[] {
  const starts = [0];
  for (let i = 0; i < text.length; i++) {
    const c = text.charCodeAt(i);
    if (c === 13 && text.charCodeAt(i + 1) === 10) {
      i++;
    }
    if (c === 10 || c === 13 || c === 0x2028 || c === 0x2029) {
      starts.push(i + 1);
    }
  }
  return starts;
}

/** Zero-based line and character of an offset, clamped to the text. */
export function lineAndCharacter(starts: readonly number[], offset: number): { line: number; character: number } {
  let low = 0;
  let high = starts.length - 1;
  while (low < high) {
    const mid = (low + high + 1) >> 1;
    if (starts[mid] <= offset) {
      low = mid;
    } else {
      high = mid - 1;
    }
  }
  return { line: low, character: Math.max(0, offset - starts[low]) };
}
