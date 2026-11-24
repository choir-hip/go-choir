// The model downloads under bun test (downloads.ts): D1 to D6.
import { beforeEach, describe, expect, test } from 'bun:test';
import { downloadsAct, downloadsView, sizeLabel } from './downloads.ts';
import { onKickSearch } from './search-kick.ts';
import { consentView, forgetConsent, modelsAgreement, CONSENT_FILE } from './consent.ts';
import { noVoice, type PlanRow, type VoiceFacts } from './onboarding.ts';
import { bindNative, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
// Invented model names and sizes on purpose: nothing in downloads.ts may know a model's name.
const row = (stage: string, group: string, name: string, model: string, bytes: number, ready = false): PlanRow => ({ stage, name, model, id: model.toLowerCase(), group, host: 'Example Hub', bytes, sizeLabel: sizeLabel(bytes), ready });
const plan = (): PlanRow[] => [row('voice', 'voice', 'The voice', 'Zeta voice', 351_000_000), row('transcriber', 'voice-rest', 'Hearing you', 'Ear one', 481_400_000), row('bounds', 'voice-rest', 'Knowing when you speak', 'Gate', 2_000_000),
  row('responder', 'voice-rest', 'Talking while it works', 'Small LM', 640_000_000), row('search', 'search', 'Search on this phone', 'Embedder', 309_855_456)];
let f: VoiceFacts; let calls: Record<string, unknown>[]; let kicks: number;
beforeEach(() => { files.clear(); forgetConsent(); resetSeam(); calls = []; kicks = 0; f = { ...noVoice(), plan: plan() };
  bindNative({ later: async (r) => { calls.push(r); if (r.op === 'embedState') return { present: false }; return {}; } }); onKickSearch(() => { kicks += 1; }); });

describe('what is offered', () => {
  test('D1 building the view calls nothing and fetches nothing', () => {
    const v = downloadsView(f, false, 0); expect(v.groups.map((g) => g.id)).toEqual(['voice', 'voice-rest', 'search']); expect(calls.length).toBe(0);
  });
  test('D2 each group lists its files with the plan\'s sizes, their sum and the host; mobile data is said before starting', () => {
    const g = downloadsView(f, false, 0).groups;
    expect(g[0].lines).toEqual(['The voice: Zeta voice, 351 MB']); expect(g[0].total).toBe('351 MB'); expect(g[0].hostLine).toContain('from Example Hub');
    expect(g[1].lines.length).toBe(3); expect(g[1].total).toBe('1.1 GB'); expect(g[2].total).toBe('310 MB');
    expect(g[0].startLabel).toBe('Download on Wi-Fi'); expect(g[0].mobileLine).toBe('');
    const m = downloadsView({ ...f, onWiFi: false }, false, 0).groups[1];
    expect(m.mobileLine).toBe('You are on mobile data. This will use about 1.1 GB of your plan.'); expect(m.startLabel).toBe('Wait for Wi-Fi'); expect(m.mobileLabel).toBe('Download now');
  });
  test('D3 a group is ready only when every file is; a newer model is an offer, not a silent fetch', () => {
    f = { ...f, plan: plan().map((r) => (r.group === 'voice' ? { ...r, ready: true } : r)), voiceReady: true };
    let g = downloadsView(f, false, 0).groups; expect(g[0].ready).toBe(true); expect(g[0].total).toBe('Nothing to download.'); expect(g[1].ready).toBe(false);
    f = { ...f, plan: plan().map((r) => (r.group === 'voice' ? { ...r, ready: true } : r)).map((r) => (r.stage === 'voice' ? { ...r, ready: false, model: 'Zeta voice 2', bytes: 400_000_000, sizeLabel: '400 MB' } : r)) };
    g = downloadsView(f, false, 0).groups; expect(g[0].ready).toBe(false); expect(g[0].lines[0]).toBe('The voice: Zeta voice 2, 400 MB'); expect(calls.length).toBe(0);
    expect(downloadsView(f, true, 0).groups[2].ready).toBe(true);                                       // the search file is on the phone: the native state says so
  });
  test('D6 sizes are worded as the native side words them', () => { expect(sizeLabel(0)).toBe('Built in'); expect(sizeLabel(2_000_000)).toBe('2 MB'); expect(sizeLabel(639_446_688)).toBe('639 MB'); expect(sizeLabel(1_810_000_000)).toBe('1.8 GB'); });
});

describe('the press', () => {
  test('D4 Wi-Fi: records the agreement and starts that group alone', async () => {
    await downloadsAct('start', 'voice', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toMatchObject({ stages: ['voice'] }); expect((await modelsAgreement(storage))).toEqual({ ok: true, mobile: false });
    calls = []; await downloadsAct('start', 'voice-rest', f, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toMatchObject({ stages: ['transcriber', 'bounds', 'responder'] });
  });
  test('D4 mobile data: the Wi-Fi press starts nothing; the mobile press records the mobile agreement and starts', async () => {
    const m = { ...f, onWiFi: false };
    await downloadsAct('start', 'voice', m, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect((await consentView(storage)).models).toBe('');
    await downloadsAct('startMobile', 'voice', m, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeDefined(); expect((await modelsAgreement(storage))).toEqual({ ok: true, mobile: true });
  });
  test('D4 search: the press records the agreement and asks the search pass to run; no voice setup starts', async () => {
    await downloadsAct('start', 'search', f, storage); expect(kicks).toBe(1); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect((await modelsAgreement(storage)).ok).toBe(true);
    expect(files.has(CONSENT_FILE)).toBe(true);
  });
  test('D5 a voice-only run does not make the other group look busy', () => {
    const rows = ['voice', 'transcriber'].map((stage) => ({ stage, name: '', model: '', sizeLabel: '', state: stage === 'voice' ? 'downloading' : 'waiting', statusLine: '', numbers: '', progress: 0.2, isFallback: false, canRetry: false, canFallback: false }));
    const g = downloadsView({ ...f, setupStarted: true, runStages: ['voice'], rows }, false, 0).groups;
    expect(g[0].busy).toBe(true); expect(g[1].busy).toBe(false);
  });
  test('D5 a press while it is downloading, on a group that is ready, or on an unknown group starts nothing', async () => {
    const busy: VoiceFacts = { ...f, setupStarted: true, rows: [{ stage: 'voice', name: '', model: '', sizeLabel: '', state: 'downloading', statusLine: '', numbers: '', progress: 0.4, isFallback: false, canRetry: false, canFallback: false }] };
    await downloadsAct('start', 'voice', busy, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined();
    expect(downloadsView(busy, false, 0).groups[0]).toMatchObject({ busy: true, status: 'Downloading, 40%.' });
    const done = { ...f, plan: plan().map((r) => ({ ...r, ready: true })) }; await downloadsAct('start', 'voice', done, storage); await downloadsAct('start', 'nonsense', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect(files.size).toBe(0);
  });
});
