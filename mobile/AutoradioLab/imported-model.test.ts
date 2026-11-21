// The imported documents' pure half: names, addresses, the text of a PDF or an e-book as sections and blocks, search, snippets. It ports what the Swift
// ReaderShelfTests and EpubTests asserted of the text and the search (ReaderShelf.swift, Epub.swift are deleted); the zip and XML tests have no subject any
// more, because the listener's computer reads the file (the server's pdftotext and pandoc, internal/content/extract.go) and the phone reads the text.
//
// Ways this can fail (each is a test below):
//  1. A name that is not a PDF or an e-book is not a document; the id of a name survives a round trip, a name with "@", "%" or spaces included.
//  2. An address with a block ("doc:<id>@12") parses; "doc:" and another scheme do not.
//  3. A PDF's text becomes a section per page that has words, titled with the real page number; a blank line ends a paragraph, a hyphen at a line end joins the word.
//  4. A long page with no blank lines is cut at sentence ends.
//  5. An e-book's Markdown becomes headings, paragraphs, quotes, items and code with the marks gone; links, images, attribute groups, divs and footnote marks do not leak into words.
//  6. An e-book's chapters are the level of heading that repeats; a book with no such heading is cut into parts; text before the first chapter is its own section.
//  7. Search folds case, accents and punctuation, finds every word (the last one as a prefix), ranks a phrase first, and an empty or punctuation-only query finds nothing.
//  8. A position beyond the end or below zero is clamped.
//  9. The words handed to the player are the blocks from a position to the end, one line each.
import { describe, expect, test } from 'bun:test';
import * as M from './imported-model.ts';

describe('names and addresses', () => {
  test('only a PDF or an e-book is a document', () => {
    expect(M.kindOfName('Report.PDF')).toBe('pdf');
    expect(M.kindOfName('book.epub')).toBe('epub');
    expect(M.kindOfName('notes.txt')).toBeNull();
    expect(M.kindOfName('pdf')).toBeNull();
    expect(M.kindOfName('')).toBeNull();
  });
  test('an id is the file name and comes back whole', () => {
    for (const n of ['a.pdf', 'My report @ 2026 (final) 100%.pdf', 'Ünï cödé — 日本語.epub', 'a b#c?d.pdf']) {
      const id = M.idOf(n);
      expect(id).not.toContain('@'); expect(id).not.toContain('/');
      expect(M.nameOfId(id)).toBe(n);
    }
    expect(M.nameOfId('%E0%A4%A')).toBeNull();
    expect(M.nameOfId('')).toBeNull();
  });
  test('an address names a document and maybe a block', () => {
    expect(M.parseAddress('doc:abc@12')).toEqual({ id: 'abc', block: 12 });
    expect(M.parseAddress('doc:abc')).toEqual({ id: 'abc', block: null });
    expect(M.parseAddress('doc:abc@x')).toEqual({ id: 'abc', block: null });
    expect(M.parseAddress('doc:')).toBeNull();
    expect(M.parseAddress('pub:x')).toBeNull();
    expect(M.addressOf('abc')).toBe('doc:abc');
    expect(M.addressOf('abc', 4)).toBe('doc:abc@4');
    expect(M.parseAddress(M.addressOf(M.idOf('a @ b.pdf'), 3))).toEqual({ id: M.idOf('a @ b.pdf'), block: 3 });
  });
  test('a name that is taken gets a number before its extension', () => {
    expect(M.uniqueName('a.pdf', new Set())).toBe('a.pdf');
    expect(M.uniqueName('a.pdf', new Set(['a.pdf']))).toBe('a 2.pdf');
    expect(M.uniqueName('a.pdf', new Set(['a.pdf', 'a 2.pdf']))).toBe('a 3.pdf');
  });
  test('a title is the name without its extension and underscores', () => {
    expect(M.titleOf('My_Notes.pdf')).toBe('My Notes');
    expect(M.titleOf('The Keeper.epub')).toBe('The Keeper');
  });
});

describe('a PDF', () => {
  test('pages and paragraphs (Swift testPagesAndParagraphs)', () => {
    const t = M.fromPdfText('First line\nsecond line\n\nNew paragraph here\f\fThird page text');
    expect(t.sections.map((s) => s.title)).toEqual(['Page 1', 'Page 3']);
    expect(t.blocks.map((b) => b.text)).toEqual(['First line second line', 'New paragraph here', 'Third page text']);
    expect(M.sectionOf(t, 2)).toBe(1);
    expect(t.sections[1]).toEqual({ title: 'Page 3', start: 2, count: 1 });
  });
  test('a blank line that holds spaces still ends a paragraph (pdftotext -layout writes them)', () => {
    expect(M.paragraphs('one  \n   \ntwo')).toEqual(['one', 'two']);
  });
  test('a hyphen at a line end joins the word (Swift testHyphenatedLineEndJoins)', () => {
    expect(M.paragraphs('a repre-\nsentation of it')).toEqual(['a representation of it']);
    expect(M.paragraphs('well-\nKnown')).toEqual(['well- Known']);
  });
  test('a long page with no blank lines is cut at sentence ends (Swift testLongPageWithoutBlankLinesIsCutAtSentences)', () => {
    const s = Array.from({ length: 40 }, (_, i) => `This is sentence number ${i}.`).join('\n');
    const ps = M.paragraphs(s);
    expect(ps.length).toBeGreaterThan(1);
    expect(ps.every((p) => p.endsWith('.'))).toBe(true);
  });
  test('no text is no blocks', () => {
    expect(M.fromPdfText('').blocks).toEqual([]);
    expect(M.fromPdfText('\f\f \f').blocks).toEqual([]);
  });
  test('text with no page marks is cut into parts, not one page of thousands of blocks', () => {
    const text = Array.from({ length: 130 }, (_, i) => `Paragraph ${i}.`).join('\n\n');
    const t = M.fromPdfText(text);
    expect(t.blocks.length).toBe(130);
    expect(t.sections.length).toBeGreaterThan(1);
    expect(t.sections[0].title).toBe('Part 1');
    expect(t.sections.reduce((n, s) => n + s.count, 0)).toBe(130);
  });
});

describe('an e-book as Markdown', () => {
  const md = [
    '# The Test Book', '', 'A. Writer', '', '# One {#ch1 .chapter}', '',
    'The *lantern* was lit & the café closed early.', '', 'Second **paragraph** with [a link](http://x.example "t") and ![a picture](img.png){width=2}.', '',
    '- apples', '- pears', '', '1. first', '2. second', '',
    '# Two', '', '> Quoted words here.', '> More quoted.', '', 'The harbour at dawn.[^1]', '', '```', 'let x = 1', '```', '', '::: {.note}', 'Inside a div.', ':::', '',
  ].join('\n');
  const t = M.fromMarkdown(md);
  test('chapters are the headings that repeat; the title before them is the front matter', () => {
    expect(t.sections.map((s) => s.title)).toEqual(['The Test Book', 'One', 'Two']);
    expect(t.sections.reduce((n, s) => n + s.count, 0)).toBe(t.blocks.length);
    expect(t.blocks[t.sections[1].start]).toMatchObject({ kind: 'heading', text: 'One', level: 1 });
  });
  test('marks, links, images, attribute groups and footnote marks are gone', () => {
    const texts = t.blocks.map((b) => b.text);
    expect(texts).toContain('The lantern was lit & the café closed early.');
    expect(texts).toContain('Second paragraph with a link and .');
    expect(texts).toContain('The harbour at dawn.');
    expect(texts).toContain('Inside a div.');
    expect(texts.join('\n')).not.toMatch(/[*`\\]|\]\(|\{#|:::|\[\^/);
    expect(texts).not.toContain('');
  });
  test('items, quotes and code keep their kind', () => {
    expect(t.blocks.filter((b) => b.kind === 'item').map((b) => `${b.marker}${b.text}`)).toEqual(['• apples', '• pears', '1. first', '2. second']);
    expect(t.blocks.find((b) => b.text.startsWith('Quoted'))).toMatchObject({ kind: 'quote', text: 'Quoted words here. More quoted.' });
    expect(t.blocks.find((b) => b.kind === 'code')?.text).toBe('let x = 1');
  });
  test('setext headings (pandoc writes them by default) are headings', () => {
    const s = M.fromMarkdown('Chapter A\n=========\n\nBody a.\n\nChapter B\n=========\n\nBody b.\n');
    expect(s.sections.map((x) => x.title)).toEqual(['Chapter A', 'Chapter B']);
    expect(s.blocks.filter((b) => b.kind === 'heading').length).toBe(2);
    expect(M.fromMarkdown('One line\n\n---\n\nAfter the rule.').blocks.map((b) => b.text)).toEqual(['One line', 'After the rule.']);
  });
  test('text before the first chapter with no heading is the Beginning', () => {
    const s = M.fromMarkdown('Preface words.\n\n# One\n\nA.\n\n# Two\n\nB.');
    expect(s.sections.map((x) => x.title)).toEqual(['Beginning', 'One', 'Two']);
  });
  test('a book with no repeating heading is cut into parts of the blocks it has', () => {
    const body = Array.from({ length: 100 }, (_, i) => `Paragraph ${i}.`).join('\n\n');
    const s = M.fromMarkdown('# Only title\n\n' + body);
    expect(s.sections.length).toBeGreaterThan(1);
    expect(s.sections[0].title).toBe('Part 1');
    expect(s.sections.reduce((n, x) => n + x.count, 0)).toBe(s.blocks.length);
  });
  test('plain removes marks and keeps escaped ones (Swift testPlainRemovesMarksAndKeepsEscapedOnes)', () => {
    expect(M.plain('a *b* **c** `d` \\*e\\*')).toBe('a b c d *e*');
  });
  test('an empty book has no blocks', () => {
    expect(M.fromMarkdown('').blocks).toEqual([]);
    expect(M.fromMarkdown('![x](y.png)\n\n---\n').blocks).toEqual([]);
  });
});

describe('search', () => {
  const blocks = (texts: string[]): M.Block[] => texts.map((text) => ({ kind: 'paragraph', level: 0, text, marker: '', section: 0 }));
  const book = blocks(['The lantern was lit & the café closed early.', 'Second paragraph with nested tags.', 'The harbour at dawn.', 'Quoted words here.']);
  const find = (q: string, b = book, limit = 50) => M.search(M.buildIndex(b), q, limit);
  test('case, accents, punctuation, a prefix of the last word, every word (Swift testSearchCasePunctuationPhraseAndPrefix)', () => {
    expect(find('LANTERN')).toEqual([0]);
    expect(find('cafe')).toEqual([0]);                 // accent folded
    expect(find('  harbour!! ')).toEqual([2]);
    expect(find('harb')).toEqual([2]);                 // prefix of the last word
    expect(find('lit the')).toEqual([0]);              // both words present, not a phrase
    expect(find('')).toEqual([]);
    expect(find('?!')).toEqual([]);
    expect(find('zebra')).toEqual([]);
    expect(find('harbour', book, 0)).toEqual([]);
    expect(find('second paragraph')).toEqual([1]);
  });
  test('a word that is only the start of an earlier word does not match (only the last word is a prefix)', () => {
    expect(find('harb dawn')).toEqual([]);
  });
  test('a phrase ranks above scattered words (Swift testPhraseRanksAboveScatteredWords)', () => {
    const b = blocks(['Dawn came late to the harbour', 'The harbour at dawn']);
    expect(find('harbour at dawn', b)).toEqual([1]);
    expect(find('dawn harbour', b)).toEqual([0, 1]);
    expect(find('harbour', b, 1)).toEqual([0]);
  });
  test('width and compatibility forms fold (full-width letters, ligatures)', () => {
    const b = blocks(['ＡＢＣ is here', 'the ﬁnal word']);
    expect(find('abc', b)).toEqual([0]);
    expect(find('final', b)).toEqual([1]);
  });
  test('tokens', () => {
    expect(M.tokens("Don't stop—Zoë's 3rd!")).toEqual(['don', 't', 'stop', 'zoe', 's', '3rd']);
  });
  test('a snippet is the words around the first match, with ellipses where it is cut', () => {
    const long = 'x '.repeat(80) + 'The harbour at dawn' + ' y'.repeat(80);
    const s = M.snippet(long, 'harbour');
    expect(s.startsWith('…')).toBe(true); expect(s.endsWith('…')).toBe(true);
    expect(s.toLowerCase()).toContain('harbour');
    expect(M.snippet('Short text here', 'zzz')).toBe('Short text here');
    expect(M.snippet('Un café noir', 'CAFE')).toBe('Un café noir');
    expect(M.snippet('a '.repeat(200), '').length).toBeLessThanOrEqual(140);
  });
});

describe('places and the words handed to the player', () => {
  const t = M.fromPdfText('One.\n\nTwo.\fThree.\n\nFour.');
  test('a position is clamped into the document (Swift testPositionSaveClampAndRestore)', () => {
    expect(M.clampBlock(3, 4)).toBe(3);
    expect(M.clampBlock(999, 4)).toBe(3);
    expect(M.clampBlock(-4, 4)).toBe(0);
    expect(M.clampBlock(2, 0)).toBe(0);
  });
  test('the words from a block to the end, one line each (Swift testBlocksFromPositionForReadAloud)', () => {
    const all = M.spokenFrom(t, 0);
    expect(all).toEqual(['One.', 'Two.', 'Three.', 'Four.']);
    expect(M.spokenFrom(t, 2)).toEqual(['Three.', 'Four.']);
    expect(M.spokenFrom(t, 500)).toEqual(['Four.']);
    expect(M.spokenFrom({ sections: [], blocks: [] }, 0)).toEqual([]);
  });
  test('a block with a newline (code) is one line for the player', () => {
    const c = M.fromMarkdown('```\na\nb\n```\n');
    expect(M.spokenFrom(c, 0)).toEqual(['a b']);
  });
  test('where reading aloud starts: the chosen block when it lies in the section on screen, else the section first block', () => {
    expect(M.startBlock(t, 1, 3)).toBe(3);
    expect(M.startBlock(t, 1, 0)).toBe(2);
    expect(M.startBlock(t, 1, -1)).toBe(2);
    expect(M.startBlock(t, 9, -1)).toBe(2);
    expect(M.startBlock({ sections: [], blocks: [] }, 0, -1)).toBeNull();
  });
  test('progress and the line under a row', () => {
    expect(M.progressLabel(undefined)).toBe('Not started');
    expect(M.progressLabel({ block: 0, length: 10 })).toBe('Not started');
    expect(M.progressLabel({ block: 5, length: 11 })).toBe('50% read');
    expect(M.progressOf({ block: 99, length: 11 })).toBe(1);
  });
});
