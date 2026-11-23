// The phone's run sheet store under bun test. Ported from RunSheetStoreTests.swift; the cases that need the player, the writer or the
// engine stay with those until they move. Numbers are the Swift list's.
//  9 a transition whose neighbours changed is shown stale · 11 pending ops survive a relaunch, stay while the route says "not yet", go when it answers
// 13 the ahead list is bounded at 100; a listener's own placement is never refused · 14 an entry left on air by a dead app is closed as failed, not resumed
// 15 "queue this" goes after the listener's last placement; "play this" cuts and keeps what is ahead · 16 previous returns the entry before
// 18 Up next is every entry ahead, in order · 20 a duplicate op_id changes nothing; a batch is at most 64 ops
// 24 honest air reports (skip, cut, left, finished, failed carry the true block, position and placer) · 25 an entry built too big is cut and applies
// 26 a long url is left out and the ref stays; a long ref is not placed; the size bound is of the carried form; an op carries nothing it does not use
// Plus: the route poster maps 404/5xx to notYet, other 4xx to answered, transport errors to offline; sign-out wipes and posts nothing afterwards.
import { describe, expect, test } from 'bun:test';
import { RunSheetStore, routePoster, MAX_BATCH_OPS, type Poster, type PostOutcome, type Storage } from './runsheet-store.ts';
import { apply, emptySheet, entryJSONSize, Bounds, type Entry, type Op } from './runsheet-model.ts';
import { paperStory, audio, webPage, boundedEntry, cutBytes } from './runsheet-entries.ts';

const story = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: 'Words.', est_seconds: 10, revision: 0, ...o });
const deskPlace = (s: RunSheetStore, e: Entry) => s.applyFromDesk({ op_id: 'o-' + e.id, by: 'desk', op: 'place', at: 'end', entry: e, reason: 'open', base_version: 0 });

class FakePoster implements Poster {
  answer: PostOutcome = { kind: 'notYet' };
  posts: string[][] = [];
  async post(ops: Op[]): Promise<PostOutcome> { this.posts.push(ops.map((o) => o.op_id)); return this.answer; }
  async fetchSheet() { return undefined; }
}
const memory = (): Storage & { text?: string } => {
  const m: Storage & { text?: string } = { text: undefined, read: async () => m.text, write: async (t) => { m.text = t; }, remove: async () => { m.text = undefined; } };
  return m;
};
/** An opening on air with `ahead` desk entries behind it, as the player does. */
function opened(s: RunSheetStore, ahead: string[]): string {
  const o: Entry = { id: 'p-open', kind: 'opening', placed_by: 'phone', state: 'ahead', title: 'Opening', text: 'Hello.', est_seconds: 5, revision: 0 };
  s.place(o, 'end', { by: 'phone' });
  s.airedOnAir(o.id);
  for (const a of ahead) deskPlace(s, story(a));
  return o.id;
}

describe('stale transitions', () => {
  test('9: a transition whose neighbours changed is shown stale and left alone', () => {
    const s = new RunSheetStore();
    deskPlace(s, story('d-a'));
    deskPlace(s, story('d-t', { kind: 'transition', text: 'And now.', between: { after: 'd-a', before: 'd-b' } }));
    deskPlace(s, story('d-b'));
    expect(s.isStale(s.entry('d-t') as Entry)).toBe(false);
    s.applyFromDesk({ op_id: 'o-rm', by: 'desk', op: 'remove', id: 'd-b', reason: 'revised', base_version: 0 });
    expect(s.isStale(s.entry('d-t') as Entry)).toBe(true);
    expect(s.entry('d-t')?.state).toBe('ahead');
  });
});

describe('pending ops and posting', () => {
  test('11: pending ops survive a relaunch and go when the route answers; an entry left on air is closed failed, not resumed', async () => {
    const mem = memory();
    const s = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    const p = new FakePoster(); s.poster = p;
    const a = paperStory('a', 'A');
    s.place(a, 'now'); await s.flushTask;
    expect(s.pending.length).toBe(1);                      // the route said not yet: the op stays, with its op_id
    const opId = s.pending[0].op_id;
    const s2 = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    const p2 = new FakePoster(); s2.poster = p2;
    await s2.restore(); await s2.flushTask;
    expect(s2.pending[0].op_id).toBe(opId);                // the same op, in order
    expect(s2.onAir()).toBeUndefined();
    expect(s2.entry(a.id)?.ended?.outcome).toBe('failed');
    p2.answer = { kind: 'answered', results: [] }; s2.flush(true); await s2.flushTask;       // forced: after the refusal the store backs off by the device's time, which another test file in the same process may have set
    expect(s2.pending.length).toBe(0);
    expect(p2.posts.at(-1)?.[0]).toBe(s2.sheet.applied_ops[0]);   // the first op sent is the first applied
  });
  test('11: offline leaves the op pending', async () => {
    const s = new RunSheetStore({ retryAfterMs: 3_600_000 });
    const p = new FakePoster(); p.answer = { kind: 'offline' }; s.poster = p;
    s.place(paperStory('a', 'A'), 'end'); await s.flushTask;
    expect(s.pending.length).toBe(1);
  });
  test('14: a crash restart never resumes work, and what was ahead stays ahead', async () => {
    const mem = memory();
    const s = new RunSheetStore({ storage: mem });
    const o = opened(s, ['d-1']);
    await s.save();
    const s2 = new RunSheetStore({ storage: mem });
    await s2.restore();
    expect(s2.onAir()).toBeUndefined();
    expect(s2.entry(o)?.state).toBe('skipped'); expect(s2.entry(o)?.ended?.outcome).toBe('failed');
    expect(s2.entry('d-1')?.state).toBe('ahead');
  });
  test('20: a duplicate op_id changes nothing and a batch is at most 64', async () => {
    const s = new RunSheetStore();
    const op: Op = { op_id: 'p-dup00001', by: 'listener', op: 'place', at: 'end', entry: paperStory('a', 'A'), base_version: 0 };
    expect(s.apply(op).applied).toBe(true);
    const again = s.apply(op);
    expect(again.applied).toBe(false); expect(again.dropped).toBe('duplicate');
    expect(s.pending.length).toBe(1);
    for (let n = 0; n < 150; n++) s.place(paperStory(`r${n}`, 'R'), 'end');
    const p = new FakePoster(); p.answer = { kind: 'answered', results: [] }; s.poster = p;
    s.flush(); await s.flushTask;
    expect(s.pending.length).toBe(0);
    expect(p.posts.every((b) => b.length <= MAX_BATCH_OPS)).toBe(true); expect(p.posts.length).toBeGreaterThan(2);
    expect(p.posts.flat()).toEqual(s.sheet.applied_ops);   // every applied op was sent once, in order
  });
  test('a place the server drops is taken off the local sheet, quietly; a duplicate_id place-now airs the server copy', () => {
    const s = new RunSheetStore();
    const a = paperStory('a', 'A'); s.place(a, 'end');
    const op = s.pending[0];
    s.acknowledge([op], [{ op_id: op.op_id, applied: false, dropped: 'invalid', version: 0 }]);
    expect(s.entry(a.id)).toBeUndefined(); expect(s.pending.length).toBe(0);
    const b = paperStory('b', 'B'); s.place(b, 'now');
    const opb = s.pending[0];
    s.acknowledge([opb], [{ op_id: opb.op_id, applied: false, dropped: 'duplicate_id', version: 0 }]);
    expect(s.entry(b.id)?.state).toBe('on_air');
    expect(s.pending[0].op).toBe('aired'); expect(s.pending[0].state).toBe('on_air');
  });
  test('a server epoch that differs takes the server sheet and applies what is pending on top', async () => {
    const s = new RunSheetStore();
    const theirs = { ...emptySheet(), version: 7, sheet_epoch: 3, entries: [story('d-1')] };
    const p: Poster = { post: async () => ({ kind: 'answeredEpoch', results: [], epoch: 3 }), fetchSheet: async () => theirs };
    s.poster = p;
    s.place(paperStory('mine', 'Mine'), 'end'); await s.flushTask;
    expect(s.sheetEpoch).toBe(3);
    expect(s.sheet.entries.map((e) => e.id).slice(0, 1)).toEqual(['d-1']);
  });
  test('sign-out wipes the sheet and the pending ops and posts nothing afterwards', async () => {
    const mem = memory();
    const s = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    const p = new FakePoster(); s.poster = p;
    s.place(paperStory('a', 'A'), 'end'); await s.flushTask;
    await s.wipe();
    expect(s.pending.length).toBe(0); expect(s.sheet.entries.length).toBe(0); expect(mem.text).toBeUndefined();
    const n = p.posts.length;
    s.place(paperStory('b', 'B'), 'end'); expect(p.posts.length).toBe(n);
  });
  test('a desk op made under an older account generation is refused', () => {
    let g = 1;
    const s = new RunSheetStore({ generation: () => g });
    g = 2;
    expect(s.applyFromDesk({ op_id: 'o1', by: 'desk', op: 'place', at: 'end', entry: story('d-1'), reason: 'x', base_version: 0 }, 1).dropped).toBe('stale_account');
  });
});

describe('the route poster', () => {
  const err = (status: number) => Object.assign(new Error(`x answered ${status}`), { status });
  test('404, 501, 429, 5xx, 403, 408 and 409 are not yet; only 400, 413 and 422 are answered and not retried; a transport error is offline', async () => {
    for (const st of [404, 501, 429, 503, 500]) expect((await routePoster(async () => { throw err(st); }).post([], 0)).kind).toBe('notYet');
    expect((await routePoster(async () => { throw err(422); }).post([], 0)).kind).toBe('answered');
    for (const st of [403, 408, 409, 401]) expect((await routePoster(async () => { throw err(st); }).post([], 0)).kind).toBe('notYet');
    expect((await routePoster(async () => { throw new Error('network'); }).post([], 0)).kind).toBe('offline');
  });
  test('an answer with a sheet_epoch is answeredEpoch; the epoch is sent only when held', async () => {
    const seen: unknown[] = [];
    const p = routePoster(async (_p, o) => { seen.push(o?.body); return { body: { results: [{ op_id: 'a', applied: true, version: 1 }], sheet_epoch: 4 } }; });
    const r = await p.post([], 2);
    expect(r.kind).toBe('answeredEpoch');
    expect(seen[0]).toEqual({ ops: [], sheet_epoch: 2 });
    await p.post([], 0);
    expect(seen[1]).toEqual({ ops: [] });
  });
});

describe('what plays next', () => {
  test('13: the ahead list is bounded at 100; the desk is refused, a listener takes the last place', () => {
    const s = new RunSheetStore();
    for (let n = 0; n < 100; n++) deskPlace(s, story(`d-${n}`));
    expect(s.ahead().length).toBe(100);
    const r = deskPlace(s, story('d-over'));
    expect(r.applied).toBe(false); expect(r.dropped).toBe('ahead_full');
    const mine = paperStory('r', 'Mine');
    expect(s.place(mine, 'end').applied).toBe(true);
    const a = s.ahead();
    expect(a.length).toBe(100); expect(a.at(-1)?.id).toBe(mine.id); expect(s.entry('d-99')).toBeUndefined();
  });
  test('15: queue this goes after the listener\'s last and play this keeps what is ahead', () => {
    const s = new RunSheetStore();
    opened(s, ['d-1', 'd-2']);
    const a = paperStory('a', 'A'), b = paperStory('b', 'B');
    s.place(a, 'queued'); s.place(b, 'queued');
    expect(s.ahead().map((e) => e.id)).toEqual([a.id, b.id, 'd-1', 'd-2']);
    const c = paperStory('c', 'C');
    s.place(c, 'now', { cutting: { block: 2, seconds: 19 } });
    expect(s.onAir()?.id).toBe(c.id);
    expect(s.ahead().map((e) => e.id)).toEqual([a.id, b.id, 'd-1', 'd-2']);
  });
  test('16: previous returns the entry before, and the one on air, to ahead', () => {
    const s = new RunSheetStore();
    const o = opened(s, ['d-1']);
    s.skipOnAir(); s.airedOnAir('d-1');
    expect(s.onAir()?.id).toBe('d-1');
    expect(s.previous().applied).toBe(true);
    expect(s.ahead().map((e) => e.id)).toEqual([o, 'd-1']);
  });
  test('18: Up next is every entry ahead, in order', () => {
    const s = new RunSheetStore();
    opened(s, ['d-1', 'd-2', 'd-3', 'd-4', 'd-5']);
    expect(s.ahead().map((e) => e.id)).toEqual(['d-1', 'd-2', 'd-3', 'd-4', 'd-5']);
    expect(s.onAir()?.title).toBe('Opening');
  });
  test('skip with nothing on air does nothing; skip is one op and the next entry is not promoted by it', () => {
    const s = new RunSheetStore();
    expect(s.skipOnAir()).toBeUndefined();
    const o = opened(s, ['d-1']);
    const before = s.pending.length;
    s.skipOnAir();
    expect(s.pending.slice(before).map((p) => p.op)).toEqual(['skip']);
    expect(s.onAir()).toBeUndefined(); expect(s.entry(o)?.ended?.outcome).toBe('skipped'); expect(s.entry('d-1')?.state).toBe('ahead');
  });
  test('reading changes nothing', () => {
    const s = new RunSheetStore();
    opened(s, ['d-1']);
    const v = s.sheet.version;
    s.ahead(3); s.heard(3); s.onAir(); s.aheadSeconds;
    expect(s.sheet.version).toBe(v);
  });
});

describe('honest air reports', () => {
  const last = (s: RunSheetStore, k: Op['op']) => s.pending.filter((o) => o.op === k).at(-1);
  test('24: a failure is reported as failed by the phone, never as a listener skip', () => {
    const s = new RunSheetStore();
    opened(s, ['d-1']);
    s.skipOnAir(); s.airedOnAir('d-1');
    const before = s.pending.length;
    s.airedEnded('d-1', 'skipped', 'failed', { block: 0, seconds: 0 }, 'could not play');
    const op = s.pending[before];
    expect(op.by).toBe('phone'); expect(op.ended?.outcome).toBe('failed'); expect(s.entry('d-1')?.ended?.outcome).toBe('failed');
  });
  test('24: sign-out leaves the air as left by the phone at the true position', () => {
    const s = new RunSheetStore();
    const o = opened(s, ['d-1']);
    s.airedEnded(o, 'heard', 'left', { block: 2, seconds: 31.5 });
    const op = last(s, 'aired') as Op;
    expect(op.id).toBe(o); expect(op.by).toBe('phone'); expect(op.state).toBe('heard');
    expect(op.ended?.outcome).toBe('left'); expect(op.ended?.last_block).toBe(2); expect(op.position_seconds).toBe(31.5);
    s.clearAhead('phone', 'sign-out');
    expect(last(s, 'clear')?.by).toBe('phone');
  });
  test('24: a natural end carries the final block and position', () => {
    const s = new RunSheetStore();
    const o = opened(s, []);
    s.airedEnded(o, 'heard', 'finished', { block: 3, seconds: 42 });
    const op = last(s, 'aired') as Op;
    expect(op.ended?.outcome).toBe('finished'); expect(op.ended?.last_block).toBe(3); expect(op.position_seconds).toBe(42);
  });
  test('24: a listener skip carries the block being spoken and the seconds in', () => {
    const s = new RunSheetStore();
    opened(s, []);
    s.skipOnAir({ block: 1, seconds: 7.5 });
    const op = last(s, 'skip') as Op;
    expect(op.by).toBe('listener'); expect(op.ended?.outcome).toBe('skipped'); expect(op.ended?.last_block).toBe(1); expect(op.position_seconds).toBe(7.5);
  });
  test('24: play now reports where it cut the entry on air', () => {
    const s = new RunSheetStore();
    const o = opened(s, []);
    s.place(paperStory('r', 'R'), 'now', { cutting: { block: 2, seconds: 19 } });
    const op = last(s, 'place') as Op;
    expect(op.by).toBe('listener'); expect(op.ended?.outcome).toBe('cut'); expect(op.ended?.last_block).toBe(2); expect(op.position_seconds).toBe(19);
    expect(s.entry(o)?.ended?.outcome).toBe('cut');
  });
  test('24: previous is a previous op by the listener and posts no heard report', () => {
    const s = new RunSheetStore();
    opened(s, ['d-1']);
    s.skipOnAir(); s.airedOnAir('d-1');
    s.previous();
    expect(last(s, 'previous')?.by).toBe('listener');
  });
  test('24: removes are by who did them', () => {
    const s = new RunSheetStore();
    opened(s, []);
    const mine = paperStory('m', 'Mine'); s.place(mine, 'queued');
    s.remove(mine.id);
    expect(last(s, 'remove')?.by).toBe('listener'); expect(last(s, 'remove')?.reason).toBeUndefined();
    const x = paperStory('x', 'X'); s.place(x, 'end');
    s.remove(x.id, 'phone', 'transition_orphaned');
    expect(last(s, 'remove')?.by).toBe('phone'); expect(last(s, 'remove')?.reason).toBe('transition_orphaned');
  });
});

describe('the server\'s bounds', () => {
  test('25: an entry built too big is cut at the bounds and applies', () => {
    const s = new RunSheetStore();
    const long = 'word '.repeat(600);
    const e: Entry = {
      id: 'x', kind: 'story', placed_by: 'desk', state: 'ahead', title: long, text: 'a'.repeat(20_000), est_seconds: 1e9, revision: 0,
      source: { ref: 'r'.repeat(900), label: long, url: 'u'.repeat(900) },
      cites: Array.from({ length: 20 }, (_, i) => ({ ref: `pub:${i}`, role: 'mentioned' as const, label: long })),
      forecast: { p_heard: 4, caused_by: 'c'.repeat(400), claim_id: 'k' },
    };
    const r = s.place(e, 'end', { by: 'desk', reason: 'r'.repeat(500) });
    expect(r.applied).toBe(true);
    const got = s.entry('x') as Entry;
    const b = (x: string) => new TextEncoder().encode(x).length;
    expect(b(got.title)).toBeLessThanOrEqual(300); expect(b(got.text as string)).toBeLessThanOrEqual(8192);
    expect(b(got.source?.ref as string)).toBe(900);          // a ref is never cut
    expect(b(got.source?.label as string)).toBeLessThanOrEqual(512); expect(b(got.source?.url as string)).toBe(900);
    expect(got.cites?.length).toBe(12); expect(got.est_seconds).toBe(86400); expect(got.forecast?.p_heard).toBe(1);
    expect(b(got.forecast?.caused_by as string)).toBeLessThanOrEqual(160);
    expect(entryJSONSize(got)).toBeLessThanOrEqual(Bounds.maxEntryJSON);
    const multi = paperStory('r', '\u{1F600}'.repeat(200));
    expect(s.place(multi, 'end').applied).toBe(true); expect(b(s.entry(multi.id)?.title as string)).toBeLessThanOrEqual(300);
  });
  test('cutBytes never splits a character', () => expect(cutBytes('é'.repeat(10), 5)).toBe('éé'));
  test('26: a long url is left out and the ref stays; a long ref is not placed; a long cite is dropped', () => {
    const s = new RunSheetStore();
    const long = 'https://x.test/' + 'a'.repeat(2500) + '.mp3';
    const a = audio({ url: long, title: 'Long', show: 'S', episode_id: 'ep-1' });
    expect(s.place(a, 'end').applied).toBe(true);
    expect(s.entry(a.id)?.source?.url).toBeUndefined();
    expect(s.entry(a.id)?.source?.ref).toBe('episode:ep-1');
    const page = webPage('https://x.test/' + 'b'.repeat(2200), 'P');
    const r = s.place(page, 'queued');
    expect(r.applied).toBe(false); expect(r.dropped).toBe('ref_too_long'); expect(s.entry(page.id)).toBeUndefined();
    const withCites = story('c1', { cites: [{ ref: 'pub:' + 'r'.repeat(2100), role: 'mentioned', label: 'L' }, { ref: 'pub:ok', role: 'subject', label: 'OK' }] });
    expect(s.place(withCites, 'end', { by: 'phone' }).applied).toBe(true);
    expect(s.entry('c1')?.cites?.map((c) => c.ref)).toEqual(['pub:ok']);
  });
  test('26: the size bound is of the carried form and the entry is cut to fit', () => {
    const s = new RunSheetStore();
    expect(entryJSONSize(story('a', { text: '<'.repeat(8192) }))).toBeGreaterThan(Bounds.maxEntryJSON);
    expect(entryJSONSize(story('b', { text: '<'.repeat(4000) }))).toBeLessThan(Bounds.maxEntryJSON);
    const quotes = story('q', { text: '"'.repeat(8000) });
    expect(s.place(quotes, 'end', { by: 'phone' }).applied).toBe(true);
    expect(entryJSONSize(s.entry('q') as Entry)).toBeLessThanOrEqual(Bounds.maxEntryJSON);
  });
  test('26: an op carries nothing it does not use', () => {
    const e = paperStory('r', 'T');
    expect(apply(emptySheet(), { op_id: 'o1', by: 'listener', op: 'remove', id: 'x', entry: e, base_version: 0 }).result.dropped).toBe('invalid');
    expect(apply(emptySheet(), { op_id: 'o2', by: 'listener', op: 'remove', id: 'x', text: 't', base_version: 0 }).result.dropped).toBe('invalid');
  });
  test('a phone path is never posted', () => {
    const s = new RunSheetStore({ home: '/Users/someone/Library', tmp: '/private/var/tmp/x' });
    const e = paperStory('r', 'T'); e.source = { ref: 'file:///Users/someone/Library/a.mp3', label: 'x' };
    expect(s.place(e, 'end').dropped).toBe('local_path');
  });
});

// The review's findings on the red poster (2026-11-21), each written to fail first.
const op = (id: string): Op => ({ op_id: id, by: 'listener', op: 'remove', id: 'x', base_version: 0 });
describe('what a refusal drops', () => {
  const err = (status: number) => Object.assign(new Error(`x answered ${status}`), { status });
  test('a 400 for a batch is sent again one op at a time: only the op the server refuses is dropped', async () => {
    const seen: string[][] = [];
    const p = routePoster(async (_path, o) => {
      const ops = (o?.body as { ops: Op[] }).ops; seen.push(ops.map((x) => x.op_id));
      if (ops.some((x) => x.op_id === 'bad')) throw err(400);
      return { body: { results: ops.map((x) => ({ op_id: x.op_id, applied: true, version: 1 })) } };
    });
    const out = await p.post([op('a'), op('bad'), op('c')], 0);
    expect(out.kind).toBe('answered');
    const r = (out as { results: { op_id: string; applied: boolean }[] }).results;
    expect(r.map((x) => [x.op_id, x.applied])).toEqual([['a', true], ['bad', false], ['c', true]]);
    expect(seen).toEqual([['a', 'bad', 'c'], ['a'], ['bad'], ['c']]);
  });
  test('a 413 for one op drops that op; a route that goes quiet while isolating leaves the whole batch pending', async () => {
    expect((await routePoster(async () => { throw err(413); }).post([op('a')], 0)).kind).toBe('answered');
    let n = 0;
    const p = routePoster(async (_p, o) => { const ops = (o?.body as { ops: Op[] }).ops; if (ops.length > 1) throw err(400); if (++n === 2) throw new Error('network'); return { body: { results: [{ op_id: ops[0].op_id, applied: true }] } }; });
    expect((await p.post([op('a'), op('b')], 0)).kind).toBe('offline');
  });
  test('a batch made under an account that has moved on is never sent', async () => {
    let called = 0, g = 2;
    const p = routePoster(async () => { called++; return { body: {} }; }, () => g);
    expect((await p.post([op('a')], 0, 1)).kind).toBe('offline'); expect(called).toBe(0);
    g = 1; expect((await p.post([op('a')], 0, 1)).kind).toBe('answered'); expect(called).toBe(1);
  });
  test('the request carries the account generation it was made under', async () => {
    let world: number | undefined;
    await routePoster(async (_p, o) => { world = o?.world; return { body: {} }; }).post([op('a')], 0, 5);
    expect(world).toBe(5);
  });
});

describe('a sign-out while an answer is on its way', () => {
  const gate = () => { let go!: () => void; const p = new Promise<void>((r) => { go = r; }); return { p, go }; };
  test('a repaired sheet that arrives after the wipe is never written for the next account', async () => {
    const mem = memory(), g = gate();
    const theirs = { ...emptySheet(), sheet_epoch: 7, entries: [story('d-old')] };
    const poster: Poster = { post: async () => ({ kind: 'answeredEpoch', results: [], epoch: 7 }), fetchSheet: async () => { await g.p; return theirs; } };
    const s = new RunSheetStore({ storage: mem, poster });
    s.place(story('l-1', { placed_by: 'listener' }), 'end');
    for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));       // the post answered; the repaired sheet is being fetched
    await s.wipe();
    g.go(); await s.flushTask;
    for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
    expect(s.sheet.entries).toEqual([]); expect(s.sheetEpoch).toBe(0); expect(mem.text).toBeUndefined();
  });
  test('a save that was in flight lands before the wipe removes the file, never after it', async () => {
    const g = gate(); const mem = memory();
    const slow: Storage & { text?: string } = { ...mem, write: async (t) => { await g.p; mem.text = t; slow.text = t; }, remove: async () => { mem.text = undefined; slow.text = undefined; } };
    const s = new RunSheetStore({ storage: slow });
    s.place(story('l-1', { placed_by: 'listener' }), 'end');        // a save is started and waits
    const wiped = s.wipe();
    g.go(); await wiped;
    expect(mem.text).toBeUndefined();
  });
  test('an answer for a batch made under an older account generation changes nothing', async () => {
    const g = gate(); let gen = 1;
    const poster: Poster = { post: async () => { await g.p; return { kind: 'answered', results: [] }; }, fetchSheet: async () => undefined };
    const s = new RunSheetStore({ poster, generation: () => gen });
    s.place(story('l-1', { placed_by: 'listener' }), 'end');
    gen = 2; g.go(); await s.flushTask;
    expect(s.pending.length).toBe(1);                                  // not marked answered under the wrong account
  });
});

describe('the deck\'s own time and ids', () => {
  test('a verdict is stamped with the time the deck gave, not the clock of the pump', () => {
    const s = new RunSheetStore({ now: () => new Date('2023-11-14T07:00:00Z') });
    const id = opened(s, ['d-a']);
    s.airedEnded(id, 'heard', 'finished', { block: 1, seconds: 4, at: '2023-11-14T03:00:00.250Z' });
    expect(s.entry(id)?.ended?.at).toBe('2023-11-14T03:00:00Z');
  });
  test('a minted id carries at least 96 random bits and is valid', () => {
    const s = new RunSheetStore();
    const a = s.mintOpId(), b = s.mintOpId();
    expect(a).not.toBe(b); expect(a.length).toBeGreaterThanOrEqual(2 + 24); expect(a).toMatch(/^p-[0-9a-f]{24}$/);
  });
});

// B1-3, B1-4, B1-8 (the train review): the post honours the route's Retry-After and backs off; the pending list is bounded by a stated rule that says what it dropped; saves coalesce; a restore that cannot read an op says so
// and keeps the rest.
describe('posting backs off and the pending list is bounded', () => {
  const at = { t: Date.parse('2023-11-14T22:00:00Z') };
  const mk = (extra: Partial<ConstructorParameters<typeof RunSheetStore>[0]> = {}) => { const p = new FakePoster(); const s = new RunSheetStore({ poster: p, now: () => new Date(at.t), retryAfterMs: 3_600_000, ...extra }); return { p, s }; };
  test('B1-3 a notYet with a Retry-After holds the next post until then; with none the wait doubles from 30 s to 5 min', async () => {
    const { p, s } = mk(); at.t = Date.parse('2023-11-14T22:00:00Z');
    p.answer = { kind: 'notYet', retryAfterMs: 120_000 };
    deskPlace(s, story('d-a')); s.place(story('l-1', { placed_by: 'listener' }), 'end', { by: 'listener' }); await s.flushTask;
    expect(p.posts.length).toBe(1);
    at.t += 60_000; s.flush(); await s.flushTask; expect(p.posts.length).toBe(1);        // inside Retry-After: not sent
    at.t += 61_000; p.answer = { kind: 'notYet' }; s.flush(); await s.flushTask; expect(p.posts.length).toBe(2);
    at.t += 59_000; s.flush(); await s.flushTask; expect(p.posts.length).toBe(2);        // the second failure in a row: 60 s (30 s doubling, the first had a Retry-After of its own)
    at.t += 2_000; s.flush(); await s.flushTask; expect(p.posts.length).toBe(3);
    at.t += 119_000; s.flush(); await s.flushTask; expect(p.posts.length).toBe(3);       // the third: 120 s
    at.t += 2_000; p.answer = { kind: 'answered', results: [] }; s.flush(); await s.flushTask; expect(p.posts.length).toBe(4);
    expect(s.pending.length).toBe(0);
  });
  test('B1-3 the sign-out settle posts whatever the hold says', async () => {
    const { p, s } = mk(); p.answer = { kind: 'notYet', retryAfterMs: 600_000 };
    s.place(story('l-1', { placed_by: 'listener' }), 'end', { by: 'listener' }); await s.flushTask;
    p.answer = { kind: 'answered', results: [] }; await s.settlePending(500);
    expect(s.pending.length).toBe(0);
  });
  test('B1-3 offline for long: the pending list is bounded at 2000 ops; the oldest air reports go first, said, counted; a place is never dropped', async () => {
    const { p, s } = mk(); void p;
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try {
      s.place(story('l-keep', { placed_by: 'listener' }), 'end', { by: 'listener' });
      for (let i = 0; i < 2100; i++) s.pending.push({ op_id: `p-${String(i).padStart(6, '0')}`, by: 'phone', op: 'aired', id: 'x', state: 'on_air', position_seconds: 0, base_version: 0 } as Op);
      s.place(story('l-two', { placed_by: 'listener' }), 'end', { by: 'listener' });
    } finally { console.log = log; }
    expect(s.pending.length).toBeLessThanOrEqual(2000);
    expect(s.lostPending).toBeGreaterThanOrEqual(100);
    expect(s.pending.filter((o) => o.op === 'place').length).toBe(2);
    expect(logs.some((l) => l.includes('pending') && l.includes('dropped'))).toBe(true);
  });
  test('B1-4 forty placements in one turn make one write and one trailing write, not forty', async () => {
    const mem = memory(); let writes = 0; const w = mem.write; mem.write = async (t) => { writes += 1; await new Promise((r) => setTimeout(r, 1)); await w(t); };
    const { s } = mk({ storage: mem });
    for (let i = 0; i < 40; i++) deskPlace(s, story('d-' + i));
    await s.save();
    expect(writes).toBeLessThanOrEqual(3);
    expect(JSON.parse(mem.text as string).sheet.entries.length).toBe(40);                // the last state is what is on disk
  });
  test('B1-8 a restore that cannot read one pending op keeps the sheet and the other ops, and says how many it lost', async () => {
    const mem = memory();
    const a = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 }); a.place(story('l-1', { placed_by: 'listener' }), 'end', { by: 'listener' }); await a.save();
    const saved = JSON.parse(mem.text as string); saved.pending.push({ nonsense: true }, saved.pending[0]); mem.text = JSON.stringify(saved);
    const logs: string[] = []; const log = console.log; console.log = (...x: unknown[]) => { logs.push(x.join(' ')); };
    const b = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    try { await b.restore(); } finally { console.log = log; }
    expect(b.entry('l-1')).toBeDefined(); expect(b.pending.length).toBeGreaterThanOrEqual(1);
    expect(logs.some((l) => /could not be read/.test(l) && /1/.test(l))).toBe(true);
  });
});

describe('the pending bound drops whole chains and says so (seat 2, item 6)', () => {
  const chain = (i: number): Op[] => {
    const id = `l-${String(i).padStart(5, '0')}`;
    return [
      { op_id: `p-${i}-a`, by: 'listener', op: 'place', at: 'end', entry: { id, kind: 'story', placed_by: 'listener', state: 'ahead', title: 'T', text: 'W.', est_seconds: 5, revision: 0 } as Entry, base_version: 0 } as Op,
      { op_id: `p-${i}-b`, by: 'phone', op: 'aired', id, state: 'on_air', position_seconds: 0, base_version: 0 } as Op,
      { op_id: `p-${i}-c`, by: 'phone', op: 'aired', id, state: 'heard', ended: { outcome: 'finished', at: '2023-11-14T22:00:00Z', last_block: 0 }, position_seconds: 1, base_version: 0 } as Op,
    ];
  };
  test('B1-3b a long offline stretch of place, air and end ops is bounded by whole entries: no entry keeps its end without its place', () => {
    const s = new RunSheetStore({ retryAfterMs: 3_600_000 });
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { for (let i = 0; i < 720; i++) s.pending.push(...chain(i)); s.place(story('l-last', { placed_by: 'listener' }), 'end', { by: 'listener' }); } finally { console.log = log; }
    expect(s.pending.length).toBeLessThanOrEqual(2000);
    const by = new Map<string, string[]>();
    for (const o of s.pending) { const k = o.entry?.id ?? o.id ?? ''; by.set(k, [...(by.get(k) ?? []), o.op + ':' + (o.state ?? '')]); }
    for (const [k, v] of by) if (k.startsWith('l-0')) expect(v, k).toEqual(['place:', 'aired:on_air', 'aired:heard']);
    expect(s.lostPending).toBeGreaterThan(0); expect(logs.some((l) => l.includes('dropped'))).toBe(true);
    expect(by.get('l-last')?.[0]).toBe('place:');                                     // the newest act is never the one dropped
  });
  test('B1-3c a restore trims a list that is over the bound', async () => {
    const mem = memory();
    const a = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    for (let i = 0; i < 720; i++) a.pending.push(...chain(i));
    await a.save();
    const b = new RunSheetStore({ storage: mem, retryAfterMs: 3_600_000 });
    const log = console.log; console.log = () => undefined; try { await b.restore(); } finally { console.log = log; }
    expect(b.pending.length).toBeLessThanOrEqual(2000);
  });
});

describe('the pending bound keeps the listener\'s acts (gen review)', () => {
  const rm = (i: number, id: string): Op => ({ op_id: `p-${i}-r`, by: 'listener', op: 'remove', id, base_version: 0 } as Op);
  const report = (i: number, id: string): Op => ({ op_id: `p-${i}-a`, by: 'phone', op: 'aired', id, state: 'on_air', position_seconds: 0, base_version: 0 } as Op);
  test('B1-3d acts (remove, move, skip) outlive reports; a skip counts as an end', () => {
    const s = new RunSheetStore({ retryAfterMs: 3_600_000 });
    const log = console.log; console.log = () => undefined;
    try {
      s.pending.push(rm(0, 'x-act'));                                                // the oldest op is a listener act with no place in the list
      for (let i = 1; i <= 2100; i++) s.pending.push(report(i, `y-${i}`));           // 2100 chains of reports only
      s.place(story('l-last', { placed_by: 'listener' }), 'end', { by: 'listener' });
    } finally { console.log = log; }
    expect(s.pending.length).toBeLessThanOrEqual(2000);
    expect(s.pending.some((o) => o.op === 'remove' && o.id === 'x-act')).toBe(true);
    const t = new RunSheetStore({ retryAfterMs: 3_600_000 });
    const log2 = console.log; console.log = () => undefined;
    try {
      t.pending.push({ op_id: 'p-sk', by: 'listener', op: 'skip', id: 'z-skipped', ended: { outcome: 'skipped', at: '2023-11-14T22:00:00Z', last_block: 0 }, base_version: 0 } as Op);
      t.pending.push(...Array.from({ length: 2001 }, (_, i) => ({ op_id: `p-pl-${i}`, by: 'listener', op: 'place', at: 'end', entry: { id: `l-${i}`, kind: 'story', placed_by: 'listener', state: 'ahead', title: 'T', text: 'W.', est_seconds: 5, revision: 0 } as Entry, base_version: 0 } as Op)));
      t.place(story('l-last', { placed_by: 'listener' }), 'end', { by: 'listener' });
    } finally { console.log = log2; }
    expect(t.pending.some((o) => o.id === 'z-skipped')).toBe(false);                    // the skipped entry's chain went first (it is over); the places stayed
    expect(t.pending.filter((o) => o.op === 'place').length).toBeGreaterThan(1990);
  });
  test('B1-3e the bound holds even when one chain is the whole excess', () => {
    const s = new RunSheetStore({ retryAfterMs: 3_600_000 });
    const log = console.log; console.log = () => undefined;
    try { for (let i = 0; i < 2100; i++) s.pending.push(report(i, 'only')); s.place(story('l-last', { placed_by: 'listener' }), 'end', { by: 'listener' }); } finally { console.log = log; }
    expect(s.pending.length).toBeLessThanOrEqual(2000);
  });
});
