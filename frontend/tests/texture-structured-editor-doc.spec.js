import { test, expect } from './helpers/fixtures.js';
import {
  projectStructuredTextureDocText,
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
