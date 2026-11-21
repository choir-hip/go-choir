// The paper's story cards and the launcher, in shared code (a port of the Swift cards lane: AutoradioModule+Cards.swift, Preferences.swift, LearnedFromPreferences.swift and the launcher
// half of OpenPagesUI.swift, all gone from the native package). Three jobs:
//   1. "What I've told the paper": the scored acts ("more of this", "less of this") kept as a list the listener can read and reset, in `app:/data/what-i-told-the-paper.json`. The weights
//      that shade a story are derived from that list on every read, never stored apart from it. Nothing is read from the old native file (the app is unlaunched).
//   2. The shading (`order`, `lift`, `why`): a story sharing a central entity with "more" acts gets a lift, one sharing with "less" acts a dip, bounded at +-25% (MAX_LIFT), decaying with
//      age (half-life 14 days); the lift moves a story at most PLACES_PER_LIFT places in the paper's own order. The shared ranking is never changed by one listener.
//   3. The launcher's data (`launcherData`): the places, the recent pages, and the local hits over open pages, places, recent pages and the front list's stories. The paper-wide search
//      is retrieval-run.ts's (`paperSearch`), merged after these rows by the contract.
// `storyMore` records the act, the steering tap (steer.ts) and answers the acknowledgement. Every request goes through the one signed-in client (api.ts).
//
// Failure list (cards.test.ts, ported from the Swift CardsTests and LearnedFromPreferencesTests, every assertion kept):
//  1 a story with no entities and no section teaches nothing: record refuses it (null). An unknown kind is refused too.   2 the same entity spelled two ways matches (case, spaces).
//  3 the same kind on the same route within 2 s is one act.   4 a corrupt file starts empty and never throws; a good file round-trips.   5 many acts saturate (tanh): a lift never passes
//  MAX_LIFT, and a "more" and a "less" cancel.   6 an act dated in the future weighs at most one; old acts fade.   7 the shaded order is a stable permutation of the input.
//  9 forget of a route with no acts and reset of nothing remove none.   Launcher: an empty query has no hits; one hit per address (open, places, recent, stories); the query is plain
//  text, never a pattern; a place line is one line of at most 60 characters; recent pages exclude the open ones and repeats and unknown schemes.
//  Sign-out: a write that began under an older account generation is refused (api.ts writeCurrent); the store is emptied by account.ts (`told-paper`).
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, writeCurrent } from './api.ts';
import { defaultTitle, kindOf, pagesView, type Page } from './pages.ts';
import { addresses as recentAddresses, events as recentEvents } from './recent-pages.ts';
import { decodeSheet } from './runsheet-model.ts';
import { enqueue, flush, onAir, tapOn } from './steer.ts';
import type { Story } from './paper-model.ts';
import { listing } from './paper.ts';

export const TOLD_FILE = 'app:/data/what-i-told-the-paper.json';
export const MAX_LIFT = 0.25;
export const HALF_LIFE_DAYS = 14;
export const PLACES_PER_LIFT = 3;
export const DOUBLE_FIRE_SECONDS = 2;

export type ToldAct = { at: number; kind: 'more' | 'less'; route: string; title: string; entities: string[]; topic: string };
export type StoryFacts = { route: string; title: string; dek: string; section: string; entities: string[] };
export const addressOf = (f: { route: string }): string => 'pub:' + f.route;

// ---- the acts

const norm = (s: string): string => s.toLowerCase().split(/\s+/).filter((x) => x !== '').join(' ');

/** The subjects an act learns from: the story's entities, else its section. Empty when there is nothing. */
export function subjects(f: StoryFacts): string[] {
  const e = f.entities.map((x) => x.trim()).filter((x) => x !== '');
  if (e.length > 0) return e;
  const t = f.section.trim();
  return t === '' ? [] : [t];
}
/** The name the acknowledgement uses: the first central entity, else the section. */
export const primary = (f: StoryFacts): string => subjects(f)[0] ?? '';
/** "More on Harbor Authority" / "Less of Harbor Authority". */
export function note(kind: 'more' | 'less', f: StoryFacts): string {
  const p = primary(f);
  if (p === '') return kind === 'more' ? 'More like this' : 'Less like this';
  return kind === 'more' ? `More on ${p}` : `Less of ${p}`;
}

export class Told {
  acts: ToldAct[] = [];
  /** The clock: a data source has none, so the caller's `now` (epoch seconds) is set before each use (`at`), and with none the newest act's time plus the double-fire window. */
  constructor(public now: () => number = () => this.acts.reduce((m, a) => Math.max(m, a.at), 0) + DOUBLE_FIRE_SECONDS) {}

  record(kind: string, f: StoryFacts): ToldAct | null {
    if (kind !== 'more' && kind !== 'less') return null;
    const subj = subjects(f);
    if (subj.length === 0 || f.route === '') return null;
    const t = this.now();
    const last = [...this.acts].reverse().find((a) => a.route === f.route && a.kind === kind);
    if (last && Math.abs(t - last.at) < DOUBLE_FIRE_SECONDS) return null;
    const a: ToldAct = { at: t, kind, route: f.route, title: f.title, entities: subj, topic: f.section };
    this.acts.push(a);
    return a;
  }
  /** Forget the acts on one story. Returns how many were removed. */
  forget(route: string): number { const n = this.acts.length; this.acts = this.acts.filter((a) => a.route !== route); return n - this.acts.length; }
  reset(): number { const n = this.acts.length; this.acts = []; return n; }
  private weight(a: ToldAct, t: number): number { return (a.kind === 'more' ? 1 : -1) * Math.pow(0.5, Math.max(0, t - a.at) / (HALF_LIFE_DAYS * 86400)); }
  /** The story's lift, in -MAX_LIFT..+MAX_LIFT. */
  lift(f: StoryFacts): number {
    const mine = new Set(subjects(f).map(norm));
    if (mine.size === 0 || this.acts.length === 0) return 0;
    const t = this.now();
    let s = 0;
    for (const a of this.acts) {
      const theirs = new Set(a.entities.map(norm));
      let shared = 0;
      for (const m of mine) if (theirs.has(m)) shared += 1;
      let m = Math.min(shared, 2);
      if (m === 0 && f.section !== '' && norm(f.section) === norm(a.topic)) m = 0.35;
      s += this.weight(a, t) * m;
    }
    return MAX_LIFT * Math.tanh(s / 1.5);
  }
  /** The paper's order, shaded: a permutation of `stories`, each with its lift. Stable for equal keys. */
  order(stories: StoryFacts[]): { story: StoryFacts; lift: number }[] {
    return stories.map((s, i) => { const l = this.lift(s); return { i, s, l, k: i - (l / MAX_LIFT) * PLACES_PER_LIFT }; })
      .sort((a, b) => (a.k !== b.k ? a.k - b.k : a.i - b.i)).map((x) => ({ story: x.s, lift: x.l }));
  }
  /** "Why this story": what the listener's acts did, after "Ranked by the paper.". */
  why(f: StoryFacts): string {
    const parts: string[] = [];
    for (const e of subjects(f)) {
      const n = norm(e);
      const count = (k: string): number => this.acts.filter((a) => a.kind === k && a.entities.some((x) => norm(x) === n)).length;
      const more = count('more'), less = count('less');
      if (more > 0) parts.push(`you asked for more on ${e} (${more === 1 ? 'once' : `${more} times`})`);
      if (less > 0) parts.push(`you asked for less of ${e} (${less === 1 ? 'once' : `${less} times`})`);
    }
    return parts.length === 0 ? 'Ranked by the paper. Nothing you told it changes this.' : 'Ranked by the paper. Shaded for you: ' + parts.join('; ') + '.';
  }
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const two = (n: number): string => String(n).padStart(2, '0');
/** "14 Nov 22:13" (the zone is the phone's; tests pass UTC). */
export function when(at: number, utc = false): string {
  const d = new Date(at * 1000);
  return utc ? `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${two(d.getUTCHours())}:${two(d.getUTCMinutes())}` : `${d.getDate()} ${MONTHS[d.getMonth()]} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

// ---- entities for a story

const STOP = new Set(['about', 'after', 'again', 'their', 'there', 'these', 'those', 'which', 'while', 'where', 'other', 'could', 'would', 'should', 'being', 'before', 'during', 'within', 'without', 'against', 'among']);
const cap = (w: string): string => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase();
/** Capitalised runs after the first word ("The council of Port Hale" -> "Port Hale"), else up to two long words. */
export function derive(title: string): string[] {
  const words = title.split(/\s+/).map((w) => w.replace(/^[\p{P}]+|[\p{P}]+$/gu, '')).filter((w) => w !== '');
  const out: string[] = [];
  let run: string[] = [];
  words.forEach((w, i) => {
    if (i > 0 && w.charAt(0) !== w.charAt(0).toLowerCase()) run.push(w);
    else if (run.length > 0) { out.push(run.join(' ')); run = []; }
  });
  if (run.length > 0) out.push(run.join(' '));
  const found = out.length > 0 ? out : words.filter((w) => w.length >= 6 && !STOP.has(w.toLowerCase())).slice(0, 2).map(cap);
  return found.slice(0, 3);
}
export function entitiesOf(route: string, title: string, given: string[] = [], map: Record<string, string[]> = {}): string[] {
  const g = given.map((x) => x.trim()).filter((x) => x !== '');
  if (g.length > 0) return g;
  if ((map[route] ?? []).length > 0) return map[route];
  return derive(title);
}
export function factsOf(s: Pick<Story, 'route' | 'title' | 'dek' | 'section' | 'entities'>): StoryFacts {
  return { route: s.route, title: s.title, dek: s.dek, section: s.section, entities: entitiesOf(s.route, s.title, (s.entities ?? []).map((e) => e.label)) };
}

// ---- the stored acts

/** The caller's clock (epoch seconds) for this use; none (0, or a test's own) leaves the newest act's time. */
let pinned = false;
function tick(now: number | undefined): void { if (!pinned) told.now = now !== undefined && now > 0 ? () => now : () => told.acts.reduce((m, a) => Math.max(m, a.at), 0) + DOUBLE_FIRE_SECONDS; }

let told = new Told();
let loaded = false;
let loading: Promise<void> | null = null;
let epoch = 0;

async function load(storage: Storage | undefined): Promise<void> {
  if (loaded) return;
  if (!loading) {
    const mine = epoch;
    const p: Promise<void> = (async () => {
      let stored: unknown = null;
      if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(TOLD_FILE))); } catch { /* none kept, or damaged: empty */ } }
      if (mine !== epoch) return;                                              // wiped while reading: the old account's file
      const raw = Array.isArray((stored as { acts?: unknown } | null)?.acts) ? (stored as { acts: unknown[] }).acts : [];
      told.acts = raw.filter((a): a is ToldAct => {
        const o = a as Partial<ToldAct> | null;
        return !!o && (o.kind === 'more' || o.kind === 'less') && typeof o.route === 'string' && o.route !== '' && typeof o.at === 'number' && Number.isFinite(o.at)
          && typeof o.title === 'string' && Array.isArray(o.entities) && o.entities.every((e) => typeof e === 'string') && typeof o.topic === 'string';
      });
      loaded = true;
    })().finally(() => { if (loading === p) loading = null; });
    loading = p;
  }
  await loading;
}

let saving: Promise<unknown> = Promise.resolve();
function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = saving.then(() => writeCurrent(gen, storage, TOLD_FILE, new TextEncoder().encode(JSON.stringify({ acts: told.acts }))), () => undefined);
  saving = run.catch(() => undefined);
  return run.then(() => undefined);
}

/** The acts as the store holds them (tests, and the learned screen). */
export async function acts(storage: Storage | undefined): Promise<ToldAct[]> { await load(storage); return [...told.acts]; }

// ---- what the paper has learned (Settings)

export type LearnedItem = { line: string; kind: string; ago: string };
export type LearnedInfo = { rows: LearnedItem[]; count: number; message: string };

/** One line per act, newest first, in plain words; no empty "()" for an act with no entities. */
export function learnedRows(list: ToldAct[], utc = false): LearnedItem[] {
  return [...list].sort((a, b) => b.at - a.at).map((a) => ({
    line: `${a.kind === 'more' ? 'More like this' : 'Less of this'}: ${a.title}${a.entities.length === 0 ? '' : ` (${a.entities.join(', ')})`}`,
    kind: a.kind, ago: when(a.at, utc),
  }));
}
export async function learnedInfo(storage: Storage | undefined): Promise<LearnedInfo> {
  await load(storage);
  const rows = learnedRows(told.acts);
  return { rows, count: rows.length, message: rows.length === 0 ? 'Nothing yet. Swipe right on a story you want more of, and it shows here.' : '' };
}
export async function learnedReset(storage: Storage | undefined): Promise<{ ok: boolean; message: string }> {
  const gen = accountGeneration();
  await load(storage);
  const n = told.reset();
  if (n > 0) await save(storage, gen);
  return { ok: true, message: '' };
}

// ---- the launcher

export type Place = { slug: string; name: string; address: string; line: string };
export type Hit = { address: string; title: string; kind: string; detail: string };
export type LauncherData = { query: string; hasQuery: boolean; places: Place[]; recent: Hit[]; hits: Hit[]; stories: number; searchNote: string };

export const PLACES: { name: string; address: string }[] = [
  { name: 'Paper', address: 'paper:front' }, { name: 'Textures', address: 'textures:' }, { name: 'Files', address: 'file:/' }, { name: 'Mail', address: 'mailbox:inbox' },
  { name: 'Podcasts', address: 'podcasts:' }, { name: 'Activity', address: 'activity:' }, { name: 'Settings', address: 'settings:root' },
];

const oneLine = (s: string): string => { const flat = s.split(/\s+/).filter((x) => x !== '').join(' '); return [...flat].length > 60 ? [...flat].slice(0, 59).join('') + '…' : flat; };
/** The launcher's places, in order, each with a one-line "what is new there"; the paper's line is the top story's title. */
export function places(topStory: string | undefined, extra: Record<string, string> = {}): Place[] {
  return PLACES.map((p) => {
    const slug = p.name.toLowerCase();
    let line = extra[slug] ?? '';
    if (slug === 'paper' && topStory) line = topStory;
    return { slug, name: p.name, address: p.address, line: oneLine(line) };
  });
}

/** Search over open pages, places, recent pages and the front list's stories: plain substring on lower-cased text, nothing is interpreted. Best score first, then group, then order. */
export function localHits(query: string, open: { address: string; title: string }[], pl: Place[], recent: { address: string; title: string }[], stories: StoryFacts[], limit = 12): Hit[] {
  const q = query.toLowerCase().split(/\s+/).filter((x) => x !== '').join(' ');
  if (q === '') return [];
  const score = (text: string): number => {
    const t = text.toLowerCase();
    if (t.startsWith(q)) return 3;
    if (t.split(/[^\p{L}\p{N}]+/u).some((w) => w !== '' && w.startsWith(q))) return 2;
    return t.includes(q) ? 1 : 0;
  };
  const out: { s: number; g: number; n: number; h: Hit }[] = [];
  const seen = new Set<string>();
  const add = (g: number, s: number, h: Hit): void => { if (s <= 0 || seen.has(h.address)) return; seen.add(h.address); out.push({ s, g, n: out.length, h }); };
  for (const p of open) add(0, score(p.title), { address: p.address, title: p.title, kind: 'open', detail: 'Open' });
  for (const p of pl) add(1, Math.max(score(p.name), score(p.line) > 0 ? 1 : 0), { address: p.address, title: p.name, kind: 'place', detail: p.line });
  for (const p of recent) add(2, score(p.title), { address: p.address, title: p.title, kind: 'recent', detail: 'Recent' });
  for (const s of stories) {
    const sc = Math.max(score(s.title), score(s.dek) > 0 ? 1 : 0, s.entities.some((e) => score(e) > 0) ? 1 : 0);
    add(3, sc, { address: addressOf(s), title: s.title, kind: 'story', detail: s.section });
  }
  return out.sort((a, b) => (a.s !== b.s ? b.s - a.s : a.g !== b.g ? a.g - b.g : a.n - b.n)).slice(0, limit).map((x) => x.h);
}

/** A page's title for a list: a story's own title when known, else the address's default. */
export function recentTitle(address: string, stories: StoryFacts[]): string {
  if (address.startsWith('pub:')) { const s = stories.find((x) => addressOf(x) === address); if (s) return s.title; }
  return defaultTitle(address);
}

/** The front list as the launcher reads it, in the paper's order shaded by what the listener told it. A list that cannot be asked is empty (the local rows still show). */
async function shadedStories(storage: Storage | undefined, front: () => Promise<Story[]>): Promise<StoryFacts[]> {
  await load(storage);
  let list: Story[] = [];
  try { list = await front(); } catch { /* no front list: no stories */ }
  return told.order(list.map(factsOf)).map((x) => x.story);
}

const frontStories = async (storage: Storage | undefined): Promise<Story[]> => (await listing('', storage)).stories;

export async function launcherData(a: { q: string; now?: number }, storage: Storage | undefined, front: () => Promise<Story[]> = () => frontStories(storage)): Promise<LauncherData> {
  const q = typeof a.q === 'string' ? a.q : '';
  tick(a.now);
  const stories = await shadedStories(storage, front);
  const view = await pagesView(storage);
  const open: Page[] = view.rows;
  const pl = places(stories[0]?.title);
  const recent = recentAddresses(await recentEvents(storage), new Set(open.map((p) => p.address)))
    .map((address) => ({ address, kind: kindOf(address), title: recentTitle(address, stories) }));
  const hasQuery = q.trim() !== '';
  return {
    query: q, hasQuery, places: pl,
    recent: recent.map((p) => ({ address: p.address, title: p.title, kind: p.kind, detail: 'Recent' })),
    hits: localHits(q, open, pl, recent, stories), stories: stories.length, searchNote: '',
  };
}

// ---- "more of this"

export type Ack = { message: string };
/** The listener asked for more of a story (the button says "Noted"). Records the act, the steering tap and answers the acknowledgement. A page that is not a story, or a story the
 * front list does not hold, is acknowledged with nothing. `sheet` is the run sheet's JSON (the radio page's facts), so the tap names the entry on air. */
export async function storyMore(a: { address: string; surface: string; sheet: string; now?: number }, storage: Storage | undefined, front: () => Promise<Story[]> = () => frontStories(storage)): Promise<Ack> {
  const gen = accountGeneration();
  tick(a.now);
  await load(storage);
  if (!a.address.startsWith('pub:')) return { message: '' };
  const route = a.address.slice(4);
  let list: Story[] = [];
  try { list = await front(); } catch { /* none */ }
  const s = list.find((x) => x.route === route);
  if (!s) return { message: '' };
  const f = factsOf(s);
  if (!told.record('more', f)) return { message: '' };
  if (gen !== accountGeneration()) return { message: '' };
  await save(storage, gen);
  let entry;
  try { entry = onAir(a.sheet ? decodeSheet(JSON.parse(a.sheet)) : undefined); } catch { entry = undefined; }
  const tap = tapOn(entry, 'more', a.address, a.now ?? 0);
  if (tap) { await enqueue(storage, tap); try { await flush(storage); } catch { /* kept queued: the next act sends it */ } }       // awaited: a fetch that goes on after its answer ended is never run
  return { message: note('more', f) };
}

/** Tests: the clock the acts are dated and weighed by. */
export function setClock(now: (() => number) | null): void { pinned = now !== null; if (now) told.now = now; }
/** Sign-out and tests. */
export function forget(): void { const now = told.now; told = new Told(); if (pinned) told.now = now; loaded = false; loading = null; epoch += 1; }
export async function wipeTold(storage: Storage | undefined): Promise<void> { forget(); if (!storage) return; try { await storage.fs.rm(TOLD_FILE); } catch { /* absent */ } }
