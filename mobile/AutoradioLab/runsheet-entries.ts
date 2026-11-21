// What each thing the listener can play is, as an entry on the run sheet; and the server's bounds, so the phone builds nothing the
// server would drop as invalid. Ported from RunSheetEntries.swift. Pure data.
//
//   a paper story read in full      kind page      source.ref pub:<route>
//   a web page                      kind page      source.ref url:<address>
//   a page the listener heard read  kind page|mail source.ref <the page's own address>; the words are held by the player (the entry has none)
//   a book or a PDF                 kind document  source.ref doc:<id>, source.start the block it is read from
//   a podcast clip                  kind clip      source.ref clip:<id>, source.url the audio, source.start and end its range
//   a podcast episode               kind episode   source.ref episode:<podcast key>, source.url the audio, source.start where it begins
import { Bounds, entryJSONSize, validId, type By, type Ended, type Entry, type Op } from './runsheet-model.ts';

const enc = new TextEncoder();
const bytes = (s: string): number => enc.encode(s).length;

/** At most `n` bytes of `s`, cut at a character boundary. */
export function cutBytes(s: string, n: number): string {
  if (bytes(s) <= n) return s;
  let out = '', used = 0;
  for (const ch of s) { const b = bytes(ch); if (used + b > n) break; out += ch; used += b; }
  return out;
}
export const oneLine = (s: string): string => s.split(/\s+/).filter((x) => x !== '').join(' ');
export const cleanTitle = (s: string): string => cutBytes(oneLine(s), Bounds.maxTitleBytes);

/** `p-` (phone), `l-` (listener) or `d-` plus twenty-four hex characters (96 bits): placer-minted, never reused. */
export function mintId(prefix: string): string {
  const b = new Uint8Array(12);
  crypto.getRandomValues(b);
  return prefix + Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
}
const prefixFor = (by: By): string => (by === 'listener' ? 'l-' : by === 'desk' ? 'd-' : 'p-');
const base = (id: string, kind: Entry['kind'], by: By, title: string, source: Entry['source']): Entry =>
  ({ id, kind, placed_by: by, state: 'ahead', title, source, est_seconds: 0, revision: 0 });

export const paperStory = (route: string, title: string, by: By = 'listener'): Entry =>
  base(mintId(by === 'listener' ? 'l-' : 'p-'), 'page', by, cleanTitle(title), { ref: 'pub:' + route, label: cleanTitle(title) });

/** A story the phone holds the whole text of (the content store): it plays with its own words when the desk cannot be reached. */
export const heldStory = (id: string, title: string, text: string, address: string): Entry =>
  ({ ...base(id, 'story', 'phone', cleanTitle(title), { ref: address, label: cleanTitle(title) }), text, est_seconds: estimateSeconds(text) });

export function webPage(url: string, title: string, by: By = 'listener'): Entry {
  let host = '';
  try { host = new URL(url).host; } catch { /* no host */ }
  const t = cleanTitle(title === '' ? (host || url) : title);
  return base(mintId(by === 'listener' ? 'l-' : 'p-'), 'page', by, t, { ref: 'url:' + url, label: host || t, url });
}
export const book = (id: string, title: string, at: number, by: By = 'listener'): Entry =>
  base(mintId(by === 'listener' ? 'l-' : 'p-'), 'document', by, cleanTitle(title), { ref: 'doc:' + id, label: cleanTitle(title), start: at });

/** A page's words, read aloud on the listener's asking (Listen on a page, "Read it to me" on a message): the address says what it is. */
export const readAloud = (address: string, title: string, by: By = 'listener'): Entry =>
  base(mintId(by === 'listener' ? 'l-' : 'p-'), address.startsWith('mail:') ? 'mail' : 'page', by, cleanTitle(title), { ref: address, label: cleanTitle(title) });

/** An audio file in Files. The entry names the file by its Files path (`file:<path>`), never by where the phone keeps a copy. */
export const filesAudio = (address: string, title: string, by: By = 'listener'): Entry =>
  base(mintId(by === 'listener' ? 'l-' : 'p-'), 'episode', by, cleanTitle(title), { ref: address, label: 'Files' });

export type Audio = { url: string; title: string; show: string; start?: number; end?: number; episode_id?: string };

/** Addresses too long for the server's bound (2048 bytes), kept on the phone by entry id: the entry names the audio by its ref and leaves its url out. */
const longURLs = new Map<string, string>();
/** Sign-out: the long web addresses of the account's entries (keyed by entry id) are forgotten. */
export function forgetLongURLs(): void { longURLs.clear(); }
export const longURL = (id: string): string | undefined => longURLs.get(id);

export function audio(a: Audio, clipID?: string, by: By = 'listener'): Entry {
  const t = cleanTitle(a.title);
  const id = mintId(by === 'listener' ? 'l-' : 'p-');
  const long = bytes(a.url) > Bounds.maxURLBytes;
  if (long) longURLs.set(id, a.url);
  const url = long ? undefined : a.url;
  const strip = <T extends Record<string, unknown>>(o: T): T => { for (const k of Object.keys(o)) if (o[k] === undefined) delete o[k]; return o; };
  if (a.end !== undefined || clipID !== undefined) {
    return base(id, 'clip', by, t, strip({ ref: 'clip:' + (clipID ?? a.episode_id ?? 'x'), label: cleanTitle(a.show), url, start: a.start, end: a.end, clip_id: clipID }));
  }
  return base(id, 'episode', by, t, strip({ ref: 'episode:' + (a.episode_id ?? (long ? 'long' : a.url)), label: cleanTitle(a.show), url, start: a.start }));
}

/** True when the entry's words are read from the phone's own store by its address (a message, a book, a file), so the entry carries none. */
export const wordsComeFromThePhone = (e: Entry): boolean =>
  e.kind === 'mail' || e.kind === 'document' || !!e.source && (e.source.ref.startsWith('mail:') || e.source.ref.startsWith('doc:') || e.source.ref.startsWith('file:'));
export const filesAddress = (e: Entry): string | undefined => ((e.kind === 'episode' || e.kind === 'clip') && e.source?.ref.startsWith('file:') ? e.source.ref : undefined);
/** True when a ref the entry names is over its bound: a ref cannot be cut, so such an entry is not placed (the store says so). */
export const refTooLong = (e: Entry): boolean => bytes(e.source?.ref ?? '') > Bounds.maxURLBytes;
/** The route of a paper story this entry reads in full. */
export const paperRoute = (e: Entry): string | undefined => (e.kind === 'page' && e.source?.ref.startsWith('pub:') ? e.source.ref.slice(4).split('@')[0] : undefined);
export const webURL = (e: Entry): string | undefined => (e.kind === 'page' && e.source?.ref.startsWith('url:') ? e.source.ref.slice(4) : undefined);
export const bookOf = (e: Entry): { id: string; at: number } | undefined => (e.kind === 'document' && e.source?.ref.startsWith('doc:') ? { id: e.source.ref.slice(4), at: Math.trunc(e.source.start ?? 0) } : undefined);

export type AudioSpec = Audio & { is_clip: boolean };
export function audioSpec(e: Entry): AudioSpec | undefined {
  if ((e.kind !== 'clip' && e.kind !== 'episode') || !e.source) return undefined;
  const url = e.source.url ?? longURL(e.id);
  if (!url) return undefined;
  const ep = e.kind === 'episode' && e.source.ref.startsWith('episode:') ? e.source.ref.slice(8) : undefined;
  const o: AudioSpec = { url, title: e.title, show: e.source.label, is_clip: e.source.end !== undefined };
  if (e.source.start !== undefined) o.start = e.source.start;
  if (e.source.end !== undefined) o.end = e.source.end;
  if (ep !== undefined) o.episode_id = ep;
  return o;
}

/** What the Up next row calls it. */
export function kindWord(e: Entry): string {
  switch (e.kind) {
    case 'episode': return 'audio';
    case 'page': return paperRoute(e) !== undefined ? 'story' : 'page';
    case 'document': return 'book';
    default: return e.kind;
  }
}
/** A title always: the real title, else what the entry points at. */
export function displayTitle(e: Entry): string {
  const t = e.title.trim();
  if (t) return t;
  const u = webURL(e);
  if (u) { try { return new URL(u).host; } catch { /* fall through */ } }
  if (e.source?.label) return e.source.label;
  const w = kindWord(e);
  return w.charAt(0).toUpperCase() + w.slice(1);
}
/** Who put it there, as a sentence. */
export const byWords = (e: Entry): string => (e.placed_by === 'listener' ? 'Added by you' : e.placed_by === 'desk' ? 'From the desk' : 'From the phone');
/** A rough time to speak it, from the words (2.6 a second). */
export const WORDS_PER_SECOND = 2.6;
export const estimateSeconds = (text: string): number => Math.max(1, Math.round(text.split(/\s+/).filter((x) => x !== '').length / WORDS_PER_SECOND));

// ---- bounds (the server's, internal/runsheet): the phone builds nothing the server would drop as invalid ----

export function boundedEnded(e: Ended): Ended { return { ...e, at: cutBytes(e.at, Bounds.maxTimeBytes), last_block: Math.max(e.last_block, 0) }; }
const clamp = (v: number, lo: number, hi: number): number => Math.min(Math.max(Number.isFinite(v) ? v : 0, lo), hi);

/** This entry with every string cut at its bound, at a character boundary, every list and number inside its range. A title is cut, never
 *  refused; an id that is too long to cut (an id names a thing) is replaced by a fresh one of the same placer. */
export function boundedEntry(entry: Entry): Entry {
  const B = Bounds;
  const e: Entry = structuredClone(entry);
  e.title = cutBytes(oneLine(e.title), B.maxTitleBytes);
  if (e.text !== undefined) { const c = cutBytes(e.text, B.maxTextBytes); if (c === '') delete e.text; else e.text = c; }
  e.est_seconds = clamp(e.est_seconds, 0, B.maxEstSeconds);
  e.revision = Math.max(e.revision, 0);
  if (e.air_at !== undefined) e.air_at = cutBytes(e.air_at, B.maxTimeBytes);
  if (e.origin !== undefined) e.origin = cutBytes(e.origin, B.maxShortBytes);
  if (e.source) {
    // A ref names the thing (a route, an address): it is never cut (the store refuses an entry whose ref is over its bound). A url is only
    // the way to fetch it: over the bound it is left out and the ref stays.
    const s = e.source;
    s.label = cutBytes(oneLine(s.label), B.maxRefBytes);
    if (s.url !== undefined && bytes(s.url) > B.maxURLBytes) delete s.url;
    if (s.clip_id !== undefined) s.clip_id = cutBytes(s.clip_id, B.maxRefBytes);
    if (s.start !== undefined && s.start < 0) s.start = 0;
    if (s.end !== undefined && s.end < 0) s.end = 0;
  }
  if (e.cites) {
    const kept = e.cites.filter((c) => bytes(c.ref) <= B.maxURLBytes).slice(0, B.maxCites).map((c) => ({ ...c, label: cutBytes(oneLine(c.label), B.maxRefBytes) }));
    if (kept.length) e.cites = kept; else delete e.cites;
  }
  if (e.between && (bytes(e.between.after) > B.maxIDBytes || bytes(e.between.before) > B.maxIDBytes)) delete e.between;   // written for neighbours it cannot name
  if (e.forecast) {
    const f = e.forecast;
    if (f.p_heard !== undefined) f.p_heard = clamp(f.p_heard, 0, 1);
    f.caused_by = cutBytes(f.caused_by, B.maxShortBytes); f.claim_id = cutBytes(f.claim_id, B.maxShortBytes);
  }
  if (e.ended) e.ended = boundedEnded(e.ended);
  if (!validId(e.id)) e.id = mintId(prefixFor(e.placed_by));
  // The 32 KB bound is of the carried form (a quote costs 4, `<` 6): words are taken from the end of the text until the entry fits.
  while (entryJSONSize(e) > B.maxEntryJSON && e.text !== undefined && [...e.text].length > 1) {
    const cs = [...e.text];
    e.text = cutBytes(cs.slice(0, Math.floor((cs.length * 3) / 4)).join(''), B.maxTextBytes);
  }
  return e;
}

/** This op with every field cut or clamped to the server's bounds (its entry too), so the server never drops it as invalid. */
export function boundedOp(op: Op): Op {
  const B = Bounds;
  const o: Op = { ...op };
  if (o.op === 'place' && o.entry) o.entry = boundedEntry(o.entry); else delete o.entry;   // an op carries nothing it does not use
  if (o.op !== 'rewrite') delete o.text; else if (o.text !== undefined) o.text = cutBytes(o.text, B.maxTextBytes);
  if (o.ended) o.ended = boundedEnded(o.ended);
  if (o.reason !== undefined) o.reason = cutBytes(o.reason, B.maxReasonBytes);
  if (o.after !== undefined && bytes(o.after) > B.maxIDBytes) { delete o.after; if (o.at === 'after') o.at = 'next'; }
  if (o.origin !== undefined) o.origin = cutBytes(o.origin, B.maxShortBytes);
  if (o.position_seconds !== undefined) o.position_seconds = clamp(o.position_seconds, 0, B.maxEstSeconds);
  return o;
}
