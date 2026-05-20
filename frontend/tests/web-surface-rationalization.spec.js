import { test, expect } from './helpers/fixtures.js';

async function openStartApp(page, appId) {
  await page.locator('[data-start-button]').click();
  await page.locator(`[data-start-app-id="${appId}"]`).click();
}

test('Apps & Changes replaces manual candidate desktop entry points', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const forbiddenRemoteDisplayRequests = [];
  page.on('request', (request) => {
    const url = request.url().toLowerCase();
    if (url.includes('vnc') || url.includes('webrtc') || url.includes('mjpeg') || url.includes('framebuffer')) {
      forbiddenRemoteDisplayRequests.push(url);
    }
  });

  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
  });

  await page.locator('[data-start-button]').click();
  await expect(page.locator('[data-start-app-id="candidate-desktop"]')).toHaveCount(0);
  await page.locator('[data-start-app-id="apps-changes"]').click();
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await expect(store.locator('[data-change-card]')).toHaveCount(4);
  await expect(store.locator('[data-change-evidence]')).toContainText('VText report pending');
  await expect(store.locator('[data-change-preview-empty]')).toBeVisible();
  await expect(store.locator('[data-candidate-desktop-input]')).toHaveCount(0);
  expect(forbiddenRemoteDisplayRequests).toEqual([]);
});

test('Web Lens API calls preserve candidate desktop selector', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const capabilityRequests = [];

  await page.route('**/api/browser/capabilities**', async (route) => {
    capabilityRequests.push(new URL(route.request().url()));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        provider: 'obscura',
        mode: 'legacy_iframe',
        substrate: 'frontend_iframe',
        available: false,
        configured: false,
        status: 'not_configured',
        supports: {
          navigate: false,
          text: false,
          html: false,
          links: false,
          screenshot: false,
          cdp_screenshot: false,
          bounded_input: false,
          input: false,
          cdp: false,
        },
        legacy_iframe_available: true,
      }),
    });
  });

  await page.evaluate(() => {
    window.history.pushState({}, '', '/?desktop_id=branch-preview');
  });
  await page.locator('[data-desktop-icon-id="browser"]').dblclick();

  const status = page.locator('[data-browser-backend-status]');
  await expect(status).toHaveAttribute('data-browser-backend-available', 'false', { timeout: 10_000 });
  expect(capabilityRequests).toHaveLength(1);
  expect(capabilityRequests[0].searchParams.get('desktop_id')).toBe('branch-preview');
});

test('Web Lens imports Obscura semantic snapshot into VText without iframe rendering', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const browserRequests = [];
  const sessionID = `web-lens-session-${Date.now()}`;

  await page.route('**/api/browser/**', async (route) => {
    const url = new URL(route.request().url());
    browserRequests.push(`${route.request().method()} ${url.pathname}${url.search}`);

    if (url.pathname === '/api/browser/capabilities') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'obscura',
          mode: 'backend',
          substrate: 'obscura_cli_fetch',
          available: true,
          configured: true,
          status: 'ready',
          supports: {
            navigate: true,
            text: true,
            html: true,
            links: true,
            screenshot: false,
            cdp_screenshot: false,
            bounded_input: false,
            input: false,
            cdp: false,
          },
          legacy_iframe_available: true,
        }),
      });
      return;
    }

    if (url.pathname === '/api/browser/sessions') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: sessionID,
          owner_id: 'test-owner',
          provider: 'obscura',
          mode: 'backend',
          execution_scope: 'host_process',
          world_kind: 'foreground',
          state: 'idle',
          current_url: '',
        }),
      });
      return;
    }

    if (url.pathname === `/api/browser/sessions/${sessionID}/navigate`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: sessionID,
          owner_id: 'test-owner',
          provider: 'obscura',
          mode: 'backend',
          execution_scope: 'host_process',
          world_kind: 'foreground',
          state: 'ready',
          current_url: 'https://example.com',
          title: 'Example Domain',
          text_snapshot: 'Example Domain\nThis domain is for use in illustrative examples.',
          html_snapshot: '<title>Example Domain</title>',
          links: [{ url: 'https://www.iana.org/domains/example', text: 'More information' }],
        }),
      });
      return;
    }

    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'unexpected browser route' }),
    });
  });

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const webLens = page.locator('[data-browser-app]').last();
  await expect(webLens).toBeVisible({ timeout: 10_000 });
  await expect(webLens.locator('[data-browser-backend-status]')).toHaveAttribute(
    'data-browser-backend-substrate',
    'obscura_cli_fetch',
    { timeout: 10_000 }
  );

  await webLens.locator('[data-browser-url-input]').fill('https://example.com');
  await webLens.locator('[data-browser-go-btn]').click();

  await expect(webLens.locator('[data-browser-backend-snapshot]')).toContainText('Example Domain', {
    timeout: 10_000,
  });
  await expect(webLens.locator('[data-browser-iframe]')).toHaveCount(0);
  await expect(webLens.locator('[data-browser-import-vtext]')).toBeVisible();

  await webLens.locator('[data-browser-import-vtext]').click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible({ timeout: 10_000 });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Web Lens import', {
    timeout: 20_000,
  });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Example Domain', {
    timeout: 20_000,
  });

  expect(browserRequests.some((entry) => entry.includes('/api/browser/sessions'))).toBe(true);
});
