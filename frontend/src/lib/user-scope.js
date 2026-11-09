// Per-user browser-storage keys. This module imports nothing: autopaper.ts
// uses it and also runs under node (the digest parity test).

const LAST_USER_KEY = 'choir.auth.lastUserId';

/** The user id the last successful session check saw, or ''. */
export function lastUserId() {
  try {
    return window.localStorage?.getItem(LAST_USER_KEY) || '';
  } catch (_err) {
    return '';
  }
}

/** Written at every successful session check and at every switch. */
export function setLastUserId(userId) {
  try {
    if (userId) window.localStorage?.setItem(LAST_USER_KEY, userId);
  } catch (_err) {
    // Storage blocked: per-user keys fall back to the unscoped name.
  }
}

/**
 * A global browser-storage key made per-user: `<base>:<userId>` once a user is
 * known, `<base>` before. Before the session is known the pointer picks the
 * right account's cache.
 */
export function scopedKey(base) {
  const id = lastUserId();
  return id ? `${base}:${id}` : base;
}
