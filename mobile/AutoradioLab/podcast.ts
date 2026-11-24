// The podcast app's data and acts, in shared code. Replaces the Swift PodcastService, PodcastFeed, PodcastSyncCodec and PodcastPlatformCodec.
//
// Where each thing lives (decided from the server's code, 2026-11-15, kept):
//  - Show search and the suggestions: Apple's public directory, asked from the phone. It takes no key, and the phone holds none: the paid or
//    keyed services (the platform's transcripts, popular list, subscription list) are the server's routes, reached with the listener's session
//    (api.ts `authedJSON`), never with a vendor key. The server's own search route asks the same public API and needs the computer awake.
//  - Episodes: the publisher's RSS feed, fetched and parsed on the phone and kept in `app:/data/podcast-feeds`. A feed can be on any host the
//    listener names, so the grant is `net.fetch https:` (EXACT.md: "A server named at run time"); plain http is not asked (the phone refused it
//    before).
//  - Subscriptions and positions: `app:/data/podcasts-library.json`. A subscribe is also pushed to the computer, and the computer's list is
//    pulled in; the server has no unsubscribe route, so an unsubscribe is local and remembered, and a pull never brings it back.
//  - Playing: the deck is native. A play answers `play` (the url, the id, where to start) as data; the page hands it to the native call.
//
// Ways this can fail (the Swift list, kept, then what only shared code adds):
//  1. Subscribing twice, unsubscribing a show that is not subscribed, a feed written two ways (case, trailing slash): one subscription.
//  2. A feed that fails to load: a plain sentence and a retry, never an empty page; a cached copy is shown when there is one.
//  3. A clip opens an episode whose feed is not loaded yet or has left the feed: the page still offers play from the clip's own audio, with the
//     title the clip carried (the address carries them, so a relaunch keeps them).
//  4. A play position for an episode that is not a podcast id, or for no duration: ignored / kept without a duration.
//  5. Unplayed: no position. Started: a position. Finished: within 20 s of the end, or reported finished.
//  6. A search answered late for an older query: dropped. Directory failing: a sentence; subscriptions still work.
//  7. The server refusing or not answering a sync: the phone's list is unchanged and a note says it is not synced; signed out: no note.
//  8. The library file missing or damaged: an empty library, never a crash.
//  9. A source is asked again for every second a podcast plays (the playhead is an argument): a feed that failed is not fetched again until the
//     page's retry (`tick`) changes; a feed already fetched this session is not fetched again until it is stale by the server's own clock.
// 10. Time is never read (`Date.now()` is refused): "now" is the Date header of the last answer any server gave, kept in the library file.

import { lap } from './lap.ts';
import type { Result, Storage } from './app.contract.d.ts';
import { authedJSON, SignedOutError, getJSON, TIMEOUT_MS, writeCurrent, accountGeneration } from './api.ts';
import { NotAFeed, parseFeed } from './podcast-feed.ts';
import {
  HOME, TOP_URL, clock, decodeChartNews, decodePopular, decodeSearch, decodeSubscriptions, decodeTranscript, durationLabel, durationsAgree, episodeAddress,
  episodeKey, feedKey, https, lookupURL, parseEpisode, parseEpisodeKey, parseShow, relative, reportBody, searchURL, show as makeShow, showAddress, webURL,
  type Episode, type Feed, type Hint, type Show, type Transcript,
} from './podcast-model.ts';

export const LIBRARY_FILE = 'app:/data/podcasts-library.json';
export const FEED_DIR = 'app:/data/podcast-feeds';
export const STALE_AFTER = 15 * 60;
export const FINISHED_WITHIN = 20;
export const MAX_FEED_BYTES = 25_000_000;
/** The pause before a typed query is asked, so each keystroke is not a request. A deadline, not a rate limit. */
export const search = { pause: 350 };

export type Position = { seconds: number; duration: number | null; finished: boolean; at: number };
type Library = { subscriptions: { show: Show; at: number }[]; removed: string[]; synced: string[]; positions: Record<string, Position>; clock: number };
type Now = { id: string; clip: boolean; start: number; elapsed: number; duration: number };

const emptyLibrary = (): Library => ({ subscriptions: [], removed: [], synced: [], positions: {}, clock: 0 });

// ---- state kept between answers (cleared by `forget`, which a test calls, and by `wipe`, which sign-out calls)

let lib: Library | null = null;
/** Raised by every wipe: a feed that was being fetched when the listener signed out is not kept. */
let generation = 0;
const feeds = new Map<string, Feed>();
const known = new Map<string, Show>();
const failed = new Map<string, string>();
const attempted = new Set<string>();
const inflight = new Map<string, Promise<void>>();
const transcripts = new Map<string, Transcript>();
const transcriptAsked = new Set<string>();
const searches = new Map<string, Show[] | 'failed'>();
let latestQuery = '';
let suggestions: Show[] = [];
let suggestionsAt = -1;
let syncAt = -1;
let syncNote = '';
let lastTick = 0;
let rev = 0;
let reportedFeeds = false;
let reportedNow = '';
const played = new Set<string>();

export function forget(): void {
  generation++;
  lib = null; feeds.clear(); known.clear(); failed.clear(); attempted.clear(); inflight.clear(); transcripts.clear(); transcriptAsked.clear(); searches.clear();
  latestQuery = ''; suggestions = []; suggestionsAt = -1; syncAt = -1; syncNote = ''; lastTick = 0; rev = 0; reportedFeeds = false; reportedNow = ''; played.clear();
}

/** Sign-out: the library, every cached feed and transcript, and the files. Called by the sign-out wipe (lane exact-signin). */
export async function wipe(storage: Storage | undefined): Promise<void> {
  forget();
  if (!storage) return;
  try { await storage.fs.rm(LIBRARY_FILE); } catch { /* none */ }
  try { await storage.fs.rm(FEED_DIR); } catch { /* none */ }
}

function resetFor(tick: number): void {
  if (tick === lastTick) return;
  lastTick = tick;
  attempted.clear(); failed.clear(); transcriptAsked.clear(); searches.clear(); suggestionsAt = -1; syncAt = -1;
}

// ---- time: the Date header of the last answer any server gave

let nowSeconds = 0;
const now = (): number => nowSeconds || (lib?.clock ?? 0);
function sawTime(headerMs: number | null): void {
  if (headerMs !== null && headerMs / 1000 > nowSeconds) { nowSeconds = headerMs / 1000; if (lib) lib.clock = nowSeconds; }
}

// ---- files

const enc = new TextEncoder(), dec = new TextDecoder();

let libGen = 0;       // the seam's account generation the library in memory was loaded under: its file writes refuse after a sign-out began (writeCurrent)
async function loadLibrary(storage: Storage | undefined): Promise<Library> {
  if (lib) return lib;
  const g = accountGeneration();
  let l = emptyLibrary();
  if (storage) {
    try {
      const o = JSON.parse(dec.decode(await storage.fs.readFile(LIBRARY_FILE))) as Partial<Library>;
      if (o && Array.isArray(o.subscriptions)) {
        l = { ...emptyLibrary(), ...o, subscriptions: o.subscriptions.filter((s) => s && s.show && typeof s.show.feedURL === 'string'), positions: o.positions && typeof o.positions === 'object' ? o.positions : {} };
        if (!Array.isArray(l.removed)) l.removed = [];
        if (!Array.isArray(l.synced)) l.synced = [];
      }
    } catch { /* missing or damaged: an empty library */ }
  }
  lib = l; libGen = g;
  for (const s of l.subscriptions) known.set(feedKey(s.show.feedURL), s.show);
  return l;
}

async function saveLibrary(storage: Storage | undefined): Promise<void> {
  if (!storage || !lib) return;
  await writeCurrent(libGen, storage, LIBRARY_FILE, enc.encode(JSON.stringify(lib)));      // a full disk loses only the copy; a sign-out in flight writes nothing, and a file that lands after it is removed
}

/** FNV-1a over the bytes, twice with different seeds: a file name, not a secret. */
function feedFile(norm: string): string {
  let a = 0x811c9dc5, b = 0x01000193 ^ 0xdeadbeef;
  for (const byte of enc.encode(norm)) {
    a = Math.imul(a ^ byte, 0x01000193) >>> 0;
    b = Math.imul(b ^ byte, 0x85ebca6b) >>> 0;
  }
  return `${FEED_DIR}/${a.toString(16).padStart(8, '0')}${b.toString(16).padStart(8, '0')}.json`;
}

async function cachedFeed(feedURL: string, storage: Storage | undefined): Promise<Feed | null> {
  const n = feedKey(feedURL);
  const have = feeds.get(n);
  if (have) return have;
  if (!storage) return null;
  const gen = generation;                                                    // a file read that lands after a sign-out is not kept
  try {
    const f = JSON.parse(dec.decode(await storage.fs.readFile(feedFile(n)))) as Feed;
    if (f && f.show && Array.isArray(f.episodes)) { if (gen === generation) feeds.set(n, f); return gen === generation ? f : null; }
  } catch { /* none */ }
  return null;
}

// ---- the network

async function fetchBody(url: string, accept: string): Promise<{ text: string; date: number | null }> {
  lap('net get feed start');
  const response = await fetch(url, { method: 'GET', headers: { accept, 'user-agent': 'Autoradio' }, exactTimeout: TIMEOUT_MS } as RequestInit);
  lap(`net get feed responded ${response.status}`);                       // no address in the line: a feed's address is the listener's own
  if (!response.ok) throw new Error(`${response.status}`);
  const len = Number(response.headers.get('content-length') ?? '0');
  if (len > MAX_FEED_BYTES) throw new TooBig();
  const text = await response.text();
  if (text.length > MAX_FEED_BYTES) throw new TooBig();
  const d = response.headers.get('date');
  const parsed = d ? Date.parse(d) : NaN;
  return { text, date: Number.isFinite(parsed) ? parsed : null };
}
class TooBig extends Error {}

async function fetchJSON(url: string): Promise<unknown> {
  const r = await fetchBody(url, 'application/json');
  sawTime(r.date);
  return JSON.parse(r.text);
}

// ---- feeds

async function load(feedURL: string, storage: Storage | undefined): Promise<void> {
  const n = feedKey(feedURL);
  const gen = generation;
  const apiGen = accountGeneration();                                        // the seam's account generation: the file write below refuses after a sign-out began
  const candidates = [https(feedURL)];
  let lastError = 'The feed could not be reached.';
  let result: Feed | null = null;
  for (const c of candidates) {
    if (webURL(c) === '') { lastError = 'That is not a web address.'; continue; }
    try {
      const r = await fetchBody(c, 'application/rss+xml, application/xml, text/xml, application/json, */*');
      sawTime(r.date);
      result = parseFeed(r.text, feedURL, now());
      break;
    } catch (e) {
      if (e instanceof NotAFeed) lastError = e.message;
      else if (e instanceof TooBig) lastError = 'That feed is too large to open.';
      else lastError = 'The feed could not be reached. Check your connection.';
    }
  }
  if (gen !== generation) return;                                           // signed out while the feed was fetched: it is not kept
  const l = await loadLibrary(storage);
  if (result) {
    const k = known.get(n);
    if (k) { if (result.show.author === '') result.show.author = k.author; if (result.show.artworkURL === '') result.show.artworkURL = k.artworkURL; }
    feeds.set(n, result); failed.delete(n); known.set(n, result.show);
    const i = l.subscriptions.findIndex((s) => feedKey(s.show.feedURL) === n);
    if (i >= 0) { l.subscriptions[i].show = result.show; await saveLibrary(storage); }
    if (storage) {
      try { await storage.fs.mkdir(FEED_DIR); } catch { /* exists */ }
      await writeCurrent(apiGen, storage, feedFile(n), enc.encode(JSON.stringify(result)));      // the copy may be lost, not the feed; a sign-out in flight writes nothing
    }
  } else failed.set(n, lastError);
}

/** Make the feed ready: fetched once a session (again when stale by the server's clock, or when `force`); one fetch per feed at a time. */
async function ensureFeed(feedURL: string, storage: Storage | undefined, force = false): Promise<Feed | null> {
  const n = feedKey(feedURL);
  const have = await cachedFeed(feedURL, storage);
  const t = now();
  const stale = !have || t === 0 || t - have.fetchedAt >= STALE_AFTER;
  if (force || (!attempted.has(n) && stale)) {
    attempted.add(n);
    let p = inflight.get(n);
    if (!p) { p = load(feedURL, storage).finally(() => inflight.delete(n)); inflight.set(n, p); }
    await p;
  } else {
    const p = inflight.get(n);
    if (p) await p;
  }
  return feeds.get(n) ?? have;
}

// ---- subscriptions and positions

const isSubscribed = (l: Library, feedURL: string): boolean => l.subscriptions.some((s) => feedKey(s.show.feedURL) === feedKey(feedURL));
const position = (l: Library, key: string): Position | null => l.positions[key] ?? null;

function resumeSeconds(l: Library, key: string): number | null {
  const p = position(l, key);
  return p && !p.finished && p.seconds >= 5 ? p.seconds : null;
}

function report(l: Library, key: string, seconds: number, duration: number | null, finished = false): boolean {
  if (parseEpisodeKey(key) === null || !Number.isFinite(seconds) || seconds < 0) return false;
  const done = finished || (duration !== null && duration > 0 && seconds >= duration - FINISHED_WITHIN);
  const before = l.positions[key];
  l.positions[key] = { seconds, duration: duration ?? before?.duration ?? null, finished: done, at: now() };
  return true;
}

async function pushShow(s: Show): Promise<boolean> {
  try {
    await authedJSON('/api/podcast/subscriptions', { method: 'POST', body: { feed_url: s.feedURL, title: s.title, author: s.author, artwork_url: s.artworkURL } });
    return true;
  } catch { return false; }
}

async function reportFeeds(l: Library, nowPlaying: { feedURL: string; episodeGuid: string; audioURL: string } | null = null): Promise<void> {
  try { await authedJSON('/api/platform/podcast-feeds', { method: 'POST', body: reportBody(l.subscriptions.map((s) => s.show), nowPlaying) }); } catch { /* 404, signed out or offline: silent */ }
}

async function syncIfDue(storage: Storage | undefined, l: Library): Promise<void> {
  const t = now();
  if (syncAt >= 0 && t > 0 && t - syncAt < 600) return;
  if (syncAt >= 0 && t === 0) return;
  syncAt = t;
  const gen = generation;                                                    // the answer of an account that was signed out meanwhile is dropped
  let list: Show[];
  try { list = decodeSubscriptions((await authedJSON('/api/podcast/subscriptions', { query: { limit: 200 } })).body); }
  catch (e) { if (gen !== generation) return; syncNote = e instanceof SignedOutError ? '' : "Not in step with your computer's podcasts yet: it did not answer. Subscriptions here are kept on this phone."; return; }
  if (gen !== generation) return;
  const removed = new Set(l.removed.map(feedKey));
  const added: Show[] = [];
  for (const s of list) {
    const n = feedKey(s.feedURL);
    if (removed.has(n) || isSubscribed(l, n)) continue;
    l.subscriptions.push({ show: s, at: now() }); known.set(n, s);
    if (!l.synced.includes(n)) l.synced.push(n);
    added.push(s);
  }
  const onServer = new Set(list.map((s) => feedKey(s.feedURL)));
  const toPush = l.subscriptions.map((s) => s.show).filter((s) => !onServer.has(feedKey(s.feedURL)));
  let failedPush = 0;
  for (const s of toPush) { if (await pushShow(s)) { const n = feedKey(s.feedURL); if (!l.synced.includes(n)) l.synced.push(n); } else failedPush++; }
  if (gen !== generation) return;
  await saveLibrary(storage);
  if (gen !== generation) return;
  syncNote = failedPush === 0 ? "In step with your computer's podcasts." : 'Some shows could not be sent to your computer yet.';
}

async function loadSuggestions(): Promise<void> {
  const t = now();
  if (suggestionsAt >= 0 && (t === 0 || t - suggestionsAt < 600)) return;
  suggestionsAt = t;
  const gen = generation;
  let out: Show[] = [];
  try { const r = await getJSON('/api/platform/podcast-popular', { limit: 30 }); sawTime(r.serverTime); out = decodePopular(r.body); } catch { /* 404 or offline: the chart */ }
  if (out.length === 0) {
    try {
      const chart = decodeChartNews(await fetchJSON(TOP_URL));
      if (chart.length > 0) out = decodeSearch(await fetchJSON(lookupURL(chart.map((c) => c.id))));
    } catch { out = []; }
  }
  if (gen !== generation) return;                                           // the chart is public, but the held list belongs to this account's session
  suggestions = out;
  for (const x of out) if (!known.has(feedKey(x.feedURL))) known.set(feedKey(x.feedURL), x);
}

async function runSearch(q: string): Promise<void> {
  if (searches.has(q)) return;
  if (search.pause > 0) await new Promise<void>((r) => setTimeout(r, search.pause));
  if (q !== latestQuery) return;                                            // a newer query was typed while this one waited: dropped
  const gen = generation;
  try {
    const shows = decodeSearch(await fetchJSON(searchURL(q)));
    if (q !== latestQuery || gen !== generation) return;
    searches.set(q, shows);
    for (const x of shows) if (!known.has(feedKey(x.feedURL))) known.set(feedKey(x.feedURL), x);
  } catch { if (q === latestQuery) searches.set(q, 'failed'); }
}

/** Search queries are compared by their words: lower case, spaces collapsed. */
export const normalize = (raw: string): string => raw.trim().replace(/\s+/g, ' ');

// ---- views

type HomeView = Result<'podcastHome'>;
type ShowView = Result<'podcastShow'>;
type EpisodeView = Result<'podcastEpisode'>;
type ShowRow = HomeView['shows'][number];
type EpisodeRow = ShowView['episodes'][number];
type Ack = Result<'podcastSubscribe'>;

const NO_PLAY: Ack['play'] = { op: '', id: '', url: '', title: '', show: '', start: 0, clip: '', feed: '', record: '' };
const ack = (status: string, message: string, extra: Partial<Ack> = {}): Ack => ({ status, message, play: NO_PLAY, open: '', openTitle: '', ...extra });

const unplayedCount = (l: Library, f: Feed): number => f.episodes.filter((e) => e.playable && position(l, episodeKey(feedKey(f.show.feedURL), e.id)) === null).length;

function row(l: Library, s: Show, subscribed: boolean): ShowRow {
  const r: ShowRow = {
    feedURL: s.feedURL, address: showAddress(s.feedURL), title: s.title, author: s.author, artwork: https(s.artworkURL), detail: '', latest: '', latestStamp: '',
    subscribed, unplayed: false, unplayedCount: 0,
  };
  const f = feeds.get(feedKey(s.feedURL));
  const e = f?.episodes[0];
  if (f && e) {
    const n = unplayedCount(l, f);
    r.latest = e.title; r.latestStamp = relative(e.published, now(), tzOffset());
    r.unplayedCount = n; r.unplayed = n > 0;
    r.detail = n > 0 ? `${n} unplayed · ${e.title}` : e.title;
  } else r.detail = s.author;
  return r;
}

/** The device's offset from UTC in minutes (east positive), for "Today" and "Yesterday". */
export const zone = { offset: null as number | null };
function tzOffset(): number {
  if (zone.offset !== null) return zone.offset;
  try { return -new Date(now() * 1000).getTimezoneOffset(); } catch { return 0; }
}

export type HomeArgs = { q: string; address: string; tick: number };

export async function podcastHome(a: HomeArgs, storage: Storage | undefined, saved = false): Promise<HomeView> {
  const v: HomeView = { status: 'ready', message: '', query: '', searchNote: '', syncNote: '', searching: false, hasQuery: false, shows: [], results: [], suggestions: [] };
  if (a.address !== HOME) { v.status = saved ? '' : 'idle'; return v; }
  const q = normalize(a.q);
  if (saved && q !== '') { v.status = ''; return v; }                       // a search is the network's: nothing to show from the phone's copy
  const gen = accountGeneration();
  if (!saved) resetFor(a.tick);
  const l = await loadLibrary(storage);
  v.query = q; v.hasQuery = q !== '';
  if (!saved) {
    latestQuery = q;
    // The computer's list first (it may add shows), then everything the page needs together: one fetch per feed, and the suggestions or the search.
    await Promise.all([syncIfDue(storage, l), q === '' ? loadSuggestions() : runSearch(q)]);
    await Promise.all(l.subscriptions.map((s) => ensureFeed(s.show.feedURL, storage)));
  } else await Promise.all(l.subscriptions.map((s) => cachedFeed(s.show.feedURL, storage)));      // only what the phone kept: no sync, no suggestions, no request
  if (saved && (gen !== accountGeneration() || l.subscriptions.length === 0)) { v.status = ''; return v; }
  const subs = l.subscriptions.map((s) => s.show);                         // read after the feeds: a feed's own title replaces the address
  if (!saved && !reportedFeeds) { reportedFeeds = true; await reportFeeds(l); }
  const shows = subs.map((s, i) => ({ r: row(l, s, true), i, t: feeds.get(feedKey(s.feedURL))?.episodes[0]?.published ?? 0 }));
  shows.sort((x, y) => (x.t !== y.t ? y.t - x.t : x.i - y.i));
  v.shows = shows.map((x) => x.r);
  if (saved) return v;
  v.syncNote = syncNote;
  if (q === '') {
    v.suggestions = suggestions.filter((s) => !isSubscribed(l, s.feedURL)).map((s) => row(l, s, false));
  } else {
    const found = searches.get(q);
    if (found === 'failed') v.searchNote = 'Could not search podcasts right now. Check your connection and try again.';
    else if (found) {
      v.results = found.map((s) => row(l, s, isSubscribed(l, s.feedURL)));
      if (found.length === 0) v.searchNote = 'No podcasts found for that.';
    } else v.searchNote = 'Searching podcasts…';
  }
  v.status = subs.length === 0 ? 'empty' : 'ready';
  v.message = subs.length === 0 ? 'You have not subscribed to any podcasts yet. Search for a show above, subscribe, and its episodes will be here.' : '';
  return v;
}
/** The home from what the phone kept (the library and the feeds on disk), at once, while `podcastHome` is out (stale, then fresh). Status "" when there is nothing kept, a search is typed or the account changed. */
export const podcastHomeSaved = (a: HomeArgs, storage: Storage | undefined): Promise<HomeView> => podcastHome(a, storage, true);

function episodeRow(l: Library, e: Episode, feed: string, nowId: string): EpisodeRow {
  const key = episodeKey(feedKey(feed), e.id);
  const pos = position(l, key);
  let progress = '';
  if (pos) {
    const d = pos.duration ?? e.duration;
    if (pos.finished) progress = 'Played';
    else if (d !== null && d > pos.seconds) progress = durationLabel(d - pos.seconds) + ' left';
    else if (pos.seconds >= 5) progress = 'At ' + clock(pos.seconds);
  }
  const first = e.notes.split('\n').find((x) => x !== '') ?? '';
  return {
    key, address: episodeAddress(feed, e.id), title: e.title, stamp: relative(e.published, now(), tzOffset()), duration: durationLabel(e.duration),
    snippet: first.length > 140 ? first.slice(0, 139) + '…' : first, progress, unplayed: pos === null && e.playable, playable: e.playable, current: nowId === key,
  };
}

export type ShowArgs = { address: string; tick: number; nowId: string; rev: number };

export async function podcastShow(a: ShowArgs, storage: Storage | undefined, saved = false): Promise<ShowView> {
  const v: ShowView = { status: 'loading', message: '', feedURL: '', title: '', author: '', artwork: '', summary: '', unplayedLine: '', subscribed: false, refreshing: false, episodes: [] };
  const feed = parseShow(a.address);
  if (!feed) { if (a.address.startsWith('podcast:show/')) { v.status = saved ? '' : 'error'; v.message = saved ? '' : "This show's address is not one the app can open."; } else v.status = ''; return v; }
  const gen = accountGeneration();
  if (!saved) resetFor(a.tick);
  const l = await loadLibrary(storage);
  const f = saved ? await cachedFeed(feed, storage) : await ensureFeed(feed, storage);
  if (saved && (!f || gen !== accountGeneration())) { v.status = ''; return v; }          // nothing kept for this show (or the account changed): the page's own loading
  const k = known.get(feedKey(feed));
  v.feedURL = feed; v.subscribed = isSubscribed(l, feed);
  v.title = k?.title ?? ''; v.author = k?.author ?? ''; v.artwork = https(k?.artworkURL ?? ''); v.summary = k?.summary ?? '';
  if (f) {
    v.status = 'ready';
    v.title = f.show.title; v.author = f.show.author === '' ? v.author : f.show.author;
    if (f.show.artworkURL !== '') v.artwork = f.show.artworkURL;
    v.summary = f.show.summary === '' ? v.summary : f.show.summary;
    v.episodes = f.episodes.map((e) => episodeRow(l, e, f.show.feedURL, a.nowId));
    const n = unplayedCount(l, f);
    v.unplayedLine = n > 0 ? `${n} unplayed` : '';
    if (v.episodes.length === 0) { v.status = 'empty'; v.message = 'This feed has no episodes yet.'; }
  } else {
    const m = failed.get(feedKey(feed));
    if (m) { v.status = 'error'; v.message = m; }
  }
  return v;
}
/** The show from the feed the phone kept, at once, while `podcastShow` is out (stale, then fresh). Status "" when none is kept. */
export const podcastShowSaved = (a: ShowArgs, storage: Storage | undefined): Promise<ShowView> => podcastShow(a, storage, true);

export type EpisodeArgs = { address: string; find: string; tick: number; nowId: string; nowClip: boolean; nowStart: number; nowElapsed: number; nowDuration: number; rev: number };

export async function podcastEpisode(a: EpisodeArgs, storage: Storage | undefined): Promise<EpisodeView> {
  const v: EpisodeView = {
    status: '', message: '', address: a.address, feedURL: '', showTitle: '', showAddress: '', title: '', stamp: '', duration: '', artwork: '', playingFrom: '', resumeLabel: '',
    note: '', notes: [], subscribed: false, playable: false, current: false, fromClip: false, hasResume: false, transcriptState: '', transcriptNote: '', transcriptSource: '', transcript: [],
  };
  const parsed = parseEpisode(a.address);
  if (!parsed) { if (a.address.startsWith('podcast:episode/')) { v.status = 'error'; v.message = "This episode's address is not one the app can open."; } return v; }
  const { feed, id, hint } = parsed;
  resetFor(a.tick);
  const l = await loadLibrary(storage);
  const f = await ensureFeed(feed, storage);
  const key = episodeKey(feedKey(feed), id);
  const k = known.get(feedKey(feed));
  v.status = 'loading';
  v.feedURL = feed; v.subscribed = isSubscribed(l, feed); v.showAddress = showAddress(feed);
  v.showTitle = k?.title ?? hint?.show ?? ''; v.artwork = https(k?.artworkURL ?? '');
  const ep = f?.episodes.find((e) => e.id === id);
  if (f) { v.showTitle = f.show.title; if (f.show.artworkURL !== '') v.artwork = f.show.artworkURL; }
  let resolved: Episode | undefined = ep;
  if (!resolved && hint && (hint.enclosure !== '' || f)) {
    resolved = { id, title: hint.title === '' ? 'Episode' : hint.title, published: null, duration: null, enclosureURL: hint.enclosure, enclosureType: '', notes: '', artworkURL: '', link: '', playable: hint.enclosure !== '' };
    if (f) v.note = "This episode is no longer in the show's feed. It can still be played from the link the clip carried.";
  }
  if (!resolved) {
    const m = failed.get(feedKey(feed));
    if (m) { v.status = 'error'; v.message = m; }
    return v;
  }
  v.status = 'ready';
  v.title = resolved.title; v.stamp = relative(resolved.published, now(), tzOffset()); v.duration = durationLabel(resolved.duration);
  if (ep && ep.artworkURL !== '') v.artwork = ep.artworkURL;
  v.notes = resolved.notes.split('\n\n').map((x) => x.trim()).filter((x) => x !== '');
  v.playable = resolved.playable || (hint?.enclosure ?? '') !== '';
  if (!v.playable) v.note = 'This episode has no audio file in the feed, so it cannot be played here.';
  v.current = a.nowId === key;
  if (v.current && a.nowClip && a.nowStart > 0) { v.fromClip = true; v.playingFrom = 'Playing from ' + clock(a.nowStart); }
  else if (hint && hint.start !== null && hint.start > 0 && !v.current) v.playingFrom = 'From the clip at ' + clock(hint.start);
  const r = resumeSeconds(l, key);
  if (r !== null) { v.hasResume = true; v.resumeLabel = 'Resume from ' + clock(r); }
  // The shared transcript: shown when the platform has one; "on its way" only for an episode that was played here.
  if (!transcripts.has(key) && !transcriptAsked.has(key)) {
    transcriptAsked.add(key);
    const gen = generation;
    try {
      const t = decodeTranscript((await authedJSON('/api/platform/podcast-transcript', { query: { feed, guid: id } })).body);
      if (t && gen === generation) transcripts.set(key, t);
    } catch { /* 404, signed out or offline: none */ }
  }
  const tr = transcripts.get(key);
  if (tr) {
    v.transcriptState = 'ready'; v.transcriptSource = tr.source;
    const fileLength = (v.current && a.nowDuration > 0 ? a.nowDuration : null) ?? resolved.duration;
    const seek = durationsAgree(fileLength, tr.durationSeconds);
    if (!seek) v.transcriptNote = 'These times are for another recording of this episode, so tapping a line will not jump to it.';
    const q = a.find.trim().toLowerCase();
    let cur = -1;
    if (v.current) for (let i = 0; i < tr.blocks.length; i++) if (tr.blocks[i].start <= a.nowElapsed) cur = i;
    tr.blocks.forEach((b, i) => {
      if (q === '' || b.text.toLowerCase().includes(q)) v.transcript.push({ key: String(i), time: clock(b.start), text: b.text, seconds: b.start, current: i === cur, seekable: seek && v.playable });
    });
    if (v.transcript.length === 0 && q !== '') v.transcriptNote = 'Nothing in the transcript matches that.';
  } else if (played.has(key) || position(l, key) !== null) { v.transcriptState = 'pending'; v.transcriptNote = 'Transcript on its way.'; }
  return v;
}

// ---- acts

export async function podcastSubscribe(feedURL: string, storage: Storage | undefined): Promise<Ack> {
  const l = await loadLibrary(storage);
  const n = feedKey(feedURL);
  if (isSubscribed(l, n)) return ack('ok', 'You already subscribe to this show.');
  const base = known.get(n) ?? (feeds.get(n)?.show) ?? makeShow(feedURL, feedURL);
  const s: Show = { ...base, feedURL };
  l.subscriptions.push({ show: s, at: now() });
  l.removed = l.removed.filter((r) => feedKey(r) !== n);
  known.set(n, s);
  await saveLibrary(storage);
  if (await pushShow(s)) { if (!l.synced.includes(n)) l.synced.push(n); await saveLibrary(storage); }
  await reportFeeds(l);
  attempted.delete(n);                                                       // the new subscription is fetched when its page is asked again
  return ack('ok', 'Subscribed. New episodes will show here.');
}

export async function podcastUnsubscribe(feedURL: string, storage: Storage | undefined): Promise<Ack> {
  const l = await loadLibrary(storage);
  const n = feedKey(feedURL);
  const before = l.subscriptions.length;
  l.subscriptions = l.subscriptions.filter((s) => feedKey(s.show.feedURL) !== n);
  const did = l.subscriptions.length !== before;
  if (did) {
    if (!l.removed.some((r) => feedKey(r) === n)) l.removed.push(feedURL);
    await saveLibrary(storage);
    await reportFeeds(l);
  }
  return ack('ok', did ? 'Unsubscribed on this phone.' : '');
}

async function resolve(address: string, storage: Storage | undefined): Promise<{ url: string; title: string; show: string; key: string; feed: string; hint: Hint | null } | null> {
  const p = parseEpisode(address);
  if (!p) return null;
  const key = episodeKey(feedKey(p.feed), p.id);
  const f = await cachedFeed(p.feed, storage);
  const ep = f?.episodes.find((e) => e.id === p.id);
  const url = (ep && ep.enclosureURL !== '' ? ep.enclosureURL : null) ?? p.hint?.enclosure ?? '';
  if (url === '') return null;
  const showTitle = f?.show.title ?? known.get(feedKey(p.feed))?.title ?? p.hint?.show ?? '';
  return { url, title: ep?.title ?? p.hint?.title ?? 'Episode', show: showTitle, key, feed: p.feed, hint: p.hint };
}

/** mode: "resume" (from where it stopped, else the start), "start", "keep" (keep listening to the clip's episode), "at:<seconds>" (a transcript block). */
export async function podcastPlay(address: string, mode: string, now_: Now, storage: Storage | undefined): Promise<Ack> {
  const r = await resolve(address, storage);
  if (!r) return ack('error', 'This episode has no audio to play.');
  const l = await loadLibrary(storage);
  let start: number | null;
  let op = 'play';
  if (mode === 'keep') {
    if (now_.id === r.key && now_.clip) { return ack('ok', 'Keeping on.', { play: { ...NO_PLAY, op: 'keep', id: r.key } }); }
    const from = Math.max(r.hint?.start ?? 0, position(l, r.key)?.seconds ?? 0);
    start = from > 0 ? from : null;
  } else if (mode === 'start') start = null;
  else if (mode.startsWith('at:')) { const at = parseFloat(mode.slice(3)); start = Number.isFinite(at) && at > 0 ? at : null; }
  else start = resumeSeconds(l, r.key);
  played.add(r.key);
  return ack('ok', 'Playing.', { play: { op, id: r.key, url: r.url, title: r.title, show: r.show, start: start ?? 0, clip: '', feed: r.feed, record: '' } });
}

export async function podcastQueue(address: string, storage: Storage | undefined): Promise<Ack> {
  const r = await resolve(address, storage);
  if (!r) return ack('error', 'This episode has no audio to queue.');
  const l = await loadLibrary(storage);
  return ack('ok', 'Added to the queue.', { play: { op: 'queue', id: r.key, url: r.url, title: r.title, show: r.show, start: resumeSeconds(l, r.key) ?? 0, clip: '', feed: r.feed, record: '' } });
}

/** The playhead of what is playing, from the deck (an argument: shared code has no timer). Kept every 5 s of progress, and when it ends; the
 *  platform is told what is playing once per episode. Answers a revision that changes whenever positions did, so the pages read them again. */
export type HeadArgs = { id: string; clip: boolean; elapsed: number; duration: number };
export async function podcastPlayhead(a: HeadArgs, storage: Storage | undefined): Promise<Result<'podcastPlayhead'>> {
  const parsed = parseEpisodeKey(a.id);
  if (!parsed || !(a.elapsed > 0)) return { rev };
  const l = await loadLibrary(storage);
  const before = l.positions[a.id];
  const duration = a.duration > 0 ? a.duration : null;
  const done = duration !== null && a.elapsed >= duration - FINISHED_WITHIN;
  if (!before || Math.abs(a.elapsed - before.seconds) >= 5 || (done && !before.finished)) {
    if (report(l, a.id, a.elapsed, duration)) { rev++; await saveLibrary(storage); }
  }
  if (reportedNow !== a.id) {
    reportedNow = a.id;
    const f = await cachedFeed(parsed.feed, storage);
    const ep = f?.episodes.find((e) => e.id === parsed.id);
    if (ep && ep.enclosureURL !== '') await reportFeeds(l, { feedURL: parsed.feed, episodeGuid: parsed.id, audioURL: ep.enclosureURL });
  }
  return { rev };
}

/** What the phone's own search reads: the subscribed shows and their cached episodes (for the search lane, lane X10, to feed the index). */
export async function podcastDocuments(storage: Storage | undefined): Promise<{ show: Show; episodes: Episode[] }[]> {
  const l = await loadLibrary(storage);
  const out: { show: Show; episodes: Episode[] }[] = [];
  for (const s of l.subscriptions) out.push({ show: s.show, episodes: (await cachedFeed(s.show.feedURL, storage))?.episodes ?? [] });
  return out;
}

/** The position of an episode, for tests and for the pages. */
export async function positionOf(key: string, storage: Storage | undefined): Promise<Position | null> { return position(await loadLibrary(storage), key); }
export async function subscriptions(storage: Storage | undefined): Promise<Show[]> { return (await loadLibrary(storage)).subscriptions.map((s) => s.show); }
