/**
 * Autopaper reader, logged out at phone width (Cut 2 unit 2b, Reader W3a;
 * docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md,
 * autopaper_live (4) and the "logged-out Playwright on choir.news at phone
 * width" acceptance).
 *
 * Runs against the deployed origin (CHOIR_DEPLOYED_BASE_URL, default
 * https://choir.news) with no session:
 *   - the front page lists each story once, newest first, marked "Updated"
 *     when a story has more than one version;
 *   - every citation on a story page shows its quote inside its numbered
 *     source block with a check badge (exact, tolerant, pending, failed), and
 *     every transclusion renders as a block;
 *   - a story with more than one version shows chevrons, and an older
 *     version offers the newer one;
 *   - no page scrolls horizontally at 375px.
 * Screenshots of each page are attached and written under test-results.
 * The spec skips when no Autopaper story is published yet.
 *
 *   cd frontend && npx playwright test tests/autopaper-reader-deployed.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const MAX_STORIES = Number(process.env.AUTOPAPER_MAX_STORIES || 8);
const CHECK_STATES = new Set(['exact', 'tolerant', 'pending', 'failed']);

test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });
test.setTimeout(180_000);

async function getJSON(request, path) {
  const res = await request.get(`${BASE_URL}${path}`, { timeout: 30_000 });
  expect(res.status(), `${path} status`).toBe(200);
  return res.json();
}

function slugOf(route) {
  return String(route || '').replace(/^\/pub\/texture\//, '').split('/')[0];
}

async function expectNoHorizontalScroll(page, label) {
  const overflow = await page.evaluate(() => {
    const els = [document.scrollingElement, document.querySelector('[data-autopaper]')].filter(Boolean);
    return els.map((el) => ({ scroll: el.scrollWidth, client: el.clientWidth }));
  });
  for (const { scroll, client } of overflow) {
    expect(scroll, `${label}: horizontal overflow (${scroll} > ${client})`).toBeLessThanOrEqual(client + 1);
  }
}

async function shoot(page, testInfo, name) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: false });
  await testInfo.attach(name, { path, contentType: 'image/png' });
  // The reader scrolls inside its own container; capture the whole story too.
  const full = testInfo.outputPath(`${name}-full.png`);
  const root = page.locator('[data-autopaper]');
  const height = await root.evaluate((el) => el.scrollHeight).catch(() => 0);
  if (height > 0) {
    await page.setViewportSize({ width: 375, height: Math.min(height + 80, 12_000) });
    await page.screenshot({ path: full, fullPage: true });
    await page.setViewportSize({ width: 375, height: 812 });
    await testInfo.attach(`${name}-full`, { path: full, contentType: 'image/png' });
  }
}

test('Autopaper front page and story pages, logged out at phone width', async ({ page, request }, testInfo) => {
  const front = await getJSON(request, '/api/platform/publications/front?limit=50');
  const stories = Array.isArray(front.stories) ? front.stories : [];
  test.skip(stories.length === 0, 'no Autopaper story is published yet');

  // The list itself: one row per story, newest first.
  const ids = stories.map((s) => s.publication_id);
  expect(new Set(ids).size, 'each story listed once').toBe(ids.length);
  const times = stories.map((s) => Date.parse(s.updated_at));
  for (let i = 1; i < times.length; i += 1) {
    expect(times[i], `story ${i} is not newer than story ${i - 1}`).toBeLessThanOrEqual(times[i - 1]);
  }

  // Front page.
  await page.goto(`${BASE_URL}/autopaper`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await expect(page.locator('[data-autopaper-front]')).toBeVisible({ timeout: 30_000 });
  const rows = page.locator('[data-autopaper-story-row]');
  await expect(rows.first()).toBeVisible({ timeout: 30_000 });
  const shown = Math.min(stories.length, 20);
  await expect(rows).toHaveCount(shown);
  const titles = await rows.locator('.ap-row-title').allInnerTexts();
  expect(titles.map((t) => t.trim())).toEqual(stories.slice(0, shown).map((s) => s.title.trim()));
  const updatedRows = stories.slice(0, shown).filter((s) => s.version_count > 1).length;
  await expect(page.locator('[data-updated-marker]')).toHaveCount(updatedRows);
  await expectNoHorizontalScroll(page, 'front page');
  await shoot(page, testInfo, 'autopaper-front');

  // Story pages: every citation's quote sits in its source block with a badge.
  const checked = [];
  for (const story of stories.slice(0, MAX_STORIES)) {
    const slug = slugOf(story.route?.bare);
    const bundle = await getJSON(request, `/api/platform/publications/resolve?route=${encodeURIComponent(story.route.bare)}`);
    const manifest = typeof bundle.payload?.manifest === 'string' ? JSON.parse(bundle.payload.manifest) : bundle.payload?.manifest || {};
    const anchors = Array.isArray(manifest.anchors) ? manifest.anchors : [];
    expect(bundle.nav?.latest, `${slug}: bare link serves the latest`).toBe(story.latest_revision);

    await page.goto(`${BASE_URL}/autopaper/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
    const article = page.locator('[data-autopaper-story]');
    await expect(article).toBeVisible({ timeout: 30_000 });
    await expect(article).toHaveAttribute('data-revision', String(story.latest_revision));

    const cites = anchors.filter((a) => a.relation !== 'embeds' || a.ref?.quote);
    const embeds = anchors.filter((a) => a.relation === 'embeds');
    if (anchors.length) {
      const numbers = await page.locator('[data-source-block]').evaluateAll((els) => els.map((el) => Number(el.getAttribute('data-source-number'))));
      expect(numbers.length, `${slug}: numbered sources`).toBeGreaterThan(0);
      expect(numbers, `${slug}: sources numbered 1..n`).toEqual(numbers.map((_, i) => i + 1));
    }
    for (const anchor of cites) {
      const item = page.locator(`[data-source-block] [data-citation-anchor="${anchor.anchor_id}"]`);
      await expect(item, `${slug}: citation ${anchor.anchor_id} in a source block`).toHaveCount(1);
      await expect(item.locator('[data-citation-quote]').first(), `${slug}: quote of ${anchor.anchor_id}`).toBeVisible();
      const badge = item.locator('[data-check-badge]');
      await expect(badge, `${slug}: badge of ${anchor.anchor_id}`).toHaveCount(1);
      expect(CHECK_STATES.has(await badge.getAttribute('data-check-badge'))).toBe(true);
    }
    for (const anchor of embeds) {
      await expect(page.locator(`[data-embed-block][data-citation-anchor="${anchor.anchor_id}"]`), `${slug}: transclusion ${anchor.anchor_id} as a block`).toHaveCount(1);
    }
    await expectNoHorizontalScroll(page, `story ${slug}`);
    checked.push({ slug, versions: story.version_count, citations: cites.length, embeds: embeds.length });
    if (checked.length === 1 || (anchors.length && !checked.some((c) => c.shot))) {
      await shoot(page, testInfo, `autopaper-story-${checked.length}`);
      checked[checked.length - 1].shot = true;
    }
  }
  testInfo.annotations.push({ type: 'stories-checked', description: JSON.stringify(checked) });

  // Chevrons and "new version available" on a story with several versions.
  const versioned = stories.find((s) => s.version_count > 1);
  if (!versioned) {
    testInfo.annotations.push({ type: 'chevrons', description: 'skipped: no story has more than one version yet' });
    return;
  }
  const slug = slugOf(versioned.route.bare);
  await page.goto(`${BASE_URL}/autopaper/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  const nav = page.locator('[data-version-nav]');
  await expect(nav).toBeVisible({ timeout: 30_000 });
  const prev = nav.locator('[data-version-prev]');
  await expect(prev).toBeVisible();
  await expect(page.locator('[data-new-version]')).toHaveCount(0);
  await prev.click();
  await expect(page).toHaveURL(new RegExp(`/autopaper/${slug}/v\\d+$`));
  const older = page.locator('[data-autopaper-story]');
  await expect(older).toBeVisible();
  const olderRevision = Number(await older.getAttribute('data-revision'));
  expect(olderRevision).toBeLessThan(versioned.latest_revision);
  await expect(page.locator('[data-version-nav] [data-version-next]')).toBeVisible();
  await expect(page.locator('[data-new-version]')).toBeVisible();
  await expectNoHorizontalScroll(page, `story ${slug} v${olderRevision}`);
  await shoot(page, testInfo, 'autopaper-story-older-version');

  await page.locator('[data-new-version-open]').click();
  await expect(page).toHaveURL(new RegExp(`/autopaper/${slug}$`));
  await expect(page.locator('[data-autopaper-story]')).toHaveAttribute('data-revision', String(versioned.latest_revision));
});
