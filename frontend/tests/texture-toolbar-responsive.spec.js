import { test, expect } from './helpers/fixtures.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

async function setTextureWindowWidth(page, width) {
  await page.evaluate((targetWidth) => {
    const windowNode = document.querySelector('[data-window-app-id="texture"]');
    if (!windowNode) throw new Error('Texture window not found');
    Object.assign(windowNode.style, {
      width: `${targetWidth}px`,
      left: '20px',
      right: 'auto',
      top: '24px',
    });
  }, width);
}

async function toolbarMetrics(page) {
  return page.evaluate(() => {
    const toolbar = document.querySelector('[data-texture-toolbar]');
    if (!toolbar) throw new Error('Texture toolbar not found');

    const visibleItems = [
      ...toolbar.querySelectorAll('.version-controls > *, .doc-state, .doc-actions > .tb-item > *, .tb-more > .more-btn'),
    ].filter((node) => {
      const style = getComputedStyle(node);
      const rect = node.getBoundingClientRect();
      return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
    }).map((node) => {
      const rect = node.getBoundingClientRect();
      return {
        text: (node.innerText || node.textContent || '').trim().replace(/\s+/g, ' '),
        rect: {
          left: rect.left,
          top: rect.top,
          right: rect.right,
          bottom: rect.bottom,
          width: rect.width,
          height: rect.height,
        },
      };
    });

    const toolbarRect = toolbar.getBoundingClientRect();
    const overlapPairs = [];
    for (let leftIndex = 0; leftIndex < visibleItems.length; leftIndex += 1) {
      for (let rightIndex = leftIndex + 1; rightIndex < visibleItems.length; rightIndex += 1) {
        const left = visibleItems[leftIndex].rect;
        const right = visibleItems[rightIndex].rect;
        const overlaps = !(left.right <= right.left || right.right <= left.left || left.bottom <= right.top || right.bottom <= left.top);
        if (overlaps) overlapPairs.push([visibleItems[leftIndex].text, visibleItems[rightIndex].text]);
      }
    }

    const topValues = visibleItems.map((item) => item.rect.top);
    const bottomValues = visibleItems.map((item) => item.rect.bottom);
    const verticalSpread = Math.max(...bottomValues) - Math.min(...topValues);

    const more = toolbar.querySelector('.more-btn');
    const moreRect = more?.getClientRects().length ? more.getBoundingClientRect() : null;
    return {
      moreLeft: moreRect ? moreRect.left : 0,
      moreVisible: !!moreRect,
      height: toolbarRect.height,
      latestCount: (toolbar.innerText.match(/\bLatest\b/g) || []).length,
      overflowX: toolbar.scrollWidth > Math.ceil(toolbar.clientWidth) + 1,
      overflowY: toolbar.scrollHeight > Math.ceil(toolbar.clientHeight) + 1,
      verticalSpread,
      overlapPairs,
      text: toolbar.innerText.trim().replace(/\s+/g, ' '),
      itemTexts: visibleItems.map((item) => item.text),
      toolbarLeft: toolbarRect.left,
      firstActionLeft: visibleItems.find((item) => ['Compare', 'Sources'].includes(item.text) || item.text.startsWith('Publish'))?.rect.left || 0,
      reviseLeft: visibleItems.find((item) => item.text === 'Revise')?.rect.left || 0,
      rightEdge: toolbarRect.right,
      publishRight: visibleItems.find((item) => item.text.startsWith('Publish'))?.rect.right || 0,
    };
  });
}

test('Texture toolbar stays one row with invariant height across window widths', async ({ page }) => {
  // The logged-out home needs no service behind it.
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/front*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"stories":[]}' }));
  await page.goto(BASE_URL);
  // The logged-out home opens Autopaper; Texture opens from its desktop icon.
  await page.locator('[data-desktop-icon-id="texture"]').click();
  await expect(page.locator('[data-texture-toolbar]')).toBeVisible();

  const widths = [1600, 1200, 900, 640, 560, 460, 390, 340, 280, 240];
  let expectedHeight = null;

  for (const width of widths) {
    await setTextureWindowWidth(page, width);
    await page.waitForTimeout(150);
    const metrics = await toolbarMetrics(page);

    expectedHeight ??= metrics.height;
    expect(metrics.height).toBeCloseTo(expectedHeight, 1);
    expect(metrics.verticalSpread, `${width}px toolbar should remain visually one row`).toBeLessThanOrEqual(36);
    expect(metrics.latestCount, `${width}px toolbar should not repeat Latest`).toBeLessThanOrEqual(1);
    expect(metrics.overflowX, `${width}px toolbar should not overflow horizontally`).toBe(false);
    expect(metrics.overflowY, `${width}px toolbar should not overflow vertically`).toBe(false);
    expect(metrics.overlapPairs, `${width}px toolbar controls should not overlap`).toEqual([]);
    expect(metrics.text, `${width}px toolbar should not use illegible action initials`).not.toMatch(/\b[RSP]\b/);
    expect(metrics.firstActionLeft - metrics.toolbarLeft, `${width}px non-Revise actions should stay left-aligned`).toBeLessThan(520);
    // Revise is the document's main action: it is the last control to go into
    // the "more" menu, so it is in the row at every width.
    expect(metrics.reviseLeft, `${width}px Revise should stay in the row`).toBeGreaterThan(0);
    expect(metrics.rightEdge, `${width}px nothing past the toolbar`).toBeGreaterThanOrEqual(metrics.reviseLeft);
    if (metrics.moreVisible) {
      expect(metrics.reviseLeft, `${width}px Revise should sit left of the more button`).toBeLessThan(metrics.moreLeft);
    } else {
      // Nothing hidden: every action is in the row, Revise right of Publish.
      expect(metrics.itemTexts.some((text) => text.startsWith('Publish')), `${width}px Publish in the row`).toBe(true);
      expect(metrics.reviseLeft, `${width}px Revise should sit to the right of Publish`).toBeGreaterThan(metrics.publishRight);
      expect(metrics.rightEdge - metrics.reviseLeft, `${width}px Revise should occupy the right action slot`).toBeLessThanOrEqual(110);
    }
  }
});

test('a control moved into the more menu keeps working: Publish opens its confirmation inside the menu', async ({ page }) => {
  await page.route('**/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"authenticated":false}' }));
  await page.route('**/api/platform/publications/front*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{"stories":[]}' }));
  await page.goto(BASE_URL);
  await page.locator('[data-desktop-icon-id="texture"]').click();
  await expect(page.locator('[data-texture-toolbar]')).toBeVisible();

  // Wide: no menu button. Narrow: Publish and Sources move into the menu.
  await setTextureWindowWidth(page, 900);
  await expect(page.locator('[data-texture-toolbar-more]')).toBeHidden();
  await setTextureWindowWidth(page, 220);
  await expect(page.locator('[data-texture-toolbar-more]')).toBeVisible();
  await expect(page.locator('[data-texture-toolbar] [data-texture-prompt]')).toBeVisible();
  await expect(page.locator('[data-texture-toolbar] [data-texture-publish]')).toHaveCount(1);
  await expect(page.locator('[data-texture-toolbar] [data-texture-publish]')).toBeHidden();

  await page.locator('[data-texture-toolbar-more]').click();
  const publish = page.locator('[data-texture-toolbar-menu] [data-texture-publish]');
  await expect(publish).toBeVisible();
  await expect(publish).toHaveAttribute('aria-haspopup', 'menu');
  await expect(publish).toHaveAttribute('aria-expanded', 'false');
  await publish.click();
  // The menu stays open and holds the confirmation, which sits in the menu's flow.
  await expect(publish).toHaveAttribute('aria-expanded', 'true');
  const confirmation = page.locator('[data-texture-toolbar-menu] [data-texture-publish-menu]');
  await expect(confirmation).toBeVisible();
  const [menuBox, confirmBox] = await Promise.all([
    page.locator('[data-texture-toolbar-menu]').boundingBox(),
    confirmation.boundingBox(),
  ]);
  expect(confirmBox.x).toBeGreaterThanOrEqual(menuBox.x - 1);
  expect(confirmBox.x + confirmBox.width).toBeLessThanOrEqual(menuBox.x + menuBox.width + 1);
  expect(confirmBox.y + confirmBox.height).toBeLessThanOrEqual(menuBox.y + menuBox.height + 1);

  // Closing the menu closes the confirmation, so it is not left open out of sight.
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-texture-toolbar-menu]')).toBeHidden();
  await expect(page.locator('[data-texture-toolbar-more]')).toBeFocused();
  await setTextureWindowWidth(page, 900);
  await expect(page.locator('[data-texture-toolbar-more]')).toBeHidden();
  await expect(page.locator('[data-texture-toolbar] [data-texture-publish]')).toBeVisible();
  await expect(page.locator('[data-texture-toolbar] [data-texture-publish]')).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('[data-texture-publish-menu]')).toHaveCount(0);
});
