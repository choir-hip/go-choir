// The prompt bar's door (slices H3 and H4): what the host does with the text a
// prompt surface (the prompt-bar package, the host's last-resort field, an
// app's submitPrompt) hands it. Kept out of Desktop.svelte, which is the window
// host. A focus sends the text to that app agent (plan MA4.8, 11.1); without
// one the conductor routes it. `route` says where the text went, for the chip.
//
// Ways it can fail (P11-P13 of
// docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md, and):
//  D1 a signed-out submit is sent: it must ask for sign-in, carrying the
//     focus, and replay after;
//  D2 a Texture continue opens a new window instead of keeping the document's,
//     or the window does not take the receipt (C3, C9);
//  D3 a failed submit leaves the bar routing (the next send is refused) or
//     shows a stale destination;
//  D4 a session that expired mid-submit shows an error instead of the sign-in.
import { get, writable, type Readable } from 'svelte/store';
import { AuthRequiredError } from './auth.js';
import { submitConductorPrompt, waitForConductorDecision } from './conductor.js';
import { TEXTURE_AGENT, type PromptFocus } from './prompt-focus';
import {
  focusWindow,
  getAppDefinition,
  getAppIcon,
  normalizeAppId,
  openApp,
  restoreWindow,
  updateWindowAppContext,
  windows,
} from './stores/desktop.js';

export type PromptRoute = { state: 'routing' | 'routed'; label: string };
export type PromptSubmitHost = {
  ready(): boolean;
  authenticated(): boolean;
  requestAuth(detail: Record<string, unknown>): void;
  toast(message: string, options?: { kind?: string }): void;
  authExpired(): void;
  scheduleSave(): void;
};

/** A submit's focus: the 1.5.0 focus (null: the conductor), or the 1.1.0 continueDocId, a Texture focus. */
export function promptFocusOf(detail: Record<string, any> = {}): PromptFocus | null {
  if (detail.focus !== undefined) return detail.focus || null;
  const docId = String(detail.continueDocId || '').trim();
  return docId ? { app_agent_id: TEXTURE_AGENT, object_ref: docId } : null;
}

function appLabel(appId: string, title = ''): string {
  const name = getAppDefinition(appId)?.name || appId || 'App';
  return title && title !== name ? `${name} · ${title}` : name;
}

const agentApp = (agent: string) => (agent === TEXTURE_AGENT ? 'texture' : agent.replace(/^app:/, ''));

export function createPromptSubmitter(host: PromptSubmitHost) {
  const routing = writable(false);
  const route = writable<PromptRoute | null>(null);
  let routeTimer: ReturnType<typeof setTimeout> | undefined;

  // A routed line clears itself.
  function showRoute(state: PromptRoute['state'] | '', label = '') {
    clearTimeout(routeTimer);
    route.set(state && label ? { state, label } : null);
    if (state === 'routed') routeTimer = setTimeout(() => route.set(null), 2400);
  }

  // showContinuedDocument keeps a prompt-bar continue in the document's own
  // window: it focuses that window (or opens one on the document if it was
  // closed) and hands it the receipt, so it follows the trajectory the
  // revision committed on (FE-02 step 1). live marks a receipt the window has
  // not followed yet, so a window that mounts with it (the reopen) follows
  // it too; the window clears it once followed.
  function showContinuedDocument(receipt: Record<string, any>, preferredWindowId = '') {
    const docId = receipt.doc_id || '';
    const promptBarContinue = {
      live: true,
      docId,
      trajectoryId: receipt.trajectory_id || '',
      revisionId: receipt.revision_id || '',
      requestId: receipt.request_id || '',
    };
    const all = get(windows) as any[];
    const isDocWindow = (win: any) => normalizeAppId(win.appId) === 'texture' && win.mode !== 'closed' && win.appContext?.docId === docId;
    const target = all.find((win) => win.windowId === preferredWindowId && isDocWindow(win)) || all.find(isDocWindow);
    if (target) {
      // restoreWindow brings back a minimized or hidden window; focusWindow
      // alone does not unhide one.
      if (target.mode === 'normal' || target.mode === 'maximized') focusWindow(target.windowId);
      else restoreWindow(target.windowId);
      updateWindowAppContext(target.windowId, { promptBarContinue });
    } else {
      openApp('texture', 'Texture', getAppIcon('texture'), { docId, allowMultiple: true, promptBarContinue });
    }
    host.scheduleSave();
    showRoute('routed', appLabel('texture', target?.title || ''));
  }

  /** Send text that is already the owner's (signed in): to the focus's app agent, or the conductor. */
  async function send(text: string, { focus = null, windowId = '' }: { focus?: PromptFocus | null; windowId?: string } = {}) {
    if (!host.ready()) {
      host.toast('Desktop is still connecting');
      return;
    }
    const fallbackWindowTitle = text.length > 28 ? `${text.slice(0, 28)}…` : text;
    try {
      routing.set(true);
      showRoute('routing', focus ? appLabel(agentApp(focus.app_agent_id)) : 'Conductor');
      const submission = await submitConductorPrompt(text, { focus });
      if (submission.continued === true) {
        routing.set(false);
        showContinuedDocument(submission, windowId);
        return;
      }
      const conductorSubmissionId = submission.submission_id || '';
      const decision = await waitForConductorDecision(conductorSubmissionId).finally(() => routing.set(false));

      if (decision.action === 'toast') {
        showRoute('');
        host.toast(decision.message || 'Conductor acknowledged the request');
        return;
      }
      if (decision.action !== 'open_app') {
        showRoute('');
        host.toast('Conductor returned an unsupported route');
        return;
      }
      if (normalizeAppId(decision.app || '') === 'texture') {
        showRoute('routed', appLabel('texture', decision.title || ''));
        openApp('texture', 'Texture', '📝', {
          windowTitle: decision.title || fallbackWindowTitle,
          docId: decision.doc_id || '',
          seedPrompt: decision.seed_prompt || text,
          initialContent: decision.initial_content || decision.seed_prompt || text,
          createInitialVersion: decision.create_initial_version !== false,
          conductorLoopId: conductorSubmissionId,
          initialLoopId: decision.initial_loop_id || '',
        });
        return;
      }
      showRoute('routed', appLabel(normalizeAppId(decision.app || 'browser'), decision.title || ''));
      openApp(decision.app || 'browser', decision.title || decision.app || fallbackWindowTitle, '', {
        windowTitle: decision.title || fallbackWindowTitle,
        sourceUrl: decision.source_url || text,
        mediaType: decision.media_type || '',
        appHint: decision.app_hint || decision.app || '',
        contentId: decision.content_id || '',
        conductorLoopId: conductorSubmissionId,
      });
    } catch (err) {
      routing.set(false);
      showRoute('');
      if (err instanceof AuthRequiredError) {
        host.authExpired();
        return;
      }
      host.toast((err as Error).message || 'Conductor submission failed', { kind: 'error' });
    }
  }

  /** A prompt surface's submit ({ text, focus, windowId } or the 1.1.0 continueDocId/continueWindowId). */
  function submit(detail: Record<string, any> = {}) {
    const text = String(detail.text || '').trim();
    if (!text) return;
    const focus = promptFocusOf(detail);
    const windowId = detail.windowId || detail.continueWindowId || '';
    if (!host.authenticated()) {
      host.requestAuth({ kind: 'prompt', text, focus, windowId });
      return;
    }
    void send(text, { focus, windowId });
  }

  return {
    routing: { subscribe: routing.subscribe } as Readable<boolean>,
    route: { subscribe: route.subscribe } as Readable<PromptRoute | null>,
    submit,
    send,
    dispose: () => clearTimeout(routeTimer),
  };
}
