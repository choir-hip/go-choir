import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

function episodeXml(index, overrides = {}) {
  const title = overrides.title || `Episode ${String(index).padStart(2, '0')}`;
  const guid = overrides.guid || `mission-radio-${index}`;
  const duration = overrides.duration || '12:34';
  const description = overrides.description || `Episode ${index} keeps the podcast list scrollable on mobile.`;
  const audio = overrides.audio || `https://example.com/audio/episode-${index}.mp3`;
  return `
    <item>
      <title>${title}</title>
      <guid>${guid}</guid>
      <link>https://example.com/mission-radio/${index}</link>
      <pubDate>Wed, 13 May 2026 ${String((8 + index) % 24).padStart(2, '0')}:00:00 GMT</pubDate>
      <itunes:duration>${duration}</itunes:duration>
      <description>${description}</description>
      <enclosure url="${audio}" type="audio/mpeg" length="${12345 + index}" />
    </item>`;
}

function buildPodcastRss(title, episodeItems) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>${title}</title>
    <link>https://example.com/mission-radio</link>
    <description>Dispatches about candidate worlds, verifier contracts, and promoted meaning.</description>
    ${episodeItems.join('\n')}
  </channel>
</rss>`;
}

const FIXTURE_RSS = buildPodcastRss('Mission Gradient Radio', [
  episodeXml(1, {
    title: 'Candidate Worlds First',
    description: 'How background mutation stays out of canonical state until promotion.',
    audio: 'https://example.com/audio/candidate-worlds.mp3',
  }),
  episodeXml(2, {
    title: 'Verifier Contracts',
    description: 'Why verification is a contract and not an agent caste.',
    audio: 'https://example.com/audio/verifier-contracts.mp3',
  }),
]);

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

async function seedPodcastFeed(page, { rss = FIXTURE_RSS, title = 'Mission Gradient Radio' } = {}) {
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
  }, { rss, title });
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

test('podcast mobile detail keeps a long episode list scrollable with player controls reachable', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, uniqueEmail());
  const title = 'Mobile Scroll Radio';
  const rss = buildPodcastRss(
    title,
    Array.from({ length: 18 }, (_, index) => episodeXml(index + 1, {
      title: `Mobile Episode ${String(index + 1).padStart(2, '0')}`,
    }))
  );
  const contentItem = await seedPodcastFeed(page, { rss, title });

  await page.locator('[data-desktop-icon-id="podcast"]').dblclick();
  const podcastWindow = page.locator('[data-window]').filter({ has: page.locator('[data-podcast-app]') }).last();
  const podcastApp = podcastWindow.locator('[data-podcast-app]');
  await expect(podcastApp.locator('[data-podcast-library]')).toBeVisible({ timeout: 10_000 });
  await podcastApp.locator('[data-podcast-library-item]').filter({ hasText: contentItem.title }).click();

  const episodeList = podcastApp.locator('[data-podcast-episodes-scroll]');
  await expect(episodeList).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-episode]')).toHaveCount(18);
  await expect(podcastApp.locator('[data-podcast-player]')).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-seek-back]')).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-play-pause]')).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-seek-forward]')).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-seek]')).toBeVisible();
  await expect(podcastApp.locator('[data-podcast-speed]')).toBeVisible();

  const beforeScroll = await episodeList.evaluate((el) => ({
    clientHeight: el.clientHeight,
    scrollHeight: el.scrollHeight,
    overflowY: getComputedStyle(el).overflowY,
  }));
  expect(beforeScroll.clientHeight).toBeGreaterThanOrEqual(180);
  expect(beforeScroll.scrollHeight).toBeGreaterThan(beforeScroll.clientHeight + 120);
  expect(beforeScroll.overflowY).toMatch(/auto|scroll/);

  await episodeList.evaluate((el) => {
    el.scrollTop = el.scrollHeight;
    el.dispatchEvent(new Event('scroll', { bubbles: true }));
  });

  const afterScroll = await episodeList.evaluate((el) => {
    const listBox = el.getBoundingClientRect();
    const last = el.querySelector('[data-podcast-episode]:last-child');
    const lastBox = last?.getBoundingClientRect();
    return {
      scrollTop: el.scrollTop,
      lastVisible: !!lastBox && lastBox.bottom <= listBox.bottom + 2 && lastBox.top >= listBox.top - 2,
    };
  });
  expect(afterScroll.scrollTop).toBeGreaterThan(0);
  expect(afterScroll.lastVisible).toBe(true);
  await expect(podcastApp.locator('[data-podcast-player]')).toBeVisible();
});
