// The Texture editor's Save body must decode under the server's strict
// decoder. POST /api/texture/documents/{id}/revisions rejects unknown fields
// with 400, and every Save failed while the client sent author_kind and
// author_label (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 1).
//
// Failure modes this pins:
//   - the client adds a field the server's request struct does not declare;
//   - the server drops a field the client still sends;
//   - the client sends authorship, which only the server decides.
//
// The allowed set is read from the Go request struct itself, so client and
// server cannot drift apart without this test failing. No browser or service
// stack is needed: fetch is stubbed and the module runs in Node.
import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createRevision } from '../src/lib/texture.js';

function serverRevisionFields() {
  const source = readFileSync(fileURLToPath(
    new URL('../../internal/textureowner/texture.go', import.meta.url),
  ), 'utf8');
  const match = source.match(/type textureCreateRevisionRequest struct \{([\s\S]*?)\n\}/);
  if (!match) throw new Error('textureCreateRevisionRequest not found in internal/textureowner/texture.go');
  const fields = [...match[1].matchAll(/json:"([^",]+)/g)].map((m) => m[1]);
  if (fields.length === 0) throw new Error('textureCreateRevisionRequest declares no json fields');
  return new Set(fields);
}

async function captureRevisionBody(args, document = { doc_id: 'doc-1' }) {
  const posted = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    const method = options.method || 'GET';
    if (method === 'GET' && String(url) === '/api/texture/documents/doc-1') {
      return new Response(JSON.stringify(document), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    if (method === 'GET' && String(url).startsWith('/api/trajectories/')) {
      return new Response(JSON.stringify({ trajectory: { lifecycle_version: 7 }, current_document_head: { revision_id: 'rev-head' } }), { status: 200 });
    }
    if (method === 'POST' && String(url) === '/api/texture/documents/doc-1/revisions') {
      posted.push(JSON.parse(options.body));
      return new Response(JSON.stringify({ revision_id: 'rev-2' }), { status: 201 });
    }
    throw new Error(`unexpected request ${method} ${url}`);
  };
  try {
    await createRevision('doc-1', args);
  } finally {
    globalThis.fetch = originalFetch;
  }
  expect(posted).toHaveLength(1);
  return posted[0];
}

test('editor Save body carries only fields the server declares, and no authorship', async () => {
  const allowed = serverRevisionFields();
  expect(allowed.has('author_kind')).toBe(false);
  expect(allowed.has('author_label')).toBe(false);

  // The three editor call shapes: saveUserVersion, the initial version, and
  // the published-derivative version (TextureEditor.svelte). Until the fix
  // each also passed authorKind and authorLabel; a caller that still does
  // must not get them forwarded.
  const authorship = { authorKind: 'user', authorLabel: 'owner@example.com' };
  const shapes = [
    { content: 'saved', bodyDoc: { schema: 'x' }, sourceEntities: [], metadata: { surface: 'texture_editor' }, parentRevisionId: 'rev-1', allowRebase: false },
    { content: 'initial', metadata: { created_from: 'conductor' } },
    { content: 'derivative', citations: [{ id: 'c-1' }], metadata: { created_from: 'published_texture_derivative' } },
  ];
  for (const shape of shapes) {
    const body = await captureRevisionBody({ ...shape, ...authorship });
    for (const key of Object.keys(body)) {
      expect(allowed.has(key), `client sends ${key}, which textureCreateRevisionRequest does not declare`).toBe(true);
    }
  }

  // A lifecycle-bound document adds the CAS fields; they must decode too.
  const lifecycleBody = await captureRevisionBody({ content: 'bound' }, { doc_id: 'doc-1', trajectory_id: 'traj-1' });
  expect(lifecycleBody.expected_lifecycle_version).toBe(7);
  expect(lifecycleBody.parent_revision_id).toBe('rev-head');
  for (const key of Object.keys(lifecycleBody)) {
    expect(allowed.has(key), `client sends ${key}, which textureCreateRevisionRequest does not declare`).toBe(true);
  }
});
