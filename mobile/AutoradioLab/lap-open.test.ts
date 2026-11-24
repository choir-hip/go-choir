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

test('L1 pagesOpen marks its entry, the load and the view; the save, the recent note and the mirror finish behind it; each carries the page kind', async () => {
  const got = await lines(async () => { await P.pagesOpen(storage, 'paper:front', ''); await P.pagesSettled(); await new Promise((r) => setTimeout(r, 5)); });
  const m = got.filter((l) => l.includes('pagesOpen'));
  expect(m.slice(0, 3)).toEqual(['lap: open pagesOpen entry paper', 'lap: open pagesOpen loaded paper', 'lap: open pagesOpen view ready paper']);
  expect(m.slice(3).sort()).toEqual(['lap: open pagesOpen mirrored paper', 'lap: open pagesOpen recent noted paper', 'lap: open pagesOpen saved paper']);
});

test('L2 a page source marks its entry, the settling and its end with the page address; a source that is not a page writes none', async () => {
  const { answer, PAGE_ADDRESS } = await import('./app.ts');
  globalThis.fetch = (async () => new Response('{"stories":[]}', { status: 200 })) as never;
  const got = await lines(() => answer('paperFront' as never, ['front', 400, 0, 'paper', ''] as never, undefined as never, storage, undefined as never));
  const open = got.filter((l) => l.startsWith('lap: open '));
  expect(open).toEqual(['lap: open answer entry paperFront paper', 'lap: open answer settled paperFront paper', 'lap: open answer done paperFront paper']);
  expect(got.some((l) => l === 'lap: net get done /api/platform/*')).toBe(true);
  for (const name of ['radioPulse', 'barInfo', 'theme', 'accountState']) expect(PAGE_ADDRESS[name]).toBeUndefined();
  for (const name of ['paperFront', 'readerDocShared', 'mailBox', 'fileList', 'podcastShow', 'takesPage', 'notebookPageShared', 'texturesList', 'settingsAccount', 'radioPage']) expect(PAGE_ADDRESS[name]).toBeDefined();
});

test('L3 no lap line carries an address, an id, a file name or a query: the kind and the route pattern only (the log is on in every build)', async () => {
  const secrets = ['my-laptop-name', 'Documents', 'Taxes 2025', 'secret-doc-id', 'msg-4f9a1c', 'my private search', 'tok3n', 'feeds.example.com'];
  const got = await lines(async () => {
    await P.pagesOpen(storage, 'file:/Documents/Taxes 2025.pdf', ''); await P.pagesOpen(storage, 'doc:secret-doc-id@12', ''); await P.pagesOpen(storage, 'mail:msg-4f9a1c', '');
    await P.pagesOpen(storage, 'pub:/pub/texture/my private search', ''); await P.pagesSettled();
    const { answer } = await import('./app.ts');
    globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never;
    for (const [src, args] of [['fileOpen', ['file:/Documents/Taxes 2025.pdf', '']], ['mailMessage', ['mail:msg-4f9a1c', false, '']], ['readerDocShared', ['pub:/pub/texture/my private search', '', '', 0]], ['fileList', ['/Documents/Taxes 2025', 'browse', 'name', false, 'my private search', '']]] as [string, unknown[]][]) {
      try { await answer(src as never, args as never, undefined as never, storage, undefined as never); } catch { /* the marks are what is read */ }
    }
    const { getJSON } = await import('./api.ts');
    try { await getJSON('/api/email/messages/msg-4f9a1c/read', { q: 'my private search' }); } catch { /* */ }
    try { await getJSON('/api/files/Documents/Taxes%202025.pdf'); } catch { /* */ }
    try { await getJSON('/api/computers/my-laptop-name/status'); } catch { /* */ }
  });
  expect(got.length).toBeGreaterThan(10);
  for (const l of got) for (const s of secrets) expect(l.includes(s), `${l} carries ${s}`).toBe(false);
  expect(got).toContain('lap: net get done /api/email/*');
  expect(got).toContain('lap: net get done /api/computers/*'); expect(got).toContain('lap: net get done /api/files/*');
  expect(got).toContain('lap: open pagesOpen entry files'); expect(got).toContain('lap: open pagesOpen entry mail'); expect(got).toContain('lap: open pagesOpen entry imported');
});
