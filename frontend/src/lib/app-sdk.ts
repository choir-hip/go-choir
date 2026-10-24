// The platform API a package may use (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A first-party package under src/packages/<name>/ may import `svelte`, its own
// files, and this file. `scripts/check-packages.mjs` refuses every other import
// at build time. Add a re-export here when a package needs a shell function;
// do not reach past it. This is the start of the "platform API range" a lock
// entry declares later.

// Media and file helpers (the shell's auth-renewing fetch lives behind these).
export {
  appTitle,
  apiFileURL,
  clampNumber,
  loadContextContentItem,
  loadRecentMedia,
  mediaSourceIdentity,
  recentMediaAppContext,
  rememberRecentMedia,
  resolveMediaSource,
} from './media-utils.js';

// Live events (the synced computer state channel).
export { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';
