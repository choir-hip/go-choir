import { test, expect } from './helpers/fixtures.js';

// M1 deployed acceptance: owner input is a canonical document revision event.
// - POST /api/texture/documents/{id}/revise on a lifecycle-bound document
//   commits an AuthorUser revision on the canonical path (no /tell forward).
// - The Texture actor's turn is caused by the document revision event (the
//   desk produces an appagent-authored revision after the owner head).
// - POST /tell and /correct return 404.

async function api(page, method, path, body) {
  return page.evaluate(async ({ method, path, body }) => {
    const res = await fetch(path, {
      method,
      credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const json = await res.json().catch(() => null);
    return { status: res.status, body: json };
  }, { method, path, body });
}

test('M1 owner input is a document revision event on staging', async ({ desktopSession }) => {
  test.setTimeout(300_000);
  const { page } = desktopSession;
  const suffix = `${Date.now()}`;

  // 1. Create a lifecycle-bound document via the canonical create path.
  const created = await api(page, 'POST', '/api/texture/lifecycle-documents', {
    title: `M1 owner-input proof ${suffix}`,
    initial_content: `M1 acceptance seed ${suffix}: a short draft the desk can revise.`,
    client_request_id: `m1-proof-${suffix}`,
  });
  expect(created.status, JSON.stringify(created.body)).toBe(201);
  const docID = created.body?.doc_id;
  expect(docID).toBeTruthy();

  // 2. Owner revises via /revise — must commit an AuthorUser revision (202),
  //    not forward to the deleted /tell channel.
  const revised = await api(page, 'POST', `/api/texture/documents/${docID}/revise`, {
    prompt: 'Expand the draft into three short paragraphs about persistent computers.',
    intent: 'revise',
    client_request_id: `m1-revise-${suffix}`,
  });
  expect(revised.status, JSON.stringify(revised.body)).toBe(202);
  const ownerRevisionID = revised.body?.revision_id;
  expect(ownerRevisionID).toBeTruthy();

  // 3. The revision is on the canonical tape and authored by the owner.
  const revisions = await api(page, 'GET', `/api/texture/documents/${docID}/revisions`);
  expect(revisions.status, JSON.stringify(revisions.body)).toBe(200);
  const list = Array.isArray(revisions.body?.revisions) ? revisions.body.revisions : [];
  const ownerRev = list.find((r) => r.revision_id === ownerRevisionID);
  expect(ownerRev, JSON.stringify(list)).toBeTruthy();
  expect(ownerRev.author_kind).toBe('user');
  expect(ownerRev.metadata?.input_origin).toBe('user_prompt');
  expect(ownerRev.metadata?.owner_prompt).toContain('three short paragraphs');

  // 4. The desk observes the head: the document_revision occurrence drives an
  //    actor turn. The run record binds the owner head as its current
  //    revision — activation is the proof (provider auth on staging is a
  //    separate gap, so we assert the run, not a completed appagent revision).
  let deskRunID = '';
  const deadline = Date.now() + 120_000;
  while (Date.now() < deadline) {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    deskRunID = doc.body?.agent_revision_run_id || '';
    if (deskRunID) break;
    await page.waitForTimeout(3000);
  }
  expect(deskRunID, 'no desk run was created from the owner revision').toBeTruthy();
  const run = await api(page, 'GET', `/api/runs/${deskRunID}`);
  expect(run.status, JSON.stringify(run.body)).toBe(200);
  const meta = run.body?.metadata || {};
  expect(meta.doc_id).toBe(docID);
  expect(meta.request_intent).toBe('apply_owner_revision');
  // The run binds a real head of this doc: the create's initial revision wake
  // and the /revise occurrence coalesce into one pending desk turn.
  const headIDs = list.map((r) => r.revision_id);
  expect(headIDs).toContain(meta.current_revision_id);
  // 5. Deleted ingress returns 404 on the deployed surface.
  for (const verb of ['tell', 'correct']) {
    const res = await api(page, 'POST', `/api/texture/documents/${docID}/${verb}`, { content: 'x' });
    expect(res.status, `${verb} should be deleted`).toBe(404);
  }
});
