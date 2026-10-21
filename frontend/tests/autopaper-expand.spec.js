/**
 * "More research" on a published story (story expansion design, phase 1,
 * docs/reports/story-expansion-request-design-2026-10-20.md), against the
 * built frontend with the platform routes mocked. The control is a button in
 * the toolbar of the story's Texture window (owner, 2026-10-21: "More
 * research should be a button in the texture toolbar"; the standalone
 * /autopaper page is gone), so every case here opens the story the way a
 * logged-out reader does: /pub/texture/<slug> on the desktop. The deployed proof is
 * autopaper-expand-deployed.spec.js; this spec pins the UI's own failure
 * modes. The failure modes below come from the design (section 7) and were
 * listed before the component; the component and this spec were then written
 * together, not strictly test first (noted in the change's report).
 *
 * Ways the control can fail:
 *   X1 Anyone but an allowed reader sees it: allowed:false, a failed or
 *      malformed read, or a server that sends allowed as a string must show
 *      nothing (no button, no blank gap, no error).
 *   X2 A failed read breaks the story (error banner, no text).
 *   X3 A double click (or a second tap) sends two asks.
 *   X4 A late read for story A lands on story B after the reader moved.
 *   X5 The fate line shows the server's words as HTML, or a state the page
 *      does not know shows a stray line.
 *   X6 Polling never stops after a settled fate, never starts for an open
 *      one, or runs faster than 30 seconds.
 *   X7 The toolbar overflows a phone sideways, or the button is too small to
 *      tap (under 44 px on a touch screen).
 *   X9 The button shows in a Texture window that is not a published story
 *      (a private document), or asks the server about it.
 *   X8 A refused ask (429 limit, 409 already asked) leaves the button stuck
 *      or the reader without a word.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/autopaper-expand.spec.js
 */
import { expect, test } from '@playwright/test';
import {
  EXPAND_POLL_MS,
  askNotice,
  expansionIsOpen,
  fateLine,
  latestRequest,
} from '../src/lib/autopaper-expand.ts';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');

// ---- the pure parts ----

test.describe('fate lines and polling decisions', () => {
  test('every known state has a plain line and an unknown one has none (X5)', () => {
    expect(fateLine({ request_id: 'rx-1', state: 'asked' })).toContain('Asked');
    expect(fateLine({ request_id: 'rx-1', state: 'accepted' })).toContain('may add research or leave the story');
    expect(fateLine({ request_id: 'rx-1', state: 'revised', to_version: 4 })).toBe('Revised since your request: version 4 is up.');
    expect(fateLine({ request_id: 'rx-1', state: 'revised', to_version: '<b>x</b>' })).toBe('Revised since your request.');
    expect(fateLine({ request_id: 'rx-1', state: 'no_new_version' })).toContain('No new version yet');
    expect(fateLine({ request_id: 'rx-1', state: 'unknown' })).toContain('Could not check');
    expect(fateLine({ request_id: 'rx-1', state: '<img src=x onerror=alert(1)>' })).toBe('');
    expect(fateLine(null)).toBe('');
  });

  test('only an asked or accepted request keeps the page polling (X6)', () => {
    const read = (state) => ({ allowed: true, requests: [{ request_id: 'rx-1', state }] });
    expect(expansionIsOpen(read('asked'))).toBe(true);
    expect(expansionIsOpen(read('accepted'))).toBe(true);
    for (const state of ['revised', 'no_new_version', 'unknown', 'whatever']) expect(expansionIsOpen(read(state)), state).toBe(false);
    expect(expansionIsOpen({ allowed: true, requests: [] })).toBe(false);
    expect(expansionIsOpen(null)).toBe(false);
    expect(latestRequest({ allowed: true, requests: [{ request_id: 'a', state: 'asked' }, { request_id: 'b', state: 'revised' }] }).request_id).toBe('a');
    expect(EXPAND_POLL_MS).toBe(30_000);
  });

  test('a refusal always has a word (X8)', () => {
    expect(askNotice(409, 'already_asked')).toContain('already asked');
    expect(askNotice(429, 'limit')).toContain('Try again later');
    expect(askNotice(429, '')).toContain('Try again later');
    expect(askNotice(429, 'address_limit')).toBe('Too many requests from your address; try later.');
    expect(askNotice(409, 'busy')).toContain('busy');
    expect(askNotice(502, 'unavailable')).toContain('not reachable');
    expect(askNotice(403, '')).toContain('not available');
    expect(askNotice(500, '')).toContain('Could not ask');
    expect(askNotice(0, '')).toContain('Could not ask');
  });
});

// ---- the page ----

function bundle(slug, title, paragraphs = 8) {
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

const slugOf = (url) => new URL(url).searchParams.get('route').replace('/pub/texture/', '').split('/')[0];

/**
 * Mock the platform routes. `state.read` is the expansion read answer (a
 * function of the slug, or { status } to fail it, or { delayMs, body });
 * `state.ask` answers the POST ({ status, body }). Requests are logged.
 */
async function mockRoutes(page, state) {
  state.reads = [];
  state.asks = [];
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/related?*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"links":[]}' }));
  await page.route('**/api/platform/publications/resolve?*', async (route) => {
    const slug = slugOf(route.request().url());
    if (new URL(route.request().url()).searchParams.get('view') === 'nav') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ nav: { revision: 1, latest: 1 } }) });
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(bundle(slug, `Story ${slug.replace('story-', '').toUpperCase()}`)) });
  });
  await page.route('**/api/platform/publications/expansion?*', async (route) => {
    const slug = slugOf(route.request().url());
    state.reads.push({ slug, at: Date.now() });
    const answer = typeof state.read === 'function' ? state.read(slug) : state.read;
    if (answer?.status) return route.fulfill({ status: answer.status, contentType: 'application/json', body: '{"error":"x"}' });
    if (answer?.delayMs) await new Promise((resolve) => setTimeout(resolve, answer.delayMs));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(answer?.body ?? answer) });
  });
  await page.route('**/api/platform/publications/expand', async (route) => {
    state.asks.push(JSON.parse(route.request().postData() || '{}'));
    const answer = state.ask || { status: 202, body: { status: 'asked', request_id: 'rx-1' } };
    await new Promise((resolve) => setTimeout(resolve, answer.delayMs || 0));
    await route.fulfill({ status: answer.status, contentType: 'application/json', body: JSON.stringify(answer.body) });
  });
}

const allowed = (extra = {}) => ({ allowed: true, route: '/pub/texture/story-a', asked: false, requests: [], ...extra });
const requestOf = (state, extra = {}) => ({ request_id: 'rx-1', state, asked_at: '2026-10-20T12:00:00Z', from_version: 1, ...extra });

test.describe('the control in a story window\'s toolbar', () => {
  test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true, storageState: { cookies: [], origins: [] } });

  async function openStory(page, slug = 'story-a') {
    await page.goto(`${BASE_URL}/pub/texture/${slug}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-texture-published-reader]')).toContainText(`Paragraph 1 of Story ${slug.replace('story-', '').toUpperCase()}`, { timeout: 30_000 });
  }

  test('shows nothing unless the read says allowed:true, and never breaks the story (X1, X2)', async ({ page }) => {
    for (const answer of [
      { allowed: false },
      { status: 500 },
      { status: 502 },
      { allowed: 'true' },
      { not: 'an answer' },
      { allowed: 1 },
    ]) {
      await page.unrouteAll({ behavior: 'ignoreErrors' });
      const state = { read: answer };
      await mockRoutes(page, state);
      await openStory(page);
      await expect.poll(() => state.reads.length).toBeGreaterThan(0);
      await page.waitForTimeout(400);
      await expect(page.locator('[data-autopaper-expand]'), JSON.stringify(answer)).toHaveCount(0);
      await expect(page.getByText('More research')).toHaveCount(0);
      await expect(page.locator('[data-texture-error], .error-banner')).toHaveCount(0);
    }
  });

  test('fits a phone, is tappable, and one ask is sent for a double click (X3, X7)', async ({ page }) => {
    const state = { read: allowed(), ask: { status: 202, delayMs: 300, body: { status: 'asked', request_id: 'rx-1' } } };
    await mockRoutes(page, state);
    await openStory(page);
    const button = page.locator('[data-texture-toolbar] [data-autopaper-expand-button]');
    await expect(button).toBeVisible();
    await expect(button).toHaveText('More research');
    // Layout height: the window opens with a small scale animation that
    // would shave a fraction off a bounding box.
    expect(await button.evaluate((el) => el.offsetHeight), 'tap target height').toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => [document.scrollingElement.scrollWidth, document.scrollingElement.clientWidth, document.querySelector('[data-texture-toolbar]').scrollWidth, document.querySelector('[data-texture-toolbar]').clientWidth]);
    expect(overflow[0]).toBeLessThanOrEqual(overflow[1] + 1);
    expect(overflow[2]).toBeLessThanOrEqual(overflow[3] + 1);

    state.read = allowed({ asked: true, requests: [requestOf('asked')] });
    await button.dblclick();
    await expect(page.locator('[data-autopaper-expand-line]')).toContainText('Asked', { timeout: 10_000 });
    expect(state.asks, 'exactly one ask for a double click').toHaveLength(1);
    expect(state.asks[0]).toEqual({ route: '/pub/texture/story-a' });
    await expect(button).toBeDisabled();
    await expect(button).toHaveText('Asked for more');
    // The line still fits a phone.
    const after = await page.evaluate(() => [document.querySelector('[data-texture-toolbar]').scrollWidth, document.querySelector('[data-texture-toolbar]').clientWidth]);
    expect(after[0]).toBeLessThanOrEqual(after[1] + 1);
  });

  test('follows an open request every 30 seconds and stops when it settles (X6)', async ({ page }) => {
    // The desktop runs many timers of its own, so moving the clock by hand
    // takes real seconds here.
    test.setTimeout(120_000);
    const state = { read: allowed({ asked: true, requests: [requestOf('accepted')] }) };
    await mockRoutes(page, state);
    // The page's own timers are fake from the start (time still flows), so
    // the poll timer the component sets can be moved by hand.
    await page.clock.install();
    await openStory(page);
    await expect(page.locator('[data-autopaper-expand-line]')).toContainText('Request received');
    const before = state.reads.length;
    await page.clock.runFor(EXPAND_POLL_MS - 1000);
    expect(state.reads.length, 'no poll inside 30 s').toBe(before);
    await page.clock.runFor(2000);
    await expect.poll(() => state.reads.length).toBe(before + 1);

    state.read = allowed({ asked: true, requests: [requestOf('revised', { to_version: 3 })] });
    await page.clock.runFor(EXPAND_POLL_MS);
    await expect(page.locator('[data-autopaper-expand-line]')).toHaveText('Revised since your request: version 3 is up.');
    const settled = state.reads.length;
    await page.clock.runFor(EXPAND_POLL_MS * 2);
    expect(state.reads.length, 'no more polls once settled').toBe(settled);
  });

  test('says why when an ask is refused, and keeps the button for another try (X8)', async ({ page }) => {
    const state = { read: allowed(), ask: { status: 429, body: { status: 'limit', error: 'too many' } } };
    await mockRoutes(page, state);
    await openStory(page);
    const button = page.locator('[data-autopaper-expand-button]');
    await button.click();
    await expect(page.locator('[data-autopaper-expand-line]')).toContainText('Try again later', { timeout: 10_000 });
    await expect(button).toBeEnabled();
    await expect(button).toHaveText('More research');

    state.ask = { status: 409, body: { status: 'already_asked' } };
    state.read = allowed({ asked: true, requests: [requestOf('accepted')] });
    await button.click();
    await expect(page.locator('[data-autopaper-expand-line]')).toContainText('already asked', { timeout: 10_000 });
  });

  test('an address over its allowance is told so in the fate line (X8)', async ({ page }) => {
    const state = { read: allowed(), ask: { status: 429, body: { status: 'address_limit', error: 'too many requests from your address; try later' } } };
    await mockRoutes(page, state);
    await openStory(page);
    await page.locator('[data-autopaper-expand-button]').click();
    await expect(page.locator('[data-autopaper-expand-line]')).toHaveText('Too many requests from your address; try later.', { timeout: 10_000 });
    await expect(page.locator('[data-autopaper-expand-button]')).toBeEnabled();
  });

  test('a late read for the story the reader left is dropped (X4)', async ({ page }) => {
    const state = { read: (slug) => (slug === 'story-a' ? { delayMs: 2000, body: allowed() } : { allowed: false }) };
    await mockRoutes(page, state);
    await openStory(page);
    // The reader moves the window to story B, as a related link does.
    await page.locator('[data-texture-published-reader]').evaluate((el) => {
      const a = document.createElement('a');
      a.href = '/pub/texture/story-b';
      a.setAttribute('data-related-link', '');
      a.setAttribute('data-related-route', '/pub/texture/story-b');
      a.setAttribute('data-related-title', 'Story B');
      a.textContent = 'Story B';
      el.appendChild(a);
      a.click();
    });
    await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story B', { timeout: 30_000 });
    await page.waitForTimeout(3000);
    await expect(page.locator('[data-autopaper-expand]'), 'story A\'s late read must not show on story B').toHaveCount(0);
  });

  test('a Texture window on a private document has no button and asks nothing (X9)', async ({ page }) => {
    const state = { read: allowed() };
    await page.setViewportSize({ width: 1280, height: 800 });
    await mockRoutes(page, state);
    await page.route('**/api/platform/publications/front*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"stories":[]}' }));
    await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    await page.locator('[data-desktop-icon-id="texture"]').click({ timeout: 20_000 });
    await expect(page.locator('[data-texture-toolbar]')).toBeVisible({ timeout: 20_000 });
    await page.waitForTimeout(500);
    await expect(page.locator('[data-autopaper-expand]')).toHaveCount(0);
    expect(state.reads, 'no expansion read for a private document').toHaveLength(0);
  });
});
