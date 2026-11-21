// The page gate (app.ts `answer`) and the account card (account-state.ts), from docs/design/account-boundary-2026-11-21.md section 3. THE FAILURE LIST:
//  G1  every source not on the exempt list throws AccountClosedError in CLEARING, in ADOPT that cannot write, and in LOCKED (enumerated from `sources`: a new source is gated by default).
//  G2  every exempt source, called in CLEARING with the old account's data in the stores, reads no account store and makes no signed-in request; the exempt list is exactly the named
//      one, one reason each (a new exempt source is a deliberate edit of this test).
//  G3  the card's action per state; no "Sign in" outside EMPTY; "Clear this phone" for a refused key and after five tries; "Try again" for ADOPT.
//  G4  every gated resource of every contract takes the one tick (acctTick) as its last argument, so a page that refused is asked again when the boundary heals.
//  G5  an exempt source never waits on a running Clear.
//  HEAL  the refusing states heal without a relaunch: a failed Clear heals when the fault goes (the card's re-read starts the wipe), the epoch moves, LOCKED heals when the key reads again,
//        "Clear this phone" is the sign-out and works in every refusing state.
//  BANNER  the banner and Settings draw the button the card names, not only Sign in, and Settings does not hide it behind `au.signedIn`.
import { afterEach, describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import * as account from './account.ts';
import * as api from './api.ts';
import * as P from './pages.ts';
import { answer, sources, EXEMPT } from './app.ts';
import { accountState } from './account-state.ts';
import { dig, events, KEY_A, KEY_B, memFs, newWorld, pagesFile, PAGES, phone, SECRET, STAMP, type Mem, type World } from './account-phone.ts';

// The same helpers run against the code before the boundary existed (the failure list was seen failing there): a missing one is a no-op.
const resetBoundary = (): void => { const a = account as Record<string, unknown>; (a.resetBoundary as (() => void) | undefined)?.(); (a.resetSettled as (() => void) | undefined)?.(); };
const idle = async (): Promise<void> => { await (account as Record<string, unknown> as { idle?: () => Promise<void> }).idle?.(); };

let W: World; let n: ReturnType<typeof phone>; let m: Mem;
const SEED: Record<string, string> = { [PAGES]: pagesFile(), 'app:/data/takes.json': 'x', 'app:/data/podcasts-library.json': 'x', 'app:/data/mail/msg-1.bin': 'x' };
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
function boot(rec: 'none' | 'A', key: 'absent' | 'A' | 'B' | 'unreadable', seed: Record<string, string> = SEED): void {
  events.length = 0;
  W = key === 'absent' ? newWorld('absent') : key === 'unreadable' ? newWorld('unreadable') : newWorld('present', key === 'A' ? KEY_A : KEY_B);
  m = memFs({ ...seed, ...(rec === 'A' ? { [STAMP]: dig(KEY_A) } : {}) });
  n = phone(() => W);
  resetBoundary(); api.resetSeam(); P.forget(); api.bindNative(n as never);
}
afterEach(() => { resetBoundary(); api.resetSeam(); });
const call = (source: string, args: unknown[] = []) => answer(source as never, args as never, undefined as never, m.storage, n as never);
const caught = async (source: string, args: unknown[] = []): Promise<string> => { try { await call(source, args); return 'answered'; } catch (e) { return (e as Error).name; } };
const card = async (tries = 0, top = '') => (await accountState('t', top, 'native', tries, m.storage, undefined)) as { state: string; message: string; action: string; alsoClear: boolean; epoch: string };

// Each refusing state, with the native wipe hung where the state must stay CLEARING.
const refusing: [string, () => void][] = [
  ['CLEARING', () => { boot('A', 'absent'); W.wipe = 'hang'; }],
  ['ADOPT that cannot write', () => { boot('none', 'A'); m.faults.write.add(STAMP); }],
  ['LOCKED', () => { boot('A', 'unreadable'); }],
];

describe('G1 every source not exempt is gated', () => {
  const gated = Object.keys(sources).filter((s) => !(s in EXEMPT));
  test('the gated list is most of the sources', () => { expect(gated.length).toBeGreaterThan(40); });
  for (const [name, setup] of refusing) {
    test(`all ${'gated'} sources throw AccountClosedError in ${name}`, async () => {
      setup();
      for (const s of gated) expect([s, await caught(s)]).toEqual([s, 'AccountClosedError']);
      expect(m.files.has(PAGES)).toBe(true);                                                       // a refusal reads and writes nothing
    });
  }
  test('a source not named anywhere is gated too (the gate is default-refuse: it is a list of what MAY answer)', async () => {
    refusing[0][1]();
    expect(await caught('aSourceThatDoesNotExist')).toBe('AccountClosedError');
  });
  test('the gate waits for no wipe: a refusal returns while the native wipe is still hung', async () => {
    refusing[0][1]();
    const t = await Promise.race([caught('pagesView', [0]), wait(500).then(() => 'waited')]);
    expect(t).toBe('AccountClosedError');
  });
});

describe('G2 the exempt list, and what an exempt source reaches', () => {
  test('the exempt list is exactly the named sources, each with a reason', () => {
    expect(Object.keys(EXEMPT).sort()).toEqual(['aboutBuild', 'accountSignOut', 'accountState', 'hostCaps', 'schemeSet', 'theme', 'themeCards', 'themeExtras', 'themeList', 'themeMigrate', 'themeRemove', 'themeSet'].sort());
    for (const [s, why] of Object.entries(EXEMPT)) expect([s, typeof why === 'string' && why.length > 15]).toEqual([s, true]);
    expect('themeFromTexture' in EXEMPT).toBe(false);                                              // it fetches a Texture document with the key
    for (const s of ['barInfo', 'radioPage', 'conversationInfo']) expect(s in EXEMPT).toBe(false);   // their arguments hold the account's words
  });
  test('in CLEARING with A\'s data in the stores and in the arguments: no answer carries it, no account store is read, no signed-in request is made', async () => {
    refusing[0][1]();
    const fetched: string[] = []; const reads: string[] = [];
    globalThis.fetch = (async (_u: string, init: RequestInit) => { fetched.push(JSON.stringify(init?.headers ?? {})); return new Response('{}', { status: 200 }); }) as never;
    const fs = (m.storage as unknown as { fs: { readFile: (p: string) => Promise<Uint8Array> } }).fs; const read = fs.readFile;
    fs.readFile = async (p: string) => { reads.push(p); return read(p); };
    const args: Record<string, unknown[]> = {
      accountState: ['t', '', 'native', 0], hostCaps: [true, 0], aboutBuild: [0], theme: ['light', false], themeList: ['light', false], themeExtras: ['light', false],
      themeCards: ['light', false], themeMigrate: ['native'], themeSet: ['chrome-pastel'], schemeSet: ['light'], themeRemove: [SECRET],
    };
    for (const s of Object.keys(EXEMPT).filter((x) => x !== 'accountSignOut')) {
      const out = await call(s, args[s] ?? []).catch((e) => ({ error: (e as Error).name }));
      expect([s, JSON.stringify(out).includes(SECRET) && s !== 'themeRemove']).toEqual([s, false]);
    }
    expect(fetched.filter((h) => h.toLowerCase().includes('authorization'))).toEqual([]);
    const accountPaths = account.wipedPaths();
    expect(reads.filter((p) => accountPaths.some((w) => p === w || p.startsWith(w + '/')))).toEqual([]);
  });
  test('G5 an exempt source answers while a Clear is running (the card, the host facts)', async () => {
    refusing[0][1]();
    await caught('pagesView', [0]);                                                               // starts the Clear, which hangs in native
    const r = await Promise.race([call('accountState', ['t', '', 'native', 0]).then((v) => v as { state: string }), wait(500).then(() => ({ state: 'waited' }))]);
    expect(r.state).toBe('clearing');
    expect(await Promise.race([call('hostCaps', [true, 0]), wait(500).then(() => 'waited')])).not.toBe('waited');
  });
});

describe('G3 the card: what it says and which button, per state', () => {
  test('EMPTY: signed out, Sign in', async () => {
    boot('none', 'absent', {}); const c = await card();
    expect(c).toMatchObject({ state: 'signedOut', action: 'signIn' }); expect(c.message).toContain('signed out');
    expect((await card(0, 'radio:')).message).toContain('public paper');
  });
  test('OPEN: nothing to say, no button', async () => { boot('A', 'A'); expect(await card()).toMatchObject({ state: 'signedIn', message: '', action: '' }); });
  test('a refused key (a 401) says so and offers Clear this phone, not Sign in', async () => {
    boot('A', 'A');
    globalThis.fetch = (async (u: string) => (String(u).endsWith('/api/compute/status') ? new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 }) : new Response('{}', { status: 401 }))) as never;
    await api.authedJSON('/api/x').catch(() => undefined);
    expect(await card()).toMatchObject({ state: 'rejected', action: 'clear' });
  });
  test('CLEARING: says so, no button at first; after five tries Try again and Clear this phone, and what failed', async () => {
    boot('A', 'absent'); m.faults.rmSilently.add('app:/data/takes.json'); await call('pagesView', [0]).catch(() => undefined); await idle();
    const c0 = await card(0); expect(c0).toMatchObject({ state: 'clearing', action: '', alsoClear: false }); expect(c0.message).toContain('Clearing');
    const c5 = await card(5); expect(c5).toMatchObject({ state: 'clearing', action: 'retry', alsoClear: true }); expect(c5.message).toContain('takes.json'); expect(c5.message).toContain('reinstall');
  });
  test('LOCKED: "Unlock your phone", after five tries Try again and Clear this phone', async () => {
    boot('A', 'unreadable');
    expect(await card(0)).toMatchObject({ state: 'locked', action: '', alsoClear: false });
    expect(await card(5)).toMatchObject({ state: 'locked', action: 'retry', alsoClear: true });
    expect((await card(0)).message).toBe('Unlock your phone to continue.');
  });
  test('ADOPT that cannot write: "could not record the account", Try again', async () => {
    boot('none', 'A'); m.faults.write.add(STAMP);
    expect(await card()).toMatchObject({ state: 'adopt', action: 'retry' });
  });
  test('Sign in is offered only in EMPTY (every other cell, every tries count)', async () => {
    const cells: ['none' | 'A', 'absent' | 'A' | 'B' | 'unreadable', string][] = [['none', 'absent', 'signIn'], ['none', 'A', ''], ['none', 'unreadable', ''], ['A', 'absent', ''], ['A', 'A', ''], ['A', 'B', ''], ['A', 'unreadable', '']];
    for (const [rec, key, signIn] of cells) for (const tries of [0, 5]) {
      boot(rec, key, rec === 'none' && key === 'absent' ? {} : SEED); if (key === 'absent' && rec === 'A') W.wipe = 'hang';
      const c = await card(tries); expect([rec, key, tries, c.action === 'signIn' ? 'signIn' : '']).toEqual([rec, key, tries, signIn]);
    }
  });
  test('the epoch is stable while nothing changes and moves when a wipe completes', async () => {
    boot('A', 'B'); W.wipe = 'hang';
    const a = await card(); const a2 = await card(); expect(a2.epoch).toBe(a.epoch);
    W.wipe = 'ok'; W.release?.(); await idle();
    const b = await card(); expect(b.epoch).not.toBe(a.epoch); expect(b.state).toBe('signedIn');
  });
});

describe('HEAL: the refusing states heal without a relaunch', () => {
  test('a failed Clear heals when the fault goes: the card\'s re-read starts the wipe, the pages read again', async () => {
    boot('A', 'B'); m.faults.rmSilently.add('app:/data/takes.json');
    expect(await caught('pagesView', [0])).toBe('AccountClosedError'); await idle();
    expect((await card()).state).toBe('clearing');
    const e0 = (await card()).epoch;
    m.faults.rmSilently.clear();
    await card(); await idle();                                                            // the 3 s re-read: it starts a Clear because none runs
    const c = await card(); expect(c.state).toBe('signedIn'); expect(c.epoch).not.toBe(e0);
    expect(await caught('pagesView', [0])).toBe('answered');
  });
  test('ADOPT retried by the card heals once the write can land', async () => {
    boot('none', 'A'); m.faults.write.add(STAMP);
    expect((await card()).state).toBe('adopt'); m.faults.write.clear();
    await card(); await idle(); expect((await card()).state).toBe('signedIn'); expect(m.text(STAMP)).toBe(dig(KEY_A));
  });
  test('LOCKED heals when the key reads again', async () => {
    boot('A', 'unreadable'); expect((await card()).state).toBe('locked');
    W.key = 'present'; W.value = KEY_A; expect((await card()).state).toBe('signedIn'); expect(await caught('pagesView', [0])).toBe('answered');
  });
  test('Clear this phone is the sign-out: it works in every refusing state and never waits on the gate', async () => {
    for (const [name, setup] of refusing) {
      setup(); W.wipe = 'ok';
      const r = await Promise.race([call('accountSignOut') as Promise<{ done: boolean }>, wait(1000).then(() => 'waited')]);
      expect([name, r === 'waited']).toEqual([name, false]);
      expect(W.log).toContain('signOut');
    }
    boot('A', 'B'); const r = (await call('accountSignOut')) as { done: boolean };                  // a refused key: the key goes, the old account's data goes
    expect(r.done).toBe(true); expect(W.key).toBe('absent'); expect([...m.files.keys()]).toEqual([]);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------- the contracts

const dir = new URL('.', import.meta.url).pathname;
const contracts = readdirSync(dir).filter((f) => f.endsWith('.contract') && !f.endsWith('.test.contract'));
/** The top-level arguments of a call's parenthesis text. */
function splitArgs(s: string): string[] {
  const out: string[] = []; let depth = 0; let cur = ''; let q = '';
  for (const ch of s) {
    if (q) { cur += ch; if (ch === q) q = ''; continue; }
    if (ch === '"' || ch === '`') { q = ch; cur += ch; continue; }
    if ('([{'.includes(ch)) depth++; if (')]}'.includes(ch)) depth--;
    if (ch === ',' && depth === 0) { out.push(cur.trim()); cur = ''; } else cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}
describe('G4 every gated resource is asked again when the boundary heals', () => {
  const gatedNames = Object.keys(sources).filter((s) => !(s in EXEMPT));
  const found: { file: string; res: string; source: string; last: string }[] = [];
  for (const f of contracts) for (const line of readFileSync(dir + f, 'utf8').split('\n')) {
    const mm = /^\s*resource (\w+) = (\w+)\((.*)\) as shape/.exec(line);
    if (mm && gatedNames.includes(mm[2])) found.push({ file: f, res: mm[1], source: mm[2], last: splitArgs(mm[3]).pop() ?? '' });
  }
  test('the resources of the gated sources are found (the parse is not empty)', () => { expect(found.length).toBeGreaterThan(15); });
  test('each takes acctTick as its last argument', () => {
    expect(found.filter((r) => !r.last.includes('acctTick')).map((r) => `${r.file} ${r.res}`)).toEqual([]);
  });
  test('acctTick is one derive from the card\'s epoch; signedOutTick is gone', () => {
    const all = contracts.map((f) => readFileSync(dir + f, 'utf8')).join('\n');
    expect(all).toMatch(/derive acctTick = acs\.epoch/); expect(all).not.toContain('signedOutTick');
    expect(all).not.toMatch(/state acctTick/);
  });
  test('the card\'s re-read covers clearing and adopt, bounded to five tries, and "Try again" restarts the count', () => {
    const c = readFileSync(dir + 'account.contract', 'utf8');
    expect(c).toMatch(/task rereadBoundary when .*acs\.state == "clearing".*acs\.state == "adopt".* and .* < 5/s);
    expect(c).toMatch(/action retryAccount[\s\S]*= 0/);
  });
});

describe('BANNER the buttons reach every page', () => {
  test('the banner injects signOut and retryAccount and draws the button the card names', () => {
    const b = readFileSync(dir + 'signedout.contract', 'utf8');
    for (const w of ['signOut: action', 'retryAccount: action']) expect(b).toContain(w);
    expect(b).toContain('acs.action == "clear"'); expect(b).toContain('acs.action == "retry"'); expect(b).toContain('acs.alsoClear');
  });
  test('Settings draws Sign in, Try again and Clear this phone from the card, not from au.signedIn', () => {
    const s = readFileSync(dir + 'settings.contract', 'utf8');
    const i = s.indexOf('settings-account-refresh'); const tail = s.slice(i, s.indexOf('// ---', i));
    expect(tail).not.toMatch(/^\s*if au\.signedIn\s*$/m);
    expect(tail).toContain('acs.action == "clear"'); expect(tail).toContain('acs.action == "retry"'); expect(tail).toContain('Clear this phone');
  });
  test('the provided members carry retryAccount to the components', () => {
    expect(readFileSync(dir + 'app.contract', 'utf8')).toContain('retryAccount');
  });
});
