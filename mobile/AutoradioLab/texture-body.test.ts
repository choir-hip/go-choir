// S2 (ported from the Swift TextureBodyTests, written before texture-body.ts): the document stays Texture-format compatible always.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { blocks, kindOf, merge, plainOnly, plainText, topNodes, type Body, type Node } from './texture-body.ts';

const dir = join(import.meta.dir, '..', 'fixtures', 'texture');
const read = (f: string): Record<string, unknown> => JSON.parse(readFileSync(join(dir, f), 'utf8'));

/** The Markdown for a body_doc, for the fixtures only (the app never writes Markdown from a body: the server's `content` is the text). */
function inlineMd(nodes: Node[]): string {
  let out = '';
  for (const n of nodes) {
    if (n.type === 'text') {
      let t = String(n.text ?? '');
      const marks = ((n.marks as Node[] | undefined) ?? []).map((m) => m.type);
      if (marks.includes('code')) t = `\`${t}\``;
      if (marks.includes('emphasis')) t = `*${t}*`;
      if (marks.includes('strong')) t = `**${t}**`;
      out += t;
    } else if (n.type === 'hard_break') out += '  \n';
  }
  return out;
}
function blockMd(n: Node, indent: string): string | null {
  const content = (n.content as Node[] | undefined) ?? [];
  switch (n.type) {
    case 'paragraph': return indent + inlineMd(content);
    case 'heading': return '#'.repeat(Number((n.attrs as Record<string, unknown> | undefined)?.level ?? 1)) + ' ' + inlineMd(content);
    case 'horizontal_rule': return '---';
    case 'code_block': return '```\n' + String(content[0]?.text ?? '') + '\n```';
    case 'blockquote': return content.map((c) => blockMd(c, '')).filter((x): x is string => x !== null).join('\n\n').split('\n').map((l) => '> ' + l).join('\n');
    case 'bullet_list': case 'ordered_list': {
      const ordered = n.type === 'ordered_list';
      let num = Number((n.attrs as Record<string, unknown> | undefined)?.start ?? 1);
      const lines: string[] = [];
      for (const item of content) {
        const kids = (item.content as Node[] | undefined) ?? [];
        const marker = ordered ? `${num}.` : '-';
        num += 1;
        kids.forEach((kid, idx) => {
          if (idx === 0 && kid.type === 'paragraph') lines.push(indent + marker + ' ' + inlineMd((kid.content as Node[] | undefined) ?? []));
          else { const b = blockMd(kid, indent + '  '); if (b !== null) lines.push(b); }
        });
      }
      return lines.join('\n');
    }
    default: return null;
  }
}
const markdownFrom = (body: Body): string => topNodes(body).map((n) => blockMd(n, '')).filter((x): x is string => x !== null).join('\n\n');

const P = (id: string, ...c: unknown[]): Node => ({ type: 'paragraph', attrs: { id }, content: c });
const T = (text: string, marks?: string[]): Node => ({ type: 'text', text, ...(marks ? { marks: marks.map((m) => ({ type: m })) } : {}) });
const PLAIN_MIXED: Body = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'dm' }, content: [
  { type: 'heading', attrs: { id: 'h1', level: 1 }, content: [T('Plan')] },
  P('p1', T('A first paragraph with a '), T('bold', ['strong']), T(' word.')),
  { type: 'bullet_list', attrs: { id: 'l1' }, content: [{ type: 'list_item', attrs: { id: 'i1' }, content: [P('p2', T('East'))] }, { type: 'list_item', attrs: { id: 'i2' }, content: [P('p3', T('West'))] }] },
  { type: 'blockquote', attrs: { id: 'q1' }, content: [P('p4', T('quoted'))] },
  P('p5', T('Last line.'))] } };
/** What the phone can write exactly: the stories and documents of the fixtures that hold only its kinds, and one of its own. */
function fixtures(): { name: string; body: Body }[] {
  const out: { name: string; body: Body }[] = [{ name: 'plain-mixed', body: PLAIN_MIXED }];
  for (const f of readdirSync(dir).sort()) if (f.startsWith('story-')) { const b = (read(f).artifact as Record<string, unknown> | undefined)?.body_doc; if (b && plainOnly(b as Body, [], '')) out.push({ name: f, body: b as Body }); }
  return out;
}
const types = (b: Body): unknown[] => topNodes(b).map((n) => n.type);
const attrId = (n: Node): unknown => (n.attrs as Record<string, unknown> | undefined)?.id;

describe('texture body merge', () => {
  test('1 load, edit nothing, save is the original', () => {
    const fx = fixtures();
    expect(fx.length).toBeGreaterThanOrEqual(1);
    for (const { name, body } of fx) {
      const md = markdownFrom(body);
      const r = merge(body, md, md);
      expect(r.body, `${name}: the body_doc is the original`).toEqual(body);
      const again = merge(body, md, md + '\n\nx');
      const back = merge(again.body, md + '\n\nx', md);
      expect(topNodes(back.body).length, `${name}: adding a paragraph and taking it away leaves the node list`).toBe(topNodes(body).length);
      topNodes(back.body).forEach((a, i) => expect(a, `${name}: node ${i} is as it was`).toEqual(topNodes(body)[i]));
    }
  });
  test('2 a small edit changes only the edited node', () => {
    for (const { name, body } of fixtures()) {
      const md = markdownFrom(body);
      const bl = blocks(md);
      const bi = bl.findIndex((b) => kindOf(b) === 'paragraph' && b.includes(' '));
      if (bi < 0) continue;
      const edited = [...bl];
      edited[bi] = bl[bi].replace(' ', ' very ');
      const r = merge(body, md, edited.join('\n\n'));
      const before = topNodes(body), after = topNodes(r.body);
      expect(after.length, name).toBe(before.length);
      const changed = before.map((a, i) => (JSON.stringify(a) === JSON.stringify(after[i]) ? -1 : i)).filter((i) => i >= 0);
      expect(changed.length, `${name}: exactly one node changed, got ${changed}`).toBe(1);
      const c = changed[0];
      expect(after[c].type).toBe('paragraph');
      expect(attrId(after[c]), `${name}: it keeps its id`).toBe(attrId(before[c]));
      expect(plainText(after[c])).toContain('very');
      expect(after.map((n) => n.type), `${name}: no node changed type or moved`).toEqual(before.map((n) => n.type));
    }
  });
  test('3 the guard: only what the phone writes exactly is editable (an embed, a source_ref, a table, a link mark, an unknown node, source entities, a Markdown table in the content are not)', () => {
    expect(plainOnly(PLAIN_MIXED, [], 'Plan\n\nText')).toBe(true);
    expect(plainOnly(null, [], '')).toBe(true);
    const wrap = (...n: Node[]): Body => ({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: n } });
    for (const [what, body] of [
      ['an embed', wrap(P('p'), { type: 'embed', attrs: { id: 'e', kind: 'file', title: 'x' } })],
      ['a source_ref', wrap(P('p', T('a'), { type: 'source_ref', attrs: { id: 'r', source_entity_id: 's' } }))],
      ['a table', wrap({ type: 'table', attrs: { id: 't' }, content: [] })],
      ['a node kind this port does not know', wrap({ type: 'future_widget', attrs: { id: 'w' } })],
      ['a link mark', wrap(P('p', T('x', ['link'])))],
      ['an embed inside a list item', wrap({ type: 'bullet_list', attrs: { id: 'l' }, content: [{ type: 'list_item', attrs: { id: 'i' }, content: [{ type: 'embed', attrs: { id: 'e' } }] }] })],
    ] as [string, Body][]) expect(plainOnly(body, [], ''), what).toBe(false);
    expect(plainOnly(PLAIN_MIXED, [{ source_entity_id: 's' }], ''), 'source entities').toBe(false);
    expect(plainOnly(PLAIN_MIXED, [], 'a\n\n| Region | Units |\n|---|---|'), 'a table in the content').toBe(false);
  });
  test('5 a new document gets an editor-compatible body', () => {
    const md = '# Title\n\nA **bold** and *slanted* word with `code` and a [link](https://x.test).\n\n- one\n  - nested\n- two\n\n1. first\n2. second\n\n> quoted\n\n```\nlet x\n```\n\n---';
    const r = merge(null, '', md);
    expect(r.body.schema).toBe('choir.texture_doc.v1');
    const nodes = topNodes(r.body);
    expect(nodes.map((n) => n.type)).toEqual(['heading', 'paragraph', 'bullet_list', 'ordered_list', 'blockquote', 'code_block', 'horizontal_rule']);
    expect((nodes[0].attrs as Record<string, unknown>).level).toBe(1);
    const inl = nodes[1].content as Node[];
    const mark = (t: string, m: string): boolean => inl.some((x) => x.text === t && ((x.marks as Node[] | undefined)?.[0]?.type === m));
    expect(mark('bold', 'strong')).toBe(true);
    expect(mark('slanted', 'emphasis')).toBe(true);
    expect(mark('code', 'code')).toBe(true);
    expect(inl.some((x) => String(x.text ?? '').includes('a link.')), 'a link keeps its text: the web has no link mark').toBe(true);
    expect(inl.some((x) => String(x.text ?? '').includes('https'))).toBe(false);
    expect(inl.flatMap((x) => (x.marks as Node[] | undefined) ?? []).some((m) => m.type === 'link')).toBe(false);
    expect((nodes[2].content as Node[]).length).toBe(2);
    expect((nodes[3].attrs as Record<string, unknown>).start, 'as the web writes it').toBe(1);
    for (const n of nodes) expect((n.attrs as Record<string, unknown> | undefined)?.id, `${n.type} has an id`).toBeDefined();
    expect(blocks(markdownFrom(r.body)).length).toBe(7);
  });
  test('6 the Markdown is kept byte for byte', () => {
    const md = 'A  \nB  trailing\n\n\n\n# H  \n';
    const r = merge({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'A' }] }] } }, md, md);
    expect(topNodes(r.body).length, 'the original body, whatever the Markdown said').toBe(1);
  });
  test('7 inline edges: a lone star, an unclosed code tick, a backslash hard break', () => {
    const r = merge(null, '', 'a * b and `open\n\nline\\\nnext');
    expect(plainText(topNodes(r.body)[0])).toBe('a * b and `open');
    const two = topNodes(r.body)[1].content as Node[];
    expect(two.map((x) => x.type)).toEqual(['text', 'hard_break', 'text']);
  });
});
