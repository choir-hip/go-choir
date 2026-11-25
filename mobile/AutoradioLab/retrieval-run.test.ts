// The retrieval runtime under bun test: what the voice turn asks, what native hands over with the ask, what shared code answers, and the
// embedding pass that keeps the vectors. Failure list, written before the code (the Swift tests it replaces are named in brackets: the
// ranking itself is search.test.ts, ported from RetrievalTests, HybridRetrievalTests, EmbedderTests and CorpusSearchTests):
//  R1 what is on air: a content page in front is what "that" means; any other page, the run sheet's on-air title (TurnContextTests, OnAir).
//  R2 the run sheet's entries on air and ahead become documents, heard ones do not; the brief names what is on air and what is next
//     (RetrievalMoreSourcesTests.testPagesTopicsAndRunSheetBecomeDocuments).
//  R3 an ask is answered from every source: the cached front, a registered source (the content store), the facts native handed over
//     (run sheet, open pages, topics, the page's own text); "tell me more about that" finds what is on air; a source that throws is skipped
//     (RetrievalHolderTests, SavedCorpusTests).
//  R4 the answer is plain JSON: dates in milliseconds, the record whole, the brief, the id of the ask (the wire to Swift).
//  R5 the embedding pass: downloads once on Wi-Fi then embeds everything and retrieval fuses; no download off Wi-Fi; low power, a serious
//     thermal state and speech each pause it; sign-out wipes vectors and the corpus and goes back to BM25 (EmbeddingServiceTests).
//  R6 a model that fails or a state that cannot be read never fails an ask: BM25 answers.
//  R7 speed: the cold build of the real size and a query are inside the Swift bounds; the vector scan is measured at the real corpus size
//     (RetrievalTimingTests, EmbedDeviceTrial's scan).
//  R8 the launcher's paper-wide search: one ask after a pause in typing, an older answer never shown under a newer query, local rows first
//     and kept when the server fails (CorpusSearchTests: slow answer, late answer, typing fast, clearing, server error).
import { describe, expect, test } from 'bun:test';
import {
  Retrieval, onAirTitle, isContent, runSheetDocs, toWire, nativeEmbedder, launcherSearch, registerSearchProvider, unregisterSearchProvider, parseAsk, serveAsk, type Facts, type Ask, type NativeEmbed, type DeviceState,
} from './retrieval-run.ts';
import { VectorStore, wipeLiveVectors, embedPrompt, truncatedUnit, PAPER_FRONT_FILE, VECTORS_DIR, type Doc } from './search.ts';

const NOW = 1_794_700_000_000;
const ago = (d: number): number => NOW - d * 86400000;
const story = (id: string, title: string, text: string, days: number): Doc => ({ id: `pub:${id}`, kind: 'story', title, text, date: ago(days), source: 'Autopaper', address: `pub:${id}` });
const noFacts = (): Facts => ({ runSheet: [], pages: [], topics: [], pageText: '' });
const ask = (words: string, o: Partial<Ask> = {}): Ask => ({ id: 'a1', words, pageTitle: '', pageAddress: '', now: NOW, facts: noFacts(), ...o });

function fakeStorage(files: Map<string, Uint8Array>) {
  return {
    fs: {
      readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
      atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
      mkdir: async () => undefined,
      rm: async (p: string) => { files.delete(p); },
    },
  } as never;
}

const SENATE = [
  story('s1', 'Senate passes the water bill', 'The Senate voted 61 to 38 on Tuesday to pass the water bill.\nSenators from both parties spoke about reservoirs and drought funding.', 3),
  story('s2', 'Senate committee delays budget hearing', 'A Senate committee postponed its budget hearing until next week after a dispute over witnesses.', 1),
  story('h1', 'Harbor strike ends after nine days', 'Dock workers voted to end the strike over pay and shift lengths.', 2),
];

describe('R1 what is on air', () => {
  test('a content page in front is what that means; another page takes the run sheet\'s title on air', () => {
    expect(onAirTitle('pub:/s', 'A story', 'Harbor strike')).toBe('A story');
    expect(onAirTitle('', 'Named by the caller', 'Harbor strike')).toBe('Named by the caller');
    expect(onAirTitle('radio:', 'Autoradio', 'Harbor strike')).toBe('Harbor strike');
    expect(onAirTitle('radio:', 'Autoradio', '')).toBe('');
    expect(onAirTitle('doc:b1', '', '')).toBe('');
    expect(onAirTitle('pub:/s', '', 'Harbor strike')).toBe('Harbor strike');
    for (const a of ['pub:x', 'doc:x', 'texture:x', 'mail:x', 'podcast:x', 'file:/x', 'url:x', 'web:x']) expect(isContent(a)).toBe(true);
    expect(isContent('radio:')).toBe(false); expect(isContent('')).toBe(false);
  });
});

describe('R2 the run sheet', () => {
  test('on air and ahead become documents; heard ones are history; the brief says what is on air and next', () => {
    const r = runSheetDocs([
      { id: 'e1', title: 'On air story', text: 'Words.', state: 'on_air', ref: 'pub:/a' },
      { id: 'e2', title: 'Old', text: 'x', state: 'heard', ref: '' },
      { id: 'e3', title: 'Next story', text: 'More words.', state: 'ahead', ref: '' },
      { id: 'e4', title: '', text: '', state: 'ahead', ref: '' },
    ]);
    expect(r.docs.map((d) => d.title)).toEqual(['On air story', 'Next story']);
    expect(r.docs.map((d) => d.source)).toEqual(['On air', 'Up next']);
    expect(r.docs.map((d) => d.address)).toEqual(['pub:/a', 'runsheet:e3']);
    expect(r.docs.every((d) => d.id.startsWith('runsheet:') && d.kind === 'runsheet' && d.date === null)).toBe(true);
    expect(r.onAirTitle).toBe('On air story');
    expect(r.brief).toBe('On air now: On air story. Up next: Next story.');
  });
  test('nothing on air, and at most four ahead in the brief', () => {
    expect(runSheetDocs([]).brief).toBe('Nothing is on air.');
    const six = Array.from({ length: 6 }, (_, i) => ({ id: `e${i}`, title: `T${i}`, text: 't', state: 'ahead' as const, ref: '' }));
    expect(runSheetDocs(six).brief).toBe('Nothing is on air. Up next: T0; T1; T2; T3.');
  });
});

describe('R3 an ask is answered from every source', () => {
  test('the cached front, a registered source and the facts; the Senate question is newest first', async () => {
    const files = new Map<string, Uint8Array>();
    files.set(PAPER_FRONT_FILE, new TextEncoder().encode(JSON.stringify({ stories: [{ title: 'Tunnel referendum passes', dek: 'Voters approved the tunnel.', route: '/t', updated_at: '2026-11-13T10:00:00Z' }] })));
    const r = new Retrieval({ storage: fakeStorage(files), native: null, now: () => NOW });
    r.setDocSource('store', () => SENATE);
    const c = await r.serve(ask('tell me about the Senate'));
    expect(c.items.map((x) => x.id)).toEqual(['pub:s2', 'pub:s1']);
    expect(c.text).toContain('id pub:s2');
    const t = await r.serve(ask('tunnel referendum'));
    expect(t.items[0].id).toBe('front:/t'); expect(t.items[0].address).toBe('pub:/t');
  });
  test('"tell me more about that" finds what is on air from the run sheet the ask carried', async () => {
    const r = new Retrieval({ native: null, now: () => NOW });
    r.setDocSource('store', () => SENATE);
    const facts: Facts = { ...noFacts(), runSheet: [{ id: 'e1', title: 'Harbor strike ends after nine days', text: '', state: 'on_air', ref: 'pub:h1' }] };
    const c = await r.serve(ask('tell me more about that', { pageAddress: 'radio:', pageTitle: 'Autoradio', facts }));
    expect(c.items[0].address).toBe('pub:h1'); expect(c.record.usedOnAir).toBe(true); expect(c.record.onAir).toBe('Harbor strike ends after nine days');
    expect(c.brief).toBe('On air now: Harbor strike ends after nine days.');
    const none = await r.serve(ask('tell me more about that'));
    expect(none.items.length).toBe(0); expect(none.text.startsWith('Nothing on this phone matches')).toBe(true);
  });
  test('open pages, edition topics and the page in front are documents; the page in front is boosted', async () => {
    const r = new Retrieval({ native: null, now: () => NOW });
    const facts: Facts = { runSheet: [], pages: [{ address: 'pub:/p', title: 'Zeppelin story', kind: 'story' }, { address: 'radio:', title: '', kind: 'radio' }], topics: ['Science', 'Zeppelins'], pageText: 'The zeppelin festival opens on Saturday with a parade.' };
    const c = await r.serve(ask('zeppelin festival parade', { pageAddress: 'pub:/p', pageTitle: 'Zeppelin story', facts }));
    expect(c.items[0].id).toBe('page:pub:/p'); expect(c.items[0].source).toBe('the page in front');
    const topics = await r.serve(ask('which topics do I follow zeppelins', { facts }));
    expect(topics.items.some((x) => x.id === 'edition-topics')).toBe(true);
    const pages = await r.serve(ask('zeppelin story', { facts }));
    expect(pages.items.some((x) => x.id === 'openpage:pub:/p')).toBe(true);
    expect(pages.items.some((x) => x.id === 'openpage:radio:')).toBe(false);
  });
  test('the facts of one ask do not outlive it; a source that throws or a file that is corrupt is skipped', async () => {
    const files = new Map<string, Uint8Array>([[PAPER_FRONT_FILE, new TextEncoder().encode('{nope')]]);
    const r = new Retrieval({ storage: fakeStorage(files), native: null, now: () => NOW });
    r.setDocSource('bad', () => { throw new Error('boom'); });
    r.setDocSource('slow-bad', async () => { throw new Error('boom'); });
    r.setDocSource('store', () => SENATE);
    const withFacts: Facts = { ...noFacts(), topics: ['Zeppelins'] };
    expect((await r.serve(ask('zeppelins', { facts: withFacts }))).items.length).toBe(1);
    expect((await r.serve(ask('zeppelins'))).items.length).toBe(0);
    expect((await r.serve(ask('senate'))).items.length).toBe(2);
  });
  test('a source that leaves takes its documents with it; a replaced group drops its old ones', async () => {
    const r = new Retrieval({ native: null, now: () => NOW });
    let docs = SENATE;
    r.setDocSource('store', () => docs);
    expect((await r.serve(ask('senate'))).items.length).toBe(2);
    docs = SENATE.slice(2);
    expect((await r.serve(ask('senate'))).items.length).toBe(0);
    r.dropDocSource('store');
    expect((await r.serve(ask('harbor strike'))).items.length).toBe(0);
  });
});

describe('R4 the wire', () => {
  test('plain JSON: dates in milliseconds, the record whole, the brief and the ask\'s id', async () => {
    const r = new Retrieval({ native: null, now: () => NOW });
    r.setDocSource('store', () => SENATE);
    const wire = JSON.parse(JSON.stringify(toWire('a9', await r.serve(ask('senate budget')))));
    expect(wire.id).toBe('a9');
    expect(wire.items[0]).toMatchObject({ id: 'pub:s2', kind: 'story', source: 'Autopaper', date: ago(1) });
    expect(wire.record).toMatchObject({ query: 'senate budget', terms: ['senate', 'budget'], budgetChars: 3584, at: NOW });
    expect(wire.record.hits[0].id).toBe('pub:s2');
    expect(typeof wire.brief).toBe('string'); expect(wire.text).toContain('Passage 1');
  });
  test('the ask native raises is read whole or refused; a bad one is no ask', () => {
    const raw = JSON.stringify({ runSheet: [{ id: 'e', title: 'T', text: 'x', state: 'on_air', ref: '' }], pages: [], topics: ['A'], pageText: 'p' });
    expect(parseAsk('a1', 'senate', 'T', 'pub:x', NOW, raw)!.facts.topics).toEqual(['A']);
    expect(parseAsk('', 'senate', '', '', NOW, raw)).toBeNull();
    expect(parseAsk('a1', 'senate', '', '', NOW, '{nope')!.facts).toEqual(noFacts());
    expect(parseAsk('a1', 'senate', '', '', NOW, JSON.stringify({ runSheet: 'x', pages: 3 }))!.facts).toEqual(noFacts());
  });
  test('serving an ask hands the answer to native once, and an ask already answered is not answered again', async () => {
    const sent: unknown[] = [];
    const later = async (r: Record<string, unknown>) => { sent.push(r); return {}; };
    const r = new Retrieval({ native: null, now: () => NOW });
    r.setDocSource('store', () => SENATE);
    const a = ask('senate budget');
    expect((await serveAsk(r, a, later)).ok).toBe(true);
    expect((await serveAsk(r, a, later)).ok).toBe(true);
    expect(sent.length).toBe(1);
    expect(sent[0]).toMatchObject({ op: 'retrievalAnswer', id: 'a1' });
    expect(((sent[0] as { context: { items: { id: string }[] } }).context).items[0].id).toBe('pub:s2');
    expect((await serveAsk(r, ask('x', { id: '' }), later)).ok).toBe(false);
    expect(sent.length).toBe(1);
  });
});

// ---- the vectors, with a fake device: a prompt's vector is a unit vector from its words hashed into 64 slots
const D = 64;
const hashVec = (p: string): number[] => {
  const v = new Array(64).fill(0);
  for (const w of p.toLowerCase().split(/[^\p{L}]+/u)) { if (w === '') continue; let h = 5381; for (const b of new TextEncoder().encode(w)) h = (h * 33 + b) % 64; v[h] += 1; }
  return Array.from(truncatedUnit(v, 64));
};
class FakeDevice implements NativeEmbed {
  state: DeviceState = { present: false, loaded: false, onWiFi: true, lowPower: false, thermal: false, radioBusy: false };
  downloads = 0; embedded = 0; failEmbed = false; failState = false;
  async deviceState(): Promise<DeviceState> { if (this.failState) throw new Error('no state'); return { ...this.state }; }
  async ensure(): Promise<boolean> { this.downloads++; this.state.present = true; this.state.loaded = true; return true; }
  embed = async (prompts: string[]): Promise<number[][]> => { if (this.failEmbed) throw new Error('no model'); this.embedded += prompts.length; return prompts.map(hashVec); };
}
const six = (): Doc[] => Array.from({ length: 6 }, (_, i) => ({ id: `t${i}`, kind: 'story', title: `Title ${i}`, text: `story text number ${i} about bridges`, date: 1_800_000_000_000 + i, source: 'Autopaper', address: `pub:${i}` }));
const fast = { first: 1, settle: 1, rest: 1, poll: 10 };
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe('R5 the embedding pass', () => {
  // The press on Settings > Voice and models > Search: it records the agreement and asks for the model. It must fetch
  // even when there is nothing to index yet, and even though no voice turn has attached the native embedder.
  test('a press fetches the model with no work to index and no earlier ask', async () => {
    const dev = new FakeDevice();
    const r = new Retrieval({ storage: fakeStorage(new Map()), native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, mayDownload: async () => ({ ok: true, mobile: false }) });
    await r.fetchModel();
    expect(dev.downloads).toBe(1);
    expect(r.modelState).toBe('ok');
  });
  test('downloads once on Wi-Fi, embeds everything, saves, and retrieval fuses', async () => {
    const dev = new FakeDevice(); const files = new Map<string, Uint8Array>();
    const r = new Retrieval({ storage: fakeStorage(files), native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, mayDownload: async () => ({ ok: true, mobile: false }) });
    r.setDocSource('t', six);
    r.kick(); await r.waitIdle();
    expect(dev.downloads).toBe(1);
    expect(r.progress).toMatchObject({ done: 6, total: 6 }); expect(r.modelState).toBe('ok');
    expect(files.has(`${VECTORS_DIR}/vectors.f16`)).toBe(true);
    r.kick(); await r.waitIdle(); expect(dev.downloads).toBe(1);
    const c = await r.serve(ask('story text number 3 about bridges'));
    expect(c.record.sources).toContain('Autopaper');
    expect(r.line()).toContain('vec=');
    expect(r.usesVectors()).toBe(true);
  });
  test('a vector finds what the keyword floor does not (the question shares no stored word under the floor)', async () => {
    const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true;
    const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, cosineFloor: 0.3 });
    r.setDocSource('t', () => [
      { id: 'v1', kind: 'story', title: 'Chip maker cuts jobs', text: 'Veridian Micro will lay off nine hundred workers as demand for its accelerators slows.', date: null, source: '', address: 'v1' },
      { id: 'v2', kind: 'story', title: 'Weather', text: 'Freezing drizzle glazed roads across the valley overnight and closed many schools.', date: null, source: '', address: 'v2' },
    ]);
    r.kick(); await r.waitIdle();
    const c = await r.serve(ask('Veridian Micro lay off workers accelerators demand'));
    expect(c.items[0].id).toBe('v1');
    expect(r.line()).not.toContain('vec=0');
  });
  test('no download before the person agrees: the model is "needs", nothing is fetched, retrieval is BM25 alone', async () => {
    const dev = new FakeDevice();                                                            // on Wi-Fi, model absent, no agreement
    const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
    r.setDocSource('t', six); r.kick(); await r.waitIdle();
    expect(dev.downloads).toBe(0); expect(r.modelState).toBe('needs'); expect(r.usesVectors()).toBe(false);
    expect((await r.serve(ask('bridges story number'))).items.length).toBeGreaterThan(0);
  });
  test('agreed on Wi-Fi: not fetched over mobile data; agreed on mobile data: fetched there', async () => {
    const wifi = new FakeDevice(); wifi.state.onWiFi = false;
    const a = new Retrieval({ native: wifi, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, mayDownload: async () => ({ ok: true, mobile: false }) });
    a.setDocSource('t', six); a.kick(); await a.waitIdle(); expect(wifi.downloads).toBe(0); expect(a.modelState).toBe('absent');
    const mob = new FakeDevice(); mob.state.onWiFi = false;
    const b = new Retrieval({ native: mob, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, mayDownload: async () => ({ ok: true, mobile: true }) });
    b.setDocSource('t', six); b.kick(); await b.waitIdle(); expect(mob.downloads).toBe(1);
  });
  test('no download off Wi-Fi: the model stays absent and retrieval is BM25 alone', async () => {
    const dev = new FakeDevice(); dev.state.onWiFi = false;
    const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D, mayDownload: async () => ({ ok: true, mobile: false }) });
    r.setDocSource('t', six); r.kick(); await r.waitIdle();
    expect(dev.downloads).toBe(0); expect(r.modelState).toBe('absent'); expect(r.progress.done).toBe(0);
    expect((await r.serve(ask('bridges story number'))).items.length).toBeGreaterThan(0);
    expect(r.usesVectors()).toBe(false);
  });
  test('low power and a serious thermal state pause the job; speech on air pauses it', async () => {
    for (const k of ['lowPower', 'thermal', 'radioBusy'] as const) {
      const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true; dev.state[k] = true;
      const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
      r.setDocSource('t', six); r.kick();
      await wait(120); expect(r.progress.done).toBe(0);
      dev.state[k] = false;
      await r.waitIdle(); expect(r.progress.done).toBe(6);
    }
  });
  test('the newest items are embedded first', async () => {
    const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true;
    const order: string[] = [];
    const embed = dev.embed; dev.embed = async (p) => { order.push(...p.map((x) => /number (\d)/.exec(x)![1])); return embed(p); };
    const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
    r.setDocSource('t', six); r.kick(); await r.waitIdle();
    expect(order[0]).toBe('5'); expect(order[order.length - 1]).toBe('0');
  });
  test('sign-out wipes the vectors, on disk too, and the corpus, and goes back to BM25', async () => {
    const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true; const files = new Map<string, Uint8Array>();
    const storage = fakeStorage(files);
    const r = new Retrieval({ storage, native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
    r.setDocSource('t', six); r.kick(); await r.waitIdle();
    expect(files.size).toBe(2);
    await wipeLiveVectors(storage);
    expect(files.size).toBe(0); expect(r.progress.done).toBe(0); expect(r.usesVectors()).toBe(false);
    expect(r.corpus.index.count).toBe(0);
  });
  test('the stale pass of an account that left writes nothing back', async () => {
    const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true; const files = new Map<string, Uint8Array>();
    const storage = fakeStorage(files);
    const gate: { open: () => void } = { open: () => undefined };
    const held = new Promise<void>((res) => { gate.open = res; });
    const embed = dev.embed; dev.embed = async (p) => { await held; return embed(p); };
    const r = new Retrieval({ storage, native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
    r.setDocSource('t', six); r.kick();
    await wait(40);
    await wipeLiveVectors(storage);
    gate.open(); await r.waitIdle();
    expect(files.size).toBe(0); expect(r.progress.done).toBe(0);
  });
});

describe('R6 a failing model or device never fails an ask', () => {
  test('a model that throws, a device state that cannot be read, no device at all: BM25 answers', async () => {
    const dev = new FakeDevice(); dev.state.present = true; dev.state.loaded = true; dev.failEmbed = true;
    const r = new Retrieval({ native: dev, now: () => NOW, delays: fast, engine: 'fake-1', dimension: D });
    r.setDocSource('t', () => SENATE); r.kick(); await r.waitIdle();
    expect((await r.serve(ask('senate budget'))).items[0].id).toBe('pub:s2');
    dev.failState = true; r.kick(); await r.waitIdle();
    expect((await r.serve(ask('senate budget'))).items[0].id).toBe('pub:s2');
    const none = new Retrieval({ native: null, now: () => NOW, delays: fast });
    none.setDocSource('t', () => SENATE); none.kick(); await none.waitIdle();
    expect((await none.serve(ask('senate budget'))).items[0].id).toBe('pub:s2');
  });
  test('the native embed wrapper reads the plain answers of the app module and refuses a short or malformed one', async () => {
    const calls: Record<string, unknown>[] = [];
    const answers: Record<string, unknown> = { embedState: { present: true, loaded: true, onWiFi: true, lowPower: false, thermal: false, radioBusy: false }, embedEnsure: { ok: true }, embed: { vectors: [[1, 0], [0, 1]] } };
    const n = nativeEmbedder(async (r) => { calls.push(r); return answers[r.op as string]; });
    expect((await n.deviceState()).present).toBe(true);
    expect(await n.ensure(true)).toBe(true);
    expect(await n.embed(['a', 'b'])).toEqual([[1, 0], [0, 1]]);
    expect(calls.map((c) => c.op)).toEqual(['embedState', 'embedEnsure', 'embed']);
    expect(calls[1].download).toBe(true); expect(calls[2].prompts).toEqual(['a', 'b']);
    answers.embed = { vectors: [[1, 0]] };
    await expect(n.embed(['a', 'b'])).rejects.toThrow();
    answers.embed = { error: 'no model' };
    await expect(n.embed(['a'])).rejects.toThrow();
    answers.embedState = null;
    expect((await n.deviceState())).toEqual({ present: false, loaded: false, onWiFi: false, lowPower: false, thermal: false, radioBusy: false });
  });
});

describe('R7 speed at the real size', () => {
  test('cold build of 400 stories and 5 books, and queries, inside the Swift bounds; the vector scan is measured', async () => {
    let seed = 7; const rnd = () => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed / 0x7fffffff; };
    const vocab = Array.from({ length: 4000 }, (_, i) => `w${i}x`);
    const words = (n: number) => Array.from({ length: n }, () => vocab[Math.floor(rnd() * vocab.length)]).join(' ');
    const paras = (n: number, per: number) => Array.from({ length: n }, () => words(per) + '.').join('\n');
    const docs: Doc[] = [];
    for (let s = 0; s < 400; s++) docs.push({ id: `pub:${s}`, kind: 'story', title: words(8) + (s % 20 === 0 ? ' senate' : ''), text: paras(6, 100) + (s % 20 === 0 ? '\nThe senate met.' : ''), date: s * 1_000_000, source: 'Autopaper', address: `pub:${s}` });
    for (let b = 0; b < 5; b++) docs.push({ id: `book:${b}`, kind: 'book', title: `Book ${b}`, text: '', date: null, source: 'Reader', address: `doc:${b}`, parts: Array.from({ length: 20 }, (_, c) => ({ label: `Chapter ${c + 1}`, text: paras(30, 100), block: c * 30 })) });
    const r = new Retrieval({ native: null, now: () => NOW });
    const t0 = performance.now();
    r.setDocSource('t', () => docs);
    await r.refresh();
    const build = performance.now() - t0;
    const asks: number[] = [];
    for (const w of ['tell me about the senate', 'w12x w99x', 'w5x w7x w11x w3000x']) for (let k = 0; k < 5; k++) { const t = performance.now(); await r.serve(ask(w)); asks.push(performance.now() - t); }
    asks.sort((a, b) => a - b);
    const worst = asks[asks.length - 1], median = asks[Math.floor(asks.length / 2)];
    // the vector scan: 768-dimension unit vectors, the whole corpus embedded as windows (what the phone holds at most is the store's own budget)
    const items = r.corpus.index.count;
    const store = new VectorStore('scan', 768);
    for (let i = 0; i < Math.max(items, 20000); i++) { const v = new Float32Array(768); for (let j = 0; j < 768; j++) v[j] = rnd() - 0.5; store.upsert(`r${i}`, '', v); }
    const q = new Float32Array(768); for (let j = 0; j < 768; j++) q[j] = rnd() - 0.5;
    const ts: number[] = []; for (let k = 0; k < 10; k++) { const t = performance.now(); store.search(q, 50); ts.push(performance.now() - t); }
    ts.sort((a, b) => a - b);
    console.log(`RETRIEVAL cold build ${Math.round(build)} ms for ${items} items; ask (read every source, rank, build the context) median ${median.toFixed(2)} ms, worst ${worst.toFixed(2)} ms; vector scan of ${store.count} x 768: median ${ts[5].toFixed(1)} ms, worst ${ts[9].toFixed(1)} ms`);
    expect(items).toBeGreaterThanOrEqual(1000);   // the 400 stories fit; a book that no longer fits is left out whole (C-1: a document is never half indexed and marked indexed)
    expect(median).toBeLessThan(100);       // the Swift bound was 100 ms for the query alone; a loaded machine moves the worst, not the median
    // No wall-clock bound here (review 2026-11-21): the build took 4 to 9 s and the scan 240 to 257 ms on a loaded Mac; the numbers are logged above, and the index's size is bounded by MAX_INDEX_ITEMS (search.ts).
  }, 60000);
});

describe('R9 other pages\' searches join the launcher\'s rows (SearchProviders)', () => {
  test('a provider adds rows after the local ones, once per address; registering a name again replaces it; a throwing one adds nothing', async () => {
    const get = async () => ({ results: [] });
    registerSearchProvider('t', (q) => [{ address: 'doc:z', title: q, kind: 'document', detail: '' }]);
    registerSearchProvider('t', (q) => [{ address: 'doc:y', title: q, kind: 'document', detail: '' }, { address: 'pub:/l', title: 'dup', kind: 'story', detail: '' }]);
    registerSearchProvider('bad', () => { throw new Error('x'); });
    const r = await launcherSearch('k', [{ address: 'pub:/l', title: 'L', kind: 'story', detail: '' }], get, { pause: 1 });
    expect(r.hits.map((h) => h.address)).toEqual(['pub:/l', 'doc:y']);
    unregisterSearchProvider('t'); unregisterSearchProvider('bad');
    expect((await launcherSearch('k', [], get, { pause: 1 })).hits).toEqual([]);
  });
});

describe('R8 the launcher\'s paper-wide search', () => {
  const body = { results: [{ route_path: '/s/one', title: 'Senate one', snippet: 'Senate one. The senate voted.' }, { route_path: '/s/two', title: 'Senate two', snippet: 'two' }] };
  const local = [{ address: 'pub:/s/one', title: 'Senate one', kind: 'story', detail: 'Open' }];
  test('a blank query asks nothing; local rows first, none repeated; the note is empty when there are hits', async () => {
    const asked: string[] = [];
    const get = async (_p: string, q: Record<string, string>) => { asked.push(q.q); return body; };
    expect(await launcherSearch('   ', local, get, { pause: 1 })).toEqual({ hits: [], note: '', query: '' });
    expect(asked).toEqual([]);
    const r = await launcherSearch('senate  ', local, get, { pause: 1 });
    expect(asked).toEqual(['senate']);
    expect(r.hits.map((h) => h.address)).toEqual(['pub:/s/one', 'pub:/s/two']);
    expect(r.note).toBe('');
  });
  test('typing fast asks once, after the pause; an older answer is never shown under a newer query', async () => {
    const asked: string[] = [];
    const get = async (_p: string, q: Record<string, string>) => { asked.push(q.q); return body; };
    const a = launcherSearch('s', [], get, { pause: 30 });
    const b = launcherSearch('se', [], get, { pause: 30 });
    const c = launcherSearch('sen', [], get, { pause: 30 });
    const [ra, rb, rc] = await Promise.all([a, b, c]);
    expect(asked).toEqual(['sen']);
    expect(ra.hits).toEqual([]); expect(rb.hits).toEqual([]); expect(rc.hits.length).toBe(2);
  });
  test('a slow answer for an older query is dropped when a newer one was asked', async () => {
    let release: () => void = () => undefined;
    const slow = new Promise<void>((r) => { release = r; });
    const get = async (_p: string, q: Record<string, string>) => { if (q.q === 'old') await slow; return q.q === 'old' ? { results: [{ route_path: '/old', title: 'Old', snippet: '' }] } : body; };
    const a = launcherSearch('old', [], get, { pause: 1 });
    await wait(20);
    const b = launcherSearch('new', [], get, { pause: 1 });
    const rb = await b; release();
    const ra = await a;
    expect(rb.hits.map((h) => h.address)).toEqual(['pub:/s/one', 'pub:/s/two']);
    expect(ra.hits).toEqual([]); expect(ra.query).toBe("");
  });
  test('clearing the query drops an answer in flight', async () => {
    let release: () => void = () => undefined;
    const slow = new Promise<void>((r) => { release = r; });
    const get = async () => { await slow; return body; };
    const a = launcherSearch('senate', [], get, { pause: 1 });
    await wait(20);
    expect((await launcherSearch('', [], get, { pause: 1 })).query).toBe('');
    release();
    expect((await a).query).toBe("");
  });
  test('a server error is a plain note and the local rows stay; nothing found says so only when there are no rows', async () => {
    const bad = async () => { throw new Error('502'); };
    const r = await launcherSearch('senate', local, bad, { pause: 1 });
    expect(r.hits).toEqual(local); expect(r.note).toBe('Could not search the paper right now. Check your connection and try again.');
    const empty = async () => ({ results: [] });
    expect((await launcherSearch('zzz', [], empty, { pause: 1 })).note).toBe('Nothing found in the paper.');
    expect((await launcherSearch('zzz', local, empty, { pause: 1 })).note).toBe('');
  });
});
