/**
 * Opinion pieces on the Autopaper front and story page
 * (docs/problems/oped-not-connected-2026-10-20.md), against the built
 * frontend with the public routes mocked. Written before the code.
 *
 * Ways it can fail:
 *   O1 An opinion piece is shown with the AI-written disclosure, because the
 *      page was opened from the Autopaper app (autopaperStory context).
 *   O2 An opinion piece has no "Opinion" label or byline on its page.
 *   O3 A story is labelled opinion, or loses the AI disclosure, because of
 *      the link or the context rather than the server's record.
 *   O4 The Opinion section is mixed into the news grid, or shows an empty
 *      heading when the server lists no opinion piece (kill switch off).
 *   O5 The author's text is injected as HTML.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/oped-opinion-label.spec.js
 */
import { expect, test } from '@playwright/test';
import { aiDisclosureHTML, AI_DISCLOSURE_TEXT, opinionLabelHTML, pieceDisclosureHTML } from '../src/lib/texture-ai-disclosure.ts';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');

test.describe('opinion label helpers', () => {
  const opinion = { kind: 'opinion', author_label: 'writer-aa11bb' };

  test('an opinion piece is labelled and never AI-disclosed (O1, O2, O3)', () => {
    const label = opinionLabelHTML(opinion);
    expect(label).toContain('data-texture-opinion-label');
    expect(label).toContain('Opinion');
    expect(label).toContain('writer-aa11bb');
    for (const ctx of [{ autopaperStory: true }, {}, null]) {
      const notice = pieceDisclosureHTML(ctx, opinion);
      expect(notice, JSON.stringify(ctx)).toContain('data-texture-opinion-notice');
      expect(notice, JSON.stringify(ctx)).not.toContain(AI_DISCLOSURE_TEXT);
      expect(notice).not.toContain('data-texture-ai-disclosure');
    }
  });

  test('a story keeps the AI disclosure and gets no opinion label, whatever the link says (O3)', () => {
    expect(opinionLabelHTML({ id: 'p', kind: '' })).toBe('');
    expect(opinionLabelHTML(null)).toBe('');
    expect(pieceDisclosureHTML({ autopaperStory: true }, { id: 'p' })).toBe(aiDisclosureHTML({ autopaperStory: true }));
    expect(pieceDisclosureHTML({}, { id: 'p' })).toBe('');
    // The server's record names a story (kind "story"), so a story opened at
    // its public Texture route, with no Autopaper window context, still
    // carries the AI line; a person's own published Texture does not.
    expect(pieceDisclosureHTML({}, { id: 'p', kind: 'story' })).toBe(aiDisclosureHTML({ autopaperStory: true }));
    expect(pieceDisclosureHTML({ publishedReadOnly: true }, { id: 'p', kind: '' })).toBe('');
  });

  test('the byline is escaped (O5)', () => {
    const hostile = { kind: 'opinion', author_label: '<img src=x onerror=alert(1)>' };
    expect(opinionLabelHTML(hostile)).not.toContain('<img');
    expect(pieceDisclosureHTML({}, hostile)).not.toContain('<img');
  });
});

function bundle(slug, title, publication) {
  const route = `/pub/texture/${slug}`;
  const text = Array.from({ length: 4 }, (_, i) => `Paragraph ${i + 1} of ${title}.`).join('\n\n');
  return {
    route: { path: route, state: 'active', bare: route, permalink: `${route}/v1` },
    publication: { id: `pub-${slug}`, title, slug, state: 'published', ...publication },
    version: { id: `ver-${slug}-1`, content_hash: `hash-${slug}`, manifest_hash: `manifest-${slug}` },
    artifact: { content: text, body_doc: null },
    retrieval: {}, citations: [], source_entities: [], transclusions: [], policy: {}, proposals: {}, provenance: {},
    payload: { revision: 1, manifest_hash: `manifest-${slug}`, manifest: '{}', resolved_blocks: [] },
    nav: { revision: 1, prev: 0, next: 0, latest: 1 },
    checks: { source: 'pending', citations: [] },
  };
}

async function mockPublished(page) {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/related?*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ links: [], basis: {}, partial: false }) }));
  await page.route('**/api/platform/publications/resolve?*', (route) => {
    const url = new URL(route.request().url());
    const slug = url.searchParams.get('route').replace('/pub/texture/', '').split('/')[0];
    if (url.searchParams.get('view') === 'nav') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ nav: { revision: 1, latest: 1 } }) });
    }
    const publication = slug === 'my-view' ? { kind: 'opinion', author_label: 'writer-aa11bb' } : { kind: 'story' };
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle(slug, `Piece ${slug}`, publication)) });
  });
}

test.describe('the opinion page', () => {
  test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });

  test('shows the Opinion label and byline, not the AI disclosure, even for a piece opened at its public Texture route (O1, O2)', async ({ page }) => {
    await mockPublished(page);
    await page.goto(`${BASE_URL}/pub/texture/my-view`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    await expect(reader.locator('[data-texture-opinion-label]')).toContainText('Opinion');
    await expect(reader.locator('[data-texture-opinion-label]')).toContainText('writer-aa11bb');
    await expect(reader.locator('[data-texture-opinion-notice]')).toBeVisible();
    await expect(reader.locator('[data-texture-ai-disclosure]')).toHaveCount(0);
  });

  test('a story opened the same way keeps the AI disclosure and no opinion label (O3)', async ({ page }) => {
    await mockPublished(page);
    await page.goto(`${BASE_URL}/pub/texture/a-story?opinion=1`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    await expect(reader.locator('[data-texture-ai-disclosure]')).toBeVisible();
    await expect(reader.locator('[data-texture-opinion-label]')).toHaveCount(0);
  });
});

function story(slug, title, extra = {}) {
  return {
    publication_id: `pub-${slug}`, title, dek: `${title} dek.`, updated_at: new Date().toISOString(), version_count: 1, latest_revision: 1,
    route: { bare: `/pub/texture/${slug}`, permalink: `/pub/texture/${slug}/v1` }, ...extra,
  };
}

async function mockFront(page, opinion) {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/front?*', (route) => {
    const url = new URL(route.request().url());
    const body = url.searchParams.get('section') === 'opinion'
      ? { stories: opinion }
      : { stories: [story('harbor', 'Harbor reopens'), story('bridge', 'Bridge closes')] };
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

test.describe('the front page', () => {
  test.use({ viewport: { width: 1100, height: 900 }, storageState: { cookies: [], origins: [] } });

  test('lists opinion pieces apart from the news, with label and byline (O2, O4)', async ({ page }) => {
    await mockFront(page, [story('my-view', 'My view on the harbor', { kind: 'opinion', author: 'writer-aa11bb' })]);
    await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    const section = page.locator('[data-autopaper-opinion]');
    await expect(section).toBeVisible({ timeout: 30_000 });
    await expect(section.locator('[data-autopaper-opinion-heading]')).toHaveText('Opinion');
    const row = section.locator('[data-autopaper-story-row]');
    await expect(row).toHaveCount(1);
    await expect(row).toContainText('My view on the harbor');
    await expect(row.locator('[data-autopaper-opinion-label]')).toHaveText('Opinion');
    await expect(row.locator('[data-autopaper-byline]')).toContainText('writer-aa11bb');
    // The news grid holds the news only.
    await expect(page.locator('[data-autopaper-grid] [data-autopaper-kind="opinion"]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper-grid]')).not.toContainText('My view on the harbor');
  });

  test('shows no Opinion heading when the server lists none (O4)', async ({ page }) => {
    await mockFront(page, []);
    await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('[data-autopaper-opinion]')).toHaveCount(0);
  });
});
