// C-1 of the train review: at MAX_INDEX_ITEMS the page in front, the entry on air and the edition topics were dropped and marked indexed.
//  K1 at the cap, a pinned document (page, run sheet, topics) is indexed and found; it evicts the oldest unpinned document, never the other way.
//  K2 a document that was not indexed is not marked indexed (contains is false) and is tried again when room appears.
//  K3 the cap holds: the count never passes MAX_INDEX_ITEMS plus the pinned documents' own passages.
//  K4 a newer unpinned document evicts an older one; an older one does not evict a newer one.
//  K5 through Corpus.hold the groups the radio asks about ('page', 'runsheet', 'open-pages', 'edition-topics') are pinned.
import { describe, expect, test } from 'bun:test';
import { Index, Corpus, MAX_INDEX_ITEMS, runJob, VectorStore, workItems, BUSY_WAIT_MAX_MS, BUSY_GIVE_UP, type Doc } from './search.ts';

const doc = (id: string, text: string, date: number | null = 1000): Doc => ({ id, kind: 'story', title: id, text, date, source: 'store', address: `pub:${id}` });
const fill = (c: Corpus | Index, n: number, date = 1000): void => {
  const docs: Doc[] = [];
  for (let i = 0; i < n; i++) docs.push(doc(`s${i}`, `filler passage number ${i} about weather`, date + i));
  if (c instanceof Corpus) c.hold('store', docs); else c.upsertAll(docs);
};

describe('the index at its cap', () => {
  test('K1 a pinned document is indexed and found at the cap; the oldest unpinned goes', () => {
    const ix = new Index(); fill(ix, MAX_INDEX_ITEMS);
    expect(ix.count).toBe(MAX_INDEX_ITEMS);
    expect(ix.upsert({ ...doc('page:x', 'a zebra crossing', 5000) }, true)).toBe(true);
    expect(ix.contains('page:x')).toBe(true);
    expect(ix.contains('s0')).toBe(false);
    expect(ix.count).toBeLessThanOrEqual(MAX_INDEX_ITEMS);
  });
  test('K2 an unindexed document is not marked indexed and is retried when room appears', () => {
    const ix = new Index(); fill(ix, MAX_INDEX_ITEMS, 5000);
    const old = doc('old', 'ancient zebra', 1);
    expect(ix.upsert(old)).toBe(false);
    expect(ix.contains('old')).toBe(false);
    ix.remove('s0');
    expect(ix.upsert(old)).toBe(true);
    expect(ix.contains('old')).toBe(true);
  });
  test('K3 the count holds at the cap under many pinned and unpinned upserts', () => {
    const ix = new Index(); fill(ix, MAX_INDEX_ITEMS + 300);
    for (let i = 0; i < 5; i++) ix.upsert(doc(`p${i}`, `pinned ${i} zebra`, 9000 + i), true);
    expect(ix.count).toBeLessThanOrEqual(MAX_INDEX_ITEMS);
    for (let i = 0; i < 5; i++) expect(ix.contains(`p${i}`)).toBe(true);
  });
  test('K4 newer evicts older, older does not evict newer', () => {
    const ix = new Index(); fill(ix, MAX_INDEX_ITEMS, 1000);
    expect(ix.upsert(doc('new', 'fresh', 99999))).toBe(true);
    expect(ix.contains('new')).toBe(true);
    expect(ix.contains('s0')).toBe(false);
    expect(ix.upsert(doc('older', 'stale', 5))).toBe(false);
    expect(ix.contains('new')).toBe(true);
  });
  test('K5 the corpus pins the groups the radio answers "that" and "this" from', () => {
    const c = new Corpus(); fill(c, MAX_INDEX_ITEMS + 100);
    c.hold('page', [doc('page:z', 'the zebra story is here', 10)]);
    c.hold('runsheet', [doc('rs:1', 'on air giraffe entry', 10)]);
    c.hold('edition-topics', [doc('topic:1', 'okapi topic', 10)]);
    c.hold('open-pages', [doc('open:1', 'tapir page', 10)]);
    for (const w of ['zebra', 'giraffe', 'okapi', 'tapir']) expect(c.retrieve({ words: w, now: 99999 }).items.length).toBeGreaterThan(0);
  });
});

// C-5: the embedding pass waited 150 ms, forever, while the radio was busy, the phone in Low Power Mode or the native state call failing.
//  B1 the wait grows (doubling, from the rest delay to BUSY_WAIT_MAX_MS) and the pass ends after BUSY_GIVE_UP busy answers in a row; it reports `gaveUp`.
//  B2 a busy answer that clears resets the wait; the job then finishes.
describe('the embedding pass while the device is busy', () => {
  const items = () => workItems((() => { const ix = new Index(); for (let i = 0; i < 6; i++) ix.upsert(doc(`d${i}`, `passage ${i}`)); return ix; })());
  const embed = async (p: string[]) => p.map(() => { const v = new Float32Array(8); v[0] = 1; return v; });
  test('B1 always busy: growing waits, a bounded count, then the pass ends', async () => {
    const waits: number[] = []; let asks = 0;
    const p = await runJob(new VectorStore('f', 8), embed as never, items(), { busy: () => { asks++; return true; }, rest: async () => undefined, wait: async (ms) => { waits.push(ms); }, restMs: 150 });
    expect(p.finished).toBe(false); expect(p.gaveUp).toBe(true);
    expect(asks).toBe(BUSY_GIVE_UP);
    expect(waits.slice(0, 4)).toEqual([150, 300, 600, 1200]);
    expect(Math.max(...waits)).toBe(BUSY_WAIT_MAX_MS);
  });
  test('B2 a busy answer that clears resets the wait', async () => {
    const waits: number[] = []; let n = 0;
    const p = await runJob(new VectorStore('f', 8), embed as never, items(), { busy: () => (n++ % 3) !== 2, rest: async () => undefined, wait: async (ms) => { waits.push(ms); }, restMs: 150, budget: { passagesPerSlice: 1, savesEverySlices: 40, maxChars: 1800 } });
    expect(p.finished).toBe(true); expect(p.gaveUp).toBe(false);
    expect(Math.max(...waits)).toBe(300);
  });
});
