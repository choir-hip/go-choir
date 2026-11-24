// The model-download agreement and the sign-in prompt's words under bun test (consent.ts): C1 to C6, against a fake storage and a fake native.
import { beforeEach, describe, expect, test } from 'bun:test';
import { consentAct, consentView, deskMode, forgetConsent, modelsAgreed, modelsAgreement, parseConsent, wipeConsent, consentPick, consentPicks, CONSENT_FILE } from './consent.ts';
import { ASK_TEXT } from './consent-text.ts';
import en from './strings/en.json';
import { bindNative, nextGeneration, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
let readFails = false;
const storage = { fs: {
  readFile: async (p: string) => { if (readFails) throw new Error('EIO'); const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
const kept = (): Record<string, unknown> => JSON.parse(new TextDecoder().decode(files.get(CONSENT_FILE)!));
beforeEach(() => { files.clear(); forgetConsent(); resetSeam(); readFails = false; });

describe('the download agreement', () => {
  test('C1 a fresh account has agreed to nothing; a bad file is fresh', async () => {
    expect(await consentView(storage)).toMatchObject({ models: '', mobileOk: false });
    expect(await modelsAgreed(storage)).toBe(false);
    for (const t of ['', 'not json', '[]', '{"models":"yes"}', '{"models":5}', 'null']) expect(parseConsent(t)).toEqual({ models: '', picks: {} });
    expect(parseConsent('{"models":"agreed-mobile"}')).toEqual({ models: 'agreed-mobile', picks: {} });
  });
  test('C2 agree (Wi-Fi), agree on mobile data, decline, agree later: each persists', async () => {
    expect((await consentAct('declineModels', storage)).models).toBe('declined'); expect(await modelsAgreed(storage)).toBe(false);
    forgetConsent(); expect((await consentView(storage)).models).toBe('declined');
    expect(await consentAct('agreeModels', storage)).toMatchObject({ models: 'agreed', mobileOk: false }); expect(await modelsAgreement(storage)).toEqual({ ok: true, mobile: false });
    expect(await consentAct('agreeModelsMobile', storage)).toMatchObject({ models: 'agreed-mobile', mobileOk: true }); expect(await modelsAgreement(storage)).toEqual({ ok: true, mobile: true });
    expect(kept()).toEqual({ models: 'agreed-mobile', picks: {} });
  });
  test('C2 unread is never yes, and nothing is written over what was not read', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{"models":"agreed"}')); readFails = true;
    expect(await modelsAgreed(storage)).toBe(false);
    expect((await consentAct('declineModels', storage)).models).toBe('');
    readFails = false; expect(await modelsAgreed(storage)).toBe(true);
  });
  test('C3 a sign-out forgets it; the account moving drops what was read', async () => {
    await consentAct('agreeModels', storage); await wipeConsent(storage); expect(files.size).toBe(0); expect(await modelsAgreed(storage)).toBe(false);
    await consentAct('agreeModels', storage); nextGeneration(); files.clear(); expect(await modelsAgreed(storage)).toBe(false);
  });
  test('C4 an unknown op changes nothing (there is no op for outside AI: that agreement is the create-account screen\'s)', async () => {
    for (const op of ['everything', 'allowAI', 'withdrawAI']) expect((await consentAct(op, storage)).models).toBe('');
    expect(files.size).toBe(0);
  });
});

describe('the picks', () => {
  test('C7 a pick persists across a restart, is read by shape, is idempotent, and goes at sign-out', async () => {
    expect(parseConsent('{"picks":{"voice":"a","bad":5,"empty":"","x":[]}}').picks).toEqual({ voice: 'a' });
    expect(parseConsent('{"picks":[1]}').picks).toEqual({}); expect(parseConsent('{"picks":"a"}').picks).toEqual({});
    await consentPick('voice', 'kokoro-x', storage); await consentPick('transcriber', 'ear', storage);
    forgetConsent(); expect(await consentPicks(storage)).toEqual({ voice: 'kokoro-x', transcriber: 'ear' });
    const rev = (await consentView(storage)).rev; await consentPick('voice', 'kokoro-x', storage); expect((await consentView(storage)).rev).toBe(rev);
    await consentPick('', 'x', storage); await consentPick('voice', '', storage); expect(await consentPicks(storage)).toEqual({ voice: 'kokoro-x', transcriber: 'ear' });
    await wipeConsent(storage); expect(files.size).toBe(0); expect(await consentPicks(storage)).toEqual({});
  });
  test('C7 a pick is not written over a file that could not be read', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{"picks":{"voice":"a"}}')); readFails = true;
    await consentPick('voice', 'b', storage); readFails = false; expect(await consentPicks(storage)).toEqual({ voice: 'a' });
  });
});

describe('the radio\'s mode', () => {
  const key = (state: string, throws = false) => bindNative({ later: async (r) => { if (r.op === 'keystate') { if (throws) throw new Error('x'); return { state }; } if (r.op === 'session') return { found: state === 'present', value: 'k', computerId: 'c' }; return {}; } });
  test('C5 the desk only with a key; locked or absent is listening only; a read that threw is the desk', async () => {
    key('present'); expect(await deskMode()).toBe('desk');
    key('absent'); expect(await deskMode()).toBe('listening');
    key('unreadable'); expect(await deskMode()).toBe('listening');
    key('present', true); expect(await deskMode()).toBe('desk');
  });
});

describe('the words', () => {
  test('C6 the sign-in prompt gives one sentence of why, and strings/en.json holds the same text', async () => {
    const t = await consentView(storage);
    expect(t.signWhy).toMatch(/Listening needs no account; asking does/); expect(t.signTitle).toBe('Sign in to ask the radio');
    for (const [k, v] of Object.entries(ASK_TEXT)) expect((en as Record<string, string>)[k]).toBe(v);
    expect(Object.keys(en).filter((k) => k.startsWith('onboarding.consent'))).toEqual([]);          // no consent screen before the radio answers
  });
  test('C6 the microphone purpose names voice, takes, and what a published take does', () => {
    const m = (en as Record<string, string>)['purpose.microphone'];
    expect(m).toMatch(/Voice/); expect(m).toMatch(/take/); expect(m).toMatch(/publish/); expect(m).toMatch(/not uploaded/);
    expect((en as Record<string, string>)['purpose.recordingInBackground']).toMatch(/stops listening when the screen locks/);
  });
});
