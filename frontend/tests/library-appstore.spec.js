// The Library as an app store (owner, 2026-11-04, on a phone).
//
// No server: the spec builds this tree's shell, serves it with the package
// contract runner's stub backend, and answers the catalog and the install
// list itself, statefully, so an install changes what the next read says. It
// runs the Library at a phone width (390x844) and at a desktop width.
//
// Failure modes, written before the code:
//   S1  the list shows an id, a track, a computer, a "reader ..." header, or a
//       card without its maker's Choir address (<NNN>@choir.news) or its thumbnail, or
//       any other email text (the sign-in email is never shown);
//   S2  the window is titled "Library (new)";
//   S3  the app page is a second drill-down: its screenshots, its Install
//       button, its description or its precommitments need another tap;
//   S4  the app page shows the metadata panel (publisher owner, manifest
//       version, built on, parent, operation, who may see it, changed paths)
//       or any id, track or computer;
//   S5  the screenshots cannot be tapped to enlarge or stepped through, or the
//       video the release has is absent;
//   S6  Install, Installed and Uninstall do not follow the install list;
//   S7  the version history is open by default or labelled with an id;
//   S8  the page scrolls sideways, or the Install button is below the first
//       screen on a phone.
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildShell, serveDist, stubBackend } from '../scripts/package-contracts.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const NAME = 'library-next';
const COMPUTER = 'contract-computer';

const hex = (seed, n = 64) => {
  let out = '';
  for (let i = 0; out.length < n; i += 1) out += ((seed * 2654435761 + i * 40503) >>> 0).toString(16).padStart(8, 'a');
  return out.slice(0, n);
};

// A 1x1 PNG, enough for an <img> to decode.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');
const WEBM = Buffer.concat([Buffer.from([0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x82, 0x84, 0x77, 0x65, 0x62, 0x6d]), Buffer.alloc(64, 5)]);

const APPS = [
  { name: 'converter-next', owner: hex(1), email: '007@choir.news', desc: 'Convert units between systems', shots: 3, video: true, versions: 2 },
  { name: 'files-next', owner: hex(2), email: '012@choir.news', desc: 'Browse and rename files', shots: 1, video: false, versions: 1 },
  { name: 'files-next', owner: hex(3), email: '10000042@choir.news', desc: '', shots: 2, video: false, versions: 1 },
];

function world() {
  const releases = new Map(); // id -> view
  const byTrack = new Map(); // name\ntrack -> [ids newest first]
  const entries = [];
  APPS.forEach((app, ai) => {
    const ids = [];
    for (let v = 1; v <= app.versions; v += 1) {
      const id = hex(100 + ai * 10 + v);
      const media = [];
      for (let s = 0; s < app.shots; s += 1) {
        const sha = hex(200 + ai * 10 + v * 3 + s);
        media.push({ kind: 'screenshot', name: `shot-${s}.png`, media_type: 'image/png', sha256: sha, bytes: PNG.length, width: 390, height: 844, url: `/api/catalog/releases/${id}/media/${sha}` });
      }
      if (app.video) {
        const sha = hex(300 + v);
        media.push({ kind: 'video', name: 'walk.webm', media_type: 'video/webm', sha256: sha, bytes: WEBM.length, url: `/api/catalog/releases/${id}/media/${sha}` });
      }
      releases.set(id, {
        release_id: id,
        published_at: v === 1 && app.versions > 1 ? '2026-11-03T09:15:00Z' : '2026-11-04T06:27:00Z',
        publisher_address: app.email,
        description: app.desc,
        own: false,
        media,
        listing: { record_id: id, package: app.name, track: app.owner, seq: v, state: 'listed', scopes: ['owner', 'public'] },
        record: {
          package: { name: app.name, manifest_version: '1.0.0' },
          publisher: { owner_id: app.owner, computer_id: `computer-${hex(ai + 50, 8)}` },
          source: { built_on: hex(9, 40), changed_paths: [`frontend/src/packages/${app.name}/App.svelte`] },
          operation: { operation_id: 'op-' + hex(7, 12) },
          precommitments: v === app.versions && ai === 0
            ? [
                { record_id: 'pc-' + hex(5, 10), question: 'Does the converter handle metric and imperial units?', p_yes_permille: 880, settled: 'yes' },
                { record_id: 'pc-' + hex(6, 10), question: 'Does it keep the last choice after a reload?', p_yes_permille: 640, settled: '' },
                { record_id: 'pc-' + hex(8, 10), question: 'Does it work with the keyboard alone?', p_yes_permille: 400, settled: 'no' },
              ]
            : [],
        },
      });
      ids.unshift(id);
    }
    byTrack.set(`${app.name}\n${app.owner}`, ids);
    const head = releases.get(ids[0]);
    entries.push({
      package: app.name, track: app.owner, head_release_id: ids[0], head_seq: app.versions, releases: app.versions,
      head_media: head.media, head_description: app.desc, head_published_at: head.published_at,
      publisher_address: app.email, own: false,
    });
  });
  return { releases, byTrack, entries, installs: [], posts: [] };
}

// answer routes the catalog and the install list for one page, statefully.
async function answerCatalog(page, w) {
  await page.route(/\/api\//, async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const p = url.pathname;
    const json = (body, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
    if (p === '/api/catalog/packages' && req.method() === 'GET') {
      const q = (url.searchParams.get('q') || '').toLowerCase();
      const list = w.entries.filter((e) => !q || e.package.includes(q));
      return json({ count: list.length, reader: hex(99), packages: list });
    }
    let m = /^\/api\/catalog\/packages\/([^/]+)\/releases$/.exec(p);
    if (m) {
      const ids = w.byTrack.get(`${decodeURIComponent(m[1])}\n${url.searchParams.get('track')}`) || [];
      return json({ package: m[1], track: url.searchParams.get('track'), count: ids.length, releases: ids.map((id) => w.releases.get(id)) });
    }
    m = /^\/api\/catalog\/releases\/([0-9a-f]{64})$/.exec(p);
    if (m) return w.releases.has(m[1]) ? json(w.releases.get(m[1])) : json({ error: 'not found' }, 404);
    m = /^\/api\/catalog\/releases\/[0-9a-f]{64}\/media\/[0-9a-f]{64}$/.exec(p);
    if (m) {
      const video = [...w.releases.values()].some((r) => r.media.some((i) => p.endsWith(i.sha256) && i.kind === 'video'));
      return route.fulfill({ status: 200, contentType: video ? 'video/webm' : 'image/png', body: video ? WEBM : PNG });
    }
    if (p === `/api/computers/${COMPUTER}/packages/installs`) {
      if (req.method() === 'GET') return json({ installs: w.installs });
      const body = JSON.parse(req.postData() || '{}');
      w.posts.push(body);
      const release = w.releases.get(body.release_id);
      const name = release ? release.listing.package : (w.installs.find((a) => a.release_id === body.release_id) || {}).package;
      for (const a of w.installs) if (a.package === name && a.state !== 'replaced') Object.assign(a, { state: 'replaced', patch_matches_release: false });
      const row = {
        install_id: 'ad-' + (w.installs.length + 1), release_id: body.release_id, package: name, action: body.action || 'install',
        mode: 'runtime', patch_matches_release: true, requested_at: '2026-11-04T10:00:00Z', publisher_computer_id: release ? release.record.publisher.computer_id : 'computer-gone',
      };
      w.installs.unshift(row);
      return json(row, 201);
    }
    return route.fallback();
  });
}

// A run of hex that is an id: eight or more hex digits with at least one
// letter (a Choir address's digits are not an id).
const HEX_RUN = /(?=[0-9a-f]*[a-f])[0-9a-f]{8,}/i;
const METADATA_WORDS = /publisher owner|manifest version|built on|made by operation|who may see|track and seq|what it changes|changed paths|release history|\btrack\b|computer-|reader /i;

// every email-looking token in a text
const addressesIn = (text) => text.match(/[^\s@]+@[^\s@]+/g) || [];

async function openLibrary(page, base) {
  await page.goto(base, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor();
  await page.locator('[data-desk-menu-button]').first().click();
  await page.locator('[data-desk-sheet]').first().waitFor();
  const entry = page.locator(`[data-desk-sheet-app][data-desk-app-id="${NAME}"]`).first();
  const launcherText = (await entry.innerText()).trim();
  await entry.click();
  const win = page.locator(`[data-window][data-window-app-id="${NAME}"]`).first();
  await win.waitFor();
  const content = win.locator('[data-window-content]').first();
  await content.locator('[data-library-app][data-view="list"]').waitFor();
  return { win, content, launcherText };
}

test.describe.configure({ mode: 'serial' });

test.describe('the Library is an app store', () => {
  let tmp;
  let server;

  test.beforeAll(async () => {
    test.setTimeout(180_000);
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-appstore-'));
    const dist = await buildShell({ outDir: path.join(tmp, 'dist') });
    server = await serveDist(dist);
  });

  test.afterAll(async () => {
    await server?.close();
    if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
  });

  for (const device of [
    { label: 'phone 390x844', viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true },
    { label: 'desktop 1280x800', viewport: { width: 1280, height: 800 }, hasTouch: false, isMobile: false },
  ]) {
    test(`shelf, app page, gallery, install and uninstall on a ${device.label}`, async ({ browser }, testInfo) => {
      test.setTimeout(90_000);
      const context = await browser.newContext({ viewport: device.viewport, hasTouch: device.hasTouch, isMobile: device.isMobile });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (e) => errors.push(String(e)));
      const w = world();
      await stubBackend(page, { api: [] });
      await answerCatalog(page, w);
      const { win, content, launcherText } = await openLibrary(page, server.base);

      // S2: the title is "Library".
      expect(launcherText).toMatch(/Library/);
      expect(launcherText).not.toMatch(/\(new\)/i);
      expect(await win.innerText()).not.toMatch(/\(new\)/i);

      // S1: the shelf. One card per app and maker, a real thumbnail, the
      // description, the maker's Choir address and a plain date; no id anywhere.
      const cards = content.locator('[data-package-row]');
      await expect(cards).toHaveCount(3);
      const shelfText = await content.innerText();
      expect(shelfText, 'shelf text').not.toMatch(HEX_RUN);
      expect(shelfText, 'shelf text').not.toMatch(METADATA_WORDS);
      expect(shelfText).not.toMatch(/releases? from this catalog/i);
      for (const address of ['007@choir.news', '012@choir.news', '10000042@choir.news']) expect(shelfText).toContain(address);
      expect(addressesIn(shelfText), 'only Choir addresses appear').toEqual(expect.arrayContaining(['007@choir.news']));
      for (const a of addressesIn(shelfText)) expect(a).toMatch(/^\d{3,}@choir\.news$/);
      expect(shelfText).toContain('Convert units between systems');
      expect(shelfText).toMatch(/Updated Nov 4/);
      const firstThumb = cards.first().locator('img');
      await expect(firstThumb).toHaveCount(1);
      await expect.poll(() => firstThumb.evaluate((img) => img.naturalWidth)).toBeGreaterThan(0);
      // The two files-next cards are two makers of one name.
      await expect(content.locator('[data-package-row][data-package="files-next"]')).toHaveCount(2);

      // S8: no sideways scroll on the shelf.
      expect(await content.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true);

      // Search stays.
      await content.locator('[data-search-input]').fill('conv');
      await expect(cards).toHaveCount(1);
      await content.locator('[data-clear-search-inline]').click();
      await expect(cards).toHaveCount(3);

      // S3: one tap opens the app page with everything on it.
      await content.locator('[data-package-row][data-package="converter-next"]').click();
      const app = content.locator('[data-app-view]');
      await app.locator('[data-app-name]').waitFor();
      await expect(content.locator('[data-app-title]')).toHaveText('converter-next');
      await expect(app.locator('[data-app-publisher]')).toHaveText('007@choir.news');
      await expect(app.locator('[data-app-desc]')).toHaveText('Convert units between systems');
      const install = app.locator('[data-install-button]');
      await expect(install).toBeVisible();
      await expect(install).toContainText('Install');
      // S8: Install is on the first screen, not below the gallery.
      const box = await install.boundingBox();
      expect(box.y + box.height, 'Install inside the first screen').toBeLessThan(device.viewport.height);

      // S4: no metadata panel and no id.
      const appText = await content.innerText();
      expect(appText, 'app page text').not.toMatch(HEX_RUN);
      expect(appText, 'app page text').not.toMatch(METADATA_WORDS);
      expect(appText).toContain('Updated Nov 4');
      for (const a of addressesIn(appText)) expect(a).toMatch(/^\d{3,}@choir\.news$/);
      // The precommitments are on the page itself, in words.
      const precs = app.locator('[data-precommitment]');
      await expect(precs).toHaveCount(3);
      const precText = await app.locator('[data-track-record]').innerText();
      expect(precText).toContain('Predicted 88% likely');
      expect(precText).toContain('Came true');
      expect(precText).toContain('Not settled yet');
      expect(precText).toContain('Did not come true');

      // S5: three screenshots to swipe, the video, a tap enlarges.
      const shots = app.locator('[data-shot]');
      await expect(shots).toHaveCount(3);
      await expect(app.locator('video')).toHaveCount(1);
      expect(await app.locator('[data-gallery]').evaluate((el) => el.scrollWidth <= el.clientWidth + 1 || getComputedStyle(el.querySelector('ul')).overflowX === 'auto')).toBe(true);
      expect(await content.evaluate((el) => el.scrollWidth <= el.clientWidth + 1), 'no sideways scroll on the app page').toBe(true);
      await shots.nth(0).click();
      const viewer = content.locator('[data-viewer]');
      await expect(viewer).toBeVisible();
      await expect(viewer).toContainText('1 of 3');
      await viewer.locator('[data-viewer-next]').click();
      await expect(viewer).toContainText('2 of 3');
      await viewer.locator('[data-viewer-prev]').click();
      await viewer.locator('[data-viewer-prev]').click();
      await expect(viewer).toContainText('3 of 3');
      await page.keyboard.press('Escape');
      await expect(viewer).toHaveCount(0);

      // S7: the version history is collapsed, plainly labelled.
      const versions = app.locator('[data-versions]');
      await expect(versions).toHaveCount(1);
      expect(await versions.evaluate((el) => el.open)).toBe(false);
      await versions.locator('summary').click();
      const versionText = await versions.innerText();
      expect(versionText).toContain('Version 2, Nov 4');
      expect(versionText).toContain('Version 1, Nov 3');
      expect(versionText).not.toMatch(HEX_RUN);
      await expect(versions.locator('[data-older]'), 'no older page when the history fits').toHaveCount(0);

      // S6: install from the app page, then uninstall.
      await install.click();
      const confirm = content.locator('[data-confirm]');
      await expect(confirm).toBeVisible();
      expect(await confirm.innerText()).not.toMatch(HEX_RUN);
      expect(await confirm.innerText()).toContain('007@choir.news');
      await confirm.locator('[data-confirm-cancel]').click();
      await expect(confirm).toHaveCount(0);
      expect(w.posts, 'a cancelled install posts nothing').toHaveLength(0);
      await install.click();
      await confirm.locator('[data-confirm-install]').click();
      await expect(app.locator('[data-installed]')).toHaveText('Installed');
      await expect(app.locator('[data-uninstall-button]')).toBeVisible();
      await expect(install).toHaveCount(0);
      expect(w.posts).toHaveLength(1);
      expect(w.posts[0]).toMatchObject({ release_id: hex(100 + 2), action: 'install' });
      await page.screenshot({ path: testInfo.outputPath(`appstore-${device.viewport.width}-installed.png`) });

      // Back to the shelf: the card says Installed.
      await content.locator('[data-back]').click();
      await expect(cards).toHaveCount(3);
      await expect(content.locator('[data-package-row][data-package="converter-next"] [data-card-installed]')).toHaveText('Installed');

      // Uninstall from the app page.
      await content.locator('[data-package-row][data-package="converter-next"]').click();
      await app.locator('[data-uninstall-button]').click();
      await expect(app.locator('[data-install-button]')).toBeVisible();
      await expect(app.locator('[data-installed]')).toHaveCount(0);
      expect(w.posts).toHaveLength(2);
      expect(w.posts[1]).toMatchObject({ action: 'uninstall' });

      // A maker with one version and no promises: no history, no empty panel.
      await content.locator('[data-back]').click();
      await content.locator('[data-package-row][data-package="files-next"]').nth(1).click();
      await expect(content.locator('[data-app-publisher]')).toHaveText('10000042@choir.news');
      await expect(content.locator('[data-versions]')).toHaveCount(0);
      const plain = await content.innerText();
      expect(plain).not.toMatch(HEX_RUN);
      expect(plain).not.toMatch(METADATA_WORDS);

      await page.screenshot({ path: testInfo.outputPath(`appstore-${device.viewport.width}-app.png`) });
      expect(errors, 'uncaught page errors').toEqual([]);
      await context.close();
    });
  }

  // No fallback: when the address service is down the read fails visibly,
  // with no card and no placeholder maker.
  test('a shelf the server could not attribute shows an error, never cards without makers', async ({ browser }) => {
    test.setTimeout(60_000);
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
    const page = await context.newPage();
    await stubBackend(page, { api: [] });
    await page.route(/\/api\/catalog\/packages$/, (route) => route.fulfill({
      status: 502, contentType: 'application/json',
      body: JSON.stringify({ error: 'publisher_address_unavailable: x', reason: 'publisher_address_unavailable' }),
    }));
    const { content } = await openLibrary(page, server.base);
    await expect(content.locator('[data-error]')).toContainText('address service did not answer');
    await expect(content.locator('[data-package-row]')).toHaveCount(0);
    expect(await content.innerText()).not.toMatch(/unnamed|unknown publisher/i);
    await context.close();
  });

  // Install state belongs to one maker. Two makers of one name: A's install
  // is not B's; B's page offers no Update and no Uninstall of A's app, and
  // installing B says it replaces A's. An installed app that is no longer
  // listed can still be uninstalled from the shelf.
  test('one maker\'s install is not another maker\'s, and an unlisted install can be removed', async ({ browser }) => {
    test.setTimeout(90_000);
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
    const page = await context.newPage();
    const w = world();
    // maker A's files-next (the 012 card) is installed; an app no longer
    // listed is installed too.
    const [aId] = w.byTrack.get(`files-next\n${hex(2)}`);
    w.installs.push(
      { install_id: 'ad-a', release_id: aId, package: 'files-next', action: 'install', mode: 'runtime', patch_matches_release: true },
      { install_id: 'ad-gone', release_id: hex(777), package: 'old-app', action: 'install', mode: 'runtime', patch_matches_release: true },
    );
    await stubBackend(page, { api: [] });
    await answerCatalog(page, w);
    const { content } = await openLibrary(page, server.base);
    const cards = content.locator('[data-package-row][data-package="files-next"]');
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0).locator('[data-card-installed]')).toHaveText('Installed');
    await expect(cards.nth(1).locator('[data-card-installed]')).toHaveCount(0);

    // The unlisted install is on the shelf's Installed section.
    const row = content.locator('[data-installed-row]', { hasText: 'old-app' });
    await expect(row).toHaveCount(1);

    // B's page: no Uninstall, no Update, a replace note, a confirm that says so.
    await cards.nth(1).click();
    const app = content.locator('[data-app-view]');
    await app.locator('[data-app-name]').waitFor();
    await expect(app.locator('[data-uninstall-button]')).toHaveCount(0);
    await expect(app.locator('[data-installed]')).toHaveCount(0);
    await expect(app.locator('[data-elsewhere]')).toContainText('from a different maker');
    await expect(app.locator('[data-install-button]')).toContainText('Install');
    await app.locator('[data-install-button]').click();
    await expect(content.locator('[data-confirm-replaces]')).toContainText('different maker');
    await content.locator('[data-confirm-cancel]').click();
    expect(w.posts).toHaveLength(0);
    await content.locator('[data-back]').click();

    // A's page: installed, up to date, Uninstall but no Update.
    await cards.nth(0).click();
    await expect(app.locator('[data-installed]')).toHaveText('Installed');
    await expect(app.locator('[data-uninstall-button]')).toBeVisible();
    await expect(app.locator('[data-install-button]')).toHaveCount(0);
    await content.locator('[data-back]').click();

    // The unlisted app is removed from the shelf.
    await content.locator('[data-installed-row]', { hasText: 'old-app' }).locator('[data-uninstall-button]').click();
    await expect.poll(() => w.posts.length).toBe(1);
    expect(w.posts[0]).toMatchObject({ release_id: hex(777), action: 'uninstall' });
    await expect(content.locator('[data-installed-row]', { hasText: 'old-app' })).toHaveCount(0);
    await context.close();
  });
});
