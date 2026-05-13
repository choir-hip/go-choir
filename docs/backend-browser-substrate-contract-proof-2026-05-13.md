# Backend Browser Substrate Contract Proof - 2026-05-13

## Mission Pressure

After text, links, HTML, and lifecycle close were proven through the backend Browser path, the next risk was overclaiming the substrate. The configured Choir product path uses `obscura fetch`, which is a snapshot extractor. The local Obscura audit docs also show patched CDP `serve` support for screenshots and input, but Choir does not yet drive that surface.

The Browser contract now exposes that boundary explicitly:

- `obscura_cli_fetch` means backend-owned snapshot extraction.
- Screenshot, input, and CDP control remain unsupported by the Choir product path until a later substrate patch wires and verifies them.
- Playwright remains the verifier; it is not treated as the product browser substrate.

## Change

- Browser capabilities now include `substrate`.
- Unconfigured Browser capabilities fail closed with a complete false support matrix.
- Configured backend Browser capabilities report `substrate: "obscura_cli_fetch"`.
- Backend support explicitly marks `navigate`, `text`, `html`, and `links` as supported.
- Backend support explicitly marks `screenshot`, `input`, and `cdp` as unsupported.
- Browser app exposes the substrate and support matrix through stable data attributes.
- Browser status copy now says "Backend snapshot" instead of implying a fully controllable browser.

## Verification

- `/Users/wiz/obscura/target/release/obscura --help`
- `/Users/wiz/obscura/target/release/obscura fetch --help`
- `rg -n "screenshot|dump|png|jpeg|input|click|control|pdf" /Users/wiz/obscura/docs /Users/wiz/obscura/src /Users/wiz/obscura/crates`
- `gofmt -w internal/runtime/browser.go internal/runtime/api_test.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestBrowser'`
- `cd frontend && pnpm build`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_BROWSER=1 npx playwright test browser-backend-obscura.spec.js --workers=1 --timeout=120000`

The live Playwright proof launches Browser through the desktop, verifies `data-browser-backend-substrate="obscura_cli_fetch"`, verifies text/html/link support is true, verifies screenshot/input/CDP support is false, then repeats the navigation, HTML/source, close, and Trace proof.

## Residual Risk

`docs/obscura-cdp-screenshot-substrate-proof-2026-05-13.md` proves the installed Obscura binary can produce screenshots over CDP. The remaining work is product ownership: `/api/browser/*` still needs a separate CDP-capable provider mode, lifecycle, artifact persistence, and Trace events. That patch should create a new control-capable provider mode rather than quietly extending `obscura_cli_fetch`.
