import { test, expect } from './helpers/fixtures.js';

test.setTimeout(240_000);
test.skip(
  process.env.GO_CHOIR_RUN_LIVE_SOURCE_REF !== '1',
  'set GO_CHOIR_RUN_LIVE_SOURCE_REF=1 with a real provider to run this product proof'
);

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(async ({ path: requestPath, options: requestOptions }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(requestOptions.headers || {}) },
      ...requestOptions,
    });
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_err) {
      body = text;
    }
    if (!res.ok) {
      throw new Error(`${requestOptions.method || 'GET'} ${requestPath} failed ${res.status}: ${text}`);
    }
    return body;
  }, { path, options });
}

test('live Texture agent preserves inline source ref anchors', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const sourceURL = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ';
  const doc = await fetchJSON(page, '/api/texture/documents', {
    method: 'POST',
    body: JSON.stringify({ title: `Live Source Ref ${Date.now()}` }),
  });
  await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: `# Source Ref Live Probe\n\nThis sentence cites [the source clip](source:src-live-youtube) and should keep that inline source anchor.`,
      author_kind: 'user',
      author_label: 'browser-test',
      metadata: {
        source_entities: [
          {
            entity_id: 'src-live-youtube',
            kind: 'youtube_video',
            label: 'Live YouTube source fixture',
            target: {
              target_kind: 'content_item',
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
              created_by: 'browser-test',
              rights_scope: 'private_user_source',
              untrusted_source_text: true,
            },
          },
        ],
      },
    }),
  });

  const revise = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revise`, {
    method: 'POST',
    body: JSON.stringify({
      intent: 'revise',
      prompt: 'Make the sentence clearer, but keep the inline source citation attached to the claim.',
    }),
  });
  expect(revise.loop_id).toBeTruthy();

  await expect.poll(async () => {
    const revisions = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`);
    return revisions.revisions.some((revision) =>
      revision.author_kind === 'appagent' &&
      revision.content?.includes('[the source clip](source:src-live-youtube)')
    );
  }, { timeout: 180_000, intervals: [3000] }).toBe(true);
});
