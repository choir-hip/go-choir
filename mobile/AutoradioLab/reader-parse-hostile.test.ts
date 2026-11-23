// A-1 of the train review: a hostile or merely odd published text must not take quadratic time in the parser (a turn is 100 ms; Hermes is about 8x slower than Bun).
// Failure list, before the code:
//  H1 sentenceEnds on "a. a. a. ..." (every period looks back).        H2 mdInline on unmatched [ * _ ` ** and "[](" runs (every marker scanned to the end).
//  H3 duplicate source_ref anchors (a suffix counter restarted at 2).  H4 runs of adjacent cites (each looked back past all the earlier ones).
//  H5 legacy ** in an unmarked text node.  H6 one huge node ends the turn at least every few thousand characters (the yield is by work, not only by node count).
// A timing is a ratio of the best of three runs at n and at 2n: doubling the input must not quadruple the time (limit 3.2: linear is about 2).
import { describe, expect, test } from 'bun:test';
import { sentenceEnds, mdInline, parseBundle, parseBundleAsync } from './reader-parse.ts';

const best = (f: () => unknown): number => { let m = Infinity; for (let i = 0; i < 3; i++) { const t = performance.now(); f(); m = Math.min(m, performance.now() - t); } return m; };
/** Doubling the input must not quadruple the time. Sizes are large enough that a quadratic routine is far over the limit. */
function scales(make: (n: number) => () => unknown, n = 40000): void {
  const a = best(make(n)), b = best(make(2 * n));
  expect(b).toBeLessThan(Math.max(a * 3.2, 25));          // the floor keeps a 1 ms routine from failing on noise; a quadratic one is hundreds of ms here
  expect(b).toBeLessThan(150);
}
const bodyDoc = (content: string) => `{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[${content}]}}`;
const bundle = (body: string) => JSON.parse(`{"publication":{"title":"Headline","kind":"story","dek":"A dek."},"route":{"bare":"headline","permalink":"headline/v2"},"artifact":{"body_doc":${JSON.stringify(body)},"content":""},"payload":{},"nav":{},"version":{}}`);

describe('hostile input is linear', () => {
  test('H1 sentenceEnds on a run of one-letter sentences', () => {
    scales((n) => { const s = 'a. '.repeat(n / 3); return () => sentenceEnds(s); });
  });
  test('H1b sentenceEnds still splits as before (abbreviation, initial, lower-case continuation)', () => {
    const s = 'Dr. Smith went home. It rained. J. K. Rowling wrote it. Then he left? Yes!';
    expect(sentenceEnds(s).map((e, i, a) => s.slice(i === 0 ? 0 : a[i - 1], e).trim())).toEqual(['Dr. Smith went home.', 'It rained.', 'J. K. Rowling wrote it.', 'Then he left?', 'Yes!']);
  });
  for (const m of ['[', '*', '_', '`', '**', '[](', '[a](', '[a](x ']) {
    test(`H2 mdInline on unmatched ${JSON.stringify(m)}`, () => {
      scales((n) => { const s = m.repeat(Math.ceil(n / m.length)) + (m.endsWith('(') || m.endsWith('x ') ? ')' : ''); return () => mdInline(s, 'p'); }, 20000);
    });
  }
  test('H2b mdInline still reads strong, emphasis, code and links', () => {
    const r = mdInline('a **b** *c* `d` [e](http://x) f [g', 'p');
    expect(r.map((x) => [x.kind, x.text, x.weight, x.slant])).toEqual([['t', 'a ', 400, 'normal'], ['t', 'b', 700, 'normal'], ['t', ' ', 400, 'normal'], ['t', 'c', 400, 'italic'], ['t', ' ', 400, 'normal'], ['code', 'd', 400, 'normal'], ['t', ' ', 400, 'normal'], ['link', 'e', 400, 'normal'], ['t', ' f [g', 400, 'normal']]);
  });
  test('H3 duplicate source_ref anchors', () => {
    scales((n) => { const body = bodyDoc(`{"type":"paragraph","content":[${Array.from({ length: n / 10 }, () => '{"type":"source_ref","attrs":{"id":"dup","source_entity_id":"e"}}').join(',')}]}`); const b = bundle(body); return () => parseBundle(b, 'pub:x'); }, 8000);
  });
  test('H3b duplicate anchors still get -2, -3, ...', () => {
    const body = bodyDoc(`{"type":"paragraph","content":[{"type":"text","text":"x"},${[1, 2, 3].map(() => '{"type":"source_ref","attrs":{"id":"a","source_entity_id":"e"}}').join(',')}]}`);
    expect(parseBundle(bundle(body), 'pub:x').cites.map((c) => c.anchor)).toEqual(['a', 'a-2', 'a-3']);
  });
  test('H4 a long run of adjacent cites', () => {
    scales((n) => { const body = bodyDoc(`{"type":"paragraph","content":[{"type":"text","text":"Lead. "},${Array.from({ length: n / 10 }, (_, i) => `{"type":"source_ref","attrs":{"id":"c${i}","source_entity_id":"e${i}"}}`).join(',')}]}`); const b = bundle(body); return () => parseBundle(b, 'pub:x'); }, 8000);
  });
  test('H5 unmatched ** in a legacy text node', () => {
    scales((n) => { const body = bodyDoc(`{"type":"paragraph","content":[{"type":"text","text":${JSON.stringify('**x '.repeat(n / 4))}}]}`); const b = bundle(body); return () => parseBundle(b, 'pub:x'); }, 8000);
  });
  test('H6 one huge paragraph ends the turn several times, not once', async () => {
    const body = bodyDoc(`{"type":"paragraph","content":[{"type":"text","text":${JSON.stringify('word '.repeat(60000))}}]}`);
    let ticks = 0;
    await parseBundleAsync(bundle(body), 'pub:x', async () => { ticks++; });
    const md = JSON.parse(`{"publication":{"title":"T","kind":"story"},"route":{},"artifact":{"content":${JSON.stringify(('word '.repeat(200) + '\n\n').repeat(300))}},"payload":{},"nav":{},"version":{}}`);
    let mdTicks = 0;
    await parseBundleAsync(md, 'pub:x', async () => { mdTicks++; });
    expect(mdTicks).toBeGreaterThan(20);
    expect(ticks).toBeGreaterThan(0);
  });
});
