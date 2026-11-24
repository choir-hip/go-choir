// The model-download agreement and the sign-in prompt's words under bun test (consent.ts): C1 to C6, against a fake storage and a fake native.
import { beforeEach, describe, expect, test } from 'bun:test';
import { consentAgree, consentView, deskMode, forgetConsent, modelsAgreement, parseConsent, wipeConsent, consentPick, consentPicks, CONSENT_FILE } from './consent.ts';
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
    expect(await modelsAgreement(storage, 'search')).toEqual({ ok: false, mobile: false });
    for (const t of ['', 'not json', '[]', '{"agreed":"yes"}', '{"agreed":{"voice":5,"search":"maybe"}}', 'null']) expect(parseConsent(t)).toEqual({ agreed: {}, picks: {} });
    expect(parseConsent('{"agreed":{"voice":"agreed-mobile","x":"agreed","y":"no"}}')).toEqual({ agreed: { voice: 'agreed-mobile', x: 'agreed' }, picks: {} });
  });
  test('C2 agreement is per group; Wi-Fi never lowers a mobile agreement; each persists', async () => {
    expect(await consentAgree('voice', false, storage)).toBe(true);
    expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: false }); expect(await modelsAgreement(storage, 'search')).toEqual({ ok: false, mobile: false });
    expect(await modelsAgreement(storage, 'voice-rest')).toEqual({ ok: false, mobile: false });
    await consentAgree('voice', true, storage); await consentAgree('voice', false, storage);
    forgetConsent(); expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: true });
    expect(kept()).toEqual({ agreed: { voice: 'agreed-mobile' }, picks: {} });
  });
  test('C2 unread is never yes, and nothing is written over what was not read', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{"agreed":{"search":"agreed"}}')); readFails = true;
    expect((await modelsAgreement(storage, 'search')).ok).toBe(false);
    expect(await consentAgree('voice', false, storage)).toBe(false);
    readFails = false; expect((await modelsAgreement(storage, 'search')).ok).toBe(true); expect((await modelsAgreement(storage, 'voice')).ok).toBe(false);
  });
  test('C2 an agreement that could not be written is not an agreement', async () => {
    const bad = { fs: { ...(storage as { fs: Record<string, unknown> }).fs, atomicWriteFile: async () => { throw new Error('ENOSPC'); } } } as never;
    expect(await consentAgree('voice', false, bad)).toBe(false); expect((await modelsAgreement(bad, 'voice')).ok).toBe(false);
  });
  test('C3 a sign-out forgets it; the account moving drops what was read', async () => {
    await consentAgree('search', false, storage); await wipeConsent(storage); expect(files.size).toBe(0); expect((await modelsAgreement(storage, 'search')).ok).toBe(false);
    await consentAgree('search', false, storage); nextGeneration(); files.clear(); expect((await modelsAgreement(storage, 'search')).ok).toBe(false);
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

// DOOR A (review round 3): native's download door holds a copy of what the person agreed to, pushed at every change, at load and at sign-out. A fetch with no push has no agreement.
describe('the door is told what was agreed', () => {
  let sent: { op: string; agreed?: Record<string, string> }[];
  beforeEach(() => { sent = []; bindNative({ later: async (r) => { if (r.op === 'obConsent') sent.push(r as never); return {}; } }); });
  test('C8 the record is pushed when it is loaded, when a group is agreed (Wi-Fi, then mobile), and emptied at sign-out', async () => {
    await consentView(storage);
    expect(sent.at(-1)?.agreed).toEqual({});
    await consentAgree('voice', false, storage); expect(sent.at(-1)?.agreed).toEqual({ voice: 'agreed' });
    await consentAgree('voice', true, storage); expect(sent.at(-1)?.agreed).toEqual({ voice: 'agreed-mobile' });
    await consentAgree('search', false, storage); expect(sent.at(-1)?.agreed).toEqual({ voice: 'agreed-mobile', search: 'agreed' });
    await wipeConsent(storage); expect(sent.at(-1)?.agreed).toEqual({});
  });
  test('C8 an agreement that could not be written is not pushed: the door stays closed', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{}'));
    const bad = { fs: { ...(storage as { fs: Record<string, unknown> }).fs, atomicWriteFile: async () => { throw new Error('EIO'); } } } as never;
    await consentView(bad); sent = [];
    expect(await consentAgree('voice', false, bad)).toBe(false);
    expect(sent.some((r) => r.agreed?.voice)).toBe(false);
  });
});

