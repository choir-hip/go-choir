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

test('Global Wire renders as a newspaper SourceMaxx surface with every article openable as VText', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.locator('text=SourceMaxx desk')).toBeVisible();
  await expect(app.locator('[data-global-wire-story]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-story-reader]').first()).toContainText('Port congestion indicators eased');
  await expect(app.locator('[data-global-wire-evidence]')).toContainText('Port authority throughput bulletin');
  await expect(app.locator('[data-global-wire-evidence]')).toContainText('Regional grid operator reserve notice');

  await expect(app.locator('[data-global-wire-open-vtext]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-open-vtext]').first()).not.toContainText('Open in VText');
  await app.locator('[data-global-wire-open-vtext]').first().click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible();
  await expect(vtext).toContainText('Source Manifest');
  await expect(vtext).toContainText('User edits create user-owned versions');
});

test('Global Wire keeps Style.vtext routing compact and source provenance visible', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await app.locator('[data-global-wire-style-switcher] button').filter({ hasText: 'Audit' }).click();
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('Cites Style.vtext: Claim Audit');
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('source provenance stays with the VText version');
  await expect(app.locator('[data-global-wire-story-reader]').first()).toContainText('strongest supported claim');
});

test('Global Wire has no nested dashboard panels, story boxes, theme selector, or Autoradio surface', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app.locator('text=Theme')).toHaveCount(0);
  await expect(app.locator('text=Autoradio')).toHaveCount(0);
  await expect(app.locator('text=Contribute')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph desk')).toHaveCount(0);

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
    await expect(app.locator('[data-global-wire-evidence]')).toBeVisible();
  }

  await page.setViewportSize({ width: 430, height: 860 });
  await expect(app.locator('[data-global-wire-story]').first()).toBeVisible();
  await expect(app.locator('[data-global-wire-evidence]')).toBeVisible();

  const layout = await app.evaluate((node) => {
    const paper = node.querySelector('.wire-paper');
    const columns = node.querySelector('.article-columns');
    return {
      paperDisplay: getComputedStyle(paper).display,
      columnCount: getComputedStyle(columns).columnCount,
    };
  });
  expect(layout.paperDisplay).toBe('block');
  expect(layout.columnCount).toBe('1');
});
