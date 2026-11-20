// The reader's document model (reader-parse.ts), written as the failure list first. The intents are those of the old Swift TextureDocTests; the oracle for real shapes is the golden documents
// (internal/texturedoc/golden): each is parsed and projected, and the projection must equal the committed expected/<name>.json.
//  P1 blocks: headings, paragraphs, unknown nodes keep their text, empty paragraphs vanish, the kind/kicker/disclosure of a story.
//  P2 no body_doc: Markdown (and a heading equal to the title is dropped); nothing at all is a visible "no text yet" block.
//  P3 lists (start, nesting, markers), quotes (depth), code (newlines kept).   P4 citations: numbering, unique anchors, missing entity, quote context, check words.
//  P5 a Texture transclusion is a "ref" run with a texture: target.   P6 sentences numbered from plain text; a cite belongs to the sentence it closes; run ids unique.
//  P7 Markdown tables and unmatched markers stay literal.   P8 versions from nav, clamped; newer only when latest > revision.   P9 addresses.   P10 hosts, check labels, quote context.
//  P11 a private document: revision rows newest first, revising, trajectory.   P12 3000 blocks stay fast.   P13 golden documents equal their committed projections.
//  P14 tables, embeds (caption, title, alt, kind), legacy ** bold, adjacent published citations of one source collapse.   P15 related reads `links`.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { checkLabel, contextOf, hostOf, mdBlocks, mdInline, parseAddress, parseBundle, parseRelated, parseRevision, sentenceTexts, splitRoute } from './reader-parse.ts';

const bodyDoc = (content: string) => `{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[${content}]}}`;
const bundle = (o: { body: string; entities?: string; trans?: string; checks?: string; resolved?: string; nav?: string; text?: string }) => JSON.parse(`
{"publication":{"title":"Headline","kind":"story","dek":"A dek."},
 "route":{"bare":"headline","permalink":"headline/v2"},
 "version":{"published_at":"2026-11-10T06:00:00Z"},
 "artifact":{"content":${JSON.stringify(o.text ?? '')},"body_doc":${o.body}},
 "source_entities":${o.entities ?? '[]'},"transclusions":${o.trans ?? '[]'},"checks":{"citations":${o.checks ?? '[]'}},
 "payload":{"text":"","resolved_blocks":${o.resolved ?? '[]'}},"nav":${o.nav ?? '{"revision":2,"prev":1,"next":0,"latest":2}'}}`);
const p = (t: string) => `{"type":"paragraph","content":[{"type":"text","text":${JSON.stringify(t)}}]}`;

describe('reader document model', () => {
  test('P1 blocks, unknown nodes keep text, empties vanish, story chrome', () => {
    const body = bodyDoc(`{"type":"heading","attrs":{"id":"h","level":2},"content":[{"type":"text","text":"Why it matters"}]},
      {"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"First "},{"type":"text","text":"bold","marks":[{"type":"strong"}]},{"type":"text","text":" and "},{"type":"text","text":"slanted","marks":[{"type":"emphasis"}]},{"type":"text","text":"."}]},
      {"type":"paragraph","content":[{"type":"hard_break"}]},{"type":"paragraph","content":[{"type":"text","text":"   "}]},
      {"type":"callout","content":[{"type":"text","text":"A future block"}]}`);
    const d = parseBundle(bundle({ body }), 'pub:headline');
    expect(d.status).toBe('ready');
    expect(d.blocks.map((b) => b.kind)).toEqual(['h', 'p', 'p']);
    expect(d.blocks[0].level).toBe(2);
    expect(d.blocks[1].runs.find((r) => r.text === 'bold')?.weight).toBe(700);
    expect(d.blocks[1].runs.find((r) => r.text === 'slanted')?.slant).toBe('italic');
    expect(d.blocks[2].text).toBe('A future block');
    expect([d.kind, d.kicker, d.dek]).toEqual(['story', 'Autopaper', 'A dek.']);
    expect(d.disclosure.startsWith('Written by AI.')).toBe(true);
  });
  test('P2 markdown fallback; the heading equal to the title is dropped; nothing is a visible empty block', () => {
    const o = { publication: { title: 'T' }, artifact: { content: '# T\n\nOne **two** three.\n\n- a\n- b\n\n1. x\n2. y' }, nav: {} };
    const d = parseBundle(o, 'pub:t');
    expect(d.blocks.map((b) => b.kind)).toEqual(['p', 'li', 'li', 'li', 'li']);
    expect(d.blocks[1].marker).toBe('•'); expect(d.blocks[3].marker).toBe('1.');
    expect(d.blocks[0].runs.find((r) => r.text === 'two')?.weight).toBe(700);
    const e = parseBundle({ publication: { title: 'T' }, artifact: { body_doc: 'not json' }, payload: {} }, 'pub:t');
    expect(e.status).toBe('ready'); expect(e.blocks.length).toBe(1); expect(e.blocks[0].text).toContain('no text yet');
  });
  test('P3 lists, quotes, code', () => {
    const body = bodyDoc(`{"type":"ordered_list","attrs":{"id":"o","start":3},"content":[
      {"type":"list_item","content":[${p('Three')},{"type":"bullet_list","content":[{"type":"list_item","content":[${p('Nested')}]}]}]},
      {"type":"list_item","content":[${p('Four')},${p('More of four')}]}]},
      {"type":"blockquote","content":[${p('Said one.')},{"type":"blockquote","content":[${p('Said two.')}]}]},
      {"type":"code_block","content":[{"type":"text","text":"let a = 1\\n  let b = 2"}]}`);
    const d = parseBundle(bundle({ body }), 'pub:x');
    expect(d.blocks.map((b) => b.kind)).toEqual(['li', 'li', 'li', 'li', 'quote', 'quote', 'code']);
    expect(d.blocks[0].marker).toBe('3.'); expect(d.blocks[1].depth).toBe(2); expect(d.blocks[2].marker).toBe('4.'); expect(d.blocks[3].marker).toBe('');
    expect([d.blocks[4].depth, d.blocks[5].depth]).toEqual([1, 2]);
    expect(d.blocks[6].text).toBe('let a = 1\n  let b = 2');
  });
  test('P4 citations: numbers, unique anchors, a missing entity, the quote in its block, the check word', () => {
    const body = bodyDoc(`{"type":"paragraph","content":[{"type":"text","text":"Claim one."},{"type":"source_ref","attrs":{"id":"a1","source_entity_id":"e1"}},{"type":"text","text":" Claim two."},{"type":"source_ref","attrs":{"id":"a2","source_entity_id":"e2"}},{"type":"source_ref","attrs":{"id":"a1","source_entity_id":"e1"}},{"type":"source_ref","attrs":{"source_entity_id":"gone"}}]}`);
    const ents = '[{"source_entity_id":"e1","entity":{"target":{"kind":"web_url","uri":"https://www.Example.com:8080/a/b"},"display":{"title":"Example story"}}},{"source_entity_id":"e2","entity":{"target":{"kind":"web_url","uri":"https://news.example.org/x"},"display":{"label":"Other"}}}]';
    const tr = '[{"anchor_id":"a1","source_entity_id":"e1","snapshot_text":"the exact words","ref":{"cid":"c1","quote":"the exact words"}}]';
    const resolved = '[{"cid":"c1","text":"Before it. The Exact  WORDS here. After it.","url":"https://example.com/a/b","title":"Example story"}]';
    const d = parseBundle(bundle({ body, entities: ents, trans: tr, checks: '[{"anchor_id":"a1","state":"EXACT"}]', resolved }), 'pub:x');
    expect(d.cites.length).toBe(4);
    expect(new Set(d.cites.map((c) => c.anchor)).size).toBe(4);
    const c1 = d.cites[0];
    expect([c1.number, c1.host, c1.title, c1.check, c1.checkLabel]).toEqual([1, 'example.com', 'Example story', 'exact', 'Exact quote']);
    expect([c1.before, c1.hit, c1.after]).toEqual(['Before it. ', 'The Exact  WORDS', ' here. After it.']);
    expect(d.cites[1].number).toBe(2); expect(d.cites[2].number).toBe(1); expect(d.cites[3].title).toBe('Source unavailable');
    expect(d.sources.map((s) => s.number)).toEqual([1, 2, 3]);
    expect(d.blocks[0].cites.length).toBe(4);
    expect(d.blocks[0].runs.filter((r) => r.kind === 'cite').map((r) => r.label)).toEqual(['1', '2', '1', '3']);
  });
  test('P5 a Texture transclusion is a ref to texture:', () => {
    const body = bodyDoc('{"type":"paragraph","content":[{"type":"text","text":"See also"},{"type":"source_ref","attrs":{"id":"t1","source_entity_id":"tx"}}]}');
    const ents = '[{"source_entity_id":"tx","entity":{"target":{"kind":"texture_span","uri":"texture:doc-9@rev-2"},"display":{"title":"Battery explainer"}}}]';
    const d = parseBundle(bundle({ body, entities: ents, trans: '[{"anchor_id":"t1","snapshot_text":"Storage shifts the peak."}]' }), 'pub:x');
    expect([d.cites[0].kind, d.cites[0].target, d.cites[0].quote]).toEqual(['texture', 'texture:doc-9@rev-2', 'Storage shifts the peak.']);
    expect(d.blocks[0].runs[d.blocks[0].runs.length - 1].kind).toBe('ref');
  });
  test('P6 sentences come from the plain text; a cite belongs to the sentence it closes; run ids are unique', () => {
    const body = bodyDoc('{"type":"paragraph","content":[{"type":"text","text":"It rained on Monday."},{"type":"source_ref","attrs":{"id":"a","source_entity_id":"e"}},{"type":"text","text":" Then it stopped. The end."}]}');
    const d = parseBundle(bundle({ body, entities: '[{"source_entity_id":"e","entity":{"target":{"kind":"web_url","uri":"https://e.example.com"},"display":{"title":"E"}}}]' }), 'pub:x');
    const b = d.blocks[0];
    expect(b.sentences).toBe(3);
    expect(b.runs.find((r) => r.kind === 'cite')?.sent).toBe(0);
    expect(sentenceTexts(b)).toEqual(['It rained on Monday.', 'Then it stopped.', 'The end.']);
    expect(b.runs[b.runs.length - 1].sent).toBe(2);
    expect(new Set(b.runs.map((r) => r.id)).size).toBe(b.runs.length);
  });
  test('P7 markdown tables; unmatched markers stay literal; links', () => {
    const md = 'Intro with snake_case and 5 * 3 and an *unmatched star.\n\n| Name | Count |\n|---|---|\n| Apples | 3 |\n| Pears |\n\n```\nunterminated code\n  kept';
    const blocks = mdBlocks(md);
    expect(blocks.map((b) => b.kind)).toEqual(['p', 'table', 'code']);
    expect(blocks[1].rows.length).toBe(3); expect(blocks[1].rows[2]).toEqual(['Pears']); expect(blocks[2].text.endsWith('  kept')).toBe(true);
    const runs = mdInline(blocks[0].text, 'p');
    expect(runs.map((r) => r.text).join('')).toBe(blocks[0].text); expect(runs.every((r) => r.slant === 'normal')).toBe(true);
    expect(mdInline('see [the docs](https://example.com/d) now', 'l').find((r) => r.kind === 'link')?.href).toBe('https://example.com/d');
  });
  test('P8 versions from nav, clamped', () => {
    const one = bodyDoc(p('x'));
    const d = parseBundle(bundle({ body: one, nav: '{"revision":2,"prev":1,"next":3,"latest":3}' }), 'pub:headline/v2');
    expect([d.revision, d.latest, d.newer]).toEqual([2, 3, true]);
    expect(d.versions.map((v) => v.number)).toEqual([3, 2, 1]); expect(d.versions[0].address).toBe('pub:headline/v3'); expect(d.versions[1].current).toBe(true);
    expect(d.meta).toBe('Version 2 of 3 · Nov 10, 2026');
    const c = parseBundle(bundle({ body: one, nav: '{"revision":9,"latest":2}' }), 'pub:headline');
    expect(c.revision).toBe(2); expect(c.newer).toBe(false);
    expect(parseBundle(bundle({ body: one, nav: '{"revision":1,"latest":99999999}' }), 'pub:headline').versions.length).toBe(500);
  });
  test('P9 addresses', () => {
    expect(parseAddress('pub:sample-x/v3').kind).toBe('pub');
    expect(splitRoute('sample-x/v3')).toEqual({ bare: 'sample-x', version: 3 }); expect(splitRoute('sample-x/vivid').version).toBe(0);
    expect(parseAddress('texture:doc1@rev7')).toMatchObject({ kind: 'texture', doc: 'doc1', rev: 'rev7' });
    expect(parseAddress('texture:@x').kind).toBe('none'); expect(parseAddress('pub:').kind).toBe('none'); expect(parseAddress('paper:front').kind).toBe('none');
  });
  test('P10 hosts, check words, the quote in its block', () => {
    expect(hostOf('https://user:pw@WWW.Example.com:8443/p')).toBe('example.com'); expect(hostOf('example.org/path')).toBe('example.org');
    expect(hostOf('not a url')).toBe(''); expect(hostOf('')).toBe('');
    expect(checkLabel('').state).toBe('pending'); expect(checkLabel('TOLERANT').label).toBe('Close match'); expect(checkLabel('weird').label).toBe('Not yet checked');
    const c = contextOf('it’s “fine” … done', 'Start. It\'s "fine" and then some, done. End.');
    expect(c).toEqual({ before: 'Start. ', hit: 'It\'s "fine" and then some, done', after: '. End.' });
    expect(contextOf('absent words', 'Some other text.')).toEqual({ before: 'Some other text.', hit: '', after: '' });
    expect(contextOf('', 'abc').hit).toBe(''); expect(contextOf('a very long quote that cannot fit', 'short').hit).toBe('');
  });
  test('P11 a private document: versions newest first, revising, an old revision is newer-marked', () => {
    const rev = { revision_id: 'r2', version_number: 1, content: '## Notes\n\nSome text.', created_at: '2026-11-09T10:00:00Z' };
    const doc = { doc_id: 'd1', title: 'Plan', agent_revision_pending: true, trajectory_id: 'traj-1' };
    const revs = [{ revision_id: 'r2', version_number: 1, author_label: 'appagent', created_at: '2026-11-09T10:00:00Z' }, { revision_id: 'r1', version_number: 0, author_label: 'you', created_at: '2026-11-08T10:00:00Z' }];
    const d = parseRevision(rev, doc, 'texture:d1', revs);
    expect([d.title, d.kind, d.revision, d.latest, d.newer, d.revising, d.trajectory]).toEqual(['Plan', 'document', 2, 2, false, true, 'traj-1']);
    expect(d.versions.map((v) => v.address)).toEqual(['texture:d1', 'texture:d1@r1']); expect(d.versions[0].author).toBe('appagent');
    expect(d.blocks.map((b) => b.kind)).toEqual(['h', 'p']); expect(d.revisingNote).toBe('The desk is writing a new version.');
    const old = parseRevision({ revision_id: 'r1', version_number: 0, content: 'Old.' }, doc, 'texture:d1@r1', revs);
    expect([old.revision, old.newer, old.historical]).toEqual([1, true, true]);
  });
  test('P12 a long document is linear', () => {
    const paras = Array.from({ length: 3000 }, (_, i) => p(`Sentence number ${i} is here. And another one follows.`));
    const t = Date.now();
    const d = parseBundle(bundle({ body: bodyDoc(paras.join(',')) }), 'pub:x');
    expect(d.blocks.length).toBe(3000); expect(Date.now() - t).toBeLessThan(5000); expect(d.blocks[2999].ordinal).toBe(2999);
  });
  test('P14 tables, embeds, legacy bold, adjacent published citations of one source collapse', () => {
    const body = bodyDoc(`{"type":"table","content":[{"type":"table_row","content":[{"type":"table_header","content":[{"type":"paragraph","content":[{"type":"text","text":"A"}]}]},{"type":"table_header","content":[{"type":"paragraph","content":[{"type":"text","text":"B"}]}]}]},{"type":"table_row","content":[{"type":"table_cell","content":[{"type":"paragraph","content":[{"type":"text","text":"1"}]}]},{"type":"table_cell","content":[{"type":"paragraph","content":[{"type":"text","text":"2"}]}]}]}]},
      {"type":"embed","attrs":{"kind":"audio","title":"A song","alt":"alt text"}},
      {"type":"embed","attrs":{"kind":"image","alt":"harbour"}},
      {"type":"paragraph","content":[{"type":"text","text":"say **loud** now"},{"type":"text","text":"x"},{"type":"source_ref","attrs":{"id":"m1","source_entity_id":"e"}},{"type":"source_ref","attrs":{"id":"m2","source_entity_id":"e"}}]}`);
    const ents = '[{"source_entity_id":"e","entity":{"target":{"kind":"web_url","uri":"https://e.example.com"},"display":{"title":"E"}}}]';
    const d = parseBundle({ ...bundle({ body, entities: ents }), payload: { manifest: { anchors: [{ anchor_id: 'm1', ref: { doc: 'd' } }, { anchor_id: 'm2', ref: { doc: 'd' } }] } } }, 'pub:x');
    expect(d.blocks[0].kind).toBe('table'); expect(d.blocks[0].rows.map((r) => r.cells.map((c) => c.runs[0].text))).toEqual([['A', 'B'], ['1', '2']]); expect(d.blocks[0].rows[0].cells[0].header).toBe(true);
    expect([d.blocks[1].kind, d.blocks[1].text, d.blocks[1].marker]).toEqual(['embed', 'A song', 'audio']); expect(d.blocks[2].text).toBe('harbour');
    const para = d.blocks[3];
    expect(para.runs.find((r) => r.text === 'loud')?.weight).toBe(700); expect(para.runs.filter((r) => r.kind === 'cite').length).toBe(1);
    expect(d.cites.length).toBe(2);
  });
  test('P15 related reads links, then the older guesses; a story with no route or title is dropped', () => {
    expect(parseRelated({ links: [{ title: 'A', route: '/pub/a', dek: 'd' }, { title: '', route: 'x' }, { title: 'C', route: { bare: 'pub/c' } }, { title: 'D' }] })).toEqual([{ address: 'pub:pub/a', title: 'A', dek: 'd' }, { address: 'pub:pub/c', title: 'C', dek: '' }]);
    expect(parseRelated({ related: [{ title: 'Z', route: 'z' }] }).length).toBe(1); expect(parseRelated({})).toEqual([]);
  });
});

describe('golden documents', () => {
  const dir = '../../internal/texturedoc/golden';
  const squash = (s: string) => s.split(/[\s ]+/).filter((x) => x !== '').join(' ');
  type R = ReturnType<typeof parseBundle>['blocks'][number]['runs'][number];
  const style = (r: R) => [r.kind === 'code' ? 'code' : '', r.weight >= 700 ? 'strong' : '', r.slant === 'italic' ? 'emphasis' : ''].filter((x) => x).sort().join('+');
  function inline(runs: R[], doc: ReturnType<typeof parseBundle>, checks: boolean) {
    let text = ''; const spans: ({ style: string; text: string } | null)[] = []; const cites: Record<string, unknown>[] = [];
    for (const r of runs) {
      if (r.kind === 'cite' || r.kind === 'ref') {
        const n = parseInt(r.label, 10) || 0; text += `[${n}]`; spans.push(null);
        const c = doc.cites.find((x) => x.anchor === r.anchor);
        const o: Record<string, unknown> = { n, title: squash(c?.title ?? ''), quote: squash(c?.quote ?? '') }; if (checks) o.check = c?.check ?? ''; cites.push(o);
      } else if (r.kind === 'br') { text += '\n'; spans.push(null); }
      else {
        text += r.text; const st = style(r);
        if (st === '') { if (squash(r.text) !== '') spans.push(null); }
        else { const l = spans[spans.length - 1]; if (l && l.style === st) l.text += r.text; else spans.push({ style: st, text: r.text }); }
      }
    }
    return { text: squash(text), spans: spans.filter((s): s is { style: string; text: string } => s !== null).map((s) => ({ style: s.style, text: squash(s.text) })).filter((s) => s.text !== ''), cites };
  }
  function project(doc: ReturnType<typeof parseBundle>, checks: boolean) {
    const out: Record<string, unknown>[] = [];
    for (const b of doc.blocks) {
      const o: Record<string, unknown> = {};
      switch (b.kind) {
        case 'h': o.kind = 'heading'; o.level = b.level; if (b.depth > 0) o.depth = b.depth; break;
        case 'p': o.kind = 'paragraph'; break;
        case 'li': o.kind = 'list_item'; o.depth = b.depth; o.marker = b.marker === '•' || b.marker === '–' ? 'bullet' : b.marker; break;
        case 'quote': o.kind = 'quote'; o.depth = b.depth; break;
        case 'code': out.push({ kind: 'code', text: b.text.replace(/\n+$/, '') }); continue;
        case 'rule': out.push({ kind: 'rule', text: '' }); continue;
        case 'table': out.push({ kind: 'table', text: '', rows: b.rows.map((r) => r.cells.map((c) => inline(c.runs, doc, false).text)) }); continue;
        case 'embed': out.push({ kind: 'embed', embed: b.marker, text: squash(b.text) }); continue;
        default: o.kind = b.kind;
      }
      const i = inline(b.runs, doc, checks); o.text = i.text;
      if (i.spans.length > 0) o.spans = i.spans; if (i.cites.length > 0) o.cites = i.cites;
      out.push(o);
    }
    return out;
  }
  const canon = (v: unknown): string => (Array.isArray(v) ? `[${v.map(canon).join(',')}]` : v && typeof v === 'object' ? `{${Object.keys(v).sort().map((k) => `${JSON.stringify(k)}:${canon((v as Record<string, unknown>)[k])}`).join(',')}}` : JSON.stringify(v));
  const names = readdirSync(dir).filter((f) => f.endsWith('.json') && f !== 'known-differences.json');
  test('P13 there are golden documents', () => { expect(names.length).toBeGreaterThanOrEqual(8); });
  for (const f of names) {
    test(`P13 ${f} projects to its committed projection`, () => {
      const o = JSON.parse(readFileSync(`${dir}/${f}`, 'utf8')); const g = o.golden;
      const doc = parseBundle(o, `pub:${o.route?.bare ?? g.name}`);
      const want = JSON.parse(readFileSync(`${dir}/expected/${f}`, 'utf8'));
      expect(doc.status).toBe('ready');
      expect(canon(project(doc, g.checks === true))).toBe(canon(want.blocks));
    });
  }
});
