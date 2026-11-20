// What the app says about the account (account-state.ts): signed in, signed out, locked (the key cannot be read yet) or rejected (the server refuses the key). Failure list, written first:
//  X1 no key (the Keychain said item-not-found) is signedOut, with the action and a sentence of what signing in gives; no handle at all (Android, web) is signedOut too.
//  X2 an unreadable key (locked before the first unlock) is "locked": no sign-in prompt, no wipe; unreadable then readable ends signedIn with signedOut never shown in between.
//  X3 a key the server refused (401) is "rejected" with "This phone's sign-in is no longer accepted. Sign in again." and the action; a new key (another digest) clears it.
//  X4 a bridge error is not signed out (nothing is shown that could be wrong).   X5 demo mode is never signed out.
//  X6 the words are the app's one list (the page words say "Sign in", not "Sign in again", unless the app knows a key existed).
import { beforeEach, describe, expect, test } from 'bun:test';
import { accountState, WORDS } from './account-state.ts';
import { adoptNative, resetSeam, setDemo, authedJSON } from './api.ts';
import { settleAccount, resetSettled, STAMP_FILE } from './account.ts';

let reply: { state: string; found?: boolean; value?: string; throws?: boolean } = { state: 'absent' };
const handle = () => ({ available: true, later: async (r: { op: string }) => {
  if (reply.throws) throw new Error('bridge');
  if (r.op === 'keystate') return { state: reply.state };
  if (r.op === 'session') return reply.state === 'present' ? { found: true, value: reply.value ?? 'test-key-A-not-real', computerId: 'c' } : { found: false };
  return { ok: true };
} }) as never;
beforeEach(() => { resetSeam(); reply = { state: 'absent' }; adoptNative(handle(), 'x'); });

describe('accountState', () => {
  test('X1 no key: signedOut with the action and a sentence; no handle: signedOut', async () => {
    const s = await accountState('');
    expect(s).toMatchObject({ state: 'signedOut', action: 'signIn' }); expect(s.message).toBe(WORDS.signedOut); expect(WORDS.signedOut).toContain('mail'); expect(WORDS.signedOut).not.toContain('again');
    resetSeam(); expect((await accountState('')).state).toBe('signedOut');
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
    expect(words(new SignedOutError()).message).toBe('Sign in to read your mail.'); expect(words(new SignedOutError('the key was refused; sign in again')).message).toBe('Sign in again to read your mail.');
    expect(files.say(new SignedOutError())).toBe('Sign in to see your files.'); expect(files.say(new SignedOutError('the key was refused; sign in again'))).toBe('Sign in again to see your files.');
  });
});
