// The radio never waits (the flip of the radio's player to shared code, 2026-11-23). The failure list, written first; each line has a test below.
//  N1  an opening placed after a quiet half hour does not throw the saved queue away: what the desk and the phone left ahead (and the listener's own) plays after it, in order.
//  N2  when the entry on air ends and nothing is ahead, the paper's front stories play next (the front page the phone holds, not told yet in this sheet), with no ask and no wait.
//  N3  the saved queue goes before the front stories: nothing is filled while something is ahead.
//  N4  a front story the sheet already holds (heard, on air, ahead, or cited) is never filled again; with every story told nothing is made up.
//  N5  the fill never changes what is on air (it is placed at the end), and it is placed by the phone with a reason.
//  N6  through the runtime and a deck: the opening ends, the first front story is fetched and handed to the deck, and the radio goes on to the next.
//  N7  a radio that was stopped (the listener's pause stopped it, or two failures) does not fill: the want is the listener's.
import { beforeEach, describe, expect, test } from 'bun:test';
import { Director, type Command } from './radio-director.ts';
import { RunSheetStore } from './runsheet-store.ts';
import { frontFill } from './radio-writer.ts';
import { RadioRuntime } from './radio-runtime.ts';
import { paperStory } from './runsheet-entries.ts';
import type { Entry } from './runsheet-model.ts';
import type { DeckEvent, DeckItem, DeckPort, DeckState } from './deck.ts';

const story = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: 'Words.', est_seconds: 10, revision: 0, ...o });
const opening = (id = 'p-open'): Entry => ({ id, kind: 'opening', placed_by: 'phone', state: 'ahead', title: 'Opening', text: 'Hello.', est_seconds: 5, revision: 0 });
const front = [{ route: 'r1', title: 'One' }, { route: 'r2', title: 'Two' }, { route: 'r3', title: 'Three' }, { route: 'r4', title: 'Four' }];

let store: RunSheetStore, d: Director, log: Command[], stories: { route: string; title: string }[];
const started = () => log.filter((c) => c.kind === 'start').map((c) => (c as { entry: Entry }).entry);
beforeEach(() => {
  store = new RunSheetStore({ now: () => new Date('2023-11-14T22:00:00Z') });
  stories = front;
  d = new Director(store, { fill: (removed) => frontFill(stories, store.sheet.entries, undefined, removed) });
  log = [];
  d.onCommand = (c) => { log.push(c); if (c.kind === 'start') d.started(c.entry.id, 'rs:' + c.entry.id); };
});

describe('what is ahead is kept', () => {
  test('N1 an opening after a quiet half hour keeps the saved queue and plays it after the opening', () => {
    d.applyFromDesk({ op_id: 'o1', by: 'desk', op: 'place', at: 'end', entry: story('d-old'), reason: 'extend', base_version: 0 }, 'extend');
    store.place({ ...paperStory('mine', 'Mine'), id: 'p-mine' }, 'end');
    store.place({ ...story('p-saved'), placed_by: 'phone' }, 'end');
    d.open(opening());
    expect(store.onAir()?.id).toBe('p-open');
    expect(store.ahead().map((e) => e.id)).toEqual(['d-old', 'p-mine', 'p-saved']);
    expect(store.pending.some((p) => p.op === 'remove')).toBe(false);
    d.finish('p-open', true);
    expect(store.onAir()?.id).toBe('d-old');
  });
});

/** A listening whose opening was heard a minute ago: a new opening is refused, and with an empty queue the front stories fill. */
const recentOpening = (s: RunSheetStore, dir: Director, clock: { t: string }): void => {
  clock.t = '2023-11-14T21:59:00Z';
  s.place(opening('p-old'), 'now', { by: 'phone' });          // placed and heard on the sheet only: the radio is not wanted, so nothing fills
  s.airedEnded('p-old', 'heard', 'finished', { block: 0, seconds: 5 } as never);
  clock.t = '2023-11-14T22:00:00Z';
};
describe('a refused opening with nothing ahead', () => {
  test('N8 the first front story is the one on air, the others wait behind it, and none is heard unplayed', () => {
    const clock = { t: '2023-11-14T22:00:00Z' };
    const s2 = new RunSheetStore({ now: () => new Date(clock.t) });
    const d2 = new Director(s2, { fill: (removed) => frontFill(stories, s2.sheet.entries, undefined, removed) });
    const cmds: Command[] = [];
    d2.onCommand = (c) => { cmds.push(c); if (c.kind === 'start') d2.started(c.entry.id, 'rs:' + c.entry.id); };
    recentOpening(s2, d2, clock);
    cmds.length = 0;
    d2.open(opening('p-new'));
    expect(cmds.filter((c) => c.kind === 'start').length).toBe(1);
    expect(s2.onAir()?.source?.ref).toBe('pub:r1');
    expect(s2.ahead().map((e) => e.source?.ref)).toEqual(['pub:r2', 'pub:r3']);
    expect(s2.sheet.entries.filter((e) => e.state === 'heard').map((e) => e.id)).toEqual(['p-old']);
    expect(s2.entry('p-new')).toBeUndefined();
  });
  test('N8b the same through a restored runtime: the deck is handed exactly one story, the first', async () => {
    const files = new Map<string, string>();
    const storage = { read: async () => files.get('s'), write: async (t: string) => { files.set('s', t); }, remove: async () => { files.delete('s'); } };
    const idle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };
    const clock = { t: '2023-11-14T21:59:00Z' };
    const a = new RadioRuntime({ deck: new Deck(), storage, now: () => new Date(clock.t) });
    recentOpening(a.store, a.director, clock); clock.t = '2023-11-14T21:59:00Z'; await idle();
    const deck = new Deck();
    clock.t = '2023-11-14T22:00:00Z';
    const b = new RadioRuntime({ deck, storage, now: () => new Date(clock.t), fill: (removed) => frontFill(front, b.store.sheet.entries, undefined, removed),
      resolvers: { story: async (route) => ({ title: 'Full ' + route, paragraphs: ['Words of ' + route + '.'] }) } });
    await b.restore();
    b.director.open(opening('p-new')); await idle();
    expect(deck.played.map((p) => p.title)).toEqual(['Full r1']);
    expect(b.store.sheet.entries.filter((e) => e.state === 'heard').map((e) => e.id)).toEqual(['p-old']);
    expect(b.store.ahead().map((e) => e.source?.ref)).toEqual(['pub:r2', 'pub:r3']);
  });
  test('N9 a story the listener removed or cleared does not come back', () => {
    d.open(opening()); d.finish('p-open', true);
    expect(store.ahead().map((e) => e.source?.ref)).toEqual(['pub:r2', 'pub:r3']);
    d.remove(0);                                                  // r2 is removed
    d.clearUpcoming();                                            // and r3 with the rest of Up next
    d.finish(store.onAir()!.id, true);                            // r1 ends: only r4 is neither told nor removed
    expect(store.onAir()?.source?.ref).toBe('pub:r4');
    d.finish(store.onAir()!.id, true);
    expect(store.onAir()).toBeUndefined();                        // r2 and r3 never came back
    expect(store.sheet.entries.some((e) => ['pub:r2', 'pub:r3'].includes(e.source?.ref ?? ''))).toBe(false);
  });
});

describe('nothing ahead: the paper\'s front stories', () => {
  test('N2 the opening ends with nothing ahead: the first front story plays, and the rest wait behind it', () => {
    d.open(opening());
    expect(store.ahead().length).toBe(0);
    d.finish('p-open', true);
    const air = store.onAir();
    expect(air?.kind).toBe('page'); expect(air?.source?.ref).toBe('pub:r1'); expect(air?.placed_by).toBe('phone');
    expect(store.ahead().map((e) => e.source?.ref)).toEqual(['pub:r2', 'pub:r3']);
    expect(started().at(-1)?.source?.ref).toBe('pub:r1');
  });
  test('N3 the saved queue goes first: with something ahead nothing is filled', () => {
    d.open(opening());
    d.applyFromDesk({ op_id: 'o1', by: 'desk', op: 'place', at: 'end', entry: story('d-1'), reason: 'extend', base_version: 0 }, 'extend');
    d.finish('p-open', true);
    expect(store.onAir()?.id).toBe('d-1'); expect(store.ahead().length).toBe(0);
    d.finish('d-1', true);                                       // d-1 was the last: now the front stories
    expect(store.onAir()?.source?.ref).toBe('pub:r1');
  });
  test('N4 a story the sheet holds is not filled again; every one told fills nothing', () => {
    d.open(opening());
    store.place(paperStory('r1', 'One', 'listener'), 'end');
    store.place({ ...story('d-2'), cites: [{ ref: 'pub:r2@v3', role: 'subject', label: 'Two' }] }, 'end');
    expect(frontFill(stories, store.sheet.entries).map((e) => e.source?.ref)).toEqual(['pub:r3', 'pub:r4']);
    stories = [front[0], front[1]];
    expect(frontFill(stories, store.sheet.entries)).toEqual([]);
    d.finish('p-open', true); d.finish(store.onAir()!.id, true); d.finish(store.onAir()!.id, true);
    expect(store.onAir()).toBeUndefined();                        // r1 and d-2 (r2) played; nothing is made up
  });
  test('N5 the fill is placed at the end by the phone and does not change what is on air', () => {
    d.open(opening());
    d.finish('p-open', true);
    const placed = store.pending.filter((p) => p.op === 'place' && p.entry?.source?.ref.startsWith('pub:'));
    expect(placed.length).toBe(3); expect(placed.every((p) => p.by === 'phone' && !!p.reason)).toBe(true);
    const air = store.onAir()?.id;
    d.finish(air!, true);
    expect(store.entry(air!)?.state).toBe('heard');
  });
  test('N7 a radio that is not wanted does not fill', () => {
    d.open(opening());
    d.stopAll();
    expect(store.onAir()).toBeUndefined(); expect(store.sheet.entries.every((e) => e.source?.ref?.startsWith('pub:') !== true)).toBe(true);
    expect(d.wants).toBe(false);
  });
});

/** A deck that only records what it is handed. */
class Deck implements DeckPort {
  played: DeckItem[] = []; journal: DeckEvent[] = []; seq = 0; acked = 0; item = ''; why: DeckState['why'] = 'empty';
  async state(): Promise<DeckState> { return { why: this.why, item: this.item, block: 0, seconds: 0, duration: 0, held: [], seq: this.seq, acked: this.acked, journal: this.journal.filter((l) => l.seq > this.acked) }; }
  async play(i: DeckItem): Promise<DeckState> { this.played.push(i); this.item = i.id; this.why = 'playing'; this.seq += 1; this.journal.push({ seq: this.seq, type: 'started', item: i.id, at: '', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '' }); return this.state(); }
  async enqueue(): Promise<DeckState> { return this.state(); }
  async act(): Promise<DeckState> { return this.state(); }
  async ack(s: number): Promise<DeckState> { this.acked = s; return this.state(); }
  end(): void { this.seq += 1; this.journal.push({ seq: this.seq, type: 'ended', item: this.item, at: '', block: 0, seconds: 5, final: 0, rendered: true, end: 'finished', command: '', reason: '' }); this.item = ''; this.why = 'empty'; }
}
describe('through the runtime and a deck', () => {
  test('N6 the opening ends, the first front story is fetched and handed to the deck, and the radio goes on to the next', async () => {
    const deck = new Deck(), fetched: string[] = [];
    const rt = new RadioRuntime({
      deck, now: () => new Date('2023-11-14T22:00:00Z'), fill: (removed) => frontFill(front, rt.store.sheet.entries, undefined, removed),
      resolvers: { story: async (route) => { fetched.push(route); return { title: 'Full ' + route, paragraphs: ['Words of ' + route + '.'] }; } },
    });
    const idle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };
    rt.director.open(opening()); await idle();
    expect(deck.played.map((p) => p.id)).toEqual(['p-open']);
    deck.end(); await rt.pump(); await idle();
    expect(fetched).toEqual(['r1']); expect(deck.played.at(-1)?.title).toBe('Full r1');
    deck.end(); await rt.pump(); await idle();
    expect(fetched).toEqual(['r1', 'r2']); expect(rt.store.onAir()?.source?.ref).toBe('pub:r2');
  });
});

describe('a restart keeps the saved queue', () => {
  test('N1b the sheet is restored with what was ahead; an opening after more than half an hour goes first and the queue follows it', async () => {
    const files = new Map<string, string>();
    const storage = { read: async () => files.get('s'), write: async (t: string) => { files.set('s', t); }, remove: async () => { files.delete('s'); } };
    const deck1 = new Deck();
    const a = new RadioRuntime({ deck: deck1, storage, now: () => new Date('2023-11-14T20:00:00Z') });
    a.director.applyFromDesk({ op_id: 'o1', by: 'desk', op: 'place', at: 'end', entry: story('d-1'), reason: 'extend', base_version: 0 }, 'extend');
    a.store.place({ ...story('d-2'), placed_by: 'phone' }, 'end');
    for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0));          // the store writes its file in the background
    const deck2 = new Deck();
    const b = new RadioRuntime({ deck: deck2, storage, now: () => new Date('2023-11-14T22:00:00Z') });
    await b.restore();
    expect(b.store.ahead().map((e) => e.id)).toEqual(['d-1', 'd-2']);
    b.director.open(opening('p-new'));
    const idle = async () => { for (let i = 0; i < 8; i++) await new Promise((r) => setTimeout(r, 0)); };
    await idle();
    expect(b.store.onAir()?.id).toBe('p-new'); expect(b.store.ahead().map((e) => e.id)).toEqual(['d-1', 'd-2']);
    deck2.end(); await b.pump(); await idle();
    expect(b.store.onAir()?.id).toBe('d-1');
  });
});
