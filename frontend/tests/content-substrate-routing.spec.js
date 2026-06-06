import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(180_000);
test.skip(
  process.env.GO_CHOIR_RUN_CONTENT_SUBSTRATE !== '1',
  'set GO_CHOIR_RUN_CONTENT_SUBSTRATE=1 to verify content substrate routing'
);

function uniqueEmail() {
  return `content-substrate-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(async ({ requestPath, requestOptions }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      ...requestOptions,
      headers: {
        'Content-Type': 'application/json',
        ...(requestOptions.headers || {}),
      },
    });
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, { requestPath: path, requestOptions: options });
}

async function waitForPromptDecision(page, submissionId, timeout = 90_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

function provenanceRungs(item) {
  return (item.provenance?.rungs || []).map((rung) => rung.name);
}

test('prompt bar routes bare content references and product APIs record extraction provenance', async ({ page, authenticator }) => {
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

  await registerAndLoadDesktop(page, uniqueEmail());

  const barePDF = 'https://example.com/choir-proof.pdf';
  const promptBarResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(barePDF);
  await page.locator('[data-prompt-input]').press('Enter');

  const response = await promptBarResponse;
  expect(response.status()).toBe(202);
  const body = await response.json();
  const decision = await waitForPromptDecision(page, body.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('pdf');
  expect(decision.source_url).toBe(barePDF);

  const contentWindow = page.locator('[data-pdf-window]').last();
  await expect(contentWindow).toBeVisible({ timeout: 30_000 });
  await expect(contentWindow.locator('[data-media-app][data-media-kind="pdf"]')).toBeVisible();

  const imported = await fetchJSON(page, '/api/content/import-url', {
    method: 'POST',
    body: JSON.stringify({
      url: 'https://example.com/',
      query: 'Example Domain',
    }),
  });
  expect(imported.content_id).toBeTruthy();
  expect(imported.source_type).toBe('extracted_url');
  expect(imported.media_type).toBe('text/markdown');
  expect(imported.app_hint).toBe('content');
  expect(imported.metadata.original_media_type).toBe('text/html');
  expect(imported.metadata.reader_artifact_kind).toBe('cleaned_reader_markdown');
  expect(imported.content_hash).toMatch(/^[a-f0-9]{64}$/);
  expect(imported.text_content).toContain('Example Domain');
  expect(provenanceRungs(imported)).toEqual(expect.arrayContaining(['direct_http', 'readability_lite']));
  expect(imported.provenance.hash_algorithm).toBe('sha256');

  const loaded = await fetchJSON(page, `/api/content/items/${encodeURIComponent(imported.content_id)}`);
  expect(loaded.content_id).toBe(imported.content_id);
  expect(loaded.provenance.rungs.length).toBeGreaterThanOrEqual(2);

  const mediaReferences = [
    ['application/pdf', 'pdf', 'https://example.com/whitepaper.pdf'],
    ['application/epub+zip', 'epub', 'https://example.com/book.epub'],
    ['image/png', 'image', 'https://example.com/image.png'],
    ['audio/mpeg', 'audio', 'https://example.com/audio.mp3'],
    ['video/youtube', 'video', 'https://www.youtube.com/watch?v=dQw4w9WgXcQ'],
    ['application/rss+xml', 'podcast', 'https://example.com/podcast.rss'],
  ];
  for (const [mediaType, appHint, sourceUrl] of mediaReferences) {
    const item = await fetchJSON(page, '/api/content/items', {
      method: 'POST',
      body: JSON.stringify({
        source_type: 'url',
        source_url: sourceUrl,
        media_type: mediaType,
        title: `${appHint} reference`,
      }),
    });
    expect(item.app_hint).toBe(appHint);
    expect(item.source_url).toBe(sourceUrl);
  }

  expect(forbiddenRequests).toHaveLength(0);
});
