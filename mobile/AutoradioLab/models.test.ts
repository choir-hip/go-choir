// Settings > Voice and models under bun test (models.ts): the one page that holds every model of every job.
// Invented model names and sizes on purpose: nothing in models.ts may know a model's name.
//
// Failure list (each line has a test below):
//  M1  Building the view calls nothing and fetches nothing.
//  M2  Every job is a section that lists ALL the options the list holds, each with name, size, credit, the recommended tag and its state; streaming recognisers are the "live words" section.
//  M3  A row's state is real: not downloaded, downloading with the native progress, on the phone, in use, failed with its reason.
//  M4  Download records the agreement for THAT model only and starts exactly that model; another model's agreement is never recorded by it.
//  M5  On mobile data the Wi-Fi press starts nothing and records nothing; the mobile press records the mobile agreement; the row says the size and "mobile data" before.
//  M6  A press while a download runs, on a model that is on the phone, or on an id that is not offered starts nothing.
//  M7  Search: the agreement under "search" and the search pass asked to run; the voice-print model starts alone with no stage.
//  M8  Use: only for a model that is on the phone, not in use and offered; it is kept as the pick and handed to native; never for a model that is not on the phone.
//  M9  Remove: only for a removable model; native deletes it.
//  M10 Test: only for a model that is on the phone; native is asked for that model; the result (words, time) is shown on the row; a test never downloads.
//  M11 A press reads the facts as they are now (the network moved since the page was drawn).
//  M12 Nothing starts when the agreement could not be recorded.
//  M13 No word on the page sends the person to another page for a download or for its size.
//  M14 The first run's lines are the recommended option of each job, the total counts each file once; a withheld option is not the first run's.
import { beforeEach, describe, expect, test } from 'bun:test';
import { modelsAct, modelsView, firstRunLines, forgetModelTests } from './models.ts';
import { onKickSearch, onTestSearch } from './search-kick.ts';
import { consentPicks, forgetConsent, modelsAgreement, CONSENT_FILE } from './consent.ts';
import { noVoice, type VoiceFacts } from './onboarding.ts';
import { sizeLabel, type OfferJob, type Option } from './offers.ts';
import { bindNative, nextGeneration, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
const opt = (id: string, model: string, bytes: number, extra: Partial<Option> = {}): Option => ({ id, model, bytes, sizeLabel: sizeLabel(bytes), recommended: false, ready: false, ...extra });
const job = (j: string, options: Option[]): OfferJob => ({ job: j, group: j === 'voice' ? 'voice' : j === 'search' ? 'search' : 'voice-rest', host: 'Example Hub', options });
const offers = (): OfferJob[] => [
  job('transcriber', [opt('ear-two', 'Ear two', 600_000_000, { recommended: true, credit: 'Ear two by Example Lab, CC BY 4.0.' }), opt('ear-one', 'Ear one', 481_400_000), opt('ear-live', 'Ear live', 250_000_000, { streams: true })]),
  job('bounds', [opt('gate', 'Gate', 2_000_000, { recommended: true })]),
  job('voice', [opt('zeta', 'Zeta voice', 351_000_000, { recommended: true }), opt('zeta-lite', 'Zeta lite', 300_000_000, { withheld: true })]),
  job('responder', [opt('lm', 'Small LM', 640_000_000, { recommended: true }), opt('lm-big', 'Big LM', 1_850_000_000)]),
  job('decision', [opt('lm', 'Small LM', 640_000_000, { recommended: true }), opt('lm-tiny', 'Tiny LM', 229_000_000)]),
  job('speaker', [opt('print', 'Print', 8_000_000, { recommended: true })]),
  job('search', [opt('embedder', 'Embedder', 309_855_456, { recommended: true })]),
];
let embedPresent = false;
let f: VoiceFacts; let calls: Record<string, unknown>[]; let kicks: number; let live: Record<string, unknown> | null = null;
const begins = () => calls.filter((c) => c.op === 'obSetupBegin');
beforeEach(() => {
  files.clear(); forgetConsent(); resetSeam(); calls = []; kicks = 0; live = null; embedPresent = false; f = { ...noVoice(), offers: offers() };
  bindNative({ later: async (r) => { if (r.op === 'obFacts') return live ?? {}; calls.push(r); if (r.op === 'embedState') return { present: embedPresent }; return {}; } });
  onKickSearch(() => { kicks += 1; });
});
const view = (g: VoiceFacts = f, search: boolean | null = false, picks: Record<string, string> = {}) => modelsView(g, search, 0, picks);
const row = (v: ReturnType<typeof view>, section: string, id: string) => v.jobs.find((j) => j.section === section)!.options.find((o) => o.id === id)!;

describe('the tabs', () => {
  test('M15 one tab per kind of model, in plain words, each section on the tab of its job; a tab with nothing on offer is not drawn', () => {
    const v = view();
    expect(v.tabs.map((x) => [x.id, x.title])).toEqual([['hearing', 'Hearing'], ['voice', 'Voice'], ['answering', 'Answering'], ['deciding', 'Deciding'], ['search', 'Search'], ['voiceprint', 'Your voice']]);
    expect(v.jobs.map((s) => [s.section, s.tab])).toEqual([['transcriber', 'hearing'], ['live', 'hearing'], ['bounds', 'hearing'], ['voice', 'voice'], ['responder', 'answering'], ['decision', 'deciding'], ['speaker', 'voiceprint'], ['search', 'search']]);
    expect(view({ ...f, offers: offers().filter((j) => j.job !== 'search' && j.job !== 'speaker') }).tabs.map((x) => x.id)).toEqual(['hearing', 'voice', 'answering', 'deciding']);
    expect(view({ ...f, offers: [] }).tabs).toEqual([]);
  });
  test('M16 every row of every tab is the same row: the same fields, so one component draws them all, and a failed download shows its last reason on its own row', () => {
    const g: VoiceFacts = { ...f, modelRuns: [{ id: 'zeta', state: 'failed', progress: 0, line: 'The download failed (offline).' }] };
    const keys = (r: object) => Object.keys(r).sort().join(',');
    const rows = view(g).jobs.flatMap((s) => s.options);
    expect(new Set(rows.map(keys)).size).toBe(1);
    expect(rows.find((r) => r.id === 'zeta' && r.section === 'voice')).toMatchObject({ state: 'failed', line: 'The download failed (offline).', canDownload: true });
  });
});

describe('what is listed', () => {
  test('M1 building the view calls nothing', () => { view(); expect(calls.length).toBe(0); expect(files.size).toBe(0); });
  test('M2 every job is a section with all its options; streaming recognisers are the live-words section', () => {
    const v = view();
    expect(v.jobs.map((j) => j.section)).toEqual(['transcriber', 'live', 'bounds', 'voice', 'responder', 'decision', 'speaker', 'search']);
    expect(v.jobs.find((j) => j.section === 'transcriber')!.options.map((o) => o.name)).toEqual(['Ear two', 'Ear one']);
    expect(v.jobs.find((j) => j.section === 'live')!.options.map((o) => o.name)).toEqual(['Ear live']);
    expect(v.jobs.find((j) => j.section === 'voice')!.options.map((o) => o.name)).toEqual(['Zeta voice', 'Zeta lite']);
    const r = row(v, 'transcriber', 'ear-two');
    expect(r.size).toBe('600 MB'); expect(r.tag).toContain('Recommended for this phone'); expect(r.credit).toBe('Ear two by Example Lab, CC BY 4.0.'); expect(r.source).toBe('Example Hub');
    expect(row(v, 'transcriber', 'ear-one').tag).not.toContain('Recommended');
    expect(row(v, 'decision', 'lm').job).toBe('decision'); expect(row(v, 'responder', 'lm').job).toBe('responder');   // one file, two jobs: each section has its own row
    expect(view({ ...f, offers: [] }).jobs).toEqual([]);                                                              // a job the list does not offer is not drawn
  });
  test('M3 a row says the truth: not downloaded, downloading with the native progress, on the phone, in use, failed', () => {
    expect(row(view(), 'bounds', 'gate')).toMatchObject({ state: 'none', line: 'Not downloaded', canDownload: true, canUse: false, canTest: false });
    const g: VoiceFacts = { ...f, modelRuns: [{ id: 'gate', state: 'downloading', progress: 0.42, line: '' }], setupStarted: true };
    expect(row(view(g), 'bounds', 'gate')).toMatchObject({ state: 'downloading', line: 'Downloading, 42%', progress: 0.42, canDownload: false });
    const on = (id: string, extra: Partial<Option>): VoiceFacts => ({ ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === id ? { ...o, ...extra } : o)) })) });
    expect(row(view(on('gate', { ready: true })), 'bounds', 'gate')).toMatchObject({ state: 'ready', line: 'On this phone', canUse: true, canTest: true, canDownload: false });
    expect(row(view(on('gate', { ready: true, inUse: true })), 'bounds', 'gate')).toMatchObject({ state: 'inuse', line: 'In use', canUse: false, canTest: true });
    const failed: VoiceFacts = { ...f, modelRuns: [{ id: 'gate', state: 'failed', progress: 0, line: 'The download did not finish.' }] };
    expect(row(view(failed), 'bounds', 'gate')).toMatchObject({ state: 'failed', line: 'The download did not finish.', canDownload: true });
  });
  test('M3 another model\'s download makes this row wait, honestly (the phone fetches one at a time)', () => {
    const g: VoiceFacts = { ...f, setupStarted: true, setupFinished: false, modelRuns: [{ id: 'gate', state: 'downloading', progress: 0.1, line: '' }] };
    expect(row(view(g), 'voice', 'zeta')).toMatchObject({ canDownload: false, line: 'Waiting for the download in progress' });
  });
  test('M3 the on-the-phone state of search is the native state of the file', () => { expect(row(view(f, true), 'search', 'embedder').state).toBe('ready'); expect(row(view(f, false), 'search', 'embedder').state).toBe('none'); });
  test('M3 an optional (withheld) option is listed and downloadable, and says it is optional', () => {
    const r = row(view(), 'voice', 'zeta-lite'); expect(r.tag).toContain('Optional'); expect(r.canDownload).toBe(true);
  });
  test('M10 a test result is on its row: running, then the words and the time', () => {
    const on: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === 'ear-two' ? { ...o, ready: true } : o)) })), modelTests: [{ key: 'transcriber:ear-two', state: 'recording', line: 'Say something now.', ms: 0 }] };
    expect(row(view(on), 'transcriber', 'ear-two')).toMatchObject({ testing: true, testLine: 'Say something now.', canTest: false });
    const done: VoiceFacts = { ...on, modelTests: [{ key: 'transcriber:ear-two', state: 'done', line: 'Heard: "hello there"', ms: 1200 }] };
    expect(row(view(done), 'transcriber', 'ear-two')).toMatchObject({ testing: false, testLine: 'Heard: "hello there" (1.2 s)', canTest: true });
  });
  test('M13 no word on the page sends the person to another page', () => {
    const all = JSON.stringify(view({ ...f, modelRuns: [{ id: 'gate', state: 'failed', progress: 0, line: 'Did not finish.' }] }));
    expect(/Downloads|Settings >/.test(all)).toBe(false);
  });
});

describe('Download', () => {
  test('M4 on Wi-Fi it records the agreement for that model only and starts exactly that model', async () => {
    await modelsAct('download', 'voice', 'zeta', f, storage);
    expect(begins()).toEqual([{ op: 'obSetupBegin', only: true, stages: ['voice'], picks: { voice: 'zeta' }, speaker: false, row: true }]);
    expect(await modelsAgreement(storage, 'zeta')).toEqual({ ok: true, mobile: false });
    expect((await modelsAgreement(storage, 'zeta-lite')).ok).toBe(false); expect((await modelsAgreement(storage, 'voice')).ok).toBe(false); expect((await modelsAgreement(storage, 'search')).ok).toBe(false);
  });
  test('M4 a press is one download: a second model needs its own press and its own agreement', async () => {
    await modelsAct('download', 'transcriber', 'ear-one', f, storage);
    expect(begins()[0]).toMatchObject({ stages: ['transcriber'], picks: { transcriber: 'ear-one' } });
    expect((await modelsAgreement(storage, 'ear-two')).ok).toBe(false); expect((await modelsAgreement(storage, 'ear-live')).ok).toBe(false);
  });
  test('M5 mobile data: the Wi-Fi press starts nothing; the mobile press records the mobile agreement and starts', async () => {
    const m = { ...f, onWiFi: false };
    const r = row(view(m), 'voice', 'zeta'); expect(r.mobileLine).toBe('You are on mobile data. This will use about 351 MB of your plan.'); expect(r.downloadLabel).toBe('Download on mobile data'); expect(row(view(), 'voice', 'zeta').mobileLine).toBe('');
    await modelsAct('download', 'voice', 'zeta', m, storage); expect(begins().length).toBe(0); expect((await modelsAgreement(storage, 'zeta')).ok).toBe(false);
    await modelsAct('downloadMobile', 'voice', 'zeta', m, storage); expect(begins().length).toBe(1); expect(await modelsAgreement(storage, 'zeta')).toEqual({ ok: true, mobile: true });
  });
  test('M5 a Wi-Fi press never lowers a mobile agreement', async () => {
    await modelsAct('downloadMobile', 'voice', 'zeta', { ...f, onWiFi: false }, storage); await modelsAct('download', 'voice', 'zeta', f, storage);
    expect(await modelsAgreement(storage, 'zeta')).toEqual({ ok: true, mobile: true });
  });
  test('M6 a press while a download runs, on a model on the phone, or on an id that is not offered starts nothing', async () => {
    const busy: VoiceFacts = { ...f, setupStarted: true, setupFinished: false, modelRuns: [{ id: 'gate', state: 'downloading', progress: 0.4, line: '' }] };
    await modelsAct('download', 'voice', 'zeta', busy, storage); expect(begins().length).toBe(0);
    const done: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => ({ ...o, ready: true })) })) };
    await modelsAct('download', 'voice', 'zeta', done, storage); expect(begins().length).toBe(0);
    await modelsAct('download', 'voice', 'nope', f, storage); await modelsAct('download', 'nonsense', 'zeta', f, storage); expect(begins().length).toBe(0); expect(files.size).toBe(0);
  });
  test('M7 search: the agreement under "search" and the search pass asked; no stage starts', async () => {
    await modelsAct('download', 'search', 'embedder', f, storage);
    expect(kicks).toBe(1); expect(begins().length).toBe(0); expect(await modelsAgreement(storage, 'search')).toEqual({ ok: true, mobile: false }); expect(files.has(CONSENT_FILE)).toBe(true);
  });
  test('M7 the voice-print model starts alone: no stage, the speaker flag', async () => {
    await modelsAct('download', 'speaker', 'print', f, storage);
    expect(begins()).toEqual([{ op: 'obSetupBegin', only: true, stages: [], picks: {}, speaker: true, row: true }]); expect(await modelsAgreement(storage, 'print')).toEqual({ ok: true, mobile: false });
  });
  test('M11 a press reads the live facts: the phone is on mobile data now, so the Wi-Fi press starts nothing', async () => {
    live = { ...f, onWiFi: false };
    await modelsAct('download', 'voice', 'zeta', f, storage); expect(begins().length).toBe(0); expect(files.size).toBe(0);
    await modelsAct('downloadMobile', 'voice', 'zeta', f, storage); expect(begins().length).toBe(1); expect((await modelsAgreement(storage, 'zeta')).mobile).toBe(true);
  });
  test('M12 nothing starts when the agreement could not be recorded', async () => {
    files.set(CONSENT_FILE, new TextEncoder().encode('{}')); const bad = { fs: { ...(storage as { fs: Record<string, unknown> }).fs, readFile: async () => { throw new Error('EIO'); } } } as never;
    await modelsAct('download', 'voice', 'zeta', f, bad); expect(begins().length).toBe(0);
  });
});

describe('the microphone line before a recording Test', () => {
  const ready = (ids: string[]): VoiceFacts => ({ ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (ids.includes(o.id) ? { ...o, ready: true } : o)) })) });
  let mic = 'granted';
  beforeEach(() => { mic = 'granted'; bindNative({ later: async (r) => { if (r.op === 'obFacts') return live ?? {}; if (r.op === 'obMicState') return { state: mic }; calls.push(r); if (r.op === 'embedState') return { present: embedPresent }; return {}; } }); });
  test('M17 unasked: the first press shows the plain line and starts nothing; the second press goes on (the system prompt follows it)', async () => {
    mic = 'unasked';
    let v = await modelsAct('test', 'transcriber', 'ear-one', ready(['ear-one']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(0);
    expect(row(v, 'transcriber', 'ear-one').testLine).toContain('needs your microphone');
    v = await modelsAct('test', 'transcriber', 'ear-one', ready(['ear-one']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest')).toEqual([{ op: 'obModelTest', job: 'transcriber', id: 'ear-one' }]);
  });
  test('M17 refused: the declined line and never a start; granted: starts at once; a job that does not listen never asks', async () => {
    mic = 'refused';
    const v = await modelsAct('test', 'bounds', 'gate', ready(['gate']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(0); expect(row(v, 'bounds', 'gate').testLine).toContain('Microphone is off');
    mic = 'granted'; calls = [];
    await modelsAct('test', 'speaker', 'print', ready(['print']), storage); expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(1);
    mic = 'unasked'; calls = [];
    await modelsAct('test', 'responder', 'lm-big', ready(['lm-big']), storage); expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(1);
  });
});

describe('a row that failed', () => {
  test('M3c a failed row keeps its reason and can be pressed again, and it does not hold the other rows (the run is over)', () => {
    const g: VoiceFacts = { ...f, setupStarted: true, setupFinished: true, modelRuns: [{ id: 'gate', state: 'failed', progress: 0, line: 'The download failed (offline).' }] };
    const v = view(g);
    expect(row(v, 'bounds', 'gate')).toMatchObject({ state: 'failed', line: 'The download failed (offline).', canDownload: true });
    expect(row(v, 'voice', 'zeta')).toMatchObject({ canDownload: true, line: 'Not downloaded' });
  });
  test('M13b a sign-out forgets the test lines (they belong to the account that ran them)', async () => {
    embedPresent = true; const was = onTestSearch(async (_q, _s) => 'It found 1 story or document on this phone for: x');
    const g: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === 'embedder' ? { ...o, ready: true } : o)) })) };
    try { await modelsAct('test', 'search', 'embedder', g, storage); expect(row(view(g, true), 'search', 'embedder').testLine).toContain('It found 1 story'); forgetModelTests(); expect(row(view(g, true), 'search', 'embedder').testLine).toBe(''); }
    finally { onTestSearch(was); }
  });
});

describe('Use, Remove, Test', () => {
  const ready = (ids: string[], extra: Partial<Option> = {}): VoiceFacts => ({ ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (ids.includes(o.id) ? { ...o, ready: true, ...extra } : o)) })) });
  test('M8 Use is for a model on the phone that is not in use: it is kept and handed to native; nothing is downloaded', async () => {
    await modelsAct('use', 'transcriber', 'ear-one', ready(['ear-one']), storage);
    expect(calls.filter((c) => c.op === 'obUseModel')).toEqual([{ op: 'obUseModel', job: 'transcriber', id: 'ear-one', live: false }]); expect(begins().length).toBe(0);
    expect(await consentPicks(storage)).toEqual({ transcriber: 'ear-one' });
  });
  test('M8 Use on a live-words model says so, and a model that is not on the phone, one in use or one not offered is refused', async () => {
    await modelsAct('use', 'live', 'ear-live', ready(['ear-live']), storage);
    expect(calls.filter((c) => c.op === 'obUseModel')).toEqual([{ op: 'obUseModel', job: 'transcriber', id: 'ear-live', live: true }]);
    calls = [];
    await modelsAct('use', 'voice', 'zeta', f, storage); await modelsAct('use', 'voice', 'zeta', ready(['zeta'], { inUse: true }), storage); await modelsAct('use', 'voice', 'nope', ready(['zeta']), storage);
    expect(calls.filter((c) => c.op === 'obUseModel').length).toBe(0);
  });
  test('M9 Remove only for a removable model', async () => {
    await modelsAct('remove', 'bounds', 'gate', ready(['gate'], { removable: true }), storage);
    expect(calls.filter((c) => c.op === 'obRemoveModel')).toEqual([{ op: 'obRemoveModel', id: 'gate' }]); calls = [];
    await modelsAct('remove', 'bounds', 'gate', ready(['gate']), storage); await modelsAct('remove', 'bounds', 'gate', f, storage); expect(calls.filter((c) => c.op === 'obRemoveModel').length).toBe(0);
    expect(row(view(ready(['gate'], { removable: true })), 'bounds', 'gate').canRemove).toBe(true); expect(row(view(ready(['gate'])), 'bounds', 'gate').canRemove).toBe(false);
  });
  test('M10 Test asks native for exactly that model, only when it is on the phone, and never downloads', async () => {
    await modelsAct('test', 'responder', 'lm-big', ready(['lm-big']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest')).toEqual([{ op: 'obModelTest', job: 'responder', id: 'lm-big' }]); expect(begins().length).toBe(0); expect(files.size).toBe(0);
    calls = []; await modelsAct('test', 'responder', 'lm-big', f, storage); await modelsAct('test', 'responder', 'nope', ready(['lm-big']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(0);
  });
  test('M10 a live-words test is the recogniser\'s test (native job transcriber)', async () => {
    await modelsAct('test', 'live', 'ear-live', ready(['ear-live']), storage);
    expect(calls.filter((c) => c.op === 'obModelTest')).toEqual([{ op: 'obModelTest', job: 'transcriber', id: 'ear-live' }]);
  });
});

describe('the search test (shared code runs it)', () => {
  test('M10 Test on the search model runs the fixed query in Exact, not native, and shows what it found', async () => {
    embedPresent = true;
    const realNow = Date.now; Date.now = () => { throw new Error('a data source may not read the clock'); };          // the bake refuses Date.now(): the test fails if the page reads it
    const asked: string[] = []; const was = onTestSearch(async (q, _s) => { asked.push(q); return 'It found 3 stories and documents on this phone for: ' + q; });
    const g: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === 'embedder' ? { ...o, ready: true } : o)) })) };
    let v: Awaited<ReturnType<typeof modelsAct>>; try { v = await modelsAct('test', 'search', 'embedder', g, storage); } finally { Date.now = realNow; onTestSearch(was); }
    expect(asked.length).toBe(1); expect(calls.filter((c) => c.op === 'obModelTest').length).toBe(0);
    expect(row(v, 'search', 'embedder')).toMatchObject({ testing: false }); expect(row(v, 'search', 'embedder').testLine).toContain('It found 3 stories and documents');
  });
});

describe('the search test across a sign-out', () => {
  test('M13c a search test that finishes after the account left writes nothing', async () => {
    embedPresent = true;
    const was = onTestSearch(async (_q, _s) => { nextGeneration(); return 'It found 3 stories and documents on this phone for: x'; });     // the account leaves while the query runs
    const g: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === 'embedder' ? { ...o, ready: true } : o)) })) };
    try { await modelsAct('test', 'search', 'embedder', g, storage); expect(row(view(g, true), 'search', 'embedder').testLine).toBe(''); }
    finally { onTestSearch(was); }
  });
});

describe('two accounts and the search test', () => {
  test('M13d account A\'s late search result does not erase account B\'s line', async () => {
    embedPresent = true; const gate: { open: () => void } = { open: () => {} };
    let calls = 0;
    const was = onTestSearch(async (_q, _s) => { calls += 1; if (calls === 1) { await new Promise<void>((r) => { gate.open = r; }); return 'A late'; } return 'B result'; });
    const g: VoiceFacts = { ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => (o.id === 'embedder' ? { ...o, ready: true } : o)) })) };
    try {
      const a = modelsAct('test', 'search', 'embedder', g, storage);                    // account A starts a search test and waits
      await new Promise((r) => setTimeout(r, 5)); nextGeneration(); forgetModelTests();         // A leaves; B signs in
      await modelsAct('test', 'search', 'embedder', g, storage);                         // B runs and finishes
      gate.open(); await a;                                                              // A's result arrives late
      expect(row(view(g, true), 'search', 'embedder').testLine).toBe('B result');
    } finally { onTestSearch(was); }
  });
});

describe('the first run', () => {
  test('M14 the lines are the recommended option of each job, the total counts a file once; a withheld option is not shown', () => {
    const l = firstRunLines({ ...f }, {});
    expect(l.lines.map((x) => x.text)).toEqual(['Ear two, 600 MB', 'Gate, 2 MB', 'Zeta voice, 351 MB', 'Small LM, 640 MB', 'Print, 8 MB']);   // not search; the decision shares the responder's file
    expect(l.total).toBe('1.6 GB');
    expect(firstRunLines({ ...f, offers: offers().map((j) => ({ ...j, options: j.options.map((o) => ({ ...o, recommended: o.id === 'zeta-lite' })) })) }, {}).lines.some((x) => x.text.startsWith('Zeta lite'))).toBe(false);
  });
  test('M14 a pick the person made in Voice and models is the first run\'s line', () => {
    expect(firstRunLines(f, { transcriber: 'ear-one' }).lines[0].text).toBe('Ear one, 481 MB');
  });
});
