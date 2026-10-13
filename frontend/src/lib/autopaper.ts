// Autopaper: the public front list of living stories, and the routes that
// name a story. Everything here reads public platform routes served from
// Store B; nothing calls a computer, so Autopaper stays up while the paper
// computer is stopped.
//
// A story is one publication. Autopaper lists stories; a story itself opens
// in the Texture app's published mode (the same Texture UI as a user's own
// documents), read from GET /api/platform/publications/resolve. Its bare
// link serves the latest version in place; /v<n> serves version n. The
// public pages mirror those links under /autopaper/<slug> and
// /autopaper/<slug>/v<n>.

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

// ---- routes ----

/** The public page named by a pathname under /autopaper. */
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

/** The slug and version of a /pub/texture/<slug>[/v<n>] publication route. */
export function parsePublicationRoute(route: string): { slug: string; revision: number } {
  const path = String(route || '');
  if (!path.startsWith(PUBLIC_TEXTURE_PREFIX)) return { slug: '', revision: 0 };
  const [slug = '', version = ''] = path.slice(PUBLIC_TEXTURE_PREFIX.length).split('/');
  return { slug, revision: /^v[1-9][0-9]*$/.test(version) ? Number(version.slice(1)) : 0 };
}

/** The public page of a story: its bare link, or version n. */
export function storyPath(slug: string, revision = 0): string {
  const base = `${AUTOPAPER_BASE}/${encodeURIComponent(slug)}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

/** The public page of a publication route. */
export function storyPathForRoute(route: string): string {
  const { slug, revision } = parsePublicationRoute(route);
  return slug ? storyPath(slug, revision) : AUTOPAPER_BASE;
}

/** The publication route a story page reads. */
export function publicationRoute(slug: string, revision = 0): string {
  const base = `${PUBLIC_TEXTURE_PREFIX}${slug}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

/**
 * The Texture window context that opens a story read-only in published
 * mode. The route makes the window restore to the same story and version.
 */
export function storyTextureContext(route: string, title = ''): Record<string, unknown> {
  return {
    publishedRoutePath: route,
    publishedReadOnly: true,
    windowTitle: title || 'Story',
    allowMultiple: true,
  };
}

// ---- front list ----

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
