# Backend Browser HTML Snapshot Proof - 2026-05-13

## Mission Pressure

The backend Browser app had server-owned text and link artifacts, but the Obscura CLI also exposes HTML source. Persisting source HTML gives later verifiers and browser-control work a richer artifact without pretending it is a live, controllable browser window.

## Change

- Browser sessions now persist `html_snapshot`.
- Runtime schema migration adds `browser_sessions.html_snapshot` for existing databases.
- Backend navigation now captures text, links, and HTML source through Obscura.
- Browser completion Trace events include `html_snapshot_bytes`.
- The Browser app renders HTML source as escaped source text inside a `details` panel, not as executable page HTML.

## Verification

- `/Users/wiz/obscura/target/release/obscura fetch https://example.com --dump html --timeout 10 --quiet`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_BROWSER=1 npx playwright test browser-backend-obscura.spec.js --workers=1 --timeout=120000`

The live Playwright proof launches Browser through the desktop, verifies backend readiness, navigates to `https://example.com`, verifies text snapshot content, verifies extracted links, verifies escaped HTML source contains `<title>Example Domain</title>`, verifies no iframe is rendered, then opens Trace and verifies the browser session trajectory includes the backend snapshot moment.

## Residual Risk

This is still source capture, not browser control. Lifecycle close is covered by `docs/backend-browser-session-lifecycle-proof-2026-05-13.md`, but screenshot capture, DOM interaction, VM identity, and Choir opening/operating Choir inside a background VM browser are still future substrate work.
