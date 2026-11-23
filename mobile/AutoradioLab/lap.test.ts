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

test('ten stories loaded at once are decoded one after the other, never in the same turn', async () => {
  const { loadReader, resetReaderForTest } = await import('./reader.ts');
  resetReaderForTest();
  const lines: string[] = []; const o = console.log; console.log = (...a: unknown[]) => { lines.push(a.join(' ')); };
  // every yield resumes all that waited together, as Exact's scheduler does: the resumptions of one batch are one turn
  const waiting: (() => void)[] = [];
  g.scheduler = { yield: () => new Promise<void>((r) => { waiting.push(r); if (waiting.length === 1) queueMicrotask(() => setTimeout(() => { const b = waiting.splice(0); for (const f of b) f(); }, 0)); }) };
  const bundleOf = (i: number): Record<string, unknown> => ({ publication: { title: 'T' + i, kind: 'story' }, artifact: { content: Array.from({ length: 40 }, (_, k) => `Para ${k} of ${i}.`).join('\n\n') }, payload: {}, nav: {}, route: {}, version: {} });
  const net = { pub: async (path: string, q?: Record<string, unknown>) => ({ body: path.endsWith('related') ? { stories: [] } : bundleOf(Number(String(q?.route).slice(-1))) }), authed: async () => ({ body: {} }) } as never;
  try { await Promise.all(Array.from({ length: 10 }, (_, i) => loadReader('pub:/pub/texture/s-' + i, net))); } finally { console.log = o; }
  // the resumptions in one batch: with the lock, the decoding steps of at most ONE story are ever in the same batch
  const decoding = lines.filter((l) => l.startsWith('lap: ') && /: (decoded body_doc|nodes|built|cites|plain text|markdown|entities|resolved blocks|before build)/.test(l));
  expect(decoding.length).toBeGreaterThan(10);
  const owners: string[] = []; for (const l of decoding) { const who = /s-\d/.exec(l)?.[0] ?? '?'; if (owners[owners.length - 1] !== who) owners.push(who); }
  expect(owners.length).toBe(new Set(owners).size);              // each story's steps are contiguous: none is interleaved with another's
});
