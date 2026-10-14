/**
 * The Autopaper front page as a broadsheet front page (owner request
 * 2026-10-14: "make a newspaper style ui, rather than the newsfeed").
 *
 * Serves the built or dev frontend with the public front list mocked: 14
 * stories on the first page (zh, ja, ko and ar headlines among them, version
 * counts from 1 to 7, updates from minutes to days old) and 3 more behind
 * next_cursor. Asserts:
 *   - the masthead (h1 "Autopaper" over a double rule, the edition line);
 *   - exactly one lead story (the only h2), and it is the top-ranked story,
 *     a developed story rather than merely the newest; every other headline
 *     is an h3, in rank order;
 *   - four columns at 1280px, one column at 390px with a 16px gutter and no
 *     horizontal overflow, and no headline (CJK, Arabic, a long unbroken
 *     word) wider than its column;
 *   - every headline links to /autopaper/<slug>, every version line to
 *     /autopaper/<slug>/v<n>, and a headline click opens the story page;
 *   - "Older stories" appears with next_cursor and pages in the rest;
 *   - the empty and error states sit under the masthead;
 *   - the in-desktop Autopaper window renders the same front page.
 * The rank rule itself (rankFrontStories in src/lib/autopaper.ts) is checked
 * directly in the first describe block; the repo has no TS unit runner, and
 * Playwright loads the TypeScript module as is.
 *
 * Screenshots at 1280px and 390px, in the dark default theme and the light
 * London Salmon theme, are written to AUTOPAPER_UI_SHOT_DIR when set (else
 * the test output dir) and attached to the report.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   AUTOPAPER_UI_SHOT_DIR=/private/tmp/autopaper-ui \
 *     npx playwright test tests/autopaper-front-page.spec.js
 *
 * PLAYWRIGHT_BASE_URL overrides the config's http://localhost:4173.
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { expect, test } from '@playwright/test';
import {
  FRONT_FRESH_WINDOW_MS,
  FRONT_MAX_LIFTED_VERSIONS,
  FRONT_VERSION_LIFT_MS,
  layoutFrontPage,
  rankFrontStories,
} from '../src/lib/autopaper.ts';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const SHOT_DIR = process.env.AUTOPAPER_UI_SHOT_DIR || '';
const MIN = 60 * 1000;
const HOUR = 60 * MIN;

function story(id, title, dek, ageMs, versions, now) {
  const slug = `story-${id}`;
  return {
    publication_id: `pub-${id}`,
    title,
    dek,
    updated_at: new Date(now - ageMs).toISOString(),
    version_count: versions,
    latest_revision: versions,
    route: { bare: `/pub/texture/${slug}`, permalink: `/pub/texture/${slug}/v${versions}` },
  };
}

function fixture(now = Date.now()) {
  const first = [
    story('harbor', 'Harbor fire contained after crews work through the night', 'Port operations resume on two of five berths; the cause is still under investigation.', 10 * MIN, 1, now),
    story('storm', 'Storm surge reaches the coast as evacuation zone widens', 'Officials extended the order to three more districts after the overnight forecast shifted the landfall north.', 60 * MIN, 3, now),
    story('zh-transit', '城市议会通过交通预算，地铁延长线明年动工', '预算案以七票对两票通过，新线路将连接东部新区与市中心。', 95 * MIN, 2, now),
    story('ja-rice', '米価格が三か月連続で上昇、政府は備蓄米の放出を検討', '農林水産省は来週にも放出量を決める方針で、店頭価格への影響が注目される。', 2 * HOUR, 1, now),
    story('ko-chip', '반도체 수출 반등, 3분기 무역수지 흑자 전환', '메모리 가격 회복이 수출을 이끌었지만 일부 품목은 여전히 부진하다.', 3 * HOUR, 4, now),
    story('ar-water', 'اتفاق إقليمي جديد لتقاسم المياه بعد مفاوضات استمرت عامين', 'يحدد الاتفاق حصصاً موسمية ويُنشئ لجنة مشتركة لمراقبة التدفقات.', 4.5 * HOUR, 5, now),
    story('court', 'Appeals court sends the zoning case back for a new hearing', 'The ruling faults the lower court for skipping the environmental review record.', 5 * HOUR, 1, now),
    story('vaccine', 'Regional clinics extend hours for the autumn vaccine drive', 'Walk-in slots open Saturday at twelve sites.', 7 * HOUR, 2, now),
    story('library', 'The library that lends seeds: a season of borrowed tomatoes', 'Gardeners return a share of the harvest so the collection grows each year.', 9 * HOUR, 1, now),
    story('longword', 'Supercalifragilisticexpialidociouslylonghyphenlessheadlinewordthatmustwrap without overflowing its column', 'A deliberately long unbroken word.', 12 * HOUR, 1, now),
    story('election', 'Turnout in the municipal runoff was the lowest in two decades', 'Fewer than one in four registered voters cast a ballot.', 20 * HOUR, 7, now),
    story('rail', 'Night trains return to the northern line after a four-year pause', 'Sleeper cars run three nights a week through the winter.', 26 * HOUR, 1, now),
    story('museum', 'Museum returns carved panels to their community of origin', 'The handover follows a provenance review that began in 2023.', 30 * HOUR, 3, now),
    story('budget-ja', '東京都、来年度予算案を発表', '', 40 * HOUR, 1, now),
  ];
  const second = [
    story('bridge', 'Bridge inspection finds corrosion on two support cables', 'Lanes stay open while engineers schedule repairs.', 50 * HOUR, 2, now),
    story('orchestra', 'Youth orchestra tours the river towns', '', 60 * HOUR, 1, now),
    story('ko-weather', '첫눈 예보, 중부지방 최저기온 영하권', '', 70 * HOUR, 1, now),
  ];
  return { first, second };
}

function slugOf(route) {
  return String(route || '').replace(/^\/pub\/texture\//, '').split('/')[0];
}

async function mockFront(page, { first, second }, { status = 200, empty = false } = {}) {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
  );
  await page.route('**/api/platform/publications/front?*', (route) => {
    if (status !== 200) return route.fulfill({ status, contentType: 'application/json', body: '{"error":"down"}' });
    const url = new URL(route.request().url());
    const cursor = url.searchParams.get('cursor');
    const body = empty
      ? { stories: [] }
      : cursor === 'page-2'
        ? { stories: second }
        : { stories: first, next_cursor: 'page-2' };
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

async function useTheme(page, id) {
  if (!id) return;
  await page.addInitScript((themeId) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify({ schema_version: 2, id: themeId }));
  }, id);
}

async function shoot(page, testInfo, name) {
  const dir = SHOT_DIR || testInfo.outputDir;
  fs.mkdirSync(dir, { recursive: true });
  const view = page.viewportSize();
  const viewPath = nodePath.join(dir, `${name}.png`);
  await page.screenshot({ path: viewPath });
  await testInfo.attach(name, { path: viewPath, contentType: 'image/png' });
  // The page scrolls inside [data-autopaper]; grow the viewport to capture all of it.
  const height = await page.locator('[data-autopaper]').evaluate((el) => el.scrollHeight);
  if (view && height > view.height) {
    await page.setViewportSize({ width: view.width, height: Math.min(height, 6000) });
    const fullPath = nodePath.join(dir, `${name}-full.png`);
    await page.screenshot({ path: fullPath });
    await testInfo.attach(`${name}-full`, { path: fullPath, contentType: 'image/png' });
    await page.setViewportSize(view);
  }
}

async function openFront(page, viewport, fx, theme = '') {
  await page.setViewportSize(viewport);
  await useTheme(page, theme);
  await mockFront(page, fx);
  await page.goto(`${BASE_URL}/autopaper`);
  await expect(page.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 20_000 });
}

async function cellLefts(page) {
  return page.locator('[data-autopaper-grid] > .ap-cell').evaluateAll((cells) =>
    cells.map((cell) => Math.round(cell.getBoundingClientRect().left)),
  );
}

async function expectNoHorizontalOverflow(page, label) {
  const boxes = await page.evaluate(() =>
    [document.scrollingElement, document.querySelector('[data-autopaper]')]
      .filter(Boolean)
      .map((el) => ({ scroll: el.scrollWidth, client: el.clientWidth })),
  );
  for (const { scroll, client } of boxes) {
    expect(scroll, `${label}: horizontal overflow (${scroll} > ${client})`).toBeLessThanOrEqual(client);
  }
  const wide = await page.locator('[data-autopaper-headline]').evaluateAll((heads) =>
    heads
      .filter((h) => h.scrollWidth > h.clientWidth + 1 || h.getBoundingClientRect().right > document.documentElement.clientWidth)
      .map((h) => h.textContent),
  );
  expect(wide, `${label}: headlines wider than their column`).toEqual([]);
}

test.describe('front-page rank', () => {
  const now = Date.parse('2026-10-14T12:00:00Z');

  test('newest first when no story has more than one version', () => {
    const list = [
      story('b', 'B', '', 2 * HOUR, 1, now),
      story('a', 'A', '', 1 * HOUR, 1, now),
      story('c', 'C', '', 3 * HOUR, 1, now),
    ];
    expect(rankFrontStories(list).map((s) => s.title)).toEqual(['A', 'B', 'C']);
  });

  test('a developed story updated within the fresh window outranks a one-version story of similar age', () => {
    const fresh = story('fresh', 'Fresh', '', 0, 1, now);
    const developed = story('developed', 'Developed', '', 50 * MIN, 3, now);
    expect(rankFrontStories([fresh, developed]).map((s) => s.title)).toEqual(['Developed', 'Fresh']);
    // One extra version is worth one lift; an hour behind with one extra version still trails.
    const oneMore = story('one-more', 'One more', '', 60 * MIN, 2, now);
    expect(rankFrontStories([fresh, oneMore]).map((s) => s.title)).toEqual(['Fresh', 'One more']);
  });

  test('versions lift only inside the fresh window, and only up to the cap', () => {
    const edition = story('edition', 'Edition', '', 0, 1, now);
    const stale = story('stale', 'Stale', '', FRONT_FRESH_WINDOW_MS + MIN, 9, now);
    const older = story('older', 'Older', '', FRONT_FRESH_WINDOW_MS + 2 * MIN, 1, now);
    expect(rankFrontStories([older, stale, edition]).map((s) => s.title)).toEqual(['Edition', 'Stale', 'Older']);

    const capped = story('capped', 'Capped', '', FRONT_MAX_LIFTED_VERSIONS * FRONT_VERSION_LIFT_MS + MIN, 50, now);
    expect(rankFrontStories([capped, edition]).map((s) => s.title)).toEqual(['Edition', 'Capped']);
  });

  test('the rank is deterministic: ties break on update time, then publication id, whatever the input order', () => {
    const x = story('x', 'X', '', HOUR, 1, now);
    const y = story('y', 'Y', '', HOUR, 1, now);
    // Same score as X and Y (one lift earlier), but an older update: ranks after them.
    const z = story('z', 'Z', '', HOUR + FRONT_VERSION_LIFT_MS, 2, now);
    const orders = [[x, y, z], [z, y, x], [y, z, x]];
    for (const order of orders) {
      expect(rankFrontStories(order).map((s) => s.title)).toEqual(['X', 'Y', 'Z']);
    }
    const bad = { ...story('bad', 'Bad', '', 0, 1, now), updated_at: 'not a date' };
    expect(rankFrontStories([bad, x]).map((s) => s.title)).toEqual(['X', 'Bad']);
    expect(rankFrontStories([])).toEqual([]);
  });

  test('the layout cuts lead, four top stories, six briefs, and the rest', () => {
    const { first } = fixture(now);
    const ranked = rankFrontStories(first);
    const layout = layoutFrontPage(ranked);
    expect(layout.lead).toBe(ranked[0]);
    expect(layout.top).toEqual(ranked.slice(1, 5));
    expect(layout.briefs).toEqual(ranked.slice(5, 11));
    expect(layout.more).toEqual(ranked.slice(11));
    expect(layoutFrontPage([])).toEqual({ lead: null, top: [], briefs: [], more: [] });
  });
});

test.describe('Autopaper front page', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('broadsheet layout at 1280px: masthead, one lead, four columns, rank order, links', async ({ page }, testInfo) => {
    const fx = fixture();
    const ranked = rankFrontStories(fx.first);
    // The lead is the developed storm story, not the newest one.
    expect(ranked[0].publication_id).toBe('pub-storm');
    expect(fx.first[0].publication_id).toBe('pub-harbor');

    await openFront(page, { width: 1280, height: 900 }, fx);
    const front = page.locator('[data-autopaper]');

    // Masthead.
    const masthead = front.locator('header[data-autopaper-masthead]');
    await expect(masthead.locator('h1')).toHaveText('Autopaper');
    await expect(masthead.locator('[data-autopaper-edition]')).toContainText('Living stories with checked sources');
    await expect(masthead.locator('[data-autopaper-updated]')).toContainText(/Updated\s+\d+ min ago/);
    const rule = await masthead.locator('h1').evaluate((el) => getComputedStyle(el).borderBottomStyle);
    expect(rule).toBe('double');
    const nameplateFont = await masthead.locator('h1').evaluate((el) => getComputedStyle(el).fontFamily);
    expect(nameplateFont).toMatch(/serif/i);

    // Headings: one h1, one h2 (the lead), every other headline an h3.
    await expect(front.locator('h1')).toHaveCount(1);
    await expect(front.locator('h2')).toHaveCount(1);
    await expect(front.locator('[data-autopaper-lead] h2')).toHaveText(ranked[0].title);
    await expect(front.locator('h3[data-autopaper-headline]')).toHaveCount(fx.first.length - 1);

    // Headlines appear in rank order, each linking to its story page.
    const headlines = front.locator('[data-autopaper-headline]');
    await expect(headlines).toHaveText(ranked.map((s) => s.title));
    const hrefs = await headlines.locator('a').evaluateAll((links) => links.map((a) => a.getAttribute('href')));
    expect(hrefs).toEqual(ranked.map((s) => `/autopaper/${slugOf(s.route.bare)}`));
    const versionHrefs = await front.locator('[data-autopaper-version-link]').evaluateAll((links) => links.map((a) => a.getAttribute('href')));
    expect(versionHrefs).toEqual(ranked.map((s) => `/autopaper/${slugOf(s.route.bare)}/v${s.latest_revision}`));
    await expect(front.locator('[data-updated-marker]')).toHaveCount(fx.first.filter((s) => s.version_count > 1).length);

    // Tiers: deks for the lead and top tier, none for briefs and more.
    await expect(front.locator('[data-autopaper-tier="top"]')).toHaveCount(4);
    await expect(front.locator('[data-autopaper-tier="brief"]')).toHaveCount(6);
    await expect(front.locator('[data-autopaper-tier="more"]')).toHaveCount(fx.first.length - 11);
    await expect(front.locator('[data-autopaper-tier="brief"] .ap-dek')).toHaveCount(0);
    await expect(front.locator('[data-autopaper-tier="lead"] .ap-dek')).toHaveText(ranked[0].dek);

    // Four columns. Row one: the lead over two columns, top stories 1 and 2
    // in columns 3 and 4. Row two: top stories 3 and 4, then the briefs.
    const grid = page.locator('[data-autopaper-grid]');
    const columns = await grid.evaluate((el) => {
      const style = getComputedStyle(el);
      return { tracks: style.gridTemplateColumns.split(' ').map(parseFloat), gap: parseFloat(style.columnGap) };
    });
    expect(columns.tracks).toHaveLength(4);
    const lefts = await cellLefts(page);
    expect(new Set(lefts).size).toBeGreaterThanOrEqual(3);
    const leadBox = await page.locator('[data-autopaper-lead]').boundingBox();
    expect(Math.abs(leadBox.width - (columns.tracks[0] + columns.tracks[1] + columns.gap))).toBeLessThanOrEqual(2);
    const topBoxes = await page.locator('.ap-cell-top').evaluateAll((cells) =>
      cells.map((cell) => {
        const r = cell.getBoundingClientRect();
        return { x: r.x, y: r.y, width: r.width };
      }),
    );
    expect(topBoxes).toHaveLength(4);
    // Top stories 1 and 2 share the lead's row, one column each, right of it.
    expect(topBoxes[0].x).toBeGreaterThan(leadBox.x + leadBox.width);
    expect(topBoxes[1].x).toBeGreaterThan(topBoxes[0].x + topBoxes[0].width);
    for (const box of topBoxes.slice(0, 2)) {
      expect(Math.abs(box.y - leadBox.y)).toBeLessThanOrEqual(1);
      expect(Math.abs(box.width - columns.tracks[2])).toBeLessThanOrEqual(2);
    }
    // Top stories 3 and 4 start the next row, then the briefs.
    expect(topBoxes[2].y).toBeGreaterThanOrEqual(leadBox.y + leadBox.height);
    expect(Math.abs(topBoxes[2].x - leadBox.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(topBoxes[3].y - topBoxes[2].y)).toBeLessThanOrEqual(1);
    const briefsBox = await page.locator('[data-autopaper-briefs]').boundingBox();
    expect(Math.abs(briefsBox.y - topBoxes[2].y)).toBeLessThanOrEqual(1);
    expect(briefsBox.x).toBeGreaterThan(topBoxes[3].x);
    const leadSize = await page.locator('[data-autopaper-lead] h2').evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    const topSize = await page.locator('.ap-cell-top h3').first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    const briefSize = await page.locator('[data-autopaper-tier="brief"] h3').first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    expect(leadSize).toBeGreaterThan(topSize);
    expect(topSize).toBeGreaterThan(briefSize);

    // Multilingual headlines render in their columns; Arabic runs right to left.
    const arabic = front.locator('[data-publication-route="/pub/texture/story-ar-water"] [data-autopaper-headline]');
    expect(await arabic.evaluate((el) => getComputedStyle(el).direction)).toBe('rtl');
    await expectNoHorizontalOverflow(page, '1280px');

    // Focus is visible on a headline link.
    await page.locator('[data-autopaper-lead] h2 a').focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    const focus = await page.locator('[data-autopaper-lead] h2 a').evaluate((el) => getComputedStyle(el).boxShadow);
    expect(focus).toContain('inset');
    await page.locator('[data-autopaper-lead] h2 a').blur();

    await shoot(page, testInfo, 'autopaper-front-1280-dark');

    // Older stories pages in the rest, below More stories.
    const older = page.locator('[data-autopaper-more]');
    await expect(older).toHaveText('Older stories');
    await older.click();
    await expect(front.locator('[data-autopaper-tier="more"]')).toHaveCount(fx.first.length - 11 + fx.second.length);
    await expect(older).toHaveCount(0);
    await expect(front.locator('h2')).toHaveCount(1);

    // A headline opens the story page at /autopaper/<slug>.
    await page.locator('[data-autopaper-lead] h2 a').click();
    await expect(page).toHaveURL(`${BASE_URL}/autopaper/story-storm`);
    await expect(page.locator('[data-autopaper-route]')).toHaveAttribute('data-autopaper-view', 'story');
  });

  test('one column at 390px with a 16px gutter and no horizontal scroll', async ({ page }, testInfo) => {
    const fx = fixture();
    const ranked = rankFrontStories(fx.first);
    await openFront(page, { width: 390, height: 844 }, fx);

    const tracks = await page.locator('[data-autopaper-grid]').evaluate((el) => getComputedStyle(el).gridTemplateColumns.split(' ').length);
    expect(tracks).toBe(1);
    const lefts = await cellLefts(page);
    expect(new Set(lefts).size).toBe(1);
    // The lead comes first.
    const first = page.locator('[data-autopaper-grid] > .ap-cell').first();
    await expect(first).toHaveAttribute('data-autopaper-lead', '');
    await expect(first.locator('h2')).toHaveText(ranked[0].title);
    const gutter = await page.locator('[data-autopaper-masthead] h1').evaluate((el) => Math.round(el.getBoundingClientRect().left));
    expect(gutter).toBe(16);
    await expectNoHorizontalOverflow(page, '390px');
    await expect(page.locator('[data-autopaper-more]')).toBeVisible();

    await shoot(page, testInfo, 'autopaper-front-390-dark');
  });

  test('light theme screenshots map paper and ink to the theme', async ({ page }, testInfo) => {
    const fx = fixture();
    await openFront(page, { width: 1280, height: 900 }, fx, 'london-salmon');
    await expect(page.locator('[data-theme-id]').first()).toHaveAttribute('data-theme-id', 'london-salmon');
    const colors = await page.locator('[data-autopaper]').evaluate((el) => {
      const style = getComputedStyle(el);
      return { paper: style.backgroundColor, ink: style.color };
    });
    expect(colors.paper).not.toBe(colors.ink);
    await shoot(page, testInfo, 'autopaper-front-1280-light');

    await page.setViewportSize({ width: 390, height: 844 });
    await expectNoHorizontalOverflow(page, '390px light');
    await shoot(page, testInfo, 'autopaper-front-390-light');
  });

  test('two columns at tablet width', async ({ page }) => {
    await openFront(page, { width: 820, height: 1000 }, fixture());
    const tracks = await page.locator('[data-autopaper-grid]').evaluate((el) => getComputedStyle(el).gridTemplateColumns.split(' ').length);
    expect(tracks).toBe(2);
    await expectNoHorizontalOverflow(page, '820px');
  });

  test('empty and error states sit under the masthead', async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await mockFront(page, fixture(), { empty: true });
    await page.goto(`${BASE_URL}/autopaper`);
    await expect(page.locator('[data-autopaper-masthead] h1')).toHaveText('Autopaper');
    await expect(page.locator('[data-autopaper-empty]')).toHaveText('No stories yet.');
    await expect(page.locator('[data-autopaper-updated]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper] h2')).toHaveCount(0);
    await shoot(page, testInfo, 'autopaper-front-390-empty');

    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await mockFront(page, fixture(), { status: 503 });
    await page.goto(`${BASE_URL}/autopaper`);
    await expect(page.locator('[data-autopaper-error]')).toContainText('Could not load stories');
    await expect(page.locator('[data-autopaper-masthead] h1')).toHaveText('Autopaper');
  });

  test('the in-desktop Autopaper window renders the same front page', async ({ page }, testInfo) => {
    const fx = fixture();
    const ranked = rankFrontStories(fx.first);
    await page.setViewportSize({ width: 1280, height: 900 });
    await mockFront(page, fx);
    await page.goto(`${BASE_URL}/?intro=0`);
    const win = page.locator('[data-window][data-window-id="public-preview-autopaper"]');
    await expect(win).toBeVisible({ timeout: 20_000 });
    await expect(win.locator('[data-autopaper-masthead] h1')).toHaveText('Autopaper');
    await expect(win.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 20_000 });
    await expect(win.locator('h2')).toHaveCount(1);
    await expect(win.locator('[data-autopaper-lead] h2')).toHaveText(ranked[0].title);
    await expect(win.locator('[data-autopaper-headline]')).toHaveText(ranked.map((s) => s.title));
    // Let the window's opening animation finish before the screenshot.
    await win.evaluate((el) =>
      Promise.all(
        el.getAnimations({ subtree: true })
          .filter((a) => Number.isFinite(a.effect?.getComputedTiming?.().endTime))
          .map((a) => a.finished.catch(() => {})),
      ),
    );
    await shoot(page, testInfo, 'autopaper-desktop-window-1280-dark');
  });
});
