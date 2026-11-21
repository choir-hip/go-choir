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
  remote(command: DeckEvent['command']): void { this.line('remote', this.current?.id ?? '', { command }); }
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
    expect(deck.played[0]).toMatchObject({ kind: 'speech', document: 'rs:p-open', paragraphs: 'Words of p-open.' });
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
  test('T6 at most three, the next entries in order, only those that need no fetch', async () => {
    rt.play(story('a', { placed_by: 'listener' }));
    for (const id of ['b', 'c', 'd', 'e']) rt.director.applyFromDesk(placeOp(story(id)), 'extend');
    await idle(); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['b', 'c', 'd']);
    rt.store.move('e', 'next'); await rt.pump(); await idle();
    expect(deck.held.map((h) => h.id)).toEqual(['e', 'b', 'c']);
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
  test('T9 previous restarts an entry that has gone on; audio seeks to its start', async () => {
    rt.play(story('a', { placed_by: 'listener' })); await idle();
    deck.block = 2; deck.seconds = 9; deck.remote('previous'); await rt.pump(); await idle();
    expect(playedIds().filter((i) => i === 'a').length).toBe(2);       // the same entry again
    rt.play(audio({ url: 'https://x/a.mp3', title: 'Ep', show: 'Show' })); await idle();
    deck.seconds = 50; deck.remote('previous'); await rt.pump(); await idle();
    expect(deck.ops).toContainEqual({ op: 'seekTo', value: 0 });
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
