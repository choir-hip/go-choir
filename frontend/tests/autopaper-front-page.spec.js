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
 *   - four columns in a wide desktop window, one column at 390px with a 16px gutter and no
 *     horizontal overflow, and no headline (CJK, Arabic, a long unbroken
 *     word) wider than its column;
 *   - every headline links to /pub/texture/<slug>, every version line to
 *     /pub/texture/<slug>/v<n>, and a headline click opens the story in a
 *     Texture window;
 *   - "Older stories" appears with next_cursor and pages in the rest;
 *   - the empty and error states sit under the masthead;
 *   - the logged-out home opens straight to Autopaper: no intro deck, the
 *     Autopaper window open on the desktop, in the London Salmon default
 *     theme (owner request 2026-10-14); the window renders the same front
 *     page.
 *   - sections (owner, 2026-10-14, LF1): with at least half the stories
 *     filed under a section, a row of five section names sits under the
 *     masthead (Front page current by default); a section shows only its
 *     stories in rank order; the chosen section is restored after a reload;
 *     section choices counted on the device lift those sections on the front
 *     page; with fewer than half filed the row is absent; at 375px the row
 *     scrolls inside itself with no page overflow.
 *   - section pages (2026-10-15): a section is fetched from the server
 *     (`?section=<id>&limit=60`), not filtered from the front page's rows,
 *     so a quiet section still lists its older stories; "Older stories"
 *     pages the section's own cursor; "No stories in this section yet."
 *     shows only when the server returns none.
 * The rank rule itself (rankFrontStories in src/lib/autopaper.ts) is checked
 * directly in the first describe block; the repo has no TS unit runner, and
 * Playwright loads the TypeScript module as is.
 *
 * Section screenshots (375px and 1440px) go to AUTOPAPER_UI_SHOT_DIR too.
 * Screenshots at 1280px and 390px, in the London Salmon default theme and
 * the dark Futuristic Noir theme, are written to AUTOPAPER_UI_SHOT_DIR when set (else
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
  SECTION_STORAGE_KEY,
  SECTION_VISITS_STORAGE_KEY,
  depthFactor,
  layoutFrontPage,
  loadSavedSection,
  loadSectionVisits,
  rankFrontStories,
  recordSectionVisit,
  saveSection,
  sectionShares,
  storiesInSection,
  usesSections,
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

// A section request (`?section=<id>`) is answered by `sectionBody(section,
// cursor)` when given, else with every fixture story filed under that
// section in one page, as corpusd does for a short section. Every request's
// query string is pushed onto `requests`.
async function mockFront(page, { first, second }, { status = 200, empty = false, sectionBody = null, requests = null } = {}) {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
  );
  await page.route('**/api/platform/publications/front?*', (route) => {
    if (status !== 200) return route.fulfill({ status, contentType: 'application/json', body: '{"error":"down"}' });
    const url = new URL(route.request().url());
    const cursor = url.searchParams.get('cursor');
    const section = url.searchParams.get('section');
    if (requests) requests.push(url.search);
    if (section) {
      const body = sectionBody
        ? sectionBody(section, cursor || '')
        : { stories: [...first, ...second].filter((s) => s.section === section) };
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    }
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

async function openFront(page, viewport, fx, theme = '', mockOptions = {}) {
  await page.setViewportSize(viewport);
  await useTheme(page, theme);
  await mockFront(page, fx, mockOptions);
  await page.goto(`${BASE_URL}/`);
  await expect(page.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 20_000 });
  // The desktop window opens with a short scale animation; measure after it.
  await page.waitForTimeout(500);
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

  const scored = (s, importance, promo) => ({ ...s, importance, ...(promo === undefined ? {} : { promo }) });

  test('with at least half scored, importance times a 24 h recency decay orders the stories', () => {
    // Serious news from 20 h ago (9 * 0.56 = 5.1) leads fresher minor news (3 * 1 = 3).
    const serious = scored(story('serious', 'Serious', '', 20 * HOUR, 1, now), 9);
    const minor = scored(story('minor', 'Minor', '', 0, 1, now), 3);
    // Unscored counts as 4: fresh and unscored (4) sits above minor, below serious.
    const unscored = story('unscored', 'Unscored', '', 0, 1, now);
    // Two days old at 9 decays to 2.25: below minor.
    const old = scored(story('old', 'Old', '', 48 * HOUR, 1, now), 9);
    expect(rankFrontStories([minor, unscored, old, serious]).map((s) => s.title)).toEqual([
      'Serious',
      'Unscored',
      'Minor',
      'Old',
    ]);
    // Without scores the same stories rank by recency (today's rule).
    const plain = [minor, unscored, old, serious].map(({ importance, ...rest }) => rest);
    expect(rankFrontStories(plain).map((s) => s.title)).toEqual(['Minor', 'Unscored', 'Serious', 'Old']);
  });

  test('promotional stories and leaked "Open a story" titles never lead or sit in the top tier', () => {
    const list = [
      scored(story('promo', 'New phone launches with three cameras', '', 0, 3, now), 9, true),
      scored(story('leak', 'Open a story on the harbor fire', '', 0, 1, now), 8),
      scored(story('leak2', 'open A STORY about rail', '', 0, 1, now), 7, false),
    ];
    for (let i = 0; i < 11; i += 1) list.push(scored(story(`n${i}`, `News ${i}`, '', (i + 1) * HOUR, 1, now), 5));
    const ranked = rankFrontStories(list);
    const layout = layoutFrontPage(ranked);
    const demoted = ['New phone launches with three cameras', 'Open a story on the harbor fire', 'open A STORY about rail'];
    expect(ranked).toHaveLength(list.length);
    for (const s of [layout.lead, ...layout.top, ...layout.briefs]) expect(demoted).not.toContain(s.title);
    expect(layout.more.map((s) => s.title).slice(-3)).toEqual(demoted);
    expect(layout.lead.title).toBe('News 0');
  });

  test('with fewer than half scored, the rank is exactly the recency rank', () => {
    const { first } = fixture(now);
    const baseline = rankFrontStories(first).map((s) => s.publication_id);
    // Score fewer than half (6 of 14), including a promo story, in reverse
    // recency order so an importance rank would visibly differ.
    const partly = first.map((s, i) => (i >= first.length - 6 ? scored(s, 10, i === first.length - 1) : s));
    expect(rankFrontStories(partly).map((s) => s.publication_id)).toEqual(baseline);
    // At half scored the importance rank applies and the order changes.
    const half = first.map((s, i) => (i >= first.length / 2 ? scored(s, 10) : s));
    expect(rankFrontStories(half).map((s) => s.publication_id)).not.toEqual(baseline);
  });

  const deep = (s, importance, source_count, words) => ({ ...s, importance, source_count, words });

  test('depth weighs importance: one source halves it, two weigh 0.75, a short body 0.8, unknown counts in full', () => {
    const base = story('x', 'X', '', 0, 1, now);
    expect(depthFactor(deep(base, 5, 3, 400))).toBe(1);
    expect(depthFactor(deep(base, 5, 2, 400))).toBe(0.75);
    expect(depthFactor(deep(base, 5, 1, 400))).toBe(0.5);
    expect(depthFactor(deep(base, 5, 0, 400))).toBe(0.5);
    expect(depthFactor(deep(base, 5, 3, 121))).toBeCloseTo(0.8);
    expect(depthFactor(deep(base, 5, 1, 121))).toBeCloseTo(0.4);
    expect(depthFactor(base)).toBe(1);
    // The Mauritius case: importance 8, one source, 121 words (8 x 0.4 = 3.2)
    // falls below a researched importance-6 story of the same hour (6).
    const stub = deep(story('stub', 'Stub', '', 0, 1, now), 8, 1, 121);
    const researched = deep(story('deep', 'Researched', '', 20 * MIN, 3, now), 6, 5, 600);
    const plain = deep(story('plain', 'Plain', '', 10 * MIN, 1, now), 3, 3, 300);
    expect(rankFrontStories([stub, researched, plain]).map((s) => s.title)).toEqual(['Researched', 'Stub', 'Plain']);
    // Without depth fields the same stories rank as before (importance x recency).
    const bare = [stub, researched, plain].map(({ source_count, words, ...rest }) => rest);
    expect(rankFrontStories(bare).map((s) => s.title)).toEqual(['Stub', 'Researched', 'Plain']);
  });

  test('a story citing fewer than two sources never leads, but keeps its place in the top tier', () => {
    // Importance 10 with one long source still outscores the rest (10 x 0.5 = 5).
    const big = deep(story('big', 'Big one-source', '', 0, 1, now), 10, 1, 500);
    const two = deep(story('two', 'Two sources', '', 30 * MIN, 1, now), 6, 2, 500);
    const three = deep(story('three', 'Three sources', '', HOUR, 1, now), 4, 3, 500);
    const ranked = rankFrontStories([three, big, two]);
    expect(ranked.map((s) => s.title)).toEqual(['Two sources', 'Big one-source', 'Three sources']);
    const layout = layoutFrontPage(ranked);
    expect(layout.lead.title).toBe('Two sources');
    expect(layout.top[0].title).toBe('Big one-source');
    // Unknown depth may lead; with no story eligible the order stands.
    const unknown = { ...story('unknown', 'Unknown depth', '', 2 * HOUR, 1, now), importance: 3 };
    expect(rankFrontStories([big, unknown])[0].title).toBe('Unknown depth');
    const lone = deep(story('lone', 'Lone', '', 0, 1, now), 2, 0, 50);
    expect(rankFrontStories([lone, big]).map((s) => s.title)).toEqual(['Big one-source', 'Lone']);
    // Promo stories still never lead.
    const promo = { ...deep(story('promo', 'Promo', '', 0, 1, now), 9, 5, 500), promo: true };
    expect(rankFrontStories([promo, big, two])[0].title).toBe('Two sources');
  });

  test('sections: filed stories, filtering, the half rule, and the reader habit lift', () => {
    const filed = (s, importance, section) => ({ ...s, importance, section });
    const world = filed(story('w', 'World story', '', 0, 1, now), 6, 'world');
    const tech = filed(story('t', 'Tech story', '', 0, 1, now), 5, 'tech_ai');
    const biz = filed(story('b', 'Business story', '', 0, 1, now), 4, 'business');
    const loose = { ...story('l', 'Unfiled story', '', 0, 1, now), importance: 3 };
    const list = [world, tech, biz, loose];
    expect(usesSections(list)).toBe(true);
    expect(usesSections([world, loose, { ...loose, publication_id: 'pub-l2' }])).toBe(false);
    expect(usesSections([{ ...world, section: 'sports' }, loose])).toBe(false);
    expect(storiesInSection(list, 'front')).toEqual(list);
    expect(storiesInSection(list, 'tech_ai')).toEqual([tech]);
    expect(storiesInSection(list, 'nonsense')).toEqual(list);
    // Habit: all visits to Tech & AI lift it by 1.25 (5 x 1.25 = 6.25 > 6).
    expect(rankFrontStories(list).map((s) => s.title)[0]).toBe('World story');
    const shares = sectionShares({ tech_ai: 4 });
    expect(rankFrontStories(list, { sectionShares: shares }).map((s) => s.title)).toEqual([
      'Tech story',
      'World story',
      'Business story',
      'Unfiled story',
    ]);
    // A split habit lifts less: half the visits lift by 1.125 (5.625 < 6).
    const split = sectionShares({ tech_ai: 2, business: 2 });
    expect(rankFrontStories(list, { sectionShares: split })[0].title).toBe('World story');
    expect(sectionShares({})).toBeNull();
  });

  test('section memory and visit counts live in local storage and survive a blocked store', () => {
    const data = new Map();
    const store = { getItem: (k) => (data.has(k) ? data.get(k) : null), setItem: (k, v) => data.set(k, String(v)) };
    expect(loadSavedSection(store)).toBe('front');
    saveSection('tech_ai', store);
    expect(loadSavedSection(store)).toBe('tech_ai');
    saveSection('sports', store);
    expect(data.get(SECTION_STORAGE_KEY)).toBe('tech_ai');
    recordSectionVisit('tech_ai', store);
    recordSectionVisit('tech_ai', store);
    recordSectionVisit('front', store);
    expect(recordSectionVisit('world', store)).toEqual({ tech_ai: 2, world: 1 });
    expect(JSON.parse(data.get(SECTION_VISITS_STORAGE_KEY))).toEqual({ tech_ai: 2, world: 1 });
    data.set(SECTION_STORAGE_KEY, 'nonsense');
    data.set(SECTION_VISITS_STORAGE_KEY, '{bad json');
    expect(loadSavedSection(store)).toBe('front');
    expect(loadSectionVisits(store)).toEqual({});
    const blocked = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    };
    expect(loadSavedSection(blocked)).toBe('front');
    expect(() => saveSection('world', blocked)).not.toThrow();
    expect(recordSectionVisit('world', blocked)).toEqual({ world: 1 });
    expect(loadSavedSection(null)).toBe('front');
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

// The fixture filed into sections and scored, as corpusd serves it once the
// scorer has run: every story but the last carries a section.
const SECTION_OF = {
  harbor: 'world',
  storm: 'world',
  'zh-transit': 'world',
  'ja-rice': 'business',
  'ko-chip': 'tech_ai',
  'ar-water': 'world',
  court: 'us_politics',
  vaccine: 'us_politics',
  library: 'world',
  longword: 'tech_ai',
  election: 'us_politics',
  rail: 'business',
  museum: 'world',
};

function sectionedFixture(now = Date.now()) {
  const fx = fixture(now);
  const file = (s, i) => {
    const key = s.publication_id.replace(/^pub-/, '');
    const out = { ...s, importance: 3 + (i % 6), source_count: 3, words: 300 };
    if (SECTION_OF[key]) out.section = SECTION_OF[key];
    return out;
  };
  return { first: fx.first.map(file), second: fx.second.map(file) };
}

test.describe('Autopaper sections', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('a section row under the masthead filters by section and restores the last section after reload', async ({ page }, testInfo) => {
    const fx = sectionedFixture();
    await openFront(page, { width: 1440, height: 900 }, fx);
    const nav = page.locator('[data-autopaper-sections]');
    await expect(nav).toBeVisible();
    await expect(nav.locator('button')).toHaveText(['Front page', 'World', 'U.S. Politics', 'Business & Finance', 'Tech & AI']);
    await expect(nav.locator('[aria-current="page"]')).toHaveText('Front page');
    // Under the edition line, inside the masthead.
    const editionBox = await page.locator('[data-autopaper-edition]').boundingBox();
    const navBox = await nav.boundingBox();
    expect(navBox.y).toBeGreaterThanOrEqual(editionBox.y + editionBox.height - 1);
    // The front page lists every story, unfiled ones included.
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(rankFrontStories(fx.first).map((s) => s.title));
    await shoot(page, testInfo, 'autopaper-sections-1440-front');

    await nav.locator('[data-autopaper-section="us_politics"]').click();
    await expect(nav.locator('[aria-current="page"]')).toHaveText('U.S. Politics');
    const politics = rankFrontStories(storiesInSection(fx.first, 'us_politics'));
    expect(politics.length).toBe(3);
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(politics.map((s) => s.title));
    await expect(page.locator('h2')).toHaveCount(1);
    await expect(page.locator('[data-autopaper-lead] h2')).toHaveText(politics[0].title);
    await shoot(page, testInfo, 'autopaper-sections-1440-us-politics');

    // A section that ends within its front tiers (the server returned no
    // cursor for it) shows no lone More stories block or Older stories
    // button, though the front page's own cursor has more.
    await expect(page.locator('[data-autopaper-more-stories]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper-more]')).toHaveCount(0);

    await nav.locator('[data-autopaper-section="tech_ai"]').click();
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(
      rankFrontStories(storiesInSection(fx.first, 'tech_ai')).map((s) => s.title),
    );
    const stored = await page.evaluate(([a, b]) => [localStorage.getItem(a), localStorage.getItem(b)], [SECTION_STORAGE_KEY, SECTION_VISITS_STORAGE_KEY]);
    expect(stored[0]).toBe('tech_ai');
    expect(JSON.parse(stored[1])).toEqual({ us_politics: 1, tech_ai: 1 });

    // Reopen: the last section is restored.
    await page.reload();
    await expect(page.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('[data-autopaper-sections] [aria-current="page"]')).toHaveText('Tech & AI');
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(
      rankFrontStories(storiesInSection(fx.first, 'tech_ai')).map((s) => s.title),
    );
  });

  test('a section pages its own older stories with the section cursor', async ({ page }) => {
    const base = sectionedFixture();
    // Every first-page story but one is World; the second page has one World story.
    const fx = {
      first: base.first.map((s, i) => (i === 0 ? { ...s, section: 'tech_ai' } : { ...s, section: 'world' })),
      second: base.second.map((s, i) => (i === 0 ? { ...s, section: 'world' } : s)),
    };
    const world = rankFrontStories(storiesInSection(fx.first, 'world'));
    expect(world.length).toBe(13);
    const requests = [];
    // The server's own section list: 13 stories, then one more behind w2.
    const sectionBody = (section, cursor) =>
      cursor === 'w2' ? { stories: [fx.second[0]] } : { stories: storiesInSection(fx.first, section), next_cursor: 'w2' };
    await openFront(page, { width: 1440, height: 900 }, fx, '', { sectionBody, requests });
    await page.locator('[data-autopaper-section="world"]').click();
    await expect(page.locator('[data-autopaper-more-stories]')).toHaveCount(1);
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(world.map((s) => s.title));
    // The first page of a section is the server's, sized like the front page's.
    expect(requests.some((q) => /(^|[?&])section=world(&|$)/.test(q) && /(^|[?&])limit=60(&|$)/.test(q) && !/cursor=/.test(q))).toBe(true);
    await page.locator('[data-autopaper-more]').click();
    await expect(page.locator('[data-autopaper-more]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper-headline]')).toHaveText([...world, fx.second[0]].map((s) => s.title));
    // "Older stories" continued the section's cursor, not the front page's.
    expect(requests.some((q) => /cursor=w2(&|$)/.test(q) && /section=world(&|$)/.test(q))).toBe(true);
    expect(requests.some((q) => /cursor=page-2/.test(q))).toBe(false);
  });

  test('a section lists its stories from the server even when none is among the front page rows', async ({ page }) => {
    const fx = sectionedFixture();
    // Politics rows on the front page (court, vaccine, election) are dropped,
    // as when the section has been quiet for more than the newest 60 stories.
    const front = { first: fx.first.filter((s) => s.section !== 'us_politics'), second: fx.second };
    const old = { ...fx.first[0], publication_id: 'pub-old-politics', title: 'Senate passes the budget after an overnight session', section: 'us_politics', importance: 6 };
    const requests = [];
    const sectionBody = (section) => (section === 'us_politics' ? { stories: [old] } : { stories: [] });
    await openFront(page, { width: 1280, height: 900 }, front, '', { sectionBody, requests });
    await expect(page.locator('[data-autopaper-sections]')).toBeVisible();
    await page.locator('[data-autopaper-section="us_politics"]').click();
    await expect(page.locator('[data-autopaper-headline]')).toHaveText([old.title]);
    await expect(page.locator('[data-autopaper-section-empty]')).toHaveCount(0);
    expect(requests.some((q) => /section=us_politics/.test(q) && /limit=60/.test(q))).toBe(true);
  });

  test('"No stories in this section yet." shows only when the server returns no story for the section', async ({ page }) => {
    const fx = sectionedFixture();
    const requests = [];
    const sectionBody = (section, cursor) => {
      if (section === 'business') return { stories: [] };
      // The server stopped scanning without a match, then found one.
      if (cursor === 'c1') return { stories: [fx.first[4]] };
      return { stories: [], next_cursor: 'c1' };
    };
    await openFront(page, { width: 1280, height: 900 }, fx, '', { sectionBody, requests });
    await page.locator('[data-autopaper-section="business"]').click();
    await expect(page.locator('[data-autopaper-section-empty]')).toHaveText('No stories in this section yet.');
    await expect(page.locator('[data-autopaper-more]')).toHaveCount(0);
    // An empty page that carries a cursor is followed, not reported empty.
    await page.locator('[data-autopaper-section="tech_ai"]').click();
    await expect(page.locator('[data-autopaper-headline]')).toHaveText([fx.first[4].title]);
    await expect(page.locator('[data-autopaper-section-empty]')).toHaveCount(0);
    expect(requests.filter((q) => /section=tech_ai/.test(q)).length).toBe(2);
    // The front page still shows its own rows, untouched by section fetches.
    await page.locator('[data-autopaper-section="front"]').click();
    // (Same stories; the visits just made lift sections, so the order may differ.)
    await expect(page.locator('[data-autopaper-headline]')).toHaveCount(fx.first.length);
    expect((await page.locator('[data-autopaper-headline]').allTextContents()).map((t) => t.trim()).sort()).toEqual(
      fx.first.map((s) => s.title).sort(),
    );
  });

  test('the reader habit lifts their sections on the front page', async ({ page }) => {
    const fx = sectionedFixture();
    await page.addInitScript((key) => {
      window.localStorage.setItem(key, JSON.stringify({ tech_ai: 9, business: 1 }));
    }, SECTION_VISITS_STORAGE_KEY);
    await openFront(page, { width: 1440, height: 900 }, fx);
    const lifted = rankFrontStories(fx.first, { sectionShares: sectionShares({ tech_ai: 9, business: 1 }) });
    const plain = rankFrontStories(fx.first);
    expect(lifted.map((s) => s.publication_id)).not.toEqual(plain.map((s) => s.publication_id));
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(lifted.map((s) => s.title));
  });

  test('at 375px the section row scrolls inside itself, with no page overflow', async ({ page }, testInfo) => {
    const fx = sectionedFixture();
    await openFront(page, { width: 375, height: 812 }, fx);
    const row = page.locator('[data-autopaper-sections] ul');
    await expect(row).toBeVisible();
    const dims = await row.evaluate((el) => ({ scroll: el.scrollWidth, client: el.clientWidth, left: el.getBoundingClientRect().left }));
    expect(dims.scroll).toBeGreaterThan(dims.client);
    expect(dims.left).toBeGreaterThanOrEqual(16);
    await expectNoHorizontalOverflow(page, '375px sections');
    // The first section is reachable at the start; the last scrolls into view.
    const firstLeft = await page.locator('[data-autopaper-section="front"]').evaluate((el) => el.getBoundingClientRect().left);
    expect(firstLeft).toBeGreaterThanOrEqual(16);
    await shoot(page, testInfo, 'autopaper-sections-375-front');
    await page.locator('[data-autopaper-section="tech_ai"]').click();
    await expect(page.locator('[data-autopaper-sections] [aria-current="page"]')).toHaveText('Tech & AI');
    await expectNoHorizontalOverflow(page, '375px tech');
    await shoot(page, testInfo, 'autopaper-sections-375-tech');
  });

  test('with fewer than half the stories filed, there is no section row and a remembered section is ignored', async ({ page }) => {
    const fx = fixture();
    await page.addInitScript((key) => window.localStorage.setItem(key, 'tech_ai'), SECTION_STORAGE_KEY);
    await openFront(page, { width: 1280, height: 900 }, fx);
    await expect(page.locator('[data-autopaper-sections]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper-headline]')).toHaveText(rankFrontStories(fx.first).map((s) => s.title));
  });
});

test.describe('Autopaper front page', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('broadsheet layout in a wide window (1440px screen): masthead, one lead, four columns, rank order, links', async ({ page }, testInfo) => {
    const fx = fixture();
    const ranked = rankFrontStories(fx.first);
    // The lead is the developed storm story, not the newest one.
    expect(ranked[0].publication_id).toBe('pub-storm');
    expect(fx.first[0].publication_id).toBe('pub-harbor');

    // The window is 84% of the screen, and four columns need 1100px of it.
    await openFront(page, { width: 1440, height: 900 }, fx);
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
    expect(hrefs).toEqual(ranked.map((s) => `/pub/texture/${slugOf(s.route.bare)}`));
    const versionHrefs = await front.locator('[data-autopaper-version-link]').evaluateAll((links) => links.map((a) => a.getAttribute('href')));
    expect(versionHrefs).toEqual(ranked.map((s) => `/pub/texture/${slugOf(s.route.bare)}/v${s.latest_revision}`));
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
    await expectNoHorizontalOverflow(page, '1440px');

    // Focus is visible on a headline link.
    await page.locator('[data-autopaper-lead] h2 a').focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    const focus = await page.locator('[data-autopaper-lead] h2 a').evaluate((el) => getComputedStyle(el).boxShadow);
    expect(focus).toContain('inset');
    await page.locator('[data-autopaper-lead] h2 a').blur();

    await shoot(page, testInfo, 'autopaper-front-1440-default');

    // Older stories pages in the rest, below More stories.
    const older = page.locator('[data-autopaper-more]');
    await expect(older).toHaveText('Older stories');
    await older.click();
    await expect(front.locator('[data-autopaper-tier="more"]')).toHaveCount(fx.first.length - 11 + fx.second.length);
    await expect(older).toHaveCount(0);
    await expect(front.locator('h2')).toHaveCount(1);

    // A headline opens the story in a Texture window, at its published route.
    await page.locator('[data-autopaper-lead] h2 a').click();
    await expect(page.locator('[data-window-app-id="texture"]')).toHaveCount(1, { timeout: 15_000 });
    await expect(page.locator('[data-window-app-id="texture"] [data-texture-app]')).toHaveCount(1);
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
    // The gutter is the page's own, measured from the Autopaper window's edge.
    const gutter = await page.locator('[data-autopaper-masthead] h1').evaluate((el) =>
      Math.round(el.getBoundingClientRect().left - el.closest('[data-autopaper]').getBoundingClientRect().left));
    expect(gutter).toBe(16);
    await expectNoHorizontalOverflow(page, '390px');
    await expect(page.locator('[data-autopaper-more]')).toBeVisible();

    await shoot(page, testInfo, 'autopaper-front-390-default');
  });

  test('dark theme screenshots map paper and ink to the theme', async ({ page }, testInfo) => {
    const fx = fixture();
    await openFront(page, { width: 1280, height: 900 }, fx, 'futuristic-noir');
    await expect(page.locator('[data-theme-id]').first()).toHaveAttribute('data-theme-id', 'futuristic-noir');
    const colors = await page.locator('[data-autopaper]').evaluate((el) => {
      const style = getComputedStyle(el);
      return { paper: style.backgroundColor, ink: style.color };
    });
    expect(colors.paper).not.toBe(colors.ink);
    await shoot(page, testInfo, 'autopaper-front-1280-dark');

    await page.setViewportSize({ width: 390, height: 844 });
    await expectNoHorizontalOverflow(page, '390px dark');
    await shoot(page, testInfo, 'autopaper-front-390-dark');
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
    await page.goto(`${BASE_URL}/`);
    await expect(page.locator('[data-autopaper-masthead] h1')).toHaveText('Autopaper');
    await expect(page.locator('[data-autopaper-empty]')).toHaveText('No stories yet.');
    await expect(page.locator('[data-autopaper-updated]')).toHaveCount(0);
    await expect(page.locator('[data-autopaper] h2')).toHaveCount(0);
    await shoot(page, testInfo, 'autopaper-front-390-empty');

    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await mockFront(page, fixture(), { status: 503 });
    await page.goto(`${BASE_URL}/`);
    await expect(page.locator('[data-autopaper-error]')).toContainText('Could not load stories');
    await expect(page.locator('[data-autopaper-masthead] h1')).toHaveText('Autopaper');
  });

  test('the logged-out home opens straight to the Autopaper window, no intro deck', async ({ page }, testInfo) => {
    const fx = fixture();
    const ranked = rankFrontStories(fx.first);
    await page.setViewportSize({ width: 1280, height: 900 });
    await mockFront(page, fx);
    await page.goto(`${BASE_URL}/`);
    const win = page.locator('[data-window][data-window-id="public-preview-autopaper"]');
    await expect(win).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('[data-landing-intro]')).toHaveCount(0);
    await expect(page.locator('.app-root')).toHaveAttribute('data-theme-id', 'london-salmon');
    expect(await page.locator('[data-window]').count()).toBe(1);
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
    await shoot(page, testInfo, 'autopaper-desktop-window-1280-default');

    // At phone width the home is the same: the Autopaper window, no deck.
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${BASE_URL}/`);
    await expect(win.locator('[data-autopaper-grid]')).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('[data-landing-intro]')).toHaveCount(0);
    await expectNoHorizontalOverflow(page, '390px home');
    await page.screenshot({ path: nodePath.join(SHOT_DIR || testInfo.outputDir, 'autopaper-home-390-default.png') });
  });
});
