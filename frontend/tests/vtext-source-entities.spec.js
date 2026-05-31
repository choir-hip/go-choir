import { test, expect } from './helpers/fixtures.js';

test('VText renders source entities as expandable sources and opens owning media surface', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Source Entity Fixture' }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceURL = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ';
    const metadata = {
      source_entities: [
        {
          entity_id: 'src-fixture-youtube',
          kind: 'youtube_video',
          label: 'YouTube source fixture',
          target: {
            target_kind: 'content_item',
            content_id: 'content-fixture-youtube',
            url: sourceURL,
            canonical_url: sourceURL,
          },
          selectors: [{ selector_kind: 'whole_resource' }],
          display: {
            inline_mode: 'chip',
            expanded_mode: 'media_player',
            open_surface: 'video',
            default_collapsed: true,
          },
          evidence: {
            state: 'available',
            research_state: 'pending',
            transcript_availability: 'unavailable',
          },
          provenance: {
            created_by: 'importer',
            rights_scope: 'private_user_source',
            untrusted_source_text: true,
          },
        },
      ],
    };
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Source Entity Fixture\n\nReview this source: ${sourceURL}`,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata,
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('[data-vtext-source-inline]')).toBeVisible({ timeout: 10000 });
  await expect(rendered.locator('[data-vtext-source-inline]')).toContainText('YouTube source fixture');
  await rendered.locator('[data-vtext-source-inline] summary').click();
  await expect(rendered.locator('[data-vtext-source-inline] iframe')).toHaveAttribute('src', /youtube\.com\/embed\/dQw4w9WgXcQ/);
  await expect(rendered.locator('[data-vtext-source-card]')).toContainText('transcript unavailable');

  const initialVideoWindows = await page.locator('[data-video-app]').count();
  await rendered.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-video-app]')).toHaveCount(initialVideoWindows + 1, { timeout: 10000 });
  await expect(page.locator('[data-video-frame]').last()).toHaveAttribute('src', /youtube\.com\/embed\/dQw4w9WgXcQ/);
});
