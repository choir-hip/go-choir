// The Texture body under bun test (texture-body.ts). Ported from TextureBodyTests (Swift, deleted with the Swift): the failure list is at the top of texture-body.ts (1..5).
//  B1 load, edit nothing, save: the original; adding a paragraph and taking it away leaves every node as it was.   B2 a small edit changes only the edited node, keeps its id and citations.
//  B3 an embed, a table and an unknown node survive edits around them; deleting the paragraph before an embed keeps the embed.   B4 an edited table keeps its node (opaqueEdits).
//  B5 a new document gets an editor-compatible body (marks, nesting, ids, no link mark).   B6 the Markdown is kept byte for byte (the original body, whatever the Markdown said).
//  B7 the pieces: blocks (fences), key, list markers, inline marks, the Markdown a body writes.
import { describe, expect, test } from 'bun:test';
import * as B from './texture-body.ts';
import { readdirSync, readFileSync } from 'node:fs';

const FX = new URL('../fixtures/', import.meta.url).pathname;
const json = (p: string) => JSON.parse(readFileSync(FX + p, 'utf8'));
function fixtures(): { name: string; body: B.Node }[] {
  const out: { name: string; body: B.Node }[] = [];
  for (const f of readdirSync(FX + 'demo').sort().filter((n) => n.startsWith('resolve-'))) { const b = json('demo/' + f)?.artifact?.body_doc; if (b) out.push({ name: f, body: b }); }
  out.push({ name: 'letter-citations-embed', body: json('texture/letter-citations-embed.json').body_doc });
  out.push({ name: 'rich-mixed', body: json('texture/rich-mixed.json').body_doc });
  return out;
}
const top = (b: B.Node) => B.topNodes(b);
const types = (b: B.Node) => top(b).map((n) => n.type);

describe('the body merge', () => {
  test('B1 load, edit nothing, save is the original', () => {
    const fx = fixtures();
    expect(fx.length).toBeGreaterThanOrEqual(3);
    for (const { name, body } of fx) {
      const md = B.markdownFrom(body);
      expect(B.merge(body, md, md).body).toEqual(body);
      const again = B.merge(body, md, md + '\n\nx');
      const back = B.merge(again.body, md + '\n\nx', md);
      expect(top(back.body).length).toBe(top(body).length);
      top(back.body).forEach((a, i) => expect(a, `${name} node ${i}`).toEqual(top(body)[i]));
    }
  });
  test('B2 a small edit changes only the edited node', () => {
    for (const { name, body } of fixtures()) {
      const md = B.markdownFrom(body);
      const bl = B.blocks(md);
      const bi = bl.findIndex((x) => B.kindOf(x) === 'paragraph' && x.includes(' '));
      if (bi < 0) continue;
      const edited = [...bl];
      edited[bi] = bl[bi].replace(' ', ' very ');
      const r = B.merge(body, md, edited.join('\n\n'));
      const before = top(body), after = top(r.body);
      expect(after.length, name).toBe(before.length);
      const changed = after.map((a, i) => (JSON.stringify(a) === JSON.stringify(before[i]) ? -1 : i)).filter((i) => i >= 0);
      expect(changed.length, `${name}: ${changed}`).toBe(1);
      const c = changed[0];
      expect(after[c].type).toBe('paragraph');
      expect(after[c].attrs?.id).toBe(before[c].attrs?.id);
      const refs = (n: B.Node) => (n.content ?? []).filter((x: B.Node) => x.type === 'source_ref');
      expect(refs(after[c])).toEqual(refs(before[c]));
      expect(B.plainText(after[c])).toContain('very');
      expect(after.map((n) => n.type)).toEqual(before.map((n) => n.type));
    }
  });
  test('B3 an embed, a table and an unknown node survive edits around them', () => {
    const body = fixtures().find((f) => f.name === 'rich-mixed')!.body;
    const md = B.markdownFrom(body);
    expect(md).not.toContain('Region');
    const edited = md.replace('West', 'West coast').replace('Last line.', 'Last line, edited.');
    const r = B.merge(body, md, edited);
    expect(types(r.body)).toEqual(types(body));
    for (const t of ['embed', 'table', 'future_widget']) expect(top(r.body).find((n) => n.type === t)).toEqual(top(body).find((n) => n.type === t));
    expect(B.plainText(top(r.body).find((n) => n.type === 'bullet_list')!)).toContain('West coast');
    const gone = md.replace(B.blocks(md)[1], '');
    expect(top(B.merge(body, md, gone).body).find((n) => n.type === 'embed')).toBeDefined();
  });
  test('B4 an edited table keeps its node; the edit lives in the Markdown', () => {
    const body = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: [
      { type: 'paragraph', attrs: { id: 'p' }, content: [{ type: 'text', text: 'Intro' }] },
      { type: 'table', attrs: { id: 't' }, content: [{ type: 'table_row', content: [{ type: 'table_cell', content: [{ type: 'text', text: 'Region' }] }] }] }] } };
    const r = B.merge(body, 'Intro\n\n| Region |\n|---|', 'Intro\n\n| Region | Units |\n|---|---|');
    expect(types(r.body)).toEqual(['paragraph', 'table']);
    expect(r.opaqueEdits).toBe(1);
    expect(top(r.body)[1]).toEqual(top(body)[1]);
  });
  test('B5 a new document gets an editor-compatible body', () => {
    const md = '# Title\n\nA **bold** and *slanted* word with `code` and a [link](https://x.test).\n\n- one\n  - nested\n- two\n\n1. first\n2. second\n\n> quoted\n\n```\nlet x\n```\n\n---';
    const r = B.merge(null, '', md);
    expect(r.body.schema).toBe('choir.texture_doc.v1');
    const nodes = top(r.body);
    expect(nodes.map((n) => n.type)).toEqual(['heading', 'paragraph', 'bullet_list', 'ordered_list', 'blockquote', 'code_block', 'horizontal_rule']);
    expect(nodes[0].attrs.level).toBe(1);
    const inl: B.Node[] = nodes[1].content;
    const mark = (t: string, m: string) => inl.some((x) => x.text === t && x.marks?.[0]?.type === m);
    expect(mark('bold', 'strong')).toBe(true); expect(mark('slanted', 'emphasis')).toBe(true); expect(mark('code', 'code')).toBe(true);
    expect(inl.some((x) => typeof x.text === 'string' && x.text.includes('a link.'))).toBe(true);
    expect(inl.some((x) => typeof x.text === 'string' && x.text.includes('https'))).toBe(false);
    expect(inl.flatMap((x) => x.marks ?? []).some((m: B.Node) => m.type === 'link')).toBe(false);
    expect(nodes[2].content.length).toBe(2);
    expect(nodes[3].attrs.start).toBe(1);
    for (const n of nodes) expect(n.attrs?.id, n.type).toMatch(/^[a-z-]+-[0-9a-f]{8}$/);
    expect(B.blocks(B.markdownFrom(r.body)).length).toBe(7);
  });
  test('B6 the original body is kept whatever the Markdown said', () => {
    const md = 'A  \nB  trailing\n\n\n\n# H  \n';
    const body = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'A' }] }] } };
    expect(top(B.merge(body, md, md).body).length).toBe(1);
  });
  test('B7 the pieces', () => {
    expect(B.blocks('a\nb\n\n```js\nx\n\ny\n```\nz')).toEqual(['a\nb', '```js\nx\n\ny\n```', 'z']);
    expect(B.blocks('a\r\n\r\nb')).toEqual(['a', 'b']);
    expect(B.kindOf('# T')).toBe('heading'); expect(B.kindOf('---')).toBe('rule'); expect(B.kindOf('> q')).toBe('quote'); expect(B.kindOf('- a')).toBe('list');
    expect(B.kindOf('1) a')).toBe('list'); expect(B.kindOf('| a |\n|---|')).toBe('table'); expect(B.kindOf('#nospace')).toBe('paragraph');
    expect(B.key('- **Hi** [there](https://x.y)!')).toBe('hithere');
    expect(B.listMarker('  12. x')).toEqual({ indent: 2, ordered: true, number: 12, rest: 'x' });
    expect(B.inline('a  \nb').map((n) => n.type)).toEqual(['text', 'hard_break', 'text']);
    expect(B.inline('**a *b* c**').map((n) => [n.text, n.marks?.map((m: B.Node) => m.type)])).toEqual([['a ', ['strong']], ['b', ['strong', 'emphasis']], [' c', ['strong']]]);
    expect(B.markdownFrom({ doc: { content: [{ type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: 'H' }] }, { type: 'embed' }, { type: 'ordered_list', attrs: { start: 3 }, content: [{ type: 'list_item', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'x', marks: [{ type: 'strong' }] }] }] }] }] } })).toBe('## H\n\n3. **x**');
  });
});
