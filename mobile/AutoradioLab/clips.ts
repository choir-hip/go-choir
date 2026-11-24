// The clips pages: a show's "Clips" rows and what a clip row does (Play, Queue, Open episode). Replaces the Swift HearIt.rows(feed:), ClipActions and
// the Joins module. The clip set is the server's public `GET /api/platform/podcast-clips` (a show's own with `?feed=`).
//
// "Clips play from fingerprinted audio" (owner rule): NOT MET TODAY. This file decides WHICH clip and whether the row can offer the clip at all (`usable`: the same test as the Swift
// ClipFingerprintRecord.isUsable), and clip-play.ts makes the entry (the episode's audio from the clip's start to its end, in the file the publisher serves). Finding the clip again in that file
// by fingerprint (ClipLocator.swift, native: decoding to PCM) is not wired to the deck: since the run sheet player was deleted nothing calls it, and the native store of clip records is gone.
// A clip with no usable fingerprint is offered as its episode ("Play episode"); it is never played as a clip. OWED: the player lane wires the locator to the deck, handing it the row.
//
// Ways this can fail:
//  1. A row with no id, no http(s) audio, or an empty or negative range: dropped.        2. A bad body, a 404 or a failure: no rows, silently.
//  3. The same clip twice: held once.         4. A show's feed asked for at most once per session or retry, never on every draw.
//  5. A clip the phone does not have (gone, never fetched): "That clip is not available any more."     6. A clip with no fingerprint: the episode.
//  7. Open episode for a clip with no feed: nothing happens.     8. A show lists at most ten clips, ordered by clip id; a story lists at most three, in the server's order (best first).
//  9. A fetch that lands after the account changed is dropped (the generation is taken before the request).   10. A fetched clip replaces the held one with the same id (a repaired fingerprint
//     or range is picked up); ids absent from a response are kept.   11. The whole set is asked again each time a story page is reached from another page and on a retry tick; a failed ask is retried then.

import type { Result } from './app.contract.d.ts';
import { accountGeneration, getJSON } from './api.ts';
import { episodeAddress, feedKey, parseShow } from './podcast-model.ts';

export const CLIPS_PATH = '/api/platform/podcast-clips';
export const GONE = 'That clip is not available any more.';
/** The fingerprint's own constants (docs/reference/PODCAST-CLIP-FINGERPRINT.md; Swift ClipFingerprint): version, bands, and 120 frames of head and tail. */
export const FP_VERSION = 1, FP_BANDS = 8, FP_FRAMES = 120;

export type Fingerprint = { version: number; head: string; tail: string; start: number; end: number; bitrateKbps: number; seekable: boolean };
export type Clip = {
  clipID: string; route: string; storyTitle: string; show: string; episodeTitle: string; episodeGUID: string; feedURL: string; audioURL: string;
  start: number; end: number; quote: string; fingerprint: Fingerprint | null;
  /** The server's row as it came: handed whole to the native player, which finds the clip in the served file by its fingerprint. */
  raw: Record<string, unknown>;
  /** The position in the server's answer (best first within a story); the later fetch's order replaces the earlier one's. */
  rank: number;
};

const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

function packedOK(b64: string): boolean {
  let raw: string;
  try { raw = atob(b64); } catch { return false; }
  if (raw.length < 4 + FP_BANDS || raw.charCodeAt(0) !== FP_VERSION || raw.charCodeAt(1) !== FP_BANDS) return false;
  const n = (raw.charCodeAt(2) << 8) | raw.charCodeAt(3);
  return n === FP_FRAMES && raw.length === 4 + n * FP_BANDS;                // a head or tail is always 12 s: a shorter clip has none
}

/** The phone can find this clip in a served file: both fingerprints decode and the file is seekable by bitrate. */
export function usable(fp: Fingerprint | null): boolean {
  return fp !== null && fp.version === FP_VERSION && fp.seekable && fp.bitrateKbps > 0 && fp.end > fp.start && packedOK(fp.head) && packedOK(fp.tail);
}

export function parseClip(o: unknown): Clip | null {
  if (!obj(o)) return null;
  const id = str(o.clip_id), audio = str(o.audio_url);
  const start = o.start, end = o.end;
  if (id === '' || !/^https?:\/\/[^\s/]+/i.test(audio) || typeof start !== 'number' || typeof end !== 'number' || !Number.isFinite(start) || !Number.isFinite(end) || start < 0 || end <= start) return null;
  const story = obj(o.story) ? o.story : {};
  const f = obj(o.fingerprint) ? o.fingerprint : null;
  const fingerprint: Fingerprint | null = f && typeof f.head === 'string' && typeof f.tail === 'string'
    ? { version: num(f.version), head: f.head, tail: f.tail, start: num(f.start), end: num(f.end), bitrateKbps: num(f.bitrate_kbps), seekable: f.seekable_by_bitrate === true } : null;
  return {
    clipID: id, route: str(story.route).replace(/^\//, ''), storyTitle: str(story.title), show: str(o.show), episodeTitle: str(o.episode_title), episodeGUID: str(o.episode_guid),
    feedURL: str(o.feed_url), audioURL: audio, start, end, quote: str(o.quote), fingerprint, raw: o, rank: 0,
  };
}

export function parseClips(body: unknown): Clip[] {
  if (!obj(body) || !Array.isArray(body.clips)) return [];
  return body.clips.map(parseClip).filter((c): c is Clip => c !== null);
}

/** "56 s", "1 min 5 s". */
export function lengthLabel(c: Clip): string {
  const s = Math.round(c.end - c.start);
  return s < 60 ? `${s} s` : s % 60 === 0 ? `${s / 60} min` : `${Math.floor(s / 60)} min ${s % 60} s`;
}

type Rows = Result<'showClips'>;
type Ack = Result<'clipAct'>;

export const STORY_MAX = 3, SHOW_MAX = 10;

const byID = new Map<string, Clip>();
const askedFeeds = new Set<string>();
let asked = false, asking: Promise<void> | null = null;
let lastTick = 0;
let seq = 0;
/** The story page last asked for ('' when the last page asked was not a story): a story reached from another page asks the whole set again. */
let lastStory = '';

/** For the sign-out test: how many clips are held in memory. */
export const remembered = (): number => byID.size;

export function forget(): void { byID.clear(); askedFeeds.clear(); asked = false; asking = null; lastTick = 0; lastStory = ''; }

/** `whole`: the whole set's answer, whose order is the server's best-first; a part (a show's fetch) keeps the rank a clip already has, and a clip new to us ranks after the whole set. */
function merge(clips: Clip[], whole = false): void { for (const c of clips) byID.set(c.clipID, { ...c, rank: whole ? ++seq : (byID.get(c.clipID)?.rank ?? ++seq) }); }

/** A retry resets what was asked, so the next draw asks again. */
function tickOver(tick: number): void { if (tick !== lastTick) { lastTick = tick; askedFeeds.clear(); asked = false; } }

/** The whole set (`GET /api/platform/podcast-clips?limit=200`): one request for every page, never one per story. (A data source has no clock: a pull to refresh is what asks again.) */
function ensureAll(): Promise<void> {
  if (asked) return asking ?? Promise.resolve();
  asked = true;                                                       // once a retry: a failure is asked again by the next one, never on every draw
  const gen = accountGeneration();                                    // a clip set that lands after the account changed is not kept
  const mine = (async () => { try { const body = (await getJSON(CLIPS_PATH, { limit: 200 })).body; if (gen === accountGeneration()) merge(parseClips(body), true); } catch { /* the route is missing or down: nothing, silently */ } })();
  asking = mine;
  return mine;
}

export function row(c: Clip): Rows['rows'][number] {
  return { clipID: c.clipID, show: c.show, episode: c.episodeTitle, length: lengthLabel(c), quote: c.quote, story: c.storyTitle, episodeOnly: !usable(c.fingerprint) };
}

/** The clips found for a show (a podcast show page's address): its own fetch once per session or retry; none for any other address. */
export async function showClips(address: string, tick: number): Promise<Rows> {
  const feed = parseShow(address) ?? '';
  tickOver(tick);
  if (feed === '') return { rows: [] };
  const k = feedKey(feed);
  if (!askedFeeds.has(k)) {
    askedFeeds.add(k);
    const gen = accountGeneration();
    try { const body = (await getJSON(CLIPS_PATH, { feed, limit: 50 })).body; if (gen === accountGeneration()) merge(parseClips(body)); } catch { /* the route is missing or down: nothing, silently */ }
  }
  const mine = [...byID.values()].filter((c) => c.feedURL !== '' && feedKey(c.feedURL) === k).sort((a, b) => (a.clipID < b.clipID ? -1 : a.clipID > b.clipID ? 1 : 0));
  return { rows: mine.slice(0, SHOW_MAX).map(row) };
}

/** "Hear it": the clips cut for the story at a `pub:<route>[/vN]` address (a version has the story's clips), at most three; none for any other page. */
export async function storyClips(address: string, tick: number): Promise<Rows> {
  tickOver(tick);
  if (!address.startsWith('pub:')) { lastStory = ''; return { rows: [] }; }
  if (lastStory === '') asked = false;                               // a story page came to the top from another page: the clips are asked for again (once), so a clip matched since, or a failed ask, shows now
  lastStory = address;                                               // (a swipe from one story to the next asks nothing more)
  const route = address.slice(4).replace(/^\//, '');
  await ensureAll();
  const all = [...byID.values()].sort((a, b) => a.rank - b.rank);   // the server's order: the best quote of a story first
  let found = all.filter((c) => c.route === route);
  if (found.length === 0) { const bare = route.replace(/\/v[1-9][0-9]*$/, ''); found = all.filter((c) => c.route === bare); }
  return { rows: found.slice(0, STORY_MAX).map(row) };
}

async function find(id: string): Promise<Clip | null> {
  if (!byID.has(id)) await ensureAll();
  return byID.get(id) ?? null;
}

const NONE = { op: '', id: '', url: '', title: '', show: '', start: 0, clip: '', feed: '', record: '' };
const ack = (message: string, extra: Partial<Ack> = {}): Ack => ({ status: 'ok', message, play: NONE, open: '', openTitle: '', ...extra });

/** What a clip row's Play, Queue and Open episode do. Play and Queue answer `play.clip`: the page hands the clip's id and row to `radioPlayClip`
 *  (clip-play.ts then makes the entry; the fingerprint locator is not wired: see the head of this file). */
export async function clipAct(op: string, id: string): Promise<Ack> {
  const c = await find(id);
  if (!c) return ack(GONE);
  switch (op) {
    case 'play': return ack('', { play: { ...NONE, op: 'play', clip: c.clipID, feed: c.feedURL, record: JSON.stringify(c.raw) } });
    case 'queue': return ack('Added to the queue.', { play: { ...NONE, op: 'queue', clip: c.clipID, feed: c.feedURL, record: JSON.stringify(c.raw) } });
    case 'open': {
      if (c.feedURL === '') return ack('');
      const guid = c.episodeGUID !== '' ? c.episodeGUID : c.audioURL;           // no guid: the audio address is the id (the feed's own rule)
      return ack('', { open: episodeAddress(c.feedURL, guid, { enclosure: c.audioURL, start: c.start, title: c.episodeTitle, show: c.show }), openTitle: c.episodeTitle === '' ? 'Episode' : c.episodeTitle });
    }
    default: return ack('');
  }
}
