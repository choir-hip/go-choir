/**
 * A Texture revision written by choir.Write, rendered in the browser at phone
 * width (docs/problems/prompt-bar-research-slow-unsynthesized-mangled-
 * 2026-10-14.md). The owner's "What's current in ai" v4 showed:
 *   - paragraphs opening with literal "**OpenAI — GPT-6 "Astra".**";
 *   - a stray full-width title line and then a lone "[5]" paragraph, where
 *     an {{embed}} transclusion sat inside a paragraph. Embeds are retired;
 *     a stored one draws as the numbered marker a cite gets
 *     (docs/problems/published-embeds-republish-source-text-2026-10-16.md);
 *   - citation numbers whose quotes were nowhere to be read.
 *
 * The revision is served as a published bundle without citation anchors, so
 * the reader draws it with the same renderer as an owner's unpublished
 * document (renderStructuredTextureDocHTML with no published citations).
 * Ways it can fail:
 *   1. bold reaches the page as "**" characters (a marked text node, or a
 *      legacy unmarked node holding paired "**");
 *   2. the stored embed prints its source's text on the page, or splits its
 *      paragraph (a title line, then a "[n]" paragraph of its own);
 *   3. the embed's marker loses its quoted words, its number or its title;
 *   4. a numbered cite's popover does not carry the citation's own quote;
 *   5. the page scrolls horizontally at phone width.
 * A screenshot of the rendered reader is written to TEXTURE_RENDER_SHOT_DIR
 * (else the test output dir) and attached to the report.
 *
 *   cd frontend && npm run build && npx vite preview --port 4174 --strictPort &
 *   PLAYWRIGHT_BASE_URL=http://localhost:4174 \
 *     npx playwright test tests/texture-write-citations-render.spec.js
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const SHOT_DIR = process.env.TEXTURE_RENDER_SHOT_DIR || '';
const SLUG = 'whats-current-in-ai-pubwrite1';
const ROUTE = `/pub/texture/${SLUG}`;

const CITE_QUOTE = 'On average, it now costs around 75% less to run.';
const EMBED_QUOTE = 'SoftBank Group Corp. completed the third tranche of its follow-on investment in OpenAI.';
const EMBED_TITLE = 'Execution of Follow-on Investment (Third Tranche) in OpenAI | SoftBank Group Corp.';

function entity(id, title, url) {
  return {
    source_entity_id: id,
    target: { kind: 'web_url', uri: url, id: `cap-${id}`, metadata: { capture_doc: `cap-${id}`, capture_version: 'v1' } },
    selectors: [{ kind: 'whole_resource' }],
    display: { mode: 'numbered_ref', title },
    evidence: { state: 'available', open_surface: 'source' },
    provenance: { created_by: 'texture_write' },
  };
}

function ref(id, entityID, mode, quote) {
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

function bundle() {
  const entities = [
    entity('src_haiku', 'Introducing Claude Haiku 5.5', 'https://example.com/haiku-5-5'),
    entity('src_softbank', EMBED_TITLE, 'https://example.com/softbank-third-tranche'),
  ];
  const bodyDoc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'doc-whats-current' },
      content: [
        { type: 'heading', attrs: { id: 'h-1', level: 1 }, content: [{ type: 'text', text: "What's current in AI" }] },
        {
          type: 'paragraph',
          attrs: { id: 'p-marked' },
          content: [
            { type: 'text', text: 'OpenAI — GPT-6 "Astra".', marks: [{ type: 'strong' }] },
            { type: 'text', text: ' The most independently measured model of the month.' },
          ],
        },
        {
          type: 'paragraph',
          attrs: { id: 'p-legacy' },
          content: [{ type: 'text', text: '**EU AI Act enforcement begins.** The Commission now enforces the transparency duties.' }],
        },
        {
          type: 'paragraph',
          attrs: { id: 'p-cite' },
          content: [
            { type: 'text', text: 'Haiku 5.5 costs about 75% less to run than Haiku 4.5' },
            ref('ref-cite', 'src_haiku', 'numbered_ref', CITE_QUOTE),
            { type: 'text', text: '.' },
          ],
        },
        {
          type: 'paragraph',
          attrs: { id: 'p-embed' },
          content: [
            { type: 'text', text: 'SoftBank closed its third OpenAI tranche.' },
            ref('ref-embed', 'src_softbank', 'expanded_ref', EMBED_QUOTE),
            { type: 'text', text: 'The money funds compute through 2027.' },
          ],
        },
        {
          type: 'bullet_list',
          attrs: { id: 'list-1' },
          content: [{ type: 'list_item', attrs: { id: 'li-1' }, content: [{ type: 'paragraph', attrs: { id: 'p-li-1' }, content: [{ type: 'text', text: 'Lists still render.' }] }] }],
        },
      ],
    },
  };
  return {
    route: { path: ROUTE, bare: ROUTE },
    publication: { title: "What's current in AI" },
    version: { id: 'pubver-1', number: 1 },
    nav: { latest: 1, current: 1 },
    artifact: { body_doc: bodyDoc, content: "What's current in AI" },
    payload: {},
    checks: {},
    source_entities: entities.map((item) => ({ source_entity_id: item.source_entity_id, entity: item })),
    transclusions: [],
  };
}

test.use({ viewport: { width: 390, height: 844 }, storageState: { cookies: [], origins: [] } });

test('a Write revision renders bold, a stored embed as a marker and its cite quotes at phone width', async ({ page }, testInfo) => {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
  );
  await page.route('**/api/platform/publications/resolve?*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle()) }),
  );

  await page.goto(`${BASE_URL}/pub/texture/${SLUG}`);
  const reader = page.locator('[data-texture-published-reader]');
  await expect(reader).toBeVisible({ timeout: 30_000 });
  const doc = reader;
  await expect(doc).toContainText('The most independently measured model', { timeout: 15_000 });

  // 1. Bold is bold, never "**" characters.
  const text = await doc.innerText();
  expect(text).not.toContain('**');
  await expect(doc.locator('p strong', { hasText: 'OpenAI — GPT-6 "Astra".' })).toHaveCount(1);
  await expect(doc.locator('p strong', { hasText: 'EU AI Act enforcement begins.' })).toHaveCount(1);
  await expect(doc.locator('ul li', { hasText: 'Lists still render.' })).toHaveCount(1);

  // 2-3. The stored embed is an inline marker in its sentence: its popover
  // carries its quote, number and title; nothing draws as a block, and no
  // paragraph is a lone "[n]".
  await expect(doc.locator('[data-embed-block]')).toHaveCount(0);
  await expect(doc.locator('blockquote')).toHaveCount(0);
  const embed = doc.locator('[data-texture-source-ref][data-texture-source-node-id="ref-embed"]');
  await expect(embed.locator('.texture-source-ref-label')).toHaveText('2');
  await expect(embed.locator('[data-texture-source-ref-popover] strong')).toHaveText(EMBED_TITLE);
  await expect(embed.locator('[data-texture-source-ref-popover] .texture-transclusion-quote')).toHaveText(EMBED_QUOTE);
  expect(await embed.evaluate((el) => el.closest('p')?.textContent.includes('SoftBank closed its third OpenAI tranche.'))).toBe(true);
  const loneMarkers = await doc.locator('p').evaluateAll((els) => els.map((el) => el.textContent.trim()).filter((t) => /^\[\d+\]$/.test(t) || t === ''));
  expect(loneMarkers).toEqual([]);
  await expect(doc.locator('p', { hasText: 'The money funds compute through 2027.' })).toHaveCount(1);

  // 4. The cite reads as a number in a whole sentence, and its popover
  // carries the citation's quote.
  const cite = doc.locator('[data-texture-source-ref][data-texture-source-node-id="ref-cite"]');
  await expect(cite.locator('.texture-source-ref-label')).toHaveText('1');
  await expect(cite.locator('[data-texture-source-ref-popover] .texture-transclusion-quote')).toHaveText(CITE_QUOTE);

  // 5. Nothing scrolls horizontally at phone width.
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);

  const shot = await reader.screenshot();
  await testInfo.attach('write-citations-390', { body: shot, contentType: 'image/png' });
  const dir = SHOT_DIR || testInfo.outputDir;
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(nodePath.join(dir, 'texture-write-citations-390.png'), shot);
});
