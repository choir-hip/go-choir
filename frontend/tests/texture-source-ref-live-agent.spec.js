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
      content: '# Source Ref Live Probe\n\nThis sentence cites [1] and should keep that inline source anchor.',
      body_doc: {
        schema: 'choir.texture_doc.v1',
        doc: {
          type: 'doc',
          attrs: { id: 'doc-live-source-ref' },
          content: [
            {
              type: 'heading',
              attrs: { id: 'h-live-source-ref', level: 1 },
              content: [{ type: 'text', text: 'Source Ref Live Probe' }],
            },
            {
              type: 'paragraph',
              attrs: { id: 'p-live-source-ref' },
              content: [
                { type: 'text', text: 'This sentence cites ' },
                { type: 'source_ref', attrs: { id: 'ref-live-youtube', source_entity_id: 'src-live-youtube', display_mode: 'numbered_ref', label: 'the source clip' } },
                { type: 'text', text: ' and should keep that inline source anchor.' },
              ],
            },
          ],
        },
      },
      source_entities: [
        {
          source_entity_id: 'src-live-youtube',
          target: {
            kind: 'content_item',
            id: 'src-live-youtube-content',
            metadata: {
              url: sourceURL,
              canonical_url: sourceURL,
            },
          },
          selectors: [{ kind: 'whole_resource' }],
          display: {
            mode: 'numbered_ref',
            title: 'Live YouTube source fixture',
            label: 'Live YouTube source fixture',
          },
          evidence: {
            state: 'available',
            open_surface: 'video',
            research_state: 'pending',
          },
          provenance: {
            created_by: 'browser-test',
            rights_scope: 'private_user_source',
            untrusted_source_text: true,
          },
        },
      ],
      author_kind: 'user',
      author_label: 'browser-test',
      metadata: {},
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
      revision.content?.includes('[1]')
    );
  }, { timeout: 180_000, intervals: [3000] }).toBe(true);
});
