// Learning the person's voice, the page's side (voicelearn.ts): Y1 to Y7.
import { describe, expect, test } from 'bun:test';
import { CONFIRM_SENTENCE, MIC_OFF_LINE, MIC_PRE_LINE, SENTENCES, learnAct, learnPage, noEnrol, type Enrol } from './voicelearn.ts';

const e = (o: Partial<Enrol> = {}): Enrol => ({ ...noEnrol(), ready: true, ...o });
const page = (en: Partial<Enrol>, mic = 'heard') => learnPage({ enrol: e(en), mic });

describe('what the page says', () => {
  test('Y1 three sentences in a fixed order, then one to confirm; the confirming one is never shown before the voice is built', () => {
    const shown = [0, 1, 2].map((n) => page({ phase: 'idle', taken: n, again: n === 0 }, 'heard')).map((v) => v.sentence);        // taken 0 with nothing heard yet is the intro, which shows none
    expect(n3(shown)).toEqual([SENTENCES[0], SENTENCES[1], SENTENCES[2]]);
    for (const p of ['idle', 'listening', 'building']) for (let t = 0; t < 3; t++) expect(page({ phase: p, taken: t }).sentence).not.toBe(CONFIRM_SENTENCE);
    const c = page({ phase: 'confirm', taken: 3, learned: true }); expect(c.sentence).toBe(CONFIRM_SENTENCE); expect(c.primaryOp).toBe('read');
    expect(page({ phase: 'listening', taken: 1 })).toMatchObject({ step: 'reading', count: 'Sentence 2 of 3', busy: true });
  });
  test('Y2 the intro starts nothing by itself and shows the pre-prompt line only when the microphone is unasked', () => {
    const asked = page({}, 'unasked'); expect(asked).toMatchObject({ step: 'intro', primary: 'Continue', primaryOp: 'start' }); expect(asked.line).toContain(MIC_PRE_LINE);
    const known = page({}, 'heard'); expect(known).toMatchObject({ step: 'intro', primary: 'Start' }); expect(known.line).not.toContain(MIC_PRE_LINE);
    expect(MIC_PRE_LINE).toBe('To talk to the radio or record a take, Autoradio 1 needs your microphone and speech recognition.');
  });
  test('Y3 a sentence that was not heard is asked again, with a line saying so', () => {
    const v = page({ phase: 'idle', taken: 0, again: true }); expect(v.step).toBe('reading'); expect(v.notice).toBe('I did not hear that. Read it again.'); expect(v.sentence).toBe(SENTENCES[0]);
    expect(page({ phase: 'confirm', taken: 3, again: true }).notice).toContain('did not hear');
  });
  test('Y4 stored only when the check matched; unsure and failed offer learning again and never say stored', () => {
    expect(page({ phase: 'stored', taken: 3, learned: true, call: 'match' })).toMatchObject({ step: 'stored', stored: true, title: 'Your voice is stored' });
    for (const p of ['unsure', 'failed']) { const v = page({ phase: p }); expect(v.stored).toBe(false); expect(v.title.toLowerCase()).not.toContain('stored'); expect(v.primaryOp).toBe('again'); }
    for (const p of ['idle', 'listening', 'building', 'confirm', 'confirming']) expect(page({ phase: p, taken: 1 }).stored).toBe(false);
  });
  test('Y5 no model on the phone: it says so and offers no press', () => {
    const v = learnPage({ enrol: { ...noEnrol(), ready: false }, mic: 'heard' }); expect(v.step).toBe('noModel'); expect(v.primary).toBe(''); expect(v.primaryOp).toBe('');
  });
  test('Y7 a refused microphone shows the declined line and no button that asks again', () => {
    const v = page({}, 'refused'); expect(v.line).toBe(MIC_OFF_LINE); expect(v.primary).toBe(''); expect(MIC_OFF_LINE).toBe('Microphone is off for Autoradio 1. You can turn it on in Settings.');
  });
});
const n3 = (a: string[]) => a;

describe('the presses', () => {
  const calls: string[] = [];
  const later = async (r: { op: string }) => { calls.push(r.op); return {}; };
  test('Y2/Y6 start and learn again call native start (it deletes the old profile) before listening; read only listens; nothing else opens the microphone', async () => {
    calls.length = 0; await learnAct('start', { enrol: e(), mic: 'heard' }, later); expect(calls).toEqual(['obVoiceStart', 'obVoiceBegin']);
    calls.length = 0; await learnAct('again', { enrol: e({ phase: 'unsure' }), mic: 'heard' }, later); expect(calls).toEqual(['obVoiceStart', 'obVoiceBegin']);
    calls.length = 0; await learnAct('read', { enrol: e({ phase: 'idle', taken: 1 }), mic: 'heard' }, later); expect(calls).toEqual(['obVoiceBegin']);
    calls.length = 0; await learnAct('forget', { enrol: e({ phase: 'stored', learned: true }), mic: 'heard' }, later); expect(calls).toEqual(['obVoiceForget']);
    calls.length = 0; await learnAct('close', { enrol: e({ phase: 'listening' }), mic: 'heard' }, later); expect(calls).toEqual(['obVoiceStop']);
  });
  test('Y2/Y5/Y7 a press the page does not offer calls nothing (no model, a refused microphone, the wrong op for the step)', async () => {
    calls.length = 0;
    await learnAct('start', { enrol: { ...noEnrol(), ready: false }, mic: 'heard' }, later); await learnAct('start', { enrol: e(), mic: 'refused' }, later);
    await learnAct('read', { enrol: e(), mic: 'heard' }, later); await learnAct('again', { enrol: e({ phase: 'listening' }), mic: 'heard' }, later); await learnAct('start', { enrol: e(), mic: 'heard' }, null);
    expect(calls).toEqual([]);
  });
});
