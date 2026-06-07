import { test, expect } from '@playwright/test';

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

test('Global Wire preserves StoryGraph, Style.vtext, VText fork, and contribution controls', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.locator('[data-global-wire-story]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-story-reader]')).toContainText('Port backlog recedes');
  await expect(app.locator('[data-global-wire-evidence] [data-source-tier="lead"]')).toContainText('Port authority throughput bulletin');
  await expect(app.locator('[data-global-wire-story-graph]')).toContainText('Grid operators add reserve alerts');

  await app.locator('[data-global-wire-style-switcher] button').filter({ hasText: 'Audit' }).click();
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('Cites Style.vtext: Claim Audit');
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('evidence manifest unchanged');

  await app.locator('[data-global-wire-open-vtext]').click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible();
  await expect(vtext).toContainText('Source Manifest');
  await expect(vtext).toContainText('Style source: Style.vtext: Claim Audit');
});

for (const themeId of ['futuristic-noir', 'carbon-fiber-kintsugi', 'london-salmon']) {
  test(`Global Wire renders core views in ${themeId}`, async ({ page }) => {
    await page.goto(BASE_URL);
    await applyTheme(page, themeId);
    await openDeskApp(page, 'global-wire');

    const app = page.locator('[data-global-wire-app]');
    await expect(app.locator('[data-global-wire-front-page]')).toBeVisible();
    await expect(app.locator('[data-global-wire-story-reader]')).toBeVisible();
    await expect(app.locator('[data-global-wire-evidence]')).toBeVisible();
    await expect(app.locator('[data-global-wire-story-graph]')).toBeVisible();
    await expect(app.locator('[data-global-wire-contribution]')).toBeVisible();

    const overflow = await app.evaluate((node) => {
      const rect = node.getBoundingClientRect();
      return {
        width: rect.width,
        scrollWidth: node.scrollWidth,
        height: rect.height,
        scrollHeight: node.scrollHeight,
      };
    });
    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.width + 2);
    expect(overflow.scrollHeight).toBeGreaterThan(0);
  });
}
