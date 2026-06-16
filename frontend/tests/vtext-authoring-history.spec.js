import { test, expect } from './helpers/fixtures.js';

async function openVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const root = page.locator('.window.window-active [data-texture-editor]').last();
  await root.waitFor({ state: 'visible', timeout: 10000 });
  const recent = root.locator('[data-texture-recent]');
  if (await recent.isVisible().catch(() => false)) {
    await root.locator('[data-texture-new-document]').click();
  }
  const editor = root.locator('[data-texture-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 10000 });
  await expect(editor).toHaveAttribute('contenteditable', 'true', { timeout: 10000 });
  return root;
}

test('vtext uses the document surface as the window and exposes version navigation', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const root = await openVText(page);

  const editor = root.locator('[data-texture-editor-area]');
  const prev = root.locator('[data-texture-prev]');
  const next = root.locator('[data-texture-next]');
  const toolbar = root.locator('[data-texture-toolbar]');
  const revisionLine = root.locator('[data-texture-draft-line]');

  await expect(editor).toBeVisible();
  await expect(revisionLine).toContainText('Latest');
  await expect(prev).toBeDisabled();
  await expect(next).toBeDisabled();
  const latestToolbarHeight = await toolbar.evaluate((el) => Math.round(el.getBoundingClientRect().height));

  await editor.fill('Version zero content.\n\nExpand this into a better document.');
  await root.locator('[data-texture-prompt]').click();

  await expect(root.locator('[data-texture-save-status]')).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });
  await expect(prev).toBeEnabled();
  await expect(next).toBeDisabled();

  await prev.click();
  await expect(revisionLine).toContainText('Historical');
  const historicalToolbarHeight = await toolbar.evaluate((el) => Math.round(el.getBoundingClientRect().height));
  expect(historicalToolbarHeight).toBe(latestToolbarHeight);
});

test('vtext publish keeps policy behind the publish menu and forwards policy', async ({ desktopSession }) => {
  const { page, baseURL } = desktopSession;
  const root = await openVText(page);

  const editor = root.locator('[data-texture-editor-area]');
  await editor.fill('Publish policy fixture.\n\nThis revision should publish from an explicit menu confirmation.');

  const publishButton = root.locator('[data-texture-publish]');
  await expect(root.locator('[data-texture-publish-policy]')).toHaveCount(0);
  await expect(root.locator('[data-texture-publish-menu]')).toHaveCount(0);
  await expect(publishButton).toBeEnabled();

  let publishPayload = null;
  await page.route('**/api/platform/texture/publications', async (route) => {
    publishPayload = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        publication_id: 'pub-policy-fixture',
        publication_version_id: 'pubver-policy-fixture',
        route_path: '/pub/texture/policy-fixture',
        public_url: `${baseURL}/pub/texture/policy-fixture`,
      }),
    });
  });

  await publishButton.click();
  const publishMenu = root.locator('[data-texture-publish-menu]');
  await expect(publishMenu).toBeVisible();
  await expect(publishMenu).toContainText('Publish v0');
  await expect(publishMenu).toContainText('This creates a public link with the current text and source snapshots.');
  await expect(publishMenu).not.toContainText('Route');
  await expect(publishMenu).not.toContainText('txt, md, html, docx, pdf');

  await root.locator('[data-texture-publish-confirm]').click();

  await expect(root.locator('[data-texture-publish-result]')).toBeVisible({ timeout: 10000 });
  await expect(root.locator('[data-texture-publish-menu]')).toHaveCount(0);
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
