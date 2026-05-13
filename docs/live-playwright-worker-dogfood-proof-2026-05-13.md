# Live Playwright Worker Dogfood Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Codex used Playwright to prompt Choir through the visible desktop. Choir then used its own runtime path:

```text
desktop prompt bar -> conductor -> VText -> persistent super -> vmctl worker handles -> delegate_worker_vm -> local worker worktrees -> export_patchset -> promotion queue -> VText integration
```

This is the first live product-path dogfood in this run where the model-driven Choir loop itself requested and delegated worker work. Codex observed, inspected traces, repaired the launcher/tool-cwd precondition, and reran the same product path.

## Initial Failure

Command:

```text
cd frontend && GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 npx playwright test vtext-background-worker-demo.spec.js --workers=1 --timeout=420000
```

Result: failed after 4.3 minutes.

The trace reached `request_worker_vm`, but successful `delegate_worker_vm` results were absent. Runtime evidence showed:

```text
tool_error: local worker isolation requires a git repository:
git rev-parse --show-toplevel: fatal: not a git repository
```

Root cause: `cmd/sandbox` defaults `RUNTIME_TOOL_CWD` to `SANDBOX_FILES_ROOT`, which is not a git repository. Local worktree fallback therefore could not derive a foreground base SHA.

## Repair

`start-services.sh` now defaults local runtime tools to the repo root:

```text
RUNTIME_TOOL_CWD="${RUNTIME_TOOL_CWD:-$(pwd)}"
```

This keeps the foreground-super mutation guard active while allowing `delegate_worker_vm` to create isolated local worker worktrees from the actual foreground repo HEAD.

The promotion queue now also prefers the vmctl worker VM handle over the export payload's sandbox id when recording candidate VM identity, and exact repeated export results from the same source run are deduped.

After the passing dogfood, the observed recurrence error was narrowed again: exact repeated addressed casts now produce multiple channel audit messages but only one still-pending inbox delivery for the target agent. That proof is recorded in `docs/inbox-delivery-idempotency-proof-2026-05-13.md`.

The worker-request recurrence was then narrowed at the vmctl lease boundary: exact repeated `request_worker_vm` calls reuse one active worker lease unless `allow_parallel` is set. That proof is recorded in `docs/worker-lease-portfolio-control-proof-2026-05-13.md`.

The promotion-candidate recurrence was then narrowed with deterministic objective fingerprints and patchset SHA-256 digests. That proof is recorded in `docs/objective-fingerprint-promotion-dedupe-proof-2026-05-13.md`.

The continuation path now also carries objective fingerprints, dedupes repeated selected/started continuations, and passes the fingerprint to child-run metadata. That proof is recorded in `docs/continuation-objective-fingerprint-proof-2026-05-13.md`.

After those recurrence controls, the visible desktop dogfood was rerun and passed in 2.2 minutes. vmctl allocated one worker and reused it for the repeated request; the live source run queued one promotion candidate with objective and patchset fingerprints. That proof is recorded in `docs/live-playwright-recurrence-control-proof-2026-05-13.md`.

## Passing Dogfood

Command:

```text
cd frontend && GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 npx playwright test vtext-background-worker-demo.spec.js --workers=1 --timeout=420000
```

Result: passed in 3.3 minutes.

Observed evidence:

- vmctl allocated worker VM handles for the desktop user;
- `delegate_worker_vm` produced successful worker-run results;
- worker runs used local worktree isolation;
- workers committed a marker README change and verified it with `grep`;
- `export_patchset` wrote manifests and patchsets with `github_push=false`;
- promotion candidates were queued;
- VText integrated the worker/export details enough for the Playwright assertion to pass;
- foreground super `bash` remained blocked by the mutation guard.

## Other Verification

```text
bash -n start-services.sh
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRunContinuationCompactsAndStartsBoundedNextGoal|TestRunCompletionCanAutoStartConfiguredContinuation|TestQueuePromotionCandidatesDedupesEquivalentPatchsetFingerprint|TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed'
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/vmctl -run 'TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestQueuePromotionCandidatesDedupesEquivalentPatchsetFingerprint|TestQueuePromotionCandidatesForWorkerExportsDedupesExactExport|TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed|TestHandler_RequestWorker|TestClient_RequestWorker'
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/vmctl -run 'TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestSuperRequestWorkerVMReturnsTypedHandle|TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed|TestHandler_RequestWorker|TestClient_RequestWorker'
```

Result: passed.

```text
cd frontend && pnpm build
```

Result: passed.

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: 41 passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestPromptBarToWorkerWorktreePromotionQueueDeterministic|TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD|TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport|TestForegroundSuperMutationGuardBlocksWritableTools'
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store ./internal/promotion
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestChannelCastDedupesPendingAddressedDelivery|TestCoagentToolsSupportAddressedCastAcrossProfiles|TestPromptBarToWorkerWorktreePromotionQueueDeterministic|TestQueuePromotionCandidatesForWorkerExportsDedupesExactExport'
```

Result: passed.

## Residual Errors

The live model-driven run still over-produced:

- multiple worker VM handles for one user prompt;
- duplicate delegate calls;
- multiple queued promotion candidates for equivalent but not identical exports;
- some VText edit attempts after the mutation window had already completed.

These did not corrupt foreground files or bypass promotion. Exact duplicate export results are now deduped, but equivalent-work dedupe still needs a stronger objective/export fingerprint.

Exact pending duplicate addressed deliveries are now deduped before target-agent consumption. Equivalent super requests and candidate portfolio leases still need semantic objective fingerprints.

Exact repeated worker requests now reuse active leases by default. Semantically equivalent but textually different objectives still need durable objective fingerprints.

Deterministic objective fingerprints and patchset digests now dedupe simple textual variance and equivalent patch contents. Rich semantic equivalence still needs a stronger objective model.

Continuation identity is now durable enough to avoid duplicate next-goal records for the same source/objective. Runtime-triggered near-context-limit compaction is still not complete.

## Next Deformation

Add idempotency and portfolio control:

- dedupe equivalent super inbox deliveries beyond exact pending casts;
- enforce semantic one-active-worker-per-objective fingerprints unless the objective explicitly asks for a portfolio;
- attach richer objective fingerprints to run memory and continuation decisions;
- make context pressure observable in the runtime loop before provider context limits;
- make VText integration turns durable/retryable without trying to edit after their mutation window closes;
- surface duplicate/blocked worker attempts in Trace and Settings as recoverable run memory, not silent noise.
