/**
 * The Texture reader's lists and Markdown, in a real browser DOM (review of the golden-web change).
 *
 * A body_doc is drawn by renderStructuredTextureDocHTML into the editor surface and serialized back by
 * serializeEditorStructuredDoc when the owner edits. Ways it can fail, written before the code:
 *   1. a list_item that holds a code_block, a blockquote, a heading or a horizontal_rule comes back from
 *      render -> serialize as one paragraph with the block's text appended (the block is lost on save);
 *   2. a list item with a paragraph, a code block and a second paragraph reorders or merges them;
 *   3. a Markdown ordered list that starts at 3 counts from 1;
 *   4. "+ item" is a paragraph in Markdown where the native reader draws a bullet;
 *   5. a source_ref with no source_entity_id takes the number of an entity that has no id either.
 * The modules are loaded by a Vite dev server into a page, so the serializer reads a real DOM.
 */
import { createServer } from 'vite';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let server;
let url;
test.beforeAll(async () => {
  server = await createServer({ root, server: { port: 4181, strictPort: true, hmr: false, watch: null }, logLevel: 'silent' });
  await server.listen();
  url = 'http://localhost:4181/';
});
test.afterAll(async () => { await server?.close(); });
test.use({ storageState: { cookies: [], origins: [] } });

const T = (text, marks) => ({ type: 'text', text, ...(marks ? { marks } : {}) });
const P = (id, ...c) => ({ type: 'paragraph', attrs: { id }, content: c });
const doc = (...content) => ({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content } });
const item = (id, ...blocks) => ({ type: 'list_item', attrs: { id }, content: blocks });
const ul = (id, ...items) => ({ type: 'bullet_list', attrs: { id }, content: items });

const docs = {
  'code in a list item': doc(ul('u', item('i', P('p', T('Run command:')), { type: 'code_block', attrs: { id: 'c' }, content: [T('npm test')] }))),
  'blockquote in a list item': doc(ul('u', item('i', P('p', T('As said:')), { type: 'blockquote', attrs: { id: 'q' }, content: [P('qp', T('Keep it short.'))] }))),
  'paragraph, code, paragraph in one item': doc(ul('u', item('i', P('p1', T('First.')), { type: 'code_block', attrs: { id: 'c' }, content: [T('x = 1')] }, P('p2', T('Second.'))))),
  'heading and rule in a list item': doc(ul('u', item('i', P('p', T('Lead')), { type: 'heading', attrs: { id: 'h', level: 3 }, content: [T('Sub')] }, { type: 'horizontal_rule', attrs: { id: 'r' } }))),
  'nested list and two paragraphs': doc({ type: 'ordered_list', attrs: { id: 'o', start: 3 }, content: [item('i', P('p1', T('One')), P('p2', T('Two')), ul('n', item('ni', P('np', T('inner')))))] }),
};

// ids are the serializer's own; compare the shape: types, levels, start, text, marks.
function shape(node) {
  const out = { type: node.type };
  if (node.attrs?.level) out.level = node.attrs.level;
  if (node.type === 'ordered_list') out.start = node.attrs?.start || 1;
  if (node.text !== undefined) out.text = node.text;
  if (node.marks) out.marks = node.marks.map((m) => m.type);
  if (node.content) out.content = node.content.map(shape);
  return out;
}

for (const [name, body] of Object.entries(docs)) {
  test(`render then serialize keeps ${name}`, async ({ page }) => {
    await page.goto(url);
    const result = await page.evaluate(async (body) => {
      const m = await import('/src/packages/texture/texture-structured-editor-doc.ts');
      const root = document.createElement('div');
      root.innerHTML = m.renderStructuredTextureDocHTML(body, []);
      return m.serializeEditorStructuredDoc(root, { existingDoc: body });
    }, body);
    expect(shape(result.doc)).toEqual(shape(body.doc));
  });
}

test('Markdown: an ordered list keeps its start, + is a bullet', async ({ page }) => {
  await page.goto(url);
  const html = await page.evaluate(async () => {
    const { renderMarkdownBlocks } = await import('/src/lib/texture-markdown-renderer.ts');
    return { three: renderMarkdownBlocks('3. Third\n4. Fourth'), plus: renderMarkdownBlocks('+ Plus item'), one: renderMarkdownBlocks('1. a\n2. b') };
  });
  expect(html.three).toBe('<ol start="3"><li>Third</li><li>Fourth</li></ol>');
  expect(html.plus).toBe('<ul><li>Plus item</li></ul>');
  expect(html.one).toBe('<ol><li>a</li><li>b</li></ol>');
});

test('a source_ref with no entity id does not take an entity that has no id', async ({ page }) => {
  await page.goto(url);
  const labels = await page.evaluate(async () => {
    const m = await import('/src/packages/texture/texture-structured-editor-doc.ts');
    const entity = (id, title) => ({ source_entity_id: id, target: { kind: 'web_url', uri: 'https://example.com/' + title }, display: { mode: 'numbered_ref', title }, evidence: { state: 'available', open_surface: 'source' }, provenance: { created_by: 't' } });
    const orphan = { label: 'orphan' };
    const body = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: [{ type: 'paragraph', attrs: { id: 'p' }, content: [
      { type: 'source_ref', attrs: { id: 'r0' } }, { type: 'source_ref', attrs: { id: 'r1', source_entity_id: 'b' } }] }] } };
    const el = document.createElement('div');
    el.innerHTML = m.renderStructuredTextureDocHTML(body, [orphan, entity('b', 'B')]);
    return [...el.querySelectorAll('[data-texture-source-node-id="r1"] .texture-source-ref-label')].map((n) => n.textContent.trim());
  });
  expect(labels).toEqual(['1']);
});
