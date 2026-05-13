# Backend Browser Trace Events Proof - 2026-05-13

## Mission Pressure

Backend browser navigation was working, but it was still weak as a Choir-in-Choir substrate because it left no durable Trace trajectory. A browser session that cannot be inspected as a control-system event is hard to verify, recover, or attach to candidate-world work.

## Change

- Browser session creation emits `browser.session.created`.
- Successful backend navigation emits `browser.navigation.completed`.
- Failed or unavailable backend navigation emits `browser.navigation.failed`.
- Browser events are owner-scoped and grouped under a browser-session Trace trajectory.
- Trace trajectory indexing now includes event-only trajectories, not only trajectories backed by run records.
- Trace summaries and tones now render browser session creation, successful snapshots, and navigation failures as first-class moments.

## Verification

- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestBrowser|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry'`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/store -run 'TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`

The browser session test now proves a fake backend Obscura navigation records create and completion events, and that `/api/trace/trajectories` returns the browser session even though it has no run record. The unavailable-backend test proves failed navigation records a browser failure event.

## Residual Risk

Trace now sees browser sessions, and `docs/backend-browser-link-snapshot-proof-2026-05-13.md` adds structured link artifacts. The local Obscura CLI reports supported dump modes of `html`, `text`, and `links`; it does not currently expose markdown or screenshot output through this command. The next safe deformation is therefore HTML snapshot persistence using the existing CLI support, or a deeper Obscura/browser substrate patch that adds screenshot/control semantics before claiming VM browser view/control.
