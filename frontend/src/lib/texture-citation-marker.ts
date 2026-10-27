// Citation markers toggle: a numbered marker inside cited text opens and
// closes its own plain source note, which sits in the flow of the text under
// the paragraph that cites it. No popover ever overlays the text and the
// marker's own box never changes, so the sentence it marks does not reflow
// (owner direction, 2026-10-27: "a citation marker is a toggle, and nothing
// overlays the text").
//
// The note is rendered with the document from load and starts open; a marker
// collapses its note to a hidden state that keeps the note's box, so the
// paragraph above it does not move either. Every marker turns only the note it
// owns: the note shares its data-citation-anchor, or is the note inside its
// own text block when the marker names no anchor. A marker whose note cannot
// be found turns nothing rather than guessing, which is what keeps two
// citations in one paragraph independent.

export const CITATION_NOTE_SELECTOR = '[data-texture-source-note]';
export const CITATION_MARKER_SELECTOR = '[data-texture-source-ref]';
const CITATION_BLOCK_SELECTOR = 'p, li, h1, h2, h3, h4, h5, h6, blockquote, td, th';
// Anchor ids are generated ids; any value outside this shape yields an
// empty selector value (no match) instead of an invalid selector.
const ANCHOR_SHAPE = /^[A-Za-z0-9_.:-]*$/;

function anchorValue(value: any): string {
  const text = String(value || '');
  return ANCHOR_SHAPE.test(text) ? text : '';
}

/** A marker's or note's citation anchor, '' when it names none. */
export function citationAnchorOf(node: any): string {
  return String(node?.getAttribute?.('data-citation-anchor') || '');
}

/** The DOM id a citation note for an anchor carries. */
export function citationNoteIDForAnchor(anchorId: string): string {
  return 'texture-source-note-' + anchorId;
}

/** The note an anchor owns inside a scope, or null. */
export function citationNoteByAnchor(anchor: string, scope: any): any {
  const owner = scope || (typeof document !== 'undefined' ? document : null);
  const value = anchorValue(anchor);
  if (!value || !owner?.querySelector) return null;
  return owner.querySelector(CITATION_NOTE_SELECTOR + '[data-citation-anchor="' + value + '"]');
}

/** Every marker of an anchor inside a scope. */
export function citationMarkersForAnchor(anchor: string, scope: any): any[] {
  const owner = scope || (typeof document !== 'undefined' ? document : null);
  const value = anchorValue(anchor);
  if (!value || !owner?.querySelectorAll) return [];
  return Array.from(owner.querySelectorAll(CITATION_MARKER_SELECTOR + '[data-citation-anchor="' + value + '"]'));
}

/**
 * The note a marker toggles: the note for its citation anchor inside the
 * surface; else, for a marker that names no anchor, the note inside its own
 * text block. A marker that names an anchor it cannot resolve toggles nothing.
 */
export function citationNoteForMarker(marker: any, surface: any = null): any {
  if (!marker) return null;
  const anchor = citationAnchorOf(marker);
  const scopes = [];
  if (surface) scopes.push(surface);
  const doc = marker.ownerDocument;
  if (doc?.body && doc.body !== surface) scopes.push(doc.body);
  if (anchor) {
    for (const scope of scopes) {
      const own = citationNoteByAnchor(anchor, scope);
      if (own) return own;
    }
    return null;
  }
  const block = marker.closest?.(CITATION_BLOCK_SELECTOR);
  return block?.querySelector?.(CITATION_NOTE_SELECTOR) || null;
}

function noteIsOpen(note: any): boolean {
  return !!note && note.getAttribute?.('data-citation-note-open') !== 'false';
}

/** Show a note; its markers report aria-expanded=true. */
export function openCitationNote(note: any, surface: any = null): boolean {
  if (!note) return false;
  note.setAttribute('data-citation-note-open', 'true');
  note.removeAttribute('hidden');
  note.removeAttribute('aria-hidden');
  citationMarkersForAnchor(citationAnchorOf(note), surface || note.ownerDocument).forEach((marker) => {
    marker.setAttribute('aria-expanded', 'true');
    marker.setAttribute('aria-pressed', 'true');
  });
  return true;
}

/** Hide a note without moving the text; its markers report aria-expanded=false. */
export function closeCitationNote(note: any, surface: any = null): boolean {
  if (!note) return false;
  note.setAttribute('data-citation-note-open', 'false');
  note.setAttribute('hidden', '');
  note.setAttribute('aria-hidden', 'true');
  citationMarkersForAnchor(citationAnchorOf(note), surface || note.ownerDocument).forEach((marker) => {
    marker.setAttribute('aria-expanded', 'false');
    marker.setAttribute('aria-pressed', 'false');
  });
  return true;
}

/**
 * Toggle one marker's own note and leave every other note as it was. Returns
 * the note it toggled, or null when the marker owns no note.
 */
export function toggleCitationNote(marker: any, surface: any = null): any {
  const note = citationNoteForMarker(marker, surface);
  if (!note) return null;
  const scope = surface || marker.ownerDocument;
  if (noteIsOpen(note)) closeCitationNote(note, scope);
  else openCitationNote(note, scope);
  return note;
}

/**
 * Make every marker's aria-expanded and pressed style agree with its note.
 * Run after a document re-renders: the notes come back open from the HTML.
 */
export function settleCitationNotes(surface: any): void {
  if (!surface?.querySelectorAll) return;
  surface.querySelectorAll(CITATION_NOTE_SELECTOR).forEach((note) => {
    if (!note.getAttribute('data-citation-note-open')) note.setAttribute('data-citation-note-open', 'true');
    if (noteIsOpen(note)) {
      note.removeAttribute('hidden');
      note.removeAttribute('aria-hidden');
    } else {
      note.setAttribute('hidden', '');
      note.setAttribute('aria-hidden', 'true');
    }
  });
  surface.querySelectorAll(CITATION_MARKER_SELECTOR).forEach((marker) => {
    const note = citationNoteForMarker(marker, surface);
    const open = !!(note && noteIsOpen(note));
    marker.setAttribute('aria-expanded', open ? 'true' : 'false');
    marker.setAttribute('aria-pressed', open ? 'true' : 'false');
  });
}
