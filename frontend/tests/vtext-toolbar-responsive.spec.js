import { test, expect } from './helpers/fixtures.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

async function setVTextWindowWidth(page, width) {
  await page.evaluate((targetWidth) => {
    const windowNode = document.querySelector('[data-window-app-id="vtext"]');
    if (!windowNode) throw new Error('VText window not found');
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
    const toolbar = document.querySelector('[data-vtext-toolbar]');
    if (!toolbar) throw new Error('VText toolbar not found');

    const visibleItems = [
      ...toolbar.querySelectorAll('.version-controls > *, .doc-state, .doc-actions > button, .doc-actions > details, .doc-actions > .publish-menu-wrap'),
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

    return {
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

test('VText toolbar stays one row with invariant height across window widths', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-vtext-toolbar]')).toBeVisible();

  const widths = [1600, 1200, 900, 640, 560, 460, 390, 340];
  let expectedHeight = null;

  for (const width of widths) {
    await setVTextWindowWidth(page, width);
    await page.waitForTimeout(100);
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
    expect(metrics.reviseLeft, `${width}px Revise should sit to the right of Publish`).toBeGreaterThan(metrics.publishRight);
    expect(metrics.rightEdge - metrics.reviseLeft, `${width}px Revise should occupy the right action slot`).toBeLessThanOrEqual(110);
  }
});
