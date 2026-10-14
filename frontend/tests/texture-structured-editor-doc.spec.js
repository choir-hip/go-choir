import { test, expect } from './helpers/fixtures.js';
import {
  projectStructuredTextureDocText,
  renderLegacyStrong,
  renderStructuredTextureDocHTML,
  serializeEditorStructuredDoc,
  sourceEntitiesForStructuredDoc,
} from '../src/lib/texture-structured-editor-doc.ts';

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

test('structured docs render expanded_ref source refs as a block with preserved source text', () => {
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

  // A transclusion is a block of its own, never a block inside a paragraph
  // (the HTML parser would close the paragraph around it).
  expect(html).toContain('data-embed-block');
  expect(html).toContain('data-texture-source-node-id="ref-expanded"');
  expect(html).toContain('data-source-display-mode="expanded_ref"');
  expect(html).toContain('Carrier service advisory');
  expect(html).toContain('Expanded source excerpt should be visible.');
  expect(html).not.toMatch(/<p>[^]*data-embed-block/);
  expect(html).not.toContain('source:src-expanded');
  expect(html).not.toContain('data-texture-source-embed');
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
// that reads only the entity's excerpt shows an empty popover, and an
// {{embed}} drawn inside its paragraph broke the paragraph: a stray title
// line, then a lone "[n]" paragraph. Ways it can fail:
//   1. the numbered cite's popover does not carry the citation's own quote;
//   2. the embed is drawn inside a <p> (block inside inline) or loses its
//      quote, its number or its title;
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
  expect(html).toMatch(/data-texture-source-ref-popover[^]*texture-transclusion-quote[^]*On average, it now costs around 75% less to run\./);
});

test('a Write embed renders as its own block with its quote, number and title', () => {
  const entity = writeEntity('src_b', 'Execution of Follow-on Investment', 'https://example.com/softbank');
  const bodyDoc = writeDoc([{
    type: 'paragraph',
    attrs: { id: 'p-2' },
    content: [
      { type: 'text', text: 'SoftBank closed its third tranche.' },
      writeRef('ref-embed', 'src_b', 'expanded_ref', 'SoftBank completed the third tranche of its follow-on investment.'),
      { type: 'text', text: 'The round values the company higher.' },
    ],
  }]);
  const html = renderStructuredTextureDocHTML(bodyDoc, [entity]);

  expect(html).toContain('<p>SoftBank closed its third tranche.</p>');
  expect(html).toContain('<p>The round values the company higher.</p>');
  expect(html).toMatch(/<\/p>\s*<figure class="texture-embed-block" data-embed-block data-texture-local-embed data-texture-source-node-id="ref-embed"/);
  expect(html).toContain('<blockquote class="texture-embed-quote">SoftBank completed the third tranche of its follow-on investment.</blockquote>');
  expect(html).toContain('<span class="texture-source-number">[1]</span>');
  expect(html).toContain('href="https://example.com/softbank"');
  expect(html).not.toMatch(/<p>[^<]*<span[^>]*data-texture-source-ref[^>]*data-source-display-mode="expanded_ref"/);

  const figure = element('figure', {
    'data-embed-block': '',
    'data-texture-local-embed': '',
    'data-texture-source-node-id': 'ref-embed',
    'data-source-entity-id': 'src_b',
  }, [text('SoftBank completed the third tranche of its follow-on investment. [1] Execution of Follow-on Investment')]);
  const roundTrip = serializeEditorStructuredDoc(element('div', {}, [figure]));
  expect(roundTrip.doc.content[0]).toMatchObject({
    type: 'paragraph',
    content: [{ type: 'source_ref', attrs: { id: 'ref-embed', source_entity_id: 'src_b', display_mode: 'expanded_ref' } }],
  });
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
