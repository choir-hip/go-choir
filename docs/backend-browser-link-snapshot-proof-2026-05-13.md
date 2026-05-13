# Backend Browser Link Snapshot Proof - 2026-05-13

## Mission Pressure

The backend Browser app had a server-owned text snapshot, but it still lacked structured page artifacts. The local Obscura CLI does not expose markdown or screenshots through `obscura fetch`, but it does expose link extraction. Persisting links is the next honest product slice before claiming visual control.

## Change

- Browser sessions now persist extracted links in `links_json`.
- Runtime schema migration adds `browser_sessions.links_json` for existing databases.
- Backend navigation now captures both text and link snapshots through Obscura.
- Link output is parsed from tab-separated `url<TAB>label` rows and limited to HTTP(S) links.
- Browser completion Trace events include `links_count`.
- The Browser app renders backend links beside the text snapshot in backend mode.

## Verification

- `/Users/wiz/obscura/target/release/obscura fetch https://example.com --dump links --timeout 10 --quiet`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_BROWSER=1 npx playwright test browser-backend-obscura.spec.js --workers=1 --timeout=120000`

The live Playwright proof launches the Browser app through the desktop, verifies backend mode, navigates to `https://example.com`, verifies the text snapshot contains `Example Domain`, verifies the extracted link panel contains `Learn more`, verifies no iframe is rendered, then opens Trace and verifies the browser session appears as a Trace trajectory with the backend snapshot moment.

## Residual Risk

This still is not a remote VM browser window. Links make the backend browser artifact more structured and Trace-visible, and `docs/backend-browser-html-snapshot-proof-2026-05-13.md` adds HTML source capture. Screenshot capture, DOM interaction, lifecycle close, VM identity, and Choir opening Choir inside this backend browser remain future substrate work.
