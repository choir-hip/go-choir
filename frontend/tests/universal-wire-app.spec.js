import { test, expect } from './helpers/fixtures.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

async function applyTheme(page, id) {
  const names = {
    'futuristic-noir': 'Futuristic Noir',
    'carbon-fiber-kintsugi': 'Carbon Fiber Kintsugi',
    'london-salmon': 'London Salmon',
  };
  await page.evaluate(({ id, name }) => {
    window.dispatchEvent(new CustomEvent('choir-theme-change', {
      detail: {
        theme: {
          schema_version: 2,
          id,
          name,
        },
      },
    }));
  }, { id, name: names[id] });
}

test('Universal Wire renders an honest empty edition instead of preview stories', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.getByRole('heading', { name: 'Universal Wire' })).toBeVisible();
  await expect(app.locator('text=SourceMaxx newsroom')).toHaveCount(0);
  await expect(app.locator('text=Living source network')).toBeVisible();
  await expect(app.locator('[data-universal-wire-story]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
  await expect(app.locator('[data-universal-wire-empty-state]')).toContainText('No Wire edition articles yet');
  await expect(app.locator('text=Port backlog recedes')).toHaveCount(0);
  await expect(app.locator('text=seed source neighborhood')).toHaveCount(0);
});

test('Universal Wire retries authenticated story loads after transient route failure', async ({ browser, authenticatedState }) => {
  const context = await browser.newContext({
    storageState: authenticatedState.storageStatePath,
  });
  const page = await context.newPage();
  let storyFetches = 0;
  let manifestEnsures = 0;
  const liveStories = Array.from({ length: 4 }, (_, index) => ({
    id: `source-network-texture-${index + 1}`,
    story_texture_doc_id: `doc-live-source-network-texture-${index + 1}`,
    headline: `Live source-network article ${index + 1}`,
    dek: 'A real source-network Texture article reached the Universal Wire front page.',
    freshness: 'updated 2 min ago',
    prominence: 90 - index,
    tension: 'source-network update',
    changeState: 'live article',
    nodeTone: 'live',
    related: [],
    manifest: { lead: [], supporting: [], contrary: [], context: [] },
    claims: ['The live source network has more than preview seed stories.'],
    projections: {
      'wire-style': 'The live article body is rendered from the authenticated Universal Wire story API after retry.',
    },
  }));
  try {
    await page.route('**/api/universal-wire/stories', async (route) => {
      storyFetches += 1;
      if (storyFetches === 1) {
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'route not ready' }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ source: 'universal-wire-texture-index', stories: liveStories }),
      });
    });
    await page.route('**/api/texture/**', async (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      const docMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)$/);
      const revisionsMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)\/revisions$/);
      const revisionMatch = url.pathname.match(/^\/api\/texture\/revisions\/([^/]+)$/);
      const manifestMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)\/manifest$/);
      const storyForDoc = (docId) => liveStories.find((story) => story.story_texture_doc_id === decodeURIComponent(docId));

      if (manifestMatch) {
        manifestEnsures += 1;
        await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected manifest ensure for .texture shortcut' }) });
        return;
      }

      if (docMatch && method === 'GET') {
        const story = storyForDoc(docMatch[1]);
        if (!story) {
          await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unknown document' }) });
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            doc_id: story.story_texture_doc_id,
            title: story.headline,
            current_revision_id: `rev-${story.story_texture_doc_id}`,
            current_version_number: 0,
          }),
        });
        return;
      }

      if (revisionsMatch && method === 'GET') {
        const story = storyForDoc(revisionsMatch[1]);
        await route.fulfill({
          status: story ? 200 : 404,
          contentType: 'application/json',
          body: JSON.stringify(story ? {
            revisions: [{
              revision_id: `rev-${story.story_texture_doc_id}`,
              doc_id: story.story_texture_doc_id,
              version_number: 0,
              author_kind: 'agent',
              author_label: 'Universal Wire',
              created_at: '2026-06-16T06:00:00Z',
            }],
          } : { error: 'unknown document' }),
        });
        return;
      }

      if (revisionMatch && method === 'GET') {
        const revisionId = decodeURIComponent(revisionMatch[1]);
        const story = liveStories.find((item) => `rev-${item.story_texture_doc_id}` === revisionId);
        await route.fulfill({
          status: story ? 200 : 404,
          contentType: 'application/json',
          body: JSON.stringify(story ? {
            revision_id: revisionId,
            doc_id: story.story_texture_doc_id,
            version_number: 0,
            content: story.projections['wire-style'],
            metadata: {
              source_path: `universal-wire/${story.id}.story.texture`,
              created_from: 'universal_wire_article',
            },
          } : { error: 'unknown revision' }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto(authenticatedState.baseURL);
    await openDeskApp(page, 'universal-wire');
    const app = page.locator('[data-universal-wire-app]');
    await expect(app).toBeVisible();
    await expect(app.locator('[data-universal-wire-story]')).toHaveCount(4, { timeout: 7000 });
    await expect(app.locator('[data-universal-wire-story]').first()).toContainText('Live source-network article 1');
    await expect(app.locator('text=Port backlog recedes')).toHaveCount(0);
    await app.locator('[data-universal-wire-story]').first().locator('[data-universal-wire-open-texture]').click();
    const textureWindow = page.locator('[data-texture-app]').last();
    await expect(textureWindow).toBeVisible({ timeout: 5000 });
    await expect(textureWindow.locator('[data-texture-editor-area]')).toContainText('authenticated Universal Wire story API after retry');
    expect(manifestEnsures).toBe(0);
    expect(storyFetches).toBeGreaterThanOrEqual(2);
  } finally {
    await context.close();
  }
});

test('Universal Wire platform read does not taint ordinary Texture document reads', async ({ page }) => {
  let normalDocReadCount = 0;
  let normalSawPlatformReadOwner = false;
  let wireSawPlatformReadOwner = false;
  let wireManifestEnsures = 0;

  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        authenticated: true,
        user: {
          id: 'user-o4-wire-read-owner-isolation',
          email: 'o4-wire-read-owner-isolation@example.com',
        },
      }),
    });
  });
  await page.route('**/api/shell/bootstrap**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sandbox_id: 'sandbox-dev' }),
    });
  });
  await page.route('**/api/desktop/state**', async (route) => {
    if (route.request().method() === 'PUT') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, updated_at: '2026-06-26T00:00:00Z' }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        owner_id: 'user-o4-wire-read-owner-isolation',
        windows: [
          {
            window_id: 'wire-platform-texture',
            app_id: 'texture',
            title: 'Wire article',
            geometry: { x: 70, y: 70, width: 760, height: 520 },
            mode: 'normal',
            z_index: 3,
            app_context: {
              windowTitle: 'Wire article',
              docId: 'doc-wire-platform',
              appHint: 'universal-wire',
              sourcePath: 'universal-wire/story.story.texture',
            },
          },
          {
            window_id: 'ordinary-texture',
            app_id: 'texture',
            title: 'Ordinary Texture',
            geometry: { x: 160, y: 120, width: 760, height: 520 },
            mode: 'normal',
            z_index: 4,
            app_context: {
              windowTitle: 'Ordinary Texture',
              docId: 'doc-normal-user',
              createInitialVersion: false,
            },
          },
        ],
        active_window_id: 'ordinary-texture',
      }),
    });
  });
  await page.route('**/api/texture/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const readOwner = url.searchParams.get('read_owner') || '';
    const docMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)$/);
    const revisionsMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)\/revisions$/);
    const revisionMatch = url.pathname.match(/^\/api\/texture\/revisions\/([^/]+)$/);
    const manifestMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)\/manifest$/);
    const streamMatch = url.pathname.match(/^\/api\/texture\/documents\/([^/]+)\/stream$/);

    if (docMatch && method === 'GET') {
      const docId = decodeURIComponent(docMatch[1]);
      if (docId === 'doc-normal-user') {
        normalDocReadCount += 1;
        normalSawPlatformReadOwner = normalSawPlatformReadOwner || readOwner === 'universal-wire-platform';
        await route.fulfill({
          status: readOwner ? 409 : 200,
          contentType: 'application/json',
          body: JSON.stringify(readOwner ? { error: 'ordinary read was tainted by platform read owner' } : {
            doc_id: 'doc-normal-user',
            title: 'Ordinary Texture',
            current_revision_id: 'rev-normal-user',
            current_version_number: 0,
          }),
        });
        return;
      }
      if (docId === 'doc-wire-platform') {
        wireSawPlatformReadOwner = wireSawPlatformReadOwner || readOwner === 'universal-wire-platform';
        await route.fulfill({
          status: readOwner === 'universal-wire-platform' ? 200 : 409,
          contentType: 'application/json',
          body: JSON.stringify(readOwner === 'universal-wire-platform' ? {
            doc_id: 'doc-wire-platform',
            title: 'Wire article',
            current_revision_id: 'rev-wire-platform',
            current_version_number: 0,
          } : { error: 'wire read missed platform owner' }),
        });
        return;
      }
    }

    if (revisionsMatch && method === 'GET') {
      const docId = decodeURIComponent(revisionsMatch[1]);
      const isWire = docId === 'doc-wire-platform';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          revisions: [{
            revision_id: isWire ? 'rev-wire-platform' : 'rev-normal-user',
            doc_id: docId,
            version_number: 0,
            author_kind: isWire ? 'agent' : 'user',
            author_label: isWire ? 'Universal Wire' : 'Yusef',
            created_at: '2026-06-26T00:00:00Z',
          }],
        }),
      });
      return;
    }

    if (revisionMatch && method === 'GET') {
      const revisionId = decodeURIComponent(revisionMatch[1]);
      const isWire = revisionId === 'rev-wire-platform';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          revision_id: revisionId,
          doc_id: isWire ? 'doc-wire-platform' : 'doc-normal-user',
          version_number: 0,
          content: isWire
            ? 'Wire article content loaded through platform read owner.'
            : 'Ordinary Texture content loaded without platform read owner.',
          metadata: {},
        }),
      });
      return;
    }

    if (manifestMatch && method === 'POST') {
      const docId = decodeURIComponent(manifestMatch[1]);
      if (docId === 'doc-wire-platform') {
        wireManifestEnsures += 1;
        await route.fulfill({
          status: 409,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'wire platform document should not ensure a user manifest' }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ doc_id: docId, source_path: '' }),
      });
      return;
    }

    if (streamMatch && method === 'GET') {
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: '',
      });
      return;
    }

    await route.continue();
  });

  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')).toBeVisible({ timeout: 10000 });
  await expect(page.locator('[data-window-id="wire-platform-texture"] [data-texture-editor-area]')).toContainText('Wire article content loaded through platform read owner');
  await expect(page.locator('[data-window-id="ordinary-texture"] [data-texture-editor-area]')).toContainText('Ordinary Texture content loaded without platform read owner');
  expect(wireSawPlatformReadOwner).toBe(true);
  expect(normalDocReadCount).toBeGreaterThan(0);
  expect(normalSawPlatformReadOwner).toBe(false);
  expect(wireManifestEnsures).toBe(0);
});

test('Universal Wire renders empty feed diagnostics without synthetic stories', async ({ page }) => {
  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        authenticated: true,
        user: {
          id: 'user-o4-wire-empty-diagnostics',
          email: 'o4-wire-empty-diagnostics@example.com',
        },
      }),
    });
  });
  await page.route('**/api/shell/bootstrap**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sandbox_id: 'sandbox-dev' }),
    });
  });
  await page.route('**/api/desktop/state**', async (route) => {
    if (route.request().method() === 'PUT') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, updated_at: '2026-06-26T00:00:00Z' }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ owner_id: 'user-o4-wire-empty-diagnostics', windows: [], active_window_id: '' }),
    });
  });
  await page.route('**/api/universal-wire/stories', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        source: 'universal-wire-texture-index',
        stories: [],
        diagnostics: {
          status: 'empty',
          summary: 'Universal Wire found no publishable Texture edition stories or graph-backed capture cards.',
          substrates: [
            {
              substrate: 'texture_edition',
              state: 'missing',
              candidate_count: 0,
              story_count: 0,
              reason: 'No Universal Wire Texture edition alias is present.',
            },
            {
              substrate: 'web_capture_graph',
              state: 'empty',
              candidate_count: 0,
              story_count: 0,
              reason: 'No non-tombstoned choir.web_capture objects were found for the Universal Wire platform.',
            },
            {
              substrate: 'source_provenance',
              state: 'not_applicable',
              candidate_count: 0,
              story_count: 0,
              reason: 'No graph capture card was available to inspect for captured_from source provenance.',
            },
          ],
        },
      }),
    });
  });

  await page.goto(process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173');
  await expect(page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')).toBeVisible({ timeout: 10000 });
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app.locator('[data-universal-wire-story]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
  await expect(app.locator('[data-universal-wire-empty-diagnostics]')).toBeVisible();
  await expect(app.locator('[data-universal-wire-empty-diagnostic="texture_edition"]')).toContainText('No Universal Wire Texture edition alias is present.');
  await expect(app.locator('[data-universal-wire-empty-diagnostic="web_capture_graph"]')).toContainText('0 candidates');
  await expect(app.locator('[data-universal-wire-empty-diagnostic="source_provenance"]')).toContainText('captured_from source provenance');
  await expect(app.locator('text=/\\/internal\\//')).toHaveCount(0);
  await expect(app.locator('text=/\\/api\\/agent\\//')).toHaveCount(0);
  await expect(app.locator('text=story_texture_doc_id')).toHaveCount(0);
});

test('Universal Wire opens graph capture sources through Source Viewer by default and Web Lens explicitly', async ({ page }) => {
  const sourceURL = 'https://example.com/o4-wire-capture-source';
  const graphCaptureStory = {
    id: 'graph-capture-wire-story',
    story_texture_doc_id: '',
    headline: 'Graph capture card reaches the Wire',
    dek: 'A graph-backed web capture appears as a real Universal Wire card.',
    freshness: 'captured 4 min ago',
    prominence: 91,
    tension: 'capture projection',
    changeState: 'graph fallback',
    nodeTone: 'source-backed',
    related: [],
    manifest: {
      lead: [{
        id: 'capture-source-handle',
        title: 'O4 graph capture source',
        standing: 'Capture projection, not a Texture publication citation.',
        role: 'lead',
        canonical_url: sourceURL,
        source_kind: 'web_url',
        target_kind: 'web_url',
        object_kind: 'choir.web_capture',
        canonical_id: 'choir.web_capture:user-1:o4-wire-capture-source',
        version_id: 'ver-o4-wire-capture-source',
        content_hash: 'sha256:o4-wire-capture-source',
        open_surface: 'source',
        live_open_surface: 'web_lens',
        reader_artifact_state: 'reader_snapshot_ready',
        reader_snapshot: {
          text_content: 'Durable graph capture reader text proves the Source Viewer opened the stored artifact, not only the original URL.',
          snapshot_kind: 'cleaned_reader_markdown',
          media_type: 'text/markdown',
          source_url: sourceURL,
          access_scope: 'private_user_source',
        },
      }],
      supporting: [],
      contrary: [],
      context: [],
    },
    claims: ['The card carries graph source identity without claiming a native Texture source_ref.'],
    projections: {
      'wire-style': 'Universal Wire renders the graph-backed capture projection while preserving source-opening policy.',
    },
  };

  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        authenticated: true,
        user: {
          id: 'user-o4-wire-source-proof',
          email: 'o4-wire-source-proof@example.com',
        },
      }),
    });
  });
  await page.route('**/api/shell/bootstrap**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sandbox_id: 'sandbox-dev' }),
    });
  });
  await page.route('**/api/desktop/state**', async (route) => {
    if (route.request().method() === 'PUT') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, updated_at: '2026-06-26T00:00:00Z' }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ owner_id: 'user-o4-wire-source-proof', windows: [], active_window_id: '' }),
    });
  });
  await page.route('**/api/browser/capabilities**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ available: false, mode: 'test_no_backend' }),
    });
  });
  await page.route('**/api/universal-wire/stories', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ source: 'universal-wire-web-capture-fallback', stories: [graphCaptureStory] }),
    });
  });

  await page.goto(process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173');
  await expect(page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')).toBeVisible({ timeout: 10000 });
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app).toBeVisible();
  await expect(app).toHaveAttribute('data-universal-wire-data-source', 'universal-wire-web-capture-fallback');
  const story = app.locator('[data-universal-wire-story]').first();
  await expect(story).toContainText('Graph capture card reaches the Wire');
  await expect(story.locator('[data-universal-wire-source-actions]')).toBeVisible();
  await expect(story.locator('[data-universal-wire-open-source]')).toHaveText('Open source');
  await expect(story.locator('[data-universal-wire-open-live-source]')).toHaveText('Web Lens');
  await expect(story.locator('[data-texture-source-ref]')).toHaveCount(0);

  await story.locator('[data-universal-wire-open-source]').click();
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Durable graph capture reader text');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('stored artifact');
  await expect(page.locator('[data-browser-app]')).toHaveCount(0);

  await page.locator('[data-window-app-id="universal-wire"]').last().click({ position: { x: 24, y: 24 } });
  await story.locator('[data-universal-wire-open-live-source]').click();
  const browserWindow = page.locator('[data-browser-app]').last();
  await expect(browserWindow).toBeVisible({ timeout: 10000 });
  await expect(browserWindow.locator('[data-browser-url-input]')).toHaveValue(sourceURL);
});

test('Universal Wire deletes detritus source chronology and bespoke style controls', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app.locator('[data-universal-wire-evidence]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-style-switcher]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-source-search]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-fetch-cycle]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-open-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-compose-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-replace-style]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-ask-choir]')).toHaveCount(0);
  await expect(app.locator('text=Chronology')).toHaveCount(0);
  await expect(app.locator('text=Style.texture')).toHaveCount(0);
  await expect(app.locator('text=Style.texture')).toHaveCount(0);
});

test('Universal Wire has no nested dashboard panels, story boxes, theme selector, or Autoradio surface', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');

  const app = page.locator('[data-universal-wire-app]');
  await expect(app.locator('text=Theme')).toHaveCount(0);
  await expect(app.locator('text=Autoradio')).toHaveCount(0);
  await expect(app.locator('text=Contribute')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph desk')).toHaveCount(0);
  await expect(app.locator('text=StoryGraph news desk')).toHaveCount(0);

  await expect(app.locator('[data-universal-wire-story]')).toHaveCount(0);
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
});

test('Universal Wire remains a responsive Choir web desktop app across all three themes', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'universal-wire');
  const app = page.locator('[data-universal-wire-app]');

  for (const themeId of ['futuristic-noir', 'carbon-fiber-kintsugi', 'london-salmon']) {
    await applyTheme(page, themeId);
    await expect(page.locator('.app-root')).toHaveAttribute('data-theme-id', themeId);
    await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();
  }

  await page.setViewportSize({ width: 430, height: 860 });
  await expect(app.locator('[data-universal-wire-empty-state]')).toBeVisible();

  const layout = await app.evaluate((node) => {
    const paper = node.querySelector('.wire-paper');
    const columns = node.querySelector('.article-columns');
    return {
      paperDisplay: getComputedStyle(paper).display,
      columnTracks: columns ? getComputedStyle(columns).gridTemplateColumns.split(' ').length : 0,
    };
  });
  expect(layout.paperDisplay).toBe('block');
  expect(layout.columnTracks).toBe(0);
});
