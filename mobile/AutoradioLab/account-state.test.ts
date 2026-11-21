// What the app says about the account (account-state.ts): signed in, signed out, locked (the key cannot be read yet) or rejected (the server refuses the key). Failure list, written first:
//  X1 no key (the Keychain said item-not-found) is signedOut, with the action and a sentence of what signing in gives; no handle at all (Android, web) is signedOut too.
//  X2 an unreadable key (locked before the first unlock) is "locked": no sign-in prompt, no wipe; unreadable then readable ends signedIn with signedOut never shown in between.
//  X3 a key the server refused (401) is "rejected" with "This phone's sign-in is no longer accepted. Sign in again." and the action; a new key (another digest) clears it.
//  X4 a bridge error is not signed out (nothing is shown that could be wrong).
//  X6 the words are the app's one list (the page words say "Sign in", not "Sign in again", unless the app knows a key existed).
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { accountState, WORDS } from './account-state.ts';
import { memFs, type Mem } from './account-phone.ts';
import { resetHost, hostMode } from './host.ts';
import { adoptNative, resetSeam } from './api.ts';
// A signed-in request states its world (api.ts: a missing one throws): the tests state the one current when they ask.
import * as apiReal from './api.ts';
const authedJSON = (p: string, o: object = {}) => apiReal.authedJSON(p, { ...o, world: apiReal.currentWorld() } as never);
import { resetBoundary, STAMP_FILE } from './account.ts';

let reply: { state: string; found?: boolean; value?: string; throws?: boolean; throwsKey?: boolean } = { state: 'absent' };
const handle = () => ({ available: true, later: async (r: { op: string }) => {
  if (reply.throws || (reply.throwsKey && r.op === 'keystate')) throw new Error('bridge');
  if (r.op === 'keystate') return { state: reply.state };
  if (r.op === 'session') return reply.state === 'present' ? { found: true, value: reply.value ?? 'test-key-A-not-real', computerId: 'c' } : { found: false };
  if (r.op === 'wipe' || r.op === 'signOut') return { done: true };
  return { ok: true };
} }) as never;
const realFetch = globalThis.fetch;
// The card now reads the boundary (account.ts: the key and the record), so every call has the phone's storage; this one holds no record (EMPTY, or ADOPT once a key is there).
let mem: Mem;
const card = (tick: string, top = '', host = '') => accountState(tick, top, host, 0, mem.storage);
beforeEach(() => { resetSeam(); resetBoundary(); resetHost(); mem = memFs(); reply = { state: 'absent' }; adoptNative(handle(), 'x'); });
afterEach(() => { globalThis.fetch = realFetch; });

describe('accountState', () => {
  test('X1 no key: signedOut with the action and a sentence; no handle: noSignIn', async () => {
    const s = await card('');
    expect(s).toMatchObject({ state: 'signedOut', action: 'signIn' }); expect(s.message).toBe(WORDS.signedOut); expect(WORDS.signedOut.length).toBeLessThan(60); expect(WORDS.signedOut).not.toContain('again');
    resetSeam(); expect((await card('', '', 'none')).state).toBe('noSignIn');                 // no handle at all: no sign-in is possible here (no dead button)
  });
  test('X2 locked then readable: locked (no sign-in action), then signedIn; signedOut never appears; nothing is wiped', async () => {
    const seen: string[] = [];
    reply = { state: 'unreadable' }; const a = await card(''); seen.push(a.state); expect(a.action).toBe(''); expect(a.message).toBe(WORDS.locked);
    reply = { state: 'present' }; const b = await card('t1'); seen.push(b.state);
    expect(seen).toEqual(['locked', 'signedIn']); expect(seen).not.toContain('signedOut');
    // and nothing is wiped while the key is unreadable, whatever the record says (account.ts: LOCKED never wipes, never adopts)
    reply = { state: 'unreadable' }; resetBoundary();
    mem = memFs({ 'app:/data/takes.json': 'x', [STAMP_FILE]: 'stamp' });
    expect((await card('t2')).state).toBe('locked');
    expect(mem.files.has('app:/data/takes.json')).toBe(true); expect(mem.files.has(STAMP_FILE)).toBe(true);
  });
  test('X3 a refused key is rejected with the action; another key clears it', async () => {
    reply = { state: 'present', value: 'test-key-A-not-real' };
    expect((await card('')).state).toBe('signedIn');
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never;
    await authedJSON('/api/x').catch(() => undefined);                           // the server refuses the key: api.ts notes which key
    const r = await card('t2'); expect(r).toMatchObject({ state: 'rejected', action: 'clear' }); expect(r.message).toBe(WORDS.rejected);   // not Sign in: signing in over the old key would leave the old data
    // another key is another account: its phone is cleared first (the old record names the old key), then it is signed in
    reply = { state: 'present', value: 'test-key-B-not-real' };
    expect((await card('t3')).state).toBe('clearing');
    const { idle } = await import('./account.ts'); await idle();
    expect((await card('t4')).state).toBe('signedIn');
  });
  test('X4 a bridge error is not shown as signed out', async () => {
    reply = { state: 'present', throws: true }; const s = await card('');
    // a keystate that throws is a key that cannot be read: LOCKED (the pages refuse, so the card must say so and offer the way out), never silence
    expect(s.state).toBe('locked'); expect(s.message).toBe(WORDS.locked); expect(s.action).toBe('');
    const stuck = await accountState('', '', 'native', 5, mem.storage);
    expect(stuck).toMatchObject({ state: 'locked', action: 'retry', alsoClear: true }); expect(stuck.message).toContain('reinstall');
  });
  test('X7 on the radio page the sentence says what the radio does signed out', async () => {
    const r = await card('', 'radio:'); expect(r).toMatchObject({ state: 'signedOut', action: 'signIn', message: WORDS.radioSignedOut });
    expect((await card('', 'paper:front')).message).toBe(WORDS.signedOut);
  });
  test('X6 the page words say "again" only when the key was refused', async () => {
    const { words } = await import('./mail-model.ts'); const files = await import('./files.ts'); const { SignedOutError } = await import('./api.ts');
    expect(words(new SignedOutError()).message).toBe('Sign in to read your mail.'); expect(words(new SignedOutError('the key was refused; sign in again', 'refused')).message).toBe('Sign in again to read your mail.');
    expect(files.say(new SignedOutError())).toBe('Sign in to see your files.'); expect(files.say(new SignedOutError('the key was refused; sign in again', 'refused'))).toBe('Sign in again to see your files.');
  });
});

describe('round 8 (seat 1 on 43200f9a7)', () => {
  test('K1 a locked key is a typed reason all the way down the request path: the error says locked, never "sign in"', async () => {
    const { SignedOutError } = await import('./api.ts');
    reply = { state: 'unreadable' };
    const e = await authedJSON('/api/x').catch((x) => x);
    expect(e).toBeInstanceOf(SignedOutError); expect((e as { reason: string }).reason).toBe('locked');
    reply = { state: 'absent' }; expect(((await authedJSON('/api/x').catch((x) => x)) as { reason: string }).reason).toBe('none');
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never; reply = { state: 'present' };
    expect(((await authedJSON('/api/x').catch((x) => x)) as { reason: string }).reason).toBe('refused');
  });
  test('K2 every page model says the unlock sentence when locked, and "Sign in" only when there is no key', async () => {
    const { SignedOutError } = await import('./api.ts'); const { words } = await import('./mail-model.ts'); const files = await import('./files.ts'); const nb = await import('./notebook.ts');
    const locked = new SignedOutError('locked', 'locked'), none = new SignedOutError('none', 'none'), refused = new SignedOutError('refused', 'refused');
    expect(words(locked).message).toBe(WORDS.locked); expect(files.say(locked)).toBe(WORDS.locked);
    expect(words(none).message).toBe('Sign in to read your mail.'); expect(files.say(none)).toBe('Sign in to see your files.');
    expect(words(refused).message).toBe('Sign in again to read your mail.'); expect(files.say(refused)).toBe('Sign in again to see your files.');
    const get = async () => { throw locked; };
    const learning = await nb.readLearning(get as never); expect(learning).toEqual({ state: 'signedout', reason: 'locked' });
    const page = nb.page({ doc: null, learning, selected: '', steerQueued: 0 });
    expect(page.scoreNotice).toBe(WORDS.locked);
    expect(nb.page({ doc: null, learning: { state: 'signedout', reason: 'none' }, selected: '', steerQueued: 0 }).scoreNotice).toBe('Sign in to see the scorecard.');
  });
  test('N1 no handle on a host with no native package is "noSignIn": no dead button, and it says what is true', async () => {
    resetSeam();
    const s = await card('', 'paper:front', 'none');
    expect(s).toMatchObject({ state: 'noSignIn', action: '' }); expect(s.message).toBe(WORDS.noSignIn);
  });
  test('U1 a signed-in cold launch never shows the sign-in card: with no handle and no host decision the answer is unknown (nothing rendered); the handle arrives and it is signedIn', async () => {
    resetSeam();
    expect((await card('', 'radio:', '')).state).toBe('unknown');
    reply = { state: 'present' }; adoptNative(handle(), 'late');
    expect((await card('', 'radio:', 'native')).state).toBe('signedIn');
  });
  test('R1 a later success with the same key clears "rejected"', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never; await authedJSON('/api/x').catch(() => undefined);
    expect((await card('a')).state).toBe('rejected');
    globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never; await authedJSON('/api/x');
    expect((await card('b')).state).toBe('signedIn');
  });
  test('W1 the words: the radio says what it is, the signed-out sentence does not imply the paper needs an account', () => {
    expect(WORDS.radioSignedOut).toBe('This is the public paper. Sign in to hear your own programme.'); expect(WORDS.signedOut).not.toMatch(/your paper/);
  });
});

describe('round 8b (seat 2)', () => {
  test('A1 a signed-in page change re-asks the card but not the pages: the card\'s state string is the same on every tick, so the Mail and Files resources (keyed on it) are not asked again', async () => {
    reply = { state: 'present' };
    const states = new Set<string>();
    for (const tick of ['true|ok||0|visible|true|0|native|false', 'true|ok||0|visible|true|1|native|false', 'true|ok||0|hidden|false|1|native|false'])
      for (const top of ['mail:', 'files:', 'radio:']) states.add((await card(tick, top, 'native')).state);
    expect([...states]).toEqual(['signedIn']);
  });
  test('H1 a native handle that arrives after the host wait corrects the state with no navigation: noSignIn, then signedIn', async () => {
    resetSeam(); resetHost();
    expect(await hostMode(20, 10)).toBe('none');
    expect((await card('', 'radio:', 'none')).state).toBe('noSignIn');
    reply = { state: 'present' }; adoptNative(handle(), 'late');
    const m = await hostMode(20, 10);                                       // the contract asks the host fact again on its tick; a decided none re-checks without waiting
    expect(m).toBe('native');
    expect((await card('', 'radio:', m)).state).toBe('signedIn');
  });
  test('F1 a refusal seen by a page shows the card at once: the 401 is recorded, and the next answer of the card is rejected with the action', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('no', { status: 401 })) as never;
    await authedJSON('/x').catch(() => undefined);
    const v = await card('after-401', 'mail:', 'native');
    expect(v).toMatchObject({ state: 'rejected', action: 'clear' });
  });
  test('B1 a bridge that throws while the key is asked says unknown, never sign in', async () => {
    reply = { state: 'absent', throwsKey: true };
    const r = await authedJSON('/x').catch((e) => e);
    expect((r as { reason?: string }).reason).toBe('unknown');
    expect(WORDS.unknown).not.toMatch(/sign in/i);
  });
  test('X1b a late handle never shows a sign-in card first: undecided is unknown, and a "none" decision is the only way to noSignIn', async () => {
    resetSeam();
    expect((await card('', 'paper:front', '')).state).toBe('unknown');
    expect((await card('', 'paper:front', 'native')).state).toBe('unknown');      // native decided but its handle not bound yet: still nothing shown
    reply = { state: 'absent' }; adoptNative(handle(), 'x');
    expect((await card('', 'paper:front', 'native')).state).toBe('signedOut');
  });
});

describe('follow-up 2 (seat 2 on d237ff989)', () => {
  const lateHandle = (ms: number) => ({ available: true, later: async (r: { op: string }) => { await new Promise((res) => setTimeout(res, ms)); return r.op === 'keystate' ? { state: 'present' } : { ok: true }; } }) as never;
  test('H2 a bridge that answers on a macrotask (a real round trip) still wins the re-probe after "none" was decided', async () => {
    resetSeam(); resetHost();
    expect(await hostMode(20, 10)).toBe('none');
    adoptNative(lateHandle(30), 'slow');
    expect(await hostMode(20, 400)).toBe('native');                 // the decided-none re-check has its own budget; it was 0 and lost to any real round trip
  });
  test('Z1 the rejected digest is cleared by a later 200 with the same key, and the card is signedIn', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('no', { status: 401 })) as never;
    await authedJSON('/x').catch(() => undefined);
    expect((await card('a', 'mail:', 'native')).state).toBe('rejected');
    globalThis.fetch = (async () => new Response('{}', { status: 200, headers: { 'content-type': 'application/json' } })) as never;
    await authedJSON('/x');
    expect((await card('b', 'mail:', 'native')).state).toBe('signedIn');
  });
});
