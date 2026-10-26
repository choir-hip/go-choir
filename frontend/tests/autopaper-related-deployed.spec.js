/**
 * "More on this" on the deployed Autopaper (related-stories design section 9,
 * docs/reports/related-stories-graph-design-2026-10-19.md, phase 1).
 *
 * Runs against the deployed origin (CHOIR_DEPLOYED_BASE_URL, default
 * https://choir.news) with no session, at phone width. It reads the public
 * front list and the public related route
 * (GET /api/platform/publications/related), then opens a story in the
 * browser. There is no event signal: every link's reason is "capture",
 * "source" or "entity" (a central entity both stories are about, slice b of
 * docs/problems/related-stories-no-entity-layer-2026-10-26.md), and
 * basis.events is 0.
 *
 * API assertions:
 *   1. Every front story's related answer is 200 with the documented shape.
 *   2. At least one story has a non-empty list; if none does the failure
 *      prints the basis of the first five stories so the cause is readable
 *      (no edges yet: run the backfill; no shared sources; kill switch).
 *   3. Every link resolves, is on the front list, is not the story itself,
 *      and appears once per list.
 *   4. A story that cites sources reports basis.captures > 0: the publish
 *      path wrote its cites_capture edges (or the backfill did).
 *   5. Hub rule: no source link is justified by a source cited by more than
 *      six stories (shared.citers); an entity link shares at least one
 *      central entity (shared.entities).
 *
 * Coverage and the entity loop (slice e):
 *   6. The share of the first 60 front stories with a link is reported with
 *      every story's basis.mode. Once the entity loop has reviewed stories on
 *      7 days, the share must reach AUTOPAPER_RELATED_COVERAGE_FLOOR (default
 *      0.25), so decay is no longer silent.
 *   7. The entity growth series (GET /api/platform/entities/growth) has its
 *      documented shape and is attached; after 14 reviewed days the
 *      convergence signals hold: existing_unmatched at most 20 percent and
 *      alias sprawl at most 15 percent over the last 7 days, the novelty
 *      "yes" rate at most 30 percent, and mints per story falling (the last
 *      7 days under 0.8 times the first 7).
 *
 * Browser assertions (a story with a non-empty list):
 *   - the related request starts after the resolve answered, so the list is
 *     fetched after the article and never inside resolve;
 *   - scrolled to its end, the article shows [data-texture-related] below
 *     the last paragraph, headed "More on this", with 1 to 5 links, and
 *     nothing scrolls horizontally at 375px;
 *   - a click on the first link (a real anchor to /pub/texture/<slug>) moves the
 *     Texture window to that story, the reader's
 *     publication id changes, and the new story renders;
 *   - a story with an empty list renders no [data-texture-related] and no
 *     heading.
 *
 * The run's artifact (every linked pair, its reason, shared counts and the
 * story's basis) is attached as related-pairs.json and written under
 * AUTOPAPER_RELATED_ARTIFACT_DIR when set, so a human can read the pairs
 * for the precision check.
 *
 *   cd frontend && npx playwright test tests/autopaper-related-deployed.spec.js
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const ARTIFACT_DIR = process.env.AUTOPAPER_RELATED_ARTIFACT_DIR || '';
// The hub cutoff of the walk (relatedTuning.HubStories), restated here as the
// property the spec checks from outside.
const HUB_STORIES = 6;
const MAX_LINKS = 5;
const COVERAGE_FLOOR = Number(process.env.AUTOPAPER_RELATED_COVERAGE_FLOOR || '0.25');

test.use({ viewport: { width: 375, height: 812 }, storageState: { cookies: [], origins: [] } });
test.setTimeout(240_000);

async function getJSON(request, path) {
  const res = await request.get(`${BASE_URL}${path}`, { timeout: 30_000 });
  expect(res.status(), `${path} status`).toBe(200);
  return res.json();
}

function slugOf(route) {
  return String(route || '').replace(/^\/pub\/texture\//, '').split('/')[0];
}

async function frontStories(request) {
  const front = await getJSON(request, '/api/platform/publications/front?limit=60');
  return Array.isArray(front.stories) ? front.stories : [];
}

// Every story on the front list, paged to its end (a link may point at a
// story older than the first page the walk is asked about).
async function allFrontRoutes(request) {
  const routes = new Set();
  let cursor = '';
  for (let page = 0; page < 20; page += 1) {
    const params = new URLSearchParams({ limit: '100' });
    if (cursor) params.set('cursor', cursor);
    const front = await getJSON(request, `/api/platform/publications/front?${params}`);
    for (const story of front.stories || []) routes.add(story.route.bare);
    cursor = front.next_cursor || '';
    if (!cursor) break;
  }
  return routes;
}

async function relatedOf(request, story) {
  return getJSON(request, `/api/platform/publications/related?route=${encodeURIComponent(story.route.bare)}&limit=${MAX_LINKS}`);
}

async function expectNoHorizontalScroll(page, label) {
  const overflow = await page.evaluate(() =>
    [document.scrollingElement, document.querySelector('[data-autopaper]'), document.querySelector('[data-texture-editor-area]')]
      .filter(Boolean)
      .map((el) => ({ scroll: el.scrollWidth, client: el.clientWidth })),
  );
  for (const { scroll, client } of overflow) {
    expect(scroll, `${label}: horizontal overflow (${scroll} > ${client})`).toBeLessThanOrEqual(client + 1);
  }
}

// The related answer of every front story, once per run.
let cache = null;
async function allRelated(request) {
  if (cache) return cache;
  const stories = await frontStories(request);
  const answers = [];
  for (const story of stories) {
    answers.push({ story, related: await relatedOf(request, story) });
  }
  cache = { stories, answers };
  return cache;
}

test('every front story has a well-formed related answer, and some are linked', async ({ request }, testInfo) => {
  const { stories, answers } = await allRelated(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');
  const frontRoutes = await allFrontRoutes(request);
  const pairs = [];

  for (const { story, related } of answers) {
    const label = story.route.bare;
    // 1. the documented shape
    expect(related.route, `${label}: route`).toBe(story.route.bare);
    expect(Number.isInteger(related.revision), `${label}: revision`).toBe(true);
    expect(Array.isArray(related.links), `${label}: links is an array, never null`).toBe(true);
    expect(typeof related.partial, `${label}: partial`).toBe('boolean');
    expect(related.basis && typeof related.basis.captures === 'number' && typeof related.basis.events === 'number' && typeof related.basis.mode === 'string', `${label}: basis`).toBe(true);
    expect(related.basis.events, `${label}: there is no event signal`).toBe(0);
    expect(Date.parse(related.generated_at), `${label}: generated_at`).not.toBeNaN();
    expect(related.links.length, `${label}: at most ${MAX_LINKS} links`).toBeLessThanOrEqual(MAX_LINKS);

    // 4. a story that cites sources has capture edges (publish path or backfill)
    if ((story.source_count || 0) > 0 && !related.partial && related.basis.mode !== 'off') {
      expect(related.basis.captures, `${label}: cites ${story.source_count} sources but basis.captures is 0; its cites_capture edges are missing (run the backfill)`).toBeGreaterThan(0);
    }

    // 3. the links
    const seen = new Set();
    for (const link of related.links) {
      const where = `${label} -> ${link.route?.bare}`;
      expect(link.route?.bare, `${where}: has a route`).toBeTruthy();
      expect(link.publication_id, `${where}: not the story itself`).not.toBe(story.publication_id);
      expect(link.route.bare, `${where}: not the story or a version of it`).not.toBe(story.route.bare);
      expect(slugOf(link.route.bare), `${where}: not a version of the story`).not.toBe(slugOf(story.route.bare));
      expect(frontRoutes.has(link.route.bare), `${where}: is on the front list`).toBe(true);
      expect(seen.has(link.publication_id), `${where}: listed once`).toBe(false);
      seen.add(link.publication_id);
      expect(['capture', 'source', 'entity'], `${where}: reason`).toContain(link.reason);
      expect(link.title, `${where}: title`).toBeTruthy();
      // 5. hub rule, read from outside
      if (link.reason === 'entity') {
        expect(link.shared?.entities, `${where}: shares at least one central entity`).toBeGreaterThanOrEqual(1);
      } else {
        expect(link.shared?.sources, `${where}: shares at least one source`).toBeGreaterThanOrEqual(1);
        expect(link.shared?.citers, `${where}: justified only by sources cited by at most ${HUB_STORIES} stories`).toBeLessThanOrEqual(HUB_STORIES);
      }
      pairs.push({
        from: story.route.bare, from_title: story.title, to: link.route.bare, to_title: link.title,
        reason: link.reason, shared: link.shared, from_basis: related.basis,
      });
    }
  }

  // 3. every link resolves (the story page the reader would open).
  const targets = [...new Set(pairs.map((p) => p.to))];
  for (const target of targets) {
    const res = await request.get(`${BASE_URL}/api/platform/publications/resolve?route=${encodeURIComponent(target)}&view=nav`, { timeout: 30_000 });
    expect(res.status(), `${target} resolves`).toBe(200);
  }

  // The artifact: the pairs a human reads for precision.
  const artifact = {
    base_url: BASE_URL,
    generated_at: new Date().toISOString(),
    stories: stories.length,
    stories_with_links: answers.filter((a) => a.related.links.length > 0).length,
    pairs,
    basis: answers.map((a) => ({ route: a.story.route.bare, links: a.related.links.length, ...a.related.basis, partial: a.related.partial })),
  };
  const body = JSON.stringify(artifact, null, 2);
  await testInfo.attach('related-pairs.json', { body, contentType: 'application/json' });
  if (ARTIFACT_DIR) {
    fs.mkdirSync(ARTIFACT_DIR, { recursive: true });
    fs.writeFileSync(nodePath.join(ARTIFACT_DIR, 'related-pairs.json'), body);
  }

  // 2. some story is linked; otherwise say why.
  const linked = answers.filter((a) => a.related.links.length > 0);
  const firstFive = answers.slice(0, 5).map((a) => `${a.story.route.bare} basis=${JSON.stringify(a.related.basis)} partial=${a.related.partial}`).join('\n');
  expect(linked.length, `no story has a related list; the first five stories' basis:\n${firstFive}`).toBeGreaterThan(0);
});

test('a story with links shows "More on this" after the article, and a click opens the linked story', async ({ page, request }) => {
  const { stories, answers } = await allRelated(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');
  const linked = answers.find((a) => a.related.links.length > 0);
  test.skip(!linked, 'no story has related links yet');
  const story = linked.story;
  const expectedCount = Math.min(MAX_LINKS, linked.related.links.length);

  const log = [];
  page.on('requestfinished', (req) => {
    const url = req.url();
    if (url.includes('/api/platform/publications/resolve') && !url.includes('view=nav')) log.push({ kind: 'resolve-finished', at: Date.now() });
  });
  page.on('request', (req) => {
    if (req.url().includes('/api/platform/publications/related')) log.push({ kind: 'related-started', at: Date.now() });
  });

  await page.goto(`${BASE_URL}/pub/texture/${slugOf(story.route.bare)}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  const reader = page.locator('[data-texture-published-reader]');
  await expect(reader).toBeVisible({ timeout: 30_000 });
  await expect(reader).toHaveAttribute('data-publication-route', story.route.bare, { timeout: 30_000 });
  const firstId = await reader.getAttribute('data-publication-id');

  const block = reader.locator('[data-texture-related]');
  await expect(block).toHaveCount(1, { timeout: 30_000 });
  const resolved = log.find((e) => e.kind === 'resolve-finished');
  const asked = log.find((e) => e.kind === 'related-started');
  expect(resolved && asked, `request log: ${JSON.stringify(log)}`).toBeTruthy();
  // A small tolerance: the browser reports a finished response and the
  // page's next request through separate events.
  expect(asked.at + 250, 'the related list is asked for after resolve answered').toBeGreaterThanOrEqual(resolved.at);

  // Scrolled to the end, the block sits below the last paragraph.
  await reader.evaluate((el) => { el.scrollTop = el.scrollHeight; });
  await expect(block).toBeInViewport();
  await expect(block.locator('h2')).toHaveText('More on this');
  const anchors = block.locator('[data-related-link]');
  const count = await anchors.count();
  expect(count, 'between 1 and 5 links').toBeGreaterThanOrEqual(1);
  expect(count).toBeLessThanOrEqual(MAX_LINKS);
  expect(count, 'the block shows what the route answered').toBe(expectedCount);
  const below = await reader.evaluate((el) => {
    const section = el.querySelector('[data-texture-related]');
    const last = [...el.querySelectorAll('p')].filter((p) => !section.contains(p)).pop();
    return !last || !!(last.compareDocumentPosition(section) & Node.DOCUMENT_POSITION_FOLLOWING);
  });
  expect(below, 'the block follows the last paragraph').toBe(true);
  await expectNoHorizontalScroll(page, 'story with related links');

  // A click moves to the linked story.
  const first = anchors.first();
  const href = await first.getAttribute('href');
  const target = linked.related.links[0];
  expect(href).toBe(`/pub/texture/${slugOf(target.route.bare)}`);
  await first.click();
  await expect(reader).toHaveAttribute('data-publication-route', target.route.bare, { timeout: 30_000 });
  expect(await reader.getAttribute('data-publication-id'), 'the reader moved to another publication').not.toBe(firstId);
  await expect(reader).not.toBeEmpty();
});

test('a story with no related links shows no block and no heading', async ({ page, request }) => {
  const { stories, answers } = await allRelated(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');
  const empty = answers.find((a) => a.related.links.length === 0 && !a.related.partial && a.related.basis.mode !== 'off');
  test.skip(!empty, 'every story has related links');

  await page.goto(`${BASE_URL}/pub/texture/${slugOf(empty.story.route.bare)}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  const reader = page.locator('[data-texture-published-reader]');
  await expect(reader).toBeVisible({ timeout: 30_000 });
  await expect(reader).toHaveAttribute('data-publication-route', empty.story.route.bare, { timeout: 30_000 });
  // Give the related request time to answer, then assert nothing was drawn.
  await page.waitForResponse((res) => res.url().includes('/api/platform/publications/related'), { timeout: 30_000 }).catch(() => null);
  await page.waitForTimeout(500);
  await expect(page.locator('[data-texture-related]')).toHaveCount(0);
  await expect(page.getByText('More on this')).toHaveCount(0);
});

// The entity loop's growth series, once per run.
let growth = null;
async function entityGrowth(request) {
  if (!growth) growth = await getJSON(request, '/api/platform/entities/growth?days=30');
  return growth;
}

function reviewedDays(g) {
  return (g.days || []).filter((d) => d.stories_reviewed > 0);
}

function sumOver(days, pick) {
  return days.reduce((n, d) => n + pick(d), 0);
}

test('the entity growth series has its shape, and converges after 14 reviewed days', async ({ request }, testInfo) => {
  const g = await entityGrowth(request);
  expect(Array.isArray(g.days) && g.days.length === 30, 'growth: 30 days').toBe(true);
  expect(Array.isArray(g.stoplist), 'growth: stoplist is an array').toBe(true);
  expect(g.lexicon && typeof g.lexicon.entities === 'number' && typeof g.lexicon.keys === 'number' && typeof g.lexicon.stopwords === 'number', 'growth: lexicon').toBe(true);
  for (const d of g.days) {
    expect(/^\d{4}-\d{2}-\d{2}$/.test(d.day), `growth day ${d.day}`).toBe(true);
    for (const k of ['stories_reviewed', 'minted', 'alias_sprawl', 'keywords_added', 'aliases_learned', 'stopwords_added', 'existing_unmatched']) {
      expect(typeof d[k], `growth ${d.day}: ${k}`).toBe('number');
    }
    for (const k of ['hit_rate', 'existing_unmatched_share', 'alias_sprawl_share', 'novelty_yes_rate']) {
      expect(d[k] >= 0 && d[k] <= 1, `growth ${d.day}: ${k} in [0, 1]`).toBe(true);
    }
  }
  const body = JSON.stringify(g, null, 2);
  await testInfo.attach('entity-growth.json', { body, contentType: 'application/json' });
  if (ARTIFACT_DIR) {
    fs.mkdirSync(ARTIFACT_DIR, { recursive: true });
    fs.writeFileSync(nodePath.join(ARTIFACT_DIR, 'entity-growth.json'), body);
  }

  const reviewed = reviewedDays(g);
  test.skip(reviewed.length < 14, `the entity loop has reviewed stories on ${reviewed.length} of the 14 days convergence needs`);
  const last7 = reviewed.slice(-7);
  const first7 = reviewed.slice(0, 7);
  const attachments = sumOver(last7, (d) => d.attachments.existing + d.attachments.extracted + d.attachments.research_named);
  const unmatched = sumOver(last7, (d) => d.existing_unmatched);
  const minted = sumOver(last7, (d) => d.minted);
  const sprawl = sumOver(last7, (d) => d.alias_sprawl);
  const stories = sumOver(last7, (d) => d.stories_reviewed);
  const noveltyYes = sumOver(last7, (d) => d.novelty_yes_rate * d.stories_reviewed);
  const perStory = (days) => sumOver(days, (d) => d.minted) / Math.max(1, sumOver(days, (d) => d.stories_reviewed));
  expect(attachments === 0 || unmatched / attachments <= 0.2, `existing_unmatched ${unmatched}/${attachments} over the last 7 days: the alias act is due`).toBe(true);
  expect(minted === 0 || sprawl / minted <= 0.15, `alias sprawl ${sprawl}/${minted} of the last 7 days' mints`).toBe(true);
  expect(noveltyYes / Math.max(1, stories) <= 0.3, `novelty "yes" rate ${(noveltyYes / Math.max(1, stories)).toFixed(2)} over the last 7 days: candidate recall is short (the NER escalation)`).toBe(true);
  expect(perStory(last7) < 0.8 * perStory(first7), `mints per story ${perStory(first7).toFixed(2)} then ${perStory(last7).toFixed(2)}: not falling`).toBe(true);
});

test('the share of front stories with a related link meets the floor once the entity loop runs', async ({ request }, testInfo) => {
  const { stories, answers } = await allRelated(request);
  test.skip(stories.length === 0, 'no Autopaper story is published yet');
  const linked = answers.filter((a) => a.related.links.length > 0).length;
  const share = linked / answers.length;
  const modes = {};
  for (const a of answers) modes[a.related.basis.mode] = (modes[a.related.basis.mode] || 0) + 1;
  const byReason = {};
  for (const a of answers) for (const l of a.related.links) byReason[l.reason] = (byReason[l.reason] || 0) + 1;
  const g = await entityGrowth(request);
  const report = { stories: answers.length, linked, share, floor: COVERAGE_FLOOR, modes, links_by_reason: byReason, reviewed_days: reviewedDays(g).length };
  await testInfo.attach('related-coverage.json', { body: JSON.stringify(report, null, 2), contentType: 'application/json' });
  console.log(`related coverage: ${linked}/${answers.length} = ${(share * 100).toFixed(1)}%; modes ${JSON.stringify(modes)}; links by reason ${JSON.stringify(byReason)}; entity loop reviewed on ${report.reviewed_days} days`);
  test.skip(report.reviewed_days < 7, `the entity loop has reviewed stories on ${report.reviewed_days} of the 7 days the floor waits for`);
  expect(share, `only ${linked} of ${answers.length} front stories have a related link (floor ${COVERAGE_FLOOR}); modes ${JSON.stringify(modes)}`).toBeGreaterThanOrEqual(COVERAGE_FLOOR);
});
