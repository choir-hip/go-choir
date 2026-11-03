// The platform API a package may use (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A first-party package under src/packages/<name>/ may import `svelte`, its own
// files, and this file. `scripts/check-packages.mjs` refuses every other import
// at build time. Add a re-export here when a package needs a shell function;
// do not reach past it. This is the start of the "platform API range" a lock
// entry declares later.

// Media and file helpers (the shell's auth-renewing fetch and its upload live
// behind these).
export {
  appTitle,
  apiFileURL,
  clampNumber,
  loadContextContentItem,
  MEDIA_FILE_ROUTES,
  mediaRouteForFileName,
  loadRecentMedia,
  mediaSourceIdentity,
  recentMediaAppContext,
  rememberRecentMedia,
  resolveMediaSource,
  uploadMediaFile,
} from './media-utils.js';

// Live events (the synced computer state channel).
export { addLiveEventListener, currentDeviceId, isOwnLiveEvent, liveEventKind, liveEventPayload } from './live-events.js';

// Auth: the renewing fetch every /api call goes through, and its error when
// the session cannot be renewed.
export { AuthRequiredError, fetchWithRenewal } from './auth.js';

// The app registry: which app opens which file kind (Open with).
export { PACKAGE_MEDIA_ROUTES, getAppDefinition } from './apps/registry';
