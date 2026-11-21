// The clips pages: a show's "Clips" rows and what a clip row does (Play, Queue, Open episode). Replaces the Swift HearIt.rows(feed:), ClipActions and
// the Joins module. The clip set is the server's public `GET /api/platform/podcast-clips` (a show's own with `?feed=`).
//
// "Clips play from fingerprinted audio" (owner rule). This file decides WHICH clip and whether the row can offer the clip at all (`usable`: the same test
// as the Swift ClipFingerprintRecord.isUsable); the native side, which holds the fingerprint records and the locator, finds where the clip is in the file
// the publisher serves and plays it from there (`clipPlay`, see AutoradioModule+Joins.swift). A clip with no usable fingerprint is offered as its episode
// ("Play episode"); it is never played as a clip.
//
// Ways this can fail:
//  1. A row with no id, no http(s) audio, or an empty or negative range: dropped.        2. A bad body, a 404 or a failure: no rows, silently.
//  3. The same clip twice: held once.         4. A show's feed asked for at most once per session or retry, never on every draw.
//  5. A clip the phone does not have (gone, never fetched): "That clip is not available any more."     6. A clip with no fingerprint: the episode.
//  7. Open episode for a clip with no feed: nothing happens.     8. A show lists at most ten clips, ordered by clip id.

import type { Result } from './app.contract.d.ts';
import { getJSON, sameAccount } from './api.ts';
import { episodeAddress, feedKey, parseShow } from './podcast-model.ts';

export const CLIPS_PATH = '/api/platform/podcast-clips';
export const GONE = 'That clip is not available any more.';
/** The fingerprint's own constants (docs/reference/PODCAST-CLIP-FINGERPRINT.md; Swift ClipFingerprint): version, bands, and 120 frames of head and tail. */
export const FP_VERSION = 1, FP_BANDS = 8, FP_FRAMES = 120;

export type Fingerprint = { version: number; head: string; tail: string; start: number; end: number; bitrateKbps: number; seekable: boolean };
export type Clip = {
  clipID: string; route: string; storyTitle: string; show: string; episodeTitle: string; episodeGUID: string; feedURL: string; audioURL: string;
  start: number; end: number; quote: string; fingerprint: Fingerprint | null;
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
    feedURL: str(o.feed_url), audioURL: audio, start, end, quote: str(o.quote), fingerprint,
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

const byID = new Map<string, Clip>();
const askedFeeds = new Set<string>();
let askedAll = false;
let lastTick = 0;

/** For the sign-out test: how many clips are held in memory. */
export const remembered = (): number => byID.size;

export function forget(): void { byID.clear(); askedFeeds.clear(); askedAll = false; lastTick = 0; }

function merge(clips: Clip[]): void { for (const c of clips) if (!byID.has(c.clipID)) byID.set(c.clipID, c); }

export function row(c: Clip): Rows['rows'][number] {
  return { clipID: c.clipID, show: c.show, episode: c.episodeTitle, length: lengthLabel(c), quote: c.quote, story: c.storyTitle, episodeOnly: !usable(c.fingerprint) };
}

/** The clips found for a show (a podcast show page's address): its own fetch once per session or retry; none for any other address. */
export async function showClips(address: string, tick: number): Promise<Rows> {
  const feed = parseShow(address) ?? '';
  if (tick !== lastTick) { lastTick = tick; askedFeeds.clear(); askedAll = false; }
  if (feed === '') return { rows: [] };
  const k = feedKey(feed);
  if (!askedFeeds.has(k)) {
    askedFeeds.add(k);
    const same = sameAccount();
    try { const c = parseClips((await getJSON(CLIPS_PATH, { feed, limit: 50 })).body); if (same()) merge(c); } catch { /* the route is missing or down: nothing, silently */ }
  }
  const mine = [...byID.values()].filter((c) => c.feedURL !== '' && feedKey(c.feedURL) === k).sort((a, b) => (a.clipID < b.clipID ? -1 : a.clipID > b.clipID ? 1 : 0));
  return { rows: mine.slice(0, 10).map(row) };
}

async function find(id: string): Promise<Clip | null> {
  const have = byID.get(id);
  if (have || askedAll) return have ?? null;
  askedAll = true;
  const same = sameAccount();
  try { const c = parseClips((await getJSON(CLIPS_PATH, { limit: 200 })).body); if (same()) merge(c); } catch { /* silent */ }
  return byID.get(id) ?? null;
}

const NONE = { op: '', id: '', url: '', title: '', show: '', start: 0, clip: '', feed: '' };
const ack = (message: string, extra: Partial<Ack> = {}): Ack => ({ status: 'ok', message, play: NONE, open: '', openTitle: '', ...extra });

/** What a clip row's Play, Queue and Open episode do. Play and Queue answer `play.clip`: the page hands the clip's id to the native `clipPlay`,
 *  which holds the fingerprint and the locator. */
export async function clipAct(op: string, id: string): Promise<Ack> {
  const c = await find(id);
  if (!c) return ack(GONE);
  switch (op) {
    case 'play': return ack('', { play: { ...NONE, op: 'play', clip: c.clipID, feed: c.feedURL } });
    case 'queue': return ack('Added to the queue.', { play: { ...NONE, op: 'queue', clip: c.clipID, feed: c.feedURL } });
    case 'open': {
      if (c.feedURL === '') return ack('');
      const guid = c.episodeGUID !== '' ? c.episodeGUID : c.audioURL;           // no guid: the audio address is the id (the feed's own rule)
      return ack('', { open: episodeAddress(c.feedURL, guid, { enclosure: c.audioURL, start: c.start, title: c.episodeTitle, show: c.show }), openTitle: c.episodeTitle === '' ? 'Episode' : c.episodeTitle });
    }
    default: return ack('');
  }
}
