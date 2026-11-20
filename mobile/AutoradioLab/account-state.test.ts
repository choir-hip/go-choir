// What the app says about the account (account-state.ts): signed in, signed out, locked (the key cannot be read yet) or rejected (the server refuses the key). Failure list, written first:
//  X1 no key (the Keychain said item-not-found) is signedOut, with the action and a sentence of what signing in gives; no handle at all (Android, web) is signedOut too.
//  X2 an unreadable key (locked before the first unlock) is "locked": no sign-in prompt, no wipe; unreadable then readable ends signedIn with signedOut never shown in between.
//  X3 a key the server refused (401) is "rejected" with "This phone's sign-in is no longer accepted. Sign in again." and the action; a new key (another digest) clears it.
//  X4 a bridge error is not signed out (nothing is shown that could be wrong).   X5 demo mode is never signed out.
//  X6 the words are the app's one list (the page words say "Sign in", not "Sign in again", unless the app knows a key existed).
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { accountState, WORDS } from './account-state.ts';
import { resetHost } from './host.ts';
import { adoptNative, resetSeam, setDemo, authedJSON } from './api.ts';
import { settleAccount, resetSettled, STAMP_FILE } from './account.ts';

let reply: { state: string; found?: boolean; value?: string; throws?: boolean } = { state: 'absent' };
const handle = () => ({ available: true, later: async (r: { op: string }) => {
  if (reply.throws) throw new Error('bridge');
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
    expect(s).toMatchObject({ state: 'signedOut', action: 'signIn' }); expect(s.message).toBe(WORDS.signedOut); expect(WORDS.signedOut).toContain('mail'); expect(WORDS.signedOut).not.toContain('again');
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
    const app = await Bun.file(new URL('./app.ts', import.meta.url)).text(); expect(app).toMatch(/accountState: 2/);
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
  test('A1 mail and files are keyed on the account state: after sign-in an open page loads at once; the card is asked again on the host fact and demo; no resource of the tick depends on the card (no cycle)', async () => {
    const read = (f: string) => Bun.file(new URL(`./${f}`, import.meta.url)).text();
    const mail = await read('members-mail.contract'), files = await read('members-files.contract'), acc = await read('account.contract');
    expect(mail).toContain('mailBox(mboxAddr, dm.on, acs.state)'); expect(mail).toContain('mailMessage(pg.top, dm.on, mailOriginal, acs.state)');
    expect(files).toContain('fileList(dm.on, fsPath, fsMode, fsSort, fsDesc, fsQuery, acs.state)'); expect(files).toContain('fileOpen(dm.on, pg.top, acs.state)');
    expect(acc).toContain('${hc.mode}|${dm.on}'); expect(acc).not.toContain('${mbx.status}'); expect(acc).not.toContain('${nb.docState}');      // a lazily-asked resource in the tick leaves the card unanswered at launch
  });
  test('X1b a late handle never shows a sign-in card first: undecided is unknown, and a "none" decision is the only way to noSignIn', async () => {
    resetSeam();
    expect((await accountState('', 'paper:front', false, '')).state).toBe('unknown');
    expect((await accountState('', 'paper:front', false, 'native')).state).toBe('unknown');      // native decided but its handle not bound yet: still nothing shown
    reply = { state: 'absent' }; adoptNative(handle(), 'x');
    expect((await accountState('', 'paper:front', false, 'native')).state).toBe('signedOut');
  });
});
