/**
 * The prompt bar builds on the Texture document in view (FE-02 step 1,
 * docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md).
 * Runs against the built frontend (vite preview) with the session, the
 * desktop state, the Texture routes and /api/prompt-bar mocked; the server
 * side is covered by internal/textureowner/prompt_bar_continue_test.go.
 * Written before the chip.
 *
 * Ways it can fail (P11-P13 in the problem doc):
 *   C1 No chip, or the chip does not default to continue, when a Texture
 *      document window is active.
 *   C2 The continue submit does not carry continue_doc_id for that document,
 *      or changes the owner's text.
 *   C3 A continue opens a new Texture window instead of keeping the
 *      document's, or the window does not take the receipt.
 *   C4 One tap does not switch to "New document", or the override still
 *      sends continue_doc_id, or it leaks to the next document in view.
 *   C5 A chip shows, or continue_doc_id is sent, with no Texture document
 *      in view.
 *   C6 The chip adds a band or covers the prompt: the prompt surface grows,
 *      the chip leaves the command field, or the input is not usable, at
 *      desktop and phone width.
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
      body = { computer_id: 'continue-test-computer' };
    } else if (path === '/api/desktop/state') {
      body = method === 'GET'
        ? { owner_id: 'owner-1', windows, active_window_id: activeWindowId, updated_at: '2026-10-26T00:00:00Z' }
        : { ok: true, updated_at: '2026-10-26T00:00:01Z' };
    } else if (path === '/api/prompt-bar' && method === 'POST') {
      const request = route.request().postDataJSON();
      submits.push(request);
      status = 202;
      body = request.continue_doc_id
        ? {
            schema: 'choir.prompt_bar_continue.v1',
            continued: true,
            command_id: request.command_id,
            doc_id: request.continue_doc_id,
            trajectory_id: 'trajectory-next',
            revision_id: `${request.continue_doc_id}-rev-2`,
            request_id: 'request-1',
            replay: false,
          }
        : { schema: 'choir.durable_work.v1', submission_id: 'submission-new', state: 'completed', status_url: '/api/prompt-bar/submissions/submission-new', doc_id: 'doc-new' };
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
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'continue');
  await expect(chip).toContainText('Continue');
  await expect(chip).toContainText('Tide notes');

  const typed = 'add a section on `neap tides` [1] and café';
  await send(page, typed);
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].continue_doc_id).toBe('doc-tides');
  expect(submits[0].text).toBe(typed);

  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(1);
  await expect(page.locator('[data-window-tray-item][data-window-id="win-tides"]')).toHaveAttribute('data-window-tray-item-active', 'true');
  // The window took the receipt: its desk shows as working.
  await expect(page.locator('[data-window][data-window-app-id="texture"] [data-texture-save-status]').first()).toContainText(/Writing first draft|Revising/);
});

test('C4: one tap switches to a new document, and the override does not leak', async ({ page }) => {
  const submits = await mockComputer(page, { windows: [textureWindow('win-tides', 'doc-tides')], activeWindowId: 'win-tides' });
  await openDesktop(page);

  const chip = page.locator('[data-command-field] [data-prompt-target]');
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'continue', { timeout: 10_000 });
  await chip.click();
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'new');
  await expect(chip).toContainText('New document');
  await expect(chip).toHaveAttribute('aria-pressed', 'false');

  await send(page, 'Write about currents');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].continue_doc_id).toBeUndefined();
  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(2, { timeout: 10_000 });

  // The new document is now in view; the chip defaults to it, not to "new".
  await expect(chip).toHaveAttribute('data-prompt-target-mode', 'continue', { timeout: 10_000 });
  await expect(chip).toContainText(DOCS['doc-new'].title);
});

test('C5: no chip and no continue without a Texture document in view', async ({ page }) => {
  const submits = await mockComputer(page);
  await openDesktop(page);
  await expect(page.locator('[data-prompt-target]')).toHaveCount(0);
  await send(page, 'Write about currents');
  await expect.poll(() => submits.length).toBe(1);
  expect(submits[0].continue_doc_id).toBeUndefined();
});

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
    // The input keeps most of the field and does not sit under the chip.
    expect(inputBox.width).toBeGreaterThan(field.width * 0.45);
    expect(inputBox.x).toBeGreaterThanOrEqual(chipBox.x + chipBox.width - 0.5);
    await page.locator('[data-prompt-input]').fill('still typeable');
    await expect(page.locator('[data-prompt-input]')).toHaveValue('still typeable');
  });
}
