import { test, expect } from './helpers/fixtures.js';

async function openVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  await page.locator('[data-texture-editor]').waitFor({ state: 'visible', timeout: 10000 });
  const recent = page.locator('[data-texture-recent]');
  if (await recent.isVisible().catch(() => false)) {
    await page.locator('[data-texture-new-document]').click();
  }
  const editor = page.locator('[data-texture-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 10000 });
  await expect(editor).toHaveAttribute('contenteditable', 'true', { timeout: 10000 });
}

test('prompt button submits a vtext agent revision request', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await openVText(page);

  const editor = page.locator('[data-texture-editor-area]');
  await editor.fill('Draft version with a note to expand the plan.');

  const revisionRequest = page.waitForResponse((response) => {
    return response.request().method() === 'POST' &&
      /\/api\/vtext\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname);
  });

  await page.locator('[data-texture-prompt]').click();

  const response = await revisionRequest;
  expect(response.status()).toBe(202);
  await expect(page.locator('[data-texture-save-status]')).toContainText(/Writing first draft|First draft ready|Agent created next version/);
});
