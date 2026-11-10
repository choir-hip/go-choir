/**
 * The prompt bar as the universal input (lane D, handoff 4.D; section 6 items
 * 2 and 3), against the built frontend with the routes mocked. The bar shows
 * what it is aimed at (an icon, a verb placeholder, the revision answered),
 * Jev classifies the text (POST /api/prompt-bar/classify), the class is shown,
 * and one tap sends the text again under another class. On a document the
 * owner owns, an edit or a comment is the one focused submit; on a published
 * story the reader does not own, a comment is a fork (POST /api/texture/fork)
 * and the fork opens as the reader's own document.
 *
 * Ways it can fail (docs/evidence/prompt-bar-universal-input-failures-2026-11-10.md):
 *   A1 the bar shows no aim, or "Conductor" anywhere;
 *   A2 the classify fails and the text is not sent (it must take the default);
 *   A3 a comment on a published story writes to the story instead of forking;
 *   A4 the class is not shown, or the one-tap re-send reuses the first command id;
 *   A7 a re-send is offered after an owner revision committed (a second identical write);
 *   A8 A9 a request kind does not reach the ask and source routes, or hides a 422's code and reason;
 *   A5 a request goes to the document (it must go to the agent, with no focus);
 *   A6 the revision shown or sent is the head when the window reads an older one.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/prompt-bar-aim.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');
const JSON_HEADERS = { 'content-type': 'application/json' };

function revision(docId, n) {
  return {
    revision_id: `${docId}-rev-${n}`, doc_id: docId, author_kind: 'user', author_label: 'owner-1',
    content: `# Tide notes\n\nVersion ${n}.`, version_number: n, created_at: `2026-11-0${n}T00:00:00Z`,
  };
}

function story(slug, title) {
  const route = `/pub/texture/${slug}`;
  const text = Array.from({ length: 6 }, (_, i) => `Paragraph ${i + 1} of ${title}. The harbor reopened at dawn.`).join('\n\n');
  return {
    route: { path: route, state: 'active', bare: route, permalink: `${route}/v1` },
    publication: { id: `pub-${slug}`, title, slug, state: 'published' },
    version: { id: `ver-${slug}-1`, content_hash: `hash-${slug}`, manifest_hash: `manifest-${slug}` },
    artifact: { content: text, body_doc: null },
    retrieval: {}, citations: [], source_entities: [], transclusions: [], policy: {}, proposals: {}, provenance: {},
    payload: { revision: 1, manifest_hash: `manifest-${slug}`, manifest: '{}', resolved_blocks: [] },
    nav: { revision: 1, prev: 0, next: 0, latest: 1 },
    checks: { source: 'pending', citations: [] },
  };
}

// mockComputer serves a signed-in owner whose desktop holds one Texture window.
// classify answers {class} (or fails when classify is null). Every prompt-bar,
// classify and fork body is recorded.
async function mockComputer(page, { appContext, title, classify = { class: 'edit', rule: 'judge', alternatives: ['comment', 'request'] }, asks = [], schedules = [], scheduleAnswer = { status: 201, body: { source_id: 's1' } } }) {
  const seen = { submits: [], classifies: [], forks: [] };
  await page.route('**/auth/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const body = pathname === '/auth/session' ? { authenticated: true, user: { id: 'owner-1', email: 'owner@example.com' } } : { ok: true };
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
      body = { computer_id: 'aim-test-computer' };
    } else if (path === '/api/desktop/state') {
      body = method === 'GET'
        ? {
            owner_id: 'owner-1', active_window_id: 'win-1', updated_at: '2026-11-10T00:00:00Z',
            windows: [{ window_id: 'win-1', app_id: 'texture', title, geometry: { x: 120, y: 80, width: 760, height: 520 }, mode: 'normal', z_index: 2, app_context: appContext }],
          }
        : { ok: true, updated_at: '2026-11-10T00:00:01Z' };
    } else if (path === '/api/autopaper/ask') {
      asks.push(route.request().postDataJSON());
      status = 202;
      body = { schema: 'choir.durable_work.v1', submission_id: 'submission-ask', state: 'running', doc_id: 'doc-fork', command_id: 'c' };
    } else if (path === '/api/autopaper/sources/schedules' && method === 'POST') {
      schedules.push(route.request().postDataJSON());
      status = scheduleAnswer.status;
      body = scheduleAnswer.body;
    } else if (path === '/api/prompt-bar/classify') {
      seen.classifies.push(route.request().postDataJSON());
      if (!classify) { await route.fulfill({ status: 500, headers: JSON_HEADERS, body: '{"error":"x"}' }); return; }
      body = classify;
    } else if (path === '/api/prompt-bar' && method === 'POST') {
      const request = route.request().postDataJSON();
      seen.submits.push(request);
      status = 202;
      body = request.focus?.app_agent_id === 'texture'
        ? { schema: 'choir.prompt_bar_continue.v1', continued: true, command_id: request.command_id, doc_id: request.focus.object_ref, trajectory_id: 't-2', revision_id: 'doc-tides-rev-3', request_id: 'rq-1', replay: false }
        : { schema: 'choir.durable_work.v1', submission_id: 'submission-agent', state: 'completed', status_url: '/api/prompt-bar/submissions/submission-agent' };
    } else if (path === '/api/prompt-bar/submissions/submission-agent') {
      body = { submission_id: 'submission-agent', state: 'completed', decision: { action: 'open_app', app: 'files', title: 'Files' } };
    } else if (path === '/api/texture/fork') {
      seen.forks.push(route.request().postDataJSON());
      status = 202;
      body = { schema: 'choir.texture_owner_revision.v1', doc_id: 'doc-fork', trajectory_id: 't-fork', revision_id: 'doc-fork-rev-1', request_id: 'rq-fork', replay: false };
    } else if (path.startsWith('/api/platform/publications/resolve')) {
      body = url.searchParams.get('view') === 'nav' ? { nav: { revision: 1, latest: 1 } } : story('story-a', 'Harbor reopens');
    } else if (path.startsWith('/api/platform/publications/related')) {
      body = { links: [] };
    } else if (docMatch && !docMatch[2]) {
      const id = decodeURIComponent(docMatch[1]);
      body = { doc_id: id, owner_id: 'owner-1', title: id === 'doc-fork' ? 'Harbor reopens' : 'Tide notes', current_revision_id: `${id}-rev-${id === 'doc-tides' ? 2 : 1}`, trajectory_id: '', created_at: '2026-11-01T00:00:00Z', updated_at: '2026-11-02T00:00:00Z' };
    } else if (docMatch && docMatch[2] === '/revisions') {
      const id = decodeURIComponent(docMatch[1]);
      body = { revisions: id === 'doc-tides' ? [revision(id, 1), revision(id, 2)] : [revision(id, 1)] };
    } else if (path.endsWith('/texture/documents')) {
      body = { documents: [] };
    }
    await route.fulfill({ status, headers: JSON_HEADERS, body: JSON.stringify(body) });
  });
  return seen;
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
  await input.press('ControlOrMeta+Enter');
}

test('A1 A6: a document you own is the aim, with a verb, an icon and the revision', async ({ page }) => {
  const seen = await mockComputer(page, { title: 'Tide notes', appContext: { docId: 'doc-tides', windowTitle: 'Tide notes' } });
  const input = await openDesktop(page);
  await expect(input).toHaveAttribute('placeholder', 'Edit Tide notes…', { timeout: 15_000 });
  await expect(page.locator('[data-prompt-aim-icon="edit"]')).toBeVisible();
  await expect(page.locator('[data-prompt-aim-revision]')).toContainText('v2');
  await expect(page.locator('[data-command-field]')).not.toContainText('Conductor');

  await send(page, 'tighten the second paragraph');
  await expect.poll(() => seen.submits.length).toBe(1);
  expect(seen.classifies[0]).toMatchObject({ text: 'tighten the second paragraph', doc_state: 'owned' });
  expect(seen.submits[0].focus).toEqual({ app_agent_id: 'texture', object_ref: 'doc-tides' });
  expect(seen.submits[0].base_revision_id, 'the head is answered without a base').toBeUndefined();
  await expect(page.locator('[data-prompt-class="edit"]')).toBeVisible();
});

test('A4 A5: a request is sent to the agent, and one tap sends the text again as the document edit', async ({ page }) => {
  const seen = await mockComputer(page, { title: 'Tide notes', appContext: { docId: 'doc-tides', windowTitle: 'Tide notes' }, classify: { class: 'request', rule: 'judge', alternatives: ['edit'] } });
  await openDesktop(page);
  await expect(page.locator('[data-prompt-input]')).toHaveAttribute('placeholder', 'Edit Tide notes…', { timeout: 15_000 });
  await send(page, 'find me a story about harbors');
  await expect(page.locator('[data-prompt-class="request"]')).toBeVisible({ timeout: 10_000 });
  expect(seen.submits[0].focus, 'a request goes to the agent, with no focus').toBeUndefined();
  // Only the other destination is offered: edit and comment are one destination.
  await expect(page.locator('[data-prompt-resend]')).toHaveCount(1);
  await page.locator('[data-prompt-resend="edit"]').click();
  await expect.poll(() => seen.submits.length).toBe(2);
  expect(seen.submits[1].focus).toEqual({ app_agent_id: 'texture', object_ref: 'doc-tides' });
  expect(seen.submits[1].text).toBe('find me a story about harbors');
  expect(seen.submits[1].command_id).not.toBe(seen.submits[0].command_id);
  // The edit committed an owner revision: no further re-send is offered.
  await expect(page.locator('[data-prompt-class="edit"]')).toBeVisible();
  await expect(page.locator('[data-prompt-resend]')).toHaveCount(0);
});

test('A7: after a committed edit or comment no re-send is offered (it would write a second identical revision)', async ({ page }) => {
  for (const cls of ['edit', 'comment']) {
    await page.unrouteAll({ behavior: 'ignoreErrors' });
    const seen = await mockComputer(page, { title: 'Tide notes', appContext: { docId: 'doc-tides', windowTitle: 'Tide notes' }, classify: { class: cls, rule: 'judge' } });
    await openDesktop(page);
    await expect(page.locator('[data-prompt-input]')).toHaveAttribute('placeholder', 'Edit Tide notes…', { timeout: 15_000 });
    await send(page, 'tighten the lede');
    await expect(page.locator(`[data-prompt-class="${cls}"]`)).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('[data-prompt-resend]')).toHaveCount(0);
    expect(seen.submits).toHaveLength(1);
  }
});

// No document in view: Jev tells the request kinds apart, and each reaches lane
// F's route from the owner's session. The plain agent request is the default.
test('A8: with no document in view, "ask for a story" asks, and the document opens', async ({ page }) => {
  const asks = [];
  const seen = await mockComputer(page, { title: 'Settings', appContext: {}, classify: { class: 'ask_story', rule: 'judge' }, asks });
  await openDesktop(page);
  await send(page, 'what happened at the harbor strike');
  await expect.poll(() => asks.length).toBe(1);
  expect(seen.classifies[0].doc_state).toBe('none');
  expect(asks[0].text).toBe('what happened at the harbor strike');
  expect(seen.submits, 'not the conductor').toHaveLength(0);
  // The fixture's own Texture window plus the asked document's.
  await expect(page.locator('[data-window][data-window-app-id="texture"]')).toHaveCount(2, { timeout: 10_000 });
});

test('A9: "add a source" posts the URL, shows the 422 code and reason, and without a URL goes to the agent', async ({ page }) => {
  const schedules = [];
  const seen = await mockComputer(page, {
    title: 'Settings', appContext: {}, classify: { class: 'add_source', rule: 'judge' }, schedules,
    scheduleAnswer: { status: 422, body: { error: 'not a feed', code: 'not_a_feed', reason: 'The address did not return a feed.' } },
  });
  await openDesktop(page);
  await send(page, 'please follow https://example.com/feed.xml, thanks.');
  await expect.poll(() => schedules.length).toBe(1);
  expect(schedules[0]).toEqual({ feed_url: 'https://example.com/feed.xml' });
  await expect(page.getByText(/not_a_feed/)).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText(/did not return a feed/)).toBeVisible();
  await send(page, 'add the harbor feed');
  await expect.poll(() => seen.submits.length).toBe(1);
  expect(schedules).toHaveLength(1);
});

test('A2: a failed classify still sends the text under the default class', async ({ page }) => {
  const seen = await mockComputer(page, { title: 'Tide notes', appContext: { docId: 'doc-tides', windowTitle: 'Tide notes' }, classify: null });
  await openDesktop(page);
  await expect(page.locator('[data-prompt-input]')).toHaveAttribute('placeholder', 'Edit Tide notes…', { timeout: 15_000 });
  await send(page, 'fix the typo');
  await expect.poll(() => seen.submits.length).toBe(1);
  expect(seen.submits[0].focus.object_ref).toBe('doc-tides');
  await expect(page.locator('[data-prompt-class="edit"]')).toBeVisible();
});

test('A3: a comment on a published story you do not own is a fork', async ({ page }) => {
  const seen = await mockComputer(page, {
    title: 'Harbor reopens', appContext: { publishedRoutePath: '/pub/texture/story-a', windowTitle: 'Harbor reopens' },
    classify: { class: 'comment', rule: 'judge', alternatives: ['request'] },
  });
  await openDesktop(page);
  await expect(page.locator('[data-prompt-input]')).toHaveAttribute('placeholder', /Reply to Harbor reopens/, { timeout: 20_000 });
  await expect(page.locator('[data-prompt-aim-icon="reply"]')).toBeVisible();
  await expect(page.locator('[data-prompt-aim-revision]')).toContainText('v1');
  await send(page, 'the second paragraph overstates the reopening');
  await expect.poll(() => seen.forks.length).toBe(1);
  expect(seen.classifies[0].doc_state).toBe('foreign');
  expect(seen.forks[0].source).toEqual({ publication_version_id: 'ver-story-a-1', manifest_hash: 'manifest-story-a' });
  expect(seen.forks[0].text).toBe('the second paragraph overstates the reopening');
  expect(seen.submits, 'nothing is written to the story or sent to the conductor').toHaveLength(0);
  await expect(page.locator('[data-prompt-class="comment"]')).toBeVisible();
});
