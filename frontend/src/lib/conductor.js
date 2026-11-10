import { fetchWithRenewal } from './auth.js';

function trimText(text) {
  return (text || '').trim();
}

export async function submitConductorPrompt(text, options = {}) {
  const prompt = trimText(text);
  if (!prompt) {
    throw new Error('Prompt is required');
  }

  const commandId = options.commandId || crypto.randomUUID();
  const body = { text: prompt, command_id: commandId };
  // A focus sends the text to the focused app agent, which decides what it
  // is for (plan MA4.8 and section 11.1, slice H3); without one the conductor
  // routes it and sees nothing of the focus. Texture's focus answers with the
  // document's owner revise (continued: true); a package desk's focus answers
  // with a submission whose decision opens that app.
  const focus = options.focus && typeof options.focus === 'object' ? options.focus : null;
  const agent = String(focus?.app_agent_id || '').trim();
  const ref = String(focus?.object_ref || '').trim();
  if (agent && ref) {
    body.focus = { app_agent_id: agent, object_ref: ref };
    if (focus.selection) body.focus.selection = String(focus.selection);
    // The revision the text answers, when the window reads an older one.
    if (options.baseRevisionId) body.base_revision_id = String(options.baseRevisionId);
  }
  const res = await fetchWithRenewal('/api/prompt-bar', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Conductor submission failed (${res.status})`);
  }

  return res.json();
}

// classifyPromptInput asks Jev what the text is: an edit, a comment or a
// request (lane D). docState is owned, foreign or none. A failure of any kind
// is null: the caller takes the default class and the text is still sent.
export async function classifyPromptInput(text, docState, options = {}) {
  try {
    const res = await fetchWithRenewal('/api/prompt-bar/classify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: trimText(text), doc_state: docState, command_id: options.commandId || crypto.randomUUID() }),
    });
    if (!res.ok) return null;
    const got = await res.json();
    return got && typeof got.class === 'string' ? got : null;
  } catch {
    return null;
  }
}

// forkPublishedVersion makes the reader's own document from a published
// version, with the reader's comment as the merge text (the one revise call
// behind POST /api/texture/fork). The receipt has doc_id, trajectory_id and
// revision_id.
export async function forkPublishedVersion(source, text, options = {}) {
  // The fork is Texture's route, so the call names Texture (MA3.10).
  const res = await fetchWithRenewal('/api/texture/fork', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Choir-App': 'texture' },
    body: JSON.stringify({
      text: trimText(text),
      source: { publication_version_id: source.publication_version_id, manifest_hash: source.manifest_hash },
      client_request_id: options.commandId || crypto.randomUUID(),
    }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Reply failed (${res.status})`);
  }
  return res.json();
}

export async function waitForConductorDecision(submissionId, options = {}) {
  if (!submissionId) {
    throw new Error('Prompt submission ID is required');
  }

  const timeoutMs = options.timeoutMs ?? 60000;
  const pollMs = options.pollMs ?? 500;
  const deadline = Date.now() + timeoutMs;

  for (;;) {
    const res = await fetchWithRenewal(`/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`, {
      method: 'GET',
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `Prompt submission status failed (${res.status})`);
    }

    const status = await res.json();
    if (status.decision) {
      return status.decision;
    }
    if (status.state === 'failed' || status.state === 'blocked' || status.state === 'cancelled') {
      throw new Error(status.error || `Prompt submission ${status.state}`);
    }
    if (Date.now() >= deadline) {
      throw new Error('Prompt submission timed out');
    }
    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }
}
