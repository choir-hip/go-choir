# Obscura CDP Screenshot Substrate Proof - 2026-05-13

## Mission Pressure

The Browser product path now honestly reports `obscura_cli_fetch` as a snapshot substrate, with screenshot/input/CDP support false. The next question was whether the installed Obscura binary can support a higher-realism browser substrate at all, or whether the mission should move to VM identity before attempting screenshot/control.

This proof is deliberately not a Choir Browser product proof. It is a gated substrate verifier for the next deformation.

## Change

- Added `frontend/tests/obscura-cdp-substrate.spec.js`.
- The test is skipped unless `GO_CHOIR_RUN_OBSCURA_CDP=1` is set.
- The verifier starts `obscura serve` on a random local port.
- It reads `/json/version`, connects Playwright over CDP, navigates to `https://example.com`, captures a screenshot, and asserts a valid PNG header plus nontrivial byte size.
- The verifier stops the Obscura process in teardown.

## Verification

- `/Users/wiz/obscura/target/release/obscura serve --help`
- `node -e "... chromium.connectOverCDP ... page.screenshot ..."`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_CDP=1 CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura npx playwright test obscura-cdp-substrate.spec.js --workers=1 --timeout=120000`

The manual probe returned:

```json
{"title":"Example Domain","screenshotBytes":38088,"pngMagic":"89504e470d0a1a0a"}
```

The committed gated verifier passed the same shape of proof.

## Residual Risk

`docs/backend-browser-cdp-screenshot-product-proof-2026-05-13.md` records the next product-owned screenshot patch. Remaining work is persistent CDP lifecycle, input/control commands, and VM browser identity. It should not silently mutate screenshot capture into a full control claim.
