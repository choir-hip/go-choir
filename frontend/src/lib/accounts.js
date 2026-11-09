/**
 * The account switcher's browser half (docs/design/account-switcher-2026-11-09.md).
 *
 * One browser holds several signed-in accounts. The server names the set by
 * the opaque choir_accounts cookie; this module only calls /auth/accounts/*,
 * tells sibling tabs, and keys per-user browser storage by user id.
 *
 * Cookie-backed and same-origin: no token ever reaches script.
 */


const CHANNEL_NAME = 'choir-accounts';
const SWITCHED_STORAGE_KEY = 'choir.accounts.switched';
const RECONCILE_KEY = 'choir.accounts.reconcile';
const RECONCILE_WINDOW_MS = 15000;

// Every broadcast carries the sender's tab id; a tab ignores its own.
const TAB_ID = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;

// ---------------------------------------------------------------------------
// Per-user storage keys
// ---------------------------------------------------------------------------

// Per-user keys live in user-scope.js, which imports nothing, so modules
// that run outside the browser shell (autopaper.ts under node) can use them.
export { lastUserId, setLastUserId, scopedKey } from './user-scope.js';
import { lastUserId, setLastUserId, scopedKey } from './user-scope.js';

// auth.js imports this module, and much imports auth.js; a static import of
// shell-services here would pull the package registry into every one of them
// (it broke the settings contract tests), so the shell is loaded on use.
async function prepareAccountSwitch() {
  const shell = await import('./shell-services');
  await shell.prepareAccountSwitch();
}

const RETURNING_KEY = 'choir.auth.returning';

export function markReturning(userId) {
  try {
    if (userId) setLastUserId(userId);
    window.localStorage?.setItem(scopedKey(RETURNING_KEY), 'true');
  } catch (_err) {
    // Best effort.
  }
}

export function isReturning() {
  try {
    const ls = window.localStorage;
    return ls?.getItem(scopedKey(RETURNING_KEY)) === 'true' || ls?.getItem(RETURNING_KEY) === 'true';
  } catch (_err) {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Server calls
// ---------------------------------------------------------------------------

export class AccountError extends Error {
  constructor(message, reason = '', status = 0) {
    super(message);
    this.name = 'AccountError';
    this.reason = reason;
    this.status = status;
  }
}

async function postJSON(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body || {}),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new AccountError(data.error || `Request failed (${res.status})`, data.reason || '', res.status);
  }
  return data;
}

/** The accounts signed in on this browser: [{ user_id, email, active, signed_in }], active first. */
export async function listAccounts() {
  const res = await fetch('/auth/accounts', { credentials: 'include' });
  if (!res.ok) throw new AccountError(`Could not list accounts (${res.status})`, '', res.status);
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

// ---------------------------------------------------------------------------
// Sibling tabs
// ---------------------------------------------------------------------------

/** Tell the other tabs the active account changed (user_id '' = signed out). */
export function announceSwitch(userId) {
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      const ch = new BroadcastChannel(CHANNEL_NAME);
      ch.postMessage({ type: 'account-switched', user_id: userId || '', from: TAB_ID });
      ch.close();
    }
  } catch (_err) {
    // The storage event below is the fallback.
  }
  try {
    window.localStorage?.setItem(SWITCHED_STORAGE_KEY, JSON.stringify({ user_id: userId || '', from: TAB_ID, at: Date.now() }));
  } catch (_err) {
    // Neither channel: the account_switched session result is the backstop.
  }
}

/** Listen for another tab's switch. Returns a disposer. */
export function onAccountSwitched(callback) {
  let ch = null;
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      ch = new BroadcastChannel(CHANNEL_NAME);
      ch.onmessage = (event) => {
        if (event?.data?.type === 'account-switched' && event.data.from !== TAB_ID) callback(event.data.user_id || '');
      };
    }
  } catch (_err) {
    ch = null;
  }
  const onStorage = (event) => {
    if (event.key !== SWITCHED_STORAGE_KEY || !event.newValue) return;
    try {
      const data = JSON.parse(event.newValue);
      if (data.from !== TAB_ID) callback(data.user_id || '');
    } catch (_err) {
      callback('');
    }
  };
  window.addEventListener('storage', onStorage);
  return () => {
    try {
      ch?.close();
    } catch (_err) {
      // Already closed.
    }
    window.removeEventListener('storage', onStorage);
  };
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

function reload() {
  window.location.reload();
}

/**
 * Switch to a signed-in account: flush desktop state, close the live channel,
 * POST switch, tell the other tabs, then reload (the simplest reset of every
 * per-user cache, socket and store). Throws AccountError (reason
 * 'member_signed_out' when that account's session is dead).
 */
export async function switchAccount(userId) {
  await prepareAccountSwitch();
  const data = await postJSON('/auth/accounts/switch', { user_id: userId });
  setLastUserId(data?.user?.id || userId);
  announceSwitch(userId);
  reload();
}

/** Sign out one account of the set. The active one hands over to the next signed-in account, if any. */
export async function signoutAccount(userId) {
  const accounts = await listAccounts().catch(() => []);
  const wasActive = accounts.find((a) => a.user_id === userId)?.active === true;
  if (wasActive) await prepareAccountSwitch();
  const data = await postJSON('/auth/accounts/signout', { user_id: userId });
  if (wasActive) {
    if (data.next_user_id) {
      const next = await postJSON('/auth/accounts/switch', { user_id: data.next_user_id });
      setLastUserId(next?.user?.id || data.next_user_id);
      announceSwitch(data.next_user_id);
    } else {
      announceSwitch('');
    }
    reload();
  }
  return data;
}

/** Sign out of every account on this browser (other devices stay signed in). */
export async function logoutAllAccounts() {
  await prepareAccountSwitch();
  await postJSON('/auth/accounts/logout-all', {});
  announceSwitch('');
  reload();
}

/**
 * The session check said account_switched: this tab's cookie jar diverged from
 * the set's active account (a lost switch response, a stale tab). Reconcile by
 * POSTing switch to the named active account, which sets its pair, then reload
 * once. A reconcile is attempted at most once per window so a jar that cannot
 * be repaired never loops. Resolves true when a reload was started.
 */
export async function reconcileActiveAccount(activeUserId) {
  try {
    const last = Number(window.sessionStorage?.getItem(RECONCILE_KEY) || 0);
    if (Date.now() - last < RECONCILE_WINDOW_MS) return false;
    window.sessionStorage?.setItem(RECONCILE_KEY, String(Date.now()));
  } catch (_err) {
    // sessionStorage blocked: the single attempt below is still bounded by the caller.
  }
  if (!activeUserId) return false;
  try {
    const data = await postJSON('/auth/accounts/switch', { user_id: activeUserId });
    setLastUserId(data?.user?.id || activeUserId);
    reload();
    return true;
  } catch (_err) {
    return false;
  }
}
