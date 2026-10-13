// Autopaper reader (Cut 2 unit 2b, Reader W3a): the public front list and
// story page. Everything here reads public platform routes served by corpusd
// from Store B (the front list and the publication bundle); nothing calls a
// computer, so the reader stays up while the paper computer is stopped.
//
// A story is one publication. Its bare link serves the latest version in
// place; /v<n> serves version n. The reader mirrors those links under
// /autopaper/<slug> and /autopaper/<slug>/v<n>.

export const AUTOPAPER_BASE = '/autopaper';
const PUBLIC_TEXTURE_PREFIX = '/pub/texture/';

export type FrontStory = {
  publication_id: string;
  title: string;
  dek: string;
  updated_at: string;
  version_count: number;
  latest_revision: number;
  route: { bare: string; permalink: string };
};

export type FrontPage = { stories: FrontStory[]; next_cursor?: string };

export type AutopaperLocation =
  | { kind: 'front' }
  | { kind: 'story'; slug: string; revision: number }
  | { kind: 'not_found' };

export type CheckState = 'exact' | 'tolerant' | 'pending' | 'failed';

export type CitationView = {
  anchorId: string;
  relation: 'cites' | 'embeds';
  number: number;
  quote: string;
  blocks: { cid: string; text: string; kind: string }[];
  check: CheckState;
  judgement: string;
  detail: string;
  checkedAt: string;
};

export type SourceView = {
  key: string;
  number: number;
  title: string;
  url: string;
  host: string;
  citations: CitationView[];
};

export type BodyBlock =
  | { type: 'heading'; level: number; html: string }
  | { type: 'p'; html: string }
  | { type: 'list'; ordered: boolean; start: number; items: string[] }
  | { type: 'quote'; html: string }
  | { type: 'code'; text: string }
  | { type: 'rule' }
  | { type: 'embed'; citation: CitationView; source: SourceView | null; more: number };

export type StoryView = {
  title: string;
  revision: number;
  nav: { revision: number; prev: number; next: number; latest: number };
  publishedAt: string;
  manifestHash: string;
  bare: string;
  permalink: string;
  blocks: BodyBlock[];
  sources: SourceView[];
  checkSource: string;
};

// ---- routes ----

/** The reader location named by a pathname under /autopaper. */
export function parseAutopaperPath(pathname: string): AutopaperLocation {
  const path = String(pathname || '').replace(/\/+$/, '');
  if (path === AUTOPAPER_BASE || path === '') return { kind: 'front' };
  if (!path.startsWith(`${AUTOPAPER_BASE}/`)) return { kind: 'not_found' };
  const parts = path.slice(AUTOPAPER_BASE.length + 1).split('/').filter(Boolean);
  if (parts.length === 0) return { kind: 'front' };
  let slug = '';
  try {
    slug = decodeURIComponent(parts[0]);
  } catch {
    return { kind: 'not_found' };
  }
  if (!slug || slug.includes('/')) return { kind: 'not_found' };
  if (parts.length === 1) return { kind: 'story', slug, revision: 0 };
  if (parts.length === 2 && /^v[1-9][0-9]*$/.test(parts[1])) {
    return { kind: 'story', slug, revision: Number(parts[1].slice(1)) };
  }
  return { kind: 'not_found' };
}

export function isAutopaperPath(pathname: string): boolean {
  const path = String(pathname || '');
  return path === AUTOPAPER_BASE || path.startsWith(`${AUTOPAPER_BASE}/`);
}

/** The publication slug of a /pub/texture/<slug>[/v<n>] route. */
export function slugFromPublicRoute(route: string): string {
  const path = String(route || '');
  if (!path.startsWith(PUBLIC_TEXTURE_PREFIX)) return '';
  return path.slice(PUBLIC_TEXTURE_PREFIX.length).split('/')[0] || '';
}

/** The reader path of a story: its bare link, or version n. */
export function storyPath(slug: string, revision = 0): string {
  const base = `${AUTOPAPER_BASE}/${encodeURIComponent(slug)}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

/** The publication route a reader location reads. */
export function publicationRoute(slug: string, revision = 0): string {
  const base = `${PUBLIC_TEXTURE_PREFIX}${slug}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

// ---- fetchers ----

export class AutopaperNotFound extends Error {}

export async function fetchFrontPage(cursor = '', limit = 20): Promise<FrontPage> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (cursor) params.set('cursor', cursor);
  const res = await fetch(`/api/platform/publications/front?${params}`, {
    credentials: 'same-origin',
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`front list returned ${res.status}`);
  const page = await res.json();
  return { stories: Array.isArray(page?.stories) ? page.stories : [], next_cursor: page?.next_cursor || '' };
}

export async function fetchStoryBundle(slug: string, revision = 0): Promise<any> {
  const route = publicationRoute(slug, revision);
  const res = await fetch(`/api/platform/publications/resolve?route=${encodeURIComponent(route)}`, {
    credentials: 'same-origin',
    cache: 'no-store',
  });
  if (res.status === 404) throw new AutopaperNotFound(route);
  if (!res.ok) throw new Error(`story returned ${res.status}`);
  return res.json();
}

// ---- checks ----

/**
 * The badge a citation's live check overlay earns: ✓ exact, ≈ tolerant,
 * pending, ✗ failed. A failed quote or an unsupported judgement is failed; a
 * quote not yet checked is pending.
 */
export function checkState(check: any): CheckState {
  const quote = String(check?.quote || '').toLowerCase();
  const judgement = String(check?.judgement || '').toLowerCase();
  if (quote === 'failed' || judgement === 'failed' || judgement === 'unsupported') return 'failed';
  if (quote === 'exact') return 'exact';
  if (quote === 'tolerant') return 'tolerant';
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
    default:
      return 'pending';
  }
}

export function checkTitle(citation: CitationView): string {
  const parts = [`Quote check: ${citation.check}`];
  if (citation.judgement && citation.judgement !== 'pending') parts.push(`support: ${citation.judgement}`);
  else parts.push('support: pending');
  if (citation.detail) parts.push(citation.detail);
  return parts.join(' · ');
}

// ---- story model ----

export function escapeHTML(value: unknown): string {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

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

function nodeText(node: any): string {
  if (!node) return '';
  if (node.type === 'text') return String(node.text || '');
  return (node.content || []).map(nodeText).join('');
}

function renderMarks(text: string, marks: any[] = []): string {
  let html = escapeHTML(text);
  for (const mark of marks || []) {
    switch (mark?.type) {
      case 'strong':
      case 'bold':
        html = `<strong>${html}</strong>`;
        break;
      case 'em':
      case 'italic':
        html = `<em>${html}</em>`;
        break;
      case 'code':
        html = `<code>${html}</code>`;
        break;
      case 'link': {
        const href = safeHref(String(mark?.attrs?.href || ''));
        if (href) html = `<a href="${escapeHTML(href)}" rel="noopener noreferrer" target="_blank">${html}</a>`;
        break;
      }
      default:
        break;
    }
  }
  return html;
}

type BuildContext = {
  anchors: Map<string, any>;
  blocksByKey: Map<string, any>;
  checks: Map<string, any>;
  entities: Map<string, any>;
  sources: SourceView[];
  sourceByKey: Map<string, SourceView>;
  citations: Map<string, CitationView>;
};

function sourceKeyOf(node: any, anchor: any): string {
  const entity = String(node?.attrs?.source_entity_id || anchor?.source_entity_id || '').trim();
  if (entity) return entity;
  const ref = anchor?.ref || parseJSON(node?.attrs?.ref) || {};
  return String(ref.doc || node?.attrs?.id || '');
}

function sourceFor(ctx: BuildContext, key: string, anchor: any): SourceView {
  const existing = ctx.sourceByKey.get(key);
  if (existing) return existing;
  const ref = anchor?.ref || {};
  const pinned = (anchor?.cids || []).map((cid: string) => ctx.blocksByKey.get(`${ref.doc}\u0000${cid}`)).find(Boolean)
    || [...ctx.blocksByKey.values()].find((b) => b.doc === ref.doc);
  const entity = ctx.entities.get(key);
  const url = String(pinned?.url || entity?.target?.uri || '');
  const title = String(pinned?.title || entity?.display?.title || entity?.display?.label || hostOf(url) || 'Source');
  const source: SourceView = { key, number: ctx.sources.length + 1, title, url: safeHref(url), host: hostOf(url), citations: [] };
  ctx.sources.push(source);
  ctx.sourceByKey.set(key, source);
  return source;
}

function citationFor(ctx: BuildContext, node: any): { citation: CitationView; source: SourceView } | null {
  const anchorId = String(node?.attrs?.id || '');
  const anchor = ctx.anchors.get(anchorId);
  const attrRef = parseJSON(node?.attrs?.ref);
  const ref = anchor?.ref || attrRef;
  if (!ref?.doc) return null;
  const key = sourceKeyOf(node, anchor);
  const source = sourceFor(ctx, key, anchor || { ref, cids: ref.cid ? [ref.cid] : [] });
  const existing = ctx.citations.get(anchorId);
  if (existing) return { citation: existing, source };
  const relation = anchor?.relation === 'embeds' || node?.attrs?.display_mode === 'expanded_ref' ? 'embeds' : 'cites';
  const cids: string[] = anchor?.cids?.length ? anchor.cids : ref.cid ? [ref.cid] : [];
  const blocks = cids
    .map((cid) => ctx.blocksByKey.get(`${ref.doc}\u0000${cid}`))
    .filter(Boolean)
    .map((b: any) => ({ cid: String(b.cid), text: String(b.text || ''), kind: String(b.kind || '') }));
  const check = ctx.checks.get(anchorId);
  const citation: CitationView = {
    anchorId,
    relation,
    number: source.number,
    quote: String(ref.quote || ''),
    blocks,
    check: checkState(check),
    judgement: String(check?.judgement || ''),
    detail: String(check?.detail || ''),
    checkedAt: String(check?.checked_at || ''),
  };
  ctx.citations.set(anchorId, citation);
  if (relation === 'cites' || citation.quote) source.citations.push(citation);
  return { citation, source };
}

function inlineHTML(ctx: BuildContext, nodes: any[] = []): string {
  let html = '';
  for (const node of nodes || []) {
    if (node?.type === 'text') html += renderMarks(String(node.text || ''), node.marks);
    else if (node?.type === 'hard_break') html += '<br>';
    else if (node?.type === 'source_ref') {
      const found = citationFor(ctx, node);
      if (!found) continue;
      const { citation, source } = found;
      if (citation.relation === 'embeds' && !citation.quote) continue;
      html += `<a class="ap-cite" href="#cite-${escapeHTML(citation.anchorId)}" data-cite-anchor="${escapeHTML(citation.anchorId)}" data-check="${citation.check}" aria-label="Source ${source.number}">[${source.number}]</a>`;
    } else if (Array.isArray(node?.content)) html += inlineHTML(ctx, node.content);
  }
  return html;
}

const EMBED_BLOCK_LIMIT = 8;

/** A paragraph split at its embeds: text runs and transcluded blocks. */
function paragraphBlocks(ctx: BuildContext, node: any): BodyBlock[] {
  const out: BodyBlock[] = [];
  let run: any[] = [];
  const flush = () => {
    const html = inlineHTML(ctx, run).trim();
    if (html && html !== '<br>') out.push({ type: 'p', html });
    run = [];
  };
  for (const child of node?.content || []) {
    const isEmbed = child?.type === 'source_ref' && child?.attrs?.display_mode === 'expanded_ref';
    if (!isEmbed) {
      run.push(child);
      continue;
    }
    flush();
    const found = citationFor(ctx, child);
    if (!found) continue;
    const all = found.citation.blocks;
    const shown = { ...found.citation, blocks: all.slice(0, EMBED_BLOCK_LIMIT) };
    out.push({ type: 'embed', citation: shown, source: found.source, more: Math.max(0, all.length - EMBED_BLOCK_LIMIT) });
  }
  flush();
  return out;
}

function listItemHTML(ctx: BuildContext, item: any): string {
  return (item?.content || [])
    .map((child: any) => (child?.type === 'paragraph' || child?.type === 'heading' ? inlineHTML(ctx, child.content) : inlineHTML(ctx, [child])))
    .join(' ');
}

function bodyBlocks(ctx: BuildContext, nodes: any[], title: string): BodyBlock[] {
  const out: BodyBlock[] = [];
  nodes.forEach((node, index) => {
    switch (node?.type) {
      case 'heading': {
        const text = nodeText(node).trim();
        // The story title is the page's h1; a leading heading that repeats it
        // is not drawn twice.
        if (index === 0 && text && text.toLowerCase() === title.trim().toLowerCase()) return;
        const level = Math.min(4, Math.max(2, Number(node?.attrs?.level || 2) + 1));
        out.push({ type: 'heading', level, html: inlineHTML(ctx, node.content) });
        return;
      }
      case 'paragraph':
        out.push(...paragraphBlocks(ctx, node));
        return;
      case 'bullet_list':
      case 'ordered_list':
        out.push({
          type: 'list',
          ordered: node.type === 'ordered_list',
          start: Number(node?.attrs?.start || 1),
          items: (node.content || []).map((item: any) => listItemHTML(ctx, item)),
        });
        return;
      case 'blockquote':
        out.push({ type: 'quote', html: (node.content || []).map((c: any) => inlineHTML(ctx, c.content)).join('<br>') });
        return;
      case 'code_block':
        out.push({ type: 'code', text: (node.content || []).map((c: any) => String(c.text || '')).join('\n') });
        return;
      case 'horizontal_rule':
        out.push({ type: 'rule' });
        return;
      default:
        if (Array.isArray(node?.content)) out.push({ type: 'p', html: inlineHTML(ctx, node.content) });
    }
  });
  return out;
}

function textBlocks(text: string, title: string): BodyBlock[] {
  return String(text || '')
    .split(/\n{2,}/)
    .map((para) => para.trim())
    .filter(Boolean)
    .flatMap((para, index): BodyBlock[] => {
      const heading = /^(#{1,6})\s+(.*)$/.exec(para);
      if (heading) {
        if (index === 0 && heading[2].trim().toLowerCase() === title.trim().toLowerCase()) return [];
        return [{ type: 'heading', level: Math.min(4, heading[1].length + 1), html: escapeHTML(heading[2]) }];
      }
      return [{ type: 'p', html: escapeHTML(para).replace(/\n/g, '<br>') }];
    });
}

/** The story page's view of a publication bundle. */
export function buildStoryView(bundle: any): StoryView {
  const payload = bundle?.payload || {};
  const manifest = parseJSON(payload.manifest) || {};
  const title = String(bundle?.publication?.title || manifest.title || 'Untitled story');
  const ctx: BuildContext = {
    anchors: new Map(),
    blocksByKey: new Map(),
    checks: new Map(),
    entities: new Map(),
    sources: [],
    sourceByKey: new Map(),
    citations: new Map(),
  };
  for (const anchor of Array.isArray(manifest.anchors) ? manifest.anchors : []) {
    if (anchor?.anchor_id) ctx.anchors.set(String(anchor.anchor_id), anchor);
  }
  for (const block of Array.isArray(payload.resolved_blocks) ? payload.resolved_blocks : []) {
    ctx.blocksByKey.set(`${block.doc}\u0000${block.cid}`, block);
  }
  for (const check of Array.isArray(bundle?.checks?.citations) ? bundle.checks.citations : []) {
    if (check?.anchor_id) ctx.checks.set(String(check.anchor_id), check);
  }
  const entities = parseJSON(bundle?.artifact?.source_entities) || parseJSON(manifest.structured_source_entities) || [];
  for (const entity of Array.isArray(entities) ? entities : []) {
    if (entity?.source_entity_id) ctx.entities.set(String(entity.source_entity_id), entity);
  }
  const bodyDoc = parseJSON(bundle?.artifact?.body_doc) || parseJSON(manifest.body_doc);
  const nodes = Array.isArray(bodyDoc?.doc?.content) ? bodyDoc.doc.content : null;
  const blocks = nodes ? bodyBlocks(ctx, nodes, title) : textBlocks(String(payload.text || bundle?.artifact?.content || ''), title);
  const nav = bundle?.nav || {};
  const revision = Number(payload.revision || nav.revision || bundle?.version?.revision || 0);
  return {
    title,
    revision,
    nav: {
      revision: Number(nav.revision || revision),
      prev: Number(nav.prev || 0),
      next: Number(nav.next || 0),
      latest: Number(nav.latest || revision),
    },
    publishedAt: String(bundle?.version?.published_at || ''),
    manifestHash: String(payload.manifest_hash || bundle?.version?.manifest_hash || ''),
    bare: String(bundle?.route?.bare || ''),
    permalink: String(bundle?.route?.permalink || ''),
    blocks,
    sources: ctx.sources,
    checkSource: String(bundle?.checks?.source || ''),
  };
}

/**
 * A pinned block's text split around the citation's quote, so the quote
 * can be marked inside its source block. When the quote is not a verbatim
 * substring (a tolerant match), the whole block is returned unmarked.
 */
export function splitAtQuote(text: string, quote: string): { before: string; quote: string; after: string } | null {
  if (!quote) return null;
  const at = text.indexOf(quote);
  if (at >= 0) return { before: text.slice(0, at), quote, after: text.slice(at + quote.length) };
  const lowered = text.toLowerCase().indexOf(quote.toLowerCase());
  if (lowered >= 0) return { before: text.slice(0, lowered), quote: text.slice(lowered, lowered + quote.length), after: text.slice(lowered + quote.length) };
  return null;
}

export function formatWhen(iso: string): string {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  const seconds = Math.round((Date.now() - date.getTime()) / 1000);
  if (seconds >= 0 && seconds < 60) return 'just now';
  if (seconds >= 0 && seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds >= 0 && seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`;
  return date.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}
