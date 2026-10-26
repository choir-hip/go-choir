// "More on this": the other published stories about the subject of the one
// being read (docs/reports/related-stories-graph-design-2026-10-19.md, 5.8).
//
// The list is served by GET /api/platform/publications/related, computed
// from the object graph in Store B and cached for five minutes. The reader
// asks for it after the article has rendered and never waits for it: a
// failure, a stale answer, or an empty list shows nothing at all.
//
// The article's HTML is written imperatively (TextureEditor's
// syncEditorSurface), and the article is the scroll container, so the block
// is rendered into that HTML after the last paragraph rather than placed
// beside the article.

import { AUTOPAPER_SECTIONS, parsePublicationRoute, storyPath } from './autopaper';
import { escapeHTML } from './texture-source-renderer';

/** The most links the block shows, whatever the server sends. */
export const RELATED_MAX_LINKS = 5;

export type RelatedLink = {
  publication_id: string;
  title: string;
  dek?: string;
  updated_at?: string;
  route: { bare: string; permalink?: string };
  section?: string;
  /** Why the story is related: a judged relation (same_incident, later_development, same_subject), "entity" (a central entity both are about), "capture" (a shared source) or "source" (a recaptured page). */
  reason?: string;
  shared?: { events?: number; sources?: number; citers?: number; entities?: number };
};

export type RelatedStories = {
  route: string;
  revision: number;
  basis?: { captures?: number; events?: number; mode?: string };
  partial?: boolean;
  generated_at?: string;
  links: RelatedLink[];
};

/**
 * The related list of a story's bare route, or null when there is nothing
 * to show: any failure, a malformed answer, a partial or switched-off one.
 * It never throws.
 */
export async function fetchRelatedStories(route: string, signal?: AbortSignal): Promise<RelatedStories | null> {
  try {
    const params = new URLSearchParams({ route, limit: String(RELATED_MAX_LINKS) });
    const res = await fetch(`/api/platform/publications/related?${params}`, {
      method: 'GET',
      credentials: 'same-origin',
      signal,
    });
    if (!res.ok) return null;
    const body = await res.json();
    if (!body || !Array.isArray(body.links)) return null;
    return body as RelatedStories;
  } catch {
    return null;
  }
}

/** The links worth showing: a title, a story route, at most RELATED_MAX_LINKS, each story once. */
export function visibleRelatedLinks(related: RelatedStories | null | undefined): RelatedLink[] {
  const links = Array.isArray(related?.links) ? related!.links : [];
  const seen = new Set<string>();
  const out: RelatedLink[] = [];
  for (const link of links) {
    const bare = String(link?.route?.bare || '');
    const { slug } = parsePublicationRoute(bare);
    const title = String(link?.title || '').trim();
    if (!slug || !title || seen.has(bare)) continue;
    seen.add(bare);
    out.push(link);
    if (out.length === RELATED_MAX_LINKS) break;
  }
  return out;
}

/** "3 h ago" style age; a date once it is more than two weeks old; '' when unknown. */
export function relatedAgeLabel(updatedAt: string | undefined, now: number = Date.now()): string {
  const ms = Date.parse(updatedAt || '');
  if (!Number.isFinite(ms)) return '';
  const minutes = Math.floor((now - ms) / 60_000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.floor(hours / 24);
  if (days <= 14) return `${days} d ago`;
  return new Date(ms).toISOString().slice(0, 10);
}

function sectionLabel(id: string | undefined): string {
  return AUTOPAPER_SECTIONS.find((s) => s.id === id && id !== 'front')?.label || '';
}

/**
 * The block, as HTML for the article. Everything the server sent is
 * escaped. Empty when there is nothing to show: no heading, no placeholder.
 * Each link is a real anchor to /pub/texture/<slug>, so a reader can open it
 * in a new tab; a plain click is taken over by the reader
 * (relatedLinkTarget).
 */
export function renderRelatedHTML(related: RelatedStories | null | undefined, now: number = Date.now()): string {
  const links = visibleRelatedLinks(related);
  if (links.length === 0) return '';
  const items = links
    .map((link) => {
      const bare = String(link.route.bare);
      const { slug } = parsePublicationRoute(bare);
      const title = String(link.title).trim();
      const dek = String(link.dek || '').trim();
      const meta = [relatedAgeLabel(link.updated_at, now), sectionLabel(link.section)].filter(Boolean).join(' · ');
      return (
        `<li class="related-stories-item">` +
        `<a class="related-story" href="${escapeHTML(storyPath(slug))}" data-related-link` +
        ` data-related-route="${escapeHTML(bare)}" data-related-title="${escapeHTML(title)}"` +
        ` data-related-publication-id="${escapeHTML(link.publication_id || '')}"` +
        ` data-related-reason="${escapeHTML(link.reason || '')}">` +
        `<span class="related-story-title">${escapeHTML(title)}</span>` +
        (dek ? `<span class="related-story-dek">${escapeHTML(dek)}</span>` : '') +
        (meta ? `<span class="related-story-meta">${escapeHTML(meta)}</span>` : '') +
        `</a></li>`
      );
    })
    .join('');
  return (
    `<section class="related-stories" data-texture-related contenteditable="false">` +
    `<h2 class="related-stories-heading">More on this</h2>` +
    `<ul class="related-stories-list">${items}</ul>` +
    `</section>`
  );
}

/**
 * Where a click on the block should take the reader: the story's bare route
 * and title, or null when it is not a plain left click on a related link (a
 * modified click or a middle click keeps the browser's own new-tab
 * behaviour, which works because every link is a real anchor).
 */
export function relatedLinkTarget(
  event: { target?: unknown; button?: number; metaKey?: boolean; ctrlKey?: boolean; shiftKey?: boolean; altKey?: boolean },
): { route: string; title: string } | null {
  const el = (event.target as { closest?: (selector: string) => Element | null } | null)?.closest?.('[data-related-link]');
  if (!el) return null;
  if ((event.button ?? 0) !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null;
  const route = el.getAttribute('data-related-route') || '';
  if (!parsePublicationRoute(route).slug) return null;
  return { route, title: el.getAttribute('data-related-title') || '' };
}
