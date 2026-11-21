// The account boundary (account.ts), from docs/design/account-boundary-2026-11-21.md. THE FAILURE LIST, written before the code; each line is a test below.
// The boundary is DERIVED, never remembered: two durable facts (the key in the Keychain, the record `app:/data/account-stamp`) are read again before every page answer
// that touches account data. A page reads or writes in exactly two cases: the record and the key name the same account (OPEN), or there is no record and no key (EMPTY).
// "Drive" below means through app.ts `answer` with a real source (pagesView) reading a seeded store; a test that only asked account.ts would repeat the mistake of the two
// builds that were sent back (they changed what is remembered and left the page gate open).
//
//  CELLS   the 4 record x 4 key cells of design section 2: the state; whether the drive answers or refuses; what one job leaves; that a second job changes nothing;
//          that a cell with no wipe leaves the generation unmoved; the host with no native module.
//  R1  the record rejecting ENOENT is none; EACCES, EBUSY, failed are unreadable (LOCKED, nothing wiped).   R2 an empty, short, upper-case or digest-plus-list record is "another".
//  R3  a record read past its deadline is LOCKED.   R4 no storage is LOCKED.   R5 keystate present, then the session read finds nothing: LOCKED.
//  R6  keystate hangs: LOCKED after the deadline (the key read has a deadline).   R7 a store path whose stat rejects anything but ENOENT is a survivor.
//  T1  Adopt writes only when there is no record.   T2 Clear runs native, the shared wipes, the survivor check, the record removal, in that order; with key B, Adopt B follows.
//  T3  the record is never rewritten from one digest to another.   T4 Clear calls native `wipe`, never `signOut`.   T5 one generation move per attempt.
//  X   a crash after each point of the crash table: a fresh process reads the same files and key; no drive reads the old store before the relaunch's job ends.
//  F1  native signOut past its deadline, the key deleted later: "did not finish"; once the key is gone the drive refuses.   F2 native throws at once, throws later, answers not done.
//  F3  the key delete fails: OPEN for A, no wipe at all, sealing unblocked.   F4 a shared wipe leaves its file: CLEARING stays, the card names the store, the next attempt retries.
//  F5  Adopt's write fails: refused, retried.   F6 the record removal fails or the file survives it: CLEARING stays.   F7 a mail cache copy survives: CLEARING stays.
//  F8  native answers done with `failed` not empty: the record stays.   F9 a native wipe that never answers: past its deadline the next attempt asks again; when one answers, the record goes.
//  C1  fifteen drives during CLEARING with key B: one Clear, all refuse at once, after it each reads B's empty data and the epoch moved.   C2 a sign-out while Clear runs waits for it.
//  C3  two sign-outs.   C4 key B appears while CLEARING: Clear, then Adopt B; B's write during CLEARING is refused.   C5 a source begun in OPEN with a wipe completing while it awaits:
//      the answer is refused and its write refused.   C6 a late native answer after its attempt ended writes nothing.   C7 a drive while the sign-out awaits native: OPEN before the key delete.
//  H   a wipe that half-finishes and is re-run does not destroy what the account now signed in wrote; native calls never overlap (the one writer).
//  O   sign-out is "delete the key, then settle", and the result follows from the facts afterwards: signed out / signed out and clearing / did not finish.
//  D   what the design removes is gone: the marker file, the remembered settle answer, the late continuations, the generation bump inside wipeAll.
//  W   the wipe list, the grants and the paths the modules name stay in step (S4, S8, S17, S19 of the old file, with the record out of WIPES).
//  S9  sign out A, sign in B: B sees nothing of A, and a reply still in flight is dropped.   S16, S18 kept (the paper title route; the epoch).
import { afterEach, describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import * as account from './account.ts';
import * as api from './api.ts';
import * as P from './pages.ts';
import { answer } from './app.ts';
import { ContentStore } from './content.ts';
import { VectorStore } from './search.ts';
import { dig, events, KEY_A, KEY_B, memFs, newWorld, pagesFile, PAGES, phone, SECRET, STAMP, type Mem, type World } from './account-phone.ts';

// The same helpers run against the code before the boundary existed (the failure list was seen failing there): a missing one is a no-op.
const resetBoundary = (): void => { const a = account as Record<string, unknown>; (a.resetBoundary as (() => void) | undefined)?.(); (a.resetSettled as (() => void) | undefined)?.(); };
const idle = async (): Promise<void> => { await (account as Record<string, unknown> as { idle?: () => Promise<void> }).idle?.(); };

const { bindNative, resetSeam, SignedOutError, accountGeneration } = api;
const authedJSON = (p: string, o: object = {}) => api.authedJSON(p, { ...o, world: api.currentWorld() } as never);
const SEED_FILES: Record<string, string> = { [PAGES]: pagesFile(), 'app:/data/takes.json': 'x', 'app:/data/podcasts-library.json': 'x', 'app:/data/mail/msg-1.bin': 'x' };
const DATA = Object.keys(SEED_FILES);

type Rec = 'none' | 'A' | 'other' | 'unreadable';
type KeyC = 'absent' | 'A' | 'B' | 'unreadable';
let W: World; let n: ReturnType<typeof phone>; let m: Mem;
/** A phone: the files (a record cell, the account's data), the Keychain and the Swift stores; the seam bound to the app module; a process with nothing remembered. */
function boot(rec: Rec | string, key: KeyC, seed: Record<string, string> | null = SEED_FILES): void {
  events.length = 0;
  W = key === 'absent' ? newWorld('absent') : key === 'unreadable' ? newWorld('unreadable') : newWorld('present', key === 'A' ? KEY_A : KEY_B);
  const files: Record<string, string> = { ...(seed ?? {}) };
  if (rec === 'A') files[STAMP] = dig(KEY_A); else if (rec === 'other') files[STAMP] = 'garbled'; else if (rec === 'unreadable') files[STAMP] = dig(KEY_A); else if (rec !== 'none') files[STAMP] = rec;
  m = memFs(files);
  if (rec === 'unreadable') m.faults.read.set(STAMP, 'EACCES');
  n = phone(() => W);
  relaunch();
}
/** The process dies and starts again over the same files, key and Swift stores. */
function relaunch(): void { resetBoundary(); resetSeam(); P.forget(); bindNative(n as never); }
afterEach(() => { resetBoundary(); resetSeam(); });

const snap = (): string => [...m.files.entries()].map(([k, v]) => `${k}=${new TextDecoder().decode(v)}`).sort().join('|');
type Probe = { threw: string | null; leaked: boolean };
/** Drive: one page answer through app.ts, over the seeded store. */
async function probe(source = 'pagesView', args: unknown[] = [0]): Promise<Probe> {
  P.forget();
  try { const out = await answer(source as never, args as never, undefined as never, m.storage, n as never); return { threw: null, leaked: JSON.stringify(out).includes(SECRET) }; }
  catch (e) { return { threw: (e as Error).name, leaked: false }; }
}
const facts = async () => account.readFacts(m.storage);
const stateNow = async (): Promise<string> => account.stateOf(await facts());
const signOut = () => answer('accountSignOut' as never, [] as never, undefined as never, m.storage, n as never) as Promise<{ done: boolean; failed: string; message: string; epoch: number }>;
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
const dataLeft = (): string[] => DATA.filter((p) => m.files.has(p));

// ------------------------------------------------------------------------------------------------------------------------------------ the sixteen cells

describe('CELLS: record x key', () => {
  const EXPECT: Record<Rec, Record<KeyC, string>> = {
    none:       { absent: 'empty',    A: 'adopt',    B: 'adopt',    unreadable: 'locked' },
    A:          { absent: 'clearing', A: 'open',     B: 'clearing', unreadable: 'locked' },
    other:      { absent: 'clearing', A: 'clearing', B: 'clearing', unreadable: 'locked' },
    unreadable: { absent: 'locked',   A: 'locked',   B: 'locked',   unreadable: 'locked' },
  };
  for (const rec of Object.keys(EXPECT) as Rec[]) for (const key of Object.keys(EXPECT[rec]) as KeyC[]) {
    const want = EXPECT[rec][key];
    test(`${rec} x ${key} is ${want}`, async () => {
      boot(rec, key, rec === 'none' && key === 'absent' ? null : SEED_FILES);
      expect(await stateNow()).toBe(want);
      const before = snap(); const g0 = accountGeneration();
      const first = await probe();
      if (want === 'clearing' || want === 'locked') { expect(first.threw).toBe('AccountClosedError'); expect(first.leaked).toBe(false); }
      else expect(first.threw).toBeNull();
      if (want === 'open') expect(first.leaked).toBe(true);                                           // its own data answers
      await idle();
      if (want === 'locked') { expect(snap()).toBe(before); expect(W.log).toEqual([]); expect(accountGeneration()).toBe(g0); }
      if (want === 'clearing') {
        expect(dataLeft()).toEqual([]);                                                               // every store wiped,
        expect(m.files.get(STAMP) ? new TextDecoder().decode(m.files.get(STAMP)!) : null).toBe(key === 'absent' ? null : dig(key === 'A' ? KEY_A : KEY_B));   // then the record removed (Adopt of the present key after it)
        expect(W.swift).toBe(false);
        expect(W.log.filter((x) => x === 'signOut')).toEqual([]);                                     // a wipe never deletes a key
        expect(W.key).toBe(key === 'absent' ? 'absent' : 'present');
      }
      if (want === 'adopt') expect(new TextDecoder().decode(m.files.get(STAMP)!)).toBe(dig(key === 'A' ? KEY_A : KEY_B));
      // a second job changes nothing
      const after = snap(); const g1 = accountGeneration();
      await probe(); await idle();
      expect(snap()).toBe(after);
      if (want !== 'clearing') expect(accountGeneration()).toBe(g0 === g1 ? g0 : g1);
      if (want === 'open' || want === 'empty' || want === 'locked') expect(accountGeneration()).toBe(g0);     // no wipe: the generation never moved
      // and the drive after the heal reads what is there (own data for open/adopt; nothing for the wiped)
      const last = await probe();
      if (want === 'clearing') { expect(last.threw).toBeNull(); expect(last.leaked).toBe(false); }
    });
  }
  test('a host with no native module: record none is EMPTY and reads; any record is LOCKED', async () => {
    boot('none', 'absent', null); bindNative(undefined);
    expect(await stateNow()).toBe('empty');
    boot('A', 'absent'); bindNative(undefined);
    expect(await stateNow()).toBe('locked');
    const r = await answer('pagesView' as never, [0] as never, undefined as never, m.storage, undefined as never).then(() => 'answered', (e) => (e as Error).name);
    expect(r).toBe('AccountClosedError');
  });
  test('a native module that answered before and is missing now is "unreadable", never "absent": nothing is wiped', async () => {
    boot('A', 'A');
    await probe();                                                                                    // the module answered: the seam knows it is expected
    bindNative(undefined);
    expect((await facts()).key.state).toBe('unreadable');
    expect(await stateNow()).toBe('locked');
    await idle(); expect(dataLeft().length).toBe(DATA.length);
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ fact reads

describe('R: the facts are read, and a fact that cannot be read refuses', () => {
  test('R1 the record: ENOENT is none; EACCES, EBUSY and failed are unreadable and nothing is wiped', async () => {
    for (const code of ['EACCES', 'EBUSY', 'failed']) {
      boot('A', 'A'); m.faults.read.set(STAMP, code);
      expect(await stateNow()).toBe('locked');
      const p = await probe(); expect(p.threw).toBe('AccountClosedError');
      await idle(); expect(dataLeft().length).toBe(DATA.length); expect(W.log).toEqual([]);
    }
    boot('none', 'absent', null); expect((await facts()).record).toEqual({ k: 'none' });
  });
  test('R2 an empty, short, upper-case or digest-plus-list record is "another": CLEARING, never adopted, never read', async () => {
    for (const rec of ['', 'abc', dig(KEY_A).toUpperCase(), dig(KEY_A) + '\nsome-store,other-store', ' ' + dig(KEY_A), dig(KEY_A) + 'a']) {
      boot(rec, 'A');
      expect((await facts()).record).toEqual({ k: 'other' });
      expect(await stateNow()).toBe('clearing');
      expect((await probe()).leaked).toBe(false);
    }
    boot(dig(KEY_A), 'A'); expect((await facts()).record).toEqual({ k: 'names', digest: dig(KEY_A) });
  });
  test('R3 a record read past its deadline is LOCKED', async () => {
    boot('A', 'A'); account.timing.recordMs = 30; m.faults.hangRead.add(STAMP);
    expect(await stateNow()).toBe('locked');
    expect((await probe()).threw).toBe('AccountClosedError');
  });
  test('R4 no storage is LOCKED (a refusal, not "nothing to protect")', async () => {
    boot('none', 'absent', null);
    const b = await account.enter(undefined, undefined);
    expect(b.readable).toBe(false); expect(b.state).toBe('locked');
  });
  test('R5 keystate says present and the session read then finds nothing: LOCKED, nothing wiped', async () => {
    boot('A', 'A'); W.sessionFails = true;
    expect((await facts()).key.state).toBe('unreadable');
    expect((await probe()).threw).toBe('AccountClosedError');
    await idle(); expect(dataLeft().length).toBe(DATA.length);
  });
  test('R6 keystate that never answers is LOCKED after the key read deadline', async () => {
    boot('A', 'A'); api.keyTiming.ms = 30; W.keystateHangs = true;
    const t = await facts(); expect(t.key.state).toBe('unreadable');
    expect((await probe()).threw).toBe('AccountClosedError');
    await idle(); expect(dataLeft().length).toBe(DATA.length);
  });
  test('R7 a store whose stat rejects anything but ENOENT is a survivor: the record stays', async () => {
    boot('A', 'absent'); m.faults.stat.set('app:/data/takes.json', 'EACCES');
    await probe(); await idle();
    expect(m.files.has(STAMP)).toBe(true); expect(await stateNow()).toBe('clearing');
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ transitions

describe('T: the writer', () => {
  test('T1 Adopt writes only when there is no record, and only the key digest', async () => {
    boot('A', 'A'); await probe(); await idle(); expect(m.writes.filter((w) => w.startsWith(STAMP))).toEqual([]);
    boot('none', 'B'); await probe(); await idle(); expect(m.writes.filter((w) => w.startsWith(STAMP))).toEqual([`${STAMP}=${dig(KEY_B)}`]);
  });
  test('T2 Clear: native wipe, then the shared wipes, then the record removal; with key B, Adopt B after', async () => {
    boot('A', 'B'); await probe(); await idle();
    const at = (e: string) => events.indexOf(e);
    expect(at('native:wipe')).toBeGreaterThanOrEqual(0);
    expect(at('native:wipe')).toBeLessThan(at(`rm:${PAGES}`));
    expect(at(`rm:${PAGES}`)).toBeLessThan(at(`rm:${STAMP}`));
    expect(at(`rm:${STAMP}`)).toBeLessThan(at(`write:${STAMP}`));
    expect(m.text(STAMP)).toBe(dig(KEY_B));
  });
  test('T3 the record is never rewritten from one digest to another (a spy on writes), through sign-out A and sign-in B', async () => {
    boot('A', 'A'); await signOut(); W.key = 'present'; W.value = KEY_B; await probe(); await idle(); await signOut();
    expect(m.spy.stampOverwrites).toBe(0);
    boot('A', 'B'); await probe(); await idle(); expect(m.spy.stampOverwrites).toBe(0);
  });
  test('T4 Clear calls native wipe, never signOut', async () => {
    boot('A', 'B'); await probe(); await idle();
    expect(W.log).toEqual(['wipe']);
  });
  test('T5 one generation move per attempt; many drives during one attempt move it once', async () => {
    boot('A', 'absent'); W.wipe = 'hang'; const g0 = accountGeneration();
    await Promise.all(Array.from({ length: 5 }, () => probe())); await wait(10);
    expect(accountGeneration()).toBe(g0 + 1);
    W.wipe = 'ok'; W.release?.(); await idle();
    expect(accountGeneration()).toBe(g0 + 1);
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ crash points

describe('X: a crash after each point, then a relaunch', () => {
  // [name, record, key, files present, whether the old data must not be read before the relaunch heals, the record after the heal, the key after]
  const rows: [string, string, KeyC, Record<string, string>, boolean, string | null][] = [
    ['sign-out tapped, before native', dig(KEY_A), 'A', SEED_FILES, false, dig(KEY_A)],
    ['the native side deleted the key, before its wipe', dig(KEY_A), 'absent', SEED_FILES, true, null],
    ['the native wipe, before the shared wipes', dig(KEY_A), 'absent', SEED_FILES, true, null],
    ['half of the shared wipes', dig(KEY_A), 'absent', { 'app:/data/takes.json': 'x' }, true, null],
    ['the wipes, before the record removal', dig(KEY_A), 'absent', {}, true, null],
    ['the record removal', '', 'absent', {}, false, null],
    ['a launch that found key B, mid-wipe', dig(KEY_A), 'B', SEED_FILES, true, dig(KEY_B)],
    ['the record removal, before Adopt B', '', 'B', {}, false, dig(KEY_B)],
    ['a torn record write', dig(KEY_A).slice(0, 31), 'A', SEED_FILES, true, dig(KEY_A)],
  ];
  for (const [name, rec, key, files, refuses, record] of rows) {
    test(`crash after: ${name}`, async () => {
      boot(rec === '' ? 'none' : rec, key, files);
      const first = await probe();
      expect(first.leaked && refuses).toBe(false);                                                   // no relaunch reads another account's data
      if (refuses) expect(first.threw).toBe('AccountClosedError');
      await idle();
      expect(m.text(STAMP) ?? null).toBe(record);
      if (refuses) expect(dataLeft()).toEqual([]);
      expect((await probe()).threw).toBeNull();
    });
  }
  test('crash after: a native delete that silently failed (OPEN, the card said "did not finish")', async () => {
    boot('A', 'A'); W.keyDeleteFails = true;
    const r = await signOut();
    expect(r.done).toBe(false); expect(r.message).toContain('did not finish');
    relaunch(); expect(await stateNow()).toBe('open'); expect(dataLeft().length).toBe(DATA.length);
  });
  test('a real sign-out killed between the key delete and the Swift wipe, then a relaunch: refused until healed, then clean', async () => {
    boot('A', 'A'); W.signOut = 'killAfterKeyDelete'; W.wipe = 'hang';
    const out = signOut(); await wait(30);
    relaunch();                                                                                      // the process dies here
    expect(await stateNow()).toBe('clearing');
    expect((await probe()).threw).toBe('AccountClosedError');
    W.wipe = 'ok'; W.release?.(); await out.catch(() => undefined);
    resetBoundary(); relaunch(); await probe(); await idle();
    expect(dataLeft()).toEqual([]); expect(m.files.has(STAMP)).toBe(false); expect(W.swift).toBe(false);
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ physical faults

describe('F: faults', () => {
  test('F1 native signOut past its deadline, the key deleted later: "did not finish"; once the key is gone the drive refuses; the late answer writes nothing', async () => {
    boot('A', 'A'); account.timing.nativeMs = 40; W.signOut = 'hang';
    const r = await signOut();
    expect(r.done).toBe(false); expect(r.message).toContain('did not finish');
    expect(await stateNow()).toBe('open'); expect((await probe()).leaked).toBe(true);                // the key is still A's: A's own data, A's own phone
    expect(api.sealingBlocked()).toBe(false);                                                        // the sealing block is lifted
    W.signOut = 'ok'; const before = snap(); const w0 = W.log.length; W.release?.(); await wait(20); await idle();
    expect(snap()).toBe(before);                                                                     // C6: the late native answer changes nothing by itself
    expect(W.log.length).toBe(w0);
    expect(W.key).toBe('absent');                                                                    // the key went (late)
    expect((await probe()).threw).toBe('AccountClosedError');                                        // so the next answer finds CLEARING
    await idle(); expect(dataLeft()).toEqual([]);
  });
  test('F2 native throws at once, answers not done: the phone stays OPEN for A, nothing wiped; it throws after the key delete: signed out, cleared by the facts', async () => {
    for (const mode of ['throw', 'notdone'] as const) {
      boot('A', 'A'); W.signOut = mode;
      const r = await signOut();
      expect(r.done).toBe(false); expect(await stateNow()).toBe('open'); expect(dataLeft().length).toBe(DATA.length); expect(api.sealingBlocked()).toBe(false);
    }
    boot('A', 'A'); W.signOut = 'throwLater';
    const r = await signOut();
    expect(r.done).toBe(true); expect(dataLeft()).toEqual([]); expect(m.files.has(STAMP)).toBe(false);
  });
  test('F3 (encodes LANDING 2 native: the shipped native still runs its own wipe after a failed key delete, ModelsEntry.swift) the key delete fails: OPEN for A, no wipe at all (native or shared), sealing unblocked', async () => {
    boot('A', 'A'); W.keyDeleteFails = true;
    const r = await signOut();
    expect(r.done).toBe(false); expect(W.log).toEqual(['signOut']); expect(dataLeft().length).toBe(DATA.length); expect(m.files.has(STAMP)).toBe(true);
    expect(api.sealingBlocked()).toBe(false); expect(await stateNow()).toBe('open'); expect(W.swift).toBe(true);
  });
  test('F4 a shared wipe leaves its file: CLEARING stays, the card names the store, the next attempt retries every store', async () => {
    boot('A', 'absent'); m.faults.rmSilently.add('app:/data/takes.json');
    await probe(); await idle();
    expect(await stateNow()).toBe('clearing'); expect(m.files.has(STAMP)).toBe(true);
    const c = await account.peek(m.storage, undefined); expect(c.failed.join(',')).toContain('takes.json');
    m.faults.rmSilently.clear(); m.files.set('app:/data/podcasts-library.json', new TextEncoder().encode('again'));
    await probe(); await idle();
    expect(dataLeft()).toEqual([]); expect(await stateNow()).toBe('empty');
  });
  test('F5 Adopt\'s write fails: refused; the next answer retries and reads', async () => {
    boot('none', 'A'); m.faults.write.add(STAMP);
    expect((await probe()).threw).toBe('AccountClosedError');
    m.faults.write.clear();
    const p = await probe(); expect(p.threw).toBeNull(); expect(m.text(STAMP)).toBe(dig(KEY_A));
  });
  test('F6 the record removal fails, or the file survives it: CLEARING stays', async () => {
    for (const how of ['rm', 'rmSilently'] as const) {
      boot('A', 'absent'); m.faults[how].add(STAMP);
      await probe(); await idle();
      expect(dataLeft()).toEqual([]); expect(await stateNow()).toBe('clearing');
    }
  });
  test('F7 a mail cache copy survives its removal: CLEARING stays', async () => {
    boot('A', 'absent', { ...SEED_FILES, 'app:/cache/mail-7.html': 'x' }); m.faults.rmSilently.add('app:/cache/mail-7.html');
    await probe(); await idle();
    expect(m.files.has(STAMP)).toBe(true); expect(await stateNow()).toBe('clearing');
  });
  test('F8 native answers done with failed not empty: the record stays', async () => {
    boot('A', 'absent'); W.wipeFailed = ['held-pages'];
    await probe(); await idle();
    expect(m.files.has(STAMP)).toBe(true); expect(await stateNow()).toBe('clearing');
    W.wipeFailed = []; await probe(); await idle(); expect(await stateNow()).toBe('empty');
  });
  test('F9 a native wipe that never answers: past its deadline the next attempt asks again; when one answers, the record goes', async () => {
    boot('A', 'absent'); account.timing.nativeMs = 40; W.wipe = 'hang';
    await probe(); await wait(10); await probe();                                                    // inside the deadline: the same attempt, not a second call
    expect(W.log).toEqual(['wipe']);
    await wait(60); await idle();                                                            // the attempt ended at its deadline: record kept
    expect(m.files.has(STAMP)).toBe(true);
    W.wipe = 'ok'; await probe(); await idle();
    expect(W.log).toEqual(['wipe', 'wipe']); expect(m.files.has(STAMP)).toBe(false); expect(await stateNow()).toBe('empty');
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ concurrency

describe('C: one writer', () => {
  test('C1 fifteen drives during CLEARING with key B: one Clear, all refuse at once; after it the epoch moved and each reads B\'s empty data', async () => {
    boot('A', 'B'); W.wipe = 'hang';
    const e0 = (await account.peek(m.storage, undefined)).epoch;
    const rs = await Promise.all(Array.from({ length: 15 }, () => probe()));
    expect(rs.every((r) => r.threw === 'AccountClosedError')).toBe(true);                            // none waited for the wipe
    expect(W.log).toEqual(['wipe']);
    W.wipe = 'ok'; W.release?.(); await idle();
    const after = await Promise.all(Array.from({ length: 15 }, () => probe()));
    expect(after.every((r) => r.threw === null && !r.leaked)).toBe(true);
    expect((await account.peek(m.storage, undefined)).epoch).not.toBe(e0);
    expect(m.text(STAMP)).toBe(dig(KEY_B));
  });
  test('C2 a sign-out while Clear runs waits for it', async () => {
    boot('A', 'B'); W.wipe = 'hang'; await probe(); await wait(5);
    let done = false; const out = signOut().then((r) => { done = true; return r; });
    await wait(40); expect(done).toBe(false);
    W.wipe = 'ok'; W.release?.(); await out; expect(done).toBe(true);
  });
  test('C3 two sign-outs at once: both end, native calls never overlap, the phone is EMPTY', async () => {
    boot('A', 'A');
    const [a, b] = await Promise.all([signOut(), signOut()]);
    expect(a.done && b.done).toBe(true); expect(W.maxInflight).toBe(1);
    expect(await stateNow()).toBe('empty'); expect(dataLeft()).toEqual([]);
  });
  test('C4 key B appears while CLEARING: Clear, then Adopt B; a write while CLEARING is refused', async () => {
    boot('A', 'absent'); W.wipe = 'hang'; await probe(); await wait(5);
    W.key = 'present'; W.value = KEY_B;
    const w = await probe('pagesOpen', ['mail:inbox-b', 'B mail']);
    expect(w.threw).toBe('AccountClosedError');                                                      // B's first write is refused
    W.wipe = 'ok'; W.release?.(); await idle();
    expect(m.text(STAMP)).toBe(dig(KEY_B)); expect(dataLeft()).toEqual([]);
    expect((await probe('pagesOpen', ['mail:inbox-b', 'B mail'])).threw).toBeNull();
    expect(m.text(PAGES)).toContain('inbox-b');
  });
  test('C5 a source begun in OPEN with a wipe completing while it awaits: the answer is refused, and its write is refused', async () => {
    boot('A', 'A'); let release: () => void = () => undefined; m.faults.hold.set(PAGES, new Promise<void>((r) => { release = r; }));
    const read = probe(); await wait(10);
    const out = signOut(); await wait(10); release();
    expect((await read).threw).toBe('AccountClosedError'); await out;
    m.faults.hold.clear(); boot('A', 'A'); release = () => undefined;
    m.faults.hold.set(PAGES, new Promise<void>((r) => { release = r; }));
    const write = probe('pagesOpen', ['mail:late', 'late']); await wait(10);
    const out2 = signOut(); await wait(10); release();
    expect((await write).threw).toBe('AccountClosedError'); await out2;
    expect(m.files.has(PAGES)).toBe(false);                                                          // nothing was written back after the wipe
  });
  test('C7 a drive while the sign-out awaits native: OPEN before the key delete; EMPTY after it', async () => {
    boot('A', 'A'); W.signOut = 'hang';
    const out = signOut(); await wait(10);
    expect((await probe()).threw).toBeNull();
    W.signOut = 'ok'; W.release?.(); const r = await out;
    expect(r.done).toBe(true); expect((await probe()).leaked).toBe(false);
  });
});

describe('E2: survivors, a stuck Clear, and the card never waiting', () => {
  test('a content.db-wal or -shm that survives its removal keeps the phone in CLEARING (the survivor check looks at both)', async () => {
    for (const f of ['app:/data/content.db-wal', 'app:/data/content.db-shm']) {
      boot('A', 'absent', { ...SEED_FILES, [f]: 'x' }); m.faults.rmSilently.add(f);
      await probe(); await idle();
      expect([f, m.files.has(STAMP), await stateNow()]).toEqual([f, true, 'clearing']);
    }
  });
  test('the card does not wait on a running job in ADOPT (it answers while a sign-out hangs in native)', async () => {
    boot('none', 'A'); W.signOut = 'hang';
    const out = signOut(); await wait(15);
    const c = await Promise.race([account.peek(m.storage, undefined).then((p) => p.state), wait(400).then(() => 'waited')]);
    expect(c).not.toBe('waited');
    W.signOut = 'ok'; W.release?.(); await out;
  });
  test('a sign-out clears the refused-key note (sign out, sign in with the same key: no "no longer accepted")', async () => {
    boot('A', 'A');
    globalThis.fetch = (async (u: string) => (String(u).endsWith('/api/compute/status') ? new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 }) : new Response('{}', { status: 401 }))) as never;
    await authedJSON('/api/x').catch(() => undefined);
    expect(api.rejectedDigest()).toBe(dig(KEY_A));
    await signOut();
    expect(api.rejectedDigest()).toBe('');
  });
});

describe('E: the epoch, and a hung shared wipe', () => {
  test('the epoch moves on sign-in (EMPTY then OPEN), so Files, Mail and Textures, which take only acctTick, ask again', async () => {
    boot('none', 'absent', null);
    const e0 = (await account.peek(m.storage, undefined)).epoch;
    W.key = 'present'; W.value = KEY_A;
    const e1 = (await account.peek(m.storage, undefined)).epoch;
    expect(e1).not.toBe(e0);
    const e2 = (await account.peek(m.storage, undefined)).epoch;
    expect(e2).toBe(e1);                                                                              // and stable while nothing changes
  });
  test('a shared wipe that never answers is a failed Clear (nothing marked clean); Clear this phone still returns; the next attempt retries', async () => {
    boot('A', 'A'); (account as { timing: { sharedMs?: number } }).timing.sharedMs = 40; m.faults.hangRm.add('app:/data/takes.json');
    const r = await Promise.race([signOut(), wait(2000).then(() => 'hung')]);
    expect(r).not.toBe('hung');
    expect((r as { done: boolean }).done).toBe(true); expect((r as { failed: string }).failed).not.toBe('');
    expect(m.files.has(STAMP)).toBe(true); expect(await stateNow()).toBe('clearing');                  // the record stays: nothing was marked clean
    // The hung removal is still in flight: the next Clear cannot finish while a write or removal of the ended world may yet land (drain), so the phone stays CLEARING until the host lets go of it.
    await probe(); await idle(); expect(await stateNow()).toBe('clearing');
    m.faults.hangRm.clear(); m.faults.releaseRm(); await wait(10); await probe(); await idle();
    expect(dataLeft()).toEqual([]); expect(await stateNow()).toBe('empty');
  });
});

describe('H: a re-run wipe never destroys the new account', () => {
  test('a wipe that half-finishes, B signs in, the wipe is re-run: B\'s data written after Adopt survives every later answer', async () => {
    boot('A', 'absent'); m.faults.rmSilently.add('app:/data/takes.json');
    await probe(); await idle(); expect(await stateNow()).toBe('clearing');                  // half-done
    m.faults.rmSilently.clear(); W.key = 'present'; W.value = KEY_B;                                   // B signs in
    await probe(); await idle();                                                              // the re-run: A's remnants go, then Adopt B
    expect(m.text(STAMP)).toBe(dig(KEY_B)); expect(dataLeft()).toEqual([]);
    expect((await probe('pagesOpen', ['mail:inbox-b', 'B mail'])).threw).toBeNull();                  // B writes
    const g = accountGeneration(); const wipes = W.log.length;
    for (let i = 0; i < 4; i++) { await probe(); await account.peek(m.storage, undefined); await idle(); }
    expect(m.text(PAGES)).toContain('inbox-b'); expect(W.log.length).toBe(wipes); expect(accountGeneration()).toBe(g);
  });
});

describe('O: sign-out is "delete the key, then settle"', () => {
  test('the native sign-out first (it carries the generation), then the shared wipes; signed out leaves no record', async () => {
    boot('A', 'A');
    const r = await signOut();
    expect(r).toMatchObject({ done: true, failed: '', message: '' });
    expect(events.indexOf('native:signOut')).toBeLessThan(events.indexOf(`rm:${PAGES}`));
    expect(m.files.size).toBe(0); expect(W.swift).toBe(false); expect(typeof W.gens[0]).toBe('number');
    expect(JSON.stringify(r)).not.toContain(KEY_A);
  });
  test('no native module: refuses and wipes nothing', async () => {
    boot('A', 'A'); bindNative(undefined);
    const r = await answer('accountSignOut' as never, [] as never, undefined as never, m.storage, undefined as never) as { done: boolean };
    expect(r.done).toBe(false); expect(dataLeft().length).toBe(DATA.length);
  });
  test('the key gone but a store cannot be cleared: "signed out", the record stays, the next launch tries again', async () => {
    boot('A', 'A'); m.faults.rmSilently.add('app:/data/takes.json');
    const r = await signOut();
    expect(r.done).toBe(true); expect(r.failed).toContain('takes.json'); expect(m.files.has(STAMP)).toBe(true);
    relaunch(); expect((await probe()).threw).toBe('AccountClosedError');
  });
  test('the epoch of a result is larger for each sign-out, and moves for a wipe', async () => {
    boot('A', 'A'); const a = await signOut();
    W.key = 'present'; W.value = KEY_A; m.files.set(STAMP, new TextEncoder().encode(dig(KEY_A))); await probe();          // the same phone signs in again
    const b = await signOut();
    expect(a.done && b.done).toBe(true); expect(b.epoch).toBeGreaterThan(a.epoch);
  });
});

describe('D: what the design removes is gone', () => {
  test('the marker file, the settle answer, the late continuations and the bump inside wipeAll', async () => {
    const a = account as Record<string, unknown>;
    for (const name of ['PENDING_FILE', 'settleAccount', 'resetSettled', 'reconcile', 'decide', 'nativeOf', 'DATA_WIPES']) expect(a[name]).toBeUndefined();
    const src = readFileSync(new URL('./account.ts', import.meta.url), 'utf8');
    expect(src).not.toContain('signout-pending'); expect(src).not.toContain('settled');
    const g = accountGeneration(); await account.wipeAll(undefined, undefined, []); expect(accountGeneration()).toBe(g);
    expect(readFileSync(new URL('./app.ts', import.meta.url), 'utf8')).not.toContain('signout-pending');
  });
});

// ------------------------------------------------------------------------------------------------------------------------------------ the wipe list

describe('W: the wipes', () => {
  test('wipeAll runs every registered wipe and returns the failed names', async () => {
    const ran: string[] = [];
    const failed = await account.wipeAll(undefined, undefined, [{ name: 'x', run: async () => { ran.push('x'); } }, { name: 'y', run: async () => { throw new Error('no'); } }]);
    expect(ran).toEqual(['x']); expect(failed).toEqual(['y']);
  });
  const PHONE_KEPT = new Set(['app:/data/theme-choice.json', 'app:/data/themes']);
  const allPaths = (): string[] => {
    const dir = new URL('.', import.meta.url).pathname;
    const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts') && !f.endsWith('.d.ts') && f !== 'app.ts');
    const found = new Set<string>();
    for (const f of files) {
      const src = readFileSync(dir + f, 'utf8');
      for (const mm of src.matchAll(/['"`](app:\/(?:data|cache|tmp)(?:\/[^'"`$]*)?)/g)) found.add(mm[1].replace(/\/$/, ''));
    }
    return [...found];
  };
  test('S4 every app:/ path any shared module names is covered by the wipe list (the record is the boundary\'s own, removed last)', () => {
    const found = allPaths();
    expect(found.length).toBeGreaterThan(10);
    const covered = (p: string) => account.wipedPaths().some((w) => p === w || p.startsWith(w + '/') || p.startsWith(w + '-'));
    const roots = ['app:/data', 'app:/cache', 'app:/tmp'];
    const unknown = found.filter((p) => !roots.includes(p) && !covered(p) && !PHONE_KEPT.has(p) && p !== STAMP && !p.startsWith('app:/cache/mail-'));
    expect(unknown).toEqual([]);
  });
  test('the record is not in WIPES (a wipe that removed it would make a half-wiped phone look empty)', () => {
    expect(account.WIPES.map((w) => w.name)).not.toContain('account-stamp');
    expect(account.wipedPaths()).not.toContain(STAMP);
  });
  test('S19 the phone\'s own themes survive a wipe; a theme installed from a Texture document does not (it carries account A\'s document: its name and colours)', async () => {
    const mm = memFs(['app:/data/theme-choice.json', 'app:/data/theme-textures/texture-ember.json', 'app:/data/themes/mine.json', 'app:/data/recent-files.json']);
    expect(await account.wipeAll(mm.storage, undefined, account.WIPES)).toEqual([]);
    expect([...mm.files.keys()].sort()).toEqual(['app:/data/theme-choice.json', 'app:/data/themes/mine.json']);
  });
  test('S19b a Texture theme loaded in memory is not listed to the next account after a Clear, and a surviving texture file keeps the phone in CLEARING', async () => {
    const T = await import('./themes.ts'); const ember = JSON.stringify(T.parseTextureTheme(JSON.parse(readFileSync(new URL('./fixtures/texture/theme-ember.json', import.meta.url), 'utf8'))).theme);
    boot('A', 'absent', { ...SEED_FILES, 'app:/data/theme-textures/texture-ember.json': ember });
    T.resetThemes();
    expect(JSON.stringify(await T.themeListShared('light', false, m.storage))).toContain('ember');
    m.faults.rmSilently.add('app:/data/theme-textures/texture-ember.json'); m.faults.rmSilently.add('app:/data/theme-textures');
    await probe(); await idle(); expect(await stateNow()).toBe('clearing');
    m.faults.rmSilently.clear(); await probe(); await idle(); expect(await stateNow()).toBe('empty');
    expect(JSON.stringify(await T.themeListShared('light', false, m.storage))).not.toContain('ember');
  });
  test('S8 the real wipes leave an in-memory file system empty (seeded at every path the modules name, mail files included)', async () => {
    const seed = [...allPaths().filter((p) => p.split('/').length > 2 && p !== STAMP && !PHONE_KEPT.has(p)).flatMap((p) => (p.endsWith('/') ? [] : [p, `${p}/child.bin`])), 'app:/data/mail/msg-1.bin', 'app:/cache/mail-1.html'];
    const mm = memFs(seed);
    expect(await account.wipeAll(mm.storage, undefined, account.WIPES)).toEqual([]);
    expect([...mm.files.keys()]).toEqual([]);
  });
});

describe('S9 sign out A, sign in B', () => {
  test('B sees nothing of A: files, a live content store, a live vector store, the remembered computer; a reply in flight is dropped', async () => {
    boot('A', 'A');
    const seen: string[] = [];
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      const h = init.headers as Record<string, string>;
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: `comp-of-${h.Authorization.includes('-A-') ? 'A' : 'B'}`, current: true }] }), { status: 200 });
      seen.push(h['X-Choir-Computer']);
      if (String(u).endsWith('/late')) { await gate; return new Response('{"a":"data"}', { status: 200 }); }
      return new Response('{}', { status: 200 });
    }) as never;
    const db = { execute: async () => undefined, query: async () => ({ rows: [] }) };
    const content = new ContentStore(db as never); const vectors = new VectorStore(); vectors.wipe(); vectors.dirty = false;
    await authedJSON('/api/one');
    const late = authedJSON('/late').then(() => 'wrote', (e) => (e instanceof SignedOutError ? 'dropped' : 'error'));
    await wait(5);
    const gen0 = content.generation;
    const r = await signOut();
    expect(r.done).toBe(true); expect([...m.files.keys()]).toEqual([]); expect(content.generation).toBeGreaterThan(gen0);
    release(); expect(await Promise.race([late, wait(60).then(() => 'never')])).toBe('never');      // the late reply of A never reaches a caller (the chain stays pending: api.ts, the world)
    W.key = 'present'; W.value = KEY_B;
    await authedJSON('/api/two');
    expect(seen[0]).not.toBe(seen[seen.length - 1]);
    const p = await probe(); expect(p.threw).toBeNull(); expect(p.leaked).toBe(false);
  });
});

describe('S16 the paper page passes the story title with the route (the loss the deck builder named)', () => {
  test('the action, the paper buttons and the interface carry the title', async () => {
    const c = await Bun.file(new URL('./account.contract', import.meta.url)).text();
    expect(c).toContain('action playStory(route: string, title: string)'); expect(c).toContain('choirPlay(route=route, title=title)');
    const p = await Bun.file(new URL('./paper.contract', import.meta.url)).text();
    expect((p.match(/playStory\(s\.route, s\.title\)/g) ?? []).length).toBeGreaterThanOrEqual(1); expect(p).not.toContain('playStory(s.route)');
    const i = JSON.parse(await Bun.file(new URL('../exact-package/exact.json', import.meta.url)).text());
    expect(i.names.find((n: { send?: string }) => n.send === 'choirPlay').options).toEqual({ route: 'string', title: 'string' });
  });
});

describe('S18 the epoch', () => {
  test('the contracts pass the one tick (acctTick) to the shared notebook, and the sign-out result carries the epoch', async () => {
    const c = await Bun.file(new URL('./members-notebook.contract', import.meta.url)).text();
    expect(c).toContain('notebookPageShared("", 0, noNative, au.signedIn, acctTick)');
    expect(await Bun.file(new URL('./account-types.contract', import.meta.url)).text()).toContain('epoch: number');
  });
});

describe('S17 the grants and the wipes name the same paths, exactly, in both directions', () => {
  const grantsOf = (): { read: Set<string>; write: Set<string> } => {
    const app = readFileSync(new URL('./app.ts', import.meta.url), 'utf8');
    const read = new Set<string>(), write = new Set<string>();
    for (const mm of app.matchAll(/^fs\.(read|write) (app:\/[^\s`;]+)/gm)) (mm[1] === 'read' ? read : write).add(mm[2]);
    return { read, write };
  };
  const WIPED_NOT_GRANTED = new Set(['app:/tmp/picked', 'app:/data/vectors', 'app:/data/content.db']);
  const GRANTED_NOT_WIPED = new Set(['app:/data/exports', 'app:/cache', 'app:/data/theme-choice.json', 'app:/data/themes', STAMP]);   // the record is the boundary's: removed last, by account.ts, not by a wipe
  test('every wiped path is granted for read and write, but the named exceptions', () => {
    const { read, write } = grantsOf();
    expect(account.wipedPaths().filter((p) => !WIPED_NOT_GRANTED.has(p)).filter((p) => !(read.has(p) && write.has(p)))).toEqual([]);
    for (const p of WIPED_NOT_GRANTED) expect(account.wipedPaths()).toContain(p);
  });
  test('every granted path is wiped (exact match), but the named exceptions; the record is granted; the marker is not', () => {
    const { read, write } = grantsOf();
    const granted = new Set([...read, ...write]);
    expect([...granted].filter((p) => !GRANTED_NOT_WIPED.has(p)).filter((p) => !account.wipedPaths().includes(p))).toEqual([]);
    for (const p of GRANTED_NOT_WIPED) expect(granted.has(p)).toBe(true);
    expect(granted.has('app:/data/signout-pending')).toBe(false);
  });
});
