// A page mounts at once (first-open lane, fix 1). Failure list, written before the code:
//  B1 pagesOpen returns the view without waiting for the open-set file, the recent note or the voice's mirror: with writes and the native call that never finish, it still answers.
//  B2 the three are STARTED before it returns, in order (open set, recent note, mirror): Exact refuses a storage or native call made after the answer ended ("refused in background work", reference.md 3036),
//     so "behind the view" means started, not awaited.
//  B3 a write that fails is logged, never silent; the open still stands in memory.
//  B4 the mirror carries the set as it was at the open (the generation the open began under), and the view is the same one.
//  B5 a sign-out that begins after the open returned: the late write does not bring the old set back (the file is removed again), as before.
import { beforeEach, expect, test } from 'bun:test';
import * as P from './pages.ts';
import * as R from './recent-pages.ts';
import { bindNative, nextGeneration, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const order: string[] = [];
let hang = false; let failWrites = false;
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { order.push('write ' + p); if (hang) await new Promise(() => undefined); if (failWrites) throw new Error('disk full'); files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;
const native = { available: true, later: async (r: Record<string, unknown>) => { order.push('native ' + String(r.op)); if (hang) await new Promise(() => undefined); return { ok: true }; } };
beforeEach(() => { files.clear(); order.length = 0; hang = false; failWrites = false; P.forget(); R.forget(); resetSeam(); bindNative(native as never); });
const within = <T>(p: Promise<T>, ms: number): Promise<T | 'late'> => Promise.race([p, new Promise<'late'>((r) => setTimeout(() => r('late'), ms))]);

test('B1 the open answers while the writes and the voice call never finish', async () => {
  await P.pagesView(storage);                                    // the launch's first read (it loads the set and hands the voice its facts)
  hang = true;
  const v = await within(P.pagesOpen(storage, 'paper:front', ''), 200);
  expect(v).not.toBe('late');
  expect((v as P.PageList).top).toBe('paper:front');
});

test('B2 the open set, the recent note and the mirror are all started before the answer, in that order', async () => {
  await P.pagesView(storage); order.length = 0;
  hang = true;
  await within(P.pagesOpen(storage, 'paper:front', ''), 200);
  expect(order).toEqual(['write app:/data/open-pages-shared.json', 'write app:/data/recent-pages.json', 'native pagesMirror']);
});

test('B3 a failed write is logged, not lost silently, and the open stands', async () => {
  await P.pagesView(storage);
  failWrites = true;
  const out: string[] = []; const o = console.log; console.log = (...a: unknown[]) => { out.push(a.join(' ')); };
  let v: P.PageList;
  try { v = await P.pagesOpen(storage, 'paper:front', ''); await P.pagesSettled(); await R.recentSettled(); } finally { console.log = o; }
  expect(v.top).toBe('paper:front');
  expect(out.some((l) => /^pages: open set not saved/.test(l))).toBe(true);
  expect(out.some((l) => /^recent-pages: .*not saved/.test(l))).toBe(true);
});

test('B4 the mirror carries the set of this open', async () => {
  const seen: { gen: number; pages: { address: string }[] }[] = [];
  bindNative({ available: true, later: async (r: Record<string, unknown>) => { if (r.op === 'pagesMirror') seen.push(r as never); return { ok: true }; } } as never);
  await P.pagesView(storage); seen.length = 0;
  const v = await P.pagesOpen(storage, 'textures:', '');
  expect(seen.length).toBe(1); expect(seen[0].pages.map((p) => p.address)).toEqual(v.rows.map((r) => r.address));
});

test('B5 a sign-out after the open returned: the late write does not bring the old set back', async () => {
  await P.pagesView(storage);
  let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
  const slow = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async (p: string, d: Uint8Array) => { await gate; files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
  await P.pagesView(slow);
  await P.pagesOpen(slow, 'paper:front', '');
  nextGeneration(); await P.wipePages(slow);
  release(); await P.pagesSettled(); await R.recentSettled();
  expect(files.has(P.PAGES_FILE)).toBe(false);
});
