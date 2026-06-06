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

test('vtext publish requires explicit public policy acknowledgement and forwards policy', async ({ desktopSession }) => {
  const { page, baseURL } = desktopSession;
  await openVText(page);

  const editor = page.locator('[data-vtext-editor-area]');
  await editor.fill('Publish policy fixture.\n\nThis revision should publish only after explicit owner approval.');

  const policyPanel = page.locator('[data-vtext-publish-policy]');
  const publishButton = page.locator('[data-vtext-publish]');
  await expect(policyPanel).toBeVisible();
  await expect(policyPanel.locator('[data-vtext-publish-policy-summary]')).toContainText('Public route');
  await expect(publishButton).toBeDisabled();

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

  await page.locator('[data-vtext-publish-public-confirm]').check();
  await expect(publishButton).toBeEnabled();
  await publishButton.click();

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
