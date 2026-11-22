// THE GATE (app.ts `answer`). Failure list, written first:
//  G1  a gated source is refused (AccountClosedError) while the key cannot be read (locked) and after the session ended; it never runs.
//  G2  a gated source is handed the storage of ITS world: what it writes lands under `accounts/<d>/`, and what another account's root holds is unreachable.
//  G3  every device source is handed the DEVICE handle, never the host's storage (I4): the theme sources read nothing under `accounts/` and send no signed request, with an account root on the disk.
//  G4  after a sign-out the card answers and the theme sources still answer (the ended card is drawn in the person's theme); a gated source refuses.
//  G5  a gated answer in flight when the session ends is refused (the page asks again).
//  G6  every gated source refuses with AccountClosedError in every state with no open world, and none runs; G7 the world-closed watcher leaves no waiter behind.
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import * as api from './api.ts';
import * as account from './account.ts';
import { answer, DEVICE, sources } from './app.ts';
import demoJSON from './fixtures/texture/theme-ember.json';
import { readFileSync } from 'node:fs';
import { dig, fakeStore, KEY_A, KEY_B, memFs, type Mem } from './account-phone.ts';

type Key = 'present' | 'absent' | 'unreadable';
let W: { forgetDone: boolean }; let m: Mem; let sent: string[]; let store: ReturnType<typeof fakeStore>;
const ROOT = (k: string): string => `app:/data/accounts/${dig(k)}`;
const nativeStub = (): never => ({ available: true, later: async (r: { op: string }) => (r.op === 'forgetAccountFiles' ? { done: W.forgetDone } : r.op === 'hello' ? { state: 'native' } : { ok: true }) }) as never;
function boot(files: Record<string, string>, key: Key = 'present'): void {
  W = { forgetDone: true }; store = fakeStore(key === 'absent' ? {} : { 'api-key': KEY_A, 'key-id': 'kid' });
  const f = { ...files }; if (key !== 'absent') f['app:/data/signed-in'] = '1'; if (key === 'unreadable') store.hide();
  m = memFs(f); sent = [];
  account.resetBoundary(); api.resetSeam(); api.bindNative(nativeStub());
  globalThis.fetch = (async (u: unknown) => { sent.push(String(u)); return new Response('{}', { status: 404 }); }) as never;
}
beforeEach(() => boot({}));
afterEach(() => { account.resetBoundary(); api.resetSeam(); });
const call = (source: string, args: unknown[] = []) => answer(source as never, args as never, store as never, m.storage as never, nativeStub());
const settle = (p: Promise<unknown>): Promise<unknown> => Promise.race([p.then(() => 'answered', () => 'refused'), new Promise((r) => setTimeout(() => r('never'), 40))]);

describe('G the gate', () => {
  test('G1 locked and ended refuse a gated source; it does not run', async () => {
    boot({ [`${ROOT(KEY_A)}/open-pages-shared.json`]: JSON.stringify({ pages: [{ address: 'mail:inbox-a', title: 'SECRET-OF-ACCOUNT-A' }] }) }, 'unreadable');
    await expect(call('pagesView', ['x', ''])).rejects.toMatchObject({ name: 'AccountClosedError', state: 'locked' });
    store.hide(false);
    expect(JSON.stringify(await call('pagesView', ['x', '']))).toContain('SECRET-OF-ACCOUNT-A');           // its own root, read through the handle
    await call('accountSignOut');
    await expect(call('pagesView', ['x', ''])).rejects.toMatchObject({ name: 'AccountClosedError', state: 'ended' });
  });
  test('G2 a gated source writes under its root and cannot reach another account\'s', async () => {
    boot({ [`${ROOT(KEY_B)}/open-pages-shared.json`]: JSON.stringify({ pages: [{ address: 'mail:inbox-b', title: 'SECRET-OF-ACCOUNT-B' }] }) });
    const v = JSON.stringify(await call('pagesOpen', ['paper:front', 'Front']));
    expect(v).not.toContain('SECRET-OF-ACCOUNT-B');
    expect(m.files.has(`${ROOT(KEY_A)}/open-pages-shared.json`)).toBe(true); expect(m.files.has('app:/data/open-pages-shared.json')).toBe(false);
    expect([...m.files.keys()].some((k) => k.includes(dig(KEY_B)))).toBe(false);                            // B's root was swept at launch: it was an orphan
  });
  test('G3 every device source gets the device handle: with an account root on the disk, no theme source reads it or signs a request', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'SECRET', [`${ROOT(KEY_A)}/theme-textures/texture-x.json`]: JSON.stringify({ schema_version: 1 }), 'app:/data/theme-choice.json': '{}' }, 'absent');
    const reads: string[] = []; const raw = m.storage as { fs: { readFile: (p: string) => Promise<Uint8Array> } }; const rf = raw.fs.readFile.bind(raw.fs); raw.fs.readFile = async (p: string) => { reads.push(p); return rf(p); };
    const DEV_ARGS: Record<string, unknown[]> = { theme: ['light', false, ''], themeList: ['light', false, ''], themeExtras: ['light', false, ''], themeCards: ['light', false, ''], themeSet: ['liquid-metal'], schemeSet: ['dark'], themeRemove: ['x'], themeMigrate: ['native'], hostCaps: [false, ''], aboutBuild: [0] };
    for (const src of Object.keys(DEVICE).filter((k) => k !== 'accountSignOut' && k !== 'accountEnding' && k !== 'accountState' && !k.startsWith('signIn'))) await call(src, DEV_ARGS[src] ?? []).catch(() => undefined);
    expect(reads.filter((p) => p.includes('/accounts/'))).toEqual([]); expect(sent.filter((u) => !u.endsWith('/health'))).toEqual([]);
    expect(reads.length).toBeGreaterThan(0);                                                               // the device paths were read, so the check above could have failed
    for (const k of Object.keys(DEVICE)) expect(Object.hasOwn(sources, k)).toBe(true);
  });
  test('G4 after a sign-out the card and the theme sources answer and a gated source refuses', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'x', 'app:/data/theme-choice.json': '{}' });
    await call('pagesView', ['x', '']); await call('accountSignOut');
    expect(await call('accountState', ['', 'native', 0])).toMatchObject({ state: 'ended' });
    await expect(call('theme', ['light', false, ''])).resolves.toBeDefined();
    await expect(call('paperFront', ['front', 400, 0, '', [], ''])).rejects.toMatchObject({ name: 'AccountClosedError' });
  });
  test('G5 an answer in flight when the session ends is refused (the page asks again)', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'x' });
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    m.faults.writeHold.set(`${ROOT(KEY_A)}/open-pages-shared.json`, gate);
    const p = call('pagesOpen', ['paper:front', 'Front']); await new Promise((r) => setTimeout(r, 10));
    await call('accountSignOut'); release();
    expect(await settle(p)).toBe('refused');
    await call('accountState', ['', 'native', 0]);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- P5: every source is driven through the REAL gate
// The tests of the modules pass the module a raw storage; the gate hands it a BOUND one (re-rooted). A source that only works with raw storage (the double bind of themeFromTexture) passed every module test and failed in the app.
// So every source of app.ts is driven here through `answer`, in an open account world, with the storage the gate gives it; the answer or the refusal must carry no path and no digest; and this list must be every source.
describe('P5 every source of app.ts is driven through the gate', () => {
  const ALL = Object.keys(sources).sort();
  const SPECIAL: Record<string, unknown[]> = { theme: ['light', false, ''], themeList: ['light', false, ''], themeExtras: ['light', false, ''], themeCards: ['light', false, ''], themeSet: ['liquid-metal'], schemeSet: ['dark'], themeRemove: ['nothing'], themeMigrate: ['native'], accountState: ['', 'native', 0], hostCaps: [false, ''], aboutBuild: [0] };
  test('the table names every source of app.ts (a new source fails here until it is driven)', () => {
    expect(ALL.length).toBeGreaterThan(90);
    expect(new Set(ALL).size).toBe(ALL.length); void readFileSync;
  });
  test('each source answers or fails inside, through answer(), in an OPEN account world; nothing it says holds a path or a digest; the boundary\'s own sources run last (they end the session)', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: JSON.stringify({ ids: [], handle: '', muted: [] }) });
    globalThis.fetch = (async () => new Response('{}', { status: 404 })) as never;
    const LAST = ['accountEnding', 'accountSignOut']; const order = [...ALL.filter((n) => !LAST.includes(n)), ...LAST];
    const driven: string[] = []; const leaks: string[] = []; const refused: string[] = [];
    for (const name of order) {
      const out = await Promise.race([call(name, SPECIAL[name] ?? []).then((v) => { driven.push(name); return JSON.stringify(v ?? null); }, (e: Error) => { driven.push(name); if (e?.name === 'AccountClosedError' && !LAST.includes(name)) refused.push(name); return String(e?.message ?? e); }), new Promise<string>((r) => setTimeout(() => r('slow'), 400))]);
      if (/accounts\/|[0-9a-f]{64}/.test(out) || /already under|is not a function|is not defined|Cannot read|TypeError|ReferenceError/.test(out)) leaks.push(`${name}: ${out.slice(0, 120)}`);      // a defect inside a source (a double bind, a crash) is not an answer
    }
    expect(leaks).toEqual([]); expect(refused).toEqual([]);                   // a refusal here means the source never ran
    expect(driven.sort()).toEqual([...ALL].sort());                           // every source answered (a slow one would be missing)
  });
  test('themeFromTexture works through the gate: it writes the theme into the account\'s root and shows it, and its error carries no path', async () => {
    boot({}); await call('pagesView', ['x', '']);
    globalThis.fetch = (async (u: unknown) => (String(u).includes('/api/compute/status') ? new Response(JSON.stringify({ computers: [{ computer_id: 'c1' }] })) : String(u).includes('/api/texture/theme') ? new Response(JSON.stringify(JSON.parse(JSON.stringify(demoJSON))), { status: 200 }) : new Response('{}', { status: 404 }))) as never;
    const ack = await call('themeFromTexture', ['d']) as { ok: boolean; message: string };
    expect(ack.message).toMatch(/^Added /); expect(ack.ok).toBe(true);
    expect([...m.files.keys()].some((k) => k.startsWith(`${ROOT(KEY_A)}/theme-textures/texture-`))).toBe(true);
    const list = JSON.stringify(await call('themeList', ['light', false, dig(KEY_A).slice(0, 16)])); expect(list).toContain('Ember');
    api.endWorld(); await expect(call('themeFromTexture', ['d'])).rejects.toMatchObject({ name: 'AccountClosedError' });
  });
});

describe('G7 the world-closed watcher', () => {
  test('N answers leave zero waiters on the world (one reaction per answer used to be left for the life of the process); a close refuses the ones still in flight and drains', async () => {
    boot({}); const w = (await account.enter(m.storage as never, store as never)).world; expect(w.watching).toBe(0);
    for (let i = 0; i < 40; i++) await call('pagesView', ['x', '']);
    expect(w.watching).toBe(0);
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; }); m.faults.writeHold.set(`${ROOT(KEY_A)}/open-pages-shared.json`, gate);
    const p = call('pagesOpen', ['paper:front', 'Front']); await new Promise((r) => setTimeout(r, 10)); expect(w.watching).toBe(1);
    await call('accountSignOut'); expect(w.watching).toBe(0); release(); expect(await settle(p)).toBe('refused');
  });
  test('a source never runs for an ended session (the gate\'s own pre-check of a world that closed between `enter` and the call has no seam: a microtask gap, named in the design note)', async () => {
    boot({}); const w = (await account.enter(m.storage as never, store as never)).world; let ran = false; const orig = (sources as Record<string, unknown>).pagesView;
    (sources as Record<string, unknown>).pagesView = () => { ran = true; return {}; };
    try { api.endWorld(); await expect(call('pagesView', ['x', ''])).rejects.toMatchObject({ name: 'AccountClosedError' }); expect(ran).toBe(false); void w; } finally { (sources as Record<string, unknown>).pagesView = orig; }
  });
});

describe('G6 every account-gated answer refuses in every state that has no open world', () => {
  // The states are the launch's (account.ts): starting (locked: the marker says an account was signed in and the secret does not read yet), blocked (native cannot forget its files) and ended (after a sign-out). `anonymous` and `account` are the two that open a world.
  const gatedNames = (): string[] => Object.keys(sources).filter((s) => !Object.hasOwn(DEVICE, s));
  const states: [string, () => void, string][] = [['notready', () => { store.hide(); }, 'locked'], ['blocked (native cannot forget its files)', () => { W.forgetDone = false; m.files.set(account.ENDING, new TextEncoder().encode('ending')); }, 'ended']];
  for (const [name, set, state] of states) {
    test(`${name}: every gated source refuses with AccountClosedError and none runs`, async () => {
      boot({ [`${ROOT(KEY_A)}/takes.json`]: 'secret' }); set(); const sent0 = sent.length;
      for (const s of gatedNames()) await expect(answer(s as never, [] as never, store as never, m.storage as never, nativeStub())).rejects.toMatchObject({ name: 'AccountClosedError', state });
      expect(sent.slice(sent0).filter((u) => !u.includes('/auth/api-keys/'))).toEqual([]); expect(m.files.get(`${ROOT(KEY_A)}/takes.json`)).toBeDefined();      // nothing was sent (a launch finishing a pending sign-out revokes the key: the one request it may make) and the account's file is untouched
    });
  }
  test('ended (after a sign-out): every gated source refuses', async () => {
    boot({}); await call('accountSignOut');
    for (const s of gatedNames()) await expect(answer(s as never, [] as never, store as never, m.storage as never, nativeStub())).rejects.toMatchObject({ name: 'AccountClosedError', state: 'ended' });
  });
  test('anonymous binds a gated source to the anonymous root only, account to its own root only', async () => {
    boot({}, 'absent'); const a = await account.enter(m.storage as never, store as never); expect(a.world.root).toBe('anonymous');
    boot({}, 'present'); const b = await account.enter(m.storage as never, store as never); expect(b.world.digest).toBe(dig(KEY_A));
  });
});

describe('B the bake and the hosts that are not phones', () => {
  // The build's bake evaluates constant data into the plan, with no native module and no account (a failed bake: `preqd`: the account boundary is closed (locked)). Failure list:
  //  B1 an answer that depends on an account reads Exact's host fact (exactHost()), which marks it NOT BAKED; the device sources do not (they may be baked)
  //  B2 a host that is not a phone whose module stub does not know `launchWipe` (answers another op's shape, nothing, or throws) has no launch wipe: it is `nomodule`, not a pending sign-out, and the boundary is not locked by it
  //  B3 on a phone (exactHost() native) a missing or unrecognised answer is NEVER "no sign-out pending": it is asked until the deadline and the launch stays LOCKED
  const stub = (later: () => Promise<unknown>): never => ({ available: true, later } as never);
  afterEach(() => { delete (globalThis as { exactHost?: unknown }).exactHost; });
  test('B1 a gated answer reads the host fact (not baked); a device source does not', async () => {
    let reads = 0; (globalThis as { exactHost?: () => string }).exactHost = () => { reads += 1; return 'web'; };
    await call('pagesView', ['x', '']); expect(reads).toBeGreaterThan(0);
    reads = 0; await call('themeList', []).catch(() => undefined); expect(reads).toBe(0);
  });
  test('B0 THE BAKE: exactHost() throws there, and the gate lets it throw for EVERY gated source (spine.contract `preqd`), so the bake leaves the resource unbaked; a device source is not asked', async () => {
    const bakeError = Object.assign(new Error('exactHost() is unavailable during bake'), { code: 'bake' });
    (globalThis as { exactHost?: () => never }).exactHost = () => { throw bakeError; };
    boot({}, 'unreadable');                                       // a bake has no key and no native answer: the boundary is closed
    await expect(call('pageRequested', [0, '', ''])).rejects.toBe(bakeError);       // not AccountClosedError, not an answer
    await expect(call('pagesView', ['x', ''])).rejects.toBe(bakeError);
    for (const s of Object.keys(sources)) { if (Object.hasOwn(DEVICE, s)) continue; await expect(answer(s as never, [] as never, store as never, m.storage as never, undefined as never)).rejects.toBe(bakeError); }
  });
});

const DRIVEN: string[] = [];
