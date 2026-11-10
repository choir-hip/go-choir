// The prompt bar's focus (plan section 11.1, slice H3; R31). The focused
// window names what the owner's text goes to: its app agent and its object.
// Texture's focus is the document in view; any other app publishes its own
// through setAppContext({ focus: { app_agent_id, object_ref, label } }). A
// focus goes only to that app agent, never to the conductor. An app with no
// desk, or with no object to name, shows as a hint: the chip names the app
// and the text goes to the conductor.
//
// Ways it can fail (written before the code):
//   F1 a focus is sent for a window that is not in view (minimized, hidden,
//      closed), for a published or guest preview, or for a signed-out visitor;
//   F2 a package desk's focus is sent with an object_ref the desk is not the
//      custodian of (another package's kind, a core kind, a malformed ref):
//      the server refuses it (promptroute.FocusKind), so the shell keeps it
//      off the wire and shows the app as a hint instead;
//   F3 a focus names an app agent other than the focused window's own app;
//   F4 a selection over the server's bound rides along and the whole submit
//      is refused: an oversized selection is dropped, the focus kept.
import { getAppDefinition, normalizeAppId } from './stores/desktop.js';

/** What a focused submit sends: the app agent (desk), the object and an optional selection. */
export type PromptFocus = { app_agent_id: string; object_ref: string; selection?: string };

/**
 * What a window says the prompt bar is aimed at (lane D): an owned document
 * ("Edit …"), or a published version the reader does not own ("Reply to …").
 * Texture states it; the shell never infers ownership.
 */
export type PromptAim = {
  state: 'owned' | 'foreign';
  title: string;
  verb: string;
  chip: string;
  revisionLabel: string;
  baseRevisionId: string;
  source: { publication_version_id: string; manifest_hash: string } | null;
};

/** What the prompt bar shows for the focused window. `focus` is null for a hint. */
export type PromptTarget = {
  windowId: string;
  appId: string;
  appName: string;
  label: string;
  focus: PromptFocus | null;
  aim?: PromptAim | null;
};

/** The aim a window stated, when it is well formed (foreign needs a pinned version). */
export function promptAimOf(raw: unknown): PromptAim | null {
  if (!raw || typeof raw !== 'object') return null;
  const v = raw as Record<string, any>;
  if (v.state !== 'owned' && v.state !== 'foreign') return null;
  const source = v.source && v.source.publication_version_id && v.source.manifest_hash
    ? { publication_version_id: String(v.source.publication_version_id), manifest_hash: String(v.source.manifest_hash) }
    : null;
  if (v.state === 'foreign' && !source) return null;
  return {
    state: v.state,
    title: String(v.title || ''),
    verb: String(v.verb || ''),
    chip: String(v.chip || ''),
    revisionLabel: String(v.revisionLabel || ''),
    baseRevisionId: String(v.baseRevisionId || ''),
    source: v.state === 'foreign' ? source : null,
  };
}

/** Texture is the default desk: its focus is a document id. */
export const TEXTURE_AGENT = 'texture';
// The server's bounds (internal/promptroute.MaxObjectRefBytes and
// internal/textureowner promptBarSelectionBytes).
export const MAX_OBJECT_REF_BYTES = 512;
export const MAX_SELECTION_BYTES = 16 << 10;

const byteLength = (value: string) => new TextEncoder().encode(value).length;

/** A package desk's name: app:<package>. */
export function packageAgentId(appId: string): string {
  return `app:${appId}`;
}

/** The kinds a package's desk can be the custodian of: choir.app.<package>.<name>. */
export function packageKindNamespace(appId: string): string {
  return `choir.app.${appId}.`;
}

/** The kind of a canonical object id (obj:<kind>:<owner>:<suffix>), or '' when it is not one. */
export function objectRefKind(ref: string): string {
  const parts = String(ref || '').split(':');
  if (parts.length !== 4 || parts[0] !== 'obj' || !parts[1] || !parts[2] || !parts[3]) return '';
  return parts[1];
}

/**
 * The focus a package window published, when it can reach its desk: the
 * app agent is the window's own package desk (F3) and the object is of a kind
 * in that package's namespace (F2). Anything else is null.
 */
export function packageFocus(appId: string, raw: unknown): PromptFocus | null {
  if (!raw || typeof raw !== 'object') return null;
  const value = raw as Record<string, unknown>;
  const agent = String(value.app_agent_id || '').trim();
  const ref = String(value.object_ref || '').trim();
  if (agent !== packageAgentId(appId) || !ref || byteLength(ref) > MAX_OBJECT_REF_BYTES) return null;
  if (!objectRefKind(ref).startsWith(packageKindNamespace(appId))) return null;
  const focus: PromptFocus = { app_agent_id: agent, object_ref: ref };
  const selection = typeof value.selection === 'string' ? value.selection : '';
  if (selection && byteLength(selection) <= MAX_SELECTION_BYTES) focus.selection = selection;
  return focus;
}

/**
 * The prompt target of the focused window, or null when nothing is focused
 * that the prompt bar can name (F1).
 */
export function promptTargetFor(windows: any[], activeWindowId: string, { authenticated = false } = {}): PromptTarget | null {
  if (!authenticated || !activeWindowId) return null;
  const win = (windows || []).find((candidate) => candidate.windowId === activeWindowId);
  if (!win || win.mode === 'closed' || win.mode === 'hidden' || win.mode === 'minimized') return null;
  const context = win.appContext || {};
  if (context.publishedGuest) return null;
  const appId = normalizeAppId(win.appId || '');
  if (!appId) return null;
  const aim = appId === TEXTURE_AGENT ? promptAimOf(context.promptAim) : null;
  // A published story is aimed at only when Texture states a pinned version:
  // the reader's comment forks it. Without one, nothing is in view as before.
  if (context.publishedRoutePath && aim?.state !== 'foreign') return null;
  const appName = getAppDefinition(appId)?.name || appId;
  const title = String(win.title || context.windowTitle || '').trim();
  if (appId === TEXTURE_AGENT) {
    if (aim?.state === 'foreign') return { windowId: win.windowId, appId, appName, label: aim.title || title || appName, focus: null, aim };
    const docId = String(context.docId || '').trim();
    if (!docId) return { windowId: win.windowId, appId, appName, label: '', focus: null };
    return { windowId: win.windowId, appId, appName, label: title || appName, focus: { app_agent_id: TEXTURE_AGENT, object_ref: docId }, aim: aim?.state === 'owned' ? aim : null };
  }
  const published = context.focus && typeof context.focus === 'object' ? context.focus as Record<string, unknown> : null;
  const focus = packageFocus(appId, published);
  const label = String(published?.label || '').trim() || (title !== appName ? title : '');
  return { windowId: win.windowId, appId, appName, label, focus };
}
