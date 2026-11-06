/**
 * The prompt bar builds on the Texture document in view (FE-02 step 1,
 * docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md),
 * and since slice H3 (plan section 11.1, R31) one chip names what the text
 * goes to for every app: the focused app agent and its object, the focused
 * app as a hint when it cannot take the text, or the conductor.
 * Runs against the built frontend (vite preview) with the session, the
 * desktop state, the Texture routes and /api/prompt-bar mocked; the server
 * side is covered by internal/textureowner/prompt_bar_continue_test.go.
 * Written before the chip.
 *
 * Ways it can fail (P11-P13 in the problem doc):
 *   C1 No chip, or the chip does not default to continue, when a Texture
 *      document window is active.
 *   C2 The continue submit does not carry focus.object_ref (was continue_doc_id) for that document,
 *      or changes the owner's text.
 *   C3 A continue opens a new Texture window instead of keeping the
 *      document's, or the window does not take the receipt.
 *   C4 One tap does not switch to "New document", or the override still
 *      sends the focus, or it leaks to the next document in view.
 *   C5 A chip shows, or a focus is sent, with no Texture document
 *      in view.
 *   C6 The chip adds a band or covers the prompt: the prompt surface grows,
 *      the chip leaves the command field, or the input is not usable, at
 *      desktop and phone width; or its tap target is under 40px tall
 *      (review 284 item 2).
 * Review 284 (written before the fixes):
 *   C7 A single URL with a document in view (superseded by the owner,
 *      2026-11-06): the pasted link goes to the focused document like any
 *      text, and a tap sends it to the conductor.
 *   C8 A quick second send while the first is routing goes out (and can
 *      continue the old document after a "New document" send).
 *   C9 A continue whose window was closed before the receipt came back
 *      reopens the document but the reopened window ignores the receipt.
 * Slice H3 (written before the chip redesign):
 *   H1 A package window's published focus does not reach the wire as
 *      { app_agent_id: app:<package>, object_ref }, or the chip does not name
 *      the app and its object.
 *   H2 A focus the package's desk cannot be the custodian of (another
 *      package's kind, a malformed ref, another app agent) is sent anyway:
 *      the chip must show the app as a hint and the text go to the conductor.
 *   H3 Clearing the chip still sends the focus, or no chip says the conductor
 *      routes when nothing is focused.
 *   H4 After a submit the bar does not show where the text went.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/prompt-bar-continue.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const JSON_HEADERS = { 'content-type': 'application/json' };

const DOCS = {
  'doc-tides': { title: 'Tide notes', content: '# Tide notes\n\nSpring tides follow the new and full moon.' },
  'doc-new': { title: 'Write about currents', content: 'Write about currents' },
};

function documentRecord(docId) {
  const doc = DOCS[docId] || { title: docId, content: '' };
  return {
    doc_id: docId,
    owner_id: 'owner-1',
    title: doc.title,
    current_revision_id: `${docId}-rev-1`,
    trajectory_id: '',
    created_at: '2026-10-26T00:00:00Z',
    updated_at: '2026-10-26T00:00:00Z',
  };
}

function revisionRecord(docId) {
  const doc = DOCS[docId] || { title: docId, content: '' };
  return {
    revision_id: `${docId}-rev-1`,
    doc_id: docId,
    author_kind: 'user',
    author_label: 'owner-1',
    content: doc.content,
    created_at: '2026-10-26T00:00:00Z',
  };
}

function textureWindow(windowId, docId, zIndex = 2) {
  return {
    window_id: windowId,
    app_id: 'texture',
    title: DOCS[docId].title,
    geometry: { x: 120, y: 80, width: 760, height: 520 },
    mode: 'normal',
    z_index: zIndex,
    app_context: { docId, windowTitle: DOCS[docId].title },
  };
}

// mockComputer serves a signed-in owner whose desktop holds the given
// windows. Every prompt-bar POST body is recorded in submits.
// gate, when given, holds every prompt-bar POST until it resolves.
async function mockComputer(page, { windows = [], activeWindowId = '', gate = null } = {}) {
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
      body = { computer_id: 'continue-test-computer' };
    } else if (path === '/api/desktop/state') {
      body = method === 'GET'
        ? { owner_id: 'owner-1', windows, active_window_id: activeWindowId, updated_at: '2026-10-26T00:00:00Z' }
        : { ok: true, updated_at: '2026-10-26T00:00:01Z' };
    } else if (path === '/api/prompt-bar' && method === 'POST') {
      const request = route.request().postDataJSON();
      submits.push(request);
      if (gate) await gate;
      status = 202;
      body = request.focus?.app_agent_id === 'texture'
        ? {
            schema: 'choir.prompt_bar_continue.v1',
            continued: true,
            command_id: request.command_id,
            doc_id: request.focus?.object_ref,
            trajectory_id: 'trajectory-next',
            revision_id: `${request.focus?.object_ref}-rev-2`,
            request_id: 'request-1',
            replay: false,
          }
        : request.focus
          ? { schema: 'choir.durable_work.v1', submission_id: 'submission-focus', state: 'completed', status_url: '/api/prompt-bar/submissions/submission-focus' }
          : { schema: 'choir.durable_work.v1', submission_id: 'submission-new', state: 'completed', status_url: '/api/prompt-bar/submissions/submission-new', doc_id: 'doc-new' };
    } else if (path === '/api/prompt-bar/submissions/submission-focus') {
      body = { submission_id: 'submission-focus', state: 'completed', decision: { action: 'open_app', app: 'files', title: 'Files' } };
    } else if (path === '/api/prompt-bar/submissions/submission-new') {
      body = {
        submission_id: 'submission-new',
        state: 'completed',
        decision: { action: 'open_app', app: 'texture', title: DOCS['doc-new'].title, doc_id: 'doc-new', seed_prompt: 'Write about currents', create_initial_version: false },
      };
    } else if (docMatch && !docMatch[2]) {
      body = documentRecord(decodeURIComponent(docMatch[1]));
    } else if (docMatch && docMatch[2] === '/revisions') {
      body = { revisions: [revisionRecord(decodeURIComponent(docMatch[1]))] };
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

async function send(page, text) {
  const input = page.locator('[data-prompt-input]');
  await input.fill(text);
  await input.press('Enter');
}

test('C1 C2 C3: the chip defaults to continuing the document in view', async ({ page }) => {
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides' });
  await openDesktop(page);

  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toBeVisible({ timeout: 10_000 });
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus');
  await expect(chip).toHaveAttribute('data-prompt-target-agent', 'texture');
  await expect(chip).toContainText('Texture');
  await expect(chip).toContainText('Tide notes');

  const typed = 'add a section on `neap tides` [1] and café';
  await send(page, typed);
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus).toEqual({ app_agent_id: 'texture', object_ref: 'doc-tides' });
  expect(submits[0].text).toBe(typed);

  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(1);
  await expect(page.locator('[data-window-tray-item][data-window-id="win-tides"]')).toHaveAttribute('data-window-tray-item-active', 'true');
  // The window took the receipt: its desk shows as working.
  await expect(page.locator('[data-window][data-window-app-id="texture"] [data-texture-save-status]').first()).toContainText(/Writing first draft|Revising/);
});

test('C4 H3: one tap clears the focus to the conductor, and the override does not leak', async ({ page }) => {
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides' });
  await openDesktop(page);

  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor');
  await expect(chip).toContainText('Conductor');
  await expect(chip).toHaveAttribute('aria-pressed', 'false');
  // A second tap restores the focus; a third clears it again.
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus');
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor');

  await send(page, 'Write about currents');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus).toBeUndefined();
  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(2, { timeout: 10_000 });
  // H4: the bar says where the text went.
  await expect(page.locator('[data-prompt-route]')).toContainText('Texture · Write about currents');

  // The new document is now in view; the chip defaults to it, not to the conductor.
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });
  await expect(chip).toContainText(DOCS['doc-new'].title);
});

test('C5 H3: with nothing focused the chip says the conductor routes, and no focus is sent', async ({ page }) => {
  const submits = await mockComputer(page);
  await openDesktop(page);
  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor', { timeout: 10_000 });
  await expect(chip).toContainText('Conductor');
  await send(page, 'Write about currents');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus).toBeUndefined();
  await expect(page.locator('[data-prompt-route]')).toContainText('Texture · Write about currents', { timeout: 10_000 });
});

function packageWindow(windowId, appId, focus, title = 'Field notes') {
  return {
    window_id: windowId,
    app_id: appId,
    title,
    geometry: { x: 120, y: 80, width: 760, height: 520 },
    mode: 'normal',
    z_index: 2,
    app_context: focus ? { focus } : {},
  };
}

test('H1: a package window\'s focus goes to its app agent and the chip names the app and object', async ({ page }) => {
  const focus = { app_agent_id: 'app:files', object_ref: 'obj:choir.app.files.folder:owner-1:field-notes', label: 'Field notes' };
  const submits = await mockComputer(page, { windows: [packageWindow('win-files', 'files', focus)], activeWindowId: 'win-files' });
  await openDesktop(page);
  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });
  await expect(chip).toHaveAttribute('data-prompt-target-agent', 'app:files');
  await expect(chip).toContainText('Files');
  await expect(chip).toContainText('Field notes');
  await send(page, 'list what changed this week');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus).toEqual({ app_agent_id: 'app:files', object_ref: 'obj:choir.app.files.folder:owner-1:field-notes' });
  await expect(page.locator('[data-prompt-route]')).toContainText('Files', { timeout: 10_000 });

  // H3: cleared, the same window's text goes to the conductor.
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor');
  await send(page, 'something else');
  await expect.poll(() => submits.length).toBe(2);
  expect(submits[1].focus).toBeUndefined();
});

for (const [name, focus] of [
  ['another package\'s kind', { app_agent_id: 'app:files', object_ref: 'obj:choir.app.mail.thread:owner-1:t1' }],
  ['another app agent', { app_agent_id: 'app:mail', object_ref: 'obj:choir.app.files.folder:owner-1:f1' }],
  ['a malformed ref', { app_agent_id: 'app:files', object_ref: 'folder-1' }],
  ['no focus', null],
]) {
  test(`H2: a package window with ${name} shows the app as a hint and the conductor routes`, async ({ page }) => {
    const submits = await mockComputer(page, { windows: [packageWindow('win-files', 'files', focus)], activeWindowId: 'win-files' });
    await openDesktop(page);
    const chip = page.locator('[data-command-field] [data-prompt-target]');
    await expect(chip).toHaveAttribute('data-prompt-target-mode', 'hint', { timeout: 10_000 });
    await expect(chip).toContainText('Files');
    await send(page, 'list what changed this week');
    await expect.poll(() => submits.length).toBe(1);
    expect(submits[0].focus).toBeUndefined();
  });
}

for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 844 }]) {
  test(`C6: the chip sits inside the command field at ${viewport.width}px`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await mockComputer(page);
    await openDesktop(page);
    const surface = page.locator('[data-prompt-surface]');
    const baseline = await surface.evaluate((el) => el.getBoundingClientRect().height);
    await page.unrouteAll({ behavior: 'ignoreErrors' });

    await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides' });
    await openDesktop(page);
    const chip = page.locator('[data-command-field] [data-prompt-target]');
    await expect(chip).toBeVisible({ timeout: 10_000 });
    const height = await surface.evaluate((el) => el.getBoundingClientRect().height);
    expect(Math.abs(height - baseline)).toBeLessThanOrEqual(1);

    const field = await page.locator('[data-command-field]').boundingBox();
    const chipBox = await chip.boundingBox();
    const inputBox = await page.locator('[data-prompt-input]').boundingBox();
    expect(chipBox.x).toBeGreaterThanOrEqual(field.x - 0.5);
    expect(chipBox.x + chipBox.width).toBeLessThanOrEqual(field.x + field.width + 0.5);
    expect(chipBox.y).toBeGreaterThanOrEqual(field.y - 0.5);
    expect(chipBox.y + chipBox.height).toBeLessThanOrEqual(field.y + field.height + 0.5);
    expect(chipBox.height).toBeGreaterThanOrEqual(40);
    // The input keeps most of the field and does not sit under the chip.
    expect(inputBox.width).toBeGreaterThan(field.width * 0.45);
    expect(inputBox.x).toBeGreaterThanOrEqual(chipBox.x + chipBox.width - 0.5);
    await page.locator('[data-prompt-input]').fill('still typeable');
    await expect(page.locator('[data-prompt-input]')).toHaveValue('still typeable');
  });
}

test('C7: a single URL goes to the focused document, and a tap sends it to the conductor', async ({ page }) => {
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides' });
  const input = await openDesktop(page);
  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });

  await input.fill('  https://example.com/tides/article  ');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus');
  await input.press('Enter');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus?.object_ref).toBe('doc-tides');

  // One send at a time with a target in view: wait for the first to route.
  await expect(page.locator('[data-prompt-surface]')).toHaveAttribute('data-routing', 'false');
  await input.fill('https://example.com/tides/article');
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor');
  await input.press('Enter');
  await expect.poll(() => submits.length).toBe(2);
  expect(submits[1].focus?.object_ref).toBeUndefined();
});

test('C8: no second send while the first is routing', async ({ page }) => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides', gate });
  const input = await openDesktop(page);
  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'focus', { timeout: 10_000 });
  await chip.click();
  await send(page, 'Write about currents');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus?.object_ref).toBeUndefined();

  await input.fill('and add a map');
  await input.press('Enter');
  await page.waitForTimeout(400);
  expect(submits.length).toBe(1);
  await expect(input).toHaveValue('and add a map');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'conductor');

  release();
  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(2, { timeout: 10_000 });
  await expect(chip).toContainText(DOCS['doc-new'].title);
  await input.press('Enter');
  await expect.poll(() => submits.length).toBe(2);
  expect(submits[1].focus?.object_ref).toBe('doc-new');
});

test('C9: a window closed before the receipt reopens and follows the continue', async ({ page }) => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides', gate });
  await openDesktop(page);
  await expect(page.locator('[data-command-field] [data-prompt-target]')).toBeVisible({ timeout: 10_000 });
  await send(page, 'add a section on neap tides');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].focus?.object_ref).toBe('doc-tides');

  await page.locator('[data-window][data-window-app-id="texture"] [data-window-close]').click();
  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(0);
  release();

  const reopened = page.locator('[data-window][data-window-app-id="texture"]');
  await expect(reopened).toHaveCount(1, { timeout: 10_000 });
  await expect(reopened.locator('[data-texture-save-status]').first()).toContainText(/Writing first draft|Revising/, { timeout: 10_000 });
});
