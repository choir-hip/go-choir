import { test, expect } from './helpers/fixtures.js';

// M1 deployed acceptance: owner input is a canonical document revision event.
// - POST /api/texture/documents/{id}/revise on a lifecycle-bound document
//   commits an AuthorUser revision on the canonical path (no /tell forward).
// - The Texture actor's turn is caused by THAT revision: the document event
//   tape must show a texture_turn_committed event whose parent_revision_id is
//   the owner's revision — the desk consumed the owner head. (A run's
//   current_revision_id cannot be the proof: it tracks the run's own
//   committed head, which is always a desk-authored revision.)
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

test.describe.configure({ timeout: 300_000 });

test('M1 owner input is a document revision event on staging', async ({ desktopSession }) => {
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

  // 4. The desk observes THE OWNER'S head: the event tape must record a
  //    texture_turn_committed event whose parent_revision_id is exactly
  //    ownerRevisionID — the turn consumed the owner revision as its input
  //    head. This is the tape receipt that the desk picked up the owner edit;
  //    the create wake and the /revise occurrence coalesce into one pending
  //    turn, so the run record alone does not prove it.
  let turnEvent = null;
  const deadline = Date.now() + 240_000;
  while (Date.now() < deadline) {
    const events = await api(page, 'GET', `/api/texture/documents/${docID}/events?limit=100`);
    const list = Array.isArray(events.body?.events) ? events.body.events : [];
    turnEvent = list.find((e) =>
      e.kind === 'texture_turn_committed' &&
      e.parent_revision_id === ownerRevisionID);
    if (turnEvent) break;
    await page.waitForTimeout(5000);
  }
  expect(
    turnEvent,
    'no texture_turn_committed event consumed the owner revision as its ' +
    'parent head — the desk turn did not pick up the owner edit',
  ).toBeTruthy();

  // 5. No owner-instruction channel rows fired: the event tape carries no
  //    instruction-kind events for this document.
  const finalEvents = await api(page, 'GET', `/api/texture/documents/${docID}/events?limit=100`);
  const finalList = Array.isArray(finalEvents.body?.events) ? finalEvents.body.events : [];
  const instructionEvents = finalList.filter((e) =>
    String(e.kind || '').includes('instruction'));
  expect(instructionEvents, JSON.stringify(instructionEvents)).toHaveLength(0);


  // 6. Deleted ingress returns 404 on the deployed surface.
  for (const verb of ['tell', 'correct']) {
    const res = await api(page, 'POST', `/api/texture/documents/${docID}/${verb}`, { content: 'x' });
    expect(res.status, `${verb} should be deleted`).toBe(404);
  }
});
