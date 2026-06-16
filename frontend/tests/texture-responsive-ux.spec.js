import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail(label) {
  return `texture-ux-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
}

async function openBlankTexture(page) {
  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  await page.locator('[data-texture-editor]').waitFor({ state: 'visible', timeout: 15000 });
  const recent = page.locator('[data-texture-recent]');
  await expect(recent).toBeVisible({ timeout: 15000 });
  await page.locator('[data-texture-new-document]').click();
  const editor = page.locator('[data-texture-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 15000 });
  return editor;
}

async function getTextureLayout(page) {
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
    const toolbar = document.querySelector('[data-texture-toolbar]');
    const controls = [...document.querySelectorAll('[data-texture-toolbar] [data-texture-version], [data-texture-toolbar] button, [data-texture-toolbar] [data-texture-state]')]
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
      toolbar: rect('[data-texture-toolbar]'),
      editor: rect('[data-texture-editor-area]'),
      promptSurface: rect('[data-prompt-surface]'),
      toolbarOpacity: toolbar ? Number(getComputedStyle(toolbar).opacity) : null,
      toolbarHidden: toolbar ? toolbar.classList.contains('toolbar-hidden') : false,
      controlBand,
    };
  });
}

async function attachScreenshot(page, testInfo, name) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: false });
  await testInfo.attach(name, { path, contentType: 'image/png' });
}

test('mobile Texture is full-screen-like, editable rendered Markdown, and quiet account UI', async ({ page, authenticator }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, uniqueEmail('mobile'));
  let editor = await openBlankTexture(page);

  let layout = await getTextureLayout(page);
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
  await page.locator('[data-texture-toolbar]').click();
  await expect(editor.locator('h1')).toContainText('Rendered Markdown Proof');
  await expect(editor.locator('strong')).toContainText('bold');
  await expect(editor.locator('code')).toContainText('code');
  await expect(page.locator('[data-texture-save-status]')).toContainText('Saved', { timeout: 7000 });

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
  editor = page.locator('[data-texture-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 15000 });
  await expect(editor).toContainText('Rendered Markdown Proof', { timeout: 15000 });

  await editor.fill([
    '# Scroll Proof',
    '',
    ...Array.from({ length: 30 }, (_, i) => `Paragraph ${i + 1}: markdown remains editable while the compact toolbar fades during reading.`),
  ].join('\n\n'));
  await page.locator('[data-texture-toolbar]').click();
  await editor.evaluate((node) => {
    node.scrollTop = 180;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await page.mouse.move(180, 520);
  await expect(page.locator('[data-texture-toolbar]')).toHaveClass(/toolbar-hidden/);
  await editor.evaluate((node) => {
    node.scrollTop = 168;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(page.locator('[data-texture-toolbar]')).toHaveClass(/toolbar-hidden/);
  await page.waitForTimeout(220);
  const hiddenLayout = await getTextureLayout(page);
  expect(hiddenLayout.toolbar.height).toBeLessThan(8);
  await page.waitForTimeout(120);
  await editor.evaluate((node) => {
    node.scrollTop = 60;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });
  await expect(page.locator('[data-texture-toolbar]')).not.toHaveClass(/toolbar-hidden/);
  await expect(page.locator('[data-texture-save-status]')).toContainText('Saved', { timeout: 7000 });

  const beforeReload = await getTextureLayout(page);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
  await page.locator('[data-texture-editor-area]').waitFor({ state: 'visible', timeout: 15000 });
  layout = await getTextureLayout(page);
  expect(Math.abs(layout.window.width - beforeReload.window.width)).toBeLessThanOrEqual(2);
  expect(Math.abs(layout.window.height - beforeReload.window.height)).toBeLessThanOrEqual(2);

  const prompt = page.locator('[data-prompt-input]');
  const promptBefore = await prompt.boundingBox();
  await prompt.fill('line one\nline two\nline three\nline four');
  const promptAfter = await prompt.boundingBox();
  expect(promptAfter.height).toBeGreaterThan(promptBefore.height);

  await expect(page.locator('[data-prompt-surface-logout]')).toHaveCount(0);
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-prompt-surface-logout]')).toBeVisible();

  await attachScreenshot(page, testInfo, 'mobile-texture');
});

test('Texture default geometry is large on tablet and desktop', async ({ page, authenticator }, testInfo) => {
  const cases = [
    { name: 'tablet', viewport: { width: 820, height: 1180 }, minWidth: 680, minHeight: 520 },
    { name: 'desktop', viewport: { width: 1440, height: 900 }, minWidth: 900, minHeight: 650 },
  ];

  for (const item of cases) {
    await page.setViewportSize(item.viewport);
    await registerAndLoadDesktop(page, uniqueEmail(item.name));
    await openBlankTexture(page);
    const layout = await getTextureLayout(page);
    expect(layout.window.width).toBeGreaterThanOrEqual(item.minWidth);
    expect(layout.window.height).toBeGreaterThanOrEqual(item.minHeight);
    expect(layout.editor.y).toBeGreaterThanOrEqual(layout.toolbar.bottom - 1);
    await attachScreenshot(page, testInfo, `${item.name}-texture`);
    await page.context().clearCookies();
  }
});
