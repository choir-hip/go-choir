// The radio's resources over a simulated minute, in the real wiring (the review of the re-cut train: the proof's launch took 41 s and the shared code asked its resources about sixteen times as often).
// Why the earlier harness (radio-position.test.ts P12) missed it: it counted the native calls of ONE pulse and its screens, asked once a second by the test itself. Nothing there asked the pulse again.
// In the app a resource is asked again whenever an argument changes or native says its source changed, and the pulse's own arguments include the radio note (`rnote.tick`), which native moves whenever it
// touches `radioView` or `barFacts`: the engine's 0.4 s progress timer does, while the programme plays. So the pulse ran 2.5 times a second more than the clock asked, each pulse moved the beat, and each beat
// asked the three screens (and what reads them) again. This file runs the graph: the resources named in the contracts, their arguments read from the contract text (a drift fails the first test), the real TypeScript
// behind them (radioPulse, queueView, barRun, radioPageFacts, radioPage, barInfo), a native that answers like the module (the clock, the deck, and the 0.4 s touch read from AutoradioModule+Bar.swift),
// and Exact's rule: a resource is asked again when the arguments it was asked with are not the ones it has now.
//   C1 the model is the contract: every resource of the graph is declared in the contracts with the arguments the model uses.
//   C2 while the programme plays in front, for a simulated minute: the pulse is asked at most twice a second, the three screens (and what reads them) at most once a second, and a pulse costs at most 3 native calls.
//   C3 paused, and locked (not in front): the pulse is asked at the slow clock's rate or less.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { barRun, getRadio, queueAct, queueView, radioPageFacts, radioPlayAudio, radioPulse, wipeRadio } from './radio-bridge.ts';
import { radioPage } from './radio-page.ts';
import { barInfo } from './bar.ts';

const src = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
const contracts = ['spine.contract', 'members-radio.contract', 'members-queue.contract', 'members-bar.contract', 'members-cards.contract'].map(src).join('\n');
const swiftModule = readFileSync(new URL('../exact-package/apple/AutoradioModule.swift', import.meta.url), 'utf8') + readFileSync(new URL('../exact-package/apple/AutoradioModule+Bar.swift', import.meta.url), 'utf8');
/** The 0.4 s progress timer of AutoradioModule+Bar.swift touches radioView and barFacts; whether that wakes the pump (radioNote) is read from the Swift. */
const timerWakesPump = !/touch\(\["radioView", "barFacts"\], pump: false\)/.test(swiftModule);

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
async function simulate(seconds: number): Promise<{ asks: Record<string, number>; calls: Record<string, number>; pulses: number }> {
  let radioTick = 0, noteTick = 0, barFactsRev = 0, viewRev = 0, slowNow = false;
  const v = (n: string): any => res.find((r) => r.name === n)!.value ?? {};
  const posKey = (): number => (v('rpulse').live && front ? v('rpulse').beat : 0);
  const res: Res[] = [
    { name: 'ds', native: true, args: () => 'ds', run: async () => ({ seq: dk.seq }), asks: 0 },
    { name: 'rnote', native: true, args: () => `note${noteTick}`, run: async () => ({ tick: noteTick }), asks: 0 },
    { name: 'bf', native: true, args: () => `bf${barFactsRev}`, run: async () => ({ at: (BASE + nowMs) / 1000, playing: dk.why === 'playing', title: 'Episode', running: true, blockCount: 4, blockIndex: Math.floor(nowMs / 7000) % 4, blockFraction: (nowMs % 7000) / 7000, onAir: true }), asks: 0 },
    { name: 'ri', native: true, args: () => `ri${viewRev}`, run: async () => ({ rev: viewRev }), asks: 0 },
    { name: 'rpulse', args: () => `${v('ds').seq}|${v('rnote').tick}|${radioTick}`, run: () => radioPulse(undefined, native as never), asks: 0 },
    { name: 'brun', args: () => `${v('rpulse').tick}|${posKey()}`, run: () => barRun(undefined, native as never), asks: 0 },
    { name: 'qv', args: () => `${v('rpulse').tick}|${posKey()}`, run: () => queueView(undefined, native as never), asks: 0 },
    { name: 'rpfacts', args: () => `${v('rpulse').tick}|${posKey()}`, run: () => radioPageFacts(undefined, native as never), asks: 0 },
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
    if (dk.why === 'playing' && nowMs >= touchAt) { touchAt = nowMs + 400; barFactsRev += 1; viewRev += 1; if (timerWakesPump) noteTick += 1; fired = true; }
    if (fired) await settle();
  }
  return { asks: Object.fromEntries(res.map((r) => [r.name, r.asks])), calls: { ...calls }, pulses: res.find((r) => r.name === 'rpulse')!.asks };
}

beforeEach(async () => { forgetOnboarding(); await wipeRadio(undefined); adoptNative(native as never, 'test'); nowMs = 0; front = true; calls = {}; Object.assign(dk, { why: 'empty', item: '', base: 0, t0: 0, seq: 0 }); });
const playing = async (): Promise<void> => { await radioPlayAudio(undefined, native as never, 'play', 'ep1', 'https://x.test/ep1.mp3', 'Episode', 'Show', 0); };

describe('C1 the model is the contract', () => {
  test('each resource of the graph is declared with the arguments the model uses', () => {
    const declared = (name: string): string => (new RegExp(`resource ${name} = [a-zA-Z]+\\(([^)]*)\\)`).exec(contracts)?.[1] ?? 'MISSING').replace(/\s/g, '');
    expect(declared('rpulse')).toBe('ds.seq,rnote.tick,radioTick');
    expect(declared('brun')).toBe('rpulse.tick,posKey'); expect(declared('qv')).toBe('rpulse.tick,posKey'); expect(declared('rpfacts')).toBe('rpulse.tick,posKey');
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
    console.log(`C2 ${timerWakesPump ? 'the timer wakes the pump' : 'the timer does not wake the pump'}: asks ${JSON.stringify(r.asks)}; native calls ${JSON.stringify(r.calls)}`);
    expect(r.pulses).toBeLessThanOrEqual(2 * 60 + 5);                               // the clock's one a second, and a little for what really changed
    for (const n of ['brun', 'qv', 'rpfacts']) expect(r.asks[n], n).toBeLessThanOrEqual(60 + 5);   // the position moves once a second
    const nativeCalls = Object.values(r.calls).reduce((a, b) => a + b, 0);
    expect(nativeCalls / 60).toBeLessThanOrEqual(7);
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
  test('locked, sounding: no screen is asked again', async () => {
    await getRadio(undefined, native as never); await playing(); front = false;
    const r = await simulate(60);
    for (const n of ['brun', 'qv', 'rpfacts']) expect(r.asks[n], n).toBeLessThanOrEqual(3);
  });
});
