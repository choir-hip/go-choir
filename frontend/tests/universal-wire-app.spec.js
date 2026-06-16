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

test('Universal Wire renders an honest empty edition instead of preview stories', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.getByRole('heading', { name: 'Universal Wire' })).toBeVisible();
  await expect(app.locator('text=SourceMaxx newsroom')).toHaveCount(0);
  await expect(app.locator('text=Living source network')).toBeVisible();
  await expect(app.locator('[data-universal-wire-story]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
  await expect(app.locator('[data-universal-wire-empty-state]')).toContainText('No Wire edition articles yet');
  await expect(app.locator('text=Port backlog recedes')).toHaveCount(0);
  await expect(app.locator('text=seed source neighborhood')).toHaveCount(0);
});

test('Universal Wire retries authenticated story loads after transient route failure', async ({ browser, authenticatedState }) => {
  const context = await browser.newContext({
    storageState: authenticatedState.storageStatePath,
  });
  const page = await context.newPage();
  let storyFetches = 0;
  const liveStories = Array.from({ length: 4 }, (_, index) => ({
    id: `source-network-texture-${index + 1}`,
    headline: `Live source-network article ${index + 1}`,
    dek: 'A real source-network Texture article reached the Universal Wire front page.',
    freshness: 'updated 2 min ago',
    prominence: 90 - index,
    tension: 'source-network update',
    changeState: 'live article',
    nodeTone: 'live',
    related: [],
    manifest: { lead: [], supporting: [], contrary: [], context: [] },
    claims: ['The live source network has more than preview seed stories.'],
    projections: {
      'wire-style': 'The live article body is rendered from the authenticated Universal Wire story API after retry.',
    },
  }));
  try {
    await page.route('**/api/universal-wire/stories', async (route) => {
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
        body: JSON.stringify({ source: 'universal-wire-texture-index', stories: liveStories }),
      });
    });

    await page.goto(authenticatedState.baseURL);
    await openDeskApp(page, 'universal-wire');
    const app = page.locator('[data-universal-wire-app]');
    await expect(app).toBeVisible();
    await expect(app.locator('[data-universal-wire-story]')).toHaveCount(4, { timeout: 7000 });
    await expect(app.locator('[data-universal-wire-story]').first()).toContainText('Live source-network article 1');
    await expect(app.locator('text=Port backlog recedes')).toHaveCount(0);
    expect(storyFetches).toBeGreaterThanOrEqual(2);
  } finally {
    await context.close();
  }
});

test('Universal Wire deletes detritus source chronology and bespoke style controls', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app.locator('[data-universal-wire-evidence]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-style-switcher]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-source-search]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-fetch-cycle]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-open-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-compose-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-replace-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-ask-choir]')).toHaveCount(0);
  await expect(app.locator('text=Chronology')).toHaveCount(0);
  await expect(app.locator('text=Style.vtext')).toHaveCount(0);
});

test('Universal Wire has no nested dashboard panels, story boxes, theme selector, or Autoradio surface', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app.locator('text=Theme')).toHaveCount(0);
  await expect(app.locator('text=Autoradio')).toHaveCount(0);
  await expect(app.locator('text=Contribute')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph desk')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph news desk')).toHaveCount(0);

  await expect(app.locator('[data-universal-wire-story]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
});

test('Universal Wire remains a responsive Choir web desktop app across all three themes', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');
  const app = page.locator('[data-universal-wire-app]');

  for (const themeId of ['futuristic-noir', 'carbon-fiber-kintsugi', 'london-salmon']) {
    await applyTheme(page, themeId);
    await expect(page.locator('.app-root')).toHaveAttribute('data-theme-id', themeId);
    await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
  }

  await page.setViewportSize({ width: 430, height: 860 });
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();

  const layout = await app.evaluate((node) => {
    const paper = node.querySelector('.wire-paper');
    const columns = node.querySelector('.article-columns');
    return {
      paperDisplay: getComputedStyle(paper).display,
      columnTracks: columns ? getComputedStyle(columns).gridTemplateColumns.split(' ').length : 0,
    };
  });
  expect(layout.paperDisplay).toBe('block');
  expect(layout.columnTracks).toBe(0);
});
