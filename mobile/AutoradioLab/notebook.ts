// "My notebook" and the learning scorecard, as shared code (station L6 of the learning loop; docs/design/autoradio-learned-document-2026-11-17.md
// sections 2, 6, 7, 8). It replaces the Swift Notebook.swift (NotebookText, EditionText, NotebookCache, NotebookPage, the store's reads and the topics
// write) and the learning route's parse in Choir/Learning.swift (LearningWire, LearningClient.read). The steer queue (more / less / mute taps) is the
// radio's and stays where it is until the run sheet moves (it is not in this file).
//
// The notebook is one Texture document: found by `GET /api/texture/edition` (title "My notebook"), read through the ordinary Texture routes. The line
// grammar is the design's 2.2: `- <text> [L<n> · heard <h> of <m>]` (lesson), `[T<n> ...]` (trying), the listener's own lines untagged.
//
// Ways this can fail (each is a test in notebook.test.ts, the Swift LearningLoopTests F3 to F8 of the same rules):
//  N1 the notebook is read as lines: sections, tags, refs (`lesson:L1`, `try:T2`, `mine:<8 hex>`); odd brackets and non-bullets stay whole or are skipped
//  N2 the topics move in once: a seed when there is no notebook, the topic lines join "What you asked for", the rest untouched, a second write adds nothing
//  N3 the DJ is sent the notebook text, else the topics as text, else nothing, inside 32 KB
//  N4 the learning route: its fixture parses, an empty card and an older shape parse; 404 and 501 are "not yet", a dead network is "offline", no key is "signed out"
//  N5 the page shows what it has and invents nothing: no route is "not available yet" with no numbers; with the route, counts, rate, kinds, and the rows behind a line
//  N6 rule 2: the held list names every kind of record held about the listener and where it is read
//  N7 an unanswered notebook ask is not "no notebook": only an answered `found: false` is none; the page falls back to the last text read
//  N8 no word of the notebook goes to a log

import type { Storage } from './app.contract.d.ts';
import { WORDS } from './words.ts';
import { authedJSON, HTTPError, SignedOutError, putFile, accountGeneration, currentWorld } from './api.ts';
import { textureSource } from './content.ts';
import { cut } from './takes.ts';

// Hoisted: a regex literal with a \p{...} class is compiled at each evaluation by Hermes (the device's engine), and these ran once per character;
// on the simulator that took paperFront 390 ms against a 100 ms turn. Compile once, here.
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u;

// ---------------------------------------------------------------------------------------------------------------- lines

export type Line = { id: string; section: string; text: string; tag: string | null; ref: string | null; heard: number | null; of: number | null };
export type Section = { key: string; heading: string; lines: Line[] };
export type Doc = { title: string; sections: Section[] };
export const TITLE = 'My notebook';
export const ASKED_HEADING = 'What you asked for';

/** Letters and digits only, lower case: "U.S." and "us" are one topic. */
export const topicKey = (s: string): string => [...s.toLowerCase()].filter((c) => LETTER_OR_DIGIT.test(c)).join('');

export function sectionKey(heading: string): string {
  const h = heading.toLowerCase();
  if (h.includes('asked') || h.includes('edition')) return 'asked';
  if (h.includes('learned')) return 'learned';
  if (h.includes('trying')) return 'trying';
  if (h.includes('steering')) return 'steering';
  return 'other-' + topicKey(heading);
}

// A small synchronous SHA-256 (the module cannot await a digest inside a parse).
const K = Uint32Array.from([0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3, 0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2]);
export function sha256(data: Uint8Array): Uint8Array {
  const h = Uint32Array.from([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
  const len = data.length, total = (((len + 9 + 63) >> 6) << 6);
  const m = new Uint8Array(total); m.set(data); m[len] = 0x80;
  const dv = new DataView(m.buffer); dv.setUint32(total - 4, (len * 8) >>> 0); dv.setUint32(total - 8, Math.floor((len * 8) / 4294967296));
  const w = new Uint32Array(64);
  const rotr = (x: number, n: number): number => (x >>> n) | (x << (32 - n));
  for (let o = 0; o < total; o += 64) {
    for (let i = 0; i < 16; i++) w[i] = dv.getUint32(o + i * 4);
    for (let i = 16; i < 64; i++) { const s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3), s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10); w[i] = (w[i - 16] + s0 + w[i - 7] + s1) | 0; }
    let [a, b, c, d, e, f, g, hh] = h;
    for (let i = 0; i < 64; i++) {
      const t1 = (hh + (rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)) + ((e & f) ^ (~e & g)) + K[i] + w[i]) | 0;
      const t2 = ((rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)) + ((a & b) ^ (a & c) ^ (b & c))) | 0;
      hh = g; g = f; f = e; e = (d + t1) | 0; d = c; c = b; b = a; a = (t1 + t2) | 0;
    }
    h[0] += a; h[1] += b; h[2] += c; h[3] += d; h[4] += e; h[5] += f; h[6] += g; h[7] += hh;
  }
  const out = new Uint8Array(32), odv = new DataView(out.buffer);
  h.forEach((x, i) => odv.setUint32(i * 4, x));
  return out;
}
/** The first 8 hex digits of the SHA-256 of the line's trimmed text: the design's `mine:` ref. */
export const mineRef = (text: string): string => 'mine:' + [...sha256(new TextEncoder().encode(text.trim())).slice(0, 4)].map((b) => b.toString(16).padStart(2, '0')).join('');

const TAG = /\s\[([LT])(\d+) · heard (\d+) of (\d+)\]\s*$/;
export function parse(md: string): Doc {
  const doc: Doc = { title: '', sections: [] };
  let cur: Section | null = null;
  for (const raw of md.replace(/\r\n/g, '\n').split('\n')) {
    const line = raw.trim();
    if (line.startsWith('# ') && !line.startsWith('## ')) { if (doc.title === '') doc.title = line.slice(2).trim(); continue; }
    if (line.startsWith('## ')) { if (cur) doc.sections.push(cur); const h = line.slice(3).trim(); cur = { key: sectionKey(h), heading: h, lines: [] }; continue; }
    if (!cur || !(line.startsWith('- ') || line.startsWith('* '))) continue;
    let text = line.slice(2).trim();
    if (text === '') continue;
    const l: Line = { id: `${cur.key}-${cur.lines.length}`, section: cur.key, text, tag: null, ref: null, heard: null, of: null };
    const m = TAG.exec(text);
    if (m) {
      l.tag = m[1] + m[2]; l.ref = (m[1] === 'L' ? 'lesson:' : 'try:') + m[1] + m[2]; l.heard = parseInt(m[3], 10); l.of = parseInt(m[4], 10);
      text = text.slice(0, m.index).trim(); l.text = text;
    } else if (cur.key === 'asked') l.ref = mineRef(text);
    cur.lines.push(l);
  }
  if (cur) doc.sections.push(cur);
  return doc;
}
const lineOf = (d: Doc, id: string): Line | undefined => d.sections.flatMap((s) => s.lines).find((l) => l.id === id);

/** The notebook with the topics among "What you asked for": a seed when there is no notebook, the missing topic lines appended otherwise, the same text when nothing is missing. */
export function merging(topics: string[], into: string | null): string {
  const wanted: string[] = [], seen = new Set<string>();
  for (const t of topics) { const k = topicKey(t); if (k !== '' && !seen.has(k)) { seen.add(k); wanted.push(t.trim()); } }
  if (into === null || into.trim() === '') return `# ${TITLE}\n\n## ${ASKED_HEADING}\n` + wanted.map((t) => `- ${t}\n`).join('');
  const have = new Set(parse(into).sections.filter((s) => s.key === 'asked').flatMap((s) => s.lines).map((l) => topicKey(l.text)));
  const add = wanted.filter((t) => !have.has(topicKey(t)));
  if (add.length === 0) return into;
  const lines = into.replace(/\r\n/g, '\n').split('\n');
  const h = lines.findIndex((l) => l.startsWith('## ') && sectionKey(l.slice(3)) === 'asked');
  if (h >= 0) {
    let end = h + 1, last = h;
    while (end < lines.length && !lines[end].startsWith('## ')) { if (lines[end].startsWith('- ') || lines[end].startsWith('* ')) last = end; end++; }
    lines.splice(last + 1, 0, ...add.map((t) => `- ${t}`));
  } else {
    let at = lines.findIndex((l) => l.startsWith('## '));
    if (at < 0) at = lines.length;
    lines.splice(at, 0, `## ${ASKED_HEADING}`, ...add.map((t) => `- ${t}`), '');
  }
  return lines.join('\n');
}

// ---------------------------------------------------------------------------------------------------------------- what the DJ is sent

export const EDITION_MAX_BYTES = 32 * 1024;
/** The notebook text when the phone holds it, else the topics as text, else null. */
export function editionText(notebook: string | null, topics: string[]): string | null {
  if (notebook !== null && notebook.trim() !== '') return cut(notebook.trim(), EDITION_MAX_BYTES);
  const t = topics.map((x) => x.trim()).filter((x) => x !== '');
  return t.length === 0 ? null : cut('What the listener follows:\n' + t.map((x) => `- ${x}`).join('\n'), EDITION_MAX_BYTES);
}

/** The last notebook text read, kept on the phone for the programme request and for the page when the computer cannot be reached. */
export const CACHE_FILE = 'app:/data/notebook-cache.md';
export async function loadCache(storage: Storage | undefined): Promise<string | null> {
  if (!storage) return null;
  try { return new TextDecoder().decode(await storage.fs.readFile(CACHE_FILE)); } catch { return null; }
}
export async function saveCache(storage: Storage | undefined, text: string, gen: number = accountGeneration()): Promise<void> {
  await putFile(storage, CACHE_FILE, new TextEncoder().encode(text));      // a failed write: the page reads the computer next time; a sign-out in flight writes nothing
}

// ---------------------------------------------------------------------------------------------------------------- the learning route

export type Op = { opID: string; by: string; outcome: string; endedAt: string; lastBlock: number; version: number; positionSeconds: number };
export type Row = {
  claimID: string; stretchID: string; segmentID: string; entryID: string; kind: string; title: string; causedBy: string; forecastAt: string; deadline: string; status: string;
  outcome: string; settledAt: string; noVerdictReason: string; pHeard: number | null; brier: number | null; heardThrough: boolean | null; op: Op | null;
};
export type Group = { label: string; claims: number; settled: number; resolved: number; rate: number | null };
export type Card = {
  forecasts: number; settled: number; heardThrough: number; open: number; overdue: number; noVerdict: number; heardThroughRate: number | null; brier: number | null;
  noVerdictReasons: Record<string, number>; byKind: Group[]; byCause: Group[];
};
export type Read = { card: Card; rows: Row[]; next: string | null; truncated: boolean };
export type Load = { state: 'ready'; read: Read } | { state: 'notyet' | 'offline' } | { state: 'signedout'; reason: string };

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const int = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? Math.trunc(v) : 0);
const dbl = (v: unknown): number | null => (typeof v === 'number' && Number.isFinite(v) ? v : null);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const group = (o: Record<string, unknown>, label: string): Group => ({ label, claims: int(o.claims), settled: int(o.settled), resolved: int(o.resolved), rate: dbl(o.base_rate) });
/** Of the resolved forecasts, how many were heard through (the server's base rate is that share). */
export const heardOf = (g: Group): number => (g.rate === null ? 0 : Math.round(g.rate * g.resolved));

export function parseLearning(body: unknown): Read | null {
  if (!isObj(body)) return null;
  const r: Read = { card: { forecasts: 0, settled: 0, heardThrough: 0, open: 0, overdue: 0, noVerdict: 0, heardThroughRate: null, brier: null, noVerdictReasons: {}, byKind: [], byCause: [] }, rows: [], next: null, truncated: false };
  if (isObj(body.scorecard)) {
    const s = body.scorecard;
    r.card = { forecasts: int(s.forecasts), settled: int(s.settled), heardThrough: int(s.heard_through), open: int(s.open), overdue: int(s.overdue), noVerdict: int(s.no_verdict),
      heardThroughRate: dbl(s.heard_through_rate), brier: dbl(s.brier), noVerdictReasons: {}, byKind: [], byCause: [] };
    if (isObj(s.no_verdict_reasons)) for (const [k, v] of Object.entries(s.no_verdict_reasons)) r.card.noVerdictReasons[k] = int(v);
    r.card.byKind = (Array.isArray(s.by_kind) ? s.by_kind : []).filter(isObj).filter((g) => str(g.project) !== '').map((g) => group(g, str(g.project)));
    r.card.byCause = (Array.isArray(s.by_cause) ? s.by_cause : []).filter(isObj).filter((g) => str(g.cause) !== '').map((g) => group(g, str(g.cause)));
  } else if (body.rows === undefined) return null;
  for (const d of Array.isArray(body.rows) ? body.rows : []) {
    if (!isObj(d) || str(d.claim_id) === '') continue;
    const p = isObj(d.op) ? d.op : null;
    r.rows.push({ claimID: str(d.claim_id), stretchID: str(d.stretch_id), segmentID: str(d.segment_id), entryID: str(d.entry_id), kind: str(d.kind), title: str(d.title), causedBy: str(d.caused_by),
      forecastAt: str(d.forecast_at), deadline: str(d.deadline), status: str(d.status), outcome: str(d.outcome), settledAt: str(d.settled_at), noVerdictReason: str(d.no_verdict_reason),
      pHeard: dbl(d.p_heard), brier: dbl(d.brier), heardThrough: typeof d.heard_through === 'boolean' ? d.heard_through : null,
      op: p ? { opID: str(p.op_id), by: str(p.by), outcome: str(p.outcome), endedAt: str(p.ended_at), lastBlock: int(p.last_block), version: int(p.version), positionSeconds: dbl(p.position_seconds) ?? 0 } : null });
  }
  r.next = str(body.next) === '' ? null : str(body.next); r.truncated = body.truncated === true;
  return r;
}

export type Get = (path: string, query?: Record<string, string | number>) => Promise<unknown>;
export const seamGet: Get = async (path, query) => (await authedJSON(path, { query, world: currentWorld() })).body;

/** The scorecard: ready, "not yet" (404, 501, or a shape this phone does not know), offline, or signed out. Never throws. */
export async function readLearning(get: Get, limit = 200): Promise<Load> {
  try {
    const read = parseLearning(await get('/api/autoradio/learning', { limit: Math.min(Math.max(limit, 1), 200) }));
    return read ? { state: 'ready', read } : { state: 'notyet' };
  } catch (e) {
    if (e instanceof SignedOutError) return { state: 'signedout', reason: e.reason };
    if (e instanceof HTTPError && (e.status === 404 || e.status === 501)) return { state: 'notyet' };
    return { state: 'offline' };
  }
}

// ---------------------------------------------------------------------------------------------------------------- the notebook's document

export type DocLoad = { state: 'found'; address: string; notebook: Doc } | { state: 'none' | 'unavailable' };

/** `{found:false}` is the normal first-run answer; a `found` answer with no document id cannot be opened, so it counts as not found. Null when the server cannot say. */
export async function lookupEdition(get: Get): Promise<{ found: boolean; docID: string } | null> {
  try {
    const o = await get('/api/texture/edition');
    if (!isObj(o)) return null;
    const id = str(o.doc_id);
    return o.found === true && id !== '' ? { found: true, docID: id } : { found: false, docID: '' };
  } catch { return null; }
}

/** The notebook, from the computer, else from the text last read. An unanswered ask is "unavailable", never "no notebook" (N7). */
export async function loadDoc(get: Get, storage: Storage | undefined): Promise<{ load: DocLoad; text: string | null }> {
  const gen = accountGeneration();
  const cached = await loadCache(storage);
  const fallback = (address: string): { load: DocLoad; text: string | null } => (cached !== null ? { load: { state: 'found', address, notebook: parse(cached) }, text: cached } : { load: { state: 'unavailable' }, text: null });
  const ed = await lookupEdition(get);
  if (ed === null) return fallback('');
  if (!ed.found) return { load: { state: 'none' }, text: null };
  const address = `texture:${ed.docID}`;
  let source: string | null = null;
  try { source = await textureSource(ed.docID, get); } catch { source = null; }
  if (source === null) return fallback(address);
  await saveCache(storage, source, gen);
  return { load: { state: 'found', address, notebook: parse(source) }, text: source };
}

// ---------------------------------------------------------------------------------------------------------------- the page

export type LineRow = { id: string; text: string; tag: string; canWhy: boolean; selected: boolean };
export type SectionRow = { key: string; heading: string; lines: LineRow[] };
export type Pair = { label: string; value: string };
export type EvidenceRow = { title: string; outcome: string; detail: string };
export type HeldRow = { what: string; whereRead: string };
export type PageInfo = {
  docState: string; docAddress: string; docNotice: string; sections: SectionRow[]; scoreState: string; scoreNotice: string; score: Pair[]; byKind: Pair[]; noVerdict: string;
  selectedText: string; evidenceNote: string; evidence: EvidenceRow[]; held: HeldRow[]; steerLine: string;
};

export const HELD: HeldRow[] = [
  { what: 'autoradio.turn: what you said to the radio, and what it answered', whereRead: "On your computer's tape. This page does not list turns yet: the server's wider ledger route is not built." },
  { what: 'autoradio.request, autoradio.stretch_start, autoradio.segment, autoradio.stretch: what was written for you', whereRead: "On your computer's tape. The title of each is in the rows behind a notebook line; the full text is not listed yet." },
  { what: 'autoradio.forecast and autoradio.settlement: each guess the DJ made, and how it came out', whereRead: 'The scorecard and the rows behind a line, on this page.' },
  { what: 'autoradio.sheet_op and autoradio.sheet_snapshot: the air log (what played, skipped, cut or left, and where)', whereRead: 'The Heard list on the Autoradio page; the op that settled a forecast is on its row here.' },
  { what: 'autoradio.no_verdict: a forecast that settled nothing because you left or the app could not play it', whereRead: "The scorecard's line about forecasts with no verdict, and the row." },
  { what: 'autoradio.lesson, autoradio.strike and autoradio.notebook_patch: what was learned, struck or written into the notebook', whereRead: 'The notebook lines on this page; tap a line for the forecasts behind it.' },
  { what: 'autoradio.note: your more and less taps, as sent to your computer', whereRead: 'Recent steering in the notebook, once the server writes it; taps waiting to be sent are counted on this page.' },
  { what: 'autoradio.take: takes you recorded', whereRead: 'Takes, on the Autoradio page.' },
  { what: 'Model call traces (the assembled prompt for a stretch)', whereRead: "Held on your computer's trace surfaces, not on this page." },
  { what: 'More and less taps (a record on this phone)', whereRead: "Settings, What I've told the paper. Only on this phone, apart from the copy sent as a steer." },
  { what: 'Topics you follow (edition-topics.json on this phone; the notebook gets them when you finish editing your edition)', whereRead: 'Edit my edition, and "What you asked for" on this page.' },
  { what: 'The run sheet: what is on air, ahead and heard', whereRead: 'The Autoradio page. A copy is on your computer.' },
  { what: 'Muted voices', whereRead: 'Takes, on the Autoradio page. Only on this phone, apart from the steer sent for a mute.' },
  { what: 'Mail', whereRead: 'The Mail page; only on this phone. It is never sent to the DJ or the scorecard.' },
];

const kindLabel = (s: string): string => { const w = s.replace(/_/g, ' '); return w.charAt(0).toUpperCase() + w.slice(1); };
export function outcomeWords(r: Row): string {
  switch (r.status) {
    case 'open': return 'Not yet heard';
    case 'overdue': return 'Not heard in time';
    case 'settled': return r.heardThrough === true ? 'Heard through' : r.op?.outcome === 'cut' ? 'Cut off' : 'Skipped';
    case 'no_verdict': return ({ left: 'You left', failed: 'The app could not play it', late: 'Settled too late' } as Record<string, string>)[r.noVerdictReason] ?? 'No verdict';
    default: return 'Not yet heard';
  }
}
const plural = (n: number, w: string): string => `${n} ${w}${n === 1 ? '' : 's'}`;

export function page(i: { doc: DocLoad | null; learning: Load | null; selected: string; steerQueued: number }): PageInfo {
  const o: PageInfo = { docState: 'loading', docAddress: '', docNotice: '', sections: [], scoreState: 'loading', scoreNotice: '', score: [], byKind: [], noVerdict: '', selectedText: '', evidenceNote: '', evidence: [], held: HELD, steerLine: '' };
  let doc: Doc | null = null;
  if (i.doc?.state === 'found') { o.docState = 'ready'; o.docAddress = i.doc.address; doc = i.doc.notebook; }
  else if (i.doc?.state === 'none') { o.docState = 'none'; o.docNotice = 'You have no notebook yet. It is started when you choose your topics, and Autoradio adds to it as it learns.'; }
  else if (i.doc?.state === 'unavailable') { o.docState = 'offline'; o.docNotice = 'Your notebook could not be reached right now.'; }
  if (doc) o.sections = doc.sections.map((s) => ({ key: s.key, heading: s.heading, lines: s.lines.map((l) => ({ id: l.id, text: l.text, tag: l.heard !== null && l.of !== null ? `heard ${l.heard} of ${l.of}` : '', canWhy: l.ref !== null, selected: l.id === i.selected })) }));
  let read: Read | null = null;
  const L = i.learning;
  if (L === null) o.scoreState = 'loading';
  else if (L.state === 'ready') { o.scoreState = 'ready'; read = L.read; }
  else if (L.state === 'notyet') { o.scoreState = 'notyet'; o.scoreNotice = 'The scorecard is not available yet. Your computer does not have it, so nothing is shown rather than a guess.'; }
  else if (L.state === 'offline') { o.scoreState = 'offline'; o.scoreNotice = 'The scorecard could not be reached. It will show when your computer answers.'; }
  else if (L.state === 'signedout') { o.scoreState = 'signedout'; o.scoreNotice = L.reason === 'locked' ? WORDS.locked : L.reason === 'unknown' ? WORDS.unknown : L.reason === 'refused' ? 'Sign in again to see the scorecard.' : 'Sign in to see the scorecard.'; }
  if (read) {
    const c = read.card;
    if (c.settled === 0) o.score = [{ label: 'Forecasts settled', value: '0' }, { label: 'Still open', value: String(c.open) }];
    else {
      const pct = c.heardThroughRate === null ? '' : ` (${Math.round(c.heardThroughRate * 100)}%)`;
      o.score = [{ label: 'Forecasts settled', value: String(c.settled) }, { label: 'Heard through', value: `${c.heardThrough} of ${c.settled}${pct}` }, { label: 'Still open', value: String(c.open) }];
    }
    o.byKind = c.byKind.map((g) => ({ label: kindLabel(g.label), value: g.resolved === 0 ? 'none settled yet' : `${heardOf(g)} of ${g.resolved} heard through` }));
    if (c.noVerdict > 0) o.noVerdict = `${plural(c.noVerdict, 'forecast')} with no verdict: you left, or the app could not play it.`;
  }
  const line = doc ? lineOf(doc, i.selected) : undefined;
  if (line) {
    o.selectedText = line.text;
    if (line.ref !== null && read) {
      o.evidence = read.rows.filter((r) => r.causedBy === line.ref).map((r) => ({ title: r.title === '' ? 'A segment' : r.title, outcome: outcomeWords(r), detail: r.pHeard === null ? '' : `The desk forecast ${Math.round(r.pHeard * 100)} in 100.` }));
      if (o.evidence.length === 0) o.evidenceNote = 'No forecast has been made from this line yet.';
    } else if (line.ref === null) o.evidenceNote = 'This line is not one the desk cites, so no forecast is listed for it.';
    else o.evidenceNote = 'The forecasts behind a line are not available yet.';
  }
  if (i.steerQueued > 0) o.steerLine = `${plural(i.steerQueued, 'more or less tap')} waiting to reach your computer.`;
  return o;
}

/** The page's own source: asks the computer (the notebook, the scorecard) and answers the page. Never throws. */
export async function notebookPage(a: { selected: string; steerQueued: number }, storage: Storage | undefined, get: Get = seamGet): Promise<PageInfo> {
  const [{ load }, learning] = await Promise.all([loadDoc(get, storage), readLearning(get)]);
  return page({ doc: load, learning, selected: a.selected, steerQueued: a.steerQueued });
}
