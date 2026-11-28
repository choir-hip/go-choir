// The radio's sentence, from the place native reports (docs/design/sentence-follow-in-shared-code-2026-11-28.md). The failure list is that file's (d), F1 to F23: each test names its number.
// Every function here is pure, so the tests only pass values in.
import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { EMPTY_STATE, readState } from './deck-native.ts';
import { digestOf, followOf, fractionOf, locateAt, NO_FOLLOW, nowOf, owns, sentenceAt, sentencesOf, unitOf, type Air, type Place } from './radio-follow.ts';

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
  test('F10 a sentence in two blocks that the entry has once lights neither: the order cannot say which (it was the first, by search)', () => {
    const two = blocks([['b1', 'First sentence here. Second one follows it. Third closes.'], ['b2', 'First sentence here. Second one follows it. Third closes.']]);
    expect(follow(ds(), air(), 'pub:/p/x', two).blockId).toBe('');
  });
  test('F13 a unit that begins in the middle of a sentence is found by the page sentence that holds its head', () => {
    const tail = { start: TEXT.indexOf('Second one follows it.'), length: 'Second one follows it. Third closes.'.length };
    expect(follow(ds({}, tail), air())).toEqual({ address: 'pub:/p/x', blockId: 'b2', sentence: 1 });
    const E = 'First sentence here. Second one follows it. Third closes.';
    expect(locateAt(E, E.indexOf('Second'), [{ id: 'b2', text: E }])).toEqual({ block: 'b2', sentence: 1 });
    expect(locateAt('Hi.', 0, [{ id: 'b', text: 'Hi.' }])).toBeUndefined();     // under six characters
    expect(locateAt(E, E.length + 5, [{ id: 'b2', text: E }])).toBeUndefined();               // an offset outside the text
  });
  test('a very long block is one sentence, as the reader numbers it, and the walk goes by that', () => {
    const long = 'Word one here. ' + 'x'.repeat(20001);
    expect(locateAt(long, 0, [{ id: 'b', text: long }])).toEqual({ block: 'b', sentence: 0 });
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
    expect(air).toContain('radioAir(rpulse.tick, posKey, ds.item)');
    const facts = contracts.split('\n').find((l) => /resource rpfacts\b/.test(l)) ?? '';
    expect(facts).toContain('radioPageFacts(rpulse.tick, posKey, ds, rair)');
    const mut = contracts.split('\n').find((l) => /mutation rpm\b/.test(l)) ?? '';
    expect(mut).toMatch(/refreshes rpfacts, rair, rp, rpulse/);       // what the page's facts read is asked again with them
  });
});

describe('the page position is the reported range, mapped by order and never searched for', () => {
  const P1 = 'Intro line here. Same words repeated here. Tail one.', P2 = 'Other opening text. Same words repeated here. Tail two.';
  const ENTRY = `${P1}\n\n${P2}`;
  const unit = (text: string, part: string, from = 0) => ({ start: text.indexOf(part, from), length: part.length });
  const run = (entry: string, part: string, page: [string, string][], fraction: number, from = 0, a: Partial<Air> = {}) => {
    const pg = blocks(page);
    return followOf({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: { entry: 'e1', digest: digestOf(entry), ...unit(entry, part, from), seconds: fraction * 10, total: 10 } }, { id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: entry, ...a }, 'pub:/p/x', pg.ids, pg.texts);
  };
  test('a sentence repeated in two blocks: speaking the second lights the second', () => {
    expect(run(ENTRY, P2, [['b1', P1], ['b2', P2]], 0.5, ENTRY.indexOf(P2))).toEqual({ address: 'pub:/p/x', blockId: 'b2', sentence: 1 });
    expect(run(ENTRY, P1, [['b1', P1], ['b2', P2]], 0.5)).toEqual({ address: 'pub:/p/x', blockId: 'b1', sentence: 1 });
  });
  test('the same opening 24 characters in two sentences of one block: the second is the second', () => {
    const B = 'The committee met on Monday and voted. The committee met on Monday and adjourned.';
    expect(run(B, B, [['b1', B]], 0.9)).toEqual({ address: 'pub:/p/x', blockId: 'b1', sentence: 1 });
    expect(run(B, B, [['b1', B]], 0.1).sentence).toBe(0);
  });
  test('a block the page has and the entry lacks, whose sentence is also in the entry: the counts differ, so nothing is lit (changed from round one: the occurrence rule gives these up)', () => {
    const E = 'Real paragraph start. Same words repeated here.';
    expect(run(E, E, [['x1', 'Same words repeated here. Filler one.'], ['x2', E]], 0.9)).toEqual(NO_FOLLOW);
  });
  test('two page sentences where the entry has one light nothing (the counts differ)', () => {
    const S = 'Same words repeated here.';
    expect(run(S, S, [['b1', S], ['b2', S]], 0.5)).toEqual(NO_FOLLOW);
  });
  test('a heading the page has and the entry lacks does not move the rest', () => {
    const E = 'First sentence here. Second one follows it. Third closes.';
    expect(run(E, E, [['h', 'A heading the entry lacks.'], ['b1', E]], 0.5)).toEqual({ address: 'pub:/p/x', blockId: 'b1', sentence: 1 });
  });
  test('a page that does not align with the entry lights nothing', () => {
    expect(run(ENTRY, P2, [['b1', 'Entirely different words on this page. Nothing alike at all.']], 0.5, ENTRY.indexOf(P2))).toEqual(NO_FOLLOW);
  });
  test('the radio page\'s own sentence for the repeated case is the second, by range (nowOf never searches)', () => {
    const d = { ...EMPTY_STATE, why: 'playing', item: 'e1', place: { entry: 'e1', digest: digestOf(ENTRY), ...unit(ENTRY, P2, ENTRY.indexOf(P2)), seconds: 5, total: 10 } };
    const r = nowOf(d, { id: 'e1', address: '', handedId: 'e1', handedText: ENTRY });
    expect(r.sentence).toBe('Same words repeated here.');
    expect(r.progress).toBeGreaterThan(0.6);
    const first = nowOf({ ...d, place: { ...d.place, ...unit(ENTRY, P1), seconds: 5 } }, { id: 'e1', address: '', handedId: 'e1', handedText: ENTRY });
    expect(first.sentence).toBe('Same words repeated here.');
    expect(first.progress).toBeLessThan(0.4);
  });
});

describe('skip, then the follow before the next pulse: never the previous item\'s', () => {
  const old = ds({}, { seconds: 4.5, total: 10 });                                  // the deck state of the item that was skipped (e1)
  test('the sheet has moved on (e2) and the deck state has not: nothing lit', () => {
    expect(follow(old, air({ id: 'e2', handedId: 'e2', handedText: 'Other words for the second entry.' }))).toEqual(NO_FOLLOW);
    expect(nowOf(old, air({ id: 'e2', handedId: 'e2', handedText: 'Other words for the second entry.' }))).toEqual({ sentence: '', progress: 0 });
  });
  test('the deck state has moved on (e2, no place yet) and the on-air entry has not: nothing lit', () => {
    expect(follow(ds({ item: 'e2' }, { entry: '' }), air())).toEqual(NO_FOLLOW);
    expect(follow(ds({ item: 'e2' }, { entry: 'e2' }), air())).toEqual(NO_FOLLOW);
  });
  test('both have moved on: the new item\'s own sentence', () => {
    const E2 = 'Second entry words come first. Then more words follow here.';
    const d = { ...EMPTY_STATE, why: 'playing', item: 'e2', place: { entry: 'e2', digest: digestOf(E2), start: 0, length: E2.length, seconds: 0, total: 10 } };
    expect(follow(d, air({ id: 'e2', handedId: 'e2', handedText: E2 }), 'pub:/p/x', blocks([['c1', E2]])).blockId).toBe('c1');
  });
});

describe('the occurrence rule: the k-th copy in the entry is the k-th copy on the page, when both have the same number of copies', () => {
  const A = 'Alpha opens the story here.', S = 'Same words repeated here.', B = 'Beta follows along now.', C = 'Gamma closes it all.';
  const each = (e: string[], pg: string[][], fraction: number, pick = e.length - 1) => {
    const entry = e.join(' ');
    const at = e.slice(0, pick).reduce((n, x) => n + x.length + 1, 0);
    const page = blocks(pg.map((b, i) => [`p${i + 1}`, b.join(' ')] as [string, string]));
    return followOf({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: { entry: 'e1', digest: digestOf(entry), start: at, length: e[pick].length, seconds: fraction * 10, total: 10 } }, { id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: entry }, 'pub:/p/x', page.ids, page.texts);
  };
  test('[A,S,B,S] against a page of [A,S]: speaking the second S lights nothing (the page has one copy, the entry two)', () => {
    expect(each([A, S, B, S], [[A, S]], 0.1)).toEqual(NO_FOLLOW);
  });
  test('[A,S,B,S] against the same: the second S is page sentence 3', () => {
    expect(each([A, S, B, S], [[A, S, B, S]], 0.1)).toEqual({ address: 'pub:/p/x', blockId: 'p1', sentence: 3 });
    expect(each([A, S, B, S], [[A, S, B, S]], 0.1, 1)).toEqual({ address: 'pub:/p/x', blockId: 'p1', sentence: 1 });
  });
  test('[S,A,S,B] against [S,B,S,A], speaking the second S: the page\'s second S', () => {
    expect(each([S, A, S, B], [[S, B, S, A]], 0.1, 2)).toEqual({ address: 'pub:/p/x', blockId: 'p1', sentence: 2 });
    expect(each([S, A, S, B], [[S, B], [S, A]], 0.1, 2)).toEqual({ address: 'pub:/p/x', blockId: 'p2', sentence: 0 });
  });
  test('a page-only duplicate lights nothing', () => {
    expect(each([A, S, B], [[A, S, B, S]], 0.1, 1)).toEqual(NO_FOLLOW);
  });
  test('unique sentences in a reordered page still light', () => {
    expect(each([A, B, C], [[C], [A], [B]], 0.1, 1)).toEqual({ address: 'pub:/p/x', blockId: 'p3', sentence: 0 });
  });
  test('a 3,000-sentence document lights the right sentence, and one call is quick', () => {
    const e = Array.from({ length: 3000 }, (_, i) => `Sentence number ${i} here with some words.`);
    const pg = Array.from({ length: 30 }, (_, b) => e.slice(b * 100, b * 100 + 100));
    followOf({ ...EMPTY_STATE }, {}, '', '', '');                                    // warm
    const entry = pg.map((b) => b.join(' ')).join('\n\n'), page = blocks(pg.map((b, i) => [`p${i + 1}`, b.join(' ')] as [string, string]));
    const t0 = performance.now();
    const r = followOf({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: { entry: 'e1', digest: digestOf(entry), start: entry.indexOf(e[2500]), length: e[2500].length, seconds: 1, total: 10 } }, { id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: entry }, 'pub:/p/x', page.ids, page.texts);
    const ms = performance.now() - t0;
    console.log(`followOf on 3000 sentences: ${ms.toFixed(1)} ms`);
    expect(r).toEqual({ address: 'pub:/p/x', blockId: 'p26', sentence: 0 });
    expect(ms).toBeLessThan(200);
  });
  test('a sentence longer than the reader\'s limit is one sentence on both sides, so it is found', () => {
    const long = 'Wording here. '.repeat(1600).trim();                              // over MAX_SENTENCE_CHARS: the reader counts it as one
    expect(long.length).toBeGreaterThan(20000);
    const entry = `${A}\n\n${long}`;
    const page = blocks([['p1', A], ['p2', long]]);
    const start = entry.indexOf('Wording here.', A.length + 5000);
    const r = followOf({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: { entry: 'e1', digest: digestOf(entry), start, length: 40, seconds: 0, total: 10 } }, { id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: entry }, 'pub:/p/x', page.ids, page.texts);
    expect(r).toEqual({ address: 'pub:/p/x', blockId: 'p2', sentence: 0 });
  });
});
