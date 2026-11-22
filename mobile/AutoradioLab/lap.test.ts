import { afterEach, expect, test } from 'bun:test';
import { lap, lapYield } from './lap.ts';
import { parseBundleAsync } from './reader-parse.ts';

const g = globalThis as { scheduler?: unknown };
afterEach(() => { delete g.scheduler; });

test('a yield point writes one lap line, then ends the turn; without a scheduler it is silent', async () => {
  const lines: string[] = []; const o = console.log; console.log = (...a: unknown[]) => { lines.push(a.join(' ')); };
  let yielded = 0; g.scheduler = { yield: async () => { yielded += 1; } };
  try { await lapYield('here'); lap('there'); delete g.scheduler; await lapYield('silent'); } finally { console.log = o; }
  expect(lines).toEqual(['lap: here', 'lap: there']); expect(yielded).toBe(1);
});

test('the story decoding labels its yield points', async () => {
  const labels: string[] = [];
  await parseBundleAsync({ publication: { title: 'T' }, artifact: { content: 'One.\n\nTwo.' }, payload: {} }, 'pub:x', async (l) => { labels.push(l); });
  expect(labels).toContain('decoded body_doc'); expect(labels.some((l) => l.startsWith('cites '))).toBe(true); expect(labels).toContain('plain text');
});
