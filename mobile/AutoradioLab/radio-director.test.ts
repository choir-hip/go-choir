// The radio's decisions under bun test: what plays next and why. Ported from RunSheetStoreTests.swift (numbers are its list); the engine is a
// recording fake that only hears commands, so no audio and no deck is involved.
//  1 the opening is on air at once · 2 the desk's entries go at the end and never change what is on air · 3 skip is one op and the next is the next entry
//  4 skip with nothing ahead stops the sound and swaps nothing in; a later arrival carries on · 5 an entry that ends is reported heard and the next is aired
//  5 the end waits for the listener's turn · 5 an entry that cannot play is skipped once and two in a row stop the radio · 15 queue and play this
//  16 previous: barely begun is the one before, well into it is its own start · 21 one opening per start (six cases) · 22 transitions
//  24 honest reports · 8 only a play changes what is on air · plus the plans (what each kind of entry starts with) and the retry delays.
import { beforeEach, describe, expect, test } from 'bun:test';
import { Director, plan, previousRestartsCurrent, retryDelay, type Command } from './radio-director.ts';
import { RunSheetStore } from './runsheet-store.ts';
import { paperStory, readAloud, audio, audioSpec } from './runsheet-entries.ts';
import type { Entry, Op } from './runsheet-model.ts';

const story = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, text: 'Words.', est_seconds: 10, revision: 0, ...o });
const deskOpening = (id = 'd-open'): Entry => ({ id, kind: 'opening', placed_by: 'desk', state: 'ahead', title: 'Opening', text: 'This is Autoradio.', est_seconds: 5, revision: 0 });
const phoneOpening = (id = 'p-open'): Entry => ({ id, kind: 'opening', placed_by: 'phone', state: 'ahead', title: 'Opening', text: 'Hello.', est_seconds: 5, revision: 0 });
const placeOp = (e: Entry, cause = 'open'): Op => ({ op_id: 'o-' + e.id, by: 'desk', op: 'place', at: 'end', entry: e, reason: cause, base_version: 0 });

let store: RunSheetStore, d: Director, log: Command[];
/** Entries the fake deck cannot start (it makes no document for them). */
let cannot = new Set<string>();
const started = () => log.filter((c) => c.kind === 'start').map((c) => (c as { entry: Entry }).entry.id);
/** The deck plays what it is told: each start makes a document. */
function setup(): void {
  store = new RunSheetStore({ now: () => new Date('2023-11-14T22:00:00Z') });
  d = new Director(store);
  log = [];
  cannot = new Set();
  d.onCommand = (c) => { log.push(c); if (c.kind === 'start' && !cannot.has(c.entry.id)) d.started(c.entry.id, 'rs:' + c.entry.id); };
}
beforeEach(setup);
const deskPlace = (e: Entry) => d.applyFromDesk(placeOp(e), 'extend');
function opened(ahead: string[]): string {
  const o = phoneOpening();
  d.open(o);
  for (const a of ahead) deskPlace(story(a));
  return o.id;
}

describe('the opening and the desk', () => {
  test('1: the opening is on air at once, with no network and no model', () => {
    const o = opened([]);
    expect(store.onAir()?.id).toBe(o); expect(started()).toEqual([o]); expect(store.pending[0].op).toBe('place');
  });
  test('2: the desk\'s entries are placed at the end and never change what is on air', () => {
    const o = opened([]);
    deskPlace(story('d-1')); deskPlace(story('d-2'));
    expect(store.sheet.entries.map((e) => e.id)).toEqual([o, 'd-1', 'd-2']); expect(store.onAir()?.id).toBe(o);
    expect(store.pending.filter((p) => p.by === 'desk').length).toBe(0);     // the desk's ops are the desk's: not posted back
  });
  test('8: only a play changes what is on air; reading changes nothing', () => {
    const o = opened(['d-1']);
    const v = store.sheet.version;
    store.ahead(3); store.heard(3);
    expect(store.sheet.version).toBe(v); expect(store.onAir()?.id).toBe(o);
    d.playNow(readAloud('mail:m1', 'A message'), { title: 'A message', paragraphs: ['Hello.'] });
    expect(store.onAir()?.kind).toBe('mail'); expect(store.entry(o)?.ended?.outcome).toBe('cut'); expect(store.ahead().map((e) => e.id)).toEqual(['d-1']);
  });
  test('14: an opening keeps the saved queue: what an earlier listening left ahead and the listener\'s own play after it', () => {
    deskPlace(story('d-old')); store.place(paperStory('mine', 'Mine'), 'end');
    const o = phoneOpening();
    d.open(o);
    expect(store.entry('d-old')?.state).toBe('ahead'); expect(store.ahead().length).toBe(2); expect(store.onAir()?.id).toBe(o.id);
    expect(store.pending.some((p) => p.op === 'remove')).toBe(false);
  });
});

describe('next, previous and ends', () => {
  test('3: skip is one op and the next is the next entry, then the phone reports it on air', () => {
    const o = opened(['d-1', 'd-2']);
    const before = store.pending.length;
    d.next();
    expect(store.onAir()?.id).toBe('d-1'); expect(store.entry(o)?.state).toBe('skipped'); expect(store.entry(o)?.ended?.outcome).toBe('skipped');
    expect(store.pending.slice(before).map((p) => p.op)).toEqual(['skip', 'aired']);
    expect(started().at(-1)).toBe('d-1');
  });
  test('3: the lock screen\'s next and a swipe are the same op', () => {
    opened(['d-1', 'd-2']);
    d.next();
    expect(store.pending.filter((p) => p.op === 'skip').length).toBe(1); expect(store.onAir()?.id).toBe('d-1');
  });
  test('4: skip with nothing ahead stops the sound and swaps nothing in; what arrives later carries on', () => {
    const o = opened([]);
    const n = started().length;
    d.next();
    expect(store.onAir()).toBeUndefined(); expect(store.entry(o)?.state).toBe('skipped'); expect(started().length).toBe(n);
    expect(log.some((c) => c.kind === 'stop')).toBe(true);
    deskPlace(story('d-late'));
    expect(store.onAir()?.id).toBe('d-late');
  });
  test('5: an entry that ends is reported heard and the next is aired; a late end closes nothing', () => {
    const o = opened(['d-1']);
    d.documentEnded('rs:' + o, true);
    expect(store.entry(o)?.state).toBe('heard'); expect(store.entry(o)?.ended?.outcome).toBe('finished');
    expect(store.onAir()?.id).toBe('d-1'); expect(started().at(-1)).toBe('d-1');
    d.documentEnded('rs:' + o, true);                       // a late end of a document that was replaced
    expect(store.onAir()?.id).toBe('d-1');
  });
  test('5: the end waits for the listener\'s turn', () => {
    let open = true;
    d.hooks.turnOpen = () => open;
    const o = opened(['d-1']);
    d.documentEnded('rs:' + o, true);
    expect(store.onAir()?.id).toBe(o);                      // the listener is talking: the entry is not closed over their words
    open = false;
    d.turnEnded();
    expect(store.onAir()?.id).toBe('d-1');
  });
  test('5: an entry that cannot play is skipped once and two in a row stop the radio', () => {
    cannot = new Set(['d-1', 'd-2']);
    opened(['d-1', 'd-2', 'd-3']);
    d.next();                                               // d-1 on air; the deck cannot start it
    d.failedToStart('d-1', 'no sound');
    expect(store.entry('d-1')?.ended?.outcome).toBe('failed'); expect(store.onAir()?.id).toBe('d-2');
    d.failedToStart('d-2', 'no sound');
    expect(store.entry('d-2')?.state).toBe('skipped'); expect(store.onAir()).toBeUndefined();        // two in a row that make no sound stop it
    expect(store.entry('d-3')?.state).toBe('ahead'); expect(d.message).not.toBe(''); expect(d.wants).toBe(false);
  });
  test('5: a document that ended with no block heard says why and counts as a silent entry', () => {
    const o = opened([]);              // with something ahead the next start clears the line again
    d.documentEnded('rs:' + o, false, undefined, 'the voice made no sound');
    expect(d.message).toBe('Can’t speak: the voice made no sound'); expect(store.entry(o)?.ended?.outcome).toBe('failed');
  });
  test('7: a skipped clip ends as skipped and its sound stops', () => {
    const c = audio({ url: 'https://x.test/a.mp3', title: 'T a', show: 'Show', start: 10, end: 30, episode_id: 'ea' });
    d.playNow(c);
    expect(store.onAir()?.id).toBe(c.id);
    d.next();
    expect(store.entry(c.id)?.state).toBe('skipped'); expect(store.entry(c.id)?.ended?.outcome).toBe('skipped'); expect(log.at(-1)?.kind).toBe('stop');
  });
  test('7: a clip that ends is heard', () => {
    const c = audio({ url: 'https://x.test/a.mp3', title: 'T a', show: 'Show', start: 10, end: 30, episode_id: 'ea' });
    d.playNow(c);
    d.remoteEnded(d.generation);
    expect(store.entry(c.id)?.state).toBe('heard');
    d.remoteEnded(d.generation - 1);                         // an end of an earlier start closes nothing
  });
  test('15: queue this goes after the listener\'s last and play this keeps what is ahead', () => {
    opened(['d-1', 'd-2']);
    const a = paperStory('a', 'A'), b = paperStory('b', 'B');
    d.queue(a); d.queue(b);
    expect(store.ahead().map((e) => e.id)).toEqual([a.id, b.id, 'd-1', 'd-2']);
    const c = paperStory('c', 'C');
    d.playNow(c);
    expect(store.onAir()?.id).toBe(c.id); expect(store.ahead().map((e) => e.id)).toEqual([a.id, b.id, 'd-1', 'd-2']);
  });
  test('16: previous airs the entry before when barely begun, else restarts', () => {
    const o = opened(['d-1']);
    d.next();
    expect(store.onAir()?.id).toBe('d-1');
    expect(d.previous({ block: 0, seconds: 0.5, isRemoteAudio: false })).toBe('previous');
    expect(store.onAir()?.id).toBe(o); expect(started().at(-1)).toBe(o);
    expect(d.previous({ block: 2, seconds: 8, isRemoteAudio: false })).toBe('restart');
    expect(store.onAir()?.id).toBe(o);
    expect(previousRestartsCurrent(0, 2.9)).toBe(false); expect(previousRestartsCurrent(0, 3)).toBe(true); expect(previousRestartsCurrent(1, 0)).toBe(true);
  });
  test('16: previous on audio seeks to its start; with nothing before it, this one again', () => {
    const c = audio({ url: 'https://x.test/a.mp3', title: 'T', show: 'S', episode_id: 'e' });
    d.playNow(c);
    expect(d.previous({ block: 0, seconds: 0, isRemoteAudio: true })).toBe('seekStart');
    expect(d.previous({ block: 0, seconds: 0, isRemoteAudio: false })).toBe('restart');
  });
  test('8: with nothing on air and the radio wanted, what arrives later starts by itself; a pause stops that', () => {
    d.wants = true;
    d.hooks.spokenPaused = () => true;
    deskPlace(story('d-1'));
    expect(store.onAir()).toBeUndefined();
    d.hooks.spokenPaused = () => false;
    d.advanceIfIdle();
    expect(store.onAir()?.id).toBe('d-1');
  });
});

describe('one opening per start (21)', () => {
  test('the desk opening before the phone\'s began replaces it', () => {
    const phone = phoneOpening();
    store.place(phone, 'end', { by: 'phone' }); d.currentOpeningId = phone.id;
    d.applyFromDesk(placeOp(deskOpening()), 'open');
    expect(store.entry(phone.id)).toBeUndefined(); expect(store.ahead().map((e) => e.id)).toEqual(['d-open']);
    expect(store.pending.find((p) => p.op === 'remove' && p.id === phone.id)?.reason).toBe('opening_replaced_by_desk');
  });
  test('while the phone\'s is on air the desk\'s is dropped and what follows is admitted', () => {
    const o = opened([]);
    d.applyFromDesk(placeOp(deskOpening()), 'open'); d.applyFromDesk(placeOp(story('d-1')), 'open');
    expect(store.onAir()?.id).toBe(o); expect(store.ahead().map((e) => e.id)).toEqual(['d-1']);
    expect(d.droppedFromDesk.map((x) => x.reason)).toEqual(['opening_one_per_start']);
  });
  test('an opening heard ten minutes ago means no opening now, thirty-one means one', () => {
    for (const [minutes, expectOpening] of [[10, false], [31, true]] as const) {
      setup();
      const o = opened([]);
      d.documentEnded('rs:' + o, true);
      expect(store.entry(o)?.state).toBe('heard');
      const ended = Date.parse(store.entry(o)?.ended?.at as string);
      store.now = () => new Date(ended + minutes * 60000);
      d.open(phoneOpening('p-open2'));
      expect(store.sheet.entries.filter((e) => e.kind === 'opening' && e.state !== 'heard').length).toBe(expectOpening ? 1 : 0);
      d.applyFromDesk(placeOp(deskOpening('d-open2')), 'open');
      expect(store.sheet.entries.filter((e) => e.kind === 'opening' && e.id !== o).length).toBe(expectOpening ? 1 : 0);   // never two in a row
    }
  });
  test('a heard opening with no time on it counts as recent', () => {
    const o = opened([]);
    store.sheet.entries.find((e) => e.id === o)!.state = 'heard';
    expect(d.openingAllowed()).toBe(false);
  });
  test('the cached opening is the opening of this start and obeys the same rule', () => {
    const cached: Entry = { ...phoneOpening('cache-1'), text: 'Good evening. Written ahead.' };
    d.open(cached);
    expect(store.onAir()?.id).toBe('cache-1');
    d.applyFromDesk(placeOp(deskOpening()), 'open'); d.applyFromDesk(placeOp(story('d-1')), 'open');
    expect(store.entry('d-open')).toBeUndefined(); expect(store.sheet.entries.filter((e) => e.kind === 'opening').length).toBe(1);
    d.documentEnded('rs:cache-1', true);
    const ended = Date.parse(store.entry('cache-1')?.ended?.at as string);
    store.now = () => new Date(ended + 600000);
    d.open({ ...cached, id: 'cache-2' });
    expect(store.entry('cache-2')).toBeUndefined();
    d.open(story('d-first', { placed_by: 'phone' }), false);      // an entry that is not an opening is never refused by it
    expect(store.entry('d-first')).toBeDefined();
  });
  test('Play without a launch opening still follows an ended opening with the next entry untouched', () => {
    store.place(deskOpening('d-open'), 'end', { by: 'desk', reason: 'test' }); store.place(story('d-1'), 'end', { by: 'desk', reason: 'test' });
    d.advanceIfIdle();
    expect(store.onAir()?.id).toBe('d-open');
    d.documentEnded('rs:d-open', true);
    expect(store.onAir()?.id).toBe('d-1');
  });
  test('an entry launching the station (not an opening) makes the desk\'s live opening a transition placed next', () => {
    d.open(story('p-first', { placed_by: 'phone' }), false);
    const got = d.admitDesk(placeOp(deskOpening()), 'open');
    expect(got?.entry?.kind).toBe('transition'); expect(got?.at).toBe('next'); expect(got?.entry?.title).toBe('');
  });
  test('a desk story with a title already on the sheet is dropped, and the reason kept', () => {
    opened([]);
    deskPlace(story('d-1', { title: 'Harbour' }));
    expect(d.applyFromDesk(placeOp(story('d-2', { title: 'harbour' })), 'extend')).toBeUndefined();
    expect(d.droppedFromDesk).toEqual([{ id: 'd-2', reason: 'title_already_on_sheet' }]);
  });
});

describe('transitions (22)', () => {
  const transition = () => story('d-t', { kind: 'transition', text: 'And now.', title: '', between: { after: 'd-a', before: 'd-b' } });
  test('a transition is not an up next row and plays between its neighbours', () => {
    opened([]);
    deskPlace(story('d-a')); deskPlace(transition()); deskPlace(story('d-b'));
    d.next();                                                 // d-a on air
    expect(store.onAir()?.id).toBe('d-a');
    d.documentEnded('rs:d-a', true);
    expect(store.onAir()?.id).toBe('d-t');                    // heard to its end: the transition plays
    d.documentEnded('rs:d-t', true);
    expect(store.onAir()?.id).toBe('d-b');
  });
  test('a skipped past neighbour drops its transition', () => {
    opened([]);
    deskPlace(story('d-a')); deskPlace(transition()); deskPlace(story('d-b'));
    d.next(); d.next();                                       // d-a on air, then skipped: its transition is orphaned
    expect(store.onAir()?.id).toBe('d-b'); expect(store.entry('d-t')).toBeUndefined();
    expect(d.droppedFromDesk.map((x) => x.reason)).toEqual(['transition_orphaned']);
    const rm = store.pending.find((p) => p.op === 'remove' && p.id === 'd-t');
    expect(rm?.by).toBe('phone'); expect(rm?.reason).toBe('transition_orphaned');
  });
});

describe('answers keep the place (the answer cuts, then hands back)', () => {
  test('an answer over a cut story puts the rest back next, from the start of its sentence', () => {
    d.hooks.spokenRemainder = () => 'The rest of the story.';
    const o = opened(['d-1']);
    const ans: Entry = { id: 'ans-1', kind: 'answer', placed_by: 'phone', state: 'ahead', title: 'Answer', text: 'Here is the answer.', est_seconds: 5, revision: 0 };
    d.playNow(ans);
    expect(store.onAir()?.id).toBe('ans-1'); expect(store.entry(o)?.ended?.outcome).toBe('cut');
    expect(d.remembered?.text).toBe('The rest of the story.');
    d.documentEnded('rs:ans-1', true);
    expect(store.onAir()?.text).toBe('The rest of the story.');         // the rest goes back on air before what was ahead
    expect(store.ahead().map((e) => e.id)).toEqual(['d-1']);
  });
  test('asking for something else drops what was kept', () => {
    d.hooks.spokenRemainder = () => 'The rest.';
    opened([]);
    d.playNow({ id: 'ans-1', kind: 'answer', placed_by: 'phone', state: 'ahead', title: 'A', text: 'Yes.', est_seconds: 1, revision: 0 });
    expect(d.remembered).toBeDefined();
    d.next();
    expect(d.remembered).toBeUndefined();
  });
});

describe('honest reports (24)', () => {
  test('sign-out leaves the air as left by the phone at the true position, and clears ahead by the phone', () => {
    const o = opened(['d-1']);
    const before = store.pending.length;
    d.stopAll({ block: 2, seconds: 31.5 });
    const ops = store.pending.slice(before);
    expect(ops.some((p) => p.op === 'skip')).toBe(false);
    const left = ops.find((p) => p.op === 'aired') as Op;
    expect([left.id, left.by, left.state, left.ended?.outcome, left.ended?.last_block, left.position_seconds]).toEqual([o, 'phone', 'heard', 'left', 2, 31.5]);
    expect(ops.find((p) => p.op === 'clear')?.by).toBe('phone');
    expect(log.at(-1)?.kind).toBe('stop');
  });
  test('sign-out of an entry that never sounded is failed, not left', () => {
    const o = opened([]);
    d.stopAll(undefined, true);
    expect(store.entry(o)?.ended?.outcome).toBe('failed');
  });
  test('a natural end carries the final block and position', () => {
    const o = opened([]);
    d.documentEnded('rs:' + o, true, { block: 3, seconds: 42 });
    const op = store.pending.filter((p) => p.op === 'aired').at(-1) as Op;
    expect([op.id, op.by, op.state, op.ended?.outcome, op.ended?.last_block, op.position_seconds]).toEqual([o, 'phone', 'heard', 'finished', 3, 42]);
  });
  test('a listener skip carries the block being spoken and the seconds in', () => {
    opened([]);
    d.next({ block: 1, seconds: 7.5 });
    const op = store.pending.filter((p) => p.op === 'skip').at(-1) as Op;
    expect([op.by, op.ended?.outcome, op.ended?.last_block, op.position_seconds]).toEqual(['listener', 'skipped', 1, 7.5]);
  });
  test('play now reports where it cut the entry on air', () => {
    const o = opened([]);
    d.playNow(paperStory('r', 'R'), undefined, { block: 2, seconds: 19 });
    const op = store.pending.filter((p) => p.op === 'place').at(-1) as Op;
    expect([op.by, op.ended?.outcome, op.ended?.last_block, op.position_seconds]).toEqual(['listener', 'cut', 2, 19]);
    expect(store.entry(o)?.ended?.outcome).toBe('cut');
  });
  test('previous is a previous op by the listener and no heard-skipped report', () => {
    opened(['d-1']); d.next();
    d.previous({ block: 0, seconds: 0.5, isRemoteAudio: false });
    expect(store.pending.filter((p) => p.op === 'previous').at(-1)?.by).toBe('listener');
    expect(store.pending.find((p) => p.op === 'aired' && p.state === 'heard' && p.ended?.outcome === 'skipped')).toBeUndefined();
  });
  test('a too long address says so in plain words and places nothing', () => {
    d.queue({ ...paperStory('x', 'P'), source: { ref: 'url:https://x.test/' + 'b'.repeat(2200), label: 'x' } });
    expect(d.message).toBe('That address is too long to add.');
  });
});

describe('what each kind of entry starts with', () => {
  test('spoken kinds speak their own words; a page speaks the words held for it, else loads its story or its address', () => {
    expect(plan(story('a'))).toEqual({ kind: 'speak', document: 'rs:a', paragraphs: ['Words.'] });
    const pg = paperStory('route-1@v2', 'T');
    expect(plan(pg)).toEqual({ kind: 'story', route: 'route-1', title: 'T' });
    expect(plan(pg, { title: 'T', paragraphs: ['one', 'two'] })).toEqual({ kind: 'speak', document: 'rs:' + pg.id, paragraphs: ['one', 'two'] });
    expect(plan({ ...pg, source: { ref: 'url:https://x.test/p', label: 'x' } }).kind).toBe('webPage');
    expect(plan({ ...pg, source: { ref: 'weird', label: 'x' } }).kind).toBe('cannot');
  });
  test('a book reads from its block; a take fetches its audio with its lead-in; a clip carries its lead-in and range', () => {
    expect(plan({ ...story('b'), kind: 'document', source: { ref: 'doc:bk1', label: 'B', start: 3 } })).toEqual({ kind: 'book', id: 'bk1', at: 3 });
    expect(plan({ ...story('t'), kind: 'take', source: { ref: 'take:t1', label: 'voice-aa' }, text: 'A take.' })).toEqual({ kind: 'take', id: 't1', lead: 'A take.' });
    const c = { ...audio({ url: 'https://x.test/a.mp3', title: 'T', show: 'S', start: 10, end: 30, episode_id: 'e' }, 'clip-1'), text: 'From a podcast.' };
    const p = plan(c, undefined, audioSpec(c));
    expect(p).toEqual({ kind: 'audio', url: 'https://x.test/a.mp3', title: 'T', show: 'S', start: 10, end: 30, lead: 'From a podcast.', clipId: 'clip-1' });
  });
  test('an episode from Files is resolved to the phone\'s copy', () => {
    const e: Entry = { id: 'f', kind: 'episode', placed_by: 'listener', state: 'ahead', title: 'F', est_seconds: 0, revision: 0, source: { ref: 'file:docs/a.mp3', label: 'Files' } };
    expect(plan(e)).toEqual({ kind: 'file', address: 'file:docs/a.mp3', title: 'F' });
  });
  test('the session retry delays are 1, 2, 4, 8, 15 and then 15 again', () => expect([1, 2, 3, 4, 5, 6, 9].map(retryDelay)).toEqual([1, 2, 4, 8, 15, 15, 15]));
  test('a clip card carries the episode it was cut from, so it can open the episode', () => {
    const c = audio({ url: 'https://x.test/a.mp3', title: 'T', show: 'S', start: 5, end: 9, episode_id: 'pod:feed/ep1' }, 'clip-abc');
    expect(c.source?.ref).toBe('clip:clip-abc#pod:feed/ep1'); expect(c.source?.clip_id).toBe('clip-abc');
    expect(audioSpec(c)?.episode_id).toBe('pod:feed/ep1');
    expect(audioSpec(audio({ url: 'https://x.test/a.mp3', title: 'T', show: 'S', start: 5, end: 9 }, 'clip-abc'))?.episode_id).toBeUndefined();
  });
});

describe('a message that is not a failure to speak goes by itself (review B2-4)', () => {
  test('the too-long-address line is gone eight seconds later, the speech failure stays', () => {
    let t = Date.parse('2023-11-14T22:00:00Z');
    store = new RunSheetStore({ now: () => new Date(t) }); d = new Director(store); d.onCommand = () => undefined;
    d.queue({ ...paperStory('x', 'P'), source: { ref: 'url:https://x.test/' + 'b'.repeat(2200), label: 'x' } });
    expect(d.visibleMessage()).toBe('That address is too long to add.');
    t += 9_000; expect(d.visibleMessage()).toBe('');
    d.message = 'Can’t speak: no sound'; d.messageAt = 0; t += 60_000;
    expect(d.visibleMessage()).toBe('Can’t speak: no sound');
  });
});
