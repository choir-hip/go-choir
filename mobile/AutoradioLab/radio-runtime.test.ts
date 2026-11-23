// The radio runtime under bun test (radio-runtime.ts): the store and the director over a FAKE deck. The deck is the native audio surface; this file is the other half of
// the rule "shared code decides, the deck plays": what is handed to the deck, and what the deck's journal makes shared code do. The failure list, written first:
//  T1  an entry placed now is handed to the deck as one item named for it (speech: its words; audio: its address); what was on air is cut and the deck is not asked twice.
//  T2  the deck's `started` line is the document's start (the director's message clears); its `ended finished` line, with sound, ends the entry heard and the NEXT is handed over.
//  T3  an end with no sound (`failed`) is skipped once (failed, never a choice), the radio goes on; two in a row stop it and say so.
//  T4  a line is read once (the journal number), and is acknowledged to the deck after it was read, never before.
//  T5  the deck plays a held item by itself while shared code is not running: the next pump airs that entry on the sheet (no second `play` to the deck), in order.
//  T6  the deck holds at most three items after the one playing, only entries whose sound needs no fetch, in the sheet's order; a reorder changes what it holds.
//  T7  skip with nothing ahead stops the deck; a later arrival starts again; clearing the queue stops nothing that is on air.
//  T8  a paper story, a page, a book, a take and a file are resolved by the phone before the deck is told: what cannot be had is skipped once and the radio goes on; an entry that
//      was cut or skipped while its words were still being fetched is never played late.
//  T9  remote audio (a clip, an episode): its end is the entry's end; the lock screen's next is one skip, previous restarts an entry that has gone on and seeks audio to its start.
//  T10 the listener's turn is open: the end waits for it and the next starts when it ends; a listener pause stops nothing from starting by itself.
//  T11 the screens' view (what is on air, the scrubber labels, the desk's notice) comes from the store, the deck's state and the player's own facts: nothing is invented.
//  T12 a restart restores the sheet from the runtime's own file; what was on air is closed `failed` (a crash is not a choice) and nothing plays by itself.
//  T13 a sign-out wipe forgets the sheet, the held items and what was on air: the deck is stopped and a late journal line for the old account does nothing.
import { beforeEach, describe, expect, test } from 'bun:test';
import { RadioRuntime, type Resolvers, type NativeFacts } from './radio-runtime.ts';
import { DECK_HELD_MAX, type DeckEvent, type DeckItem, type DeckOp, type DeckPort, type DeckState, type DeckWhy } from './deck.ts';
import type { Storage } from './runsheet-store.ts';
import type { Entry, Op } from './runsheet-model.ts';
import { audio, paperStory, readAloud, book, webPage, filesAudio } from './runsheet-entries.ts';

const story = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: `Words of ${id}.`, est_seconds: 10, revision: 0, ...o });
const transition = (id: string, after: string, before: string): Entry => ({ id, kind: 'transition', placed_by: 'desk', state: 'ahead', title: '', text: 'And now.', est_seconds: 3, revision: 0, between: { after, before } });
const placeOp = (e: Entry, cause = 'extend'): Op => ({ op_id: 'o-' + e.id, by: 'desk', op: 'place', at: 'end', entry: e, reason: cause, base_version: 0 });

/** A deck as the native one behaves for what shared code can see: one item playing, a held list, a numbered journal; an item that ends plays the next held one by itself. */
class FakeDeck implements DeckPort {
  why: DeckWhy = 'empty'; item = ''; held: DeckItem[] = []; seq = 0; acked = 0; journal: DeckEvent[] = []; seconds = 0; block = 0; duration = 0;
  played: DeckItem[] = []; ops: { op: DeckOp; value?: number }[] = []; acks: number[] = [];
  current?: DeckItem; silentRun = 0;
  private line(type: DeckEvent['type'], item: string, o: Partial<DeckEvent> = {}): void {
    this.seq += 1;
    this.journal.push({ seq: this.seq, type, item, at: '2023-11-14T22:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '', ...o });
  }
  async state(): Promise<DeckState> {
    return { why: this.why, item: this.item, block: this.block, seconds: this.seconds, duration: this.duration, held: this.held.map((h) => h.id), seq: this.seq, acked: this.acked, journal: this.journal.filter((l) => l.seq > this.acked) };
  }
  async play(item: DeckItem): Promise<DeckState> {
    if (this.current) this.line('ended', this.current.id, { end: 'replaced' });
    this.current = item; this.item = item.id; this.why = 'playing'; this.played.push(item); this.line('started', item.id);
    return this.state();
  }
  async enqueue(item: DeckItem, at?: number): Promise<DeckState> {
    this.held = this.held.filter((h) => h.id !== item.id);
    if (at !== undefined && at >= 0 && at < this.held.length) this.held.splice(at);
    if (this.held.length < DECK_HELD_MAX) this.held.push(item);
    return this.state();
  }
  async act(op: DeckOp, value?: number): Promise<DeckState> {
    this.ops.push({ op, value });
    if (op === 'stop') { if (this.current) this.line('ended', this.current.id, { end: 'stopped' }); this.current = undefined; this.item = ''; this.held = []; this.why = 'stopped'; }
    if (op === 'trim') this.held.splice(value ?? 0);
    if (op === 'pause') this.why = 'paused_listener';
    if (op === 'resume') this.why = 'playing';
    return this.state();
  }
  async ack(seq: number): Promise<DeckState> { this.acks.push(seq); this.acked = Math.max(this.acked, seq); return this.state(); }
  // ---- what the audio does, from the test
  /** The playing item ends; the deck plays on through its held items by itself (shared code may be frozen). */
  end(end: DeckEvent['end'] = 'finished', o: { rendered?: boolean; block?: number; seconds?: number; final?: number; reason?: string } = {}): void {
    const c = this.current; if (!c) return;
    this.line('ended', c.id, { end, rendered: o.rendered ?? end === 'finished', block: o.block ?? 0, seconds: o.seconds ?? 0, final: o.final ?? 0, reason: o.reason ?? '' });
    this.current = undefined; this.item = ''; this.why = 'empty';
    this.silentRun = (o.rendered ?? end === 'finished') ? 0 : this.silentRun + 1;      // rule 4: two in a row that made no sound stop the deck
    if (this.silentRun >= 2) { this.why = 'stopped'; return; }
    const next = this.held.shift();
    if (next) { this.current = next; this.item = next.id; this.why = 'playing'; this.line('started', next.id); }
  }
  remote(command: DeckEvent['command']): void { this.line('remote', this.current?.id ?? '', { command, block: this.block, seconds: this.seconds }); }
  /** The lock screen's next as Deck.remoteNext does it: the line first, then the skip and the next held item plays on, with shared code not running. */
  remoteNext(): void {
    this.remote('next');
    const c = this.current; if (!c) return;
    this.line('ended', c.id, { end: 'skipped', block: this.block, seconds: this.seconds });
    this.current = undefined; this.item = ''; this.why = 'empty';
    const next = this.held.shift();
    if (next) { this.current = next; this.item = next.id; this.why = 'playing'; this.line('started', next.id); }
  }
}

let deck: FakeDeck, rt: RadioRuntime, files: Map<string, string>, facts: NativeFacts, fetched: string[];
let resolvers: Partial<Resolvers>;
const NOW = new Date('2023-11-14T22:00:00Z');
const storage = (): Storage => ({ read: async () => files.get('sheet'), write: async (t) => { files.set('sheet', t); }, remove: async () => { files.delete('sheet'); } });
const make = (): RadioRuntime => new RadioRuntime({ deck, storage: storage(), now: () => NOW, resolvers, native: { facts: async () => facts } });
const idle = async () => { for (let i = 0; i < 6; i++) await new Promise((r) => setTimeout(r, 0)); };
const playedIds = () => deck.played.map((p) => p.id);

beforeEach(() => {
  deck = new FakeDeck(); files = new Map(); fetched = [];
  facts = { turnOpen: false, remainder: '', audio: undefined, position: { block: 0, seconds: 0 }, remote: undefined, wants: false };
  resolvers = {
    story: async (route) => { fetched.push('story:' + route); return route === 'missing' ? undefined : { title: 'Full ' + route, paragraphs: [`Para one of ${route}.`, `Para two of ${route}.`] }; },
    webPage: async (url) => { fetched.push('web:' + url); return { title: 'A page', paragraphs: ['Page words.'] }; },
    book: async (id, at) => { fetched.push(`book:${id}@${at}`); return { title: 'A book', paragraphs: ['Block one.', 'Block two.'] }; },
    take: async (id) => { fetched.push('take:' + id); return id === 'gone' ? undefined : { url: 'app:/cache/takes/' + id + '.m4a' }; },
    file: async (a) => { fetched.push('file:' + a); return { url: 'app:/cache/files/h/' + a.split('/').pop() }; },
  };
  rt = make();
});

describe('what the deck is handed', () => {
  test('T1 a story placed now is one speech item named for the entry; what was on air is cut', async () => {
    rt.director.open({ ...story('p-open'), kind: 'opening', placed_by: 'phone', title: 'Opening' });
    await idle();
    expect(playedIds()).toEqual(['p-open']);
    expect(deck.played[0]).toMatchObject({ kind: 'speech', document: 'rs:p-open', paragraphs: 'Words of p-open.', segment: 'opening' });
    rt.play(story('l-1', { placed_by: 'listener' }));
    await idle();
    expect(rt.store.entry('p-open')?.ended?.outcome).toBe('cut');
    expect(playedIds()).toEqual(['p-open', 'l-1']);
  });
  test('T1 an audio entry is an audio item at its address and range', async () => {
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Clip', show: 'Show', start: 10, end: 40 }, 'clip-1'));
    await idle();
    expect(deck.played[0]).toMatchObject({ kind: 'audio', url: 'https://x/a.mp3', show: 'Show', start: 10, end: 40 });
  });
});

describe('what the deck reports', () => {
  test('T2 a finished end with sound is heard, and the next entry is handed over', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend');
    await idle();
    deck.end('finished', { block: 1, seconds: 8 }); await rt.pump(); await idle();
    expect(rt.store.entry('a')?.state).toBe('heard'); expect(rt.store.entry('a')?.ended?.outcome).toBe('finished'); expect(rt.store.entry('a')?.ended?.last_block).toBe(1);
    expect(rt.store.onAir()?.id).toBe('b');
  });
  test('T3 an end with no sound is skipped once as failed and the radio goes on; two in a row stop it', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle();
    deck.end('failed', { rendered: false, reason: 'the engine could not start it' }); await rt.pump(); await idle();
    expect(rt.store.entry('a')?.state).toBe('skipped'); expect(rt.store.entry('a')?.ended?.outcome).toBe('failed');
    expect(rt.store.onAir()?.id).toBe('b');
    deck.end('failed', { rendered: false }); await rt.pump(); await idle();
    expect(rt.director.wants).toBe(false); expect(rt.director.message).toContain('could not play'); expect(rt.store.onAir()).toBeUndefined();
  });
  test('T4 a journal line is read once and acknowledged after it was read', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend');
    await idle();
    await rt.pump(); await rt.pump();
    expect(deck.acks.length).toBeGreaterThan(0); expect(deck.acks.at(-1)).toBe(deck.seq);
    const before = rt.store.sheet.version;
    await rt.pump(); expect(rt.store.sheet.version).toBe(before);
    deck.end('finished'); await rt.pump(); await idle();
    const v = rt.store.sheet.version;
    await rt.pump(); expect(rt.store.sheet.version).toBe(v);
  });
  test('T5 the deck played a held item while shared code was frozen: the sheet follows, with no second play', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); rt.director.applyFromDesk(placeOp(story('c')), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['b', 'c']);
    deck.end('finished'); deck.end('finished');                       // a ends, b plays on its own, b ends, c plays on its own: shared code was not running
    await rt.pump(); await idle();
    expect(rt.store.entry('a')?.state).toBe('heard'); expect(rt.store.entry('b')?.state).toBe('heard'); expect(rt.store.onAir()?.id).toBe('c');
    expect(playedIds()).toEqual(['a']);                                // the deck was asked to play only the first
  });
});

describe('what the deck holds', () => {
  test('T6 every ready entry ahead, in order, only those that need no fetch', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd', 'e']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['b', 'c', 'd', 'e']);
    rt.store.move('e', 'next'); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['e', 'b', 'c', 'd']);
  });
  test('T6 a story to fetch is not held; what follows it is not either (order is kept)', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    rt.director.applyFromDesk(placeOp(story('b')), 'extend'); rt.store.place(paperStory('/pub/x', 'X'), 'end'); rt.director.applyFromDesk(placeOp(story('d')), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['b']);
  });
  test('T6 a transition is held with its rule: it plays only after an entry heard to its end', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(transition('t', 'a', 'b')), 'extend'); rt.director.applyFromDesk(placeOp(story('b')), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['t', 'b']);
    expect(deck.held[0].requiresPreviousHeard).toBe(true); expect(deck.held[1].requiresPreviousHeard).toBe(false);
  });
});

describe('the deck plays on without shared code', () => {
  test('T6b six ready entries are all held, so the radio plays through them while shared code is frozen', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd', 'e', 'f', 'g']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['b', 'c', 'd', 'e', 'f', 'g']);
    for (let i = 0; i < 7; i++) deck.end('finished');
    await rt.pump(); await idle();
    expect(['a', 'b', 'c', 'd', 'e', 'f', 'g'].every((id) => rt.store.entry(id)?.state === 'heard')).toBe(true);
    expect(playedIds()).toEqual(['a']);
  });
});

describe('a deck that cannot be read is not an empty deck (review B1-5)', () => {
  test('T14 a failed state read during the pump replays nothing: the entry sounding goes on and is not handed over again', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    expect(playedIds()).toEqual(['a']);
    let fail = true; const read = deck.state.bind(deck);
    deck.state = async () => { if (fail) throw new Error('the module did not answer'); return read(); };
    rt.director.applyFromDesk(placeOp(story('b')), 'extend');
    await rt.pump(); await idle();
    fail = false; await rt.pump(); await idle();
    expect(playedIds()).toEqual(['a']);                                   // not 'a' again from a failed read that looked like an empty deck
  });
});

describe('what the runtime remembers does not grow without end (review B1-7)', () => {
  test('T14b an entry that left the sheet is forgotten by the next pump', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(rt.knownCount()).toBeGreaterThanOrEqual(4);
    rt.clear(); await idle(); await rt.pump(); await idle();
    expect(rt.knownCount()).toBeLessThanOrEqual(1);                       // the one on air (and what the sheet still holds)
  });
});

describe('seat 2 items 3 and 7', () => {
  test('T14c what was handed to the deck for an entry that left the sheet is forgotten too', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(rt.heldSentCount()).toBeGreaterThanOrEqual(3);
    rt.clear(); await idle(); await rt.pump(); await idle();
    expect(rt.heldSentCount()).toBe(0);
  });
  test('T8d a pause made while the next entry\'s words are being fetched is not overridden: the entry starts, and is paused', async () => {
    let release: (v: { title: string; paragraphs: string[] }) => void = () => undefined;
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    resolvers.story = () => new Promise((r) => { release = r; });
    rt = make();
    rt.play(story('a2', { placed_by: 'listener' })); await idle();
    rt.play(paperStory('/pub/slow', 'Slow')); await idle();
    expect(await rt.toggle()).toBe('paused');                                  // the listener presses Pause while the words are on their way
    release({ title: 'Slow', paragraphs: ['Words.'] }); await idle(); await rt.pump(); await idle();
    expect(deck.why).toBe('paused_listener');                                  // not 'playing': the start did not override the pause
    expect(await rt.toggle()).toBe('resumed'); expect(deck.why).toBe('playing');
  });
});

describe('stopping and starting', () => {
  test('T7 skip with nothing ahead stops the deck; a later arrival starts again', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    rt.skip(); await idle();
    expect(deck.ops.map((o) => o.op)).toContain('stop'); expect(rt.store.onAir()).toBeUndefined();
    rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    expect(rt.store.onAir()?.id).toBe('b'); expect(playedIds().at(-1)).toBe('b');
  });
  test('T7 clearing the queue keeps what is on air', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    rt.clear(); await idle();
    expect(rt.store.onAir()?.id).toBe('a'); expect(rt.store.ahead()).toEqual([]); expect(deck.ops.map((o) => o.op)).not.toContain('stop');
  });
});

describe('what the phone resolves first', () => {
  test('T8 a paper story is fetched in full and handed over as its paragraphs, named for its route', async () => {
    rt.play(paperStory('/pub/x', 'X')); await idle();
    expect(fetched).toEqual(['story:/pub/x']);
    expect(deck.played[0]).toMatchObject({ kind: 'speech', paragraphs: 'Para one of /pub/x.\n\nPara two of /pub/x.', document: 'pub:/pub/x' });
  });
  test('T8 a web page, a book, a take and a file', async () => {
    rt.play(webPage('https://e.org/p', 'P')); await idle();
    expect(deck.played.at(-1)).toMatchObject({ kind: 'speech', paragraphs: 'Page words.' });
    rt.play(book('b1', 'B', 7)); await idle();
    expect(fetched).toContain('book:b1@7'); expect(deck.played.at(-1)?.paragraphs).toBe('Block one.\n\nBlock two.');
    rt.play({ id: 'l-take', kind: 'take', placed_by: 'listener', state: 'ahead', title: 'T', est_seconds: 5, revision: 0, source: { ref: 'take:t1', label: 'Ana' } } as Entry); await idle();
    expect(deck.played.at(-1)).toMatchObject({ kind: 'audio', url: 'app:/cache/takes/t1.m4a' });
    rt.play(filesAudio('file:/Music/a.mp3', 'A')); await idle();
    expect(deck.played.at(-1)).toMatchObject({ kind: 'audio', url: 'app:/cache/files/h/a.mp3' });
  });
  test('T8 what cannot be had is skipped once as failed and the radio goes on', async () => {
    rt.play(paperStory('missing', 'Gone')); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    expect(rt.store.entry(rt.store.sheet.entries[0].id)?.ended?.outcome).toBe('failed'); expect(rt.store.onAir()?.id).toBe('b');
    expect(playedIds().at(-1)).toBe('b');
  });
  test('T8b a skip while the on-air entry\'s words are being fetched leaves the NEXT entry playing, not stopped behind it (review B1-1)', async () => {
    let release: (v: { title: string; paragraphs: string[] }) => void = () => undefined;
    resolvers.story = () => new Promise((r) => { release = r; });
    rt = make();
    rt.play(paperStory('/pub/slow', 'Slow')); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    rt.skip(); await idle();                                  // the stop (for the slow one) and the start of b, while the pump waits for the words
    release({ title: 'Slow', paragraphs: ['Late words.'] }); await idle(); await rt.pump(); await idle();
    expect(rt.store.onAir()?.id).toBe('b');
    expect(deck.item).toBe('b'); expect(deck.why).toBe('playing');       // the deck plays what the sheet has on air
    expect(playedIds()).toEqual(['b']);
  });
  test('T8c twice Next through two slow stories ends with the last one on air and sounding', async () => {
    const waiting: ((v: { title: string; paragraphs: string[] }) => void)[] = [];
    resolvers.story = () => new Promise((r) => { waiting.push(r); });
    rt = make();
    rt.play(paperStory('/pub/one', 'One')); rt.director.applyFromDesk(placeOp(paperStory('/pub/two', 'Two')), 'extend'); rt.director.applyFromDesk(placeOp(story('c')), 'extend'); await idle();
    rt.skip(); await idle(); rt.skip(); await idle();
    for (const w of waiting) w({ title: 'x', paragraphs: ['Words.'] });
    await idle(); await rt.pump(); await idle();
    expect(rt.store.onAir()?.id).toBe('c'); expect(deck.item).toBe('c'); expect(deck.why).toBe('playing');
  });
  test('T8 an entry cut while its words were being fetched is never played late', async () => {
    let release: (v: { title: string; paragraphs: string[] }) => void = () => undefined;
    resolvers.story = () => new Promise((r) => { release = r; });
    rt = make();
    rt.play(paperStory('/pub/slow', 'Slow')); await idle();
    rt.play(story('l-now', { placed_by: 'listener' })); await idle();
    release({ title: 'Slow', paragraphs: ['Late words.'] }); await idle();
    expect(playedIds()).toEqual(['l-now']);
  });
});

describe('remote audio and the lock screen', () => {
  test('T9 the end of audio is the end of its entry; the lock screen\'s next is one skip', async () => {
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Ep', show: 'Show' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    deck.end('finished', { seconds: 300 }); await rt.pump(); await idle();
    expect(rt.store.sheet.entries.find((e) => e.kind === 'episode')?.ended?.outcome).toBe('finished'); expect(rt.store.onAir()?.id).toBe('b');
    deck.remote('next'); await rt.pump(); await idle();
    expect(rt.store.entry('b')?.ended?.outcome).toBe('skipped'); expect(rt.store.onAir()).toBeUndefined();
  });
  test('T9 the lock screen\'s next, acted on by the deck while shared code was not running, is one skip cut where it was, and the next entry is not started twice', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); rt.director.applyFromDesk(placeOp(story('c')), 'extend');
    await idle(); await rt.pump(); await idle();
    deck.block = 3; deck.seconds = 11; deck.remoteNext(); deck.block = 0; deck.seconds = 0;
    await rt.pump(); await idle();
    expect(rt.store.entry('a')?.state).toBe('skipped'); expect(rt.store.entry('a')?.ended?.last_block).toBe(3); expect(rt.store.entry('a')?.ended?.outcome).toBe('skipped');
    expect(rt.store.onAir()?.id).toBe('b'); expect(playedIds()).toEqual(['a']);             // b played on by the deck: shared code did not play it again
    expect(deck.held.map((h) => h.id)).toEqual(['c']);
  });
  test('T9 previous on an entry that has gone on, or on audio, was done by the deck: shared code does not play it again', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    deck.block = 2; deck.seconds = 9; deck.remote('previous'); await rt.pump(); await idle();
    expect(playedIds()).toEqual(['a']);
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Ep', show: 'Show' })); await idle();
    deck.block = 0; deck.seconds = 50; deck.remote('previous'); await rt.pump(); await idle();
    expect(deck.ops.filter((o) => o.op === 'seekTo')).toEqual([]);
  });
  test('T9 previous on an entry just begun is the entry before it: shared code plays that', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle(); await rt.pump(); await idle();
    deck.end('finished'); await rt.pump(); await idle();            // b on air
    deck.block = 0; deck.seconds = 1; deck.remote('previous'); await rt.pump(); await idle();
    expect(rt.store.onAir()?.title).toBe('T a');
  });
});

describe('the listener speaks', () => {
  test('T10 the end waits for the turn and the next starts when it ends', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    facts.turnOpen = true; await rt.pump();
    deck.end('finished'); await rt.pump(); await idle();
    expect(rt.store.onAir()?.id).toBe('a');                           // closed only when the turn ends
    facts.turnOpen = false; await rt.turnEnded(); await idle();
    expect(rt.store.entry('a')?.state).toBe('heard'); expect(rt.store.onAir()?.id).toBe('b');
  });
  test('T10 a listener pause starts nothing by itself', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    deck.why = 'paused_listener'; await rt.pump();
    expect(rt.view().playing).toBe(false); expect(rt.view().paused).toBe(true);
  });
});

describe('the screens\' view', () => {
  test('T11 on air, up next and the desk come from the store, the deck and the facts', async () => {
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Ep', show: 'Show' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    deck.duration = 250; deck.seconds = 62;
    facts.remote = { elapsed: 62, duration: 250 };
    rt.setDesk({ notice: 'The desk is out.', writing: true, dry: false });
    await rt.pump();
    const v = rt.view();
    expect(v).toMatchObject({ current: 'Ep', currentKind: 'audio', audio: true, playing: true, elapsedLabel: '1:02', remainingLabel: '-3:08', notice: 'The desk is out.', writing: true });
    expect(v.fraction).toBeCloseTo(62 / 250, 5);
    expect(rt.view().waiting).toBe(false); deck.why = 'held_session'; await rt.pump(); expect(rt.view().waiting).toBe(true);
  });
  test('T11 with nothing on air the view is empty and plain', async () => {
    expect(rt.view()).toMatchObject({ current: '', audio: false, playing: false, onAir: false, duration: 0, elapsedLabel: '' });
  });
});

describe('a launch after the deck played while shared code was gone', () => {
  const line = (n: number, type: DeckEvent['type'], item: string, o: Partial<DeckEvent> = {}): DeckEvent =>
    ({ seq: n, type, item, at: '2023-11-14T03:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '', ...o });
  test('T12b a finished end the deck journalled keeps its verdict and does not replay; the entry it began alone is aired and ended', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); rt.director.applyFromDesk(placeOp(story('c')), 'extend');
    await idle(); await rt.pump(); await idle();
    const lines = [line(2, 'ended', 'a', { end: 'finished', rendered: true, block: 1, seconds: 8 }), line(3, 'started', 'b'), line(4, 'ended', 'b', { end: 'finished', rendered: true, block: 2, seconds: 9, at: '2023-11-14T03:05:00Z' }), line(5, 'started', 'c')];
    deck = new FakeDeck(); deck.journal = lines; deck.seq = 5;
    const again = make(); await again.restore(); await idle();
    expect(again.store.entry('a')?.ended).toMatchObject({ outcome: 'finished', last_block: 1 });
    expect(again.store.entry('b')?.state).toBe('heard'); expect(again.store.entry('b')?.ended?.at).toBe('2023-11-14T03:05:00Z');
    expect(again.store.entry('c')?.ended?.outcome).toBe('failed');       // c was sounding when the process died: interrupted, no verdict
    expect(deck.played).toEqual([]); expect(deck.acks.at(-1)).toBe(5);
  });
  test('T12c with no journal an entry on air at the kill is closed failed, as before', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    deck = new FakeDeck(); const again = make(); await again.restore();
    expect(again.store.entry('a')?.ended?.outcome).toBe('failed');
  });
});

describe('restart and sign-out', () => {
  test('T12 a restart restores the sheet; what was on air is closed failed and nothing plays', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle();
    expect(files.has('sheet')).toBe(true);
    deck = new FakeDeck(); const again = make(); await again.restore(); await idle();
    expect(again.store.entry('a')?.ended?.outcome).toBe('failed'); expect(again.store.ahead().map((e) => e.id)).toEqual(['b']); expect(again.store.onAir()).toBeUndefined();
    expect(deck.played).toEqual([]);
  });
  test('T13 sign-out stops the deck, forgets the sheet and ignores a late line', async () => {
    rt.play(story('a', { placed_by: 'listener' })); rt.director.applyFromDesk(placeOp(story('b')), 'extend'); await idle(); await rt.pump(); await idle();
    await rt.signOut(); await idle();
    expect(deck.ops.map((o) => o.op)).toContain('stop'); expect(rt.store.sheet.entries).toEqual([]); expect(files.has('sheet')).toBe(false);
    deck.end('finished'); await rt.pump(); await idle();
    expect(rt.store.sheet.entries).toEqual([]); expect(rt.view().onAir).toBe(false);
  });
});

// RED: the posted operations (what the phone tells the computer about the run sheet). T14 to T17, written first.
//  T14 a listener's placement and the phone's own `aired` reports are posted, in order, with op ids; the desk's entries are the desk's and are not posted back.
//  T15 a route that is not there yet (404) or no network leaves the ops pending, in order; they go when it answers, once each.
//  T16 an end is reported with where it ended (the deck's block and seconds), `heard` and `finished`; a failed start is `skipped` and `failed`, never a choice.
//  T17 sign-out reports the entry on air as `left` while the old key is held, then forgets what was never answered: nothing is posted afterwards.
import type { Poster, PostOutcome } from './runsheet-store.ts';
describe('the clip card', () => {
  test('T18 keep listening plays the episode on past the clip from where it is; the card stops being a clip', async () => {
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Ep', show: 'Show', start: 10, end: 40, episode_id: 'pod:f/g' }, 'clip-1')); await idle();
    expect(rt.view()).toMatchObject({ clip: true, keepTitle: 'Ep (Show)' });
    deck.seconds = 25; await rt.keepListening(); await idle();
    expect(deck.played.at(-1)).toMatchObject({ kind: 'audio', url: 'https://x/a.mp3', end: 0, seconds: 25 });
    expect(rt.view().clip).toBe(false); expect(rt.store.onAir()?.id).toBe(deck.played[0].id);
  });
});

describe('the deck\'s time', () => {
  test('T19 an end is posted with the time the deck wrote it, not the pump\'s', async () => {
    const posted: Op[] = [];
    const prt = new RadioRuntime({ deck, storage: storage(), poster: { post: async (ops) => { posted.push(...ops); return { kind: 'answered', results: [] }; }, fetchSheet: async () => undefined }, now: () => new Date('2023-11-14T07:00:00Z'), resolvers });
    prt.play(story('a', { placed_by: 'listener' })); await idle();
    deck.end('finished', { block: 1, seconds: 3 }); deck.journal[deck.journal.length - 1].at = '2023-11-14T03:00:00Z';
    await prt.pump(); await idle(); await prt.store.flushTask;
    expect(posted.find((o) => o.op === 'aired' && o.id === 'a' && o.state === 'heard')?.ended?.at).toBe('2023-11-14T03:00:00Z');
  });
});

describe('posted ops', () => {
  let posts: Op[][], outcome: PostOutcome, prt: RadioRuntime;
  const poster: Poster = { post: async (ops) => { posts.push(ops); return outcome; }, fetchSheet: async () => undefined };
  const flat = () => posts.flat().map((o) => [o.op, o.id ?? o.entry?.id, o.state ?? o.at]);
  beforeEach(() => {
    posts = []; outcome = { kind: 'answered', results: [] }; deck = new FakeDeck(); files = new Map();
    prt = new RadioRuntime({ deck, storage: storage(), poster, now: () => NOW, resolvers });
  });
  test('T14 the listener\'s placement and the phone\'s reports are posted; the desk\'s entries are not', async () => {
    prt.play(story('a', { placed_by: 'listener' })); prt.director.applyFromDesk(placeOp(story('b')), 'extend');
    await idle(); await prt.store.flushTask;
    expect(flat()[0]).toEqual(['place', 'a', 'now']);
    deck.end('finished', { block: 2, seconds: 9 }); await prt.pump(); await idle(); await prt.store.flushTask;
    const ops = posts.flat();
    expect(ops.some((o) => o.op === 'place' && o.entry?.id === 'b')).toBe(false);
    const ended = ops.find((o) => o.op === 'aired' && o.id === 'a' && o.state === 'heard');
    expect(ended?.ended).toMatchObject({ outcome: 'finished', last_block: 2 }); expect(ended?.position_seconds).toBe(9);
    expect(ops.some((o) => o.op === 'aired' && o.id === 'b' && o.state === 'on_air')).toBe(true);
    expect(new Set(ops.map((o) => o.op_id)).size).toBe(ops.length);
  });
  test('T15 a route that is not there leaves the ops pending, in order, and they go once when it answers', async () => {
    outcome = { kind: 'notYet' };
    prt.play(story('a', { placed_by: 'listener' })); await idle(); await prt.store.flushTask;
    prt.skip(); await idle(); await prt.store.flushTask;
    expect(prt.store.pending.map((o) => o.op)).toEqual(['place', 'skip']);
    outcome = { kind: 'answered', results: [] }; posts = [];
    prt.store.flush(); await prt.store.flushTask; expect(posts.length).toBe(0);            // backing off after the refusal (30 s): not posted again at once (review B1-3)
    prt.store.flush(true); await prt.store.flushTask;                                       // the wait over
    expect(posts.flat().map((o) => o.op)).toEqual(['place', 'skip']); expect(prt.store.pending).toEqual([]);
    prt.store.flush(); await prt.store.flushTask; expect(posts.length).toBe(1);
  });
  test('T16 a failed start is skipped and failed, never a choice', async () => {
    prt.play(story('a', { placed_by: 'listener' })); await idle();
    deck.end('failed', { rendered: false }); await prt.pump(); await idle(); await prt.store.flushTask;
    const o = posts.flat().find((x) => x.op === 'aired' && x.id === 'a' && x.state === 'skipped');
    expect(o?.ended?.outcome).toBe('failed');
  });
  test('T17 sign-out reports left with the old key, then nothing more is posted', async () => {
    prt.play(story('a', { placed_by: 'listener' })); await idle(); await prt.store.flushTask;
    posts = [];
    await prt.signOut(); await idle();
    const left = posts.flat().find((o) => o.op === 'aired' && o.id === 'a');
    expect(left).toMatchObject({ state: 'heard' }); expect(left?.ended?.outcome).toBe('left');
    const n = posts.length;
    prt.store.flush(); await prt.store.flushTask; expect(posts.length).toBe(n);
    expect(prt.store.pending).toEqual([]); expect(prt.store.poster).toBeUndefined();
  });
});

describe('sign-out when the route does not answer', () => {
  test('T17b a poster that never answers does not hold the sign-out, and nothing is posted under the next account', async () => {
    let posts = 0;
    const silent: Poster = { post: () => { posts++; return new Promise(() => undefined); }, fetchSheet: async () => undefined };
    const prt = new RadioRuntime({ deck, storage: storage(), poster: silent, now: () => NOW, resolvers });
    prt.play(story('a', { placed_by: 'listener' })); await idle();
    const t0 = Date.now(); await prt.signOut();
    expect(Date.now() - t0).toBeLessThan(5000);
    expect(prt.store.pending).toEqual([]); expect(prt.store.poster).toBeUndefined(); expect(files.has('sheet')).toBe(false);
    const n = posts; prt.store.flush(); expect(posts).toBe(n);
  });
});

describe('what the pulse says is due (the pump itself is a mutation: it is not let go with the pulse)', () => {
  test('T14 the desk places entries: held list and starts are due until a pump, then nothing is', async () => {
    rt.director.open({ ...story('p-open'), kind: 'opening', placed_by: 'phone', title: 'Opening' });
    await idle(); await rt.pump(); await idle();
    expect(rt.needsWork()).toBe(false);
    rt.director.applyFromDesk(placeOp(story('d-1')), 'extend'); rt.director.applyFromDesk(placeOp(story('d-2')), 'extend');
    await rt.peek();
    expect(rt.needsWork()).toBe(true);                         // the deck holds neither: someone must enqueue them
    await rt.pump(); await rt.peek();
    expect(deck.held.map((h) => h.id)).toEqual(['d-1', 'd-2']);
    expect(rt.needsWork()).toBe(false);
  });
  test('T14b a journal line the pump has not read is work', async () => {
    rt.director.open({ ...story('p-open'), kind: 'opening', placed_by: 'phone', title: 'Opening' });
    await idle(); await rt.pump(); await idle();
    deck.end('finished'); await rt.peek();
    expect(rt.needsWork()).toBe(true);
    await rt.pump(); await rt.peek();
    expect(rt.needsWork()).toBe(false);
  });
});
