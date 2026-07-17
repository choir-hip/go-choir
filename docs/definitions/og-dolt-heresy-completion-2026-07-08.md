# OG / Dolt / Heresy Completion Mission

## Subordinate Invocation Semantics

This document supplied storage, promotion, heresy-detector, and deletion
contracts to the completed
[audited-construction Definition](choir-audited-autoputer-construction-2026-07-15.md).
Neither document is currently executable. Historical conclusions and
detector/deletion contracts remain load-bearing; they do not create an
orchestrator, mutation authority, or state capsule.

This document previously superseded the deleted 2026-07-07 program and heresy
paradocs. Their conclusions remain absorbed here as historical evidence. The
completed audited-construction mission superseded this document's independent execution authority; no current product Definition inherits it.

## Source Authority Order

1. Current doctrine and any future separately promoted Definition for execution.
2. The completed audited-construction Definition and this document's detector,
   deletion, and historical evidence contracts.
3. Owner statements 2026-07-07/08: object graph becomes canonical by hard
   cutover; Dolt version-control features become load-bearing; all named
   heresies eliminated with executable enforcement; candidate computers are
   capsules over substrate-independent audited computers, not VMs; one
   comprehensive mission encompasses incomplete cleanup debt.
4. `docs/computer-ontology.md` (ComputerVersion, materializer,
   route-over-computer-version).
5. `docs/choir-doctrine.md` heresy registry (H001–H031).
6. Historical source in Git history for imported definition graphs, phase
   inventories, deletion inventories, and completion criteria.
7. Pre-purge evidence snapshot at Git commit `8f62fe3b`.
8. Agentic-consensus panel reviews 2026-07-08/09 as adjudicated external
   second-opinion evidence.
9. `AGENTS.md`.

Where this document conflicts with older mission docs or ledgers that label
work "complete" while carrying populated `remaining_error_field` /
`unproven_or_partial_claims` blocks, this document governs: those are
`checkpoint_incomplete`.

## Mutation Class

Authoring and doc-correction passes are **yellow**. Execution is **red**
wherever it touches Texture canonical writes, run acceptance,
promotion/rollback, conductor routing, the store schema, vmctl lifecycle,
proxy request path, or public API routes; red passes use the AGENTS.md
ceremony (conjecture delta, protected surfaces, admissible evidence class,
rollback path, heresy delta) and the full Landing Loop
(push → CI → deploy → staging proof).

## Real Artifact / Object Of Work

The real object is a **system whose invariants are mechanically enforced and
whose progress claims are falsifiable**:

- every named heresy eliminated (code deleted or replaced, tests inverted,
  external contracts migrated) with a CI detector as permanent guard;
- the object graph canonical by hard cutover, SQL dual paths deleted;
- Dolt version-control features load-bearing (history reads, branch-or-tag
  promotion with a settled spec↔implementation relationship);
- routes resolving to ComputerVersion records, never VM identities;
- the mission corpus itself truthful: no ledger labeled complete while
  carrying unproven claims, no seam commit readable as phase completion.

Subordinate projections: detector manifest + CI job; timeout-hardened
request path; per-cluster deletion diffs; inverted tests; corrected docs;
the evidence ledger in this document.

## Historical Purpose Receipt

The retired program sought to finish the OG/Dolt and heresy-eradication work
without false progress. This purpose record supplies no current execution
sequence, mutation permission, or completion authority.

**Non-purpose:**

- Runtime business-logic extraction receipts remain historical evidence under
  `docs/definitions/choir-autoputer-completion-2026-07-14.md`. This Definition
  now supplies subordinate D-ROUTE, detector, and deletion contracts to the
  audited-construction mission; it is not a competing `/goal` spine.
- Not the grip/RL research program; that retired narrative remains in Git
  history and its research forks are out of scope.
- Not new product surface (headless CLI Phase 1.5 verbs, MCP, reader UX
  options B/C stay deferred unless a node here requires them).
- Not detector theater: a detector that cannot fail is not evidence.
- Not motion theater: a pass that changes no node status and no verifier is
  not progress.

## Definition Graph

Imported nodes: `heresy`, `eliminated`, detector semantics, registry-close
semantics from historical source in Git history — status carried as
settled there.

### T1. Term: `seam`

```yaml
id: seam
kind: term
status: settled
source: observed (consensus 2026-07-08, unanimous)
definition: A commit that introduces an interface, adapter, or option for a target architecture without making it load-bearing in any production binary or default configuration.
non_definition:
  - Phase completion.
  - Heresy repair.
  - Evidence that the target architecture works.
examples:
  - e5c1d38a — WithPromotionAdapter defined, never called from cmd/.
  - e393eb5c — LineageBasedRouteResolver active only when PROXY_RUNTIME_DB_PATH is set (default unset); falls back to hard-coded platform VM identity.
observables:
  - grep for the new symbol under cmd/; check default env/config activation path.
execution_effect:
  - Commit-log or ledger language describing a seam must not use "landed", "complete", or "repaired H0xx" without the load-bearing evidence class.
  - Node W5 (labeling correction) applies to all existing seam commits.
forbidden_collapses:
  - seam merged -> phase landed
  - adapter exists -> promotion is Dolt-native
settlement:
  rule: Settled by this definition; reopened if a seam is found being cited as completion evidence.
  settled_by: orchestrator
```

### T2. Term: `checkpoint_incomplete` (corpus-wide)

```yaml
id: checkpoint-incomplete-corpus
kind: term
status: settled
source: observed (ledger sweep 2026-07-08)
definition: Any mission document whose status field claims completion while its own remaining_error_field, unproven_or_partial_claims, or open-edge notes are non-empty is checkpoint_incomplete regardless of its label.
examples:
  - A historical substrate checkpoint with MPCal TLC unverified and embed refactor deferred.
  - A historical cross-substrate checkpoint whose gates 4/5 remained unproven.
execution_effect:
  - Work item C4 must relabel these documents; no downstream mission may cite them as complete.
```

### I1. Invariant: `route-over-computer-version` (H031 bar)

```yaml
id: route-over-computer-version
kind: invariant
status: settled (definition) / violated (implementation)
source: computer-ontology.md; choir-doctrine.md H031
definition: >-
  No product route resolves to a VM or desktop identity at the routing decision
  layer; routes must key off `ComputerVersion = (CodeRef, ArtifactProgramRef)`
  records. Translation from the resolved `ComputerVersion` to a materializer/
  `vmctl` endpoint (SandboxURL) is an implementation seam, not a route target.
observables:
  - `internal/proxy/route_resolver.go` returning hard-coded
    `UniversalWirePlatformOwnerID`/`DesktopID` constants as the route target
    (violation: no ComputerVersion lookup).
  - `LineageBasedRouteResolver` parsing `route_profile` as owner/desktop and
    treating that as the route target instead of first resolving to a
    `ComputerVersion` record (violation at the decision layer; physical
    dispatch from the resolved ComputerVersion is the permitted seam).
counterexamples:
  - Default seeded `route_profile` "route:computer-universal-wire-platform" has no
    slash, fails the parser, and falls back to the hard-coded VM identity.
execution_effect:
  - H031 may not be recorded as repaired until the observables above are gone,
    the resolver reads a receipted `ComputerVersion` from D-ROUTE's vmctl-owned
    ledger before materialization, and the detector for the banned pattern is
    green.
forbidden_collapses:
  - resolver reads route_profile -> route is over ComputerVersion
  - ComputerVersion -> SandboxURL materialization seam treated as a route-over-VM
```

### I2. Invariant: spec claims match implementation reach

```yaml
id: spec-impl-conformance
kind: invariant
status: settled (definition) / violated (implementation)
source: observed (glm52/cursor findings, verified 2026-07-08)
definition: A TLC-green spec invariant may be cited as system evidence only for properties the shipped implementation can provide, established by a conformance check or an explicit scope note in the spec.
observables:
  - specs/promotion_protocol.tla asserts BranchIsolation; internal/computerversion/dolt_promotion_adapter.go is tag-only (DOLT_TAG/DOLT_COMMIT/DOLT_RESET), comments state isolation "must come from a different layer".
execution_effect:
  - BranchIsolation is a property of promotion operations on the VM's EMBEDDED store (D-STORES taxonomy), not the world-wire store. The spec stays branch-based as target-state; its scope header must name the embedded store and note the tag-only adapter is interim. The adapter rewrite direction follows the D-PROMO experiment; W6 adds the conformance binding before "spec implemented" can be claimed.
  - Until the branch adapter + conformance binding land, "promotion protocol model-checked" claims must carry the scope caveat.
  - The spec rewrite must model merge and tag as SEPARATE preparation steps (Dolt docs: DOLT_MERGE implicitly commits the transaction; merge+tag cannot be one SQL transaction). Those preparation steps are idempotent/resumable. Promotion atomicity is exclusively D-ROUTE's route-slot CAS + receipt transaction.
```

### I3. Invariant: bounded request path

```yaml
id: bounded-request-path
kind: invariant
status: settled (definition and implementation)
source: observed staging traces adjudicated into this Definition (pre-fix api.resolve max 180,029ms / 23 errors; post-fix max 60,001ms with a bounded 504)
definition: No public request may hang for the vmctl client default; the proxy fails fast with a 504 within a bounded window.
observables:
  - internal/vmctl/client.go:22 DefaultClientTimeout = 60s.
  - internal/proxy/config.go:83 DefaultVmctlTimeout = 60s.
  - internal/server/server.go:60-61 defaultReadTimeout / defaultWriteTimeout = 120s and http.Server wired with ReadTimeout/WriteTimeout.
  - internal/proxy/handlers.go:46 sandboxResolveRetryWindow = 10s reconciled against the 60s bound.
  - nix/node-b.nix:350 PROXY_VMCTL_TIMEOUT=60s; start-services.sh:126 PROXY_VMCTL_TIMEOUT default 60s.
  - staging /api/universal-wire/stories under induced resolve failure returns 504 within 60s (max_duration_ms 60,001).
execution_effect:
  - Work item W2 is satisfied and is a Phase-A gate; later phase staging proofs are now legible.
```

### I4. Invariant: destructive rollback is forbidden in embedded mode

```yaml
id: no-destructive-embedded-rollback
kind: invariant
status: settled
source: reviewer (gemini35, glm52), confirmed by adapter source
definition: DOLT_RESET --hard against the embedded main branch is not an admissible rollback mechanism while concurrent writers share that branch; rollback must be a route flip or occur on an isolated branch.
execution_effect:
  - The tag-based adapter's rollback path may not be enabled in any production promotion flow before embedded-store branch isolation (D-PROMO) settles.
```

### B1. Boundary: authority of narrative documents

```yaml
id: narrative-authority-boundary
kind: boundary
status: settled
source: reviewer (gpt55), consistent with doc-authority-manifest
definition: Narrative and philosophy documents cannot override doctrine, definitions, specs, or evidence.
execution_effect:
  - Work item C6 records this in the docs index; agents must not cite grip narrative as execution authority.
```

### D-STORES. Historical term: three-domain Dolt taxonomy — SUPERSEDED IN PART

```yaml
id: dolt-store-taxonomy
kind: term
status: superseded_by_active_mission_two_store_topology
source: orchestrator-settled synthesis, unratified; owner two-store directive governs
superseded_claim: A distinct third Dolt domain owns ComputerVersion route control.
superseded_by: docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
term: Dolt store taxonomy
definition: >-
  The live topology has exactly two product-state Dolt stores: (1) the
  WORLD-WIRE STORE — the platform ObjectGraphStore
  (`internal/platform/objectgraph_store.go`, served by corpusd; HTTP access via
  `internal/objectgraph/http_store.go`) serving the world-wire system; and
  (2) VM-LOCAL EMBEDDED STORES — one embedded Dolt per user VM
  (`internal/objectgraph/dolt_store.go`), shared by all capsules in that VM.
  HISTORICAL, NON-EXECUTABLE CLAIM: this node formerly asserted a third,
  distinct durable vmctl-owned Dolt route-control domain.
  PROMOTION REMAINS AN OPERATION, NOT A WORKSPACE: ComputerVersion
  fork/merge/tag preparation executes against the VM's embedded store
  (DoltPromotionAdapter.WorkspacePath is the filesystem path to that
  embedded workspace); served-route activation executes as a CAS in the
  ComputerVersion route ledger. Promotion is NOT a property of the world-wire
  store, and there are no separate per-app promotion workspaces.
forbidden_collapses:
  - wire store -> promotion substrate
  - VM-local embedded store -> shared served-route authority
  - ComputerVersion route ledger -> promotion workspace or application store
  - world-wire lineage or route_profile -> served-route authority
  - VMOwnership.Published -> ComputerVersion route activation
  - sql-server decision for the wire store -> promotion mechanics decided
execution_effect:
  - Every spec, doc, and work item in this mission must name which persistence domain it means; "platform Dolt" without qualification is vocabulary drift (candidate rename in Phase E alongside World Wire).
  - The former defense of a third route-control Dolt domain is superseded; owner-settled two-store authority requires route-slot tables on the corpusd sql-server.
  - The proxy reads route decisions through vmctl's route-ledger contract; it never opens any of these Dolt stores directly to decide a route.
```

### D-WIRE. Decision node: world-wire store topology — SETTLED (owner, 2026-07-08)

```yaml
id: wire-store-sql-server
kind: term
status: settled
source: user-stated (owner, 2026-07-08)
definition: >-
  The world-wire store moves to sql-server mode now (multi-writer: proxy,
  runtime, and wire agents share it). No embedded-vs-sql-server experiment
  needed for the decision; migration engineering still needs its own probes
  (connection topology, migration path, concurrency tests).
execution_effect:
  - Unblocks honest multi-writer world-wire state without PROXY_RUNTIME_DB_PATH file-sharing hacks. Docs research 2026-07-08 confirms this hack is structurally impossible anyway — embedded mode holds an exclusive directory lock, so proxy and runtime can never share the embedded store across processes; sql-server is the only multi-process topology.
  - This migration does not make world-wire lineage a route resolver. D-ROUTE assigns served-route authority to the vmctl-owned ComputerVersion route ledger; the proxy consumes that contract and then asks vmctl to materialize the resolved version.
migration_notes:
  - NO DATA MIGRATION (owner, 2026-07-08): the universal-wire/world-wire loop has never worked end-to-end and the current wire-store data is junk. Stand up the sql-server store fresh; discard existing data. No stop-the-world window, no blue/green, no preservation ceremony.
  - Cutover is therefore code-only: swap dolthub/driver file DSN for go-sql-driver/mysql TCP DSN in the wire-store paths; config.yaml max_connections/timeouts govern multi-writer. Delete PROXY_RUNTIME_DB_PATH and the proxy's direct-file-open path with it. Rollback ref for this red-class change is the pre-swap commit SHA plus the old `config.yaml`/DSN values; because there is no data migration, git-revert of the DSN swap is sufficient.
  - Auto-GC is default-on since Dolt 1.75 (behavior.auto_gc_behavior, ~125MB-growth heuristic); manual dolt_gc() blocks writes and breaks connections — schedule it, never call it on the hot path.
settlement:
  settled_by: human
  invalidation_triggers:
    - hard blocker in migration evidence (retain as a requirement risk for any future promoted Definition)
```

### D-PROMO. Conjecture: branch isolation on the embedded store — SETTLED

```yaml
id: embedded-branch-isolation
kind: conjecture
status: settled  # Phase A pinned-connection experiment passed go test -count=10
source: opened 2026-07-08 (split from the conflated D-SQL; owner: promotion uses the VM's embedded Dolt, no separate workspaces)
claim: >-
  The VM's embedded Dolt store, under single-writer-per-process discipline,
  can carry real candidate branches: DOLT_BRANCH fork per candidate,
  session checkout onto the candidate branch for capsule-transaction
  appends, DOLT_MERGE to main + DOLT_TAG on promote — giving the
  BranchIsolation the spec models without sql-server mode and without
  DOLT_RESET rollback.
prior_evidence:  # MUST be adjudicated, not ignored (consensus 2026-07-08 caught this omission)
  - The 2026-07-07 experiment lives in internal/computerversion/dolt_branch_experiment_test.go and dolt_branch_isolation_diag_test.go; the adapter comment (dolt_promotion_adapter.go:18-22) recorded its conclusion as "DOLT_CHECKOUT in embedded mode is a no-op for the working set."
  - Re-run 2026-07-08: the diagnostic's own log contains the smoking gun — DOLT_CHECKOUT('candidate-1') succeeds, then `active branch after checkout: main`. The checkout and the follow-up query ran on DIFFERENT pooled connections. Meanwhile the sibling experiment test in the same file set concludes branch promotion IS feasible. Panel reproduction: 5/5 isolation failures on pooled runs, one flip when the pool reused a connection.
  - Diagnosis (gemini35 panelist, mechanism-verified by them with a pinned sql.Conn variant; consistent with the driver source: each connection is its own DoltSession): the falsification was a FALSE FALSIFICATION caused by database/sql connection pooling — DOLT_CHECKOUT changes only the session of the connection that executed it. On a pinned connection, isolation reportedly works.
evidence_2026_07_08:  # docs research + driver source read (v1.84.1 module cache)
  - Embedded is semantically the same engine as sql-server for sessions and branches. Driver source confirms: each connection is a fresh DoltSession, never reused (conn.go ResetSession returns ErrBadConn precisely because sessions hold per-branch working-set heads); the DSN `database` param is passed verbatim to gmsCtx.SetCurrentDatabase (connector.go:137), the same path as USE, so revision-qualified names (db/branch) resolve in the DSN too.
  - Per-session branch checkout and concurrent sessions on different branches are documented engine behavior; COMMIT is optimistic CAS on the branch HEAD — the losing concurrent writer rolls back and retries at application level.
  - Differences from sql-server are process-level only: exclusive directory lock (single process), no cross-process sharing, no connection/session reuse.
documented_constraints:
  - DOLT_MERGE and DOLT_RESET --hard implicitly commit the current transaction: promote (merge) + DOLT_TAG CANNOT be one atomic SQL transaction. Promotion must be idempotent/resumable; "atomic promotion" lives at the route-flip layer, not in a single transaction.
  - DOLT_CHECKOUT working-set semantics differ from Git (uncommitted changes do not transfer).
  - Docs are inconsistent on isolation level (Read Committed vs per-branch REPEATABLE_READ) and silent on hard-reset effects on concurrent sessions.
test: >-
  Phase A settlement test (cheap, <2s — pulled forward from Phase D per
  consensus): rewrite the branch-isolation test to run every statement on a
  single pinned connection (db.Conn(ctx)) or transaction; verify checkout
  sticks, candidate writes isolate from main, merge + tag (two steps,
  resumable) land, rollback restores. Settlement bar is repeat-N
  determinism (go test -count=10 clean), not a single pass — the pooled
  variant was observably flaky. Also correct the adapter comment and retire
  the 2026-07-07 conclusion if the pinned test passes.
evidence_artifact:
  - file: internal/computerversion/dolt_branch_isolation_pinned_test.go
  - command: go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10
  - result: 10/10 passes; branch isolation, merge, tag, and rollback are deterministic on a pinned *sql.Conn in embedded mode.
  - note: the 2026-07-07 no-op conclusion was a connection-pooling artifact, not an embedded-engine limitation.
falsifier: >-
  The pinned-connection test still fails isolation deterministically, or
  capsule writers cannot live with application-level CAS retry.
adapter_requirements_if_supported:
  - All promotion operations for a candidate MUST run on a pinned sql.Conn or sql.Tx, never through the pool; connections must be closed/returned on success, failure, and panic (leak risk is real).
  - Concurrent capsule writers within the VM serialize through the store's single-writer discipline; CAS-retry at application level.
requirement_effect:
  - No current Definition assigns branch-adapter, specification, or implementation work; future mutation requires separate promotion.
  - The tag-based adapter remains interim and disabled; `DOLT_RESET` rollback stays forbidden.
  - Any future adapter must use pinned connections and preserve single-writer isolation.
settlement:
  rule: Settled by the pinned-connection experiment; the result is requirement evidence, not phase authority.
  settled_by: evidence
```

### D-ROUTE. Boundary: one vmctl-owned ComputerVersion route writer — CAS SEMANTICS SETTLED / THIRD-STORE TOPOLOGY SUPERSEDED

```yaml
id: promotion-route-receipt
kind: boundary
status: settled_cas_semantics / superseded_third_store_persistence / violated_implementation
source: CAS/receipt semantics retained; orchestrator-settled third-store synthesis was unratified and is demoted by owner two-store authority
superseded_claim: A distinct Dolt-backed platform-control ledger is required.
superseded_by: docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
authority_object:
  id: computer-version-route-ledger
  name: ComputerVersion route ledger
  owner: vmctl
  persistence: >-
    Route-slot generations, current ComputerVersion values, and immutable
    transition receipts are tables on the corpusd world-wire sql-server. vmctl
    is the sole CAS writer. They are not a third Dolt domain, VM-local
    application state, JSON ownership registry, or promotion workspace.
definition: >-
  For each logical served-route slot, the ComputerVersion route ledger is the
  sole authority for the ComputerVersion served to ordinary requests. vmctl is
  the sole compare-and-swap writer: one durable transaction compares the
  expected generation and old ComputerVersion, writes the new ComputerVersion
  and generation, and appends an immutable transition receipt. Promotion and
  rollback are names for successful transitions through this same writer.
  Everything else is preparation, evidence, materialization, or a projection.
state_shape:
  route_slot:
    - route_slot_id
    - current_computer_version  # (CodeRef, ArtifactProgramRef)
    - generation
    - latest_receipt_id
  immutable_transition_receipt:
    - receipt_id
    - route_slot_id
    - transition_kind  # bootstrap | promote | rollback
    - old_computer_version
    - new_computer_version
    - expected_generation
    - committed_generation
    - approval_and_promotion_certificate_refs
    - rollback_target_and_prior_receipt_ref
    - idempotency_key
    - committed_at
non_definition:
  - owner approval, adoption verification, or package admission
  - Dolt branch, merge, tag, or candidate materialization state
  - ComputerSourceLineage.ActiveSourceRef or route_profile
  - VMOwnership.Published or desktop visibility
  - an AppAdoption status, UI label, Trace event, or run-acceptance level
problem_cluster:
  - live-app promotion persists adopted and can report success before an optional route adapter exists or succeeds
  - rollback can advance lineage after a best-effort DOLT_RESET failure
  - candidate-package intake contains a second switch/rollback state machine despite its evidence-only boundary
  - proxy routing resolves owner/desktop and fallback constants rather than a receipted ComputerVersion
  - vmctl PublishDesktop is a separate publication notion, not a ComputerVersion route flip
  - specs/promotion_protocol.tla carries a computer route and version route without a receipt-backed single authority
authority_boundary:
  - Runtime may submit a validated transition command but never write route state, lineage, status, Trace, or acceptance as a substitute for the route CAS.
  - vmctl alone validates expected generation/version, certificate and approval refs, materialization readiness, and idempotency before committing the route transition.
  - Proxy obtains the slot's ComputerVersion and receipt identity through vmctl, then resolves that version through the vmctl materializer. Proxy never selects an owner/desktop identity as product route truth.
  - VM lifecycle and VMOwnership.Published remain materialization/visibility facts and cannot mutate or imply the route ledger.
receipt_projection_contract:
  - Adoption may become adopted/promoted/rolled_back only after its matching durable receipt is read back; it stores the receipt id as provenance.
  - ComputerSourceLineage is a rebuildable receipt projection/index. It cannot authorize a route transition and disagreement resolves in favor of the route ledger.
  - UI active, rollback-available, and rolled-back claims require a matching receipt and generation; status strings alone render no activation claim.
  - Trace promotion/rollback events are emitted after receipt durability and carry receipt id, route slot, old/new ComputerVersion, and generation.
  - Run acceptance may project a promotion level only after independently reading and validating the receipt against the current or claimed historical route generation. An event alone is inadmissible.
fail_closed_invariants:
  - Missing ledger, missing slot, missing executor, stale expected generation/version, invalid certificate/approval, failed materialization preflight, persistence error, or receipt read-back failure leaves the slot unchanged and returns failure.
  - A route slot is initialized only through the same writer with an explicit bootstrap receipt; there is no unreceipted default or hard-coded fallback after cutover.
  - The route update and receipt append commit atomically; neither may exist without the other.
  - Rollback is a new CAS transition from the current version to the recorded prior version through the same writer, never DOLT_RESET and never a lineage rewrite.
existing_replacement: >-
  AppChangePackage admission, ComputerVersion identity, PromotionCertificate,
  owner approval, and materializer contracts already provide the command and
  evidence vocabulary. The missing object is one durable route ledger and CAS
  executor, not another activation status.
conjecture_delta: >-
  Before this decision, several records could claim activation while no object
  owned the served-route change. The executable conjecture is now singular:
  if vmctl durably commits one version-to-version CAS and receipt, ordinary
  routing and all human-visible evidence can be made faithful projections of
  that fact; without the receipt, no activation claim exists.
subordinate_contract:
  consumed_by:
    - B-resolve-immutable-inputs
    - D-verify-and-route
    - F-cutover-owner-and-close
  requirements:
    - "Define one pure RouteTransitionCommand/Receipt and vmctl-owned CAS port."
    - "Fail closed when no matching executor receipt exists; adoption, lineage, UI, Trace, and run acceptance remain unchanged."
    - "Implement ComputerVersion route-slot and receipt tables on the corpusd sql-server; route lookup is route slot → ComputerVersion → materializer."
    - "Seed existing slots through the sole writer with explicit bootstrap receipts after materialization preflight."
    - "Keep D-PROMO branch/tag operations as preparation only; they never activate a route."
  deletion_targets:
    - internal/runtime/app_promotion.go: RollForwardAppAdoption; a rolled-back version may be promoted again only by a fresh validated CAS command
    - internal/runtime/candidate_package_intake.go: SwitchCandidatePackageIntakeAdoptionReview, RollbackCandidatePackageIntakeAdoptionReview, and RollForwardCandidatePackageIntakeAdoptionReview, plus their mutating API routes/callers; retain read-only intake/review evidence
    - frontend/src/lib/FeaturesApp.svelte: canRollForward and status-only active/rollback affordances
    - LineageBasedRouteResolver, PROXY_RUNTIME_DB_PATH/RuntimeDBPath, lineage/hard-coded routing, and route uses of ActiveSourceRef or RouteProfile
  verifier_contract: >-
    Active phases B/D/F must prove deleted routes and callers absent and prove
    nil, error, stale-receipt, failed-persistence, and failed-materialization
    paths leave adoption, lineage, UI, Trace, and run acceptance unchanged.
    This subordinate document supplies requirements only; it owns no next
    action, implementation order, phase gate, or completion decision.
formalization_contract:
  - Require the specification and Go writer to satisfy the named route-slot, receipt, generation, and ComputerVersion conformance contract; new implementation requires a separately promoted Definition.
  - Model receipt append and route change as one CAS action, rollback as the same action in reverse, and require NoReceiptWithoutRouteChange, NoRouteChangeWithoutReceipt, AtMostOneWinnerPerGeneration, and NoProjectionBeforeReceipt.
protected_surfaces:
  - ComputerVersion route authority
  - promotion and rollback
  - candidate computers
  - vmctl persistence and materialization
  - run acceptance and Trace evidence
admissible_evidence:
  contract:
    - concurrent CAS property test proves exactly one writer wins an expected generation
    - idempotent retry returns the same receipt; stale version/generation and certificate mismatch fail without mutation
    - injected persistence failure proves neither route nor receipt advances; restart reload proves both do
    - negative projection tests prove no receipt means no adopted/active/rolled_back lineage, UI, Trace, or acceptance claim
  formal:
    - TLC-green rewritten spec plus a conformance test binding the modeled CAS/receipt fields to the Go command, slot, and receipt types
  deployed_product_path:
    - staging ordinary request records explicit old ComputerVersion and bootstrap receipt, then new ComputerVersion and promotion receipt, then old ComputerVersion and rollback receipt
    - each response's materialization and each API/UI/Trace/acceptance projection name the matching receipt and generation
    - missing/failing executor, ledger, persistence, or materializer produces failure and no success projection
settlement:
  definition: settled by the authority decision in this node and D-STORE/D-STORES
  implementation: >-
    Open until the contract, formal, and deployed product-path evidence above
    are recorded. Merge/tag, lineage, adoption, or desktop publication alone
    cannot settle it.
requirement_effect: >-
  Active phases B/D/F may consume D-PROMO's settled branch mechanics but cannot
  claim promotion from merge, tag, adoption, or desktop publication. Until the
  implementation settles, active and rollback-available product claims fail
  closed. Duplicate roll-forward and candidate-intake switch paths are deletion
  targets, not compatibility paths.
rollback_path:
  documentation: revert this authority-decision hunk; no runtime state changed by this pass
  future_red_rollout:
    - record the pre-change commit and disable promotion before the truth-gate slice; rollback returns to disabled/fail-closed behavior, never to legacy false-success paths
    - seed old versions only with bootstrap receipts and verify materialization before proxy cutover; once a slot is ledger-authoritative there is no lineage, desktop, or hard-coded fallback
    - after any promotion, operational rollback is a receipted CAS to the recorded prior ComputerVersion; a code rollback is admissible only while preserving ledger reads and fail-closed semantics
heresy_delta:
  discovered:
    - D-STORES omitted the shared control authority that D-ROUTE required, while D-WIRE implied world-wire lineage could resolve routes
    - five competing activation/publication meanings without one served-route writer
    - promotion acceptance can be inferred from events/status without durable route evidence
    - the TLA model carries dual route meanings and no immutable transition receipt
  introduced: []
  repaired: []
definition_correction: >-
  The storage/route authority contradiction is repaired in semantic authority
  only. No live H031, promotion, acceptance, or rollback implementation heresy
  is claimed repaired by this documentation pass.
```

### D-STORE. Historical decision node: all-in on Dolt — PRODUCT-STATE SEMANTICS RETAINED / ROUTE-STORE CONSEQUENCE SUPERSEDED

```yaml
id: storage-fork
kind: term
status: settled_product_state / superseded_route_store_consequence
source: owner all-in-on-Dolt authority for product state; orchestrator-added third route-store consequence was unratified
superseded_claim: Route control requires a distinct Dolt persistence domain.
superseded_by: docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
definition: >-
  Choir commits to Dolt as the load-bearing persistence substrate for durable
  product state. ComputerVersion route-control rows live as tables on the
  corpusd world-wire sql-server; vmctl remains their sole CAS writer.
  Application-level revision/provenance chains and lifecycle registries remain
  useful indexes, caches, or projections but do not reopen the database choice
  and cannot become route authority.
non_definition:
  - every ephemeral cache or materializer fact must be stored in Dolt
  - the vmctl JSON ownership registry is durable enough for route authority
  - ComputerSourceLineage can substitute for the ComputerVersion route ledger
requirement_effect:
  - Active phases B/D/F consume the Dolt product-state requirement and the corpusd route-slot topology.
  - Commit boundaries, rollback, history correctness/latency, throughput, build friction, and replication remain verification axes, not sequencing or decision authority.
  - Any demonstrated feasibility contradiction remains a requirement risk; escalation or sequencing requires a separately promoted Definition.
settlement:
  rule: Settled by owner authority. Verification tasks may change implementation tactics but not the chosen substrate without a new explicit owner decision.
  settled_by: human
```

### D-HISTORY. Conjecture: native Texture audit history requires an explicit commit boundary — SETTLED

```yaml
id: texture-native-history
kind: conjecture
status: settled
source: observed 2026-07-10 (Phase B source-path reconciliation)
claim: >-
  The existing `choir texture history` route is not yet a load-bearing Dolt
  audit read. `Store.GetHistory` walks immutable `choir.texture_revision`
  objects from the current object-graph working set, while normal VM-local
  object-graph writes issue SQL transactions but no `DOLT_COMMIT`; therefore
  `dolt_history_og_objects` / `AS OF` cannot yet supply the route's history.
observables:
  - `internal/store/texture.go:GetHistory` calls `GetDocument` and `GetRevision` and never queries a Dolt history table or `AS OF`.
  - `internal/objectgraph/dolt_store.go` commits SQL transactions but does not create Dolt commits for normal writes.
  - production `DOLT_COMMIT` callers are limited to unrelated cycle/platform paths and the inert promotion adapter.
test: >-
  Add a focused embedded-Dolt contract that creates Texture revisions through
  the production store, queries `dolt_history_og_objects`, and proves whether
  a durable per-revision audit boundary exists. Then make the smallest
  canonical-write change that yields deterministic native history and route
  `GetHistory` through it without changing the public response shape.
falsifier: >-
  The production write path already creates separately addressable Dolt commits
  for each Texture revision and the history system table can reconstruct the
  same ordered entries without any write-path change.
scope_if_supported: VM-local embedded Texture/object-graph state only.
execution_effect: >-
  Phase B may not claim the Dolt audit-read gate until the focused contract is
  green, latency is recorded, and the CLI/API history route is observed using
  native Dolt history. The change touches Texture canonical writes and is red;
  rollback ref is f1e2d7a3.
evidence_2026_07_10:
  - The pre-fix focused contract observed zero distinct native revision commits.
  - Startup/backfill, Texture document/revision creation, and revision metadata
    patches mark a serialized native-history batch dirty. The first history read
    creates one VM-state checkpoint for the accumulated working set; repeat reads
    without intervening mutations create no commit.
  - `GetHistory` selects committed revision snapshots from
    `dolt_history_og_objects`, traverses the canonical parent chain, and resolves
    only the requested page through `og_objects AS OF '<validated-hash>'`.
  - The embedded driver panics when `AS OF ?` uses a bound placeholder; the
    implementation validates Dolt-returned hashes as lowercase alphanumeric
    before interpolation, while owner/document/canonical IDs remain bound.
  - `TestTextureHistoryHasNativeDoltAuditCommits` proves 25 immutable revisions
    are addressable from one batched native checkpoint, repeat reads are clean,
    and latest-10 history resolves in 10.243ms locally.
  - Focused `-race` contracts pass in 7.701s; `go test ./internal/store -count=1`
    passes (the 193.222s wall time was under parallel race-test contention and is
    not used as a performance baseline).
performance_contradiction_2026_07_10: >-
  After push of 1870452c, GitHub Actions run 29072160790 kept the selected
  runtime-shard-0 and non-runtime race lanes in progress beyond 10 minutes,
  while the two preceding main runs (29071521464 and 29067067716) completed the
  entire workflow in roughly 3 minutes. The new per-document/per-revision
  `DOLT_COMMIT` boundary is the only overlapping store-path change and is
  therefore the leading causal hypothesis. Treat the current implementation as
  a performance regression, not a landed proof.
observer_upgrade: >-
  Replace eager per-write checkpoints with a serialized dirty batch and a
  history-read barrier: writes remain durable in the Dolt working set; the
  first native history read after mutations creates one VM-state checkpoint,
  then queries dolt_history + AS OF. Re-run focused correctness/latency, full
  store tests, CI race lanes, deploy, and staging product proof.
replacement_result_2026_07_10: >-
  Implemented the dirty-batch history-read barrier. Local correctness,
  repeat-read, injection-guard, metadata-concurrency, vet, and focused race
  contracts are green. Fresh run 29072918594 returned every normal and race lane
  green, deployed b7f512f2 to Node B, and passed the authenticated staging proof.
remaining_edge: >-
  None for the Phase B native audit-read slice. Broader Dolt batching/throughput,
  rollback recovery, build friction, and replication remain mission-level axes.
settlement:
  rule: >-
    Settled by b7f512f2: focused native-history and race contracts, all fresh CI
    test/race lanes green in run 29072918594, Node B health reporting b7f512f2,
    and an authenticated deployed create/revise/history proof with cleanup.
  settled_by: evidence
```

### M3.1a. H011/H012 role-keyword oracle deletion — SETTLED

```yaml
id: role-keyword-oracle-deletion
kind: conjecture
status: proven
source: choir-doctrine H011/H012 + Phase B M3.1
claim: >-
  Narrative words such as researcher, code, deploy, test, or verify must not
  select Texture prompt-policy branches. Structured metadata may carry explicit
  intent, and Texture's unconditional Probe/Execute affordances remain available
  for its own judgment.
existing_replacement: >-
  `runMetadataExplicitResearcher` already provides a structured researcher-intent
  input, while the base revision policy already exposes `spawn_agent` and
  `request_super_execution` by evidence class. The substring functions and the
  worker-overlay Execute branch are therefore superseded control residue and can
  be deleted rather than patched.
construction:
  - delete `texturePromptNeedsSuperExecution` and `texturePromptExplicitlyRequestsResearcher`
  - let `integrate_worker_findings` depend only on structured intent
  - retain the researcher overlay only for structured metadata
  - remove the keyword-selected Execute worker overlay
  - enforce the H011/H012 detector at zero production hits with docs/tests allow-contexts
local_evidence:
  - focused inverted prompt tests pass
  - textureprompts package passes
  - all 345 standard runtime tests pass across four local shards
  - H011/H012 reports enforced=true and total_hits=0
  - a temporary production marker makes `--fail-on-regression` exit 1; marker removed
deployed_evidence:
  - CI run 29074494439 passed, including all standard/race lanes and the enforced Heresy Detector
  - Node B health reported deployed commit 82839687ff092549483a4da17128c3cc4818508f
  - a real-passkey Texture request containing researcher/code/deploy/test/verify returned 202 and produced appagent revision 62dc25fd-37d2-4569-924b-a6f004a3a979
  - proof document b3ac94d8-b0b0-4cb6-b5c0-23ee9e6e5a97 was deleted successfully
execution_effect: >-
  H011/H012 are repaired at the Phase B deletion bar: the branches are absent,
  inverted tests and zero enforcement prevent their return, and the deployed
  Texture path remains healthy. This does not settle H009/H010 or the rest of
  M3.1. Rollback ref is d6ce587d.
```

### M3.1b. H010 post-write email forcing deletion — SETTLED

```yaml
id: post-write-email-forcing-deletion
kind: conjecture
status: proven
source: choir-doctrine H010 + Phase B M3.1
problem: >-
  After a Texture write succeeds, `requiredContinuationAfterTextureEdit` parses
  the original prompt and canonical document prose, synthesizes an email intent,
  and directly invokes `request_email_draft`. Narrative content therefore still
  selects and executes an exact next tool after the canonical write.
classification: symptom on the Texture prompt-policy layer; not a substrate defect
existing_replacement: >-
  `request_email_draft` is already an unconditional typed Texture tool, and the
  revision policy already tells Texture when the Email appagent handoff is
  legitimate. The actor can choose that affordance from the structured owner
  request and stored artifact without a backend prose oracle.
conjecture: >-
  Deleting the post-write parser/executor will remove hidden email routing while
  preserving owner-requested draft creation through Texture's typed tool.
protected_surfaces:
  - canonical Texture revision aftermath
  - Email appagent draft creation and approval boundary
admissible_evidence:
  - deletion diff and inverted tests proving a write result carries no forced email continuation
  - direct typed request_email_draft contract remains green
  - full runtime/race CI, Node B identity, and deployed Texture product smoke
local_evidence:
  - 491 lines deleted across the forcing path, prose parser, and superseded parser tests
  - both initial-user and grounded-worker write contracts prove no email continuation fields are synthesized
  - direct typed request_email_draft creation and sanitization contracts remain green
  - all 338 standard runtime tests pass across four local shards; focused race and go vet pass
deployed_evidence:
  - CI run 29075852884 passed all standard/race gates and deployed fd492b912b77639fc06143f55f05787fafa2a4f4 to Node B
  - real-passkey Texture loop 8c6cb802-84b3-4684-9a8f-b0e9ab10c405 accepted old-parser-shaped inert email prose and produced appagent revision 5c2e7279-b331-47cd-b1fc-ca2d2cf5175b
  - proof document 8b457c2b-e066-45f6-b910-05b002e287e9 was deleted successfully
rollback_ref: 73657a8f
heresy_delta:
  discovered:
    - H010 post-write prose parser directly executes request_email_draft
  introduced: []
  repaired:
    - H010 post-write prose parser and direct request_email_draft executor
execution_effect: >-
  The post-write H010 site is repaired at the deletion/inverted-test/deployed
  product bar. The broader H010/H024/H026 family remains open and report-only;
  rollback remains 73657a8f.
```

## Historical Determined-State Receipt (2026-07-08)

This receipt cannot sequence, resume, complete, or authorize current work.

```yaml
determined_state:
  settled:
    - claim: Phase 4 seams landed before Phase 0 foundations; e393eb5c and e5c1d38a are seams, not completions.
      source: observed (grep-verified 2026-07-08; unanimous panel)
      execution_effect: sequencing corrected below; W5 labeling applies.
    - claim: W2 timeout hardening is built: `DefaultVmctlTimeout` 60s, `http.Server` Read/Write timeouts, and fast 504 staging proof for `/api/universal-wire/stories`.
      source: observed
      execution_effect: I3 bounded-request-path invariant is now satisfied.
    - claim: W1 detector manifest + CI discovery is wired. `docs/heresy-detectors.md` includes H030/H031 and the I4 destructive-rollback guard; `scripts/check-heresies.sh` parses the manifest and supports per-row path exclusions; the `Heresy Detector Discovery` CI job reports counts in the `check` gate.
      source: observed
      execution_effect: H031/I4 binding is in CI discovery; fail-on-regression enforcement is deferred per phase.
    - claim: W3 landing-loop evidence for e393eb5c/e5c1d38a is recorded: both commits are in main history, their own CI runs were cancelled/failed, observed deploys are `67fff296` (first 60s timeout, 2026-07-09T04:56:18Z), `1ed41f2b` (2026-07-09T05:12:21Z), and `14f56211` (2026-07-09T05:42:19Z), the lineage resolver is not active in staging (no `PROXY_RUNTIME_DB_PATH`), and no production binary configures the promotion adapter.
      source: observed (CI logs, staging health, grep)
      execution_effect: seam labels are accurate; W3 closed.
    - claim: WithPromotionAdapter has zero cmd/ callers; adapter is dead in production.
      source: observed
      execution_effect: no promotion claims admissible; adapter wiring blocked on S1 + D-PROMO.
    - claim: D-PROMO pinned-connection branch isolation is settled by `TestDoltEmbeddedBranchIsolationPinnedConnection -count=10`.
      source: observed (go test, 10/10 passes)
      execution_effect: the embedded Dolt store can provide the branch isolation the spec models; the tag-only adapter remains non-conformant until Phase D.
    - claim: S1 spec↔adapter reconciliation is settled: `specs/promotion_protocol.tla` scope header names the embedded store, references D-PROMO, and notes the current tag-only adapter does not implement branch isolation.
      source: observed
      execution_effect: the spec is target-state with a conformance gap; W6 will add the binding when the branch adapter lands.
    - claim: C1-C7 doc truth corrections are committed; C4 relabeled `substrate-hardening` and `cross-substrate-proof` as `checkpoint_incomplete`.
      source: observed
      execution_effect: no mission doc may be cited as complete while carrying unproven claims.
    - claim: a703bf44 docs checkpoint pushed to origin/main 2026-07-08.
      source: observed
      execution_effect: W4 closed.
    - claim: Migration completeness baseline — actor substrate 95%, wire wiring 70%, OG integration 60%, business-logic extraction 0%, continuation deletion 0%, parent/child deletion 5%, texture-forcing removal 0%.
      source: pre-purge evidence snapshot at Git commit 8f62fe3b
      execution_effect: variant baseline below.
    - claim: H030 (mailbox polling) repaired 2026-06-27; registry update only.
      source: settled-definition (heresy-eradication doc)
    - claim: Retrieval search returns zero results for terms that exist; /api/trajectories ignores ?limit=.
      source: observed (assessment)
      execution_effect: C-RETR and C-PAGE work items exist in Phase E.
    - claim: The existing Texture history route walks current object-graph revision objects and normal VM-local object-graph writes create no explicit Dolt commits.
      source: observed (Phase B source reconciliation, 2026-07-10)
      execution_effect: D-HISTORY is settled; Phase B proceeds to the M3.1/M3.2 heresy kill waves.
    - claim: H011/H012 production substring-oracle callsites are deleted and their detector is promoted to zero enforcement.
      source: observed (deletion diff + inverted tests + detector negative proof + CI/staging landing loop, 2026-07-10)
      execution_effect: M3.1a is settled; M3.1 continues with the H009/H010 forcing cluster.
    - claim: Texture still parses prompt/document prose after a canonical write and directly executes request_email_draft.
      source: observed (`executeTextureEditTool` → `requiredContinuationAfterTextureEdit` → `extractEmailDraftIntent`, 2026-07-10)
      execution_effect: M3.1b is settled by deletion, inverted tests, CI, Node B identity, and a deployed Texture proof; broader H010/H024/H026 work remains.
    - claim: Historical D-ROUTE third-store topology is superseded; retained CAS/receipt semantics assign ordinary served-route authority to corpusd sql-server route-slot tables with vmctl as sole writer.
      source: owner two-store directive, applied by the completed audited-construction two-store-topology node
      superseded_claim: one distinct durable Dolt-backed ComputerVersion route-control domain
      superseded_by: docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
      completed_definition_consumption: >-
        Active phases B-resolve-immutable-inputs, D-verify-and-route, and
        F-cutover-owner-and-close own implementation. Receipt projection,
        fail-closed truth, idempotency, and sole-writer contracts remain
        subordinate inputs.
  settled_2026_07_08_owner:
    - claim: D-STORE is all-in on Dolt; native history/branch behavior becomes load-bearing. Storage inventory questions are engineering homework, not a renewed decision gate.
      source: owner authority, reaffirmed 2026-07-09
      requirement_effect: Dolt remains the settled product-state substrate; new tactics, sequencing, or escalation require a separately promoted Definition.
    - claim: Dolt persistence taxonomy — two product-state stores (world-wire sql-server and per-VM embedded stores) plus one narrow vmctl-owned ComputerVersion route-control ledger; promotion preparation is an operation on the embedded store and activation is a CAS in the ledger.
      source: user-stated product-store constraints + orchestrator-settled route-control consequence + observed
      settled_effect: D-WIRE, D-PROMO, D-STORE, and D-ROUTE requirements are settled; promotion is decoupled from the wire store and VM ownership/desktop publication.
    - claim: Universal→World Wire rename was historical Phase E work and has no current execution assignment.
      source: user-stated
    - claim: Current wire-store data is junk (the wire loop has never worked end-to-end); the sql-server store stands up fresh with no data migration.
      source: user-stated
      requirement_effect: D-WIRE supplies settled store and deletion constraints; new sequencing or mutation requires a separately promoted Definition.
  open: []
```

## Historical Value Criterion

The retired program prioritized falsifiable claims, staging unblockers,
deletions with inverted tests, and finally cutover construction. This ordering
is evidence only and cannot sequence current work.

## Historical Variant Receipt

The following closed counts record the retired mission's last observed state.
They are not a current progress measure or resumption authority.

Baseline 2026-07-08. Productive execution reduces these counts:

```yaml
variant:
  heresy_families_without_ci_detector: 0         # 12 aggregate detector families (H001-H031 + I4) are wired to CI discovery via docs/heresy-detectors.md and scripts/check-heresies.sh; target 0
  heresy_families_without_ci_enforcement: 11      # H011/H012 is deployed at zero enforcement; target 0
  heresy_families_live: 9                        # live-site clusters, target 0: texture forcing (H009-12/H024a,b/H026), parent/child (H001-05 + H015-16), continuations (H006-08), acceptance/obligations (H013-14/H017-18), surface residue (H019-23), vocabulary (H025/H027-29), candidate-VM (H031+new), route-over-CV violation, dual-store SQL paths
  doc_corrections_open: 0                        # C1–C7 committed, target 0
  spec_impl_gaps_open: 0                         # S1 settled with scope/conformance note, target 0
  unbounded_request_paths: 0                     # W2 committed and staging-proven, target 0
  seam_commits_unlabeled: 0                      # e393eb5c, e5c1d38a evidence recorded in W3
  mislabeled_complete_missions: 0                # substrate-hardening, cross-substrate-proof relabeled in C4
  past_mission_open_edges_untriaged: 0           # P-TRIAGE table committed below, target 0
  decision_nodes_unresolved: 0                   # D-STORE, D-PROMO, D-WIRE, and D-ROUTE authority are settled; D-ROUTE implementation violation remains counted above
  sql_dual_paths_live: 9                         # ~8–10 per assessment
```

Bad variants (forbidden): elapsed time, files touched, commit count,
percentage feelings.

## Historical Execution Receipts

The former Phase A–E execution program, receding-horizon rules, phase-gate
protocol, and per-phase exit bars are immutable historical evidence. They own
no current status, sequencing, next action, mutation permission, resumption, or
completion decision. Git history is the forensic source for their retired
topology and wording.

The completed audited-construction phases B, D, and F consumed the settled
D-ROUTE and H031 requirements named in `subordinate_contract` above. No current
mutation or closure authority remains. A direct `/goal` on this document is
forbidden.
## Retained Evidence Contracts

The following proof obligations and ledger entries are subordinate evidence
consumed by the completed audited-construction B/D/F gates.

- Staging traces for request-path claims (`api.resolve` latency, 504s).
- `go test ./...` plus inverted tests per deletion.
- TLC in CI for spec changes.
- Detector-count deltas as the per-pass heartbeat.

## Evidence Classes And Claim Scope

Per the definition skill. Specific bindings:

- "H0xx repaired" requires: deletion diff + inverted test + detector at
  fail-on-regression showing zero live sites.
- "Phase D promotion works" requires: staging trace of an atomic route flip
  between explicit ComputerVersions plus the immutable vmctl ledger receipt
  and generation for that flip and a demonstrated receipted rollback flip —
  not a TLC-green spec, merge/tag, lineage mutation, event, adoption status,
  desktop publication, or adapter unit test.
- "ComputerVersion route ledger is load-bearing" requires: CAS concurrency,
  idempotency, injected-persistence-failure, and restart-recovery contracts;
  proxy ordinary-request evidence reading the ledger before materialization;
  and negative proof that no fallback or downstream success projection occurs
  when ledger read/write or materialization fails.
- "Promotion accepted" requires run acceptance to independently resolve the
  named receipt against the claimed ledger generation. A Trace event or status
  without that lookup is evidence of an attempted projection, not promotion.
- "Cutover complete" per entity requires: OG reads default in production +
  SQL fallback exercised or expired + dual-write deleted.
- Panel/reviewer statements are `external second opinion` — adjudicated,
  never directly promoted.

## Evidence Ledger

```yaml
- claim: e393eb5c and e5c1d38a are seams (not load-bearing) as of 2026-07-08.
  definition_node: seam
  evidence_class: observed file/tool result
  command_or_observation: grep WithPromotionAdapter cmd/ (zero hits); grep PROXY_RUNTIME_DB_PATH (env-gated, default unset); route_resolver.go:47 hard-coded constants.
  result: confirmed
  uncertainty: W3 closed by landing-loop evidence below.
- claim: Promotion had no assigned durable served-route authority and seven live code/model surfaces supplied competing activation meanings.
  definition_node: promotion-route-receipt, dolt-store-taxonomy, storage-fork
  evidence_class: observed file/tool result + orchestrator authority settlement
  command_or_observation: >-
    Source audit 2026-07-10: internal/runtime/app_promotion.go persists adopted
    before the optional adapter and contains a separate RollForwardAppAdoption;
    internal/runtime/candidate_package_intake.go contains switch, rollback, and
    roll-forward lineage mutators despite declaring deployed mutation blocked;
    internal/vmctl/ownership.go stores VMOwnership.Published in a JSON registry
    whose saveLocked reports and returns on persistence errors;
    internal/proxy/lineage_route_resolver.go resolves route_profile to
    owner/desktop; frontend/src/lib/FeaturesApp.svelte renders active and
    rollback/roll-forward affordances from status plus rollback refs;
    internal/runtime/run_acceptance.go raises promotion acceptance from
    promotion events; specs/promotion_protocol.tla has dual route meanings and
    no immutable transition receipt.
  result: >-
    D-STORES/D-STORE/D-ROUTE now assign one distinct Dolt-backed
    ComputerVersion route ledger to vmctl's sole CAS writer. Adoption, lineage,
    UI, Trace, and acceptance are receipt projections; roll-forward and
    candidate-intake switch mutation paths are explicit future deletions.
  uncertainty: >-
    Definition authority is settled only. No ledger, receipt projection gate,
    deletion, proxy cutover, formal rewrite, or staging proof exists yet.
  heresy_delta:
    discovered:
      - unassigned served-route authority behind competing activation paths
      - status/event-based promotion acceptance without durable route evidence
    introduced: []
    repaired: []
- claim: W3 landing-loop evidence for seam commits e393eb5c and e5c1d38a.
  definition_node: w3
  evidence_class: observed tool result + observed staging state
  command_or_observation: >-
    e393eb5c CI run 28963931647 (2026-07-08T17:50:58Z) was cancelled after
    runner acquisition failure; Go Vet + Test + Build failed, Deploy to Staging
    (Node B) was cancelled. e5c1d38a CI run 28964053923 (2026-07-08T17:52:55Z)
    completed with Deploy to Staging (Node B) successful and Generate SBOMs
    failing. The timeout fix was first observed at deployed SHA 67fff296
    (2026-07-09T04:56:18Z). A later deploy at 1ed41f2b (2026-07-09T05:12:21Z)
    and the live staging health check at 14f56211 (2026-07-09T05:42:19Z) both
    show the same 60s bound (`api.resolve.max_duration_ms: 60024`). The
    deployed SHA changes with each CI deploy; the W3 evidence is the
    time-scrolled sequence of observed deploys, not an evergreen "current"
    identity. Staging proxy log shows no "route resolver: wired lineage-based
    resolver" line; nix/node-b.nix does not set PROXY_RUNTIME_DB_PATH, so the
    proxy uses the hard-coded VM identity fallback. grep for DoltPromotionAdapter
    or WithPromotionAdapter under cmd/ returns zero hits; no production binary
    configures the promotion adapter.
  result: >-
    e393eb5c and e5c1d38a are in main history and are present on Node B via
    later deploys, but their own CI runs were not clean green/cancelled. The
    lineage resolver is not active in staging; the promotion adapter is not
    wired in any production flow. Both commits remain seams (not load-bearing).
- claim: a703bf44 pushed to origin/main.
  definition_node: w4
  evidence_class: observed tool result
  command_or_observation: git push origin main → e5c1d38a..a703bf44
  result: shared
- claim: timeout invariant violated.
  definition_node: bounded-request-path
  evidence_class: observed file result + staging trace (assessment)
  command_or_observation: internal/vmctl/client.go:22 (180s); no ReadTimeout/WriteTimeout in proxy server; staging api.resolve max 180,029ms.
  result: fixed by W2 (commit 67fff296 + prior server.go timeout defaults; staging api.resolve max now 60,001ms; raw staging proof removed from the worktree after this result was adjudicated here)
- claim: Dolt operational semantics for promotion and topology (per-session branch checkout; embedded exclusive directory lock; optimistic-CAS commit with app-level retry; DOLT_MERGE/DOLT_RESET implicitly commit the transaction so merge+tag is never one transaction; branch-in-DSN undocumented for embedded driver; auto-GC default since 1.75, embedded applicability unverified; no official embedded→sql-server migration guide).
  definition_node: embedded-branch-isolation, wire-store-sql-server
  evidence_class: external documentation review (docs.dolthub.com, dolthub/driver README, DoltHub blog) + observed test result, 2026-07-08 / 2026-07-09
  command_or_observation: web research agent report; source URLs recorded in the D-PROMO and D-WIRE nodes; go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10
  result: D-PROMO settled by the pinned-connection -count=10 determinism test; D-WIRE multi-process rationale confirmed; spec constraint added to S1
  uncertainty: isolation-level docs inconsistent; hard-reset effects on concurrent sessions undocumented
- claim: The embedded driver is semantically equivalent to sql-server for session/branch semantics — fresh DoltSession per connection (never reused), DSN database param passed verbatim to SetCurrentDatabase so db/branch revision names work in the DSN; differences are process-level only (exclusive lock, single process).
  definition_node: embedded-branch-isolation
  evidence_class: observed file result (driver source read) + observed test result
  command_or_observation: ~/go/pkg/mod/github.com/dolthub/driver@v1.84.1 — conn.go ResetSession/IsValid, connector.go:136-137, parse_dsn.go:57-70; go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10
  result: D-PROMO is settled by the pinned-connection branch-isolation determinism test (go test -count=10, 10/10 passes); the prior 2026-07-07 falsification is diagnosed as a connection-pooling artifact, and a pinned sql.Conn/BeginTx variant isolates correctly.
  uncertainty: revision-name resolution via SetCurrentDatabase is inferred from the engine's USE path; the integration test confirms it as a side effect
- claim: The public Texture history shape exists but its implementation is an application revision-chain read, not a native Dolt audit read.
  definition_node: texture-native-history
  evidence_class: observed file result
  command_or_observation: >-
    rg/sed inspection of cmd/choir/main.go, internal/runtime/texture.go,
    internal/store/texture.go, internal/store/graph_store.go, and
    internal/objectgraph/dolt_store.go on f1e2d7a3.
  result: D-HISTORY opened as testing before any behavior fix.
  uncertainty: Native history contents and latency remain to be measured by the focused embedded-Dolt contract.
- claim: The local Texture history implementation is backed by committed Dolt snapshots and bounded AS OF reads.
  definition_node: texture-native-history
  evidence_class: integration/contract test
  command_or_observation: >-
    go test ./internal/store -run TestTextureHistoryHasNativeDoltAuditCommits
    -count=1 -v; go test ./internal/store -count=1
  result: >-
    25 immutable revisions are addressable from one batched checkpoint; latest-10
    history returned in 10.243ms; repeat reads created no commit; focused race
    contracts passed in 7.701s; full internal/store package passed.
  uncertainty: >-
    No deployed product-path proof yet. The comprehensive runtime test target is
    independently unbuildable because stale tests reference removed response and
    request fields; this change does not rely on that suite as evidence.
- claim: Eager per-Texture-write Dolt commits are not an admissible D-HISTORY implementation tactic.
  definition_node: texture-native-history
  evidence_class: CI timing observation
  command_or_observation: >-
    GitHub Actions run 29072160790 remained in selected race lanes beyond 10m;
    prior main runs 29071521464 and 29067067716 completed in about 3m.
  result: D-HISTORY weakened; replace eager commits with a dirty-batch history-read barrier.
  uncertainty: The replacement must be verified by a fresh CI run; attribution is a leading causal inference until that comparison lands.
- claim: D-HISTORY dirty-batch native Texture history is load-bearing on staging.
  definition_node: texture-native-history
  evidence_class: deployed staging proof + CI + integration/contract test
  command_or_observation: >-
    Commit b7f512f2; GitHub Actions run 29072918594; curl
    https://choir.news/health; authenticated browser product path POST document,
    POST two revisions, GET history, DELETE proof document.
  result: >-
    All fresh normal and race CI lanes green; deploy job green; health status ok
    with deployed_commit b7f512f294fae82d87976a77e4cb2157950547e7
    (deployed_at 2026-07-10T06:17:45Z). Staging created document
    36559ad8-8d79-43fe-941a-348e99a40dde (201, 32.3ms), created revisions
    e7dc018f-e0d2-4b80-8f5f-8014041c40b4 (201, 39.4ms) and
    92f2d0e4-b180-4662-9a96-2529d30e2559 (201, 49.8ms), then returned both
    newest-first with the exact parent link from GET history (200, 29.8ms).
    Cleanup DELETE returned 200 in 112ms.
  uncertainty: >-
    Evidence settles the Phase B audit-read slice, not the remaining Dolt
    batching/throughput, rollback, replication, or later cutover axes.
  heresy_delta:
    discovered:
      - application-chain history had no native Dolt commit boundary
      - embedded driver panics on a bound AS OF placeholder
      - eager per-write checkpoints caused a CI performance regression
    introduced: []  # eager regression was never deployed and was superseded before acceptance
    repaired:
      - native history now uses a dirty-batch checkpoint plus validated-hash AS OF reads
- claim: M3.1a removes production role-keyword policy switches and promotes H011/H012 to deployed zero enforcement.
  definition_node: role-keyword-oracle-deletion
  evidence_class: deletion diff + unit/inverted test + executable detector + deployed product proof
  command_or_observation: >-
    go test ./internal/runtime -run
    'TestTexturePromptNarrativeRoleWordsDoNotSwitchPolicyBranches|TestExplicitNoWorkerDecisionParsesWithoutNarrativeRouteOracle|TestTexturePromptForPartialFindingsForbidsFalseFollowupClaims'
    -count=1; go test ./internal/textureprompts -count=1;
    scripts/check-heresies.sh --fail-on-regression; temporary production marker
    negative proof; CI run 29074494439; Node B /health; real-passkey staging
    create/revision/revise/poll/delete probe.
  result: >-
    Focused and full runtime tests green; production H011/H012 detector count
    zero; enforced detector passes clean and fails with exit 1 when a banned
    production marker is temporarily introduced. CI passed, Node B reported
    82839687, and deployed Texture loop d562f055-b21a-4678-93f4-79cabcb11796
    produced appagent revision 62dc25fd-37d2-4569-924b-a6f004a3a979 before
    proof cleanup.
  uncertainty: broader H009/H010 forcing cluster remains live; staging product health does not independently reveal internal branch selection.
  heresy_delta:
    discovered: []
    introduced: []
    repaired:
      - H011 narrative role-word policy oracle
      - H012 narrative execution-word policy oracle
- claim: M3.1b deletes the post-write prose parser and direct Email draft executor while preserving the typed request_email_draft contract.
  definition_node: post-write-email-forcing-deletion
  evidence_class: deletion diff + inverted/direct-tool tests + deployed product proof
  command_or_observation: >-
    go test ./internal/runtime -run
    'Test(EditTextureEmailProseDoesNotForceEmailAppagentContinuation|GroundedEmailArtifactDoesNotForceEmailAppagentContinuation|TextureRequestEmailDraftCreatesTraceVisibleEmailAgentRun)'
    -count=1; PARALLEL_SHARDS=1 scripts/go-test-runtime-shards; focused -race;
    go vet; CI run 29075852884; Node B /health; real-passkey staging
    create/revision/revise/poll/delete probe.
  result: >-
    495 net lines removed; both old forcing cases now return only stored revision
    data; direct typed draft creation remains green. CI passed, Node B reported
    fd492b91, and deployed Texture loop 8c6cb802-84b3-4684-9a8f-b0e9ab10c405
    produced appagent revision 5c2e7279-b331-47cd-b1fc-ca2d2cf5175b before
    proof cleanup.
  uncertainty: broader H010/H024/H026 detector family remains report-only and contains mechanical/typed candidates that require classification.
  heresy_delta:
    discovered:
      - H010 post-write prose parser directly executed request_email_draft
    introduced: []
    repaired:
      - H010 post-write prose parser and direct request_email_draft executor
- claim: Plan-review consensus round 2026-07-08 (4/4 panelists returned; gpt55 output empty/failed-silently) adjudicated. Confirmed blockers, all fixed in this document — D-STORES file mapping was inverted (world-wire store is internal/platform/objectgraph_store.go, not internal/objectgraph/dolt_store.go); D-PROMO had ignored the prior 2026-07-07 experiment (adapter comment + two test files), whose falsification is diagnosed as a connection-pooling artifact (checkout ran on one pooled conn, queries on others; pinned-conn variant reportedly isolates correctly) — settlement pulled into Phase A with a -count=10 determinism bar; completion criterion 3 gained a falsified-D-PROMO fallback clause; Phases B–E gained explicit exit bars; gate adjudication must be committed as auditable evidence; supersession must be machine-readable (C5 expanded to mission-graph superseded nodes + doc-authority-manifest entries for all three docs).
  definition_node: seam, embedded-branch-isolation, dolt-store-taxonomy, phase-gate-protocol
  evidence_class: external second opinion (panel) + observed (repo re-verification of B1/B2; diag test re-run showing pooled-connection checkout non-stick; Phase A -count=10 determinism test run 2026-07-09)
  command_or_observation: panel findings adjudicated into this Definition; go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10
  result: all confirmed category-(a) findings fixed in-document; D-PROMO pinned-conn determinism test is Phase A work and has been independently reproduced
  uncertainty: none
```

## Requirement-Risk Handoff

The former escalation rules are retired. D-WIRE contradiction, D-PROMO
architecture changes, D-STORE contradiction, irreversible SQL drops, external
route/API removals, vocabulary cutovers, and red mutations without rollback are
requirement risks historically supplied to completed phases B/D/F. Any new
escalation, mutation, or sequencing requires a separately promoted Definition.

## Retained False-Completion Guards

These guards constrained completed B/D/F evidence; they do not define phases:

- TLC green → implementation isolates.
- adapter exists → promotion is Dolt-native.
- adoption, lineage, UI, Trace, or acceptance says promoted → a receipted route transition occurred.
- merge/tag exists → the ComputerVersion route slot changed.
- rollback resets lineage → receipted CAS to the prior ComputerVersion.
- resolver reads `route_profile` → route is over ComputerVersion.
- detector written → detector enforces.
- panel consensus → repository-verified truth.

## Historical Completion And Resumption Receipts

The former completion checklist, rollback/resumption policy, mutable
`status: working` card, next executable probe, mission report policy, and
suggested-goal section are retired historical evidence. They own no live
completion, checkpoint, resumption, phase-gate, or next-action authority.

Retained settled evidence:

- D-HISTORY native Texture history through bounded `AS OF` reads
  (`b7f512f2`).
- H011/H012 role-keyword oracle deletion and detector enforcement
  (`82839687`).
- H010 post-write email forcing/parser deletion (`fd492b91`).
- D-PROMO pinned-connection branch-isolation settlement.
- W1 detector manifest, W2 timeout hardening, W3 seam receipts, C1–C7
  corrections, and the P-TRIAGE historical table.

Retained rollback references:

- `a703bf44` — pre-mission documentation state.
- `f1e2d7a3` — pre-D-HISTORY behavior.
- `1870452c` — superseded eager-checkpoint implementation.
- `d6ce587d` — pre-M3.1a behavior.
- `73657a8f` — pre-M3.1b behavior.

The audited-construction successor completed on 2026-07-17 and is retained as
historical evidence. No product `/goal` is currently authorized.
