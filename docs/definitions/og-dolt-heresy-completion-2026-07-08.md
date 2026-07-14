# OG / Dolt / Heresy Completion Mission

## Subordinate Invocation Semantics

This document supplies storage, promotion, heresy-detector, and deletion
contracts to:

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-14.md
```

Do not invoke it independently while the active mission is `working`. Runtime,
Wire, lifecycle, acceptance, promotion, and vocabulary mutations execute only
through R1/R4/R5/R7. Historical conclusions and evidence remain load-bearing;
they do not create a second orchestrator or state capsule.

This document previously superseded the deleted 2026-07-07 program and heresy
paradocs. Their conclusions remain absorbed here as historical evidence. The
active mission now supersedes this document's independent execution authority.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-2026-07-14.md` for
   active execution order, evidence, checkpoints, and resumption.
2. This document's detector, deletion, and historical evidence contracts.
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

## Mission Purpose And Non-Purpose

**Purpose:** Finish what the og-dolt program and heresy-eradication mission
started — and retire the accumulated open edges of every prior mission — in an
order that makes false progress impossible: foundations and detectors first,
truth-corrections to docs and specs next, then the kill waves and cutovers,
then promotion-over-ComputerVersion, then deletion and doctrine replacement.

**Non-purpose:**

- Runtime business-logic extraction is now owned by
  `docs/definitions/choir-autoputer-completion-2026-07-14.md` R1. This
  Definition supplies subordinate detector/deletion contracts; it is not a
  competing `/goal` spine.
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
superseded_by: docs/definitions/choir-autoputer-completion-2026-07-13.md#two-store-topology
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
    - hard blocker in migration evidence (escalate, don't silently revert)
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
execution_effect:
  - Settled supported: Phase D rewrites the adapter to branch operations on the embedded store; the spec's BranchIsolation scope header names the embedded store; sql-server stays a wire-store-only concern.
  - The tag-based adapter remains interim and off until the branch rewrite lands; DOLT_RESET rollback stays forbidden (I4) for the tag-based path.
  - All adapter operations must use pinned connections; the store's single-writer discipline and connection pinning provide isolation.
settlement:
  rule: Settled by the Phase A pinned-connection experiment; result gates Phase D spec and adapter work.
  settled_by: evidence
```

### D-ROUTE. Boundary: one vmctl-owned ComputerVersion route writer — CAS SEMANTICS SETTLED / THIRD-STORE TOPOLOGY SUPERSEDED

```yaml
id: promotion-route-receipt
kind: boundary
status: settled_cas_semantics / superseded_third_store_persistence / violated_implementation
source: CAS/receipt semantics retained; orchestrator-settled third-store synthesis was unratified and is demoted by owner two-store authority
superseded_claim: A distinct Dolt-backed platform-control ledger is required.
superseded_by: docs/definitions/choir-autoputer-completion-2026-07-13.md#two-store-topology
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
construction:
  next_executable_slice:
    mutation_class: red
    objective: >-
      Install the truth gate before the route writer: define the pure
      RouteTransitionCommand/Receipt and required executor port; make promotion
      and rollback fail closed when no executor/receipt exists; gate adoption,
      lineage, UI, Trace, and run acceptance on a receipt; delete the duplicate
      roll-forward and candidate-intake mutation paths. With no executor wired,
      the product truthfully reports promotion unavailable.
    deletion_targets:
      - internal/runtime/app_promotion.go: RollForwardAppAdoption; a rolled-back version may be promoted again only by a fresh validated CAS command
      - internal/runtime/candidate_package_intake.go: SwitchCandidatePackageIntakeAdoptionReview, RollbackCandidatePackageIntakeAdoptionReview, and RollForwardCandidatePackageIntakeAdoptionReview, plus their mutating API routes/callers; retain read-only intake/review evidence
      - frontend/src/lib/FeaturesApp.svelte: canRollForward and status-only active/rollback affordances
    verifier: >-
      Inverted tests prove all deleted routes/callers are absent and prove that
      nil/error/stale receipt paths leave adoption, lineage, UI-visible state,
      Trace, and run acceptance unchanged.
  following_slice:
    - implement the ComputerVersion route-slot tables on the corpusd sql-server and vmctl-only CAS/read APIs, including restart recovery and idempotent receipt lookup
    - seed each existing slot through the writer with an explicit bootstrap receipt after materialization preflight
    - cut proxy routing to route-slot read -> ComputerVersion -> vmctl materializer, then delete lineage/hard-coded routing and PROXY_RUNTIME_DB_PATH
    - replace the tag-only adapter with D-PROMO branch preparation while keeping route activation exclusively in the ledger
formalization_effect:
  - Rewrite specs/promotion_protocol.tla before the Go writer so the only route variable maps a route slot to ComputerVersion plus generation; remove any authoritative computer/desktop route.
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
execution_effect: >-
  Phase D may use D-PROMO's settled branch mechanics, but it cannot claim
  promotion from merge/tag/adoption alone. Until the implementation settles,
  active and rollback-available product claims fail closed. The roll-forward
  and candidate-intake switch state machines are superseded future deletions,
  not compatibility paths to preserve.
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
superseded_by: docs/definitions/choir-autoputer-completion-2026-07-13.md#two-store-topology
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
execution_effect:
  - Phase B/C history-read work and Phase D promotion work execute against Dolt.
  - Phase D implements D-ROUTE as route-slot and receipt tables on the corpusd sql-server with vmctl as sole atomic CAS writer; it adds neither a third Dolt domain nor a promotion workspace.
  - Per-write commit/batching, rollback mechanics, AS OF/DOLT_LOG correctness and latency, throughput, ICU/cgo build friction, and replication/sync are engineering verification axes inside the relevant phases, not decision gates.
  - If evidence exposes an actual feasibility contradiction, document and escalate it; do not silently degrade or re-open the choice by implication.
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

## Determined State Snapshot (2026-07-08)

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
      source: owner two-store directive, applied by the active mission's two-store-topology node
      superseded_claim: one distinct durable Dolt-backed ComputerVersion route-control domain
      superseded_by: docs/definitions/choir-autoputer-completion-2026-07-13.md#two-store-topology
      execution_effect: >-
        R5 owns implementation. Receipt projection, fail-closed truth,
        idempotency, and sole-writer contracts remain subordinate inputs.
  settled_2026_07_08_owner:
    - claim: D-STORE is all-in on Dolt; native history/branch behavior becomes load-bearing. Storage inventory questions are engineering homework, not a renewed decision gate.
      source: owner authority, reaffirmed 2026-07-09
      execution_effect: Phase B/C/D proceed against Dolt; escalate only on demonstrated feasibility contradiction.
    - claim: Dolt persistence taxonomy — two product-state stores (world-wire sql-server and per-VM embedded stores) plus one narrow vmctl-owned ComputerVersion route-control ledger; promotion preparation is an operation on the embedded store and activation is a CAS in the ledger.
      source: user-stated product-store constraints + orchestrator-settled route-control consequence + observed
      execution_effect: D-WIRE, D-PROMO, D-STORE, and D-ROUTE authority are settled; promotion is decoupled from the wire store and VM ownership/desktop publication.
    - claim: Universal→World Wire rename will be executed in Phase E.
      source: user-stated
    - claim: Current wire-store data is junk (the wire loop has never worked end-to-end); the sql-server store stands up fresh with no data migration.
      source: user-stated
      execution_effect: D-WIRE cutover is code-only and cheap; it need not wait for Phase D if sequencing benefits from doing it earlier. It deletes PROXY_RUNTIME_DB_PATH and restores the world-wire boundary; D-ROUTE, not lineage, supplies honest route resolution.
  open: []
```

## Value Criterion

Every pass must reduce the mission variant (below) or buy decision evidence
for implementation conjectures. Prefer, in order: (1) work that makes future claims
falsifiable (detectors, conformance checks, relabeling), (2) work that
unblocks staging proof (timeouts), (3) deletions with inverted tests,
(4) cutover construction.

## Variant / Progress Measure

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

## Execution Phases

Receding-horizon: phases order pressure, not permission — safe in-bound work
from a later phase may run early only if it cannot create false progress
(i.e., detectors and labeling for it already exist). The original program's
error was the inverse; do not repeat it.

**Phases do not terminate execution.** A phase exit is not a checkpoint, not
a report-and-stop, not a request for approval. Each phase exit triggers the
Phase Gate Protocol below; when the gate clears, execution proceeds
immediately into the next phase within the same run. The run ends only at
the document's completion semantics or a genuine escalation/blocker per the
escalation rules.

## Phase Gate Protocol (agentic consensus between phases)

At each phase exit:

1. **Run agentic consensus** using `skills/agentic-consensus/` against the
   phase's claimed exit state: the phase's deliverables, evidence-ledger
   entries, and the diff/commits landed. Raw output is ephemeral; only the
   adjudicated conclusion belongs in a current authority document.
2. **Adjudicate** panel findings as `external second opinion` evidence:
   confirm each against the repo (the panel is not authority; grep/test/trace
   verification is). Sort confirmed findings into: (a) phase-exit defects
   (the phase's own bar not met), (b) new definition nodes (register, don't
   silently absorb), (c) out-of-scope noise (record and drop). The
   adjudication table (finding → category → one-line reasoning) MUST be
   committed to this Definition's evidence ledger before the gate can clear. The executing
   agent MUST NOT be the sole adjudicator for red-class gates; either the
   owner signs off on the table, or a non-implementing
   independent agent (not the consensus runner) verifies the table and the
   repo state. Unjustified `retired` triage dispositions and unjustified
   category-(c) reclassifications are themselves category-(a) defects for the
   next round.
3. **Iterate**: fix all category-(a) findings, update the definition graph
   and evidence ledger, then re-run the panel on the delta.
4. **Clear** means: a panel round produces zero confirmed category-(a)
   findings. Failed panelists (CLI errors, timeouts) don't block the gate if
   at least three independent panelists returned; note the failures. Use the
   same panel configuration across rounds of one phase (panelist churn looks
   like non-convergence). For red-class gates — determined by the protected
   surfaces the phase has touched (vmctl lifecycle, proxy request path, public
   API routes, promotion/rollback, SQL drops), not by phase letter — require at
   least four returned panelists, retrying failed ones once.
5. **Proceed** into the next phase in the same run, updating the Run
   Checkpoint section in passing. For green/yellow gates proceed on clear. For
   red-class gates proceed only after the adjudication table is approved by the
   owner or a non-implementing independent agent, the required panel count has
   returned (four), and any escalation rule has been resolved. Phase A has red
   work (W2, D-PROMO settlement), so its gate is red-class even though its
   yellow/green doc work can run in parallel. Do not stop, summarize-and-exit,
   or await owner input unless an escalation rule fires or a decision node
   blocks the specific next phase.

If three consecutive panel rounds on the same phase fail to converge (new
category-(a) findings each round), that is evidence of an unsettled
definition, not reviewer noise: open the definition node and, if it is
group-level, escalate with the `human_escalation` shape.

### Phase A — Foundations and truth (execute first, parallel-safe for green/yellow work)

Phase A contains both yellow doc corrections and orange/red runtime work (W2
proxy/vmctl timeout hardening touches the public request path; D-PROMO settlement
tests promotion mechanics). The red-class parts of Phase A must follow the
red-gate adjudication rules in the Phase Gate Protocol, not the default
yellow/green auto-proceed rule.

- **W1** Detector manifest + CI discovery job: verify the existing
  `docs/heresy-detectors.md` manifest has correct H030/H031 rows and refine
  allow-contexts as needed; create `scripts/check-heresies.sh` mapping H001–H031
  families to discovery-mode patterns, wire a CI job reporting counts without
  failing, and commit baseline counts as evidence. Bind the existing H031
  route-over-VM banned pattern and H030 actor-runtime-polling registry row into
  CI discovery, add a detector for `DOLT_RESET --hard` in production (non-test)
  paths (I4 guard), and mark the trivial H030 registry closure (repaired
  2026-06-27) as closed. Promote families to fail-on-regression as their
  clusters are eliminated.
- **W2** Timeout hardening: bounded vmctl resolve timeout (30–60s),
  `http.Server` Read/WriteTimeouts in `internal/server/server.go`, fast 504,
  reconcile the 10s retry window. Staging proof: `/api/universal-wire/stories`
  under induced resolve failure returns 504 fast.
- **W3** Landing-loop evidence for e393eb5c / e5c1d38a: record CI status,
  deployed identity, whether staging uses lineage resolver or fallback,
  whether any flow has the promotion adapter configured. Enter results in the
  evidence ledger.
- **C1–C7** Doc truth corrections (yellow, one pass):
  - C1 `current-architecture.md` — verify the capsule/substrate section clearly
    separates "partially implemented but inert" capsule substrate from any claim that
    promotion-bearing capsule transactions are proven.
  - C2 candidate-verb gate — its settled ComputerVersion/capsule semantics are
    absorbed into `computer-ontology.md`; the superseded design was removed.
  - C3 `choir-doctrine.md` — verify H031 heresy entry is complete (it already
    exists) and Banned Patterns list #16 is present; ensure detector refs point
    to `docs/heresy-detectors.md` H030/H031 rows and W1's CI job. Do not
    duplicate the heresy entry.
  - C4 Historical substrate and cross-substrate sources were relabeled
    `checkpoint_incomplete`, then removed from the live worktree.
  - C5 (FIRST Phase A commit — now landed in the green docs alignment pass;
    verify-and-close) — supersession made machine-readable, not just prose:
    pointer notes in historical source in Git history
    (plus its post-commit state note: tag adapter is an embedded-mode interim
    hook; freezes but does not settle H031) and
    historical source in Git history; `docs/mission-graph.yaml`
    nodes for both absorbed docs with `status: superseded` pointing at this
    node; all three documents registered in `docs/doc-authority-manifest.yaml`
    with correct roles/witnesses.
  - C6 Docs index / authority manifest — grip checkpoint is narrative layer.
  - C7 `README.md` — lead with "human-improving, machine-compounding
    mainframe".
- **D-PROMO settlement test** (pulled forward from Phase D — cheap): the
  pinned-connection isolation test with `-count=10` determinism bar, per the
  D-PROMO node; on pass, correct the adapter comment's 2026-07-07 conclusion.
- **S1** Spec↔adapter reconciliation: condition `BranchIsolation` (and
  related invariants) on sql-server mode in `specs/promotion_protocol.tla`
  with an explicit scope header, or record the decision to hold the spec as
  target-state with a conformance gap note; **W6** add the conformance
  binding (test or check) so "spec implemented" cannot be declared without
  isolation verified against concurrent writers.
- **P-TRIAGE** Past-mission open-edge triage: for each of the ~25 open edges
  in the ledger sweep (mission-3c APIHandler extraction, texture hard-cutover
  C43, transclusion cutover, long-running-agent R1–R7, durable-thread link
  route, product-loop failure path, coagent source-centric follow-ups,
  wire-agent-pipeline staging proof, stabilization substrate boot,
  lifecycle-cutover residues, conductor-URL H029 repair,
  doc-truth-drift checker review, node-B fail-closed auth,
  deferred-reliability sandbox→computer rename / SQLite cleanup / node-B
  retention, news-live landing, orchestrator C15/M9/M10, autoradio verifier
  review, substrate-hardening MPCal TLC + cmd dedup, cross-substrate
  extractor, wire-on-settlement, continuation-deletion sequencing): assign
  each a disposition — `absorbed:<phase>` (it is this mission's work),
  `retired` (no longer real, with reason), or `external:<successor>` (own
  mission, with pointer). Record the triage table in this document. No edge
  may remain untriaged.

#### P-TRIAGE — past-mission open-edge triage table

| Open edge | Disposition | Reason / pointer |
|---|---|---|
| actor/runtime business-logic extraction | absorbed: R1 | `choir-autoputer-completion-2026-07-14.md` owns live caller cutovers, ratchets, and final directory extinction. |
| Texture product-loop, transclusion, and durable-thread gaps | subordinate evidence for R1/R4 | Resume only through the active mission from current code and staging evidence. |
| source-system follow-ups | external dependency | Requires a fresh Definition if the active mission does not absorb it. |
| Wire staging and substrate proof | settled predecessor receipt | D-WIRE conformance is imported without rerunning it. |
| lifecycle-cutover residues (texture forcing / parent-child) | absorbed: R1 | This Definition's H009-H012 and H001-H005 detector contracts remain the deletion bar. |
| lifecycle-cutover residues (continuations / acceptance) | split: R1/R4 | R1 deletes continuation/parent-child residue; R4 establishes the surviving acceptance authority. |
| conductor-URL H029 repair | absorbed: Phase E | og-dolt Phase E surface cleanup (H019-H029). |
| docs truth drift | external: documentation authority Definition | Governed by `documentation-authority-reduction-2026-07-09.md`. |
| node-B fail-closed auth | external dependency | Platform/auth operations are not og-dolt work. |
| sandbox→computer rename | absorbed: Phase E | og-dolt Phase E supplies detectors; active mission R7 owns the cutover. |
| SQLite/sourcecycled cleanup | external dependency | Object-graph consolidation remains outside og-dolt scope; no live successor Definition owns it. |
| node-B retention, news, orchestrator, autoradio, and cross-substrate gaps | external dependencies | Their checkpoint chains were removed; any resumed work requires fresh Definitions from current evidence. |
| wire-on-settlement | external dependency | Route-switch evidence gate is not og-dolt work. |
| continuation-deletion sequencing | absorbed: R1 | H006-H008 deletion executes inside runtime dissolution after zero production callers are verified. |

Phase A exit bar (what the gate panel reviews): detectors reporting in CI;
timeouts proven in staging; all corrections committed; S1 settled or
explicitly scoped; triage table full. Then the Phase Gate Protocol runs and,
on clear, execution continues directly into Phase B.

Active-mission ownership update (2026-07-13): Phases B/C/E remain authoritative
detector and deletion specifications, but overlapping runtime, continuation,
parent-child, acceptance, promotion, and vocabulary mutations execute only as
R1/R4/R5/R7 work. A direct `/goal` on this document must not race or duplicate
those cutovers.

### Phase B — Heresy kill wave 1 (M3.1, M3.2) + Dolt audit reads

As specified in mission-og-dolt Phase 1 (imported): texture-forcing removal
(H009–H012, H024a/b, H026; proof gate: honest first revisions, unforced
delegation — heed the mutually-gated transition: verify agent behavior before
deleting forcing cues); parent/child deletion (H001–H005, H015–H016, H005
first; proof gate: all authority trajectory-scoped); Dolt audit read-path
(`texture history` from `dolt_history_<table>` + `AS OF`) — this is the first
load-bearing D-STORE verification (history latency, latest-revision cost).
Detector families for each cluster flip to fail-on-regression as they close.

Phase B exit bar: M3.1 and M3.2 clusters at the `eliminated` bar (deletion
diff + inverted tests + detector at fail-on-regression, zero live sites for
H009–H012, H024a/b, H026, H001–H005, H015–H016); both proof gates evidenced;
`texture history` served from `dolt_history` with latency numbers recorded as
Dolt implementation evidence.

### Phase C — Kill wave 2 + cold-entity cutover

Imported from mission-og-dolt Phase 2: acceptance/durable obligations
(H013–H014, H017–H018); continuation deletion (H006–H008; gate: verified
zero production callers) before store entities migrate; cold-entity OG
cutover (runs → trajectories/work items → acceptances → texture →
run memory) with per-entity dual-write flip and SQL fallback window. The
batch-commit infrastructure (write-batcher: N mutations or T ms → one
commit, agent-identity commit messages — mission-og-dolt Phase 0 item) is a
prerequisite of this phase's cutovers; build it here if Phase A didn't.

Phase C exit bar: H013–H014, H017–H018 and H006–H008 at the `eliminated`
bar; continuations code and routes deleted with zero production callers
verified; every cold entity reading from OG by default in production with
SQL fallback exercised; detector families for these clusters enforcing.

### Phase D — Historical promotion contract, consumed only by R5

Do not execute this phase independently. R5 consumes its retained fail-closed
truth, sole-writer CAS, immutable receipt, idempotency, failure, and projection
contracts under the owner-settled topology: route-slot/receipt tables on the
corpusd sql-server with vmctl as sole writer. The former distinct Dolt-backed
third-ledger implementation is superseded.

R5 deletes duplicate roll-forward and candidate switch/rollback mutation paths
with their callers; rewrites `promotion_protocol.tla` around one route-slot →
ComputerVersion CAS plus immutable receipt; implements vmctl-only read/write
APIs; cuts proxy routing to route-slot read → ComputerVersion → materializer;
and proves an old → new → old sequence with explicit receipts and generations.
Branch merge/tag remains preparation, never activation.

The retained exit contract requires concurrency, idempotency, injected-failure,
restart, failed-writer/materializer negative projection, and zero-caller
evidence for deleted mutation surfaces. Only the active mission capsule may
record R5 complete.

### Phase E — Historical cleanup contract, consumed only by R7

Do not execute this phase independently. R7 consumes its detector and
surface-deletion contracts for H019-H029, Universal→World Wire, and
sandbox→autoputer vocabulary. Any non-rename deletion still owned by R1 must
already be complete. R7 owns atomic iteration ordering, staging proof,
independent review, and completion state; compatibility aliases are forbidden.

## Dense Feedback Channels

- CI on every push (detector counts visible per family).
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

## Authority Boundaries / Escalation Rules

Escalate to owner only for: a hard blocker in the wire-store sql-server
migration that would reopen D-WIRE; a D-PROMO answer forcing a group-level
architecture change; D-STORE contradiction of
the standing Dolt decision; SQL table drops (irreversible); external contract
changes (route/API removals beyond registered heresy inventories);
Universal→World Wire rename execution; any red mutation without an accepted
rollback path. Everything else resolves through the critical process inside
the boundaries above. Escalations use the skill's `human_escalation` shape.

## Forbidden Collapses (mission-specific, atop the skill's list)

- seam merged → phase landed.
- TLC green → implementation isolates.
- adapter exists → promotion is Dolt-native.
- adoption or desktop publication exists → a ComputerVersion is active.
- lineage/status/UI/Trace/acceptance says promoted → a route transition occurred.
- merge/tag exists → the ComputerVersion route ledger changed.
- rollback means resetting or rewriting lineage → receipted CAS to the prior ComputerVersion.
- resolver reads route_profile → route is over ComputerVersion.
- ledger says complete → mission complete (check remaining_error_field).
- detector written → detector enforces (discovery mode is not enforcement).
- triage table row filled → edge closed (absorbed edges must still execute).
- phase gate cleared → run may stop (the gate authorizes continuation, never exit).
- panel consensus → truth (panel findings must be repo-verified before acting).

## Completion Semantics

Status: `working | complete | checkpoint_incomplete | blocked_incomplete | superseded`.

`complete` requires all of (carrying forward mission-og-dolt lines 211–226,
plus this mission's additions):

1. Heresy detector CI at fail-on-regression for every family; registry
   entries closed with detector references.
2. `internal/store` SQL tables dropped; OG the only durable model;
   `go test ./...` green without dual-path code.
3. `choir texture history` served from `dolt_history`; at least one promotion
   executed as an atomic route flip between ComputerVersions with a
   durable vmctl route-ledger receipt/generation and a demonstrated receipted
   rollback flip through the same CAS writer, using the D-PROMO-settled
   preparation mechanism
   — pinned-connection branches if supported, or the escalated-and-accepted
   fallback (serialized single writer / separate database per candidate) if
   D-PROMO falsifies — with the spec↔implementation conformance binding
   green. Negative deployed proof must show failed route persistence or
   materialization produces no adoption/lineage/UI/Trace/acceptance success
   projection. Criterion 3 is satisfiable under EITHER D-PROMO outcome; a
   falsified D-PROMO changes preparation, not route authority or the criterion.
4. No route resolves to a VM identity (I1 observables gone); vmctl
   candidate-desktop lifecycle and the duplicate roll-forward/candidate-intake
   route-switch paths deleted; ordinary routing reads the receipted
   ComputerVersion ledger before materialization.
5. choir-doctrine.md reduced to thesis + invariants + enforcement pointers;
   no live heresy entries.
6. `choir` CLI `trajectory`/`texture` verbs read identical shapes before and
   after, verified against production — evidence artifact: a recorded
   before/after CLI output diff against production, summarized in this
   Definition's evidence ledger.
7. Request path bounded (I3) with staging proof.
8. All C1–C7 corrections landed; no mission document mislabeled complete.
9. Past-mission triage table complete with every `absorbed` edge executed
   and every `retired`/`external` edge annotated.

Before any non-complete exit, verify no safe executable probe remains in
bounds. A checkpoint is not completion.

## Rollback And Resumption Policy

Every red pass names its rollback ref before mutation. Entity cutovers keep
SQL fallback for a declared window. Promotion-path work is inert-by-default
until its gate settles (never enabled speculatively). D-ROUTE rollout seeds
each old version through the same writer with a bootstrap receipt; after a slot
becomes ledger-authoritative it has no lineage/desktop/hard-coded fallback, and
operational rollback is a new receipted CAS to the recorded prior
ComputerVersion. A code rollback returns promotion to disabled/fail-closed
operation and may not restore the deleted false-success paths. Doc corrections
are git-revertable individually. This document's Run Checkpoint section is the
resumption state; update it every pass.

## Mission Report Policy

Maintain an owner-readable report section (or companion report doc) once
red passes begin: what shipped, what was proven vs attempted, invariants
preserved/violated, rollback refs, next probe. Link evidence; do not dump
logs.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: D-HISTORY, M3.1a, and M3.1b settled on staging
  current_artifact_state: >-
    Phase A deliverables committed and exit gate cleared: W1 detector manifest +
    CI discovery job (including the I4 destructive-rollback guard), W2 proxy/vmctl
    timeout hardening with staging 504 proof, W3 seam-commit landing-loop
    evidence, C1–C7 doc truth corrections, D-PROMO pinned-connection
    branch-isolation settlement, S1 spec↔adapter scope/conformance note, and
    P-TRIAGE past-mission open-edge table. The Phase A exit panel adjudication is
    committed. D-STORE, D-PROMO, D-WIRE, and D-ROUTE semantic authority are
    settled. D-ROUTE implementation remains violated: the source audit found
    no durable route ledger and multiple status/lineage/desktop/event paths
    claiming activation. Phase B inspection
    found that the public Texture history route still walked the application
    revision chain and that normal object-graph writes created no explicit Dolt
    commits. The local implementation now checkpoints canonical Texture writes
    and reads committed snapshots through dolt_history + AS OF. The eager
    per-write commit tactic caused a CI performance contradiction and was
    replaced by a dirty-batch history-read barrier. Fresh CI, Node B identity,
    and authenticated staging create/revise/history proof are green. M3.1a
    deletes the production H011/H012 substring oracles, preserves structured
    intent/agentic affordances, and promotes that detector family to deployed
    zero enforcement. CI, Node B identity, and a real-passkey narrative-word
    Texture revision are green. M3.1b also removes the post-write email prose
    parser/direct executor while retaining the typed Email appagent handoff;
    its full red landing loop and inert-email-prose staging revision are green.
    This docs-only authority pass assigns a distinct Dolt-backed
    ComputerVersion route ledger to vmctl's sole CAS writer and makes every
    adoption/lineage/UI/Trace/acceptance claim a receipt projection; it changes
    no runtime behavior and claims no implementation repair.
  what_shipped:
    - W1 detector manifest + CI discovery job (scripts/check-heresies.sh, docs/heresy-detectors.md H030/H031/I4 refs, CI heresy-detector job)
    - W2 proxy/vmctl timeout hardening (60s default, fast 504 staging proof)
    - W3 seam-commit evidence for e393eb5c/e5c1d38a
    - C1–C7 doc truth corrections
    - D-PROMO pinned-connection branch-isolation settlement test
    - S1 promotion_protocol.tla scope and conformance note
    - P-TRIAGE past-mission open-edge triage table
    - D-HISTORY dirty-batch native Texture history (b7f512f2)
    - M3.1a H011/H012 role-keyword oracle deletion and detector enforcement (82839687)
    - M3.1b H010 post-write email forcing/parser deletion (fd492b91)
  what_was_proven:
    - seam status of the two Red commits (observed, grep-verified)
    - timeout invariant violation (observed) and fix (staging 504)
    - lineage resolver not active in staging; promotion adapter not wired in production
    - embedded Dolt branch isolation on a pinned connection is deterministic (D-PROMO -count=10)
    - all past-mission open edges triaged (absorbed/external/retired)
    - native Texture history reads committed dolt_history snapshots through bounded AS OF queries; staging history returned the exact two-revision parent chain in 29.8ms
    - H011/H012 production branches are absent and zero-enforced; staging Texture loop d562f055-b21a-4678-93f4-79cabcb11796 accepted narrative role words and produced appagent revision 62dc25fd-37d2-4569-924b-a6f004a3a979
    - Texture writes no longer parse prose or directly create Email drafts; the typed request_email_draft boundary remains green and staging loop 8c6cb802-84b3-4684-9a8f-b0e9ab10c405 produced revision 5c2e7279-b331-47cd-b1fc-ca2d2cf5175b
  unproven_or_partial_claims:
    - Dolt engineering verification axes: history latency/correctness,
      batching/throughput, rollback recovery, build friction, and replication
    - heresy live-site counts (families still in discovery; fail-on-regression and allowlist enforcement deferred per phase)
    - D-ROUTE ledger, receipt projection gates, duplicate-path deletions, TLA rewrite, proxy cutover, and deployed old-new-old proof
    - Phase B–E kill waves, cutovers, and deletion not yet executed
  remaining_error_field: see Variant below
  highest_impact_remaining_uncertainty: remaining H009/H010 forcing sites + M3.2 authority residues
  next_executable_probe: >-
    Return control to R1. Reconcile remaining H009/H010 and M3.2 detector hits
    against current production callers before any deletion.
  queued_phase_d_executable_slice: >-
    R5 consumes D-ROUTE's fail-closed truth gate and sole-writer contract.
    It must prevent adoption, lineage, UI, Trace, and run acceptance from
    advancing without durable receipt read-back and must delete duplicate
    roll-forward/switch/rollback mutators atomically.
  suggested_goal_string: "/goal docs/definitions/choir-autoputer-completion-2026-07-14.md"
  evidence_artifact_refs:
    - this Definition's adjudicated evidence ledger
    - https://github.com/choir-hip/go-choir/actions/runs/29072918594
    - https://github.com/choir-hip/go-choir/actions/runs/29074494439
    - https://github.com/choir-hip/go-choir/actions/runs/29075852884
  rollback_refs:
    - a703bf44 (pre-mission docs state)
    - f1e2d7a3 (pre-D-HISTORY behavior state)
    - 1870452c (superseded eager-checkpoint implementation)
    - d6ce587d (pre-M3.1a behavior state)
    - 73657a8f (pre-M3.1b behavior state)
```

## Suggested Goal String

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-14.md
```
