/**
 * The AI-written disclosure on Autopaper stories (owner notes §27 item 14:
 * "Published stories carry a one-line AI-written disclosure"), against the
 * built frontend with the public routes mocked. Written before the code.
 *
 * Ways it can fail:
 *   D1 An Autopaper story renders without the disclosure.
 *   D2 The disclosure is not at the article bottom: it must follow the last
 *      story paragraph and sit directly before the related block (owner notes
 *      §28), inside the scrolling article.
 *   D3 A published document that is not an Autopaper story (a person's own
 *      published Texture) is labelled AI-written.
 *   D4 The label is lost when the reader moves to another story through a
 *      "More on this" link (the editor context is rebuilt on navigation).
 *   D5 The related block stops being the article's last child.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/autopaper-ai-disclosure.spec.js
 */
import { expect, test } from '@playwright/test';
import { aiDisclosureHTML, AI_DISCLOSURE_TEXT } from '../src/lib/texture-ai-disclosure.ts';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');

test.describe('aiDisclosureHTML', () => {
  test('labels Autopaper stories only (D1, D3)', () => {
    expect(aiDisclosureHTML({ autopaperStory: true, publishedReadOnly: true })).toContain(AI_DISCLOSURE_TEXT);
    expect(aiDisclosureHTML({ autopaperStory: true, publishedReadOnly: true })).toContain('data-texture-ai-disclosure');
    for (const ctx of [null, undefined, {}, { publishedReadOnly: true }, { autopaperStory: 'yes' }]) {
      expect(aiDisclosureHTML(ctx), JSON.stringify(ctx)).toBe('');
    }
  });
});

function bundle(slug, title) {
  const route = `/pub/texture/${slug}`;
  const text = Array.from({ length: 8 }, (_, i) => `Paragraph ${i + 1} of ${title}.`).join('\n\n');
  return {
    route: { path: route, state: 'active', bare: route, permalink: `${route}/v1` },
    publication: { id: `pub-${slug}`, title, slug, state: 'published', kind: 'story' },
    version: { id: `ver-${slug}-1`, content_hash: `hash-${slug}`, manifest_hash: `manifest-${slug}` },
    artifact: { content: text, body_doc: null },
    retrieval: {}, citations: [], source_entities: [], transclusions: [], policy: {}, proposals: {}, provenance: {},
    payload: { revision: 1, manifest_hash: `manifest-${slug}`, manifest: '{}', resolved_blocks: [] },
    nav: { revision: 1, prev: 0, next: 0, latest: 1 },
    checks: { source: 'pending', citations: [] },
  };
}

async function mockRoutes(page) {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/resolve?*', (route) => {
    const url = new URL(route.request().url());
    const slug = url.searchParams.get('route').replace('/pub/texture/', '').split('/')[0];
    if (url.searchParams.get('view') === 'nav') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ nav: { revision: 1, latest: 1 } }) });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle(slug, `Story ${slug}`)) });
  });
  await page.route('**/api/platform/publications/related?*', (route) => {
    const url = new URL(route.request().url());
    const slug = url.searchParams.get('route').replace('/pub/texture/', '').split('/')[0];
    const links = slug === 'story-a' ? [{
      publication_id: 'pub-story-b', title: 'Story B', dek: 'Dek B.', updated_at: new Date().toISOString(),
      route: { bare: '/pub/texture/story-b', permalink: '/pub/texture/story-b/v1' }, section: 'world', reason: 'capture',
      shared: { events: 0, sources: 1, citers: 2 },
    }] : [];
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ route: `/pub/texture/${slug}`, revision: 1, basis: { captures: 1, events: 0, mode: 'captures' }, partial: false, links }) });
  });
}

test.describe('the disclosure on a story page', () => {
  test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });

  test('closes the article, after the last paragraph and before related stories, and survives a related link (D1, D2, D4, D5)', async ({ page }) => {
    await mockRoutes(page);
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    const label = reader.locator('[data-texture-ai-disclosure]');
    await expect(label).toHaveCount(1, { timeout: 15_000 });
    await expect(label).toHaveText(AI_DISCLOSURE_TEXT);
    await expect(reader.locator('[data-texture-related]')).toHaveCount(1, { timeout: 15_000 });
    const placement = await reader.evaluate((el) => {
      const label = el.querySelector('[data-texture-ai-disclosure]');
      const related = el.querySelector('[data-texture-related]');
      const story = [...el.querySelectorAll('p')].filter((p) => !label.contains(p) && !related.contains(p));
      const last = story[story.length - 1];
      return {
        afterLastParagraph: Boolean(last.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING),
        nextIsRelated: label.nextElementSibling === related,
      };
    });
    expect(placement.afterLastParagraph, 'the label follows the last story paragraph').toBe(true);
    expect(placement.nextIsRelated, 'the related block follows the label directly').toBe(true);
    expect(await reader.evaluate((el) => el.lastElementChild?.hasAttribute('data-texture-related'))).toBe(true);

    await reader.locator('[data-related-link]').first().click();
    await expect(reader).toContainText("Paragraph 1 of Story story-b", { timeout: 30_000 });
    await expect(reader.locator('[data-texture-ai-disclosure]')).toHaveText(AI_DISCLOSURE_TEXT, { timeout: 15_000 });
  });
});
