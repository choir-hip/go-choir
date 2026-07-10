# Choir Contradiction and Seam Repair

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-seam-repair-2026-07-10.md
```

Read this document as executable semantic authority for repairing the
code/docs contradictions and incomplete consolidations left by the
guardrail-shaped run ending at `caf16a88`. Execute the phases in order, stopping
at any blocking `agentic-consensus` finding, test failure, or human escalation
until the resolved state matches the invariants and completion semantics below.

## Source Authority Order

1. This Definition document.
2. `AGENTS.md` and `docs/choir-doctrine.md`.
3. `docs/definitions/choir-product-completion-2026-07-10.md` (the product completion
   Definition whose stale statuses are being repaired).
4. `docs/agent-product-doctrine.md` (authority boundaries, harness minimalism,
   Texture control plane, runtime configuration, product-path verification).
5. `docs/computer-ontology.md` (before changing VM, sandbox, candidate-world,
   promotion, package, or persistent-state behavior).
6. Observed repository state at `caf16a88` and the current test/staging evidence.

## Mutation Class

The mission is **red** overall because it touches deployment identity,
promotion/routing, auth session boundaries, and persistent Base state. The
individual phases are classified below:

- **Phase A** (`buildinfo` service scoping): **red** — changes what `/health`
  reports and which commit a service claims to be.
- **Phase B** (`RouteProfile` format): **red** — changes the route-slot promotion
  path and proxy routing identity.
- **Phase C** (source-workspace identity fallback): **red** — changes sandbox
  source-lineage identity.
- **Phase D** (dead-code deletion): **black** — removes tracked files; requires
  explicit human approval per batch and a pre-delete rollback commit.
- **Phase E** (doc state refresh): **green** — updates statuses and variant
  counts in the product completion Definition.

## Real Artifact / Object Of Work

A single, coherent authority graph for five product paths:

1. **Deployment identity** — one exact, per-service, receipt-verified commit
   reported by each service `/health`.
2. **Promotion route** — one route-slot writer that produces `RouteProfile`
   values the proxy resolver can parse and route through.
3. **Source-workspace identity** — one compiled commit identity for sandbox
   source lineage, with no mutable env fallback.
4. **Dead-code removal** — no unwired `route:`, SyncEngine, or contract-builder
   paths remain as product authority.
5. **Definition state** — the product completion Definition accurately reflects
   which PC nodes are settled, testing, or open.

The object is not a rewrite of Choir Doctrine or a new feature. It is a
consolidation of the half-finished seams from the previous run.

## Mission Purpose And Non-Purpose

**Purpose:**

- Close the two verified contradictions: unscoped `buildinfo.DeployMetadata`
  and the `RouteProfile` format mismatch.
- Remove the remaining mutable env fallback for sandbox source lineage.
- Delete the dead code left behind by the previous deletion pass.
- Refresh the product completion Definition so its statuses match the code.
- Use `agentic-consensus` as a gate before every red mutation and as a final
  review before staging proof.

**Non-purpose:**

- Not a new feature or a new product surface.
- Not a revert of the correct deletions (graph-side wire activation, raw-token
  desktop exchange, Wails SyncService registration).
- Not a migration that drops the `access_token`/`refresh_token` columns in
  `internal/auth/store.go`; the empty-column contract is an acceptable
  intermediate state.
- Not a wiring of Base handlers, Wails SyncService, or File Provider before the
  PC-5 acceptance matrix passes.
- Not a new deployment pipeline or a new receipt schema.

## Definition Graph

### S1. `deployment_identity_service_scoped`

```yaml
id: deployment_identity_service_scoped
kind: boundary
status: open
source: observed
term: Deployment identity is per-service
definition: >-
  A service's /health response may claim a deployed commit only when that
  service's own artifact is present in the activation receipt and its status is
  "active" or "installed". No service may claim deployment because another
  service's artifact is active.
non_definition:
  - A global receipt that proves the whole system is at a target commit.
  - A service reporting a target commit because the receipt exists for other
    artifacts.
  - Any env variable like CHOIR_DEPLOYED_COMMIT or RUNTIME_WORKER_REPO_BASE_SHA
    being treated as deployment identity.
observables:
  - internal/buildinfo/buildinfo.go deployMetadata accepts a service argument.
  - Snapshot(service) passes service to deployMetadata.
  - deployMetadata checks receipt.Artifacts[service].Commit and Status.
  - A new test proves a service with an active frontend artifact does not report
    deployed if its own artifact is missing or not active.
  - Existing tests in internal/buildinfo still pass.
execution_effect:
  - The false-deployment-identity window is closed.
  - PC-0 may advance to "settled" only after staging acceptance proves the
    behavior end-to-end.
settlement:
  rule: Code change plus test coverage plus a green CI run.
  settled_by: formal-check
  invalidation_triggers:
    - A service still reports a commit when its artifact is not in the receipt.
    - A test passes because it only checks the happy path with the same service.
```

### S2. `route_profile_owner_computer_format`

```yaml
id: route_profile_owner_computer_format
kind: boundary
status: open
source: observed
term: RouteProfile is owner_id/computer_id
definition: >-
  The route_profile field in a ComputerSourceLineageRecord is always
  "owner_id/computer_id". It is never "route:" + computer_id, never empty,
  and never contains a space. The proxy resolver splits on exactly one "/" and
  uses the resulting owner and computer IDs.
non_definition:
  - A route_profile that is a free-form string with a "route:" prefix.
  - A route_profile that carries the platform constants as a fallback.
  - A resolver that silently falls back to hard-coded owner/desktop IDs.
observables:
  - internal/proxy/lineage_route_resolver.go splitRouteProfile is unchanged and
    continues to require exactly two non-empty slash-separated segments.
  - internal/runtime/app_promotion.go EnsureComputerSourceLineage writes
    ownerID + "/" + computerID.
  - internal/runtime/app_promotion.go CreateAppAdoption and
    internal/runtime/candidate_package_intake.go set RouteProfile correctly.
  - PromoteAppAdoption and RollbackAppAdoption preserve or correct the format.
  - A test proves that a malformed legacy "route:" value is repaired or rejected
    on promotion.
  - go test ./internal/proxy and focused runtime tests pass.
execution_effect:
  - The route-over-ComputerVersion seam actually routes through the promoted
    lineage instead of falling back to the hard-coded platform constants.
  - PC-4 may advance from "dependency on OG Phase D" to a tested, active route
    writer.
settlement:
  rule: Code change plus a test that exercises the malformed-legacy-RouteProfile
    path plus green CI.
  settled_by: formal-check
  invalidation_triggers:
    - The resolver still returns error and falls back for the new format.
    - A real promotion still writes "route:" + computer_id.
```

### S3. `source_workspace_compiled_identity_only`

```yaml
id: source_workspace_compiled_identity_only
kind: boundary
status: open
source: observed
term: Sandbox source lineage uses only compiled commit
definition: >-
  The source workspace projection uses the commit compiled into the running
  sandbox binary as its platform_base_commit. It does not fall back to
  CHOIR_DEPLOYED_COMMIT, CHOIR_BUILD_SHA, or RUNTIME_WORKER_REPO_BASE_SHA.
  vmctl no longer writes these env vars into choir-runtime.env for identity
  purposes.
non_definition:
  - An env variable that happens to be the same commit as a fallback.
  - A local dev build that uses "unknown" because the compiled commit is "local".
  - A mutable env string being treated as equivalent to a compiled linker stamp.
observables:
  - internal/sandbox/source_workspace.go baseCommit uses
    compiledSourceWorkspaceCommit() only and drops the env fallback.
  - internal/vmctl/handlers.go no longer emits CHOIR_DEPLOYED_COMMIT or
    RUNTIME_WORKER_REPO_BASE_SHA in choir-runtime.env.
  - A test or staging proof confirms that a sandbox reports its compiled commit
    in source lineage even when the env vars are unset.
  - go test ./internal/sandbox passes.
execution_effect:
  - The sandbox has one identity source for its source lineage.
  - The env fallback path is removed.
settlement:
  rule: Code change plus test/staging proof that the sandbox still reports the
    correct commit.
  settled_by: evidence
  invalidation_triggers:
    - The sandbox still reads an env var for source-lineage commit.
    - A production build reports "unknown" because the fallback was removed.
```

### S4. `dead_code_excision`

```yaml
id: dead_code_excision
kind: object
status: open
source: observed
term: Dead code from the deletion pass is removed
definition: >-
  The following surfaces are removed because they are no longer wired, have no
  non-test callers, or are explicitly marked for deletion by the PC-5
  deletion/authority map:

  - internal/runtime/universal_wire.go lines 527-531 (wireRevisionIsUniversalWireSynthesis).
  - internal/computerversion/base_*_contract.go (38 files) and their paired
    base_*_contract_test.go (38 files).
  - cmd/desktop/syncservice.go (the unregistered Wails SyncService).
  - The internal/desktop SyncEngine subtree used only by the old sync path:
    sync.go, sync_test.go, localtree.go, localtree_test.go, conflicts.go,
    conflicts_test.go, syncstatus.go, syncstatus_test.go, and fileprovider/.

  Before deleting internal/desktop files, audit cmd/baseharness, cmd/baseobserve,
  and cmd/evidenceroot. If they depend on the nonconformant SyncEngine path,
  either migrate them to the Base substrate primitives or delete them with the
  same approval.
non_definition:
  - Deleting the concrete Base substrate files in internal/computerversion
    (base_event.go, base_journal.go, base_tree.go, base_blob.go,
    base_current_state*.go, state_generator.go, tree_to_fs.go).
  - Deleting cmd/baseharness or cmd/evidenceroot without deciding whether to
    migrate them to internal/base primitives.
  - Restoring the old SyncEngine or registering it in Wails.
observables:
  - grep for wireRevisionIsUniversalWireSynthesis returns no definitions.
  - find internal/computerversion -name 'base_*_contract*.go' returns nothing.
  - find internal/desktop -name 'sync*.go' returns nothing; fileprovider/ is gone.
  - cmd/desktop/main.go still contains no newSyncService registration.
  - go build ./... passes.
  - No test in the suite treats the deleted code as authority.
execution_effect:
  - The repository no longer compiles the PC-5-deletion-map targets.
  - Future agents cannot re-wire the old SyncEngine or contract builders.
settlement:
  rule: Files deleted, build green, tests green, owner approval recorded.
  settled_by: human
  invalidation_triggers:
    - A build or test failure from a missed import.
    - A remaining non-test caller of the deleted surfaces.
```

### S5. `definition_doc_state_refresh`

```yaml
id: definition_doc_state_refresh
kind: object
status: testing
source: observed; Phase E doc refresh 2026-07-10
term: Product completion Definition matches the code
definition: >-
  docs/definitions/choir-product-completion-2026-07-10.md is updated so that
  every PC node status, variant count, and contradiction claim matches the
  post-repair state. In particular:

  - PC-0 stays at testing until service-scoped deployment identity is proven.
  - PC-4 is no longer framed as only an OG dependency; it is a route-slot
    writer with an active format bug that this mission fixes.
  - PC-6 autopaper_authoritative_activation_paths is 1, not 2.
  - PC-7 no longer claims SyncService is registered.
  - PC-5 deletion/authority map reflects the deleted contract files.
  - The variant section counts match the resolved state.
non_definition:
  - Rewriting doctrine or authority order.
  - Declaring a node settled before the code evidence exists.
  - Adding new PC nodes beyond the current scope.
observables:
  - A diff of docs/definitions/choir-product-completion-2026-07-10.md that
    updates only statuses, counts, and evidence references.
  - The doc truth checker passes.
execution_effect:
  - Future /goal runs read the correct belief state and do not re-discover
    already-resolved contradictions.
settlement:
  rule: Doc changes pass the docs truth checker and contain no unsupported
    "settled" claims.
  settled_by: formal-check
  invalidation_triggers:
    - A doc claims a PC node is settled while the code still contradicts it.
    - The variant counts do not match the resolved code.
```

### A1. `agentic_consensus_gate`

```yaml
id: agentic_consensus_gate
kind: operator
status: proposed
source: user-stated
term: Agentic consensus before red mutation
definition: >-
  Before any red mutation, run the agentic-consensus runner against the planned
  diff and the active contradiction. The panel reviews the proposed changes for
  hidden code paths, contradictions, and whether the change realizes the real
  purpose. The orchestrator adjudicates the findings, updates the graph, and
  may proceed, modify, or escalate. A severe finding from any panel member
  blocks the mutation until the orchestrator verifies or the human rules.

  The runner is invoked as:

  ```
  skill://agentic-consensus/agentic-consensus-runner.sh \
    --prompt-file /tmp/choir-seam-repair-<phase>-prompt.md \
    --cwd /Users/wiz/go-choir \
    --out-dir /tmp/choir-seam-repair-<phase>-consensus \
    --keep-going
  ```

  If the `codex` agent cannot be invoked because the runner still passes the
  unsupported `--ask-for-approval` flag, either fix the runner first or run with
  `--exclude codex` and record the missing member. The next mission is to fix
  the runner; this mission should not be blocked on that fix.
non_definition:
  - A panel majority vote over a locally demonstrated fact.
  - A consensus pass that replaces targeted tests or staging evidence.
  - A panel that does not inspect the actual files or diff.
observables:
  - manifest.tsv shows the panel ran and each agent status.
  - The prompt and outputs are preserved under /tmp/choir-seam-repair-*-consensus.
  - The orchestrator records which findings were accepted, rejected, or
    escalated.
execution_effect:
  - No red mutation proceeds without a completed consensus pass and an
    adjudicated record in the evidence ledger.
settlement:
  rule: The panel has completed; the orchestrator has reviewed findings; the
    planned mutation is approved, modified, or escalated with a recorded reason.
  settled_by: orchestrator
  invalidation_triggers:
    - The planned mutation changes materially after the consensus run.
    - New evidence (e.g., staging failure) contradicts a panel assumption.
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: The previous run's deletions (graph-side wire activation, raw
        desktop token exchange, Wails SyncService registration) are correct and
        must not be reverted.
      source: observed
      execution_effect: The mission builds on them, not against them.
    - claim: buildinfo.Snapshot is already called with a service name in every
        production use.
      source: observed
      execution_effect: deployMetadata can be scoped without changing callers.
    - claim: The proxy resolver expects "owner_id/computer_id" and rejects
        "route:computer_id".
      source: observed
      execution_effect: RouteProfile writes must match that format.
    - claim: internal/computerversion/base_*_contract.go files have no
        non-test callers outside the closed definition cluster.
      source: observed
      execution_effect: They are safe to delete.
    - claim: cmd/desktop/syncservice.go is no longer registered in main.go.
      source: observed
      execution_effect: It is safe to delete or hard-gate.
  contested:
    - node: internal/desktop scope
      issue: cmd/baseharness, cmd/baseobserve, and cmd/evidenceroot may import
        the internal/desktop SyncEngine path. A decision is needed before
        deleting those files.
      next_resolution_step: Audit the three cmd tools and either migrate them to
        internal/base primitives or add them to the deletion batch.
  open:
    - node: deployment_identity_service_scoped
      missing: code change to internal/buildinfo/buildinfo.go and a new test.
    - node: route_profile_owner_computer_format
      missing: code changes in internal/runtime and a malformed-legacy test.
    - node: source_workspace_compiled_identity_only
      missing: code changes in internal/sandbox/source_workspace.go and
        internal/vmctl/handlers.go plus a test/staging proof.
    - node: dead_code_excision
      missing: audit of cmd/base* tools and owner approval for the deletion
        batch.
    - node: definition_doc_state_refresh
      missing: doc diff and truth-checker pass.
```

## Invariants

These must remain true across every phase:

1. `buildinfo.Commit` is immutable; `buildinfo.DeployedCommit` is derived only
   from a per-service receipt check.
2. `RouteProfile` is always `owner_id/computer_id` when present; the resolver
   never silently falls back to hard-coded constants for a well-formed profile.
3. Sandbox source lineage uses the compiled binary commit as its identity.
4. No Wails SyncService or `/api/base` handler is wired or registered as product
   authority.
5. The product completion Definition does not declare a node `settled` before
   the code evidence supports it.
6. Every red mutation is preceded by an agentic-consensus gate.

## Authority Boundaries

- `internal/buildinfo` is the sole authority for deployment identity.
- `internal/proxy/lineage_route_resolver` and `internal/runtime` promotion route
  together are the sole authority for platform routing.
- `internal/sandbox` is the sole authority for sandbox source-lineage commit.
- `internal/base` and `internal/computerversion` concrete substrate files are
  the future Base authority; `internal/desktop` SyncEngine is not.
- `docs/definitions/choir-product-completion-2026-07-10.md` is the sole
  authority for the product completion state; this mission only repairs its
  stale fields.

## Conjectures And Belief State

```yaml
conjectures:
  - id: c1_deletions_are_correct
    claim: The previous run's deletions should not be reverted.
    status: settled
    evidence_class: observed code + AGENTS doctrine
    falsifier: A deleted path is still required by a non-test caller.
  - id: c2_two_seams_cause_contradictions
    claim: The unscoped deployMetadata and the RouteProfile format mismatch are
      the two load-bearing contradictions that must be fixed first.
    status: testing
    evidence_class: observed code + agentic-consensus panel
    falsifier: A third contradiction is found that blocks these two.
  - id: c3_dead_code_safe_to_delete
    claim: The listed dead code has no non-test product caller and can be
      removed without breaking the build.
    status: testing
    evidence_class: code search + build/test
    falsifier: A build or runtime failure after deletion.
  - id: c4_env_fallback_can_be_removed
    claim: The sandbox Nix build sets buildinfo.Commit, so the env fallback can
      be removed.
    status: testing
    evidence_class: build evidence + staging proof
    falsifier: A production build reports "unknown" after the fallback is removed.
```

## Variant / Progress Measure

```yaml
variant:
  unscoped_deployment_identity: 1
  route_profile_format_mismatch: 1
  source_workspace_env_fallback: 1
  dead_code_surfaces: 4
  stale_definition_statuses: 4
  agentic_consensus_gates_passed: 0
  staging_acceptance_proof: 0
```

The mission reduces all variant counts to 0 and raises the two proof counts to
1. Any phase that changes files without changing these counts is motion theater.

## Execution Operators

```text
define(node)      # make a missing meaning executable
probe(node)       # test a claim under the current observer
construct(node)   # mutate the artifact under invariants
verify(node)      # check an artifact or claim
agentic_consensus(node)  # run the agentic-consensus runner before a red mutation
settle(node)      # promote/weaken/falsify/supersede/escalate
shift(node)       # change observer, vocabulary, or prover
```

## Execution Phases

### Phase 0 — Consensus on the whole plan (green/yellow)

- Write a prompt that names the five contradictions, the proposed fixes, and
  the invariants. Run the agentic-consensus runner.
- Adjudicate the findings. If a severe finding changes the plan, update the
  definition graph and re-run consensus on the changed plan.
- Produce an evidence ledger entry for the consensus.

### Phase A — Service-scoped deployment identity (red)

- Run agentic-consensus on the specific diff for `internal/buildinfo/buildinfo.go`
  and `internal/buildinfo/buildinfo_test.go`.
- Modify `deployMetadata` to accept `service` and verify only
  `receipt.Artifacts[service]`.
- Add a test that proves an active frontend artifact does not make `proxy`
  report `deployed_commit`.
- Run `go test ./internal/buildinfo`.
- Update the evidence ledger.

### Phase B — RouteProfile format (red)

- Run agentic-consensus on the specific diff for `internal/runtime/app_promotion.go`
  and `internal/runtime/candidate_package_intake.go`.
- Change `EnsureComputerSourceLineage` to write `ownerID + "/" + computerID`.
- Change `CreateAppAdoption` and the candidate-package-intake adoption path to
  set `RouteProfile` to `ownerID + "/" + targetComputerID` and to reject or
  normalize legacy `route:` prefixes.
- Ensure `PromoteAppAdoption` and `RollbackAppAdoption` preserve or correct the
  format.
- Add a test that passes a malformed legacy `route:` value and proves the
  resolver routes correctly after promotion.
- Run `go test ./internal/proxy ./internal/runtime -run 'Route|Lineage|Promotion'`.
- Update the evidence ledger.

### Phase C — Source workspace compiled identity (red)

- Run agentic-consensus on the diff for `internal/sandbox/source_workspace.go`
  and `internal/vmctl/handlers.go`.
- Remove the env fallback in `sourceWorkspaceProjection` baseCommit.
- Remove `CHOIR_DEPLOYED_COMMIT` and `RUNTIME_WORKER_REPO_BASE_SHA` from the
  `choir-runtime.env` emitted by `vmctl` (or keep them only for non-identity
  tooling if a separate need is documented).
- Add or update a test that verifies the sandbox source lineage uses the
  compiled commit.
- Run `go test ./internal/sandbox ./internal/vmctl`.
- Update the evidence ledger.

### Phase D — Dead code excision (black)

- Audit `cmd/baseharness`, `cmd/baseobserve`, and `cmd/evidenceroot` for
  `internal/desktop` SyncEngine imports. Decide: migrate to `internal/base`
  primitives or add to deletion.
- Run agentic-consensus on the deletion batch.
- Obtain explicit human approval for the deletion batch.
- Create a pre-delete rollback commit.
- Delete:
  - `internal/runtime/universal_wire.go` lines 527-531.
  - `internal/computerversion/base_*_contract.go` and
    `internal/computerversion/base_*_contract_test.go`.
  - `cmd/desktop/syncservice.go`.
  - The `internal/desktop` SyncEngine subtree (`sync.go`, `sync_test.go`,
    `localtree.go`, `localtree_test.go`, `conflicts.go`, `conflicts_test.go`,
    `syncstatus.go`, `syncstatus_test.go`, `fileprovider/`).
- Run `go build ./...` and targeted tests. Iterate if imports remain.
- Update the evidence ledger.

### Phase E — Product completion Definition refresh (green)

- Run doccheck / the docs truth checker.
- Update `docs/definitions/choir-product-completion-2026-07-10.md`:
  - PC-0 to `testing` with a pointer to the service-scoped receipt evidence.
  - PC-4 to `testing` and correct the frame from OG-dependency to active route
    writer with a fixed format.
  - PC-6 `autopaper_authoritative_activation_paths` to `1`.
  - PC-7 to remove the claim that `SyncService` is registered.
  - PC-5 deletion map to reflect the deleted files.
  - Variant counts to match the resolved state.
- Run the docs truth checker again.
- Update the evidence ledger.

### Phase F — Final consensus and staging proof (red)

- Run agentic-consensus on the full diff.
- Run the full CI suite (`go test ./...`, `vet`, `frontend build`,
  `deploy-impact-classify`, `deploy-workflow-contract-test`).
- Push and monitor CI/deploy per the AGENTS landing loop.
- Run deployed staging acceptance against `choir.news`:
  - `/health` for each service shows the correct compiled and deployed commits.
  - A promotion/rollback exercises the route-slot and reports the resolved
    `RouteProfile`.
- Record the staging evidence and update the evidence ledger.
- Update the `variant` and `run_checkpoint` sections.

## Evidence Ledger

Initial entries:

```yaml
evidence_ledger:
  - claim: buildinfo.DeployMetadata is unscoped and allows cross-service
      masquerading.
    definition_node: deployment_identity_service_scoped
    evidence_class: observed file
    source: internal/buildinfo/buildinfo.go
    command_or_observation: read lines 57-92; Snapshot passes service but
      deployMetadata ignores it.
    result: verified
    uncertainty: none
  - claim: RouteProfile is written as "route:" + computer_id but the resolver
      expects owner_id/computer_id.
    definition_node: route_profile_owner_computer_format
    evidence_class: observed file
    source: internal/runtime/app_promotion.go:98, :272;
      internal/runtime/candidate_package_intake.go:511;
      internal/proxy/lineage_route_resolver.go:49-97
    result: verified
    uncertainty: none
  - claim: source_workspace has a mutable env fallback for source-lineage commit.
    definition_node: source_workspace_compiled_identity_only
    evidence_class: observed file
    source: internal/sandbox/source_workspace.go:98-103;
      internal/vmctl/handlers.go:1201-1209
    result: verified
    uncertainty: none
  - claim: wireRevisionIsUniversalWireSynthesis has no callers.
    definition_node: dead_code_excision
    evidence_class: observed file
    source: grep wireRevisionIsUniversalWireSynthesis
    result: verified
    uncertainty: none
  - claim: base_*_contract.go files are unused and marked for deletion by PC-5.
    definition_node: dead_code_excision
    evidence_class: observed file
    source: internal/computerversion/base_*_contract.go; PC-5 deletion map
    result: verified
    uncertainty: exact count of non-test callers
  - claim: product completion Definition statuses and variant counts match post-repair code.
    definition_node: definition_doc_state_refresh
    evidence_class: observed file + formal-check
    source: docs/definitions/choir-product-completion-2026-07-10.md;
      docs/doc-authority-manifest.yaml; docs/mission-graph.yaml; docs/ACTIVE.md
    command_or_observation: >-
      PC-0/PC-4 set to testing; autopaper_authoritative_activation_paths=1;
      SyncService registration claims removed; PC-5 deletion map updated;
      og-dolt demoted from authority-root so L4 has one product Definition;
      seam-repair registered as non-root maintenance Definition.
      ./scripts/doccheck --mode=live passed.
    result: verified
    uncertainty: staging proof of service-scoped identity and RouteProfile still open under Phase F
```

## Forbidden Collapses

- receipt exists -> service-scoped identity.
- RouteProfile string changed -> resolver actually uses it.
- dead code deleted -> nothing else used it.
- tests pass -> staging behavior is proven.
- agentic consensus agrees -> no local verification needed.
- docs updated -> code is settled.
- go build green -> no runtime contradiction remains.

## Completion Semantics

The mission is complete when all of the following are true with named evidence:

1. `deployMetadata` is service-scoped and a new test proves cross-service
   leakage is impossible.
2. `RouteProfile` is always `owner_id/computer_id` and the resolver routes
   through the lineage record instead of falling back to hard-coded constants.
3. `source_workspace` uses only the compiled commit and `vmctl` does not inject
   identity env vars.
4. The dead code surfaces are deleted, `go build ./...` passes, and no test
   treats the deleted code as authority.
5. `docs/definitions/choir-product-completion-2026-07-10.md` reflects the
   resolved state and passes the docs truth checker.
6. Agentic-consensus gates were run before every red mutation and the final
   staging acceptance proof is recorded.

Any exit before these are met is `checkpoint_incomplete` or `blocked_incomplete`.

## Rollback And Resumption Policy

- Code phases (A, B, C) are reversible by reverting the commit; the rollback
  refs are recorded in the evidence ledger.
- Phase D (deletion) requires a pre-delete rollback commit. The exact deleted
  paths are listed. Recovery is via Git history or the pre-delete ref.
- Phase E (docs) is reversible by reverting the doc commit.
- If a staging proof fails, the mission is `blocked_incomplete` and the
  rollback ref is the last green commit before the failure.
- If the `agentic-consensus` runner is missing agents (e.g., `codex` is broken),
  record the missing member and continue with `--exclude` or `--include` until
  the runner is fixed in the next mission.

## Human Escalation

Escalate to the owner for:

- The Phase D deletion batch approval.
- Any agentic-consensus finding that the orchestrator cannot adjudicate.
- A contradiction discovered during execution that changes the mission scope.
- Any proposed change to a protected surface (auth, promotion, deployment,
  Base persistent state) outside the scope of this Definition.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: caf16a88 (HEAD at mission start)
  current_artifact_state: >-
    Two verified contradictions in buildinfo and RouteProfile; env fallback in
    source workspace; dead code from previous deletion pass; stale product
    completion Definition.
  what_shipped: []
  what_was_proven:
    - buildinfo.DeployMetadata is unscoped.
    - RouteProfile format mismatch causes resolver fallback.
    - source_workspace uses env fallback.
    - Dead code remains after previous deletions.
  unproven_or_partial_claims:
    - The exact list of cmd/base* tools that depend on internal/desktop.
  highest_impact_remaining_uncertainty: >-
    Whether the internal/desktop SyncEngine deletion requires migrating or
    deleting cmd/baseharness, cmd/baseobserve, and cmd/evidenceroot.
  next_executable_probe: >-
    Phase 0: write the consensus prompt and run agentic-consensus on the full
    repair plan.
  suggested_goal_string: /goal docs/definitions/choir-seam-repair-2026-07-10.md
  evidence_artifact_refs:
    - caf16a88 (mission start)
  rollback_refs:
    - caf16a88
```

## Suggested Goal String

```text
/goal docs/definitions/choir-seam-repair-2026-07-10.md
```
