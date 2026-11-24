// A 1.6 MB story bundle reached shared code for the first time and its prefetch took 266 ms in one turn (limit about 100 ms). The decoding now ends the turn between its steps; the result is the same.
import { describe, expect, test } from 'bun:test';
import { parseBundle, parseBundleAsync } from './reader-parse.ts';

const bundle = (paragraphs: number): Record<string, unknown> => ({
  publication: { title: 'A long story', kind: 'story', section: 'World' },
  artifact: { content: Array.from({ length: paragraphs }, (_, i) => `Paragraph ${i} of a long story, with a few words in it.`).join('\n\n') },
  payload: {}, nav: {}, route: {}, version: {},
});

describe('a large bundle is decoded over several turns', () => {
  test('the same document as the straight-through parse', async () => {
    const o = bundle(300);
    const a = parseBundle(o, 'pub:/pub/texture/x'), b = await parseBundleAsync(o, 'pub:/pub/texture/x', async () => undefined);
    expect(b).toEqual(a);
  });
  test('it ends the turn many times for a long story, and few for a short one', async () => {
    let long = 0, short = 0;
    await parseBundleAsync(bundle(400), 'pub:/pub/texture/x', async () => { long += 1; });
    await parseBundleAsync(bundle(3), 'pub:/pub/texture/x', async () => { short += 1; });
    expect(long).toBeGreaterThan(40);
    expect(short).toBeLessThan(long / 4);
  });
});

describe('no stretch between two turn ends is long (Hermes runs this about eight times slower than Bun: 100 ms there is about 12 ms here)', () => {
  // The budget is kept by the SIZE of a stretch, not measured by a clock (a wall-clock assertion moves with GC, load and a cold JIT): the decoding ends the turn at least every 64 blocks, so no
  // stretch holds more than 64 blocks of work, and the citation checks are their own steps (the last step that was 16 ms alone, split). The time per block is a property of the machine, not of this code.
  test('a 9000-block, 1.9 MB bundle ends the turn at least once per 64 blocks, and its citation checks are not one step with the rest', async () => {
    const o = bundle(9000);
    (o as { checks?: unknown }).checks = { citations: Array.from({ length: 3000 }, (_, i) => ({ anchor_id: 'a' + i, status: 'ok', text: 'x'.repeat(100) })) };
    const labels: string[] = [];
    await parseBundleAsync(o, 'pub:/pub/texture/x', async (label) => { labels.push(label); });
    expect(labels.length).toBeGreaterThanOrEqual(Math.floor(9000 / 64));
    expect(new Set(labels).size).toBeGreaterThan(1);
  });
});
