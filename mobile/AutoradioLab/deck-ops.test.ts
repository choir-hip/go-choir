// The deck's held items kept in the sheet's order (deck-ops.ts), under bun test with a fake deck: the part of the first design (journal to posted ops, a poster over the route)
// that shared code uses. The rest of that design was never wired (the run sheet store posts its own ops: runsheet-store.ts) and is deleted.
//  D6  heldPlan keeps what already matches, drops from the first mismatch, and never holds more than the deck's limit.
//  D13 the names in deck.ts, deck-types.contract and interface/deck.json are one list.   D16 the interface carries no `level` op and no `rate` field.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { heldPlan, syncHeld } from './deck-ops.ts';
import { DECK_HELD_MAX, DECK_NAMES, item, type DeckItem, type DeckPort, type DeckState } from './deck.ts';

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
    const deck = (j.names as { source?: string; send?: string }[]).map((n) => n.source ?? n.send ?? '').filter((n) => n.startsWith('deck'));
    expect(deck).toEqual(['deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe']);
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
    wipe: async () => { calls.push('wipe'); s = state(); return s; },
  };
}

describe('D13 one list of names', () => {
  const iface = JSON.parse(readFileSync(join(import.meta.dir, '..', 'exact-package', 'interface', 'deck.json'), 'utf8'));
  const contract = readFileSync(join(import.meta.dir, 'deck-types.contract'), 'utf8');
  test('names', () => {
    const names = iface.names.map((n: { source?: string; send?: string }) => n.source ?? n.send).sort();
    expect(names).toEqual(['deckAck', 'deckAct', 'deckEnqueue', 'deckPlay', 'deckState', 'deckWipe']);
    expect(DECK_NAMES).toEqual(['deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe']);
    expect(iface.names[0].options).toEqual({ tick: 'number' });                   // read once per radio tick by the contract (spine.contract `ds`)
  });
  test('shape fields match in the interface and the contract', () => {
    for (const shape of ['DeckItem', 'DeckEvent', 'DeckState']) {
      const fields = Object.keys(iface.shapes[shape]);
      const block = contract.split(`shape ${shape}\n`)[1].split(/\n\n|\nshape /)[0];
      const have = block.split('\n').map((l) => l.trim().split(':')[0]).filter(Boolean);
      expect(have).toEqual(fields);
    }
    const itemFields = contract.split('shape DeckItem\n')[1].split(/\n\n|\n\/\/|\nshape /)[0].split('\n').map((l) => l.trim().split(':')[0]).filter(Boolean);
    // The item crosses as the `deck` op's args: the Swift that reads them (AutoradioModule+Deck.swift `deckItem`) reads every field of the shape, and no other.
    const swift = readFileSync(join(import.meta.dir, '..', 'exact-package', 'apple', 'AutoradioModule+Deck.swift'), 'utf8');
    const reader = swift.slice(swift.indexOf('private func deckItem('), swift.indexOf('func playableAudioAddress('));
    const read = [...reader.matchAll(/a\["(\w+)"\]|num\(a, "(\w+)"\)|DeckItem\(id: a\["(\w+)"\]/g)].map((m) => m[1] ?? m[2] ?? m[3]);
    expect([...new Set(read)].sort()).toEqual([...itemFields].sort());
    expect(swift).toContain('args["at"]');                                          // deckEnqueue is deckPlay with a place
  });
});


describe('D7 an entry the desk rewrote while the deck held it is handed over again (review B1-6)', () => {
  const it = (id: string): DeckItem => item({ id, kind: 'speech', paragraphs: id });
  test('heldPlan with what was sent: the same id with other words is not kept', () => {
    const a = it('a'), b = { ...it('b'), paragraphs: 'new words' }, c = it('c');
    const sent = new Map([['a', JSON.stringify(a)], ['b', JSON.stringify(it('b'))], ['c', JSON.stringify(c)]]);
    expect(heldPlan(['a', 'b', 'c'], [a, b, c], undefined, sent)).toEqual({ keep: 1, enqueue: [{ at: 1, item: b }, { at: 2, item: c }] });
    expect(heldPlan(['a', 'b', 'c'], [a, it('b'), c], undefined, sent)).toEqual({ keep: 3, enqueue: [] });
    expect(heldPlan(['a', 'b'], [a, it('b')], undefined, new Map())).toEqual({ keep: 2, enqueue: [] });         // an item the deck holds that was not handed over by this process: kept
  });
});
