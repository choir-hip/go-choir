import { test, expect } from './helpers/fixtures.js';

async function openVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  await page.locator('[data-vtext-editor]').waitFor({ state: 'visible', timeout: 10000 });
  const recent = page.locator('[data-vtext-recent]');
  if (await recent.isVisible().catch(() => false)) {
    await page.locator('[data-vtext-new-document]').click();
  }
  const editor = page.locator('[data-vtext-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 10000 });
  await expect(editor).toHaveAttribute('contenteditable', 'true', { timeout: 10000 });
}

test('vtext uses the document surface as the window and exposes version navigation', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await openVText(page);

  const editor = page.locator('[data-vtext-editor-area]');
  const prev = page.locator('[data-vtext-prev]');
  const next = page.locator('[data-vtext-next]');
  const toolbar = page.locator('[data-vtext-toolbar]');
  const revisionLine = page.locator('[data-vtext-draft-line]');

  await expect(editor).toBeVisible();
  await expect(revisionLine).toContainText('Latest');
  await expect(prev).toBeDisabled();
  await expect(next).toBeDisabled();
  const latestToolbarHeight = await toolbar.evaluate((el) => Math.round(el.getBoundingClientRect().height));

  await editor.fill('Version zero content.\n\nExpand this into a better document.');
  await page.locator('[data-vtext-prompt]').click();

  await expect(page.locator('[data-vtext-save-status]')).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });
  await expect(prev).toBeEnabled();
  await expect(next).toBeDisabled();

  await prev.click();
  await expect(revisionLine).toContainText('Historical');
  const historicalToolbarHeight = await toolbar.evaluate((el) => Math.round(el.getBoundingClientRect().height));
  expect(historicalToolbarHeight).toBe(latestToolbarHeight);
});

test('vtext publish keeps policy behind the publish menu and forwards policy', async ({ desktopSession }) => {
  const { page, baseURL } = desktopSession;
  await openVText(page);

  const editor = page.locator('[data-vtext-editor-area]');
  await editor.fill('Publish policy fixture.\n\nThis revision should publish from an explicit menu confirmation.');

  const publishButton = page.locator('[data-vtext-publish]');
  await expect(page.locator('[data-vtext-publish-policy]')).toHaveCount(0);
  await expect(page.locator('[data-vtext-publish-menu]')).toHaveCount(0);
  await expect(publishButton).toBeEnabled();

  let publishPayload = null;
  await page.route('**/api/platform/vtext/publications', async (route) => {
    publishPayload = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        publication_id: 'pub-policy-fixture',
        publication_version_id: 'pubver-policy-fixture',
        route_path: '/pub/vtext/policy-fixture',
        public_url: `${baseURL}/pub/vtext/policy-fixture`,
      }),
    });
  });

  await publishButton.click();
  const publishMenu = page.locator('[data-vtext-publish-menu]');
  await expect(publishMenu).toBeVisible();
  await expect(publishMenu).toContainText('Publish v0');
  await expect(publishMenu).toContainText('This creates a public link with the current text and source snapshots.');
  await expect(publishMenu).not.toContainText('Route');
  await expect(publishMenu).not.toContainText('txt, md, html, docx, pdf');

  await page.locator('[data-vtext-publish-confirm]').click();

  await expect(page.locator('[data-vtext-publish-result]')).toBeVisible({ timeout: 10000 });
  await expect(page.locator('[data-vtext-publish-menu]')).toHaveCount(0);
  expect(publishPayload).toMatchObject({
    access_policy: {
      visibility: 'public',
      route: 'public',
    },
    export_policy: {
      copy_allowed: true,
      download_allowed: true,
      formats: ['txt', 'md', 'html', 'docx', 'pdf'],
    },
  });
});
