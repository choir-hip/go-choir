# Backend Browser Bounded Control Proof - 2026-05-13

## Mission Pressure

The persistent CDP lifecycle patch gave Choir a reusable backend browser session, but it still could only navigate and capture screenshots. That was not enough for Choir-in-Choir, because a background browser window must eventually accept bounded product commands without exposing generic CDP, arbitrary JavaScript, or frontend iframe control.

This slice adds the smallest honest control contract: `fill` and `click` by CSS selector, scoped to an owner-owned Browser session and available only when the opt-in Obscura CDP screenshot substrate is enabled.

## Change

- Runtime now exposes `POST /api/browser/sessions/{id}/control`.
- The route accepts only:
  - `fill` with `selector` and `value`;
  - `click` with `selector`.
- Browser capabilities now advertise:
  - `bounded_input: true`
  - `fill: true`
  - `click: true`
  only under `CHOIR_OBSCURA_CDP_SCREENSHOTS=1`.
- Browser capabilities still advertise generic `input: false` and `cdp: false`.
- Control actions require an active owner-scoped backend CDP session and reject closed/inactive sessions.
- Successful control actions refresh the backend screenshot and emit `browser.control.completed`.
- Failed/rejected control actions emit `browser.control.failed`.
- Trace now summarizes bounded browser control events.
- Browser app now renders a bounded control bar for selector/value/fill/click when capability data allows it.
- Runtime browser operations are serialized per Browser session so concurrent navigations, controls, and close operations cannot retarget the same persistent CDP session out of order.
- Browser app no longer performs a hidden default navigation on mount. The URL bar can prefill a default, but backend navigation is now an explicit user/app intent.

## Recovery Evidence

The first live product proof failed usefully:

- direct Go-level CDP control against `https://httpbin.org/forms/post` passed;
- the Browser product path rendered the `Customer name` snapshot;
- the fill command failed with `selector not found`.

The Playwright trace showed two concurrent navigations on the same Browser session: an automatic default Wikipedia navigation and the user/test navigation to httpbin. The UI ignored the stale response, but the stale backend request could still retarget the persistent CDP session before the fill command. The fix was to make backend browser operations owner-session-serial and remove the hidden default navigation.

This is an important control-system learning: frontend sequence guards are not enough when a backend session has mutable process state. The session owner needs an operation boundary.

## Verification

- `gofmt -w internal/types/task.go internal/runtime/browser.go internal/runtime/api_trace.go internal/runtime/api_test.go internal/runtime/browser_live_test.go`
- `gofmt -w internal/runtime/runtime.go internal/runtime/browser.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestLoadConfig|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `GO_CHOIR_RUN_OBSCURA_CDP=1 CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestCaptureObscuraCDPScreenshotLive|TestRuntimeReusesObscuraCDPSessionLive|TestRuntimeControlsObscuraCDPSessionLive' -v`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_OBSCURA_CDP_SCREENSHOTS=1 CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_CDP_BROWSER=1 npx playwright test browser-backend-obscura-cdp.spec.js --workers=1 --timeout=120000`

Final live Playwright result:

```text
2 passed (15.7s)
```

The product proof launches Browser through the desktop, verifies the hybrid Obscura CDP screenshot substrate, navigates to `https://example.com`, verifies a persisted PNG screenshot, proves the backend CDP session ID remains stable across navigation, navigates to `https://httpbin.org/forms/post`, fills `input[name=custname]`, clicks `input[name=topping]`, verifies the backend session ID remains stable, opens Trace, and verifies `browser fill` and `browser click` moments.

## Residual Risk

This is bounded host-process CDP control, not background VM browser identity. It does not stream a live browser viewport, does not expose keyboard/mouse primitives, does not survive runtime restart, and does not yet bind a Browser session to a VM lease or snapshot.

The next safe deformation is background VM browser session identity: bind Browser sessions to candidate-world/VM lease metadata without exposing vmctl internals through the browser-public API.
