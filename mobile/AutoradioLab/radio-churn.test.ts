// The radio's resources over a simulated minute, in the real wiring (the review of the re-cut train: the proof's launch took 41 s and the shared code asked its resources about sixteen times as often).
// Why the earlier harness (radio-position.test.ts P12) missed it: it counted the native calls of ONE pulse and its screens, asked once a second by the test itself. Nothing there asked the pulse again.
// In the app a resource is asked again whenever an argument changes or native says its source changed, and the pulse's own arguments include the radio note (`rnote.tick`), which native moves whenever it
// touches `radioView` or `barFacts`: the engine's 0.4 s progress timer does, while the programme plays. So the pulse ran 2.5 times a second more than the clock asked, each pulse moved the beat, and each beat
// asked the three screens (and what reads them) again. This file runs the graph: the resources named in the contracts, their arguments read from the contract text (a drift fails the first test), the real TypeScript
// behind them (radioPulse, queueView, barRun, radioPageFacts, radioPage, barInfo), a native that answers like the module (the clock, the deck, and the 0.4 s touch, if the Swift still has one),
// and Exact's rule: a resource is asked again when the arguments it was asked with are not the ones it has now.
//   C1 the model is the contract: every resource of the graph is declared in the contracts with the arguments the model uses.
//   C2 while the programme plays in front, for a simulated minute: the pulse and the three screens (brun, qv, rpfacts) are asked about once a second (and the beat advances: a dead pulse is not green), the page and the bar's
//      readers of the bar's facts (rp, bi) follow native's 2.5 a second, and the native calls counted are the pulse's, the screens' AND native's own sources (barFacts, radioView).
//   C3 paused, and locked (not in front): the pulse is asked at the slow clock's rate or less.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { radioAir, radioHold, radioWrite, barRun, getRadio, queueAct, queueView, radioPageFacts, radioPlayAudio, radioPulse, wipeRadio } from './radio-bridge.ts';
import { radioPage } from './radio-page.ts';
import { barInfo } from './bar.ts';

const src = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
const contracts = ['spine.contract', 'members-radio.contract', 'members-queue.contract', 'members-bar.contract', 'members-cards.contract'].map(src).join('\n');
const swiftModule = readFileSync(new URL('../exact-package/apple/AutoradioModule.swift', import.meta.url), 'utf8') + readFileSync(new URL('../exact-package/apple/AutoradioModule+Bar.swift', import.meta.url), 'utf8');
/** The 0.4 s progress timer of AutoradioModule+Bar.swift used to touch radioView and barFacts while the programme played (the bar's progress moved with it). The bar's progress is shared code's now (the deck's place), and the timer
 *  only moves the queue view when the block does. If a timer touch of those two comes back, the model fires it again, and whether that wakes the pump (radioNote) is read from the Swift. */
const timerTouches = /if s\.running && !s\.paused \{ self\.touch\(\["radioView", "barFacts"\]/.test(swiftModule);
const timerWakesPump = timerTouches && !/if s\.running && !s\.paused \{ self\.touch\(\["radioView", "barFacts"\], pump: false\) \}/.test(swiftModule);

const BASE = 1_700_000_000_000;
let nowMs = 0, front = true, calls: Record<string, number> = {};
const dk = { why: 'empty', item: '', base: 0, t0: 0, duration: 100000, seq: 0 };
const pos = (): number => (dk.why === 'playing' ? dk.base + (nowMs - dk.t0) / 1000 : dk.base);
const native = {
  available: true,
  later: async (r: { op: string; name?: string; peek?: boolean; args?: Record<string, unknown> }) => {
    const k = r.op === 'deck' ? `deck:${r.name}` : r.op + (r.peek ? ':peek' : ''); calls[k] = (calls[k] ?? 0) + 1;
    if (r.op === 'radioFacts') return { at: BASE + nowMs, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: front, sentences: [], sentenceIndex: 0, progress: 0 };
    if (r.op === 'deck') {
      const a = r.args ?? {};
      if (r.name === 'deckPlay') { dk.item = String(a.id); dk.why = 'playing'; dk.base = 0; dk.t0 = nowMs; dk.seq += 1; }
      if (r.name === 'deckAct' && a.op === 'pause') { dk.base = pos(); dk.why = 'paused_listener'; }
      return { why: dk.why, item: dk.item, block: 0, seconds: pos(), duration: dk.item ? dk.duration : 0, held: [], seq: dk.seq, acked: 0, journal: [] };
    }
    return { ok: true };
  },
};

type Res = { name: string; args: () => string; run: () => Promise<unknown>; value?: unknown; last?: string; asks: number; native?: boolean };
async function simulate(seconds: number): Promise<{ asks: Record<string, number>; calls: Record<string, number>; pulses: number; beat: number }> {
  let radioTick = 0, noteTick = 0, barFactsRev = 0, viewRev = 0, slowNow = false;
  const v = (n: string): any => res.find((r) => r.name === n)!.value ?? {};
  const posKey = (): number => (v('rpulse').live && front ? v('rpulse').beat : 0);
  const res: Res[] = [
    { name: 'ds', native: true, args: () => `ds${radioTick}`, run: () => native.later({ op: 'deck', name: 'deckState' }), asks: 0 },       // the package source, read once per radio tick (spine.contract)
    { name: 'rnote', native: true, args: () => `note${noteTick}`, run: async () => ({ tick: noteTick }), asks: 0 },
    { name: 'bf', native: true, args: () => `bf${barFactsRev}`, run: async () => (calls.barFacts = (calls.barFacts ?? 0) + 1, { at: (BASE + nowMs) / 1000 }), asks: 0 },
    { name: 'ri', native: true, args: () => `ri${viewRev}`, run: async () => (calls.radioView = (calls.radioView ?? 0) + 1, { rev: viewRev }), asks: 0 },
    { name: 'rpulse', args: () => `${JSON.stringify(v('ds'))}|${v('rnote').tick}|${radioTick}`, run: () => radioPulse(undefined, native as never, v('ds')), asks: 0 },
    { name: 'rair', args: () => `${v('rpulse').tick}|${posKey()}|${v('ds').item}`, run: () => radioAir(undefined, native as never), asks: 0 },
    { name: 'brun', args: () => `${v('rpulse').tick}|${posKey()}|${JSON.stringify(v('ds'))}|${JSON.stringify(v('rair'))}`, run: () => barRun(undefined, native as never, v('ds'), v('rair')), asks: 0 },
    { name: 'qv', args: () => `${v('rpulse').tick}|${posKey()}`, run: () => queueView(undefined, native as never), asks: 0 },
    { name: 'rpfacts', args: () => `${v('rpulse').tick}|${posKey()}|${JSON.stringify(v('ds'))}|${JSON.stringify(v('rair'))}`, run: () => radioPageFacts(undefined, native as never, v('ds'), v('rair')), asks: 0 },
    { name: 'rp', args: () => JSON.stringify([v('rpfacts'), v('bf')]), run: async () => radioPage(v('rpfacts'), v('bf'), { entries: [] }, 0), asks: 0 },
    { name: 'lnc', args: () => `${v('rpfacts').at}`, run: async () => ({ at: v('rpfacts').at }), asks: 0 },
    { name: 'bi', args: () => JSON.stringify([v('bf'), v('brun')]), run: async () => barInfo({ ...v('bf'), ...v('brun') }, { entries: [] }), asks: 0 },
  ];
  async function settle(): Promise<void> {
    for (let pass = 0; pass < 20; pass++) {
      let again = false;
      for (const r of res) { const k = r.args(); if (k !== r.last) { r.last = k; r.value = await r.run(); r.asks += 1; again = true; } }
      slowNow = !!v('rpulse').slow;
      if (!again) return;
    }
    throw new Error('the resources never settled: a loop');
  }
  await settle();
  for (const r of res) r.asks = 0;
  calls = {};
  let clockAt = 1000, touchAt = 400;
  for (nowMs = 0; nowMs <= seconds * 1000; nowMs += 50) {
    let fired = false;
    if (nowMs >= clockAt) { radioTick += 1; clockAt = nowMs + (slowNow ? 5000 : 1000); fired = true; }
    if (timerTouches && dk.why === 'playing' && nowMs >= touchAt) { touchAt = nowMs + 400; barFactsRev += 1; viewRev += 1; if (timerWakesPump) noteTick += 1; fired = true; }
    if (fired) await settle();
  }
  return { asks: Object.fromEntries(res.map((r) => [r.name, r.asks])), calls: { ...calls }, pulses: res.find((r) => r.name === 'rpulse')!.asks, beat: v('rpulse').beat as number };
}

beforeEach(async () => { forgetOnboarding(); await wipeRadio(undefined); adoptNative(native as never, 'test'); nowMs = 0; front = true; calls = {}; Object.assign(dk, { why: 'empty', item: '', base: 0, t0: 0, seq: 0 }); });
const playing = async (): Promise<void> => { await radioPlayAudio(undefined, native as never, 'play', 'ep1', 'https://x.test/ep1.mp3', 'Episode', 'Show', 0); };

describe('C1 the model is the contract', () => {
  test('each resource of the graph is declared with the arguments the model uses', () => {
    const declared = (name: string): string => (new RegExp(`resource ${name} = [a-zA-Z]+\\(([^)]*)\\)`).exec(contracts)?.[1] ?? 'MISSING').replace(/\s/g, '');
    expect(declared('rpulse')).toBe('ds,rnote.tick,radioTick');
    expect(/resource ds = deckState\(tick=radioTick\)/.test(contracts)).toBe(true);
    expect(declared('brun')).toBe('rpulse.tick,posKey,ds,rair'); expect(declared('qv')).toBe('rpulse.tick,posKey'); expect(declared('rpfacts')).toBe('rpulse.tick,posKey,ds,rair'); expect(declared('rair')).toBe('rpulse.tick,posKey,ds.item');
    expect(declared('rp')).toBe('rpfacts,bf,cvraw'); expect(declared('lnc')).toBe('lq,pg.count,rpfacts.at');
    expect(declared('bi')).toContain('bf,brun,cvraw');
    expect(/derive posKey = rpulse\.live and hpg\.visibilityState == "visible" \? rpulse\.beat : 0/.test(contracts)).toBe(true);
    expect(/task radioClock when rpulse\.tick >= 0 and not rpulse\.slow key=radioTick\n\s+after\(1000, radioBump\)/.test(contracts)).toBe(true);
  });
});

describe('C2 while the programme plays in front, for a simulated minute', () => {
  test('the pulse and the screens it wakes are asked about as often as the clock says, not as often as native touches', async () => {
    await getRadio(undefined, native as never); await playing();
    const r = await simulate(60);
    console.log(`C2 ${timerTouches ? (timerWakesPump ? 'the timer wakes the pump' : 'the timer touches but does not wake the pump') : 'there is no timer touch'}: asks ${JSON.stringify(r.asks)}; native calls ${JSON.stringify(r.calls)}`);
    expect(r.pulses).toBeGreaterThanOrEqual(55); expect(r.pulses).toBeLessThanOrEqual(2 * 60 + 5);   // the clock's one a second (not fewer: a dead pulse is red), and a little for what really changed
    expect(r.beat).toBeGreaterThanOrEqual(55);                                     // the beat moved about once a second: the position labels are not frozen
    for (const n of ['brun', 'qv', 'rpfacts']) { expect(r.asks[n], n).toBeGreaterThanOrEqual(55); expect(r.asks[n], n).toBeLessThanOrEqual(60 + 5); }   // the position moves once a second
    const nativeCalls = Object.values(r.calls).reduce((a, b) => a + b, 0);
    expect(nativeCalls / 60).toBeGreaterThanOrEqual(2); expect(nativeCalls / 60).toBeLessThanOrEqual(9);    // the pulse's 2 a second, the deck's state and the facts (and 5 more if a timer touch of barFacts and radioView came back)
    expect(timerTouches, 'native touches the bar\'s facts on events, not on a timer').toBe(false);
  });
});

describe('C3 paused and locked', () => {
  test('paused: the pulse is asked at the slow clock\'s rate', async () => {
    await getRadio(undefined, native as never); await playing(); await queueAct(undefined, native as never, 'toggle', '');
    (await getRadio(undefined, native as never)).rt.store.pending.length = 0;       // the fake has no server to take the ops: they would be "due" for ever
    const r = await simulate(60);
    console.log(`C3 paused: asks ${JSON.stringify(r.asks)}`);
    expect(r.pulses).toBeLessThanOrEqual(20);
  });
  test('locked, sounding: the queue is not asked again, and the bar and the page only on the slow clock', async () => {
    await getRadio(undefined, native as never); await playing(); front = false;
    const r = await simulate(60);
    expect(r.asks.qv, 'qv').toBeLessThanOrEqual(3);
    // The bar's run and the page's facts take the deck's state (`ds`) as an argument, as the contract says, and the deck's state is read on the slow clock (every 5 s) when nothing is in front: they are asked then and not each second.
    for (const n of ['brun', 'rpfacts']) expect(r.asks[n], n).toBeLessThanOrEqual(60 / 5 + 6);
  });
});

// C4 (the gaps review): the radio's hand-offs in the real pulse wiring.
//  C4a one radioWrite at a time: the pulse never says `write` while a radioWrite is in flight. A mutation sent again REPLACES the first (Exact: the first is let go), and the first is the one waiting on the desk's
//      reply: it was let go at the moment the reply arrived, the stretch "failed" after its first entry was placed, and the rest of a 30 s model call was lost (the proof's 11:40:00 hand-off).
//  C4a2 the flag is the radio's, is marked by the pulse that calls for the write (so a pulse computed between the send and the body cannot call for a second send), and goes stale: a body that never ends (a continuation
//      that never runs), a sign-out mid-ask and a throw cannot stop the radio asking. Exact's task rules (docs/reference/exact/ui-model.md "Tasks"): closing the gate cancels the pending `after`, a changed key restarts it.
//  C4b an entry that ends with the next on the sheet and not on the deck: the next starts at once, in front, in the background and while the clock is slow (the deck's journal moves `ds.seq`, which asks the pulse).
describe('C4a one radioWrite at a time', () => {
  test('while a radioWrite waits for the desk the pulse does not call for another, whatever is pending', async () => {
    const r = await getRadio(undefined, native as never); r.rt.store.poster = undefined;      // no server: the pending op stays due
    let release: () => void = () => undefined; const gate = new Promise<void>((res) => { release = res; });
    r.writer.checkAhead = () => true; r.writer.settleWithin = () => gate;           // an ask in flight that the first radioWrite waits for
    r.rt.store.pending.push({ op_id: 'p-x', by: 'phone', op: 'aired', id: 'x', state: 'on_air', position_seconds: 0, base_version: 0 } as never);
    expect((await radioPulse(undefined, native as never)).write).toBe(true);        // posting is due: a radioWrite is sent
    const first = radioWrite(undefined, native as never);
    await new Promise((x) => setTimeout(x, 20));
    for (let i = 0; i < 3; i++) expect((await radioPulse(undefined, native as never)).write, 'pulse ' + i).toBe(false);
    release(); await first;
    expect((await radioPulse(undefined, native as never)).write).toBe(true);        // done: the posting still due is sent again
  });
});

describe('C4b the HAND-OFF rule: the next entry starts at once when one ends and the deck holds nothing (this pins the rule that held before the C4a guard; it does not test that guard)', () => {
  const item = (id: string) => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: id, text: `Words of ${id}.`, est_seconds: 30, revision: 0 });
  for (const [name, inFront, slow] of [['in front', true, false], ['in the background', false, false], ['with the slow clock', false, true]] as const) {
    test(name, async () => {
      front = inFront;
      const played: string[] = []; let seq = 0, current = ''; const journal: Record<string, unknown>[] = [];
      const deck = { available: true, later: async (rq: { op: string; name?: string; peek?: boolean; args?: Record<string, unknown> }) => {
        if (rq.op === 'radioFacts') return { at: BASE + nowMs, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: front };
        if (rq.op === 'deck') {
          const a = rq.args ?? {};
          if (rq.name === 'deckPlay') { current = String(a.id); played.push(current); seq += 1; journal.push({ seq, type: 'started', item: current, at: '2023-11-14T22:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '' }); }
          if (rq.name === 'deckAck') { const t = Number(a.through); for (let i = journal.length - 1; i >= 0; i--) if ((journal[i].seq as number) <= t) journal.splice(i, 1); }
          return { why: current ? 'playing' : 'empty', item: current, block: 0, seconds: 0, duration: 30, held: [], seq, acked: 0, journal: [...journal] };
        }
        return { ok: true };
      } };
      adoptNative(deck as never, 'test');
      const r = await getRadio(undefined, deck as never); r.rt.store.poster = undefined;
      r.rt.play(item('a') as never); r.rt.queue(item('b') as never);
      await radioHold(undefined, deck as never);
      expect(played).toEqual(['a']);                                                // b is on the sheet, not on the deck (this deck holds nothing)
      if (slow) {                                                                   // the pulse is on the five-second clock: nothing sounds in front and nothing is due as far as it can tell (this deck cannot hold b)
        r.rt.store.pending.length = 0; const real = r.rt.needsWork.bind(r.rt); r.rt.needsWork = () => false;
        expect((await radioPulse(undefined, deck as never)).slow).toBe(true); r.rt.needsWork = real;
      }
      nowMs += 20_000;                                                              // a's end comes: the deck writes the line, which moves ds.seq and asks the pulse
      seq += 1; journal.push({ seq, type: 'ended', item: 'a', at: '2023-11-14T22:00:20Z', block: 2, seconds: 20, final: 0, rendered: true, end: 'finished', command: '', reason: '' }); current = '';
      const p = await radioPulse(undefined, deck as never);
      expect(p.hold).toBe(true);                                                    // the pump is asked for, in the same turn
      await radioHold(undefined, deck as never);
      expect(played).toEqual(['a', 'b']);                                           // and b is started: no wait for the clock
    });
  }
});

describe('C4a2 the write flag cannot stop the radio asking', () => {
  const setup = async () => {
    const r = await getRadio(undefined, native as never); r.rt.store.poster = undefined;
    r.rt.store.pending.push({ op_id: 'p-x', by: 'phone', op: 'aired', id: 'x', state: 'on_air', position_seconds: 0, base_version: 0 } as never);
    return r;
  };
  test('the pulse that calls for a write marks it: the next pulse, computed before any radioWrite starts, does not call again; an unsent mark goes stale in a few seconds', async () => {
    await setup();
    expect((await radioPulse(undefined, native as never)).write).toBe(true);
    expect((await radioPulse(undefined, native as never)).write).toBe(false);       // a second send in the gap between the send and the body would replace the first
    nowMs += 6000;
    expect((await radioPulse(undefined, native as never)).write).toBe(true);        // the write was never sent (the gate closed): the mark is stale
  });
  test('a radioWrite whose continuation never ends stops blocking after 260 s', async () => {
    const r = await setup();
    r.writer.checkAhead = () => true; r.writer.settleWithin = () => new Promise<void>(() => undefined);
    void radioWrite(undefined, native as never);
    await new Promise((x) => setTimeout(x, 20));
    expect((await radioPulse(undefined, native as never)).write).toBe(false);
    nowMs += 261_000;
    expect((await radioPulse(undefined, native as never)).write).toBe(true);
  });
  test('a sign-out in the middle of an ask: the next radio is not held by the old one\'s flag', async () => {
    const r = await setup();
    r.writer.checkAhead = () => true; r.writer.settleWithin = () => new Promise<void>(() => undefined);
    void radioWrite(undefined, native as never);
    await new Promise((x) => setTimeout(x, 20));
    await wipeRadio(undefined);
    const r2 = await getRadio(undefined, native as never); r2.rt.store.poster = undefined;
    r2.rt.store.pending.push({ op_id: 'p-y', by: 'phone', op: 'aired', id: 'y', state: 'on_air', position_seconds: 0, base_version: 0 } as never);
    expect((await radioPulse(undefined, native as never)).write).toBe(true);
  });
  test('a radioWrite that throws clears the flag', async () => {
    const r = await setup();
    r.writer.checkAhead = () => { throw new Error('boom'); };
    await radioWrite(undefined, native as never).catch(() => undefined);
    expect((await radioPulse(undefined, native as never)).write).toBe(true);
  });
  test('a radioHold that never ends stops blocking the hold after 260 s', async () => {
    const r = await getRadio(undefined, native as never);
    r.rt.needsWork = () => true;
    r.rt.pump = () => new Promise<void>(() => undefined);
    void radioHold(undefined, native as never);
    await new Promise((x) => setTimeout(x, 20));
    expect((await radioPulse(undefined, native as never)).hold).toBe(false);
    nowMs += 261_000;
    expect((await radioPulse(undefined, native as never)).hold).toBe(true);
  });
});
