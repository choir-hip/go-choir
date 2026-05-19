import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';
import { buildEpubBytes, buildPdfBytes } from './helpers/media-fixtures.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(180_000);
test.skip(
  process.env.GO_CHOIR_RUN_CONTENT_APPS !== '1',
  'set GO_CHOIR_RUN_CONTENT_APPS=1 to verify content app routing'
);

function uniqueEmail() {
  return `content-apps-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, path);
}

async function waitForPromptDecision(page, submissionId, timeout = 90_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(500);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

async function submitBareReference(page, sourceUrl) {
  const promptBarResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(sourceUrl);
  await page.locator('[data-prompt-input]').press('Enter');
  const response = await promptBarResponse;
  expect(response.status()).toBe(202);
  const body = await response.json();
  return waitForPromptDecision(page, body.submission_id);
}

async function routeReaderFixtures(page) {
  const pdfBytes = buildPdfBytes('Choir section seven PDF search proof');
  const epubBytes = await buildEpubBytes({
    title: 'Choir Section Seven EPUB',
    chapters: [
      {
        title: 'Section Seven',
        body: [
          'This EPUB fixture proves that prompt-routed EPUB sources can become real reader content.',
          'The reader should expose chapter navigation and searchable text.',
        ],
      },
    ],
  });
  await page.route('https://example.com/choir-section7.pdf', (route) => route.fulfill({
    status: 200,
    contentType: 'application/pdf',
    headers: { 'Access-Control-Allow-Origin': '*' },
    body: Buffer.from(pdfBytes),
  }));
  await page.route('https://example.com/choir-section7.epub', (route) => route.fulfill({
    status: 200,
    contentType: 'application/epub+zip',
    headers: { 'Access-Control-Allow-Origin': '*' },
    body: Buffer.from(epubBytes),
  }));
  await page.route('https://example.com/choir-mobile-doc.pdf', (route) => route.fulfill({
    status: 200,
    contentType: 'application/pdf',
    headers: { 'Access-Control-Allow-Origin': '*' },
    body: Buffer.from(pdfBytes),
  }));
}

test('bare content references open the dedicated content apps from the prompt bar', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  const forbiddenRequests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (
      url.pathname.startsWith('/api/agent/') ||
      url.pathname.startsWith('/api/prompts') ||
      url.pathname.startsWith('/api/test/') ||
      url.pathname.startsWith('/internal') ||
      url.pathname === '/api/events'
    ) {
      forbiddenRequests.push(`${request.method()} ${url.pathname}`);
    }
  });

  await routeReaderFixtures(page);
  await registerAndLoadDesktop(page, uniqueEmail());

  const references = [
    { app: 'pdf', mediaType: 'application/pdf', url: 'https://example.com/choir-section7.pdf' },
    { app: 'epub', mediaType: 'application/epub+zip', url: 'https://example.com/choir-section7.epub' },
    { app: 'image', mediaType: 'image/png', url: 'https://example.com/choir-section7.png' },
    { app: 'audio', mediaType: 'audio/mpeg', url: 'https://example.com/choir-section7.mp3' },
    { app: 'video', mediaType: 'video/youtube', url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ' },
    { app: 'podcast', mediaType: 'application/rss+xml', url: 'https://podcasts.files.bbci.co.uk/p02nq0gn.rss' },
  ];

  for (const reference of references) {
    const decision = await submitBareReference(page, reference.url);
    expect(decision.action).toBe('open_app');
    expect(decision.app).toBe(reference.app);
    expect(decision.media_type).toBe(reference.mediaType);
    expect(decision.source_url).toBe(reference.url);

    if (reference.app === 'podcast') {
      const viewer = page.locator('[data-podcast-app]').last();
      await expect(viewer).toBeVisible({ timeout: 30_000 });
      await expect(viewer.locator('[data-podcast-episode]').first()).toBeVisible({ timeout: 45_000 });
      await expect(viewer.locator('[data-podcast-controls]').first()).toBeVisible();
      await expect(viewer.locator('[data-podcast-seek]').first()).toBeVisible();
      await expect(viewer.locator('[data-podcast-audio]').first()).toHaveAttribute('src', /.+/);
    } else {
      const viewer = page.locator(`[data-media-app][data-media-kind="${reference.app}"]`).last();
      await expect(viewer).toBeVisible({ timeout: 30_000 });
      await expect(viewer.locator('[data-media-title]')).toBeVisible();

      if (reference.app === 'image') {
        await expect(viewer.locator('[data-image-toolbar]')).toBeVisible();
        await expect(viewer.locator('[data-image-fit]')).toBeVisible();
        await expect(viewer.locator('[data-image-zoom-in]')).toBeVisible();
        await expect(viewer.locator('[data-image-rotate-right]')).toBeVisible();
        await viewer.locator('[data-image-rotate-right]').click();
        await expect(viewer.locator('[data-image-rotation]')).toContainText('90deg');
        await expect(viewer.locator('[data-image-viewer]')).toHaveAttribute('src', reference.url);
      } else if (reference.app === 'audio') {
        await expect(viewer.locator('[data-media-player]')).toBeVisible();
        await expect(viewer.locator('[data-media-play]')).toBeVisible();
        await expect(viewer.locator('[data-media-seek]')).toBeVisible();
        await expect(viewer.locator('[data-media-speed]')).toBeVisible();
        await expect(viewer.locator('[data-media-position-status]')).toContainText('saved');
        await expect(viewer.locator('[data-audio-element]')).toHaveAttribute('src', reference.url);
      } else if (reference.app === 'video') {
        await expect(viewer.locator('[data-video-toolbar]')).toBeVisible();
        await expect(viewer.locator('[data-video-embedded-controls]')).toBeVisible();
        await expect(viewer.locator('[data-video-frame]')).toHaveAttribute('src', /youtube\.com\/embed/);
      } else if (reference.app === 'pdf') {
        await expect(viewer.locator('[data-pdf-toolbar]')).toBeVisible();
        await expect(viewer.locator('[data-pdf-page]')).toBeVisible();
        await expect(viewer.locator('[data-pdf-zoom]')).toBeVisible();
        await expect(viewer.locator('[data-pdf-reader]')).toHaveAttribute('data-pdf-rendered', 'true', { timeout: 15_000 });
        await viewer.locator('[data-pdf-search]').fill('section seven');
        await expect(viewer.locator('[data-pdf-search-count]')).toContainText('1 matches', { timeout: 10_000 });
      } else if (reference.app === 'epub') {
        await expect(viewer.locator('[data-epub-reader]')).toBeVisible({ timeout: 15_000 });
        await expect(viewer.locator('[data-epub-chapter-title]')).toContainText('Section Seven');
        await viewer.locator('[data-epub-search]').fill('real reader');
        await expect(viewer.locator('[data-epub-search-count]')).toContainText('1 matches');
      }
    }
  }

  expect(forbiddenRequests).toHaveLength(0);
});

test('media controls stay reachable in the mobile desktop window geometry', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await page.setViewportSize({ width: 390, height: 844 });
  await routeReaderFixtures(page);
  await registerAndLoadDesktop(page, uniqueEmail());

  const references = [
    {
      app: 'image',
      mediaType: 'image/png',
      url: 'https://example.com/choir-mobile-image.png',
      required: ['[data-image-toolbar]', '[data-image-fit]', '[data-image-zoom-in]', '[data-image-rotate-right]', '[data-image-reset]'],
    },
    {
      app: 'audio',
      mediaType: 'audio/mpeg',
      url: 'https://example.com/choir-mobile-audio.mp3',
      required: ['[data-media-player]', '[data-media-play]', '[data-media-seek]', '[data-media-speed]', '[data-media-position-status]'],
    },
    {
      app: 'pdf',
      mediaType: 'application/pdf',
      url: 'https://example.com/choir-mobile-doc.pdf',
      required: ['[data-pdf-toolbar]', '[data-pdf-page]', '[data-pdf-zoom]', '[data-pdf-reader][data-pdf-rendered="true"]'],
    },
  ];

  for (const reference of references) {
    const decision = await submitBareReference(page, reference.url);
    expect(decision.action).toBe('open_app');
    expect(decision.app).toBe(reference.app);
    expect(decision.media_type).toBe(reference.mediaType);

    const viewer = page.locator(`[data-media-app][data-media-kind="${reference.app}"]`).last();
    await expect(viewer).toBeVisible({ timeout: 30_000 });
    for (const selector of reference.required) {
      await expect(viewer.locator(selector)).toBeVisible();
    }

    const windowEl = page.locator('[data-window]').filter({ has: viewer }).last();
    const [windowBox, stageBox] = await Promise.all([
      windowEl.boundingBox(),
      viewer.locator('[data-media-stage]').boundingBox(),
    ]);
    expect(windowBox.width).toBeGreaterThanOrEqual(260);
    expect(windowBox.height).toBeGreaterThanOrEqual(320);
    expect(windowBox.x).toBeGreaterThanOrEqual(0);
    expect(windowBox.y).toBeGreaterThanOrEqual(0);
    expect(windowBox.x + windowBox.width).toBeLessThanOrEqual(390 + 2);
    expect(stageBox.width).toBeGreaterThanOrEqual(220);
    expect(stageBox.height).toBeGreaterThanOrEqual(160);
  }

  const overflowX = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflowX).toBeLessThanOrEqual(2);
});
