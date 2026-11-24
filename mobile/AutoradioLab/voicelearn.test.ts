// Learning the person's voice, the page's side (voicelearn.ts): Y1 to Y9, against a fake native that records its calls.
import { beforeEach, describe, expect, test } from 'bun:test';
import { CONFIRM_SENTENCE, MIC_OFF_LINE, MIC_PRE_LINE, SENTENCES, forgetLearn, learnAct, learnPage, noEnrol, wipeLearn, type Enrol } from './voicelearn.ts';
import { bindNative, resetSeam } from './api.ts';

let calls: string[]; let usable = true; let built = true; let verdict = 'match'; let stored = true; let kept = 0;
const later = async (r: { op: string }) => {
  calls.push(r.op);
  switch (r.op) {
    case 'obEnrolKeep': kept += 1; return { kept, usable };
    case 'obEnrolBuild': return { ok: built };
    case 'obEnrolCheck': return { call: verdict };
    case 'obEnrolStore': return { ok: stored };
    case 'obEnrolDiscard': kept = 0; return {};
    default: return {};
  }
};
const en = (o: Partial<Enrol> = {}): Enrol => ({ ...noEnrol(), ready: true, ...o });
const F = (o: Partial<Enrol> = {}, mic = 'heard') => ({ enrol: en(o), mic });
beforeEach(() => { forgetLearn(); resetSeam(); calls = []; usable = true; built = true; verdict = 'match'; stored = true; kept = 0; });
const view = (f = F()) => learnPage(f, later);
const press = (op: string, f = F()) => learnAct(op, f, later);
/** One sentence read: the press, then native says the take ended (usable or not). */
async function read(op: string, usableTake = true): Promise<void> { await press(op); await view(F({ state: 'done', usable: usableTake })); }

describe('the sentences and the pre-prompt', () => {
  test('Y1 three sentences in a fixed order, then the confirming one only after the voice is built', async () => {
    expect((await view()).step).toBe('intro');
    const shown: string[] = [];
    await press('start'); shown.push((await view(F({ state: 'listening' }))).sentence);
    await view(F({ state: 'done', usable: true })); shown.push((await view()).sentence);
    await press('read'); await view(F({ state: 'done', usable: true })); shown.push((await view()).sentence);
    expect(shown).toEqual([SENTENCES[0], SENTENCES[1], SENTENCES[2]]);
    await press('read'); expect((await view(F({ state: 'done', usable: true }))).step).toBe('confirm');
    const c = await view(); expect(c.sentence).toBe(CONFIRM_SENTENCE); expect(c.primaryOp).toBe('read'); expect(calls.filter((x) => x === 'obEnrolBuild').length).toBe(1);
  });
  test('Y2 nothing opens the microphone before a press; the pre-prompt line shows only while the microphone is unasked', async () => {
    const asked = await view(F({}, 'unasked')); expect(asked).toMatchObject({ step: 'intro', primary: 'Continue', primaryOp: 'start' }); expect(asked.line).toContain(MIC_PRE_LINE);
    expect((await view(F({}, 'heard'))).line).not.toContain(MIC_PRE_LINE);
    expect(MIC_PRE_LINE).toBe('To talk to the radio or record a take, Autoradio 1 needs your microphone and speech recognition.');
    expect(calls).toEqual([]);                                                                       // looking at the page calls nothing
    await press('start'); expect(calls).toEqual(['obEnrolDiscard', 'obEnrolBegin']);
  });
});

describe('what each take does', () => {
  test('Y3 a sentence that was not heard is asked again, keeps nothing, and the same sentence stays', async () => {
    await press('start'); const v = await view(F({ state: 'done', usable: false }));
    expect(v).toMatchObject({ step: 'reading', sentence: SENTENCES[0], notice: 'I did not hear that. Read it again.' }); expect(calls).not.toContain('obEnrolKeep'); expect(calls).toContain('obEnrolDiscardTake');
  });
  test('Y4 stored only after the check matched and native stored it; the confirming sentence checks before it stores', async () => {
    await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    await press('read'); calls.length = 0; const v = await view(F({ state: 'done', usable: true }));
    expect(v).toMatchObject({ step: 'stored', stored: true }); expect(calls).toEqual(['obEnrolCheck', 'obEnrolStore']);
  });
  test('Y4 an unsure check stores nothing, drops what is in progress and offers Learn again; so does a build that failed', async () => {
    await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    await press('read'); verdict = 'mismatch'; calls.length = 0; const v = await view(F({ state: 'done', usable: true }));
    expect(v).toMatchObject({ step: 'unsure', stored: false, primaryOp: 'again' }); expect(calls).toEqual(['obEnrolCheck', 'obEnrolDiscard']); expect(calls).not.toContain('obEnrolStore');
    forgetLearn(); calls = []; built = false;
    await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    expect(await view()).toMatchObject({ step: 'failed', stored: false, primaryOp: 'again' }); expect(calls).toContain('obEnrolDiscard');
  });
  test('Y4 a match whose store failed is not stored', async () => {
    stored = false; await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    await press('read'); expect((await view(F({ state: 'done', usable: true }))).stored).toBe(false);
  });
});

describe('the edges', () => {
  test('Y5 no model: it says so and a press calls nothing', async () => {
    const v = await view({ enrol: noEnrol(), mic: 'heard' }); expect(v).toMatchObject({ step: 'noModel', primary: '' });
    await learnAct('start', { enrol: noEnrol(), mic: 'heard' }, later); await learnAct('read', { enrol: noEnrol(), mic: 'heard' }, later); expect(calls).toEqual([]);
  });
  test('Y6 Learn again, Not now and Forget drop what is in progress; Forget deletes the stored voice; Done keeps it', async () => {
    await press('start'); calls.length = 0; await press('close'); expect(calls).toEqual(['obEnrolDiscard']); expect((await view()).step).toBe('intro');
    calls.length = 0; await press('forget'); expect(calls).toEqual(['obVoiceForget']);
    calls.length = 0; await press('rewind'); expect(calls).toEqual(['obEnrolDiscard']);
    await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    await press('read'); await view(F({ state: 'done', usable: true })); expect((await view()).step).toBe('stored'); calls.length = 0;
    await press('close'); expect((await view()).step).toBe('stored'); expect(calls).not.toContain('obVoiceForget');
  });
  test('Y7 a refused microphone says so; a failed capture ends the attempt with a way to try again; a take always has Not now', async () => {
    expect(await view(F({}, 'refused'))).toMatchObject({ line: MIC_OFF_LINE, primary: '', secondary: 'Not now' }); expect(MIC_OFF_LINE).toBe('Microphone is off for Autoradio 1. You can turn it on in Settings.');
    await press('start'); const v = await view(F({ state: 'listening' })); expect(v).toMatchObject({ busy: true, secondary: 'Not now' });
    const f = await view(F({ state: 'error', reason: 'The microphone could not be used.' })); expect(f).toMatchObject({ step: 'failed', primaryOp: 'again' }); expect(f.line).toContain('microphone');
  });
  test('Y8 one native step at a time: looking at the facts twice at once keeps the take once', async () => {
    await press('start'); const f = F({ state: 'done', usable: true });
    await Promise.all([view(f), view(f), view(f)]); expect(calls.filter((x) => x === 'obEnrolKeep').length).toBe(1);
  });
  test('Y9 a sign-out forgets it: the next account does not see a stored voice it never made', async () => {
    await press('start'); for (let i = 0; i < 3; i++) { await view(F({ state: 'done', usable: true })); if (i < 2) await press('read'); }
    await press('read'); await view(F({ state: 'done', usable: true })); expect((await view()).step).toBe('stored');
    bindNative({ later: async (r) => { calls.push(r.op); return {}; } }); calls.length = 0; await wipeLearn(); expect(calls).toContain('obEnrolDiscard');
    expect((await view()).step).toBe('intro');
  });
});
