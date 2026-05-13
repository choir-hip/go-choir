# Run Control Memory Synthesis Proof - 2026-05-13

## Mission Pressure

Run memory and continuations existed, but the next objective still had to be supplied manually or through source-run metadata. That is not enough for Choir-in-Choir. The controller needs a bounded, auditable way to choose what should happen next from durable signals.

## Change

- Runtime now exposes deterministic run-control synthesis:
  - `SynthesizeRunContinuation`
  - `SelectSynthesizedRunContinuation`
- Queued or integrated promotion candidates synthesize verifier-first `vsuper` continuation objectives.
- Verification-failed candidates synthesize recovery objectives without canonical mutation.
- Verified candidates synthesize owner-review preparation objectives under `cosuper` authority.
- If no promotion candidate signal applies, synthesis falls back to the mission-gradient document objective.
- Synthesized continuations still go through the existing `SelectRunContinuation` path, so they compact first, carry objective fingerprints, dedupe repeated selections, and emit continuation Trace events.
- The selector records durable details such as `selection_source`, `signal`, `candidate_id`, `candidate_status`, `verifier_target`, and `canonical_mutation=forbidden_until_verified_and_approved`.
- Authenticated product APIs now expose continuation operation:
  - `GET /api/continuations?source_loop_id=...`
  - `POST /api/continuations`
  - `GET /api/continuations/{id}`
  - `POST /api/continuations/{id}/start`
- Starting a continuation is explicit and owner-scoped. Selection alone does not start work.
- Trace now includes a visible run-control surface for completed or blocked trajectories:
  - `data-trace-select-continuation`
  - `data-trace-continuation-proposal`
  - `data-trace-start-continuation`
  This keeps next-objective control next to the evidence and still requires explicit start.

## Verification

- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRunControl|TestRunContinuation|TestRunCompletionCanAutoStartConfiguredContinuation'`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/store -run 'TestRunContinuation|TestPromotionCandidate'`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRunControl|TestRunContinuation|TestRunCompletionCanAutoStartConfiguredContinuation|TestRunContinuationPublic'`
- `cd frontend && pnpm build`
- `CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && npx playwright test trace-settings-registry.spec.js --grep "Trace selects a synthesized next objective" --workers=1 --timeout=180000`

The tests prove a queued promotion candidate produces a verifier-first synthesized continuation, repeated synthesis dedupes to the same continuation, absence of candidate signals falls back to the mission doc, public continuation routes require authentication, other users cannot read the selected continuation, and explicit start creates a bounded `vsuper` child run. The frontend build verifies the Trace control surface compiles against the continuation API client.

The Playwright proof uses a real registered desktop user, creates a completed prompt-bar source run through the visible prompt bar, seeds only the candidate precondition through an internal test fixture, opens Trace, selects the completed source trajectory, clicks `Next Objective`, verifies the browser-public `/api/continuations` response selects the queued candidate, verifies the proposal panel contains the candidate-specific verifier objective, and verifies no browser request hits `/internal`.

The Playwright proof intentionally does not click `Start`. Starting arbitrary synthesized worker work in the local foreground runtime is not the same isolation claim as microVM execution. The API start path is covered by focused backend tests; the next live start proof should run under real microVM isolation or a deliberately constrained local worktree objective.

## Residual Risk

This is still a deterministic low-resolution controller, not semantic objective invention. It does not auto-start unless a caller explicitly starts the selected continuation, and it does not inspect every artifact class yet. The next safe deformation is richer signal inputs: failed verifier contracts, run memory compactions, open mission docs, and product proof gaps. The next live start proof should wait for microVM isolation or an explicitly constrained local worktree objective.
