/**
 * Citation markers are toggles, and nothing overlays the text (owner
 * direction, 2026-10-27). In a rendered Texture document:
 *
 *   1. every numbered marker sits inside its sentence and stays there when its
 *      note opens, with the same box and the same text font as its paragraph;
 *   2. clicking (or pressing Enter/Space on) a marker opens the plain source
 *      note in the text flow under its paragraph, and the marker reports
 *      aria-expanded=true with the pressed style;
 *   3. pressing it again closes that note and no other; two markers open two
 *      independent notes;
 *   4. hovering or focusing a marker shows no popover over the text: the
 *      marker's popover span keeps its content for existing readers but is
 *      never visible.
 *
 * The bundle below is a published revision with three citation anchors in two
 * paragraphs; the backend is mocked, so the test needs only a served build.
 *
 *   cd frontend && npm run build && npx vite preview --port 4174 --strictPort &
 *   PLAYWRIGHT_BASE_URL=http://localhost:4174 \\
 *     node_modules/.bin/playwright test tests/texture-citation-marker-toggle.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\+$/, '');
const SLUG = 'citation-toggle-proof';
const ROUTE = '/pub/texture/' + SLUG;

const QUOTES = {
  'anchor-1': 'Haiku 5.5 costs around 75% less to run at the same quality.',
  'anchor-2': 'SoftBank completed the third tranche of its follow-on investment in OpenAI.',
  'anchor-3': 'The commission enforces the transparency duties from this month.',
};

function entity(id, title, url) {
  return {
    source_entity_id: id,
    target: { kind: 'web_url', uri: url, id: 'cap-' + id, metadata: { capture_doc: 'cap-' + id, capture_version: 'v1' } },
    selectors: [{ kind: 'whole_resource' }],
    display: { mode: 'numbered_ref', title },
    evidence: { state: 'available', open_surface: 'source' },
    provenance: { created_by: 'texture_write' },
  };
}

function bundle() {
  const entities = [
    entity('src_haiku', 'Introducing Claude Haiku 5.5', 'https://example.com/haiku-5-5'),
    entity('src_softbank', 'SoftBank follow-on investment', 'https://example.com/softbank'),
    entity('src_euai', 'EU AI Act enforcement', 'https://example.com/eu-ai-act'),
  ];
  const bodyDoc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'doc-citation-toggle' },
      content: [
        { type: 'heading', attrs: { id: 'h-1', level: 1 }, content: [{ type: 'text', text: 'Citation toggle proof' }] },
        {
          type: 'paragraph',
          attrs: { id: 'p-one' },
          content: [
            { type: 'text', text: 'Haiku 5.5 costs about 75% less to run at the same quality' },
            { type: 'source_ref', attrs: { id: 'anchor-1', source_entity_id: 'src_haiku', display_mode: 'numbered_ref', ref: { quote: QUOTES['anchor-1'] } } },
            { type: 'text', text: ', and SoftBank closed its third OpenAI tranche' },
            { type: 'source_ref', attrs: { id: 'anchor-2', source_entity_id: 'src_softbank', display_mode: 'numbered_ref', ref: { quote: QUOTES['anchor-2'] } } },
            { type: 'text', text: '.' },
          ],
        },
        {
          type: 'paragraph',
          attrs: { id: 'p-two' },
          content: [
            { type: 'text', text: 'Separately, the commission now enforces the transparency duties' },
            { type: 'source_ref', attrs: { id: 'anchor-3', source_entity_id: 'src_euai', display_mode: 'numbered_ref', ref: { quote: QUOTES['anchor-3'] } } },
            { type: 'text', text: ' for every provider in the bloc.' },
          ],
        },
      ],
    },
  };
  // The published bundle pins each anchor to one captured block: its quote
  // sits inside that block, so the note shows the quote in its source text.
  const anchors = [
    { anchor: 'anchor-1', doc: 'cap-src_haiku', cid: 'h1', quote: QUOTES['anchor-1'], text: 'Our note: ' + QUOTES['anchor-1'] + ' That is the headline claim.' },
    { anchor: 'anchor-2', doc: 'cap-src_softbank', cid: 's1', quote: QUOTES['anchor-2'], text: QUOTES['anchor-2'] + ' The round values the company higher.' },
    { anchor: 'anchor-3', doc: 'cap-src_euai', cid: 'e1', quote: QUOTES['anchor-3'], text: 'Brussels — ' + QUOTES['anchor-3'] + ' Providers have a year to comply.' },
  ];
  return {
    route: { path: ROUTE, bare: ROUTE },
    publication: { title: 'Citation toggle proof' },
    version: { id: 'pubver-citation-toggle', number: 1 },
    nav: { latest: 1, current: 1 },
    artifact: { body_doc: bodyDoc, content: 'Citation toggle proof' },
    payload: {
      manifest: JSON.stringify({
        anchors: anchors.map((item) => ({
          anchor_id: item.anchor,
          relation: 'cites',
          ref: { doc: item.doc, version: 'v1', cid: item.cid, quote: item.quote },
          cids: [item.cid],
        })),
      }),
      resolved_blocks: anchors.map((item) => ({ doc: item.doc, cid: item.cid, text: item.text, kind: 'p' })),
    },
    checks: { citations: anchors.map((item) => ({ anchor_id: item.anchor, state: 'exact', quote: 'exact' })) },
    source_entities: entities.map((item) => ({ source_entity_id: item.source_entity_id, entity: item })),
    transclusions: [],
  };
}

test.use({ viewport: { width: 390, height: 844 }, storageState: { cookies: [], origins: [] } });

async function openDocument(page) {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
  );
  await page.route('**/api/platform/publications/resolve?*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle()) }),
  );
  await page.goto(BASE_URL + ROUTE);
  const reader = page.locator('[data-texture-published-reader]');
  await expect(reader).toBeVisible({ timeout: 30_000 });
  await expect(reader).toContainText('for every provider in the bloc.', { timeout: 15_000 });
  return reader;
}

function marker(reader, anchorId) {
  return reader.locator('[data-texture-source-ref][data-texture-source-node-id="' + anchorId + '"]');
}

function note(reader, anchorId) {
  return reader.locator('[data-texture-source-note][data-citation-anchor="' + anchorId + '"]');
}

async function boxOf(locator) {
  return locator.evaluate((el) => {
    const rect = el.getBoundingClientRect();
    return { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) };
  });
}

/** Font family and size of the paragraph text around a marker. */
async function paragraphFont(locator) {
  return locator.evaluate((el) => {
    const style = getComputedStyle(el.closest('p') || el);
    return { family: style.fontFamily, size: style.fontSize };
  });
}

test('a citation marker toggles its own note under its paragraph and nothing overlays the text', async ({ page }) => {
  const reader = await openDocument(page);

  // The markers are in their sentences, numbered, and their notes sit under
  // the paragraphs that cite them.
  await expect(reader.locator('[data-texture-source-ref]')).toHaveCount(3);
  await expect(marker(reader, 'anchor-1').locator('.texture-source-ref-label')).toHaveText('1');
  await expect(marker(reader, 'anchor-2').locator('.texture-source-ref-label')).toHaveText('2');
  await expect(marker(reader, 'anchor-3').locator('.texture-source-ref-label')).toHaveText('3');
  await expect(note(reader, 'anchor-1')).toHaveCount(1);
  await expect(note(reader, 'anchor-2')).toHaveCount(1);
  await expect(note(reader, 'anchor-3')).toHaveCount(1);
  // Each note is a child of a citation-notes block, and that block directly
  // follows the paragraph the marker sits in: the note reads as sitting under
  // the paragraph that cites it, and its markers' paragraph is its own.
  const notePlacement = await Promise.all(['anchor-1', 'anchor-2', 'anchor-3'].map(async (anchorId) =>
    note(reader, anchorId).evaluate((el) => {
      const block = el.parentElement;
      const paragraph = block ? block.previousElementSibling : null;
      const markerHere = paragraph ? paragraph.querySelector('[data-texture-source-ref][data-texture-source-node-id="' + el.getAttribute('data-citation-anchor') + '"]') : null;
      return !!paragraph && paragraph.tagName === 'P' && block.getAttribute('data-texture-citation-notes') !== null && !!markerHere;
    }),
  ));
  expect(notePlacement).toEqual([true, true, true]);

  // No popover is visible before any interaction, and a hover opens none.
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);
  await marker(reader, 'anchor-1').hover();
  await page.waitForTimeout(150);
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);
  await marker(reader, 'anchor-3').focus();
  await page.waitForTimeout(150);
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);

  // Collapse everything first, then open marker 1 and hold the initial state
  // of the marker and of the paragraph text to compare against.
  for (const anchorId of ['anchor-1', 'anchor-2', 'anchor-3']) {
    if ((await marker(reader, anchorId).getAttribute('aria-expanded')) === 'true') {
      await marker(reader, anchorId).click();
    }
  }
  await expect(reader.locator('[data-texture-source-note][data-citation-note-open="true"]')).toHaveCount(0);

  const before = {
    marker: await boxOf(marker(reader, 'anchor-1')),
    font: await paragraphFont(marker(reader, 'anchor-1')),
    paragraph: await boxOf(reader.locator('p', { hasText: 'Haiku 5.5 costs about 75% less' })),
    text: await reader.locator('p', { hasText: 'Haiku 5.5 costs about 75% less' }).innerText(),
  };

  await marker(reader, 'anchor-1').click();
  await expect(note(reader, 'anchor-1')).toHaveAttribute('data-citation-note-open', 'true');
  await expect(note(reader, 'anchor-1')).toBeVisible();
  await expect(marker(reader, 'anchor-1')).toHaveAttribute('aria-expanded', 'true');
  await expect(marker(reader, 'anchor-1')).toHaveAttribute('aria-pressed', 'true');
  // 1. The marker did not move or resize, and the sentence kept its font.
  expect(await boxOf(marker(reader, 'anchor-1'))).toEqual(before.marker);
  expect(await paragraphFont(marker(reader, 'anchor-1'))).toEqual(before.font);
  // The line the marker is on did not move: the paragraph above it is intact.
  expect(await boxOf(reader.locator('p', { hasText: 'Haiku 5.5 costs about 75% less' }))).toEqual(before.paragraph);
  expect(await reader.locator('p', { hasText: 'Haiku 5.5 costs about 75% less' }).innerText()).toBe(before.text);
  // The note carries the citation's own quote, its number and its title.
  await expect(note(reader, 'anchor-1')).toContainText(QUOTES['anchor-1']);
  await expect(note(reader, 'anchor-1').locator('.texture-source-number')).toHaveText('[1]');
  await expect(note(reader, 'anchor-1')).toContainText('Introducing Claude Haiku 5.5');
  // No popover showed when the note opened.
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);

  // 3. A second marker opens a second, independent note.
  await marker(reader, 'anchor-2').click();
  await expect(note(reader, 'anchor-1')).toHaveAttribute('data-citation-note-open', 'true');
  await expect(note(reader, 'anchor-2')).toHaveAttribute('data-citation-note-open', 'true');
  await expect(note(reader, 'anchor-2')).toContainText(QUOTES['anchor-2']);
  await expect(reader.locator('[data-texture-source-note][data-citation-note-open="true"]')).toHaveCount(2);
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);

  // Pressing marker 1 again closes marker 1's note and leaves marker 2's open.
  await marker(reader, 'anchor-1').click();
  await expect(note(reader, 'anchor-1')).toHaveAttribute('data-citation-note-open', 'false');
  await expect(note(reader, 'anchor-2')).toHaveAttribute('data-citation-note-open', 'true');
  await expect(marker(reader, 'anchor-1')).toHaveAttribute('aria-expanded', 'false');
  await expect(marker(reader, 'anchor-1')).toHaveAttribute('aria-pressed', 'false');
  await expect(marker(reader, 'anchor-2')).toHaveAttribute('aria-expanded', 'true');
  expect(await boxOf(marker(reader, 'anchor-1'))).toEqual(before.marker);

  // Keyboard toggling works the same way.
  await marker(reader, 'anchor-1').focus();
  await page.keyboard.press('Enter');
  await expect(note(reader, 'anchor-1')).toHaveAttribute('data-citation-note-open', 'true');
  await page.keyboard.press('Space');
  await expect(note(reader, 'anchor-1')).toHaveAttribute('data-citation-note-open', 'false');

  // 4. The marker's popover span still holds its content for existing readers
  // of it, and is never visible.
  await expect(marker(reader, 'anchor-1').locator('[data-texture-source-ref-popover]')).toHaveCount(1);
  await expect(reader.locator('[data-texture-source-ref-popover]:visible')).toHaveCount(0);
});
