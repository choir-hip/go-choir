// The real deck port (deck-native.ts): the deck's five names over the app module's one `deck` op. A fake `later` stands in for the module.
//  N1 each method is one `deck` call named for the deck's name, carrying exactly the item's fields (and `at`, `op`/`value`, `through`).
//  N2 the answer is read as a DeckState: missing and mistyped fields are the empty state; the journal keeps only well-formed lines, in order.
//  N3 a module that does not answer, throws or is absent is the empty state, never a throw: shared code goes on and the next pump asks again.
import { describe, expect, test } from 'bun:test';
import * as api from './api.ts';
import { nativeDeck } from './deck-native.ts';
import { item } from './deck.ts';

const calls: Record<string, unknown>[] = [];
const answer = { why: 'playing', item: 'a', block: 2, seconds: 5.5, duration: 0, held: ['b', 'c'], seq: 7, acked: 3, journal: [
  { seq: 4, type: 'started', item: 'a', at: '2023-11-14T22:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '' },
  { seq: 5, type: 'ended', item: 'a', at: '2023-11-14T22:00:09Z', block: 2, seconds: 8, final: 3, rendered: true, end: 'finished', command: '', reason: '' }, 'junk', { type: 'ended' }] };
const later = async (r: Record<string, unknown>) => { calls.push(r); return answer; };

describe('the native deck port', () => {
  test('N1 one deck call per method, named for the deck name', async () => {
    calls.length = 0;
    const d = nativeDeck(() => later);
    await d.state(); await d.play(item({ id: 'x', kind: 'speech', paragraphs: 'Words.', segment: 'story' })); await d.enqueue(item({ id: 'y', kind: 'audio', url: 'https://a/b.mp3' }), 1);
    await d.act('seekBy', 15); await d.ack(5);
    expect(calls.map((c) => [c.op, c.name])).toEqual([['deck', 'deckState'], ['deck', 'deckPlay'], ['deck', 'deckEnqueue'], ['deck', 'deckAct'], ['deck', 'deckAck']]);
    expect(calls[1].args).toMatchObject({ id: 'x', kind: 'speech', paragraphs: 'Words.', segment: 'story', requiresPreviousHeard: false });
    expect(calls[2].args).toMatchObject({ id: 'y', kind: 'audio', url: 'https://a/b.mp3', at: 1 });
    expect(calls[3].args).toEqual({ op: 'seekBy', value: 15 }); expect(calls[4].args).toEqual({ through: 5 });
  });
  test('N2 the answer is a DeckState', async () => {
    const s = await nativeDeck(() => later).state();
    expect(s).toMatchObject({ why: 'playing', item: 'a', block: 2, seconds: 5.5, held: ['b', 'c'], seq: 7, acked: 3 });
    expect(s.journal.map((l) => l.seq)).toEqual([4, 5]); expect(s.journal[1]).toMatchObject({ end: 'finished', rendered: true, block: 2, final: 3 });
    const bad = await nativeDeck(() => async () => ({ why: 4, held: 'x', journal: 'no', seq: 'a' })).state();
    expect(bad).toEqual({ why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] });
  });
  test('N3 no module, a thrown call and no answer are the empty state', async () => {
    const empty = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
    expect(await nativeDeck(() => null).state()).toEqual(empty);
    expect(await nativeDeck(() => async () => { throw new Error('x'); }).play(item({ id: 'a', kind: 'speech' }))).toEqual(empty);
    expect(await nativeDeck(() => async () => undefined).act('stop')).toEqual(empty);
  });
  test('N4 the DEFAULT port is the inert call: after its world closed, a play from a radio source still in flight is never delivered', async () => {
    calls.length = 0; api.resetSeam(); api.bindNative({ available: true, later } as never); const w = api.openWorld('anonymous');
    const d = nativeDeck(); await d.state(); expect(calls.length).toBe(1);                       // a live world's call goes through
    api.endWorld(); calls.length = 0;
    void d.play(item({ id: 'x', kind: 'speech', paragraphs: 'Words.', segment: 'story' })); void d.act('seekBy', 15); void d.enqueue(item({ id: 'y', kind: 'audio', url: 'https://a/b.mp3' }), 1);      // a closed world's call stays pending forever (api.ts inert)
    await new Promise((r) => setTimeout(r, 30)); expect(calls).toEqual([]); void w; api.resetSeam();
  });
});
