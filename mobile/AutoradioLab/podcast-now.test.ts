// "What the player is playing" (podcast-now.ts) is read by the root every second while something sounds: it must NOT read the native facts a second time (the pulse's read is the one), and it answers from the
// run sheet's entry on air and the deck's state.
import { beforeEach, describe, expect, test } from 'bun:test';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { getRadio, radioPlayAudio, wipeRadio } from './radio-bridge.ts';
import { podcastNow } from './podcast-now.ts';

let calls: string[] = [];
let playing = '';
const native = { available: true, later: async (r: { op: string; name?: string; args?: Record<string, unknown> }) => {
  calls.push(r.op === 'deck' ? `deck:${r.name}` : r.op);
  if (r.op === 'radioFacts') return { foreground: true, at: 1_700_000_000_000, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [], sentences: [], sentenceIndex: 0, progress: 0 };
  if (r.op === 'deck') {
    if (r.name === 'deckPlay') playing = String(r.args?.id ?? '');
    return { why: playing ? 'playing' : 'empty', item: playing, block: 0, seconds: 12, duration: 300, held: [], seq: 1, acked: 0, journal: [] };
  }
  return {};
} };

beforeEach(async () => { forgetOnboarding(); await wipeRadio(undefined); adoptNative(native as never, 'test'); calls = []; playing = ''; });

describe('podcastNow', () => {
  test('P6 it reads the store and the deck, and never the native facts (the pulse reads them)', async () => {
    await getRadio(undefined, native as never);
    await radioPlayAudio(undefined, native as never, 'play', 'pod:feed|guid', 'https://x.test/a.mp3', 'An episode', 'A show', 0);
    calls = [];
    await podcastNow(undefined, native as never);
    expect(calls.filter((c) => c === 'radioFacts')).toEqual([]);
    expect(calls).toContain('deck:deckState');
  });
  test('P7 nothing on air answers the idle shape', async () => {
    expect(await podcastNow(undefined, native as never)).toEqual({ episodeId: '', clip: false, start: 0, elapsed: 0, duration: 0 });
  });
});
