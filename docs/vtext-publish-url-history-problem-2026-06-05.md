# VText Publish URL History Problem

Date: 2026-06-05

## Problem

Publishing a VText should not reload the page. The intended behavior is that the browser address bar becomes the public publication URL while the running VText surface remains mounted. The previous same-tab fix used `window.location.assign(...)`, which avoided a new tab but replaced one disruptive behavior with another: a full page navigation.

## Evidence

- `frontend/src/lib/VTextEditor.svelte` currently updates publication navigation through `window.location.assign(publicURL)`.
- That forces a reload into the public route.
- The earlier new-tab behavior did not reload the publishing editor; it only opened a separate tab.

## Desired Behavior

- On publish, update the current tab's URL to the public publication URL without reload.
- The visible public link and Open link action should use the same no-reload URL update.
- Copy link remains an explicit affordance.

## Remaining Error Field

History-only URL updates mean the mounted app state and URL can diverge until refresh. That is acceptable for this correction because the user goal is a shareable address-bar URL without losing the publishing surface. A later route-state integration can teach the desktop shell to reconcile this state without remounting.
