// The reader, in shared code for both platforms (it was Swift: TextureReaderSource.swift's ReaderStore and the readerDoc, readerStop, readerRestore, readerReload and readerPrefetch handlers).
// The story or document at an address, as blocks of runs with citations, sources, versions, related stories and the clips cut from it (the parse is reader-parse.ts), and what the page does to it.
//   pub:<route>            GET /api/platform/publications/resolve?route=  (public: no key)  and  .../related?route=&limit=5
//   texture:<doc>[@<rev>]  GET /api/texture/documents/{id}, /api/texture/revisions/{rev}, /api/texture/documents/{id}/revisions   (the listener's key)
// Held in memory for the session (never on disk: nothing here is stored, so nothing here needs a wipe beyond forgetting); keyed on the account generation, so another account never sees a held
// document and an answer that began under an older generation is never held. A story and a revision never change, so they are held; a document's head changes (an edit, the desk's revision),
// so it is asked for again each time and the held copy only stands in when the ask fails. Logs carry counts only, never a title.
import { accountGeneration, authedJSON, getJSON, HTTPError, nativeLater, segment, SignedOutError, type Authed, type Query, type Reply } from './api.ts';
import { lapYield } from './lap.ts';
import { blankDoc, parseAddress, parseBundleAsync, parseRelated, parseRevision, sentenceTexts, splitRoute, type ReaderDoc } from './reader-parse.ts';
import { WORDS } from './words.ts';

export type { ReaderDoc };
export type Ack = { ok: boolean };
export type Net = {
  pub(path: string, query?: Query): Promise<Reply>;
  authed(path: string, opts?: Authed): Promise<Reply>;
};
const real: Net = { pub: (p, q) => getJSON(p, q), authed: (p, o) => authedJSON(p, o) };

/** How many documents the session holds; the least recently used goes first. A story is a few tens of kilobytes decoded. */
export const HELD_LIMIT = 40;
const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');

/** Ends the turn (Exact gives one about 100 ms of CPU; Bun has no scheduler). */
/** One story is decoded at a time. THE CAUSE of `radioWrite` overrunning (the proof's firstrun, bar and search drives, 163-171 ms, with ten stories' lap lines interleaved in the console): the writer loads ten stories in parallel,
 *  and every yield of ten decodings resumes in the SAME turn of the one mutation, so ten stretches of 16 ms made one turn of 160 ms. Exact's budget is per turn, and a turn is every resumption that lands together
 *  (docs/reference/exact/internals.md:40-42). The fetches stay parallel; only the decoding is in line. */
let decoding: Promise<unknown> = Promise.resolve();
const decodeOne = <T>(f: () => Promise<T>): Promise<T> => { const run = decoding.then(f, f); decoding = run.then(() => undefined, () => undefined); return run; };
const yieldTurn = (label = 'reader'): Promise<void> => lapYield(label);
let held = new Map<string, ReaderDoc>();
let inflight = new Map<string, Promise<ReaderDoc>>();
let heldGen = -1;
export function resetReaderForTest(): void { held = new Map(); inflight = new Map(); heldGen = -1; }
export const heldCount = (): number => held.size;
function current(): number {
  const g = accountGeneration();
  if (g !== heldGen) { held = new Map(); inflight = new Map(); heldGen = g; }
  return g;
}
function put(d: ReaderDoc): void {
  held.delete(d.address); held.set(d.address, d);
  while (held.size > HELD_LIMIT) { const oldest = held.keys().next().value as string; if (oldest === d.address) break; held.delete(oldest); }
}

function why(e: unknown): string {
  if (e instanceof HTTPError) return e.message;
  return 'Check the connection and try again.';
}
function failDoc(address: string, e: unknown): ReaderDoc {
  if (e instanceof SignedOutError) {
    const m = e.reason === 'locked' ? WORDS.locked : e.reason === 'unknown' ? WORDS.unknown : e.reason === 'refused' ? 'Sign in again to open your documents.' : 'Sign in to open your documents.';
    return blankDoc(address, 'error', m);
  }
  return blankDoc(address, 'error', `Could not open this page. ${why(e)}`);
}

/** One ask of the server for an address: throws on a failure (the callers say it in words). */
async function fetchDoc(address: string, net: Net): Promise<ReaderDoc> {
  const a = parseAddress(address);
  if (a.kind === 'pub') {
    const bare = splitRoute(a.route).bare;
    const related = net.pub('/api/platform/publications/related', { route: bare, limit: 5 }).then((r) => parseRelated(r.body), () => []);       // a route that is not there: the story still opens
    const body = (await net.pub('/api/platform/publications/resolve', { route: a.route })).body;
    if (!obj(body)) throw new Error('the computer sent a story that is not a story');
    await yieldTurn(`${address}: reply decoded, before the bundle's decoding`);   // the reply's own text and JSON are done: the decoding of a large story starts in a turn of its own
    const d = await decodeOne(() => parseBundleAsync(body, address, (label) => yieldTurn(`${address.slice(-24)}: ${label}`)));
    d.related = await related;                                  // "Hear it" is the page's own source (clips.ts storyClips): a slow clip route does not hold the story
    console.log(`reader: story blocks=${d.blocks.length} cites=${d.cites.length} related=${d.related.length}`);
    return d;
  }
  if (a.kind === 'texture') {
    const docBody = (await net.authed(`/api/texture/documents/${segment(a.doc)}`)).body;
    if (!obj(docBody)) throw new Error('the computer sent a document that is not a document');
    const docObj = obj(docBody.document) ? docBody.document : docBody;
    const rid = a.rev !== '' ? a.rev : str(docObj.current_revision_id);
    const list = net.authed(`/api/texture/documents/${segment(a.doc)}/revisions`).then((r) => (obj(r.body) && Array.isArray(r.body.revisions) ? r.body.revisions.filter(obj) : []), () => []);
    let revObj: Record<string, unknown> = {};
    if (rid !== '') {
      const rb = (await net.authed(`/api/texture/revisions/${segment(rid)}`)).body;
      if (obj(rb)) revObj = obj(rb.revision) ? rb.revision : rb;
    }
    const d = parseRevision(revObj, docObj, address, await list);
    d.docId = a.doc; d.source = str(revObj.content); d.editable = a.rev === '' && d.status === 'ready';
    console.log(`reader: document blocks=${d.blocks.length} versions=${d.versions.length}`);
    return d;
  }
  return blankDoc(address, 'loading');
}

/** The document at an address, fetched now and held nowhere (the callers hold it). A failure is an error document with words. */
export async function loadReader(address: string, net: Net = real): Promise<ReaderDoc> {
  try { return await fetchDoc(address, net); } catch (e) { return failDoc(address, e); }
}

/** A story and a revision never change: the held copy is the answer. A document's head is asked for again. */
const cacheable = (address: string): boolean => { const a = parseAddress(address); return a.kind === 'pub' || (a.kind === 'texture' && a.rev !== ''); };

async function ask(address: string, net: Net): Promise<ReaderDoc> {
  const a = parseAddress(address);
  if (a.kind === 'none') return blankDoc(address, 'loading');
  const gen = current();
  const have = held.get(address);
  if (have && cacheable(address)) { held.delete(address); held.set(address, have); return have; }
  const key = address;
  const shared = inflight.get(key);
  if (shared) return shared;
  const p = (async (): Promise<ReaderDoc> => {
    let d: ReaderDoc;
    try { d = await fetchDoc(address, net); }
    catch (e) {
      if (gen !== accountGeneration()) return blankDoc(address, 'loading');
      return !(e instanceof SignedOutError) && held.get(address) ? held.get(address)! : failDoc(address, e);
    }
    if (gen !== accountGeneration()) return blankDoc(address, 'loading');           // another account began while this was on its way: neither held nor shown
    if (d.status === 'ready') put(d);
    return d;
  })().finally(() => { if (inflight.get(key) === p) inflight.delete(key); });
  inflight.set(key, p);
  return p;
}

// THE VOICE HOLDS THE PAGE IN FRONT. The one native call this file makes, with the reason only native code can meet: the voice turn (Swift today, Kotlin on Android) answers "what is this page
// about" and matches what the radio is saying against the page, and it runs in the native engine, which cannot ask this module. So the page in front, once read, is handed over as its blocks with their
// sentences (`{op: 'pageText'}` on the app module; it holds at most 40 and forgets them at sign-out). Nothing is read back. A host with no native module, a failed call or a stale account is silent.
type HandOver = (request: Record<string, unknown>) => Promise<unknown>;
let handOver: HandOver | null | undefined;
/** Test only: the native hand-over (null: no native module; undefined: the real one). */
export function setHandOverForTest(f: HandOver | null | undefined): void { handOver = f; }
function hand(d: ReaderDoc, gen: number): void {
  const send = handOver === undefined ? (nativeLater() as HandOver | null) : handOver;     // api.ts types the request for the key reads; the module's `later` takes any op
  if (!send || d.status !== 'ready' || gen !== accountGeneration()) return;
  const blocks = d.blocks.filter((b) => b.text !== '').map((b) => ({ id: b.id, kind: b.kind, text: b.text, sentences: sentenceTexts(b) }));
  void Promise.resolve().then(() => send({ op: 'pageText', address: d.address, title: d.title, dek: d.dek, kind: d.kind, blocks })).then((r) => { console.log(obj(r) && r.ok === true ? `reader: handed to the voice blocks=${blocks.length}` : 'reader: the voice did not take the page'); }, () => undefined);
}

/** The source behind the reader page. `accState` and `saved` are only there to ask again: a sign-in from the banner, and the editor's save. */
export async function readerDocShared(address: string, _accState: string, _saved: boolean, net: Net = real): Promise<ReaderDoc> {
  const gen = accountGeneration();
  const d = await ask(address, net);
  hand(d, gen);
  return d;
}

const ack = (ok: boolean): Ack => ({ ok });

/** Forget a document: the next ask reads it again (the page menu's Reload). */
export async function readerReload(address: string): Promise<Ack> { current(); held.delete(address); return ack(true); }

/** A version as the latest (the web's "Restore"): POST /api/texture/documents/{id}/restore. The head is already the latest. */
export async function readerRestore(address: string, version: string, net: Net = real): Promise<Ack> {
  const a = parseAddress(address), r = parseAddress(version);
  current();
  if (a.kind !== 'texture') return ack(false);
  if (r.rev === '') return ack(true);
  try { await net.authed(`/api/texture/documents/${segment(a.doc)}/restore`, { method: 'POST', body: { revision_id: r.rev, mode: 'restore_as_latest' } }); }
  catch { held.delete(address); return ack(false); }
  held.delete(address);
  return ack(true);
}

/** Stop the desk's revision in flight (the web's "Stop"): POST /api/texture/lifecycle/{trajectory}/cancel. One already finished (404, 409) is fine; the document is read again. */
export async function readerStop(address: string, net: Net = real): Promise<Ack> {
  current();
  const traj = held.get(address)?.trajectory ?? '';
  held.delete(address);
  if (traj === '') return ack(true);
  try { await net.authed(`/api/texture/lifecycle/${segment(traj)}/cancel`, { method: 'POST' }); }
  catch (e) { return ack(e instanceof HTTPError && (e.status === 404 || e.status === 409)); }
  return ack(true);
}

/** Fetch the stories either side of the card on top now, so a swipe finds them held. Held ones, documents, other pages and the empty address are skipped; a failure is kept nowhere. */
export async function readerPrefetch(prev: string, next: string, net: Net = real): Promise<Ack> {
  current();
  const seen = new Set<string>();
  const todo = [prev, next].filter((a) => a !== '' && parseAddress(a).kind === 'pub' && !seen.has(a) && (seen.add(a), true) && held.get(a)?.status !== 'ready');
  // One story at a time, a new turn between: two large stories decoded in one turn went over Exact's 100 ms of CPU per turn (the reply was refused: the proof's read drive, 961 KB for two stories).
  for (const a of todo) {
    await ask(a, net);
    await lapYield('prefetch: between two stories');
  }
  return ack(true);
}
