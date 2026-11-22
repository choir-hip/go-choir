// A PLAIN launch has no driver whose dumps pull resources in: the radio must start from what the page itself asks. This holds the chain on the shared-code side with a fake native module:
// the pulse says the start is due once the native side says the first run is over (`ready`), and `radioHold` then opens the radio and hands the deck its first entry.
import { describe, expect, test } from 'bun:test';
import { adoptNative } from './api.ts';
import { radioHold, radioPulse, wipeRadio } from './radio-bridge.ts';

const calls: { op: string; name?: string }[] = [];
let ready = false;
let at = 1_700_000_000_000;
const native = {
  available: true,
  later: async (r: { op: string; name?: string; [k: string]: unknown }) => {
    calls.push({ op: r.op, name: r.name });
    if (r.op === 'radioFacts') return { at, ready, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [] };
    if (r.op === 'deck') return { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
    return {};
  },
};

describe('the radio starts in a launch nothing but the page drives', () => {
  test('not ready: nothing is due; ready: the pulse says hold, and radioHold plays the opening', async () => {
    await wipeRadio(undefined);                // another test file may have made the module's one radio
    adoptNative(native as never, 'test');
    const before = await radioPulse(undefined, native as never);
    expect(before.hold).toBe(false);
    ready = true;
    const due = await radioPulse(undefined, native as never);
    expect(due.hold).toBe(true);
    at += 3000;                                // the front page may not come (no network here): 2 s after the first run was over the radio opens without it
    await radioHold(undefined, native as never);
    expect(calls.some((c) => c.op === 'deck' && c.name === 'deckPlay')).toBe(true);
  });
});
