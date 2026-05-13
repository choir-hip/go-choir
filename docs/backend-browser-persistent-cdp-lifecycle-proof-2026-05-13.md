# Backend Browser Persistent CDP Lifecycle Proof - 2026-05-13

## Mission Pressure

The CDP screenshot product patch proved that Choir can own screenshot capture through Obscura, but it still started a short-lived CDP process per screenshot navigation. That was a weak lifecycle model for the browser-in-VM frontier because it had no reusable session identity for later input, control, or VM binding.

This slice makes CDP screenshot sessions persistent inside the runtime and keyed to the owner-scoped Browser session. It does not claim background VM identity yet; the execution scope is explicitly `host_process`.

## Change

- Runtime now keeps an in-memory CDP session map keyed by Browser `session_id`.
- CDP screenshot mode starts Obscura `serve` once for a Browser session, attaches to a CDP target, and reuses the same attached `sessionId` across navigations.
- Browser session records now persist:
  - `execution_scope`
  - `backend_session_id`
- `execution_scope` is `host_process` for backend Browser sessions.
- Browser app exposes both fields through stable data attributes.
- Browser close tears down the runtime CDP session and emits whether a CDP session was closed.
- Runtime `Stop` closes any active CDP sessions.
- Live Go verification now proves the same backend CDP session ID is reused across two navigations and closes idempotently.
- Live Browser Playwright proof now verifies the backend session ID remains stable across two navigations through the desktop UI.

## Verification

- `gofmt -w internal/runtime/runtime.go internal/types/browser.go internal/store/store.go internal/store/browser.go internal/runtime/browser.go internal/runtime/api_test.go internal/runtime/browser_live_test.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestLoadConfig|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `GO_CHOIR_RUN_OBSCURA_CDP=1 CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestCaptureObscuraCDPScreenshotLive|TestRuntimeReusesObscuraCDPSessionLive' -v`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_OBSCURA_CDP_SCREENSHOTS=1 CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_CDP_BROWSER=1 npx playwright test browser-backend-obscura-cdp.spec.js --workers=1 --timeout=120000`

The product proof launches Browser through the desktop, verifies the hybrid screenshot substrate, navigates to `https://example.com`, verifies a persisted screenshot, records the backend CDP session ID, navigates again to `https://example.com/?choir=1`, and verifies the backend CDP session ID is unchanged.

## Residual Risk

This is persistent host-process CDP lifecycle, not a background VM browser window. The bounded input/control contract landed in `docs/backend-browser-bounded-control-proof-2026-05-13.md`; the next deformation should bind the browser session identity to a background VM/snapshot lease.
