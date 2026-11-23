// Sign-in in the system sign-in sheet (signin-sheet.ts; docs/reference/exact/auth-session.md). Failure list, written before the code.
// The device flow is unchanged: Swift begins it, keeps the device code, polls and writes the Keychain. Shared code only OPENS the approval page in the
// sheet (ASWebAuthenticationSession / Custom Tabs, through Exact's `openAuthSession`) and tells the contract how the sheet ended.
//  S1 the sheet opens the device-flow link with the app's own callback scheme (`return`, from authCallback()) and a fresh nonce (`state`, 16+ chars,
//     different every time), keeps the link's own parameters (the code), and opens nothing that is not the granted origin over https.
//  S2 the session is asked with that callback and that same state, and not ephemeral (the sheet must share the browser's passkeys and session).
//  S3 the callback returns: outcome "returned", and the result is only that word. The callback's URL (it may carry anything) is in no result and no log line.
//  S4 the owner cancels (499): outcome "cancelled", no message: no error noise. The ten-minute bound is the same 499.
//  S5 the platform refuses (501 no browser, 502 cannot start, 403 origin, a throw, no callback granted): outcome "failed" with a plain sentence and no
//     detail from the platform, and the state is not left holding anything.
//  S6 a callback with no approval ("returned" while the server still says pending) changes nothing here: the account is still signed out
//     (the key's presence is read from native, never inferred from the sheet).
//  S7 a link that is not the granted origin over https (a lookalike host, http, a script, garbage) is refused before any session opens.
//  S8 the app's grants: auth.session for the one server origin, auth.callback `<scheme>:/signed-in` where the scheme is the app id and each variant has its own
//     (app.json env SIGNIN_SCHEME), the callback path is the one the server builds (/signed-in).
//  S10 (review) the result names the sign-in it belongs to: it carries the link it was opened for (the non-secret verification link the card already shows), in every
//      outcome, so the contract acts only on the pending sign-in's own result; and a nonce that cannot be made is a "failed", never a throw.
//  S11 (review) closing the sheet never abandons by itself: the contract sends the cancel as `by="sheet"` (native polls first), only for this sign-in's own "cancelled", never for
//      "returned" or "failed"; a failed sheet offers the same link in the system browser (Exact's `href`, no Swift); a new sign-in clears the last result.
//  S9 sign-in never moves the key: this module imports nothing that writes a store or the key.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { sheetUrl, signInSheet, RETURN_PATH, type SheetEnv } from './signin-sheet.ts';

const ORIGIN = 'https://choir.news';
const LINK = `${ORIGIN}/device?code=ABCD-EFGH`;
const CALLBACK = 'news.choir.autoradio.lab:/signed-in';

type Call = { url: string; opts: { callback: string; state: string; ephemeral?: boolean; logout?: boolean } };
function env(over: Partial<SheetEnv> & { answer?: (c: Call) => Promise<string> } = {}): { env: SheetEnv; calls: Call[]; logs: string[] } {
  const calls: Call[] = []; const logs: string[] = [];
  const e: SheetEnv = {
    origin: ORIGIN,
    authCallback: () => CALLBACK,
    openAuthSession: (url, opts) => { const c = { url, opts }; calls.push(c); return (over.answer ?? (async (x: Call) => `${CALLBACK}?state=${x.opts.state}`))(c); },
    log: (l) => logs.push(l),
    ...(over as Partial<SheetEnv>),
  };
  return { env: e, calls, logs };
}
const refusal = (status: number, message = 'platform words'): Error => Object.assign(new Error(message), { status });

describe('sheetUrl', () => {
  test('S1 keeps the link, adds return and state', () => {
    const u = new URL(sheetUrl(LINK, CALLBACK, 'abcdefghijklmnop0123', ORIGIN)!);
    expect(u.origin).toBe(ORIGIN); expect(u.pathname).toBe('/device');
    expect(u.searchParams.get('code')).toBe('ABCD-EFGH');
    expect(u.searchParams.get('return')).toBe('news.choir.autoradio.lab'); expect(u.searchParams.get('state')).toBe('abcdefghijklmnop0123');
    expect([...u.searchParams.keys()].sort()).toEqual(['code', 'return', 'state']);
    expect(RETURN_PATH).toBe('/signed-in');
  });
  test('S1 the variant has its own scheme', () => {
    const u = new URL(sheetUrl(LINK, 'news.choir.autoradio.lab.one:/signed-in', 'abcdefghijklmnop0123', ORIGIN)!);
    expect(u.searchParams.get('return')).toBe('news.choir.autoradio.lab.one');
  });
  test('S7 refuses anything but the granted origin over https, and a callback that is not ours', () => {
    for (const bad of ['http://choir.news/device?code=A', 'https://choir.news.evil.example/device?code=A', 'https://evil.example/device', 'javascript:alert(1)', 'not a url', '', 'https://user@choir.news/device', 'https://choir.news:8443/device'])
      expect(sheetUrl(bad, CALLBACK, 'abcdefghijklmnop0123', ORIGIN)).toBeNull();
    for (const bad of ['https://choir.news/x', 'news.choir.autoradio.lab:/other', 'news.choir.autoradio.lab://h/signed-in', ':/signed-in', ''])
      expect(sheetUrl(LINK, bad, 'abcdefghijklmnop0123', ORIGIN)).toBeNull();
  });
});

describe('signInSheet', () => {
  test('S1 S2 the session opens with the callback, the same state, and shares the browser', async () => {
    const t = env();
    await signInSheet(LINK, t.env);
    expect(t.calls.length).toBe(1);
    const c = t.calls[0]; const u = new URL(c.url);
    expect(c.opts.callback).toBe(CALLBACK); expect(c.opts.ephemeral ?? false).toBe(false); expect(c.opts.logout ?? false).toBe(false);
    expect(c.opts.state.length).toBeGreaterThanOrEqual(16); expect(u.searchParams.get('state')).toBe(c.opts.state);
    expect(/^[A-Za-z0-9_-]+$/.test(c.opts.state)).toBe(true);        // the server's shape for a state
    await signInSheet(LINK, t.env);
    expect(t.calls[1].opts.state).not.toBe(c.opts.state);              // fresh each time
  });
  test('S3 returned: only the word; the callback URL is nowhere', async () => {
    const secret = 'leaked-code-123';
    const t = env({ answer: async (c) => `${CALLBACK}?state=${c.opts.state}&code=${secret}&api_key=${secret}` });
    const r = await signInSheet(LINK, t.env);
    expect(r).toEqual({ outcome: 'returned', message: '', link: LINK });
    expect(JSON.stringify(r) + t.logs.join('\n')).not.toContain(secret);
    expect(t.logs).toEqual(['signin: sheet opened', 'signin: sheet returned']);
  });
  test('S4 cancelled, with no message; the bound is the same', async () => {
    for (const msg of ['cancelled', 'the wait ended at the bound']) {
      const t = env({ answer: async () => { throw refusal(499, msg); } });
      expect(await signInSheet(LINK, t.env)).toEqual({ outcome: 'cancelled', message: '', link: LINK });
      expect(t.logs).toEqual(['signin: sheet opened', 'signin: sheet cancelled']);
    }
  });
  test('S5 refused: failed with our own sentence, never the platform’s', async () => {
    for (const status of [501, 502, 403, 409, 428]) {
      const t = env({ answer: async () => { throw refusal(status, 'https://choir.news/device?code=SECRET-1 reason'); } });
      const r = await signInSheet(LINK, t.env);
      expect(r.outcome).toBe('failed'); expect(r.message.length).toBeGreaterThan(10);
      expect(r.message + t.logs.join('\n')).not.toContain('SECRET'); expect(t.logs).toContain(`signin: sheet failed status=${status}`);
    }
    const thrown = env({ answer: async () => { throw new Error('no status at all'); } });
    expect((await signInSheet(LINK, thrown.env)).outcome).toBe('failed');
    const none = env({ authCallback: () => { throw new Error('the grants name no auth.callback'); } });
    expect((await signInSheet(LINK, none.env)).outcome).toBe('failed'); expect(none.calls.length).toBe(0);
    const noSession = env({ openAuthSession: undefined as never });
    expect((await signInSheet(LINK, noSession.env)).outcome).toBe('failed');
  });
  test('S7 a link that is not ours opens no session', async () => {
    const t = env();
    const r = await signInSheet('https://evil.example/device?code=A', t.env);
    expect(r.outcome).toBe('failed'); expect(t.calls.length).toBe(0);
    expect((await signInSheet('', t.env)).outcome).toBe('failed'); expect(t.calls.length).toBe(0);
  });
});

describe('the result belongs to its sign-in', () => {
  test('S10 every outcome carries the link it was opened for', async () => {
    const link2 = `${ORIGIN}/device?code=WXYZ-WXYZ`;
    expect((await signInSheet(link2, env().env)).link).toBe(link2);
    expect((await signInSheet(link2, env({ answer: async () => { throw refusal(499); } }).env)).link).toBe(link2);
    expect((await signInSheet(link2, env({ answer: async () => { throw refusal(501); } }).env)).link).toBe(link2);
    expect((await signInSheet('https://evil.example/x', env().env)).link).toBe('https://evil.example/x');     // even a refused one: the contract compares, it does not trust
  });
  test('S10 a nonce that cannot be made is failed, not thrown', async () => {
    const real = crypto.getRandomValues.bind(crypto);
    (crypto as { getRandomValues: unknown }).getRandomValues = () => { throw new Error('no entropy'); };
    try { const t = env(); const r = await signInSheet(LINK, t.env); expect(r.outcome).toBe('failed'); expect(t.calls.length).toBe(0); }
    finally { (crypto as { getRandomValues: unknown }).getRandomValues = real; }
  });
});

describe('the contract wiring', () => {
  const account = readFileSync(new URL('./account.contract', import.meta.url), 'utf8');
  const signedIn = readFileSync(new URL('./onboarding.contract', import.meta.url), 'utf8');
  test('S11 only this sign-in\'s cancelled result closes it, and by asking native to poll first', () => {
    // (A contract test on the web host cannot drive `au`: the native source answers its idle default, and a *.test.contract cannot set it. The decision is therefore kept to
    // three plain lines, and read here.)
    expect(account).toMatch(/derive sheetOutcome = match signInEnded \{ case some\(r\) => \(r\.link == au\.verificationURL \? r\.outcome : ""\), case none => "" \}/);
    expect(account).toMatch(/derive sheetNote = match signInEnded \{ case some\(r\) => \(r\.link == au\.verificationURL and r\.outcome == "failed" \? r\.message : ""\), case none => "" \}/);
    const act = /action sheetEnded\n((?:    .*\n)+)/.exec(account)![1];
    expect(act).toContain('if sheetOutcome == "cancelled" and au.status == "pending"');
    expect(act).toContain('send acct = choirCancelSignIn(by="sheet")');
    expect(act).not.toMatch(/returned|failed/);
    expect((account.match(/choirCancelSignIn\(/g) ?? []).length).toBe(2);        // the sheet's (by="sheet") and the card's own Cancel
    expect(account).toMatch(/action cancelSignIn\n    send acct = choirCancelSignIn\(by="card"\)/);
  });
  test('S11 a new sign-in clears the last result, so a stale one cannot show or act', () => {
    expect(account).toMatch(/action signIn\n    signInEnded = none\n    send acct = choirSignIn\(\)/);
  });
  test('S11 a sheet that could not open offers the same link in the system browser, from shared code', () => {
    expect(signedIn).toMatch(/if sheetNote != ""[\s\S]*href=au\.verificationURL/);
  });
});

describe('the callback is a hint, not proof', () => {
  test('S6 returned with no approval: still signed out', async () => {
    const { adoptNative, resetSeam } = await import('./api.ts');
    const { accountState } = await import('./account-state.ts');
    const { resetHost } = await import('./host.ts');
    resetSeam(); resetHost();
    adoptNative({ available: true, later: async (r: { op: string }) => (r.op === 'keystate' ? { state: 'absent' } : { found: false }) } as never, 'x');
    const t = env();
    expect((await signInSheet(LINK, t.env)).outcome).toBe('returned');
    expect(await accountState('after-callback')).toMatchObject({ state: 'signedOut', action: 'signIn' });
  });
});

describe('the app declares what Exact needs', () => {
  const app = readFileSync(new URL('./app.ts', import.meta.url), 'utf8');
  const json = JSON.parse(readFileSync(new URL('./app.json', import.meta.url), 'utf8'));
  test('S8 grants: one session origin, one callback per app id', () => {
    expect(app).toContain('auth.session ${ORIGIN}');
    expect(app).toContain('auth.callback ${SIGNIN_SCHEME}:/signed-in');
    expect(app).toMatch(/const SIGNIN_SCHEME = import\.meta\.env\.SIGNIN_SCHEME \?\? '([^']+)'/);
    const dflt = /const SIGNIN_SCHEME = import\.meta\.env\.SIGNIN_SCHEME \?\? '([^']+)'/.exec(app)![1];
    expect(dflt).toBe(json.app.id);                                       // the scheme IS the app id, so two installs never share one
    expect(json.env).toContain('SIGNIN_SCHEME');
    const seen = new Set([dflt]);
    for (const v of Object.values(json.variants) as { idSuffix: string; env: { SIGNIN_SCHEME: string } }[]) {
      expect(v.env.SIGNIN_SCHEME).toBe(json.app.id + v.idSuffix);
      expect(seen.has(v.env.SIGNIN_SCHEME)).toBe(false); seen.add(v.env.SIGNIN_SCHEME);
    }
    expect(seen.size).toBe(1 + Object.keys(json.variants).length);
    expect(RETURN_PATH).toBe('/signed-in');
  });
  test('S9 the module writes no store and no key', () => {
    const src = readFileSync(new URL('./signin-sheet.ts', import.meta.url), 'utf8').split('\n').filter((l) => !l.trim().startsWith('//')).map((l) => l.replace(/\s\/\/ .*$/, '').replace(/\/\*\*.*?\*\//g, '')).join('\n');   // code, not comments
    expect(src).not.toMatch(/from '\.\/(api|account|store|files)[^']*'/); expect(src).not.toMatch(/\bsecret\b|keychain|atomicWriteFile|storage\./i);
  });
});
