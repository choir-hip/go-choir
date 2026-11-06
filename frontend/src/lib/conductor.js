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
