// Work shared between Exact answers (docs/problems/shared-settle-promise-fails-every-request-2026-11-25.md), under the model of testkit/exact-answers.ts (R1 to R4, each the pinned prelude's).
// Failure list, written first:
//  F1 the answer that asks first is let go before the launch's settle finishes: later answers still answer.
//  F2 a sign-out the last launch left unfinished is finished at this launch, whoever asked first and was let go.
//  F3 a settle that rejects is not kept: the next settle reconciles again.
//  F4 with a sign-out to finish, every long timer of the settle (the native deadline, the generation announcement) is armed by the settle's own answer, never by an answer that asked first.
//  W1 an answer waiting for the settle neither fails ("pending on nothing") nor answers on a partial run when the settle's owner is let go in the middle of its native wipe (A2); the settle asked again finishes it.
//  A1 hostMode: the answer that started a probe is let go while the probe sleeps (its timers dropped): another answer still gets a decision.
//  N1 the notebook page: an answer let go in the middle of the read does not hand its refused read to the next answer with the same key.
//  M1 mail's cipher probe: a probe refused because its answer was let go is not kept as "no cipher" for the session.
//  B1 the boot announcement refused because its answer was let go is sent again by the next native call.
//  C1 (control) two answers at launch: one reconcile, the native sign-out once.
//  K1 a key read that THREW (a bridge error, not a locked key) is not published as the launch's answer: the settle fails, the waiting answer keeps waiting, the settle asked again publishes.
//  K2 a key read that throws every time: after KEY_ERROR_RUNS settles "nothing" is published (logged), so the app is not held forever.
//  D1 a slow settle keeps every other source waiting (no deadline: a source that ran before the settle could read or write the previous account's data); they answer once it publishes, after its wipe.
//  S1 a run superseded or reset while it ran (a sign-in) does not answer as a success: it fails, so the contract asks again; the next run publishes.
//  K3 the phone's key read is the secret store's: a store.get that throws (not InvalidAccessError) is a key-read error, not published; InvalidAccessError is "locked", published as "nothing".
//  D2 the contract always asks the settle: a root resource with constant arguments, read by the page set (spine.contract pgs) and the account card.
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { createHash } from 'node:crypto';
import { answer } from './app.ts';
import { resetSettled, settleAccount, KEY_ERROR_RUNS, PENDING_FILE, STAMP_FILE } from './account.ts';
import { readFileSync } from 'node:fs';
import { resetDemoLeftovers } from './demo-leftovers.ts';
import { hostMode, resetHost } from './host.ts';
import { adoptNative, nativeLater, resetSeam } from './api.ts';
import { notebookShared, forgetNotebook } from './notebook-run.ts';
import { forget as steerForget } from './steer.ts';
import { canSeal, forgetProbe } from './mail-seal.ts';
import { resetRadio } from './radio-bridge.ts';
import { armedBy, ask, current, hostOp, install, letGo, newCall, tick, uninstall } from './testkit/exact-answers.ts';

// A fresh process each test (resetRadio: a radio another test file left, with ops pending, would hold the sign-out's wipe for its 3 s).
beforeEach(() => { install(); resetRadio(); resetSeam(); resetSettled(); resetHost(); resetDemoLeftovers(); forgetNotebook(); steerForget(); forgetProbe(); });
afterEach(() => { uninstall(); });

type World = { key: 'present' | 'absent'; ops: string[]; hold: Set<string>; open: () => void; gate: Promise<void>; keyThrows?: number };
const KEY = 'test-key-A-not-real';
function world(key: World['key'], hold: string[] = []): World {
  let open!: () => void; const gate = new Promise<void>((r) => { open = r; });
  return { key, ops: [], hold: new Set(hold), open, gate };
}
/** The app module: every op a host operation of the answer that makes it (R3). An op named in `hold` waits for `open()`, once. */
const phone = (w: World) => ({ available: true, later: (r: { op: string; boot?: boolean }) => { const c = current(); return hostOp(async () => {
  if (w.hold.has(r.op)) { w.hold.delete(r.op); await w.gate; if (c?.letGo) throw new Error('withdrawn: its answer was let go before it was sent'); }      // a held call is one not yet sent: letting its answer go withdraws it (prelude.js:1017)
  w.ops.push(r.op + (r.boot ? ':boot' : ''));
  if (r.op === 'keystate') { if (w.keyThrows && w.keyThrows > 0) { w.keyThrows -= 1; throw new Error('the Keychain bridge failed'); } return { state: w.key }; }
  if (r.op === 'signOut') { w.key = 'absent'; return { done: true }; }
  if (r.op === 'wipe') return { done: true };
  if (r.op === 'mailCan') return { ok: true };
  return { ok: true };
}); } });
/** The key as the app keeps it (an Exact secret): read through the store, as on the phone. */
const store = (w: World) => ({ get: (n: string) => (n === 'choir.key' && w.key === 'present' ? KEY : null), set: () => undefined, forget: () => undefined });

function memFs(seed: string[] = []) {
  const files = new Map<string, Uint8Array>();
  for (const p of seed) files.set(p, new TextEncoder().encode('x'));
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    stat: async (p: string) => { if (files.has(p) || kids(p).length) return { size: 1, modifiedMs: 0 }; throw new Error('ENOENT'); },
    rm: async (p: string) => { files.delete(p); for (const k of kids(p)) files.delete(k); },
    mkdir: async () => undefined,
    readdir: async (p: string) => { const k = kids(p); if (!k.length) throw new Error('ENOENT'); return [...new Set(k.map((x) => x.slice(p.length + 1).split('/')[0]))]; },
  };
  return { files, storage: { fs, sqlite: {} } as never };
}
const TAKES = 'app:/data/takes.json';
const ans = (id: string, source: string, w: World, m: ReturnType<typeof memFs>, args: unknown[] = ['x']) =>
  ask(newCall(id), () => answer(source as never, args as never, store(w) as never, m.storage, phone(w) as never));
const settle = (id: string, w: World, m: ReturnType<typeof memFs>) => ans(id, 'accountSettle', w, m, []);
const outcome = <T>(p: Promise<T>) => { const o: { done: boolean; error: string } = { done: false, error: '' }; p.then(() => { o.done = true; }, (e: Error) => { o.done = true; o.error = e.message; }); return o; };

describe('the launch settle', () => {
  test('F1 the answer that asks first is let go before the settle finishes: later answers still answer', async () => {
    const w = world('present'); const m = memFs();
    const a = newCall('A');
    void ask(a, () => answer('answerDismissShared' as never, ['x'] as never, store(w) as never, m.storage, phone(w) as never)).catch(() => undefined);
    await tick(1);
    letGo(a);
    const s = settle('S', w, m);
    const b = await ans('B', 'answerDismissShared', w, m) as { ok: boolean };
    expect(b.ok).toBe(true); await s;
  });

  test('F2 a sign-out left unfinished is finished at this launch, though the answer that asked first was let go', async () => {
    const w = world('absent'); const m = memFs([TAKES, PENDING_FILE, STAMP_FILE]);
    const a = newCall('A');
    const first = ask(a, () => answer('answerDismissShared' as never, ['x'] as never, store(w) as never, m.storage, phone(w) as never)).catch(() => undefined);
    letGo(a);
    const s = settle('S', w, m);
    const b = ans('B', 'answerDismissShared', w, m);
    await s; await b; await first;
    expect(w.ops).toContain('signOut');
    expect(m.files.has(TAKES)).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(false);
  });

  test('F3 a settle that rejects is not kept: the next settle reconciles again', async () => {
    const w = world('absent'); adoptNative(phone(w) as never, 't');
    const m = memFs([TAKES, PENDING_FILE]);
    let throws = true;
    const flaky = { signOut: async () => { if (throws) { throws = false; throw new Error('a bridge that threw once'); } return true; }, wipe: async () => true };
    await expect(ask(newCall('S1'), () => settleAccount(m.storage, undefined, flaky))).rejects.toThrow('threw once');
    const r = await ask(newCall('S2'), () => settleAccount(m.storage, undefined, flaky));
    expect(r.action).toBe('finish'); expect(m.files.has(PENDING_FILE)).toBe(false);
  });

  test('F4 with a sign-out to finish, the long timers of the settle are its own answer\'s, never those of an answer that asked first', async () => {
    const w = world('absent'); const m = memFs([TAKES, PENDING_FILE, STAMP_FILE]);
    const a = ans('A', 'answerDismissShared', w, m);
    await tick(2);
    await settle('S', w, m); await a;
    expect(w.ops).toContain('signOut');
    const long = armedBy.filter((t) => t.ms >= 1000).map((t) => t.id);
    expect(long.length).toBeGreaterThan(0);
    expect([...new Set(long)]).toEqual(['S']);
  });

  test('W1 the settle\'s owner is let go during the native wipe: a waiting answer neither fails nor answers on the partial run; the settle asked again finishes it', async () => {
    const w = world('present', ['wipe']); const m = memFs([TAKES]); m.files.set(STAMP_FILE, new TextEncoder().encode('another account'));
    const s1c = newCall('S1');
    const s1 = outcome(ask(s1c, () => answer('accountSettle' as never, [] as never, store(w) as never, m.storage, phone(w) as never)));
    await tick(5);
    const b = outcome(ans('B', 'answerDismissShared', w, m));
    await tick(5);
    letGo(s1c); await tick(30);
    expect(b.done).toBe(false);                                                   // no answer goes on with a run whose native wipe was refused
    w.open();
    await settle('S2', w, m); await tick(30);
    expect(b.done).toBe(true); expect(b.error).toBe('');
    expect(w.ops).toContain('wipe');
    const digest = createHash('sha256').update(KEY).digest('hex');
    expect(new TextDecoder().decode(m.files.get(STAMP_FILE)!)).toBe(digest);       // the account change finished: the new account's stamp
    void s1;
  });

  test('C1 two answers at launch share one reconcile (the native sign-out runs once)', async () => {
    const w = world('absent'); const m = memFs([TAKES, PENDING_FILE]);
    const x = ans('X', 'answerDismissShared', w, m); const y = ans('Y', 'answerDismissShared', w, m);
    await settle('S', w, m); await Promise.all([x, y]);
    expect(w.ops.filter((o) => o === 'signOut').length).toBe(1);
    expect(m.files.has(PENDING_FILE)).toBe(false);
  });
});

describe('other work shared between answers', () => {
  test('A1 hostMode: the probe\'s answer is let go while the probe sleeps; another answer still gets a decision', async () => {
    const quick = { available: true, later: () => hostOp(async () => ({})) };            // answers at once, never a key state: the probe loop sleeps between tries
    adoptNative(quick as never, 'x');
    const a = newCall('A');
    void ask(a, () => hostMode(150, 20)).catch(() => undefined);
    await tick(3);
    const second = ask(newCall('B'), () => hostMode(150, 20)).then((v) => v, (e: Error) => `error: ${e.message}`);
    await tick(1);
    letGo(a);
    expect(await second).toBe('none');
  });

  test('N1 the notebook page: a read whose answer was let go is not handed to the next answer with the same key', async () => {
    const files = new Map<string, Uint8Array>();
    const storage = { fs: { readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    let open!: () => void; const gate = new Promise<void>((r) => { open = r; }); let held = true;
    const reply = (p: string): unknown => (p === '/api/autoradio/learning' ? { items: [] } : p === '/api/texture/edition' ? { found: true, doc_id: 'nb1' } : p === '/api/texture/documents/nb1' ? { document: { current_revision_id: 'r1' } } : { revision: { content: '# My notebook\n\n- one line\n' } });
    const get = (p: string) => hostOp(async () => { if (held) await gate; return reply(p); });
    const env = { get, device: null };
    const args = { selected: '', tick: 1, active: true, signedOutTick: '' };
    const a = newCall('N1');
    void ask(a, () => notebookShared(args, storage, env)).catch(() => undefined);
    await tick(3);
    letGo(a); held = false; open(); await tick(5);
    const r = await ask(newCall('N2'), () => notebookShared(args, storage, env));
    expect(r.docState).toBe('ready');
  });

  test('M1 mail\'s cipher probe refused because its answer was let go is not kept as "no cipher"', async () => {
    const w = world('present', ['mailCan']); adoptNative(phone(w) as never, 'm');
    const a = newCall('A');
    const first = ask(a, () => canSeal()).catch(() => false);
    await tick(3);
    letGo(a); w.open(); await first;
    expect(await ask(newCall('B'), () => canSeal())).toBe(true);
  });

  test('B1 the boot announcement refused because its answer was let go is sent by the next native call', async () => {
    const w = world('present', ['generation']); adoptNative(phone(w) as never, 'b');
    const a = newCall('A');
    const first = ask(a, async () => { await nativeLater()!({ op: 'keystate' }); }).catch(() => undefined);
    await tick(3);
    letGo(a); w.open(); await first;
    await ask(newCall('B'), async () => { await nativeLater()!({ op: 'keystate' }); });
    expect(w.ops).toContain('generation:boot');
  });
});

describe('the settle\'s failures, and the answers that wait for it', () => {
  const noStore = (id: string, source: string, w: World, m: ReturnType<typeof memFs>, args: unknown[] = ['x']) =>
    ask(newCall(id), () => answer(source as never, args as never, undefined as never, m.storage, phone(w) as never));     // no secret store: the key is read through the app module, as a bridge
  test('K1 a key read that threw is not published: the settle fails, the waiter waits, the settle asked again publishes', async () => {
    const w = world('absent'); w.keyThrows = 1; const m = memFs([TAKES, PENDING_FILE]);
    await expect(noStore('S1', 'accountSettle', w, m, [])).rejects.toThrow('could not read the key');
    const b = outcome(noStore('B', 'answerDismissShared', w, m));
    await tick(10);
    expect(b.done).toBe(false);
    await noStore('S2', 'accountSettle', w, m, []); await tick(10);
    expect(b.done).toBe(true); expect(b.error).toBe('');
    expect(w.ops).toContain('signOut'); expect(m.files.has(PENDING_FILE)).toBe(false);         // the unfinished sign-out was finished once the key could be read
  });
  test('K2 a key read that always throws: after KEY_ERROR_RUNS settles "nothing" is published and nothing is wiped', async () => {
    const w = world('absent'); w.keyThrows = 100; const m = memFs([TAKES, PENDING_FILE]);
    for (let i = 1; i < KEY_ERROR_RUNS; i++) await expect(noStore(`S${i}`, 'accountSettle', w, m, [])).rejects.toThrow('could not read the key');
    const r = await noStore('S', 'accountSettle', w, m, []) as { action: string };
    expect(r.action).toBe('nothing'); expect(m.files.has(TAKES)).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(true);
    expect((await noStore('B', 'answerDismissShared', w, m) as { ok: boolean }).ok).toBe(true);
  });
  test('D1 a slow settle keeps the sources waiting; they answer after it publishes, after its wipe', async () => {
    const w = world('absent', ['signOut']); const m = memFs([TAKES, PENDING_FILE]);
    const st = outcome(settle('S', w, m));
    const b = outcome(ans('B', 'answerDismissShared', w, m));
    let wipedFirst = false; void ans('C', 'answerDismissShared', w, m).then(() => { wipedFirst = !m.files.has(TAKES); });
    await tick(80);
    expect(b.done).toBe(false); expect(st.done).toBe(false);
    w.open(); await tick(40);
    expect(st.error).toBe(''); expect(b.done).toBe(true); expect(b.error).toBe(''); expect(wipedFirst).toBe(true);
  });
  test('S1 a run reset while it ran (a sign-in) fails instead of answering; the next run publishes', async () => {
    const w = world('absent', ['signOut']); const m = memFs([TAKES, PENDING_FILE]);
    const s1 = outcome(settle('S1', w, m));
    await tick(5);
    resetSettled(); w.open(); await tick(20);
    expect(s1.error).toContain('superseded or reset');
    const b = outcome(ans('B', 'answerDismissShared', w, m)); await tick(5);
    expect(b.done).toBe(false);
    await settle('S2', w, m); await tick(10);
    expect(b.done).toBe(true); expect(b.error).toBe('');
  });
  test('K3 the secret store: a get that throws is a key-read error (not published); InvalidAccessError is locked ("nothing", published)', async () => {
    const w = world('present'); const m = memFs([TAKES]);
    const throwing = (name: string) => ({ get: () => { throw Object.assign(new Error(name === 'InvalidAccessError' ? 'InvalidAccessError: locked' : 'the store failed'), { name }); }, set: () => undefined, forget: () => undefined });
    await expect(ask(newCall('S1'), () => answer('accountSettle' as never, [] as never, throwing('Error') as never, m.storage, phone(w) as never))).rejects.toThrow('could not read the key');
    resetSeam(); resetSettled();
    const r = await ask(newCall('S2'), () => answer('accountSettle' as never, [] as never, throwing('InvalidAccessError') as never, m.storage, phone(w) as never)) as { action: string };
    expect(r.action).toBe('nothing'); expect(m.files.has(TAKES)).toBe(true);
  });
  test('D2 the contract always asks the settle', () => {
    const read = (f: string) => readFileSync(new URL(`./${f}`, import.meta.url), 'utf8');
    expect(read('app.contract')).toContain('members "./account.contract"');
    expect(read('app.contract')).toContain('members "./spine.contract"');
    expect(read('account.contract')).toMatch(/^\s*resource acctSettle = accountSettle\(\) as shape AccountSettle else empty\(\) transient$/m);       // constant arguments: nothing lets it go
    expect(read('spine.contract')).toMatch(/^\s*resource pgs = pagesView\([^)]*acctSettle\.epoch[^)]*\)/m);                                       // the page set (always read: every page) reads it, so it is asked
    expect(read('spine.contract')).toMatch(/^\s*derive pg = pgs$/m);
    expect(read('account.contract')).toMatch(/^\s*task settleAgain when failed\(acctSettle\)/m);
  });
});

// R1 Exact's refusals of a let-go answer's work, as the pinned runtime words them (each beside its file and line at f53347503); isRefusal must know every one, and must not take a real failure for one.
describe('isRefusal knows the pinned runtime\'s refusals', () => {
  const PINNED: [string, string][] = [
    ['js/src/fetch.js', 'the answer was let go before this fetch'],                                   // :212
    ['js/src/fetch.js', 'fetch() called from background work: it was never run. Fetch inside an answer'],   // :207
    ['js/src/prelude.js', 'the answer was let go before this reply; the request may already have been sent'],   // :1033, a fetch in flight when its answer is let go
    ['js/src/prelude.js', 'the answer was let go'],                                                    // :345, :1007, :1038 (a yield or a secret lease)
    ['js/src/prelude.js', 'native.call() called from background work: it was never run. Call it inside an answer'],   // :777
    ['js/src/prelude.js', 'native call outside an answer'],                                            // :779
    ['js/src/prelude.js', ' is unavailable here: a timer belongs to the answer that arms it'],         // :1364 (timerOwner, prefixed by the API's name)
  ];
  test('R1 every pinned refusal is a refusal; a network or server failure is not', async () => {
    const { isRefusal } = await import('./api.ts');
    for (const [, m] of PINNED) expect(isRefusal(new Error(m.startsWith(' ') ? 'setTimeout()' + m : m))).toBe(true);
    expect(isRefusal(new Error('network: the answer was let go before this fetch'))).toBe(true);          // api.ts send() wraps a fetch failure's message
    for (const m of ['network: The Internet connection appears to be offline.', 'timeout: request timed out', 'HTTP 500', 'the Keychain bridge failed']) expect(isRefusal(new Error(m))).toBe(false);
  });
  // CI has no Exact checkout (.github/workflows/ci.yml autoradio-shared-tests: "No Exact checkout"): skipped there, by name. Anywhere else a missing checkout fails here.
  test.skipIf(!!process.env.CI)('R2 the messages are still the pinned runtime\'s (mobile/.exact-src)', () => {
    const root = new URL('../.exact-src/', import.meta.url);
    const src = Object.fromEntries(['js/src/fetch.js', 'js/src/prelude.js'].map((f) => [f, readFileSync(new URL(f, root), 'utf8')]));
    for (const [f, m] of PINNED) expect(src[f]).toContain(m.trim());
  });
});
