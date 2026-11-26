// The real deck port (deck-native.ts): all six names are generated @choir/autoradio package functions, never an Apple app-module op.
//  N1 each method makes one package call with the declared scalar options and reads its DeckState answer.
//  N2 missing and mistyped fields normalize to the empty state; malformed journal lines are dropped in order.
//  N3 an absent package is empty; a failed send is empty; a failed state read throws so shared code keeps its last known state.
//  N4 deckWipe is explicit and is not hidden inside stop.
import { afterEach, describe, expect, test } from 'bun:test';
import { nativeDeck } from './deck-native.ts';
import { item } from './deck.ts';

const answer = { why: 'playing', item: 'a', block: 2, seconds: 5.5, duration: 0, held: ['b', 'c'], seq: 7, acked: 3, journal: [
  { seq: 4, type: 'started', item: 'a', at: '2023-11-14T22:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '' },
  { seq: 5, type: 'ended', item: 'a', at: '2023-11-14T22:00:09Z', block: 2, seconds: 8, final: 3, rendered: true, end: 'finished', command: '', reason: '' }, 'junk', { type: 'ended' }] };
const unavailable = (): never => { throw Object.assign(new Error('package absent'), { name: 'NotSupportedError' }); };
const globals = globalThis as typeof globalThis & { __autoradioDeckPackage?: (name: string, args: Record<string, unknown>) => Promise<unknown> };
const originalHook = globals.__autoradioDeckPackage;
afterEach(() => { if (originalHook) globals.__autoradioDeckPackage = originalHook; else delete globals.__autoradioDeckPackage; });

describe('the generated package deck port', () => {
  test('N1 one package call per method, named for the deck name', async () => {
    const calls: { name: string; args: Record<string, unknown> }[] = [];
    const d = nativeDeck(async (name, args) => { calls.push({ name, args }); return answer; });
    await d.state(); await d.play(item({ id: 'x', kind: 'speech', paragraphs: 'Words.', segment: 'story' })); await d.enqueue(item({ id: 'y', kind: 'audio', url: 'https://a/b.mp3' }), 1);
    await d.act('seekBy', 15); await d.ack(5); await d.wipe();
    expect(calls.map((c) => c.name)).toEqual(['deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe']);
    expect(calls[0].args).toEqual({ tick: 0 });
    expect(calls[1].args).toEqual({ item: item({ id: 'x', kind: 'speech', paragraphs: 'Words.', segment: 'story' }) });
    expect(calls[2].args).toMatchObject({ item: { id: 'y', kind: 'audio', url: 'https://a/b.mp3' }, at: 1 });
    expect(calls[3].args).toEqual({ op: 'seekBy', value: 15 }); expect(calls[4].args).toEqual({ through: 5 }); expect(calls[5].args).toEqual({});
  });

  test('N2 the answer is a DeckState', async () => {
    const s = await nativeDeck(async () => answer).state();
    expect(s).toMatchObject({ why: 'playing', item: 'a', block: 2, seconds: 5.5, held: ['b', 'c'], seq: 7, acked: 3 });
    expect(s.journal.map((l) => l.seq)).toEqual([4, 5]); expect(s.journal[1]).toMatchObject({ end: 'finished', rendered: true, block: 2, final: 3 });
    const bad = await nativeDeck(async () => ({ why: 4, held: 'x', journal: 'no', seq: 'a' })).state();
    expect(bad).toEqual({ why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] });
  });

  test('N3 no package, a thrown send and no answer are the empty state', async () => {
    const empty = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
    expect(await nativeDeck(unavailable).state()).toEqual(empty);
    expect(await nativeDeck(async () => { throw new Error('x'); }).play(item({ id: 'a', kind: 'speech' }))).toEqual(empty);
    expect(await nativeDeck(async () => undefined).act('stop')).toEqual(empty);
    expect(await nativeDeck(async () => { throw new Error('wipe failed'); }).wipe()).toEqual(empty);
  });

  test('N-fail a real state-read failure throws; the other five names answer empty', async () => {
    const d = nativeDeck(async () => { throw new Error('gone'); });
    await expect(d.state()).rejects.toThrow('gone');
    expect((await d.ack(1)).why).toBe('empty');
  });

  test('N5 the Bun package stub exercises the generated-name import, with no app-module request', async () => {
    const calls: { name: string; args: Record<string, unknown> }[] = [];
    globals.__autoradioDeckPackage = async (name, args) => { calls.push({ name, args }); return answer; };
    const d = nativeDeck();
    await d.state(); await d.play(item({ id: 'stub', kind: 'speech' })); await d.wipe();
    expect(calls.map((c) => c.name)).toEqual(['deckState', 'deckPlay', 'deckWipe']);
    expect(calls.every((c) => !('op' in c.args))).toBe(true);
  });
});
