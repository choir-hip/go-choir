// The radio's sentence, from the place native reports (docs/design/sentence-follow-in-shared-code-2026-11-28.md). The failure list is that file's (d), F1 to F23: each test names its number.
// Every function here is pure, so the tests only pass values in.
import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { EMPTY_STATE, readState } from './deck-native.ts';
import { digestOf, followOf, fractionOf, locate, NO_FOLLOW, nowOf, owns, sentenceAt, sentencesOf, unitOf, type Air, type Place } from './radio-follow.ts';

const TEXT = 'First sentence here. Second one follows it. Third closes.\n\nA new paragraph begins now. It has two sentences.';
const at = (s: string, from = 0) => { const start = TEXT.indexOf(s, from); return { start, length: s.length }; };
const place = (over: Partial<Place> = {}): Place => ({ entry: 'e1', digest: digestOf(TEXT), ...at('First sentence here. Second one follows it. Third closes.'), seconds: 0, total: 0, ...over });
const ds = (over: Record<string, unknown> = {}, p: Partial<Place> = {}) => ({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: place(p), ...over });
const air = (over: Partial<Air> = {}): Air => ({ id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: TEXT, ...over });
const blocks = (xs: [string, string][]) => ({ ids: xs.map((x) => x[0]).join('\n'), texts: xs.map((x) => x[1]).join('\u001F') });
const PAGE = blocks([['b1', 'An unrelated opening line, with no match.'], ['b2', 'First sentence here. Second one follows it. Third closes.'], ['b3', 'A new paragraph begins now. It has two sentences.']]);
const follow = (d: unknown, a: Air, address = 'pub:/p/x', pg = PAGE) => followOf(d, a, address, pg.ids, pg.texts);

describe('the digest (the same on the iPhone, on Android and here)', () => {
  test('length, then a 32-bit FNV-1a over UTF-16 code units', () => {
    expect(digestOf('')).toBe('0:811c9dc5');
    expect(digestOf('a')).toBe('1:e40c292c');
    expect(digestOf('foobar')).toBe('6:bf9cf968');              // the published FNV-1a test vector
    expect(digestOf('café 😀')).toBe('7:' + digestOf('café 😀').slice(2));  // an emoji is two code units: the length says 7
    expect(digestOf('café 😀')).toMatch(/^7:[0-9a-f]{8}$/);
  });
  test('the pinned value for text with accents and an emoji, which Swift and Kotlin hold to the same string', () => {
    expect(digestOf('café 😀')).toBe('7:' + PINNED_NON_ASCII);
  });
});
const PINNED_NON_ASCII = 'f1b38d91';

describe('sentences and the place in a unit', () => {
  test('F1 a seek back inside the unit: the sentence goes back, nothing is held between calls', () => {
    const s = sentencesOf('One two three. Four five six. Seven eight nine.');
    expect(sentenceAt(s, 0.9)).toBe(2);
    expect(sentenceAt(s, 0.1)).toBe(0);
    expect(sentenceAt(s, 0.9)).toBe(2);
  });
  test('F2 speed does not matter (media time over media time); no total gives the first sentence and does not throw', () => {
    const s = sentencesOf('One two three. Four five six. Seven eight nine.');
    expect(sentenceAt(s, fractionOf(place({ seconds: 5, total: 10 })))).toBe(sentenceAt(s, fractionOf(place({ seconds: 2.5, total: 5 }))));
    expect(fractionOf(place({ seconds: 4, total: 0 }))).toBe(0);
    expect(sentenceAt(s, fractionOf(place({ seconds: 4, total: 0 })))).toBe(0);
  });
  test('F12 audio longer than the estimate keeps the last sentence lit; NaN, negative and huge are clamped', () => {
    const s = sentencesOf('One two three. Four five six. Seven eight nine.');
    for (const f of [1, 2, 99, Infinity]) expect(sentenceAt(s, f)).toBe(2);
    for (const f of [NaN, -1, -Infinity]) expect(sentenceAt(s, f)).toBe(0);
    expect(sentenceAt(s, fractionOf(place({ seconds: 99, total: 10 })))).toBe(2);
    expect(sentenceAt(s, fractionOf(place({ seconds: Number.NaN, total: 10 })))).toBe(0);
  });
  test('F22 a negative total (ExoPlayer\'s unknown duration) is zero: the first sentence', () => {
    expect(fractionOf(place({ seconds: 4, total: -9223372036854775 }))).toBe(0);
  });
  test('F11 the two splitters agree: the reader\'s rule, with its cases', () => {
    expect(sentencesOf('Dr. Smith went home. He slept.')).toEqual(['Dr. Smith went home.', 'He slept.']);
    expect(sentencesOf('It cost 3.5 dollars. Then more.')).toEqual(['It cost 3.5 dollars.', 'Then more.']);
    expect(sentencesOf('He said “Stop.” Then left.')).toEqual(['He said “Stop.”', 'Then left.']);
    expect(sentencesOf('Wait… Then go.')).toEqual(['Wait…', 'Then go.']);
    expect(sentencesOf('no stop at all')).toEqual(['no stop at all']);
    expect(sentencesOf('   ')).toEqual([]);
    expect(sentencesOf('')).toEqual([]);
  });
});

describe('the unit, by range, from the text shared code handed over', () => {
  test('F19 a duplicated sentence is lit at its range, not at its first match; a changed text gives none; a range past the end gives none', () => {
    const dup = 'Same words here. Other words. Same words here.';
    const second = { start: dup.lastIndexOf('Same words here.'), length: 'Same words here.'.length };
    const p = { entry: 'e1', digest: digestOf(dup), ...second, seconds: 0, total: 0 };
    expect(unitOf(p, dup)).toBe('Same words here.');
    expect(dup.indexOf(unitOf(p, dup))).toBe(0);                                // the text alone would find the first; the range is the second
    expect(unitOf(p, dup + ' More.')).toBe('');                                // revised after the hand-over: the digest differs
    expect(unitOf({ ...p, start: dup.length - 3 }, dup)).toBe('');
    expect(unitOf({ ...p, start: -1 }, dup)).toBe('');
    expect(unitOf({ ...p, length: 0 }, dup)).toBe('');
    expect(unitOf({ ...p, length: 1.5 }, dup)).toBe('');
  });
  test('the digest and the range are the entry\'s own text, so a digest of other words lights nothing', () => {
    expect(unitOf(place({ digest: digestOf('other') }), TEXT)).toBe('');
    expect(unitOf(place(), TEXT)).toBe('First sentence here. Second one follows it. Third closes.');
  });
});

describe('who owns the air', () => {
  test('F3 Skip: the old place arrives with the new ds.item', () => {
    expect(owns(ds({ item: 'e2' }), air({ id: 'e2', handedId: 'e2' }), ['playing'])).toBe(false);
    expect(owns(ds(), air(), ['playing'])).toBe(true);
  });
  test('F4 another entry of the same story: place A with item A and B on air gives none, and place A with item B gives none', () => {
    expect(follow(ds(), air({ id: 'e2', handedId: 'e2' })).blockId).toBe('');
    expect(follow(ds({ item: 'e2' }), air({ id: 'e2', handedId: 'e2' })).blockId).toBe('');
    expect(follow(ds(), air({ handedId: 'e2' })).blockId).toBe('');            // the text handed is another entry's
  });
  test('F5 pause and a held session light nothing in the reader (parity with Swift)', () => {
    expect(follow(ds({ why: 'paused_listener' }), air()).blockId).toBe('');
    expect(follow(ds({ why: 'held_session' }), air()).blockId).toBe('');
    expect(follow(ds({ why: 'paused_system' }), air()).blockId).toBe('');
    expect(follow(ds(), air()).blockId).toBe('b2');
  });
  test('F7 stopped, wiped or empty: no item, and an empty entry never matches an empty id', () => {
    expect(owns(ds({ item: '', why: 'stopped' }, { entry: '' }), air({ id: '', handedId: '' }), ['playing'])).toBe(false);
    expect(follow(ds({ item: '', why: 'empty' }, { entry: '' }), air({ id: '', handedId: '' })).blockId).toBe('');
    expect(follow({ ...EMPTY_STATE }, air()).blockId).toBe('');
  });
  test('F14 the station\'s own words (no address), a unit under 6 characters and an empty unit light nothing in the reader', () => {
    expect(follow(ds(), air({ address: '' })).blockId).toBe('');
    const short = 'Hi. Ok.';
    expect(follow({ ...ds(), place: { entry: 'e1', digest: digestOf(short), start: 0, length: 3, seconds: 0, total: 0 } }, air({ handedText: short })).blockId).toBe('');
    expect(follow({ ...ds(), place: { entry: 'e1', digest: digestOf(''), start: 0, length: 0, seconds: 0, total: 0 } }, air({ handedText: '' })).blockId).toBe('');
  });
  test('F16 after a sign-out wipe (the deck\'s state is empty) the follow is none', () => {
    expect(follow(EMPTY_STATE, air())).toEqual(NO_FOLLOW);
    expect(follow(undefined, air())).toEqual(NO_FOLLOW);
    expect(follow('garbage', air())).toEqual(NO_FOLLOW);
  });
});

describe('the reader\'s follow', () => {
  test('lights the block and sentence of the page that hold the unit, at the place in it', () => {
    expect(follow(ds({}, { seconds: 0, total: 10 }), air())).toEqual({ address: 'pub:/p/x', blockId: 'b2', sentence: 0 });
    expect(follow(ds({}, { seconds: 4.5, total: 10 }), air()).sentence).toBe(1);
    expect(follow(ds({}, { seconds: 9.9, total: 10 }), air()).sentence).toBe(2);
  });
  test('F6 and F15 the same arguments give the same value (purity), and an old ds against a changed on-air entry gives none', () => {
    const a = follow(ds({}, { seconds: 4.5, total: 10 }), air()), b = follow(ds({}, { seconds: 4.5, total: 10 }), air());
    expect(a).toEqual(b);
    expect(follow(ds({}, { seconds: 4.5, total: 10 }), air({ id: 'e9', handedId: 'e9' }))).toEqual(NO_FOLLOW);
  });
  test('F8 the page was edited: a moved sentence gives the new index, a deleted one gives none (not the old block)', () => {
    const moved = blocks([['b1', 'First sentence here. Second one follows it. Third closes.']]);
    expect(follow(ds({}, { seconds: 4.5, total: 10 }), air(), 'pub:/p/x', moved).blockId).toBe('b1');
    const reordered = blocks([['b9', 'Something new first. First sentence here. Second one follows it. Third closes.']]);
    expect(follow(ds({}, { seconds: 4.5, total: 10 }), air(), 'pub:/p/x', reordered).sentence).toBe(2);
    expect(follow(ds(), air(), 'pub:/p/x', blocks([['b1', 'Nothing alike in this page at all.']]))).toEqual(NO_FOLLOW);
  });
  test('the page in front must be the page on air', () => {
    expect(follow(ds(), air(), 'pub:/other')).toEqual(NO_FOLLOW);
    expect(follow(ds(), air(), '')).toEqual(NO_FOLLOW);
  });
  test('F10 a sentence in two blocks lights the first (no memory, no tie-break); this pins the limit', () => {
    const two = blocks([['b1', 'First sentence here. Second one follows it. Third closes.'], ['b2', 'First sentence here. Second one follows it. Third closes.']]);
    expect(follow(ds(), air(), 'pub:/p/x', two).blockId).toBe('b1');
  });
  test('F13 a unit that begins in the middle of a sentence is found by the page sentence that holds its head', () => {
    const tail = { start: TEXT.indexOf('Second one follows it.'), length: 'Second one follows it. Third closes.'.length };
    expect(follow(ds({}, tail), air())).toEqual({ address: 'pub:/p/x', blockId: 'b2', sentence: 1 });
    expect(locate('Second one follows it.', [{ id: 'b2', text: 'First sentence here. Second one follows it. Third closes.' }])).toEqual({ block: 'b2', sentence: 1 });
    expect(locate('hello', [{ id: 'b', text: 'hello there' }])).toBeUndefined();     // under six characters
  });
  test('a very long block is one sentence, as the reader numbers it', () => {
    const long = 'Word one here. ' + 'x'.repeat(20001);
    expect(locate('Word one here.', [{ id: 'b', text: long }])).toEqual({ block: 'b', sentence: 0 });
  });
});

describe('the radio page: the sentence and the progress, from the same place', () => {
  const now = (d: unknown, a = air()) => nowOf(d, a);
  test('the sentence is the one at the place, in the unit; progress is the unit\'s start plus its share, over the entry\'s length', () => {
    const r = now(ds({}, { seconds: 4.5, total: 10 }));
    expect(r.sentence).toBe('Second one follows it.');
    const p = place();
    expect(r.progress).toBeCloseTo((p.start + 0.45 * p.length) / TEXT.length, 6);
  });
  test('the second unit counts from where it begins in the whole text, not from its own start', () => {
    const second = at('A new paragraph begins now. It has two sentences.');
    const r = now(ds({}, { ...second, seconds: 0, total: 10 }));
    expect(r.sentence).toBe('A new paragraph begins now.');
    expect(r.progress).toBeCloseTo(second.start / TEXT.length, 6);
  });
  test('it still shows while the listener has paused (the entry is on air and the place stands), and not while another entry is', () => {
    expect(now(ds({ why: 'paused_listener' }, { seconds: 0, total: 10 })).sentence).toBe('First sentence here.');
    expect(now(ds({}, { seconds: 0, total: 10 }), air({ id: 'e2', handedId: 'e2' }))).toEqual({ sentence: '', progress: 0 });
    expect(now(ds({ why: 'stopped' }), air())).toEqual({ sentence: '', progress: 0 });
    expect(now(undefined)).toEqual({ sentence: '', progress: 0 });
  });
  test('an entry with no address still shows its sentence (the station\'s own words)', () => {
    expect(now(ds({}, { seconds: 0, total: 10 }), air({ address: '' })).sentence).toBe('First sentence here.');
  });
  test('progress stays inside 0 to 1 whatever the seconds say', () => {
    expect(now(ds({}, { seconds: 999, total: 10 })).progress).toBeLessThanOrEqual(1);
    expect(now(ds({}, { seconds: -5, total: 10 })).progress).toBeGreaterThanOrEqual(0);
  });
});

describe('the deck state carries the place', () => {
  test('readState reads it, defaulting every field, and drops a wrong-typed one', () => {
    expect(readState({ why: 'playing', item: 'e1' }).place).toEqual({ entry: '', digest: '', start: 0, length: 0, seconds: 0, total: 0 });
    expect(readState({ why: 'playing', item: 'e1', place: { entry: 'e1', digest: '3:ab', start: 4, length: 9, seconds: 1.5, total: 8 } }).place).toEqual({ entry: 'e1', digest: '3:ab', start: 4, length: 9, seconds: 1.5, total: 8 });
    expect(readState({ why: 'playing', place: { entry: 5, start: 'x', total: NaN } }).place).toEqual({ entry: '', digest: '', start: 0, length: 0, seconds: 0, total: 0 });
    expect(EMPTY_STATE.place.entry).toBe('');
  });
});

describe('F15 and F23 the module holds nothing, and the resources ask again on every value they read', () => {
  const here = new URL('.', import.meta.url).pathname;
  const src = readFileSync(`${here}radio-follow.ts`, 'utf8').replace(/^\s*\/\/.*$/gm, '').replace(/\/\*[\s\S]*?\*\//g, '');
  const contracts = readdirSync(here).filter((f) => f.endsWith('.contract') && !f.includes('.test.')).map((f) => readFileSync(`${here}${f}`, 'utf8')).join('\n');
  test('F15 no module-level variable that can change, and no clock', () => {
    expect(src).not.toMatch(/^(let|var)\s/m);
    expect(src).not.toMatch(/Date\.now|performance\.now|new Date|Math\.random/);
    expect(src).not.toMatch(/^import .*\b(api|storage|clock)\b/m);
  });
  test('F21 the reader\'s follow lists every value it reads: the deck state, the on-air entry with its address and its handed id and text, and the page\'s address, ids and texts', () => {
    const line = contracts.split('\n').find((l) => /resource rfollow\b/.test(l)) ?? '';
    expect(line).toContain('readerFollowShared(ds, rair,');
    expect(line).toContain('rdoc.address');
    expect(line).toMatch(/rdoc\.blocks\.map\(b => b\.id\)/);
    expect(line).toMatch(/rdoc\.blocks\.map\(b => b\.text\)/);
    expect(contracts).not.toMatch(/resource rfollow = readerFollow\(\)/);      // the native op with no arguments is gone
  });
  test('F23 the on-air entry is asked again on the beat (posKey) and on every pulse; the page\'s sentence is asked with the deck state and that entry', () => {
    const air = contracts.split('\n').find((l) => /resource rair\b/.test(l)) ?? '';
    expect(air).toContain('radioAir(rpulse.tick, posKey)');
    const facts = contracts.split('\n').find((l) => /resource rpfacts\b/.test(l)) ?? '';
    expect(facts).toContain('radioPageFacts(rpulse.tick, posKey, ds, rair)');
  });
});
