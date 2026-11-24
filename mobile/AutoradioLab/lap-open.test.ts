// The laps of a first open (lap.ts `lapOpen`, read by mobile/scripts/laps.sh): a page's open and its source's answer each write their marks, with the page's address, in the order the work happens.
import { beforeEach, expect, test } from 'bun:test';
import * as P from './pages.ts';
import { resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
} } as never;
beforeEach(() => { files.clear(); P.forget(); resetSeam(); });
async function lines<T>(run: () => Promise<T>): Promise<string[]> {
  const out: string[] = []; const o = console.log; console.log = (...a: unknown[]) => { out.push(a.join(' ')); };
  try { await run(); } finally { console.log = o; }
  return out.filter((l) => l.startsWith('lap: '));
}

test('L1 pagesOpen marks its entry, the load and the view; the save, the recent note and the mirror finish behind it; each carries the page address', async () => {
  const got = await lines(async () => { await P.pagesOpen(storage, 'paper:front', ''); await P.pagesSettled(); await new Promise((r) => setTimeout(r, 5)); });
  const m = got.filter((l) => l.includes('pagesOpen'));
  expect(m.slice(0, 3)).toEqual(['lap: open pagesOpen entry paper:front', 'lap: open pagesOpen loaded paper:front', 'lap: open pagesOpen view ready paper:front']);
  expect(m.slice(3).sort()).toEqual(['lap: open pagesOpen mirrored paper:front', 'lap: open pagesOpen recent noted paper:front', 'lap: open pagesOpen saved paper:front']);
});

test('L2 a page source marks its entry, the settling and its end with the page address; a source that is not a page writes none', async () => {
  const { answer, PAGE_ADDRESS } = await import('./app.ts');
  globalThis.fetch = (async () => new Response('{"stories":[]}', { status: 200 })) as never;
  const got = await lines(() => answer('paperFront' as never, ['front', 400, 0, 'paper', ''] as never, undefined as never, storage, undefined as never));
  const open = got.filter((l) => l.startsWith('lap: open '));
  expect(open).toEqual(['lap: open answer entry paperFront paper:front', 'lap: open answer settled paperFront paper:front', 'lap: open answer done paperFront paper:front']);
  expect(got.some((l) => l.startsWith('lap: net get /api/platform/publications/front') && l.endsWith('done'))).toBe(true);
  for (const name of ['radioPulse', 'barInfo', 'theme', 'accountState']) expect(PAGE_ADDRESS[name]).toBeUndefined();
  for (const name of ['paperFront', 'readerDocShared', 'mailBox', 'fileList', 'podcastShow', 'takesPage', 'notebookPageShared', 'texturesList', 'settingsAccount', 'radioPage']) expect(PAGE_ADDRESS[name]).toBeDefined();
});
