// The Autoradio page's view model and the ops its gestures send, under bun test. Ported from RadioPageTests.swift (the failure list is at the
// top of radio-model.ts; numbers are the same).
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { make, kindLabel, length, perform, builtOn, hello, dateLine, type PageInput } from './radio-model.ts';
import { RunSheetStore } from './runsheet-store.ts';
import { decodeSheet, emptySheet, type Entry } from './runsheet-model.ts';

const NOW = 1_700_000_000_000;   // Tue 14 Nov 2023 22:13 UTC
const entry = (id: string, kind: Entry['kind'] = 'story', o: Partial<Entry> & { outcome?: string } = {}): Entry => {
  const { outcome, ...rest } = o;
  const e: Entry = { id, kind, placed_by: 'desk', state: 'ahead', title: `Title ${id}`, est_seconds: 60, revision: 0, ...rest };
  if (['opening', 'transition', 'story', 'answer', 'result'].includes(kind)) e.text = 'Words.';
  if (outcome) e.ended = { outcome: outcome as never, at: '', last_block: 0 };
  return e;
};
const input = (entries: Entry[], o: Partial<PageInput> = {}): PageInput => ({ sheet: { ...emptySheet(), entries, on_air: entries.find((e) => e.state === 'on_air')?.id }, now: NOW, ...o });

describe('status and state', () => {
  test('1: the status line for each state', () => {
    const on = input([entry('a', 'story', { state: 'on_air' }), entry('b')], { playing: true });
    expect(make(on).statusLine).toBe('On air'); expect(make(on).state).toBe('onair');
    expect(make({ ...on, playing: false }).statusLine).toBe('Paused');
    const ready = make(input([entry('h', 'story', { state: 'heard', outcome: 'finished' }), entry('b')]));
    expect(ready.statusLine).toBe('Ready'); expect(ready.state).toBe('ready');
    expect(make(input([entry('h', 'story', { state: 'heard', outcome: 'finished' })], { writing: true })).statusLine).toBe('Writing the next stretch');
    const off = make(input([entry('a', 'story', { state: 'on_air' })], { playing: true, notice: "The desk can't be reached right now." }));
    expect(off.state).toBe('offline'); expect(off.statusLine).toBe('Offline: the desk cannot write ahead'); expect(off.playing).toBe(true);
  });
  test('1b: before the first audio it is the cold start with one play', () => {
    const p = make(input([entry('a'), entry('b')]));
    expect(p.state).toBe('cold'); expect(p.onAir).toBe(false); expect(p.playing).toBe(false); expect(p.sentence).toBe('');
    expect(p.greeting.startsWith('Good evening. Two things are ready')).toBe(true);
  });
  test('2: listening dims the card and shows the words when recognised', () => {
    const base = input([entry('a', 'story', { state: 'on_air' })], { playing: true, barState: 'listening', heardWords: 'Listening…' });
    let p = make(base);
    expect(p.statusLine).toBe('Listening'); expect(p.dim).toBe(true); expect(p.heardWords).toBe('');
    p = make({ ...base, barState: 'talking', heardWords: 'tell me about the harbour' });
    expect(p.heardWords).toBe('tell me about the harbour'); expect(p.state).toBe('listening');
    expect(make({ ...base, barState: 'idle' }).dim).toBe(false);
  });
  test('3: cannot speak names the reason with one action', () => {
    const base = input([entry('a', 'story', { state: 'on_air' })], { playing: false, message: 'Can’t speak: the voice made no sound' });
    let p = make(base);
    expect(p.statusLine).toBe('Can’t speak: the voice made no sound'); expect(p.cannotSpeak).toBe('the voice made no sound'); expect(p.action).toBe('Try again'); expect(p.state).toBe('error');
    p = make({ ...base, message: 'Could not play “X”.' });
    expect(p.cannotSpeak).toBe('Could not play “X”.');
    expect(make({ ...base, playing: true }).state).toBe('onair');          // a stale message does not hide a voice that is speaking
    const c = make(input([entry('a'), entry('b')], { message: 'Can’t speak: the audio session could not be taken' }));
    expect(c.greeting).toBe(''); expect(c.state).toBe('error'); expect(c.action).toBe('Try again');
  });
  test('4: an answer names what it goes back to', () => {
    const cut = entry('s', 'story', { state: 'skipped', title: 'The harbour barrier', outcome: 'cut' });
    const older = entry('o', 'story', { state: 'heard', title: 'Older', outcome: 'cut' });
    const p = make(input([older, cut, entry('ans', 'answer', { placed_by: 'phone', state: 'on_air', title: 'Answer' }), entry('b')], { playing: true }));
    expect(p.kindLabel).toBe('Answer'); expect(p.thenBack).toBe('Then back to: The harbour barrier');
    expect(make(input([entry('a', 'story', { state: 'on_air' })], { playing: true })).thenBack).toBe('');
  });
  test('16b: writing and wanted is one line with no Play', () => {
    const i = input([entry('h', 'story', { state: 'heard', outcome: 'finished' })], { writing: true, wants: true });
    let p = make(i);
    expect(p.statusLine).toBe('Writing the next stretch. It will start by itself.'); expect(p.canPlay).toBe(false); expect(p.onAir).toBe(false);
    p = make({ ...i, wants: false });
    expect(p.statusLine).toBe('Writing the next stretch'); expect(p.canPlay).toBe(true);
    expect(make({ ...i, writing: false }).canPlay).toBe(true);
  });
});

describe('the greeting', () => {
  test('5: the cold start greets with counts and minutes', () => {
    const open = entry('d-open', 'opening', { placed_by: 'phone', title: 'Opening', est_seconds: 120 });
    const rest = [1, 2, 3, 4, 5].map((n) => entry(`d-${n}`, 'story', { est_seconds: 96 }));
    const p = make(input([open, ...rest]));
    expect(p.state).toBe('cold'); expect(p.statusLine).toBe('Ready');
    expect(p.greeting).toBe('Good evening. Six things are ready, about ten minutes. Press play and I’ll start with the opening.');
    const after = make(input([entry('h', 'story', { state: 'heard', outcome: 'finished' }), entry('b')]));
    expect(after.greeting.startsWith('Good evening. One thing is ready')).toBe(true); expect(after.state).toBe('ready');
    expect(make(input([entry('only', 'story', { title: 'Alone', est_seconds: 30 })])).greeting).toBe('Good evening. One thing is ready, about a minute. Press play and I’ll start with Alone.');
    expect(make(input([])).greeting).toBe('');
  });
  test('5b: cold means nothing heard lately', () => {
    const ended = (at: string) => { const e = entry('h', 'story', { state: 'heard' }); e.ended = { outcome: 'finished', at, last_block: 0 }; return e; };
    let p = make(input([ended('2023-11-13T09:00:00Z'), entry('a', 'opening', { placed_by: 'phone', est_seconds: 120 }), entry('b', 'story', { est_seconds: 1680 })]));
    expect(p.state).toBe('cold'); expect(p.greeting.toLowerCase()).toContain('two things are ready'); expect(p.greeting).toContain('about thirty minutes');
    p = make(input([ended('2023-11-14T22:00:00Z'), entry('a')]));
    expect(p.state).toBe('ready');                  // heard fifteen minutes ago: not a cold start
  });
  test('5d: the day part is one function, and after midnight it is plain Hello', () => {
    const at = (h: number) => Date.UTC(2026, 10, 17, h, 53);
    expect(hello(at(0))).toBe('Hello'); expect(hello(at(5))).toBe('Good morning'); expect(hello(at(12))).toBe('Good afternoon'); expect(hello(at(18))).toBe('Good evening');
    expect(hello(at(23), 120)).toBe('Hello');          // a zone two hours east: 01:53 there
    expect(make(input([entry('a')], { now: at(0) })).greeting.startsWith('Hello. One thing is ready')).toBe(true);
  });
});

describe('rows', () => {
  test('6: a transition is neither up next nor heard', () => {
    const p = make(input([entry('th', 'transition', { state: 'heard', outcome: 'finished' }), entry('hh', 'story', { state: 'heard', outcome: 'finished' }),
      entry('t', 'transition', { title: '' }), entry('a'), entry('b')]));
    expect(p.ahead.map((r) => r.id)).toEqual(['a', 'b']); expect(p.heard.map((r) => r.id)).toEqual(['hh']);
  });
  test('7d: the entry on air is not listed in up next; all of it appears when more than five are ahead', () => {
    const p = make(input([entry('h', 'story', { state: 'heard', outcome: 'finished' }), entry('on', 'opening', { placed_by: 'phone', state: 'on_air', title: 'Opening' }), entry('a'), entry('b')]));
    expect(p.ahead.map((r) => r.id)).toEqual(['a', 'b']); expect(p.title).toBe('Opening'); expect(p.aheadMore).toBe(0);
    const nine = make(input([entry('on', 'story', { state: 'on_air' }), ...[1, 2, 3, 4, 5, 6, 7, 8].map((n) => entry(`e${n}`))]));
    expect(nine.ahead.length).toBe(8); expect(nine.aheadMore).toBe(3);
  });
  test('7: up next has every entry in order with who placed each', () => {
    const es = [1, 2, 3, 4, 5, 6, 7, 8].map((n) => entry(`e${n}`, 'story', { placed_by: n === 2 ? 'listener' : 'desk', est_seconds: n === 3 ? 0 : 200 }));
    const p = make(input(es));
    expect(p.ahead.map((r) => r.id)).toEqual(es.map((e) => e.id)); expect(p.ahead.map((r) => r.n)).toEqual([0, 1, 2, 3, 4, 5, 6, 7]); expect(p.aheadMore).toBe(3);
    expect(p.ahead[1].byWord).toBe('You'); expect(p.ahead[0].byWord).toBe('Desk');
    expect(p.ahead[0].length).toBe('3 min'); expect(p.ahead[2].length).toBe(''); expect(p.ahead[0].kind).toBe('Story');
  });
  test('7b: the empty list says what is true', () => {
    let i = input([]);
    expect(make(i).emptyNote).toBe('The desk will write more shortly.');
    expect(make({ ...i, dry: true }).emptyNote.startsWith('Nothing is coming up.')).toBe(true);
    expect(make({ ...i, dry: true, writing: true }).emptyNote).toBe('The desk is writing what comes next.');
    expect(make({ ...i, notice: 'x' }).emptyNote).toBe('Nothing is coming up yet.');
    expect(make(input([entry('a')])).emptyNote).toBe('');
  });
  test('7c: a stale transition neighbour is noted', () => expect(make(input([entry('a')], { stale: ['a'] })).ahead[0].note).toBe('Written for other neighbours'));
  test('12: heard is newest first with one word; a failed entry is not listed', () => {
    const tail = [['h1', 'heard', 'finished'], ['h2', 'skipped', 'skipped'], ['h3', 'heard', 'left'], ['h4', 'skipped', 'failed'], ['h5', 'skipped', 'cut'], ['h6', 'heard', 'finished']]
      .map(([id, st, oc]) => entry(id, 'story', { state: st as never, outcome: oc }));
    const p = make(input([...tail, entry('a')]));
    expect(p.heard.map((r) => r.id)).toEqual(['h6', 'h5', 'h3', 'h2', 'h1']);
    expect(p.heard.map((r) => r.outcome)).toEqual(['finished', 'cut', 'left', 'skipped', 'finished']);
  });
  test('12b: the date and kinds read; 15: a time of zero is no length', () => {
    expect(make(input([])).dateLine).toBe('Tue 14 Nov'); expect(dateLine(NOW, 180)).toBe('Wed 15 Nov');
    expect(kindLabel(entry('b', 'document'))).toBe('Book'); expect(kindLabel(entry('o', 'opening'))).toBe('Opening');
    expect(length(0)).toBe(''); expect(length(45)).toBe('45 sec'); expect(length(600)).toBe('10 min'); expect(length(NaN)).toBe('');
  });
});

describe('the on-air card', () => {
  test('8: why this says what the entry carries', () => {
    let e = entry('a', 'story', { state: 'on_air' });
    let p = make(input([e], { playing: true }));
    expect(p.canWhy).toBe(true); expect(p.why).toBe('The desk wrote this for you.'); expect(p.forecast).toBe('');
    e = { ...e, cites: [{ ref: 'pub:x', role: 'subject', label: 'Harbour Ledger' }] };
    expect(make(input([e])).why).toBe('The desk wrote this for you. It draws on Harbour Ledger.');
    expect(make(input([entry('l', 'story', { placed_by: 'listener', state: 'on_air' })])).why).toBe('You added this.');
    expect(make(input([entry('o', 'opening', { placed_by: 'phone', state: 'on_air' })])).why).toBe('The station opens each listening, from the front page it holds.');
    e = { ...e, forecast: { p_heard: 0.72, caused_by: 'you finished two stories on the harbour yesterday', claim_id: 'c1' } };
    p = make(input([e]));
    expect(p.why).toBe('You finished two stories on the harbour yesterday.'); expect(p.forecast).toBe('The desk forecast a 72 in 100 chance that you hear it through.');
    e = { ...e, forecast: { p_heard: 0.5, caused_by: '', claim_id: 'c1' } };
    p = make(input([e]));
    expect(p.why.startsWith('The desk wrote this')).toBe(true); expect(p.forecast).toBe('The desk forecast a 50 in 100 chance that you hear it through.');
    expect(make(input([entry('n')])).canWhy).toBe(false);      // nothing on air: no control
  });
  test('9: the sentence is the one shared code made from the deck\'s place; none when there is none', () => {
    const i = input([entry('a', 'story', { state: 'on_air', est_seconds: 100 })], { playing: true, sentence: 'Two.', progress: 0.25 });
    let p = make(i);
    expect(p.sentence).toBe('Two.');
    // no "N of M": the sentence is the one in the unit on air, so a total would change under the listener (the owner read "1 of 1" then "2 of 2" in one story as wrong)
    expect('sentenceNumber' in p).toBe(false); expect('sentenceCount' in p).toBe(false);
    expect(p.elapsed).toBe('0:25'); expect(p.remaining).toBe('-1:15'); expect(p.fraction).toBeCloseTo(0.25, 3);
    p = make({ ...i, sentence: '' });
    expect(p.sentence).toBe('');
  });
  test('10: audio has no sentence and uses its own clock', () => {
    const e = entry('p', 'episode', { state: 'on_air', title: 'Episode', source: { ref: 'episode:show', label: 'Harbour Hour', url: 'https://example.com/a.mp3' } });
    const p = make(input([e], { playing: true, remoteAudio: true, sentence: 'x', elapsedLabel: '1:12', remainingLabel: '-3:40' }));
    expect(p.sentence).toBe(''); expect(p.elapsed).toBe('1:12'); expect(p.remaining).toBe('-3:40'); expect(p.kindLabel).toBe('Podcast'); expect(p.source).toBe('Harbour Hour');
  });
  test('a take on air shows its publisher, transcript and record', () => {
    const e = entry('t', 'take', { state: 'on_air', source: { ref: 'take:take-1', label: 'voice-3fa2c1' } });
    const p = make(input([e], { takeLookup: (id) => (id === 'take-1' ? { transcript: 'what they said', record_words: 'their record' } : undefined) }));
    expect(p.takePublisher).toBe('voice-3fa2c1'); expect(p.takeTranscript).toBe('what they said'); expect(p.takeRecord).toBe('their record');
    expect(make(input([entry('t', 'take', { state: 'heard', outcome: 'finished', source: { ref: 'take:x', label: 'voice-aa' } }), entry('a')])).heard[0].publisher).toBe('voice-aa');
  });
  test('15: built on lists the stories the entry was built on; none drawn when there are none', () => {
    const e = entry('h', 'story', { state: 'on_air' });
    e.cites = [{ ref: 'pub:harbour-barrier@v2', role: 'subject', label: 'The harbour barrier passes' }, { ref: 'pub:tram-fares@v1', role: 'quoted', label: 'Tram fares change' },
      { ref: 'pub:harbour-barrier@v3', role: 'mentioned', label: 'again' }, { ref: 'clip:x', role: 'subject', label: 'not a story' }];
    e.source = { ref: 'pub:harbour-barrier', label: 'The harbour barrier passes' };
    const b = make(input([e])).builtOn;
    expect(b.map((x) => x.title)).toEqual(['The harbour barrier passes', 'Tram fares change']);
    expect(b.map((x) => x.address)).toEqual(['pub:harbour-barrier', 'pub:tram-fares']); expect(b.map((x) => x.n)).toEqual([0, 1]);
    expect(make(input([entry('o', 'opening', { placed_by: 'phone', state: 'on_air' })])).builtOn).toEqual([]);
    const t = entry('t', 'story', { state: 'on_air' }); t.cites = [{ ref: 'pub:x', role: 'subject', label: '' }];
    expect(make(input([t])).builtOn[0].title).toBe('x');
    const heard = entry('hh', 'story', { state: 'heard', outcome: 'finished' }); heard.cites = e.cites;
    expect(make(input([heard, entry('a')])).heard[0].builtOn.length).toBe(2);
  });
  test('15c: a desk story title opens its source and a version difference is noted', () => {
    const e = entry('s', 'story', { state: 'on_air', source: { ref: 'pub:harbour-barrier@v2', label: 'The harbour barrier' } });
    e.cites = [{ ref: 'pub:harbour-barrier@v2', role: 'subject', label: 'The harbour barrier' }, { ref: 'pub:tram-fares@v1', role: 'quoted', label: 'Tram fares' }];
    const p = make(input([e], { versions: { 'harbour-barrier': 3, 'tram-fares': 1 } }));
    expect(p.openAddress).toBe('pub:harbour-barrier');
    expect(p.builtOn.map((x) => x.note)).toEqual(['cited v2, now v3', '']); expect(p.builtOn.map((x) => x.role)).toEqual(['', 'quoted']);
  });
  test('15b: a clip\'s story ref reaches the entry as a cite', () => {
    const s = entry('c', 'clip', { cites: [{ ref: 'pub:harbour-barrier', role: 'subject', label: '' }] });
    expect(builtOn(s).map((x) => x.address)).toEqual(['pub:harbour-barrier']);
  });
  test('12c: the title opens the story or book', () => {
    expect(make(input([entry('s', 'page', { state: 'on_air', source: { ref: 'pub:harbour-barrier@v2', label: 'Harbour Ledger' } })])).openAddress).toBe('pub:harbour-barrier');
    expect(make(input([entry('b', 'document', { state: 'on_air', source: { ref: 'doc:bk1', label: 'A Book', start: 3 } })])).openAddress).toBe('doc:bk1');
    expect(make(input([entry('x', 'story', { state: 'on_air' })])).openAddress).toBe('');
  });
});

describe('the gestures', () => {
  const player = (es: Entry[]) => { const s = new RunSheetStore(); for (const e of es) s.place(e, 'end', { by: e.placed_by, reason: 'test' }); return s; };
  const ids = (s: RunSheetStore) => s.ahead().map((e) => e.id);
  const act = (s: RunSheetStore, g: Parameters<typeof perform>[1]) => {
    const a = perform(s.sheet, g, s.mintOpId());
    if (a.kind === 'op') return s.apply(a.op);
    if (a.kind === 'remove') return s.remove(a.id);
    if (a.kind === 'playUpcoming') return s.move(s.ahead()[a.index].id, 'now');
    if (a.kind === 'placeNow') return s.place(a.entry, 'now');
    return undefined;
  };
  test('13a: a reorder is one move op', () => {
    const s = player(['a', 'b', 'c', 'd'].map((n) => entry(n)));
    const before = s.pending.length;
    expect(act(s, { kind: 'reorder', id: 'd', before: 'b' })).toBeDefined();
    expect(ids(s)).toEqual(['a', 'd', 'b', 'c']); expect(s.pending.length).toBe(before + 1);
    const op = s.pending.at(-1)!;
    expect([op.op, op.at, op.after, op.id, op.by]).toEqual(['move', 'after', 'a', 'd', 'listener']);
    act(s, { kind: 'reorder', id: 'c', before: 'a' }); expect(ids(s)).toEqual(['c', 'a', 'd', 'b']); expect(s.pending.at(-1)?.at).toBe('next');
    act(s, { kind: 'reorder', id: 'c' }); expect(ids(s)).toEqual(['a', 'd', 'b', 'c']); expect(s.pending.at(-1)?.at).toBe('end');
    const n = s.pending.length;
    expect(act(s, { kind: 'reorder', id: 'c' })).toBeUndefined(); expect(s.pending.length).toBe(n);   // already last: nothing is sent
    expect(act(s, { kind: 'reorder', id: 'zzz', before: 'a' })).toBeUndefined();
  });
  test('13b: a swipe is one remove op', () => {
    const s = player(['a', 'b', 'c'].map((n) => entry(n)));
    act(s, { kind: 'remove', id: 'b' });
    expect(ids(s)).toEqual(['a', 'c']); expect([s.pending.at(-1)?.op, s.pending.at(-1)?.id, s.pending.at(-1)?.by]).toEqual(['remove', 'b', 'listener']);
    const n = s.pending.length;
    expect(act(s, { kind: 'remove', id: 'b' })).toBeUndefined(); expect(s.pending.length).toBe(n);
  });
  test('13c: a tap is the play now op', () => {
    const s = player(['a', 'b', 'c'].map((n) => entry(n)));
    act(s, { kind: 'playNow', id: 'c' });
    expect([s.pending.at(-1)?.op, s.pending.at(-1)?.at, s.pending.at(-1)?.id]).toEqual(['move', 'now', 'c']);
    expect(s.onAir()?.id).toBe('c'); expect(ids(s)).toEqual(['a', 'b']);
  });
  test('13d: replay from heard places a fresh copy now', () => {
    const s = player(['a', 'b'].map((n) => entry(n)));
    s.airedOnAir('a'); s.airedEnded('a', 'heard', 'finished');
    expect(s.heard()[0].id).toBe('a');
    act(s, { kind: 'replay', id: 'a' });
    const op = s.pending.at(-1)!;
    expect([op.op, op.at]).toEqual(['place', 'now']); expect(op.entry?.id).not.toBe('a'); expect(op.entry?.title).toBe('Title a');
    expect(s.onAir()?.title).toBe('Title a'); expect(s.onAir()?.state).toBe('on_air');
    expect(act(s, { kind: 'replay', id: 'b' })).toBeUndefined();   // an entry still ahead is not replayed
  });
  test('14: a gesture for an entry no longer ahead changes nothing', () => {
    const s = player(['a', 'b'].map((n) => entry(n)));
    const v = s.sheet.version;
    expect(act(s, { kind: 'playNow', id: 'gone' })).toBeUndefined(); expect(act(s, { kind: 'remove', id: 'gone' })).toBeUndefined();
    expect(s.sheet.version).toBe(v);
  });
});
