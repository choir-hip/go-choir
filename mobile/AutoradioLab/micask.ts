// The microphone is asked where it is first needed, after one plain line (owner, 2026-11-23: asked at first need with "Not now"; docs/ship/consent-and-prompts.md). The first run's step has its own (onboarding.ts, voicelearn.ts); this is the
// same line for the other two first needs: the bar's voice button and recording a take, for a person who skipped that step. The page and the words are shared code; native only reports the system's answer (`obMicState`, the one thing only
// the platform knows) and shows the system prompt when the press goes on.
//
// Failure list (each line has a test in micask.test.ts):
//  M1 Unasked shows the pre-prompt line; refused shows the declined line; granted, or an answer not known, shows nothing.
//  M2 The words are the first run's.
//  M3 Reading the state asks nothing of the person.
//  M4 No native side, or one that fails, is "granted": nothing is gated that cannot be asked.
import { nativeLater } from './api.ts';
export const MIC_PRE_LINE = 'To talk to the radio, record a take or learn your voice, Autoradio 1 needs your microphone and speech recognition.';
export const MIC_OFF_LINE = 'Microphone is off for Autoradio 1. You can turn it on in Settings.';

export const MIC_TEXT = { pre: MIC_PRE_LINE, off: MIC_OFF_LINE, continue: 'Continue', notNow: 'Not now', ok: 'OK' } as const;
export type MicPerm = { state: string; gate: string };
/** The sheet to show before the microphone is used: '' (none), `micpre` or `micoff`. */
export const micGateFor = (state: string): string => (state === 'unasked' ? 'micpre' : state === 'refused' ? 'micoff' : '');

/** Source: the system's microphone permission, asked again when `tick` changes. */
export async function micPermission(_tick: string): Promise<MicPerm> {
  let state = 'granted';
  try { const r = await nativeLater()?.({ op: 'obMicState' }); const s = r && typeof r === 'object' ? (r as { state?: unknown }).state : undefined; if (s === 'unasked' || s === 'refused' || s === 'granted') state = s; } catch { /* nothing is gated */ }
  return { state, gate: micGateFor(state) };
}
