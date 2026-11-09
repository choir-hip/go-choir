/**
 * The prompt bar dock (owner, 2026-11-09: "the window button/icons in the
 * dock/prompt bar ... aren't well aligned and their behavior and visibility
 * is odd ... redesign the whole prompt bar to work better and more
 * intuitively"). One 44px grid, window buttons that focus, minimise and
 * restore, a target pill that names only the desk, and one window list
 * popover (the desktop +N and the phone windows button). Runs against the
 * built frontend (vite preview) with the session, desktop state and Texture
 * routes mocked.
 *
 * Ways it can fail:
 *   D1 A control is not 44px, sits off the shared centre line, overlaps its
 *      neighbour or leaves the bar; the input is too narrow on a phone.
 *   D2 A window button does not show focused, open or minimised.
 *   D3 A click does not focus, minimise or restore; a middle click acts.
 *   D4 Overflow hides the focused window, loses titles, or the popover does
 *      not list, restore, close or return focus.
 *   D5 The pill repeats the tray, names the wrong desk or is wrongly active.
 *   D6 The phone hides the input behind a switcher, or the windows button
 *      fails to list and switch windows.
 *   D7 The tab order, roving arrows, focus ring or Enter rule is wrong.
 *   D8 Multiline text or a route line moves a control off the bottom row.
 *   D9 A theme hides the state marks or the focus ring.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/prompt-bar-dock.spec.js
 *   (PROMPT_BAR_SHOT_DIR=/some/dir saves after-{viewport}-{fixture}.png)
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const JSON_HEADERS = { 'content-type': 'application/json' };
const SHOT_DIR = process.env.PROMPT_BAR_SHOT_DIR || '';
const DOC = { id: 'doc-ai', title: "What's latest in ai", content: "# What's latest in ai\n\nNotes." };

function win(id, app, title, mode, z, x = 40, y = 40, ctx = {}) {
  return { window_id: id, app_id: app, title, geometry: { x, y, width: 640, height: 420 }, mode, z_index: z, app_context: ctx };
}

const FIXTURES = {
  w0: { windows: [], active: '' },
  w1: { windows: [win('win-set', 'settings', 'Settings', 'normal', 2)], active: 'win-set' },
  w4: {
    windows: [
      win('win-files', 'files', 'Files', 'normal', 2, 60, 60),
      win('win-set', 'settings', 'Settings', 'normal', 3, 120, 90),
      win('win-mail', 'mail', 'Mail', 'minimized', 1, 200, 120),
      win('win-ai', 'texture', DOC.title, 'normal', 5, 260, 60, { docId: DOC.id, windowTitle: DOC.title }),
    ],
    active: 'win-ai',
  },
  w10: {
    windows: Array.from({ length: 10 }, (_, i) => {
      const app = ['files', 'settings', 'mail'][i % 3];
      const name = app[0].toUpperCase() + app.slice(1);
      return win(`win-${i}`, app, `${name} window ${i}`, i === 2 ? 'minimized' : 'normal', i + 2, 30 + i * 24, 30 + i * 16);
    }),
    active: 'win-9',
  },
};

async function mockComputer(page, { windows = [], active = '' } = {}) {
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
      body = { computer_id: 'dock-test-computer' };
    } else if (path === '/api/desktop/state') {
      body = method === 'GET'
        ? { owner_id: 'owner-1', windows, active_window_id: active, updated_at: '2026-11-09T00:00:00Z' }
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
    } else if (docMatch && !docMatch[2]) {
      body = {
        doc_id: DOC.id, owner_id: 'owner-1', title: DOC.title, current_revision_id: `${DOC.id}-rev-1`,
        trajectory_id: '', created_at: '2026-11-09T00:00:00Z', updated_at: '2026-11-09T00:00:00Z',
      };
    } else if (docMatch && docMatch[2] === '/revisions') {
      body = {
        revisions: [{
          revision_id: `${DOC.id}-rev-1`, doc_id: DOC.id, author_kind: 'user', author_label: 'owner-1',
          content: DOC.content, created_at: '2026-11-09T00:00:00Z',
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
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();
  return input;
}

function overlaps(a, b) {
  return a.x < b.x + b.width - 0.5 && b.x < a.x + a.width - 0.5 && a.y < b.y + b.height - 0.5 && b.y < a.y + a.height - 0.5;
}

async function shot(page, name) {
  if (SHOT_DIR) await page.screenshot({ path: `${SHOT_DIR}/${name}.png` });
}

const ROW_SELECTOR = [
  '[data-desk-menu-button]',
  '[data-window-tray-item]',
  '[data-window-overflow]',
  '[data-window-switcher-button]',
  '[data-prompt-target]',
  '[data-prompt-input]',
  '[data-agent-audio-button]',
  '[data-prompt-submit]',
].join(', ');

// Every row control's box, the bar's box and the icon boxes.
async function rowBoxes(page) {
  return page.evaluate((selector) => {
    const box = (el) => {
      const b = el.getBoundingClientRect();
      return { x: b.x, y: b.y, width: b.width, height: b.height, bottom: b.bottom, cy: b.y + b.height / 2 };
    };
    const surface = document.querySelector('[data-prompt-surface]');
    const cs = getComputedStyle(surface);
    return {
      bar: { ...box(surface), border: parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth), borderBottom: parseFloat(cs.borderBottomWidth) },
      controls: [...surface.querySelectorAll(selector)].map((el) => ({
        name: el.getAttribute('data-window-id') || el.getAttributeNames().find((n) => n.startsWith('data-') && !n.startsWith('data-window-') && !n.startsWith('data-stop')) || el.tagName,
        isInput: el.matches('[data-prompt-input]'),
        isWindowButton: el.matches('[data-window-tray-item]'),
        box: box(el),
        icon: el.matches('[data-window-tray-item]') ? box(el.querySelector('.window-tray-icon')) : null,
      })),
    };
  }, ROW_SELECTOR);
}

async function states(page) {
  return page.evaluate(() => Object.fromEntries(
    [...document.querySelectorAll('[data-window-tray-item]')].map((el) => [el.dataset.windowId, el.dataset.windowState]),
  ));
}

async function assertAlignment(page, vpName, { emptyField = true } = {}) {
  const { bar, controls } = await rowBoxes(page);
  const submit = controls.find((c) => c.name === 'data-prompt-submit');
  expect(submit, 'submit is always shown').toBeTruthy();
  // 44 + 2 x 6 of padding, plus the border some themes draw on the bar.
  expect(Math.abs(bar.height - (56 + bar.border))).toBeLessThanOrEqual(1);
  for (const c of controls) {
    expect(Math.abs(c.box.cy - submit.box.cy), `${c.name} centre y`).toBeLessThanOrEqual(1);
    if (emptyField || !c.isInput) expect(Math.abs(c.box.height - 44), `${c.name} height`).toBeLessThanOrEqual(0.5);
    expect(c.box.x).toBeGreaterThanOrEqual(bar.x - 0.5);
    expect(c.box.x + c.box.width).toBeLessThanOrEqual(bar.x + bar.width + 0.5);
    expect(c.box.y).toBeGreaterThanOrEqual(bar.y - 0.5);
    expect(c.box.y + c.box.height).toBeLessThanOrEqual(bar.y + bar.height + 0.5);
    if (c.icon) expect(Math.abs(c.icon.width - 20)).toBeLessThanOrEqual(0.5);
    if (c.isInput && vpName === 'phone') expect(c.box.width).toBeGreaterThanOrEqual(110);
  }
  for (let i = 0; i < controls.length; i += 1) {
    for (let j = i + 1; j < controls.length; j += 1) {
      expect(overlaps(controls[i].box, controls[j].box), `${controls[i].name} overlaps ${controls[j].name}`).toBe(false);
    }
  }
  return { bar, controls };
}

const VIEWPORTS = [
  { name: 'desktop', width: 1280, height: 800 },
  { name: 'phone', width: 390, height: 844 },
];

for (const vp of VIEWPORTS) {
  test.describe(`prompt bar dock at ${vp.name} ${vp.width}x${vp.height}`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });
    const phone = vp.name === 'phone';

    for (const key of ['w0', 'w1', 'w4']) {
      test(`D1 alignment (${key})`, async ({ page }) => {
        await mockComputer(page, FIXTURES[key]);
        await openDesktop(page);
        await expect(page.locator('[data-window-tray-item], [data-window-switcher-button]')).toHaveCount(
          key === 'w0' ? 0 : phone ? 1 : FIXTURES[key].windows.length,
        );
        await page.waitForTimeout(400);
        await assertAlignment(page, vp.name);
        await shot(page, `after-${vp.name}-${key}`);
      });
    }

    test('D1 the submit is always shown: empty, disabled-free, with a popover and with the sheet open', async ({ page }) => {
      await mockComputer(page, FIXTURES.w4);
      await openDesktop(page);
      await expect(page.locator('[data-prompt-submit]')).toBeVisible();
      await page.locator('[data-desk-menu-button]').click();
      await expect(page.locator('[data-desk-sheet]')).toBeVisible();
      await expect(page.locator('[data-prompt-submit]')).toBeVisible();
      await expect(page.locator('[data-prompt-input]')).toBeVisible();
    });

    test('D5 the pill names where the text goes, and never repeats the tray', async ({ page }) => {
      const label = (text) => (phone ? text : `To ${text}`);
      // W1: a focused app with no desk; the conductor routes.
      await mockComputer(page, FIXTURES.w1);
      await openDesktop(page);
      const pill = page.locator('[data-command-field] [data-prompt-target]');
      const pillLabel = pill.locator('.prompt-target-label');
      await expect(pillLabel).toHaveText(label('Conductor'));
      await expect(pill).toBeDisabled();
      await expect(pill).toHaveAttribute('data-prompt-target-mode', 'hint');
      const texts = await page.locator('[data-command-field]').evaluate((root) => {
        const out = [];
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        while (walker.nextNode()) out.push(walker.currentNode.textContent.trim());
        return out;
      });
      expect(texts).not.toContain('Settings');
      const placeholder = await page.locator('[data-prompt-input]').getAttribute('placeholder');
      if (phone) expect(placeholder).toBe('Message');
      else expect(placeholder).toMatch(/Enter for a new line/);
    });

    test('D5 the pill (focus and W0)', async ({ page }) => {
      const label = (text) => (phone ? text : `To ${text}`);
      await mockComputer(page, FIXTURES.w4);
      await openDesktop(page);
      const pill = page.locator('[data-command-field] [data-prompt-target]');
      const pillLabel = pill.locator('.prompt-target-label');
      await expect(pill).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });
      await expect(pillLabel).toHaveText(label('Texture'));
      await expect(pill).toHaveAttribute('aria-pressed', 'true');
      await expect(pill.locator('.prompt-target-pill')).not.toContainText("What's latest in ai");
      await expect(pill).toHaveAttribute('title', /What's latest in ai/);
      await pill.click();
      await expect(pillLabel).toHaveText(label('Conductor'));
      await expect(pill).toHaveAttribute('aria-pressed', 'false');
      await pill.click();
      await expect(pillLabel).toHaveText(label('Texture'));

      const empty = await browserFreshPage(page);
      await mockComputer(empty, FIXTURES.w0);
      await openDesktop(empty);
      await expect(empty.locator('[data-prompt-target] .prompt-target-label')).toHaveText(label('Conductor'));
      await expect(empty.locator('[data-prompt-target]')).toBeDisabled();
    });

    test('D8 multiline text and a route line keep the bottom row aligned', async ({ page }) => {
      const submits = await mockComputer(page, FIXTURES.w4);
      const input = await openDesktop(page);
      await page.waitForTimeout(400);
      const before = await rowBoxes(page);
      const pillBefore = before.controls.find((c) => c.name === 'data-prompt-target').box.width;
      await input.fill(Array.from({ length: 6 }, (_, i) => `line ${i + 1}`).join('\n'));
      await page.waitForTimeout(300);
      const after = await rowBoxes(page);
      const submit = after.controls.find((c) => c.name === 'data-prompt-submit');
      for (const c of after.controls) {
        expect(Math.abs(c.box.bottom - submit.box.bottom), `${c.name} bottom`).toBeLessThanOrEqual(1);
        // The grown input is the multiline control; the rest sit on the bottom row.
        if (!c.isInput) expect(Math.abs(c.box.cy - (after.bar.y + after.bar.height - after.bar.borderBottom - 6 - 22)), `${c.name} centre`).toBeLessThanOrEqual(1);
      }
      // The input is the multiline control: it grew, and the bar's bottom did not move.
      const inputBox = after.controls.find((c) => c.isInput).box;
      expect(inputBox.height).toBeGreaterThan(80);
      expect(Math.abs(after.bar.y + after.bar.height - (before.bar.y + before.bar.height))).toBeLessThanOrEqual(1);
      const pillAfter = after.controls.find((c) => c.name === 'data-prompt-target').box.width;
      expect(Math.abs(pillAfter - pillBefore)).toBeLessThanOrEqual(1);
      await shot(page, `after-${vp.name}-multiline`);

      // A send makes a route line appear inside the field, above the textarea.
      await input.press('ControlOrMeta+Enter');
      await expect.poll(() => submits.length).toBe(1);
      const route = page.locator('[data-prompt-route]');
      await expect(route).toBeVisible({ timeout: 10_000 });
      const geometry = await page.evaluate(() => {
        const field = document.querySelector('[data-command-field]').getBoundingClientRect();
        const r = document.querySelector('[data-prompt-route]').getBoundingClientRect();
        const t = document.querySelector('[data-prompt-input]').getBoundingClientRect();
        return { field, r, t };
      });
      expect(geometry.r.top).toBeGreaterThanOrEqual(geometry.field.top - 0.5);
      expect(geometry.r.bottom).toBeLessThanOrEqual(geometry.t.top + 0.5);
      const routed = await rowBoxes(page);
      const routedSubmit = routed.controls.find((c) => c.name === 'data-prompt-submit');
      for (const c of routed.controls) {
        expect(Math.abs(c.box.bottom - routedSubmit.box.bottom), `${c.name} bottom`).toBeLessThanOrEqual(1);
      }
    });
  });
}

// A second page for the W0 half of D5 in the same viewport.
async function browserFreshPage(page) {
  const fresh = await page.context().newPage();
  await fresh.setViewportSize(page.viewportSize());
  return fresh;
}

test.describe('prompt bar dock, desktop only', () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test('D2 states: focused, open and minimised are drawn and named', async ({ page }) => {
    await mockComputer(page, FIXTURES.w4);
    await openDesktop(page);
    await page.waitForTimeout(400);
    const button = (id) => page.locator(`[data-window-tray-item][data-window-id="${id}"]`);
    await expect(button('win-ai')).toHaveAttribute('data-window-state', 'focused');
    await expect(button('win-ai')).toHaveAttribute('aria-pressed', 'true');
    await expect(button('win-mail')).toHaveAttribute('data-window-state', 'minimized');
    await expect(button('win-files')).toHaveAttribute('data-window-state', 'open');
    await expect(button('win-set')).toHaveAttribute('data-window-state', 'open');
    await expect(button('win-mail')).toHaveAttribute('aria-label', /, minimised$/);
    await assertMarks(page);
  });

  test('D3 click focuses, minimises and restores; a middle click does nothing', async ({ page }) => {
    await mockComputer(page, FIXTURES.w4);
    await openDesktop(page);
    await page.waitForTimeout(400);
    const button = (id) => page.locator(`[data-window-tray-item][data-window-id="${id}"]`);
    const winEl = (id) => page.locator(`[data-window][data-window-id="${id}"]`);

    await button('win-files').click();
    await expect(button('win-files')).toHaveAttribute('data-window-state', 'focused');
    await expect(button('win-ai')).toHaveAttribute('data-window-state', 'open');

    await button('win-files').click();
    await expect(winEl('win-files')).not.toBeVisible();
    await expect(button('win-files')).toHaveAttribute('data-window-state', 'minimized');
    await expect(page.locator('[data-window-tray-item][data-window-state="focused"]')).toHaveCount(1);
    expect(await button('win-files').getAttribute('data-window-state')).not.toBe('focused');

    await button('win-files').click();
    await expect(winEl('win-files')).toBeVisible();
    await expect(button('win-files')).toHaveAttribute('data-window-state', 'focused');

    await button('win-mail').click();
    await expect(winEl('win-mail')).toBeVisible();
    await expect(button('win-mail')).toHaveAttribute('data-window-state', 'focused');

    const before = await states(page);
    await button('win-set').click({ button: 'middle' });
    await page.waitForTimeout(250);
    expect(await states(page)).toEqual(before);
  });

  test('D4 overflow: +N, the focused window stays, and the popover lists, restores and closes', async ({ page }) => {
    await mockComputer(page, FIXTURES.w10);
    await openDesktop(page);
    await page.waitForTimeout(500);
    const overflow = page.locator('[data-window-overflow]');
    await expect(overflow).toBeVisible();
    const hidden = Number((await overflow.textContent()).trim().replace('+', ''));
    expect(hidden).toBeGreaterThan(0);
    const items = page.locator('[data-window-tray-item]');
    expect(await items.count() + hidden).toBe(10);
    await expect(page.locator('[data-window-tray-item][data-window-id="win-9"]')).toBeVisible();
    for (const item of await items.all()) {
      const box = await item.boundingBox();
      expect(box.width).toBeGreaterThanOrEqual(111.5);
      const title = await item.locator('.window-tray-title').boundingBox();
      expect(title.width).toBeGreaterThan(40);
      expect((await item.locator('.window-tray-title').textContent()).trim()).not.toBe('');
    }
    expect((await page.locator('[data-command-field]').boundingBox()).width).toBeGreaterThanOrEqual(319.5);
    await assertAlignment(page, 'desktop');
    await shot(page, 'after-desktop-w8');

    await overflow.click();
    const popover = page.locator('[data-window-switcher]');
    await expect(popover).toBeVisible();
    await expect(page.locator('[data-window-switcher-item]')).toHaveCount(10);
    await page.keyboard.press('Escape');
    await expect(popover).toHaveCount(0);
    await expect(overflow).toBeFocused();

    await overflow.click();
    const mailRow = page.locator('[data-window-switcher-item][data-window-id="win-2"]');
    await expect(mailRow).toHaveAttribute('data-window-state', 'minimized');
    await mailRow.click();
    await expect(popover).toHaveCount(0);
    await expect(page.locator('[data-window][data-window-id="win-2"]')).toBeVisible();
    await expect(page.locator('[data-window][data-window-id="win-2"]')).toHaveAttribute('data-window-active', 'true');

    await page.locator('[data-window-overflow]').click();
    await page.locator('[data-window-switcher-close]').nth(3).click();
    await expect(page.locator('[data-window-switcher-item]')).toHaveCount(9);
  });

  test('D7 keyboard order, roving arrows, focus ring, and the Enter rule', async ({ page }) => {
    const submits = await mockComputer(page, FIXTURES.w4);
    const input = await openDesktop(page);
    await page.waitForTimeout(400);
    const accent = await page.evaluate(() => {
      const probe = document.createElement('div');
      probe.style.color = 'var(--choir-accent)';
      document.body.appendChild(probe);
      const color = getComputedStyle(probe).color;
      probe.remove();
      return color;
    });
    const ring = () => page.evaluate(() => {
      const cs = getComputedStyle(document.activeElement);
      return { style: cs.outlineStyle, width: cs.outlineWidth, color: cs.outlineColor };
    });
    await page.locator('[data-desk-menu-button]').focus();
    await page.keyboard.press('Tab'); // focus-visible needs a keyboard focus
    const order = [
      'tray:win-ai',
      'pill',
      'textarea',
      'voice',
      'submit',
    ];
    const where = () => page.evaluate(() => {
      const el = document.activeElement;
      if (el.matches('[data-window-tray-item]')) return `tray:${el.dataset.windowId}`;
      if (el.matches('[data-prompt-target]')) return 'pill';
      if (el.matches('[data-prompt-input]')) return 'textarea';
      if (el.matches('[data-agent-audio-button]')) return 'voice';
      if (el.matches('[data-prompt-submit]')) return 'submit';
      if (el.matches('[data-desk-menu-button]')) return 'mark';
      return el.tagName;
    });
    expect(await where()).toBe(order[0]);
    expect(await ring()).toEqual({ style: 'solid', width: '2px', color: accent });
    // Arrows move between the window buttons.
    await page.keyboard.press('ArrowLeft');
    expect(await where()).not.toBe('tray:win-ai');
    expect((await where()).startsWith('tray:')).toBe(true);
    expect(await ring()).toEqual({ style: 'solid', width: '2px', color: accent });
    await page.keyboard.press('ArrowRight');
    expect(await where()).toBe('tray:win-ai');
    for (const stop of order.slice(1)) {
      await page.keyboard.press('Tab');
      expect(await where()).toBe(stop);
      if (stop === 'textarea') {
        const shadow = await page.locator('[data-command-field]').evaluate((el) => getComputedStyle(el).boxShadow);
        expect(shadow).toMatch(/ 2px(?! \d)|\) 0px 0px 0px 2px| 0px 2px(?=,|$)/);
      } else {
        expect(await ring()).toEqual({ style: 'solid', width: '2px', color: accent });
      }
    }

    await input.focus();
    await input.type('one');
    await input.press('Enter');
    await input.type('two');
    await expect(input).toHaveValue('one\ntwo');
    await page.waitForTimeout(200);
    expect(submits.length).toBe(0);
    await input.press('ControlOrMeta+Enter');
    await expect.poll(() => submits.length).toBe(1);
  });
});

test.describe('prompt bar dock, phone only', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('D6 one window list, and the input is never hidden', async ({ page }) => {
    await mockComputer(page, FIXTURES.w0);
    await openDesktop(page);
    await expect(page.locator('[data-window-tray]')).toHaveCount(0);
    await expect(page.locator('[data-window-switcher-button]')).toHaveCount(0);

    const fresh = await browserFreshPage(page);
    await mockComputer(fresh, FIXTURES.w4);
    await openDesktop(fresh);
    await expect(fresh.locator('[data-window-tray]')).toHaveCount(0);
    await expect(fresh.locator('[data-window-switcher-button] [data-window-count-label]')).toHaveText('4');
    await fresh.locator('[data-window-switcher-button]').click();
    await expect(fresh.locator('[data-window-switcher]')).toBeVisible();
    await expect(fresh.locator('[data-window-switcher-item]')).toHaveCount(4);
    await expect(fresh.locator('[data-prompt-input]')).toBeVisible();
    await expect(fresh.locator('[data-prompt-input]')).toBeEnabled();
    await expect(fresh.locator('[data-prompt-submit]')).toBeVisible();
    await expect(fresh.locator('[data-desk-sheet]')).toHaveCount(0);
    await shot(fresh, 'after-phone-w4-switcher');
    for (const row of await fresh.locator('[data-window-switcher-item]').all()) {
      expect((await row.boundingBox()).height).toBeGreaterThanOrEqual(44);
    }
    for (const close of await fresh.locator('[data-window-switcher-close]').all()) {
      expect((await close.boundingBox()).height).toBeGreaterThanOrEqual(44);
    }
    const popover = await fresh.locator('[data-window-switcher]').boundingBox();
    const bar = await fresh.locator('[data-prompt-surface]').boundingBox();
    expect(popover.y + popover.height).toBeLessThanOrEqual(bar.y + 0.5);

    await fresh.locator('[data-window-switcher-item][data-window-id="win-mail"]').click();
    await expect(fresh.locator('[data-window][data-window-id="win-mail"]')).toBeVisible();
    await expect(fresh.locator('[data-window][data-window-id="win-mail"]')).toHaveAttribute('data-window-active', 'true');
    await expect(fresh.locator('[data-window-switcher]')).toHaveCount(0);

    await fresh.locator('[data-window-switcher-button]').click();
    await expect(fresh.locator('[data-window-switcher]')).toBeVisible();
    await fresh.locator('[data-desk-menu-button]').click();
    await expect(fresh.locator('[data-desk-sheet]')).toBeVisible();
    await expect(fresh.locator('[data-window-switcher]')).toHaveCount(0);
    await expect(fresh.locator('[data-prompt-input]')).toBeVisible();
  });
});

// D9: the marks and the ring in the shipped themes and an OLED black fixture.
const OLED = {
  schema_version: 2,
  id: 'oled-black-test',
  name: 'OLED test',
  colors: {
    bg: '#000000', bg2: '#000000', panel: '#000000', panelStrong: '#000000', panelSoft: '#0a0a0a', inputBg: '#000000',
    controlBg: '#000000', promptSurfaceBg: '#000000', sheetBg: '#000000', selected: 'rgba(255,255,255,0.12)',
    fg: '#f2f2f2', muted: '#8c8c8c', accent: '#4da3ff', border: 'rgba(255,255,255,0.18)',
    borderStrong: 'rgba(255,255,255,0.32)', success: '#20d686', onAccent: '#000000',
  },
};
const THEMES = [
  { schema_version: 2, id: 'london-salmon', name: 'London Salmon' },
  { schema_version: 2, id: 'frutiger-aero', name: 'Frutiger Aero' },
  { schema_version: 2, id: 'futuristic-noir', name: 'Futuristic Noir' },
  OLED,
];

async function assertMarks(page) {
  const marks = await page.evaluate(() => {
    const accent = (() => {
      const probe = document.createElement('div');
      probe.style.color = 'var(--choir-accent)';
      document.body.appendChild(probe);
      const color = getComputedStyle(probe).color;
      probe.remove();
      return color;
    })();
    const after = (id) => {
      const el = document.querySelector(`[data-window-tray-item][data-window-id="${id}"]`);
      const cs = getComputedStyle(el, '::after');
      return { content: cs.content, width: cs.width, height: cs.height, background: cs.backgroundColor, radius: cs.borderRadius };
    };
    const iconOpacity = (id) => getComputedStyle(document.querySelector(`[data-window-tray-item][data-window-id="${id}"] .window-tray-icon`)).opacity;
    const bar = getComputedStyle(document.querySelector('[data-prompt-surface]')).backgroundColor;
    return { accent, focused: after('win-ai'), open: after('win-files'), minimized: after('win-mail'), mailIcon: iconOpacity('win-mail'), fileIcon: iconOpacity('win-files'), bar };
  });
  expect(marks.focused.width).toBe('16px');
  expect(marks.focused.height).toBe('3px');
  expect(marks.focused.background).toBe(marks.accent);
  expect(marks.open.width).toBe('4px');
  expect(marks.open.height).toBe('4px');
  expect(marks.minimized.content).toBe('none');
  expect(marks.mailIcon).toBe('0.5');
  expect(marks.fileIcon).toBe('1');
  return marks;
}

for (const vp of VIEWPORTS) {
  test.describe(`D9 themes at ${vp.name}`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });
    for (const theme of THEMES) {
      test(`${theme.id}: alignment, marks and ring`, async ({ page }) => {
        await mockComputer(page, FIXTURES.w4);
        await openDesktop(page);
        await page.evaluate((detail) => {
          window.dispatchEvent(new CustomEvent('choir-theme-change', { detail: { theme: detail } }));
        }, theme);
        await page.waitForTimeout(600);
        await assertAlignment(page, vp.name);
        if (vp.name === 'desktop') {
          const marks = await assertMarks(page);
          expect(marks.focused.background).not.toBe(marks.bar);
          await page.locator('[data-desk-menu-button]').focus();
          await page.keyboard.press('Tab');
          const ring = await page.evaluate(() => {
            const cs = getComputedStyle(document.activeElement);
            return { style: cs.outlineStyle, width: cs.outlineWidth };
          });
          expect(ring).toEqual({ style: 'solid', width: '2px' });
        } else {
          await page.locator('[data-desk-menu-button]').focus();
          await page.keyboard.press('Tab');
          const ring = await page.evaluate(() => {
            const cs = getComputedStyle(document.activeElement);
            return { style: cs.outlineStyle, width: cs.outlineWidth };
          });
          expect(ring).toEqual({ style: 'solid', width: '2px' });
        }
      });
    }
  });
}
