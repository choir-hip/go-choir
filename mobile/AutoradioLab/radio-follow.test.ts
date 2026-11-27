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
//  S5  a place at the very end of a block is capped at 0.99 (as native always capped it): progress mid-programme is never the next block's start; a block or count that is not a number is 0, never NaN.
//  S6  text after the last stop is a sentence of its own; an ellipsis, a newline with no stop, and a stop with no space after it behave as the one splitter says; a mark made of several code points counts as several characters (see the test).
//  N1  native's one answer is read whole every pulse: the document, the words, the block number and count, the place; no document is nothing.
//  N2  the answer is the entry on air exactly: the entry that was on air when it arrived is on air now, the document is `rs:<that entry>` or the story's whole address (a revision is another address), and the page shown is the
//      page on air. A different entry of the same story, another revision of the address, or another page shown lights nothing.
//  N3  an older answer never overwrites a newer one (two reads resolving out of order), a failed read clears only what is older than itself.
//  N4  the sign-out wipe clears the held answer: the page facts return no words from before it, and nothing is lit.
//  L1-L7 readerFollow through its real path (the radio, the pulse, native's answers faked, the reader's held page): playing on a held multi-block page lights its block and sentence; paused lights nothing; a page not held lights
//      nothing; after the wipe nothing is lit and the next play lights again; a document replaced mid-play, with what native speaks, is followed in its new blocks; another entry, another revision and another page shown light
//      nothing; one native read a pulse serves the page facts and the follow.
import { describe, expect, test, beforeEach } from 'bun:test';
import { followOf, locate, NO_FOLLOW, NO_SPOKEN, normalised, ownsAir, readSpoken, saying, sentenceAt, sentencesOf, spokenNow } from './radio-follow.ts';
import { parseBundle } from './reader-parse.ts';
import { adoptNative, nextGeneration, resetSeam, type Reply } from './api.ts';
import { getRadio, radioListen, radioPageFacts, radioPulse, readerFollow, wipeRadio } from './radio-bridge.ts';
import * as R from './reader.ts';

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
    expect(spokenNow({ spoken: 'A. B.', block: 0, blocks: 1, fraction: 7 }).progress).toBe(0.99);
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


describe('S5 and S6 the edges', () => {
  test('S5 the end of a block is not the next block', () => {
    expect(spokenNow({ spoken: 'A. B.', block: 0, blocks: 2, fraction: 1 }).progress).toBe(0.495);
    expect(spokenNow({ spoken: 'A. B.', block: 1, blocks: 2, fraction: 1 }).progress).toBe(0.995);
    expect(spokenNow({ spoken: 'A. B.', block: Number.NaN, blocks: 2, fraction: 0.5 }).progress).toBe(0.25);
    expect(spokenNow({ spoken: 'A. B.', block: 0, blocks: Number.NaN, fraction: 0.5 })).toEqual({ sentences: [], sentenceIndex: 0, progress: 0 });
    expect(spokenNow({ spoken: 'A. B.', block: Infinity, blocks: 4, fraction: 0 }).progress).toBe(0);
  });
  test('S6 trailing words, ellipsis, newline, no space, CJK, combining marks', () => {
    expect(sentencesOf('First. Trailing')).toEqual(['First.', 'Trailing']);
    expect(sentencesOf('Wait\u2026 then go. Done')).toEqual(['Wait\u2026', 'then go.', 'Done']);    // an ellipsis ends a sentence even before a lower-case word; a full stop before one does not
    expect(sentencesOf('Heading\nBody text here.')).toEqual(['Heading', 'Body text here.']);
    expect(sentencesOf('Version 3.5 is out. Yes.')).toEqual(['Version 3.5 is out.', 'Yes.']);
    expect(sentencesOf('\u4eca\u65e5\u306f\u6674\u308c\u3002\u660e\u65e5\u3082\u3002')).toEqual(['\u4eca\u65e5\u306f\u6674\u308c\u3002\u660e\u65e5\u3082\u3002']);    // the splitter has no CJK stop: one sentence (a highlight that does not move, not a wrong one)
    const flag = '\u{1F1EF}\u{1F1F5}', e = 'e\u0301';                         // a flag is 2 code points and one grapheme, e + accent 2 and 1
    const two = [`Some ${flag}${flag}${flag} words ${e}${e}.`, 'Short.'];       // 23 + 6 = 29 code points; by the platform's grapheme count 18 + 6 = 24
    expect(sentenceAt(two, 0.7)).toBe(0);
    expect(sentenceAt(two, 0.77)).toBe(0);                                      // by graphemes the boundary is 18/24 = 0.75 and this place would be the second sentence: marks like these can move the lit sentence early or late, only for a place between 0.75 and 0.793 here (a few percent of one block's length)
    expect(sentenceAt(two, 0.8)).toBe(1);
  });
});

describe('N1 and N2 native answers', () => {
  test('N1 the answer whole, no document is nothing', () => {
    expect(readSpoken({ document: 'rs:a', spoken: 'Words.', block: 1, blocks: 3, blockFraction: 0.5 }, 'a')).toEqual({ document: 'rs:a', entry: 'a', spoken: 'Words.', block: 1, blocks: 3, fraction: 0.5 });
    expect(readSpoken({ spoken: 'Words.' }, 'a')).toEqual(NO_SPOKEN);
  });
  const snap = (document: string, entry: string) => ({ document, entry, spoken: 'x', block: 0, blocks: 1, fraction: 0 });
  test('N2 exactly the entry on air', () => {
    const air = { id: 'e1', address: 'pub:a/x' };
    expect(ownsAir(snap('rs:e1', 'e1'), air, 'pub:a/x')).toBe(true);
    expect(ownsAir(snap('pub:a/x', 'e1'), air, 'pub:a/x')).toBe(true);
    expect(ownsAir(snap('rs:e1', 'e0'), air, 'pub:a/x')).toBe(false);                                   // another entry's answer
    expect(ownsAir(snap('rs:e0', 'e1'), air, 'pub:a/x')).toBe(false);                                   // another entry's document
    expect(ownsAir(snap('pub:a/x', 'e1'), { id: 'e1', address: 'pub:a/x@3' }, 'pub:a/x@3')).toBe(false);   // a revision is not the bare address
    expect(ownsAir(snap('pub:a/x@3', 'e1'), { id: 'e1', address: 'pub:a/x@3' }, 'pub:a/x@3')).toBe(true);
    expect(ownsAir(snap('pub:a/x@3', 'e1'), { id: 'e1', address: 'pub:a/x@3' }, 'pub:a/x')).toBe(false);   // another page shown
    expect(ownsAir(snap('', 'e1'), air, 'pub:a/x')).toBe(false);
  });
});

// The real path: the radio on air, native's answers faked (the deck and the facts), the reader's held page.
type Deck = { why: string; item: string; seq: number };
let deck: Deck, nowS: number, factCalls: number;
const P1 = 'The first sentence is here. The second sentence follows it.', P2 = 'A middle paragraph speaks next. It has two sentences too.', P3 = 'The last paragraph ends it. Nothing follows.';
const PAGE = `${P1}\n\n${P2}\n\n${P3}`;
const said = { text: P2, fraction: 0.8, document: undefined as string | undefined };
const nat = {
  available: true,
  later: async (r: { op: string; name?: string; args?: Record<string, unknown> }) => {
    if (r.op === 'radioFacts') {
      factCalls++;
      return { foreground: true, at: nowS * 1000, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [], document: said.document ?? `rs:${deck.item}`, spoken: said.text, block: 1, blocks: 3, blockFraction: said.fraction };
    }
    if (r.op === 'deck') {
      const a = r.args ?? {};
      if (r.name === 'deckPlay') { deck.item = String(a.id); deck.why = 'playing'; deck.seq += 1; }
      if (r.name === 'deckAct' && a.op === 'pause') deck.why = 'paused_listener';
      if (r.name === 'deckAct' && a.op === 'resume') deck.why = 'playing';
      return { why: deck.why, item: deck.item, block: 0, seconds: 0, duration: 240, held: [], seq: deck.seq, acked: 0, journal: [] };
    }
    return {};
  },
};
const pulse = async (): ReturnType<typeof radioPulse> => { nowS += 1; return radioPulse(undefined, nat as never, await nat.later({ op: 'deck', name: 'deckState' })); };
const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
const net = (content: () => string): R.Net => ({ pub: async (path) => reply(path.endsWith('/related') ? { links: [] } : { publication: { title: 'Harbor', kind: 'story' }, route: { bare: 'pub/x', permalink: 'pub/x/v1' }, artifact: { content: content() }, nav: { revision: 1, latest: 1 } }), authed: async () => reply({}) });
const ADDRESS = 'pub:pub/x';
async function playPage(content: () => string = () => PAGE, address = ADDRESS): Promise<R.Net> {
  const n = net(content);
  R.setHandOverForTest(null);
  await R.readerDocShared(address, 'in', false, n);
  await radioListen(undefined, nat as never, address, 'Harbor', PAGE);
  await pulse(); await pulse();
  return n;
}
const follow = (shown = ADDRESS): ReturnType<typeof readerFollow> => readerFollow(undefined, nat as never, shown);
const blocksOf = (address = ADDRESS) => R.heldDocument(address)!.blocks.filter((b) => b.text !== '');

describe('L1-L7 readerFollow', () => {
  beforeEach(async () => {
    resetSeam(); R.resetReaderForTest();
    await wipeRadio(undefined); adoptNative(nat as never, 'test');
    deck = { why: 'empty', item: '', seq: 0 }; nowS = 1_700_000_000; factCalls = 0;
    said.text = P2; said.fraction = 0.8; said.document = undefined;
  });
  test('L1 playing on a held page of three blocks lights the block being said and the sentence in it', async () => {
    await playPage();
    const b = blocksOf();
    expect(b.length).toBe(3);
    expect(await follow()).toEqual({ address: ADDRESS, blockId: b[1].id, sentence: 1 });
    said.fraction = 0.1; await pulse();
    expect(await follow()).toEqual({ address: ADDRESS, blockId: b[1].id, sentence: 0 });
    said.text = P3; said.fraction = 0.1; await pulse();
    expect(await follow()).toEqual({ address: ADDRESS, blockId: b[2].id, sentence: 0 });
  });
  test('L2 paused lights nothing, resumed lights again', async () => {
    await playPage();
    const lit = { address: ADDRESS, blockId: blocksOf()[1].id, sentence: 1 };
    expect(await follow()).toEqual(lit);
    await nat.later({ op: 'deck', name: 'deckAct', args: { op: 'pause' } }); await pulse();
    expect(await follow()).toEqual(NO_FOLLOW);
    await nat.later({ op: 'deck', name: 'deckAct', args: { op: 'resume' } }); await pulse();
    expect(await follow()).toEqual(lit);
  });
  test('L3 a page that is not held lights nothing', async () => {
    R.setHandOverForTest(null);
    await radioListen(undefined, nat as never, ADDRESS, 'Harbor', PAGE); await pulse(); await pulse();
    expect(R.heldDocument(ADDRESS)).toBeUndefined();
    expect(await follow()).toEqual(NO_FOLLOW);
  });
  test('L4 the wipe clears the held answer: no words, nothing lit, and the next play lights again (the reader is not reset by hand)', async () => {
    await playPage();
    expect((await radioPageFacts(undefined, nat as never)).sentences.length).toBe(2);
    expect((await follow()).blockId).not.toBe('');
    const old = await getRadio(undefined, nat as never, false);
    await wipeRadio(undefined); nextGeneration();
    expect(old.spoken).toEqual(NO_SPOKEN);                                       // the held answer itself is cleared, not only left behind                                // sign-out: the radio is wiped and the account generation moves; the reader drops its pages by that generation alone
    deck = { why: 'empty', item: '', seq: 0 };
    const after = await radioPageFacts(undefined, nat as never);
    expect(after.sentences).toEqual([]); expect(after.sentenceIndex).toBe(0); expect(after.progress).toBe(0);
    expect(await follow()).toEqual(NO_FOLLOW);
    expect(R.heldDocument(ADDRESS)).toBeUndefined();
    await playPage();
    expect(await follow()).toEqual({ address: ADDRESS, blockId: blocksOf()[1].id, sentence: 1 });
  });
  test('L4b a new account generation alone returns no held words', async () => {
    await playPage();
    expect((await radioPageFacts(undefined, nat as never)).sentences.length).toBe(2);
    nextGeneration();                                                            // the account moved; the wipe has not run yet
    expect((await radioPageFacts(undefined, nat as never)).sentences).toEqual([]);
    expect(await follow()).toEqual(NO_FOLLOW);
  });
  test('L5 a document replaced mid-play, and what native speaks with it, is followed in its new blocks', async () => {
    let content = PAGE;
    const n = await playPage(() => content);
    const before = await follow();
    const NEW = 'A new heading line here.';
    content = `${NEW}\n\n${P1}\n\n${P3}`;
    await R.readerReload(ADDRESS); await R.readerDocShared(ADDRESS, 'in', false, n);
    expect(await follow()).toEqual(NO_FOLLOW);                                   // native still speaks the old second paragraph, which the new page does not hold
    said.text = P1; said.fraction = 0.1; await pulse();
    const b = blocksOf();
    expect(b.length).toBe(3);
    expect(await follow()).toEqual({ address: ADDRESS, blockId: b[1].id, sentence: 0 });
    expect(before.blockId).not.toBe('');
  });
  test('L6 another entry, another revision and another page shown light nothing', async () => {
    await playPage();
    const lit = { address: ADDRESS, blockId: blocksOf()[1].id, sentence: 1 };
    expect(await follow()).toEqual(lit);
    // a different entry of the same story: the answer read for the first entry is not the second's, until the pulse reads again
    await radioListen(undefined, nat as never, ADDRESS, 'Harbor again', PAGE);
    const r = await getRadio(undefined, nat as never, false);
    r.spoken = { ...r.spoken, entry: 'the-entry-before' };
    expect(await follow()).toEqual(NO_FOLLOW);
    await pulse();
    expect(await follow()).toEqual(lit);
    // another page shown while this one plays
    await R.readerDocShared('pub:pub/other', 'in', false, net(() => PAGE));
    expect(await follow('pub:pub/other')).toEqual(NO_FOLLOW);
    expect(await follow(ADDRESS)).toEqual(lit);
  });
  test('L6b another revision of the same address lights nothing', async () => {
    const REV = `${ADDRESS}@3`;
    await playPage();
    await R.readerDocShared(REV, 'in', false, net(() => PAGE));                   // the revision is held too
    said.document = REV; await pulse();                                           // native speaks the revision, the entry on air is the bare address
    expect(await follow(ADDRESS)).toEqual(NO_FOLLOW);
    expect(await follow(REV)).toEqual(NO_FOLLOW);
    said.document = ADDRESS; await pulse();
    expect(await follow(ADDRESS)).toEqual({ address: ADDRESS, blockId: blocksOf()[1].id, sentence: 1 });
    expect(await follow(REV)).toEqual(NO_FOLLOW);                                 // the page shown is the revision, not the page on air
  });
  test('L7 one native read a pulse serves both the page facts and the follow', async () => {
    await playPage();
    factCalls = 0;
    for (let i = 0; i < 3; i++) { await pulse(); await follow(); await radioPageFacts(undefined, nat as never); }
    expect(factCalls).toBe(3);
    const f = await radioPageFacts(undefined, nat as never);
    expect(f.sentences).toEqual(['A middle paragraph speaks next.', 'It has two sentences too.']);
    expect(f.sentenceIndex).toBe(1); expect(f.progress).toBeCloseTo((1 + 0.8) / 3, 9);
  });
});

describe('N3 the order of answers', () => {
  type Pending = { resolve: (v: unknown) => void; reject: (e: unknown) => void };
  test('an older answer resolving later never overwrites a newer one; a failed read clears only what is older', async () => {
    await wipeRadio(undefined);
    const pending: Pending[] = [];
    const slow = { available: true, later: (r: { op: string }) => (r.op === 'radioFacts' ? new Promise((resolve, reject) => { pending.push({ resolve, reject }); }) : Promise.resolve({})) };
    adoptNative(slow as never, 'test');
    const answer = (words: string) => ({ at: 1_700_000_000_000, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: true, document: 'rs:e', spoken: words, block: 0, blocks: 1, blockFraction: 0 });
    const r = await getRadio(undefined, slow as never, false);
    const first = getRadio(undefined, slow as never, true), second = getRadio(undefined, slow as never, true);
    await new Promise((res) => setTimeout(res, 20));
    expect(pending.length).toBe(2);
    pending[1].resolve(answer('Newer words.')); await second;
    expect(r.spoken.spoken).toBe('Newer words.');
    pending[0].resolve(answer('Older words.')); await first;
    expect(r.spoken.spoken).toBe('Newer words.');
    const third = getRadio(undefined, slow as never, true), fourth = getRadio(undefined, slow as never, true);
    await new Promise((res) => setTimeout(res, 20));
    pending[3].resolve(answer('Fourth words.')); await fourth;
    pending[2].reject(new Error('down')); await third;
    expect(r.spoken.spoken).toBe('Fourth words.');                               // the older read failed: the newer answer stays
    const fifth = getRadio(undefined, slow as never, true);
    await new Promise((res) => setTimeout(res, 20));
    pending[4].reject(new Error('down')); await fifth;
    expect(r.spoken.spoken).toBe('');                                            // a failed read newer than what is held clears it
  });
});
