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
status: settled
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
```

## Active Red Mutation Ceremony

```yaml
active_red_mutation:
  conjecture_delta: >-
    The lifecycle/retry hypotheses are superseded by deployed proof that sandbox
    startup unconditionally replays populated relational `choir.run` rows into OG.
    Wiring the existing per-kind emptiness gate will preserve first migration while
    making subsequent boots independent of historical row count.
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
    introduced:
      - a3ebc171 temporarily equates a non-empty OG kind with completed migration;
        SQL remains intact, so the risk is reversible but the completion claim is invalid.
    repaired:
      - Populated `choir.run` no longer blocks startup with per-record replay.
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
  last_checkpoint: live Node B observation 2026-07-10T18:30Z-19:31Z
  current_artifact_state: >-
    No code changes yet. Staging reproduces the loop: sourcecycled projections
    trigger a platform guest boot, the guest reaches its runtime launcher but
    never binds port 8085, vmctl times out after three minutes and kills/marks
    the guest failed, and the next projection repeats the recovery.
  what_shipped: []
  what_was_proven:
    - The loop is platform guest readiness/recovery churn, not a host daemon restart.
    - The guest never reaches cmd/sandbox's post-store runtime-topology log or HTTP listen.
    - Host memory, CPU, IO, and state-disk pressure did not cause the observed loop.
  unproven_or_partial_claims:
    - The root cause of the platform/sourcecycled reboot loop.
    - Platform VM stability for a full source cycle.
    - Sourcecycled liveness for a full source cycle.
    - Single authoritative activation under retry/restart.
    - End-to-end Autopaper edition visibility.
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
  remaining_error_field:
    - The platform computer is rebooting in a loop because its runtime never becomes ready.
    - Autopaper has not produced a visible edition on staging.
  highest_impact_remaining_uncertainty: resumable migration completion semantics
  next_executable_probe: >-
    Replace non-empty-as-complete with a completion-aware, resumable migration
    protocol that does not block runtime health during large first migrations;
    preserve SQL until completion and prove restart resumes missing objects.
  suggested_goal_string: /goal docs/definitions/choir-autopaper-activation-2026-07-10.md
  evidence_artifact_refs:
    - Evidence Ledger entry for the 2026-07-10T18:30Z-19:31Z Node B observation.
    - CI run 29118850649 and Node B deploy job 86450906080 for c6b422bb.
    - CI run 29120219036 and Node B deploy job 86455558809 for e8dda030.
  rollback_refs: []
```

## Suggested Goal String

```text
/goal docs/definitions/choir-autopaper-activation-2026-07-10.md
```
