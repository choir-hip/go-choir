// Takes, the phone's side, as shared code: the wire (publish, list, mine, delete, record, audio), what a take is, the candidates the programme
// request offers, the muted voices and the listener's own ids, the Takes section's rows and words. It replaces the Swift Takes.swift (TakesClient,
// Take, MyTake, TakeRecordSummary, TakeOwn, TakeMutes, TakeStore, TakeCandidates, Mp4Header, the multipart body) and TakesPage.swift.
// What stays native is the RECORDER (TakeSession.swift: the microphone, the recogniser, the preview player): it is the voice. The session over it is take-session.ts.
//
// THE WIRE (docs/reference/AUTORADIO-TAKES.md; every route is signed in through the listener's own computer):
//   POST   /api/platform/takes                multipart/form-data: `audio` (take.m4a, audio/mp4; an MP4 whose `mvhd` is 3 to 60 s and within 2 s of the
//                                             claim), `transcript` (1 to 2,000 bytes), `duration_seconds`, `sha256`, `responds_to` (JSON, absent when none).
//   GET    /api/platform/takes?limit=50       `{generated_at, takes:[...]}`; GET .../takes?mine=1 the caller's own live takes with `outcomes`.
//   GET    /api/platform/takes/{id}/audio     audio/mp4; 404 once deleted or yanked.      DELETE /api/platform/takes/{id}
//   GET    /api/platform/takes/record?publisher=<handle>   flat: new, offered, heard, skipped, listeners, takes, summary (+ drill, rows).
//   POST   /api/platform/takes/{id}/outcome   NOT the phone's: the listener's computer posts it after settlement.
// A 404 on the takes routes means the server is not deployed yet: "not available yet".
//
// Owner rulings kept: takes "arise implicitly from voice mode" and the Take button stays; "build no anti-fraud; drill data is marked and never counted":
// an outcome's `drill_*` fields are read and kept as their own marks and are never added to what a record says (`counted`).
//
// Ways this can fail (each is a test in takes.test.ts):
//   T1  the 3 s floor, the 60 s ceiling, the 512 KB bound and the 2,000 byte transcript bound are the spec's, and only those
//   T2  a list row without an id, a handle or a live state is not a take; a deleted or yanked take is never a candidate
//   T3  candidates: never the listener's own (by id or handle), never a muted publisher's, at most 3, newest first, kind `take`; they count toward the 40
//   T5  the upload is a multipart body whose audio, transcript and sha256 agree
//   T6  a mute is kept on the phone, listed, undone, and survives a relaunch; the same handle twice is one mute
//   T7  the listener's own record is read by their own handle; an unknown handle reads as "no outcomes yet", not a failure
//   T8  the audio address of a take is followed only when it is a path on the Choir host (the key goes to that host and nowhere else)
//   T9  the page: the control offered matches the recorder's state; Publish only with a transcript; the sentence and the rights line on the review

import type { Storage } from './app.contract.d.ts';
import { authedBytes, authedJSON, HTTPError } from './api.ts';

export const BOUNDS = {
  minSeconds: 3, maxSeconds: 60,
  /** The recorder stops this much before the ceiling: a tick is late and the encoder adds about 0.11 s, and the server refuses a claim above 60. */
  stopEarly: 0.5,
  maxBytes: 512 * 1024, maxTranscriptBytes: 2000, maxCandidates: 3, maxTitleBytes: 120, listLimit: 50, maxItems: 40,
  sampleRate: 24000, bitRate: 32000,
  publishLine: 'Your voice will be offered to other Autoradio listeners. You can delete it at any time.',
  rightsLine: 'A take with music or another person’s recording will be removed.',
};

// ---------------------------------------------------------------------------------------------------------------- text

const STRIPPED = new Set<number>([0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069, 0x200e, 0x200f, 0x061c, 0x200b, 0xfeff]);
const isControl = (c: number): boolean => c < 0x20 || (c >= 0x7f && c < 0xa0);
const utf8 = (s: string): number => new TextEncoder().encode(s).length;

/** A stranger's longer text: bidi and zero-width characters dropped, controls other than a newline become spaces; the lines stay. */
export function stripUnsafe(s: string): string {
  let out = '';
  for (const ch of s) {
    const c = ch.codePointAt(0)!;
    if (c === 10) { out += ch; continue; }
    if (isControl(c)) { out += ' '; continue; }
    if (STRIPPED.has(c)) continue;
    out += ch;
  }
  return out;
}
/** One clean line: controls and newlines become spaces, runs of space collapse, capped in bytes at a character boundary. */
export function oneLine(s: string, maxBytes = 200): string {
  let out = '';
  for (const ch of s) {
    const c = ch.codePointAt(0)!;
    if (isControl(c)) out += ' '; else if (!STRIPPED.has(c)) out += ch;
  }
  return cut(out.replace(/\s+/g, ' ').trim(), maxBytes).trim();
}
/** Cut to at most `bytes` UTF-8 bytes at a character boundary. */
export function cut(s: string, bytes: number): string {
  let used = 0, end = 0;
  for (const ch of s) {
    const c = ch.codePointAt(0)!;
    const n = c < 0x80 ? 1 : c < 0x800 ? 2 : c < 0x10000 ? 3 : 4;
    if (used + n > bytes) return s.slice(0, end);
    used += n; end += ch.length;
  }
  return s;
}

// ---------------------------------------------------------------------------------------------------------------- the file's own length

/** The length the file's own `moov/mvhd` box states: what the server checks the claim against. Null when the file is not a readable MP4. */
export function mp4Seconds(d: Uint8Array): number | null {
  const u32 = (o: number): number | null => (o >= 0 && o + 4 <= d.length ? ((d[o] << 24) | (d[o + 1] << 16) | (d[o + 2] << 8) | d[o + 3]) >>> 0 : null);
  const u64 = (o: number): number | null => { const hi = u32(o), lo = u32(o + 4); return hi === null || lo === null ? null : hi * 4294967296 + lo; };
  const type = (o: number): string => String.fromCharCode(d[o], d[o + 1], d[o + 2], d[o + 3]);
  const find = (t: string, from: number, to: number): { body: number; end: number } | null => {
    let o = from;
    while (o + 8 <= to) {
      let size = u32(o);
      if (size === null) return null;
      let head = 8;
      if (size === 1) { const big = u64(o + 8); if (big === null) return null; size = big; head = 16; } else if (size === 0) size = to - o;
      if (size < head || o + size > to) return null;
      if (type(o + 4) === t) return { body: o + head, end: o + size };
      o += size;
    }
    return null;
  };
  if (d.length < 12 || type(4) !== 'ftyp') return null;
  const moov = find('moov', 0, d.length);
  const mv = moov ? find('mvhd', moov.body, moov.end) : null;
  if (!mv || mv.body >= d.length) return null;
  const v = d[mv.body];
  const scale = v === 1 ? u32(mv.body + 20) : u32(mv.body + 12), dur = v === 1 ? u64(mv.body + 24) : u32(mv.body + 16);
  return scale === null || scale <= 0 || dur === null ? null : dur / scale;
}

// ---------------------------------------------------------------------------------------------------------------- takes

export type Responds = { route: string; title: string };
export type Outcomes = { heard: number; skipped: number; drillHeard: number; drillSkipped: number };
export type Record_ = { offered: number; heard: number; skipped: number; listeners: number; takes: number; isNew: boolean; summary: string };
export type Take = {
  takeID: string; publisher: string; publishedAt: number | null; duration: number; transcript: string; respondsTo: Responds | null; record: Record_ | null; audioPath: string; state: string;
};
export type MyTake = { takeID: string; duration: number; transcript: string; published: boolean; publisher: string; at: number | null; outcomes: Outcomes | null };

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const n = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
export const validID = (s: string): boolean => s !== '' && utf8(s) <= 64 && /^[A-Za-z0-9_-]+$/.test(s);
export const validHandle = (s: string): boolean => s !== '' && utf8(s) <= 64 && ![...s].some((c) => /\s/.test(c) || isControl(c.codePointAt(0)!));
export const audioPath = (id: string): string => `/api/platform/takes/${encodeURIComponent(id)}/audio`;

/** ISO 8601 with any fraction to epoch ms, or null. */
export function isoMs(s: unknown): number | null {
  if (typeof s !== 'string') return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$/.exec(s.trim());
  if (!m) return null;
  let t = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6], m[7] ? parseInt((m[7] + '00').slice(0, 3), 10) : 0);
  if (m[8] !== 'Z') { const sign = m[8][0] === '-' ? -1 : 1; const d = m[8].slice(1).replace(':', ''); t -= sign * (parseInt(d.slice(0, 2), 10) * 60 + parseInt(d.slice(2), 10)) * 60000; }
  return Number.isFinite(t) ? t : null;
}

/** "heard through by 7 of 11 listeners over 5 takes"; "new: no outcomes yet" when nothing has been reported. */
export function recordWords(r: Record_ | null): string {
  const x = r ?? { offered: 0, heard: 0, skipped: 0, listeners: 0, takes: 0, isNew: true, summary: '' };
  if (x.summary !== '') return x.summary;
  if (x.isNew || x.heard + x.skipped === 0) return 'new: no outcomes yet';
  return `heard through by ${x.heard} of ${x.heard + x.skipped} listeners over ${x.takes} take${x.takes === 1 ? '' : 's'}`;
}
export function parseRecord(v: unknown): Record_ | null {
  if (!isObj(v)) return null;
  const r: Record_ = { offered: Math.trunc(n(v.offered)), heard: Math.trunc(n(v.heard)), skipped: Math.trunc(n(v.skipped)), listeners: Math.trunc(n(v.listeners)), takes: Math.trunc(n(v.takes)), isNew: true, summary: '' };
  r.summary = cut(oneLine(typeof v.summary === 'string' ? v.summary : ''), 200);
  r.isNew = typeof v.new === 'boolean' ? v.new : r.heard + r.skipped === 0;
  return r;
}
/** What an outcome count says: the drill marks are kept apart and never added in. */
export const counted = (o: Outcomes): { heard: number; skipped: number } => ({ heard: o.heard, skipped: o.skipped });
export function parseOutcomes(v: unknown): Outcomes | null {
  if (!isObj(v)) return null;
  return { heard: Math.trunc(n(v.heard)), skipped: Math.trunc(n(v.skipped)), drillHeard: Math.trunc(n(v.drill_heard)), drillSkipped: Math.trunc(n(v.drill_skipped)) };
}

/** A take as the public list serves it. Null when the row is not a take the phone can offer (no id, no handle, not live). */
export function parseTake(o: unknown): Take | null {
  if (!isObj(o)) return null;
  const id = o.take_id, who = o.publisher;
  if (typeof id !== 'string' || !validID(id) || typeof who !== 'string' || !validHandle(who)) return null;
  const state = typeof o.state === 'string' ? o.state : 'live';
  if (state !== 'live') return null;
  const d = n(o.duration_seconds);
  return {
    takeID: id, publisher: who, publishedAt: isoMs(o.published_at), duration: Math.max(0, d), transcript: cut(stripUnsafe(typeof o.transcript === 'string' ? o.transcript : ''), BOUNDS.maxTranscriptBytes),
    respondsTo: isObj(o.responds_to) && typeof o.responds_to.route === 'string' && o.responds_to.route !== '' ? { route: o.responds_to.route, title: typeof o.responds_to.title === 'string' ? o.responds_to.title : '' } : null,
    record: parseRecord(o.record), audioPath: pathOf(typeof o.audio_url === 'string' ? o.audio_url : '') ?? audioPath(id), state,
  };
}
/** Only a path on the Choir host is followed. */
export function pathOf(s: string): string | null {
  if (s === '') return null;
  if (s.startsWith('/') && !s.startsWith('//')) return s;
  const m = /^[a-z][a-z0-9+.-]*:\/\/[^/?#]+(\/[^?#]*)?(\?[^#]*)?/i.exec(s);
  return m && m[1] ? m[1] + (m[2] ?? '') : null;
}
export function parseMine(o: unknown): MyTake | null {
  if (!isObj(o) || typeof o.take_id !== 'string' || !validID(o.take_id)) return null;
  const state = typeof o.state === 'string' ? o.state : '';
  if (state === 'deleted' || state === 'yanked') return null;
  return {
    takeID: o.take_id, duration: Math.max(0, n(o.duration_seconds)), transcript: cut(stripUnsafe(typeof o.transcript === 'string' ? o.transcript : ''), BOUNDS.maxTranscriptBytes),
    published: typeof o.published === 'boolean' ? o.published : typeof o.published_at === 'string', publisher: typeof o.publisher === 'string' ? o.publisher : '',
    at: isoMs(o.published_at) ?? isoMs(o.created_at), outcomes: parseOutcomes(o.outcomes),
  };
}
/** The first line of the transcript, cut to 120 bytes. */
export function title(transcript: string): string {
  const first = transcript.split(/\r?\n/).find((l) => l.trim() !== '') ?? transcript;
  return cut(oneLine(first, BOUNDS.maxTitleBytes), BOUNDS.maxTitleBytes);
}
export const takesOf = (json: unknown): Take[] => {
  const rows = isObj(json) && Array.isArray(json.takes) ? json.takes : Array.isArray(json) ? json : [];
  const seen = new Set<string>();
  return rows.map(parseTake).filter((t): t is Take => t !== null && !seen.has(t.takeID) && (seen.add(t.takeID), true));
};
export const mineOf = (json: unknown): MyTake[] => (isObj(json) && Array.isArray(json.takes) ? json.takes : Array.isArray(json) ? json : []).map(parseMine).filter((t): t is MyTake => t !== null);

// ---------------------------------------------------------------------------------------------------------------- what the phone keeps

export type Kept = { ids: string[]; handle: string; muted: string[]; /** The Swift app's own ids, handle and mutes have been carried in (take-session.ts carryOver). */ migrated: boolean };
export const KEPT_FILE = 'app:/data/takes.json';
export const emptyKept = (): Kept => ({ ids: [], handle: '', muted: [], migrated: false });

export async function loadKept(storage: Storage | undefined): Promise<Kept> {
  if (!storage) return emptyKept();
  try {
    const o = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(KEPT_FILE)));
    return { ids: Array.isArray(o.ids) ? o.ids.filter((x: unknown): x is string => typeof x === 'string') : [], handle: typeof o.handle === 'string' ? o.handle : '',
      muted: Array.isArray(o.muted) ? o.muted.filter((x: unknown): x is string => typeof x === 'string' && validHandle(x)) : [], migrated: o.migrated === true };
  } catch { return emptyKept(); }
}
/** True when the file was written (a full disk loses only a convenience, but the carry-over must know). */
export async function saveKept(storage: Storage | undefined, k: Kept): Promise<boolean> {
  if (!storage) return false;
  try { await storage.fs.atomicWriteFile(KEPT_FILE, new TextEncoder().encode(JSON.stringify({ ids: [...k.ids].sort(), handle: k.handle, muted: k.muted, migrated: k.migrated }))); return true; } catch { return false; }
}
/** Sign-out: the ids, the handle and the muted voices are the account's. */
export async function wipeKept(storage: Storage | undefined): Promise<void> { if (!storage) return; try { await storage.fs.rm(KEPT_FILE); } catch { /* absent */ } }

export const owns = (k: Kept, t: { takeID: string; publisher: string }): boolean => k.ids.includes(t.takeID) || (k.handle !== '' && t.publisher === k.handle);
export const addOwn = (k: Kept, id: string, handle: string): Kept => ({ ...k, ids: k.ids.includes(id) ? k.ids : [...k.ids, id], handle: validHandle(handle) ? handle : k.handle });
export const removeOwn = (k: Kept, id: string): Kept => ({ ...k, ids: k.ids.filter((x) => x !== id) });
export const isMuted = (k: Kept, h: string): boolean => k.muted.includes(h);
/** The same handle twice is one mute; a handle that is not one is refused. */
export const mute = (k: Kept, h: string): { kept: Kept; added: boolean } => (!validHandle(h) || k.muted.includes(h) ? { kept: k, added: false } : { kept: { ...k, muted: [...k.muted, h] }, added: true });
export const unmute = (k: Kept, h: string): { kept: Kept; had: boolean } => ({ kept: { ...k, muted: k.muted.filter((x) => x !== h) }, had: k.muted.includes(h) });
/** The notebook's own words for a mute. */
export const muteLine = (h: string): string => `Never play takes by ${h}.`;

// ---------------------------------------------------------------------------------------------------------------- candidates

/** At most 3 takes for the request: live, not the listener's own, not a muted publisher's, newest first. */
export function candidates(takes: Take[], k: Kept, limit = BOUNDS.maxCandidates): Take[] {
  const sorted = takes.map((t, i) => ({ t, i })).sort((a, b) => {
    const x = a.t.publishedAt, y = b.t.publishedAt;
    if (x !== null && y !== null && x !== y) return y - x;
    if (x !== null && y === null) return -1;
    if (x === null && y !== null) return 1;
    return a.i - b.i;
  }).map((e) => e.t);
  return sorted.filter((t) => t.state === 'live' && !owns(k, t) && !isMuted(k, t.publisher) && t.transcript !== '').slice(0, Math.max(0, limit));
}
/** `{kind, take_id, publisher, title, transcript, duration_seconds, responds_to, record}`. The audio address stays on the phone. */
export function candidateJSON(t: Take): Record<string, unknown> {
  const o: Record<string, unknown> = { kind: 'take', take_id: t.takeID, publisher: t.publisher, title: title(t.transcript), transcript: t.transcript, duration_seconds: Math.round(t.duration * 10) / 10, record: recordWords(t.record) };
  if (t.respondsTo) o.responds_to = { route: t.respondsTo.route, title: t.respondsTo.title };
  return o;
}
/** The request's candidates: the stories, then the takes, together at most 40 (a take counts toward the bound). */
export function merged(stories: Record<string, unknown>[], takes: Take[]): Record<string, unknown>[] {
  const ts = takes.slice(0, BOUNDS.maxCandidates).map(candidateJSON);
  return [...stories.slice(0, Math.max(0, BOUNDS.maxItems - ts.length)), ...ts];
}

// ---------------------------------------------------------------------------------------------------------------- the upload

export type Upload = { audio: Uint8Array; transcript: string; duration: number; respondsTo: Responds | null };
export async function sha256Hex(d: Uint8Array): Promise<string> {
  const h = new Uint8Array(await crypto.subtle.digest('SHA-256', d as unknown as BufferSource));
  return [...h].map((b) => b.toString(16).padStart(2, '0')).join('');
}
const enc = new TextEncoder();
/** The multipart body and its content type, as bytes: the audio, the transcript and the hash agree. */
export async function multipart(u: Upload, boundary: string): Promise<{ body: Uint8Array; contentType: string }> {
  const parts: Uint8Array[] = [];
  const field = (name: string, value: string): void => { parts.push(enc.encode(`--${boundary}\r\nContent-Disposition: form-data; name="${name}"\r\n\r\n${value}\r\n`)); };
  field('transcript', u.transcript);
  field('duration_seconds', u.duration.toFixed(2));
  field('sha256', await sha256Hex(u.audio));
  if (u.respondsTo) field('responds_to', JSON.stringify({ route: u.respondsTo.route, title: u.respondsTo.title }));
  parts.push(enc.encode(`--${boundary}\r\nContent-Disposition: form-data; name="audio"; filename="take.m4a"\r\nContent-Type: audio/mp4\r\n\r\n`));
  parts.push(u.audio);
  parts.push(enc.encode(`\r\n--${boundary}--\r\n`));
  const body = new Uint8Array(parts.reduce((a, p) => a + p.length, 0));
  let o = 0;
  for (const p of parts) { body.set(p, o); o += p.length; }
  return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

// ---------------------------------------------------------------------------------------------------------------- the client

/** The one thing the client needs of the network: a signed-in request that answers its status and body, refusals included. */
export type Net = (method: 'GET' | 'POST' | 'DELETE', path: string, o?: { query?: Record<string, string | number>; body?: Uint8Array; contentType?: string }) => Promise<{ status: number; json: unknown; bytes: Uint8Array }>;

/** The production Net over the signed-in seam (api.ts): a refusal arrives as an HTTPError carrying its status and the server's short `error` string.
 * The seam passes a Uint8Array body through as it is (api.ts send), so the multipart upload goes with its own content type. */
export const seamNet: Net = async (method, path, o = {}) => {
  try {
    if (o.body !== undefined || path.endsWith('/audio')) {
      const r = await authedBytes(path, { method, query: o.query, body: o.body, headers: o.contentType ? { 'Content-Type': o.contentType } : undefined });
      let json: unknown = null;
      try { json = JSON.parse(new TextDecoder().decode(r.bytes)); } catch { /* audio is not JSON */ }
      return { status: 200, json, bytes: r.bytes };
    }
    const r = await authedJSON(path, { method, query: o.query });
    return { status: 200, json: r.body, bytes: new Uint8Array() };
  } catch (e) {
    if (e instanceof HTTPError) return { status: e.status, json: { error: e.message.replace(/^(?:HTTP \d+|\d+: rate limited|\d+: the key lacks the scope for \S+|\d+)(?:: )?/, '') }, bytes: new Uint8Array() };
    throw e;
  }
};

export class TakesUnavailable extends Error { constructor() { super('Takes are not available yet.'); } }
export class TakesRefused extends Error {}

export type Client = {
  publish(u: Upload, boundary: string): Promise<Take>;
  list(): Promise<Take[]>;
  mine(): Promise<MyTake[]>;
  remove(id: string): Promise<void>;
  record(publisher: string): Promise<Record_>;
  audio(path: string): Promise<Uint8Array>;
};
const NEW: Record_ = { offered: 0, heard: 0, skipped: 0, listeners: 0, takes: 0, isNew: true, summary: '' };
const ok = (s: number): boolean => s >= 200 && s < 300;
export function client(net: Net): Client {
  const gone = (s: number): void => { if (s === 404) throw new TakesUnavailable(); };
  return {
    async publish(u, boundary) {
      if (u.audio.length > BOUNDS.maxBytes) throw new TakesRefused(`The take is over ${BOUNDS.maxBytes / 1024} KB.`);
      const m = await multipart(u, boundary);
      const r = await net('POST', '/api/platform/takes', { body: m.body, contentType: m.contentType });
      gone(r.status);
      if (!ok(r.status)) {
        const msg = isObj(r.json) && typeof r.json.error === 'string' ? r.json.error : '';
        throw new TakesRefused(msg === '' ? `The server refused the take (HTTP ${r.status}).` : msg.slice(0, 300));
      }
      const t = parseTake(r.json);
      if (!t) throw new Error('not a take');
      return t;
    },
    async list() { const r = await net('GET', '/api/platform/takes', { query: { limit: BOUNDS.listLimit } }); gone(r.status); return takesOf(r.json); },
    async mine() { const r = await net('GET', '/api/platform/takes', { query: { mine: 1 } }); gone(r.status); return mineOf(r.json); },
    async remove(id) { await net('DELETE', `/api/platform/takes/${encodeURIComponent(id)}`); }, // 404 and 409: it is already gone
    async record(publisher) {
      const r = await net('GET', '/api/platform/takes/record', { query: { publisher } });
      if (r.status === 404) return { ...NEW }; // an unknown handle, or no server yet: no outcomes
      const o = isObj(r.json) ? r.json : {};
      return parseRecord(isObj(o.record) ? o.record : o) ?? { ...NEW };
    },
    async audio(path) {
      const p = pathOf(path);
      if (p === null) throw new Error('not a path on the Choir host');
      const r = await net('GET', p);
      if (r.bytes.length === 0 || r.bytes.length > BOUNDS.maxBytes * 2) throw new Error('no audio');
      return r.bytes;
    },
  };
}

// ---------------------------------------------------------------------------------------------------------------- the Takes section

/** What the native recorder reports (the voice's own state: the microphone, the recogniser, the preview). */
export type Recorder = {
  /** idle | recording | transcribing | review | publishing */
  state: string; elapsed: number; transcript: string; previewing: boolean; notice: string; respondsTitle: string; canPublish: boolean;
};
export type MineRow = { id: string; title: string; length: string; state: string };
export type MutedRow = { handle: string; line: string };
export type TakesPageInfo = {
  state: string; elapsed: string; limit: string; fraction: number; canRecord: boolean; canStop: boolean; canPublish: boolean; canDiscard: boolean; previewing: boolean;
  transcript: string; respondsTo: string; publishLine: string; rightsLine: string; notice: string; justPublished: string; mine: MineRow[]; muted: MutedRow[]; record: string; handle: string;
};
export const clock = (s: number): string => { const k = Math.max(0, Math.trunc(s)); return `${Math.floor(k / 60)}:${String(k % 60).padStart(2, '0')}`; };
export function lengthWords(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 1) return '';
  return seconds < 90 ? `${Math.round(seconds)} sec` : `${Math.round(seconds / 60)} min`;
}

export function page(rec: Recorder, o: { mine: MyTake[]; kept: Kept; myRecord: Record_ | null; justPublished: string }): TakesPageInfo {
  const review = rec.state === 'review' || rec.state === 'publishing';
  return {
    state: rec.state, elapsed: clock(rec.elapsed), limit: clock(BOUNDS.maxSeconds),
    fraction: rec.state === 'recording' ? Math.min(1, Math.max(0, rec.elapsed / BOUNDS.maxSeconds)) : 0,
    canRecord: rec.state === 'idle', canStop: rec.state === 'recording', canDiscard: rec.state === 'review', canPublish: rec.canPublish, previewing: rec.previewing,
    transcript: review ? rec.transcript : '', publishLine: review ? BOUNDS.publishLine : '', rightsLine: review ? BOUNDS.rightsLine : '', notice: rec.notice,
    respondsTo: (rec.state === 'recording' || review) && rec.respondsTitle !== '' ? `Answering: ${rec.respondsTitle}` : '',
    justPublished: o.justPublished === '' ? '' : `Published as ${o.justPublished}.`,
    mine: [...o.mine].sort((a, b) => (b.at ?? -Infinity) - (a.at ?? -Infinity)).map((t) => ({ id: t.takeID, title: title(t.transcript) === '' ? 'A take' : title(t.transcript), length: lengthWords(t.duration), state: t.published ? 'Published' : 'On this phone' })),
    muted: o.kept.muted.map((h) => ({ handle: h, line: muteLine(h) })), handle: o.kept.handle, record: o.myRecord && o.kept.handle !== '' ? recordWords(o.myRecord) : '',
  };
}
