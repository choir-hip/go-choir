// The retrieval runtime: what the voice turn asks, how it is answered, and the embedding pass that keeps the vectors. Shared code; it replaces
// the Swift SavedCorpus, RetrievalSources, SearchProviders, the paper-wide half of CorpusSearch, EmbeddingService and the job and store of
// Embedder.swift. The ranking itself is search.ts (ported unchanged, tested against the Swift tests' own fixtures).
//
// WHY THE VOICE TURN ASKS THROUGH HERE (docs/reference/exact/native-modules.md, "Out (Swift to the UI)"): the turn engine is native (the
// microphone and the speech are) and needs the phone's passages for the words just heard. Exact has no call from native into this module;
// the way back is a source asked again. So native raises ONE ask (the source `retrievalAsk`: an id, the words, the page in front, and the
// facts only native holds: the run sheet's entries on air and ahead, the open pages, the edition topics, the page's own text), this module
// answers it with the whole context as plain JSON through ONE native op (`retrievalAnswer`), and native waits for that with a deadline. The
// ranking, the index and the vectors never leave shared code; native hands facts and reads one answer.
//
// WHAT STAYS NATIVE (each with the reason beside it in Swift): the vector for a text (an on-device model: `embed`), the model's presence and
// download (`embedEnsure`), and the conditions the radio is held by (speech, Low Power Mode, thermal state, Wi-Fi: `embedState`).
//
// The corpus is in memory only and is rebuilt from its sources (an unchanged document is not indexed again). The vectors are a file under
// app:/data/vectors (search.ts), wiped at sign-out with the corpus (search.ts `onSignOut`).
import type { Storage } from './app.contract.d.ts';
import {
  Corpus, VectorStore, retrieve, fusedRetriever, embedQuery, workItems, runJob, coverage, saveVectors, loadVectors, loadPaperFront, editionTopicDocs, openPageDocs,
  onSignOut, searchPaper, corpusRows, corpusNote, normalizeQuery, ENGINE, DIMENSION, MIN_COVERAGE, COSINE_FLOOR, type Doc, type Context, type Embed, type Row,
} from './search.ts';

// ---------------------------------------------------------------------------------------------------------------- the facts native hands over

export type RunSheetFact = { id: string; title: string; text: string; state: 'on_air' | 'ahead'; ref: string };
export type PageFact = { address: string; title: string; kind: string };
export type Facts = { runSheet: RunSheetFact[]; pages: PageFact[]; topics: string[]; pageText: string };
export type Ask = { id: string; words: string; pageTitle: string; pageAddress: string; now: number; facts: Facts };

const CONTENT = ['pub:', 'doc:', 'texture:', 'mail:', 'podcast:', 'file:', 'url:', 'web:'];
/** A page that is a text (a story, book, document, mail, podcast, file): what "that" and "this" mean when it is in front. */
export const isContent = (address: string): boolean => CONTENT.some((p) => address.startsWith(p));
/** What is on air, for "that": the content page in front, else the run sheet's title on air. No address: the caller named what is on air. */
export function onAirTitle(pageAddress: string, pageTitle: string, hint: string): string {
  if ((isContent(pageAddress) || pageAddress === '') && pageTitle !== '') return pageTitle;
  return hint === '' ? (isContent(pageAddress) ? pageTitle : '') : hint;
}

export function runSheetDocs(entries: RunSheetFact[]): { docs: Doc[]; onAirTitle: string; brief: string } {
  let air = '';
  const docs: Doc[] = [];
  for (const e of entries) {
    if ((e.state !== 'on_air' && e.state !== 'ahead') || (e.title === '' && e.text === '')) continue;
    const on = e.state === 'on_air';
    if (on && air === '') air = e.title;
    docs.push({ id: `runsheet:${e.id}`, kind: 'runsheet', title: e.title, text: e.text, date: null, source: on ? 'On air' : 'Up next', address: e.ref !== '' ? e.ref : `runsheet:${e.id}` });
  }
  const ahead = docs.filter((d) => d.source === 'Up next').slice(0, 4).map((d) => d.title);
  const brief = (air === '' ? 'Nothing is on air.' : `On air now: ${air}.`) + (ahead.length === 0 ? '' : ` Up next: ${ahead.join('; ')}.`);
  return { docs, onAirTitle: air, brief };
}

function fnv(s: string): string { let h = 0x811c9dc5; for (const b of new TextEncoder().encode(s)) h = Math.imul(h ^ b, 0x01000193) >>> 0; return h.toString(16); }
const pageDoc = (address: string, title: string, text: string, now: number): Doc =>
  ({ id: `page:${address}`, kind: 'page', title, text, date: now, source: 'the page in front', address, key: `${text.length}|${fnv(text)}` });

const str = (v: unknown): string => (typeof v === 'string' ? v : '');
/** The ask native raised, read whole. An ask without an id is no ask; facts that do not parse are no facts (the question is still answered). */
export function parseAsk(id: string, words: string, pageTitle: string, pageAddress: string, now: number, factsRaw: string): Ask | null {
  if (id === '') return null;
  const facts: Facts = { runSheet: [], pages: [], topics: [], pageText: '' };
  try {
    const o = JSON.parse(factsRaw) as Record<string, unknown>;
    if (Array.isArray(o.runSheet)) for (const e of o.runSheet as Record<string, unknown>[]) if (e && typeof e === 'object' && (e.state === 'on_air' || e.state === 'ahead')) facts.runSheet.push({ id: str(e.id), title: str(e.title), text: str(e.text), state: e.state, ref: str(e.ref) });
    if (Array.isArray(o.pages)) for (const p of o.pages as Record<string, unknown>[]) if (p && typeof p === 'object') facts.pages.push({ address: str(p.address), title: str(p.title), kind: str(p.kind) });
    if (Array.isArray(o.topics)) facts.topics = (o.topics as unknown[]).filter((t): t is string => typeof t === 'string');
    facts.pageText = str(o.pageText);
  } catch { /* no facts */ }
  return { id, words, pageTitle, pageAddress, now, facts };
}

// ---------------------------------------------------------------------------------------------------------------- the device (native)

export type DeviceState = { present: boolean; loaded: boolean; onWiFi: boolean; lowPower: boolean; thermal: boolean; radioBusy: boolean };
export type NativeEmbed = {
  deviceState(): Promise<DeviceState>;
  /** Make the model usable: fetch it first when `download` (only on Wi-Fi: the caller decides), then load it. */
  ensure(download: boolean): Promise<boolean>;
  embed: Embed;
};
type Later = (request: Record<string, unknown>) => Promise<unknown>;
const NO_DEVICE: DeviceState = { present: false, loaded: false, onWiFi: false, lowPower: false, thermal: false, radioBusy: false };
/** The three native ops of the app module (modules/apple/ModelsEntry.swift, AutoradioKit/EmbedDevice.swift). */
export function nativeEmbedder(later: Later): NativeEmbed {
  return {
    async deviceState() {
      const r = (await later({ op: 'embedState' })) as Partial<DeviceState> | null;
      if (!r || typeof r !== 'object') return { ...NO_DEVICE };
      return { present: r.present === true, loaded: r.loaded === true, onWiFi: r.onWiFi === true, lowPower: r.lowPower === true, thermal: r.thermal === true, radioBusy: r.radioBusy === true };
    },
    async ensure(download) { const r = (await later({ op: 'embedEnsure', download })) as { ok?: unknown } | null; return !!r && r.ok === true; },
    async embed(prompts) {
      const r = (await later({ op: 'embed', prompts })) as { vectors?: unknown; error?: unknown } | null;
      if (!r || !Array.isArray(r.vectors) || r.vectors.length !== prompts.length) throw new Error(`embed: ${typeof r?.error === 'string' ? r.error : 'a different number of vectors'}`);
      return r.vectors as number[][];
    },
  };
}

// ---------------------------------------------------------------------------------------------------------------- the runtime

export type Delays = { first: number; settle: number; rest: number; poll?: number };
export const DELAYS: Delays = { first: 15000, settle: 3000, rest: 150 };
export type Options = { storage?: Storage; native?: NativeEmbed | null; now?: () => number; delays?: Delays; engine?: string; dimension?: number; cosineFloor?: number };
export type Served = Context & { brief: string };
const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

export class Retrieval {
  readonly corpus = new Corpus();
  storage: Storage | undefined;
  native: NativeEmbed | null;
  private now: () => number;
  private delays: Delays;
  private engine: string; private dimension: number; private cosineFloor: number;
  private sources = new Map<string, () => Doc[] | Promise<Doc[]>>();
  private store: VectorStore;
  private loaded: Promise<void> | null = null;
  private gen = 0;
  private wanted = false;
  private runnerGen = -1;
  private running = new Set<Promise<void>>();
  private lastLine = '';
  private answered = new Map<string, Promise<{ id: string; ok: boolean }>>();
  modelState: 'ok' | 'downloading' | 'absent' = 'absent';
  private prog = { done: 0, total: 0, rate: 0 };
  private off: () => void;

  constructor(o: Options = {}) {
    this.storage = o.storage; this.native = o.native ?? null; this.now = o.now ?? (() => 0);   // data sources read no clock (Exact refuses Date.now): the rate is 0 until a caller passes one
    ; this.delays = o.delays ?? DELAYS;
    this.engine = o.engine ?? ENGINE; this.dimension = o.dimension ?? DIMENSION; this.cosineFloor = o.cosineFloor ?? COSINE_FLOOR;
    this.store = new VectorStore(this.engine, this.dimension);
    this.off = onSignOut(() => this.reset());
  }
  dispose(): void { this.off(); }

  get progress(): { done: number; total: number; rate: number } { return { ...this.prog }; }
  line(): string { return this.lastLine; }
  usesVectors(): boolean { return this.store.count > 0 && coverage(this.store, this.prog.total) >= MIN_COVERAGE; }

  /** A store of documents (the content store, the reader's books): read before every ask; a document whose key is unchanged is not indexed again. */
  setDocSource(name: string, fn: () => Doc[] | Promise<Doc[]>): void { this.sources.set(name, fn); }
  dropDocSource(name: string): void { this.sources.delete(name); this.corpus.drop(name); }

  /** Sign-out: nothing of the account stays in the index; a running pass is stale and writes nothing more. */
  reset(): void {
    this.gen++; this.wanted = false; this.runnerGen = -1; this.prog = { done: 0, total: 0, rate: 0 }; this.lastLine = '';
    this.corpus.reset(); this.answered.clear();
  }

  private async openVectors(): Promise<void> {
    this.loaded ??= loadVectors(this.storage, this.engine, this.dimension).then((s) => { this.store = s; });
    await this.loaded;
  }

  /** Read every source: the cached front (the phone's own file), then each registered store. A source that throws leaves its old documents. */
  async refresh(): Promise<void> {
    this.corpus.hold('paper-front', await loadPaperFront(this.storage, this.corpus.storyDates));
    for (const [name, fn] of [...this.sources]) {
      try { this.corpus.hold(name, await fn()); } catch { /* a store that cannot be read is skipped this time */ }
    }
  }

  async serve(a: Ask): Promise<Served> {
    await this.openVectors();
    await this.refresh();
    const rs = runSheetDocs(a.facts.runSheet);
    this.corpus.hold('runsheet', rs.docs);
    this.corpus.hold('open-pages', openPageDocs(a.facts.pages));
    this.corpus.hold('edition-topics', editionTopicDocs(a.facts.topics));
    this.corpus.hold('page', a.pageAddress !== '' && a.facts.pageText !== '' ? [pageDoc(a.pageAddress, a.pageTitle, a.facts.pageText, a.now)] : []);
    const q = { words: a.words, onAirTitle: onAirTitle(a.pageAddress, a.pageTitle, rs.onAirTitle), pageID: a.pageAddress === '' ? null : `page:${a.pageAddress}`, now: a.now };
    let queryVector: number[] | null = null;
    if (this.usesVectors() && this.native) queryVector = await embedQuery((p) => this.native!.embed(p), q);
    const r = fusedRetriever(this.corpus.index, this.store, { queryVector, coverage: coverage(this.store, this.prog.total), cosineFloor: this.cosineFloor });
    const ctx = retrieve(r, q);
    this.lastLine = r.line();
    return { ...ctx, brief: rs.brief };
  }

  // ------------------------------------------------------------------------------------------------------------ the embedding pass

  /** Something changed (a store write, Wi-Fi came back): run a pass soon. Passes never overlap; the content lane calls this after a sync. */
  kick(first = false): void {
    this.wanted = true;
    if (this.runnerGen === this.gen) return;
    this.runnerGen = this.gen;
    const g = this.gen;
    const p: Promise<void> = this.loop(g, first).finally(() => { this.running.delete(p); });
    this.running.add(p);
  }
  async waitIdle(): Promise<void> { while (this.running.size > 0) await Promise.all([...this.running]); }

  private async loop(g: number, first: boolean): Promise<void> {
    let delay = first ? this.delays.first : this.delays.settle;
    for (;;) {
      await sleep(delay); delay = this.delays.settle;
      if (g !== this.gen) return;
      if (!this.wanted) { if (this.runnerGen === g) this.runnerGen = -1; return; }
      this.wanted = false;
      await this.pass(g);
    }
  }

  private async pass(g: number): Promise<void> {
    await this.openVectors();
    await this.refresh();
    if (g !== this.gen) return;
    const items = workItems(this.corpus.index);
    this.prog.total = items.length; this.prog.done = Math.min(this.store.count, items.length);
    if (items.length === 0) return;
    const n = this.native;
    if (!n) { this.modelState = 'absent'; return; }
    try {
      const st = await n.deviceState();
      if (!st.loaded) {
        if (!st.present) {
          if (!st.onWiFi) { this.modelState = 'absent'; return; }
          this.modelState = 'downloading';
        }
        if (!(await n.ensure(!st.present))) { this.modelState = 'absent'; return; }
      }
    } catch { this.modelState = 'absent'; return; }
    this.modelState = 'ok';
    const store = this.store, started = this.now(), before = store.count;
    await runJob(store, (p) => n.embed(p), items, {
      busy: async () => { try { const s = await n.deviceState(); return s.radioBusy || s.lowPower || s.thermal; } catch { return true; } },
      save: async () => { if (g === this.gen) await saveVectors(store, this.storage); },
      rest: async () => {
        if (g === this.gen) { this.prog.done = Math.min(store.count, items.length); const dt = (this.now() - started) / 1000; this.prog.rate = dt > 0 ? Math.max(0, store.count - before) / dt : 0; }
        await sleep(this.delays.rest);
      },
      cancelled: () => g !== this.gen,
    });
    if (g !== this.gen) { store.wipe(); store.dirty = false; return; }       // a sign-out came while the last slice was running
    this.prog.done = Math.min(store.count, items.length);
  }

  // ------------------------------------------------------------------------------------------------------------ answering an ask

  /** Answer one ask once: re-reads of the same ask (the source is asked again) share the one answer. */
  answer(a: Ask, later: Later): Promise<{ id: string; ok: boolean }> {
    if (a.id === '') return Promise.resolve({ id: '', ok: false });
    const have = this.answered.get(a.id);
    if (have) return have;
    const p = (async () => {
      try { await later({ op: 'retrievalAnswer', id: a.id, context: toWire(a.id, await this.serve(a)) }); return { id: a.id, ok: true }; }
      catch { this.answered.delete(a.id); return { id: a.id, ok: false }; }
    })();
    this.answered.set(a.id, p);
    if (this.answered.size > 50) this.answered.delete(this.answered.keys().next().value as string);
    return p;
  }
}

export const serveAsk = (r: Retrieval, a: Ask, later: Later): Promise<{ id: string; ok: boolean }> => r.answer(a, later);

/** The answer native reads: plain JSON, dates in milliseconds (the Swift turn's RetrievalContext, AutoradioKit/RetrievalFacts.swift). */
export function toWire(id: string, c: Served): Served & { id: string } { return { id, ...c }; }

// ---------------------------------------------------------------------------------------------------------------- the launcher's paper-wide search

/** Other pages' own searches (the reader's books, podcasts): a name and a function from the query to rows. Registering a name again replaces it. */
const providers = new Map<string, (q: string) => Row[]>();
export function registerSearchProvider(name: string, search: (q: string) => Row[]): void { providers.set(name, search); }
export function unregisterSearchProvider(name: string): void { providers.delete(name); }
/** Local rows, then each provider's rows, none repeated by address; a provider that throws adds nothing. */
export function withProviders(q: string, local: Row[]): Row[] {
  const seen = new Set(local.map((r) => r.address)), out = [...local];
  for (const f of providers.values()) { try { for (const r of f(q)) if (!seen.has(r.address)) { seen.add(r.address); out.push(r); } } catch { /* skipped */ } }
  return out;
}

export type Found = { hits: Row[]; note: string; query: string; stale?: boolean };
let latest = 0;
/**
 * The paper-wide half of the launcher's search (the server's substring search over every published story, public: no key). One ask goes out
 * after a pause in typing; an answer for an older query is dropped (stale: the caller's own rows stay); a failure keeps the local rows and
 * says so; a blank query asks nothing and cancels what is in flight.
 */
export async function launcherSearch(raw: string, local: Row[], get: (path: string, query: Record<string, string>) => Promise<unknown>, o: { pause: number }): Promise<Found> {
  const q = normalizeQuery(raw);
  const mine = ++latest;
  if (q === '') return { hits: [], note: '', query: '' };
  await sleep(o.pause);
  if (mine !== latest) return { hits: local, note: '', query: q, stale: true };
  const status = await searchPaper(q, get);
  if (mine !== latest) return { hits: local, note: '', query: q, stale: true };
  const base = withProviders(q, local);
  const hits = status.state === 'done' ? corpusRows(status.hits, base) : base;
  return { hits, note: corpusNote(status, hits.length > 0), query: q };
}
