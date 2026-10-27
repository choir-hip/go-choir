// Published citations: the per-anchor view of a publication bundle that the
// Texture renderer draws in published mode. A bundle whose manifest carries
// citation anchors (Autopaper stories, and any Texture published with
// anchors) pins each citation to resolved source blocks and carries a live
// check per anchor. This module turns that into:
//
//   - one numbered source per cited source, numbered in reading order;
//   - per citation: its quote, the resolved blocks it is pinned to, and its
//     check state (exact ✓, tolerant ≈, pending, unread source, failed ✗);
//   - HTML for the inline numbered marker (with its quote popover) and for
//     the source list. A stored embed (relation embeds) is the same marker:
//     the reader never prints a source's text as a block
//     (docs/problems/published-embeds-republish-source-text-2026-10-16.md).
//
// Everything is read from the bundle the public resolve route returns; it
// never calls a computer.

import { escapeHTML, sourceEntityID, sourceEntityTitle, sourceEntityTargetURL } from './texture-source-renderer';
import { citationNoteIDForAnchor } from './texture-citation-marker';

export type CheckState = 'exact' | 'tolerant' | 'pending' | 'unread' | 'failed';

export type PinnedBlock = { cid: string; text: string; kind: string };

/**
 * Where the quote check found a citation's quote in its block (Ref.span,
 * H-OG3): code-point offsets [start, end), and one range per ellipsis
 * segment. Refs written before spans have none.
 */
export type QuoteRange = { start: number; end: number };
export type QuoteSpan = QuoteRange & { segments?: QuoteRange[] };

export type PublishedCitation = {
  anchorId: string;
  relation: 'cites' | 'embeds';
  sourceKey: string;
  quote: string;
  quoteCid: string;
  span: QuoteSpan | null;
  blocks: PinnedBlock[];
  check: CheckState;
  judgement: string;
  detail: string;
};

export type PublishedSource = {
  key: string;
  number: number;
  entityID: string;
  title: string;
  url: string;
  host: string;
  citations: PublishedCitation[];
};

export type PublishedCitationContext = {
  byAnchor: Map<string, PublishedCitation>;
  sources: PublishedSource[];
  sourceByKey: Map<string, PublishedSource>;
};

// ---- checks ----

/**
 * The badge a citation's live check earns: ✓ exact, ≈ tolerant, pending,
 * unread source, ✗ failed. A failed quote or an unsupported judgement is
 * failed; a quote whose source was never captured is unread; a quote not yet
 * checked is pending.
 */
export function checkState(check: any): CheckState {
  const quote = String(check?.quote || '').toLowerCase();
  const judgement = String(check?.judgement || '').toLowerCase();
  if (quote === 'failed' || judgement === 'failed' || judgement === 'unsupported') return 'failed';
  if (quote === 'exact') return 'exact';
  if (quote === 'tolerant') return 'tolerant';
  if (quote === 'unread') return 'unread';
  return 'pending';
}

export function checkLabel(state: CheckState): string {
  switch (state) {
    case 'exact':
      return '✓ exact';
    case 'tolerant':
      return '≈ tolerant';
    case 'failed':
      return '✗ failed';
    case 'unread':
      return 'unread source';
    default:
      return 'pending';
  }
}

function checkTitle(citation: PublishedCitation): string {
  const parts = [`Quote check: ${citation.check}`];
  parts.push(citation.judgement && citation.judgement !== 'pending' ? `support: ${citation.judgement}` : 'support: pending');
  if (citation.detail) parts.push(citation.detail);
  return parts.join(' · ');
}

/**
 * A pinned block's text split around the citation's quote, so the quote can
 * be marked inside its source block. When the quote is not a substring (a
 * tolerant match), null is returned and the block is shown unmarked.
 */
export function splitAtQuote(text: string, quote: string): { before: string; quote: string; after: string } | null {
  if (!quote) return null;
  const at = text.indexOf(quote);
  if (at >= 0) return { before: text.slice(0, at), quote, after: text.slice(at + quote.length) };
  const lowered = text.toLowerCase().indexOf(quote.toLowerCase());
  if (lowered >= 0) {
    return { before: text.slice(0, lowered), quote: text.slice(lowered, lowered + quote.length), after: text.slice(lowered + quote.length) };
  }
  return null;
}

/** A Ref's span, or null when it is missing or malformed. */
export function parseQuoteSpan(value: any): QuoteSpan | null {
  const range = (r: any): QuoteRange | null =>
    r && Number.isInteger(r.start) && Number.isInteger(r.end) && r.start >= 0 && r.end > r.start ? { start: r.start, end: r.end } : null;
  const whole = range(value);
  if (!whole) return null;
  const segments = Array.isArray(value.segments) ? value.segments.map(range) : [];
  if (segments.some((r: QuoteRange | null) => r === null)) return null;
  return segments.length > 1 ? { ...whole, segments: segments as QuoteRange[] } : whole;
}

/**
 * A pinned block's text cut at the citation's recorded span, so the exact
 * words the quote check matched are marked, the right occurrence of a
 * repeated phrase and every segment of an ellipsis quote. Null when there
 * is no span or the block no longer reads as the quote at those offsets;
 * the caller then falls back to splitAtQuote.
 */
export function splitAtSpan(text: string, quote: string, span: QuoteSpan | null): { text: string; marked: boolean }[] | null {
  if (!span) return null;
  const chars = Array.from(text);
  const ranges = span.segments && span.segments.length > 1 ? span.segments : [{ start: span.start, end: span.end }];
  const pieces: { text: string; marked: boolean }[] = [];
  const marked: string[] = [];
  let at = 0;
  for (const r of ranges) {
    if (r.start < at || r.end > chars.length) return null;
    const words = chars.slice(r.start, r.end).join('');
    pieces.push({ text: chars.slice(at, r.start).join(''), marked: false }, { text: words, marked: true });
    marked.push(words);
    at = r.end;
  }
  pieces.push({ text: chars.slice(at).join(''), marked: false });
  if (quote && marked.join(' ... ') !== quote) return null;
  return pieces;
}

// ---- model ----

function parseJSON(value: any): any {
  if (value == null) return null;
  if (typeof value === 'object') return value;
  try {
    return JSON.parse(String(value));
  } catch {
    return null;
  }
}

function hostOf(url: string): string {
  try {
    return new URL(url).host.replace(/^www\./, '');
  } catch {
    return '';
  }
}

function safeHref(url: string): string {
  return /^https?:\/\//i.test(url) ? url : '';
}

function collectSourceRefs(node: any, out: any[]) {
  if (!node) return;
  if (node.type === 'source_ref') out.push(node);
  for (const child of node.content || []) collectSourceRefs(child, out);
}

/**
 * The citation context of a publication bundle, or null when the bundle has
 * no citation anchors (a Texture published without them renders as before).
 */
export function buildPublishedCitationContext(bundle: any, sourceEntities: any[] = []): PublishedCitationContext | null {
  const payload = bundle?.payload || {};
  const manifest = parseJSON(payload.manifest) || {};
  const anchorList = Array.isArray(manifest.anchors) ? manifest.anchors : [];
  if (anchorList.length === 0) return null;

  const anchors = new Map<string, any>();
  for (const anchor of anchorList) {
    if (anchor?.anchor_id) anchors.set(String(anchor.anchor_id), anchor);
  }
  const blocksByKey = new Map<string, any>();
  for (const block of Array.isArray(payload.resolved_blocks) ? payload.resolved_blocks : []) {
    blocksByKey.set(`${block.doc}\u0000${block.cid}`, block);
  }
  const checks = new Map<string, any>();
  for (const check of Array.isArray(bundle?.checks?.citations) ? bundle.checks.citations : []) {
    if (check?.anchor_id) checks.set(String(check.anchor_id), check);
  }
  const entities = new Map<string, any>();
  for (const entity of sourceEntities) {
    const id = sourceEntityID(entity);
    if (id) entities.set(id, entity);
  }

  const ctx: PublishedCitationContext = { byAnchor: new Map(), sources: [], sourceByKey: new Map() };
  const bodyDoc = parseJSON(bundle?.artifact?.body_doc) || parseJSON(manifest.body_doc);
  const refs: any[] = [];
  collectSourceRefs(bodyDoc?.doc, refs);

  for (const node of refs) {
    const anchorId = String(node?.attrs?.id || '');
    const anchor = anchors.get(anchorId);
    const ref = anchor?.ref || parseJSON(node?.attrs?.ref);
    if (!anchorId || !ref?.doc || ctx.byAnchor.has(anchorId)) continue;
    const entityID = String(node?.attrs?.source_entity_id || anchor?.source_entity_id || '').trim();
    const key = entityID || String(ref.doc);
    const cids: string[] = anchor?.cids?.length ? anchor.cids : ref.cid ? [ref.cid] : [];
    const resolved = cids.map((cid) => blocksByKey.get(`${ref.doc}\u0000${cid}`)).filter(Boolean);

    let source = ctx.sourceByKey.get(key);
    if (!source) {
      const pinned = resolved[0] || [...blocksByKey.values()].find((b) => b.doc === ref.doc);
      const entity = entities.get(entityID);
      const url = String(pinned?.url || (entity ? sourceEntityTargetURL(entity) : '') || '');
      const entityTitle = entity ? String(entity?.display?.title || '') : '';
      const title = String(pinned?.title || entityTitle || hostOf(url) || (entity ? sourceEntityTitle(entity) : '') || 'Source');
      source = { key, number: ctx.sources.length + 1, entityID, title, url: safeHref(url), host: hostOf(url), citations: [] };
      ctx.sources.push(source);
      ctx.sourceByKey.set(key, source);
    }

    const check = checks.get(anchorId);
    const citation: PublishedCitation = {
      anchorId,
      relation: anchor?.relation === 'embeds' || node?.attrs?.display_mode === 'expanded_ref' ? 'embeds' : 'cites',
      sourceKey: key,
      quote: String(ref.quote || ''),
      quoteCid: String(ref.cid || ''),
      span: parseQuoteSpan(ref.span),
      blocks: resolved.map((b: any) => ({ cid: String(b.cid), text: String(b.text || ''), kind: String(b.kind || '') })),
      check: checkState(check),
      judgement: String(check?.judgement || ''),
      detail: String(check?.detail || ''),
    };
    ctx.byAnchor.set(anchorId, citation);
    if (citation.relation === 'cites' || citation.quote) source.citations.push(citation);
  }
  return ctx.byAnchor.size > 0 ? ctx : null;
}

// ---- HTML ----

function badgeHTML(citation: PublishedCitation): string {
  return `<span class="texture-check-badge" data-check-badge="${citation.check}" title="${escapeHTML(checkTitle(citation))}">${escapeHTML(checkLabel(citation.check))}</span>`;
}

function judgementHTML(citation: PublishedCitation): string {
  if (citation.judgement === 'supported') return '<span class="texture-citation-judgement">supports the sentence</span>';
  if (citation.judgement === 'unsupported') {
    return '<span class="texture-citation-judgement texture-citation-judgement--failed">does not support the sentence</span>';
  }
  return '';
}

/** One pinned block with the citation's quote marked inside it. */
function pinnedBlockHTML(block: PinnedBlock, citation: PublishedCitation, tag: 'span' | 'blockquote'): string {
  const quote = citation.quote;
  const pieces = block.cid === citation.quoteCid ? splitAtSpan(block.text, quote, citation.span) : null;
  if (pieces) {
    const body = pieces
      .map((p) => (p.marked ? `<mark class="texture-citation-quote" data-citation-quote data-citation-span>${escapeHTML(p.text)}</mark>` : escapeHTML(p.text)))
      .join('');
    return `<${tag} class="texture-transclusion-quote texture-citation-block" data-pinned-cid="${escapeHTML(block.cid)}">${body}</${tag}>`;
  }
  const split = splitAtQuote(block.text, quote);
  const body = split
    ? `${escapeHTML(split.before)}<mark class="texture-citation-quote" data-citation-quote>${escapeHTML(split.quote)}</mark>${escapeHTML(split.after)}`
    : `${quote ? `<mark class="texture-citation-quote texture-citation-quote--standalone" data-citation-quote>“${escapeHTML(quote)}”</mark>` : ''}${escapeHTML(block.text)}`;
  return `<${tag} class="texture-transclusion-quote texture-citation-block" data-pinned-cid="${escapeHTML(block.cid)}">${body}</${tag}>`;
}

/** The quote-in-source-block body shared by the popover and the source list. */
function citationBodyHTML(citation: PublishedCitation, tag: 'span' | 'blockquote'): string {
  const blocks = citation.blocks.length
    ? citation.blocks.map((block) => pinnedBlockHTML(block, citation, tag)).join('')
    : citation.quote
      ? `<${tag} class="texture-transclusion-quote texture-citation-block"><mark class="texture-citation-quote" data-citation-quote>“${escapeHTML(citation.quote)}”</mark></${tag}>`
      : '';
  return `${blocks}<span class="texture-citation-check">${badgeHTML(citation)}${judgementHTML(citation)}</span>`;
}

/** The inline numbered marker for a citation, with its quote popover. */
export function renderPublishedCitationRef(ctx: PublishedCitationContext, anchorId: string): string | null {
  const citation = ctx.byAnchor.get(anchorId);
  if (!citation) return null;
  const source = ctx.sourceByKey.get(citation.sourceKey);
  if (!source) return null;
  const number = String(source.number);
  const entityID = source.entityID;
  const label = `Source ${number}: ${source.title}, quote check ${citation.check}`;
  return `<span class="texture-source-ref texture-source-ref--cited" data-texture-source-ref data-texture-citation-transclusion data-texture-source-node-id="${escapeHTML(anchorId)}" data-source-display-mode="numbered_ref" data-source-entity-id="${escapeHTML(entityID)}" data-source-label="${escapeHTML(number)}" data-source-number="${escapeHTML(number)}" data-citation-anchor="${escapeHTML(anchorId)}" data-check="${citation.check}" data-source-expansion-surface="journal" contenteditable="false" tabindex="0" role="button" aria-expanded="true" aria-pressed="true" aria-controls="${escapeHTML(citationNoteIDForAnchor(anchorId))}" aria-label="${escapeHTML(label)}">
    <span class="texture-source-ref-label">${escapeHTML(number)}</span>
    <span class="texture-source-ref-popover" data-texture-source-ref-popover data-texture-inline-transclusion role="note">
      <strong>${escapeHTML(source.title)}</strong>
      ${citation.quote ? `<span class="texture-transclusion-body texture-citation-body" data-texture-transclusion-body data-texture-citation-body>${citationBodyHTML(citation, 'span')}</span>` : ''}
      ${entityID ? `<button type="button" class="texture-source-open" data-texture-open-source data-source-entity-id="${escapeHTML(entityID)}">Open source</button>` : ''}
    </span>
  </span>`;
}

/** The numbered source list closing a published document. */
export function renderPublishedSourcesHTML(ctx: PublishedCitationContext | null): string {
  if (!ctx || ctx.sources.length === 0) return '';
  const items = ctx.sources.map((source) => {
    const title = source.url
      ? `<a href="${escapeHTML(source.url)}" rel="noopener noreferrer" target="_blank">${escapeHTML(source.title)}</a>`
      : escapeHTML(source.title);
    const host = source.host && source.host !== source.title ? `<span class="texture-published-source-host">${escapeHTML(source.host)}</span>` : '';
    const citations = source.citations.map((citation) => `<div class="texture-published-citation" data-citation-anchor="${escapeHTML(citation.anchorId)}" data-citation-check="${citation.check}">${citationBodyHTML(citation, 'blockquote')}</div>`).join('');
    return `<li class="texture-published-source" data-source-block data-source-number="${source.number}" data-source-entity-id="${escapeHTML(source.entityID)}">
      <div class="texture-published-source-head"><span class="texture-source-number">[${source.number}]</span> <span class="texture-published-source-title">${title}</span>${host}</div>
      ${citations}
    </li>`;
  }).join('');
  return `<section class="texture-published-sources" data-texture-published-sources contenteditable="false" aria-label="Sources">
    <h2>Sources</h2>
    <ol>${items}</ol>
  </section>`;
}
/**
 * The plain citation note a marker toggles. It sits in the flow of the text
 * under the paragraph that cites it, carries the same quote-in-source-block
 * body and live check the Sources list shows, and is never a popover: the note
 * is the only surface a marker opens (owner direction, 2026-10-27). The inline
 * popover span stays on the marker for every existing reader of it, but the
 * stylesheet in texture-citation-notes.css keeps it out of view.
 */
export function renderCitationNoteHTML(ctx: PublishedCitationContext | null, anchorId: string): string {
  if (!ctx) return '';
  const citation = ctx.byAnchor.get(anchorId);
  if (!citation) return '';
  const source = ctx.sourceByKey.get(citation.sourceKey);
  if (!source) return '';
  const number = String(source.number);
  // The note is plain text plus one Close button: no link and no nested action,
  // so a press anywhere in it is never mistaken for a marker press or a
  // navigation. The source's actions stay on the marker and the Sources list
  // (owner direction, 2026-10-27).
  return `<aside class="texture-citation-note" id="${escapeHTML(citationNoteIDForAnchor(anchorId))}" data-texture-source-note data-citation-anchor="${escapeHTML(anchorId)}" data-citation-source-key="${escapeHTML(source.key)}" data-citation-number="${escapeHTML(number)}" data-citation-check="${escapeHTML(citation.check)}" data-citation-note-open="true" role="note" aria-label="${escapeHTML(`Source ${number} note`)}">`
    + `<div class="texture-citation-note-head"><span class="texture-source-number">[${escapeHTML(number)}]</span> <span class="texture-published-source-title">${escapeHTML(source.title)}</span></div>`
    // A note shows the citation's own quoted words and its live check, never a
    // source's captured page text: a whole-resource embed republishes nothing
    // (docs/problems/published-embeds-republish-source-text-2026-10-16.md).
    + (citation.quote ? citationBodyHTML({ ...citation, blocks: [] }, 'blockquote') : '')
    + `<button type="button" class="texture-citation-note-close" data-citation-note-close aria-label="${escapeHTML(`Close source ${number} note`)}">Close</button>`
    + '</aside>';
}

/**
 * The citation notes for the citations an HTML fragment marks, in reading
 * order, one per anchor, wrapped in one block placed under the fragment's
 * paragraph. Empty when the fragment cites nothing this bundle resolves.
 */
export function renderCitationNotesHTML(ctx: PublishedCitationContext | null, anchors: string[] = []): string {
  if (!ctx || !anchors.length) return '';
  const seen = new Set<string>();
  const notes = anchors
    .map((anchorId) => String(anchorId || '').trim())
    .filter((anchorId) => {
      if (!anchorId || seen.has(anchorId)) return false;
      seen.add(anchorId);
      return true;
    })
    .map((anchorId) => renderCitationNoteHTML(ctx, anchorId))
    .filter(Boolean)
    .join('');
  if (!notes) return '';
  // One notes block follows the paragraph that cites it, and each note is a
  // direct child of it: a note's own previousElementSibling is the notes
  // block's parent paragraph, so the note reads as sitting under the
  // paragraph it belongs to (owner direction, 2026-10-27).
  return `<div class="texture-citation-notes" data-texture-citation-notes aria-label="Citation notes">${notes}</div>`;
}

/**
 * The citation anchors an HTML fragment marks, in reading order. The markers
 * carry an escaped anchor id in a dedicated attribute and never a quote, so
 * reading that attribute back finds every citation in a block.
 */
export function citationAnchorsInHTML(html: string): string[] {
  const anchors: string[] = [];
  const re = / data-citation-anchor="([^"<>\s]+)"/g;
  let match = re.exec(html);
  while (match) {
    anchors.push(match[1]);
    match = re.exec(html);
  }
  return anchors;
}
