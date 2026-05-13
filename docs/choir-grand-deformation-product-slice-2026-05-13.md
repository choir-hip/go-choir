# Choir Grand Deformation Product Slice Dogfood

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

This slice used desktop product pressure to exercise the broader Choir-in-Choir mission path:

- bottom-left desktop control now opens a Start/app launcher while preserving show-desktop compatibility;
- Files app exposes upload UI backed by authenticated `PUT /api/files/{path}`;
- Settings exposes editable validated theme config and presets for System Noir, NeXT, classic Mac, Aqua, Frutiger Aero, GTK, and Y3K directions;
- prompt-bar VText routing now materializes a canonical VText route immediately from the server-owned conductor run, then lets conductor/VText workers continue asynchronously;
- VText editor exposes current document identity for product-path verification of `.vtext` shortcuts.

## Error Field

Observed failures and corrections:

- Service startup launched background jobs that died when the shell exited. Verification kept a launcher shell alive while Playwright ran.
- `RUNTIME_LLM_PROVIDER=stub` made prompt-bar routing fail through gateway with `unsupported provider: stub`; prompt routing was rerun with gateway `chatgpt`.
- Live conductor inference sometimes omitted `initial_content` or took longer than prompt-bar UI/test expectations. Runtime now creates the canonical VText route immediately for prompt-bar VText intent, with seed-prompt fallback content, while preserving conductor ownership and later worker refinement.
- Start launcher styling pushed bottom bar height to 62px. Bottom bar now stays within the existing 52-60px shell contract.
- Existing Playwright shared user state made Files/VText shortcut tests depend on stale workspace ordering. Tests now use unique folder names and assert against the current VText document id.

## Verification

Commands run:

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `39 passed`.

Rerun after the local launcher was changed to include `vmctl`:

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `40 passed`.

```text
cd frontend && npm run build
```

Result: build passed. Existing warning remains for the large `ghostty-web` chunk.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test ./...
```

Result: all Go packages passed.

```text
git diff --check
```

Result: passed.

## Boundary

This is verified product-pressure substrate, not yet proof that the patch was produced and promoted through a background candidate world. It improves the surface needed to dogfood Choir-in-Choir but should not be counted as full candidate-world promotion proof.

## Next Deformation

Use the now-working prompt/product path to drive one narrow candidate-world task:

- start from a prompt-bar or super-owned objective;
- create a background VM or branch-scoped candidate;
- record base SHA, worker head, verifier contract, and rollback command;
- export the candidate delta;
- verify via focused Go/frontend checks;
- promote only through the promotion queue semantics;
- write a promotion report and continuation record.

The next product pressure should be either the promotion queue UI/API for this proof, or the first podcast/radio slice if candidate-world promotion is already sufficiently observable.
