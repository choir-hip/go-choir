// What the app says about the account (account-state.ts): signed in, signed out, locked (the key cannot be read yet) or rejected (the server refuses the key). Failure list, written first:
//  X1 no key (the Keychain said item-not-found) is signedOut, with the action and a sentence of what signing in gives; no handle at all (Android, web) is signedOut too.
//  X2 an unreadable key (locked before the first unlock) is "locked": no sign-in prompt, no wipe; unreadable then readable ends signedIn with signedOut never shown in between.
//  X3 a key the server refused (401) is "rejected" with "This phone's sign-in is no longer accepted. Sign in again." and the action; a new key (another digest) clears it.
//  X4 a bridge error is not signed out (nothing is shown that could be wrong).   X5 demo mode is never signed out.
//  X6 the words are the app's one list (the page words say "Sign in", not "Sign in again", unless the app knows a key existed).
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { accountState, WORDS } from './account-state.ts';
import { resetHost, hostMode } from './host.ts';
import { adoptNative, resetSeam, setDemo, authedJSON } from './api.ts';
import { settleAccount, resetSettled, STAMP_FILE } from './account.ts';

let reply: { state: string; found?: boolean; value?: string; throws?: boolean; throwsKey?: boolean } = { state: 'absent' };
const handle = () => ({ available: true, later: async (r: { op: string }) => {
  if (reply.throws || (reply.throwsKey && r.op === 'keystate')) throw new Error('bridge');
  if (r.op === 'keystate') return { state: reply.state };
  if (r.op === 'session') return reply.state === 'present' ? { found: true, value: reply.value ?? 'test-key-A-not-real', computerId: 'c' } : { found: false };
  return { ok: true };
} }) as never;
const realFetch = globalThis.fetch;
beforeEach(() => { resetSeam(); resetHost(); reply = { state: 'absent' }; adoptNative(handle(), 'x'); });
afterEach(() => { globalThis.fetch = realFetch; });

describe('accountState', () => {
  test('X1 no key: signedOut with the action and a sentence; no handle: noSignIn', async () => {
    const s = await accountState('');
    expect(s).toMatchObject({ state: 'signedOut', action: 'signIn' }); expect(s.message).toBe(WORDS.signedOut); expect(WORDS.signedOut.length).toBeLessThan(60); expect(WORDS.signedOut).not.toContain('again');
    resetSeam(); expect((await accountState('', '', false, 'none')).state).toBe('noSignIn');                 // no handle at all: no sign-in is possible here (no dead button)
  });
  test('X2 locked then readable: locked (no sign-in action), then signedIn; signedOut never appears; nothing is wiped', async () => {
    const seen: string[] = [];
    reply = { state: 'unreadable' }; const a = await accountState(''); seen.push(a.state); expect(a.action).toBe(''); expect(a.message).toBe(WORDS.locked);
    reply = { state: 'present' }; const b = await accountState('t1'); seen.push(b.state);
    expect(seen).toEqual(['locked', 'signedIn']); expect(seen).not.toContain('signedOut');
    // and the launch's reconcile does nothing while the key is unreadable (never wipes, never adopts)
    reply = { state: 'unreadable' }; resetSettled();
    const files = new Map<string, Uint8Array>([['app:/data/takes.json', new Uint8Array([1])], [STAMP_FILE, new TextEncoder().encode('stamp')]]);
    const storage = { fs: { stat: async (p: string) => { if (!files.has(p)) throw new Error('ENOENT'); return { size: 1 }; }, readFile: async (p: string) => { const f = files.get(p); if (!f) throw new Error('ENOENT'); return f; }, rm: async (p: string) => { files.delete(p); }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, readdir: async () => { throw new Error('ENOENT'); } } } as never;
    await settleAccount(storage, undefined, { signOut: async () => true, wipe: async () => true });
    expect(files.has('app:/data/takes.json')).toBe(true); expect(files.has(STAMP_FILE)).toBe(true);
  });
  test('X3 a refused key is rejected with the action; another key clears it', async () => {
    reply = { state: 'present', value: 'test-key-A-not-real' };
    expect((await accountState('')).state).toBe('signedIn');
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never;
    await authedJSON('/api/x').catch(() => undefined);                           // the server refuses the key: api.ts notes which key
    const r = await accountState('t2'); expect(r).toMatchObject({ state: 'rejected', action: 'signIn' }); expect(r.message).toBe("This phone's sign-in is no longer accepted. Sign in again.");
    reply = { state: 'present', value: 'test-key-B-not-real' };
    expect((await accountState('t3')).state).toBe('signedIn');
  });
  test('X4 a bridge error is not shown as signed out', async () => {
    reply = { state: 'present', throws: true }; const s = await accountState('');
    expect(s.state).toBe('unknown'); expect(s.message).toBe(''); expect(s.action).toBe('');
  });
  test('X7 on the radio page the sentence says what the radio does signed out', async () => {
    const r = await accountState('', 'radio:'); expect(r).toMatchObject({ state: 'signedOut', action: 'signIn', message: WORDS.radioSignedOut });
    expect((await accountState('', 'paper:front')).message).toBe(WORDS.signedOut);
  });
  test('X5 demo mode is never signed out', async () => { setDemo(true); expect((await accountState('')).state).toBe('signedIn'); });
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
    const s = await accountState('', 'paper:front', false, 'none');
    expect(s).toMatchObject({ state: 'noSignIn', action: '' }); expect(s.message).toBe(WORDS.noSignIn);
  });
  test('U1 a signed-in cold launch never shows the sign-in card: with no handle and no host decision the answer is unknown (nothing rendered); the handle arrives and it is signedIn', async () => {
    resetSeam();
    expect((await accountState('', 'radio:', false, '')).state).toBe('unknown');
    reply = { state: 'present' }; adoptNative(handle(), 'late');
    expect((await accountState('', 'radio:', false, 'native')).state).toBe('signedIn');
  });
  test('D1 a demo launch never shows the banner (accountState carries the demo flag)', async () => {
    resetSeam(); const s = await accountState('', 'radio:', true, ''); expect(s).toMatchObject({ state: 'signedIn', message: '' });
  });
  test('R1 a later success with the same key clears "rejected"', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never; await authedJSON('/api/x').catch(() => undefined);
    expect((await accountState('a')).state).toBe('rejected');
    globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never; await authedJSON('/api/x');
    expect((await accountState('b')).state).toBe('signedIn');
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
      for (const top of ['mail:', 'files:', 'radio:']) states.add((await accountState(tick, top, false, 'native')).state);
    expect([...states]).toEqual(['signedIn']);
  });
  test('H1 a native handle that arrives after the host wait corrects the state with no navigation: noSignIn, then signedIn', async () => {
    resetSeam(); resetHost();
    expect(await hostMode(20, 10)).toBe('none');
    expect((await accountState('', 'radio:', false, 'none')).state).toBe('noSignIn');
    reply = { state: 'present' }; adoptNative(handle(), 'late');
    const m = await hostMode(20, 10);                                       // the contract asks the host fact again on its tick; a decided none re-checks without waiting
    expect(m).toBe('native');
    expect((await accountState('', 'radio:', false, m)).state).toBe('signedIn');
  });
  test('F1 a refusal seen by a page shows the card at once: the 401 is recorded, and the next answer of the card is rejected with the action', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('no', { status: 401 })) as never;
    await authedJSON('/x').catch(() => undefined);
    const v = await accountState('after-401', 'mail:', false, 'native');
    expect(v).toMatchObject({ state: 'rejected', action: 'signIn' });
  });
  test('B1 a bridge that throws while the key is asked says unknown, never sign in', async () => {
    reply = { state: 'absent', throwsKey: true };
    const r = await authedJSON('/x').catch((e) => e);
    expect((r as { reason?: string }).reason).toBe('unknown');
    expect(WORDS.unknown).not.toMatch(/sign in/i);
  });
  test('X1b a late handle never shows a sign-in card first: undecided is unknown, and a "none" decision is the only way to noSignIn', async () => {
    resetSeam();
    expect((await accountState('', 'paper:front', false, '')).state).toBe('unknown');
    expect((await accountState('', 'paper:front', false, 'native')).state).toBe('unknown');      // native decided but its handle not bound yet: still nothing shown
    reply = { state: 'absent' }; adoptNative(handle(), 'x');
    expect((await accountState('', 'paper:front', false, 'native')).state).toBe('signedOut');
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
  test('D2 a demo launch never reads the key (it cannot say locked first)', async () => {
    let keystateCalls = 0;
    adoptNative({ available: true, later: async (r: { op: string }) => { if (r.op === 'keystate') keystateCalls++; return { state: 'unreadable' }; } } as never, 'x');
    const v = await accountState('', 'radio:', true, 'native');
    expect(v.state).toBe('signedIn'); expect(keystateCalls).toBe(0);
  });
  test('Z1 the rejected digest is cleared by a later 200 with the same key, and the card is signedIn', async () => {
    reply = { state: 'present' };
    globalThis.fetch = (async () => new Response('no', { status: 401 })) as never;
    await authedJSON('/x').catch(() => undefined);
    expect((await accountState('a', 'mail:', false, 'native')).state).toBe('rejected');
    globalThis.fetch = (async () => new Response('{}', { status: 200, headers: { 'content-type': 'application/json' } })) as never;
    await authedJSON('/x');
    expect((await accountState('b', 'mail:', false, 'native')).state).toBe('signedIn');
  });
});
