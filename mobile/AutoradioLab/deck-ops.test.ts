// What the phone posts from the deck's journal, under bun test: deck-ops.ts with a fake deck and a fake poster. Nothing plays.
// Failure list, written before the code (each line has a test below):
//  D1  every fixture in fixtures/deck: the ops built from the journal are exactly the ops the fixture carries (the Go reducer test applies the same
//      ops: internal/runsheet/deck_fixtures_test.go), the journal is consumed up to the same number, and the same anomalies are named.
//  D2  an item is heard only if the deck says sound was rendered; "finished" with none is failed.        D3  the same journal builds the same op ids.
//  D4  op ids are valid ids of at most 64 characters, whatever the entry id.       D5  an op never leaves before the end of the item before it.
//  D6  heldPlan keeps what already matches, drops from the first mismatch, and never holds more than the deck's limit.
//  D7  postJournal acknowledges only what the server answered; a route that is not there or no network leaves the journal whole and in order.
//  D8  a batch is at most 64 ops and 256 KB; a failed batch stops the rest; a later call sends from where it stopped.
//  D9  a refused op (unknown id, duplicate) is acknowledged: resending cannot help, and a duplicate means it was already counted.
//  D10 the server's repaired-sheet answer is acknowledged and handed back so the sheet is taken before more is posted.
//  D11 two posts at once post once.      D12 the HTTP answers map as the Swift poster did.
//  D13 the names in deck.ts, deck-types.contract and interface/deck.json are one list.
//  D14 a gap at the FIRST unacknowledged line is named (the cap's loss looks like this).   D15 a `dropped` system line is named, with its number.
//  D16 the interface carries no `level` op and no `rate` field (a no-op and a constant are lies shared code would believe).
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
  BATCH_BYTES, BATCH_OPS, heldPlan, opId, opsFromJournal, outcomeFromHTTP, postJournal, syncHeld, type PostOutcome, type Poster, type WireOp,
} from './deck-ops.ts';
import { DECK_HELD_MAX, DECK_NAMES, item, type DeckEvent, type DeckItem, type DeckPort, type DeckState } from './deck.ts';

const DIR = join(import.meta.dir, '..', 'fixtures', 'deck');
const fixtures = readdirSync(DIR).filter((f) => f.endsWith('.json')).sort().map((f) => JSON.parse(readFileSync(join(DIR, f), 'utf8')));

const ev = (seq: number, type: DeckEvent['type'], over: Partial<DeckEvent> = {}): DeckEvent => ({
  seq, type, item: 'a', at: '2026-11-19T10:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '', ...over,
});

describe('D1 fixtures', () => {
  test('there are fixtures', () => expect(fixtures.length).toBeGreaterThanOrEqual(10));
  for (const fx of fixtures) {
    test(fx.name, () => {
      const built = opsFromJournal(fx.journal);
      expect(built.ops).toEqual(fx.ops);
      expect(built.through).toBe(fx.through);
      expect(built.anomalies).toEqual(fx.anomalies);
    });
  }
});

describe('D2 heard needs sound', () => {
  test('finished with sound is heard; without it is skipped and failed', () => {
    const a = opsFromJournal([ev(1, 'ended', { end: 'finished', rendered: true, final: 4, block: 4 })]).ops[0];
    expect([a.state, a.ended?.outcome]).toEqual(['heard', 'finished']);
    const b = opsFromJournal([ev(1, 'ended', { end: 'finished', rendered: false, final: 4 })]).ops[0];
    expect([b.state, b.ended?.outcome]).toEqual(['skipped', 'failed']);
  });
  test('no line that is not a finished one with sound makes a finished outcome', () => {
    for (const end of ['skipped', 'replaced', 'stopped', 'failed'] as const) {
      for (const rendered of [true, false]) {
        const op = opsFromJournal([ev(1, 'ended', { end, rendered })]).ops[0];
        expect(op.ended?.outcome).not.toBe('finished');
      }
    }
  });
  test('a started line, a sounding line, a remote line or a system line alone never says heard', () => {
    const ops = opsFromJournal([ev(1, 'started'), ev(2, 'sounding', { rendered: true }), ev(3, 'remote', { command: 'next' }), ev(4, 'system', { command: 'route-lost' })]).ops;
    expect(ops.map((o) => o.state)).toEqual(['on_air']);
  });
});

describe('D3 D4 op ids', () => {
  test('the same line makes the same id, a different number a different one', () => {
    const l = ev(7, 'ended', { end: 'finished', rendered: true });
    expect(opsFromJournal([l]).ops[0].op_id).toBe(opsFromJournal([l]).ops[0].op_id);
    expect(opId(l)).not.toBe(opId({ ...l, seq: 8 }));
    expect(opId(l)).not.toBe(opId({ ...l, at: '2026-11-19T10:00:01Z' }));
  });
  test('valid whatever the entry id', () => {
    for (const id of ['a', 'd-' + 'x'.repeat(100), 'p-1a2b3c4d', 'weird id/with:chars', '']) {
      const o = opId(ev(123456, 'started', { item: id }));
      expect(o.length).toBeLessThanOrEqual(64);
      expect(/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(o)).toBe(true);
    }
  });
});

describe('D5 order', () => {
  test('an end then a start is in order; a start over an open item stops the build before it', () => {
    const ok = opsFromJournal([ev(1, 'started'), ev(2, 'ended', { end: 'skipped', rendered: true }), ev(3, 'started', { item: 'b' })]);
    expect(ok.ops.map((o) => `${o.id}:${o.state}`)).toEqual(['a:on_air', 'a:skipped', 'b:on_air']);
    const bad = opsFromJournal([ev(1, 'started'), ev(2, 'started', { item: 'b' })]);
    expect(bad.ops.length).toBe(1);
    expect(bad.through).toBe(1);
    expect(bad.anomalies).toEqual(['start_before_end:2']);
  });
  test('after a given number only later lines are read', () => {
    const lines = [ev(1, 'started'), ev(2, 'ended', { end: 'finished', rendered: true })];
    const r = opsFromJournal(lines, 1);
    expect(r.ops.length).toBe(1);
    expect(r.through).toBe(2);
  });
  test('the same item started twice is one start', () => {
    expect(opsFromJournal([ev(1, 'started'), ev(2, 'started')]).ops.length).toBe(1);
  });
});

describe('D14 D15 what a loss looks like', () => {
  test('the first unacknowledged line is not the next number: the gap is named, the ops are still built', () => {
    const r = opsFromJournal([ev(3, 'started'), ev(4, 'ended', { end: 'finished', rendered: true })], 0);
    expect(r.anomalies).toEqual(['gap:1']); expect(r.ops.length).toBe(2);
    const s = opsFromJournal([ev(5, 'started'), ev(6, 'ended', { end: 'finished', rendered: true })], 2);
    expect(s.anomalies).toEqual(['gap:3']);
    expect(opsFromJournal([ev(3, 'started'), ev(4, 'ended', { end: 'finished', rendered: true })], 2).anomalies).toEqual([]);
  });
  test('a system line that says lines were dropped is an anomaly with its number', () => {
    const r = opsFromJournal([ev(1, 'started'), ev(2, 'system', { item: '', command: 'dropped', reason: 'dropped 200 unacknowledged lines' }), ev(3, 'ended', { end: 'finished', rendered: true })]);
    expect(r.anomalies).toEqual(['dropped:2']); expect(r.ops.length).toBe(2); expect(r.through).toBe(3);
    expect(opsFromJournal([ev(1, 'system', { item: '', command: 'engine-lost' })]).anomalies).toEqual([]);
  });
});

describe('D16 the interface tells no lie', () => {
  test('no level op and no rate field in deck.ts, deck-types.contract, interface/deck.json', () => {
    const dir = import.meta.dir;
    for (const f of [join(dir, 'deck.ts'), join(dir, 'deck-types.contract'), join(dir, '..', 'exact-package', 'interface', 'deck.json')]) {      // (exact.json is generated from the fragments; the next test reads its deck entries too)
      const src = readFileSync(f, 'utf8');
      expect(src).not.toMatch(/'level'|"level"|\brate\b/);
    }
  });
});

describe('D16b the generated interface agrees', () => {
  test('exact.json declares no level op and no DeckState.rate', () => {
    const j = JSON.parse(readFileSync(join(import.meta.dir, '..', 'exact-package', 'exact.json'), 'utf8'));
    expect(JSON.stringify(j.shapes.DeckState ?? {})).not.toContain('rate');
    expect(JSON.stringify(j)).not.toMatch(/"level":\s*"string"[^}]*deck/i);
    const act = (j.names as { send?: string; options?: Record<string, string> }[]).find((n) => n.send === 'deckAct');
    expect(act).toBeDefined();
  });
});

describe('D6 heldPlan', () => {
  const it = (id: string): DeckItem => item({ id, kind: 'speech', paragraphs: id });
  test('what matches stays; a new one is appended', () => {
    expect(heldPlan(['a', 'b'], [it('a'), it('b'), it('c')])).toEqual({ keep: 2, enqueue: [{ at: 2, item: it('c') }] });
  });
  test('nothing to do', () => expect(heldPlan(['a'], [it('a')])).toEqual({ keep: 1, enqueue: [] }));
  test('a removed one in the middle drops the tail and holds it again, in order', () => {
    expect(heldPlan(['a', 'b', 'c'], [it('a'), it('c')])).toEqual({ keep: 1, enqueue: [{ at: 1, item: it('c') }] });
  });
  test('a shorter wanted list trims', () => expect(heldPlan(['a', 'b', 'c'], [it('a')])).toEqual({ keep: 1, enqueue: [] }));
  test('a reorder is made in order', () => {
    expect(heldPlan(['a', 'b'], [it('b'), it('a')])).toEqual({ keep: 0, enqueue: [{ at: 0, item: it('b') }, { at: 1, item: it('a') }] });
  });
  test('never more than the deck holds, and an id once', () => {
    const many = Array.from({ length: DECK_HELD_MAX + 5 }, (_, i) => it('i' + i));
    const p = heldPlan([], [...many, it('i0')]);
    expect(p.enqueue.length).toBe(DECK_HELD_MAX);
    expect(p.enqueue.map((e) => e.item.id)).toEqual(many.slice(0, DECK_HELD_MAX).map((i) => i.id));
  });
  test('syncHeld trims then holds through the port', async () => {
    const calls: string[] = [];
    const port = fakeDeck({ held: ['a', 'b', 'c'] }, calls);
    await syncHeld(port, [it('a'), it('x')]);
    expect(calls).toEqual(['enqueue:x@1']);
  });
  test('a shorter list is only trimmed', async () => {
    const calls: string[] = [];
    await syncHeld(fakeDeck({ held: ['a', 'b', 'c'] }, calls), [it('a')]);
    expect(calls).toEqual(['act:trim:1']);
  });
  test('nothing changed, nothing sent', async () => {
    const calls: string[] = [];
    await syncHeld(fakeDeck({ held: ['a'] }, calls), [it('a')]);
    expect(calls).toEqual([]);
  });
});

// ---- the post

function state(over: Partial<DeckState> = {}): DeckState {
  return { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [], ...over };
}
function fakeDeck(over: Partial<DeckState>, calls: string[] = []): DeckPort {
  let s = state(over);
  return {
    state: async () => s,
    play: async (i) => { calls.push('play:' + i.id); return s; },
    enqueue: async (i, at) => { calls.push(`enqueue:${i.id}@${at}`); return s; },
    act: async (op, v) => { calls.push(`act:${op}:${v}`); return s; },
    ack: async (n) => { calls.push('ack:' + n); s = { ...s, acked: n, journal: s.journal.filter((l) => l.seq > n) }; return s; },
  };
}
function finishedPair(n: number): DeckEvent[] {
  const out: DeckEvent[] = [];
  for (let i = 0; i < n; i++) {
    out.push(ev(i * 2 + 1, 'started', { item: 'i' + i }), ev(i * 2 + 2, 'ended', { item: 'i' + i, end: 'finished', rendered: true }));
  }
  return out;
}
function poster(script: PostOutcome[] | ((ops: WireOp[]) => PostOutcome), sent: WireOp[][] = []): Poster {
  let n = 0;
  return { post: async (ops) => { sent.push(ops); return typeof script === 'function' ? script(ops) : script[Math.min(n++, script.length - 1)]; } };
}
const answered = (ops: WireOp[], dropped = ''): PostOutcome => ({ kind: 'answered', results: ops.map((o) => ({ op_id: o.op_id, applied: !dropped, dropped })) });

describe('D7 acknowledged only when answered', () => {
  test('answered: acknowledged through the last line', async () => {
    const calls: string[] = [];
    const sent: WireOp[][] = [];
    const r = await postJournal(fakeDeck({ journal: finishedPair(2) }, calls), poster(answered, sent));
    expect(sent.map((b) => b.length)).toEqual([4]);
    expect(calls).toEqual(['ack:4']);
    expect(r.acked).toBe(4);
  });
  test('route not there, busy or offline: nothing is acknowledged, and the journal is as it was', async () => {
    for (const kind of ['notYet', 'offline'] as const) {
      const calls: string[] = [];
      const r = await postJournal(fakeDeck({ journal: finishedPair(1) }, calls), poster([{ kind }]));
      expect(calls).toEqual([]);
      expect(r.acked).toBe(0);
      expect(r.pending).toBe(2);
    }
  });
  test('an empty journal posts nothing', async () => {
    const sent: WireOp[][] = [];
    await postJournal(fakeDeck({}), poster(answered, sent));
    expect(sent).toEqual([]);
  });
  test('lines that make no op are acknowledged without a post', async () => {
    const calls: string[] = [];
    const sent: WireOp[][] = [];
    await postJournal(fakeDeck({ journal: [ev(1, 'remote', { command: 'pause' }), ev(2, 'system', { command: 'route-lost' })] }, calls), poster(answered, sent));
    expect(sent).toEqual([]);
    expect(calls).toEqual(['ack:2']);
  });
  test('ops after the acknowledged number only', async () => {
    const calls: string[] = [];
    const sent: WireOp[][] = [];
    await postJournal(fakeDeck({ acked: 2, journal: finishedPair(2) }, calls), poster(answered, sent));
    expect(sent[0].map((o) => o.id)).toEqual(['i1', 'i1']);
  });
});

describe('D8 batches', () => {
  test('64 ops a batch, in order', async () => {
    const sent: WireOp[][] = [];
    const calls: string[] = [];
    await postJournal(fakeDeck({ journal: finishedPair(40) }, calls), poster(answered, sent));
    expect(sent.map((b) => b.length)).toEqual([BATCH_OPS, 16]);
    expect(sent.flat().map((o) => o.op_id)).toEqual(opsFromJournal(finishedPair(40)).ops.map((o) => o.op_id));
    expect(calls).toEqual(['ack:64', 'ack:80']);
  });
  test('a batch over the byte limit is split', async () => {
    const sent: WireOp[][] = [];
    await postJournal(fakeDeck({ journal: finishedPair(20) }), poster(answered, sent), { maxBytes: 3000 });
    expect(sent.length).toBeGreaterThan(2);
    expect(sent.flat().length).toBe(40);
    for (const b of sent) expect(JSON.stringify({ ops: b }).length).toBeLessThanOrEqual(3000);
  });
  test('the real byte limit is the server body limit', () => expect(BATCH_BYTES).toBe(256 * 1024));
  test('a failed batch stops the rest; what was answered stays acknowledged; the next call goes on', async () => {
    const calls: string[] = [];
    const sent: WireOp[][] = [];
    const deck = fakeDeck({ journal: finishedPair(40) }, calls);
    const first = await postJournal(deck, poster([{ kind: 'answered', results: [] }, { kind: 'offline' }], sent));
    expect(first.acked).toBe(64);
    expect(sent.length).toBe(2);
    expect(first.pending).toBe(16);
    const sent2: WireOp[][] = [];
    await postJournal(deck, poster(answered, sent2));
    expect(sent2.map((b) => b.length)).toEqual([16]);
    expect(calls).toEqual(['ack:64', 'ack:80']);
  });
});

describe('D9 refusals', () => {
  test('unknown_id and duplicate are acknowledged and counted', async () => {
    for (const why of ['unknown_id', 'duplicate', 'not_on_air']) {
      const calls: string[] = [];
      const r = await postJournal(fakeDeck({ journal: finishedPair(1) }, calls), poster((ops) => answered(ops, why)));
      expect(calls).toEqual(['ack:2']);
      expect(r.dropped[why]).toBe(2);
    }
  });
  test('the log says counts and ids, no words', async () => {
    const lines: string[] = [];
    await postJournal(fakeDeck({ journal: [ev(1, 'started', { item: 'a' }), ev(2, 'ended', { item: 'a', end: 'failed', reason: 'could not play' })] }), poster((ops) => answered(ops, 'unknown_id')), { log: (l) => lines.push(l) });
    expect(lines.length).toBeGreaterThan(0);
    expect(lines.join('\n')).not.toContain('could not play');
  });
});

describe('D10 epoch', () => {
  test('acknowledged, stops, and hands the epoch back', async () => {
    const calls: string[] = [];
    const sent: WireOp[][] = [];
    const r = await postJournal(fakeDeck({ journal: finishedPair(40) }, calls), poster((ops) => ({ kind: 'answeredEpoch', results: [], epoch: 9 }) satisfies PostOutcome, sent));
    expect(r.epoch).toBe(9);
    expect(sent.length).toBe(1);
    expect(calls).toEqual(['ack:64']);
  });
  test('the epoch the phone holds goes with the post', async () => {
    let seen = -1;
    await postJournal(fakeDeck({ journal: finishedPair(1) }), { post: async (ops, epoch) => { seen = epoch; return answered(ops); } }, { epoch: 4 });
    expect(seen).toBe(4);
  });
});

describe('D11 once', () => {
  test('two posts at once post once', async () => {
    const sent: WireOp[][] = [];
    const deck = fakeDeck({ journal: finishedPair(1) });
    const slow: Poster = { post: async (ops) => { sent.push(ops); await new Promise((r) => setTimeout(r, 10)); return answered(ops); } };
    const [a, b] = await Promise.all([postJournal(deck, slow), postJournal(deck, slow)]);
    expect(sent.length).toBe(1);
    expect([a.busy, b.busy].sort()).toEqual([false, true]);
  });
  test('a thrown error frees it', async () => {
    const deck = fakeDeck({ journal: finishedPair(1) });
    await expect(postJournal(deck, { post: async () => { throw new Error('x'); } })).rejects.toThrow();
    const r = await postJournal(deck, poster(answered));
    expect(r.busy).toBe(false);
  });
});

describe('D12 http', () => {
  test('as the Swift poster answered', () => {
    expect(outcomeFromHTTP(200, { results: [{ op_id: 'a', applied: true }] })).toEqual({ kind: 'answered', results: [{ op_id: 'a', applied: true }] });
    expect(outcomeFromHTTP(200, { results: [], sheet_epoch: 5 })).toEqual({ kind: 'answeredEpoch', results: [], epoch: 5 });
    for (const s of [404, 501, 503, 429, 500, 502]) expect(outcomeFromHTTP(s, {}).kind).toBe('notYet');
    expect(outcomeFromHTTP(null, null).kind).toBe('offline');
    expect(outcomeFromHTTP(400, {}).kind).toBe('refused');
    expect(outcomeFromHTTP(413, {}).kind).toBe('refused');
    expect(outcomeFromHTTP(200, 'not an object')).toEqual({ kind: 'answered', results: [] });
  });
});

describe('D13 one list of names', () => {
  const iface = JSON.parse(readFileSync(join(import.meta.dir, '..', 'exact-package', 'interface', 'deck.json'), 'utf8'));
  const contract = readFileSync(join(import.meta.dir, 'deck-types.contract'), 'utf8');
  test('names', () => {
    const names = iface.names.map((n: { source?: string; send?: string }) => n.source ?? n.send).sort();
    expect(names).toEqual([...DECK_NAMES].sort());
  });
  test('shape fields match in the interface and the contract', () => {
    for (const shape of ['DeckEvent', 'DeckState']) {
      const fields = Object.keys(iface.shapes[shape]);
      const block = contract.split(`shape ${shape}\n`)[1].split(/\n\n|\nshape /)[0];
      const have = block.split('\n').map((l) => l.trim().split(':')[0]).filter(Boolean);
      expect(have).toEqual(fields);
    }
    const itemFields = contract.split('shape DeckItem\n')[1].split(/\n\n|\n\/\/|\nshape /)[0].split('\n').map((l) => l.trim().split(':')[0]).filter(Boolean);
    const send = iface.names.find((n: { send?: string }) => n.send === 'deckPlay');
    expect(Object.keys(send.options)).toEqual(itemFields);
  });
  test('deckEnqueue is deckPlay with a place', () => {
    const play = iface.names.find((n: { send?: string }) => n.send === 'deckPlay');
    const enq = iface.names.find((n: { send?: string }) => n.send === 'deckEnqueue');
    expect(Object.keys(enq.options)).toEqual([...Object.keys(play.options), 'at']);
  });
});

describe('D14 the poster over the signed-in seam', () => {
  test('answers, refusals and silence', async () => {
    const { routePoster, OPS_PATH } = await import('./deck-post.ts');
    const { HTTPError, SignedOutError } = await import('./api.ts');
    const seen: unknown[] = [];
    const ok = routePoster(async (path, o) => { seen.push([path, o]); return { body: { results: [{ op_id: 'x', applied: true }], sheet_epoch: 3 }, serverTime: null, raw: '' }; });
    expect(await ok.post([], 3)).toEqual({ kind: 'answeredEpoch', results: [{ op_id: 'x', applied: true }], epoch: 3 });
    expect(seen[0]).toEqual([OPS_PATH, { method: 'POST', body: { ops: [], sheet_epoch: 3 } }]);
    const kind = async (e: Error) => (await routePoster(async () => { throw e; }).post([], 0)).kind;
    expect(await kind(new HTTPError(503, 'x'))).toBe('notYet');
    expect(await kind(new HTTPError(429, 'x', 5))).toBe('notYet');
    expect(await kind(new HTTPError(400, 'x'))).toBe('refused');
    expect(await kind(new SignedOutError('x'))).toBe('offline');
    expect(await kind(new Error('network: x'))).toBe('offline');
  });
});
