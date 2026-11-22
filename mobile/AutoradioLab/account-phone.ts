// Test scaffolding for the account boundary (account.test.ts, app-gate.test.ts): a real in-memory file system with the error codes Exact's storage gives
// (`error.code`, reference.md "storage refusal": 'ENOENT', 'EACCES', 'EBUSY', 'failed'), faults switchable per path, and the app module as the model of the phone:
// and a fake of Exact's secret store (`fakeStore`). Test-only; nothing in the app imports it.
import { createHash } from 'node:crypto';
import { openWorld, resetWorld, currentWorld, holdSession, type World as ApiWorld } from './api.ts';

export const dig = (k: string): string => createHash('sha256').update(k).digest('hex');
/** A process that has just signed in as the account whose key has digest `d`: a fresh world for it (tests only). */
export const reopen = (d: string, key?: string, computerId = ''): ApiWorld => { resetWorld(); const w = openWorld(d); if (key) holdSession(key, '', computerId); return w; };
/** A test's "the account moved" for the generation guards some modules still keep: the same key in a NEW world, the generation moved. (The real app never does this: a sign-out ends the session.) */
export const moveAccount = (): ApiWorld => { const d = currentWorld().digest; resetWorld(); return d ? openWorld(d) : currentWorld(); };
export const KEY_A = 'test-key-A-not-real';
export const KEY_B = 'test-key-B-not-real';
export const STAMP = 'app:/data/account-stamp';
export const PAGES = 'app:/data/open-pages-shared.json';
/** What a page would show of account A: a title seeded in the open-pages file. A gated source that returns it read A's store. */
export const SECRET = 'SECRET-OF-ACCOUNT-A';
export const pagesFile = (title = SECRET): string => JSON.stringify({ pages: [{ address: 'mail:inbox-a', title }], recency: ['mail:inbox-a'] });

/** Every file and native event in order (a test reads the order of a wipe). */
export const events: string[] = [];
export class FsError extends Error { code: string; constructor(code: string) { super(code); this.code = code; } }

export type Faults = {
  rm: Set<string>;                       // rm rejects with EACCES
  rmSilently: Set<string>;               // rm resolves and the file stays (a store that swallows removal errors)
  write: Set<string>;                    // atomicWriteFile rejects with 'full'
  read: Map<string, string>;             // readFile rejects with this code
  stat: Map<string, string>;             // stat rejects with this code
  hangRm: Set<string>;                   // rm does not answer until `releaseRm()` (a store the system will not release yet)
  releaseRm: () => void;
  reappear: Set<string>;                // the first rm of this folder finds a child that was just written (a late write) and refuses with ENOTEMPTY
  strictDirs: boolean;                  // rm of a folder that still has children refuses (ENOTEMPTY), as a host that is not recursive would
  rmGate: Map<string, Promise<void>>;   // the first rm of this path waits for the promise (one-shot)
  writeHold: Map<string, Promise<void>>; // atomicWriteFile of this path waits for the promise, then lands (a write in flight)
  hangRead: Set<string>;                 // readFile never answers
  holdOnce: Map<string, Promise<void>>;  // like hold, for the FIRST read of the path only
  hold: Map<string, Promise<void>>;      // readFile takes the bytes, THEN waits (a read that began before a wipe and ends after it)
  noStorage: boolean;
};

/** A real in-memory file system: files and folders, rm removes a folder's children, readdir lists them. */
export function memFs(seed: Record<string, string> | string[] = []) {
  const files = new Map<string, Uint8Array>();
  const enc = new TextEncoder();
  if (Array.isArray(seed)) for (const p of seed) files.set(p, enc.encode('x')); else for (const [p, v] of Object.entries(seed)) files.set(p, enc.encode(v));
  const faults: Faults = { rm: new Set(), rmSilently: new Set(), write: new Set(), read: new Map(), stat: new Map(), hangRm: new Set(), reappear: new Set(), strictDirs: false, rmGate: new Map(), writeHold: new Map(), releaseRm: () => { for (const r of held.splice(0)) r(); }, hangRead: new Set(), hold: new Map(), holdOnce: new Map(), noStorage: false };
  const writes: string[] = [];
  const held: (() => void)[] = [];
  const spy = { stampOverwrites: 0 };
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { events.push('writecall:' + p); { const h = faults.writeHold.get(p); if (h) await h; } if (faults.write.has(p)) throw new FsError('full'); if (p === STAMP && files.has(p)) spy.stampOverwrites += 1; events.push('write:' + p); writes.push(p + '=' + new TextDecoder().decode(b)); files.set(p, b); },
    readFile: async (p: string) => {
      if (faults.hangRead.has(p)) await new Promise(() => undefined);
      const c = faults.read.get(p); if (c) throw new FsError(c);
      const b = files.get(p); if (!b) throw new FsError('ENOENT');
      const h = faults.hold.get(p); if (h) await h;
      const h1 = faults.holdOnce.get(p); if (h1) { faults.holdOnce.delete(p); await h1; }
      return b;
    },
    stat: async (p: string) => {
      const c = faults.stat.get(p); if (c) throw new FsError(c);
      if (files.has(p) || kids(p).length) return { size: 1, modifiedMs: 0 }; throw new FsError('ENOENT');
    },
    rm: async (p: string) => {
      events.push('rm:' + p);
      { const g = faults.rmGate.get(p); if (g) { faults.rmGate.delete(p); await g; } }
      if (faults.hangRm.has(p)) await new Promise<void>((r) => { held.push(r); });
      if (faults.rm.has(p)) throw new FsError('EACCES');
      if (faults.rmSilently.has(p)) return;
      if (faults.reappear.has(p)) { faults.reappear.delete(p); files.set(p + '/late.bin', enc.encode('late')); throw new FsError('ENOTEMPTY'); }
      if (faults.strictDirs && kids(p).length) throw new FsError('ENOTEMPTY');
      files.delete(p); for (const k of kids(p)) files.delete(k);
    },
    mkdir: async () => undefined,
    readdir: async (p: string) => [...new Set(kids(p).map((k) => k.slice(p.length + 1).split('/')[0]))],
  };
  const storage = { fs, sqlite: {} } as never;
  return { files, faults, writes, spy, storage: storage as never, get text() { return (p: string): string | undefined => { const b = files.get(p); return b ? new TextDecoder().decode(b) : undefined; }; } };
}
export type Mem = ReturnType<typeof memFs>;

/** A fake of Exact's secret store (`secret.keep`): what an answer is handed as `store`. `hide()` makes every read answer null while the value stays kept (a Keychain the host could not read yet). */
export function fakeStore(init: Record<string, string> = {}) {
  const kept = new Map(Object.entries(init)); let hidden = false;
  return { kept, hide: (on = true): void => { hidden = on; }, get: (n: string): string | null => (hidden ? null : kept.get(n) ?? null), set: (n: string, v: string): void => { kept.set(n, v); }, forget: (n: string): void => { kept.delete(n); }, has: (n: string): boolean => kept.has(n) };
}
