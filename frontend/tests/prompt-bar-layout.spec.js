/**
 * The prompt bar layout (owner, 2026-11-09, from iPhone screenshots): the
 * chip and the input sit on one row with no overlap, the status dot is a real
 * send button, the input grows to several lines and then scrolls, and the
 * Texture Sources button works while Texture is revising (with the Models and
 * Search blocks that say what served the document). Runs against the built
 * frontend (vite preview) with the session, desktop state, Texture routes
 * and /api/prompt-bar mocked. Written before the fix.
 *
 * Ways it can fail:
 *   L1 The chip, the typed text and the send button overlap, or the text
 *      renders outside the bar (mobile 390x844 and desktop 1280x800).
 *   L2 The send button is missing, hidden, not clickable, has no accessible
 *      label, or does not submit. Enter must NOT submit (owner, 2026-11-09:
 *      "the enter button on keyboard should be a newline, not submit. Let the
 *      submit button submit"); Ctrl/Cmd+Enter is the keyboard send.
 *   L3 Long text does not grow the input to several lines, grows without a
 *      cap, or grows downward over the bottom edge / the bar covers the
 *      window; Enter or Shift+Enter does not insert a newline.
 *   L4 The Sources button is disabled while Texture is revising, or the panel
 *      it opens lacks the Models and Search blocks.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/prompt-bar-layout.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const JSON_HEADERS = { 'content-type': 'application/json' };
const SHOT_DIR = process.env.PROMPT_BAR_SHOT_DIR || '';
const DOC = { id: 'doc-ai', title: "What's latest in ai", content: "# What's latest in ai\n\nNotes." };

function textureWindow() {
  return {
    window_id: 'win-ai',
    app_id: 'texture',
    title: DOC.title,
    geometry: { x: 20, y: 20, width: 700, height: 420 },
    mode: 'maximized',
    z_index: 2,
    app_context: { docId: DOC.id, windowTitle: DOC.title },
  };
}

async function mockComputer(page, { windows = [], activeWindowId = '' } = {}) {
  const submits = [];
  await page.route('**/auth/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const body = pathname === '/auth/session'
      ? { authenticated: true, user: { id: 'owner-1', email: 'owner@example.com' } }
      : { ok: true };
    await route.fulfill({ status: 200, headers: JSON_HEADERS, body: JSON.stringify(body) });
  });
  await page.route('**/health', (route) => route.fulfill({ status: 200, headers: JSON_HEADERS, body: '{"status":"ok"}' }));
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const path = url.pathname;
    let body = {};
    let status = 200;
    const docMatch = path.match(/\/texture\/documents\/([^/]+)(\/[^/]+)?$/);
    if (path === '/api/shell/bootstrap') {
      body = { computer_id: 'layout-test-computer' };
    } else if (path === '/api/desktop/state') {
      body = method === 'GET'
        ? { owner_id: 'owner-1', windows, active_window_id: activeWindowId, updated_at: '2026-11-09T00:00:00Z' }
        : { ok: true, updated_at: '2026-11-09T00:00:01Z' };
    } else if (path === '/api/prompt-bar' && method === 'POST') {
      const request = route.request().postDataJSON();
      submits.push(request);
      status = 202;
      body = {
        schema: 'choir.prompt_bar_continue.v1',
        continued: true,
        command_id: request.command_id,
        doc_id: request.focus?.object_ref || DOC.id,
        trajectory_id: 'trajectory-next',
        revision_id: `${DOC.id}-rev-2`,
        request_id: 'request-1',
        replay: false,
      };
    } else if (docMatch && docMatch[2] === '/models') {
      body = {
        schema: 'choir.model_provenance.v1',
        doc_id: DOC.id,
        runs: 3,
        runs_served: 3,
        roles: [{ role: 'research', models: [{ provider: 'deepseek', model: 'v4-flash', calls: 4, runs: 1 }] }],
        search: {
          roles: [
            {
              role: 'research_web',
              providers: [
                { provider: 'keenable', calls: 3, answered: 3, results: 24 },
                { provider: 'exa', calls: 2, answered: 1, results: 9 },
              ],
            },
          ],
        },
      };
    } else if (docMatch && !docMatch[2]) {
      body = {
        doc_id: DOC.id,
        owner_id: 'owner-1',
        title: DOC.title,
        current_revision_id: `${DOC.id}-rev-1`,
        trajectory_id: '',
        created_at: '2026-11-09T00:00:00Z',
        updated_at: '2026-11-09T00:00:00Z',
      };
    } else if (docMatch && docMatch[2] === '/revisions') {
      body = {
        revisions: [{
          revision_id: `${DOC.id}-rev-1`,
          doc_id: DOC.id,
          author_kind: 'user',
          author_label: 'owner-1',
          content: DOC.content,
          created_at: '2026-11-09T00:00:00Z',
        }],
      };
    } else if (path.endsWith('/texture/documents')) {
      body = { documents: [] };
    }
    await route.fulfill({ status, headers: JSON_HEADERS, body: JSON.stringify(body) });
  });
  return submits;
}

async function openDesktop(page) {
  await page.goto(BASE_URL);
  const input = page.locator('[data-prompt-input]');
  await expect(input).toBeEnabled({ timeout: 20_000 });
  return input;
}

function overlaps(a, b) {
  return a.x < b.x + b.width - 0.5 && b.x < a.x + a.width - 0.5 && a.y < b.y + b.height - 0.5 && b.y < a.y + a.height - 0.5;
}

async function shot(page, name) {
  if (SHOT_DIR) await page.screenshot({ path: `${SHOT_DIR}/${name}.png` });
}

const VIEWPORTS = [
  { name: 'mobile', width: 390, height: 844 },
  { name: 'desktop', width: 1280, height: 800 },
];

for (const vp of VIEWPORTS) {
  test.describe(`prompt bar at ${vp.name} ${vp.width}x${vp.height}`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });

    test('L1 L2: chip, text and send button sit on one row without overlap, and the button sends', async ({ page }) => {
      const submits = await mockComputer(page, { windows: [textureWindow()], activeWindowId: 'win-ai' });
      const input = await openDesktop(page);
      const chip = page.locator('[data-command-field] [data-prompt-target]');
      const send = page.locator('[data-prompt-submit]');
      // The pill names the desk only; the object's name stays in its label.
      await expect(chip).toHaveAttribute('title', /What's latest in ai/, { timeout: 10_000 });
      await shot(page, `after-${vp.name}-empty`);

      await input.fill('skakkaka');
      const bar = await page.locator('[data-prompt-surface]').boundingBox();
      const chipBox = await chip.boundingBox();
      const inputBox = await input.boundingBox();
      const sendBox = await send.boundingBox();
      expect(overlaps(chipBox, inputBox)).toBe(false);
      expect(overlaps(inputBox, sendBox)).toBe(false);
      expect(overlaps(chipBox, sendBox)).toBe(false);
      // One row: the chip and the input share their vertical centre line.
      expect(Math.abs((chipBox.y + chipBox.height / 2) - (inputBox.y + inputBox.height / 2))).toBeLessThanOrEqual(2);
      for (const box of [chipBox, inputBox, sendBox]) {
        expect(box.x).toBeGreaterThanOrEqual(bar.x - 0.5);
        expect(box.x + box.width).toBeLessThanOrEqual(bar.x + bar.width + 0.5);
        expect(box.y).toBeGreaterThanOrEqual(bar.y - 0.5);
        expect(box.y + box.height).toBeLessThanOrEqual(bar.y + bar.height + 0.5);
      }
      await shot(page, `after-${vp.name}-typed`);

      // L2: visible, labelled, tall enough to tap, and it sends.
      await expect(send).toBeVisible();
      await expect(send).toHaveAttribute('aria-label', /Send prompt/);
      expect(sendBox.width).toBeGreaterThanOrEqual(40);
      expect(sendBox.height).toBeGreaterThanOrEqual(40);
      await send.click();
      await expect.poll(() => submits.length).toBe(1);
      expect(submits[0].text).toBe('skakkaka');
      await expect(input).toHaveValue('');
      // The button stays after the send, and an empty click sends nothing.
      await expect(send).toBeVisible();
      await send.click();
      await page.waitForTimeout(200);
      expect(submits.length).toBe(1);

      // Enter is a newline, never a send; Ctrl/Cmd+Enter sends.
      await expect(page.locator('[data-prompt-surface]')).toHaveAttribute('data-routing', 'false', { timeout: 10_000 });
      await input.fill('second line of thought');
      await input.press('Enter');
      await page.waitForTimeout(200);
      expect(submits.length).toBe(1);
      await expect(input).toHaveValue('second line of thought\n');
      await input.fill('second line of thought');
      await input.press('ControlOrMeta+Enter');
      await expect.poll(() => submits.length).toBe(2);
      expect(submits[1].text).toBe('second line of thought');
    });

    test('L1: a long chip label does not push into the text or the button', async ({ page }) => {
      const win = textureWindow();
      win.title = 'A very long document title that cannot possibly fit inside the chip on a phone';
      await mockComputer(page, { windows: [win], activeWindowId: 'win-ai' });
      const input = await openDesktop(page);
      const chip = page.locator('[data-command-field] [data-prompt-target]');
      await expect(chip).toBeVisible({ timeout: 10_000 });
      const chipBox = await chip.boundingBox();
      const inputBox = await input.boundingBox();
      const sendBox = await page.locator('[data-prompt-submit]').boundingBox();
      expect(overlaps(chipBox, inputBox)).toBe(false);
      expect(overlaps(inputBox, sendBox)).toBe(false);
      expect(inputBox.width).toBeGreaterThan(80);
    });

    test('L3: long text grows the input to several lines, caps, then scrolls; Enter is a newline', async ({ page }) => {
      const submits = await mockComputer(page, { windows: [textureWindow()], activeWindowId: 'win-ai' });
      const input = await openDesktop(page);
      const surface = page.locator('[data-prompt-surface]');
      const one = await input.boundingBox();
      const surfaceOne = await surface.boundingBox();

      await input.fill('what is the latest in ai research this week, and which labs published results on agents, reasoning and long context? '.repeat(4));
      const several = await input.boundingBox();
      expect(several.height).toBeGreaterThan(one.height + 20);
      const grown = await surface.boundingBox();
      // The bar grows upward: its bottom edge stays, its top moves up.
      expect(Math.abs(grown.y + grown.height - (surfaceOne.y + surfaceOne.height))).toBeLessThanOrEqual(1);
      expect(grown.y).toBeLessThan(surfaceOne.y);
      expect(grown.y).toBeGreaterThanOrEqual(0);
      await shot(page, `after-${vp.name}-multiline`);
      // Every line of text is inside the bar.
      const inside = await input.evaluate((el) => {
        const bar = el.closest('[data-prompt-surface]').getBoundingClientRect();
        const box = el.getBoundingClientRect();
        return box.top >= bar.top - 0.5 && box.bottom <= bar.bottom + 0.5;
      });
      expect(inside).toBe(true);

      // A very long text hits the cap and scrolls.
      await input.fill(Array.from({ length: 40 }, (_, i) => `line ${i}`).join('\n'));
      const capped = await input.boundingBox();
      expect(capped.height).toBeLessThanOrEqual(vp.height * 0.3 + 1);
      const scrolls = await input.evaluate((el) => el.scrollHeight > el.clientHeight && getComputedStyle(el).overflowY === 'auto');
      expect(scrolls).toBe(true);
      expect((await surface.boundingBox()).y).toBeGreaterThan(vp.height * 0.4);

      // Shrinks back when cleared.
      await input.fill('');
      const cleared = await input.boundingBox();
      expect(Math.abs(cleared.height - one.height)).toBeLessThanOrEqual(1);

      // Enter and Shift+Enter insert newlines and do not submit; the send
      // button submits all of it.
      await input.fill('first');
      await input.press('Enter');
      await input.pressSequentially('second');
      await input.press('Shift+Enter');
      await input.pressSequentially('third');
      await expect(input).toHaveValue('first\nsecond\nthird');
      await page.waitForTimeout(200);
      expect(submits.length).toBe(0);
      await page.locator('[data-prompt-submit]').click();
      await expect.poll(() => submits.length).toBe(1);
      expect(submits[0].text).toBe('first\nsecond\nthird');
    });

    test('L4: Sources is pressable while Texture is revising, and shows models and search providers', async ({ page }) => {
      await mockComputer(page, { windows: [textureWindow()], activeWindowId: 'win-ai' });
      const input = await openDesktop(page);
      await expect(page.locator('[data-prompt-target]')).toBeVisible({ timeout: 10_000 });
      await input.fill('add a section on agents');
      await page.locator('[data-prompt-submit]').click();
      const texture = page.locator('[data-window][data-window-app-id="texture"]').first();
      await expect(texture.locator('[data-texture-save-status]').first()).toContainText(/Writing first draft|Revising/, { timeout: 10_000 });

      const sources = texture.locator('[data-texture-source-panel]');
      await expect(sources).toBeEnabled();
      await sources.click();
      const panel = texture.locator('[data-texture-source-model-provenance]');
      await expect(panel).toBeVisible();
      await expect(texture.locator('[data-texture-source-model="deepseek/v4-flash"]')).toBeVisible();

      const search = texture.locator('[data-texture-source-search-provenance]');
      await expect(search).toBeVisible();
      await expect(search.locator('[data-texture-source-search-role="research_web"]')).toBeVisible();
      const keenable = search.locator('[data-texture-source-search-provider="keenable"]');
      await expect(keenable).toContainText('Keenable');
      await expect(keenable).toContainText('3 calls');
      await expect(keenable).toContainText('24 results');
      const exa = search.locator('[data-texture-source-search-provider="exa"]');
      await expect(exa).toContainText('Exa');
      await expect(exa).toContainText('1 unanswered');
      await shot(page, `after-${vp.name}-sources-revising`);

      // Publish may stay disabled while revising; the toolbar's Sources toggles closed again.
      await sources.click();
      await expect(texture.locator('[data-texture-source-model-provenance]')).toHaveCount(0);
    });
  });
}
