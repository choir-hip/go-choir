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

