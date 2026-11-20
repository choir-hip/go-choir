// The Texture body under bun test (texture-body.ts). Ported from TextureBodyTests (Swift, deleted with the Swift): the failure list is at the top of texture-body.ts (1..5).
//  B1 load, edit nothing, save: the original; adding a paragraph and taking it away leaves every node as it was.   B2 a small edit changes only the edited node, keeps its id and citations.
//  B3 an embed, a table and an unknown node survive edits around them; deleting the paragraph before an embed keeps the embed.   B4 an edited table keeps its node (opaqueEdits).
//  B5 a new document gets an editor-compatible body (marks, nesting, ids, no link mark).   B6 the Markdown is kept byte for byte (the original body, whatever the Markdown said).
//  B7 the pieces: blocks (fences), key, list markers, inline marks, the Markdown a body writes.
// Review round 1 (2026-11-20), the failure list for the merge written before the code:
//  B8 a book-length document (6000 blocks): one edit is cheap (well under the 100 ms turn), every untouched node is the original, nothing allocates a table of 6000 x 6000.
//  B9 a document with EVERY block changed (the worst case for a diff) finishes in time and still writes a body with one node per block; blocks that did not change keep their node.
//  B10 diffPairs: equal to the exact longest common subsequence where the table is affordable; pairs are increasing; a large middle with no unique lines is bounded, not hung.
//  B11 merge3: edits in different places combine; the same line changed two ways is a conflict (null); an insertion by each side at different places combine; text neither side touched is byte for byte.
import { describe, expect, test } from 'bun:test';
import * as B from './texture-body.ts';
import { readdirSync, readFileSync } from 'node:fs';

const FX = new URL('../fixtures/', import.meta.url).pathname;
const json = (p: string) => JSON.parse(readFileSync(FX + p, 'utf8'));
function fixtures(): { name: string; body: B.Node }[] {
  const out: { name: string; body: B.Node }[] = [];
  for (const f of readdirSync(FX + 'texture').sort().filter((n) => n.startsWith('story-'))) { const b = json('texture/' + f)?.artifact?.body_doc; if (b) out.push({ name: f, body: b }); }
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
  const doc = (n: number, f: (i: number) => string = (i) => `Paragraph ${i} has its own words ${i * 7919}.`): { body: B.Node; md: string } => {
    const nodes = Array.from({ length: n }, (_, i) => ({ type: 'paragraph', attrs: { id: `p${i}` }, content: [{ type: 'text', text: f(i) }] }));
    return { body: { schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: nodes } }, md: nodes.map((x) => x.content[0].text).join('\n\n') };
  };
  test('B8 a book-length document: one edit is cheap and keeps every other node', () => {
    const { body, md } = doc(6000);
    const edited = md.replace('Paragraph 3000 has', 'Paragraph 3000 now has');
    const t0 = performance.now();
    const r = B.merge(body, md, edited);
    expect(performance.now() - t0).toBeLessThan(100);
    const before = top(body), after = top(r.body);
    expect(after.length).toBe(6000);
    const changed = after.map((a, i) => (a === before[i] ? -1 : i)).filter((i) => i >= 0);
    expect(changed).toEqual([3000]);
    expect(after[3000].attrs.id).toBe('p3000');
  });
  test('B9 every block changed, and blocks inserted at the front: bounded', () => {
    const { body, md } = doc(3000);
    const all = doc(3000, (i) => `Rewritten ${i} entirely differently ${i * 31}.`).md;
    const t0 = performance.now();
    const r = B.merge(body, md, all);
    expect(performance.now() - t0).toBeLessThan(500);
    expect(top(r.body).length).toBe(3000);
    const shifted = 'New first\n\n' + md;                                                // everything moves down by one: the anchors find the rest
    const r2 = B.merge(body, md, shifted);
    expect(top(r2.body).length).toBe(3001);
    expect(top(r2.body).filter((n, i) => i > 0 && n === top(body)[i - 1]).length).toBeGreaterThan(2900);
  });
  test('B10 diffPairs is the exact LCS where affordable and bounded where not', () => {
    const lcs = (a: string[], b: string[]): number => { const t = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0)); for (let i = a.length - 1; i >= 0; i--) for (let k = b.length - 1; k >= 0; k--) t[i][k] = a[i] === b[k] ? t[i + 1][k + 1] + 1 : Math.max(t[i + 1][k], t[i][k + 1]); return t[0][0]; };
    const pick = (seed: number, n: number, alpha: number): string[] => { let x = seed; return Array.from({ length: n }, () => { x = (x * 1103515245 + 12345) & 0x7fffffff; return String.fromCharCode(97 + (x >> 8) % alpha); }); };
    for (const [sa, sb] of [[1, 2], [3, 4], [5, 6]]) {
      const a = pick(sa, 40, 4), b = pick(sb, 35, 4);
      const pairs = B.diffPairs(a, b);
      expect(pairs.length).toBe(lcs(a, b));
      pairs.forEach(([i, k], x) => { expect(a[i]).toBe(b[k]); if (x > 0) { expect(i).toBeGreaterThan(pairs[x - 1][0]); expect(k).toBeGreaterThan(pairs[x - 1][1]); } });
    }
    const big = pick(7, 4000, 3), big2 = pick(8, 4000, 3);                                // no unique lines at all: the gap stays unpaired, in time
    const t0 = performance.now();
    const bp = B.diffPairs(big, big2);
    expect(performance.now() - t0).toBeLessThan(300);
    bp.forEach(([i, k]) => expect(big[i]).toBe(big2[k]));
    expect(B.diffPairs([], ['a'])).toEqual([]); expect(B.diffPairs(['a'], ['a'])).toEqual([[0, 0]]);
  });
  test('B11 merge3', () => {
    const base = 'one\ntwo\nthree\nfour\nfive';
    expect(B.merge3(base, 'one\nTWO\nthree\nfour\nfive', 'one\ntwo\nthree\nfour\nFIVE')).toBe('one\nTWO\nthree\nfour\nFIVE');
    expect(B.merge3(base, 'one\nTWO\nthree\nfour\nfive', 'one\nDEUX\nthree\nfour\nfive')).toBeNull();
    expect(B.merge3(base, 'one\nTWO\nthree\nfour\nfive', 'one\nTWO\nthree\nfour\nfive')).toBe('one\nTWO\nthree\nfour\nfive');
    expect(B.merge3(base, 'zero\n' + base, base + '\nsix')).toBe('zero\n' + base + '\nsix');
    expect(B.merge3(base, base, 'x')).toBe('x'); expect(B.merge3(base, 'y', base)).toBe('y');
    expect(B.merge3(base, 'one\n\n\ntwo\nthree\nfour\nfive', 'one\ntwo\nthree\nfour\nFIVE  ')).toBe('one\n\n\ntwo\nthree\nfour\nFIVE  ');      // blank lines and trailing spaces are theirs, untouched
    expect(B.merge3(base, 'one\ntwo\nthree\nfour', 'one\ntwo\nTHREE\nfour\nfive')).toBe('one\ntwo\nTHREE\nfour');       // one side deleted the last line, the other changed a middle one
    const big = Array.from({ length: 8000 }, (_, i) => `line ${i}`).join('\n');
    const t0 = performance.now();
    const m = B.merge3(big, big.replace('line 10\n', 'line ten\n'), big.replace('line 7000\n', 'line seven thousand\n'));
    expect(performance.now() - t0).toBeLessThan(100);
    expect(m).toBe(big.replace('line 10\n', 'line ten\n').replace('line 7000\n', 'line seven thousand\n'));
  });
});
