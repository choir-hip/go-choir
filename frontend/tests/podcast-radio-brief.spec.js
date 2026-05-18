import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

const FIXTURE_RSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Mission Gradient Radio</title>
    <link>https://example.com/mission-radio</link>
    <description>Dispatches about candidate worlds, verifier contracts, and promoted meaning.</description>
    <item>
      <title>Candidate Worlds First</title>
      <guid>mission-radio-1</guid>
      <link>https://example.com/mission-radio/1</link>
      <pubDate>Wed, 13 May 2026 10:00:00 GMT</pubDate>
      <itunes:duration>12:34</itunes:duration>
      <description>How background mutation stays out of canonical state until promotion.</description>
      <enclosure url="https://example.com/audio/candidate-worlds.mp3" type="audio/mpeg" length="12345" />
    </item>
    <item>
      <title>Verifier Contracts</title>
      <guid>mission-radio-2</guid>
      <link>https://example.com/mission-radio/2</link>
      <pubDate>Wed, 13 May 2026 11:00:00 GMT</pubDate>
      <description>Why verification is a contract and not an agent caste.</description>
      <enclosure url="https://example.com/audio/verifier-contracts.mp3" type="audio/mpeg" length="67890" />
    </item>
  </channel>
</rss>`;

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(120_000);

function uniqueEmail() {
  return `podcast-radio-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

async function seedPodcastFeed(page) {
  return page.evaluate(async (rss) => {
    const res = await fetch('/api/content/items', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source_type: 'url',
        media_type: 'application/rss+xml',
        app_hint: 'podcast',
        title: 'Mission Gradient Radio',
        source_url: 'https://example.com/mission-radio.rss',
        canonical_url: 'https://example.com/mission-radio',
        text_content: rss,
        metadata: {
          fixture: 'podcast-radio-brief',
        },
      }),
    });
    const body = await res.text();
    if (!res.ok) throw new Error(`seed podcast failed: ${res.status} ${body}`);
    return JSON.parse(body);
  }, FIXTURE_RSS);
}

test('podcast app opens a durable feed artifact as a full player app', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await registerAndLoadDesktop(page, uniqueEmail());
  const contentItem = await seedPodcastFeed(page);

  const podcastIcon = page.locator('[data-desktop-icon-id="podcast"]');
  await expect(podcastIcon).toBeVisible();
  await podcastIcon.dblclick();

  const podcastWindow = page.locator('[data-podcast-app]').last();
  await expect(podcastWindow.locator('[data-podcast-library]')).toBeVisible({ timeout: 10_000 });
  const seededFeed = podcastWindow
    .locator('[data-podcast-library-item]')
    .filter({ hasText: contentItem.title });
  await expect(seededFeed).toBeVisible();
  await seededFeed.click();

  await expect(podcastWindow.locator('[data-radio-listen-path]')).toBeVisible();
  await expect(podcastWindow.locator('[data-radio-listen-path]')).toContainText('Mission Gradient Radio');
  await expect(podcastWindow.locator('.podcast-topbar h2')).toContainText('Mission Gradient Radio');
  await expect(podcastWindow.locator('.podcast-topbar h2')).not.toContainText(/\.xml|[0-9a-f]{8}-[0-9a-f]{4}/i);
  await expect(podcastWindow.locator('[data-content-provenance]')).toHaveCount(0);
  await expect(podcastWindow.locator('[data-podcast-episode]')).toHaveCount(2);
  await expect(podcastWindow.locator('[data-podcast-player]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-player]')).toContainText('Candidate Worlds First');
  await expect(podcastWindow.locator('[data-podcast-controls]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-seek-back]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-play-pause]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-seek-forward]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-seek]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-speed]')).toBeVisible();
  await expect(podcastWindow.locator('[data-podcast-audio]')).toHaveAttribute('src', /candidate-worlds\.mp3$/);

  await podcastWindow.locator('[data-podcast-select-episode]').last().click();
  await expect(podcastWindow.locator('[data-podcast-player]')).toContainText('Verifier Contracts');
  await expect(podcastWindow.locator('[data-podcast-audio]')).toHaveAttribute('src', /verifier-contracts\.mp3$/);
  await podcastWindow.locator('[data-podcast-back]').click();
  await expect(podcastWindow.locator('[data-podcast-library]')).toBeVisible();
  await expect(podcastWindow.locator('text=Loading podcast artifacts...')).toHaveCount(0);
  await expect(podcastWindow.locator('[data-podcast-import]')).not.toBeVisible();
  await expect(seededFeed).toBeVisible();
});
