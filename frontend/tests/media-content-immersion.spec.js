import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';
import { buildEpubBytes, buildPdfBytes } from './helpers/media-fixtures.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

const OUTPUT_DIR = path.join(process.cwd(), 'test-results', `media-content-immersion-${Date.now()}`);

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(300_000);
test.skip(
  process.env.GO_CHOIR_RUN_MEDIA_IMMERSION !== '1',
  'set GO_CHOIR_RUN_MEDIA_IMMERSION=1 to verify media content immersion'
);

function uniqueEmail(label) {
  return `media-immersion-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function episodeXml(index, overrides = {}) {
  const title = overrides.title || `Episode ${String(index).padStart(2, '0')}`;
  const audio = overrides.audio || `https://example.com/audio/immersion-${index}.mp3`;
  return `
    <item>
      <title>${title}</title>
      <guid>media-immersion-${index}</guid>
      <pubDate>Wed, 13 May 2026 ${String((8 + index) % 24).padStart(2, '0')}:00:00 GMT</pubDate>
      <itunes:duration>18:30</itunes:duration>
      <description>Episode ${index} proves the podcast app remains a real desktop app while content stays primary.</description>
      <enclosure url="${audio}" type="audio/mpeg" length="${12345 + index}" />
    </item>`;
}

function buildPodcastRss(title) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>${title}</title>
    <link>https://example.com/media-immersion</link>
    <description>Media immersion acceptance proof feed.</description>
    ${Array.from({ length: 8 }, (_, index) => episodeXml(index + 1)).join('\n')}
  </channel>
</rss>`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

async function fetchJSON(page, requestPath) {
  return page.evaluate(async (path) => {
    const res = await fetch(path, { credentials: 'include' });
    const body = await res.text();
    if (!res.ok) throw new Error(`${path} failed: ${res.status} ${body}`);
    return body ? JSON.parse(body) : null;
  }, requestPath);
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

async function routeMediaFixtures(page) {
  const pdfBytes = buildPdfBytes('Choir media immersion PDF proof');
  const epubBytes = await buildEpubBytes({
    title: 'Choir Media Immersion EPUB',
    chapters: [
      {
        title: 'Full Window Reading',
        body: [
          'Reader content should own the window by default.',
          'Controls and metadata live in explicit overlays instead of stealing vertical space.',
        ],
      },
    ],
  });
  const png = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lAbfWQAAAABJRU5ErkJggg==',
    'base64'
  );
  const routes = [
    ['https://example.com/choir-immersion.pdf', 'application/pdf', Buffer.from(pdfBytes)],
    ['https://example.com/choir-immersion.epub', 'application/epub+zip', Buffer.from(epubBytes)],
    ['https://example.com/choir-immersion.png', 'image/png', png],
  ];
  for (const [url, contentType, body] of routes) {
    await page.route(url, (route) => route.fulfill({
      status: 200,
      contentType,
      headers: { 'Access-Control-Allow-Origin': '*' },
      body,
    }));
  }
}

async function seedPodcastFeed(page, title) {
  return page.evaluate(async ({ rss, title }) => {
    const res = await fetch('/api/content/items', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source_type: 'url',
        media_type: 'application/rss+xml',
        app_hint: 'podcast',
        title,
        source_url: 'https://example.com/media-immersion.rss',
        canonical_url: 'https://example.com/media-immersion',
        text_content: rss,
        metadata: { fixture: 'media-content-immersion' },
      }),
    });
    const body = await res.text();
    if (!res.ok) throw new Error(`seed podcast failed: ${res.status} ${body}`);
    return JSON.parse(body);
  }, { rss: buildPodcastRss(title), title });
}

async function appGeometry(page, viewer, app, viewportLabel, phase) {
  const metrics = await viewer.evaluate((root) => {
    const stage = root.querySelector('[data-media-stage]');
    if (!stage) return null;
    const rootBox = root.getBoundingClientRect();
    const stageBox = stage.getBoundingClientRect();
    const ratio = (stageBox.width * stageBox.height) / Math.max(1, rootBox.width * rootBox.height);
    return {
      root: {
        width: Math.round(rootBox.width),
        height: Math.round(rootBox.height),
      },
      stage: {
        width: Math.round(stageBox.width),
        height: Math.round(stageBox.height),
      },
      ratio: Number(ratio.toFixed(3)),
    };
  });
  expect(metrics, `${app} ${viewportLabel} ${phase} should have a media stage`).toBeTruthy();
  const windowEl = page.locator('[data-window]').filter({ has: viewer }).last();
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  await windowEl.screenshot({
    path: path.join(OUTPUT_DIR, `${viewportLabel}-${app}-${phase}.png`),
  });
  console.log(`media-immersion ${viewportLabel} ${app} ${phase} ${JSON.stringify(metrics)}`);
  return metrics;
}

async function expectStageOccupancy(page, viewer, app, viewportLabel, threshold) {
  const details = viewer.locator('details');
  const detailCount = await details.count();
  for (let index = 0; index < detailCount; index += 1) {
    await expect(details.nth(index), `${app} ${viewportLabel} detail ${index} closed by default`).toHaveJSProperty('open', false);
  }
  const closedChrome = await viewer.evaluate((root) =>
    Array.from(root.querySelectorAll('details')).map((detail) => {
      const rect = detail.getBoundingClientRect();
      return {
        width: Math.round(rect.width),
        height: Math.round(rect.height),
        text: (detail.querySelector('summary')?.innerText || '').trim(),
      };
    })
  );
  for (const chrome of closedChrome) {
    expect(chrome.width, `${app} ${viewportLabel} closed chrome width`).toBeLessThanOrEqual(48);
    expect(chrome.height, `${app} ${viewportLabel} closed chrome height`).toBeLessThanOrEqual(48);
    expect(chrome.text, `${app} ${viewportLabel} closed chrome label`).not.toMatch(/Controls|Info|Source|Provenance/i);
  }
  await expect(viewer.locator('[data-media-open-source]').first()).not.toBeVisible();

  const before = await appGeometry(page, viewer, app, viewportLabel, 'closed');
  expect(before.ratio, `${app} ${viewportLabel} stage ratio`).toBeGreaterThanOrEqual(threshold);

  const controls = viewer.locator('[data-media-controls]').first();
  if (await controls.count()) {
    await expect(controls).toHaveJSProperty('open', false);
    await controls.locator('summary').click();
    await expect(controls).toHaveJSProperty('open', true);
    await appGeometry(page, viewer, app, viewportLabel, 'controls-open');
    await controls.locator('summary').click();
    await expect(controls).toHaveJSProperty('open', false);
  }

  const after = await appGeometry(page, viewer, app, viewportLabel, 'controls-closed-again');
  expect(after.ratio, `${app} ${viewportLabel} restored stage ratio`).toBeGreaterThanOrEqual(threshold);
  for (let index = 0; index < detailCount; index += 1) {
    await expect(details.nth(index), `${app} ${viewportLabel} detail ${index} reclosed`).toHaveJSProperty('open', false);
  }
  await expect(viewer.locator('[data-media-open-source]').first()).not.toBeVisible();
}

async function openPromptRoutedApp(page, reference) {
  const decision = await submitBareReference(page, reference.url);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe(reference.app);
  const viewer = page.locator(`[data-media-app][data-media-kind="${reference.app}"]`).last();
  await expect(viewer).toBeVisible({ timeout: 30_000 });
  if (reference.readySelector) {
    await expect(viewer.locator(reference.readySelector)).toBeVisible({ timeout: reference.readyTimeout || 30_000 });
  }
  return viewer;
}

async function openPodcastApp(page, title) {
  const contentItem = await seedPodcastFeed(page, title);
  await page.locator('[data-desktop-icon-id="podcast"]').dblclick();
  const viewer = page.locator('[data-podcast-app]').last();
  await expect(viewer.locator('[data-podcast-library]')).toBeVisible({ timeout: 10_000 });
  await viewer.locator('[data-podcast-library-item]').filter({ hasText: contentItem.title }).click();
  await expect(viewer.locator('[data-podcast-feed]')).toBeVisible();
  await expect(viewer.locator('[data-podcast-player]')).toBeVisible();
  return viewer;
}

for (const viewport of [
  { label: 'desktop', size: { width: 1280, height: 900 } },
  { label: 'mobile390', size: { width: 390, height: 844 } },
]) {
  test(`media apps keep primary content full-window by default on ${viewport.label}`, async ({ page, authenticator }) => {
    expect(authenticator.authenticatorId).toBeTruthy();
    await page.setViewportSize(viewport.size);
    await routeMediaFixtures(page);
    await registerAndLoadDesktop(page, uniqueEmail(viewport.label));

    const podcast = await openPodcastApp(page, `Immersion Radio ${viewport.label}`);
    await expectStageOccupancy(page, podcast, 'podcast', viewport.label, 0.8);

    const promptRoutedApps = [
      {
        app: 'image',
        url: 'https://example.com/choir-immersion.png',
        threshold: 0.85,
        readySelector: '[data-image-viewer]',
      },
      {
        app: 'pdf',
        url: 'https://example.com/choir-immersion.pdf',
        threshold: 0.85,
        readySelector: '[data-pdf-reader][data-pdf-rendered="true"]',
        readyTimeout: 20_000,
      },
      {
        app: 'epub',
        url: 'https://example.com/choir-immersion.epub',
        threshold: 0.85,
        readySelector: '[data-epub-reader]',
        readyTimeout: 20_000,
      },
      {
        app: 'audio',
        url: 'https://example.com/choir-immersion.mp3',
        threshold: 0.8,
        readySelector: '[data-audio-player]',
      },
      {
        app: 'video',
        url: 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
        threshold: 0.85,
        readySelector: '[data-video-frame]',
      },
    ];

    for (const reference of promptRoutedApps) {
      const viewer = await openPromptRoutedApp(page, reference);
      await expectStageOccupancy(page, viewer, reference.app, viewport.label, reference.threshold);
    }
  });
}
