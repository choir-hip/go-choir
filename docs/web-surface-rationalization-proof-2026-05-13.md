# Web Surface Rationalization Proof

Date: 2026-05-13
Mission: `docs/mission-web-surface-rationalization-v0.md`

## Result

Choir's web surface is now split into two explicit product paths:

- Candidate Desktop Viewer renders the normal Choir Svelte shell at `/?desktop_id=<candidate>&embedded=1` inside a desktop window. The candidate VM serves sandbox APIs through proxy/vmctl routing; there is no VNC, WebRTC, framebuffer, MJPEG, or screenshot-stream path.
- Web Lens keeps the existing `browser` app ID for compatibility but presents the surface as web snapshots/imports. `/api/browser/*` calls now use the same authenticated, desktop-aware client as other app APIs. Obscura snapshots can be opened directly into VText.

## Changed Files

- `frontend/src/lib/CandidateDesktopViewer.svelte`
- `frontend/src/lib/BrowserApp.svelte`
- `frontend/src/lib/Desktop.svelte`
- `frontend/src/lib/BottomBar.svelte`
- `frontend/src/lib/stores/desktop.js`
- `frontend/tests/web-surface-rationalization.spec.js`
- `frontend/tests/trace-settings-registry.spec.js`
- `README.md`

## Verification

Frontend build:

```text
pnpm build
```

Result: passed.

Runtime/browser/store focused tests:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'
```

Result: passed.

Proxy/vmctl trust-boundary tests:

```text
go test -count=1 ./internal/proxy ./internal/vmctl -run 'TestVMctlRouting|TestProviderRoutesDenied|TestProviderRouteDeniedWithAuth|TestVMctlDeny|TestHandler_ForkDesktop|TestHandler_PublishDesktop'
```

Result: passed.

Product-path Playwright proof:

```text
pnpm exec playwright test tests/web-surface-rationalization.spec.js --workers=1
```

Result: 3 passed.

Covered behavior:

- Candidate Desktop Viewer creates a same-Svelte iframe route with `desktop_id` and `embedded=1`.
- The candidate viewer test records no remote-display protocol requests containing `vnc`, `webrtc`, `mjpeg`, or `framebuffer`.
- Web Lens `/api/browser/capabilities` preserves a candidate `desktop_id` through `fetchWithRenewal` and `withDesktopSelector`.
- Web Lens Obscura semantic snapshot path renders text/links/source without iframe rendering.
- Web Lens import opens the semantic snapshot into VText.

Compatibility smoke:

```text
pnpm exec playwright test tests/trace-settings-registry.spec.js --grep "Trace and Settings stay product-safe" --workers=1
pnpm exec playwright test tests/browser-app.spec.js --grep "browser app launches|url input bar" --workers=1
```

Result: passed.

## Residual Risks

- Candidate Desktop Viewer currently accepts a manually entered desktop ID. A later product pass should connect it to candidate/promotion records so users do not need to type IDs.
- The embedded candidate desktop is a same-origin Svelte iframe. That is intentional for this slice, but the `embedded=1` flag is not yet used to simplify the nested shell chrome.
- Obscura still returns text/html/links plus optional screenshots. A richer DOM/AX/form snapshot would improve agent utility without changing the no-remote-display invariant.
- The Web Lens app ID remains `browser` for compatibility with persisted desktop state and tests.
