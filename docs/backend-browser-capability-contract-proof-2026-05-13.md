# Backend Browser Capability Contract Proof - 2026-05-13

## Mission Pressure

The Browser app is still iframe-based, while the Choir-in-Choir path requires backend-owned browsing that can eventually view/control background VM browser sessions. The next safe deformation was to add a product contract for backend browser capability without pretending iframe browsing is solved.

## Change

- Runtime config now accepts `CHOIR_OBSCURA_BIN` or `OBSCURA_BIN`.
- Runtime exposes authenticated `GET /api/browser/capabilities`.
- The capability response reports the backend provider (`obscura`), mode, availability, configured state, status, binary name, support matrix, and legacy iframe fallback state.
- The endpoint detects configured executable paths without running browser work in the request path.
- Browser app now reads the capability endpoint and renders the backend/legacy mode state with stable data attributes.

## Verification

- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestBrowserCapabilities|TestHandlePromptBarRejectsBrowserRuntimeMetadata'`
- `cd frontend && pnpm build`
- `git diff --check`

The focused runtime tests prove unauthenticated browser callers are denied, the default state is unavailable/not configured, and a configured executable moves the contract into backend-ready mode with only the currently claimed support matrix.

## Residual Risk

This is a contract and capability surface, not a complete backend browser app. Navigation sessions, screenshots, markdown snapshots, Trace events, and background VM browser identity remain future implementation. The value is that the next patch has a product-safe route and no need to add browser-public internal control surfaces.
