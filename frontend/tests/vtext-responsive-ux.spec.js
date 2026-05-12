import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail(label) {
  return `vtext-ux-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
}

async function openBlankVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  await page.locator('[data-vtext-editor]').waitFor({ state: 'visible', timeout: 15000 });
  const recent = page.locator('[data-vtext-recent]');
  await expect(recent).toBeVisible({ timeout: 15000 });
  await page.locator('[data-vtext-new-document]').click();
  const editor = page.locator('[data-vtext-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 15000 });
  return editor;
}

async function getVTextLayout(page) {
  return page.evaluate(() => {
    const rect = (selector) => {
      const el = document.querySelector(selector);
      if (!el) return null;
      const box = el.getBoundingClientRect();
      return {
        x: box.x,
        y: box.y,
        width: box.width,
        height: box.height,
        right: box.right,
        bottom: box.bottom,
      };
    };
    const toolbar = document.querySelector('[data-vtext-toolbar]');
    const controls = [...document.querySelectorAll('[data-vtext-toolbar] [data-vtext-version], [data-vtext-toolbar] button, [data-vtext-toolbar] [data-vtext-state]')]
      .filter((el) => el.offsetParent !== null)
      .map((el) => {
        const box = el.getBoundingClientRect();
        return { top: box.top, bottom: box.bottom };
      });
    const controlBand = controls.length
      ? Math.max(...controls.map((item) => item.bottom)) - Math.min(...controls.map((item) => item.top))
      : 0;
    const meta = document.querySelector('meta[name="viewport"]')?.getAttribute('content') || '';
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      viewportMeta: meta,
      window: rect('[data-window]'),
      titlebar: rect('[data-window-titlebar]'),
      toolbar: rect('[data-vtext-toolbar]'),
      editor: rect('[data-vtext-editor-area]'),
      bottomBar: rect('[data-bottom-bar]'),
      toolbarOpacity: toolbar ? Number(getComputedStyle(toolbar).opacity) : null,
      controlBand,
    };
  });
}

async function attachScreenshot(page, testInfo, name) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: false });
  await testInfo.attach(name, { path, contentType: 'image/png' });
}

test('mobile VText is full-screen-like, editable rendered Markdown, and quiet account UI', async ({ page, authenticator }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, uniqueEmail('mobile'));
  let editor = await openBlankVText(page);

  let layout = await getVTextLayout(page);
  expect(layout.viewportMeta).toContain('maximum-scale=1');
  expect(layout.viewportMeta).toContain('interactive-widget=resizes-content');
  expect(layout.window.width).toBeGreaterThanOrEqual(360);
  expect(layout.window.height).toBeGreaterThanOrEqual(700);
  expect(layout.window.x).toBeLessThanOrEqual(16);
  expect(layout.window.right).toBeGreaterThanOrEqual(374);
  expect(layout.toolbar.height).toBeLessThanOrEqual(52);
  expect(layout.controlBand).toBeLessThanOrEqual(44);
  expect(layout.editor.y).toBeGreaterThanOrEqual(layout.toolbar.bottom - 1);

  await expect(editor).toHaveAttribute('contenteditable', 'true');
  await expect(page.getByRole('button', { name: 'Read' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Edit' })).toHaveCount(0);

  await editor.fill('# Rendered Markdown Proof\n\nThis is **bold** and `code`.\n\n- first\n- second');
  await page.locator('[data-vtext-toolbar]').click();
  await expect(editor.locator('h1')).toContainText('Rendered Markdown Proof');
  await expect(editor.locator('strong')).toContainText('bold');
  await expect(editor.locator('code')).toContainText('code');

  await editor.fill([
    '# Scroll Proof',
    '',
    ...Array.from({ length: 30 }, (_, i) => `Paragraph ${i + 1}: markdown remains editable while the compact toolbar fades during reading.`),
  ].join('\n\n'));
  await page.locator('[data-vtext-toolbar]').click();
  await editor.evaluate((node) => {
    node.scrollTop = 180;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await page.mouse.move(180, 520);
  await expect(page.locator('[data-vtext-toolbar]')).toHaveCSS('opacity', /0\.1[0-9]|0\.2[0-5]/);

  const beforeReload = await getVTextLayout(page);
  await page.waitForTimeout(800);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
  await expect(page.locator('[data-vtext-recent]')).toBeVisible({ timeout: 15000 });
  layout = await getVTextLayout(page);
  expect(Math.abs(layout.window.width - beforeReload.window.width)).toBeLessThanOrEqual(2);
  expect(Math.abs(layout.window.height - beforeReload.window.height)).toBeLessThanOrEqual(2);

  const prompt = page.locator('[data-prompt-input]');
  const promptBefore = await prompt.boundingBox();
  await prompt.fill('line one\nline two\nline three\nline four');
  const promptAfter = await prompt.boundingBox();
  expect(promptAfter.height).toBeGreaterThan(promptBefore.height);

  await expect(page.locator('[data-bottom-logout]')).toHaveCount(0);
  await page.locator('[data-show-desktop-btn]').click();
  await expect(page.locator('[data-bottom-logout]')).toBeVisible();

  await attachScreenshot(page, testInfo, 'mobile-vtext');
});

test('VText default geometry is large on tablet and desktop', async ({ page, authenticator }, testInfo) => {
  const cases = [
    { name: 'tablet', viewport: { width: 820, height: 1180 }, minWidth: 680, minHeight: 520 },
    { name: 'desktop', viewport: { width: 1440, height: 900 }, minWidth: 900, minHeight: 650 },
  ];

  for (const item of cases) {
    await page.setViewportSize(item.viewport);
    await registerAndLoadDesktop(page, uniqueEmail(item.name));
    await openBlankVText(page);
    const layout = await getVTextLayout(page);
    expect(layout.window.width).toBeGreaterThanOrEqual(item.minWidth);
    expect(layout.window.height).toBeGreaterThanOrEqual(item.minHeight);
    expect(layout.editor.y).toBeGreaterThanOrEqual(layout.toolbar.bottom - 1);
    await attachScreenshot(page, testInfo, `${item.name}-vtext`);
    await page.context().clearCookies();
  }
});
