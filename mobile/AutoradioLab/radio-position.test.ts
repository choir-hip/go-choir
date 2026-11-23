// The position while the radio plays, with no event between reads. The failure list, written first (owner, on a real iPhone: the elapsed and remaining times, the progress line, the bar's line and
// the queue scrubber did not move while audio played; a play or pause made them jump):
//  P1  playing, nothing else happening: each second's re-ask of the screens' resources (the contract's rule below) gives a LATER position on the page's facts, the queue view and the bar's run.
//  P2  paused: the pulse says it is not live, so the key stops moving and nothing is asked again; the shown position holds.
//  P3  hidden (the page not visible, the app in the background): no re-ask at all, counted in native asks; visible again: the true position at once.
//  P4  native asks per second: playing and visible adds exactly ONE (the page facts' `radioFacts` peek); paused adds none.
//  P5  a seek, Next and a new entry change the pulse's tick at once (the mutation's own pump), so the screens re-anchor on the next key, not a second later; and the contract refreshes the pulse after them.
//  P6  never past the entry's end: the elapsed label, the remaining label (never negative) and the fraction (never above 1).
//  P7  the contract wires it: `posKey` is derived from the pulse's `live` and `beat` and the page's visibility, and the three screens' resources take it.
// The contract's rule (spine.contract): a resource is asked again when its arguments change. `ask` below is that rule: the screens are read when `tick` or `posKey` moved since the last second.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { radioPage } from './radio-page.ts';
import type { Entry } from './runsheet-model.ts';
import { barRun, getRadio, queueAct, queueView, radioMove, radioPageFacts, radioPlayAudio, radioPulse, wipeRadio } from './radio-bridge.ts';

type Call = { op: string; name?: string; peek?: boolean };
let calls: Call[] = [];
let nowS = 1_700_000_000;                                            // the device's clock, seconds
type Fake = { why: string; item: string; base: number; t0: number; duration: number; seq: number; journal: Record<string, unknown>[]; acked: number };
let dk: Fake;
const pos = (): number => (dk.why === 'playing' ? dk.base + (nowS - dk.t0) : dk.base);

const native = {
  available: true,
  later: async (r: { op: string; name?: string; args?: Record<string, unknown>; peek?: boolean; [k: string]: unknown }) => {
    calls.push({ op: r.op, name: r.name, peek: r.peek });
    if (r.op === 'radioFacts') return { at: nowS * 1000, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [], sentences: [], sentenceIndex: 0, progress: 0 };
    if (r.op === 'deck') {
      const a = r.args ?? {};
      if (r.name === 'deckPlay') { dk.item = String(a.id); dk.why = 'playing'; dk.base = Number(a.start ?? 0); dk.t0 = nowS; dk.seq += 1; dk.journal.push({ seq: dk.seq, type: 'started', item: dk.item, at: '2023-11-14T22:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '' }); }
      if (r.name === 'deckAct') {
        const op = a.op, v = Number(a.value ?? 0);
        if (op === 'pause') { dk.base = pos(); dk.why = 'paused_listener'; }
        if (op === 'resume') { dk.t0 = nowS; dk.why = 'playing'; }
        if (op === 'seekTo') { dk.base = v; dk.t0 = nowS; }
        if (op === 'seekBy') { dk.base = pos() + v; dk.t0 = nowS; }
        if (op === 'stop') { dk.why = 'stopped'; dk.item = ''; }
      }
      if (r.name === 'deckAck') { dk.acked = Number(a.through); dk.journal = dk.journal.filter((l) => (l.seq as number) > dk.acked); }
      return { why: dk.why, item: dk.item, block: 0, seconds: pos(), duration: dk.item ? dk.duration : 0, held: [], seq: dk.seq, acked: dk.acked, journal: dk.journal };
    }
    return {};
  },
};

beforeEach(async () => {
  forgetOnboarding();
  await wipeRadio(undefined);
  adoptNative(native as never, 'test');
  calls = []; nowS = 1_700_000_000;
  dk = { why: 'empty', item: '', base: 0, t0: 0, duration: 240, seq: 0, journal: [], acked: 0 };
});

const play = async (id = 'ep1'): Promise<void> => { await radioPlayAudio(undefined, native as never, 'play', id, `https://x.test/${id}.mp3`, `Episode ${id}`, 'Show', 0); };

/** One second of the contract's clock: time moves, the pulse is asked, and the screens are asked again only when the key they are keyed on moved. */
type Shown = { elapsed: number; label: string; fraction: number; barFraction: number; pageElapsed: number };
const screens = { last: '', shown: undefined as Shown | undefined, reasks: 0 };
async function ask(seconds: number, visible: boolean): Promise<{ pulse: Awaited<ReturnType<typeof radioPulse>>; reasked: boolean }> {
  nowS += seconds;
  const pulse = await radioPulse(undefined, native as never);
  const posKey = pulse.live && visible ? pulse.beat : 0;                 // spine.contract: derive posKey
  const key = `${pulse.tick}|${posKey}`;
  let reasked = false;
  if (key !== screens.last) {
    screens.last = key; reasked = true; screens.reasks += 1;
    const f = await radioPageFacts(undefined, native as never), q = await queueView(undefined, native as never), b = await barRun(undefined, native as never);
    screens.shown = { elapsed: q.elapsed, label: q.elapsedLabel, fraction: q.fraction, barFraction: b.remoteFraction as number, pageElapsed: f.remoteElapsed };
  }
  return { pulse, reasked };
}
const fresh = (): void => { screens.last = ''; screens.shown = undefined; screens.reasks = 0; };

describe('P1 the position moves while the audio plays and no event comes', () => {
  test('page facts, queue view and the bar run all advance each second', async () => {
    fresh(); await play(); await ask(0, true);
    const seen: Shown[] = [screens.shown as Shown];
    for (let i = 0; i < 5; i++) { await ask(1, true); seen.push(screens.shown as Shown); }
    expect(seen.map((s) => s.elapsed)).toEqual([0, 1, 2, 3, 4, 5]);
    expect(seen.map((s) => s.pageElapsed)).toEqual([0, 1, 2, 3, 4, 5]);
    expect(seen.map((s) => s.label)).toEqual(['0:00', '0:01', '0:02', '0:03', '0:04', '0:05']);
    for (let i = 1; i < seen.length; i++) { expect(seen[i].fraction).toBeGreaterThan(seen[i - 1].fraction); expect(seen[i].barFraction).toBeGreaterThan(seen[i - 1].barFraction); }
  });
});

describe('P2 paused: nothing is asked again and the position holds', () => {
  test('after a pause the pulse is not live; five seconds later the screens were not asked and show the same position', async () => {
    fresh(); await play(); await ask(0, true); await ask(3, true);
    await queueAct(undefined, native as never, 'toggle', '');           // pause (a mutation: its own pump)
    const r = await ask(0, true);
    expect(r.pulse.live).toBe(false);
    const held = screens.shown as Shown; const before = screens.reasks;
    for (let i = 0; i < 5; i++) await ask(1, true);
    expect(screens.reasks).toBe(before);
    expect(screens.shown).toEqual(held);
    expect(held.elapsed).toBe(3);
  });
});

describe('P3 hidden: no re-ask at all', () => {
  test('the page not visible: five seconds of the clock ask the screens nothing; visible again shows the true position at once', async () => {
    fresh(); await play(); await ask(0, true); await ask(1, true);
    await ask(1, false);                                                    // the key falls to its constant: ONE re-ask of the same position, as any change of a key is
    const before = screens.reasks; calls = [];
    for (let i = 0; i < 5; i++) await ask(1, false);
    expect(screens.reasks).toBe(before);                                    // then none at all
    expect(calls.filter((c) => c.op === 'radioFacts' && c.peek === true).length).toBe(0);
    expect((screens.shown as Shown).elapsed).toBe(2);                       // what was last shown is held, not guessed
    await ask(0, true);
    expect((screens.shown as Shown).elapsed).toBe(7);                       // visible again: the true position at once
  });
});

describe('P4 native asks per second', () => {
  test('playing and visible adds one `radioFacts` peek per second (the page facts); the pulse keeps its own; paused adds none', async () => {
    fresh(); await play(); await ask(0, true);
    calls = [];
    for (let i = 0; i < 10; i++) await ask(1, true);
    const peeks = calls.filter((c) => c.op === 'radioFacts' && c.peek === true).length;
    expect(peeks).toBe(10);                                                   // one per second
    const perPulse = (n: string): number => calls.filter((c) => (c.op === n) || c.name === n).length;
    expect(perPulse('deckState')).toBe(10);                                   // the pulse's own deck read, as before this change
    expect(calls.filter((c) => c.op === 'deck' && c.name !== 'deckState' && c.name !== 'deckAck').length).toBe(0);
    await queueAct(undefined, native as never, 'toggle', '');
    await ask(0, true);
    calls = [];
    for (let i = 0; i < 10; i++) await ask(1, true);
    expect(calls.filter((c) => c.op === 'radioFacts' && c.peek === true).length).toBe(0);
  });
});

describe('P5 a seek, Next and a new entry re-anchor at once', () => {
  test('the mutation moves the tick, so the next key re-asks without waiting a second; the contract also refreshes the pulse', async () => {
    fresh(); await play(); await ask(0, true); await ask(2, true);
    const t0 = (await radioPulse(undefined, native as never)).tick;
    await queueAct(undefined, native as never, 'seek', '0.5');
    const p = await radioPulse(undefined, native as never);
    expect(p.tick).toBeGreaterThan(t0);
    fresh(); await ask(0, true);
    expect((screens.shown as Shown).elapsed).toBe(120);
    const t1 = (await radioPulse(undefined, native as never)).tick;
    await play('ep2');
    expect((await radioPulse(undefined, native as never)).tick).toBeGreaterThan(t1);
    await ask(0, true);
    expect((screens.shown as Shown).elapsed).toBe(0);
    await ask(2, true);
    await radioMove(undefined, native as never, 'next');
    const after = await queueView(undefined, native as never);
    expect(after.elapsed).toBe(0);
  });
  test('the contract refreshes the pulse after a mutation that moves the position', () => {
    const c = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
    expect(c('members-queue.contract')).toMatch(/mutation qm as shape QueueInfo refreshes [^\n]*rpulse/);
    expect(c('members-radio.contract')).toMatch(/mutation rpm as shape SettingsAck refreshes [^\n]*rpulse/);
    expect(c('account.contract')).toMatch(/mutation racct as shape SettingsAck refreshes [^\n]*rpulse/);
    expect(c('members-bar.contract')).toMatch(/mutation rm2 [^\n]*refreshes [^\n]*rpulse/);
  });
});

describe('P6 never past the end', () => {
  test('elapsed is held at the duration, remaining is zero, the fraction is one', async () => {
    fresh(); await play(); await ask(0, true);
    await ask(500, true);                                                    // the audio's position ran past the length (the deck has not ended it yet)
    const s = screens.shown as Shown;
    expect(s.elapsed).toBe(240); expect(s.fraction).toBe(1); expect(s.barFraction).toBe(1); expect(s.pageElapsed).toBe(240);
    const q = await queueView(undefined, native as never);
    expect(q.remainingLabel).toBe('-0:00'); expect(q.elapsedLabel).toBe('4:00');
  });
});

describe('P7 the contract is wired to the key', () => {
  const c = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
  test('posKey is the pulse\'s beat while it is live and the page is visible, else a constant', () => {
    expect(c('spine.contract')).toMatch(/derive posKey = rpulse\.live and hpg\.visibilityState == "visible" \? rpulse\.beat : 0/);
  });
  test('the page facts, the queue view and the bar run take it', () => {
    expect(c('members-radio.contract')).toContain('radioPageFacts(rpulse.tick, posKey)');
    expect(c('members-queue.contract')).toContain('queueView(rpulse.tick, posKey)');
    expect(c('spine.contract')).toContain('barRun(rpulse.tick, posKey)');
  });
  test('the pulse says live and beat (RadioTick)', () => {
    const t = c('radio-types.contract');
    expect(t).toMatch(/live: bool/); expect(t).toMatch(/beat: number/);
  });
});

// P8 the cost of one second (the review's S2): the per-second path on a run sheet at its bounds (150 entries of 8 KB of text, about 1.2 MB as JSON) is measured, and the sheet is not
// serialised and read again when nothing in it moved. The turn budget is 100 ms (Exact answers over it fail the resource); the bound below leaves a slow phone a margin of ten.
describe('P8 one second of the position, on a large run sheet', () => {
  const big = async (): Promise<void> => {
    await play();
    const r = await getRadio(undefined, native as never);
    const text = 'Words of a story, sentence after sentence. '.repeat(190);              // about 8 KB
    const entries: Entry[] = [];
    for (let i = 0; i < 150; i++) entries.push({ id: `l-${String(i).padStart(4, '0')}`, kind: 'story', placed_by: 'desk', state: i < 50 ? 'heard' : 'ahead', title: `Story ${i}`, est_seconds: 60, revision: 0, text } as Entry);
    r.rt.store.sheet = { ...r.rt.store.sheet, entries: [...entries, ...r.rt.store.sheet.entries] };
  };
  test('radioPageFacts + radioPage per second: time reported, and the sheet is read once while it does not move', async () => {
    fresh(); await big(); await ask(0, true);
    const wire = (await radioPageFacts(undefined, native as never)).sheet;
    const reps = 40; const t0 = performance.now();
    for (let i = 0; i < reps; i++) { nowS += 1; await radioPulse(undefined, native as never); const f = await radioPageFacts(undefined, native as never); radioPage(f, {}, { entries: [] }, 0); }
    const perSecond = (performance.now() - t0) / reps;
    console.log(`P8: sheet ${(wire.length / 1024).toFixed(0)} KiB; per second (pulse + page facts + page) ${perSecond.toFixed(2)} ms`);
    expect(wire.length).toBeGreaterThan(1_000_000);
    expect(perSecond).toBeLessThan(10);
  });
  test('the sheet is written out and read back once while it does not move, and again when it does', async () => {
    fresh(); await big(); await ask(0, true);
    const realStringify = JSON.stringify, realParse = JSON.parse; let out = 0, back = 0;
    JSON.stringify = ((v: unknown, ...rest: unknown[]) => { if (v && typeof v === 'object' && Array.isArray((v as { entries?: unknown[] }).entries) && (v as { entries: unknown[] }).entries.length >= 150) out++; return (realStringify as (...a: unknown[]) => string)(v, ...rest); }) as typeof JSON.stringify;
    JSON.parse = ((t: string, ...rest: unknown[]) => { if (typeof t === 'string' && t.length > 1_000_000) back++; return (realParse as (...a: unknown[]) => unknown)(t, ...rest); }) as typeof JSON.parse;
    try {
      for (let i = 0; i < 6; i++) { nowS += 1; await radioPulse(undefined, native as never); radioPage(await radioPageFacts(undefined, native as never), {}, { entries: [] }, 0); }
      expect(out).toBeLessThanOrEqual(1); expect(back).toBeLessThanOrEqual(1);
      const r = await getRadio(undefined, native as never);
      r.rt.store.sheet = { ...r.rt.store.sheet, version: r.rt.store.sheet.version + 1, entries: r.rt.store.sheet.entries.slice(1) };      // the sheet moved (a skip, an op)
      const before = out;
      radioPage(await radioPageFacts(undefined, native as never), {}, { entries: [] }, 0);
      expect(out).toBe(before + 1);
      expect((await radioPageFacts(undefined, native as never)).sheet).toBe((await radioPageFacts(undefined, native as never)).sheet);
    } finally { JSON.stringify = realStringify; JSON.parse = realParse; }
  });
});
