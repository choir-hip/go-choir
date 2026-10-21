/**
 * "More on this" on a published story (related-stories design 5.8,
 * docs/reports/related-stories-graph-design-2026-10-19.md), against the
 * built frontend with the public routes mocked. The deployed proof is
 * autopaper-related-deployed.spec.js; this spec pins the UI's own failure
 * modes. Written before the code.
 *
 * Ways the block can fail:
 *   U1 A title, dek or route from the server is trusted as HTML (a model
 *      writes the titles): markup in them must render as text, and a link
 *      whose route is not a /pub/texture/ story never becomes a link.
 *   U2 An empty, failed, partial or malformed answer leaves a heading or a
 *      placeholder on the page ("More on this" over nothing).
 *   U3 More than five links, or one story twice.
 *   U4 The list is fetched inside resolve or before the article renders, so
 *      a slow list holds up the story: the article must be on screen while
 *      the related request is still pending, and the related request must
 *      start after the resolve answered.
 *   U5 A late answer for story A lands on story B after the reader moved.
 *   U6 The block sits outside the scrolling article (it would take height
 *      from the reader at all times) or above the last paragraph.
 *   U7 A click leaves the page, opens nothing, or does not move to the top
 *      of the new story; a modified click is hijacked so a new tab is lost.
 *   U8 The block overflows a phone horizontally.
 *   U9 A failed related request breaks the story (error banner, no text).
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/autopaper-related.spec.js
 */
import { expect, test } from '@playwright/test';
import {
  RELATED_MAX_LINKS,
  relatedAgeLabel,
  relatedLinkTarget,
  renderRelatedHTML,
  visibleRelatedLinks,
} from '../src/lib/texture-related-stories.ts';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const NOW = Date.parse('2026-10-20T12:00:00Z');
const HOUR = 3_600_000;

function link(id, overrides = {}) {
  return {
    publication_id: `pub-${id}`,
    title: `Story ${id}`,
    dek: `Dek of story ${id}.`,
    updated_at: new Date(NOW - 3 * HOUR).toISOString(),
    route: { bare: `/pub/texture/story-${id}`, permalink: `/pub/texture/story-${id}/v2` },
    section: 'world',
    reason: 'capture',
    shared: { events: 0, sources: 1, citers: 2 },
    ...overrides,
  };
}

const related = (links, extra = {}) => ({ route: '/pub/texture/story-a', revision: 1, basis: { captures: 2, events: 0, mode: 'captures' }, partial: false, links, ...extra });

// ---- the renderer ----

test.describe('renderRelatedHTML', () => {
  test('renders the heading and real anchors, escaped (U1, U3)', () => {
    const html = renderRelatedHTML(related([
      link('b', { title: '<img src=x onerror=alert(1)> & "quotes"', dek: '<script>alert(2)</script>' }),
      link('c'),
    ]), NOW);
    expect(html).toContain('data-texture-related');
    expect(html).toContain('More on this');
    expect(html).not.toContain('<img');
    expect(html).not.toContain('<script');
    expect(html).toContain('&lt;img src=x onerror=alert(1)&gt; &amp; &quot;quotes&quot;');
    expect(html).toContain('href="/pub/texture/story-b"');
    expect(html).toContain('data-related-link');
    expect(html).toContain('data-related-reason="capture"');
    expect(html).toContain('3 h ago · World');
  });

  test('renders nothing for an empty, failed or malformed answer (U2)', () => {
    for (const value of [null, undefined, {}, { links: [] }, related([]), { links: 'x' }]) {
      expect(renderRelatedHTML(value, NOW), JSON.stringify(value)).toBe('');
    }
  });

  test('drops links that are not stories, repeats and extras (U1, U3)', () => {
    const many = Array.from({ length: 9 }, (_, i) => link(`n${i}`));
    expect(visibleRelatedLinks(related(many))).toHaveLength(RELATED_MAX_LINKS);
    const odd = related([
      link('ok'),
      link('ok'),
      link('js', { route: { bare: 'javascript:alert(1)' } }),
      link('other', { route: { bare: '/api/secret' } }),
      link('notitle', { title: '   ' }),
      link('slug/with/slash', { route: { bare: '/pub/texture/' } }),
    ]);
    const shown = visibleRelatedLinks(odd);
    expect(shown.map((l) => l.publication_id)).toEqual(['pub-ok']);
    const html = renderRelatedHTML(odd, NOW);
    expect(html).not.toContain('javascript:');
    expect(html.match(/data-related-link/g)).toHaveLength(1);
  });

  test('ages read like a newspaper and tolerate junk (U1)', () => {
    expect(relatedAgeLabel(new Date(NOW - 20_000).toISOString(), NOW)).toBe('just now');
    expect(relatedAgeLabel(new Date(NOW - 12 * 60_000).toISOString(), NOW)).toBe('12 min ago');
    expect(relatedAgeLabel(new Date(NOW - 5 * HOUR).toISOString(), NOW)).toBe('5 h ago');
    expect(relatedAgeLabel(new Date(NOW - 3 * 24 * HOUR).toISOString(), NOW)).toBe('3 d ago');
    expect(relatedAgeLabel(new Date(NOW - 40 * 24 * HOUR).toISOString(), NOW)).toBe('2026-09-10');
    expect(relatedAgeLabel('not a date', NOW)).toBe('');
    expect(relatedAgeLabel(undefined, NOW)).toBe('');
  });

  test('a modified click is left to the browser (U7)', () => {
    const el = { getAttribute: (n) => ({ 'data-related-route': '/pub/texture/story-b', 'data-related-title': 'B' })[n] || '' };
    const target = { closest: () => el };
    expect(relatedLinkTarget({ target, button: 0 })).toEqual({ route: '/pub/texture/story-b', title: 'B' });
    expect(relatedLinkTarget({ target, button: 1 })).toBeNull();
    expect(relatedLinkTarget({ target, button: 0, ctrlKey: true })).toBeNull();
    expect(relatedLinkTarget({ target, button: 0, metaKey: true })).toBeNull();
    expect(relatedLinkTarget({ target: { closest: () => null }, button: 0 })).toBeNull();
  });
});

// ---- the page ----

function bundle(slug, title, paragraphs = 30) {
  const route = `/pub/texture/${slug}`;
  const text = Array.from({ length: paragraphs }, (_, i) => `Paragraph ${i + 1} of ${title}. ` + 'The harbor reopened at dawn after a week of storms. '.repeat(3)).join('\n\n');
  return {
    route: { path: route, state: 'active', bare: route, permalink: `${route}/v1` },
    publication: { id: `pub-${slug}`, title, slug, state: 'published' },
    version: { id: `ver-${slug}-1`, content_hash: `hash-${slug}`, manifest_hash: `manifest-${slug}` },
    artifact: { content: text, body_doc: null },
    retrieval: {},
    citations: [],
    source_entities: [],
    transclusions: [],
    policy: {},
    proposals: {},
    provenance: {},
    payload: { revision: 1, manifest_hash: `manifest-${slug}`, manifest: '{}', resolved_blocks: [] },
    nav: { revision: 1, prev: 0, next: 0, latest: 1 },
    checks: { source: 'pending', citations: [] },
  };
}

/**
 * Mock the public routes. `relatedFor(slug)` answers a related request:
 * return a body, or { delayMs, body } to hold it, or { status } to fail it.
 * `log` collects { kind, slug, at } for resolve and related requests.
 */
async function mockRoutes(page, relatedFor, log = []) {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/resolve?*', async (route) => {
    const url = new URL(route.request().url());
    const slug = url.searchParams.get('route').replace('/pub/texture/', '').split('/')[0];
    log.push({ kind: 'resolve', slug, at: Date.now() });
    if (url.searchParams.get('view') === 'nav') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ nav: { revision: 1, latest: 1 } }) });
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle(slug, `Story ${slug.replace('story-', '').toUpperCase()}`)) });
    log.push({ kind: 'resolved', slug, at: Date.now() });
  });
  await page.route('**/api/platform/publications/related?*', async (route) => {
    const url = new URL(route.request().url());
    const slug = url.searchParams.get('route').replace('/pub/texture/', '').split('/')[0];
    log.push({ kind: 'related', slug, at: Date.now() });
    const answer = relatedFor(slug);
    if (answer?.status) {
      return route.fulfill({ status: answer.status, contentType: 'application/json', body: '{"error":"down"}' });
    }
    if (answer?.delayMs) await new Promise((resolve) => setTimeout(resolve, answer.delayMs));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(answer?.body ?? answer) });
  });
}

test.describe('the block on a story page', () => {
  test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });

  test('sits at the end of the scrolling article, below the last paragraph (U6, U8)', async ({ page }) => {
    await mockRoutes(page, () => related([link('b'), link('c'), link('d')]));
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    const block = reader.locator('[data-texture-related]');
    await expect(block).toHaveCount(1, { timeout: 15_000 });
    await expect(block.locator('h2')).toHaveText('More on this');
    await expect(block.locator('[data-related-link]')).toHaveCount(3);
    // Inside the article, after the last paragraph.
    const order = await reader.evaluate((el) => {
      const section = el.querySelector('[data-texture-related]');
      const paragraphs = el.querySelectorAll('p');
      return { last: el.lastElementChild === section, afterLast: paragraphs[paragraphs.length - 1].compareDocumentPosition(section) & Node.DOCUMENT_POSITION_FOLLOWING };
    });
    expect(order.last, 'the block is the article\'s last child').toBe(true);
    expect(order.afterLast).toBeTruthy();
    // Reachable by scrolling the article, and nothing else scrolls sideways.
    await reader.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(block).toBeInViewport();
    const overflow = await page.evaluate(() => [document.scrollingElement, document.querySelector('[data-texture-editor-area]')].map((el) => [el.scrollWidth, el.clientWidth]));
    for (const [scroll, client] of overflow) expect(scroll).toBeLessThanOrEqual(client + 1);
  });

  test('is fetched after the article is on screen and never holds it up (U4)', async ({ page }) => {
    const log = [];
    await mockRoutes(page, () => ({ delayMs: 2500, body: related([link('b')]) }), log);
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    await expect(reader).toContainText('Paragraph 1 of Story A');
    // The article is readable while the list is still pending.
    await expect(page.locator('[data-texture-related]')).toHaveCount(0);
    await expect(page.locator('[data-texture-related]')).toHaveCount(1, { timeout: 15_000 });
    const resolved = log.find((e) => e.kind === 'resolved');
    const asked = log.find((e) => e.kind === 'related');
    expect(resolved && asked, JSON.stringify(log)).toBeTruthy();
    expect(asked.at, 'related asked after resolve answered').toBeGreaterThanOrEqual(resolved.at);
  });

  test('shows nothing for an empty list, a failure, or a switched-off answer (U2, U9)', async ({ page }) => {
    for (const answer of [
      related([]),
      related([], { basis: { mode: 'off' } }),
      { status: 500 },
      { status: 502 },
      { not: 'a list' },
    ]) {
      await page.unrouteAll({ behavior: 'ignoreErrors' });
      await mockRoutes(page, () => answer);
      await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
      const reader = page.locator('[data-texture-published-reader]');
      await expect(reader).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
      await page.waitForTimeout(600);
      await expect(page.locator('[data-texture-related]'), JSON.stringify(answer)).toHaveCount(0);
      await expect(page.getByText('More on this')).toHaveCount(0);
      await expect(page.locator('[data-texture-error], .error-banner')).toHaveCount(0);
    }
  });

  test('a late answer for the story the reader left is dropped (U5)', async ({ page }) => {
    await mockRoutes(page, (slug) => (slug === 'story-a' ? { delayMs: 2000, body: related([link('late')]) } : related([])));
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    // The reader moves the window to story B before A's list arrives, as a
    // related link does.
    await reader.evaluate((el) => {
      const a = document.createElement('a');
      a.href = '/pub/texture/story-b';
      a.setAttribute('data-related-link', '');
      a.setAttribute('data-related-route', '/pub/texture/story-b');
      a.setAttribute('data-related-title', 'Story B');
      a.textContent = 'Story B';
      el.appendChild(a);
      a.click();
    });
    await expect(reader).toContainText('Paragraph 1 of Story B', { timeout: 30_000 });
    await page.waitForTimeout(3000);
    await expect(page.locator('[data-texture-related]'), 'story A\'s late list must not appear on story B').toHaveCount(0);
  });

  test('a click moves to the linked story at its top; a modified click is left alone (U7)', async ({ page, context }) => {
    await mockRoutes(page, (slug) => (slug === 'story-a' ? related([link('b')]) : related([])));
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    const anchor = reader.locator('[data-related-link]').first();
    await expect(anchor).toHaveAttribute('href', '/pub/texture/story-b');
    await reader.evaluate((el) => { el.scrollTop = el.scrollHeight; });

    // A ctrl-click is the browser's: this window stays on story A.
    await anchor.click({ modifiers: ['Control'], noWaitAfter: true });
    await page.waitForTimeout(300);
    await expect(page).toHaveURL(/\/pub\/texture\/story-a$/);
    await expect(reader).toContainText('Paragraph 1 of Story A');

    // A plain click moves this window to story B (the desktop keeps the
    // address of the window it opened).
    await anchor.click();
    await expect(reader).toContainText('Paragraph 1 of Story B', { timeout: 30_000 });
    await expect(reader).toHaveAttribute('data-publication-id', 'pub-story-b');
    expect(await reader.evaluate((el) => el.scrollTop), 'the new story opens at its top').toBe(0);
    await expect(page.locator('[data-texture-related]')).toHaveCount(0);
  });
});
