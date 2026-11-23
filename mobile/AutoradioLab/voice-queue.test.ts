// The turn route's queue words (voice-queue.ts), ported from RunSheetStoreTests (Swift). Failure list: Q1 an entry's wire item (story, page, book, audio; the programme's spoken entries are `story` with their own route; over-bound addresses left out);
// Q2 the snapshot carries current, upcoming (at most 50, who added), version and the entry on air with its claim and stories; Q3 each op of a turn applies by the sheet's rules (additions always; remove, move, skip
// only for what is ahead / on air; clear and previous only at the base version); Q4 a turn applies once (turn_id) and every op has an id made from it; Q5 a garbled op stops no other and nothing throws.
import { beforeEach, describe, expect, test } from 'bun:test';
import { RadioRuntime } from './radio-runtime.ts';
import { turnQueueJSON, applyTurnOps, wireItem, resetTurnLog } from './voice-queue.ts';
import { paperStory, audio, book, webPage } from './runsheet-entries.ts';
import type { DeckPort, DeckState } from './deck.ts';
import type { Entry } from './runsheet-model.ts';

const EMPTY: DeckState = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
const deck: DeckPort = { state: async () => EMPTY, play: async () => EMPTY, enqueue: async () => EMPTY, act: async () => EMPTY, ack: async () => EMPTY };
const story = (id: string): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: 'W.', est_seconds: 5, revision: 0 });
let rt: RadioRuntime;
beforeEach(() => { rt = new RadioRuntime({ deck, now: () => new Date('2023-11-14T22:00:00Z') }); resetTurnLog(); });

describe('the wire', () => {
  test('Q1 wire items', () => {
    expect(wireItem(paperStory('/pub/x', 'X'))).toEqual({ kind: 'story', route: '/pub/x', title: 'X' });
    expect(wireItem(webPage('https://e.org/p', 'P'))).toEqual({ kind: 'page', url: 'https://e.org/p', title: 'P' });
    expect(wireItem(book('b1', 'B', 4))).toEqual({ kind: 'document', id: 'b1', title: 'B', at: 4 });
    expect(wireItem(audio({ url: 'https://a/b.mp3', title: 'E', show: 'S', start: 3, episode_id: 'ep1' }))).toEqual({ kind: 'audio', url: 'https://a/b.mp3', title: 'E', show: 'S', start: 3, episode_id: 'ep1' });
    expect(wireItem(story('s'))).toEqual({ kind: 'story', route: 'programme:s', title: 'T s' });     // the programme's own words (Q6 to Q9 hold the rest)
    expect(wireItem(paperStory('/pub/' + 'x'.repeat(300), 'Long'))).toBeUndefined();
  });
  test('Q2 the snapshot', () => {
    rt.play(paperStory('/pub/a', 'A'));
    rt.store.place(paperStory('/pub/b', 'B', 'desk'), 'end'); rt.store.place(story('spoken'), 'end');
    const q = JSON.parse(turnQueueJSON(rt, 3));
    expect(q.version).toBe(rt.store.sheet.version); expect(q.current).toMatchObject({ item: { kind: 'story', route: '/pub/a' }, position_seconds: 3 });
    expect(q.upcoming.map((u: { item: { route: string } }) => u.item.route)).toEqual(['/pub/b', 'programme:spoken']); expect(q.upcoming[0].added_by).toBe('dj');
    expect(q.on_air.title).toBe('A');
    expect(turnQueueJSON(rt).indexOf('"current"')).toBeLessThan(turnQueueJSON(rt).indexOf('"on_air"'));   // keys are sorted
  });
});

describe('the ops', () => {
  test('Q3 additions, remove, move and skip', () => {
    rt.play(paperStory('/pub/a', 'A'));
    const a = applyTurnOps(rt, JSON.stringify({ turn_id: 't1', ops: [{ op: 'enqueue', item: { kind: 'story', route: '/pub/b', title: 'B' } }, { op: 'play_next', item: { kind: 'story', route: '/pub/c', title: 'C' } }, { op: 'remove', id: 'nope' }] }));
    expect(a).toMatchObject({ applied: 2, dropped: 1 });
    expect(rt.store.ahead().map((e) => e.title)).toEqual(['C', 'B']);
    turnQueueJSON(rt);                                                      // every turn is sent with a snapshot, and a move is resolved through it
    const mv = applyTurnOps(rt, JSON.stringify({ turn_id: 't2', ops: [{ op: 'move', id: rt.store.ahead()[1].id, to_index: 0 }] }));
    expect(mv.applied).toBe(1); expect(rt.store.ahead().map((e) => e.title)).toEqual(['B', 'C']);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 't3', ops: [{ op: 'skip' }] })).applied).toBe(1);
    expect(rt.store.sheet.entries.find((e) => e.title === 'A')?.ended?.outcome).toBe('skipped');
  });
  test('Q3 clear and previous only at the base version', () => {
    rt.play(paperStory('/pub/a', 'A')); rt.store.place(paperStory('/pub/b', 'B'), 'end');
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'c1', base_version: rt.store.sheet.version + 9, ops: [{ op: 'clear' }] })).applied).toBe(0);
    expect(rt.store.ahead().length).toBe(1);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'c2', base_version: rt.store.sheet.version, ops: [{ op: 'clear' }] })).applied).toBe(1);
    expect(rt.store.ahead().length).toBe(0);
  });
  test('Q4 a turn applies once', () => {
    const t = JSON.stringify({ turn_id: 'same', ops: [{ op: 'enqueue', item: { kind: 'story', route: '/pub/b', title: 'B' } }] });
    expect(applyTurnOps(rt, t).applied).toBe(1); expect(applyTurnOps(rt, t)).toMatchObject({ applied: 0, dropped: 1 });
    expect(rt.store.ahead().length).toBe(1);
  });
  test('Q5 garbled ops and bodies', () => {
    expect(applyTurnOps(rt, 'not json')).toMatchObject({ applied: 0, dropped: 0 });
    expect(applyTurnOps(rt, '5')).toMatchObject({ applied: 0, dropped: 0 });
    const r = applyTurnOps(rt, JSON.stringify({ turn_id: 'g', ops: [7, { op: 5 }, { op: 'enqueue', item: { kind: 'nonsense' } }, { op: 'enqueue', item: { kind: 'story', route: '/pub/ok', title: 'OK' } }] }));
    expect(r).toMatchObject({ applied: 1, dropped: 3 });
  });
});

// ---- The queue the model is told about (Q6 to Q12). The owner's radio was made of the programme desk's own entries (kind story, opening, transition), and every one of them was left
// off the wire: the server wrote "Playing: nothing / Upcoming: nothing" and the model said so while a story was on air with 7 in the deck.
// Failure list, written first:
//  Q6  a radio of programme stories sends its current and upcoming entries, titled, in order (the server's `story` kind needs a route: the programme entry's own).
//  Q7  a mixed queue (paper story, programme story, web page, book, audio, mail) keeps the order the listener sees.
//  Q8  nothing is sent for what the listener cannot see: the station's transitions between entries, entries already heard or skipped, entries past the server's 50.
//  Q9  every kind the run sheet can hold, on air or ahead, either has a wire item the server accepts or is a transition ahead (hidden).
//  Q10 an op the server returns for a programme entry (remove, move, skip) is applied by the sheet; one that cannot be applied is refused loudly (counted, reasoned, logged), never dropped quietly.
//  Q11 the wire shape is the server's request type: the keys, the kinds, what each kind needs, the bounds and the id grammar are read from internal/autoradioqueue/queue.go (the fixture is built
//      from its json tags and a test fails if the Go source moves away from the fixture).
import { readFileSync } from 'node:fs';
import { KINDS, validId } from './runsheet-model.ts';
import wirefix from './fixtures/autoradio-queue-wire.json';

const spoken = (id: string, kind: Entry['kind'] = 'story', title = `T ${id}`, extra: Partial<Entry> = {}): Entry => ({ id, kind, placed_by: 'desk', state: 'ahead', title: kind === 'transition' ? '' : title, text: 'Words to speak.', est_seconds: 30, revision: 0, ...extra });
const wireQueue = (r: RadioRuntime = rt) => JSON.parse(turnQueueJSON(r, 0)) as { version: number; current: { id: string; item: Record<string, unknown>; position_seconds: number } | null; upcoming: { id: string; item: Record<string, unknown>; added_by: string }[] };

describe('the queue the model is told about', () => {
  test('Q6 a radio of programme stories sends what is on air and what is up next', () => {
    rt.play(spoken('d-air', 'story', 'Harbor strike ends'));
    for (const n of ['a', 'b', 'c']) rt.store.place(spoken('d-' + n, 'story', 'Story ' + n), 'end');
    const q = wireQueue();
    expect(q.current).toMatchObject({ id: 'd-air', item: { kind: 'story', title: 'Harbor strike ends' } });
    expect(typeof q.current!.item.route).toBe('string'); expect(q.current!.item.route).not.toBe('');
    expect(q.upcoming.map((u) => u.item.title)).toEqual(['Story a', 'Story b', 'Story c']);
    expect(q.upcoming.map((u) => u.id)).toEqual(['d-a', 'd-b', 'd-c']);
    expect(q.upcoming.every((u) => u.item.kind === 'story' && u.added_by === 'dj')).toBe(true);
    expect(new Set(q.upcoming.map((u) => u.item.route)).size).toBe(3);                    // each entry has its own route: the desk never sees two as one
  });
  test('Q6 a programme story that tells a paper story carries that story\'s route', () => {
    const told = spoken('d-t', 'story', 'Told', { source: { ref: 'pub:/pub/harbor@v3', label: 'Harbor' } });
    expect(wireItem(told)).toEqual({ kind: 'story', route: '/pub/harbor', title: 'Told' });
  });
  test('Q7 a mixed queue keeps the order shown', () => {
    rt.play(spoken('d-air', 'opening', 'Opening'));
    rt.store.place(paperStory('/pub/p', 'Paper story', 'desk'), 'end');
    rt.store.place(spoken('d-s', 'story', 'Spoken story'), 'end');
    rt.store.place(webPage('https://e.org/w', 'Web page'), 'end');
    rt.store.place(book('b1', 'A book', 0), 'end');
    rt.store.place(audio({ url: 'https://a/b.mp3', title: 'A podcast', show: 'Show' }), 'end');
    rt.store.place(spoken('d-m', 'story', 'Mail', { kind: 'mail', source: { ref: 'mail:m1', label: 'Alice' } }), 'end');
    rt.store.place(spoken('d-k', 'take', 'A take', { source: { ref: 'take:t1', label: 'Pub' } }), 'end');
    const q = wireQueue();
    expect(q.upcoming.map((u) => [u.item.kind, u.item.title])).toEqual([['story', 'Paper story'], ['story', 'Spoken story'], ['page', 'Web page'], ['document', 'A book'], ['audio', 'A podcast'], ['document', 'Mail'], ['story', 'A take']]);
    expect(q.current).toMatchObject({ id: 'd-air', item: { kind: 'story', title: 'Opening' } });
  });
  test('Q8 nothing is sent for what the listener cannot see', () => {
    rt.play(spoken('d-air', 'story', 'Now'));
    rt.store.place(spoken('d-1', 'story', 'One'), 'end');
    rt.store.place(spoken('d-x', 'transition', ''), 'end');
    rt.store.place(spoken('d-2', 'story', 'Two'), 'end');
    expect(wireQueue().upcoming.map((u) => u.item.title)).toEqual(['One', 'Two']);          // the transition is not on the page, so the desk is not told of it
    rt.play(spoken('d-next', 'story', 'Playing after'));                                    // 'Now' ended; it is not on air and not ahead
    const q = wireQueue();
    expect(q.current?.id).toBe('d-next'); expect(JSON.stringify(q)).not.toContain('"Now"');
    const empty = new RadioRuntime({ deck, now: () => new Date('2023-11-14T22:00:00Z') });
    expect(wireQueue(empty)).toMatchObject({ current: null, upcoming: [] });
  });
  test('Q8 at most 50 are ahead on the wire, in order', () => {
    rt.play(spoken('d-air', 'story'));
    for (let i = 0; i < 60; i++) rt.store.place(spoken('d-' + String(i).padStart(2, '0'), 'story', 'S' + i), 'end');
    const u = wireQueue().upcoming;
    expect(u.length).toBe(50); expect(u[0].item.title).toBe('S0'); expect(u[49].item.title).toBe('S49');
  });
  test('Q9 every kind the run sheet can hold has a wire item the server accepts', () => {
    const make = (k: Entry['kind']): Entry => {
      const e = spoken('d-' + k, k, 'Title ' + k);
      if (k === 'clip') e.source = { ref: 'clip:c1#ep1', label: 'Show', url: 'https://a/c.mp3', start: 1, end: 9, clip_id: 'c1' };
      if (k === 'episode') e.source = { ref: 'episode:ep1', label: 'Show', url: 'https://a/e.mp3' };
      if (k === 'page') e.source = { ref: 'url:https://e.org/p', label: 'e.org' };
      if (k === 'document') e.source = { ref: 'doc:book1', label: 'Book', start: 2 };
      if (k === 'mail') e.source = { ref: 'mail:m1', label: 'Alice' };
      if (k === 'take') e.source = { ref: 'take:t1', label: 'Pub' };
      return e;
    };
    for (const k of KINDS) {
      const item = wireItem(make(k));
      expect(item, `kind ${k}`).toBeDefined();
      expect(wirefix.kinds[item!.kind as keyof typeof wirefix.kinds], `kind ${k} -> ${String(item!.kind)}`).toBeDefined();
    }
    for (const k of KINDS) {                                                              // on air, any kind is described, a transition too (the listener hears it)
      const r = new RadioRuntime({ deck, now: () => new Date('2023-11-14T22:00:00Z') });
      r.play(make(k)); expect(wireQueue(r).current, `on air ${k}`).not.toBeNull();
    }
  });
});

describe('what the server answers for programme entries', () => {
  const programme = (r: RadioRuntime = rt): void => {
    r.play(spoken('d-air', 'story', 'On air'));
    for (const n of ['1', '2', '3']) r.store.place(spoken('d-' + n, 'story', 'Story ' + n), 'end');
  };
  test('Q10 remove, move and skip reach programme entries', () => {
    programme(); turnQueueJSON(rt);
    const mv = applyTurnOps(rt, JSON.stringify({ turn_id: 'm1', ops: [{ op: 'move', id: 'd-3', to_index: 0, reason: 'asked' }] }));
    expect(mv).toMatchObject({ applied: 1, dropped: 0 }); expect(rt.store.ahead().map((e) => e.id)).toEqual(['d-3', 'd-1', 'd-2']);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'm2', ops: [{ op: 'remove', id: 'd-1' }] }))).toMatchObject({ applied: 1, dropped: 0 });
    expect(rt.store.ahead().map((e) => e.id)).toEqual(['d-3', 'd-2']);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'm3', ops: [{ op: 'skip', id: 'd-air' }] }))).toMatchObject({ applied: 1, dropped: 0 });
    expect(rt.store.sheet.entries.find((e) => e.id === 'd-air')?.ended?.outcome).toBe('skipped');
  });
  test('Q10 a move counts places the way the listener sees them: the station\'s transitions are not places', () => {
    rt.play(spoken('d-air', 'story'));
    for (const [id, k] of [['d-a', 'story'], ['d-t', 'transition'], ['d-b', 'story'], ['d-c', 'story']] as const) rt.store.place(spoken(id, k), 'end');
    // the server saw [a, b, c]; "put a second" is to_index 1: a goes after b, and the transition the listener never sees must not count as a place
    const seen = (): string[] => rt.store.ahead().filter((e) => e.kind !== 'transition').map((e) => e.id);
    turnQueueJSON(rt);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'm4', ops: [{ op: 'move', id: 'd-a', to_index: 1 }] })).applied).toBe(1);
    expect(seen()).toEqual(['d-b', 'd-a', 'd-c']);
    turnQueueJSON(rt);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'm5', ops: [{ op: 'move', id: 'd-c', to_index: 0 }] })).applied).toBe(1);
    expect(seen()).toEqual(['d-c', 'd-b', 'd-a']);
  });
  test('Q10 an op that cannot apply is refused loudly: counted, with its reason, and logged', () => {
    programme();
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    let r: ReturnType<typeof applyTurnOps>;
    try {
      r = applyTurnOps(rt, JSON.stringify({ turn_id: 'x1', ops: [{ op: 'remove', id: 'd-gone' }, { op: 'skip', id: 'd-2' }, { op: 'move', to_index: 1 }, { op: 'remove', id: 'd-2' }] }));
    } finally { console.log = log; }
    expect(r).toMatchObject({ applied: 1, dropped: 3 });
    expect(r.refused.length).toBe(3);
    expect(r.refused[0]).toContain('remove'); expect(r.refused[0]).toContain('d-gone');
    expect(r.refused[1]).toContain('skip'); expect(r.refused[1]).toContain('d-2');
    expect(r.refused[2]).toContain('move');
    expect(logs.filter((l) => l.includes('turn ops refused')).length).toBe(3);
    expect(rt.store.ahead().map((e) => e.id)).toEqual(['d-1', 'd-3']);
  });
  test('Q10 a replayed turn is refused loudly too', () => {
    programme();
    const t = JSON.stringify({ turn_id: 'again', ops: [{ op: 'remove', id: 'd-1' }] });
    expect(applyTurnOps(rt, t).applied).toBe(1);
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    let r: ReturnType<typeof applyTurnOps>;
    try { r = applyTurnOps(rt, t); } finally { console.log = log; }
    expect(r).toMatchObject({ applied: 0, dropped: 1 }); expect(r.refused[0]).toContain('replayed'); expect(logs.some((l) => l.includes('replayed'))).toBe(true);
  });
});

describe('the wire is the server\'s request type', () => {
  const go = readFileSync(new URL('../../internal/autoradioqueue/queue.go', import.meta.url), 'utf8');
  const tags = (name: string): string[] => {
    const m = go.match(new RegExp(`type ${name} struct \\{([\\s\\S]*?)\\n\\}`));
    if (!m) throw new Error(`no struct ${name} in queue.go`);
    return [...m[1].matchAll(/`json:"([^",]+)/g)].map((x) => x[1]);
  };
  test('Q11 the fixture is the Go struct\'s json tags, kinds, bounds and id grammar', () => {
    expect(wirefix.snapshot).toEqual(tags('Snapshot')); expect(wirefix.current).toEqual(tags('Current'));
    expect(wirefix.entry).toEqual(tags('Entry')); expect(wirefix.item).toEqual(tags('Item'));
    const validate = go.match(/func validateItem[\s\S]*?\n\}\n/)![0];
    const need = Object.fromEntries([...validate.matchAll(/case "(\w+)":\s*\n\s*return need\("(\w+)"/g)].map((m) => [m[1], m[2]]));
    expect(wirefix.kinds).toEqual(need);
    const bound = (n: string): number => Number(go.match(new RegExp(`\\b${n}\\s*=\\s*(\\d+)`))![1]);
    expect(wirefix.bytes).toEqual({ title: bound('maxTitleBytes'), route: bound('maxRouteBytes'), url: bound('maxURLBytes'), id: bound('maxIDBytes'), show: bound('maxShowBytes'), episode_id: bound('maxIDBytes') });
    expect(wirefix.maxUpcoming).toBe(bound('MaxUpcoming')); expect(wirefix.maxAddedBy).toBe(bound('maxAddedBy'));
    expect(wirefix.entryId).toBe(go.match(/entryID = regexp\.MustCompile\(`([^`]+)`\)/)![1]);
  });
  test('Q11 the run sheet\'s id grammar is the server\'s: no entry on the sheet carries an id that would refuse the turn', () => {
    const re = new RegExp(wirefix.entryId);
    for (const id of ['d-ok', 'p-0123456789abcdef01234567', 'a', 'A.b_c-d', '', '-x', '.x', '_x', 'has space', 'a:b', 'x'.repeat(64), 'x'.repeat(65), 'é', 'a\n']) expect(validId(id), JSON.stringify(id)).toBe(re.test(id));
  });
  test('Q11 a queue of every kind is a snapshot the server\'s own rules accept', () => {
    rt.play(spoken('d-air', 'story', 'On air'));
    const long = 'x'.repeat(400);
    const entries: Entry[] = [
      paperStory('/pub/a', 'Paper', 'desk'), spoken('d-s', 'story', long), spoken('d-o', 'opening'), webPage('https://e.org/w', 'Web'), book('b1', 'Book', 3),
      audio({ url: 'https://a/b.mp3', title: 'Pod', show: 'S'.repeat(300), start: 3, end: 9, episode_id: 'ep1' }),
      spoken('d-m', 'mail', 'Mail', { source: { ref: 'mail:m1', label: 'A' } }), spoken('d-r', 'result'), spoken('d-an', 'answer'),
    ];
    for (const e of entries) rt.store.place(e, 'end');
    const q = wireQueue();
    const re = new RegExp(wirefix.entryId); const bytes = (s: unknown): number => new TextEncoder().encode(String(s)).length;
    expect(Object.keys(q).filter((k) => !wirefix.snapshot.includes(k) && !wirefix.ignored.includes(k))).toEqual([]);
    expect(q.upcoming.length).toBe(entries.length);
    const ids = new Set<string>(); let n = 0;
    for (const e of [q.current!, ...q.upcoming]) {
      n++;
      expect(re.test(e.id), e.id).toBe(true); expect(ids.has(e.id)).toBe(false); ids.add(e.id);
      expect(Object.keys(e).filter((k) => !(('added_by' in e ? wirefix.entry : wirefix.current) as string[]).includes(k))).toEqual([]);
      const it = e.item, kind = String(it.kind);
      expect(Object.keys(it).filter((k) => !wirefix.item.includes(k))).toEqual([]);
      const needed = wirefix.kinds[kind as keyof typeof wirefix.kinds]; expect(needed, `kind ${kind}`).toBeDefined();
      expect(String(it[needed] ?? '').trim(), `${kind} needs ${needed}`).not.toBe('');
      for (const [f, max] of Object.entries(wirefix.bytes)) if (f in it) expect(bytes(it[f])).toBeLessThanOrEqual(max);
    }
    expect(n).toBe(entries.length + 1);
    for (const u of q.upcoming) expect(bytes(u.added_by)).toBeLessThanOrEqual(wirefix.maxAddedBy);
    expect(q.current!.position_seconds).toBeGreaterThanOrEqual(0);
  });
});

// Q12 (the review's S4): a move is resolved by the entry ids of the list that was SENT with the turn, not by the places the sheet shows when the reply arrives. The failure list, written first:
//  Q12a an entry with no wire item (an over-long page address) is not in the list the server counted: "put A after C" (to_index 1 of [A, C, D]) puts A after C, not after B.
//  Q12b the on-air entry ended between the snapshot and the reply: the place the desk named was after it; it is gone from ahead, so the move goes to the front, not one place further.
//  Q12c the sheet changed after the snapshot (an entry added at the front): the sent list still decides where after.
//  Q12d a version the phone never sent, or an entry the list did not hold, is refused loudly with its reason, and nothing moves.
describe('Q12 a move is resolved through the list that was sent', () => {
  const spokenEntry = (id: string): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: 'W.', est_seconds: 5, revision: 0 });
  const long = (id: string): Entry => ({ ...webPage('https://e.org/' + 'p'.repeat(1100), 'Long'), id });
  const ids = (): string[] => rt.store.ahead().map((e) => e.id);
  test('Q12a an entry with no wire item is not a place the server counted', () => {
    rt.play(spokenEntry('d-air'));
    rt.store.place(spokenEntry('d-a'), 'end'); rt.store.place(long('d-b'), 'end'); rt.store.place(spokenEntry('d-c'), 'end'); rt.store.place(spokenEntry('d-d'), 'end');
    const sent = JSON.parse(turnQueueJSON(rt, 0));
    expect(sent.upcoming.map((u: { id: string }) => u.id)).toEqual(['d-a', 'd-c', 'd-d']);
    const r = applyTurnOps(rt, JSON.stringify({ turn_id: 'q12a', base_version: sent.version, ops: [{ op: 'move', id: 'd-a', to_index: 1 }] }));
    expect(r.applied).toBe(1); expect(ids()).toEqual(['d-b', 'd-c', 'd-a', 'd-d']);
  });
  test('Q12b the entry on air ended before the reply: the move still lands where the desk meant', () => {
    rt.play(spokenEntry('d-air'));
    for (const n of ['n', 'a', 'b']) rt.store.place(spokenEntry('d-' + n), 'end');
    const sent = JSON.parse(turnQueueJSON(rt, 0));                          // the desk saw [n, a, b]; "put b right after n" is to_index 1
    rt.store.applyFromDesk({ op_id: 'q12b-gone', by: 'desk', op: 'remove', id: 'd-n', reason: 'n aired: it is no longer ahead', base_version: 0 });   // n left the places ahead (it went on air): ahead is [a, b]
    expect(ids()).toEqual(['d-a', 'd-b']);
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'q12b', base_version: sent.version, ops: [{ op: 'move', id: 'd-b', to_index: 1 }] })).applied).toBe(1);
    expect(ids()).toEqual(['d-b', 'd-a']);
  });
  test('Q12c an entry added after the snapshot does not shift the places', () => {
    rt.play(spokenEntry('d-air'));
    for (const n of ['a', 'b', 'c']) rt.store.place(spokenEntry('d-' + n), 'end');
    const sent = JSON.parse(turnQueueJSON(rt, 0));
    rt.store.place(spokenEntry('d-new'), 'next');
    expect(applyTurnOps(rt, JSON.stringify({ turn_id: 'q12c', base_version: sent.version, ops: [{ op: 'move', id: 'd-a', to_index: 2 }] })).applied).toBe(1);
    expect(ids()).toEqual(['d-new', 'd-b', 'd-c', 'd-a']);
  });
  test('Q12d a list the phone never sent, or an entry it did not hold, is refused loudly', () => {
    rt.play(spokenEntry('d-air'));
    for (const n of ['a', 'b']) rt.store.place(spokenEntry('d-' + n), 'end'); rt.store.place(long('d-x'), 'end');
    const sent = JSON.parse(turnQueueJSON(rt, 0));
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    let unknown: ReturnType<typeof applyTurnOps>, held: ReturnType<typeof applyTurnOps>;
    try {
      unknown = applyTurnOps(rt, JSON.stringify({ turn_id: 'q12d1', base_version: sent.version + 77, ops: [{ op: 'move', id: 'd-a', to_index: 1 }] }));
      held = applyTurnOps(rt, JSON.stringify({ turn_id: 'q12d2', base_version: sent.version, ops: [{ op: 'move', id: 'd-x', to_index: 0 }] }));
    } finally { console.log = log; }
    expect(unknown.applied).toBe(0); expect(unknown.refused[0]).toContain('list');
    expect(held.applied).toBe(0); expect(held.refused[0]).toContain('d-x');
    expect(logs.filter((l) => l.includes('turn ops refused')).length).toBe(2);
    expect(ids()).toEqual(['d-a', 'd-b', 'd-x']);
  });
});

describe('Q13 a refusal line is one short plain line (review N6)', () => {
  test('control characters are stripped and a long id is cut, in the result and in the log', () => {
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    let r: ReturnType<typeof applyTurnOps>;
    try { r = applyTurnOps(rt, JSON.stringify({ turn_id: 'q13', ops: [{ op: 'remove', id: 'a\nFORGED: line\u001b[31m' + 'x'.repeat(500) }] })); } finally { console.log = log; }
    expect(r.refused[0]).not.toMatch(/[\u0000-\u001f]/); expect(r.refused[0].length).toBeLessThanOrEqual(300);
    expect(logs.filter((l) => /[\n\u001b]/.test(l)).length).toBe(0);
  });
});
