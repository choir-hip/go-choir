/**
 * Autopaper, logged out at phone width: the front list, and stories read in
 * the Texture app's read-only published mode (owner direction 2026-10-13:
 * "open Textures in the Texture app rather than opening the stories in the
 * Autopaper app"; the logged-out home opens Autopaper).
 *
 * Runs against the deployed origin (CHOIR_DEPLOYED_BASE_URL, default
 * https://choir.news) with no session:
 *   - the logged-out home opens the Autopaper window with the front list, and
 *     a story clicked there opens as a published Texture window titled with
 *     the story title;
 *   - the front list route returns each story once, newest first; /autopaper
 *     lays them out as a front page in rank order (rankFrontStories:
 *     importance x depth x recency when most stories are scored, else
 *     recency lifted by versions; this device has no section habit, and the
 *     front page is the default section), each marked "Updated" when a
 *     story has more than one version, and a story clicked there opens as a
 *     Texture published window (the standalone /autopaper page is gone, and
 *     an old /autopaper/<slug> link lands on /pub/texture/<slug>);
 *   - the text carries only numbered citation markers; the numbered source
 *     list stays closed until the toolbar's Sources button opens it above the
 *     text;
 *   - every citation renders as a numbered source whose quote sits inside its
 *     pinned source block (inline popover and the source list) with a check
 *     badge (exact, tolerant, pending, unread, failed); a stored embed is
 *     a numbered marker too, and no source text draws as a block; the view
 *     offers no editing affordance;
 *   - a story with more than one version moves between versions with the
 *     Texture chevrons, and an older version offers the newer one;
 *   - nothing scrolls horizontally at 375px;
 *   - the edge compresses the story bundle, the page and its scripts and
 *     styles (docs/archive/problems/edge-serves-everything-uncompressed-2026-10-16.md).
 * Screenshots are attached and written under test-results (or
 * AUTOPAPER_SCREENSHOT_DIR when set). Skips when no story is published.
 *
 *   cd frontend && npx playwright test tests/autopaper-reader-deployed.spec.js
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { expect, test } from '@playwright/test';
import { rankFrontStories } from '../src/lib/autopaper.ts';

const BASE_URL = (process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const MAX_STORIES = Number(process.env.AUTOPAPER_MAX_STORIES || 8);
const SHOT_DIR = process.env.AUTOPAPER_SCREENSHOT_DIR || '';
// The front page shows the first page of the front list (60 stories,
// FRONT_FIRST_PAGE_SIZE), ranked.
const FRONT_PAGE_SIZE = 60;
const CHECK_STATES = new Set(['exact', 'tolerant', 'pending', 'unread', 'failed']);

test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });
test.setTimeout(240_000);

async function getJSON(request, path) {
  const res = await request.get(`${BASE_URL}${path}`, { timeout: 30_000 });
  expect(res.status(), `${path} status`).toBe(200);
  return res.json();
}

async function publishedStories(request) {
  const front = await getJSON(request, '/api/platform/publications/front?limit=60');
  return Array.isArray(front.stories) ? front.stories : [];
}

function slugOf(route) {
  return String(route || '').replace(/^\/pub\/texture\//, '').split('/')[0];
}

async function expectNoHorizontalScroll(page, label) {
  const overflow = await page.evaluate(() => {
    const els = [
      document.scrollingElement,
      document.querySelector('[data-autopaper]'),
      document.querySelector('[data-texture-editor-area]'),
    ].filter(Boolean);
    return els.map((el) => ({ scroll: el.scrollWidth, client: el.clientWidth }));
  });
  for (const { scroll, client } of overflow) {
    expect(scroll, `${label}: horizontal overflow (${scroll} > ${client})`).toBeLessThanOrEqual(client + 1);
  }
}

async function shoot(page, testInfo, name, scroller = '') {
  const files = [];
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path });
  files.push(path);
  await testInfo.attach(name, { path, contentType: 'image/png' });
  // The document scrolls inside its own container; capture all of it too.
  if (scroller) {
    const height = await page.locator(scroller).first().evaluate((el) => el.scrollHeight).catch(() => 0);
    if (height > 0) {
      const full = testInfo.outputPath(`${name}-full.png`);
      await page.setViewportSize({ width: 375, height: Math.min(height + 160, 12_000) });
      await page.waitForTimeout(250);
      await page.screenshot({ path: full });
      await page.setViewportSize({ width: 375, height: 812 });
      files.push(full);
      await testInfo.attach(`${name}-full`, { path: full, contentType: 'image/png' });
    }
  }
  if (SHOT_DIR) {
    fs.mkdirSync(SHOT_DIR, { recursive: true });
    for (const file of files) fs.copyFileSync(file, nodePath.join(SHOT_DIR, nodePath.basename(file)));
  }
}

function manifestOf(bundle) {
  const manifest = bundle.payload?.manifest;
  return typeof manifest === 'string' ? JSON.parse(manifest) : manifest || {};
}

// Stories publish new versions while the spec runs, so "the latest" is read
// from the page: the bare link serves a version equal to its own latest, and
// never older than the front list said.
async function expectServesLatest(reader, atLeast, label) {
  await expect(reader).toHaveAttribute('data-publication-revision', /^[1-9][0-9]*$/, { timeout: 30_000 });
  const revision = Number(await reader.getAttribute('data-publication-revision'));
  const latest = Number(await reader.getAttribute('data-publication-latest'));
  expect(revision, `${label}: bare link serves the latest`).toBe(latest);
  expect(revision, `${label}: not older than the front list`).toBeGreaterThanOrEqual(atLeast);
  return revision;
}

async function expectReadOnly(reader, page, label) {
  await expect(reader, `${label}: not editable`).toHaveAttribute('contenteditable', 'false');
  await expect(page.locator('[data-texture-edit-published]'), `${label}: no edit action`).toHaveCount(0);
  await expect(page.locator('[data-texture-prompt]'), `${label}: no revise action`).toHaveCount(0);
  await expect(page.locator('[data-texture-publish]'), `${label}: no publish action`).toHaveCount(0);
}

test('logged-out home opens Autopaper; a story opens as a published Texture window', async ({ page, request }, testInfo) => {
  const stories = await publishedStories(request);

  await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await expect(page.locator('[data-desktop]')).toHaveAttribute('data-authenticated', 'false', { timeout: 30_000 });
  const home = page.locator('[data-window][data-window-id="public-preview-autopaper"]');
  await expect(home).toBeVisible({ timeout: 30_000 });
  await expect(home.locator('[data-autopaper-front]')).toBeVisible({ timeout: 30_000 });
  await expect(page.locator('[data-window]')).toHaveCount(1);
  await expect(page.locator('[data-texture-editor]')).toHaveCount(0);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');

  const firstRow = home.locator('[data-autopaper-story-row]').first();
  await expect(firstRow).toBeVisible({ timeout: 30_000 });
  await shoot(page, testInfo, 'home-autopaper-window');

  const lead = rankFrontStories(stories.slice(0, FRONT_PAGE_SIZE))[0];
  await firstRow.locator('[data-autopaper-headline] a').click();
  const reader = page.locator('[data-texture-app] [data-texture-published-reader]').last();
  await expect(reader).toBeVisible({ timeout: 30_000 });
  await expect(reader).toHaveAttribute('data-publication-route', lead.route.bare);
  const storyWindow = page.locator('[data-window]').filter({ has: reader });
  await expect(storyWindow.locator('[data-window-titlebar]')).toContainText(lead.title.trim().slice(0, 24));
  await expectReadOnly(reader, page, 'home story window');
  await expectNoHorizontalScroll(page, 'home story window');
  await shoot(page, testInfo, 'home-story-texture-window');
});

test('Autopaper front list and stories as Texture published views, logged out at phone width', async ({ page, request }, testInfo) => {
  const stories = await publishedStories(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');

  // The list itself: one row per story, newest first.
  const ids = stories.map((s) => s.publication_id);
  expect(new Set(ids).size, 'each story listed once').toBe(ids.length);
  const times = stories.map((s) => Date.parse(s.updated_at));
  for (let i = 1; i < times.length; i += 1) {
    expect(times[i], `story ${i} is not newer than story ${i - 1}`).toBeLessThanOrEqual(times[i - 1]);
  }

  // Front list, in the logged-out home's Autopaper window.
  await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await expect(page.locator('[data-autopaper-front]')).toBeVisible({ timeout: 30_000 });
  const rows = page.locator('[data-autopaper-story-row]');
  await expect(rows.first()).toBeVisible({ timeout: 30_000 });
  const shown = Math.min(stories.length, FRONT_PAGE_SIZE);
  const ranked = rankFrontStories(stories.slice(0, shown));
  await expect(rows).toHaveCount(shown);
  await expect(page.locator('[data-autopaper] h2')).toHaveCount(1);
  const titles = await rows.locator('[data-autopaper-headline]').allInnerTexts();
  expect(titles.map((t) => t.trim())).toEqual(ranked.map((s) => s.title.trim()));
  const updatedRows = stories.slice(0, shown).filter((s) => s.version_count > 1).length;
  await expect(page.locator('[data-updated-marker]')).toHaveCount(updatedRows);
  await expectNoHorizontalScroll(page, 'front list');
  await shoot(page, testInfo, 'autopaper-front', '[data-autopaper]');

  // The lead's headline opens the story as a Texture published window.
  await rows.first().locator('[data-autopaper-headline] a').click();
  const opened = page.locator('[data-texture-app] [data-texture-published-reader]').last();
  await expect(opened).toBeVisible({ timeout: 30_000 });
  await expect(opened).toHaveAttribute('data-publication-route', ranked[0].route.bare);
  await expectServesLatest(opened, ranked[0].latest_revision, 'lead story');
  await expect(page.locator('[data-window]').filter({ has: opened }).locator('[data-window-titlebar]')).toContainText(ranked[0].title.trim().slice(0, 24));

  // Each story: numbered sources, quotes in their source blocks, badges, embeds.
  const checked = [];
  for (const story of stories.slice(0, MAX_STORIES)) {
    const slug = slugOf(story.route?.bare);
    const bundle = await getJSON(request, `/api/platform/publications/resolve?route=${encodeURIComponent(story.route.bare)}`);
    const anchors = Array.isArray(manifestOf(bundle).anchors) ? manifestOf(bundle).anchors : [];
    expect(bundle.nav?.latest, `${slug}: bare link serves the latest`).toBeGreaterThanOrEqual(story.latest_revision);

    await page.goto(`${BASE_URL}/pub/texture/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
    const reader = page.locator('[data-texture-published-reader]');
    await expect(reader).toBeVisible({ timeout: 30_000 });
    await expectServesLatest(reader, story.latest_revision, slug);
    await expectReadOnly(reader, page, slug);

    const cites = anchors.filter((a) => a.relation !== 'embeds' || a.ref?.quote);
    const embeds = anchors.filter((a) => a.relation === 'embeds');
    const sourcesPanel = page.locator('[data-texture-published-sources-panel]');
    if (anchors.length) {
      await expect(reader.locator('[data-source-block]'), `${slug}: no source list inside the text`).toHaveCount(0);
      await expect(sourcesPanel, `${slug}: sources closed by default`).toHaveCount(0);
      await page.locator('[data-texture-published-sources-toggle]').first().click();
      await expect(sourcesPanel, `${slug}: Sources opens the list`).toBeVisible();
      const panelBox = await sourcesPanel.boundingBox();
      const readerBox = await reader.boundingBox();
      expect(panelBox.y, `${slug}: sources open above the text`).toBeLessThan(readerBox.y);
      const numbers = await sourcesPanel.locator('[data-source-block]').evaluateAll((els) => els.map((el) => Number(el.getAttribute('data-source-number'))));
      expect(numbers.length, `${slug}: numbered sources`).toBeGreaterThan(0);
      expect(numbers, `${slug}: sources numbered 1..n`).toEqual(numbers.map((_, i) => i + 1));
    }
    for (const anchor of cites) {
      const marker = reader.locator(`[data-texture-source-ref][data-citation-anchor="${anchor.anchor_id}"]`);
      await expect(marker, `${slug}: numbered marker for ${anchor.anchor_id}`).toHaveCount(1);
      await expect(marker.locator('[data-texture-source-ref-popover] [data-citation-quote]'), `${slug}: popover quote of ${anchor.anchor_id}`).not.toHaveCount(0);
      const state = await marker.getAttribute('data-check');
      expect(CHECK_STATES.has(state), `${slug}: marker check state ${state}`).toBe(true);

      const item = sourcesPanel.locator(`[data-source-block] [data-citation-anchor="${anchor.anchor_id}"]`);
      await expect(item, `${slug}: citation ${anchor.anchor_id} in a source block`).toHaveCount(1);
      await expect(item.locator('[data-citation-quote]').first(), `${slug}: quote of ${anchor.anchor_id}`).toBeVisible();
      const badge = item.locator('[data-check-badge]');
      await expect(badge, `${slug}: badge of ${anchor.anchor_id}`).toHaveCount(1);
      expect(CHECK_STATES.has(await badge.getAttribute('data-check-badge'))).toBe(true);
    }
    // A stored embed is a numbered marker; the reader never prints a
    // source's text as a block (published-embeds-republish-source-text-2026-10-16).
    await expect(reader.locator('[data-embed-block]'), `${slug}: no source text drawn as a block`).toHaveCount(0);
    for (const anchor of embeds) {
      await expect(reader.locator(`[data-texture-source-ref][data-citation-anchor="${anchor.anchor_id}"]`), `${slug}: embed ${anchor.anchor_id} as a marker`).toHaveCount(1);
    }
    await expectNoHorizontalScroll(page, `story ${slug}`);
    checked.push({ slug, versions: story.version_count, citations: cites.length, embeds: embeds.length });

    if (checked.length === 1) {
      await shoot(page, testInfo, 'autopaper-story-1', '[data-texture-editor-area]');
      if (cites.length) {
        // A number opens its quote in its source block beside the text.
        const marker = reader.locator(`[data-texture-source-ref][data-citation-anchor="${cites[0].anchor_id}"]`);
        await marker.scrollIntoViewIfNeeded();
        await marker.click();
        const note = reader.locator('[data-texture-source-flow-note]').first();
        await expect(note).toBeVisible({ timeout: 10_000 });
        await expect(note.locator('[data-citation-quote]').first()).toBeVisible();
        await expect(note.locator('[data-check-badge]')).toHaveCount(1);
        await expectNoHorizontalScroll(page, `story ${slug} source note`);
        await shoot(page, testInfo, 'autopaper-story-1-source-note');
        await note.locator('[data-texture-source-flow-collapse]').click();
      }
    }
  }
  testInfo.annotations.push({ type: 'stories-checked', description: JSON.stringify(checked) });

  // Version chevrons and "new version available" on a story with several versions.
  const versioned = stories.find((s) => s.version_count > 1);
  if (!versioned) {
    testInfo.annotations.push({ type: 'chevrons', description: 'skipped: no story has more than one version yet' });
    return;
  }
  const slug = slugOf(versioned.route.bare);
  await page.goto(`${BASE_URL}/pub/texture/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  const reader = page.locator('[data-texture-published-reader]');
  const latestRevision = await expectServesLatest(reader, versioned.latest_revision, slug);
  const prev = page.locator('[data-texture-prev]');
  await expect(prev).toBeEnabled();
  await expect(page.locator('[data-texture-next]')).toBeDisabled();
  await expect(page.locator('[data-texture-new-version]')).toHaveCount(0);
  await prev.click();
  await expect(reader).not.toHaveAttribute('data-publication-revision', String(latestRevision), { timeout: 30_000 });
  const olderRevision = Number(await reader.getAttribute('data-publication-revision'));
  expect(olderRevision).toBeLessThan(latestRevision);
  await expect(page.locator('[data-texture-version]')).toHaveText(`v${olderRevision}`);
  await expect(page.locator('[data-texture-next]')).toBeEnabled();
  await expect(page.locator('[data-texture-new-version]')).toBeVisible();
  await expectNoHorizontalScroll(page, `story ${slug} v${olderRevision}`);
  await shoot(page, testInfo, 'autopaper-story-older-version');

  await page.locator('[data-texture-new-version]').click();
  await expect(reader).not.toHaveAttribute('data-publication-revision', String(olderRevision), { timeout: 30_000 });
  await expectServesLatest(reader, latestRevision, `${slug} latest again`);

  // An old /autopaper link lands on the same story's public Texture route,
  // at its version when the link names one.
  await page.goto(`${BASE_URL}/autopaper/${slug}/v${olderRevision}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await expect(page).toHaveURL(`${BASE_URL}/pub/texture/${slug}/v${olderRevision}`, { timeout: 30_000 });
  await expect(reader).toHaveAttribute('data-publication-revision', String(olderRevision), { timeout: 30_000 });
});

// A story bundle is up to about 460 KB of JSON and the app ships about
// 475 KB of script and CSS; uncompressed, a phone pays for every byte.
// Ways this could fail: the edge sends no Content-Encoding; it compresses
// the page but not the API or the hashed assets; the decoded body differs.
test('the edge compresses the story bundle, the page, and its scripts and styles', async ({ request }) => {
  // Playwright's request client decodes gzip and br, not zstd; browsers
  // that advertise zstd get it.
  const headers = { 'Accept-Encoding': 'gzip, br' };
  const stories = await publishedStories(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');
  const largest = [...stories].sort((a, b) => (b.source_count || 0) - (a.source_count || 0))[0];
  const bundlePath = `/api/platform/publications/resolve?route=${encodeURIComponent(largest.route.bare)}`;
  const page = await request.get(`${BASE_URL}/`, { headers, timeout: 30_000 });
  expect(page.status()).toBe(200);
  const html = await page.text();
  const assets = [...new Set(html.match(/\/assets\/[A-Za-z0-9._-]+\.(?:js|css)/g) || [])];
  expect(assets.length, 'the page names its scripts and styles').toBeGreaterThan(0);
  for (const path of ['/', bundlePath, ...assets]) {
    const res = await request.get(`${BASE_URL}${path}`, { headers, timeout: 30_000 });
    expect(res.status(), `${path} status`).toBe(200);
    expect(res.headers()['content-encoding'] || '', `${path} is compressed`).toMatch(/^(gzip|br)$/);
  }
  const bundle = await getJSON(request, bundlePath);
  expect(bundle.route?.bare, 'the compressed bundle decodes to the story').toBe(largest.route.bare);
});
