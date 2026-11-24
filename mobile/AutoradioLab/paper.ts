// The paper page's data source. `paperFront` answers one view of the front (Front page, My edition or a category) as the
// placed bands the page draws. It asks the public front list (`GET /api/platform/publications/front`), keeps the last good list
// on the phone for an offline start, and composes with paper-layout.ts. It replaces the Swift PaperSource, PaperComposer and
// LayoutEngine; the radio's own reads of the front stay in Swift (RadioProgramme, StretchCandidates).
//
// Ways this can fail, and what the page then says (the page already has these states; "Loading" is never one of them):
//  - no network, a timeout, a status outside 200 to 299, a body that is not the front list: the saved list with status "offline",
//    or with no saved list status "error" and no stories. A category that cannot be asked falls back to the front list filtered.
//  - an empty front: status "empty" with the view's own line.
//  - a story with no title or route is dropped; an unknown view id is a category of that name.
// Time: the server's `Date` header is "now" (ranking, "Updated 2 h ago"); with none, the newest story's time.

import type { Storage } from './app.contract.d.ts';
import { composition, compose, stack, type Band } from './paper-layout.ts';
import {
  FRONT, FRONT_LIMIT, LIST_LIMIT, MINE, buildMine, belongs, dateLine, display, isCategory, parseList, parseLiveSections, relative, resolveView, sections,
  type LiveSection, type Section, type Story,
} from './paper-model.ts';
import { getJSON, writeCurrent, accountGeneration } from './api.ts';
import { clock } from './clock.ts';

export const FRONT_PATH = '/api/platform/publications/front';
/** The last good front list, kept for an offline start. */
export const CACHE_FILE = 'app:/data/paper-front.json';

export type PaperArgs = { view: string; width: number; tick: number; focus: string; picks: string };

export type PaperFront = {
  status: string; message: string; layout: string; composition: string; dateLine: string; updated: string; sectionId: string;
  sectionLabel: string; viewId: string; viewLabel: string; mineState: string; mineFrom: string; note: string; count: number;
  sections: Section[]; bands: Band[]; routes: string[]; titles: string[];
};

type Listing = { stories: Story[]; status: string; message: string; sections: LiveSection[]; serverTime: number | null };

const OFFLINE = 'Offline. Showing the last front page.';
const UNREACHABLE = 'Could not reach choir.news. Pull to try again.';

// The newest answer of each list is kept until the next pull to refresh (`tick` changes), so switching views and coming back from a
// story ask nothing: one ask per list, never one per view change.
const kept = new Map<string, Promise<Listing>>();
let lastTick = 0;

async function readCache(storage: Storage | undefined): Promise<unknown | null> {
  if (!storage) return null;
  try { return JSON.parse(new TextDecoder().decode(await storage.fs.readFile(CACHE_FILE))); } catch { return null; }
}

async function writeCache(storage: Storage | undefined, raw: string, gen: number): Promise<void> {
  await writeCurrent(gen, storage, CACHE_FILE, new TextEncoder().encode(raw));       // a full disk or no storage loses only the offline copy; a sign-out in flight writes nothing
}

/** The front list (`section` empty) or one category's list. Only the front list is written to the phone. */
export function listing(section: string, storage: Storage | undefined): Promise<Listing> {
  const hit = kept.get(section);
  if (hit) return hit;
  // The promise is kept, so two answers asked together (the picks arrive a moment after the page) make one request.
  const p = load(section, storage).then((r) => {
    if (!(r.status === 'ready' && (r.stories.length > 0 || section !== ''))) kept.delete(section);
    return r;
  });
  kept.set(section, p);
  return p;
}

async function load(section: string, storage: Storage | undefined): Promise<Listing> {
  const gen = accountGeneration();
  try {
    const query: Record<string, string | number> = { limit: LIST_LIMIT };
    if (section !== '') query.section = section;
    const r = await getJSON(FRONT_PATH, query);
    const stories = parseList(r.body);
    if (section === '' && stories.length > 0) await writeCache(storage, r.raw, gen);
    const res: Listing = { stories, status: 'ready', message: '', sections: parseLiveSections(r.body), serverTime: r.serverTime };
    console.log(`paper: list ${section === '' ? 'front' : 'section ' + section} stories=${stories.length}`);
    return res;
  } catch (e) {
    console.log(`paper: list ${section === '' ? 'front' : 'section ' + section} failed`);
    if (section === '') {
      const body = await readCache(storage);
      if (body !== null) {
        try {
          const stories = parseList(body);
          if (stories.length > 0) return { stories, status: 'offline', message: OFFLINE, sections: parseLiveSections(body), serverTime: null };
        } catch { /* a corrupt file is ignored */ }
      }
    }
    return { stories: [], status: 'error', message: UNREACHABLE, sections: [], serverTime: null };
  }
}

export function pickList(joined: string): string[] { return joined.split('\n').map((s) => s.trim()).filter((s) => s !== ''); }

/** The paper's front for one view. `stories` is the front list (newest first, as the server sends it). */
let composes = 0;
/** How many times a page has been composed (tests count them: a first open composes once). */
export const composeCount = (): number => composes;
export function composeFront(stories: Story[], o: {
  view: string; picks: string[]; category: Story[] | null; live: LiveSection[]; width: number; now: number | null; status: string; message: string;
}): PaperFront {
  composes += 1;
  const newest = (xs: Story[]): number | null => xs.reduce<number | null>((m, s) => (s.updated !== null && (m === null || s.updated > m) ? s.updated : m), null);
  const at = o.now ?? newest(stories) ?? 0;
  const secs = sections(o.live, stories);
  let viewId = FRONT, viewLabel = 'Front page', sectionId = '', sectionLabel = '';
  let shown: Story[] = stories.slice(0, FRONT_LIMIT);
  let priority: Record<string, number> = {};
  let mineState = '', mineFrom = '', note = '', empty = 'Nothing on the front page yet.';
  if (o.view === MINE) {
    viewId = MINE; viewLabel = 'My edition';
    const m = buildMine(stories, o.picks, secs);
    mineState = m.state; mineFrom = m.from;
    if (m.state === 'ready') { shown = m.stories; priority = m.priority; }
    else if (m.state === 'nopicks') note = 'Choose the topics you care about and this page arranges the paper around them. Until then, here is the front page.';
    else note = "Nothing in today's paper matches your topics yet. Here is the front page.";
  } else if (isCategory(o.view)) {
    const sec = resolveView(o.view, secs) ?? { id: o.view, label: display(o.view), count: 0, current: false };
    viewId = sec.id; viewLabel = sec.label; sectionId = sec.id; sectionLabel = sec.label;
    shown = o.category ?? stories.filter((s) => belongs(s, sec));
    empty = `Nothing in ${sec.label} yet.`;
  }
  const bands = compose(shown, o.width, at, priority);
  const st = stack(bands);
  const latest = newest(shown);
  return {
    status: shown.length === 0 && o.status === 'ready' ? 'empty' : o.status, message: shown.length === 0 && o.message === '' ? empty : o.message,
    layout: 'Autopaper default', composition: composition(bands), dateLine: dateLine(at), updated: latest === null ? '' : `Updated ${relative(latest, at)}`,
    sectionId, sectionLabel, viewId, viewLabel, mineState, mineFrom, note, count: shown.length, sections: secs, bands, routes: st.routes, titles: st.titles,
  };
}

/** What `empty()` gives before the first answer: status "" is "loading" to the page. */
export const blankFront = (): PaperFront => ({
  status: '', message: '', layout: '', composition: '', dateLine: '', updated: '', sectionId: '', sectionLabel: '', viewId: '', viewLabel: '', mineState: '', mineFrom: '', note: '', count: 0,
  sections: [], bands: [], routes: [], titles: [],
});

// The page asks again as each of its arguments settles (the width the page reports, the focus when the deck catches up, the picks a moment after): what the answer is built from is the list, the view, the width and,
// for My edition only, the picks and the focus. The last composition is kept with the key it was built from, so an ask that changes nothing the page is built from answers the same page without composing it again.
let composed: { key: string; r: Listing; c: Listing | null; out: PaperFront } | null = null;

export async function paperFront(a: PaperArgs, storage?: Storage): Promise<PaperFront> {
  const view = a.view === '' ? FRONT : a.view;
  if (a.tick !== lastTick) { kept.clear(); lastTick = a.tick; composed = null; }
  const width = a.width >= 200 ? a.width : 393;                              // no width reported yet: the reading order the article stack and the radio's public list need does not wait for the page to be laid out
  const picks = view === MINE ? pickList(a.picks) : [];
  const r = await listing('', storage);
  let category: Story[] | null = null; let c: Listing | null = null;
  if (isCategory(view)) {
    const secs = sections(r.sections, r.stories);
    const id = resolveView(view, secs)?.id ?? view;
    c = await listing(id, storage);
    if (c.status === 'ready') category = c.stories;
  }
  const key = [view, width, view === MINE ? picks.join('\n') + '|' + a.focus : ''].join('|');
  if (composed && composed.key === key && composed.r === r && composed.c === c) return composed.out;
  const out = composeFront(r.stories, { view, picks, category, live: r.sections, width, now: r.serverTime, status: r.status, message: r.message });
  composed = { key, r, c, out };
  return out;
}

/** The page from the list the phone saved, at once, while the fetch is out (stale, then fresh: the page shows `paperSaved` until `paperFront` answers). It composes only what the saved list can say alone:
 * the front page and a category, not My edition (its picks come a moment later and would show its "choose your topics" line first), and not before the page has a width. It reads only the file, never the network;
 * an account change during the read drops it. No saved list, or a failed read, is an empty answer: the page's own loading, never an invented page. Status "ready" with no message: the offline line is the fetch's to
 * say if it fails. */
export async function paperSaved(a: PaperArgs, storage?: Storage): Promise<PaperFront> {
  const view = a.view === '' ? FRONT : a.view;
  if (view !== FRONT || a.width < 200) return blankFront();         // only the front page: My edition waits for its picks, and a category from the front list alone could say "empty" for a section that has stories (only fresh data may say empty)
  const gen = accountGeneration();
  const body = await readCache(storage);
  if (body === null || gen !== accountGeneration()) return blankFront();
  try {
    const stories = parseList(body);
    if (stories.length === 0) return blankFront();
    const out = composeFront(stories, { view, picks: [], category: null, live: parseLiveSections(body), width: a.width, now: clock.ms > 0 ? clock.ms : null, status: 'ready', message: '' });
    if (clock.ms <= 0) out.updated = '';                                // no device time: the age of the list is not known, and "Updated just now" would be a claim
    return out;
  } catch { return blankFront(); }
}

/** Forget the kept lists (tests). */
export function forget(): void { kept.clear(); lastTick = 0; composed = null; }

/** Sign-out (account.ts): the cached front list on disk and what this session holds. Never throws. */
export async function wipePaper(storage: Storage | undefined): Promise<void> {
  kept.clear(); lastTick = 0; composed = null;
  if (!storage) return;
  try { await storage.fs.rm(CACHE_FILE); } catch { /* absent */ }
}
