import { test, expect } from './helpers/fixtures.js';

test.setTimeout(300_000);
test.skip(
  process.env.GO_CHOIR_RUN_LIVE_VTEXT_EDIT !== '1',
  'set GO_CHOIR_RUN_LIVE_VTEXT_EDIT=1 to run the live VText fluid-editing proof'
);

async function fetchJSON(page, requestPath, options = {}) {
  return page.evaluate(async ({ requestPath, options }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers || {}),
      },
      ...options,
    });
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_err) {
      body = text;
    }
    if (!res.ok) {
      throw new Error(`${options.method || 'GET'} ${requestPath} failed ${res.status}: ${text}`);
    }
    return body;
  }, { requestPath, options });
}

function legalCloudLongDraft() {
  const sections = Array.from({ length: 18 }, (_, index) => {
    const n = index + 1;
    return [
      `## ${n}. Operating Requirement ${n}`,
      `Legal cloud requirement ${n} explains why professional work product needs durable source memory, governed access, and a document-first revision surface. This paragraph is intentionally long enough to make the proof exercise a long document without needing artificial filler outside the VText artifact.`,
      `The platform should preserve evidence, citations, and document structure while allowing focused edits. Requirement ${n} should remain unchanged unless the user's direct document edit targets it.`,
    ].join('\n\n');
  });
  return [
    '# Proposal for Redacted: Private Legal Cloud',
    '',
    '## Executive Summary',
    '',
    'Status: Draft needs tightening.',
    '',
    'The proposal recommends a private legal cloud that treats documents, sources, and reviewable revisions as primary work product.',
    '',
    ...sections,
    '',
    '## Appendix: Glossary',
    '',
    '| Term | Definition |',
    '| --- | --- |',
    '| Work product | Durable, reviewable professional output. |',
    '| Source entity | A citation-backed source object that can expand inline. |',
    '| VText | Canonical versioned document artifact. |',
  ].join('\n');
}

function directUserEditDraft(previous) {
  return previous.replace(
    'Status: Draft needs tightening.',
    [
      'Status: Final recommendation: choose the private legal cloud route.',
      '',
      'Instruction for VText: remove this instruction line from the final draft and keep the appendix table formatted as a Markdown table.',
    ].join('\n')
  );
}

async function waitForAppagentRevision(page, docId, baselineCount) {
  const deadline = Date.now() + 240_000;
  let latest = null;
  while (Date.now() < deadline) {
    latest = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions?limit=10000`);
    const revisions = latest.revisions || [];
    if (revisions.length > baselineCount && revisions.some((revision) => revision.author_kind === 'appagent')) {
      return revisions;
    }
    await page.waitForTimeout(3000);
  }
  return latest?.revisions || [];
}

test('live VText revision consumes direct edit instructions and keeps long-document structure', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const title = `Live Long VText Fluid Edit ${Date.now()}`;
  const initialContent = legalCloudLongDraft();
  const editedContent = directUserEditDraft(initialContent);

  const doc = await fetchJSON(page, '/api/vtext/documents', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  const initial = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: initialContent,
      author_kind: 'appagent',
      author_label: 'vtext',
      metadata: {
        source: 'live_long_doc_fluid_edit_seed',
        proof: 'long_doc_fluid_editing',
      },
    }),
  });
  await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: editedContent,
      author_kind: 'user',
      author_label: 'browser-test direct edit',
      parent_revision_id: initial.revision_id,
      metadata: {
        source: 'live_long_doc_direct_user_edit',
        proof: 'instruction_bearing_diff',
      },
    }),
  });

  const revisionsBefore = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions?limit=10000`);
  const revise = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revise`, {
    method: 'POST',
    body: JSON.stringify({ intent: 'revise' }),
  });
  expect(revise.loop_id).toBeTruthy();

  const revisions = await waitForAppagentRevision(page, doc.doc_id, revisionsBefore.revisions.length);
  const appagent = [...revisions].reverse().find((revision) => revision.author_kind === 'appagent');
  expect(appagent, JSON.stringify(revisions.map((revision) => ({
    version_number: revision.version_number,
    author_kind: revision.author_kind,
    metadata: revision.metadata,
  })), null, 2)).toBeTruthy();

  expect(appagent.content).toContain('Final recommendation: choose the private legal cloud route');
  expect(appagent.content).not.toContain('Instruction for VText');
  expect(appagent.content).not.toContain('Draft needs tightening');
  expect(appagent.content).toContain('| Term | Definition |');
  expect(appagent.content).toContain('| Work product | Durable, reviewable professional output. |');

  const diagnosis = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/diagnosis?limit=3`);
  expect((diagnosis.runs || []).some((run) => run.loop_id === revise.loop_id)).toBe(true);
  expect(appagent.metadata?.vtext_context_mode).toBe('current_head_plus_user_edit_diff');
  expect(appagent.metadata?.vtext_edit_operation).toBe('apply_edits');
  expect(Number(appagent.metadata?.vtext_run_prompt_chars || 0)).toBeLessThan(40_000);
});
