// Test scaffolding for the account boundary (account.test.ts, app-gate.test.ts): a real in-memory file system with the error codes Exact's storage gives
// (`error.code`, reference.md "storage refusal": 'ENOENT', 'EACCES', 'EBUSY', 'failed'), faults switchable per path, and the app module as the model of the phone:
// the Keychain item (three answers), the Swift stores, and the native sign-out and wipe with their own faults. Test-only; nothing in the app imports it.
import { createHash } from 'node:crypto';

export const dig = (k: string): string => createHash('sha256').update(k).digest('hex');
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
  hangRm: Set<string>;                   // rm never answers (a store the system will not release)
  hangRead: Set<string>;                 // readFile never answers
  hold: Map<string, Promise<void>>;      // readFile takes the bytes, THEN waits (a read that began before a wipe and ends after it)
  noStorage: boolean;
};

/** A real in-memory file system: files and folders, rm removes a folder's children, readdir lists them. */
export function memFs(seed: Record<string, string> | string[] = []) {
  const files = new Map<string, Uint8Array>();
  const enc = new TextEncoder();
  if (Array.isArray(seed)) for (const p of seed) files.set(p, enc.encode('x')); else for (const [p, v] of Object.entries(seed)) files.set(p, enc.encode(v));
  const faults: Faults = { rm: new Set(), rmSilently: new Set(), write: new Set(), read: new Map(), stat: new Map(), hangRm: new Set(), hangRead: new Set(), hold: new Map(), noStorage: false };
  const writes: string[] = [];
  const spy = { stampOverwrites: 0 };
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { if (faults.write.has(p)) throw new FsError('full'); if (p === STAMP && files.has(p)) spy.stampOverwrites += 1; events.push('write:' + p); writes.push(p + '=' + new TextDecoder().decode(b)); files.set(p, b); },
    readFile: async (p: string) => {
      if (faults.hangRead.has(p)) await new Promise(() => undefined);
      const c = faults.read.get(p); if (c) throw new FsError(c);
      const b = files.get(p); if (!b) throw new FsError('ENOENT');
      const h = faults.hold.get(p); if (h) await h;
      return b;
    },
    stat: async (p: string) => {
      const c = faults.stat.get(p); if (c) throw new FsError(c);
      if (files.has(p) || kids(p).length) return { size: 1, modifiedMs: 0 }; throw new FsError('ENOENT');
    },
    rm: async (p: string) => {
      events.push('rm:' + p);
      if (faults.hangRm.has(p)) await new Promise(() => undefined);
      if (faults.rm.has(p)) throw new FsError('EACCES');
      if (faults.rmSilently.has(p)) return;
      files.delete(p); for (const k of kids(p)) files.delete(k);
    },
    mkdir: async () => undefined,
    readdir: async (p: string) => [...new Set(kids(p).map((k) => k.slice(p.length + 1).split('/')[0]))],
  };
  const storage = { fs, sqlite: {} } as never;
  return { files, faults, writes, spy, storage: storage as never, get text() { return (p: string): string | undefined => { const b = files.get(p); return b ? new TextDecoder().decode(b) : undefined; }; } };
}
export type Mem = ReturnType<typeof memFs>;

export type NativeMode = 'ok' | 'hang' | 'throw' | 'throwLater' | 'notdone' | 'killAfterKeyDelete';
export type World = {
  key: 'present' | 'absent' | 'unreadable'; value: string;
  swift: boolean;                              // the Swift stores still hold the old account's data
  log: string[];                               // every native op, with the generation it carried
  gens: (number | undefined)[];
  signOut: NativeMode; wipe: NativeMode;
  keyDeleteFails: boolean;                     // the Keychain delete failed and the native side answers not done (ChoirSignIn.swift:128)
  sessionFails: boolean;                       // keystate says present, the session read finds nothing
  keystateHangs: boolean;
  release: (() => void) | null;                // lets a hung native call go on
  wipeFailed: string[];                        // what the native wipe reports as failed (landing 2)
  inflight: number; maxInflight: number;       // native sign-out and wipe calls running at once
};
export const newWorld = (key: World['key'] = 'present', value = KEY_A): World => ({
  key, value, swift: true, log: [], gens: [], signOut: 'ok', wipe: 'ok', keyDeleteFails: false, sessionFails: false, keystateHangs: false, release: null, wipeFailed: [], inflight: 0, maxInflight: 0,
});

/** The app module as the model of the phone. `w` is read at each call, so a test changes the world while the app runs. */
export function phone(w: () => World) {
  const gate = async (mode: NativeMode, W: World): Promise<'go' | { done: boolean }> => {
    if (mode === 'hang') await new Promise<void>((res) => { W.release = res; });
    if (mode === 'throw') throw new Error('native threw');
    if (mode === 'notdone') return { done: false };
    return 'go';
  };
  return {
    available: true,
    later: async (r: { op: string; generation?: number }): Promise<unknown> => {
      const W = w();
      if (r.op === 'keystate') {
        if (W.keystateHangs) await new Promise(() => undefined);
        return { state: W.key };
      }
      if (r.op === 'session') return W.key === 'present' && !W.sessionFails ? { found: true, value: W.value, computerId: '' } : { found: false };
      if (r.op === 'signOut') {
        W.log.push('signOut'); W.gens.push(r.generation); events.push('native:signOut'); W.inflight += 1; W.maxInflight = Math.max(W.maxInflight, W.inflight);
        const g = await gate(W.signOut, W).finally(() => { W.inflight -= 1; }); if (g !== 'go') return g;
        if (W.keyDeleteFails) return { done: false };
        W.key = 'absent';                                              // ChoirSignIn.swift:128, the key goes first
        if (W.signOut === 'killAfterKeyDelete') throw new Error('killed between the key delete and the Swift wipe');
        W.swift = false;                                               // :129, AccountWipe
        if (W.signOut === 'throwLater') throw new Error('threw after the key delete');
        return { done: true };
      }
      if (r.op === 'wipe') {
        W.log.push('wipe'); W.gens.push(r.generation); events.push('native:wipe'); W.inflight += 1; W.maxInflight = Math.max(W.maxInflight, W.inflight);
        const g = await gate(W.wipe, W).finally(() => { W.inflight -= 1; }); if (g !== 'go') return g;
        W.swift = false;
        return W.wipeFailed.length ? { done: true, failed: W.wipeFailed } : { done: true };
      }
      return { ok: true };
    },
  };
}
