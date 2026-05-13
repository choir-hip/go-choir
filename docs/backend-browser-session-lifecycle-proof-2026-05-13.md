# Backend Browser Session Lifecycle Proof - 2026-05-13

## Mission Pressure

The backend Browser path could create sessions and persist text, links, and HTML source, but it did not yet have an explicit lifecycle boundary. Without close semantics, Choir could not distinguish an active backend browser session from a completed or disposable candidate, and later VM/browser work would have a weak rollback surface.

Lifecycle close is a small patch, but it preserves the full topology:

- Browser sessions are owner-scoped runtime records.
- Browser work remains server-owned.
- Trace records lifecycle causality.
- Closing a session does not delete audit artifacts.
- Further navigation after close fails closed.

## Change

- Added `closed` as a browser session state.
- Added authenticated `POST /api/browser/sessions/{id}/close`.
- Close is owner-scoped, idempotent, and emits `browser.session.closed` only on the first transition.
- Closed sessions reject later navigation with `409 Conflict`.
- Browser app backend mode now exposes a `Close` control, clears local rendered snapshots after close, and re-creates a backend session on the next navigation intent.
- Trace renders closed-session events as successful lifecycle moments.

## Verification

- `gofmt -w internal/types/browser.go internal/types/task.go internal/runtime/browser.go internal/runtime/api_trace.go internal/runtime/api_test.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_BROWSER=1 npx playwright test browser-backend-obscura.spec.js --workers=1 --timeout=120000`

The Go proof covers owner scoping, idempotent close, single close-event emission, and navigation rejection after close.

The live Playwright proof launches Browser through the desktop, verifies backend readiness, navigates to `https://example.com`, verifies text, links, and escaped HTML source from Obscura, closes the backend session through the UI, verifies the rendered snapshot clears, then opens Trace and verifies both snapshot and closed-session moments are visible.

## Residual Risk

Close semantics are now explicit for persisted backend sessions. `docs/backend-browser-substrate-contract-proof-2026-05-13.md` records the snapshot-vs-control boundary; screenshot capture, DOM input/control, VM identity, and opening Choir inside a background VM browser remain the next frontier.
