import { test, expect } from './helpers/fixtures.js';
import {
  projectStructuredTextureDocText,
  renderLegacyStrong,
  renderStructuredTextureDocHTML,
  serializeEditorStructuredDoc,
  sourceEntitiesForStructuredDoc,
} from '../src/lib/texture-structured-editor-doc.ts';
import { buildPublishedCitationContext } from '../src/lib/texture-published-citations.ts';

const ELEMENT_NODE = 1;
const TEXT_NODE = 3;

function text(value) {
  return {
    nodeType: TEXT_NODE,
    textContent: value,
  };
}

function element(tagName, attrs = {}, children = []) {
  const node = {
    nodeType: ELEMENT_NODE,
    tagName,
    childNodes: children,
    children: children.filter((child) => child.nodeType === ELEMENT_NODE),
    textContent: children.map((child) => child.textContent || '').join(''),
    getAttribute(name) {
      return attrs[name] || '';
    },
    matches(selector) {
      const attr = selector.match(/^\[([^\]]+)\]$/)?.[1];
      return attr ? Object.prototype.hasOwnProperty.call(attrs, attr) : false;
    },
  };
  return node;
}

function sourceEntity(id = 'src-1') {
  return {
    source_entity_id: id,
    target: {
      kind: 'web_source',
      uri: 'https://example.com/carrier-advisory',
    },
    display: {
      title: 'Carrier service advisory',
      mode: 'numbered_ref',
    },
    evidence: {
      state: 'confirms',
      open_surface: 'source',
    },
    provenance: {
      created_by: 'frontend-test',
    },
  };
}

test('editor source ref atoms serialize as StructuredTextureDoc source_ref nodes', () => {
  const root = element('div', {}, [
    element('p', {}, [
      text('Lead '),
      element('span', {
        'data-texture-source-ref': '',
        'data-source-entity-id': 'src-1',
        'data-source-label': 'Carrier service advisory',
      }),
      text(' holds.'),
    ]),
  ]);

  const bodyDoc = serializeEditorStructuredDoc(root);

  expect(bodyDoc.schema).toBe('choir.texture_doc.v1');
  expect(bodyDoc.doc.content[0].content[1]).toMatchObject({
    type: 'source_ref',
    attrs: {
      source_entity_id: 'src-1',
      display_mode: 'numbered_ref',
    },
  });
  expect(projectStructuredTextureDocText(bodyDoc)).toBe('Lead [1] holds.');
  expect(JSON.stringify(bodyDoc)).not.toContain('[Carrier service advisory](source:src-1)');
  expect(JSON.stringify(bodyDoc)).not.toContain('source:src-1');
});

test('structured docs render source refs as native non-link citation atoms', () => {
  const bodyDoc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'doc-structured-render' },
      content: [{
        type: 'paragraph',
        attrs: { id: 'p-1' },
        content: [
          { type: 'text', text: 'Lead ' },
          {
            type: 'source_ref',
            attrs: {
              id: 'ref-1',
              source_entity_id: 'src-1',
              display_mode: 'numbered_ref',
            },
          },
          { type: 'text', text: ' holds.' },
        ],
      }],
    },
  };

  const html = renderStructuredTextureDocHTML(bodyDoc, [sourceEntity()]);

  expect(html).toContain('data-texture-source-ref');
  expect(html).toContain('data-texture-source-node-id="ref-1"');
  expect(html).toContain('data-source-entity-id="src-1"');
  expect(html).not.toContain('href=');
  expect(html).not.toContain('(source:src-1)');
  expect(html).not.toContain('source:src-1');
});

// An embed ({{embed}}, display_mode expanded_ref) is a transclusion: the same
// inline numbered marker a cite gets, never the source's text as a block on
// the page (docs/problems/published-embeds-republish-source-text-2026-10-16.md).
test('structured docs render expanded_ref source refs as an inline numbered marker', () => {
  const bodyDoc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'doc-source-expanded-ref-render' },
      content: [{
        type: 'paragraph',
        attrs: { id: 'p-expanded-ref' },
        content: [{
          type: 'source_ref',
          attrs: {
            id: 'ref-expanded',
            source_entity_id: 'src-expanded',
            display_mode: 'expanded_ref',
          },
        }],
      }],
    },
  };
  const entity = {
    ...sourceEntity('src-expanded'),
    selectors: [{
      kind: 'text_quote',
      data: { exact: 'Expanded source excerpt should be visible.' },
    }],
    reader_snapshot: {
      text_content: 'Expanded source excerpt should be visible.\n\nFull source viewer text remains available separately.',
    },
  };

  const html = renderStructuredTextureDocHTML(bodyDoc, [entity]);

  expect(html).toMatch(/^<p><span class="texture-source-ref" data-texture-source-ref[^>]*data-source-display-mode="expanded_ref"/);
  expect(html).toContain('data-texture-source-node-id="ref-expanded"');
  expect(html).toContain('<span class="texture-source-ref-label">1</span>');
  // The popover span stays for existing readers, but the reading surface is
  // the plain note under the paragraph, inside the citation notes block.
  // Without a published citation context there is no note; the marker keeps
  // the source's title in its (never shown) popover, exactly as before.
  expect(html).toMatch(/data-texture-source-ref-popover[^]*Carrier service advisory[^]*Open source/);
  expect(html).not.toContain('data-embed-block');
  expect(html).not.toContain('texture-source-ref-block');
  expect(html).not.toContain('Full source viewer text');
  expect(html).not.toContain('source:src-expanded');
});

test('structured doc source entities keep only attached entities', () => {
  const bodyDoc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'doc-entity-filter' },
      content: [{
        type: 'paragraph',
        attrs: { id: 'p-1' },
        content: [{
          type: 'source_ref',
          attrs: {
            id: 'ref-1',
            source_entity_id: 'src-1',
            display_mode: 'numbered_ref',
          },
        }],
      }],
    },
  };

  expect(sourceEntitiesForStructuredDoc(bodyDoc, [sourceEntity('src-1'), sourceEntity('src-detached')])).toEqual([
    sourceEntity('src-1'),
  ]);
});

// Write-era citations (docs/problems/prompt-bar-research-slow-unsynthesized-
// mangled-2026-10-14.md). A Write citation carries its quote in attrs.ref and
// its entity's selector is the whole resource, so an unpublished renderer
// that reads only the entity's excerpt shows an empty popover. An {{embed}}
// was drawn as a block of the source's text; it is now the same inline
// marker as a cite (docs/problems/published-embeds-republish-source-text-
// 2026-10-16.md). Ways it can fail:
//   1. the numbered cite's popover does not carry the citation's own quote;
//   2. the embed splits its paragraph, prints the source's text on the page,
//      or loses its quote, its number or its title from the popover;
//   3. the embed does not serialize back to the same expanded_ref citation;
//   4. bold that reached a text node as literal ** stays literal, or
//      asterisks that are not emphasis turn bold, or marked text is touched.
function writeEntity(id, title, url) {
  return {
    source_entity_id: id,
    target: { kind: 'web_url', uri: url, id: `cap-${id}`, metadata: { capture_doc: `cap-${id}`, capture_version: 'v1' } },
    selectors: [{ kind: 'whole_resource' }],
    display: { mode: 'numbered_ref', title },
    evidence: { state: 'available', open_surface: 'source' },
    provenance: { created_by: 'texture_write' },
  };
}

function writeRef(id, entityID, mode, quote) {
  return {
    type: 'source_ref',
    attrs: {
      id,
      source_entity_id: entityID,
      display_mode: mode,
      ref: { doc: `cap-${entityID}`, version: 'v1', cid: 'c3', hash: 'h', quote },
      quote_tier: 'exact',
    },
  };
}

function writeDoc(content) {
  return { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'doc-write' }, content } };
}

test('a Write citation popover shows the citation quote', () => {
  const html = renderStructuredTextureDocHTML(writeDoc([{
    type: 'paragraph',
    attrs: { id: 'p-1' },
    content: [
      { type: 'text', text: 'Haiku 5.5 costs about 75% less to run than Haiku 4.5' },
      writeRef('ref-1', 'src_a', 'numbered_ref', 'On average, it now costs around 75% less to run.'),
      { type: 'text', text: '.' },
    ],
  }]), [writeEntity('src_a', 'Introducing Claude Haiku 5.5', 'https://example.com/haiku')]);

  expect(html).toContain('data-texture-source-node-id="ref-1"');
  // Without a published citation context there is no note: the marker keeps the
  // citation's quote in its (never shown) popover, exactly as before.
  expect(html).toMatch(/data-texture-source-ref-popover[^]*texture-transclusion-quote[^]*On average, it now costs around 75% less to run\./);
});

test('a Write embed renders inline as a numbered marker whose popover carries its quote', () => {
  const entity = writeEntity('src_b', 'Execution of Follow-on Investment', 'https://example.com/softbank');
  const bodyDoc = writeDoc([{
    type: 'paragraph',
    attrs: { id: 'p-2' },
    content: [
      { type: 'text', text: 'SoftBank closed its third tranche.' },
      writeRef('ref-embed', 'src_b', 'expanded_ref', 'SoftBank completed the third tranche of its follow-on investment.'),
      { type: 'text', text: ' The round values the company higher.' },
    ],
  }]);
  const html = renderStructuredTextureDocHTML(bodyDoc, [entity]);

  expect(html.startsWith('<p>SoftBank closed its third tranche.<span class="texture-source-ref"')).toBe(true);
  expect(html).toMatch(/<span class="texture-source-ref"[^>]*data-source-display-mode="expanded_ref"[^>]*>/);
  expect(html).toContain('data-texture-source-node-id="ref-embed"');
  expect(html).toMatch(/ The round values the company higher\.<\/p>$/);
  expect(html).toContain('<span class="texture-source-ref-label">1</span>');
  expect(html).toMatch(/data-texture-source-ref-popover[^]*Execution of Follow-on Investment[^]*texture-transclusion-quote[^]*SoftBank completed the third tranche of its follow-on investment\./);
  expect(html).not.toContain('data-embed-block');
  expect(html).not.toContain('<blockquote');

  const marker = element('span', {
    'data-texture-source-ref': '',
    'data-texture-source-node-id': 'ref-embed',
    'data-source-entity-id': 'src_b',
    'data-source-display-mode': 'expanded_ref',
  }, [text('1')]);
  const roundTrip = serializeEditorStructuredDoc(element('div', {}, [element('p', {}, [text('SoftBank closed its third tranche.'), marker])]));
  expect(roundTrip.doc.content[0]).toMatchObject({
    type: 'paragraph',
    content: [
      { type: 'text', text: 'SoftBank closed its third tranche.' },
      { type: 'source_ref', attrs: { id: 'ref-embed', source_entity_id: 'src_b', display_mode: 'expanded_ref' } },
    ],
  });
});

// A published embed pins capture blocks, which keep page furniture ("Save",
// "Share", photo credits). The reader shows the marker; the pinned blocks
// never reach the page, and a quoted embed's popover marks its quote.
test('a published embed renders as an inline marker and never prints its pinned blocks', () => {
  const bodyDoc = writeDoc([{
    type: 'paragraph',
    attrs: { id: 'p-pub' },
    content: [
      { type: 'text', text: 'Commercial goods stay out.' },
      { type: 'source_ref', attrs: { id: 'ref-whole', source_entity_id: 'src_g', display_mode: 'expanded_ref', ref: { doc: 'cap-g', version: 'v1' } } },
      { type: 'text', text: ' Aid groups object.' },
      writeRef('ref-quoted', 'src_b', 'expanded_ref', 'We cannot go back'),
      { type: 'text', text: ' Talks continue.' },
    ],
  }]);
  const bundle = {
    artifact: { body_doc: bodyDoc },
    payload: {
      manifest: JSON.stringify({ anchors: [
        { anchor_id: 'ref-whole', relation: 'embeds', ref: { doc: 'cap-g', version: 'v1' }, cids: ['g1', 'g2', 'g3'] },
        { anchor_id: 'ref-quoted', relation: 'embeds', ref: { doc: 'cap-src_b', version: 'v1', cid: 'c3', quote: 'We cannot go back' }, cids: ['c3'] },
      ] }),
      resolved_blocks: [
        { doc: 'cap-g', cid: 'g1', text: 'Save', kind: 'p', url: 'https://example.com/gaza', title: 'Israel halts commercial goods' },
        { doc: 'cap-g', cid: 'g2', text: 'Share', kind: 'p' },
        { doc: 'cap-g', cid: 'g3', text: 'Image caption, a charity kitchen', kind: 'p' },
        { doc: 'cap-src_b', cid: 'c3', text: 'Pires said: We cannot go back to that.', kind: 'p', url: 'https://example.com/unicef', title: 'UNICEF briefing' },
      ],
    },
    checks: { citations: [{ anchor_id: 'ref-quoted', state: 'exact' }] },
  };
  const citations = buildPublishedCitationContext(bundle, []);
  const html = renderStructuredTextureDocHTML(bodyDoc, [], { citations });

  expect(html.startsWith('<p>Commercial goods stay out.<span class="texture-source-ref')).toBe(true);
  // The sentence keeps its markers, and the citations' notes follow it.
  expect(html).toMatch(/ Talks continue\.<\/p><div class="texture-citation-notes"[^]*data-texture-source-note[^]*<\/aside><\/div>$/);
  expect(html).not.toContain('data-embed-block');
  expect(html).not.toContain('Save');
  expect(html).not.toContain('Share');
  expect(html).not.toContain('Image caption');
  expect(html).toMatch(/data-citation-anchor="ref-whole"[^>]*>\s*<span class="texture-source-ref-label">1<\/span>[^]*Israel halts commercial goods[^]*Open source/);
  expect(html).toMatch(/data-citation-anchor="ref-quoted"[^>]*>\s*<span class="texture-source-ref-label">2<\/span>[^]*<mark class="texture-citation-quote"[^>]*>We cannot go back<\/mark>/);
});

test('literal ** in an unmarked text node renders bold; other asterisks and marked text stay as they are', () => {
  expect(renderLegacyStrong('**OpenAI — GPT-6 &quot;Astra&quot;.** The most measured.')).toBe('<strong>OpenAI — GPT-6 &quot;Astra&quot;.</strong> The most measured.');
  expect(renderLegacyStrong('Text **A** and **B.** end')).toBe('Text <strong>A</strong> and <strong>B.</strong> end');
  expect(renderLegacyStrong('5 ** 3 and a ** b')).toBe('5 ** 3 and a ** b');
  expect(renderLegacyStrong('snake**case**words')).toBe('snake**case**words');
  expect(renderLegacyStrong('an unclosed **bold')).toBe('an unclosed **bold');

  const html = renderStructuredTextureDocHTML(writeDoc([
    { type: 'paragraph', attrs: { id: 'p-legacy' }, content: [{ type: 'text', text: '**EU AI Act enforcement begins.** The Commission enforces it.' }] },
    { type: 'paragraph', attrs: { id: 'p-marked' }, content: [{ type: 'text', text: 'Marked', marks: [{ type: 'strong' }] }, { type: 'text', text: ' text.' }] },
  ]), []);
  expect(html).toContain('<p><strong>EU AI Act enforcement begins.</strong> The Commission enforces it.</p>');
  expect(html).toContain('<p><strong>Marked</strong> text.</p>');
  expect(html).not.toContain('**');
});

// Citation notes (owner direction, 2026-10-27): a numbered marker is a toggle
// whose note is plain text in the flow under the paragraph that cites it.
// The note is rendered chrome, never document content: it must never
// serialize back into the doc, or a save would type the notes into the text.
test('citation markers carry toggle state and the note sits under its paragraph', () => {
  const bodyDoc = writeDoc([
    {
      type: 'paragraph',
      attrs: { id: 'p-a' },
      content: [
        { type: 'text', text: 'First claim' },
        writeRef('ref-a', 'src_a', 'numbered_ref', 'First quote.'),
        { type: 'text', text: ' and second claim' },
        writeRef('ref-b', 'src_b', 'numbered_ref', 'Second quote.'),
        { type: 'text', text: '.' },
      ],
    },
    { type: 'paragraph', attrs: { id: 'p-b' }, content: [{ type: 'text', text: 'Another paragraph.' }] },
  ]);
  const bundle = {
    artifact: { body_doc: bodyDoc },
    payload: {
      manifest: JSON.stringify({ anchors: [
        { anchor_id: 'ref-a', relation: 'cites', ref: { doc: 'cap-a', version: 'v1', cid: 'a1', quote: 'First quote.' }, cids: ['a1'] },
        { anchor_id: 'ref-b', relation: 'cites', ref: { doc: 'cap-b', version: 'v1', cid: 'b1', quote: 'Second quote.' }, cids: ['b1'] },
      ] }),
      resolved_blocks: [
        { doc: 'cap-a', cid: 'a1', text: 'First quote.', kind: 'p', url: 'https://example.com/a', title: 'Source A' },
        { doc: 'cap-b', cid: 'b1', text: 'Second quote.', kind: 'p', url: 'https://example.com/b', title: 'Source B' },
      ],
    },
    checks: { citations: [
      { anchor_id: 'ref-a', state: 'exact', quote: 'exact' },
      { anchor_id: 'ref-b', state: 'exact', quote: 'exact' },
    ] },
  };
  const citations = buildPublishedCitationContext(bundle, []);
  const html = renderStructuredTextureDocHTML(bodyDoc, [], { citations });

  // The markers stay inside their sentence and report their toggle state.
  const firstParagraph = html.slice(0, html.indexOf('</p>'));
  expect(firstParagraph).toContain('data-citation-anchor="ref-a"');
  expect(firstParagraph).toContain('data-citation-anchor="ref-b"');
  expect(firstParagraph).not.toContain('data-texture-source-note');
  expect(html).toMatch(/data-citation-anchor="ref-a"[^>]*aria-expanded="true"[^>]*aria-pressed="true"[^>]*aria-controls="texture-source-note-ref-a"/);

  // One notes block follows the paragraph, one note per citation, in order;
  // each note is a direct child of that block, so the note reads as sitting
  // under the paragraph that cites it.
  expect(html).toContain('</p><div class="texture-citation-notes" data-texture-citation-notes');
  const notesBlock = html.slice(html.indexOf('data-texture-citation-notes'));
  const noteOrder = Array.from(notesBlock.matchAll(/data-texture-source-note data-citation-anchor="([^"]+)"/g)).map((m) => m[1]);
  expect(noteOrder).toEqual(['ref-a', 'ref-b']);
  expect(notesBlock).toContain('id="texture-source-note-ref-a"');
  expect(notesBlock).toContain('data-citation-note-open="true"');
  expect(notesBlock).toContain('First quote.');
  expect(notesBlock).toContain('Second quote.');
  // The note carries the live check and a Close button, and no link.
  expect(notesBlock).toContain('data-check-badge="exact"');
  expect(notesBlock).toContain('data-citation-note-close');
  const beforeSecondParagraph = notesBlock.slice(0, notesBlock.indexOf('Another paragraph.') >= 0 ? notesBlock.indexOf('Another paragraph.') : notesBlock.length);
  expect(beforeSecondParagraph).not.toContain('<a ');
  // The second paragraph carries no notes block of its own.
  expect(html.endsWith('Another paragraph.</p>')).toBe(true);
});

test('a rendered citation note never serializes back into the document', () => {
  const note = element('aside', {
    'data-texture-source-note': '',
    'data-citation-anchor': 'ref-a',
    'data-citation-note-open': 'true',
  }, [text('First quote.')]);
  const notes = element('div', { 'data-texture-citation-notes': '' }, [note]);
  const paragraph = element('p', {}, [
    text('First claim'),
    element('span', {
      'data-texture-source-ref': '',
      'data-texture-source-node-id': 'ref-a',
      'data-source-entity-id': 'src_a',
      'data-source-display-mode': 'numbered_ref',
    }, [text('1')]),
    text('.'),
  ]);
  const roundTrip = serializeEditorStructuredDoc(element('div', {}, [paragraph, notes]));

  expect(roundTrip.doc.content).toHaveLength(1);
  expect(roundTrip.doc.content[0]).toMatchObject({
    type: 'paragraph',
    content: [
      { type: 'text', text: 'First claim' },
      { type: 'source_ref', attrs: { id: 'ref-a', source_entity_id: 'src_a', display_mode: 'numbered_ref' } },
      { type: 'text', text: '.' },
    ],
  });
  expect(JSON.stringify(roundTrip)).not.toContain('First quote.');
});
