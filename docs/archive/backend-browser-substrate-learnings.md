# Backend Browser Substrate Learnings

This note preserves the durable signal from the old 2026-05-13 backend-browser
proof shards while allowing the dated artifacts to be deleted.

## Consolidated Lesson

The Browser surface is not a remote-display product. It should be a product
path for server-owned web evidence and bounded actions:

```text
authenticated owner -> browser session -> snapshot/control event
-> durable BrowserSession record -> Trace moment -> optional VText import
```

The proof sequence established useful constraints:

- Capability discovery must be explicit. The product should say which backend
  provider is configured, which substrate is active, and which operations are
  supported instead of pretending iframe browsing or CDP control is always
  available.
- Backend browser sessions are owner-scoped records. They need lifecycle state,
  current URL, text/html/link snapshots, optional screenshots, errors, and Trace
  events.
- Browser work must fail closed when the backend is unavailable, a URL is not
  HTTP(S), a session is closed, or an unsupported action is requested.
- Text, links, and HTML snapshots are semantic evidence. Screenshots are visual
  proof. CDP is an implementation detail and should not leak as a generic
  product capability.
- Bounded control means named actions such as `fill` and `click` against a
  session-owned page. It does not mean arbitrary JavaScript, keyboard/mouse
  tunneling, or browser-public internal APIs.
- Frontend sequence guards are insufficient for mutable backend browser state.
  Runtime operations for a browser session need serialization so a stale
  navigation cannot retarget the same CDP session before a later control action.
- Closing a session should preserve audit artifacts and reject later mutation.
- Candidate desktop viewing should reuse the Choir Svelte shell and desktop
  routing. It should not introduce VNC, WebRTC, framebuffer, MJPEG, or hidden
  screenshot-stream protocols as a substitute for product-state proof.

## What Was Removed

The removed proof shards covered capability reporting, Obscura text snapshots,
Trace events, link and HTML capture, lifecycle close, CDP screenshot probing,
product-owned screenshot capture, persistent CDP lifecycle, bounded control,
and Web Lens/Candidate Desktop rationalization.

Their ongoing spec value now lives here and in:

- `docs/archive/mission-web-surface-rationalization-v0.md`
- `docs/frontend-app-building-api.md`
- `docs/runtime-invariants.md`
- browser/runtime/frontend tests that exercise the current product behavior

## Negative Rules

- Do not claim Browser provides live VM view/control because a host CDP session
  can capture screenshots.
- Do not add browser-public internal routes for raw CDP, vmctl, or test-only
  state mutation.
- Do not treat iframe success or screenshot presence as proof of product-path
  browser automation.
- Do not bind Browser sessions to candidate computers without owner scoping,
  lease/epoch identity, lifecycle close, and Trace evidence.

