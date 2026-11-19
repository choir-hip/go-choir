// Sign-out orchestrated from shared code (account.ts). Failure list, each line a test below:
//  S1 the native sign-out (key delete, Swift stores, revoke) runs FIRST and the shared wipes only after it answered: the old key is
//     still held while the native side settles what it posts; pending shared work is never sent under a later account.
//  S2 when the native sign-out fails or is missing in a native build, nothing shared is wiped and the answer says so (retry is safe).
//  S3 one wipe that throws does not stop the others; the answer names what failed, never why in the listener's words.
//  S4 every path the shared modules keep under app:/data or app:/cache is removed by some wipe (a new store without a wipe fails here).
//  S5 the answer never carries a key.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { accountSignOut, WIPES, wipeAll, WIPED_PATHS } from './account.ts';

type Fs = Record<string, unknown>;
function memStorage(files: string[]) {
  const have = new Set(files); const removed: string[] = [];
  return {
    removed,
    storage: {
      fs: {
        directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
        rm: async (p: string) => { removed.push(p); have.delete(p); },
        readdir: async () => [], mkdir: async () => undefined, readFile: async () => { throw new Error('none'); },
      } as unknown as Fs,
      sqlite: {} as never,
    } as never,
  };
}

describe('accountSignOut', () => {
  test('S1 native first, then the shared wipes', async () => {
    const order: string[] = [];
    const { storage } = memStorage([]);
    const native = { available: true, later: async (r: { op: string }) => { order.push(`native:${r.op}`); return { done: true }; } };
    const r = await accountSignOut({ demo: false }, storage, undefined, native as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual(['native:signOut', 'wipe:a']);
    expect(r).toEqual({ done: true, failed: '', message: '' });
  });
  test('S2 a native sign-out that fails wipes nothing shared', async () => {
    const order: string[] = [];
    const native = { available: true, later: async () => { throw new Error('boom'); } };
    const r = await accountSignOut({ demo: false }, undefined, undefined, native as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual([]); expect(r.done).toBe(false); expect(r.message).toContain('Sign-out did not finish');
  });
  test('S2 a native answer that says not done wipes nothing shared', async () => {
    const native = { available: true, later: async () => ({ done: false }) };
    const r = await accountSignOut({ demo: false }, undefined, undefined, native as never, { wipes: [{ name: 'a', run: async () => { throw new Error('must not run'); } }] });
    expect(r.done).toBe(false);
  });
  test('S2 no native module (the web build): the shared wipes run, there is no key to delete', async () => {
    const order: string[] = [];
    const r = await accountSignOut({ demo: false }, undefined, undefined, null, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual(['wipe:a']); expect(r.done).toBe(true);
  });
  test('S3 a throwing wipe does not stop the rest and is named', async () => {
    const order: string[] = [];
    const native = { available: true, later: async () => ({ done: true }) };
    const r = await accountSignOut({ demo: false }, undefined, undefined, native as never, { wipes: [
      { name: 'first', run: async () => { throw new Error('secret words of the listener'); } },
      { name: 'second', run: async () => { order.push('second'); } },
    ] });
    expect(order).toEqual(['second']); expect(r.done).toBe(true); expect(r.failed).toBe('first'); expect(JSON.stringify(r)).not.toContain('secret words');
  });
});

describe('the wipes', () => {
  test('wipeAll runs every registered wipe and returns the failed names', async () => {
    const ran: string[] = [];
    const failed = await wipeAll(undefined, undefined, [{ name: 'x', run: async () => { ran.push('x'); } }, { name: 'y', run: async () => { throw new Error('no'); } }]);
    expect(ran).toEqual(['x']); expect(failed).toEqual(['y']);
  });
  test('S4 every app:/data and app:/cache path a shared module keeps is covered', () => {
    const dir = new URL('.', import.meta.url).pathname;
    const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts') && !f.endsWith('.d.ts') && f !== 'account.ts' && f !== 'app.ts');
    const found = new Set<string>();
    for (const f of files) {
      const src = readFileSync(dir + f, 'utf8');
      for (const m of src.matchAll(/(?:const|let)\s+[A-Z_]+\s*(?::\s*string)?\s*=\s*'(app:\/(?:data|cache)\/[^']*)'/g)) found.add(m[1]);
    }
    expect(found.size).toBeGreaterThan(8);
    const unknown = [...found].filter((p) => !WIPED_PATHS.some((w) => p === w || p.startsWith(w + '/')));
    expect(unknown).toEqual([]);
  });
  test('S4 the standard wipes remove the files the modules keep (a stub storage sees the rm calls)', async () => {
    const { storage, removed } = memStorage([]);
    const failed = await wipeAll(storage, undefined, WIPES);
    expect(failed).toEqual([]);
    for (const p of ['app:/data/podcasts-library.json', 'app:/data/recent-files.json', 'app:/data/paper-front.json', 'app:/data/takes.json', 'app:/data/notebook-cache.md']) expect(removed).toContain(p);
  });
});
