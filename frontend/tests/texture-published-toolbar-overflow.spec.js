/**
 * The Texture toolbar of a published story: one row, with a "⋯" menu for the
 * controls that do not fit (owner, 2026-10-21: "The toolbar should have a
 * '3 dots' menu that expands if the content is too wide for the screen"), and
 * the old /autopaper links (owner: "Get rid of the standalone page";
 * docs/problems/research-more-slow-and-hidden-2026-10-21.md, E2). Built
 * frontend (vite preview) with the public platform routes mocked.
 *
 * Ways it can fail:
 *   R1 At 375 px the toolbar overflows sideways (its content wider than the
 *      toolbar, a control past the screen edge, or a page scrollbar), or wraps
 *      to a second row, at the default text size or with the root font raised
 *      to 150% or 200%.
 *   R2 A control is dropped, not moved into the menu, to make a phone fit, or
 *      the menu holds a copy that diverges from the toolbar's control (one
 *      "More research" button must exist, and it keeps its disabled state).
 *   R3 On a desktop width the menu button shows with nothing to hold, or the
 *      toolbar grows tall: it must stay one compact row.
 *   R4 An old /autopaper/<slug> or /autopaper/<slug>/v<n> link lands on a
 *      blank or missing page; /autopaper alone must reach the desktop home.
 *   R5 The menu is not usable: no aria state on the button, Escape or a click
 *      outside leaves it open, arrow keys do not move through it, a control in
 *      it is under 44 px on a touch screen, or acting on it leaves it open.
 *   R6 The fit is a fixed breakpoint: widening the window after a narrow load
 *      must bring the controls back into the row and remove the menu button,
 *      and narrowing it again must bring the button back.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/texture-published-toolbar-overflow.spec.js
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

/**
 * `asked: true` is an open request (the button reads "Asked for more" and is
 * disabled, and the fate line shows); `asked: false` is a button that can be
 * pressed, and the POST is logged in `asks`.
 */
async function mockRoutes(page, { allowed = true, asked = true, resolved = [], asks = [] } = {}) {
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
    const open = asked || asks.length > 0;
    const body = !allowed
      ? { allowed: false }
      : { allowed: true, route: '/pub/texture/story-a', asked: open, requests: open ? [{ request_id: 'rx-1', state: 'accepted', asked_at: '2026-10-21T12:00:00Z', from_version: 12 }] : [] };
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await page.route('**/api/platform/publications/expand', (route) => {
    asks.push(JSON.parse(route.request().postData() || '{}'));
    return route.fulfill({ status: 202, contentType: 'application/json', body: '{"status":"asked","request_id":"rx-1"}' });
  });
}

/** Every number the overflow checks need, read in the page (the open menu's controls are not the row's). */
async function toolbarFit(page) {
  return page.evaluate(() => {
    const toolbar = document.querySelector('[data-texture-toolbar]');
    const tr = toolbar.getBoundingClientRect();
    const controls = [...toolbar.querySelectorAll('button, summary, [data-texture-version]')]
      .filter((el) => el.getClientRects().length > 0 && (el.tagName === 'SUMMARY' || !el.closest('details:not([open])')))
      .filter((el) => !el.closest('[data-texture-toolbar-menu]'))
      .map((el) => {
        const r = el.getBoundingClientRect();
        return { label: (el.innerText || '').trim().slice(0, 24), left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), bottom: Math.round(r.bottom) };
      });
    const more = toolbar.querySelector('[data-texture-toolbar-more]');
    const row = toolbar.querySelector('.tb-row').getBoundingClientRect();
    return {
      viewport: window.innerWidth,
      moreVisible: !!more && more.getClientRects().length > 0,
      pageScroll: [document.scrollingElement.scrollWidth, document.scrollingElement.clientWidth],
      toolbarScroll: [toolbar.scrollWidth, toolbar.clientWidth],
      toolbarBox: { left: Math.round(tr.left), right: Math.round(tr.right), height: Math.round(tr.height) },
      rowHeight: Math.round(row.height),
      controls,
    };
  });
}

// The button exists once the proxy says this reader may ask; it sits in the
// row or, when the row is full, in the menu (display:none until opened).
async function openStory(page) {
  await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
  await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
  await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-button]')).toHaveCount(1, { timeout: 15_000 });
  await page.waitForTimeout(300);
}

async function setRootFont(page, percent) {
  await page.addStyleTag({ content: `html { font-size: ${percent}% !important; }` });
  await page.waitForTimeout(400);
}

const moreButton = (page) => page.locator('[data-texture-toolbar] [data-texture-toolbar-more]');
const menu = (page) => page.locator('[data-texture-toolbar] [data-texture-toolbar-menu]');
const expandButton = (page) => page.locator('[data-texture-toolbar] [data-autopaper-expand-button]');

/** The labels of the controls in the menu, in order (a Download item is one label). */
const menuLabels = (page) => menu(page).evaluate((el) => [...el.querySelectorAll(':scope > [data-tb-item]')]
  .filter((item) => item.firstElementChild)
  .map((item) => (item.querySelector('button, summary')?.innerText || '').trim().replace(/\s+/g, ' ')));

function expectOneRow(fit, label) {
  const tops = fit.controls.map((c) => c.top);
  expect(Math.max(...tops) - Math.min(...tops), `${label}: all controls on one row`).toBeLessThanOrEqual(12);
  expect(fit.pageScroll[0], `${label}: page scrolls sideways`).toBeLessThanOrEqual(fit.pageScroll[1]);
  expect(fit.toolbarScroll[0], `${label}: toolbar content wider than the toolbar`).toBeLessThanOrEqual(fit.toolbarScroll[1] + 1);
  for (const c of fit.controls) {
    expect(c.left, `${label}: ${c.label} left edge`).toBeGreaterThanOrEqual(fit.toolbarBox.left - 1);
    expect(c.right, `${label}: ${c.label} right edge`).toBeLessThanOrEqual(fit.toolbarBox.right + 1);
  }
}

test.describe('the toolbar at phone width', () => {
  test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true, storageState: { cookies: [], origins: [] } });

  for (const percent of [100, 150, 200]) {
    test(`is one row and every control is in the row or the menu, root font ${percent}% (R1, R2)`, async ({ page }) => {
      await mockRoutes(page, { asked: false });
      await openStory(page);
      await setRootFont(page, percent);

      const fit = await toolbarFit(page);
      expectOneRow(fit, `${percent}%`);
      const labels = fit.controls.map((c) => c.label);
      for (const want of ['v12', '<', '>']) expect(labels.join('|'), want).toContain(want);
      // A phone is too narrow for everything: the menu button is there.
      expect(fit.moreVisible, 'the menu button').toBe(true);

      // Nothing is dropped: each control is in the row or in the menu, once.
      await moreButton(page).click();
      const inMenu = await menuLabels(page);
      const all = [...labels, ...inMenu].join('|');
      for (const want of ['More research', 'Edit', 'Copy text', 'Download']) expect(all, want).toContain(want);
      await expect(expandButton(page), 'one More research button, not a copy').toHaveCount(1);
      // More research is visible in the row or the first item of the menu.
      if (!labels.some((l) => l.startsWith('More research'))) expect(inMenu[0]).toContain('More research');
    });
  }

  test('at 200% text: one row, the menu button, the hidden controls, and More research works (R1, R2, R5)', async ({ page }) => {
    const asks = [];
    await mockRoutes(page, { asked: false, asks });
    await openStory(page);
    await setRootFont(page, 200);

    const fit = await toolbarFit(page);
    expectOneRow(fit, '200%');
    expect(fit.rowHeight, 'the row is one control tall, not wrapped').toBeLessThan(2 * Math.max(...fit.controls.map((c) => c.bottom - c.top)));
    expect(fit.moreVisible).toBe(true);
    await expect(moreButton(page)).toBeVisible();
    await expect(moreButton(page)).toHaveAttribute('aria-haspopup', 'menu');
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'false');
    // Closed, the hidden controls are not on screen (nor reachable by Tab).
    await expect(menu(page)).toBeHidden();
    await expect(expandButton(page)).toBeHidden();

    await moreButton(page).click();
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'true');
    await expect(menu(page)).toBeVisible();
    // The hidden controls, in priority order, with More research first.
    expect(await menuLabels(page)).toEqual(['More research', 'Edit', 'Copy text', 'Download']);
    await expect(expandButton(page)).toBeVisible();
    await expect(expandButton(page)).toBeEnabled();

    // The menu is on screen and every control in it is a finger-sized target.
    const geometry = await menu(page).evaluate((el) => {
      const box = el.getBoundingClientRect();
      return {
        left: box.left,
        right: box.right,
        bottom: box.bottom,
        viewport: [window.innerWidth, window.innerHeight],
        heights: [...el.querySelectorAll(':scope > [data-tb-item] > button, :scope > [data-tb-item] summary, :scope > [data-tb-item] [data-autopaper-expand-button]')]
          .map((n) => Math.round(n.getBoundingClientRect().height)),
      };
    });
    expect(geometry.left).toBeGreaterThanOrEqual(0);
    expect(geometry.right).toBeLessThanOrEqual(geometry.viewport[0]);
    expect(geometry.bottom).toBeLessThanOrEqual(geometry.viewport[1]);
    expect(geometry.heights.length).toBe(4);
    for (const h of geometry.heights) expect(h, 'menu control height').toBeGreaterThanOrEqual(44);

    // The control in the menu is the toolbar's control: pressing it asks once,
    // closes the menu, and its fate line shows under the row.
    await expandButton(page).click();
    await expect.poll(() => asks.length).toBe(1);
    expect(asks[0]).toEqual({ route: '/pub/texture/story-a' });
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'false');
    await expect(menu(page)).toBeHidden();
    const line = page.locator('[data-texture-toolbar] [data-autopaper-expand-line]');
    await expect(line).toBeVisible({ timeout: 10_000 });
    const lineBox = await line.boundingBox();
    expect(lineBox.x).toBeGreaterThanOrEqual(0);
    expect(lineBox.x + lineBox.width).toBeLessThanOrEqual(fit.viewport + 1);
    // Reopened, it is the same button, now disabled by the request it made.
    await moreButton(page).click();
    await expect(expandButton(page)).toBeDisabled();
    await expect(expandButton(page)).toHaveText('Asked for more');
    await expect(expandButton(page)).toHaveCount(1);
    // The toolbar did not eat the screen: the article still starts on screen.
    const readerTop = await page.locator('[data-texture-published-reader]').evaluate((el) => el.getBoundingClientRect().top);
    expect(readerTop, 'article start').toBeLessThan(700);
  });

  test('a request already open shows disabled in the menu, with its line in the toolbar (R2)', async ({ page }) => {
    await mockRoutes(page, { asked: true });
    await openStory(page);
    await setRootFont(page, 200);
    await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-line]')).toBeVisible();
    await moreButton(page).click();
    await expect(expandButton(page)).toBeDisabled();
    await expect(expandButton(page)).toHaveText('Asked for more');
  });

  test('keyboard: Enter opens, arrows move, Escape closes and returns to the button (R5)', async ({ page }) => {
    await mockRoutes(page, { asked: false });
    await openStory(page);
    await setRootFont(page, 200);

    const focused = () => page.evaluate(() => (document.activeElement?.innerText || '').trim().replace(/\s+/g, ' '));
    await moreButton(page).focus();
    await page.keyboard.press('Enter');
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'true');
    await expect.poll(focused).toBe('More research');
    await page.keyboard.press('ArrowDown');
    await expect.poll(focused).toBe('Edit');
    await page.keyboard.press('End');
    await expect.poll(focused).toBe('Download');
    await page.keyboard.press('ArrowDown');
    await expect.poll(focused).toBe('More research');
    await page.keyboard.press('ArrowUp');
    await expect.poll(focused).toBe('Download');
    await page.keyboard.press('Home');
    await expect.poll(focused).toBe('More research');

    await page.keyboard.press('Escape');
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'false');
    await expect(menu(page)).toBeHidden();
    await expect(moreButton(page)).toBeFocused();
  });

  test('a click outside closes the menu; a control that opens something stays inside it (R5)', async ({ page }) => {
    await mockRoutes(page, { asked: false });
    await openStory(page);
    await setRootFont(page, 200);

    await moreButton(page).click();
    await expect(menu(page)).toBeVisible();
    // Download opens its own list inside the menu and the menu stays open.
    await page.locator('[data-texture-toolbar-menu] [data-texture-download-menu] summary').click();
    await expect(page.locator('[data-texture-toolbar-menu] [data-texture-download-md]')).toBeVisible();
    await expect(menu(page)).toBeVisible();
    await expect(page.locator('[data-texture-toolbar-menu] [data-texture-download-md]')).toBeEnabled();

    await page.locator('[data-texture-published-reader]').click({ position: { x: 10, y: 10 } });
    await expect(moreButton(page)).toHaveAttribute('aria-expanded', 'false');
    await expect(menu(page)).toBeHidden();
  });
});

test.describe('the toolbar follows the width it is given', () => {
  test.use({ viewport: { width: 1280, height: 800 }, storageState: { cookies: [], origins: [] } });

  // The story window keeps the size it opened with when the browser window
  // changes, so the width is moved the way a reader drags the window.
  const setWindowWidth = (page, width) => page.evaluate((w) => {
    const win = document.querySelector('[data-window-app-id="texture"]');
    Object.assign(win.style, { width: `${w}px`, left: '20px', right: 'auto' });
  }, width);

  test('narrowing the window fills the menu, widening it empties it (R6)', async ({ page }) => {
    await mockRoutes(page, { asked: false });
    await openStory(page);
    await expect(moreButton(page)).toBeHidden();

    await setWindowWidth(page, 360);
    await expect(moreButton(page)).toBeVisible();
    expectOneRow(await toolbarFit(page), 'narrow');
    await expect(expandButton(page)).toHaveCount(1);

    await setWindowWidth(page, 1100);
    await expect(moreButton(page)).toBeHidden();
    const wide = await toolbarFit(page);
    for (const want of ['More research', 'Edit', 'Copy text', 'Download']) expect(wide.controls.map((c) => c.label).join('|'), want).toContain(want);
    expectOneRow(wide, 'wide');
    await expect(menu(page)).toBeHidden();

    // A step between: some controls stay in the row, the rest are in the menu.
    await setWindowWidth(page, 600);
    const mid = await toolbarFit(page);
    expectOneRow(mid, 'middle');
    if (mid.moreVisible) {
      await moreButton(page).click();
      const all = [...mid.controls.map((c) => c.label), ...(await menuLabels(page))].join('|');
      for (const want of ['More research', 'Edit', 'Copy text', 'Download']) expect(all, want).toContain(want);
    }
  });
});

test.describe('the toolbar on a desktop width', () => {
  test.use({ viewport: { width: 1280, height: 800 }, storageState: { cookies: [], origins: [] } });

  test('has no menu button and stays one compact row with the button in it (R3)', async ({ page }) => {
    await mockRoutes(page, { allowed: false });
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-texture-published-reader]')).toContainText('Paragraph 1 of Story A', { timeout: 30_000 });
    const bare = await toolbarFit(page);
    expect(bare.moreVisible, 'menu button without the research button').toBe(false);
    expect(bare.toolbarBox.height, 'toolbar height without the button').toBeLessThanOrEqual(64);
    await page.unrouteAll({ behavior: 'ignoreErrors' });

    await mockRoutes(page);
    await page.goto(`${BASE_URL}/pub/texture/story-a`, { waitUntil: 'domcontentloaded' });
    await expect(expandButton(page)).toBeVisible({ timeout: 30_000 });
    // The fate line sits in the toolbar too; the toolbar grows by at most one line of it.
    await expect(page.locator('[data-texture-toolbar] [data-autopaper-expand-line]')).toBeVisible();
    await expect(moreButton(page)).toBeHidden();
    const fit = await toolbarFit(page);
    expect(fit.moreVisible, 'menu button on a desktop').toBe(false);
    expectOneRow(fit, 'desktop');
    expect(fit.toolbarBox.height, 'toolbar height with the button and its line').toBeLessThanOrEqual(96);
    for (const want of ['v12', '<', '>', 'Copy text', 'Download', 'Asked for more']) expect(fit.controls.map((c) => c.label).join('|'), want).toContain(want);
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
