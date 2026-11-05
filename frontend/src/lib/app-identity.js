// The app identity an app UI names itself with on its API calls (modular
// architecture plan MA3.10). The server's route-to-kind table counts a call to
// an app's route from another app, or from none, and refuses it on a route
// that serves package kinds (internal/routekinds). Until the Phase 6 iframe
// sandbox the identity is the app's own claim.
export const APP_HEADER = 'X-Choir-App';
export const APP_QUERY = 'choir_app';

/** Returns fetch options that carry the app's identity. */
export function withAppIdentity(app, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set(APP_HEADER, app);
  return { ...options, headers };
}

/**
 * Appends the identity to a URL, for a call that cannot set a header
 * (EventSource). Only for routes that accept the extra query parameter: the
 * capsule-evidence projection refuses any query but its one key, so name the
 * app with a header there.
 */
export function withAppQuery(app, url) {
  return `${url}${url.includes('?') ? '&' : '?'}${APP_QUERY}=${encodeURIComponent(app)}`;
}
