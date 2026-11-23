// The desk's writer under bun test (radio-writer.ts). The failure list, written first:
//  W1 the candidates are the front's stories, the paper's weight first, at most 40, each with a title and a dek; a route with a space, `@` or `#`, or no dek, is left out; told routes are skipped on an extend; text only on the first ten, cut at a paragraph.
//  W2 the request carries the cause, the ahead titles (at most 20, cut), the edition text and the candidates; a server that refuses `ahead_titles` (400) is asked once more without it.
//  W3 an `open` ask places the desk's entries at the end of the sheet; the answer's segments become entries (an opening, stories) and the desk's ops are not posted back.
//  W4 `extend` is asked when fewer than two playable entries are ahead or the spoken time is short, one ask at a time, and not again while the entry it found nothing for is on air.
//  W5 a desk that fails says so once in words (signed out, unreachable) and leaves the sheet as it is; the writer fills from the phone's own stories when fewer than two are ahead.
//  W6 an answer that arrives after a sign-out (stop) is dropped whole.
//  W7 the phone's opening is a greeting by the hour, the first story with its dek once, at most two more headlines; with none it says it has no front page yet.
import { beforeEach, describe, expect, test } from 'bun:test';
import { Writer, ASK_DEADLINE_MS, DRY_FLOOR_MS, fromFront, shapeText, candidateBody, requestBody, phoneOpening, type WriterDeps } from './radio-writer.ts';
import { RadioRuntime } from './radio-runtime.ts';
import type { DeckPort, DeckState } from './deck.ts';
import type { Entry } from './runsheet-model.ts';

const NOW = new Date('2023-11-14T22:00:00Z');
const row = (route: string, o: Record<string, unknown> = {}) => ({ route, title: `Title ${route}`, dek: `Dek of ${route}.`, importance: 5, sources: 3, version_count: 2, updated_at: '2023-11-14T20:00:00Z', ...o });
const front = (rows: unknown[]) => ({ stories: rows });
const EMPTY: DeckState = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
const deck: DeckPort = { state: async () => EMPTY, play: async () => EMPTY, enqueue: async () => EMPTY, act: async () => EMPTY, ack: async () => EMPTY };
const seg = (o: Record<string, unknown>) => 'event: segment\ndata: ' + JSON.stringify({ stretch_id: 'st-1', ...o }) + '\n\n';
const answer = (...segs: string[]) => segs.join('') + 'event: done\ndata: {"stretch_id":"st-1"}\n\n';

let skewMs = 0;                                                                  // the device's clock moving on (the deadline test)
let rt: RadioRuntime, w: Writer, bodies: Record<string, unknown>[], reply: () => Promise<string>, fr: unknown, gen: number, fill: Entry[];
const settle = async () => { await w.settle(); await new Promise((r) => setTimeout(r, 0)); await w.settle(); };
beforeEach(() => {
  rt = new RadioRuntime({ deck, now: () => NOW });
  bodies = []; gen = 1; fill = []; skewMs = 0;
  fr = front([row('/pub/a'), row('/pub/b'), row('/pub/c')]);
  reply = async () => answer(seg({ segment_id: 'st-1/0', kind: 'ident', text: 'Hello.' }), seg({ segment_id: 'st-1/1', kind: 'story', title: 'A', text: 'Story A words.', cites: [{ ref: 'pub:/pub/a@v2', role: 'subject', label: 'A' }] }));
  const d: WriterDeps = {
    rt, front: async () => fr, texts: async (routes) => Object.fromEntries(routes.map((r) => [r, [`Para one of ${r}.`, 'Para two.']])),
    takes: async () => ({ takes: [], kept: { ids: [], handle: '', muted: [] } }), edition: async () => 'topics: harbours',
    post: async (b) => { bodies.push(b); return reply(); }, now: () => new Date(NOW.getTime() + skewMs), tz: () => 'UTC', requestId: () => 'radio-test', generation: () => gen, offlineFill: () => fill,
  };
  w = new Writer(d);
});

describe('the candidates', () => {
  test('W1 weight first, bounded, valid routes, told routes skipped, text on the first ten', () => {
    const rows = [row('/pub/low', { importance: 1 }), row('/pub/high', { importance: 10 }), row('/pub/has space'), row('/pub/at@v1'), row('/pub/nodek', { dek: '' }), row('/pub/high')];
    const c = fromFront(front(rows), NOW.getTime());
    expect(c.map((x) => x.route)).toEqual(['/pub/high', '/pub/low']);
    expect(c[0]).toMatchObject({ version: 2, title: 'Title /pub/high' });
    expect(fromFront(front(rows), NOW.getTime(), new Set(['/pub/high'])).map((x) => x.route)).toEqual(['/pub/low']);
    expect(fromFront(front(rows), NOW.getTime(), new Set(), ['/pub/low', '/pub/high', '/pub/low']).map((x) => x.route)).toEqual(['/pub/low', '/pub/high']);
    const many = fromFront(front(Array.from({ length: 60 }, (_, i) => row(`/pub/s${i}`))), NOW.getTime());
    expect(many.length).toBe(40);
    const body = candidateBody(many, Object.fromEntries(many.map((m) => [m.route, ['Words.']])), []);
    expect(body.slice(0, 10).every((b) => b.text === 'Words.')).toBe(true); expect(body[10].text).toBeUndefined();
  });
  test('W1 text is cut at a paragraph that fits in 8 KB', () => {
    const p = 'x'.repeat(3000);
    expect(shapeText([p, p, p])?.length).toBe(6002);
    expect(shapeText(['y'.repeat(9000)])?.length).toBe(8192);
    expect(shapeText(['  ', ''])).toBeUndefined();
  });
});

describe('the request', () => {
  test('W2 cause, ahead titles, edition and candidates; a 400 about ahead_titles is asked once more without them', async () => {
    const b = requestBody('extend', [{ route: '/pub/a' }], ['One', 'x'.repeat(400), ''], 'topics', NOW, 'UTC', 'r1');
    expect(b).toMatchObject({ request_id: 'r1', cause: 'extend', tz: 'UTC', local_time: '2023-11-14T22:00:00Z', edition_text: 'topics' });
    expect((b.ahead_titles as string[]).length).toBe(2); expect((b.ahead_titles as string[])[1].length).toBe(300);
    rt.store.place({ id: 'a', kind: 'story', placed_by: 'desk', state: 'ahead', title: 'Ahead one', text: 'W.', est_seconds: 5, revision: 0 }, 'end');
    let n = 0;
    reply = async () => { n += 1; if (n === 1) throw Object.assign(new Error('400'), { status: 400 }); return answer(seg({ segment_id: 's/0', kind: 'story', title: 'S', text: 'Words.' })); };
    w.open(); await settle();
    expect(bodies.length).toBeGreaterThanOrEqual(2); expect(bodies[0].ahead_titles).toEqual(['Ahead one']); expect(bodies[1].ahead_titles).toBeUndefined();
    expect(bodies[0].candidates).toBeDefined();
  });
});

describe('placing', () => {
  test('W3 an open ask places the desk\'s entries at the end, not posted back', async () => {
    w.open(); await settle();
    const ids = rt.store.sheet.entries.map((e) => e.kind);
    expect(ids).toContain('opening'); expect(ids).toContain('story');
    expect(rt.store.pending.length).toBe(0);
    expect(rt.store.ahead().find((e) => e.kind === 'story')?.cites?.[0].ref).toBe('pub:/pub/a@v2');
  });
  test('W4 extend is asked while little is ahead, one at a time, and stops when the desk has nothing new', async () => {
    rt.store.place({ id: 'on', kind: 'story', placed_by: 'desk', state: 'ahead', title: 'On', text: 'W.', est_seconds: 5, revision: 0 }, 'end');
    rt.director.advanceIfIdle();                                                    // 'on' is on air
    let n = 0;
    reply = async () => { n += 1; return n === 1 ? answer(seg({ segment_id: 'e/0', kind: 'story', title: 'E0', text: 'Words zero.' })) : answer(seg({ segment_id: 'e/0', kind: 'story', title: 'E0', text: 'Words zero.' })); };
    w.openAhead(); await settle();
    w.checkAhead(); await settle();                                                 // the pulse says write again and the next radioWrite asks: an ask never starts another from its own tail (W8)
    expect(w.asks.every((c) => c === 'extend')).toBe(true); expect(n).toBe(2);        // the first added one; the repeat added nothing and the writer is quiet
    const asked = w.asks.length;
    w.checkAhead(); await settle();
    expect(w.asks.length).toBe(asked);                                              // not asked again with the same entry on air
    expect(w.dry).toBe(true);
  });
  test('W5 a desk that fails says so once and the sheet is left alone; the phone\'s own stories fill in', async () => {
    reply = async () => { throw Object.assign(new Error('x'), { kind: 'transport' }); };
    fill = [{ id: 'l-1', kind: 'page', placed_by: 'phone', state: 'ahead', title: 'Held story', est_seconds: 30, revision: 0, source: { ref: 'pub:/pub/h', label: 'Held story' } } as Entry];
    w.open(); await settle();
    expect(w.notice).toBe("The desk can't be reached right now.");
    expect(rt.view().notice).toBe("The desk can't be reached right now.");
    expect(rt.store.ahead().map((e) => e.id)).toEqual(['l-1']);
  });
  test('W5 a front with nothing to tell is no failure the listener hears about', async () => {
    fr = front([]);
    w.openAhead(); await settle();
    expect(w.notice).toBe(''); expect(bodies.length).toBe(0);
  });
  test('W6 an answer after a sign-out is dropped whole', async () => {
    let release: (s: string) => void = () => undefined;
    reply = () => new Promise((r) => { release = r; });
    w.open(); await new Promise((r) => setTimeout(r, 0));
    w.stop(); gen = 2;
    release(answer(seg({ segment_id: 's/0', kind: 'story', title: 'Late', text: 'Late words.' })));
    await settle();
    expect(rt.store.sheet.entries).toEqual([]);
  });
});

describe('the phone\'s opening', () => {
  test('W7 greeting by the hour, the first story with its dek once, two more headlines', () => {
    const lines = [{ title: 'Harbour strike ends', dek: 'Dock workers voted on Tuesday. The union says talks resume Monday.' }, { title: 'Tram fares change' }, { title: 'Library opens late.' }, { title: 'Fourth' }];
    const e = phoneOpening(lines, NOW, 0, 'p-open');
    expect(e).toMatchObject({ id: 'p-open', kind: 'opening', placed_by: 'phone', title: 'Opening' });
    expect(e.text).toBe("Good evening. This is Autoradio. From the paper's front page: Harbour strike ends. Dock workers voted on Tuesday. Also: Tram fares change; and Library opens late.");
    expect(phoneOpening(lines, new Date('2023-11-14T03:00:00Z'), 0, 'x').text.startsWith('Hello.')).toBe(true);
    expect(phoneOpening(lines, new Date('2023-11-14T09:00:00Z'), 0, 'x').text.startsWith('Good morning.')).toBe(true);
    expect(phoneOpening(lines, new Date('2023-11-14T14:00:00Z'), 0, 'x').text.startsWith('Good afternoon.')).toBe(true);
    expect(phoneOpening([], NOW, 0, 'x').text).toBe("Good evening. This is Autoradio. I don't have the front page yet.");
    expect(phoneOpening([{ title: 'Same', dek: 'Same.' }], NOW, 0, 'x').text).not.toMatch(/Same\. Same\./);
  });
});

// W8 to W11 (the radio runs dry when the writer's chained ask hangs; the failure list, written first):
//  W8  an ask never starts another from its own tail: after an ask that added entries and left the time ahead short, exactly one ask has run; the next is the pulse's (`checkAhead` from `radioWrite`).
//  W9  a desk slower than the deadline is asked once more for the spell, not in a loop (a 150 s desk: ten minutes make two calls), and the first answer to arrive is used; an ask that never answers is let go at twice the deadline and asked again; `abandon` names its ask.
//  W10 `settleWithin(ms, id)` returns at its deadline and lets go that ask only.
//  W11 a dry sheet leads to an ask at the next pulse: nothing ahead, nothing in flight, the radio opened, the entry it last found nothing for no longer on air.
describe('the writer never hangs the radio dry', () => {
  test('W8 an ask does not start another from its own tail', async () => {
    w.open(); await settle();
    expect(w.asks).toEqual(['open']);
    expect(bodies.length).toBe(1);
  });
  const longStory = (n: string): string => seg({ segment_id: `s/${n}`, kind: 'story', title: `Story ${n}`, text: 'word '.repeat(700) });     // minutes of speech each
  test('W9 a desk slower than the deadline: ten minutes make fewer than five calls, the first answer to arrive is used, the other is dropped', async () => {
    const waiting: ((s: string) => void)[] = [];
    reply = () => new Promise((r) => { waiting.push(r); });
    w.openAhead(false);
    const answerAt = new Map<number, number>();                                        // call number -> the clock second its answer arrives (150 s after it was asked)
    let seenCalls = 0;
    for (let t = 0; t <= 600; t += 10) {
      skewMs = t * 1000;
      if (w.wouldAsk()) w.checkAhead();
      await new Promise((r) => setTimeout(r, 0));
      while (seenCalls < bodies.length) { answerAt.set(seenCalls, t + 150); seenCalls += 1; }
      for (const [n, at] of [...answerAt]) if (at <= t) { answerAt.delete(n); waiting[n](answer(longStory(`${n}a`), longStory(`${n}b`), longStory(`${n}c`))); await new Promise((r) => setTimeout(r, 40)); }
    }
    expect(bodies.length).toBeLessThan(5); expect(bodies.length).toBeLessThanOrEqual(2);
    const titles = rt.store.ahead().map((e) => e.title);
    expect(titles.length).toBe(3); expect(new Set(titles).size).toBe(3);               // entries arrived, once: the second answer was dropped as older
    await new Promise((r) => setTimeout(r, 40)); expect(w.inFlight).toBe(false);
  });
  test('W9 an ask that never answers is let go at twice the deadline and the radio is asked again', async () => {
    reply = () => new Promise(() => undefined);
    w.openAhead(false); w.checkAhead(); await new Promise((r) => setTimeout(r, 0));
    skewMs = 2 * ASK_DEADLINE_MS + 1000;
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { expect(w.checkAhead()).toBe(true); } finally { console.log = log; }
    await new Promise((r) => setTimeout(r, 20));
    expect(logs.some((l) => l.includes('did not answer') && l.includes('letting it go'))).toBe(true);
    expect(bodies.length).toBe(2);
  });
  test('W9 abandon names the ask it lets go: another ask that started meanwhile is left alone', async () => {
    reply = async () => answer(seg({ segment_id: 'a/0', kind: 'story', title: 'A', text: 'Short.' }));
    w.openAhead(false); w.checkAhead(); const first = w.lastId; await settle();
    let release: (s: string) => void = () => undefined;
    reply = () => new Promise((r) => { release = r; });
    expect(w.checkAhead()).toBe(true); const second = w.lastId; await new Promise((r) => setTimeout(r, 0));
    expect(second).not.toBe(first); expect(w.inFlight).toBe(true);
    w.abandon('a stale caller', first);                                               // the first radioWrite wakes late and abandons ITS ask, which is long gone
    expect(w.inFlight).toBe(true);
    w.abandon('the right caller', second);
    expect(w.inFlight).toBe(false);
    release(answer());
  });
  test('W10 settleWithin returns at its deadline, lets go only the ask it waited for', async () => {
    reply = () => new Promise(() => undefined);
    w.openAhead(false); w.checkAhead(); const id = w.lastId; await new Promise((r) => setTimeout(r, 0));
    expect(w.inFlight).toBe(true);
    const t0 = Date.now(); await w.settleWithin(30, id);
    expect(Date.now() - t0).toBeLessThan(1000);
    expect(w.inFlight).toBe(false); expect(w.wouldAsk()).toBe(true);
  });
  test('W11 a dry sheet leads to exactly one ask at once; after a dry answer the next waits for a changed input, the listener, or the floor (review B1-2)', async () => {
    reply = async () => answer();                                                     // the desk answers with nothing
    expect(w.wouldAsk()).toBe(false);                                                 // not opened: the desk is asked once the radio is opened (the pulse's start does that, openRadioOf)
    w.openAhead(false); expect(w.wouldAsk()).toBe(true);                              // nothing on air, nothing ahead, nothing tried: ask at once
    expect(w.checkAhead()).toBe(true); await settle();
    expect(bodies.length).toBe(1);
    expect(w.dry).toBe(true); expect(w.wouldAsk()).toBe(false);                       // the desk had nothing: not asked again for the same inputs
    skewMs = DRY_FLOOR_MS - 1000; expect(w.wouldAsk()).toBe(false);
    w.listenerActed(); expect(w.wouldAsk()).toBe(true);                               // the listener acted: the answer may differ
  });
  /** One pulse a second for `seconds`: the pulse says write, the mutation asks, and the ask settles. Returns the desk's calls. */
  const run = async (seconds: number): Promise<number> => {
    for (let t = 0; t <= seconds; t++) { skewMs = t * 1000; if (w.wouldAsk()) w.checkAhead(); await new Promise((r) => setTimeout(r, 0)); }
    return bodies.length;
  };
  test('W12 ten minutes with the desk down: a handful of asks, backing off (review B1-2)', async () => {
    reply = async () => { throw Object.assign(new Error('x'), { kind: 'transport' }); };
    w.openAhead(false);
    const n = await run(600);
    expect(n).toBeGreaterThanOrEqual(4); expect(n).toBeLessThanOrEqual(8);
  });
  test('W12 ten minutes with a dry desk and nothing changing: one ask, two at the most', async () => {
    reply = async () => answer();
    w.openAhead(false);
    const n = await run(600);
    expect(n).toBeGreaterThanOrEqual(1); expect(n).toBeLessThanOrEqual(2);
  });
  test('W12 a healthy desk is asked as before: the sheet fills in as many asks as it needs, one right after another', async () => {
    let k = 0;
    reply = async () => { k += 1; return answer(seg({ segment_id: `h/${k}`, kind: 'story', title: `H${k}`, text: 'Short words.' })); };      // one short entry an answer: the sheet stays short
    w.openAhead(false);
    const n = await run(8);
    expect(n).toBeGreaterThanOrEqual(5);                                              // not throttled while it answers
  });
  test('W13 with no device time (the clock reads 1970) a failure holds until the listener acts, a dry answer likewise, and no deadline fires on a clock that does not move (review B1-9)', async () => {
    skewMs = -NOW.getTime();                                                         // the clock says 0: native has not said the time
    reply = async () => { throw Object.assign(new Error('x'), { kind: 'transport' }); };
    w.openAhead(false); expect(w.checkAhead()).toBe(true); await settle();
    for (let i = 0; i < 30; i++) expect(w.wouldAsk()).toBe(false);                   // not a loop
    w.listenerActed(); expect(w.wouldAsk()).toBe(true);
    reply = () => new Promise(() => undefined); expect(w.checkAhead()).toBe(true); await new Promise((r) => setTimeout(r, 0));
    expect(w.wouldAsk()).toBe(false);                                                // an ask in flight is not "overdue" on a clock that stands at 0
  });
  test('W12 a failure is forgotten on success and on the listener\'s act', async () => {
    reply = async () => { throw Object.assign(new Error('x'), { kind: 'transport' }); };
    w.openAhead(false); expect(w.checkAhead()).toBe(true); await settle();
    expect(w.wouldAsk()).toBe(false);                                                 // backing off
    w.listenerActed(); expect(w.wouldAsk()).toBe(true);
  });
});
