// Texture's API calls name the Texture app (modular architecture plan MA3.10):
// the server's route-to-kind table (internal/routekinds) counts or refuses a
// call to Texture's routes from another app. The header and query names are
// the shell's protocol constants (src/lib/app-identity.js); a package in the
// shell's realm sets its own identity.
const APP = 'texture';
const APP_HEADER = 'X-Choir-App';
const APP_QUERY = 'choir_app';

/** Fetch options that carry Texture's identity. */
export function withAppIdentity(options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set(APP_HEADER, APP);
  return { ...options, headers };
}

/** A URL that carries Texture's identity, for a call that cannot set a header (EventSource). */
export function withAppQuery(url) {
  return `${url}${url.includes('?') ? '&' : '?'}${APP_QUERY}=${encodeURIComponent(APP)}`;
}

/**
 * An /api URL scoped to the desktop the page names (?desktop_id=), for an
 * EventSource: the SDK's fetchWithRenewal adds it to fetches itself.
 */
export function withDesktopSelector(input) {
  if (typeof window === 'undefined') return input;
  const page = new URL(window.location.href);
  const desktopId = (page.searchParams.get('desktop_id') || page.searchParams.get('desktop') || '').trim();
  if (!desktopId || desktopId === 'primary') return input;
  const isAbsolute = /^[a-zA-Z][a-zA-Z\d+.-]*:/.test(input);
  const url = new URL(input, window.location.origin);
  if (!url.pathname.startsWith('/api/')) return input;
  if (!url.searchParams.get('desktop_id')) url.searchParams.set('desktop_id', desktopId);
  return isAbsolute ? url.toString() : `${url.pathname}${url.search}${url.hash}`;
}
