# Choir Autopaper: Source-to-Edition Activation and Sourcecycled Stability

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-autopaper-activation-2026-07-10.md
```

Read this document as executable semantic authority for getting Autopaper
working end-to-end in the deployed platform. Reconcile current source and
staging state, execute the receding-horizon loop below, update the definition
graph and evidence ledger, and continue until the completion semantics are
satisfied with named evidence or a hard blocker/supersession condition is
produced. A checkpoint is not completion.

## Source Authority Order

1. This Definition.
2. `AGENTS.md` and `docs/choir-doctrine.md`.
3. `docs/definitions/choir-product-completion-2026-07-10.md` (PC-6 Autopaper
   single authoritative activation; this mission is a sub-mission of PC-6).
4. `docs/computer-ontology.md` (VM, sandbox, candidate-world, promotion,
   package, and persistent-state behavior).
5. `docs/agent-product-doctrine.md` (authority boundaries, harness minimalism,
   Texture control plane, runtime configuration, product-path verification).
6. `docs/runtime-invariants.md` and `docs/texture-agentic-invariants-2026-06-13.md`.
7. `docs/source-external-data-publication.md` (source cycle → publication
   semantics).
8. Observed source:
   - `cmd/sourcecycled/main.go` and `cmd/sourcecycled/main_test.go`
   - `internal/cycle/ingestion_handoff.go` and `internal/cycle/cycle.go`
   - `internal/runtime/sourcecycled_web_captures.go`
   - `internal/runtime/api.go` (typed ingestion handoff idempotency)
   - `internal/vmctl/platform_computer.go` and `internal/vmctl/handlers.go`
     (`HandleSandboxProxy`, `HandleResolve`)
   - `internal/proxy/handlers.go` (`/api/universal-wire/stories`)
9. Staging evidence: `systemctl` status/logs, `journalctl`, `vmctl` pulse/health,
   sourcecycled `/health/ready`, runtime `/health`, and the resulting run,
   Texture, and universal-wire objects.

This Definition does not override OG/Dolt/heresy promotion semantics or
RouteProfile authority. It is a product-path sub-mission whose fixes must be
compatible with the settled seam-repair state.

## Mutation Class

The mission is **red** overall because it touches VM lifecycle, sourcecycled
dispatch, runtime ingestion admission, and the public `/api/universal-wire/stories`
path. Individual phases may be lower class:

- **Phase 0**: doc and observation-only updates (this Definition, log captures,
  graph state) — **green**.
- **Phase 1**: targeted tests and instrumentation that do not change product
  behavior — **yellow**.
- **Phase 2**: runtime/sourcecycled configuration, timeouts, retry/backoff, and
  logging changes — **orange** (rollback to pre-change config + runtime flags).
- **Phase 3**: VM lifecycle, recovery, health-check, or route resolution changes —
  **red** (full ceremony: conjecture delta, protected surfaces, admissible
  evidence, rollback, heresy delta).
- **Phase 4**: ingestion-handoff idempotency or activation path changes — **red**.
- **Phase 5**: public API/Texture publication contract changes — **red**.

## Real Artifact / Object Of Work

The real object is the deployed source-to-edition product path, not the
historical Autopaper corpus or a separate service. It is:

- `cmd/sourcecycled` polling configured sources and producing one typed ingestion
  handoff per cycle;
- `internal/vmctl` resolving and maintaining the platform computer sandbox so
  sourcecycled can reach `/internal/runtime/objectgraph/web-captures` and
  `/internal/runtime/runs`;
- `internal/runtime` receiving web captures, durably storing them, and accepting
  exactly one processor run and one reconciler run per ingestion handoff;
- the processor/reconciler runs producing canonical Texture artifacts and an
  edition inside the platform computer;
- `internal/proxy` exposing `/api/universal-wire/stories` that returns the
  current edition's stories;
- `cmd/choir wire stories` and the web UI consuming the same endpoint.

The object is not a new autopaper service, a new scheduler, a resurrected
projection-triggered `wire_synthesis` path, or a passing unit test that does not
run through the deployed sourcecycled → platform → runtime → Texture path.

## Mission Purpose And Non-Purpose

**Purpose:**

1. Stop the platform computer and/or sourcecycled reboot or restart loop that
   prevents Autopaper from completing a cycle.
2. Prove one sourcecycled cycle produces exactly one durable ingestion handoff,
   one processor run, and one reconciler run.
3. Prove that handoff is not duplicated under retry, restart, or overlapping
   sourcecycled dispatch.
4. Prove the platform computer stays stable (no unplanned reboot/recovery) for
   at least one full source cycle and the resulting runtime runs.
5. Prove `/api/universal-wire/stories` returns stories produced from the cycle
   (a canonical Texture edition is visible).

**Non-purpose:**

- This mission does not design a new Autopaper service or a free-form scheduler.
- It does not resurrect projection-triggered activation.
- It does not settle personal ownership, per-user schedule, or edition
  acceptance semantics beyond the platform computer (`universal-wire-platform`)
  and the sourcecycled configuration already present.
- It does not add new provider/model integrations.
- It does not change the settled OG/Dolt/heresy promotion spine or RouteProfile
  format.

## Definition Graph

```yaml
id: autopaper
kind: term
status: proposed
source: user-restated 2026-07-10 + PC-6
definition: >-
  Autopaper is the automatic publication program inside a Choir platform
  computer: scheduled source configurations produce typed observations; one
  authoritative ingestion handoff activates a processor run and a reconciler run;
  canonical Texture artifacts become an edition only through explicit publication
  contracts. It is not a separate service and does not bypass Texture or
  provenance authority.
non_definition:
  - Any separate binary or service named "autopaper".
  - Any projection that starts a run before the typed ingestion handoff.
  - Any manual UI click or CLI command that triggers the same publication.
observables:
  - A sourcecycled cycle summary with one processor and one reconciler request.
  - A runtime run object with `agent_profile: processor` and
    `agent_profile: reconciler` tied to the same `ingestion_handoff_cycle_id`.
  - A Texture edition doc visible to `/api/universal-wire/stories`.
execution_effect:
  - All product paths that consume the universal-wire feed must target the
    edition produced by this single path.
settlement:
  rule: The definition is settled when the completion semantics of this mission are met.
  settled_by: evidence
  invalidation_triggers:
    - A second activation path is discovered.
    - The platform route returns stories from a different source than the edition.

---
id: sourcecycled
kind: object
status: settled
source: observed
term: Sourcecycled daemon
definition: >-
  The `cmd/sourcecycled` host process that polls `sources.Registry`, projects
  captured items to the objectgraph, builds `IngestionHandoff`, and dispatches
  processor/reconciler requests to `/internal/runtime/runs` via the vmctl
  sandbox proxy.
non_definition:
  - A service that boots the platform computer.
  - A service that writes Texture editions directly.
observables:
  - `go-choir-sourcecycled` systemctl status.
  - `/internal/source-service/health` and `/health/ready` responses.
  - `journalctl -u go-choir-sourcecycled` restart/crash frequency.
execution_effect:
  - The sourcecycled daemon is the only source-cycle trigger for the Autopaper
    processor/reconciler.
settlement:
  rule: Observed to be running and cycling without unplanned restart during a proof cycle.
  settled_by: evidence

---
id: platform_computer
kind: object
status: settled
source: observed
term: Universal Wire platform computer
definition: >-
  The always-on `universal-wire-platform` VM managed by `vmctl`. It hosts the
  runtime API (`/internal/runtime/*`), objectgraph, and the processor/reconciler
  runs that produce the Autopaper edition.
non_definition:
  - A physical machine.
  - The sourcecycled daemon itself.
observables:
  - `vmctl` list/pulse output showing `universal-wire-platform` state.
  - `vmctl` `/internal/vmctl/resolve` for the platform owner.
  - Runtime `/health` inside the sandbox.
  - `journalctl` for `go-choir-vmctl` and VM boot logs.
execution_effect:
  - The platform computer is the execution substrate for the Autopaper runs.
settlement:
  rule: Observed to be active and healthy for the duration of a proof cycle.
  settled_by: evidence

---
id: vmctl_sandbox_proxy
kind: object
status: settled
source: observed
term: vmctl sandbox proxy
definition: >-
  The `HandleSandboxProxy` path `/internal/vmctl/sandbox-proxy/{owner}/{rest}`
  that resolves the owner (booting the platform computer if needed) and
  reverse-proxies the request to the sandbox runtime.
non_definition:
  - A direct TCP/HTTP connection from sourcecycled to the runtime.
observables:
  - `internal/vmctl/handlers.go` route registration.
  - Proxy logs showing `EnsureUniversalWirePlatformComputer` and forwarded path.
execution_effect:
  - Sourcecycled must reach the runtime through this proxy and no other path.
settlement:
  rule: Observed to forward sourcecycled requests to the runtime inside the platform VM.
  settled_by: evidence

---
id: ingestion_handoff
kind: object
status: settled
source: observed
term: Typed ingestion handoff
definition: >-
  The `IngestionHandoff` produced by `internal/cycle.BuildIngestionHandoff` and
  stored by sourcecycled. It contains one `ProcessorRequest` and optional
  `ReconcilerRequest` records, each with `ingestion_handoff_request_id`,
  `ingestion_handoff_request_kind`, and `ingestion_handoff_cycle_id`.
non_definition:
  - A raw source item.
  - A web-capture object.
  - A projection-triggered run.
observables:
  - `/internal/source-service/ingestion-handoff/latest` response.
  - `cycle.ProcessorRequest` and `cycle.ReconcilerRequest` records.
  - Runtime `ListRunsByIngestionHandoff` result.
execution_effect:
  - The handoff is the sole durable activation identity for the processor and
    reconciler runs.
settlement:
  rule: Observed in sourcecycled and runtime with matching IDs.
  settled_by: observed

---
id: single_authoritative_activation
kind: invariant
status: proposed
source: PC-6
definition: >-
  One non-empty sourcecycled cycle must produce exactly one processor run and one
  reconciler run, regardless of sourcecycled restarts, runtime restarts, or
  retry storms. The runtime's `ListRunsByIngestionHandoff` and fingerprint check
  must reject duplicates.
observables:
  - Two consecutive sourcecycled cycles with the same `cycle_id` produce one
    processor run ID.
  - Runtime `og_objects` for `choir.run` has exactly one `processor` and one
    `reconciler` per `ingestion_handoff_cycle_id`.
  - `/internal/source-service/ingestion-handoff/latest` shows `submitted` status
    for the processor and no second `submitted` or `running` record.
execution_effect:
  - No second processor run may be created for the same handoff.
settlement:
  rule: Proven by integration or deployed test with restart/retry injection.
  settled_by: formal-check

---
id: platform_computer_stability
kind: invariant
status: proposed
source: user-stated 2026-07-10
definition: >-
  The platform computer must not enter an unplanned reboot, recovery, or
  stop/start loop during a sourcecycled cycle or the resulting runtime run.
  `vmctl` may intentionally boot a stopped VM once, but must not repeatedly
  recover it.
observables:
  - `journalctl -u go-choir-vmctl` shows at most one boot/recovery for the
    platform VM in the proof window.
  - `vmctl` `/internal/vmctl/pulse` shows the platform VM state as `running`
    throughout the proof cycle.
  - `vmctl` `/internal/vmctl/resolve` returns `200` within bounded time and does
    not return `503` on repeated calls.
execution_effect:
  - Sourcecycled and runtime requests are not interrupted by VM lifecycle churn.
settlement:
  rule: Proven by staging observation over a full source cycle.
  settled_by: evidence

---
id: sourcecycled_liveness
kind: invariant
status: proposed
source: user-stated 2026-07-10
definition: >-
  The `go-choir-sourcecycled` service must not crash or be restarted by systemd
  in a loop. A restart may happen once on deploy, but repeated restarts within a
  source-cycle window indicate a liveness bug.
observables:
  - `systemctl status go-choir-sourcecycled` shows `Active` for the duration.
  - `journalctl -u go-choir-sourcecycled` shows no crash or exit between cycle
    start and edition visibility.
  - `/health/ready` returns `200` with non-degraded status.
execution_effect:
  - The source cycle is not re-triggered from a clean state mid-flight.
settlement:
  rule: Proven by staging observation over a full source cycle.
  settled_by: evidence

---
id: ingestion_handoff_idempotency
kind: invariant
status: proposed
source: observed + PC-6
definition: >-
  Dispatching the same `IngestionHandoff` more than once (sourcecycled retry,
  drain ticker overlap, or sourcecycled restart) must not create a second run
  with the same `ingestion_handoff_request_id` and `request_kind`. The runtime
  must return the existing run or `409 Conflict`.
observables:
  - `internal/runtime/api.go` `ListRunsByIngestionHandoff` and fingerprint logic.
  - `cmd/sourcecycled/main.go` `dispatch` and `submit` retry logic.
  - `internal/runtime/internal_run_idempotency_test.go`.
execution_effect:
  - Duplicate sourcecycled dispatch is safe and observable.
settlement:
  rule: Proven by code review plus a test that retries a handoff and asserts one run.
  settled_by: formal-check

---
id: root_cause_reboot_loop
kind: conjecture
status: testing
source: user-stated 2026-07-10
definition: >-
  The platform computer and/or sourcecycled are observed to reboot or restart in
  a loop. The root cause is currently unknown. Candidate hypotheses:
  - H1: vmctl health check / `recoverOrRestartActiveVM` fails repeatedly because
    the runtime inside the platform computer is not healthy, causing a VM
    recovery loop.
  - H2: sourcecycled's short retry/backoff (8 attempts, 2-second delay) hammers
    the sandbox proxy while the VM is still booting, causing the proxy to
    trigger repeated `EnsureUniversalWirePlatformComputer` attempts.
  - H3: sourcecycled's 5-minute HTTP timeout and 1-minute drain ticker overlap,
    creating concurrent dispatch attempts that overload the VM.
  - H4: the processor or reconciler run inside the VM exits or OOMs, causing the
    VM health check to fail and vmctl to reboot.
  - H5: an external caller (e.g., health check, UI, `/api/universal-wire/stories`
    polling) repeatedly calls `vmctl` resolve, and each failed VM boot attempt
    produces a new recovery loop.
observables:
  - Staging logs and `systemctl` status for `go-choir-sourcecycled`,
    `go-choir-vmctl`, and the platform VM.
  - `vmctl` pulse/health and runtime health over the failure window.
  - Sourcecycled `/health/ready` and `/internal/source-service/health`.
  - Runtime `choir.run` objects created during the loop.
  - CPU, memory, and disk pressure for the platform VM.
execution_effect:
  - The chosen hypothesis determines whether the fix belongs in vmctl,
    sourcecycled, runtime, processor prompt, or the health-check caller.
  - If no hypothesis is supported, the mission is blocked and requires an
    observer upgrade (more logging, synthetic probe, or manual instrumentation).
settlement:
  rule: The true hypothesis is the one whose predicted evidence matches the observed logs.
  settled_by: evidence

---
id: observe_and_reproduce
kind: operator
status: proposed
source: inferred
definition: >-
  Start by observing staging without changing code. Capture the failing window
  in `journalctl`, `systemctl status`, `vmctl` pulse, and sourcecycled health.
  If the loop is not active, force a single sourcecycled cycle by temporarily
  reducing the source interval or using a synthetic source, and capture the
  behavior.
observables:
  - Time-ordered log of sourcecycled start, cycle, dispatch, runtime response,
    vmctl boot/recovery, and VM health.
  - Count of `choir.run` objects created per `ingestion_handoff_cycle_id`.
execution_effect:
  - This is the first move. No code changes until the loop is reproduced or
    explained by existing logs.
settlement:
  rule: A reproducible trace or a bounded capture of the live loop is in hand.
  settled_by: evidence

---
id: trace_and_fix
kind: operator
status: proposed
source: inferred
definition: >-
  After the root cause is identified, make the smallest fix that removes the
  loop while preserving the single-authoritative-activation invariant. The fix
  may be in vmctl, sourcecycled, runtime, or the processor/reconciler prompt.
observables:
  - A diff whose changes correspond exactly to the supported hypothesis.
  - Tests or probes that fail before the fix and pass after.
execution_effect:
  - Changes the behavior of the source-to-edition path.
settlement:
  rule: The fix is accepted only after the supported hypothesis is stated,
    protected surfaces are named, and a rollback ref is recorded.
  settled_by: human for red changes; orchestrator for orange.

---
id: prove_autopaper_end_to_end
kind: operator
status: proposed
source: inferred
definition: >-
  Run a complete source cycle (natural or synthetic) and prove that it produces
  one processor run, one reconciler run, a stable platform computer, a stable
  sourcecycled service, and a visible `/api/universal-wire/stories` response
  from a canonical Texture edition.
observables:
  - Sourcecycled cycle summary with one processor and one reconciler.
  - One `processor` run and one `reconciler` run per `ingestion_handoff_cycle_id`.
  - Platform VM state `running` with no unplanned recovery in the window.
  - `/api/universal-wire/stories` returns `200` with at least one story and
    `source: universal-wire-edition-texture`.
  - `cmd/choir wire stories` returns the same stories.
execution_effect:
  - Completes the mission if all evidence is in scope.
settlement:
  rule: All completion-semantics observables are recorded.
  settled_by: evidence
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: The seam-repair 2026-07-10 state is settled (RouteProfile format,
        build identity, dead-code deletion, and doc state).
      source: settled-definition
      execution_effect: No code or doc from this mission may contradict the settled seam.
    - claim: Autopaper is a program, not a separate service. The sole authoritative
        activation path is sourcecycled → typed ingestion handoff → runtime runs.
      source: PC-6 in choir-product-completion
      execution_effect: The mission must not add a new Autopaper service or resurrect
        projection-triggered activation.
    - claim: Sourcecycled polls sources, projects web captures to the objectgraph,
        builds `IngestionHandoff`, and dispatches processor/reconciler requests.
      source: observed
      execution_effect: All fixes must keep this flow intact; any change here is a
        red mutation.
    - claim: Sourcecycled dispatches to the runtime through the vmctl sandbox proxy
        (`/internal/vmctl/sandbox-proxy/universal-wire-platform/...`).
      source: observed
      execution_effect: Sourcecycled cannot bypass the proxy; the proxy is the
        only route to the runtime and the platform computer.
    - claim: The vmctl sandbox proxy calls `EnsureUniversalWirePlatformComputer`,
        which may boot, resume, or recover the platform VM.
      source: observed
      execution_effect: The platform computer lifecycle is triggered by
        sourcecycled requests and any other call to the proxy/resolve path.
    - claim: The runtime API has typed ingestion handoff idempotency checks
        (`ListRunsByIngestionHandoff` + fingerprint + `409 Conflict`).
      source: observed
      execution_effect: Duplicates must be rejected by the runtime, but the mission
        must verify this under retry and restart.
    - claim: The host sourcecycled and vmctl services remain running while the
        universal-wire platform guest repeatedly fails readiness and is recovered.
      source: observed
      execution_effect: The active failure is a platform guest/runtime startup
        failure, not a systemd restart loop in either host daemon.
    - claim: The universal-wire guest reaches its sandbox runtime service launcher,
        but the runtime does not bind port 8085 before vmctl's three-minute readiness
        deadline.
      source: observed
      execution_effect: Investigation moves inside sandbox startup before HTTP
        serving; sourcecycled retry cadence is downstream amplification, not yet
        the root cause.
    - claim: A fresh universal-wire guest stalls inside relational-to-objectgraph
        startup backfill after every earlier sandbox startup phase completes.
      source: observed
      execution_effect: The next probe belongs inside `backfillOGFromSQL`; changing
        vmctl readiness or sourcecycled retry timing would mask the substrate bug.
    - claim: The `choir.run` legacy backfill is the blocking branch, and the existing
        per-kind emptiness gate is implemented but not connected to startup migration.
      source: observed
      execution_effect: Connect `ogKindIsEmpty` at the per-kind backfill boundary;
        do not extend readiness timeouts or patch sourcecycled retries.
  open:
    - node: platform_computer_stability
      missing: Proof that the platform VM stays stable for a full cycle.
    - node: sourcecycled_liveness
      missing: Proof that sourcecycled does not restart during a cycle.
    - node: single_authoritative_activation
      missing: End-to-end proof under retry/restart.
    - node: prove_autopaper_end_to_end
      missing: A deployed source cycle producing a visible edition.
  contested: []
```

## Invariants

- The sourcecycled daemon is the only trigger that turns a source cycle into a
  processor/reconciler run.
- Web-capture projection persists observations; it does not start a run.
- The typed ingestion handoff is the only durable activation identity.
- The platform computer must be stable for the duration of a source cycle and
  its runs.
- The sourcecycled service must not restart during a cycle.
- Exactly one processor run and one reconciler run may be created per ingestion
  handoff.
- `/api/universal-wire/stories` must return the edition produced by the
  reconciler, not a stale or manually seeded artifact.

## Authority Boundaries

- This mission may change `cmd/sourcecycled`, `internal/vmctl` (platform computer
  and sandbox proxy), `internal/runtime` (typed ingestion handoff and web
  captures), and `internal/proxy` (`/api/universal-wire/stories` routing) only
  after the root cause is identified and the red mutation ceremony is applied.
- It may not change `docs/choir-doctrine.md`, `AGENTS.md`, `docs/computer-ontology.md`,
  or `docs/definitions/og-dolt-heresy-completion-2026-07-08.md`.
- It may not change the settled RouteProfile semantics or the build-identity
  per-service contract.
- It may not change the OG/Dolt/heresy promotion spine.

## Conjectures And Belief State

- **Primary conjecture**: the reboot loop is a downstream effect of the platform
  computer being unable to reach a healthy runtime state before sourcecycled
  (or another caller) retries, causing vmctl to recover the VM repeatedly.
- **Secondary conjecture**: sourcecycled's retry and drain cadence is too aggressive
  for the platform VM boot time and lacks awareness of the VM's readiness.
- **Tertiary conjecture**: the processor or reconciler run is crashing or OOMing
  inside the VM, making the VM appear unhealthy and triggering recovery.
- **Belief state**: the typed ingestion handoff path is structurally correct; the
  failure is in lifecycle stability, retry timing, or run payload, not in the
  authority path.

## Variant / Progress Measure

The variant is the count of unresolved blockers that prevent a single deployed
source cycle from producing a visible edition:

- `unresolved_reboot_loop`: 1 until the root cause is identified and fixed.
- `unstable_platform_computer`: 1 until the VM stays running for a full cycle.
- `unstable_sourcecycled`: 1 until the daemon stays alive for a full cycle.
- `unproven_single_activation`: 1 until retry/restart produces exactly one run.
- `unproven_end_to_end`: 1 until `/api/universal-wire/stories` returns the edition.

A passing move must reduce one of these. Motion that only produces more logs or
more test artifacts without reducing the variant is theater.

## Execution Operators

- `observe`: gather staging logs, metrics, and system state without changing code.
- `reproduce`: trigger a source cycle with a synthetic or trimmed source and capture
  the behavior.
- `trace`: add short-lived logging or metrics to identify the exact transition that
  causes the loop.
- `instrument`: add a focused test or probe that reproduces the failure.
- `construct`: apply the code/config fix that removes the loop.
- `verify`: run the targeted integration/staging proof and confirm the invariant.
- `settle`: promote a definition node to `settled` when evidence is in scope.
- `escalate`: raise a group-level decision to the human if the root cause is a
  doctrine/authority change, an expensive substrate replacement, or a trade-off.

## Receding-Horizon Control Loop

1. **Select** the open node that most reduces mission uncertainty. The first node
   is `root_cause_reboot_loop`.
2. **State** what the current observer can see and cannot see. Name the blind
   spots (e.g., whether the VM is actually booting or the runtime is failing
   health checks).
3. **Choose** the next move: `observe`, `reproduce`, `trace`, `instrument`,
   `construct`, `verify`, `settle`, or `escalate`.
4. **Bound** the mutation radius. For code changes, identify the protected
   surfaces and the rollback path. For observation, identify what to capture and
   for how long.
5. **Execute** the move.
6. **Verify** the evidence. Do not trust process logs; use actual state (run
   objects, VM status, endpoint responses, edition visibility).
7. **Update** the definition graph, determined state, evidence ledger, and run
   checkpoint.
8. **Continue** if any variant is non-zero. If the variant is zero and all
   completion semantics are satisfied, report `complete`. If a safe move is
   impossible, report `blocked_incomplete` with the exact blocker and required
   authority.

## Forbidden Collapses

- `sourcecycled is running` does not mean `Autopaper is working`.
- `the platform VM is active` does not mean `the VM is healthy`.
- `one run object exists` does not mean `single authoritative activation` is proven.
- `a test passes` does not mean `the deployed path is stable`.
- `a story appears in /api/universal-wire/stories` does not mean it came from the
  intended source cycle unless the sourcecycled, runtime, and Texture lineage are
  traced.
- `a fix removes the loop locally` does not mean it is proven on staging.
- `a checkpoint is reached` is not `complete`.

## Evidence Ledger

```yaml
evidence_ledger:
  - claim: Sourcecycled is the only source-cycle trigger and dispatches through the vmctl proxy.
    definition_node: sourcecycled
    evidence_class: code-level proof
    source: cmd/sourcecycled/main.go + internal/vmctl/handlers.go
    command_or_observation: grep and read
    artifact_path: internal/cycle/ingestion_handoff.go
    result: Confirmed
    uncertainty: Does not cover behavior under sourcecycled restart.
    promotion_relevance: Justifies the single-authoritative-activation path.
  - claim: Runtime has typed ingestion handoff idempotency checks.
    definition_node: ingestion_handoff_idempotency
    evidence_class: code-level proof
    source: internal/runtime/api.go
    command_or_observation: grep ListRunsByIngestionHandoff
    artifact_path: internal/runtime/api.go
    result: Confirmed
    uncertainty: Does not cover races between sourcecycled retries and runtime commits.
    promotion_relevance: Single-authoritative-activation is partly enforced.
  - claim: The platform computer lifecycle is triggered by the sandbox proxy.
    definition_node: platform_computer
    evidence_class: code-level proof
    source: internal/vmctl/platform_computer.go
    command_or_observation: grep EnsureUniversalWirePlatformComputer
    artifact_path: internal/vmctl/platform_computer.go
    result: Confirmed
    uncertainty: Does not identify why the VM reboots.
    promotion_relevance: The fix surface may be in vmctl.
  - claim: The deployed failure is a repeated platform-guest readiness failure,
      while sourcecycled and vmctl themselves remain live.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof
    source: Node B systemd and journal observation on 2026-07-10
    command_or_observation: >-
      systemctl show go-choir-sourcecycled go-choir-vmctl; journalctl -u
      go-choir-sourcecycled -u go-choir-vmctl --since 2026-07-10T18:30:00Z;
      GET http://127.0.0.1:{8083/health,8787/health/ready}
    artifact_path: docs/definitions/choir-autopaper-activation-2026-07-10.md
    result: >-
      From 18:30 UTC through the observation, vmctl logged 17 Firecracker boots,
      16 guest-readiness timeouts, and sourcecycled logged 16 failed web-capture
      projections. Every timeout ended after three minutes with connection refused
      on the guest's port 8085 and a killed/failed Firecracker process. Both host
      services reported NRestarts=0 and healthy service endpoints. The host pressure
      sample reported 82.3% memory available and no memory, CPU, IO, or disk pressure.
    uncertainty: >-
      The guest console reports that the sandbox runtime service started and that
      its wire publish URL is configured, but emits none of cmd/sandbox's later
      runtime-topology logs. The exact blocking call before HTTP listen is not yet
      identified; persistent workspace bootstrap, Dolt maintenance, and store.Open
      remain candidates.
    promotion_relevance: >-
      Supports H1 and falsifies a sourcecycled systemd restart or host-pressure/OOM
      explanation. Does not yet authorize a VM lifecycle or runtime fix.
  - claim: The blocking pre-listen transition is `backfillOGFromSQL`.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof
    source: CI run 29118850649, Node B deploy job 86450906080, and vmctl guest console
    command_or_observation: >-
      Deploy c6b422bb; verify https://choir.news/health reports c6b422bb; inspect
      journalctl -u go-choir-vmctl from 2026-07-10T19:55:00Z.
    artifact_path: cmd/sandbox/main.go + internal/store/store.go
    result: >-
      On a fresh boot, source workspace bootstrap, Dolt maintenance, workspace open,
      runtime schema, objectgraph schema, Texture schema, and legacy import all
      completed. `objectgraph-backfill status=starting` appeared at 19:56:21 UTC;
      no completion marker followed, and vmctl marked the guest failed at 19:58:14
      after its three-minute readiness bound. The guest never bound port 8085.
    uncertainty: >-
      `backfillOGFromSQL` serially executes agents, runs, events, channel messages,
      worker updates, run acceptance/continuation, browser session, trajectory,
      work-item, and Texture-table backfills. The stalled branch is not yet named.
    promotion_relevance: >-
      Authorizes per-kind backfill instrumentation. Does not authorize extending
      the boot timeout or skipping legacy data migration.
  - claim: Unconditionally replaying relational runs is the direct readiness blocker.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof + code-level proof
    source: CI run 29120219036, Node B deploy job 86455558809, and store migration source
    command_or_observation: >-
      Deploy e8dda030; inspect fresh guest journal from 2026-07-10T20:19:24Z;
      read internal/store/{migration.go,graph_store.go}.
    artifact_path: internal/store/migration.go
    result: >-
      The fresh guest completed `agents` in 0.43 seconds, entered `runs` at
      20:19:27 UTC, emitted no completion marker, and was marked failed at
      20:22:19 after the readiness deadline. `backfillRunsOG` loads every legacy
      run and performs one OG lookup per record. `ogKindIsEmpty` already exists
      with the explicit contract that per-kind backfill runs only for empty OG
      kinds, but migration does not call it.
    uncertainty: >-
      A populated-kind fast path must be proved not to suppress first-time migration
      of an empty kind. Later kinds may have the same scaling defect and must use
      the same gate rather than fixing only runs.
    promotion_relevance: >-
      Settles `root_cause_reboot_loop` and authorizes the bounded substrate repair.
  - claim: A populated-kind gate fixes the completed run migration but is not a
      sufficient completion protocol for an interrupted first migration.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof + code-level counterexample
    source: a3ebc171 fresh-boot trace on Node B
    command_or_observation: >-
      POST /internal/vmctl/resolve for universal-wire-platform after deployment;
      inspect per-kind vmctl guest-console markers from 2026-07-10T20:45:57Z.
    artifact_path: internal/store/migration.go
    result: >-
      A fresh guest skipped populated agents, runs, and events; migrated empty
      channel-messages in 18 seconds; then remained in empty worker-updates. If
      vmctl kills that guest mid-kind, the next boot observes a non-empty but
      incomplete worker-update kind. Therefore non-empty is not equivalent to
      migration-complete.
    uncertainty: >-
      The repair needs a completion-aware/resumable migration protocol and must
      keep the runtime health endpoint available long enough for a large first
      migration to finish without lifecycle recovery.
    promotion_relevance: >-
      Prevents promoting `platform_computer_stability` from the reattach trace and
      reopens the repair implementation while leaving the root cause settled.
  - claim: Deferred migration must begin only after synchronous runtime recovery
      and listener publication.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof
    source: 94f6c744 fresh-boot trace on Node B
    command_or_observation: >-
      Inspect vmctl guest-console ordering after deploy job 86464853189.
    artifact_path: cmd/sandbox/main.go
    result: >-
      On the persistent platform guest, `objectgraph-backfill status=deferred` and
      orchestration topology logged, then the background migration entered `runs`.
      The synchronous `rt.Start` recovery did not return and the server-listen marker
      did not appear because both paths contend for the single embedded-Dolt handle.
      A small-store guest completed `rt.Start`, bound 0.0.0.0:8085, and finished the
      same migration, proving the deferred protocol but falsifying its launch order.
    uncertainty: >-
      Runtime recovery must finish against the already-authoritative OG state before
      migration starts; migration should start only after the TCP listener is published.
    promotion_relevance: >-
      Authorizes an ordering-only correction. Completion markers and SQL preservation
      remain the migration authority.
  - claim: A stale concurrent platform ensure can fail a newer healthy VM generation.
    definition_node: root_cause_reboot_loop
    evidence_class: deployed staging proof
    source: cb694846 fresh-boot observation on Node B
    command_or_observation: >-
      POST /internal/vmctl/resolve for universal-wire-platform at 21:29:04 UTC;
      inspect vmctl journal and direct guest health/process state through 21:31:41 UTC.
    artifact_path: internal/vmmanager + internal/vmctl platform lifecycle
    result: >-
      Generation 10.200.138.2 started at 21:29:05, bound :8085 at 21:29:21,
      and vmmanager recorded epoch 8095 booted at 21:29:22. At 21:30:47 an older
      concurrent ensure waiting on 10.200.137.2 hit its three-minute deadline,
      marked the shared vm-universal-wire-platform identity failed, and the newer
      Firecracker process disappeared. Direct health to 10.200.138.2 then timed out.
    uncertainty: >-
      The exact stale-write guard is not yet identified. Epoch/generation authority
      must prevent an older waiter from mutating or killing the current generation.
    promotion_relevance: >-
      Reopens platform stability and the reboot-loop conjecture. The resumable store
      migration repair remains supported but cannot settle lifecycle stability alone.
  - claim: The exact-SHA deploy verifier collapses workflow identity and sandbox
      artifact identity during vmctl-only active-computer refreshes.
    definition_node: platform_computer_stability
    evidence_class: CI/deploy trace + code-level proof
    source: CI run 29125293553, failed Node B deploy job 86471236444
    command_or_observation: >-
      Inspect the deploy impact classes, selected builds, refresh trace, and
      wait_for_sandbox_commit implementation for 83b1f594.
    artifact_path: .github/workflows/ci.yml + .github/scripts/deploy-impact-classify
    result: >-
      All standard and race lanes passed. The deploy selected and activated only
      the vmctl host package, intentionally skipped the sandbox package and guest
      images, refreshed an active computer successfully, then waited for that guest
      to report workflow commit 83b1f594. The guest correctly reported the installed
      sandbox artifact cb694846, so the verifier timed out and recorded incomplete
      deployment evidence. f2d1d330 introduced the unconditional workflow-SHA
      comparison while the classifier still models vmctl and sandbox as independent
      deployed artifacts.
    uncertainty: >-
      A refreshed guest must prove readiness and the identity of the sandbox artifact
      actually selected for that deployment. Workflow-SHA equality remains mandatory
      only when sandbox or the ordinary guest artifact was rebuilt from that SHA.
    promotion_relevance: >-
      Blocks exact-SHA deployment acceptance for the lifecycle repair without
      falsifying its tests or host activation. The verifier must preserve artifact
      identity boundaries before the staging lifecycle proof can resume.
  - claim: Sourcecycled can report a fresh processor submission while runtime
      returns a completed run receipt from an older ingestion cycle.
    definition_node: single_authoritative_activation
    evidence_class: deployed staging trace + code-level proof
    source: exact-SHA 838a4799 staging window beginning 2026-07-10T22:11:06Z
    command_or_observation: >-
      Correlate sourcecycled dispatch events after cycle_75496e7e24d94c238f8d6788
      with runtime lifecycle logs and GET /internal/runtime/runs/{id}.
    artifact_path: cmd/sourcecycled/main.go + internal/runtime/api.go
    result: >-
      Sourcecycled recorded processor_submitted=1 at 22:18:52, 22:19:51,
      22:22:03, and 22:22:50 with no errors. Runtime created only
      ecd89fa9-6f09-466c-9891-370d95dc28a3 at 22:18:51 for request
      processor_6fbe0869375a2d455cf05036 in cycle_59a77f8f0b5c99d93ddba86b,
      then completed it at 22:20:25. The later cycle_7998f11f3eff16dabd1817ec
      dispatch event again reported that same run id even though its own two
      processor requests had different request ids and remained queued. No new
      runtime submission lifecycle entry existed for the later drains.
    uncertainty: >-
      The exact stale-receipt boundary is not yet settled. Typed runtime
      idempotency is request-id scoped in source, while the live sourcecycled
      MemoryStore path is not covered by the Dolt-backed dispatcher regression
      test. The source ledger update, queue selection, proxy status reconciliation,
      and runtime lookup must be reproduced together before repair.
    promotion_relevance: >-
      Reopens single authoritative activation. A reused completed receipt can
      mark a new handoff submitted without processing its source handles, so a
      stable VM alone cannot produce an authoritative Autopaper edition.
  - claim: Universal Wire publication bootstraps a missing edition alias but
      cannot recover when the alias points to a missing Texture document.
    definition_node: edition_visible
    evidence_class: deployed staging trace + canonical Texture artifact inspection
    source: exact-SHA ce6b6455 staging window beginning 2026-07-10T22:47:34Z
    command_or_observation: >-
      Inspect eligible Texture revisions and vmctl guest logs, then query the
      authenticated /api/universal-wire/stories diagnostics.
    artifact_path: internal/runtime/wire_publication.go
    result: >-
      Processor 1c8dc4a9-4484-4c59-9f9f-7397f78d3685 completed with
      all_source_items_decided_with_story_route for cycle_1bf612bf298d883333169770.
      Texture produced canonical cited revisions for documents c783a45a and
      456f920a; revision 14024b34 carries a corpusd_publication_ref and exact
      ingestion lineage. Later eligible articles failed at 23:06:45 and 23:07:43
      with `load wire edition document: record not found`. Stories returned 200
      with texture_edition state=missing, candidate_count=0, and no stories.
      The bootstrap code creates an edition only when GetDocumentAlias returns
      ErrNotFound; it returns immediately when the alias resolves but GetDocument
      reports ErrNotFound.
    uncertainty: >-
      The origin of the dangling alias predates this activation window and is not
      required to repair the live invariant. The canonical publication writer can
      safely replace the alias with a newly bootstrapped edition document while
      preserving all story documents and revisions.
    promotion_relevance: >-
      Reopens edition visibility. The existing missing-alias bootstrap is the
      intended substrate repair and must also cover a dangling alias target before
      the reconciler debounce and canonical feed can operate.
  - claim: The retained processor slot is a symptom of the production MemoryStore
      selection, while the durable sourcecycled Storage replacement remains unwired.
    definition_node: single_authoritative_activation
    evidence_class: deployed staging trace + code/history inspection
    source: exact-SHA 2ebbb682 staging window beginning 2026-07-11T01:11:39Z
    command_or_observation: >-
      Deploy the read-only /internal/source-service/dispatch-state diagnostic; observe
      sourcecycled restart, its first all-source cycle, platform runtime logs, and the
      production store constructor; inspect commits 3a4afd47 and d5fada6a.
    artifact_path: cmd/sourcecycled/main.go + internal/cycle/{mem_store.go,storage.go}
    result: >-
      Restart erased the entire queue and poll ledger, so the previously retained
      request/run identity disappeared before it could be read. The empty MemoryStore
      immediately re-fetched 4,974 items and attempted one monolithic objectgraph
      projection; it timed out after five minutes while platform-runtime admission
      queries repeatedly reported canceled objectgraph/Dolt reads. A following
      five-item Telegram cycle queued two processors, but its first submission then
      occupied the dispatch call's five-minute timeout behind the same store pressure.
      Production explicitly constructs NewMemoryStore. The already-implemented
      cycle.Storage uses the live Node B Dolt SQL server at 127.0.0.1:13306; that
      server is healthy and the platform database is reachable, but sourcecycled no
      longer wires it after 3a4afd47 replaced a then-failing relational dependency.
    uncertainty: >-
      The exact old retained request cannot survive a sourcecycled diagnostic deploy
      because MemoryStore is process-local. Shared UpdatedAt verdict/runtime clocks
      can still renew submitted capacity and require focused regression coverage, but
      patching that symptom alone preserves queue loss, cursor loss, cold replay, and
      the inability to inspect state across deployment.
    promotion_relevance: >-
      Documents the required connection opportunity before patching MemoryStore.
      Reconnect the existing durable Storage substrate, then reproduce and repair any
      remaining terminal-capacity transition against the production implementation.
  - claim: The remaining staging obstruction is the unfinished legacy event
      backfill monopolizing the platform computer's embedded Dolt store, not
      sourcecycled admission capacity or projection batch size.
    definition_node: single_authoritative_activation
    evidence_class: exact-SHA deployed staging trace + guest lifecycle logs
    source: exact-SHA c508ab94 staging window beginning 2026-07-11T01:36:13Z
    command_or_observation: >-
      Observe sourcecycled dispatch-state and journal together with vmctl guest
      console logs after CI run 29134738859 and deploy job 86497123300.
    artifact_path: internal/store/migration.go + internal/store/store.go
    result: >-
      The durable source ledger survived activation and reported queued_count=2,
      recent_in_flight_count=0, and no reconcilable request. The one-time cold
      poll still fetched 4,924 items because no durable cursor existed before this
      release. Its first bounded 100-item projection timed out after five minutes,
      and a later three-item projection plus processor admission encountered the
      same store stall. Every guest trace remained in `objectgraph backfill
      kind=events status=starting`; concurrent processor-count metadata reads were
      canceled approximately every fifteen seconds for the entire observation.
      The event migration performs an existence query and individual object write
      for every legacy SQL event and receives a completion marker only after the
      whole pass, so guest recycling restarts the expensive pass from its beginning.
    uncertainty: >-
      The exact legacy event cardinality and completed-prefix size still need a
      read-only staging measurement. Existing kind-populated gating was replaced by
      resumable per-kind completion because it could collapse a partial migration
      into done; restoring that shortcut would violate the migration contract.
    promotion_relevance: >-
      Blocks product-path acceptance despite the sourcecycled repair. Repair must
      make the event migration incrementally resumable or otherwise bound its store
      occupancy without weakening completeness; raising submitCap cannot help.
  - claim: Replacing per-event existence probes with one unbounded existing-ID scan
      does not restore platform availability because that scan monopolizes the
      runtime's single Dolt connection.
    definition_node: platform_computer
    evidence_class: exact-SHA deployed staging trace + code inspection
    source: exact-SHA d376169f staging window beginning 2026-07-11T03:23:39Z
    command_or_observation: >-
      Deploy d376169f through CI run 29135912106 attempt 2 and Node B deploy job
      86505510999; observe the refreshed platform guest, direct runtime health,
      sourcecycled dispatch-state, and the event migration implementation.
    artifact_path: internal/store/migration.go + internal/store/texture.go
    result: >-
      The refreshed guest reached `runtime: started` and began the event backfill,
      but remained at `kind=events status=starting` for more than three minutes.
      Direct `/health` timed out, vmctl reported the VM unhealthy every fifteen
      seconds, and runtime processor-count queries were repeatedly canceled.
      `ogMetadataValueSet` streams the entire event kind through one query while
      the Texture/ObjectGraph handle is intentionally configured with
      `SetMaxOpenConns(1)`, so the replacement removes quadratic query count but
      still denies the foreground runtime any turn on the store connection.
      Sourcecycled remained stable with NRestarts=0 and its durable queue intact;
      capacity had already returned to one before activation, confirming that
      submitCap is not the blocker.
    uncertainty: >-
      The unbounded scan may eventually finish if the guest survives long enough,
      but it already violates the required availability and full-cycle stability
      semantics. The safe next probe is a keyset-paginated scan that closes its
      rows between bounded pages, allowing foreground work to acquire the single
      connection while preserving complete existing-ID discovery.
    promotion_relevance: >-
      Falsifies the one-scan availability conjecture. The next repair must bound
      connection occupancy, retain completion-aware migration semantics, and keep
      submitCap at one.
  - claim: Closing keyset pages and sleeping one millisecond does not transfer
      the sole Dolt connection to waiting runtime health requests.
    definition_node: platform_computer_stability
    evidence_class: exact-SHA deployed staging trace + direct health sampling
    source: exact-SHA c4320c8a epoch 8170 beginning 2026-07-11T03:48:52Z
    command_or_observation: >-
      Run the full internal/store suite; observe CI run 29138217441 and staging
      deployment; sample http://10.200.177.2:8085/health fifteen times during the
      event migration; correlate vmctl health probes and Firecracker resource use.
    artifact_path: internal/store/migration.go + internal/store/texture.go
    result: >-
      The focused pagination regression and full internal/store suite passed, and
      every standard/race CI lane passed. Epoch 8170 reported the exact c4320c8a
      sandbox artifact, listened at 03:48:52, and entered the event migration.
      Fifteen direct health samples from 03:49:33 through 03:50:46 all timed out
      after three seconds; vmctl independently recorded failures every fifteen
      seconds through 03:51:02. Firecracker remained alive at about 4.15 GiB RSS
      and 225% CPU. Keyset pages therefore preserve scan completeness but the
      close/Gosched/one-millisecond sequence does not provide an enforceable
      foreground scheduling boundary on the one-connection pool.
    uncertainty: >-
      Page duration and database/sql waiter ordering need not be separately guessed:
      voluntary sleep is not an availability contract. The migration must execute
      only a bounded unit per invocation and reschedule continuation after returning,
      or use an explicit store-level priority/lease boundary that foreground health
      can verify. Completion authority must remain durable across those invocations.
    promotion_relevance: >-
      Falsifies c4320c8a as an availability repair. The next implementation must
      make yielding structural rather than scheduler-advisory before another staging
      activation attempt.
```

## Active Red Mutation Ceremony

```yaml
active_red_mutation:
  conjecture_delta: >-
    Completion-aware migration and listener-first startup repair guest readiness.
    Per-VM operation serialization and generation-guarded failure transitions should
    prevent a stale ensure from failing a newer healthy generation. Exact staging
    proof is now gated by a deploy verifier that compares an unchanged sandbox
    artifact to the workflow SHA during a vmctl-only boot-contract refresh.
  protected_surfaces:
    - Embedded Dolt runtime and objectgraph startup/migration.
    - Existing OG objects, including newer OG-only state that legacy SQL must not overwrite.
    - VM guest readiness and platform-computer lifecycle.
  admissible_evidence_class:
    - Focused unit/integration tests for empty-kind migration and populated-kind skip.
    - Full internal/store test suite.
    - CI and race lanes for the exact pushed SHA.
    - Deployed fresh-boot trace through runtime HTTP readiness, followed by a source cycle.
  rollback_path: >-
    Revert the repair commit to e8dda030 behavior; instrumentation remains available
    for diagnosis. If staging regresses, roll back the deployed commit and restart
    vmctl/platform runtime through the normal deployment path.
  heresy_delta:
    discovered:
      - The stated per-kind migration gate exists in graph_store.go but is disconnected.
      - Every boot replays populated legacy kinds, allowing historical run count to block readiness.
      - Concurrent platform ensures share one VM identity without an effective
        generation guard at the failure transition.
      - Active-computer deployment verification collapses workflow identity into
        sandbox artifact identity even when the sandbox artifact was not selected.
      - Fresh source handoff drains can reuse a completed runtime receipt from an
        older cycle while reporting processor_submitted=1 for the new drain.
      - The Universal Wire edition alias exists but targets a missing Texture
        document, and first-publication bootstrap does not repair that state.
      - Publish-debounced reconciler activation retains only document and revision
        handles; it drops the canonical ingestion cycle/request lineage needed to
        prove one processor and one reconciler for the same handoff.
      - The sourcecycled MemoryStore can retain one processor as recently submitted
        after terminal runtime state, holding the sole dispatch slot at zero capacity.
      - The publish-debounced reconciler receives only opaque canonical document and
        revision ids, but its available corpus/source search tools do not resolve those
        handles to Texture content. The deployed reconciler therefore completed without
        reviewing the two documents that triggered it.
      - Per-cycle reconciler deduplication treats a terminal failed receipt as
        authoritative, so a provider failure before iteration zero has no same-run
        retry path and later same-cycle publish batches cannot recover it.
    introduced:
      - a3ebc171 temporarily equates a non-empty OG kind with completed migration;
        SQL remains intact, so the risk is reversible but the completion claim is invalid.
      - 94f6c744 launches deferred migration before synchronous runtime recovery,
        allowing migration to block listener startup on the single Dolt handle.
    repaired:
      - Populated `choir.run` no longer blocks startup with per-record replay.
      - Partial migration is no longer collapsed into completion; successful passes
        receive durable markers and interrupted passes remain resumable.
      - Runtime recovery now completes and publishes :8085 before deferred migration.
      - Same-identity VM lifecycle operations are serialized, different identities
        remain concurrent, and stale process monitors cannot fail a replacement instance.
      - Refreshed-guest deploy verification respects the independently selected
        sandbox artifact identity.
      - Sourcecycled run-status reads traverse the vmctl sandbox proxy, so completed
        handoffs are reconciled instead of reset and resubmitted with stale receipts.
      - A dangling Universal Wire edition alias is replaced through the existing
        canonical bootstrap, allowing eligible Texture articles into Wire.texture.
      - Publish-debounced reconcilers carry exact cycle provenance, use their own
        deterministic request identity, and deduplicate repeated batches per cycle.
```

## Completion Semantics

The mission is `complete` when all of the following are observed on staging:

1. `sourcecycled` starts a cycle and stays running (`systemctl`/`journalctl` show
   no unplanned restart) until the edition is visible.
2. The platform computer is active and healthy for the entire cycle (no
   `vmctl` recovery/reboot in the window).
3. Exactly one `processor` run and one `reconciler` run exist for the cycle's
   `ingestion_handoff_cycle_id`.
4. The processor and reconciler complete without crash or OOM.
5. `/api/universal-wire/stories` returns `200` with stories whose
   `source: universal-wire-edition-texture` and whose `story_texture_doc_id` is
   a canonical Texture doc produced by the reconciler.
6. The diff, if any, is committed and the staging deployment health reports the
   same SHA.

The mission is `blocked_incomplete` if:

- The root cause cannot be identified after reasonable log and trace probes;
- The fix requires a group-level decision (e.g., new service, doctrine change,
  expensive substrate replacement);
- The staging environment is unavailable or unobservable.

The mission is `superseded` if the investigation shows that Autopaper must be a
new service or a different architecture, replacing the current sourcecycled path.

## Rollback And Resumption Policy

- For every code/config change, record the pre-change commit SHA and the exact
  protected surface.
- VM lifecycle changes: rollback to the previous `vmctl` config and restart
  `go-choir-vmctl`.
- Sourcecycled changes: rollback to the previous `sourcecycled` config and
  restart `go-choir-sourcecycled`.
- Runtime changes: rollback the commit and restart the platform computer/runtime.
- Keep the mission log and `run_checkpoint` in this Definition so another agent
  can resume from the last verified state.

## Human Escalation Policy

Escalate to the human before implementing changes that:

- Add a new service, binary, or long-lived scheduler for Autopaper.
- Change the authority boundary between sourcecycled, vmctl, and runtime.
- Modify the `universal-wire-platform` ownership or VM lifecycle doctrine.
- Introduce a substrate replacement (e.g., a new VM manager, a different sandbox
  model) rather than a targeted fix.
- Require a trade-off between retry reliability and VM boot time that cannot be
  decided by observational evidence.
- Leave a root cause unresolved after three probes.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: grounded reconciler provider-circuit failure 2026-07-11T05:18Z-05:48Z
  current_artifact_state: >-
    949342e2 is deployed as the exact sandbox/gateway artifact and resumes legacy-event
    projection one legacy event per bounded invocation. The platform remained
    on one guest while sourcecycled completed multiple cycles and processor, Texture,
    and reconciler runs. Sourcecycled released terminal processor capacity with
    submitCap still fixed at one. Cycle cycle_231bc41ce13fe398f9cbe51b produced
    processor 8a906447, canonical CDC Texture documents aad3f0d2 and fe6518d2, and one
    cycle-correlated reconciler 7aba21d6. The authenticated stories route returns both
    CDC stories from edition 3b9cdc8b. The reconciler completed, but could not resolve
    its opaque doc/revision handles through corpus/source search and made no review edit.
    Commit 60d9b29a adds authoritative canonical title/revision/content context to the
    reconciler handoff and passed local runtime shards, focused race coverage, and all
    exact-SHA CI gates. Its Node B deploy partially activated the ordinary guest at the
    exact SHA, but failed acceptance because universal-wire-platform remained registered
    active at an unreachable sandbox URL (10.200.143.2:8085, HTTP 000). No staging
    reconciler proof for 60d9b29a is admissible yet.
    Deploy rerun attempt 2 subsequently produced an exact-SHA activation receipt and
    the platform returned healthy. Fresh cycle cycle_e7e5c01f012b267c5a33673c then
    completed exactly one processor runtime run, published two canonical Texture
    documents, and dispatched exactly one grounded reconciler. That reconciler failed
    before tool iteration zero because the DeepSeek gateway circuit was open, so its
    editorial completion and canonical revision effect remain unproven.
  what_shipped:
    - 94f6c744 completion-aware resumable OG migration with deferred-open support.
    - cb694846 runtime recovery and listener publication before background migration.
    - 83b1f594 per-VM lifecycle serialization and stale-instance failure guard.
    - 838a4799 artifact-aware refreshed-sandbox deploy verification.
    - ce6b6455 sourcecycled status reconciliation through the vmctl sandbox proxy.
    - 614a3c9a dangling Universal Wire edition-alias recovery.
    - 20644c66 cycle-correlated, per-cycle-deduplicated publish reconciler activation
      with queue/timer/dispatch lifecycle markers.
    - 949342e2 structurally bounded and durably resumable legacy-event projection.
    - 60d9b29a canonical Texture context in publish-reconciler handoffs, activated on
      exact-SHA sandbox/gateway artifacts in CI attempt 2.
  what_was_proven:
    - The loop is platform guest readiness/recovery churn, not a host daemon restart.
    - The guest never reaches cmd/sandbox's post-store runtime-topology log or HTTP listen.
    - Host memory, CPU, IO, and state-disk pressure did not cause the observed loop.
  unproven_or_partial_claims:
    - Reconciler content review over the canonical documents that triggered its run.
    - Event-projection migration completion; it remains resumable and bounded while
      foreground product work proceeds.
    - Restart-injection proof beyond runtime idempotency and deployed repeated drains.
  belief_state_changes:
    - H1 is supported: repeated guest readiness failure drives VM recovery.
    - A sourcecycled process restart and host pressure/OOM are falsified for the
      observed window.
    - H2/H3 describe amplification and overlap but do not explain why a booted
      guest never listens on port 8085.
    - Deployed phase markers localize the readiness failure to `backfillOGFromSQL`;
      all earlier persistent-store startup stages complete within seconds.
    - Per-kind markers settle `choir.run` as the blocking kind and expose the
      disconnected `ogKindIsEmpty` replacement as the intended substrate repair.
    - Fresh-boot verification falsifies non-empty-kind as a universal completion
      marker: channel messages complete, but worker updates can be interrupted mid-kind.
    - Completion-aware migration is correct, but its first launch ordering must move
      after `rt.Start` and listener publication to avoid single-handle contention.
    - Fresh cb694846 proof confirms that ordering, then reveals a stale ensure for an
      older tap IP can kill the newer booted epoch by shared VM ID.
    - 83b1f594 passes all CI and race lanes; its deploy verifier fails because a
      vmctl-only refresh cannot make an unchanged sandbox artifact report that SHA.
    - 838a4799 passes all CI/race lanes and deploys successfully; receipt and every
      host health endpoint report the exact SHA.
    - Concurrent platform resolves return the same active epoch 8102, which remains
      healthy beyond the former three-minute stale-waiter horizon with no recovery.
    - Fresh source cycles complete without sourcecycled restart, but later queue
      drains can return ecd89fa9 from cycle_59a77 for unrelated newer requests.
    - ce6b6455 repairs status reconciliation: run 1c8dc4a9 completed and the next
      drain created distinct run e14f6e73 instead of reusing its receipt.
    - Canonical Texture story revisions now exist with publication refs and source
      lineage, but edition publication fails because the edition alias target is missing.
    - 614a3c9a passed all local runtime shards and its focused race test, then deployed
      as exact sandbox/gateway artifact SHA in CI run 29129997504, deploy job
      86484654190. The post-deploy Texture run 6f783283 completed without crash/OOM;
      `/api/universal-wire/stories` returned 200 from
      `universal-wire-edition-texture` with canonical doc d608c407 and edition doc
      3b9cdc8b, proving the dangling alias repair.
    - The visible story retains cycle_91067dc98d316d6bb3190b71 and request
      processor_782a8418545784fe2207ccf2 in its canonical Texture metadata.
      `dispatchStoryCorpusReconcilerFromPublishBatch`, however, accepts only doc and
      revision handles and writes no ingestion_handoff_cycle_id or request id.
    - 20644c66 passed 351 local runtime tests and focused race coverage, then every
      exact-SHA standard/race lane in CI run 29131530054. Deploy job 86488846194
      activated sandbox/gateway commit 20644c66 at 00:10:27Z; sourcecycled and vmctl
      remained active with NRestarts=0.
    - Fresh RSS and Telegram cycles completed after deployment, but every dispatch
      reported in-flight=1 and submitCap=0. Telegram cycle
      cycle_1881203e0b9e6dadf812e494 queued three processors while the ledger reported
      28 queued/skipped requests and submitted none.
    - 949342e2 passed all required standard/race CI gates in run 29138998386 and
      activated on Node B at 04:18:34Z. The platform runtime listened at 04:19:11Z;
      bounded event batches continued without guest recovery.
    - Processor 935efe9c completed while migration was active. Sourcecycled reconciled
      its explicit terminal no-story resolution, logged in-flight=0/submitCap=1 at
      04:27:36Z, and admitted the next processor without raising the cap.
    - Processor 8a906447 for cycle_231bc41ce13fe398f9cbe51b opened the CDC story,
      Texture run 2e7feb86 produced canonical revision 857097b3, and a second canonical
      CDC document/revision fe6518d2/21195704 joined the same publish batch.
    - The five-minute debounce fired at 04:42:06Z with mixed_lineage=false and dispatched
      exactly one reconciler run 7aba21d6 for cycle_231bc41ce13fe398f9cbe51b. It completed
      at 04:45:04Z without crash or OOM.
    - Authenticated GET /api/universal-wire/stories returned 200 from
      universal-wire-edition-texture. Edition doc 3b9cdc8b revision 35427eea includes
      both CDC canonical docs and reports updated_at 04:40:24Z.
    - Reconciler 7aba21d6 searched the opaque document/revision handles as corpus/source
      queries, found no matches, and requested titles/corpus-visible ids instead of
      assessing consensus, contradiction, or drift. The activation and edition path is
      proven; the reconciler input contract is not yet sufficient for useful review.
    - 60d9b29a passed all standard and race lanes in CI run 29140336567. Deploy job
      86512939750 installed the exact SHA on ordinary guest vm-5b0c1bef (health 200),
      then failed because vmctl still reported universal-wire-platform active at
      10.200.143.2:8085 while direct health returned HTTP 000. The deploy recorded
      incomplete evidence at deploy-failures/29140336567-1.json; exact-SHA platform
      activation and product acceptance are therefore unproven.
    - CI run 29140336567 attempt 2 and deploy job 86514067793 completed successfully;
      deploy-receipt.json records sandbox and gateway active on exact SHA 60d9b29a at
      05:18:59Z. The reattached platform runtime became healthy on the same SHA without
      vmctl recovery or service restart.
    - Cycle cycle_e7e5c01f012b267c5a33673c has one actual processor runtime run,
      3e871ac5; its other two processor requests were superseded before submission.
      The processor completed and opened canonical documents 9a50ce65 and 7302b267.
      Their revisions 4bc409ce and cfb7d31b entered one lineage-pure debounce batch,
      which fired with docs=2 and dispatched one reconciler e289af46.
    - Reconciler e289af46 failed at iteration zero with `provider deepseek: circuit
      open (upstream unhealthy)`. It did not crash or OOM, but it did not complete or
      produce a canonical editorial revision. Existing per-cycle deduplication treats
      any prior reconciler run as authoritative, including a terminal failed run, so
      the same cycle cannot obtain a second run without violating the one-run contract.
    - The same provider-pressure window blocked processor b3b8bfbe with an HTTP 429.
      Runtime deliberately models `blocked` as non-terminal, but sourcecycled has no
      generic continuation or rewarm path for a blocked processor. Its durable request
      remains status=runtime_status=submitted, occupying the one allowed processor
      admission slot while later drains report submitted=0. This is a sourcecycled/runtime
      lifecycle contract gap, not merely provider availability.
    - f1ceba58 repairs that projection locally, but CI deploy attempts 1 and 2 both
      failed their active-platform identity probe. Each attempt refreshed the platform
      guest, waited exactly 60 one-second probes, and recorded HTTP 000; the same URL
      returned HTTP 200 with exact SHA f1ceba58 shortly afterward. Because every rerun
      refreshes the guest again, rerunning cannot converge while the verifier deadline
      remains shorter than observed store-backed guest startup.
    - The refresh also passivated processor run 73dacea4 for request
      processor_00ccb60732afc992c47e25b8. At 06:32Z the runtime reported state=passivated,
      passivated_reason=runtime_restarted, while its trajectory remained live with 51
      open work items and processor resolution awaiting 50 source-item decisions.
      Sourcecycled still projected the request as submitted/submitted and reported
      in-flight=1, submitCap=0 through later drains. This is distinct from the repaired
      blocked-state projection: a refreshed live trajectory has neither resumed nor
      reached a terminal state that releases admission.
    - Forced CI run 29143023440 passed every standard and race gate. Deploy job
      86520721334 completed in 5m05s, and deploy-receipt.json records exact target
      5035bfa2 with active host, frontend, and active-computer artifacts at 06:54:00Z.
      The platform computer was healthy on its refreshed address 10.200.146.2 with
      exact sandbox SHA 5035bfa2; sourcecycled, vmctl, and gateway were active with
      NRestarts=0.
    - The first exact-SHA all-source cycle fetched 3,624 items and reported
      in-flight=0, submitCap=1, but every bounded submission attempt returned runtime
      429 `too many active processor runs`. Runtime run 671d7610 was state=completed
      while its trajectory remained live with two open work items and only 19 of 20
      source items resolved. Sourcecycled had projected its durable request runtime
      status to completed, while sandbox health still reported one running processor.
    - After capacity released, cycle cycle_865b8c07e12f746f4581139b admitted processor
      run 0f6db0fe. It opened canonical Texture docs 9d824cd2 and 3f70e054; their
      processor-owned Texture runs 4926ffc6 and 7adf7d23 produced canonical revisions
      473bce95 and 43088743. Authenticated `/api/universal-wire/stories` returned 200
      from `universal-wire-edition-texture`, edition revision 0ad9f2d9, with both docs.
    - Publish-debounced reconciler activity ran through the grounded 20-tool prompt and
      completed without crash/OOM, but no reconciler-owned Texture revision was created.
      The platform revision ledger has no revision after 07:14Z; both visible docs retain
      `input_origin=processor_handoff`, their processor cycle/request provenance, and
      processor-owned Texture loop ids. Completion item 5 therefore remains false: the
      visible canonical docs were produced by processor-spawned Texture, not reconciler-
      spawned Texture.
  root_cause_clustering_assessment:
    trigger: >-
      Three sourcecycled/runtime lifecycle symptoms were observed in one mission and
      one subsystem, so symptom patching stops pending a substrate-level assessment.
    symptoms:
      - >-
        Substrate: runtime state=blocked is deliberately non-terminal in the generic
        runtime, but sourcecycled has no continuation authority and previously retained
        the request as submitted indefinitely.
      - >-
        Substrate: runtime refresh passivates a live processor trajectory without a
        sourcecycled resume or terminalization contract, leaving 51 work items open.
      - >-
        Substrate: run state=completed can coexist with a live, unresolved processor
        trajectory; sourcecycled releases its ledger capacity while runtime admission
        still counts an active processor and rejects the next submission.
    common_cause: >-
      Processor completion and admission authority are split across run state,
      trajectory state, processor-resolution state, sourcecycled request state, and
      runtime active-run accounting. No single shared projection defines when a
      source-network processor owns capacity, can resume, or is irrecoverably terminal.
    replacement_or_alternative_code: >-
      Existing processor-resolution and trajectory projections already expose the
      semantic facts needed for a shared decision, and sourcecycled's bounded runtime
      polling is wired. The missing connection is to runtime admission accounting and
      restart/passivation recovery; adding more sourcecycled-only terminal cases would
      preserve the split authority rather than repair it.
    substrate_vs_symptom: >-
      All three observed failures are substrate-level lifecycle/authority mismatches,
      not independent ingestion symptoms. The next behavior change must establish one
      processor capacity/completion contract across runtime admission and sourcecycled,
      or explicitly connect an existing shared projection if one is present.
    deletion_first_assessment: >-
      Prefer deleting duplicate sourcecycled capacity inference in favor of runtime's
      semantic processor projection, or making runtime admission consume that same
      projection. Do not add another isolated state-name exception before tracing the
      runtime admission counter and recovery path.
  remaining_error_field:
    - The canonical context handoff is repaired and proven in the exact deployed prompt,
      but the first grounded run failed before iteration zero on provider availability.
    - The bounded event migration has not yet emitted its durable completion marker;
      intermittent foreground health latency remains while batches continue.
    - Exact-SHA platform reattachment still exposes a short startup interval in which
      vmctl reports the VM active before sandbox health is ready; attempt 2 completed
      its receipt and the runtime has remained healthy afterward.
    - The first grounded reconciler encountered an upstream provider circuit before its
      first tool iteration. The dedupe contract has no same-run recovery semantics for
      this terminal failure, so a fresh lineage is currently required for another attempt.
    - A blocked processor cannot currently release sourcecycled admission capacity or
      be resumed by sourcecycled, so a transient provider 429 can freeze all later cycles.
    - The deploy verifier's 60-second sandbox identity window is shorter than the
      repeatedly observed platform guest startup, preventing an admissible f1ceba58 receipt.
    - Runtime refresh can leave a processor passivated while its trajectory remains live;
      sourcecycled counts that request as in flight indefinitely and has no demonstrated
      resume or bounded terminal projection for this state.
    - Completed run state can coexist with a live unresolved processor trajectory, so
      sourcecycled and runtime disagree about capacity and fresh submissions receive 429.
    - The grounded reconciler prompt makes canonical revision conditional on its own
      `when warranted` judgment, so a successful review can end narratively without
      producing the reconciler-authored canonical Texture required by completion item 5.
  highest_impact_remaining_uncertainty: reconciler canonical Texture execution contract
  next_executable_probe: >-
    Strengthen the publish-batch reconciler prompt so the run must select at least one
    listed canonical document and spawn exactly one existing-doc Texture revision grounded
    in the supplied title, revision id, and content. Preserve per-cycle dedupe and lineage,
    deploy the change, and prove the resulting reconciler-owned canonical revision through
    platform metadata and authenticated edition visibility. Keep the processor lifecycle
    authority cluster open as residual substrate work rather than patching another state.
  suggested_goal_string: /goal docs/definitions/choir-autopaper-activation-2026-07-10.md
  evidence_artifact_refs:
    - Evidence Ledger entry for the 2026-07-10T18:30Z-19:31Z Node B observation.
    - CI run 29118850649 and Node B deploy job 86450906080 for c6b422bb.
    - CI run 29120219036 and Node B deploy job 86455558809 for e8dda030.
    - CI run 29125293553 and failed Node B deploy job 86471236444 for 83b1f594.
    - CI run 29126218631, deploy job 86474002411, and activation receipt for 838a4799.
    - CI run 29128036529, deploy job 86479161920, activation receipt for ce6b6455,
      processor 1c8dc4a9, and Texture revisions 14024b34/ec37a6ff.
    - CI run 29129997504, deploy job 86484654190, activation receipt for 614a3c9a,
      Texture run 6f783283, edition doc 3b9cdc8b, and story doc d608c407.
    - CI run 29131530054, deploy job 86488846194, and activation receipt for 20644c66.
    - CI run 29138998386 and activation receipt for 949342e2; processor runs 935efe9c
      and 8a906447; Texture run 2e7feb86; reconciler run 7aba21d6; edition doc 3b9cdc8b.
    - CI run 29140336567, failed deploy job 86512939750, and incomplete deploy evidence
      deploy-failures/29140336567-1.json for 60d9b29a.
    - CI run 29140336567 attempt 2, successful deploy job 86514067793, exact-SHA
      deploy receipt, processor 3e871ac5, Texture runs 3078e94c/e92df686, documents
      9a50ce65/7302b267, and failed grounded reconciler e289af46.
    - CI run 29142172894 deploy attempts 1 and 2, failed jobs 86517633591 and
      86518660425, incomplete deploy evidence deploy-failures/29142172894-{1,2}.json,
      and subsequent direct exact-SHA f1ceba58 platform health.
    - Processor run 73dacea4, request processor_00ccb60732afc992c47e25b8, and the
      06:32Z runtime status showing passivated/live with 51 open work items.
    - CI run 29143023440, deploy job 86520721334, exact-SHA 5035bfa2 deploy receipt,
      refreshed platform health at 10.200.146.2, runtime run 671d7610, and the 06:57Z
      bounded submission attempts rejected by runtime active-processor admission.
    - Cycle cycle_865b8c07e12f746f4581139b; processor run 0f6db0fe; documents
      9d824cd2/3f70e054; processor Texture runs 4926ffc6/7adf7d23; revisions
      473bce95/43088743; edition revision 0ad9f2d9; and the post-reconciler platform
      revision query showing no reconciler-owned canonical write after 07:14Z.
  rollback_refs: []
```

## Suggested Goal String

```text
/goal docs/definitions/choir-autopaper-activation-2026-07-10.md
```
