# Backend Browser CDP Screenshot Product Proof - 2026-05-13

## Mission Pressure

The prior substrate proof showed that Obscura `serve` can produce screenshots over CDP, but Choir still only owned the CLI snapshot path. The next deformation was to make screenshot capture product-owned without falsely claiming full browser control.

The result is an opt-in hybrid provider:

- default Browser backend remains `obscura_cli_fetch`;
- `CHOIR_OBSCURA_CDP_SCREENSHOTS=1` enables `obscura_cli_fetch+obscura_cdp_screenshot`;
- text, links, and HTML still come from the CLI snapshot path;
- screenshots are captured through a runtime-owned Obscura CDP session;
- input and full CDP control remain unsupported.

## Change

- Added `Config.ObscuraCDPScreenshots`, loaded from `CHOIR_OBSCURA_CDP_SCREENSHOTS` or `OBSCURA_CDP_SCREENSHOTS`.
- Propagated that config through `cmd/sandbox`.
- Added `browser_sessions.screenshot_png_base64` with additive migration.
- Added `screenshot_png_base64` to browser session API records.
- Added a Go CDP client that starts `obscura serve`, discovers `/json/version`, creates a target, attaches to a session, navigates, captures `Page.captureScreenshot`, validates base64 PNG data, and tears the process down.
- Browser capabilities now report the hybrid substrate and `screenshot`/`cdp_screenshot` support only when screenshot mode is enabled.
- Browser app renders persisted screenshots as data URLs.
- Trace summaries say `browser screenshot` when the navigation event includes screenshot bytes.
- Added gated verifiers:
  - `internal/runtime/browser_live_test.go`
  - `frontend/tests/browser-backend-obscura-cdp.spec.js`

## Verification

- `gofmt -w cmd/sandbox/main.go internal/runtime/config.go internal/runtime/config_test.go internal/types/browser.go internal/store/store.go internal/store/browser.go internal/runtime/browser.go internal/runtime/api_trace.go internal/runtime/api_test.go internal/runtime/browser_live_test.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./cmd/sandbox ./internal/runtime ./internal/store -run 'TestLoadConfig|TestBrowser|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `GO_CHOIR_RUN_OBSCURA_CDP=1 CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run TestCaptureObscuraCDPScreenshotLive -v`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_OBSCURA_CDP_SCREENSHOTS=1 CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_CDP_BROWSER=1 npx playwright test browser-backend-obscura-cdp.spec.js --workers=1 --timeout=120000`

The first live product run exposed a real config bug: `cmd/sandbox` loaded the runtime config but did not copy the new screenshot flag into the runtime instance. After fixing that, the isolated Go CDP verifier exposed a protocol bug: direct page WebSocket commands did not have a CDP session, so `Runtime.enable` failed with `No page`. The Go client now uses the browser WebSocket and explicitly calls `Target.createTarget` and `Target.attachToTarget`.

The final live Playwright proof launches Browser through the desktop, verifies the hybrid substrate and screenshot support attributes, navigates to `https://example.com`, verifies the text snapshot, verifies a persisted PNG data URL with nontrivial byte count, opens Trace, and verifies the screenshot navigation moment.

## Residual Risk

`docs/backend-browser-persistent-cdp-lifecycle-proof-2026-05-13.md` records the next lifecycle patch. Remaining work is bounded input/control commands or binding the session identity to a background VM browser window.
