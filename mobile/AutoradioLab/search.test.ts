// Search under bun test. Failure list, written before the code (each has a test below; the numbers of the Swift lists in Retrieval.swift
// and HybridRetrieval.swift in parentheses; these tests replace RetrievalTests, HybridRetrievalTests, EmbedderTests and CorpusSearchTests):
//  S1 "tell me about the Senate" returns the Senate stories newest first and nothing else (1); plural and singular meet.
//  S2 a short follow-up uses the title on air; a long question ignores it (2); a question nothing matches says so in one plain sentence (3).
//  S3 one shared word is not relevance for a long question; the floor is half the question's words.
//  S4 a book hit carries its chapter and an address that opens that block (4); at most two passages of one parent (11).
//  S5 the context never exceeds 3,584 characters (5); every id round-trips to an item (6); upsert replaces, remove drops, the same key is a no-op (7).
//  S6 one item per address (8); empty, odd and huge inputs never throw (10); the record names query, terms, ids and scores (12).
//  S7 the snippet is the best window, not the first paragraph.
//  S8 the prompt strings are the model card's; a vector is cut and made unit length; half-precision storage round-trips.
//  S9 the store reopens, an engine or dimension change starts empty, a corrupt file starts empty.
//  S10 the job embeds only what changed, removes what left, saves every N slices and at the end, waits for speech, stops when the model is missing.
//  S11 fusion: vectors only, RRF, vectors with exact terms (capped at half), no vectors falls back to BM25 (1, 3).
//  S12 the fused retriever: BM25 alone without vectors or with partial coverage, a vector finds what BM25 does not, a rare name is kept, windows
//      collapse to the item, an unrelated question returns nothing (1 to 4).
//  S13 the paper's cached front (app:/data/paper-front.json, written by paper.ts) is searchable again: the source "paper-front" the first Exact move dropped.
//  S14 the paper-wide route: real shape decoded, unopenable and repeated hits skipped, a body that is not JSON is `failed` and the local rows stay, the
//      query is public and encoded, the note says what is true.
//  S15 mail, files, topics and open pages become documents; a replaced group drops its old documents.
import { describe, expect, test } from 'bun:test';
import {
  Index, Corpus, VectorStore, BUDGET_CHARS, retrieve, bm25Retriever, tokens, stem, passages, queryTerms, embedPrompt, truncatedUnit, toHalf, fromHalf, day,
  fuse, rrf, fusedRetriever, itemID, workItems, todo, runJob, contentKey, loadVectors, saveVectors, wipeVectors, coverage, embedQuery, paperFrontDocs,
  loadPaperFront, PAPER_FRONT_FILE, decodeCorpus, corpusRows, corpusNote, searchPaper, normalizeQuery, CORPUS_PATH, fileNameDocs, editionTopicDocs,
  openPageDocs, ENGINE, DIMENSION, VECTORS_DIR, type Doc, type Query, type Embed, type Fusion,
} from './search.ts';
import { getJSON } from './api.ts';
import corpusBody from '../fixtures/search/corpus-search.json';
import cachedFront from '../fixtures/search/paper-front-cached.json';

const NOW = 1_794_700_000_000; // 2026-11-14T23:06 UTC
const ago = (days: number): number => NOW - days * 86400000;
const story = (id: string, title: string, text: string, days: number): Doc => ({ id: `pub:${id}`, kind: 'story', title, text, date: ago(days), source: 'Autopaper', address: `pub:${id}` });
const q = (words: string, air = '', page: string | null = null): Query => ({ words, onAirTitle: air, pageID: page, now: NOW });

function fixture(): Index {
  const i = new Index();
  i.upsert(story('s1', 'Senate passes the water bill', 'The Senate voted 61 to 38 on Tuesday to pass the water bill.\nSenators from both parties spoke about reservoirs and drought funding.', 3));
  i.upsert(story('s2', 'Senate committee delays budget hearing', 'A Senate committee postponed its budget hearing until next week after a dispute over witnesses.', 1));
  i.upsert(story('s3', 'Senators trade charges over the filibuster', 'Two senators traded charges on the floor about the filibuster rules.', 9));
  i.upsert(story('h1', 'Harbor strike ends after nine days', 'Dock workers voted to end the strike over pay and shift lengths.', 2));
  i.upsert(story('t1', 'Tide barrier approved', 'The council approved a tide barrier for the harbor.', 5));
  i.upsert({
    id: 'book:b1', kind: 'book', title: 'The Keeper of Lamps', text: '', date: ago(30), source: 'Reader', address: 'doc:b1',
    parts: [
      { label: 'Chapter 1', text: 'The keeper walked the shore at dusk and counted the boats. '.repeat(40), block: 3 },
      { label: 'Chapter 2', text: 'A storm came in over the cliffs and the lamp burned all night. '.repeat(40), block: 90 },
    ],
  });
  return i;
}
const ask = (i: Index, w: string, air = '') => retrieve(bm25Retriever(i), q(w, air));

describe('S1 to S7 the lexical side', () => {
  test('S1 the Senate question returns the Senate stories, newest first, and nothing else', () => {
    const c = ask(fixture(), 'tell me about the Senate');
    expect(c.items.map((x) => x.id)).toEqual(['pub:s2', 'pub:s1']);
    expect(c.text).not.toContain('Harbor');
    expect(c.text).toContain('id pub:s2');
    expect(c.text).toContain('Today is 2026-11-14.');
  });
  test('S1 plural and singular meet', () => {
    expect(ask(fixture(), 'what did the senators say').items.map((x) => x.id)).toContain('pub:s3');
    expect(stem('senators')).toBe(stem('senator'));
  });
  test('S2 a short follow-up uses the title on air; without one there is nothing', () => {
    const i = fixture();
    const c = ask(i, 'tell me more about that', 'Harbor strike ends after nine days');
    expect(c.items[0].id).toBe('pub:h1');
    expect(c.record.usedOnAir).toBe(true);
    expect(ask(i, 'tell me more about that').items.length).toBe(0);
  });
  test('S2 a long specific question ignores the title on air', () => {
    const c = ask(fixture(), 'senate budget hearing witnesses dispute', 'Harbor strike ends after nine days');
    expect(c.record.usedOnAir).toBe(false);
    expect(c.items[0].id).toBe('pub:s2');
  });
  test('S2 no match says so plainly', () => {
    const c = ask(fixture(), 'what is the weather on Mars');
    expect(c.items.length).toBe(0);
    expect(c.text.startsWith('Nothing on this phone matches')).toBe(true);
    expect(c.text).toContain('do not answer from memory');
  });
  test('S3 one shared word is not relevance for a long question', () => {
    expect(ask(fixture(), 'filibuster reform in the Canadian parliament').items.length).toBe(0);
  });
  test('S4 a book passage cites its chapter and opens its block, at most two of one parent', () => {
    const c = ask(fixture(), 'storm over the cliffs lamp');
    const it = c.items[0];
    expect(it.kind).toBe('book');
    expect(it.place.startsWith('Chapter 2')).toBe(true);
    expect(it.address).toBe('doc:b1@90');
    expect(c.text).toContain('Chapter 2');
    expect(c.items.length).toBeLessThanOrEqual(2);
  });
  test('S5 the budget is never exceeded; newest first', () => {
    const i = new Index();
    const long = Array.from({ length: 60 }, (_, n) => `Paragraph ${n} says the Senate and the Senate again about taxes and roads and schools in some detail.`).join('\n');
    for (let n = 0; n < 30; n++) i.upsert(story(`l${n}`, `Senate long story number ${n} ` + 'word '.repeat(40), long, n));
    const c = ask(i, 'Senate taxes');
    expect(c.text.length).toBeLessThanOrEqual(BUDGET_CHARS);
    expect(c.record.budgetChars).toBe(3584);
    expect(c.items.length).toBeGreaterThan(2);
    const dates = c.items.map((x) => x.date!);
    expect(dates).toEqual([...dates].sort((a, b) => b - a));
  });
  test('S5 ids round-trip to addresses; upsert replaces, remove drops, an unchanged key is a no-op', () => {
    const i = fixture();
    for (const it of ask(i, 'senate').items) { expect(it.address.startsWith('pub:')).toBe(true); expect(i.contains(it.id)).toBe(true); }
    expect(i.upsert(story('s2', 'Senate committee delays budget hearing', 'A Senate committee postponed its budget hearing until next week after a dispute over witnesses.', 1))).toBe(false);
    expect(i.upsert(story('s2', 'Senate committee delays budget hearing', 'Now about glaciers.', 1))).toBe(true);
    expect(ask(i, 'glaciers').items[0].id).toBe('pub:s2');
    expect(ask(i, 'senate').items.some((x) => x.id === 'pub:s2' && x.text.includes('postponed'))).toBe(false);
    i.remove('pub:s2');
    expect(ask(i, 'glaciers').items.some((x) => x.id === 'pub:s2')).toBe(false);
    const n = i.count; i.remove('pub:s2'); expect(i.count).toBe(n);
  });
  test('S6 one item per address', () => {
    const i = new Index();
    i.upsert({ id: 'front:x', kind: 'headline', title: 'Senate vote', text: 'Senate vote today.', date: ago(1), source: '', address: 'pub:x' });
    i.upsert({ id: 'pub:x', kind: 'story', title: 'Senate vote', text: 'Senate vote today in full with more words.', date: ago(1), source: '', address: 'pub:x' });
    expect(ask(i, 'senate vote').items.length).toBe(1);
  });
  test('S6 empty and odd inputs never throw; non-Latin words are indexed', () => {
    expect(ask(new Index(), 'senate').items.length).toBe(0);
    const f = fixture();
    for (const w of ['', '   ', 'the of and', '日本の上院', '😀😀', 'ünïcödé senate']) ask(f, w);
    f.upsert(story('u', 'Zürich opens a café', 'Der Zürich café öffnet.', 1));
    expect(ask(f, 'zürich café').items[0].id).toBe('pub:u');
    f.upsert(story('j', '上院が予算を可決', '日本の上院は火曜日に予算を可決した。', 1));
    expect(ask(f, '上院が予算を可決').items.length).toBeGreaterThan(0); // no word breaker: a run of kana and kanji is one word, as it was in Swift
    const huge = 'word '.repeat(200000);
    f.upsert(story('big', 'A huge document', huge, 1));
  });
  test('S6 the record names the query, terms, ids and scores', () => {
    const r = ask(fixture(), 'senate budget').record;
    expect(r.query).toBe('senate budget'); expect(r.terms).toEqual(['senate', 'budget']);
    expect(r.hits[0].id).toBe('pub:s2'); expect(r.hits[0].score).toBeGreaterThan(0);
    expect(r.sources).toEqual(['Autopaper', 'Reader']);
  });
  test('S7 the snippet is the best window, not the first paragraph', () => {
    const i = new Index();
    const text = Array.from({ length: 30 }, (_, n) => `Filler paragraph ${n} about nothing in particular at all and so on and on.`).join('\n') + '\nThe zeppelin festival opens Saturday.';
    i.upsert(story('z', 'Weekend', text, 1));
    expect(ask(i, 'zeppelin festival').items[0].text).toContain('zeppelin');
  });
  test('tokens: stop words, stems, digits, passages', () => {
    expect(tokens('Tell me about the Senate votes 61 to 38')).toEqual(['senate', 'vote', '61', '38']);
    expect(queryTerms(q('tell me more about that', 'Harbor strike')).usedOnAir).toBe(true);
    expect(passages(('word '.repeat(100) + '\n').repeat(6)).length).toBeGreaterThan(1);
    expect(day(NOW)).toBe('2026-11-14'); expect(day(null)).toBe('undated');
  });
  test('a page in front is boosted modestly', () => {
    const i = fixture();
    const a = i.candidates(q('senate'), 5).find((x) => x.id === 'pub:s1')!.score;
    const b = i.candidates(q('senate', '', 'pub:s1'), 5).find((x) => x.id === 'pub:s1')!.score;
    expect(b / a).toBeCloseTo(1.25, 6);
  });
});

// ---- the vectors, with a fake model: a prompt's vector is a unit vector from its words hashed into 64 slots (the Swift tests' own hash)
const hashVec = (p: string): number[] => {
  const v = new Array(64).fill(0);
  for (const w of p.toLowerCase().split(/[^\p{L}]+/u)) { if (w === '') continue; let h = 5381; for (const b of new TextEncoder().encode(w)) h = (h * 33 + b) % 64; v[h] += 1; }
  return Array.from(truncatedUnit(v, 64));
};
const fakeEmbed: Embed = async (prompts) => prompts.map(hashVec);
const D = 64;

describe('S8 the prompts and the vectors', () => {
  test('S8 the prompt strings are the model card\'s (docs/reference/EMBEDDINGGEMMA.md section 3)', () => {
    expect(embedPrompt.document(null, 'x')).toBe('title: none | text: x');
    expect(embedPrompt.document('  ', 'x')).toBe('title: none | text: x');
    expect(embedPrompt.document('A', 'x')).toBe('title: A | text: x');
    expect(embedPrompt.query('why')).toBe('task: search result | query: why');
    expect(ENGINE).toBe('embeddinggemma-2-q8_0@llama.cpp-b12259'); expect(DIMENSION).toBe(768);
  });
  test('S8 a vector is cut to its first values and made unit length', () => {
    const v = truncatedUnit([3, 4, 12, 0], 2);
    expect(v[0]).toBeCloseTo(0.6, 6); expect(v[1]).toBeCloseTo(0.8, 6);
    const raw = Array.from({ length: 768 }, (_, i) => Math.sin(i) * 500); // the model's vectors are not unit length (norms 259 to 1394)
    const u = truncatedUnit(raw, 768);
    expect(Math.hypot(...u)).toBeCloseTo(1, 5);
  });
  test('S8 half precision round-trips within its resolution', () => {
    for (const x of [0, 0.5, -0.25, 0.0123, 0.9999, -0.7071]) expect(fromHalf(toHalf(x))).toBeCloseTo(x, 3);
  });
});

const items = ['alpha', 'bravo', 'charlie', 'delta', 'echo', 'foxtrot', 'golf', 'hotel', 'india', 'juliet'].map((w, n) => ({ id: `d${n}`, key: `k${n}`, title: 't', text: `story about bridges and senate votes ${w}` }));

describe('S9 and S10 the store and the job', () => {
  test('S10 embeds only what changed, survives a reopen, finds the passage whose words the query shares', async () => {
    const files = new Map<string, Uint8Array>();
    const storage = fakeStorage(files);
    const s1 = new VectorStore('fake-1', D);
    const p1 = await runJob(s1, fakeEmbed, items, { save: () => saveVectors(s1, storage) });
    expect(p1.embedded).toBe(10); expect(p1.finished).toBe(true);
    const s2 = await loadVectors(storage, 'fake-1', D);
    expect(s2.count).toBe(10);
    const changed = items.map((x) => ({ ...x })); changed[3].key = 'k3-new'; changed.pop();
    const p2 = await runJob(s2, fakeEmbed, changed);
    expect(p2.embedded).toBe(1); expect(p2.skipped).toBe(8); expect(p2.removed).toBe(1);
    expect(s2.count).toBe(9);
    expect(s2.search(hashVec(embedPrompt.query('delta')), 3)[0].id).toBe('d3');
  });
  test('S9 an engine or dimension change starts empty; a corrupt file starts empty; missing rows are dropped', async () => {
    const files = new Map<string, Uint8Array>();
    const storage = fakeStorage(files);
    const s = new VectorStore('a', D);
    await runJob(s, fakeEmbed, items, { save: () => saveVectors(s, storage) });
    expect((await loadVectors(storage, 'b', D)).count).toBe(0);
    expect((await loadVectors(storage, 'a', 8)).count).toBe(0);
    expect((await loadVectors(storage, 'a', D)).count).toBe(10);
    files.set(`${VECTORS_DIR}/rows.json`, new TextEncoder().encode('{not json'));
    expect((await loadVectors(storage, 'a', D)).count).toBe(0);
    const { vectors, rows } = s.serialize();
    expect(VectorStore.parse(vectors.slice(0, 2 * D * 4), rows, 'a', D).count).toBe(4); // a kill between the two files
  });
  test('S10 a cancelled job resumes; it saves every N slices and at the end', async () => {
    const s = new VectorStore('fake-1', D);
    let n = 0;
    const p = await runJob(s, fakeEmbed, items, { cancelled: () => n++ >= 1, budget: { passagesPerSlice: 4, savesEverySlices: 40, maxChars: 1800 } });
    expect(p.finished).toBe(false); expect(s.count).toBe(4);
    const q2 = await runJob(s, fakeEmbed, items);
    expect(q2.embedded).toBe(6); expect(q2.skipped).toBe(4); expect(s.count).toBe(10);
    let saves = 0;
    const s3 = new VectorStore('fake-1', D);
    await runJob(s3, fakeEmbed, items, { save: async () => { saves++; }, budget: { passagesPerSlice: 1, savesEverySlices: 3, maxChars: 1800 } });
    expect(saves).toBe(3 + 1); // after slices 3, 6 and 9, and at the end
  });
  test('S10 speech wins: the job waits while the radio needs the processor', async () => {
    const s = new VectorStore('fake-1', D);
    let busy = 3;
    const p = await runJob(s, fakeEmbed, items, { busy: () => busy-- > 0 });
    expect(p.waitedForSpeech).toBe(3); expect(p.embedded).toBe(10);
  });
  test('S10 a missing model stops the job and says how much is left', async () => {
    const s = new VectorStore('fake-1', D);
    const p = await runJob(s, async () => { throw new Error('no model'); }, items);
    expect(p.failed).toBe(10); expect(p.finished).toBe(false); expect(s.count).toBe(0);
  });
  test('S10 the work list is newest first, windows a long item, and is derived (rows without a vector)', () => {
    const i = new Index();
    i.upsert(story('old', 'Old story', 'bridge crews', 9)); i.upsert(story('new', 'New story', 'bridge crews', 1));
    i.upsert({ id: 'long', kind: 'story', title: 'A long story', text: 'bridge crews steel deck river '.repeat(200), date: ago(5), source: '', address: 'pub:long' });
    const w = workItems(i);
    expect(w.map((x) => x.id).slice(0, 2)).toEqual(['pub:new', 'long~w1']);
    expect(w.some((x) => x.id === 'long~w2')).toBe(true);
    const s = new VectorStore('fake-1', D);
    s.upsert('pub:old', w.find((x) => x.id === 'pub:old')!.key, hashVec('x'));
    expect(todo(w, s).todo.some((x) => x.id === 'pub:old')).toBe(false);
    expect(contentKey('a', 'b')).toBe(contentKey('a', 'b')); expect(contentKey('a', 'b')).not.toBe(contentKey('a', 'c'));
  });
  test('sign-out wipes the vectors in memory and on disk', async () => {
    const files = new Map<string, Uint8Array>();
    const storage = fakeStorage(files);
    const s = new VectorStore('fake-1', D);
    await runJob(s, fakeEmbed, items, { save: () => saveVectors(s, storage) });
    expect(files.size).toBe(2);
    await wipeVectors(s, storage);
    expect(s.count).toBe(0); expect(files.size).toBe(0);
  });
});

describe('S11 fusion', () => {
  const vec = Array.from({ length: 30 }, (_, i) => `v${i + 1}`);
  test('vectors only keeps the vector order', () => { expect(fuse('vectorsOnly', vec, ['b1', 'v3'], ['b1'], 5)).toEqual(['v1', 'v2', 'v3', 'v4', 'v5']); });
  test('RRF rewards agreement over either list', () => {
    const out = fuse('rrf', ['a', 'b', 'c'], ['c', 'd', 'a'], [], 10);
    expect(out[0]).toBe('a'); expect(new Set(out)).toEqual(new Set(['a', 'b', 'c', 'd']));
    expect(rrf([['a'], ['a']])).toEqual(['a']);
  });
  test('exact-term hits are kept even when the vectors do not rank them', () => {
    const out = fuse('vectorsWithExactTerms', vec, ['name1', 'v2'], ['name1'], 20);
    expect(out.length).toBe(20); expect(out).toContain('name1'); expect(out.slice(0, 5)).toEqual(['v1', 'v2', 'v3', 'v4', 'v5']);
  });
  test('an exact hit already high stays where the vectors put it', () => {
    const out = fuse('vectorsWithExactTerms', vec, ['v2'], ['v2'], 20);
    expect(out.slice(0, 3)).toEqual(['v1', 'v2', 'v3']); expect(out.filter((x) => x === 'v2').length).toBe(1);
  });
  test('exact hits are capped at half the list', () => {
    const many = Array.from({ length: 30 }, (_, i) => `e${i + 1}`);
    const out = fuse('vectorsWithExactTerms', vec, many, many, 20);
    expect(out.length).toBe(20); expect(out.filter((x) => x.startsWith('e')).length).toBe(10);
  });
  test('no vectors falls back to BM25 for every strategy', () => {
    for (const s of ['vectorsOnly', 'rrf', 'vectorsWithExactTerms'] as Fusion[]) expect(fuse(s, [], ['a', 'b'], ['a'], 5)).toEqual(['a', 'b']);
  });
  test('a window id stands for its item', () => { expect(itemID('long~w3')).toBe('long'); expect(itemID('long')).toBe('long'); expect(itemID('a~wx')).toBe('a~wx'); });
});

describe('S12 the fused retriever', () => {
  const docs: Doc[] = [
    { id: 's1', kind: 'story', title: 'Harlow viaduct closes', text: 'The viaduct over the river will close to cars for eleven months while crews replace the deck.', date: null, source: '', address: 's1' },
    { id: 's2', kind: 'story', title: 'Senate votes', text: 'The Senate passed the harbor bill sixty one to thirty eight on Tuesday night.', date: null, source: '', address: 's2' },
    { id: 's3', kind: 'story', title: 'Chip maker cuts jobs', text: 'Veridian Micro will lay off nine hundred workers as demand for its accelerators slows.', date: null, source: '', address: 's3' },
    { id: 's4', kind: 'story', title: 'Weather', text: 'Freezing drizzle glazed roads across the valley overnight and closed many schools.', date: null, source: '', address: 's4' },
  ];
  async function make(list: Doc[], embedded = true) {
    const idx = new Index(); idx.upsertAll(list);
    const store = new VectorStore('fake-1', D);
    if (embedded) for (const it of idx.snapshot()) store.upsert(it.id, 'k', hashVec(embedPrompt.document(it.title, it.text)));
    return { idx, store };
  }
  const qv = async (w: string) => embedQuery(fakeEmbed, { words: w, now: NOW });
  test('without vectors it is BM25 alone and the line says no vectors were used', async () => {
    const { idx, store } = await make(docs, false);
    const r = fusedRetriever(idx, store, { queryVector: await qv('viaduct closes'), coverage: 0 });
    expect(r.candidates(q('viaduct closes'), 10).map((x) => x.id)).toEqual(idx.candidates(q('viaduct closes'), 10).map((x) => x.id));
    expect(r.line()).toContain('vec=0');
  });
  test('vectors find an item the keyword floor does not', async () => {
    const { idx, store } = await make(docs);
    const w = 'Veridian Micro lay off workers accelerators demand';
    const r = fusedRetriever(idx, store, { queryVector: await qv(w), coverage: 1, cosineFloor: 0.3 });
    expect(r.candidates(q(w), 5)[0].id).toBe('s3');
    expect(r.line().startsWith('AUTORADIO retrieve retrieved=')).toBe(true);
  });
  test('a rare name keeps its passage even when the vectors rank it low', async () => {
    const { idx, store } = await make([...docs, { id: 's5', kind: 'story', title: 'Local news', text: 'Councilwoman Quillfeather spoke about parking at the meeting.', date: null, source: '', address: 's5' }]);
    const w = 'viaduct river deck cars crews Quillfeather';
    const r = fusedRetriever(idx, store, { queryVector: await qv(w), coverage: 1, cosineFloor: 0.3 });
    expect(r.candidates(q(w), 20).map((x) => x.id)).toContain('s5');
  });
  test('windows of a long item collapse to the item', async () => {
    const idx = new Index();
    idx.upsert({ id: 'long', kind: 'story', title: 'A long story', text: 'bridge crews steel deck river '.repeat(200), date: null, source: '', address: 'long' });
    const store = new VectorStore('fake-1', D);
    const w = workItems(idx);
    expect(w.length).toBeGreaterThan(1);
    for (const it of w) store.upsert(it.id, it.key, hashVec(embedPrompt.document(it.title, it.text)));
    const r = fusedRetriever(idx, store, { queryVector: await qv('bridge steel deck'), coverage: 1, cosineFloor: 0.3 });
    expect(r.candidates(q('bridge steel deck'), 10).map((x) => x.id)).toEqual(['long']);
  });
  test('an unrelated question returns nothing; partial coverage stays BM25', async () => {
    const { idx, store } = await make(docs);
    const r = fusedRetriever(idx, store, { queryVector: await qv('zebra quartz marmalade'), coverage: 1, cosineFloor: 0.3 });
    expect(r.candidates(q('zebra quartz marmalade'), 10)).toEqual([]);
    const partial = fusedRetriever(idx, store, { queryVector: await qv('viaduct closes'), coverage: 0.3 });
    expect(partial.candidates(q('viaduct closes'), 10).map((x) => x.id)).toEqual(idx.candidates(q('viaduct closes'), 10).map((x) => x.id));
    expect(coverage(store, 4)).toBe(1); expect(coverage(new VectorStore('x', D), 0)).toBe(0);
  });
  test('a model that fails leaves the query vector null and the retriever BM25', async () => {
    const bad = await embedQuery(async () => { throw new Error('no model'); }, { words: 'x', now: NOW });
    expect(bad).toBeNull();
    const { idx, store } = await make(docs);
    const r = fusedRetriever(idx, store, { queryVector: null, coverage: 1 });
    expect(r.candidates(q('viaduct closes'), 10).length).toBeGreaterThan(0);
    expect(retrieve(r, q('viaduct closes')).items.length).toBeGreaterThan(0);
  });
});

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

describe('S13 the paper\'s cached front is searchable again', () => {
  test('the cached list becomes documents with the old source\'s id, address, kind and date', () => {
    const dates = new Map<string, number>();
    const docs = paperFrontDocs(cachedFront, dates);
    expect(docs.map((d) => d.id)).toEqual(['front:/pub/texture/tide-barrier-pub1', 'front:/pub/texture/glass-hall-pub2']);
    expect(docs[0]).toMatchObject({ kind: 'story', source: 'Autopaper', address: 'pub:/pub/texture/tide-barrier-pub1', text: 'The council voted seven to two to fund the barrier.' });
    expect(docs[0].date).toBe(Date.parse('2026-11-18T12:16:30.512Z'));
    expect(dates.get('pub:/pub/texture/tide-barrier-pub1')).toBe(docs[0].date!);
  });
  test('the file paper.ts writes is read from the phone and searched offline', async () => {
    const files = new Map<string, Uint8Array>([[PAPER_FRONT_FILE, new TextEncoder().encode(JSON.stringify(cachedFront))]]);
    const corpus = new Corpus();
    corpus.hold('paper-front', await loadPaperFront(fakeStorage(files), corpus.storyDates));
    const c = corpus.retrieve({ words: 'tide barrier harbor', now: NOW });
    expect(c.items[0].address).toBe('pub:/pub/texture/tide-barrier-pub1');
    expect(c.text).toContain('Autopaper');
    expect(corpus.storyDates.size).toBe(2);
  });
  test('a missing or corrupt cache is no documents, never a throw; a new list replaces the old', async () => {
    expect(await loadPaperFront(fakeStorage(new Map()))).toEqual([]);
    expect(await loadPaperFront(fakeStorage(new Map([[PAPER_FRONT_FILE, new TextEncoder().encode('{nope')]])))).toEqual([]);
    expect(await loadPaperFront(undefined)).toEqual([]);
    const corpus = new Corpus();
    corpus.hold('paper-front', paperFrontDocs(cachedFront));
    corpus.hold('paper-front', paperFrontDocs({ stories: [cachedFront.stories[1]] }));
    expect(corpus.retrieve({ words: 'tide barrier harbor', now: NOW }).items.length).toBe(0);
    expect(corpus.retrieve({ words: 'glass hall lamps', now: NOW }).items.length).toBe(1);
  });
  test('the page\'s own file is the grant paper.ts and the search both name', async () => {
    const { CACHE_FILE } = await import('./paper.ts');
    expect(PAPER_FRONT_FILE).toBe(CACHE_FILE);
  });
});

describe('S14 the paper-wide route', () => {
  test('the real shape: unopenable and repeated hits are skipped, the heading is not repeated', () => {
    const hits = decodeCorpus(corpusBody);
    expect(hits.map((h) => h.route)).toEqual(['/pub/texture/tide-barrier-a', '/pub/texture/harbor-strike-b']);
    expect(hits[0].address).toBe('pub:/pub/texture/tide-barrier-a');
    expect(hits[0].snippet).toContain('harbor controversy');
    expect(hits[0].snippet).not.toContain('\n');
    expect(hits[1].snippet).toBe('Dock workers voted to end the strike');
  });
  test('no results and bad bodies', () => {
    expect(decodeCorpus({ query: 'zzz', results: [] })).toEqual([]);
    expect(decodeCorpus({ query: 'zzz', results: null })).toEqual([]);
    expect(() => decodeCorpus('<html>')).toThrow();
    expect(() => decodeCorpus({ results: 'no' })).toThrow();
    expect(corpusNote({ state: 'done', hits: [] }, false)).toBe('Nothing found in the paper.');
    expect(corpusNote({ state: 'done', hits: [] }, true)).toBe('');
    expect(corpusNote({ state: 'failed', hits: [] }, false)).not.toBe('');
    expect(corpusNote({ state: 'idle', hits: [] }, false)).toBe('');
  });
  test('a blank query asks nothing; the query is normalised; the route is public and the query encoded', async () => {
    expect(normalizeQuery('  \n\t ')).toBe('');
    expect(normalizeQuery('  Harbor   chips ')).toBe('Harbor chips');
    let asked = 0;
    expect((await searchPaper('   ', async () => { asked++; return {}; })).state).toBe('idle'); expect(asked).toBe(0);
    const calls: string[] = [];
    const prev = globalThis.fetch;
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push(String(input)); expect(JSON.stringify(init)).not.toMatch(/authorization|cookie/i);
      return new Response(JSON.stringify(corpusBody), { status: 200 });
    }) as typeof fetch;
    try {
      for (const [w, want] of [['AT&T', 'q=AT%26T'], ['C++ x', 'q=C%2B%2B%20x'], ['a#b=c', 'q=a%23b%3Dc'], ['café', 'q=caf%C3%A9']]) {
        const s = await searchPaper(w, async (path, query) => (await getJSON(path, query)).body);
        expect(s.state).toBe('done');
        expect(calls[calls.length - 1]).toBe(`https://choir.news${CORPUS_PATH}?${want}`);
      }
    } finally { globalThis.fetch = prev; }
  });
  test('a server error or a body that is not JSON is `failed`, and the local rows stay', async () => {
    const s = await searchPaper('harbor', async () => { throw new Error('answered 500'); });
    expect(s.state).toBe('failed');
    const local = [{ address: 'pub:/pub/texture/harbor-strike-b', title: 'Harbor strike ends', kind: 'story', detail: 'Tech' }];
    expect(corpusRows(s.hits, local)).toEqual(local);
  });
  test('rows keep local first and dedupe by address', () => {
    const local = [{ address: 'pub:/pub/texture/harbor-strike-b', title: 'Harbor strike ends', kind: 'story', detail: 'Tech' }];
    const rows = corpusRows(decodeCorpus(corpusBody), local);
    expect(rows.map((r) => r.address)).toEqual(['pub:/pub/texture/harbor-strike-b', 'pub:/pub/texture/tide-barrier-a']);
    expect(rows[0].detail).toBe('Tech');
  });
});

describe('S15 files, topics and open pages', () => {
  test('they become documents; a replaced group drops its old documents', () => {
    // (no mail documents: mail text stays out of the index until a search over sealed mail is designed)
    const files = fileNameDocs([{ name: 'budget-notes.md', path: '/docs/budget-notes.md', isDirectory: false, modified: null }, { name: 'docs', path: '/docs', isDirectory: true, modified: null }]);
    expect(files.map((f) => f.title)).toEqual(['budget-notes.md']);
    expect(editionTopicDocs(['Science', 'Senate'])[0].text).toContain('Senate'); expect(editionTopicDocs([])).toEqual([]);
    expect(openPageDocs([{ address: 'pub:/p/x', kind: 'story', title: 'Open story' }])[0].address).toBe('pub:/p/x');
    const c = new Corpus();
    c.hold('files:/', fileNameDocs([{ name: 'zeppelin-plans.md', path: '/z.md', isDirectory: false, modified: null }]));
    expect(c.retrieve({ words: 'zeppelin plans', now: NOW }).items[0].address).toBe('file:/z.md');
    c.hold('files:/', []);
    expect(c.retrieve({ words: 'zeppelin plans', now: NOW }).items.length).toBe(0);
    c.hold('texture', [{ id: 'texturedoc:1', kind: 'document', title: 'Garden notes', text: 'Tomatoes and basil.', date: NOW, source: 'Texture', address: 'texture:1' }]);
    expect(c.retrieve({ words: 'tomatoes basil', now: NOW }).items[0].address).toBe('texture:1');
    c.reset();
    expect(c.retrieve({ words: 'tomatoes basil', now: NOW }).items.length).toBe(0);
  });
});

// S16 ported from TalkTests.RetrievalIndexPortedTests (the old PassageIndexTests, on the one index)
describe('S16 the passage index tests of the talk lane', () => {
  const idx = (): Index => {
    const i = new Index();
    i.upsert({ id: 'a', kind: 'story', title: 'Harbor strike ends after nine days', text: 'Dock workers at the harbor voted on Tuesday to end a nine day strike over pay and shift lengths.\nThe union said talks with the port authority resume Monday.', date: null, source: '', address: 'pub:a' });
    i.upsert({ id: 'b', kind: 'story', title: 'Central bank holds interest rates', text: 'The central bank left interest rates unchanged on Wednesday, citing steady inflation and a cooling labour market.', date: null, source: '', address: 'pub:b' });
    return i;
  };
  const find = (i: Index, w: string, page: string | null = null) => retrieve(bm25Retriever(i), q(w, '', page)).items;
  test('finds the right story; plural and singular match; unanswerable questions return nothing; one shared common word is not relevance', () => {
    expect(find(idx(), 'what happened with the harbor strike')[0].id).toBe('a');
    expect(find(idx(), 'what did the bank do about interest rate')[0].id).toBe('b');
    expect(find(idx(), 'what is the weather on Mars today')).toEqual([]);
    expect(find(idx(), 'who won the football match')).toEqual([]);
    expect(find(idx(), 'lightning strike survivors in Peru')).toEqual([]);
  });
  test('stop words only, empty, and an empty index return nothing; indexing again replaces', () => {
    expect(find(idx(), 'what about it')).toEqual([]); expect(find(idx(), '')).toEqual([]); expect(find(new Index(), 'harbor strike')).toEqual([]);
    const i = idx(); const n = i.count;
    i.upsert({ id: 'a', kind: 'story', title: 'Harbor strike ends after nine days', text: 'Dock workers ended the harbor strike on Tuesday after nine days of talks and pay.', date: null, source: '', address: 'pub:a' });
    expect(i.count).toBeLessThanOrEqual(n); expect(find(i, 'harbor strike').filter((x) => x.id === 'a').length).toBe(1);
  });
  test('a huge line is cut into bounded passages; non-ASCII text is indexed; the page in front is preferred', () => {
    const parts = passages(Array.from({ length: 400 }, (_, n) => `word${n} harbor`).join(' '));
    expect(parts.length).toBeGreaterThan(1); expect(parts.every((p) => p.length <= 2000)).toBe(true);
    const i = new Index();
    i.upsert({ id: 'c', kind: 'story', title: 'Café reopens', text: 'The café on rue Saint-Honoré reopens after the renovation, with a new espresso menu.', date: null, source: '', address: 'pub:c' });
    expect(find(i, 'café renovation reopens')[0].id).toBe('c');
    const j = new Index();
    const body = 'Interest rates stayed unchanged at the central bank this week.';
    j.upsert({ id: 'x', kind: 'story', title: 'Rates', text: body, date: null, source: '', address: 'pub:x' });
    j.upsert({ id: 'page:y', kind: 'page', title: 'Rates', text: body, date: null, source: '', address: 'pub:y' });
    expect(find(j, 'central bank interest rates', 'page:y')[0].id).toBe('page:y');
  });
});
