// Embed blocks in the Texture web renderer (station TE,
// docs/design/texture-evidence-embeds-te-2026-10-25.md section 5).
//
// An embed is a top-level block of choir.texture_doc.v1 with a typed target.
// capsule_evidence is one capture of a frozen self-development candidate:
// the bytes come from the owner's asset route (/api/assets/sha256/<digest>,
// authorized by the owner's session), and the candidate's identity (bundle,
// build, capture receipt) is read live from the operation's visual-evidence
// listing, never from the revision. Unknown kinds render as a plain card.
// Nothing here runs script in the document: the figure holds an <img> and
// text, and the component handles a failed load with a capture-phase
// listener (markEmbedUnresolvable), not an inline handler.
//
// This module imports nothing, so its tests load it on their own.

type EmbedNode = {
  type: string;
  attrs?: Record<string, any>;
};

type ExistingDoc = { doc?: { content?: EmbedNode[] } } | null | undefined;

export const ASSET_ROUTE = '/api/assets/sha256/';
const DIGEST_RE = /^[0-9a-f]{64}$/;

function escapeHTML(value: unknown): string {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function str(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function dimension(value: unknown): number {
  const n = Number(value);
  return Number.isInteger(n) && n >= 1 && n <= 16384 ? n : 0;
}

function shortDigest(digest: string): string {
  return digest.slice(0, 12);
}

type EmbedFields = {
  id: string;
  kind: string;
  digest: string;
  operationID: string;
  computerID: string;
  route: string;
  width: number;
  height: number;
  blank: boolean;
  caption: string;
  alt: string;
};

export function embedFields(node: EmbedNode): EmbedFields {
  const attrs = node?.attrs || {};
  const target = attrs.target && typeof attrs.target === 'object' ? attrs.target : {};
  return {
    id: str(attrs.id),
    kind: str(attrs.kind),
    digest: str(target.digest),
    operationID: str(target.operation_id),
    computerID: str(target.computer_id),
    route: str(target.route),
    width: dimension(target.width),
    height: dimension(target.height),
    blank: target.blank === true,
    caption: str(attrs.caption),
    alt: str(attrs.alt),
  };
}

/** The embed's line in the text projection; the server writes the same. */
export function embedProjectionText(node: EmbedNode): string {
  const e = embedFields(node);
  if (e.kind === 'capsule_evidence') return `[evidence: ${e.caption}]`;
  if (e.kind === 'image') return `[image: ${e.caption || e.alt}]`;
  return '[embed]';
}

/** Operations every one of whose captures in these blocks is blank. */
export function allBlankOperations(blocks: EmbedNode[] = []): Set<string> {
  const seen = new Map<string, boolean>();
  for (const node of blocks) {
    if (node?.type !== 'embed') continue;
    const e = embedFields(node);
    if (e.kind !== 'capsule_evidence' || !e.operationID) continue;
    seen.set(e.operationID, (seen.get(e.operationID) ?? true) && e.blank);
  }
  return new Set([...seen].filter(([, allBlank]) => allBlank).map(([op]) => op));
}

function figureAttrs(e: EmbedFields, extra: Record<string, string> = {}): string {
  const attrs: Record<string, string> = {
    class: `texture-embed texture-embed-${e.kind === 'capsule_evidence' || e.kind === 'image' ? e.kind.replace('_', '-') : 'unknown'}`,
    contenteditable: 'false',
    'data-texture-embed': '',
    'data-texture-embed-id': e.id,
    'data-embed-kind': e.kind,
    ...extra,
  };
  return Object.entries(attrs)
    .map(([key, value]) => (value === '' ? ` ${key}` : ` ${key}="${escapeHTML(value)}"`))
    .join('');
}

function imageHTML(e: EmbedFields, alt: string): string {
  const size = e.width && e.height ? ` width="${e.width}" height="${e.height}"` : '';
  return `<img src="${ASSET_ROUTE}${e.digest}" alt="${escapeHTML(alt)}"${size} loading="lazy" decoding="async" referrerpolicy="no-referrer" data-texture-embed-media>`;
}

function unresolvableHTML(e: EmbedFields, hidden: boolean): string {
  const digest = DIGEST_RE.test(e.digest) ? `sha256:${shortDigest(e.digest)}` : 'a malformed digest';
  return `<div class="texture-embed-unresolvable" data-texture-embed-unresolvable${hidden ? ' hidden' : ''}>Evidence not retained or unreadable (${escapeHTML(digest)})</div>`;
}

/** One embed block as HTML. */
export function renderEmbedBlockHTML(node: EmbedNode, options: { allBlank?: Set<string> } = {}): string {
  const e = embedFields(node);
  const validDigest = DIGEST_RE.test(e.digest);
  if (e.kind === 'capsule_evidence') {
    const state = validDigest ? 'loading' : 'unresolvable';
    const media = validDigest ? imageHTML(e, e.alt || `Screenshot of ${e.route}`) : '';
    const badge = e.blank ? '<span class="texture-embed-badge">blank</span> ' : '';
    const allBlank = e.operationID && options.allBlank?.has(e.operationID)
      ? '<span class="texture-embed-warning">Every capture of this candidate is blank.</span>'
      : '';
    return `<figure${figureAttrs(e, {
      'data-embed-digest': validDigest ? e.digest : '-',
      'data-embed-operation': e.operationID,
      'data-embed-computer': e.computerID,
      'data-embed-blank': e.blank ? 'true' : 'false',
      'data-embed-state': state,
    })}><div class="texture-embed-frame">${media}${unresolvableHTML(e, validDigest)}</div>`
      + `<figcaption>${badge}<span class="texture-embed-caption">${escapeHTML(e.caption)}</span>`
      + (e.route ? ` <code>${escapeHTML(e.route)}</code>` : '')
      + `<span class="texture-embed-identity" data-texture-embed-identity>candidate ${escapeHTML(e.operationID)}</span>${allBlank}</figcaption></figure>`;
  }
  if (e.kind === 'image') {
    const state = validDigest ? 'loading' : 'unresolvable';
    const media = validDigest ? imageHTML(e, e.alt) : '';
    const caption = e.caption || e.alt;
    return `<figure${figureAttrs(e, { 'data-embed-digest': validDigest ? e.digest : '-', 'data-embed-state': state })}><div class="texture-embed-frame">${media}${unresolvableHTML(e, validDigest)}</div>`
      + (caption ? `<figcaption><span class="texture-embed-caption">${escapeHTML(caption)}</span></figcaption>` : '')
      + '</figure>';
  }
  const digest = validDigest ? ` · sha256:${shortDigest(e.digest)}` : '';
  return `<figure${figureAttrs(e)}><figcaption>Embedded ${escapeHTML(e.kind || 'item')}${escapeHTML(digest)} (this version of Texture does not show it)</figcaption></figure>`;
}

/**
 * The editor's serializer keeps an embed only by its id from the revision it
 * loaded: the DOM never builds one, so a forged attribute cannot become
 * evidence, and an owner save does not drop it.
 */
export function serializeEmbedElement(element: any, existingDoc: ExistingDoc): EmbedNode | null {
  const id = String(element?.getAttribute?.('data-texture-embed-id') || '').trim();
  if (!id) return null;
  const original = (existingDoc?.doc?.content || []).find((node) => node?.type === 'embed' && str(node?.attrs?.id) === id);
  return original ? JSON.parse(JSON.stringify(original)) : null;
}

/** A failed image load switches its figure to the unresolvable state. */
export function markEmbedUnresolvable(target: any): boolean {
  if (!target?.matches?.('img[data-texture-embed-media]')) return false;
  const figure = target.closest?.('[data-texture-embed]');
  if (!figure) return false;
  figure.dataset.embedState = 'unresolvable';
  target.hidden = true;
  const notice = figure.querySelector?.('[data-texture-embed-unresolvable]');
  if (notice) notice.hidden = false;
  return true;
}

type ListingResult = { status: number; listing?: any };

/** The candidate's identity line, from the live listing. */
export function evidenceIdentityText(operationID: string, result: ListingResult | null): string {
  const head = `candidate ${operationID}`;
  if (!result) return head;
  if (result.status === 404 || result.status === 410) return `${head} · bundle no longer retained`;
  if (result.status !== 200 || !result.listing) return `${head} · identity unavailable`;
  const l = result.listing;
  const parts = [head];
  if (str(l.bundle_digest)) parts.push(`bundle ${shortDigest(str(l.bundle_digest).replace(/^sha256:/, ''))}`);
  if (str(l.frontend_sha256)) parts.push(`build ${shortDigest(str(l.frontend_sha256))}`);
  if (str(l.capture_receipt)) parts.push(`receipt ${shortDigest(str(l.capture_receipt).replace(/^.*sha256:/, ''))}`);
  return parts.join(' · ');
}

export function evidenceListingPath(computerID: string, operationID: string): string {
  return `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(operationID)}/visual-evidence`;
}

const identityCache = new Map<string, Promise<ListingResult>>();

/**
 * Fills each evidence figure's identity line from its operation's listing,
 * one request per operation per page load.
 */
export function hydrateEmbedIdentity(root: any, fetchFn: (url: string, init?: any) => Promise<any>): void {
  const figures = Array.from(root?.querySelectorAll?.('[data-texture-embed][data-embed-kind="capsule_evidence"]') || []) as any[];
  for (const figure of figures) {
    const operationID = String(figure.getAttribute('data-embed-operation') || '');
    const computerID = String(figure.getAttribute('data-embed-computer') || '');
    const line = figure.querySelector('[data-texture-embed-identity]');
    if (!operationID || !computerID || !line) continue;
    const key = `${computerID}\u0000${operationID}`;
    let pending = identityCache.get(key);
    if (!pending) {
      pending = fetchFn(evidenceListingPath(computerID, operationID), { credentials: 'same-origin' })
        .then(async (res: any) => ({ status: res.status, listing: res.ok ? await res.json() : undefined }))
        .catch(() => ({ status: 0 }));
      identityCache.set(key, pending);
    }
    pending.then((result) => {
      line.textContent = evidenceIdentityText(operationID, result);
    });
  }
}
