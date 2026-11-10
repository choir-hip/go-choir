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
import { classifyPromptInput, forkPublishedVersion, submitConductorPrompt, waitForConductorDecision } from './conductor.js';
import { TEXTURE_AGENT, promptAimOf, type PromptAim, type PromptFocus } from './prompt-focus';
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

/** The class a send was made under (lane D) and the others one tap re-sends it as. */
export type PromptSentAs = { class: string; alternatives: string[] };
export type PromptRoute = { state: 'routing' | 'routed'; label: string } & Partial<PromptSentAs>;
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

  // What the last aimed send was, so one tap can send the same text under
  // another class. A re-send is a new command: the first one already happened.
  let last: { text: string; focus: PromptFocus | null; windowId: string; aim: PromptAim } | null = null;
  let sentAs: PromptSentAs | null = null;

  // A routed line clears itself; one that shows its class stays longer, so
  // the owner can tap it.
  function showRoute(state: PromptRoute['state'] | '', label = '') {
    clearTimeout(routeTimer);
    route.set(state && label ? { state, label, ...(state === 'routed' && sentAs ? sentAs : {}) } : null);
    if (state === 'routed') routeTimer = setTimeout(() => route.set(null), sentAs ? 12000 : 2400);
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

  /**
   * Send text that is already the owner's (signed in): to the focus's app
   * agent, or the agent. With an aim (lane D) Jev first classifies the text as
   * an edit, a comment or a request; an edit or a comment on an owned document
   * is the one revise call, a comment on a published version is a fork the
   * owner owns, a request is the agent's. inputClass sends it under that class.
   */
  async function send(text: string, { focus = null, windowId = '', aim = null, inputClass = '', baseRevisionId = '' }: { focus?: PromptFocus | null; windowId?: string; aim?: PromptAim | null; inputClass?: string; baseRevisionId?: string } = {}) {
    if (!host.ready()) {
      host.toast('Desktop is still connecting');
      return;
    }
    if (aim) {
      await sendAimed(text, { focus, windowId, aim, inputClass });
      return;
    }
    if (!inputClass) sentAs = null;
    const fallbackWindowTitle = text.length > 28 ? `${text.slice(0, 28)}…` : text;
    try {
      routing.set(true);
      showRoute('routing', focus ? appLabel(agentApp(focus.app_agent_id)) : 'Agent');
      const submission = await submitConductorPrompt(text, { focus, baseRevisionId });
      if (submission.continued === true) {
        routing.set(false);
        showContinuedDocument(submission, windowId);
        return;
      }
      const conductorSubmissionId = submission.submission_id || '';
      const decision = await waitForConductorDecision(conductorSubmissionId).finally(() => routing.set(false));

      if (decision.action === 'toast') {
        showRoute('');
        host.toast(decision.message || 'The agent acknowledged the request');
        return;
      }
      if (decision.action !== 'open_app') {
        showRoute('');
        host.toast('The agent returned an unsupported route');
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
      host.toast((err as Error).message || 'Submission failed', { kind: 'error' });
    }
  }

  // sendAimed is the prompt bar aimed at a document (lane D): classify, send
  // under the class, and show the class so one tap sends it under another. The
  // classify never blocks the send: a failure is the default class.
  async function sendAimed(text: string, { focus, windowId, aim, inputClass }: { focus: PromptFocus | null; windowId: string; aim: PromptAim; inputClass: string }) {
    const classes = aim.state === 'owned' ? ['edit', 'comment', 'request'] : ['comment', 'request'];
    try {
      routing.set(true);
      showRoute('routing', aim.chip || 'Agent');
      let cls = inputClass;
      if (!cls) {
        const got = await classifyPromptInput(text, aim.state);
        cls = got && classes.includes(got.class) ? got.class : classes[0];
      }
      if (!classes.includes(cls)) cls = classes[0];
      last = { text, focus, windowId, aim };
      sentAs = { class: cls, alternatives: classes.filter((c) => c !== cls) };
      if (cls === 'request') {
        // The agent, not this document: no focus, the conductor route as ever.
        routing.set(false);
        const mark = sentAs;
        await send(text, { focus: null, windowId: '', inputClass: 'request' });
        sentAs = mark;
        return;
      }
      if (aim.state === 'owned' && focus) {
        const submission = await submitConductorPrompt(text, { focus, baseRevisionId: aim.baseRevisionId });
        routing.set(false);
        if (submission.continued === true) {
          showContinuedDocument(submission, windowId);
          return;
        }
        showRoute('');
        return;
      }
      if (aim.state === 'foreign' && aim.source) {
        const receipt = await forkPublishedVersion(aim.source, text);
        routing.set(false);
        showContinuedDocument(receipt);
        return;
      }
      routing.set(false);
      showRoute('');
    } catch (err) {
      routing.set(false);
      showRoute('');
      if (err instanceof AuthRequiredError) {
        host.authExpired();
        return;
      }
      host.toast((err as Error).message || 'Submission failed', { kind: 'error' });
    }
  }

  /** A prompt surface's submit ({ text, focus, windowId, aim } or the 1.1.0 continueDocId/continueWindowId). resendClass sends the last text under another class. */
  function submit(detail: Record<string, any> = {}) {
    const resendClass = String(detail.resendClass || '').trim();
    if (resendClass) {
      if (!last || !host.authenticated()) return;
      void send(last.text, { focus: last.focus, windowId: last.windowId, aim: last.aim, inputClass: resendClass });
      return;
    }
    const text = String(detail.text || '').trim();
    if (!text) return;
    const focus = promptFocusOf(detail);
    const windowId = detail.windowId || detail.continueWindowId || '';
    const aim = promptAimOf(detail.aim);
    if (!host.authenticated()) {
      host.requestAuth({ kind: 'prompt', text, focus, windowId });
      return;
    }
    void send(text, { focus, windowId, aim, baseRevisionId: detail.baseRevisionId || aim?.baseRevisionId || '' });
  }

  return {
    routing: { subscribe: routing.subscribe } as Readable<boolean>,
    route: { subscribe: route.subscribe } as Readable<PromptRoute | null>,
    submit,
    send,
    dispose: () => clearTimeout(routeTimer),
  };
}
