// First run under bun test: the steps, their order and what is kept (O1 to O14), against a fake voice side and a fake storage.
import { beforeEach, describe, expect, test } from 'bun:test';
import { onboardingAct, onboardingPage, forgetOnboarding, wipeOnboarding, parseProgress, startStep, noVoice, PROGRESS_FILE, EDIT, SETUP,
  MIC_REFUSED_LINE, MIC_SILENT_LINE, TALK_REFUSED_LINE, HEARING_AGAIN_LINE, type Env, type VoiceFacts, type Device } from './onboarding.ts';
import { resetSeam, nextGeneration, bindNative } from './api.ts';
import { firstRunOver, liveEnv, NOT_RECORDED_LINE } from './onboarding.ts';
import type { OfferJob } from './offers.ts';
import { consentPick, forgetConsent } from './consent.ts';
import { FAILED_LINE, type Writer } from './edition-writer.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
const kept = (): Record<string, unknown> => JSON.parse(new TextDecoder().decode(files.get(PROGRESS_FILE)!));

let v: VoiceFacts; let calls: { op: string; args?: Record<string, unknown> }[]; let closed: string[]; let opened: string[]; let edition: unknown;
const device: Device = { facts: async () => ({ ...v }), op: async (op, args) => { calls.push({ op, args }); } };
const env = (o: Partial<Env> = {}): Env => ({ storage, device, get: async () => edition, closePage: async (a) => { closed.push(a); }, openPage: async (a) => { opened.push(a); }, ...o });
const act = (op: string, arg = '', topics = '', top = SETUP, e: Env = env()) => onboardingAct(op, arg, topics, top, e);
const ops = (): string[] => calls.map((c) => c.op);
beforeEach(() => { files.clear(); forgetOnboarding(); forgetConsent(); resetSeam(); v = { ...noVoice(), offers: [{ job: 'voice', group: 'voice', host: 'Example Hub', options: [{ id: 'v', model: 'V', bytes: 1_000_000, sizeLabel: '1 MB', recommended: true, ready: false }] }] }; calls = []; closed = []; opened = []; edition = { found: false }; });

describe('the steps', () => {
  test('O1 the walk, in order, ends at done with the edition made and the station released', async () => {
    expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('welcome');
    expect((await act('begin')).step).toBe('setup'); expect(ops()).toContain('obSetupBegin');
    v = { ...v, setupStarted: true, setupFinished: true };
    expect((await onboardingPage(v, 1, SETUP, env())).step).toBe('mic');                         // O3
    expect(kept().setupDone).toBe(true);
    expect((await act('micNext')).step).toBe('hearing'); expect(ops()).toContain('obHearingPlay');
    expect((await act('hearing', 'true')).step).toBe('learn');                                   // O5, O17
    expect((await act('learnDone')).step).toBe('voice'); expect(ops()).toContain('obEnrolDiscard');
    v = { ...v, voicePicked: 1 };
    expect((await onboardingPage(v, 2, SETUP, env())).step).toBe('ready');                       // O6
    expect((await act('topics')).step).toBe('topics');
    const done = await act('topicsDone', '', 'Space\nChess');
    expect(done).toMatchObject({ step: 'done', editionStatus: 'ready', editionDone: true, editionLines: ['What I follow', 'Space', 'Chess'], card: '', aim: '' });
    expect(calls.filter((c) => c.op === 'obMirror').pop()?.args).toEqual({ finished: true });    // O14
    expect(calls.find((c) => c.op === 'obNotebookTopics')?.args).toEqual({ topics: ['Space', 'Chess'] });
    expect(kept()).toMatchObject({ topicsStarted: true, editionDone: true });
  });
  test('O1 a button that does not belong to the step changes nothing', async () => {
    for (const op of ['micNext', 'hearing', 'mic', 'retry', 'topicsDone']) { const s = await act(op, 'true'); expect(s.step).toBe('welcome'); }
    expect(calls.filter((c) => c.op !== 'obMirror' && c.op !== 'obTalk').length).toBe(0);
    expect(files.size).toBe(0);
  });
  test('O2 off Wi-Fi it asks first; yes starts once; a second begin while it runs is a no-op', async () => {
    v = { ...v, onWiFi: false };
    const asked = await act('begin'); expect(asked).toMatchObject({ step: 'setup', askMobile: true }); expect(ops()).not.toContain('obSetupBegin');
    const go = await act('begin', 'true'); expect(go.askMobile).toBe(false); expect(ops().filter((o) => o === 'obSetupBegin').length).toBe(1);
    v = { ...v, setupStarted: true }; await act('begin', 'true'); expect(ops().filter((o) => o === 'obSetupBegin').length).toBe(1);
  });
  test('O2 on Wi-Fi it starts without asking; retry and fallback are narrow calls', async () => {
    expect((await act('begin')).askMobile).toBe(false);
    await act('retry', 'voice'); await act('fallback', 'ears');
    expect(calls.filter((c) => c.op === 'obSetupRetry' || c.op === 'obSetupFallback').map((c) => c.args)).toEqual([{ stage: 'voice' }, { stage: 'ears' }]);
  });
  test('O3 a finished setup is not lost by a restart', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env());
    forgetOnboarding(); v = noVoice();
    expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('ready');
  });
  test('O4 a refused microphone is kept and said; Talk then says so and starts nothing; a silent one says so', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env());
    await act('mic'); expect(ops()).toContain('obMicStart');
    v = { ...v, mic: 'refused' };
    expect(await onboardingPage(v, 1, SETUP, env())).toMatchObject({ step: 'mic', message: MIC_REFUSED_LINE }); expect(kept().micRefused).toBe(true);
    await act('micNext'); await act('hearing', 'true'); await act('learnDone'); await act('topics'); calls = [];
    const t = await act('talk'); expect(t.message).toBe(TALK_REFUSED_LINE); expect(ops().filter((o) => o === 'obTalk').length).toBe(0);
    v = { ...v, mic: 'silent' }; forgetOnboarding();
    await act('begin'); v = { ...v, setupFinished: true, mic: 'silent' };
    expect((await onboardingPage(v, 2, SETUP, env())).message).toBe(MIC_SILENT_LINE);
  });
  test('O4 a heard microphone is not "refused" any more', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true, mic: 'refused' }; await onboardingPage(v, 0, SETUP, env()); expect(kept().micRefused).toBe(true);
    v = { ...v, mic: 'heard', heard: '' }; const p = await onboardingPage(v, 1, SETUP, env());
    expect(kept().micRefused).toBe(false); expect(p.heard).toBe('Your microphone is working.');
    expect((await onboardingPage({ ...v, heard: 'hello there' }, 2, SETUP, env())).heard).toBe('hello there');
  });
  test('O5 "No" plays it again and says what to try; "Yes" goes on', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env()); await act('micNext'); calls = [];
    const no = await act('hearing', 'false'); expect(no).toMatchObject({ step: 'hearing', message: HEARING_AGAIN_LINE }); expect(ops()).toContain('obHearingPlay');
    expect((await act('hearing', 'true')).step).toBe('learn'); expect(kept().setupDone).toBe(true);
  });
  test('Talk toggles the voice at the topic screen', async () => {
    await act('topics'); calls = [];
    await act('talk'); expect(calls.find((c) => c.op === 'obTalk')?.args).toEqual({ on: true });
    v = { ...v, talking: true }; calls = []; await act('talk'); expect(calls.find((c) => c.op === 'obTalk')?.args).toEqual({ on: false });
  });
});

describe('leaving and coming back', () => {
  test('O7 skip stops the voice and is kept; a skipped run that finished setup comes back at ready', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env());
    const s = await act('skip'); expect(s).toMatchObject({ step: 'skipped', skipped: true, card: '' }); expect(ops()).toContain('obStop'); expect(kept().skipped).toBe(true);
    expect(calls.filter((c) => c.op === 'obMirror').pop()?.args).toEqual({ finished: true });
    forgetOnboarding(); calls = [];
    expect((await onboardingPage(v, 1, SETUP, env())).step).toBe('skipped');
    const b = await act('begin'); expect(b.step).toBe('ready'); expect(ops()).not.toContain('obSetupBegin'); expect(kept().skipped).toBe(false);
  });
  test('O7 a skip before any setup begins at setup again', async () => {
    await act('skip'); expect((await act('begin')).step).toBe('setup'); expect(ops()).toContain('obSetupBegin'); expect(kept().skipped).toBe(false);
  });
  test('O12 the prompt card says where the listener left off', async () => {
    expect((await onboardingPage(v, 0, SETUP, env())).card).toBe('Make your edition');
    v = { ...v, setupStarted: true }; expect((await onboardingPage(v, 0, SETUP, env())).card).toBe('Finish voice setup');
    await act('topics'); expect((await onboardingPage(v, 0, SETUP, env())).card).toBe('Choose your topics');
    forgetOnboarding(); expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('topics');          // coming back shows the topics step
  });
  test('O13 a corrupt, empty or wrong-typed file starts fresh and never throws', () => {
    for (const t of [undefined, '', 'not json', '[]', '{"skipped":"yes","voicePicked":-3}', 'null']) expect(parseProgress(t)).toEqual({ offered: false, skipped: false, setupDone: false, micRefused: false, topicsStarted: false, editionDone: false, voicePicked: 0 });
    expect(startStep(parseProgress('{"editionDone":true,"skipped":true}'))).toBe('done');
    expect(startStep(parseProgress('{"topicsStarted":true,"editionDone":true}'))).toBe('done');
    expect(startStep(parseProgress('{"topicsStarted":true}'))).toBe('topics');
    expect(startStep(parseProgress('{"skipped":true,"setupDone":true}'))).toBe('skipped');
  });
  test('O13 sign-out removes the file and forgets the run; a file that cannot go says so', async () => {
    await act('topics'); expect(files.has(PROGRESS_FILE)).toBe(true);
    await wipeOnboarding(storage); expect(files.has(PROGRESS_FILE)).toBe(false);
    expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('welcome');
    files.set(PROGRESS_FILE, new TextEncoder().encode('{}'));
    const stuck = { fs: { rm: async () => { throw new Error('busy'); }, stat: async () => ({}) } } as never;
    await expect(wipeOnboarding(stuck)).rejects.toThrow('busy');
  });
});

describe('the edition', () => {
  test('O8 an empty list is the general edition; a failure and a deadline say so and keep the topics', async () => {
    await act('topics');
    const general = await act('topicsDone', ''); expect(general.editionLines).toEqual(['What I follow', 'The whole paper, until I say otherwise.']);
    forgetOnboarding(); files.clear(); calls = []; await act('topics');
    const boom: Writer = async () => { throw new Error('no'); };
    const failed = await act('topicsDone', '', 'Space', SETUP, env({ writer: boom })); expect(failed).toMatchObject({ step: 'writing', editionStatus: 'failed', editionError: FAILED_LINE });
    expect(ops()).not.toContain('obNotebookTopics');
    const never: Writer = (_t, _p, signal) => new Promise((_r, rej) => { signal.addEventListener('abort', () => rej(new Error('aborted'))); });
    const slow = await act('editionRetry', '', 'Space', SETUP, env({ writer: never, deadlineSeconds: 0.05 })); expect(slow.editionStatus).toBe('failed'); expect(slow.editionError).toContain('too long');
    expect((await act('editionRetry', '', 'Space')).step).toBe('done');                              // Retry from the failed state succeeds
  });
  test('O15 an edition that finishes after the account moved on is dropped: no file, no notebook write, no memory for the next account', async () => {
    await act('topics');
    let release: () => void = () => undefined;
    const slow: Writer = async (topics) => { await new Promise<void>((r) => { release = r; }); return { title: 'T', intro: '', writingLines: [], sections: [{ heading: 'H', lines: topics }] }; };
    const pending = act('topicsDone', '', 'Space', SETUP, env({ writer: slow }));
    await new Promise((r) => setTimeout(r, 10));
    nextGeneration(); files.clear(); calls = [];
    release(); await pending;
    expect(ops()).not.toContain('obNotebookTopics'); expect(files.has(PROGRESS_FILE)).toBe(false);
    expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('welcome');
  });
  test('O9 a Continue while one is writing is ignored; Continue anyway is the general edition', async () => {
    await act('topics');
    let release: () => void = () => undefined; let started = 0;
    const gate: Writer = async (t) => { started++; await new Promise<void>((r) => { release = r; }); return { title: 'My edition', intro: '', writingLines: [], sections: [{ heading: 'h', lines: t }] }; };
    const first = act('topicsDone', '', 'A', SETUP, env({ writer: gate }));
    await new Promise((r) => setTimeout(r, 5));
    const second = await act('topicsDone', '', 'A', SETUP, env({ writer: gate })); expect(started).toBe(1); expect(second.editionStatus).toBe('writing');
    release(); expect((await first).step).toBe('done');
    forgetOnboarding(); files.clear();
    const any = await act('continueAnyway'); expect(any).toMatchObject({ step: 'done', editionStatus: 'ready', editionDone: true });
  });
  test('O10 editing after the first run leaves the step; Done closes the page; Leave closes it without writing', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env());
    const before = (await onboardingPage(v, 0, EDIT, env()));
    expect(before.editing).toBe(true); expect(before.step).toBe('mic');
    const done = await act('editDone', '', 'Space', EDIT); expect(done.step).toBe('mic'); expect(done.editionStatus).toBe('ready'); expect(done.editionDone).toBe(true);
    expect(closed).toEqual([EDIT]);
    closed = []; const left = await act('editLeave', '', '', EDIT); expect(left.editionStatus).toBe('ready'); expect(closed).toEqual([EDIT]);
    forgetOnboarding(); files.clear(); closed = [];
    expect((await act('editLeave', '', '', EDIT)).editionStatus).toBe('');
    expect((await onboardingPage(v, 0, SETUP, env())).editing).toBe(false);
  });
  test('the done page names the edition address the reader draws', async () => { expect((await onboardingPage(v, 0, SETUP, env())).editionAddress).toBe(EDIT); });
});

describe('the offer', () => {
  test('O11 a fresh install is offered the first run once; one with an edition is not; no answer offers it', async () => {
    await act('offer'); expect(opened).toEqual([SETUP]); expect(kept().offered).toBe(true);
    await act('offer'); expect(opened).toEqual([SETUP]);
    forgetOnboarding(); files.clear(); opened = []; edition = { found: true, doc_id: 'd1' };
    const found = await act('offer'); expect(opened).toEqual([]); expect(found).toMatchObject({ step: 'done', editionDone: true }); expect(kept().editionDone).toBe(true);
    forgetOnboarding(); files.clear(); edition = 'garbage';
    await act('offer'); expect(opened).toEqual([SETUP]);
    forgetOnboarding(); files.clear(); opened = [];
    await act('offer', '', '', SETUP, env({ get: async () => { throw new Error('offline'); } })); expect(opened).toEqual([SETUP]);
  });
  test('O11 a skipped or finished first run is not offered again', async () => {
    await act('skip'); opened = []; await act('offer'); expect(opened).toEqual([]);
  });
  test('without a native module the page still answers, and every act does nothing native', async () => {
    const e = env({ device: null });
    expect((await onboardingPage(undefined, 0, SETUP, e)).step).toBe('welcome');
    expect((await act('begin', '', '', SETUP, e)).step).toBe('welcome');        // nothing offered, nothing started (O19)
  });
});

describe('the progress file is read once, and a failed read is not "no progress"', () => {
  const logged: string[] = [];
  const spy = (): (() => void) => { const o = console.log; console.log = (...a: unknown[]) => { logged.push(a.join(' ')); }; return () => { console.log = o; }; };
  test('an absent file (ENOENT) is no progress and says nothing', async () => {
    const un = spy(); logged.length = 0;
    const page = await onboardingPage(v, 0, SETUP, env());
    un();
    expect(page.step).toBe('welcome');
    expect(logged.filter((l) => l.startsWith('onboarding:'))).toEqual([]);
  });
  test('a seeded file is read', async () => {
    files.set(PROGRESS_FILE, new TextEncoder().encode(JSON.stringify({ offered: true, skipped: false, setupDone: false, micRefused: false, topicsStarted: false, editionDone: true, voicePicked: 0 })));
    expect((await onboardingPage(v, 0, SETUP, env())).step).toBe('done');
  });
  test('a step taken while the read is in flight is saved after it, on top of what was stored', async () => {
    files.set(PROGRESS_FILE, new TextEncoder().encode(JSON.stringify({ offered: true, skipped: false, setupDone: true, micRefused: false, topicsStarted: false, editionDone: false, voicePicked: 2 })));
    let release!: () => void; const gate = new Promise<void>((r) => { release = r; });
    const slow = { fs: { ...(storage as never as { fs: object }).fs, readFile: async (p: string) => { await gate; return files.get(p)!; } } } as never;
    const first = onboardingPage(v, 0, SETUP, env({ storage: slow }));
    const step = act('skip', '', '', SETUP, env({ storage: slow }));            // taken before the read resolves
    release(); await first; await step;
    expect(kept()).toMatchObject({ skipped: true, setupDone: true, voicePicked: 2 });    // the stored values are kept, the step is added
  });
  test('a read that fails refuses the step visibly, writes nothing, and the next try reads', async () => {
    files.set(PROGRESS_FILE, new TextEncoder().encode(JSON.stringify({ offered: true, skipped: false, setupDone: true, micRefused: false, topicsStarted: false, editionDone: false, voicePicked: 2 })));
    let fail = true; const writes: string[] = [];
    const flaky = { fs: { ...(storage as never as { fs: object }).fs, readFile: async (p: string) => { if (fail) throw new Error('dropped'); return files.get(p)!; }, atomicWriteFile: async (p: string) => { writes.push(p); } } } as never;
    const un = spy();
    const refused = await act('skip', '', '', SETUP, env({ storage: flaky }));
    un();
    expect(refused.message).toContain('could not be read'); expect(writes).toEqual([]);
    fail = false;
    expect((await act('skip', '', '', SETUP, env({ storage: flaky }))).step).toBe('skipped'); expect(writes).toContain(PROGRESS_FILE);
  });
});

const liveAgreeSpy = (asked: string[][]) => async (m: boolean, ids?: string[]): Promise<boolean> => { asked.push(ids ?? []); return (await liveEnv(storage).agree!(m, ids ?? [])) === true; };
describe('first run without an account, and the agreement', () => {
  test('O15 with no key the first run is over for the station; with a locked key it is not', async () => {
    bindNative({ later: async (r) => (r.op === 'keystate' ? { state: 'absent' } : {}) });
    expect(await firstRunOver(storage)).toBe(true);
    forgetOnboarding(); bindNative({ later: async (r) => (r.op === 'keystate' ? { state: 'unreadable' } : {}) });
    expect(await firstRunOver(storage)).toBe(false);
  });
  test('O16 begin records the agreement: Wi-Fi on Wi-Fi, mobile data on mobile data after the yes; asking first records nothing', async () => {
    const agreed: boolean[] = [];
    const e = env({ agree: async (m) => { agreed.push(m); } });
    await act('begin', '', '', SETUP, e); expect(agreed).toEqual([false]); expect(ops()).toContain('obSetupBegin');
    forgetOnboarding(); files.clear(); calls = []; agreed.length = 0; v = { ...v, onWiFi: false };
    await act('begin', '', '', SETUP, e); expect(agreed).toEqual([]); expect(ops()).not.toContain('obSetupBegin');
    await act('begin', 'true', '', SETUP, e); expect(agreed).toEqual([true]); expect(ops()).toContain('obSetupBegin');
  });
  test('O16b the first run agrees to the files it shows, model by model (the same keys Voice and models uses), never to a model it did not show', async () => {
    const asked: string[][] = [];
    const o = (job: string, group: string, ids: string[]): OfferJob => ({ job, group, host: 'Example Hub', options: ids.map((id, i) => ({ id, model: id, bytes: 1_000_000, sizeLabel: '1 MB', recommended: i === 0, ready: false })) });
    v = { ...v, offers: [o('voice', 'voice', ['a', 'b']), o('transcriber', 'voice-rest', ['c']), o('speaker', 'voice-rest', ['print']), o('search', 'search', ['e'])] };
    await act('begin', '', '', SETUP, env({ agree: async (_m, ids) => { asked.push(ids ?? []); } }));
    expect(asked).toEqual([['a', 'c', 'print']]);
  });
  test('O16c every model shown is already on the phone (downloaded from Voice and models): the first run still begins, and agrees to nothing new', async () => {
    const o = (job: string, group: string, id: string): OfferJob => ({ job, group, host: 'Example Hub', options: [{ id, model: id, bytes: 1_000_000, sizeLabel: '1 MB', recommended: true, ready: true }] });
    v = { ...v, offers: [o('voice', 'voice', 'a'), o('transcriber', 'voice-rest', 'c')] };
    expect(await liveEnv(storage).agree!(false, [])).toBe(true);                                  // nothing to agree to is not a failure to agree
    const asked: string[][] = [];
    const r = await act('begin', '', '', SETUP, env({ agree: liveAgreeSpy(asked) }));
    expect(asked).toEqual([[]]); expect(r.message).not.toBe(NOT_RECORDED_LINE); expect(r.step).toBe('setup');
  });
});

describe('the voice learning step and the choices', () => {
  const offerOf = (job: string, group: string, ids: string[], rec: number): OfferJob => ({ job, group, host: 'Example Hub', options: ids.map((id, i) => ({ id, model: id, bytes: 1_000_000, sizeLabel: '1 MB', recommended: i === rec, ready: false })) });
  test('O17 the learn step ends with learnDone into the voice step (native told to stop listening); Not now at the microphone skips to the voice step without opening it', async () => {
    await act('begin'); v = { ...v, setupStarted: true, setupFinished: true }; await onboardingPage(v, 0, SETUP, env());
    await act('micNext'); await act('hearing', 'true'); expect((await onboardingPage(v, 1, SETUP, env())).step).toBe('learn');
    expect(await act('learnDone')).toMatchObject({ step: 'voice' }); expect(ops()).toContain('obEnrolDiscard'); expect(ops()).not.toContain('obEnrolBegin');   // the ops native really answers (a stale name is dropped silently)
    expect((await act('learnDone')).step).toBe('voice');                                           // not a step it is on: nothing moves
    forgetOnboarding(); files.clear(); calls = [];
    await act('begin'); await onboardingPage({ ...v, setupFinished: true }, 0, SETUP, env()); v = { ...v, setupFinished: true };
    await onboardingPage(v, 0, SETUP, env()); const s = await act('micSkip'); expect(s.step).toBe('voice'); expect(ops()).not.toContain('obMicStart');
  });
  test('O18 begin hands native the chosen option of every job (the pick, else the recommendation) and the voice-print model', async () => {
    v = { ...v, offers: [offerOf('voice', 'voice', ['a', 'b'], 0), offerOf('transcriber', 'voice-rest', ['c', 'd'], 1), offerOf('speaker', 'voice-rest', ['print'], 0), offerOf('search', 'search', ['e'], 0)] };
    await consentPick('transcriber', 'c', storage);
    await act('begin');
    expect(calls.find((c) => c.op === 'obSetupBegin')?.args).toEqual({ only: true, stages: ['voice', 'transcriber'], picks: { voice: 'a', transcriber: 'c' }, speaker: true });
  });
  test('O19 the press runs only the shown stages (the decision stage too when it is shown), never the configurator, and refuses when nothing is offered', async () => {
    v = { ...v, offers: [offerOf('voice', 'voice', ['a'], 0), offerOf('decision', 'voice-rest', ['q'], 0), offerOf('responder', 'voice-rest', ['l'], 0)] };
    await act('begin'); expect(calls.find((c) => c.op === 'obSetupBegin')?.args).toMatchObject({ only: true, stages: ['voice', 'decision', 'responder'] });
    forgetOnboarding(); files.clear(); calls = []; v = { ...noVoice(), offers: [] };
    const r = await act('begin'); expect(ops()).not.toContain('obSetupBegin'); expect(r.step).toBe('welcome'); expect(r.message).toContain('not available');
  });
  // The setup page's total (the mobile-data card too) is the picks' sum, the welcome's number, not native's plan for the whole ladder (review 3).
  test('O20 the total on the setup page is what will be fetched: the chosen options, each file once, the voice print too', async () => {
    const big = (job: string, group: string, id: string, bytes: number): OfferJob => ({ job, group, host: 'H', options: [{ id, model: id, bytes, sizeLabel: '', recommended: true, ready: false }] });
    v = { ...v, totalLabel: 'About 2.5 GB. On Wi-Fi this takes a few minutes.', offers: [big('voice', 'voice', 'a', 351_000_000), big('responder', 'voice-rest', 'r', 1_260_000_000), big('speaker', 'voice-rest', 'print', 8_000_000)] };
    expect((await onboardingPage(v, 0, SETUP, env())).totalLabel).toBe('About 1.6 GB. On Wi-Fi this takes a few minutes.');
    v = { ...v, onWiFi: false }; expect((await onboardingPage(v, 0, SETUP, env())).totalLabel).toBe('About 1.6 GB. You are on mobile data, so I will ask before I start.');
    v = { ...v, offers: [] }; expect((await onboardingPage(v, 0, SETUP, env())).totalLabel).toBe('About 2.5 GB. On Wi-Fi this takes a few minutes.');          // nothing offered: native's own words
  });
  test('O21 the first run never fetches a withheld option, even one picked on Settings; with nothing else offered for a group it starts nothing for it', async () => {
    const w = (job: string, group: string, id: string): OfferJob => ({ job, group, host: 'H', options: [{ id, model: id, bytes: 350_000_000, sizeLabel: '350 MB', recommended: false, ready: false, withheld: true } as never] });
    v = { ...v, offers: [w('voice', 'voice', 'kokoro-x'), offerOf('bounds', 'voice-rest', ['gate'], 0)] };
    await consentPick('voice', 'kokoro-x', storage);
    await act('begin');
    expect(calls.find((c) => c.op === 'obSetupBegin')?.args).toEqual({ only: true, stages: ['bounds'], picks: { bounds: 'gate' }, speaker: false });
  });
});

