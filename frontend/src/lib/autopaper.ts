// Autopaper: the public front list of living stories, and the routes that
// name a story. Everything here reads public platform routes served from
// Store B; nothing calls a computer, so Autopaper stays up while the paper
// computer is stopped.
//
// A story is one publication. Autopaper lists stories; a story itself opens
// in the Texture app's published mode (the same Texture UI as a user's own
// documents), read from GET /api/platform/publications/resolve. Its bare
// link /pub/texture/<slug> serves the latest version in place;
// /pub/texture/<slug>/v<n> serves version n. There is no page of Autopaper's
// own any more: the old /autopaper/<slug>[/v<n>] links are replaced with
// those routes (legacyAutopaperRedirect), and /autopaper itself goes to the
// desktop home, which opens the front page. The front page is laid out like
// a broadsheet's: rankFrontStories orders the stories and layoutFrontPage
// cuts them into the lead, the top tier, the briefs, and more stories.

const PUBLIC_TEXTURE_PREFIX = '/pub/texture/';
const LEGACY_BASE = '/autopaper';

export type FrontStory = {
  publication_id: string;
  title: string;
  dek: string;
  updated_at: string;
  version_count: number;
  latest_revision: number;
  route: { bare: string; permalink: string };
  /** Model-judged importance 1..10 (host front-importance scorer); absent when unscored. */
  importance?: number;
  /** The scorer judged the story promotional (or an internal instruction). */
  promo?: boolean;
  /** Distinct sources the latest version cites (the reader's "Sources N"); absent when unknown. */
  source_count?: number;
  /** Words in the latest version's body; absent when unknown. */
  words?: number;
  /** The front-page section the scorer filed the story under; absent when unscored. */
  section?: string;
  /** "opinion" for a row of the Opinion section; absent for an Autopaper story. */
  kind?: string;
  /** An opinion piece's byline (a handle, never an email); absent for a story. */
  author?: string;
};

export type FrontPage = { stories: FrontStory[]; next_cursor?: string };

// ---- routes ----

/**
 * Where an old Autopaper page address goes now, or null when the pathname is
 * not one: /autopaper/<slug> and /autopaper/<slug>/v<n> become the public
 * Texture route of the story (the desktop opens it in a Texture window), and
 * /autopaper alone, or anything else under it, becomes the desktop home.
 */
export function legacyAutopaperRedirect(pathname: string): string | null {
  const path = String(pathname || '').replace(/\/+$/, '');
  if (path !== LEGACY_BASE && !path.startsWith(`${LEGACY_BASE}/`)) return null;
  const parts = path.slice(LEGACY_BASE.length + 1).split('/').filter(Boolean);
  if (parts.length === 0) return '/';
  let slug = '';
  try {
    slug = decodeURIComponent(parts[0]);
  } catch {
    return '/';
  }
  if (!slug || slug.includes('/')) return '/';
  if (parts.length === 1) return storyPath(slug);
  if (parts.length === 2 && /^v[1-9][0-9]*$/.test(parts[1])) return storyPath(slug, Number(parts[1].slice(1)));
  return '/';
}

/** The slug and version of a /pub/texture/<slug>[/v<n>] publication route. */
export function parsePublicationRoute(route: string): { slug: string; revision: number } {
  const path = String(route || '');
  if (!path.startsWith(PUBLIC_TEXTURE_PREFIX)) return { slug: '', revision: 0 };
  const [slug = '', version = ''] = path.slice(PUBLIC_TEXTURE_PREFIX.length).split('/');
  return { slug, revision: /^v[1-9][0-9]*$/.test(version) ? Number(version.slice(1)) : 0 };
}

/** The address of a story: its bare link, or version n. */
export function storyPath(slug: string, revision = 0): string {
  const base = `${PUBLIC_TEXTURE_PREFIX}${encodeURIComponent(slug)}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

/** The address of a publication route (its own path, normalized); the desktop home for a route that names no story. */
export function storyPathForRoute(route: string): string {
  const { slug, revision } = parsePublicationRoute(route);
  return slug ? storyPath(slug, revision) : '/';
}

/** The publication route a story window reads. */
export function publicationRoute(slug: string, revision = 0): string {
  const base = `${PUBLIC_TEXTURE_PREFIX}${slug}`;
  return revision > 0 ? `${base}/v${revision}` : base;
}

/**
 * The Texture window context that opens a story read-only in published
 * mode. The route makes the window restore to the same story and version.
 */
export function storyTextureContext(route: string, title = '', opinion = false): Record<string, unknown> {
  return {
    publishedRoutePath: route,
    publishedReadOnly: true,
    // An Autopaper story: the reader shows the AI-written disclosure. An
    // opinion piece is a person's own text; the reader labels it from the
    // server's record (publication.kind), not from this context.
    autopaperStory: !opinion,
    windowTitle: title || 'Story',
    allowMultiple: true,
  };
}

// ---- front list ----

/**
 * How many stories the front page's first load reads: about a day of
 * stories, so importance ranking can lift an important story that is not
 * among the newest twenty. Later pages ("more stories") read 20 at a time.
 */
export const FRONT_FIRST_PAGE_SIZE = 60;

/**
 * One page of the front list. With a section id (not the front page's) the
 * server returns only that section's stories, newest first, and next_cursor
 * continues that section; a section page may be short or empty with a
 * next_cursor when the server stopped scanning (the caller keeps following).
 */
export async function fetchFrontPage(cursor = '', limit = 20, section = ''): Promise<FrontPage> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (cursor) params.set('cursor', cursor);
  if (section && section !== FRONT_SECTION_ID) params.set('section', section);
  const res = await fetch(`/api/platform/publications/front?${params}`, {
    credentials: 'same-origin',
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`front list returned ${res.status}`);
  const page = await res.json();
  return { stories: Array.isArray(page?.stories) ? page.stories : [], next_cursor: page?.next_cursor || '' };
}

/** The section id the server lists opinion pieces under. */
export const OPINION_SECTION_ID = 'opinion';

/** The newest opinion pieces (an empty list when the Opinion section is off). */
export async function fetchOpinionPage(limit = 8): Promise<FrontStory[]> {
  const page = await fetchFrontPage('', limit, OPINION_SECTION_ID);
  return page.stories.filter((story) => story?.kind === 'opinion');
}

// ---- front-page rank ----
//
// How prominent a story is on the front page. The base order is recency:
// the later a story's last update, the higher it sits. A living story that
// is still moving earns a lift: when its last update falls within the fresh
// window of the edition (the newest update on the page), every version past
// its first counts as one lift of extra recency, up to a cap. So a story in
// its third version updated an hour before a one-version story leads it,
// while a much-revised story from yesterday never outranks today's news.
// Ties fall to the later update, then to the publication id. The rank reads
// no clock, only the list itself, so the same list ranks the same way on
// every reader's screen.

/** How close to the edition an update must be for versions to lift it. */
export const FRONT_FRESH_WINDOW_MS = 6 * 60 * 60 * 1000;
/** The recency one extra version is worth inside the fresh window. */
export const FRONT_VERSION_LIFT_MS = 45 * 60 * 1000;
/** The most extra versions that lift a story. */
export const FRONT_MAX_LIFTED_VERSIONS = 4;

function updatedMs(story: FrontStory): number {
  const ms = Date.parse(story?.updated_at || '');
  return Number.isFinite(ms) ? ms : Number.NEGATIVE_INFINITY;
}

/** The newest update among the stories: the edition time of the page. */
export function editionTime(stories: FrontStory[]): number {
  let newest = Number.NEGATIVE_INFINITY;
  for (const story of stories || []) newest = Math.max(newest, updatedMs(story));
  return newest;
}

/** A story's prominence against an edition time: recency plus its version lift. */
export function frontProminence(story: FrontStory, edition: number): number {
  const updated = updatedMs(story);
  if (!Number.isFinite(updated)) return updated;
  const extraVersions = Math.max(0, Math.floor(Number(story.version_count) || 0) - 1);
  const fresh = Number.isFinite(edition) && edition - updated <= FRONT_FRESH_WINDOW_MS;
  const lift = fresh ? Math.min(extraVersions, FRONT_MAX_LIFTED_VERSIONS) * FRONT_VERSION_LIFT_MS : 0;
  return updated + lift;
}

// Importance rank. When at least half the stories carry a model-judged
// importance (1..10, attached by corpusd from the host scorer's file), a
// story's prominence is its importance times its depth times a recency
// decay with a 24-hour half-life, measured from the edition time (so it
// still reads no clock). Depth is the quality of our article, not of the
// event (docs/archive/problems/front-page-leads-with-a-one-source-stub-2026-10-14.md):
// one source (or none) halves it, two sources weigh 0.75, three or more
// count in full, and a body under FRONT_THIN_WORDS words weighs a further
// 0.8. A story citing fewer than two sources never leads; it can still sit
// in the top tier. A story whose depth corpusd did not report counts in
// full. On the front page the reader's own section habit (visits counted
// on this device) lifts stories of the sections they read most, by up to
// FRONT_SECTION_LIFT.
// An unscored story counts as importance 4. A story judged promotional, or
// whose title is a leaked internal instruction ("Open a story on ..."),
// never leads and never sits in the top tier: it ranks after every other
// story, still listed. With fewer than half scored, the rank above applies
// unchanged.

/** The recency half-life of the importance rank. */
export const FRONT_IMPORTANCE_HALF_LIFE_MS = 24 * 60 * 60 * 1000;
/** The importance an unscored story counts as under the importance rank. */
export const FRONT_DEFAULT_IMPORTANCE = 4;

function storyImportance(story: FrontStory): number | null {
  if (story?.importance === undefined || story?.importance === null) return null;
  const n = Number(story.importance);
  return Number.isFinite(n) && n >= 1 && n <= 10 ? n : null;
}

/** Under this many body words a story counts as short. */
export const FRONT_THIN_WORDS = 150;
/** The fewest cited sources a lead story may have. */
export const FRONT_LEAD_MIN_SOURCES = 2;
/** The most a reader's section habit lifts a story (a share of 1 lifts by this much). */
export const FRONT_SECTION_LIFT = 0.25;

function finiteCount(value: unknown): number | null {
  if (value === undefined || value === null) return null;
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : null;
}

/** How much of a story's importance our article earns: its sources and length. 1 when unknown. */
export function depthFactor(story: FrontStory): number {
  let factor = 1;
  const sources = finiteCount(story?.source_count);
  if (sources !== null) factor *= sources <= 1 ? 0.5 : sources < 3 ? 0.75 : 1;
  const words = finiteCount(story?.words);
  if (words !== null && words < FRONT_THIN_WORDS) factor *= 0.8;
  return factor;
}

/** Whether a story may lead: not demoted, and citing at least two sources (or depth unknown). */
export function canLead(story: FrontStory): boolean {
  if (isDemotedStory(story)) return false;
  const sources = finiteCount(story?.source_count);
  return sources === null || sources >= FRONT_LEAD_MIN_SOURCES;
}

/** Whether enough stories are scored for the importance rank to apply. */
export function usesImportanceRank(stories: FrontStory[]): boolean {
  const list = Array.isArray(stories) ? stories : [];
  if (list.length === 0) return false;
  const scored = list.filter((story) => storyImportance(story) !== null).length;
  return scored * 2 >= list.length;
}

/** A story that may not lead or sit in the top tier under the importance rank. */
export function isDemotedStory(story: FrontStory): boolean {
  return story?.promo === true || /^\s*open a story/i.test(String(story?.title || ''));
}

/** The lift a reader's section habit gives a story: 1 + FRONT_SECTION_LIFT x its section's share of visits. */
export function sectionLift(story: FrontStory, sectionShares?: Record<string, number> | null): number {
  if (!sectionShares) return 1;
  const share = Number(sectionShares[String(story?.section || '')]);
  if (!Number.isFinite(share) || share <= 0) return 1;
  return 1 + FRONT_SECTION_LIFT * Math.min(1, share);
}

/** A story's importance-rank prominence against an edition time. */
export function importanceProminence(story: FrontStory, edition: number, sectionShares?: Record<string, number> | null): number {
  const updated = updatedMs(story);
  if (!Number.isFinite(updated)) return Number.NEGATIVE_INFINITY;
  const age = Number.isFinite(edition) ? Math.max(0, edition - updated) : 0;
  const importance = storyImportance(story) ?? FRONT_DEFAULT_IMPORTANCE;
  return importance * depthFactor(story) * sectionLift(story, sectionShares) * Math.pow(0.5, age / FRONT_IMPORTANCE_HALF_LIFE_MS);
}

export type RankOptions = {
  /** The reader's share of section visits (0..1 by section id); lifts those sections' stories. */
  sectionShares?: Record<string, number> | null;
};

/** The stories in front-page order, most prominent first. Pure and stable. */
export function rankFrontStories(stories: FrontStory[], options: RankOptions = {}): FrontStory[] {
  const list = Array.isArray(stories) ? stories : [];
  const edition = editionTime(list);
  if (usesImportanceRank(list)) {
    const ranked = list
      .map((story) => ({
        story,
        demoted: isDemotedStory(story) ? 1 : 0,
        score: importanceProminence(story, edition, options.sectionShares),
        updated: updatedMs(story),
      }))
      .sort((a, b) => {
        if (a.demoted !== b.demoted) return a.demoted - b.demoted;
        if (a.score !== b.score) return b.score > a.score ? 1 : -1;
        if (a.updated !== b.updated) return b.updated > a.updated ? 1 : -1;
        const ida = String(a.story.publication_id || '');
        const idb = String(b.story.publication_id || '');
        return ida < idb ? -1 : ida > idb ? 1 : 0;
      })
      .map((entry) => entry.story);
    // The lead is the first story that may lead; the stories it passes keep
    // their order behind it. With none eligible the order stands.
    const leadAt = ranked.findIndex(canLead);
    if (leadAt > 0) ranked.unshift(...ranked.splice(leadAt, 1));
    return ranked;
  }
  return list
    .map((story) => ({ story, score: frontProminence(story, edition), updated: updatedMs(story) }))
    .sort((a, b) => {
      if (a.score !== b.score) return b.score > a.score ? 1 : -1;
      if (a.updated !== b.updated) return b.updated > a.updated ? 1 : -1;
      const ida = String(a.story.publication_id || '');
      const idb = String(b.story.publication_id || '');
      return ida < idb ? -1 : ida > idb ? 1 : 0;
    })
    .map((entry) => entry.story);
}

// ---- sections ----
//
// The paper's sections (owner, 2026-10-14, LF1): the front page, and four
// sections the scorer files each story under. A section page lists only
// that section's stories, in the same rank; a story with no section is on
// the front page only. The switcher shows only when at least half the
// stories carry a section. The reader's last section is remembered on this
// device and restored on reopen; their visits per section are counted on
// this device (never sent anywhere) and lift those sections on the front
// page.

export const FRONT_SECTION_ID = 'front';

export const AUTOPAPER_SECTIONS: { id: string; label: string }[] = [
  { id: FRONT_SECTION_ID, label: 'Front page' },
  { id: 'world', label: 'World' },
  { id: 'us_politics', label: 'U.S. Politics' },
  { id: 'business', label: 'Business & Finance' },
  { id: 'tech_ai', label: 'Tech & AI' },
];

const SECTION_IDS = new Set(AUTOPAPER_SECTIONS.map((s) => s.id));
export const SECTION_STORAGE_KEY = 'choir.autopaper.section.v1';
export const SECTION_VISITS_STORAGE_KEY = 'choir.autopaper.section-visits.v1';

export function isSectionId(id: unknown): id is string {
  return typeof id === 'string' && SECTION_IDS.has(id);
}

/** Whether enough stories carry a section for the switcher to show. */
export function usesSections(stories: FrontStory[]): boolean {
  const list = Array.isArray(stories) ? stories : [];
  if (list.length === 0) return false;
  const filed = list.filter((story) => isSectionId(story?.section) && story.section !== FRONT_SECTION_ID).length;
  return filed * 2 >= list.length;
}

/** The stories a section page lists: all of them on the front page. */
export function storiesInSection(stories: FrontStory[], section: string): FrontStory[] {
  const list = Array.isArray(stories) ? stories : [];
  if (!isSectionId(section) || section === FRONT_SECTION_ID) return list;
  return list.filter((story) => story?.section === section);
}

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>;

function storage(): StorageLike | null {
  try {
    return typeof window !== 'undefined' && window.localStorage ? window.localStorage : null;
  } catch {
    return null;
  }
}

/** The section the reader last chose on this device, or the front page. */
export function loadSavedSection(store: StorageLike | null = storage()): string {
  try {
    const id = store?.getItem(SECTION_STORAGE_KEY) || '';
    return isSectionId(id) ? id : FRONT_SECTION_ID;
  } catch {
    return FRONT_SECTION_ID;
  }
}

export function saveSection(section: string, store: StorageLike | null = storage()): void {
  if (!isSectionId(section)) return;
  try {
    store?.setItem(SECTION_STORAGE_KEY, section);
  } catch {
    // Storage blocked: the section is simply not remembered.
  }
}

/** The reader's visits per section on this device. */
export function loadSectionVisits(store: StorageLike | null = storage()): Record<string, number> {
  const out: Record<string, number> = {};
  try {
    const raw = JSON.parse(store?.getItem(SECTION_VISITS_STORAGE_KEY) || '{}');
    for (const [id, n] of Object.entries(raw || {})) {
      const count = Number(n);
      if (isSectionId(id) && id !== FRONT_SECTION_ID && Number.isFinite(count) && count > 0) out[id] = Math.floor(count);
    }
  } catch {
    // Unreadable: no habit yet.
  }
  return out;
}

/** Counts one visit to a section (the front page is not counted) and returns the new counts. */
export function recordSectionVisit(section: string, store: StorageLike | null = storage()): Record<string, number> {
  const visits = loadSectionVisits(store);
  if (!isSectionId(section) || section === FRONT_SECTION_ID) return visits;
  visits[section] = (visits[section] || 0) + 1;
  try {
    store?.setItem(SECTION_VISITS_STORAGE_KEY, JSON.stringify(visits));
  } catch {
    // Storage blocked: the visit is not kept.
  }
  return visits;
}

/** Each section's share of the reader's section visits (0..1), or null with none. */
export function sectionShares(visits: Record<string, number>): Record<string, number> | null {
  const total = Object.values(visits || {}).reduce((sum, n) => sum + (Number(n) > 0 ? Number(n) : 0), 0);
  if (total <= 0) return null;
  const out: Record<string, number> = {};
  for (const [id, n] of Object.entries(visits)) if (Number(n) > 0) out[id] = Number(n) / total;
  return out;
}

/** How many stories after the lead carry a dek, and how many run as briefs. */
export const FRONT_TOP_COUNT = 4;
export const FRONT_BRIEF_COUNT = 6;

export type FrontLayout = {
  lead: FrontStory | null;
  top: FrontStory[];
  briefs: FrontStory[];
  more: FrontStory[];
};

/**
 * The front page cut from ranked stories: the lead, the top tier (headline
 * and dek), the briefs (headline only), and the rest as more stories.
 */
export function layoutFrontPage(ranked: FrontStory[]): FrontLayout {
  const list = Array.isArray(ranked) ? ranked : [];
  const topEnd = 1 + FRONT_TOP_COUNT;
  const briefEnd = topEnd + FRONT_BRIEF_COUNT;
  return {
    lead: list[0] || null,
    top: list.slice(1, topEnd),
    briefs: list.slice(topEnd, briefEnd),
    more: list.slice(briefEnd),
  };
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
