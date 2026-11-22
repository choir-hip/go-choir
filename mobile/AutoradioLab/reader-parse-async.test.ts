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
