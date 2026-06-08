import { test, expect } from './helpers/fixtures.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

async function applyTheme(page, id) {
  const names = {
    'futuristic-noir': 'Futuristic Noir',
    'carbon-fiber-kintsugi': 'Carbon Fiber Kintsugi',
    'london-salmon': 'London Salmon',
  };
  await page.evaluate(({ id, name }) => {
    window.dispatchEvent(new CustomEvent('choir-theme-change', {
      detail: {
        theme: {
          schema_version: 2,
          id,
          name,
        },
      },
    }));
  }, { id, name: names[id] });
}

test('Global Wire renders as a living newspaper surface with every article openable as VText', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.getByRole('heading', { name: 'Global Wire' })).toBeVisible();
  await expect(app.locator('text=SourceMaxx newsroom')).toHaveCount(0);
  await expect(app.locator('text=Living source network')).toBeVisible();
  await expect(app.locator('[data-global-wire-story]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-story-reader]').first()).toContainText('Port congestion indicators eased');

  await expect(app.locator('[data-global-wire-open-vtext]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-open-vtext]').first()).not.toContainText('Open in VText');
  await app.locator('[data-global-wire-open-vtext]').first().click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible();
  await expect(vtext.locator('[data-vtext-source-ref]').first()).toBeVisible();
  await expect(vtext.locator('[data-vtext-related-ref]').first()).toBeVisible();
  await expect(vtext).not.toContainText('Source Manifest');
  await expect(vtext).not.toContainText('User edits create user-owned versions');
  await expect(vtext).not.toContainText('The current version keeps');

  await vtext.locator('[data-vtext-related-ref]').first().click();
  const relatedVText = page.locator('[data-vtext-editor]').last();
  await expect(relatedVText).toContainText('Grid operators add reserve alerts as heat forecast shifts north');
  await expect(relatedVText).toContainText('Forecast changes moved stress');
});

test('Global Wire retries authenticated story loads after transient route failure', async ({ browser, authenticatedState }) => {
  const context = await browser.newContext({
    storageState: authenticatedState.storageStatePath,
  });
  const page = await context.newPage();
  let storyFetches = 0;
  const liveStories = Array.from({ length: 4 }, (_, index) => ({
    id: `source-network-vtext-${index + 1}`,
    headline: `Live source-network article ${index + 1}`,
    dek: 'A real source-network VText article reached the Global Wire front page.',
    freshness: 'updated 2 min ago',
    prominence: 90 - index,
    tension: 'source-network update',
    changeState: 'live article',
    nodeTone: 'live',
    related: [],
    manifest: { lead: [], supporting: [], contrary: [], context: [] },
    claims: ['The live source network has more than preview seed stories.'],
    projections: {
      'wire-style': 'The live article body is rendered from the authenticated Global Wire story API after retry.',
    },
  }));
  try {
    await page.route('**/api/global-wire/stories', async (route) => {
      storyFetches += 1;
      if (storyFetches === 1) {
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'route not ready' }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ source: 'durable-storygraph+source-network-vtexts', stories: liveStories }),
      });
    });

    await page.goto(authenticatedState.baseURL);
    await openDeskApp(page, 'global-wire');
    const app = page.locator('[data-global-wire-app]');
    await expect(app).toBeVisible();
    await expect(app.locator('[data-global-wire-story]')).toHaveCount(4, { timeout: 7000 });
    await expect(app.locator('[data-global-wire-story]').first()).toContainText('Live source-network article 1');
    await expect(app.locator('text=Port backlog recedes')).toHaveCount(0);
    expect(storyFetches).toBeGreaterThanOrEqual(2);
  } finally {
    await context.close();
  }
});

test('Global Wire deletes detritus source chronology and bespoke style controls', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app.locator('[data-global-wire-evidence]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-style-switcher]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-source-search]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-fetch-cycle]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-open-style]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-compose-style]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-replace-style]')).toHaveCount(0);
  await expect(app.locator('[data-global-wire-ask-choir]')).toHaveCount(0);
  await expect(app.locator('text=Chronology')).toHaveCount(0);
  await expect(app.locator('text=Style.vtext')).toHaveCount(0);
});

test('Global Wire has no nested dashboard panels, story boxes, theme selector, or Autoradio surface', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app.locator('text=Theme')).toHaveCount(0);
  await expect(app.locator('text=Autoradio')).toHaveCount(0);
  await expect(app.locator('text=Contribute')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph desk')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph news desk')).toHaveCount(0);

  const storyBoxBorder = await app.locator('[data-global-wire-story]').first().evaluate((node) => {
    const style = getComputedStyle(node);
    return {
      borderTopWidth: style.borderTopWidth,
      overflowY: style.overflowY,
    };
  });
  expect(storyBoxBorder.borderTopWidth).toBe('0px');
  expect(storyBoxBorder.overflowY).not.toBe('auto');
});

test('Global Wire remains a responsive Choir web desktop app across all three themes', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');
  const app = page.locator('[data-global-wire-app]');

  for (const themeId of ['futuristic-noir', 'carbon-fiber-kintsugi', 'london-salmon']) {
    await applyTheme(page, themeId);
    await expect(page.locator('.app-root')).toHaveAttribute('data-theme-id', themeId);
    await expect(app.locator('[data-global-wire-story]').first()).toBeVisible();
  }

  await page.setViewportSize({ width: 430, height: 860 });
  await expect(app.locator('[data-global-wire-story]').first()).toBeVisible();

  const layout = await app.evaluate((node) => {
    const paper = node.querySelector('.wire-paper');
    const columns = node.querySelector('.article-columns');
    return {
      paperDisplay: getComputedStyle(paper).display,
      columnTracks: getComputedStyle(columns).gridTemplateColumns.split(' ').length,
    };
  });
  expect(layout.paperDisplay).toBe('block');
  expect(layout.columnTracks).toBe(1);
});
