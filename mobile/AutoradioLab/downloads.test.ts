// The model downloads under bun test (downloads.ts): D1 to D6.
import { beforeEach, describe, expect, test } from 'bun:test';
import { downloadsAct, downloadsView, sizeLabel } from './downloads.ts';
import { onKickSearch } from './search-kick.ts';
import { consentPicks, consentView, forgetConsent, modelsAgreement, CONSENT_FILE } from './consent.ts';
import { noVoice, type VoiceFacts } from './onboarding.ts';
import type { OfferJob } from './offers.ts';
import { bindNative, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
// Invented model names and sizes on purpose: nothing in downloads.ts may know a model's name.
const opt = (id: string, model: string, bytes: number, recommended = false, ready = false) => ({ id, model, bytes, sizeLabel: sizeLabel(bytes), recommended, ready });
const job = (j: string, group: string, options: ReturnType<typeof opt>[]): OfferJob => ({ job: j, group, host: 'Example Hub', options });
const offers = (): OfferJob[] => [
  job('voice', 'voice', [opt('zeta', 'Zeta voice', 351_000_000, true), opt('zeta-lite', 'Zeta lite', 300_000_000)]),
  job('transcriber', 'voice-rest', [opt('ear-two', 'Ear two', 600_000_000, true), opt('ear-one', 'Ear one', 481_400_000), opt('ear-small', 'Ear small', 250_000_000)]),
  job('bounds', 'voice-rest', [opt('gate', 'Gate', 2_000_000, true)]),
  job('responder', 'voice-rest', [opt('lm', 'Small LM', 640_000_000, true)]),
  job('speaker', 'voice-rest', [opt('print', 'Print', 8_000_000, true)]),
  job('search', 'search', [opt('embedder', 'Embedder', 309_855_456, true)]),
];
let f: VoiceFacts; let calls: Record<string, unknown>[]; let kicks: number; let live: Record<string, unknown> | null = null;
beforeEach(() => { files.clear(); forgetConsent(); resetSeam(); calls = []; kicks = 0; f = { ...noVoice(), offers: offers() };
  live = null;
  bindNative({ later: async (r) => { if (r.op === 'obFacts') return live ?? {}; calls.push(r); if (r.op === 'embedState') return { present: false }; return {}; } }); onKickSearch(() => { kicks += 1; }); });

describe('what is offered', () => {
  test('D1 building the view calls nothing and fetches nothing', () => {
    const v = downloadsView(f, false, 0); expect(v.groups.map((g) => g.id)).toEqual(['voice', 'voice-rest', 'search']); expect(calls.length).toBe(0);
  });
  test('D2/D7 each job lists its options with the list\'s sizes, the recommended one tagged and chosen; the total is what is chosen, not the sum of options', () => {
    const g = downloadsView(f, false, 0).groups;
    expect(g[0].jobs[0].options.map((o) => o.text)).toEqual(['Zeta voice, 351 MB', 'Zeta lite, 300 MB']);
    expect(g[0].jobs[0].options.map((o) => o.tag)).toEqual(['Recommended for this phone', '']); expect(g[0].jobs[0].options.map((o) => o.selected)).toEqual([true, false]);
    expect(g[0].total).toBe('351 MB'); expect(g[0].hostLine).toContain('from Example Hub');
    expect(g[1].total).toBe('1.3 GB');                                                                  // 600 + 2 + 640 + 8 MB: the recommended recogniser, not all three
    expect(g[2].total).toBe('310 MB');
    expect(g[0].startLabel).toBe('Download on Wi-Fi'); expect(g[0].mobileLine).toBe('');
    const m = downloadsView({ ...f, onWiFi: false }, false, 0).groups[1];
    expect(m.mobileLine).toBe('You are on mobile data. This will use about 1.3 GB of your plan.'); expect(m.startLabel).toBe('Wait for Wi-Fi'); expect(m.mobileLabel).toBe('Download now');
  });
  test('D7 a pick changes the chosen option and the total; a pick that is no longer offered falls back to the recommendation', () => {
    let g = downloadsView(f, false, 0, { transcriber: 'ear-small' }).groups[1];
    expect(g.jobs[0].options.map((o) => o.selected)).toEqual([false, false, true]); expect(g.total).toBe('900 MB');
    g = downloadsView(f, false, 0, { transcriber: 'gone' }).groups[1]; expect(g.jobs[0].options.map((o) => o.selected)).toEqual([true, false, false]);
  });
  test('D3 a group is ready only when what is chosen is on the phone; a newer model is an offer, not a silent fetch', () => {
    const on = (id: string) => offers().map((j) => ({ ...j, options: j.options.map((o) => (j.job === 'voice' && o.id === id ? { ...o, ready: true } : o)) }));
    f = { ...f, offers: on('zeta'), voiceReady: true };
    let g = downloadsView(f, false, 0).groups; expect(g[0].ready).toBe(true); expect(g[0].total).toBe('Nothing to download.'); expect(g[1].ready).toBe(false);
    g = downloadsView(f, false, 0, { voice: 'zeta-lite' }).groups; expect(g[0].ready).toBe(false); expect(g[0].total).toBe('300 MB'); expect(calls.length).toBe(0);
    expect(downloadsView(f, true, 0).groups[2].ready).toBe(true);                                       // the search file is on the phone: the native state says so
  });
  test('D6 sizes are worded as the native side words them', () => { expect(sizeLabel(0)).toBe('Built in'); expect(sizeLabel(2_000_000)).toBe('2 MB'); expect(sizeLabel(639_446_688)).toBe('639 MB'); expect(sizeLabel(1_810_000_000)).toBe('1.8 GB'); });
  test('D3 a job with no option that fits this phone says it keeps what is built in, and needs nothing', () => {
    f = { ...f, offers: offers().map((j) => (j.job === 'responder' ? { ...j, options: j.options.map((o) => ({ ...o, recommended: false })) } : j)) };
    const g = downloadsView(f, false, 0).groups[1]; const r = g.jobs.find((j) => j.job === 'responder')!;
    expect(r.ready).toBe(true); expect(r.none).toContain('built in'); expect(g.total).toBe('610 MB');
  });
});

describe('the press', () => {
  test('D4/D8 Wi-Fi: records the agreement and starts that group alone, with exactly the chosen option of each job', async () => {
    await downloadsAct('start', 'voice', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toMatchObject({ only: true, stages: ['voice'], picks: { voice: 'zeta' }, speaker: false }); expect((await modelsAgreement(storage, 'voice'))).toEqual({ ok: true, mobile: false });
    calls = []; await downloadsAct('pick', 'transcriber', f, storage, 'ear-small'); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined();     // a pick fetches nothing
    await downloadsAct('start', 'voice-rest', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toMatchObject({ only: true, stages: ['transcriber', 'bounds', 'responder'], picks: { transcriber: 'ear-small', bounds: 'gate', responder: 'lm' }, speaker: true });
  });
  test('D8 a pick is kept, only for a job and an option that are on offer', async () => {
    await downloadsAct('pick', 'voice', f, storage, 'zeta-lite'); expect(await consentPicks(storage)).toEqual({ voice: 'zeta-lite' });
    await downloadsAct('pick', 'voice', f, storage, 'nope'); await downloadsAct('pick', 'nonsense', f, storage, 'zeta'); expect(await consentPicks(storage)).toEqual({ voice: 'zeta-lite' });
    forgetConsent(); expect(await consentPicks(storage)).toEqual({ voice: 'zeta-lite' });                                                              // across a restart
    expect(downloadsView(f, false, 0, await consentPicks(storage)).groups[0].jobs[0].options.map((o) => o.selected)).toEqual([false, true]);
  });
  test('D8 only the voice-print model left to fetch: the press starts it and no stage', async () => {
    f = { ...f, offers: offers().map((j) => (j.group === 'voice-rest' && j.job !== 'speaker' ? { ...j, options: j.options.map((o) => ({ ...o, ready: true })) } : j)) };
    await downloadsAct('start', 'voice-rest', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toMatchObject({ only: true, stages: [], speaker: true });
  });
  test('D4 mobile data: the Wi-Fi press starts nothing; the mobile press records the mobile agreement and starts', async () => {
    const m = { ...f, onWiFi: false };
    await downloadsAct('start', 'voice', m, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect((await modelsAgreement(storage, 'voice')).ok).toBe(false);
    await downloadsAct('startMobile', 'voice', m, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeDefined(); expect((await modelsAgreement(storage, 'voice'))).toEqual({ ok: true, mobile: true });
  });
  test('D4 search: the press records the agreement and asks the search pass to run; no voice setup starts', async () => {
    await downloadsAct('start', 'search', f, storage); expect(kicks).toBe(1); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect((await modelsAgreement(storage, 'search')).ok).toBe(true);
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
    const done = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => ({ ...o, ready: true })) })) }; await downloadsAct('start', 'voice', done, storage); await downloadsAct('start', 'nonsense', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect(files.size).toBe(0);
  });
  test('D10 agreement is per group: a voice press leaves the search model unagreed, the search press agrees only search, and a Wi-Fi press never lowers a mobile agreement', async () => {
    await downloadsAct('start', 'voice', f, storage); await downloadsAct('start', 'voice-rest', f, storage);
    expect(await modelsAgreement(storage, 'search')).toEqual({ ok: false, mobile: false });
    expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: false });
    await downloadsAct('start', 'search', f, storage); expect(await modelsAgreement(storage, 'search')).toEqual({ ok: true, mobile: false });
    const m = { ...f, onWiFi: false }; await downloadsAct('startMobile', 'voice', m, storage); await downloadsAct('start', 'voice', f, storage);
    expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: true });
  });
  test('D11 nothing starts when the agreement could not be recorded (an unreadable file)', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{}')); const bad = { fs: { ...(storage as { fs: Record<string, unknown> }).fs, readFile: async () => { throw new Error('EIO'); } } } as never;
    await downloadsAct('start', 'voice', f, bad); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined();
  });
  test('D12 the setup total is the chosen options of the two voice groups, each file once, voice-print model included; the decision job is shown and counted', () => {
    const o = offers().concat([job('decision', 'voice-rest', [opt('lm', 'Small LM', 640_000_000, true)])]);   // the same file as the responder's: one download
    const v = downloadsView({ ...f, offers: o }, false, 0);
    expect(v.setupTotal).toBe('1.6 GB');                                              // 351 voice + 600 ear + 2 gate + 640 lm (once) + 8 print = 1601 MB
    expect(v.groups[1].jobs.map((j) => j.job)).toContain('decision');
    expect(downloadsView({ ...f, offers: [] }, false, 0).setupTotal).toBe('');
  });
  test('D13 an option\'s limit (a licence) is shown with it', () => {
    const lim = 'Commercial use above a revenue line needs a licence.';
    f = { ...f, offers: [job('responder', 'voice-rest', [{ ...opt('big', 'Big LM', 1_260_000_000, true), note: lim }, opt('small', 'Small LM', 640_000_000)])] };
    const o = downloadsView(f, false, 0).groups[0].jobs[0].options; expect(o[0].note).toBe(lim); expect(o[1].note).toBe('');
  });
  test('D9 an option appears only when the list offers it: no name is known here', () => {
    const v = downloadsView(f, false, 0); const all = v.groups.flatMap((g) => g.jobs.flatMap((j) => j.options.map((o) => o.text)));
    expect(all.some((t) => /pro|fluid|parakeet|kokoro/i.test(t))).toBe(false);
  });
  // B1 (review round 2): the page was drawn on Wi-Fi and the phone moved to mobile data before the press. The press reads the facts as they are NOW.
  test('D14 a press reads the live facts: on mobile data now, the Wi-Fi press starts nothing and records nothing; the mobile press records the mobile agreement', async () => {
    live = { ...f, onWiFi: false };
    await downloadsAct('start', 'voice', f, storage);                                   // f says Wi-Fi: it is stale
    expect(calls.find((c) => c.op === 'obSetupBegin')).toBeUndefined(); expect(files.size).toBe(0);
    await downloadsAct('startMobile', 'voice', f, storage);
    expect(calls.find((c) => c.op === 'obSetupBegin')).toBeDefined(); expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: true });
  });
  test('D14 native that answers nothing: the facts the page had are used', async () => {
    live = null; await downloadsAct('start', 'voice', f, storage); expect(calls.find((c) => c.op === 'obSetupBegin')).toBeDefined();
  });
  test('D15 a licence credit (CC BY 4.0) is shown with its option: it comes from the list and this file writes none', () => {
    const credited = offers().map((j) => j.job === 'transcriber' ? { ...j, options: j.options.map((o) => o.id === 'ear-two' ? { ...o, credit: 'Ear two by Example Labs. CC BY 4.0.' } : o) } : j);
    const v = downloadsView({ ...f, offers: credited }, false, 0);
    const opts = v.groups.flatMap((g) => g.jobs).flatMap((j) => j.options);
    expect(opts.find((o) => o.id === 'ear-two')?.credit).toBe('Ear two by Example Labs. CC BY 4.0.');
    expect(opts.find((o) => o.id === 'ear-one')?.credit).toBe('');
  });
  test('D16 a "Download now" pressed while the phone is on Wi-Fi (the page was drawn before the path was known) records a Wi-Fi agreement only, never a mobile one', async () => {
    live = { ...f, onWiFi: true };
    await downloadsAct('startMobile', 'voice', { ...f, onWiFi: false }, storage);
    expect(await modelsAgreement(storage, 'voice')).toEqual({ ok: true, mobile: false });
    expect(calls.find((c) => c.op === 'obSetupBegin')).toBeDefined();
  });
  // Round 5: a library model with no local-only load is WITHHELD (no manifest yet): shown on Settings only, never chosen by default, never in the first run's total; an explicit pick on Settings fetches it.
  test('D17 a withheld option is shown with its flag, is never the default, and is not in the first run total; picking it on Settings makes it the job\'s choice', () => {
    const withheld = (id: string, bytes: number, rec = false): ReturnType<typeof opt> & { withheld: boolean } => ({ ...opt(id, id, bytes, rec), withheld: true });
    const offs = [job('voice', 'voice', [withheld('kokoro-x', 350_000_000)]), job('bounds', 'voice-rest', [opt('gate', 'Gate', 2_000_000, true)])];
    const v = downloadsView({ ...f, offers: offs }, false, 0);
    const voice = v.groups.find((g) => g.id === 'voice')!;
    expect(voice.jobs[0].options[0]).toMatchObject({ id: 'kokoro-x', withheld: true, selected: false });
    expect(voice.ready).toBe(true);                                                   // nothing is chosen for it: the system voice stays
    expect(v.setupTotal).toBe('2 MB');                                                // the welcome counts the gate alone
    const picked = downloadsView({ ...f, offers: offs }, false, 0, { voice: 'kokoro-x' });
    expect(picked.groups.find((g) => g.id === 'voice')!.ready).toBe(false); expect(picked.groups.find((g) => g.id === 'voice')!.total).toBe('350 MB');
    expect(picked.setupTotal).toBe('2 MB');                                           // and still not in the first run's
  });
  test('D18 when the voice job is withheld the first run says so in one line: the iPhone voice for now, a better one in Settings > Downloads; with a voice on offer or no voice job it says nothing', () => {
    const w = (id: string) => ({ ...opt(id, id, 350_000_000), withheld: true });
    const line = "The radio uses the iPhone's voice for now. A better voice is in Settings > Downloads.";
    expect(downloadsView({ ...f, offers: [job('voice', 'voice', [w('kokoro-x')]), job('bounds', 'voice-rest', [opt('gate', 'Gate', 2_000_000, true)])] }, false, 0).voiceNote).toBe(line);
    expect(downloadsView({ ...f, offers: [job('voice', 'voice', [opt('plain', 'Plain', 350_000_000, true), w('kokoro-x')])] }, false, 0).voiceNote).toBe('');
    expect(downloadsView({ ...f, offers: [job('bounds', 'voice-rest', [opt('gate', 'Gate', 2_000_000, true)])] }, false, 0).voiceNote).toBe('');
  });
  // Round 9 (2): Remove, on every downloaded model that is not in use.
  test('D19 a downloaded model that is not in use can be removed from its card; the press goes to native with its id; one that is in use, or not on the phone, cannot', async () => {
    const ro = (id: string, ready: boolean, removable: boolean) => ({ ...opt(id, id, 350_000_000, false, ready), removable });
    const offs = [job('voice', 'voice', [ro('mine', true, true), ro('inuse', true, false), ro('absent', false, false), ro('halfway', false, true)])];
    const v = downloadsView({ ...f, offers: offs }, false, 0);
    const o = v.groups[0].jobs[0].options;
    expect(o.map((x) => [x.id, x.removable])).toEqual([['mine', true], ['inuse', false], ['absent', false], ['halfway', true]]);                // a model that is not ready but whose files are on the phone can be removed
    for (const id of ['mine', 'inuse', 'absent', 'halfway', 'nonsense']) await downloadsAct('remove', 'voice', { ...f, offers: offs }, storage, id);
    expect(calls.filter((c) => c.op === 'obRemoveModel')).toEqual([{ op: 'obRemoveModel', id: 'mine' }, { op: 'obRemoveModel', id: 'halfway' }]);
  });
});

