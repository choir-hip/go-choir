/**
 * The Texture toolbar of a published story at phone width and large text, and
 * the old /autopaper links (owner, 2026-10-21: "the toolbar should be
 * responsive for mobile — currently it overflows with large text" and "Get
 * rid of the standalone page";
 * docs/problems/research-more-slow-and-hidden-2026-10-21.md, E2). Built
 * frontend (vite preview) with the public platform routes mocked.
 *
 * Ways it can fail:
 *   R1 At 375 px the toolbar overflows sideways (its content wider than the
 *      toolbar, a control past the screen edge, or a page scrollbar), at the
 *      default text size or with the root font raised to 150% or 200%.
 *   R2 A control is dropped, not wrapped, to make a phone fit.
 *   R3 On a desktop width the toolbar grows tall: it must stay one compact row.
 *   R4 An old /autopaper/<slug> or /autopaper/<slug>/v<n> link lands on a
 *      blank or missing page; /autopaper alone must reach the desktop home.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/texture-published-toolbar-wrap.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');

function bundle(slug, title, revision = 1) {
  const route = `/pub/texture/${slug}`;
  const text = Array.from({ length: 6 }, (_, i) => `Paragraph ${i + 1} of ${title}. ` + 'The harbor reopened at dawn after a week of storms. '.repeat(3)).join('\n\n');
  return {
    route: { path: route, state: 'active', bare: route, permalink: `${route}/v${revision}` },
    publication: { id: `pub-${slug}`, title, slug, state: 'published' },
    version: { id: `ver-${slug}-${revision}`, content_hash: `hash-${slug}`, manifest_hash: `manifest-${slug}` },
    artifact: { content: text, body_doc: null },
    retrieval: {},
    citations: [],
    source_entities: [],
    transclusions: [],
    policy: {},
    proposals: {},
    provenance: {},
    payload: { revision, manifest_hash: `manifest-${slug}`, manifest: '{}', resolved_blocks: [] },
    // Version 12 of 12: the toolbar's widest version label.
    nav: { revision: 12, prev: 11, next: 0, latest: 12 },
    checks: { source: 'pending', citations: [] },
  };
}

const slugOf = (url) => new URL(url).searchParams.get('route').replace('/pub/texture/', '').split('/')[0];

async function mockRoutes(page, { allowed = true, resolved = [] } = {}) {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/related?*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"links":[]}' }));
  await page.route('**/api/platform/publications/resolve?*', async (route) => {
    const url = new URL(route.request().url());
    resolved.push(url.searchParams.get('route'));
    if (url.searchParams.get('view') === 'nav') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ nav: { revision: 12, latest: 12 } }) });
    }
    const slug = slugOf(route.request().url());
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle(slug, `Story ${slug.replace('story-', '').toUpperCase()}`)) });
  });
  await page.route('**/api/platform/publications/expansion?*', (route) => {
    // An open request: the button reads "Asked for more" and the fate line shows.
    const body = allowed
      ? { allowed: true, route: '/pub/texture/story-a', asked: true, requests: [{ request_id: 'rx-1', state: 'accepted', asked_at: '2026-10-21T12:00:00Z', from_version: 12 }] }
      : { allowed: false };
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

/** Every number the overflow checks need, read in the page. */
async function toolbarFit(page) {
  return page.evaluate(() => {
    const toolbar = document.querySelector('[data-texture-toolbar]');
    const tr = toolbar.getBoundingClientRect();
    const controls = [...toolbar.querySelectorAll('button, summary, [data-texture-version]')]
      .filter((el) => el.getClientRects().length > 0 && (el.tagName === 'SUMMARY' || !el.closest('details:not([open])')))
      .map((el) => {
        const r = el.getBoundingClientRect();
        return { label: (el.textContent || '').trim().slice(0, 24), left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), bottom: Math.round(r.bottom) };
      });
    return {
      viewport: window.innerWidth,
      pageScroll: [document.scrollingElement.scrollWidth, document.scrollingElement.clientWidth],
      toolbarScroll: [toolbar.scrollWidth, toolbar.clientWidth],
      toolbarBox: { left: Math.round(tr.left), right: Math.round(tr.right), height: Math.round(tr.height) },
      controls,
    };
  });
}

async function openStory(page) {
  await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
  await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
  await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-button]')).toBeVisible({ timeout: 15_000 });
}

test.describe('the toolbar at phone width', () => {
  test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true, storageState: { cookies: [], origins: [] } });

  for (const percent of [100, 150, 200]) {
    test(`wraps instead of overflowing with the root font at ${percent}% (R1, R2)`, async ({ page }) => {
      await mockRoutes(page);
      await openStory(page);
      await page.addStyleTag({ content: `html { font-size: ${percent}% !important; }` });
      await page.waitForTimeout(400);

      const fit = await toolbarFit(page);
      expect(fit.pageScroll[0], 'page scrolls sideways').toBeLessThanOrEqual(fit.pageScroll[1]);
      expect(fit.toolbarScroll[0], 'toolbar content wider than the toolbar').toBeLessThanOrEqual(fit.toolbarScroll[1] + 1);
      for (const c of fit.controls) {
        expect(c.left, `${c.label} left edge`).toBeGreaterThanOrEqual(fit.toolbarBox.left - 1);
        expect(c.right, `${c.label} right edge`).toBeLessThanOrEqual(fit.toolbarBox.right + 1);
      }
      // Nothing is dropped to make it fit.
      const labels = fit.controls.map((c) => c.label).join('|');
      for (const want of ['v12', '<', '>', 'Copy text', 'Download', 'Asked for more']) expect(labels, want).toContain(want);
      // The fate line wraps under the buttons and stays on screen.
      const line = page.locator('[data-texture-toolbar] [data-autopaper-expand-line]');
      await expect(line).toBeVisible();
      const lineBox = await line.boundingBox();
      expect(lineBox.x).toBeGreaterThanOrEqual(0);
      expect(lineBox.x + lineBox.width).toBeLessThanOrEqual(fit.viewport + 1);
      // The toolbar did not eat the screen: the article still starts in the top half.
      const readerTop = await page.locator('[data-texture-published-reader]').evaluate((el) => el.getBoundingClientRect().top);
      expect(readerTop, 'article start').toBeLessThan(percent === 200 ? 700 : 400);
    });
  }
});

test.describe('the toolbar on a desktop width', () => {
  test.use({ viewport: { width: 1280, height: 800 }, storageState: { cookies: [], origins: [] } });

  test('stays one compact row with the button in it (R3)', async ({ page }) => {
    await mockRoutes(page, { allowed: false });
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    const bare = await toolbarFit(page);
    expect(bare.toolbarBox.height, 'toolbar height without the button').toBeLessThanOrEqual(64);
    await page.unrouteAll({ behavior: 'ignoreErrors' });

    await mockRoutes(page);
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-button]')).toBeVisible({ timeout: 30_000 });
    // The fate line sits in the toolbar too; the toolbar grows by at most one line of it.
    await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-line]')).toBeVisible();
    const fit = await toolbarFit(page);
    const tops = fit.controls.map((c) => c.top);
    expect(Math.max(...tops) - Math.min(...tops), 'all controls on one row').toBeLessThanOrEqual(12);
    expect(fit.toolbarBox.height, 'toolbar height with the button and its line').toBeLessThanOrEqual(96);
  });
});

test.describe('old /autopaper links', () => {
  test.use({ viewport: { width: 1280, height: 800 }, storageState: { cookies: [], origins: [] } });

  test('a story link opens the story in the desktop at /pub/texture/<slug> (R4)', async ({ page }) => {
    const resolved = [];
    await mockRoutes(page, { allowed: false, resolved });
    await page.goto(`${BASE_URL}/autopaper/story-a`, { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(`${BASE_URL}/pub/texture/story-a`, { timeout: 15_000 });
    await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    expect(await page.evaluate(() => history.length), 'the old URL is replaced, not kept for Back to bounce on').toBeLessThanOrEqual(2);
  });

  test('a versioned link keeps its version (R4)', async ({ page }) => {
    const resolved = [];
    await mockRoutes(page, { allowed: false, resolved });
    await page.goto(`${BASE_URL}/autopaper/story-a/v3`, { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(`${BASE_URL}/pub/texture/story-a/v3`, { timeout: 15_000 });
    await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    expect(resolved).toContain('/pub/texture/story-a/v3');
  });

  test('/autopaper alone goes to the desktop home, which shows the front page (R4)', async ({ page }) => {
    await mockRoutes(page, { allowed: false });
    await page.route('**/api/platform/publications/front*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ stories: [] }) }));
    await page.goto(`${BASE_URL}/autopaper`, { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(`${BASE_URL}/`, { timeout: 15_000 });
    await expect(page.locator('[data-autopaper]')).toBeVisible({ timeout: 20_000 });
  });
});
