# MissionGradient: Runtime Test Pyramid Hardening v0

**Status:** checkpoint_incomplete
**Date:** 2026-05-24
**Purpose:** make Choir's runtime/provider/API test suite fast enough for tight landing loops without weakening real coverage.
**Related missions:** [mission-runtime-model-context-substrate-v0.md](mission-runtime-model-context-substrate-v0.md), [mission-runtime-human-proof-experiment-rerun-v1.md](mission-runtime-human-proof-experiment-rerun-v1.md), [mission-ci-throughput-continuation-hardening-v0.md](mission-ci-throughput-continuation-hardening-v0.md)

## One-Line Goal String

```text
/goal Run docs/mission-runtime-test-pyramid-hardening-v0.md as a Codex-operated MissionGradient mission: harden Choir's runtime/provider/API test pyramid so fast policy, routing, model-selection, prompt, and small API behavior tests do not spin up embedded Dolt or full Runtime fixtures. Extract pure helpers and test them directly, add a fake or in-memory RuntimeStore interface for unit-level runtime/API tests, keep embedded Dolt only for a smaller persistence/restart/integration suite, mark heavyweight worker/live/browser/restart tests with explicit integration targets or build tags, and add per-test/package timing output in CI so slow tests are visible immediately. Preserve real coverage for Dolt persistence, restarts, worker/candidate behavior, and staging product paths; do not skip slow tests without an equivalent integration/manual/scheduled coverage path. Land through git/CI/deploy when behavior or CI changes require it, verify staging identity if runtime behavior changed, and finish with timing evidence, coverage tradeoffs, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

Recent runtime work exposed the wrong testing shape. Small decisions such as
VText routing, creative-vs-grounded prompt policy, model max-token selection,
and provider catalog lookup should be cheap pure tests. Instead, many nearby
tests reach for `Runtime`, API setup, and embedded Dolt fixtures, making local
feedback slow and causing full runtime/provider package runs to time out or
hide the actual slow tests.

The target is not "less testing." The target is a sharper pyramid:

```text
pure helpers and policy tests: milliseconds, no Dolt, no Runtime
runtime/API unit tests: fake/in-memory store, deterministic, no external services
embedded Dolt integration tests: persistence, restart, schema, migration truth
worker/browser/live tests: explicit integration targets and staging/product proof
```

The mission should make the common edit-test loop fast while preserving the
integration evidence that catches the bugs only real Dolt, worker VMs, browser
proof, and staging can expose.

## Current Belief State

Known observations:

- Focused pure-style tests for VText/model/provider policy can pass in roughly
  seconds under `nix develop`.
- Full `go test ./internal/runtime ./internal/provider` has timed out locally
  after several minutes without actionable timing output.
- Some tests that only need prompt/routing/model-policy decisions currently use
  API or Runtime setup patterns that open embedded Dolt.
- Embedded Dolt remains necessary for persistence, revision, restart, schema,
  and migration behavior. It is the wrong default for pure policy tests.
- CI does not yet make slow individual tests obvious enough at first glance.

Highest-impact uncertainty:

```text
Which runtime/provider/API tests dominate wall-clock time, and which of those
are genuinely integration tests versus unit tests wearing an integration
fixture?
```

First probe: add or run package/test timing instrumentation before refactoring
so the mission attacks measured slow paths, not guessed slow paths.

## Real Artifact

The artifact is a tested, documented, and CI-visible runtime test pyramid:

```text
measured slow-test profile
  -> pure helper extraction for policy/routing/model/prompt decisions
  -> fake/in-memory RuntimeStore seam for unit-level runtime/API tests
  -> embedded Dolt suite narrowed to persistence/restart/integration truth
  -> heavyweight worker/live/browser/restart tests explicit and separately runnable
  -> CI timing output surfaces slow tests/packages immediately
  -> coverage and staging acceptance remain meaningful
```

## Hard Invariants

- Do not remove coverage because it is slow. Move it to the right layer.
- Do not hide failures by broad-skipping tests in normal CI unless an equivalent
  integration, scheduled, manual, or staging path is added and documented.
- Do not hand-enter Dolt ICU `CGO_*` flags as the durable solution. Use
  `nix develop -c ...` or an equivalent declared Nix environment.
- Do not replace Dolt integration truth with mocks for behavior that depends on
  Dolt persistence, branch/source lineage, revision history, schema migrations,
  restart recovery, or disk-backed state.
- Do not require browser/Playwright capability in ordinary unit-test VMs.
  Browser proof belongs to a specialized worker/browser class or staging proof.
- Do not mutate Node B directly as a shortcut.
- If platform behavior changes, complete the landing loop:

```text
commit -> push origin main -> monitor CI -> deploy -> staging identity -> product proof
```

## Value Criterion

Minimize:

```text
unit-test wall-clock time
+ repeated embedded Dolt startup
+ full Runtime fixture use for pure decisions
+ hidden slow-test opacity
+ CI/deploy feedback latency
+ false confidence from skipped coverage
```

while preserving:

```text
real Dolt persistence/restart coverage
+ worker/candidate integration coverage
+ browser/product proof where required
+ traceable CI timing evidence
+ readable test ownership boundaries
```

Success is not a single speed number. Success is a suite whose slow parts are
intentional, named, and covered by the right target, while the common runtime
edit loop is fast enough to use repeatedly during a mission.

## Homotopy Axes

- Measurement:
  no visibility -> package timing -> per-test timing -> CI slow-test summary
- Store dependency:
  embedded Dolt everywhere -> fake store for unit tests -> Dolt only where truth requires it
- Runtime dependency:
  full `Runtime` fixture by default -> pure helpers and small interfaces -> full runtime only for integration
- Test classification:
  mixed package soup -> unit/integration/browser/live targets -> documented coverage map
- CI behavior:
  opaque `go test ./...` -> tiered, timed jobs -> scheduled/manual heavyweight coverage where appropriate

## Implementation Gradient

### 1. Measure Before Refactoring

Run the fastest safe timing probes first:

```text
nix develop -c go test -json ./internal/runtime ./internal/provider ./internal/api ...
```

Capture:

- slowest packages;
- slowest individual tests;
- repeated embedded Dolt setup counts;
- sleeps, retries, polls, live-ish workers, and browser dependencies;
- tests that are pure in intent but integration-shaped in fixture.

If the package list is too broad, shrink it to the currently implicated
runtime/provider/API packages and record why.

### 2. Extract Pure Helpers

Move small decisions behind pure functions that can be tested without store,
Runtime, API setup, or Dolt:

- prompt routing decisions;
- VText grounding requirements;
- model max-token/catalog lookup;
- provider capability classification;
- tool/role policy selection;
- idempotency and state-machine transitions that can be represented as values.

Keep the helpers in production code, not test-only shadow logic. Tests should
exercise the same functions runtime uses.

### 3. Add A Fake Or In-Memory RuntimeStore Interface

Where a test truly needs runtime/API behavior but not disk persistence, introduce
a narrow interface seam instead of opening embedded Dolt.

The interface should be driven by real runtime use, not a broad mock of all
store behavior. It may begin with the VText/runtime/API methods needed by the
slowest unit-shaped tests.

Required properties:

- deterministic;
- no network;
- no Dolt process or filesystem database;
- records enough calls/state to assert behavior;
- small enough that adding it does not create a parallel product database.

Do not force a whole-store abstraction if a narrower `RuntimeStore` or
feature-specific store interface is enough.

### 4. Preserve Embedded Dolt For Integration Truth

Keep embedded Dolt tests for:

- VText revision persistence and reload;
- computer/source lineage persistence;
- app package/adoption records;
- compaction/run-memory persistence;
- schema migrations;
- restart/recovery behavior;
- any bug whose root cause depends on real Dolt behavior.

Reduce duplication by sharing setup helpers where safe, but avoid hidden global
state that makes tests order-dependent.

### 5. Split Heavyweight Targets Explicitly

Mark or move tests whose purpose is inherently heavy:

- worker VM lifecycle;
- live gateway/model/search calls;
- browser/Playwright proof;
- restart/recovery with real processes;
- long polling/retry paths;
- staging smoke.

Preferred shape:

```text
go test ./...                         # fast meaningful default
go test -tags=integration ./...       # embedded Dolt/restart/heavy integration
go test -tags=browser ./...           # browser proof tests where applicable
manual/staging workflows              # real worker/staging proof
```

Only split a test out of default CI when there is an explicit replacement
coverage path and the mission doc/final report names it.

### 6. Add CI Timing Visibility

CI should make slow tests impossible to miss.

At minimum, add package-level timing. Prefer per-test timing summaries for the
top slow tests.

Good output:

```text
slowest Go packages:
  internal/runtime  42.1s
  internal/api      18.4s

slowest tests:
  TestRuntimeRestartPreservesVText  12.7s
  TestWorkerBootstrapRetries        9.3s
```

The timing report should be available in GitHub Actions logs and, if practical,
as an artifact.

## Evidence Ledger Requirements

For each meaningful change, record:

- before and after timing for affected package/test targets;
- whether a test moved from embedded Dolt to pure/fake store;
- what coverage remains in default CI;
- what coverage moved to integration/browser/manual/staging;
- exact command outputs or artifact refs;
- CI run URL/id after push;
- staging identity and product proof only if runtime behavior changed.

## Anti-Goodhart Constraints

- Do not improve timing by deleting assertions, swallowing errors, or replacing
  behavior tests with shallow existence tests.
- Do not create fake helpers whose only caller is the test.
- Do not make fake-store behavior more permissive than production store behavior
  in ways that hide bugs.
- Do not leave both old integration-shaped tests and new pure tests if the old
  tests no longer prove additional behavior.
- Do not create a broad mock framework that becomes harder to trust than the
  embedded Dolt tests it replaces.

## Rollback Policy

Rollback is straightforward for test-only refactors: revert the commit.

For CI workflow changes, preserve the previous workflow behavior in git history
and make the final report name:

- old command shape;
- new command shape;
- which targets run by default;
- which targets run as integration/browser/manual/scheduled.

If a split target misses a real failure or creates ambiguity, restore the
previous default coverage before attempting a more surgical split.

## Stopping Condition

`complete` requires all of:

- measured slow-test baseline captured;
- pure helper extraction performed for the most obvious policy/routing/model
  tests currently paying Runtime/Dolt cost;
- fake or in-memory store seam added for at least one runtime/API unit-test
  class where it removes real embedded Dolt cost without weakening assertions;
- embedded Dolt tests retained for persistence/restart/integration behavior and
  explicitly separated from pure unit tests;
- heavyweight worker/live/browser/restart tests classified into explicit targets
  or documented as remaining debt with exact blockers;
- CI emits useful test timing output;
- focused local tests pass under `nix develop`;
- CI passes after push;
- final report includes timing evidence, coverage tradeoffs, rollback refs,
  residual risks, and next realism axis.

`checkpoint_incomplete` is acceptable only if useful refactors land but the
suite remains too slow or classification is partial. The mission doc must then
include a resumable checkpoint and the next executable probe.

`blocked_incomplete` is reserved for a named blocker after root-cause probes,
such as a Dolt/store API that cannot be safely abstracted without an architecture
decision, or a CI limitation that needs human/operator authority.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: first hardening pass extracted prompt/policy tests away from embedded Dolt, added a narrow fake run-submission store seam, classified live/browser/real-LLM tests behind an integration tag, and added timing summaries to local/CI Go test paths
current artifact state: common prompt/model-policy tests no longer need full Runtime/Dolt; run-submission persistence has a narrow store interface tested by an in-memory fake; integration-tagged smoke exists and is wired into CI; many runtime/API tests still open embedded Dolt/full Runtime and dominate wall-clock
what shipped: not yet pushed from this checkpoint
what was proven:
  - baseline full `nix develop -c go test -json ./internal/runtime ./internal/provider ./internal/modelcatalog` timed out at 300s with internal/runtime still active
  - focused prompt/policy/fake-store validation passed; timing summary showed internal/runtime 5.13s and provider 2.15s, with pure prompt/policy tests at 0.00s and only intentionally persisted tests around 0.9-1.0s
  - integration-tagged smoke passed in local dev shell; provider 1.25s, runtime 4.17s, live/browser/real-LLM tests skipped immediately without credentials or explicit env
  - representative runtime shard 0/4 passed: 118 of 470 tests, internal/runtime 100.57s package time, 119.37s wall including dev shell; slowest tests were mostly 1-3s full Runtime/Dolt tests
unproven or partial claims:
  - full runtime shard matrix has not been rerun after this pass
  - CI has not yet been pushed/observed for the timing-summary jobs
  - API/runtime fake-store seams beyond run submission are not implemented
coverage tradeoff:
  - live/browser/real-LLM tests are no longer default unit tests, but compile/run under `scripts/go-test-integration` with `-tags=integration`; env-gated live behavior remains explicit
  - embedded Dolt remains in default runtime shards for persistence/restart/worker/API behavior
belief-state changes:
  - test opacity is partly solved by timing summaries
  - the major remaining cost is not a few pathological tests but many full Runtime/Dolt tests clustered around 0.9-1.5s each
  - prompt/policy tests should use `testPromptRuntime` or pure helpers by default
remaining error field: runtime/API tests still overuse full Runtime fixtures for behavior that could be tested through pure helpers or narrow fake stores
highest-impact remaining uncertainty: which API handlers and worker/channel helpers can safely move to fake-store tests without losing persistence/restart coverage
next executable probe: extract an API/runtime unit fixture around run status/events/VText queue behavior and leave one Dolt-backed integration test per persistence contract
suggested resume goal string: rerun this mission with focus on replacing one-second API/runtime fixtures, not further timing instrumentation
evidence artifact refs:
  - `/usr/bin/time -p timeout 180s nix develop -c scripts/go-test-with-timing ... focused prompt/policy/fake-store tests`
  - `/usr/bin/time -p timeout 120s nix develop -c scripts/go-test-integration`
  - `/usr/bin/time -p timeout 240s nix develop -c env TOTAL_SHARDS=4 SHARD_INDEX=0 scripts/go-test-runtime-shards`
rollback refs: revert the eventual commit that stages this checkpoint; no deployed platform state changed yet
```
