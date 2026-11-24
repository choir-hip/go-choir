// The microphone, asked where it is first needed (micask.ts): the plain line before the system prompt for the bar's voice and for a take, as the first run's step already does (review 9, 2026-11-24).
import { beforeEach, describe, expect, test } from 'bun:test';
import { micPermission, micGateFor, MIC_TEXT } from './micask.ts';
import { MIC_PRE_LINE, MIC_OFF_LINE } from './voicelearn.ts';
import { bindNative, resetSeam } from './api.ts';

let calls: Record<string, unknown>[]; let answer: unknown;
beforeEach(() => { resetSeam(); calls = []; answer = { state: 'unasked' }; bindNative({ later: async (r) => { calls.push(r); return answer; } }); });

describe('the line before the microphone is asked', () => {
  test('M1 an unasked microphone shows the pre-prompt line first; a refused one shows the declined line; a granted one shows nothing', () => {
    expect(micGateFor('unasked')).toBe('micpre'); expect(micGateFor('refused')).toBe('micoff'); expect(micGateFor('granted')).toBe('');
    expect(micGateFor('')).toBe(''); expect(micGateFor('something new')).toBe('');                 // an answer we do not know asks nothing
  });
  test('M2 the words are the first run\'s own: one pre-prompt line with Continue and Not now, one declined line', () => {
    expect(MIC_TEXT.pre).toBe(MIC_PRE_LINE); expect(MIC_TEXT.off).toBe(MIC_OFF_LINE);
    expect(MIC_TEXT.continue).toBe('Continue'); expect(MIC_TEXT.notNow).toBe('Not now'); expect(MIC_TEXT.ok).toBe('OK');
  });
  test('M3 the permission is read from native, and reading it asks nothing of the person', async () => {
    expect(await micPermission('t')).toEqual({ state: 'unasked', gate: 'micpre' });
    answer = { state: 'refused' }; expect(await micPermission('t')).toEqual({ state: 'refused', gate: 'micoff' });
    answer = { state: 'granted' }; expect(await micPermission('t')).toEqual({ state: 'granted', gate: '' });
    expect(calls.filter((c) => c.op === 'obMicState').length).toBe(3); expect(calls.some((c) => c.op === 'obMicStart' || c.op === 'obEnrolBegin')).toBe(false);   // it only reads
  });
  test('M4 no native side (a web build) or a failing one means no gate: nothing is asked of the person here', async () => {
    resetSeam(); expect(await micPermission('t')).toEqual({ state: 'granted', gate: '' });
    bindNative({ later: async () => { throw new Error('x'); } }); expect(await micPermission('t')).toEqual({ state: 'granted', gate: '' });
    bindNative({ later: async () => 'junk' }); expect(await micPermission('t')).toEqual({ state: 'granted', gate: '' });
  });
});
