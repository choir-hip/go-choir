// The shell functions of SDK 1.1.0 (slice S1; docs/design/modular-architecture-plan-2026-11-04.md
// section 5, docs/design/modular-architecture-2026-11-04.md section 3).
//
// Each function is thin over the shell: the desktop binds its handlers once at
// mount (bindShellHost, which returns the disposer) and every call goes to
// them. A package never receives the host, only these functions, so a T1
// package is grant-checked at the bridge and an in-tree package calls the same
// ones. They replaced the forwarded events (authexpired, launchapp, ...),
// which SDK 2.0.0 (X1) deleted: an app asks the shell only through these.
//
// Failure modes:
//  F1 a call with no desktop mounted (logged-out boot, a unit): functions that
//     answer return their empty answer, functions that act do nothing, and
//     none throws; the shell never shows a half state because of it.
//  F2 a second desktop binds while the first is alive: the later bind wins and
//     the earlier disposer is a no-op for the newer binding.
//  F3 copyText on a browser that refuses the clipboard: false, and an error
//     toast, never an exception into the app.
//  F4 a toast message that holds a secret: copyText names what was copied by
//     the label the caller gives, never by the text.
//  F5 openFile on a path the shell has no opener for: the same download the
//     Files app gives, not a silent nothing.
//  F6 windowAction with an action the shell has no meaning for: nothing happens;

import { mediaRouteForFileName } from './media-utils.js';
import { isTextFileName } from './open-with';
import { BUILD_INFO } from './build-info.js';

export type ToastKind = 'info' | 'error';
export type ToastOptions = { kind?: ToastKind; durationMs?: number };
export type AuthRequest = { kind?: string; appId?: string; appName?: string; email?: string };
export type OpenAppContext = { appName?: string; icon?: string; appContext?: Record<string, unknown> } & Record<string, unknown>;
export type WindowSummary = { windowId: string; appId: string; title: string; mode: string; focused: boolean };
export type WindowAction = 'focus' | 'restore' | 'minimize' | 'maximize' | 'close';
/** A focused submit's focus (1.5.0, slice H3): the app agent (desk), its object and an optional selection. */
export type PromptFocus = { app_agent_id: string; object_ref: string; selection?: string };
/**
 * focus (1.5.0) sends the text to that app agent; null sends it to the
 * conductor. continueDocId is the 1.1.0 spelling of a Texture focus.
 */
export type PromptOptions = { continueDocId?: string; continueWindowId?: string; focus?: PromptFocus | null };
export type AppContextChange = { windowId: string; appContext?: Record<string, unknown>; title?: string };

/** What the desktop binds. Every member is optional so a host binds what it has. */
export interface ShellHost {
  toast?(message: string, options?: ToastOptions): void;
  requireAuth?(request: AuthRequest): void;
  sessionExpired?(reason?: string): void;
  openApp?(appId: string, context: OpenAppContext): void;
  openTextFile?(detail: { pathSegments: string[]; fileName: string; importToTexture?: boolean }): void;
  openMediaFile?(detail: Record<string, unknown>): void;
  setAppContext?(change: AppContextChange): void;
  listWindows?(): WindowSummary[];
  closeOtherWindows?(keepId: string): Promise<void> | void;
  resetDesktop?(): void;
  windowAction?(windowId: string, action: WindowAction): void;
  showOverview?(): void;
  showDesktop?(): void;
  logout?(): void;
  prepareAccountSwitch?(): Promise<void> | void;
  submitPrompt?(text: string, options: PromptOptions): Promise<void> | void;
}

let bound: ShellHost | null = null;

/** The desktop binds its handlers at mount; the disposer unbinds (F2: only its own binding). */
export function bindShellHost(host: ShellHost): () => void {
  bound = host;
  return () => {
    if (bound === host) bound = null;
  };
}

const host = (): ShellHost => bound || {};

export function toast(message: string, options: ToastOptions = {}): void {
  const text = String(message ?? '').trim();
  if (!text) return;
  host().toast?.(text, { kind: options.kind === 'error' ? 'error' : 'info', durationMs: options.durationMs });
}

/** The preview-mode action-time prompt: ask a signed-out visitor to sign in for this action. */
export function requireAuth(request: AuthRequest = {}): void {
  host().requireAuth?.({ kind: request.kind, appId: request.appId, appName: request.appName, ...(request.email ? { email: request.email } : {}) });
}

/** The app's fetch failed with AuthRequiredError and the session cannot be renewed. */
export function sessionExpired(reason?: string): void {
  host().sessionExpired?.(reason);
}

/**
 * Copy text to the clipboard and tell the owner once. `label` names what was
 * copied in the toast ("Copied <label>"); the text itself is never shown (F4).
 */
export async function copyText(text: string, label = 'to the clipboard'): Promise<boolean> {
  const value = String(text ?? '');
  try {
    if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) throw new Error('no clipboard');
    await navigator.clipboard.writeText(value);
  } catch (_err) {
    toast('Could not copy to the clipboard', { kind: 'error' });
    return false;
  }
  toast(`Copied ${label}`);
  return true;
}

/** Open an app window; `context` carries appName and icon for the title and the app's own appContext. */
export function openApp(appId: string, context: OpenAppContext = {}): void {
  host().openApp?.(String(appId || ''), context || {});
}

/**
 * Open a Files path ('Documents/notes.md') with the app that opens it: the
 * media app its extension routes to, Texture for text, else a download (F5).
 * `appId` and `mediaType` open it with the app the owner chose (Open with);
 * `asText` opens it in Texture whatever its extension, and `importToTexture`
 * makes a new Texture document from it rather than reading the file.
 */
export type OpenFileOptions = {
  appId?: string;
  mediaType?: string;
  openedWith?: 'default' | 'choice';
  asText?: boolean;
  importToTexture?: boolean;
};

export function openFile(path: string, options: OpenFileOptions = {}): void {
  const pathSegments = String(path || '').split('/').filter(Boolean);
  const fileName = pathSegments[pathSegments.length - 1] || '';
  if (!fileName) return;
  const h = host();
  if (options.asText) {
    h.openTextFile?.({ pathSegments, fileName, importToTexture: options.importToTexture === true });
    return;
  }
  const route = options.appId
    ? { appId: options.appId, mediaType: options.mediaType || '', openedWith: options.openedWith || 'choice' }
    : mediaRouteForFileName(fileName);
  if (route) {
    h.openMediaFile?.({ ...route, pathSegments, filePath: pathSegments.join('/'), fileName });
    return;
  }
  if (isTextFileName(fileName)) {
    h.openTextFile?.({ pathSegments, fileName });
    return;
  }
  if (typeof document === 'undefined') return;
  const a = document.createElement('a');
  a.href = '/api/files/' + pathSegments.map(encodeURIComponent).join('/');
  a.download = fileName;
  a.style.display = 'none';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

/** An app tells the shell its window's context and title (the app knows its own windowId). */
export function setAppContext(change: AppContextChange): void {
  if (!change || !change.windowId) return;
  host().setAppContext?.(change);
}

export function listWindows(): WindowSummary[] {
  return host().listWindows?.() ?? [];
}

/** Close every window except `keepId` and save that desktop; an empty id clears them all. */
export async function closeOtherWindows(keepId = ''): Promise<void> {
  await host().closeOtherWindows?.(String(keepId || ''));
}

const WINDOW_ACTIONS = new Set<string>(['focus', 'restore', 'minimize', 'maximize', 'close']);

/** Raise, restore, minimize, maximize or close one window (what the tray and the overview do). */
export function windowAction(windowId: string, action: WindowAction): void {
  if (!windowId || !WINDOW_ACTIONS.has(action)) return;
  host().windowAction?.(windowId, action);
}

/** The window overview (every open window at once). */
export function showOverview(): void {
  host().showOverview?.();
}

/** Minimize everything to show the desktop, or bring it back. */
export function showDesktop(): void {
  host().showDesktop?.();
}

/** Sign out. */
export function logout(): void {
  host().logout?.();
}

/** Before an account switch: the desktop saves its state and closes its live channel. */
export async function prepareAccountSwitch(): Promise<void> {
  await host().prepareAccountSwitch?.();
}

/** Send text through the prompt bar's door: the focused app agent takes it, or the conductor routes it. */
export async function submitPrompt(text: string, options: PromptOptions = {}): Promise<void> {
  const value = String(text ?? '').trim();
  if (!value) return;
  const forwarded: PromptOptions = { continueDocId: options.continueDocId || '', continueWindowId: options.continueWindowId || '' };
  if (options.focus !== undefined) forwarded.focus = options.focus;
  await host().submitPrompt?.(value, forwarded);
}

/** The running shell's build: service, version, commit and built_at. */
export function buildInfo(): { service: string; version: string; commit: string; built_at: string } {
  return { ...BUILD_INFO };
}

export function resetDesktop(): void {
  host().resetDesktop?.();
}
