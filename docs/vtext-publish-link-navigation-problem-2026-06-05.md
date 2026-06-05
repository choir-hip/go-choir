# VText Publish Link Navigation Problem

Date: 2026-06-05

## Problem

After the publish URL behavior was restored, publication navigation still used new-tab semantics. That makes clicking the published link feel startling and does not match the intended behavior: the browser URL should become the public publication URL in the current tab.

## Evidence

- `frontend/src/lib/VTextEditor.svelte` renders the visible published link with `target="_blank"`.
- `openPublishedURL(...)` calls `window.open(publicURL, '_blank', ...)`.

## Desired Behavior

- Publishing may navigate to the public URL, but it should do so in the current browser tab.
- Clicking the visible public link should navigate in the current tab.
- The explicit copy affordance remains available for users who need the URL but do not use the browser address bar.

## Remaining Error Field

Same-tab navigation means the private VText editor is left behind after publish. That matches the current product preference, but future publishing UX may need a less disruptive confirmation path inside the public VText route.
