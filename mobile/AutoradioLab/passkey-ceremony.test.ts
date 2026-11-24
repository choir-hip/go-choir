import { describe, expect, test } from 'bun:test';
import { CeremonyError, ceremonyMessage, createCeremony, getCeremony, sendRegisterCode, type CeremonyEnv, type Proof } from './passkey-ceremony.ts';

const PROOF: Proof = { attemptId: 'att', code: '12345678', binding: 'b'.repeat(32) };

const ORIGIN = 'https://choir.news';
const reply = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

function env(routes: Record<string, (body: unknown) => Response | Promise<Response>>, passkeys: Partial<CeremonyEnv['passkeys']> = {}) {
  const calls: { path: string; body: unknown }[] = [];
  const fetcher = (async (url: string, init: { body: string }) => {
    const path = url.slice(ORIGIN.length);
    const body = JSON.parse(init.body);
    calls.push({ path, body });
    const route = routes[path];
    if (!route) throw new TypeError('network');
    return route(body);
  }) as unknown as typeof fetch;
  const e: CeremonyEnv = {
    origin: ORIGIN, fetch: fetcher,
    passkeys: { create: async () => ({ id: 'c' }), get: async () => ({ id: 'g' }), ...passkeys },
  };
  return { e, calls };
}
const kindOf = async (p: Promise<unknown>) => { try { await p; return 'none'; } catch (x) { return x instanceof CeremonyError ? x.kind : 'other'; } };

describe('create', () => {
  test('begins with the email, shows the platform screen with the inner options, answers the credential', async () => {
    let seen: unknown;
    const { e, calls } = env({ '/auth/register/begin': () => reply(200, { publicKey: { challenge: 'ch', rp: { id: 'choir.news' } } }) },
      { create: async (o) => { seen = o; return { id: 'made' }; } });
    expect(await createCeremony(e, 'a@b.example', PROOF)).toEqual({ id: 'made' });
    expect(calls).toEqual([{ path: '/auth/register/begin', body: { email: 'a@b.example', attempt_id: 'att', code: '12345678', binding: 'b'.repeat(32) } }]);
    expect(seen).toEqual({ publicKey: { challenge: 'ch', rp: { id: 'choir.news' } } });
  });
  test('sign-up closed, existing email, bad email, busy and offline are their own kinds, and the screen never opens', async () => {
    let opened = 0;
    const pk = { create: async () => { opened++; return {}; } };
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(403, { code: 'registration_waitlist', error: 'Choir is full' }) }, pk).e, 'a@b.c', PROOF))).toBe('closed');
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(409, { error: 'email already registered' }) }, pk).e, 'a@b.c', PROOF))).toBe('exists');
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(400, { error: 'please enter a valid email address' }) }, pk).e, 'x', PROOF))).toBe('email');
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(429, {}) }, pk).e, 'a@b.c', PROOF))).toBe('busy');
    expect(await kindOf(createCeremony(env({}, pk).e, 'a@b.c', PROOF))).toBe('offline');
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(500, {}) }, pk).e, 'a@b.c', PROOF))).toBe('failed');
    expect(opened).toBe(0);
  });
  test('the platform rejections: cancel, an excluded credential, no support, a refused association, anything else', async () => {
    const begin = { '/auth/register/begin': () => reply(200, { publicKey: {} }) };
    const rejecting = (name: string) => env(begin, { create: async () => { throw Object.assign(new Error('x'), { name }); } }).e;
    expect(await kindOf(createCeremony(rejecting('NotAllowedError'), 'a@b.c', PROOF))).toBe('cancelled');
    expect(await kindOf(createCeremony(rejecting('AbortError'), 'a@b.c', PROOF))).toBe('cancelled');
    expect(await kindOf(createCeremony(rejecting('InvalidStateError'), 'a@b.c', PROOF))).toBe('exists');
    expect(await kindOf(createCeremony(rejecting('NotSupportedError'), 'a@b.c', PROOF))).toBe('unsupported');
    expect(await kindOf(createCeremony(rejecting('SecurityError'), 'a@b.c', PROOF))).toBe('refused');
    expect(await kindOf(createCeremony(rejecting('Weird'), 'a@b.c', PROOF))).toBe('failed');
  });
  test('a begin with no options is a failure, not a crash', async () => {
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(200, {}) }).e, 'a@b.c', PROOF))).toBe('failed');
  });
});

describe('get', () => {
  test('asks with no email and answers the credential', async () => {
    const { e, calls } = env({ '/auth/app/login/begin': () => reply(200, { publicKey: { challenge: 'c', rpId: 'choir.news' } }) });
    expect(await getCeremony(e)).toEqual({ id: 'g' });
    expect(calls).toEqual([{ path: '/auth/app/login/begin', body: {} }]);
  });
  test('another begin route can be named (a deletion asks for a fresh assertion)', async () => {
    const { e, calls } = env({ '/auth/x/begin': () => reply(200, { publicKey: {} }) });
    await getCeremony(e, '/auth/x/begin', { intent: 'delete' });
    expect(calls).toEqual([{ path: '/auth/x/begin', body: { intent: 'delete' } }]);
  });
  test('cancel, offline and a busy server', async () => {
    const begin = { '/auth/app/login/begin': () => reply(200, { publicKey: {} }) };
    const cancelled = env(begin, { get: async () => { throw Object.assign(new Error('x'), { name: 'NotAllowedError' }); } }).e;
    expect(await kindOf(getCeremony(cancelled))).toBe('cancelled');
    expect(await kindOf(getCeremony(env({}).e))).toBe('offline');
    expect(await kindOf(getCeremony(env({ '/auth/app/login/begin': () => reply(503, {}) }).e))).toBe('busy');
  });
});

describe('the words', () => {
  test('each failure has a plain sentence, none empty, none naming a code or a status', () => {
    for (const mode of ['create', 'signin'] as const) {
      for (const kind of ['cancelled', 'exists', 'closed', 'offline', 'unsupported', 'busy', 'refused', 'email', 'failed'] as const) {
        const m = ceremonyMessage(new CeremonyError(kind, 'raw 500 detail'), mode);
        expect(m.length).toBeGreaterThan(10);
        expect(m).not.toMatch(/\d{3}|raw|Error/);
      }
    }
    expect(ceremonyMessage(new Error('x'), 'create')).toBe('Something went wrong. Try again.');
    expect(ceremonyMessage(new CeremonyError('closed'), 'create')).toContain('full');
    expect(ceremonyMessage(new CeremonyError('offline'), 'signin')).toContain('connection');
  });
});

describe('the mailed code', () => {
  test('asks for the code with the address and the binding, answers the attempt id', async () => {
    const { e, calls } = env({ '/auth/register/mail': () => reply(200, { attempt_id: 'att-9' }) });
    expect(await sendRegisterCode(e, 'a@b.example', 'b'.repeat(32))).toBe('att-9');
    expect(calls).toEqual([{ path: '/auth/register/mail', body: { email: 'a@b.example', binding: 'b'.repeat(32) } }]);
  });
  test('a refused or empty answer is a failure, a closed sign-up is its own kind', async () => {
    expect(await kindOf(sendRegisterCode(env({ '/auth/register/mail': () => reply(403, { code: 'registration_waitlist' }) }).e, 'a@b.c', 'x'))).toBe('closed');
    expect(await kindOf(sendRegisterCode(env({ '/auth/register/mail': () => reply(200, {}) }).e, 'a@b.c', 'x'))).toBe('failed');
    expect(await kindOf(sendRegisterCode(env({ '/auth/register/mail': () => reply(429, {}) }).e, 'a@b.c', 'x'))).toBe('busy');
  });
  test('a wrong, spent or missing code is the code kind; an unproved account is its own kind', async () => {
    const pk = {};
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(400, { error: 'invalid or expired code' }) }, pk).e, 'a@b.c', PROOF))).toBe('code');
    expect(await kindOf(createCeremony(env({ '/auth/register/begin': () => reply(400, { code: 'proof_required', error: 'verify your address first' }) }, pk).e, 'a@b.c', PROOF))).toBe('code');
    expect(await kindOf(getCeremony(env({ '/auth/app/login/begin': () => reply(403, { code: 'address_unproved', error: 'x' }) }, pk).e))).toBe('unproved');
    expect(ceremonyMessage(new CeremonyError('code'), 'create')).toContain('code did not work');
  });
});
