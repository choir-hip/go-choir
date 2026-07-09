# OG / Dolt / Heresy Completion Mission

## Harness Invocation Semantics

```text
/goal docs/definitions/og-dolt-heresy-completion-2026-07-08.md
```

Read this document as executable semantic authority. Execute autonomously until
its completion semantics are satisfied with named evidence, or until a sharply
evidenced escalation, blocker, or supersession condition is met. Do not treat
this document as a plan, checklist, or summary. Its definitions govern what
"done," "seam," "repaired," "promoted," and "complete" are allowed to mean.

This document **supersedes as executable authority**:

- `docs/archive/mission-og-dolt-heresy-hard-cutover-v0.md` (the 2026-07-07 program
  paradoc) — its phases were partially executed and its sequencing was violated
  in practice (Phase 4 seams landed before Phase 0 foundations); this document
  absorbs its remaining work and corrects the sequencing.
- `docs/archive/heresy-eradication-2026-07-07.md` — its definition graph
  (heresy, eliminated, detector, registry-close semantics) is imported by
  reference; its execution state is absorbed here.

Both documents remain valid as **source material and per-heresy authority**;
neither remains an independent execution target. A future `/goal` against
either should redirect here.

## Source Authority Order

1. This document (definition graph + determined state + completion semantics).
2. Owner statements 2026-07-07/08: object graph becomes canonical by hard
   cutover; Dolt version-control features become load-bearing; all named
   heresies eliminated with executable enforcement; candidate computers are
   capsules over substrate-independent audited computers, not VMs; **one
   comprehensive mission encompasses the incomplete og-dolt and
   heresy-eradication runs plus all cleanup/completion debt from past
   missions** (2026-07-08).
3. `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
   (ComputerVersion, materializer, route-over-computer-version).
4. `docs/choir-doctrine.md` heresy registry (H001–H031) — per-heresy authority
   for bad pattern and blessed replacement.
5. `docs/archive/heresy-eradication-2026-07-07.md` — imported definition
   graph for `heresy`, `eliminated`, detector semantics.
6. `docs/archive/mission-og-dolt-heresy-hard-cutover-v0.md` — phase inventories,
   deletion inventories, completion criteria (imported, resequenced here).
7. `docs/assessment-overall-state-2026-07-07.md` — evidence baseline
   (completeness percentages, timeout diagnosis, storage-fork analysis).
8. Agentic-consensus panel reviews 2026-07-08
   (`docs/evidence/agentic-consensus-2026-07-08-docs-review/`,
   `docs/evidence/agentic-consensus-2026-07-08-docs-review-2/`,
   `docs/evidence/agentic-consensus-2026-07-08-mission-readiness/`) — reviewer
   evidence class; findings adjudicated into this document, not authority on their
   own.
9. `AGENTS.md` (repo operating contract, mutation ceremony, Landing Loop).

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

- Not a rewrite of `internal/runtime`; business-logic extraction remains its
  own mission (mission-3c_2 Phase 2.5) — this mission deletes dual paths
  inside what exists and records that extraction as an open dependency.
- Not the grip/RL research program (`choir-grip-checkpoint-2026-07-07.md` is
  narrative authority only; its research forks are out of scope).
- Not new product surface (headless CLI Phase 1.5 verbs, MCP, reader UX
  options B/C stay deferred unless a node here requires them).
- Not detector theater: a detector that cannot fail is not evidence.
- Not motion theater: a pass that changes no node status and no verifier is
  not progress.

## Definition Graph

Imported nodes: `heresy`, `eliminated`, detector semantics, registry-close
semantics from `docs/archive/heresy-eradication-2026-07-07.md` — status carried as
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
  - docs/missions/substrate-hardening-v0.md (MPCal TLC unverified, embed refactor deferred).
  - docs/missions/cross-substrate-proof-v0.md (gates 4/5 listed as unproven while checkpoint text claims them satisfied).
execution_effect:
  - Work item C4 must relabel these documents; no downstream mission may cite them as complete.
```

### I1. Invariant: `route-over-computer-version` (H031 bar)

```yaml
id: route-over-computer-version
kind: invariant
status: settled (definition) / violated (implementation)
source: docs/definitions/substrate-independent-audited-computer-2026-07-04.md; choir-doctrine.md H031
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
    the resolver's product-level routing table resolves through `ComputerVersion`
    records, and the detector for the banned pattern is green.
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
  - The spec rewrite must model merge and tag as SEPARATE steps (Dolt docs: DOLT_MERGE implicitly commits the transaction; merge+tag cannot be one SQL transaction). Promotion atomicity is a route-flip property; the ledger operations are idempotent/resumable steps with a recovery path between them.
```

### I3. Invariant: bounded request path

```yaml
id: bounded-request-path
kind: invariant
status: settled (definition and implementation)
source: docs/assessment-overall-state-2026-07-07.md (historical staging trace: api.resolve max 180,029ms, 23 errors); docs/evidence/w2-timeout-staging-proof-2026-07-09.md (post-fix: api.resolve max 60,001ms, 504 within 60s)
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
definition: choir-grip-checkpoint-2026-07-07.md and other narrative/philosophy documents cannot override doctrine, definitions, specs, or evidence.
execution_effect:
  - Work item C6 records this in the docs index; agents must not cite grip narrative as execution authority.
```

### D-STORES. Term: the two Dolt stores — SETTLED (owner + observed, 2026-07-08)

```yaml
id: dolt-store-taxonomy
kind: term
status: settled
source: user-stated (owner, 2026-07-08) + observed (code)
term: Dolt store taxonomy
definition: >-
  The tree has two Dolt stores that must never be conflated:
  (1) the WORLD-WIRE STORE — the platform ObjectGraphStore
  (internal/platform/objectgraph_store.go, served by corpusd; HTTP access
  via internal/objectgraph/http_store.go), ill-named "platform Dolt"; it
  serves the world-wire system only. (2) VM-LOCAL EMBEDDED STORES — one
  embedded Dolt per user VM (internal/objectgraph/dolt_store.go: DoltStore
  is the VM-LOCAL store, not the platform one), shared by all capsules
  running in that VM.
  PROMOTION IS AN OPERATION, NOT A STORE: ComputerVersion
  fork/promote/rollback executes against the VM's embedded store
  (DoltPromotionAdapter.WorkspacePath is the filesystem path to that
  embedded workspace). Promotion is NOT a property of the world-wire store,
  and there are no separate per-app promotion workspaces.
forbidden_collapses:
  - wire store -> promotion substrate
  - promotion -> its own store
  - sql-server decision for the wire store -> promotion mechanics decided
execution_effect:
  - Every spec, doc, and work item in this mission must name which store it means; "platform Dolt" without qualification is vocabulary drift (candidate rename in Phase E alongside World Wire).
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
  - Unblocks the lineage route resolver reading live wire state without PROXY_RUNTIME_DB_PATH file-sharing hacks. Docs research 2026-07-08 confirms this hack is structurally impossible anyway — embedded mode holds an exclusive directory lock, so proxy and runtime can never share the embedded store across processes; sql-server is the only multi-process topology.
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

### D-STORE. Decision node: storage fork

```yaml
id: storage-fork
kind: term
status: unresolved (requires_human_authority)
source: docs/assessment-overall-state-2026-07-07.md lines 90-119
definition: Commit to Dolt version-control features as load-bearing (per owner 2026-07-07 direction) vs acknowledge an application-level audit trail. Owner direction says Dolt; the six open storage-inventory questions (starting with per-write commit semantics and rollback mechanics) remain unanswered.
execution_effect:
  - Phase C history-read work and Phase D promotion work execute against the Dolt answer; if the experiment evidence contradicts feasibility, escalate rather than silently degrade.
settlement:
  rule: Answer the six storage-inventory questions with experiments; escalate only if evidence contradicts the standing owner decision.
  settled_by: evidence, escalating to human on contradiction
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
    - claim: W3 landing-loop evidence for e393eb5c/e5c1d38a is recorded: both commits are in main history, their own CI runs were cancelled/failed, the deployed SHA is `1ed41f2b`, the lineage resolver is not active in staging (no `PROXY_RUNTIME_DB_PATH`), and no production binary configures the promotion adapter.
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
      source: docs/assessment-overall-state-2026-07-07.md
      execution_effect: variant baseline below.
    - claim: H030 (mailbox polling) repaired 2026-06-27; registry update only.
      source: settled-definition (heresy-eradication doc)
    - claim: Retrieval search returns zero results for terms that exist; /api/trajectories ignores ?limit=.
      source: observed (assessment)
      execution_effect: C-RETR and C-PAGE work items exist in Phase E.
  settled_2026_07_08_owner:
    - claim: Two-store Dolt taxonomy — world-wire store (moves to sql-server now) and per-VM embedded stores shared by that VM's capsules; promotion is an operation on the embedded store.
      source: user-stated + observed
      execution_effect: D-WIRE settled; D-PROMO settled (pinned-connection branch isolation); promotion explicitly decoupled from the wire store; S1 scope header names the embedded store.
    - claim: Universal→World Wire rename will be executed in Phase E.
      source: user-stated
    - claim: Current wire-store data is junk (the wire loop has never worked end-to-end); the sql-server store stands up fresh with no data migration.
      source: user-stated
      execution_effect: D-WIRE cutover is code-only and cheap; it need not wait for Phase D if sequencing benefits from doing it earlier (it deletes PROXY_RUNTIME_DB_PATH and unblocks honest route resolution).
  open:
    - node: storage-fork (D-STORE)
      missing: answers to six storage-inventory questions.
```

## Value Criterion

Every pass must reduce the mission variant (below) or buy decision evidence
for D-PROMO / D-STORE. Prefer, in order: (1) work that makes future claims
falsifiable (detectors, conformance checks, relabeling), (2) work that
unblocks staging proof (timeouts), (3) deletions with inverted tests,
(4) cutover construction.

## Variant / Progress Measure

Baseline 2026-07-08. Productive execution reduces these counts:

```yaml
variant:
  heresy_families_without_ci_detector: 0         # 12 aggregate detector families (H001-H031 + I4) are wired to CI discovery via docs/heresy-detectors.md and scripts/check-heresies.sh; target 0
  heresy_families_without_ci_enforcement: 12      # fail-on-regression and allowlist contexts are deferred per phase; target 0
  heresy_families_live: 9                        # live-site clusters, target 0: texture forcing (H009-12/H024a,b/H026), parent/child (H001-05 + H015-16), continuations (H006-08), acceptance/obligations (H013-14/H017-18), surface residue (H019-23), vocabulary (H025/H027-29), candidate-VM (H031+new), route-over-CV violation, dual-store SQL paths
  doc_corrections_open: 0                        # C1–C7 committed, target 0
  spec_impl_gaps_open: 0                         # S1 settled with scope/conformance note, target 0
  unbounded_request_paths: 0                     # W2 committed and staging-proven, target 0
  seam_commits_unlabeled: 0                      # e393eb5c, e5c1d38a evidence recorded in W3
  mislabeled_complete_missions: 0                # substrate-hardening, cross-substrate-proof relabeled in C4
  past_mission_open_edges_untriaged: 0           # P-TRIAGE table committed below, target 0
  decision_nodes_unresolved: 1                   # D-STORE storage fork remains unresolved; D-PROMO and D-WIRE settled 2026-07-08
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
   entries, and the diff/commits landed. Output directory:
   `/tmp/agentic-consensus-<date>-phase-<X>/`, preserved into
   `docs/evidence/` on gate close.
2. **Adjudicate** panel findings as `external second opinion` evidence:
   confirm each against the repo (the panel is not authority; grep/test/trace
   verification is). Sort confirmed findings into: (a) phase-exit defects
   (the phase's own bar not met), (b) new definition nodes (register, don't
   silently absorb), (c) out-of-scope noise (record and drop). The
   adjudication table (finding → category → one-line reasoning) MUST be
   committed to `docs/evidence/` before the gate can clear. The executing
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
   (D-PROMO, D-STORE) blocks the specific next phase.

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
    separates "designed, not built" capsule substrate from any claim that
    promotion-bearing capsule transactions are proven.
  - C2 `design-choir-headless-surface-v0.md` — strengthen the candidate-verb
    gate (spec models ComputerVersion/capsule semantics + route-over-CV
    load-bearing + atomic-or-degraded promotion + staging proof).
  - C3 `choir-doctrine.md` — verify H031 heresy entry is complete (it already
    exists) and Banned Patterns list #16 is present; ensure detector refs point
    to `docs/heresy-detectors.md` H030/H031 rows and W1's CI job. Do not
    duplicate the heresy entry.
  - C4 Relabel `missions/substrate-hardening-v0.md` and
    `missions/cross-substrate-proof-v0.md` to `checkpoint_incomplete`.
  - C5 (FIRST Phase A commit — now landed in the green docs alignment pass;
    verify-and-close) — supersession made machine-readable, not just prose:
    pointer notes in `docs/archive/mission-og-dolt-heresy-hard-cutover-v0.md`
    (plus its post-commit state note: tag adapter is an embedded-mode interim
    hook; freezes but does not settle H031) and
    `docs/archive/heresy-eradication-2026-07-07.md`; `docs/mission-graph.yaml`
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
| mission-3c APIHandler extraction | external: `docs/mission-3c_2-actor-runtime-migration-real-v0.md` | Actor/runtime extraction is outside og-dolt scope (mission-3c_2). |
| texture hard-cutover C43 | external: `texture-product-loop-recovery-v0` | `texture-hard-cutover-v0` superseded; C43 folded into active product-loop recovery. |
| transclusion cutover | external: `texture-structured-document-transclusion-cutover-v0` | Active Texture successor mission; not og-dolt. |
| long-running-agent R1–R7 | retired | `texture-long-running-agent-v0` superseded; R1–R7 folded into `texture-durable-thread-v1` and `texture-product-loop-recovery-v0`. |
| durable-thread link route | external: `texture-durable-thread-v1` | Active successor mission; not og-dolt. |
| product-loop failure path | external: `texture-product-loop-recovery-v0` | Active product-loop mission; not og-dolt. |
| coagent source-centric follow-ups | external: `source-system-loop8-simplify-v0` | `update-coagent-source-centric-deletion-v0` settled; remaining VText/source follow-ups live in the active source-system loop. |
| wire-agent-pipeline staging proof | external: `universal-wire-stabilization-v1` | Active successor to `universal-wire-agent-pipeline-v1`; staging proof belongs there. |
| stabilization substrate boot | external: `universal-wire-stabilization-v1` | Active stabilization mission; not og-dolt. |
| lifecycle-cutover residues (texture forcing / parent/child) | absorbed: Phase B | og-dolt Phase B heresy kill wave 1 (M3.1 texture forcing, M3.2 parent/child). |
| lifecycle-cutover residues (continuations / acceptance) | absorbed: Phase C | og-dolt Phase C heresy kill wave 2 (M4 continuation deletion, M3.3 acceptance). |
| conductor-URL H029 repair | absorbed: Phase E | og-dolt Phase E M5 surface cleanup (H019–H029); source-intake routing overlap remains in `conductor-url-source-routing-h029-v0`. |
| doc-truth-drift checker review | external: `docs-truth-system-v1` | `doc-truth-drift-context-v0` superseded; active successor is docs-truth-system-v1. |
| node-B fail-closed auth | external: `overnight-autoradio-platform-checklist-v0` | Platform/auth ops checklist; not og-dolt. |
| sandbox→computer rename | absorbed: Phase E | og-dolt Phase E surface cleanup / rename machinery. |
| SQLite cleanup | external: `docs/mission-unified-object-graph-v0.md` | Object graph consolidation / sourcecycled SQLite sidecar removal; not og-dolt. |
| node-B retention | external: `node-b-storage-retention-v0` | `node-b-nix-store-retention-v0` settled; vm-state/recovery budget remains in `node-b-storage-retention-v0`. |
| news-live landing | external: `news-live-pr-merge-model-default-v0` | Active news-live mission; not og-dolt. |
| orchestrator C15/M9/M10 | external: `orchestrator-suite-2026-06-28`; `docs-revision-v1`; `campaign-compiler-selfdev-v0` | Own missions (orchestrator suite, docs revision, campaign compiler); not og-dolt. |
| autoradio verifier review | external: `overnight-autoradio-platform-checklist-v0` | Active platform checklist; not og-dolt. |
| substrate-hardening MPCal TLC | external: `docs/missions/substrate-hardening-v0.md` | `checkpoint_incomplete`; not og-dolt. |
| substrate-hardening cmd dedup | external: `docs/missions/substrate-hardening-v0.md` | `checkpoint_incomplete`; not og-dolt. |
| cross-substrate extractor | external: `docs/missions/cross-substrate-proof-v0.md` | `checkpoint_incomplete`; not og-dolt. |
| wire-on-settlement | external: `m5-wire-on-settlement` | M5 route-switch evidence gate; not og-dolt. |
| continuation-deletion sequencing | absorbed: Phase C | og-dolt Phase C continuation deletion (H006–H008). |

Phase A exit bar (what the gate panel reviews): detectors reporting in CI;
timeouts proven in staging; all corrections committed; S1 settled or
explicitly scoped; triage table full. Then the Phase Gate Protocol runs and,
on clear, execution continues directly into Phase B.

### Phase B — Heresy kill wave 1 (M3.1, M3.2) + Dolt audit reads

As specified in mission-og-dolt Phase 1 (imported): texture-forcing removal
(H009–H012, H024a/b, H026; proof gate: honest first revisions, unforced
delegation — heed the mutually-gated transition: verify agent behavior before
deleting forcing cues); parent/child deletion (H001–H005, H015–H016, H005
first; proof gate: all authority trajectory-scoped); Dolt audit read-path
(`texture history` from `dolt_history_<table>` + `AS OF`) — this doubles as
the first D-STORE evidence probe (history latency, latest-revision cost).
Detector families for each cluster flip to fail-on-regression as they close.

Phase B exit bar: M3.1 and M3.2 clusters at the `eliminated` bar (deletion
diff + inverted tests + detector at fail-on-regression, zero live sites for
H009–H012, H024a/b, H026, H001–H005, H015–H016); both proof gates evidenced;
`texture history` served from `dolt_history` with latency numbers recorded
as D-STORE evidence.

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

### Phase D — Hot-path cutover + promotion over ComputerVersion

Imported from mission-og-dolt Phases 3, 4, 4b, gated on D-STORE resolution,
D-PROMO settlement (by Phase A experiment), and the S1 scope header landing. This phase includes the world-wire store's sql-server
migration (D-WIRE, decided): batch-commit infrastructure, hot-table cutover
with a rollback latency budget **defined before cutover**, storage-growth
measurement before victory; promotion_protocol rewrite over ComputerVersion
(TLC-checked before Go changes), adapter made load-bearing or deleted per the
D-PROMO outcome; candidate-VM residue elimination (vmctl candidate lifecycle,
candidate_computer_package*, cmd audits, wire platform routing, data.img
residue, sandbox→autoputer rename) under the C3-assigned heresy number.
H031 closes only when I1's observables are gone.

Phase D exit bar: hot entities cut over within the pre-declared rollback
latency budget with storage growth measured; wire store on sql-server with
`PROXY_RUNTIME_DB_PATH` and the direct-file-open path deleted; rewritten
promotion spec TLC-green with the W6 conformance binding green; one staging
promotion + rollback route flip executed (completion criterion 3's
evidence); I1 observables gone and the H031/candidate-VM detector enforcing.

### Phase E — Deletion, doctrine replacement, surface cleanup

Imported from mission-og-dolt Phase 5: drop SQL tables after stability
window; delete dual-write code and `backfillOGFromSQL`; M5 surface cleanup
(H019–H029); doctrine shrinks to thesis + invariants + enforcement pointers;
registry closes with every entry referencing its detector. Plus assessment
items: **C-RETR** wire retrieval ingestion (search finds its own evidence),
**C-PAGE** server-side paging on `/api/trajectories`, and the
Universal→World Wire rename (owner-decided 2026-07-08: **execute**, alongside
the sandbox→autoputer rename machinery).

Phase E exit bar: completion semantics 1–9 (the mission's own completion is
this phase's gate; the final panel round reviews the full criteria list).

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
  between ComputerVersions plus a demonstrated rollback flip — not a
  TLC-green spec, not an adapter unit test.
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
- claim: W3 landing-loop evidence for seam commits e393eb5c and e5c1d38a.
  definition_node: w3
  evidence_class: observed tool result + observed staging state
  command_or_observation: >-
    e393eb5c CI run 28963931647 (2026-07-08T17:50:58Z) was cancelled after
    runner acquisition failure; Go Vet + Test + Build failed, Deploy to Staging
    (Node B) was cancelled. e5c1d38a CI run 28964053923 (2026-07-08T17:52:55Z)
    completed with Deploy to Staging (Node B) successful and Generate SBOMs
    failing. The timeout fix was first observed at deployed SHA 67fff296
    (2026-07-09T04:56:18Z); the current deployed SHA on choir.news is 1ed41f2b
    (2026-07-09T05:12:21Z). Staging proxy log shows no "route resolver: wired
    lineage-based resolver" line; nix/node-b.nix does not set
    PROXY_RUNTIME_DB_PATH, so the proxy uses the hard-coded VM identity fallback.
    grep for DoltPromotionAdapter or WithPromotionAdapter under cmd/ returns zero
    hits; no production binary configures the promotion adapter.
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
  result: fixed by W2 (commit 67fff296 + prior server.go timeout defaults; staging api.resolve max now 60,001ms; see docs/evidence/w2-timeout-staging-proof-2026-07-09.md)
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
- claim: Plan-review consensus round 2026-07-08 (4/4 panelists returned; gpt55 output empty/failed-silently) adjudicated. Confirmed blockers, all fixed in this document — D-STORES file mapping was inverted (world-wire store is internal/platform/objectgraph_store.go, not internal/objectgraph/dolt_store.go); D-PROMO had ignored the prior 2026-07-07 experiment (adapter comment + two test files), whose falsification is diagnosed as a connection-pooling artifact (checkout ran on one pooled conn, queries on others; pinned-conn variant reportedly isolates correctly) — settlement pulled into Phase A with a -count=10 determinism bar; completion criterion 3 gained a falsified-D-PROMO fallback clause; Phases B–E gained explicit exit bars; gate adjudication must be committed as auditable evidence; supersession must be machine-readable (C5 expanded to mission-graph superseded nodes + doc-authority-manifest entries for all three docs).
  definition_node: seam, embedded-branch-isolation, dolt-store-taxonomy, phase-gate-protocol
  evidence_class: external second opinion (panel) + observed (repo re-verification of B1/B2; diag test re-run showing pooled-connection checkout non-stick; Phase A -count=10 determinism test run 2026-07-09)
  command_or_observation: docs/evidence/agentic-consensus-2026-07-08-plan/ (raw outputs); go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10
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
   demonstrated rollback flip, using the D-PROMO-settled isolation mechanism
   — pinned-connection branches if supported, or the escalated-and-accepted
   fallback (serialized single writer / separate database per candidate) if
   D-PROMO falsifies — with the spec↔implementation conformance binding
   green. Criterion 3 is satisfiable under EITHER D-PROMO outcome; a
   falsified D-PROMO changes the mechanism, not the criterion.
4. No route resolves to a VM identity (I1 observables gone); vmctl
   candidate-desktop lifecycle deleted.
5. choir-doctrine.md reduced to thesis + invariants + enforcement pointers;
   no live heresy entries.
6. `choir` CLI `trajectory`/`texture` verbs read identical shapes before and
   after, verified against production — evidence artifact: a recorded
   before/after CLI output diff against production, committed to
   `docs/evidence/`.
7. Request path bounded (I3) with staging proof.
8. All C1–C7 corrections landed; no mission document mislabeled complete.
9. Past-mission triage table complete with every `absorbed` edge executed
   and every `retired`/`external` edge annotated.

Before any non-complete exit, verify no safe executable probe remains in
bounds. A checkpoint is not completion.

## Rollback And Resumption Policy

Every red pass names its rollback ref before mutation. Entity cutovers keep
SQL fallback for a declared window. Promotion-path work is inert-by-default
until its gate settles (never enabled speculatively). Doc corrections are
git-revertable individually. This document's Run Checkpoint section is the
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
  last_checkpoint: Phase A exit panel round 2 completed; I3/D-PROMO evidence aligned and adjudication committed; pending round 3 confirmation
  current_artifact_state: >-
    Phase A deliverables committed: W1 detector manifest + CI discovery job
    (including the I4 destructive-rollback guard), W2 proxy/vmctl timeout
    hardening with staging 504 proof, W3 seam-commit landing-loop evidence,
    C1–C7 doc truth corrections, D-PROMO pinned-connection branch-isolation
    settlement, S1 spec↔adapter scope/conformance note, and P-TRIAGE past-mission
    open-edge table. Phase A exit panel round 2 completed: category-(a) findings
    from round 1 fixed, I3/D-PROMO evidence ledger aligned with the settled
    state, and the Phase A exit adjudication committed. D-STORE storage fork
    remains unresolved; D-PROMO and D-WIRE settled.
  what_shipped:
    - W1 detector manifest + CI discovery job (scripts/check-heresies.sh, docs/heresy-detectors.md H030/H031/I4 refs, CI heresy-detector job)
    - W2 proxy/vmctl timeout hardening (60s default, fast 504 staging proof)
    - W3 seam-commit evidence for e393eb5c/e5c1d38a
    - C1–C7 doc truth corrections
    - D-PROMO pinned-connection branch-isolation settlement test
    - S1 promotion_protocol.tla scope and conformance note
    - P-TRIAGE past-mission open-edge triage table
  what_was_proven:
    - seam status of the two Red commits (observed, grep-verified)
    - timeout invariant violation (observed) and fix (staging 504)
    - lineage resolver not active in staging; promotion adapter not wired in production
    - embedded Dolt branch isolation on a pinned connection is deterministic (D-PROMO -count=10)
    - all past-mission open edges triaged (absorbed/external/retired)
  unproven_or_partial_claims:
    - D-STORE storage-fork six questions
    - heresy live-site counts (families still in discovery; fail-on-regression and allowlist enforcement deferred per phase)
    - Phase B–E kill waves, cutovers, and deletion not yet executed
  remaining_error_field: see Variant below
  highest_impact_remaining_uncertainty: D-STORE storage fork + Phase B heresy elimination evidence + wire-store sql-server migration mechanics
  next_executable_probe: >-
    Phase A exit panel round 3 (delta-2 review) on the adjudication and I3/D-PROMO
    alignment; on clear, begin Phase B heresy kill wave 1 + Dolt audit reads.
  suggested_goal_string: "/goal docs/definitions/og-dolt-heresy-completion-2026-07-08.md"
  evidence_artifact_refs:
    - docs/evidence/agentic-consensus-2026-07-08/ (plan review panel raw outputs)
    - docs/evidence/agentic-consensus-2026-07-09-phase-a-exit/ (Phase A exit panel round 1 + adjudication)
    - docs/evidence/agentic-consensus-2026-07-09-phase-a-exit-delta/ (Phase A exit panel round 2)
    - docs/evidence/w2-timeout-staging-proof-2026-07-09.md
    - docs/assessment-overall-state-2026-07-07.md
  rollback_refs:
    - a703bf44 (pre-mission docs state)
```

## Suggested Goal String

```text
/goal docs/definitions/og-dolt-heresy-completion-2026-07-08.md
```
