// The phone's side of the shared projection fixture (mobile/fixtures/texture-body/projection.json; the server's side is internal/texturedoc/phone_projection_test.go).
// The server derives a revision's `content` from its `body_doc` and refuses any other (HTTP 400 "content must match derived body_doc projection"), so the phone's `projectBody` must give what the server's
// `Project` gives, and what the phone writes for a typed text must project to that text. The `content` of every case in the fixture is written by the Go test from the server's own function
// (UPDATE_PHONE_PROJECTION=1); this test never writes it. The inputs and the phone's own body_docs are written here with UPDATE_PHONE_BODIES=1 and checked otherwise, so the fixture cannot be edited by hand.
// Ways it can fail: a node kind the projection renders differently (a heading, a list, a quote, a code fence, a hard break); a single newline typed in the editor (Enter) joined into a space or a paragraph;
// a trailing newline; the owner's own text (2026-11-24): "Hello", a new line, "This is my first time using texture editing on autoradio 1".
import { describe, expect, test } from 'bun:test';
import { readFileSync, writeFileSync } from 'node:fs';
import { merge, projectBody, canon, type Body } from './texture-body.ts';

const FILE = new URL('../fixtures/texture-body/projection.json', import.meta.url).pathname;
const doc = (...content: unknown[]): Body => ({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'doc-1' }, content } });
const p = (id: string, ...content: unknown[]) => ({ type: 'paragraph', attrs: { id }, content });
const t = (text: string, marks?: string[]) => ({ type: 'text', text, ...(marks ? { marks: marks.map((m) => ({ type: m })) } : {}) });
const br = { type: 'hard_break' };
const li = (id: string, ...content: unknown[]) => ({ type: 'list_item', attrs: { id }, content });

// what the server projects, by case (inputs only: the Go test writes `content`)
const PROJECTION: { name: string; body_doc: Body }[] = [
  { name: 'one paragraph', body_doc: doc(p('p1', t('Hello world.'))) },
  { name: 'two paragraphs are a blank line apart', body_doc: doc(p('p1', t('One.')), p('p2', t('Two.'))) },
  { name: 'a hard break is one newline', body_doc: doc(p('p1', t('Hello'), br, t('This is my first time using texture editing on autoradio 1'))) },
  { name: 'an empty paragraph', body_doc: doc(p('p1')) },
  { name: 'a trailing empty paragraph is trimmed', body_doc: doc(p('p1', t('A.')), p('p2')) },
  { name: 'a heading', body_doc: doc({ type: 'heading', attrs: { id: 'h1', level: 2 }, content: [t('Title')] }, p('p1', t('Body.'))) },
  { name: 'marks are not in the text', body_doc: doc(p('p1', t('bold', ['strong']), t(' and '), t('em', ['emphasis']), t(' and '), t('code', ['code']))) },
  { name: 'a bullet list', body_doc: doc({ type: 'bullet_list', attrs: { id: 'l1' }, content: [li('i1', p('p1', t('a'))), li('i2', p('p2', t('b')))] }) },
  { name: 'an ordered list from 3', body_doc: doc({ type: 'ordered_list', attrs: { id: 'l1', start: 3 }, content: [li('i1', p('p1', t('a'))), li('i2', p('p2', t('b')))] }) },
  { name: 'a nested list', body_doc: doc({ type: 'bullet_list', attrs: { id: 'l1' }, content: [li('i1', p('p1', t('a')), { type: 'bullet_list', attrs: { id: 'l2' }, content: [li('i2', p('p2', t('b')))] })] }) },
  { name: 'a quote of two paragraphs', body_doc: doc({ type: 'blockquote', attrs: { id: 'q1' }, content: [p('p1', t('one')), p('p2', t('two'))] }) },
  { name: 'a code block with a language', body_doc: doc({ type: 'code_block', attrs: { id: 'c1', language: 'go' }, content: [t('a := 1'), t('b := 2')] }) },
  { name: 'a padded code language is trimmed by the server', body_doc: doc({ type: 'code_block', attrs: { id: 'c1', language: ' go ' }, content: [t('x')] }) },
  { name: 'a start that is not an integer counts from 1', body_doc: doc({ type: 'ordered_list', attrs: { id: 'l1', start: 2.5 }, content: [li('i1', p('p1', t('a')))] }) },
  { name: 'a list item of two paragraphs', body_doc: doc({ type: 'bullet_list', attrs: { id: 'l1' }, content: [li('i1', p('p1', t('a')), p('p2', t('b')))] }) },
  { name: 'an empty code block', body_doc: doc({ type: 'code_block', attrs: { id: 'c1' }, content: [] }) },
  { name: 'unicode', body_doc: doc(p('p1', t('Caf\u00e9 \u2014 \u65e5\u672c\u8a9e \ud83d\ude00'))) },
  { name: 'a rule between paragraphs', body_doc: doc(p('p1', t('A.')), { type: 'horizontal_rule', attrs: { id: 'r1' } }, p('p2', t('B.'))) },
];

type MergeCase = { name: string; text: string; head_content: string; head_body: Body | null };
const SECOND = doc(p('p1', t('First.')), p('p2', t('Second.')));
const MERGE: MergeCase[] = [
  { name: "the owner's text on a new document (no revision)", text: 'Hello\nThis is my first time using texture editing on autoradio 1', head_content: '', head_body: null },
  { name: 'the same text on an empty body', text: 'Hello\nThis is my first time using texture editing on autoradio 1', head_content: '', head_body: doc(p('p0')) },
  { name: 'two paragraphs', text: 'One.\n\nTwo.', head_content: '', head_body: null },
  { name: 'three lines and a paragraph', text: 'a\nb\nc\n\nd', head_content: '', head_body: null },
  { name: 'a trailing newline (Enter at the end)', text: 'Hello\n', head_content: '', head_body: null },
  { name: 'a heading and a list', text: '# Title\n\n- a\n- b\n\n1. x\n2. y', head_content: '', head_body: null },
  { name: 'a quote and a code block', text: '> q1\n> q2\n\n```go\na := 1\nb := 2\n```', head_content: '', head_body: null },
  { name: 'a rule', text: 'A.\n\n---\n\nB.', head_content: '', head_body: null },
  { name: 'an edit of the second paragraph', text: 'First.\n\nSecond, edited.\nA new line.', head_content: 'First.\n\nSecond.', head_body: SECOND },
  { name: 'an unchanged text', text: 'First.\n\nSecond.', head_content: 'First.\n\nSecond.', head_body: SECOND },
  { name: 'markup is not in the text', text: 'some **bold** and *em* and `code`', head_content: '', head_body: null },
];

const strip = (v: unknown): unknown => (Array.isArray(v) ? v.map(strip) : v && typeof v === 'object' ? Object.fromEntries(Object.entries(v as Record<string, unknown>).map(([k, x]) => [k, k === 'id' ? '#' : strip(x)])) : v);

type File = { comment: string; projection: { name: string; body_doc: Body; content: string }[]; merge: { name: string; text: string; head_content: string; head_body: Body | null; body_doc: Body; content: string }[] };
const read = (): File => JSON.parse(readFileSync(FILE, 'utf8')) as File;
const built = (): File => ({
  comment: 'Shared by internal/texturedoc/phone_projection_test.go (the server) and mobile/AutoradioLab/texture-body-projection.test.ts (the phone). `content` is written by the Go test from the server\'s Project; the rest by the phone test (UPDATE_PHONE_BODIES=1). Do not edit by hand.',
  projection: PROJECTION.map((c) => ({ ...c, content: '' })),
  merge: MERGE.map((c) => ({ ...c, body_doc: merge(c.head_body, c.head_content, c.text).body, content: '' })),
});
if (process.env.UPDATE_PHONE_BODIES === '1') {
  let old: File | null = null; try { old = read(); } catch { /* the first write */ }
  const b = built();
  b.projection.forEach((c, i) => { c.content = old?.projection[i]?.content ?? ''; });
  b.merge.forEach((c, i) => { c.content = old?.merge[i]?.content ?? ''; });
  writeFileSync(FILE, JSON.stringify(b, null, 2) + '\n');
}

describe('the phone projects as the server does', () => {
  const f = read();
  test('the fixture holds exactly the cases this test defines (no hand edit)', () => {
    expect(f.projection.map((c) => c.name)).toEqual(PROJECTION.map((c) => c.name));
    expect(f.projection.map((c) => c.body_doc)).toEqual(PROJECTION.map((c) => c.body_doc));
    expect(f.merge.map((c) => [c.name, c.text, c.head_content, c.head_body])).toEqual(MERGE.map((c) => [c.name, c.text, c.head_content, c.head_body]));
  });
  for (const c of f.projection) {
    test(`P: ${c.name}`, () => { expect(projectBody(c.body_doc)).toBe(c.content); });
  }
  for (const c of f.merge) {
    test(`M: ${c.name}`, () => {
      const out = merge(c.head_body, c.head_content, c.text).body;
      expect(strip(out)).toEqual(strip(c.body_doc));                          // what the phone writes is what the server was shown
      expect(projectBody(out)).toBe(c.content);                                // and the text it sends is what the server derives from it
      expect(canon(c.text)).toBe(c.content);      // the text the listener typed is that text, by the one rule that compares them
    });
  }
  test("the owner's text keeps its line break and is two lines of ONE paragraph (a single Enter is a hard break, as the server's plain-text rule and the web's Shift+Enter)", () => {
    const c = f.merge[0];
    expect(c.content).toBe('Hello\nThis is my first time using texture editing on autoradio 1');
    const top = (c.body_doc.doc as { content: { type: string; content: { type: string }[] }[] }).content;
    expect(top.length).toBe(1); expect(top[0].content.map((x) => x.type)).toEqual(['text', 'hard_break', 'text']);
  });
});
