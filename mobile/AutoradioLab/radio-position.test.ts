// The position while the radio plays, with no event between reads. The failure list, written first (owner, on a real iPhone: the elapsed and remaining times, the progress line, the bar's line and
// the queue scrubber did not move while audio played; a play or pause made them jump):
//  P1  playing, nothing else happening: each second's re-ask of the screens' resources (the contract's rule below) gives a LATER position on the page's facts, the queue view and the bar's run.
//  P2  paused: the pulse says it is not live, so the key stops moving and nothing is asked again; the shown position holds.
//  P3  hidden (the page not visible, the app in the background): no re-ask at all, counted in native asks; visible again: the true position at once.
//  P4  native asks per second: playing and visible adds exactly ONE (the page facts' `radioFacts` peek); paused adds none.
//  P5  a seek, Next and a new entry change the pulse's tick at once (the mutation's own pump), so the screens re-anchor on the next key, not a second later; and the contract refreshes the pulse after them.
//  P6  never past the entry's end: the elapsed label, the remaining label (never negative) and the fraction (never above 1).
//  P9  the app in the background or the phone locked (the native fact `foreground` false, with the page still saying "visible", as the reference says an iPhone may): the pulse is not live, the beat stands still, nothing
//      is asked again for hours; in front again the beat moves at once.
//  P7  the contract wires it: `posKey` is derived from the pulse's `live` and `beat` and the page's visibility, and the three screens' resources take it.
// The contract's rule (spine.contract): a resource is asked again when its arguments change. `ask` below is that rule: the screens are read when `tick` or `posKey` moved since the last second.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { radioPage } from './radio-page.ts';
import { airAddressOf, respondsOf } from './voice-facts.ts';
import type { Entry } from './runsheet-model.ts';
import { barRun, getRadio, radioHold, queueAct, queueView, radioMove, radioPageFacts, radioPlayAudio, radioPlayStory, radioPulse, wipeRadio } from './radio-bridge.ts';

type Call = { op: string; name?: string; peek?: boolean };
let calls: Call[] = [];
let nowS = 1_700_000_000;
let front = true;                                                    // the native fact `foreground`: the app is in front (UIApplication, the voice's own signal)
type Fake = { why: string; item: string; base: number; t0: number; duration: number; seq: number; journal: Record<string, unknown>[]; acked: number };
let dk: Fake;
const pos = (): number => (dk.why === 'playing' ? dk.base + (nowS - dk.t0) : dk.base);

const native = {
  available: true,
  later: async (r: { op: string; name?: string; args?: Record<string, unknown>; peek?: boolean; [k: string]: unknown }) => {
    calls.push({ op: r.op, name: r.name, peek: r.peek });
    if (r.op === 'radioFacts') return { foreground: front, at: nowS * 1000, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [], sentences: [], sentenceIndex: 0, progress: 0 };
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
  calls = []; nowS = 1_700_000_000; front = true;
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

describe('P9 the phone locked or the app in the background: the page may still say visible, and nothing is asked again', () => {
  test('foreground false: not live, the beat stands, no re-ask in a long stretch; foreground true: it moves again at once', async () => {
    fresh(); dk.duration = 10_000; await play(); await ask(0, true); await ask(1, true);
    front = false;
    const gone = await ask(1, true);                                         // `visible` is passed as true: the page fact is not what this rests on
    expect(gone.pulse.live).toBe(false);
    const before = screens.reasks; calls = [];
    for (let i = 0; i < 3600; i++) await ask(1, true);                       // an hour with the screen off, the audio sounding
    expect(screens.reasks).toBe(before);
    expect(calls.filter((c) => c.op === 'radioFacts' && c.peek === true).length).toBe(0);
    front = true;
    const back = await ask(0, true);
    expect(back.pulse.live).toBe(true);
    expect((screens.shown as Shown).elapsed).toBeGreaterThan(3000);          // the true place at once on return (clamped to the entry's end)
    const n = screens.reasks; await ask(1, true); await ask(1, true);
    expect(screens.reasks).toBe(n + 2);
  });
  test('a host that gives no such fact (Android, a host with no native package) is taken as in front: the page\'s own visibility still gates', async () => {
    fresh(); await play();
    const plain = { ...native, later: async (r: { op: string; [k: string]: unknown }) => { const o = await native.later(r as never) as Record<string, unknown>; if (r.op === 'radioFacts') delete o.foreground; return o; } };
    adoptNative(plain as never, 'test');
    expect((await radioPulse(undefined, plain as never)).live).toBe(true);
  });
});

describe('P10 the spoken entry\'s own clock: an entry with no estimate still counts the time the voice has run', () => {
  test('playing counts, a pause holds, resuming goes on; the page facts carry it', async () => {
    fresh();
    const r = await getRadio(undefined, native as never);
    r.rt.play({ id: 'sp-1', kind: 'story', placed_by: 'phone', state: 'ahead', title: 'Played from the paper', est_seconds: 0, revision: 0, text: 'Words of a story.', source: { ref: 'pub:/pub/x', label: 'x' } } as Entry);
    await r.rt.pump();
    dk.item = 'sp-1'; dk.why = 'playing'; dk.base = 0; dk.t0 = nowS; dk.seq += 1;
    await radioPulse(undefined, native as never);
    for (let i = 0; i < 5; i++) { nowS += 1; await radioPulse(undefined, native as never); }
    expect((await radioPageFacts(undefined, native as never)).spokenElapsed).toBe(5);
    dk.why = 'paused_listener'; nowS += 1; await radioPulse(undefined, native as never);
    for (let i = 0; i < 4; i++) { nowS += 1; await radioPulse(undefined, native as never); }
    expect((await radioPageFacts(undefined, native as never)).spokenElapsed).toBe(5);
    dk.why = 'playing'; await radioPulse(undefined, native as never);
    for (let i = 0; i < 3; i++) { nowS += 1; await radioPulse(undefined, native as never); }
    expect((await radioPageFacts(undefined, native as never)).spokenElapsed).toBe(8);
  });
  test('the contract draws a progress value the dump can read, and the proof reads it', () => {
    const c = readFileSync(new URL('radio.contract', import.meta.url), 'utf8');
    expect(c).toMatch(/testId="radio-progress"[^\n]*aria-label=`Progress \$\{/);
    expect(readFileSync(new URL('../scripts/proof-real.sh', import.meta.url), 'utf8')).toContain('radio-progress');
  });
});

// D-2 (cross-account write): every push to a native mirror carries the account generation shared code computed it under; native drops a stale one (AutoradioKit SharedGeneration).
describe('P11 a pulse begun before sign-out and delivered after it changes nothing', () => {
  test('radioSet carries the generation the pulse began under; a fake native that refuses a stale one ends with nothing of account A', async () => {
    let floor = 0, seenGen = -2; const sets: { gen: number; snapshot: string }[] = []; let held: (() => void) | undefined;
    const slow = { available: true, later: async (r: { op: string; gen?: number; snapshot?: string; peek?: boolean }) => {
      if (r.op === 'radioSet' && r.snapshot !== undefined) {
        const gen = r.gen ?? -1; seenGen = gen;
        await new Promise<void>((res) => { held = res; });                 // the call is in flight while the account changes
        if (gen >= floor) { floor = gen; sets.push({ gen, snapshot: r.snapshot }); }
        return { ok: gen >= floor };
      }
      return native.later(r as never);
    } };
    adoptNative(slow as never, 'test');
    await play();
    const gen0 = (await import('./api.ts')).accountGeneration();
    const begun = radioPulse(undefined, slow as never);                    // begun under generation gen0, its radioSet waits
    for (let i = 0; i < 50 && held === undefined; i++) await new Promise((r) => setTimeout(r, 5));
    const { nextGeneration } = await import('./api.ts'); const gen1 = nextGeneration();   // sign-out begins: the generation moves
    floor = gen1;                                                          // native announced it (SharedGeneration.announce at the signOut call)
    held?.(); await begun;
    expect(sets).toEqual([]);                                              // nothing of the old account was stored
    expect(seenGen).toBe(gen0);                                            // and the call said which account it was computed for
    expect(gen1).toBeGreaterThan(gen0);
  });
  test('V5 radioSet hands the voice and the take recorder what they read: the facts of the entries on air and ahead, and what a take responds to; no run sheet', async () => {
    const pushed: Record<string, unknown>[] = [];
    const spy = { available: true, later: async (r: { op: string }) => { if (r.op === 'radioSet') pushed.push(r as never); return native.later(r as never); } };
    adoptNative(spy as never, 'test');
    await play();
    await radioPulse(undefined, spy as never);
    const set = pushed.find((p) => Array.isArray(p.facts));
    expect(set).toBeDefined();
    const facts = set!.facts as { id: string; state: string }[];
    expect(facts.some((x) => x.state === 'on_air')).toBe(true);
    expect(set!.airAddress).toBe('');                                              // an episode on air is no page: an empty address, not a made-up one
    expect(typeof set!.airAddress).toBe('string'); expect(typeof set!.respondsRoute).toBe('string'); expect(typeof set!.respondsTitle).toBe('string');
    expect(set!.sheet).toBeUndefined(); expect(set!.onAir).toBeUndefined();
  });
  test('V5b a story on air: radioSet names its page (the reader\'s follow-along) and what a take responds to', async () => {
    const pushed: Record<string, unknown>[] = [];
    const spy = { available: true, later: async (r: { op: string }) => { if (r.op === 'radioSet') pushed.push(r as never); return native.later(r as never); } };
    adoptNative(spy as never, 'test');
    await radioPlayStory(undefined, spy as never, 'harbor-fees', 'Harbor fees rise');
    await radioPulse(undefined, spy as never);
    const set = pushed.filter((p) => Array.isArray(p.facts)).pop();
    const r = await getRadio(undefined, spy as never);
    const air = r.rt.store.onAir();                                                   // whatever is on air, the push names ITS page and ITS story (voice-facts.test.ts V3, V6 hold the rules)
    expect(set!.airAddress).toBe(airAddressOf(air)); expect(set!.respondsRoute).toBe(respondsOf(air)?.route ?? '');
  });
  test('the stamped calls: radioSet, editionMirror and the sign-out calls carry gen', () => {
    const c = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
    expect(c('radio-bridge.ts')).toMatch(/op: 'radioSet', gen,/);
    expect(c('edition-picks.ts')).toMatch(/op: 'editionMirror', topics, gen/);
    expect(c('account.ts')).toMatch(/later\(\{ op, gen: accountGeneration\(\) \}\)/);
    expect(c('account.ts')).toMatch(/op: 'signOut', gen: accountGeneration\(\)/);
  });
});

// P12 native calls a second (review B2-1): what the once-a-second pulse and the screens it wakes ask of native, by scenario. The numbers are logged; the bounds are what the fix holds.
describe('P12 native calls per second', () => {
  const perSecond = async (setup: () => Promise<void>, visible = true, seconds = 10): Promise<{ total: number; byOp: Record<string, number>; sheetPushes: number }> => {
    fresh(); await setup(); await ask(0, visible); await ask(1, visible);
    calls = []; let sets = 0; const before = calls.length; void before;
    for (let i = 0; i < seconds; i++) await ask(1, visible);
    const byOp: Record<string, number> = {};
    for (const c of calls) { const k = c.op === 'deck' ? `deck:${c.name}` : c.op + (c.peek ? ':peek' : ''); byOp[k] = (byOp[k] ?? 0) + 1; if (c.op === 'radioSet') sets += 1; }
    return { total: calls.length / seconds, byOp, sheetPushes: sets };
  };
  const report = (what: string, r: { total: number; byOp: Record<string, number> }): void => { console.log(`P12 ${what}: ${r.total} native calls a second ${JSON.stringify(r.byOp)}`); };
  test('playing and visible, paused, idle, backgrounded', async () => {
    const playing = await perSecond(async () => { await play(); });
    report('playing (in front)', playing);
    const paused = await perSecond(async () => { await play(); await queueAct(undefined, native as never, 'toggle', ''); });
    report('paused', paused);
    const idle = await perSecond(async () => undefined);
    report('idle (nothing on air)', idle);
    front = false;
    const background = await perSecond(async () => { await play(); }, true);
    report('backgrounded, audio sounding', background);
    front = true;
    // the clock asks again every five seconds, not one, when nothing sounds in front and nothing is due: 0.4 native calls a second in those three (the fake has no server, so its pending ops are cleared: they would be "due")
    const quiet = async (): Promise<boolean> => { (await getRadio(undefined, native as never)).rt.store.pending.length = 0; return (await radioPulse(undefined, native as never)).slow; };
    await perSecond(async () => { await play(); await queueAct(undefined, native as never, 'toggle', ''); }); expect(await quiet()).toBe(true);     // paused
    front = false; await perSecond(async () => { await play(); }); expect(await quiet()).toBe(true); front = true;                              // locked, sounding: the deck holds the next entries; a refill shows as `hold` or `write`
    await perSecond(async () => { await play(); }); expect((await radioPulse(undefined, native as never)).slow).toBe(false);
    expect(playing.byOp.radioSet ?? 0).toBeLessThanOrEqual(1);                      // the sheet is sent when it changed, not each second
    expect(idle.total).toBeLessThanOrEqual(2); expect(paused.total).toBeLessThanOrEqual(2);
    expect(background.total).toBeLessThanOrEqual(3);
    expect(playing.total).toBeLessThanOrEqual(4);
  });
});

describe('P13 a voice turn\'s messages survive a radio that is replaced while they are read (review B1-10)', () => {
  test('the facts read for a radio that a sign-out replaced hand their inbox to the new radio', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const msg = JSON.stringify({ verb: 'turnEnded' });
    let first = true;
    const slow = { available: true, later: async (r: { op: string; peek?: boolean }) => {
      if (r.op === 'radioFacts' && r.peek !== true && first) { first = false; await gate; return { at: nowS * 1000, ready: true, inbox: [msg], position: { block: 0, seconds: 0 } }; }
      return native.later(r as never);
    } };
    adoptNative(slow as never, 'test');
    const pending = getRadio(undefined, slow as never);
    await new Promise((r) => setTimeout(r, 20));
    await wipeRadio(undefined);                                               // the sign-out replaces the radio while its facts are being read
    release();
    const r2 = await pending;
    expect(r2.inbox).toContain(msg);
  });
});

describe('P14 a sign-out forgets the lists sent with the turns (review B2 nit)', () => {
  test('wipeRadio resets the sent lists and the turn log', async () => {
    const { turnQueueJSON, applyTurnOps } = await import('./voice-queue.ts');
    const r = await getRadio(undefined, native as never);
    turnQueueJSON(r.rt, 0);
    const v = r.rt.store.sheet.version;
    await wipeRadio(undefined);
    const r2 = await getRadio(undefined, native as never);
    const out = applyTurnOps(r2.rt, JSON.stringify({ turn_id: 'after-wipe', base_version: v, ops: [{ op: 'move', id: 'x', to_index: 0 }] }));
    expect(out.refused[0]).toContain('is not one this phone sent');            // the sent lists went with the account
  });
});

describe('P15 the native place verb is a listener act (seat 2, item 4)', () => {
  test('a podcast added by the native path tells the writer the listener acted', async () => {
    const r = await getRadio(undefined, native as never);
    let n = 0; const orig = r.writer.listenerActed.bind(r.writer); r.writer.listenerActed = () => { n += 1; orig(); };
    r.inbox.push(JSON.stringify({ verb: 'place', mode: 'end', entry: { id: 'l-pod', kind: 'story', placed_by: 'listener', state: 'ahead', title: 'Episode', text: 'Words.', est_seconds: 30, revision: 0 } }));
    await radioHold(undefined, native as never);
    expect(r.rt.store.entry('l-pod')).toBeDefined();
    expect(n).toBeGreaterThanOrEqual(1);
  });
});

describe('P16 what was lost offline reaches the log line the status reads (seat 2, item 6)', () => {
  test('the pulse line carries lost=N', async () => {
    const r = await getRadio(undefined, native as never); r.rt.store.lostPending = 7;
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { await radioPulse(undefined, native as never); r.rt.store.lostPending = 8; await radioPulse(undefined, native as never); } finally { console.log = log; }
    expect(logs.some((l) => l.includes('pulse') && /lost=8/.test(l))).toBe(true);
  });
});
