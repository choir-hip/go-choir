// The places the reader model must match the web reader on the golden documents (internal/texturedoc/golden/known-differences.json), ported from the Swift WebParityTests (deleted with the Swift parser).
// Each test names the web rule it matches:
//  1 the quote of a checked citation is the quoted words, not the check's outcome word ("exact"); a sample-shaped check whose quote IS the words still works.   2 a citation's own attrs.ref.quote is read when no transclusion or check has it.
//  3 an embed shows its caption (else the title, else the alt text).   4 a heading in a blockquote keeps its own level.   5 a code block of two text nodes joins them with a newline.
//  6 adjacent published citations of one source draw once when the bundle has manifest anchors, twice without; a word between them, or a hard break, ends the run.   7 paired ** in unmarked text is bold; "2 ** 3 ** 4", "a**b**c" and a lone ** stay literal.
import { describe, expect, test } from 'bun:test';
import { parseBundle } from './reader-parse.ts';

const doc = (content: string, o: { entities?: string; payload?: unknown; checks?: string } = {}) =>
  parseBundle(JSON.parse(`{"publication":{"title":"T"},"artifact":{"body_doc":{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[${content}]}}},"source_entities":${o.entities ?? '[]'},"checks":{"citations":${o.checks ?? '[]'}},"payload":${JSON.stringify(o.payload ?? {})}}`), 'pub:t');
const ents = '[{"source_entity_id":"s1","entity":{"target":{"kind":"web_url","uri":"https://example.com/a"},"display":{"title":"A"}}}]';
const ref = (id: string, quote?: string) => `{"type":"source_ref","attrs":{"id":"${id}","source_entity_id":"s1"${quote ? `,"ref":{"doc":"d","cid":"c1","quote":"${quote}"}` : ''}}}`;
const text = (t: string) => `{"type":"text","text":${JSON.stringify(t)}}`;
const para = (...kids: string[]) => `{"type":"paragraph","content":[${kids.join(',')}]}`;

describe('web parity', () => {
  test('1 the quote is the quoted words, not the check outcome', () => {
    const manifest = { manifest: { anchors: [{ anchor_id: 'a1', ref: { doc: 'd', cid: 'c1', quote: 'quoted words' } }] } };
    const d = doc(para(text('x'), ref('a1')), { entities: ents, payload: manifest, checks: '[{"anchor_id":"a1","state":"exact","quote":"exact"}]' });
    expect(d.cites[0].quote).toBe('quoted words'); expect(d.cites[0].check).toBe('exact');
    const d2 = doc(para(text('x'), ref('a1')), { entities: ents, checks: '[{"anchor_id":"a1","state":"exact","quote":"the words"}]' });
    expect(d2.cites[0].quote).toBe('the words');
  });
  test('2 a citation\'s own ref quote', () => { expect(doc(para(text('x'), ref('a1', 'own quote')), { entities: ents }).cites[0].quote).toBe('own quote'); });
  test('3 an embed is labelled by its caption, else its alt', () => {
    const d = doc('{"type":"embed","attrs":{"id":"e1","kind":"image","target":{"digest":"x"},"alt":"alt words","caption":"caption words"}},{"type":"embed","attrs":{"id":"e2","kind":"image","target":{"digest":"x"},"alt":"only alt"}}');
    expect(d.blocks[0].text).toBe('caption words'); expect(d.blocks[1].text).toBe('only alt');
  });
  test('4 and 5 a quote heading keeps its level; a code block joins its text nodes with a newline', () => {
    const d = doc('{"type":"blockquote","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"H"}]}]},{"type":"code_block","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}');
    expect(d.blocks[0].level).toBe(2); expect(d.blocks[1].text).toBe('a\nb');
  });
  test('6 adjacent published citations of one source collapse; both cards stay for the Sources list', () => {
    const manifest = { manifest: { anchors: ['a1', 'a2', 'a3'].map((id) => ({ anchor_id: id, ref: { doc: 'd', cid: 'c1' } })) } };
    const marks = (d: ReturnType<typeof doc>) => d.blocks[0].runs.filter((r) => r.kind === 'cite').length;
    const two = para(text('x'), ref('a1'), ref('a2'));
    const d = doc(two, { entities: ents, payload: manifest });
    expect(marks(d)).toBe(1); expect(d.cites.length).toBe(2);
    expect(marks(doc(two, { entities: ents }))).toBe(2);                                           // no manifest anchors: the web draws both
    expect(marks(doc(para(text('x'), ref('a1'), text(' '), ref('a2'), text(' and '), ref('a3')), { entities: ents, payload: manifest }))).toBe(2);   // a blank keeps the run, a word ends it
    expect(marks(doc(para(text('x'), ref('a1'), '{"type":"hard_break"}', ref('a2')), { entities: ents, payload: manifest }))).toBe(2);
  });
  test('7 legacy double asterisk is bold; the look-alikes stay literal', () => {
    const runs = (t: string) => doc(para(text(t))).blocks[0].runs;
    const r = runs('**Legacy bold.** Then plain.');
    expect(r[0].text).toBe('Legacy bold.'); expect(r[0].weight).toBe(700); expect(r.map((x) => x.text).join('')).toBe('Legacy bold. Then plain.');
    for (const literal of ['2 ** 3 ** 4', 'a**b**c', 'a lone ** here']) { expect(runs(literal).map((x) => x.text).join('')).toBe(literal); expect(runs(literal).every((x) => x.weight === 400)).toBe(true); }
  });
});
