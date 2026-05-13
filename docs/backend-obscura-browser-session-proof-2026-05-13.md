# Backend Obscura Browser Session Proof - 2026-05-13

## Mission Pressure

The Browser app needed to stop treating a frontend iframe as the only product path. For Choir-in-Choir, browser state must become server-owned, owner-scoped, inspectable, and eventually attachable to a background VM session.

This patch does not claim full browser control. It proves the next invariant-preserving slice: a configured backend Obscura binary can create a durable browser session, navigate through the runtime, persist a text snapshot, and render that snapshot in the Choir desktop Browser app without an iframe.

## Change

- Runtime now stores owner-scoped browser sessions in `browser_sessions`.
- Runtime exposes authenticated browser session endpoints:
  - `POST /api/browser/sessions`
  - `GET /api/browser/sessions`
  - `GET /api/browser/sessions/{id}`
  - `POST /api/browser/sessions/{id}/navigate`
- Navigation normalizes HTTP(S) URLs, strips fragments, and fails closed when no backend provider is configured.
- A configured Obscura binary is invoked server-side with `fetch <url> --dump text`.
- The Browser app creates a backend session when capabilities report backend mode and renders the persisted text snapshot through `data-browser-backend-snapshot`.
- Legacy iframe mode remains only as fallback when backend browser capability is unavailable.
- The sandbox service now carries `ObscuraPath` through to the runtime config, so `CHOIR_OBSCURA_BIN` works in the real app path.

## Verification

- `/Users/wiz/obscura/target/release/obscura fetch https://example.com --dump text --timeout 10 --quiet`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./cmd/sandbox ./internal/runtime -run 'TestBrowser|TestBrowserSession|TestBrowserCapabilities'`
- `cd frontend && pnpm build`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_BROWSER=1 npx playwright test browser-backend-obscura.spec.js --workers=1 --timeout=120000`

The gated Playwright proof registers a real user through the desktop path, launches the Browser app, waits for backend capability readiness, navigates to `https://example.com`, verifies `Example Domain` in the backend snapshot, and verifies no iframe is rendered in backend mode.

## Residual Risk

This is still a text-snapshot browser, not a controllable VM browser window. It does not yet provide screenshots, DOM interaction, close semantics, VM identity, or a Choir-in-Choir loop that opens Choir inside the backend browser.

Trace events were added in `docs/backend-browser-trace-events-proof-2026-05-13.md`. The next safe deformation is links/html snapshot persistence or a deeper Obscura screenshot/control substrate, followed by attaching a browser session to a background VM identity. Only after that should the Browser app claim background VM view/control.
