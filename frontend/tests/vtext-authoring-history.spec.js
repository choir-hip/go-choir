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

  await expect(editor).toBeVisible();
  await expect(prev).toBeDisabled();
  await expect(next).toBeDisabled();

  await editor.fill('Version zero content.\n\nExpand this into a better document.');
  await page.locator('[data-vtext-prompt]').click();

  await expect(page.locator('[data-vtext-save-status]')).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });
  await expect(prev).toBeEnabled();
  await expect(next).toBeDisabled();
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
  await expect(publishMenu.locator('[data-vtext-publish-policy-summary]')).toContainText('Route');
  await expect(publishMenu.locator('[data-vtext-publish-policy-summary]')).toContainText('Public');
  await expect(publishMenu.locator('[data-vtext-publish-policy-summary]')).toContainText('Snapshots included');
  await expect(publishMenu.locator('[data-vtext-publish-policy-summary]')).toContainText('txt, md, html, docx, pdf');

  await page.locator('[data-vtext-publish-confirm]').click();

  await expect(page.locator('[data-vtext-publish-result]')).toBeVisible({ timeout: 10000 });
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
