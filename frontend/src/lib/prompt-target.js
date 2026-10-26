import { normalizeAppId } from './stores/desktop.js';

// promptContinueTarget names the Texture document the prompt bar builds on
// by default: the active window's, when it is an owner Texture document in
// view. Anything else (another app, a minimized window, the recents landing,
// a published or guest preview, a signed-out visitor) has no target, and the
// prompt bar creates a new document as before (FE-02 step 1,
// docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md).
export function promptContinueTarget(windows, activeWindowId, { authenticated = false } = {}) {
  if (!authenticated || !activeWindowId) return null;
  const win = (windows || []).find((candidate) => candidate.windowId === activeWindowId);
  if (!win || normalizeAppId(win.appId || '') !== 'texture') return null;
  if (win.mode === 'closed' || win.mode === 'hidden' || win.mode === 'minimized') return null;
  const context = win.appContext || {};
  const docId = String(context.docId || '').trim();
  if (!docId || context.publishedRoutePath || context.publishedGuest) return null;
  const title = String(win.title || context.windowTitle || '').trim() || 'Texture';
  return { windowId: win.windowId, docId, title };
}

// isSingleHttpUrl mirrors the conductor's content rule
// (content.ClassifyPromptBarContentIntent): text that is one absolute http or
// https URL opens its display app. With a document in view the chip
// defaults to a new document for it, so a pasted link still reaches its app
// unless the owner taps to continue (review 284 item 1).
export function isSingleHttpUrl(text) {
  const fields = String(text || '').trim().split(/\s+/).filter(Boolean);
  if (fields.length !== 1) return false;
  try {
    const parsed = new URL(fields[0]);
    return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.host !== '';
  } catch {
    return false;
  }
}
