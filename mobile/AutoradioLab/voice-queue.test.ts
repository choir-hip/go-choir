// The turn route's queue words (voice-queue.ts), ported from RunSheetStoreTests (Swift). Failure list: Q1 an entry's wire item (story, page, book, audio; spoken entries have none; over-bound addresses left out);
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
    expect(wireItem(story('s'))).toBeUndefined();
    expect(wireItem(paperStory('/pub/' + 'x'.repeat(300), 'Long'))).toBeUndefined();
  });
  test('Q2 the snapshot', () => {
    rt.play(paperStory('/pub/a', 'A'));
    rt.store.place(paperStory('/pub/b', 'B', 'desk'), 'end'); rt.store.place(story('spoken'), 'end');
    const q = JSON.parse(turnQueueJSON(rt, 3));
    expect(q.version).toBe(rt.store.sheet.version); expect(q.current).toMatchObject({ item: { kind: 'story', route: '/pub/a' }, position_seconds: 3 });
    expect(q.upcoming.map((u: { item: { route: string } }) => u.item.route)).toEqual(['/pub/b']); expect(q.upcoming[0].added_by).toBe('dj');
    expect(q.on_air.title).toBe('A');
    expect(turnQueueJSON(rt).indexOf('"current"')).toBeLessThan(turnQueueJSON(rt).indexOf('"on_air"'));   // keys are sorted
  });
});

describe('the ops', () => {
  test('Q3 additions, remove, move and skip', () => {
    rt.play(paperStory('/pub/a', 'A'));
    const a = applyTurnOps(rt, JSON.stringify({ turn_id: 't1', ops: [{ op: 'enqueue', item: { kind: 'story', route: '/pub/b', title: 'B' } }, { op: 'play_next', item: { kind: 'story', route: '/pub/c', title: 'C' } }, { op: 'remove', id: 'nope' }] }));
    expect(a).toEqual({ applied: 2, dropped: 1 });
    expect(rt.store.ahead().map((e) => e.title)).toEqual(['C', 'B']);
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
    expect(applyTurnOps(rt, t).applied).toBe(1); expect(applyTurnOps(rt, t)).toEqual({ applied: 0, dropped: 1 });
    expect(rt.store.ahead().length).toBe(1);
  });
  test('Q5 garbled ops and bodies', () => {
    expect(applyTurnOps(rt, 'not json')).toEqual({ applied: 0, dropped: 0 });
    expect(applyTurnOps(rt, '5')).toEqual({ applied: 0, dropped: 0 });
    const r = applyTurnOps(rt, JSON.stringify({ turn_id: 'g', ops: [7, { op: 5 }, { op: 'enqueue', item: { kind: 'nonsense' } }, { op: 'enqueue', item: { kind: 'story', route: '/pub/ok', title: 'OK' } }] }));
    expect(r).toEqual({ applied: 1, dropped: 3 });
  });
});
