// The sentence the radio is saying and where it is on the page (radio-follow.ts). Ported in meaning from BarStateTests (sentences and index), PlayheadTests (the audio's place picks
// the sentence), ProgrammePlayerTests F19 (the passage is still the block), HeldPagesTests H6/H6b (the follow-along finds the block by text, never a wrong one) and ReaderWipeTests
// (what is said again after the wipe is lit again), which this replaces. The failure list, written first:
//  S1  a block's words are cut at the one splitter's stops: "Dr." and "$3.5" do not end a sentence; words with no stop are one sentence; no words are none; no sentence is empty.
//  S2  the sentence a place falls in goes by the share of characters: place 0 is the first, 0.99 the last, one sentence or none is 0, a place past the end is the last.
//  S3  progress is the block number plus the place, over the blocks, at most 1; a block number past the end is the last block; no blocks is no progress and no sentences; a place outside
//      0 to 1 or not a number is clamped, never trapped.
//  S4  the audio's place moves the sentence and nothing else does: the same facts give the same answer (wall time is no input).
//  F1  the follow finds the block holding the spoken words and the sentence holding their start; the first block that holds them.
//  F2  never a wrong block: words in no block, empty words and words under 6 characters light nothing; a page not held lights nothing; code in a block of its own is matched like any text.
//  F3  curly quotes, dashes and white space do not stop a match; the sentence numbers are the splitter's ("Dr. Smith paid $3.5 million." is one).
//  F4  the sentence said is nothing unless the radio plays and a page address is on air.
//  F5  radioPageFacts carries what native handed over, made into sentences here (the page's sentence line and fraction).
import { describe, expect, test, beforeEach } from 'bun:test';
import { followOf, locate, NO_FOLLOW, normalised, saying, sentenceAt, sentencesOf, spokenNow } from './radio-follow.ts';
import { parseBundle } from './reader-parse.ts';
import { adoptNative } from './api.ts';
import { radioPageFacts, readerFollow, wipeRadio } from './radio-bridge.ts';

const page = (content: string, address = 'pub:pub/x') => parseBundle({ publication: { title: 'Harbor', kind: 'story', dek: 'A dek.' }, artifact: { content }, nav: {} } as never, address);

describe('S1 sentences', () => {
  test('cut at stops, abbreviations and decimals do not end one', () => {
    expect(sentencesOf('One short. Two is a longer sentence here. Three.')).toEqual(['One short.', 'Two is a longer sentence here.', 'Three.']);
    expect(sentencesOf('Dr. Smith paid $3.5 million. He said "Stop." Then left.')).toEqual(['Dr. Smith paid $3.5 million.', 'He said "Stop."', 'Then left.']);
  });
  test('no stop is one sentence, no words none, white space is not a sentence', () => {
    expect(sentencesOf('A clip with no stop')).toEqual(['A clip with no stop']);
    expect(sentencesOf('')).toEqual([]);
    expect(sentencesOf('   \n  ')).toEqual([]);
    expect(sentencesOf('Two.\n\nThree.')).toEqual(['Two.', 'Three.']);
  });
});

describe('S2 the sentence a place falls in', () => {
  const s = ['One short.', 'Two is a longer sentence here.', 'Three.'];      // 10, 30 and 6 characters of 46
  test('by the share of characters', () => {
    expect(sentenceAt(s, 0)).toBe(0);
    expect(sentenceAt(s, 0.2)).toBe(0);
    expect(sentenceAt(s, 0.3)).toBe(1);
    expect(sentenceAt(s, 0.87)).toBe(2);
    expect(sentenceAt(s, 0.99)).toBe(2);
    expect(sentenceAt(s, 5)).toBe(2);
  });
  test('one sentence or none is the first', () => {
    expect(sentenceAt([], 0.5)).toBe(0);
    expect(sentenceAt(['Only.'], 0.9)).toBe(0);
  });
});

describe('S3 and S4 the radio now', () => {
  const doc = 'First sentence here. Second one follows it. Third closes.';     // 20, 22 and 13 characters of 55
  test('the audio place picks the sentence, and progress is along the programme', () => {
    const half = spokenNow({ spoken: doc, block: 1, blocks: 2, fraction: 0.5 });
    expect(half.sentences).toEqual(['First sentence here.', 'Second one follows it.', 'Third closes.']);
    expect(half.sentenceIndex).toBe(1);
    expect(half.progress).toBe(0.75);
    expect(spokenNow({ spoken: doc, block: 1, blocks: 2, fraction: 0.95 }).sentenceIndex).toBe(2);
    expect(spokenNow({ spoken: doc, block: 1, blocks: 2, fraction: 0 }).sentenceIndex).toBe(0);
    expect(spokenNow({ spoken: doc, block: 1, blocks: 2, fraction: 0.5 })).toEqual(half);       // S4: the same facts, the same answer
  });
  test('a block past the end is the last, no blocks is nothing, a bad place is clamped', () => {
    expect(spokenNow({ spoken: 'Block 99. More words.', block: 500, blocks: 100, fraction: 0.5 }).progress).toBe(0.995);
    expect(spokenNow({ spoken: 'Block 99. More words.', block: -3, blocks: 100, fraction: 0 }).progress).toBe(0);
    expect(spokenNow({ spoken: 'x. y.', block: 0, blocks: 0, fraction: 0.5 })).toEqual({ sentences: [], sentenceIndex: 0, progress: 0 });
    expect(spokenNow({ spoken: 'A. B.', block: 0, blocks: 1, fraction: 7 }).progress).toBe(1);
    expect(spokenNow({ spoken: 'A. B.', block: 0, blocks: 1, fraction: Number.NaN }).progress).toBe(0);
    expect(spokenNow({ spoken: 'Two.', block: 1, blocks: 3, fraction: 0 }).sentences).toEqual(['Two.']);     // F19: the passage being read is still the block
  });
});

describe('F1-F3 the follow-along', () => {
  const d = page('The first sentence is here. The second sentence follows it.\n\n```\nlet a = 1\n```\n\nA later paragraph about batteries.');
  test('F1 the block and the sentence by the spoken words', () => {
    const hit = d.blocks.find((b) => b.text.startsWith('The first'))!;
    const later = d.blocks.find((b) => b.text.startsWith('A later'))!;
    expect(followOf('pub:pub/x', 'The second sentence follows it.', d)).toEqual({ address: 'pub:pub/x', blockId: hit.id, sentence: 1 });
    expect(followOf('pub:pub/x', 'The first sentence is here.', d)).toEqual({ address: 'pub:pub/x', blockId: hit.id, sentence: 0 });
    expect(followOf('pub:pub/x', 'later paragraph about', d)).toEqual({ address: 'pub:pub/x', blockId: later.id, sentence: 0 });
    expect(locate('later paragraph about', d.blocks)?.block).toBe(later.id);
  });
  test('F2 never a wrong block', () => {
    expect(followOf('pub:pub/x', 'nothing like this anywhere', d)).toEqual(NO_FOLLOW);
    expect(followOf('pub:pub/x', '', d)).toEqual(NO_FOLLOW);
    expect(followOf('pub:pub/x', '  ', d)).toEqual(NO_FOLLOW);
    expect(followOf('pub:pub/x', 'The', d)).toEqual(NO_FOLLOW);
    expect(followOf('pub:pub/x', 'later', d)).toEqual(NO_FOLLOW);
    expect(followOf('pub:pub/x', 'later paragraph about', undefined)).toEqual(NO_FOLLOW);
    expect(followOf('', 'later paragraph about', d)).toEqual(NO_FOLLOW);
    const code = d.blocks.find((b) => b.kind === 'code')!;
    expect(locate('let a = 1', d.blocks)?.block).toBe(code.id);
  });
  test('F3 quotes, dashes and white space are not a difference; the sentences are the splitter\'s', () => {
    const t = page('Dr. Smith paid $3.5 million. He said “Stop.” Then left — quickly.');
    expect(followOf('a', 'Smith paid $3.5 million.', t).sentence).toBe(0);
    expect(followOf('a', 'He said "Stop."', t).sentence).toBe(1);
    expect(followOf('a', 'Then left. ', t)).toEqual(NO_FOLLOW);              // "Then left." is not in the page's words: "Then left — quickly."
    expect(followOf('a', 'Then   left - quickly.', t).sentence).toBe(2);
    expect(normalised('  It’s  “A” b\n')).toBe('it\'s "a" b');
  });
});

describe('F4 the sentence said', () => {
  const now = spokenNow({ spoken: 'One. Two. Three.', block: 0, blocks: 1, fraction: 0.5 });
  test('only while it plays and a page is on air', () => {
    expect(saying(true, 'pub:a', now)).toBe('Two.');
    expect(saying(false, 'pub:a', now)).toBe('');
    expect(saying(true, '', now)).toBe('');
    expect(saying(true, 'pub:a', spokenNow({ spoken: '', block: 0, blocks: 1, fraction: 0 }))).toBe('');
  });
});

describe('F5 radioPageFacts makes the sentences from the facts', () => {
  let facts: Record<string, unknown> = {};
  const native = { available: true, later: async (r: { op: string }) => (r.op === 'radioFacts' ? { at: 1_700_000_000_000, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [], foreground: true, ...facts } : {}) };
  beforeEach(async () => { await wipeRadio(undefined); adoptNative(native as never, 'test'); facts = {}; });
  test('the sentences, the one said and the progress', async () => {
    facts = { spoken: 'First sentence here. Second one follows it. Third closes.', block: 1, blocks: 2, blockFraction: 0.5 };
    const f = await radioPageFacts(undefined, native as never);
    expect(f.sentences).toEqual(['First sentence here.', 'Second one follows it.', 'Third closes.']);
    expect(f.sentenceIndex).toBe(1);
    expect(f.progress).toBe(0.75);
  });
  test('no facts is no sentences', async () => {
    const f = await radioPageFacts(undefined, native as never);
    expect(f.sentences).toEqual([]); expect(f.sentenceIndex).toBe(0); expect(f.progress).toBe(0);
  });
});

describe('F6 readerFollow (the source the reader asks)', () => {
  test('nothing on air lights nothing', async () => {
    await wipeRadio(undefined);
    const native = { available: true, later: async () => ({}) };
    adoptNative(native as never, 'test');
    expect(await readerFollow(undefined, native as never)).toEqual(NO_FOLLOW);
  });
});
