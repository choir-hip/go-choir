# Choir Autoputer Completion Suite

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
```

This document is the single executable authority for completing the Choir
**autoputer** before opening Choir-in-Choir or resuming Autopaper editorial
work. Unlike a passive suite index, this `/goal` invocation authorizes the
orchestrator to execute the ordered subgoals below, update this document's
durable state, and continue until the suite completion semantics are satisfied.

The orchestrator must not ask the owner to invoke each member Definition. It
reads member Definitions as subordinate subgoal specifications and executes
them within this one mission run.

The same command resumes an interrupted run. On every invocation the
orchestrator must reconcile this document with repository state, pushed and
deployed commit identity, member evidence ledgers, CI, staging, and any
in-flight subagent work before selecting the next action. A phase boundary,
agent exit, context loss, terminal disconnect, or partial landing is a
checkpoint, never completion.

If learning changes the mission identity or owner-settled topology, the
orchestrator must mark this document `superseded`, name the successor Definition
and exact resumption command, update all registries atomically, and stop
mutating under this authority. It must not silently rewrite the mission into a
different object.

## Mutation Class And Ceremony

- **Definition/registry updates:** green; **doccheck live-packet update:** yellow
  because it changes future validation pressure, not product behavior.
- **Suite execution:** red overall because it touches run acceptance, persistent
  computers, promotion/rollback, deployment routing, and agent credentials.
- **Protected surfaces:** staging deployment, run lifecycle and admission,
  world-wire authority, actor/runtime execution, ComputerVersion construction,
  promotion/rollback, API-key authority, Choir-in-Choir admission.
- **Admissible evidence:** focused tests plus pushed commit, CI, deployed commit
  identity, and product-path staging proof for every behavior-changing phase.
- **Rollback:** each independently accepted atomic landing records its own
  pre-mutation SHA, rollback commit/ref, deployed identity, and acceptance
  receipt. Revert only the smallest landing implicated by evidence; earlier
  accepted S3 ratchets remain unless the failure crosses their authority edge.
  Whole-phase revert is reserved for a single-landing phase. Compatibility
  fallbacks are not rollback.
- **Heresy delta:** `discovered` — the former run-truth suite competed with the
  owner-settled autoputer spine and runtime extraction had no live owner;
  `introduced` — none by this documentation change; `repaired` — one executable
  suite and a ratcheted runtime-dissolution subgoal are established in semantic
  authority only.

## Source Authority Order

1. Owner direction in this Definition: one resumable grand mission suite;
   autoputer before autopaper; runtime dissolution is a subgoal, not a competing
   suite.
2. `docs/standing-questions.md`, `AGENTS.md`, `docs/choir-doctrine.md`.
3. `docs/computer-ontology.md`, `docs/agent-product-doctrine.md`.
4. This Definition's determined state, subgoal graph, evidence ledger, and
   checkpoint state.
5. Subordinate member Definitions named below, within their assigned scope.
6. `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` for overlapping
   deletion gates and executable detectors.
7. Observed repository, CI, staging, and product artifacts.
8. Historical runtime extraction plan in Git history at
   `99a995eb^:docs/runtime-deletion-and-extraction-plan-2026-06-27.md` as
   evidence only, not live authority.

A subordinate Definition cannot reorder this suite or widen its own scope. A
conflict is opened as a definition node and settled here before mutation.

## Standing Dictum And Real Artifact

**Autoputer before autopaper.** The product object is a persistent user computer
that an external agent can inspect, operate, change through a candidate,
promote, verify, and roll back through the Choir CLI without SSH. Only after
that path works and contained agent authority is proven may Choir-in-Choir open.
Autopaper editorial work is a successor mission, not a suite phase.

The real artifact is not a collection of packages or passing tests. It is:

```text
external agent + scoped key + choir CLI
  -> inspect one persistent computer and its serving generation
  -> start work and observe one truthful lifecycle
  -> fetch the required artifact
  -> propose a package
  -> materialize and verify a candidate computer
  -> promote through one receipted ComputerVersion route transition
  -> observe the new generation
  -> roll back through the same authority
  -> diagnose failures from bounded product evidence without SSH
```

## Mission Purpose

1. Restore the landing/deploy loop needed for all later staging proof.
2. Establish the two-store topology and remove VM-fate-shared Wire state.
3. Dissolve `internal/runtime` through repeated atomic deletion/cutover
   iterations before new autoputer product architecture is built on it.
4. Prove the audited-computer construction and observation contract.
5. Establish truthful run lifecycle and artifact-verified completion on the
   extracted core boundary.
6. Prove self-development through candidate, verification, promotion, receipt,
   observation, and rollback.
7. Contain internal-agent credentials so authority cannot widen itself.
8. Open Choir-in-Choir only after the same external operator test passes under a
   contained co-super key.

## Mission Non-Purpose

- No Autopaper editorial/reconciler activation.
- No parallel vocabulary sweep while deletion is active; names are cut over
  after correctness and deletion stabilize, without aliases.
- No extraction framework, plugin system, code-generation registry, or generic
  app abstraction unless a concrete live cutover requires it and the phase
  checkpoint approves it.
- No wrapper, facade, alias, compatibility path, dual read/write, shadow path,
  or unused replacement counted as progress.
- No SSH as acceptance. SSH remains platform break-glass diagnosis only.

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: Autoputer precedes Choir-in-Choir and Autopaper.
      source: owner
      execution_effect: No editorial activation before suite completion.
    - claim: This document is the single executable suite authority.
      source: owner
      execution_effect: One /goal invocation sequences and resumes all subgoals.
    - claim: Runtime dissolution is a subgoal of this suite.
      source: owner
      execution_effect: It cannot become a competing mission spine.
    - claim: Each behavior cutover deletes its superseded production path atomically.
      source: owner
      execution_effect: Additive migration and cleanup-later plans are rejected.
    - claim: internal/runtime had 48,551 production lines, 55,340 test lines, and 144 Go files at the 2026-07-11 audit.
      source: observed
      execution_effect: These are initial ratchet baselines and must be freshly remeasured at suite start.
    - claim: The historical target deletes internal/runtime after moving only live core and app behavior.
      source: observed
      execution_effect: Directory absence, not a smaller god package, is the dissolution artifact.
  contested: []
  open:
    - node: runtime-disposition-inventory
      missing: Fresh file/export/route/tool/caller classification at execution start.
    - node: final-autoputer-package-boundaries
      missing: Boundaries are settled one cutover at a time from live callers; the suite forbids speculative framework design.
```

## Definition Graph

```yaml
definition_graph:
  - id: grand-suite-authority
    kind: authority_rule
    status: settled
    settled_by: owner
    definition: This Definition is the orchestrator's single execution and resumption authority.
    execution_effect:
      - Member Definitions are subordinate specifications, not separate goal runs.
      - The orchestrator owns integration, ordering, evidence, rollback, and checkpoint state.
  - id: atomic-cutover
    kind: invariant
    status: settled
    settled_by: owner
    definition: A replacement is live only when every production caller uses it and the superseded path is deleted in the same landing.
    forbidden_collapses:
      - new package exists -> extraction complete
      - deprecated wrapper -> old path deleted
      - tests call API -> production consumer exists
      - lower runtime LOC -> behavior is not duplicated elsewhere
  - id: runtime-package-extinction-target
    kind: completion_semantics
    status: settled
    settled_by: owner
    deletion_target_reference: internal/runtime
    definition: The typed target is absent, with zero imports, wrappers, aliases, registrations, or state authorities and no untyped live documentation citers.
  - id: phase-checkpoint
    kind: evidence_class
    status: settled
    settled_by: orchestrator
    definition: A phase passes only after implementer evidence, independent micro-verification, agentic-consensus checkpoint review, orchestrator adjudication, and product-path proof.
  - id: resumable-execution
    kind: invariant
    status: settled
    settled_by: owner
    definition: Every landed mutation and reviewed phase leaves enough durable state in this document to resume with the same /goal command without repeating or guessing.
  - id: route-ledger-topology
    kind: adjudication
    status: settled
    settled_by: owner
    definition: >-
      ComputerVersion route authority is a route-slot/receipt table set on the
      corpusd world-wire sql-server, with vmctl as sole CAS writer; it is never
      a third Dolt domain. OG/Dolt D-STORES, D-ROUTE, and D-STORE language that
      required a distinct Dolt-backed platform-control ledger was
      orchestrator-settled, unratified, and is demoted in place by the owner
      two-store directive.
    execution_effect:
      - S7 implements the route-slot tables on corpusd and vmctl-only CAS/read APIs.
      - Route rows remain control authority and never become world-wire article state.
  - id: suite-supersession
    kind: escalation_rule
    status: settled
    settled_by: owner
    definition: A changed mission identity requires an explicit successor Definition and registry cutover; the old suite becomes a forwarding historical authority only.
```

## Orchestration Contract

The `/goal` agent is the suite **orchestrator**, not the default implementer.
It must:

1. maintain the only authoritative subgoal and operation state in this document;
2. decompose the active subgoal into non-overlapping execution slices;
3. persist each planned writing slice, exact mutation locks, implementer,
   rollback ref, and close condition in the delegation ledger **before**
   dispatch;
4. delegate implementation slices with exact targets, authority boundaries,
   acceptance criteria, and explicit instructions to skip project-wide tests
   and formatters;
5. delegate micro-verification to a recorded different agent/session;
6. use agentic consensus at every phase checkpoint, with the exact diff,
   evidence, ratchet report, staging receipts, and unresolved risks;
7. adjudicate consensus rather than vote-counting or accepting reviewer claims;
8. perform or coordinate final integration, focused checks, landing loop, and
   staging product proof;
9. update every operation stage and this document after each landing and
   checkpoint before selecting the next subgoal.

Parallel subagents are allowed only for independent files or read-only audits.
Two agents must not concurrently mutate shared bootstrap, lifecycle authority,
route registration, Wire authority, promotion state, or the same destination
package. The orchestrator must serialize cutovers that share an authority edge.

A worker saying `done`, a passing focused test, a new package, a consensus
majority, or a checkpoint artifact cannot advance phase status by itself.

### Durable Delegation Transaction

Each writing slice is a durable transaction with stages:

```text
planned -> dispatch_intent -> dispatched -> implementing
-> implementation_returned -> verifying -> committed -> pushed -> ci_passed
-> deployed -> accepted -> consensus -> adjudicated -> landed
```

Terminal alternatives are `abandoned` and `rolled_back`. Before dispatch, the
orchestrator records the slice ID, dispatch nonce, exact
files/packages/routes/state authorities, forbidden targets, locked authority
edges, implementer identity, independent verifier identity or assignment rule,
workspace/branch ref, pre-mutation SHA, mutation class, protected surfaces,
acceptance contract, timestamps/lock expiry, and close condition. No other slice may
touch an overlapping target or authority edge until that record is `landed`,
`abandoned`, or `rolled_back`.

The durable write boundary is Git. The suite lands to `origin/main` per
`AGENTS.md`; isolated subagent branches/worktrees start from the recorded
`origin/main` SHA and never become alternate authority. Every
`dispatch_intent` and transition before an irreversible external action records
a stable `transition_id`, expected parent SHA, and transition contents, then is
committed and pushed to the controlling branch before the action. The
containing commit SHA is derived from Git history during reconciliation; a
record never attempts to contain its own SHA. The dispatch assignment carries
the recorded nonce and transition ID.
Subsequent records bind the returned agent/session handle, workspace,
implementation commit, pushed ref, CI run, deploy attempt, deployed SHA,
acceptance receipt, consensus, and adjudication.
No stage name collapses local commit, push, deployment, or acceptance.

The canonical journal ref is `refs/heads/main` at `origin`. B0 is the sole
bootstrap exception described below; the ordinary lock protocol applies only
after its first authority landing succeeds. Thereafter, before writing state or
dispatching, an orchestrator acquires or renews the journal lock by committing
its identity, monotonically increasing epoch, expiry, unique lock transition
ID, and expected parent SHA, then pushing by ordinary fast-forward CAS. Exactly
one contender can advance that parent; a rejected push confers no authority and
must fetch/reconcile. Only the unexpired lock holder may mutate the ledger or
dispatch. The holder must renew before expiry during worker/CI/deploy waits and
must stop acting immediately if renewal fails or expiry passes. Takeover uses
the same expected-parent CAS and completes nonce/effect reconciliation first.
This contract requires fast-forward push access to `origin/main` and durable
agent/job or branch/patch references. If either is unavailable, B0 is
`blocked_incomplete`; the orchestrator must not degrade to in-memory state or
untracked shared-worktree dispatch.

A stage is valid when its unique transition ID and expected parent occur in one
commit reachable from the canonical ref; reconciliation derives that commit
SHA from Git history. `stage_history` is append-only and records `{status,
transition_id, recorded_at, actor, expected_parent_sha, precondition,
postcondition, external_operation_id}`. Prepare-only state is a durable
incomplete transition: recovery may finish it only after proving its
precondition and external-effect state, otherwise it remains blocked.
The suite document's `delegation_ledger` and
`run_checkpoint_and_resumption_state` are the durable checkpoint. Every
transition is committed and pushed to the same canonical ref the `/goal`
command resolves before any dispatch, external action, or intentional
interruption; branch-local state is never sufficient.

On restart, reconcile by dispatch nonce across every declared durable substrate:
agent/job records, isolated branches/worktrees, commits, patch/output artifacts,
and attributable dirty paths. Record `last_reconciled_at` and
`reconciliation_result`. Adopt exactly one matching result; quarantine multiple
or conflicting matches. After lock expiry, autonomously mark the intent
`abandoned` only when every declared substrate proves absence. Require human
authority only for conflicting evidence or a protected external effect whose
authority cannot be queried. Never redispatch from stage name alone.

Every protected external mutation—cancel, candidate creation, promotion,
rollback, key issuance, Choir-in-Choir admission, and any later equivalent—has
a precommitted `external_operation_id`, authoritative effect owner, receipt
lookup, expected precondition, and observed postcondition. Recovery queries by
operation ID, adopts an existing receipt, or retries only when the authority
proves the first attempt did not commit and the operation is idempotent.
Unclaimed dirty paths remain unknown user/agent WIP and must not be overwritten.

A verifier is independent only when its different agent/session identity is
recorded. Consensus is phase evidence only after the adjudication record
classifies every material finding. `escalated` never clears a blocker: the
phase remains `blocked_incomplete` until the named authority durably changes
the requirement or explicitly accepts the risk. Only `repaired` or
`rejected_with_evidence` clears a blocking finding autonomously.

Writing subagents use an isolated worktree/branch or return a patch artifact for
orchestrator application. Direct shared-worktree mutation is allowed only for
one active writing slice with a recorded exception and justification. The
orchestrator alone integrates, commits, pushes, and lands. Lock acquisition and
release, dispatch prompts, jobs, outputs, verifier jobs/outputs, and mutation
delivery mode are durable ledger refs, not interaction memory.
No phase implementer or micro-verifier may be the sole consensus reviewer or
adjudicator for that phase; reviewer identities and independence are recorded.
For every red slice, orchestrator, implementer, and micro-verifier are three
distinct recorded identities. The orchestrator may integrate but cannot author
or certify the slice it adjudicates.

## Phase Checkpoint Protocol

Every behavior-changing phase uses this sequence. S0 and docs/checker-only
steps stop after focused proof, independent review, consensus, adjudication,
and durable checkpoint update; they do not manufacture a deploy:

```text
reconcile ledger + repo + agents + CI/staging
-> state active conjecture and mutation radius
-> persist planned slices and authority locks
-> delegate implementation slices
-> record returned implementation state
-> independent micro-verification
-> integrate and run focused local proof
-> commit and push; record landed SHA
-> monitor CI and staging deploy
-> verify deployed commit identity
-> run product-path staging acceptance
-> record phase checkpoint as consensus_pending
-> run agentic consensus on diff + evidence + ratchets
-> record findings and adjudication_pending
-> orchestrator verifies and adjudicates every material finding
-> repair and repeat checkpoint if needed
-> persist passed checkpoint, suite evidence, and resumption state
-> advance exactly one phase
```

Consensus output is external second-opinion evidence. A phase passes only when
all confirmed blocking findings are `repaired` or `rejected_with_evidence`.
An escalated finding keeps the phase `blocked_incomplete` until the named
authority records a durable requirement change or explicit risk acceptance.

## Runtime Dissolution Ratchets

At suite start, remeasure and record:

- non-test and test LOC under `internal/runtime`;
- Go file count;
- production packages importing `internal/runtime`;
- exported runtime symbols;
- runtime-owned routes and tool registrations;
- `*runtime.Runtime` and `*runtime.APIHandler` embeds/wrappers;
- compatibility markers tied to old/new runtime paths;
- state writers for run lifecycle, Wire, and promotion;
- every `internal/runtime` citer across `docs/`, `specs/`, `skills/`,
  `AGENTS.md`, code comments, manifests, CI configuration, and generated
  detector manifests.

After the bounded deploy-unblock exception, no ratchet may increase. Every
runtime-dissolution iteration must remove at least one production importer or
runtime-owned authority and reduce the applicable file/symbol/line counts.
Repository production LOC cannot validate progress if equivalent behavior was
copied while the old path remains.

Every citer receives `delete | redirect_to_successor |
deletion_target_reference | historical_evidence | block` disposition; there is
no silent allowlist. `deletion_target_reference` is mechanically valid only as
a structured verifier/completion field naming
`runtime-package-extinction-target`; it cannot authorize, navigate to, import,
wrap, or preserve the implementation. The ratchet invocation checks every S3
iteration. The final S3 landing rewrites all active prose and contracts to the
stable target/artifact ID, leaving the literal path only in typed
`deletion_target_reference` fields or append-only, explicitly non-authorizing
historical evidence.

S1 is the sole bounded growth exception. Before S1 lands, every added runtime
file, symbol, test, route, configuration field, and production caller must be
appended to `s1_runtime_exception_disposition` with its Deploy necessity,
production caller, rollback ref, destination or deletion owner, and expected S3
iteration. S3 cannot start until an independent verifier confirms that table is
complete and the ratchet baseline has been rebased to include the exception.

Hard gates:
- **S9-only alias window:** the S3 no-alias gate applies to runtime extraction.
  S9 may carry only its recorded transitional HTTP/read alias between S9
  iterations; no alias may survive S9 completion or suite completion.

- **No wrapper:** destination packages import no `internal/runtime` and do not
  embed or forward runtime types.
- **No alias:** no exported alias, forwarding constructor, deprecated registrar,
  or compatibility re-export preserves the old surface.
- **Production caller:** every new exported symbol has a non-test production
  caller in the same landing.
- **Test-only deletion:** tests do not keep otherwise unused production APIs
  alive.
- **Atomic cutover:** new caller wiring and old path/API/test/config deletion
  land together.
- **Single authority:** one writer for run lifecycle, Wire publication/state,
  and ComputerVersion promotion.
- **No compatibility rollback:** rollback reverts the smallest implicated atomic
  landing; it does not reconnect an old path behind a switch.
- **Independent audit:** the implementer cannot certify its own cutover.

## Ordered Subgoal Graph

Subgoals execute in this order. `waiting_on_predecessor` is an ordinary
dependency state, not `blocked_incomplete`. A later subgoal becomes `working`
only after its predecessor reports `complete` with named evidence.
Subgoal statuses are
`waiting_on_predecessor | working | checkpoint_incomplete |
blocked_incomplete | complete | superseded`; they are distinct from suite
status and delegation-transaction status enums.
The pre-graph `definition_gate` passes only after a post-repair panel is
adjudicated and live/full docs checks have no errors. Before B0, that result is
prepared in the worktree but is not yet canonical authority.

### B0 — Persist Suite Authority

B0 is the sole bootstrap transaction:

1. read and record the current `origin/main` parent;
2. create a stable `suite_run_id`, journal lock epoch 1, lock expiry/transition
   ID, `definition_gate.status: passed`, cleared `open_findings`,
   `current_subgoal: B0`, and B0 `working` state in the authority landing;
3. commit this Definition, registry cutover, subordinate demotions, doccheck
   live-packet update, and consensus evidence, then push by fast-forward CAS;
4. acquire authority only if that push succeeds; on rejection, fetch and
   reconcile without dispatching;
5. after success, make a normal lock-governed checkpoint commit recording the
   first landing as `suite_authority_sha`, `scripts/doccheck -mode live`
   command/result, B0 `complete`, `current_subgoal: S0`, and S0 `working`.

An interruption after step 3 resumes from canonical B0 `working` state and
finishes step 5; it never repeats the authority landing.

**Exit:** the exact `/goal` document and all registries are retrievable from
the recorded authority SHA; live docs truth passes; B0 is complete.

### S0 — Reconcile And Baseline

- Reconcile the current repo, CI, staging, and all subordinate Definitions.
- Record fresh runtime ratchets and a complete `delete | core | <domain>`
  disposition for every production runtime file, export, route, tool, and
  external caller. There is no `later` bucket.
- Install executable ratchet checks before non-break-glass runtime mutation.

**Exit:** inventory and ratchets are durable, mechanically checkable, and
independently reviewed.

### S1 — Restore Deployability

Subordinate specification:
`docs/definitions/choir-run-deploy-unblock-2026-07-11.md`.

Maximum scope: reuse existing cancellation authority; expose the minimum product
operator path; ensure stale active processor work cannot retain admission
indefinitely; drain the current run; prove the next runtime-bearing deploy.
Before S1 lands, disposition every added runtime surface in
`s1_runtime_exception_disposition`; do not invent future S0 entries.

**Exit:** `running_runs: 0` or authoritative equivalent, green deployed commit,
and cancel/stale-capacity regression proof.

### S2 — Wire Authority Cutover

Subordinate specification:
`docs/definitions/choir-wire-store-conformance-2026-07-11.md`.
This is Autoputer Phase 0 and runtime-dissolution iteration 1.

**Exit:** corpusd/world-wire is the sole Wire authority; boot migration and
runtime-local publication/read/fallback paths are deleted; the feed survives
VM stop, restart, and deploy; thin CLI observation is available.

### S3 — Runtime Dissolution

Execute repeated, independently checked iterations. Boundaries may be adjusted
from fresh caller evidence, but the dependency order is fixed:

1. delete dead/test-only/continuation/parent-child and compatibility surfaces;
2. extract live execution and tool-loop core to the smallest existing domain
   package proved by the S0 caller graph (default `internal/agentcore` only if
   that graph supports it), and remove `*runtime.Runtime` embedding;
3. move real API/config/bootstrap ownership and remove the `apihandler` wrapper
   plus direct `cmd/sandbox` runtime imports;
4. cut over one live app/domain per landing, deleting the old path atomically;
5. retire duplicate candidate/promotion mutation paths and align the extracted
   boundary to the one receipted ComputerVersion route contract; S3 does not
   build or activate the CAS writer—grand S7 owns that product mutation;
6. move final core residue, delete the typed
   `runtime-package-extinction-target`, and atomically rewrite every active
   path citation to the stable target/artifact ID.

Suggested first app extraction is Browser as the smallest self-contained proof.
Texture, Wire residue, content/media, promotion, researcher, super, conductor,
vmctl tools, podcast, email, desktop, search/model policy, and prompt ownership
follow according to the S0 caller graph. Dead code is deleted, never moved.

**Exit:** `runtime-package-extinction-target` is satisfied; every
`s1_runtime_exception_disposition` row is deleted or relocated to its recorded
owner; no imports/wrappers/aliases, runtime-owned registrations, or untyped live
citers remain; one state authority per domain; focused and staging product
proofs pass; independent consensus validates the extinction artifact.
Every S3 atomic cutover iteration is its own full behavior checkpoint: focused
proof, landing loop, deployed acceptance, independent micro-verification,
consensus, adjudication, ratchet update, and durable slice completion. S3
becomes `complete` only after the final extinction iteration passes.

### S4 — Audited Computer

Subordinate specification:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 1
(audited-computer operator/receipt surface).

Subordinate inputs:

- `docs/computer-ontology.md` Target Candidate Contract;
- these named PC-5 pre-wiring gates from
  `docs/definitions/choir-product-completion-2026-07-10.md`: **Computer and
  owner scope**, **Exact bytes**, **Stable identity**, **Explicit ancestry**,
  **Cursor retention**, **All resolutions**, **Restart durability**,
  **Idempotent delivery**, **Canonical replay and materialization**, and
  **Artifact and Dolt boundary**;
- D-ROUTE detector/receipt contracts as forward dependencies of S7, not S4
  completion authority.

Grand S4 explicitly unpauses only these ten named PC-5 gates plus the
Candidate Contract. All PC-5 post-gate service ownership and other paused
product-completion work remain outside S4.

Build `ComputerVersion(CodeRef, ArtifactProgramRef)` and prove deterministic
materialization from durable state, positive candidate equivalence, negative
mismatch rejection, and CLI-visible status/generation/receipt evidence on
staging without SSH. S4 makes no served-route promotion or rollback claim and
owns no run-lifecycle authority. It does not pull Wails, Base product APIs,
File Provider, or post-gate service wiring forward.

**Exit:** the ComputerVersion construction and materializer are product-path
proven; candidate equivalence and mismatch receipts are CLI-visible; no served
route has been promoted; staging proves the real computer path without SSH.

### S5 — Observation And Receipts

Subordinate specification:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 2.

**Exit:** an external agent can inspect readiness, health, serving generation,
deploy/restart history, and bounded failure evidence through `choir` CLI; receipts
match deployed reality.

### S6 — Run Truth

Subordinate specification:
`docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`.
Implement against the extracted lifecycle boundary, not `internal/runtime`.

**Exit:** one durable lifecycle authority, retryable terminal failures,
artifact-verified completion, truthful `choir run status`, and the external
operator can start, observe, and fetch required output on staging.

### S7 — Self-Development

Subordinate specification:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 4.

**Exit:** external agent performs package -> candidate -> verification ->
receipted promotion -> serving-generation observation -> receipted rollback
through CLI only. Exactly one promotion state machine and writer exist.

### S8 — Containment And Choir-In-Choir

Subordinate specification:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 5.

A contained key is capability- and resource-scoped, cannot mint broader
authority, cannot select another owner/computer, and cannot bypass receipted
promotion. First prove the external operator test under the scoped key; then give
a co-super the same bounded surface.

**Exit:** a co-super passes the complete operator/self-development test on its
assigned computer while negative tests prove cross-owner, cross-computer,
key-escalation, and platform-admin operations are denied.

### S9 — Vocabulary Cutover And Successor Handoff

Run the clean rename cutover only after deletion and correctness stabilize.
Subordinate specification:
`docs/definitions/choir-vocabulary-cutover-2026-07-11.md`.
No aliases or deprecated names remain.

Create or select a new Autopaper Definition only after S8. Autopaper does not
execute under this suite.

**Exit:** product vocabulary matches the settled object model, all old names are
dead, and the next Autopaper mission (if any) names this suite's completion
artifacts as prerequisites.

## Suite Completion Semantics

Status is `complete` only when:

1. B0 and S0–S9 are each `complete` with phase-checkpoint evidence;
2. the external and contained co-super operator tests pass on staging;
3. `runtime-package-extinction-target` is satisfied with zero
   importers/wrappers/aliases and zero untyped live citers; any retained literal
   path is a structured deletion-target field or isolated, explicitly
   non-authorizing historical evidence;
4. Wire, run lifecycle, and promotion each have one durable authority;
5. every behavior-changing landing records pushed SHA, CI, deployed SHA,
   staging proof, verifier contracts, acceptance identifiers, and rollback ref;
6. Choir-in-Choir is open under contained authority;
7. Autopaper remains unstarted or is governed by an explicitly registered
   successor Definition.

Statuses:

- `working`: a safe in-bound probe remains.
- `checkpoint_incomplete`: durable progress exists but suite completion is not
  satisfied; resume with the same `/goal` command.
- `blocked_incomplete`: no safe in-bound probe remains after root-cause and
  observer-shift attempts; exact missing authority/prerequisite is recorded.
- `superseded`: a successor Definition has become the registered authority.
- `complete`: all completion predicates above are observed.

The orchestrator must not return `checkpoint_incomplete` merely because an
agent, process, or interaction session is ending. Before any intentional
handoff it must persist the checkpoint below. An unintentional interruption is
recovered by reconciliation on the next identical `/goal` invocation.

## Evidence Ledger

```yaml
evidence_ledger:
  - claim: Runtime dissolution baseline before suite execution.
    evidence_class: observed repository measurement
    source: 2026-07-11 audit
    result: 48551 production lines; 55340 test lines; 144 Go files; intended extraction destinations absent
    uncertainty: Must be remeasured at S0 because the worktree may change.
  - claim: Historical runtime reduction target was directory deletion, not a smaller runtime package.
    evidence_class: observed Git history
    source: 99a995eb^:docs/runtime-deletion-and-extraction-plan-2026-06-27.md
    result: approximately 40K live app lines move, 6K-8K core moves, internal/runtime deleted
    uncertainty: Package boundaries are evidence, not automatically binding current design.
  - claim: Pre-definition consensus supports deploy-unblock, Wire cutover, ratcheted dissolution, then autoputer phases.
    evidence_class: external second opinion plus local verification
    source: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md
    result: six completed ordering opinions; Cursor stalled and contributed no opinion
    uncertainty: Superseded by the post-diff validation and adjudication recorded in the same evidence file.
```

## Run Checkpoint And Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  suite_authority: docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
  current_subgoal: S2
  last_completed_subgoal: S1
  definition_gate:
    status: passed
    consensus_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
    adjudication_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
  suite_run_id: choir-autoputer-completion-2026-07-11-01
  canonical_journal_ref: refs/heads/main@origin
  journal_expected_parent_sha: 464e58cc
  orchestrator_lock:
    holder: Main
    epoch: 12
    expires_at: 2026-07-12T15:35:02Z
    expected_parent_sha: 464e58cc
    lock_transition_id: s3-lock-renewal-88
  suite_authority_sha: 008a7b88cf200119c0f762cc51cfba6be3007445
  subgoal_status:
    B0: {status: complete, started_at_sha: 27db14c36c482e321b56a056f6ce5e0accb338a4, completed_at_sha: 008a7b88cf200119c0f762cc51cfba6be3007445, evidence_refs: [008a7b88cf200119c0f762cc51cfba6be3007445, docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md], rollback_refs: [27db14c36c482e321b56a056f6ce5e0accb338a4], blockers: []}
    S0: {status: complete, started_at_sha: 008a7b88cf200119c0f762cc51cfba6be3007445, completed_at_sha: 2327fcef4716aef070eb4b819296f01b44267364, evidence_refs: [docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md, docs/evidence/choir-autoputer-s0-consensus-2026-07-11.md, agent://S0RatchetVerifier, artifact://461, https://github.com/choir-hip/go-choir/actions/runs/29176500535], rollback_refs: [008a7b88cf200119c0f762cc51cfba6be3007445], blockers: []}
    S1: {status: complete, started_at_sha: 2327fcef4716aef070eb4b819296f01b44267364, completed_at_sha: 9dff3690, evidence_refs: [docs/definitions/choir-run-deploy-unblock-2026-07-11.md, docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md, agent://S1DeployVerifier, https://github.com/choir-hip/go-choir/actions/runs/29179656372, /tmp/choir-s1-final-consensus-20260712, /tmp/choir-s1-post-repair-consensus-20260712], rollback_refs: [2327fcef4716aef070eb4b819296f01b44267364], blockers: []}
    S2: {status: complete, started_at_sha: 9dff3690, completed_at_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50, evidence_refs: [docs/definitions/choir-wire-store-conformance-2026-07-11.md, docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md, agent://S2LifecycleVerifier, agent://S2MigrationVerifier, /tmp/choir-s2-final-repair-consensus-20260712, https://github.com/choir-hip/go-choir/actions/runs/29188248479], rollback_refs: [9dff3690, 481fb8c8], blockers: []}
    S3: {status: working, started_at_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50, completed_at_sha: '', evidence_refs: [docs/runtime-dissolution-inventory.yaml, docs/evidence/s3-runtime-dissolution-dispatch-2026-07-12.md, docs/evidence/s3-runtime-dead-helper-dispatch-2026-07-12.md], rollback_refs: [b7b1262e455a779ca00c8d968ef28b3fa6af9b50], blockers: []}
    S4: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S3]}
    S5: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S4]}
    S6: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S5]}
    S7: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S6]}
    S8: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S7]}
    S9: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S8]}
  active_phase_checkpoint:
    subgoal: S3
    status: working
    deployed_sha: af0479db1e2afe0fafb5c3ca017f71c2d85cbdb4
    ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29192061906
    staging_ref: activation_receipt_29192061906_sandbox_af0479db_at_2026-07-12T12:20:01Z
    product_proof_refs: [docs/evidence/s3-runtime-dissolution-dispatch-2026-07-12.md#s3-i1-deployment-and-consensus-receipt]
    consensus_ref: /tmp/choir-s3-i1-final-consensus-20260712
    open_findings: [S3_I2_declaration_only_helper_deletion_dispatch_intent]
    adjudication_ref: S3_I1_PASS_S3_I2_pending
  delegation_ledger_schema:
    required_fields:
      - slice_id
      - subgoal
      - suite_run_id
      - orchestrator_lock_epoch
      - status
      - dispatch_nonce
      - dispatch_ref
      - agent_session_ref
      - dispatch_prompt_ref
      - implementer_job_ref
      - implementer_output_ref
      - verifier_job_ref
      - verifier_output_ref
      - worktree_or_branch_ref
      - declared_reconciliation_substrates
      - mutation_delivery_mode
      - direct_shared_worktree_allowed
      - direct_shared_worktree_justification
      - lock_acquired_ref
      - lock_release_ref
      - stage_started_at
      - transition_id
      - expected_parent_sha
      - stage_history
      - lock_expires_at
      - mutation_class
      - protected_surfaces
      - exact_files_packages_routes_state_authorities
      - forbidden_targets
      - authority_edges_locked
      - implementer_agent
      - verifier_agent
      - pre_mutation_sha
      - rollback_commit_or_ref
      - accepted_slice_dependency_refs
      - external_operation_id
      - effect_authority
      - receipt_lookup
      - expected_precondition
      - observed_postcondition
      - external_operation_idempotent
      - implementation_sha_or_dirty_snapshot
      - implementation_commit_sha
      - push_ref
      - ci_run_ref
      - deploy_ref
      - deployed_sha
      - acceptance_ref
      - acceptance_contract
      - evidence_refs
      - open_findings
      - landed_commit_sha
      - adjudication
      - last_reconciled_at
      - reconciliation_result
      - close_condition
    allowed_statuses: [planned, dispatch_intent, dispatched, implementing, implementation_returned, verifying, committed, pushed, ci_passed, deployed, accepted, consensus, adjudicated, landed, blocked_incomplete, abandoned, rolled_back]
    verifier_independence: implementer_agent_must_differ_from_verifier_agent
  delegation_ledger:
    - slice_id: S0-runtime-inventory-ratchet-01
      subgoal: S0
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 5
      status: landed
      dispatch_nonce: s0-runtime-inventory-ratchet-01-nonce-01
      dispatch_ref: S0RatchetImplementer
      agent_session_ref: agent://S0RatchetImplementer
      dispatch_prompt_ref: docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md
      implementer_job_ref: S0RatchetImplementer
      implementer_output_ref: agent://S0RatchetImplementer
      verifier_job_ref: S0RatchetVerifier
      verifier_output_ref: agent://S0RatchetVerifier
      worktree_or_branch_ref: s0-runtime-inventory-ratchet-01@f41d0f05981809ced2e185ccbe8fe3f42cc79948
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: 1a9a90b63f6541fcb8d96502e85a158b8446d14e
      lock_release_ref: 2327fcef4716aef070eb4b819296f01b44267364
      stage_started_at: 2026-07-11T21:11:54Z
      transition_id: s0-runtime-inventory-ratchet-landed-35
      expected_parent_sha: 2327fcef4716aef070eb4b819296f01b44267364
      stage_history:
        - {status: dispatch_intent, transition_id: s0-runtime-inventory-ratchet-dispatch-intent-01, recorded_at: 2026-07-11T21:11:54Z, actor: Main, expected_parent_sha: 1a9a90b63f6541fcb8d96502e85a158b8446d14e, precondition: S0_working_and_lock_epoch_3_held, postcondition: dispatch_prompt_and_exact_mutation_lock_are_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s0-runtime-inventory-ratchet-dispatched-02, recorded_at: 2026-07-11T21:14:41Z, actor: Main, expected_parent_sha: f72a141ef0f97fbec6521831dc3f5836b9526631, precondition: canonical_dispatch_intent_and_live_lock_epoch_3, postcondition: implementation_agent_started_with_recorded_nonce, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s0-runtime-inventory-ratchet-returned-03, recorded_at: 2026-07-11T21:23:47Z, actor: Main, expected_parent_sha: eca2f134cca65c85a02971af8f7e1140b7fc7f44, precondition: exactly_one_matching_agent_result_for_dispatch_nonce, postcondition: isolated_commit_recorded_for_integration, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-verifier-intent-04, recorded_at: 2026-07-11T21:28:23Z, actor: Main, expected_parent_sha: d2cde593b2b6e7b1ab407e74e713eee5534b8c42, precondition: corrected_implementation_integrated_and_orchestrator_smoke_passed, postcondition: independent_verifier_assignment_is_canonical, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-verifier-failed-05, recorded_at: 2026-07-11T21:31:09Z, actor: S0RatchetVerifier, expected_parent_sha: 5629347ba0a5c344341c4f2220f6ebb4ab10450a, precondition: independent_read_only_verification_of_canonical_slice, postcondition: two_blocking_findings_recorded_for_repair, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-repair-returned-06, recorded_at: 2026-07-11T21:38:51Z, actor: Main, expected_parent_sha: 1392a724d4381f3f4d9ca41478e8395acf87154b, precondition: both_blocking_findings_have_targeted_regressions_and_local_focused_pass, postcondition: repaired_commit_integrated_and_ready_for_independent_reverification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-reverify-failed-07, recorded_at: 2026-07-11T21:41:24Z, actor: S0RatchetVerifier, expected_parent_sha: ccbb6c172df996542f959982195a70dd6d560be4, precondition: independent_reverification_of_repaired_caller_gate, postcondition: ordinary_exported_method_call_false_rejection_recorded, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-type-repair-returned-08, recorded_at: 2026-07-11T22:02:29Z, actor: Main, expected_parent_sha: 022174f0a44335ad2332e2d64a7007fad233bd9f, precondition: type_aware_stdlib_only_caller_resolution_integrated_and_local_default_mode_passed, postcondition: S0_RAT_003_repaired_pending_independent_reverification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-debt-gate-failed-09, recorded_at: 2026-07-11T22:05:28Z, actor: S0RatchetVerifier, expected_parent_sha: 09d5610f9ccacfc6a585be1032575fcf83792720, precondition: final_independent_reverification_including_debt_no_growth, postcondition: mutable_baseline_debt_laundering_blocker_recorded, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-debt-repair-returned-10, recorded_at: 2026-07-11T22:10:16Z, actor: Main, expected_parent_sha: bdc47dfc98384d62d21941b928db1c35616e7c09, precondition: Git_authority_debt_no_growth_repair_integrated_and_local_passed, postcondition: S0_RAT_004_repaired_pending_independent_reverification, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-consensus-pending-11, recorded_at: 2026-07-11T22:22:43Z, actor: Main, expected_parent_sha: ad1a4213c7a83812814ddb2524d870d36ab991da, precondition: focused_pass_independent_verifier_pass_and_required_CI_gates_passed, postcondition: default_agentic_consensus_requested_on_exact_S0_diff_and_evidence, external_operation_id: agentic_consensus_S0_20260711_01}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-consensus-blocked-12, recorded_at: 2026-07-11T22:33:16Z, actor: Main, expected_parent_sha: 93b67ee6f2b321692716defea6b17c4c8690f772, precondition: seven_agent_panel_complete_and_material_findings_locally_checked, postcondition: S0_CONS_001_confirmed_blocking_and_other_findings_adjudicated, external_operation_id: agentic_consensus_S0_20260711_01}
        - {status: consensus, transition_id: s0-lock-renewal-13, recorded_at: 2026-07-11T22:34:21Z, actor: Main, expected_parent_sha: aea36c0853357758f913d3886b0c3e57a918fab1, precondition: lock_epoch_3_held_and_consensus_repair_in_progress, postcondition: lock_epoch_4_held_through_repair_verification_and_checkpoint, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-consensus-repair-returned-14, recorded_at: 2026-07-11T22:45:53Z, actor: Main, expected_parent_sha: aacfbbe49124238134966f0a10290aa35181c715, precondition: S0_CONS_001_type_aware_store_writer_repair_and_consensus_citer_rebase_integrated_and_local_passed, postcondition: confirmed_consensus_blocker_repaired_pending_independent_micro_verification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-patch-writer-failed-15, recorded_at: 2026-07-11T22:49:38Z, actor: S0RatchetVerifier, expected_parent_sha: 4aa1d5a44132fe5cf1048fd0e3f7246c98f2b1cc, precondition: independent_micro_verification_of_type_aware_store_writer_repair, postcondition: PatchRevisionMetadata_Wire_writer_allowlist_omission_recorded, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-patch-writer-repair-returned-16, recorded_at: 2026-07-11T22:56:29Z, actor: Main, expected_parent_sha: 14c376dbe15b8544c75b337ccb3740a50895b469, precondition: Patch_store_mutation_and_consensus_citer_repair_integrated_and_local_passed, postcondition: S0_CONS_002_repaired_pending_independent_verification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-writer-substrate-failed-17, recorded_at: 2026-07-11T22:59:42Z, actor: S0RatchetVerifier, expected_parent_sha: f4392ada9a79da7a57c7da26c11f912c86f9ec5e, precondition: independent_reverification_after_Patch_repair, postcondition: positive_mutation_verb_allowlist_root_cause_cluster_recorded_with_Claim_Release_Cancel_omissions, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-writer-substrate-repair-returned-18, recorded_at: 2026-07-11T23:21:26Z, actor: Main, expected_parent_sha: 672eb8876fddf57751d9726b3a002484c62193cc, precondition: exhaustive_fail_closed_store_method_classification_integrated_and_local_passed, postcondition: S0_CONS_003_substrate_repaired_pending_independent_verification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-read-prefix-failed-19, recorded_at: 2026-07-11T23:23:48Z, actor: S0RatchetVerifier, expected_parent_sha: bf16aacf3381dbc09c99fe0e7b9169e4ad02bece, precondition: independent_verification_of_exhaustive_store_call_partition, postcondition: read_prefix_fallback_fail_open_counterexample_recorded, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-exact-store-disposition-returned-20, recorded_at: 2026-07-11T23:33:28Z, actor: Main, expected_parent_sha: 11c6dff6071555154e60f3a1aea953f802ef8ffc, precondition: exact_baseline_authority_for_all_typed_store_calls_integrated_and_local_passed, postcondition: S0_CONS_004_repaired_pending_independent_verification, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-post-repair-consensus-pending-21, recorded_at: 2026-07-11T23:35:57Z, actor: Main, expected_parent_sha: 9319eca895bd49b21199bfbebc59ac1e839cdf76, precondition: exact_store_call_baseline_authority_and_independent_verifier_PASS, postcondition: post_repair_default_panel_requested_on_exact_repaired_diff, external_operation_id: agentic_consensus_S0_post_repair_20260711_02}
        - {status: consensus, transition_id: s0-lock-takeover-22, recorded_at: 2026-07-12T00:38:32Z, actor: Main, expected_parent_sha: 0873733302bea93e3f8278fc8b830ea005564809, precondition: epoch_4_expired_origin_main_matches_HEAD_no_external_effect_pending_and_one_attributable_dirty_evidence_path, postcondition: epoch_5_acquired_after_nonce_job_output_and_dirty_path_reconciliation, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-post-panel-blocked-23, recorded_at: 2026-07-12T00:39:43Z, actor: Main, expected_parent_sha: 2a200ecc7c96a22476e97ecb85e731e03f40ff71, precondition: six_panel_outputs_complete_Devin_stalled_and_runner_deadline_elapsed, postcondition: interface_and_method_value_bypasses_confirmed_blocking_while_stalled_member_does_not_stall_suite, external_operation_id: agentic_consensus_S0_post_repair_20260711_02}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-indirect-calls-repair-returned-24, recorded_at: 2026-07-12T00:50:05Z, actor: Main, expected_parent_sha: 12924ef57eef5e3004e9a74806722aeebc4fc291, precondition: method_value_interface_and_receiver_scope_repairs_integrated_and_local_passed, postcondition: S0_POST_001_and_002_repaired_pending_independent_verification, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-interface-provenance-failed-25, recorded_at: 2026-07-12T00:52:47Z, actor: S0RatchetVerifier, expected_parent_sha: 40fd3321feeb081f40044f50658d705e148f5d3a, precondition: independent_verification_of_method_value_and_interface_repairs, postcondition: same_signature_unrelated_interface_false_positive_recorded, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-interface-provenance-repair-returned-26, recorded_at: 2026-07-12T01:00:05Z, actor: Main, expected_parent_sha: 01731bc507b84dd27d564f1ce2f8dfd5793fe31d, precondition: concrete_Store_flow_provenance_analysis_integrated_and_local_passed, postcondition: S0_POST_003_repaired_pending_independent_verification, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-final-panel-pending-27, recorded_at: 2026-07-12T01:02:43Z, actor: Main, expected_parent_sha: 0a391d0848b7390e7b34847020c3ed7bf28cb3d1, precondition: S0_POST_001_through_003_repaired_and_independent_verifier_PASS, postcondition: six_member_non_stalled_final_panel_requested_with_Cursor_included_and_Devin_excluded, external_operation_id: agentic_consensus_S0_final_20260712_03}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-final-panel-blocked-28, recorded_at: 2026-07-12T01:21:57Z, actor: Main, expected_parent_sha: 8caab0c153dad6d8b6aff25727f187d8101ea531, precondition: six_non_stalled_panel_members_completed_with_Cursor_ok, postcondition: return_conversion_composite_interface_bypasses_clustered_to_bespoke_provenance_substrate_and_candidate_authority_selected, external_operation_id: agentic_consensus_S0_final_20260712_03}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-candidate-authority-returned-29, recorded_at: 2026-07-12T01:36:47Z, actor: Main, expected_parent_sha: e50bff04f3cdf537cd52f40b38ecab395dd9822a, precondition: conservative_candidate_authority_repair_integrated, postcondition: local_focused_and_baseline_PASS_with_461_store_calls_4_interface_candidates_151_citers_pending_independent_verification, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-post-substrate-panel-pending-30, recorded_at: 2026-07-12T01:41:17Z, actor: Main, expected_parent_sha: 56ef34cec6ee02bbf77883c6b0f7831abc82fb7e, precondition: candidate_authority_focused_and_independent_verification_PASS, postcondition: final_six_member_non_stalled_post_substrate_panel_requested, external_operation_id: agentic_consensus_S0_post_substrate_20260712_04}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-post-substrate-blocked-31, recorded_at: 2026-07-12T01:56:34Z, actor: Main, expected_parent_sha: 63b55b4e4f8675fd1fa20c17b56870ae734ba37a, precondition: six_non_stalled_members_completed_with_Cursor_ok, postcondition: non_store_negative_authority_writer_laundering_and_promoted_interface_bypass_recorded_for_single_authority_repair, external_operation_id: agentic_consensus_S0_post_substrate_20260712_04}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-semantic-authority-returned-32, recorded_at: 2026-07-12T02:10:38Z, actor: Main, expected_parent_sha: 7994dfa62e3e9ba8420a5bb4810aae9be87a4ae1, precondition: final_authority_repair_integrated, postcondition: local_focused_and_baseline_PASS_with_exhaustive_called_method_semantics_all_candidates_conservative_and_promoted_interface_coverage_pending_independent_verification, external_operation_id: not_applicable}
        - {status: consensus, transition_id: s0-runtime-inventory-ratchet-final-authority-panel-pending-33, recorded_at: 2026-07-12T02:14:34Z, actor: Main, expected_parent_sha: 45f25ac953ea262d3836bc269cf54372f576fc7f, precondition: final_semantic_authority_focused_and_independent_verification_PASS, postcondition: final_post_authority_six_member_non_stalled_panel_requested, external_operation_id: agentic_consensus_S0_final_authority_20260712_05}
        - {status: adjudicated, transition_id: s0-runtime-inventory-ratchet-adjudicated-34, recorded_at: 2026-07-12T02:34:34Z, actor: Main, expected_parent_sha: d8e637382d9906d9693c047eb0a8c2dd735ffb8a, precondition: four_substantive_final_panel_PASS_two_incomplete_no_blocker_independent_verifier_PASS_and_required_CI_PASS, postcondition: S0_checkpoint_PASS_ready_for_landing_transition, external_operation_id: agentic_consensus_S0_final_authority_20260712_05}
        - {status: landed, transition_id: s0-runtime-inventory-ratchet-landed-35, recorded_at: 2026-07-12T02:37:02Z, actor: Main, expected_parent_sha: 2327fcef4716aef070eb4b819296f01b44267364, precondition: S0_checkpoint_adjudicated_PASS, postcondition: S0_complete_S1_working_lock_epoch_6_acquired, external_operation_id: not_applicable}
      lock_expires_at: 2026-07-12T02:38:32Z
      mutation_class: yellow
      protected_surfaces: []
      exact_files_packages_routes_state_authorities: [cmd/runtime-ratchet/**, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [internal/runtime/**, runtime_production_callers, route_registrations, tool_registrations, run_lifecycle_authority, Wire_authority, promotion_authority, suite_and_registry_docs, CI, deployment]
      authority_edges_locked: [runtime_disposition_inventory, runtime_dissolution_ratchet_baseline]
      implementer_agent: S0RatchetImplementer
      verifier_agent: S0RatchetVerifier
      pre_mutation_sha: 1a9a90b63f6541fcb8d96502e85a158b8446d14e
      rollback_commit_or_ref: 1a9a90b63f6541fcb8d96502e85a158b8446d14e
      accepted_slice_dependency_refs: [B0@008a7b88cf200119c0f762cc51cfba6be3007445]
      external_operation_id: not_applicable_yellow_slice
      effect_authority: canonical_git_ref
      receipt_lookup: git_history_and_agent_job_record
      expected_precondition: clean_agent_worktree_at_pre_mutation_sha
      observed_postcondition: semantic_authority_repair_22b50a1dec2e0ee42e98bf542a4a2729ea068118_integrated_as_7994dfa6_with_461_store_calls_4_interface_candidates_151_citers_and_local_pass
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: 22b50a1dec2e0ee42e98bf542a4a2729ea068118
      implementation_commit_sha: 22b50a1dec2e0ee42e98bf542a4a2729ea068118
      push_ref: d8e637382d9906d9693c047eb0a8c2dd735ffb8a@origin/main
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29176500535
      deploy_ref: not_applicable_yellow_slice
      deployed_sha: not_applicable_yellow_slice
      acceptance_ref: artifact://461; agent://S0RatchetVerifier
      acceptance_contract: go_test_cmd_runtime_ratchet_and_baseline_invocation_pass_with_regression_fixtures_failing
      evidence_refs: [docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md, docs/evidence/choir-autoputer-s0-consensus-2026-07-11.md]
      open_findings: []
      landed_commit_sha: 2327fcef4716aef070eb4b819296f01b44267364
      adjudication: PASS_four_substantive_panel_verdicts_no_blockers_two_incomplete_outputs_no_vote_independent_verifier_and_CI_PASS
      last_reconciled_at: 2026-07-12T02:37:02Z
      reconciliation_result: S0_complete_S1_working
      close_condition: independently_verified_inventory_and_ratchet_landed_then_S0_consensus_adjudicated
    - slice_id: S1-deploy-unblock-01
      subgoal: S1
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 7
      status: landed
      dispatch_nonce: s1-deploy-unblock-01-nonce-01
      dispatch_ref: S1DeployImplementer
      agent_session_ref: agent://S1DeployImplementer
      dispatch_prompt_ref: docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md
      implementer_job_ref: S1DeployImplementer
      implementer_output_ref: agent://S1DeployImplementer
      verifier_job_ref: S1DeployVerifier
      verifier_output_ref: agent://S1DeployVerifier
      worktree_or_branch_ref: s1-deploy-unblock-implementer@47abce2a4de850a64cd121f63a24d8048eca7bc9
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: 063d42aef8df4e59101a2ed2eed20f8185d9fb31
      lock_release_ref: 9dff3690
      stage_started_at: 2026-07-12T02:47:34Z
      transition_id: s1-landed-s2-started-49
      expected_parent_sha: 9dff3690
      stage_history:
        - {status: dispatch_intent, transition_id: s1-deploy-unblock-dispatch-intent-36, recorded_at: 2026-07-12T02:47:34Z, actor: Main, expected_parent_sha: 063d42aef8df4e59101a2ed2eed20f8185d9fb31, precondition: S0_complete_S1_working_lock_epoch_6_held_and_red_ceremony_recorded, postcondition: exact_mutation_lock_existing_replacement_connection_and_acceptance_contract_are_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s1-deploy-unblock-dispatched-37, recorded_at: 2026-07-12T02:49:53Z, actor: Main, expected_parent_sha: f05b065b46b3fa734e91b1393b57c77c70ba3b9b, precondition: canonical_dispatch_intent_and_live_lock_epoch_6, postcondition: S1DeployImplementer_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s1-deploy-unblock-returned-38a, recorded_at: 2026-07-12T03:14:45Z, actor: S1DeployImplementer, expected_parent_sha: a47cecef55dadb768e55475e313cc89b14121e10, precondition: one_matching_agent_result_for_dispatch_nonce, postcondition: isolated_commit_47abce2a_recorded_and_integrated, external_operation_id: not_applicable}
        - {status: committed, transition_id: s1-deploy-unblock-committed-38, recorded_at: 2026-07-12T03:14:45Z, actor: Main, expected_parent_sha: a47cecef55dadb768e55475e313cc89b14121e10, precondition: implementation_integrated_focused_tests_and_S0_ratchet_PASS, postcondition: canonical_code_ready_for_push_and_independent_verifier_dispatch, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s1-deploy-unblock-pushed-39a, recorded_at: 2026-07-12T03:17:32Z, actor: Main, expected_parent_sha: 26d7aa2a96e8748b63afcd4074636eb8b563994e, precondition: canonical_commit_and_doccheck_PASS, postcondition: origin_main_contains_S1_implementation_and_verifier_intent_can_dispatch, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s1-deploy-unblock-verifying-39, recorded_at: 2026-07-12T03:17:32Z, actor: Main, expected_parent_sha: 26d7aa2a96e8748b63afcd4074636eb8b563994e, precondition: pushed_S1_implementation_and_exact_verifier_contract, postcondition: independent_S1DeployVerifier_started, external_operation_id: not_applicable}
        - {status: blocked_incomplete, transition_id: s1-deploy-unblock-verification-failed-40, recorded_at: 2026-07-12T03:21:53Z, actor: S1DeployVerifier, expected_parent_sha: e649ee28c4661071a07526637c82585b7a7a9b9f, precondition: independent_read_only_verification_of_canonical_S1, postcondition: stale_two_prospective_citer_entries_make_default_ratchet_fail_165_vs_163_while_lifecycle_behavior_passes, external_operation_id: not_applicable}
        - {status: committed, transition_id: s1-ratchet-baseline-repaired-41, recorded_at: 2026-07-12T03:25:20Z, actor: Main, expected_parent_sha: 210800a0eb56b1f7e7fd9a424d1d8c1d2a4591f0, precondition: S1_VER_001_reproduced_and_documented, postcondition: final_canonical_inventory_regenerated_default_ratchet_PASS_and_independent_reverification_PASS, external_operation_id: not_applicable}
        - {status: deployed, transition_id: s1-deploy-unblock-deployed-41a, recorded_at: 2026-07-12T03:54:15Z, actor: GitHub_Actions, expected_parent_sha: 26d7aa2accda63e20daa19c42381d13aec14baed, precondition: full_CI_gates_green_and_stale_runs_passivated, postcondition: activation_receipt_records_ordinary_guest_sandbox_active_computers_and_gateway_at_S1_commit, external_operation_id: github_actions_run_29178010201_attempt_3}
        - {status: accepted, transition_id: s1-deploy-unblock-accepted-42, recorded_at: 2026-07-12T03:57:52Z, actor: Main, expected_parent_sha: 210800a0eb56b1f7e7fd9a424d1d8c1d2a4591f0, precondition: green_deploy_identity_and_owner_scoped_product_routes, postcondition: list_observed_active_run_cancel_returned_200_and_durable_state_cancelled_with_finished_at, external_operation_id: run_8d203e02-29b7-4f6b-a7e2-bfb95434cf9d}
        - {status: blocked_incomplete, transition_id: s1-final-consensus-blocked-43, recorded_at: 2026-07-12T04:04:00Z, actor: Codex_consensus_reviewer, expected_parent_sha: 76a26022d90554c6f4c43bd2fceb7eaf8abc6d86, precondition: deployed_acceptance_and_final_S1_checkpoint_panel, postcondition: S1_CONS_001_confirmed_passivation_direct_write_can_overwrite_cancelled_or_failed_terminal_state, external_operation_id: consensus_dir_tmp_choir_s1_final_consensus_20260712}
        - {status: committed, transition_id: s1-consensus-race-repaired-45, recorded_at: 2026-07-12T04:24:23Z, actor: Main, expected_parent_sha: 4973ee40570382c25398ea50e15148569cf351ab, precondition: S1_CONS_001_documented_and_reproduced, postcondition: idle_passivation_uses_stored_terminal_wins_guard_regression_and_ratchet_PASS_independent_verifier_PASS, external_operation_id: not_applicable}
        - {status: deployed, transition_id: s1-consensus-race-repair-deployed-46, recorded_at: 2026-07-12T04:37:20Z, actor: GitHub_Actions, expected_parent_sha: 4973ee40570382c25398ea50e15148569cf351ab, precondition: full_CI_and_race_shards_green, postcondition: sandbox_and_gateway_activation_receipt_at_repair_commit, external_operation_id: github_actions_run_29179656372}
        - {status: accepted, transition_id: s1-consensus-race-repair-accepted-46a, recorded_at: 2026-07-12T04:49:30Z, actor: Main, expected_parent_sha: 4973ee40570382c25398ea50e15148569cf351ab, precondition: repair_deployed_and_product_CLI_available, postcondition: actual_choir_run_cancel_returned_cancelled_for_two_active_runs, external_operation_id: runs_2d37e688_and_7b0cb532}
        - {status: consensus, transition_id: s1-post-repair-consensus-open-47, recorded_at: 2026-07-12T04:51:11Z, actor: Main, expected_parent_sha: 4973ee40570382c25398ea50e15148569cf351ab, precondition: repaired_green_deployed_independently_verified_and_CLI_accepted, postcondition: six_member_post_repair_panel_running, external_operation_id: consensus_dir_tmp_choir_s1_post_repair_consensus_20260712}
        - {status: adjudicated, transition_id: s1-post-repair-consensus-adjudicated-48, recorded_at: 2026-07-12T05:15:00Z, actor: Main, expected_parent_sha: 9dff3690, precondition: four_explicit_PASS_no_completed_blocker_one_incomplete_no_verdict_one_stalled_no_output_and_final_ratchet_PASS, postcondition: S1_CONS_001_repaired_and_S1_checkpoint_PASS, external_operation_id: consensus_dir_tmp_choir_s1_post_repair_consensus_20260712}
        - {status: landed, transition_id: s1-landed-s2-started-49, recorded_at: 2026-07-12T05:15:00Z, actor: Main, expected_parent_sha: 9dff3690, precondition: S1_all_exit_evidence_green_and_no_open_findings, postcondition: S1_complete_S2_working_lock_epoch_8_acquired, external_operation_id: not_applicable}
      lock_expires_at: 2026-07-12T07:15:00Z
      mutation_class: red
      protected_surfaces: [run_acceptance, admission_occupancy, owner_scoped_cancellation, choir_run_CLI, staging_hot_refresh_deploy]
      exact_files_packages_routes_state_authorities: [internal/provideriface/provider.go, internal/runtime/config.go, internal/runtime/config_test.go, internal/runtime/runtime.go, internal/runtime/runtime_test.go, internal/runtime/api.go, internal/runtime/api_test.go, cmd/choir/main.go, cmd/choir/main_test.go, docs/runtime-dissolution-inventory.yaml, RunRecord.State, Runtime.CancelRun, /api/agent/loops, /api/agent/cancel]
      forbidden_targets: [second_lifecycle_state_machine, admission_counter_rewrite, retry_policy, VM_reprovisioning, Wire_authority, promotion_authority, deployment_configuration]
      authority_edges_locked: [RunRecord.State_single_lifecycle_authority, Runtime.CancelRun_owner_scoped_transition, S0_runtime_ratchets]
      implementer_agent: S1DeployImplementer
      verifier_agent: S1DeployVerifier
      pre_mutation_sha: 063d42aef8df4e59101a2ed2eed20f8185d9fb31
      rollback_commit_or_ref: 063d42aef8df4e59101a2ed2eed20f8185d9fb31
      accepted_slice_dependency_refs: [S0@2327fcef4716aef070eb4b819296f01b44267364]
      external_operation_id: not_applicable_before_staging_drain
      effect_authority: canonical_git_ref_then_staging_product_API
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: existing_cancel_and_list_handlers_unwired_active_execution_unbounded_staging_deployed_at_6e893d90
      observed_postcondition: routes_and_CLI_connected_activation_budget_60m_immediate_terminal_cancellation_and_late_write_guard_integrated_with_focused_and_ratchet_PASS
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: 47abce2a4de850a64cd121f63a24d8048eca7bc9
      implementation_commit_sha: 47abce2a4de850a64cd121f63a24d8048eca7bc9
      push_ref: 4973ee40570382c25398ea50e15148569cf351ab@origin/main
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29179656372
      deploy_ref: activation_receipt_29179656372_attempt_1_at_2026-07-12T04:37:20Z
      deployed_sha: 4973ee40570382c25398ea50e15148569cf351ab
      acceptance_ref: docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md#s1-cons-001-repair-receipt
      acceptance_contract: owner_scoped_product_CLI_cancel_and_60m_activation_budget_terminalize_runs_release_admission_and_restore_hot_refresh
      evidence_refs: [docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md, agent://S1DeployVerifier, https://github.com/choir-hip/go-choir/actions/runs/29179656372, /tmp/choir-s1-final-consensus-20260712, /tmp/choir-s1-post-repair-consensus-20260712]
      open_findings: []
      landed_commit_sha: 9dff3690
      adjudication: PASS_S1_CONS_001_repaired_four_explicit_panel_PASS_no_completed_blocker_final_canonical_ratchet_PASS
      last_reconciled_at: 2026-07-12T05:15:00Z
      reconciliation_result: S1_complete_and_S2_authorized
      close_condition: staging_running_runs_zero_or_authoritative_equivalent_green_deployed_commit_cancel_deadline_regressions_independent_verification_and_consensus_adjudication
    - slice_id: S2-A-delete-runtime-migration
      subgoal: S2
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 8
      status: landed
      dispatch_nonce: s2-wire-authority-cutover-01-nonce-01-A
      dispatch_ref: S2MigrationDelete
      agent_session_ref: agent://S2MigrationDelete
      dispatch_prompt_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#s2-a--delete-boot-time-retired-sql-replay
      implementer_job_ref: S2MigrationDelete
      implementer_output_ref: agent://S2MigrationDelete
      verifier_job_ref: S2RepairVerifier
      verifier_output_ref: agent://S2MigrationVerifier
      worktree_or_branch_ref: s2-a-delete-replay@9fcfc978d14f5e5a9eafa216ec86609d877b6145
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      lock_release_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      stage_started_at: 2026-07-12T05:21:52Z
      transition_id: s2-a-dispatch-intent-50
      expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      stage_history:
        - {status: dispatch_intent, transition_id: s2-a-dispatch-intent-50, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591, precondition: S1_complete_S2_working_fresh_inventory_recorded, postcondition: exact_migration_deletion_slice_is_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s2-a-dispatched-51, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: 5da44349, precondition: canonical_dispatch_intent_and_live_lock_epoch_8, postcondition: S2MigrationDelete_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s2-a-returned-52, recorded_at: 2026-07-12T06:26:17Z, actor: S2MigrationDelete, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: one_matching_agent_result, postcondition: isolated_commit_integrated_and_focused_store_sandbox_tests_PASS, external_operation_id: not_applicable}
        - {status: committed, transition_id: s2-a-committed-53, recorded_at: 2026-07-12T06:26:17Z, actor: Main, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: integrated_atomic_S2_and_final_ratchet_PASS, postcondition: canonical_code_ready_for_push_and_independent_verification, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s2-a-pushed-54, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: integrated_atomic_S2_doccheck_and_ratchet_PASS, postcondition: origin_main_contains_S2_implementation, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s2-a-verifying-55, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: pushed_S2_implementation_and_exact_verifier_contract, postcondition: independent_S2IndependentVerifier_authorized, external_operation_id: not_applicable}
        - {status: landed, transition_id: s2-a-landed-61, recorded_at: 2026-07-12T10:13:27Z, actor: Main, expected_parent_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50, precondition: S2_repairs_deployed_independent_verifiers_PASS_consensus_findings_repaired, postcondition: S2_A_complete_and_S3_authorized, external_operation_id: github_run_29188248479}
      lock_expires_at: 2026-07-12T07:15:00Z
      mutation_class: red
      protected_surfaces: [VM_lifecycle, VM_local_private_store]
      exact_files_packages_routes_state_authorities: [internal/store/migration.go, internal/store/migration_test.go, internal/store/store.go, cmd/sandbox/main.go, cmd/sandbox/main_test.go, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [migration_shim, compatibility_alias, feature_flag, corpusd_state, production_deploy_before_atomic_S2_landing]
      authority_edges_locked: [VM_local_private_state_only, no_boot_time_retired_SQL_replay]
      implementer_agent: S2MigrationDelete
      verifier_agent: S2RepairVerifier
      pre_mutation_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      rollback_commit_or_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      accepted_slice_dependency_refs: [S1@9dff3690]
      external_operation_id: not_applicable
      effect_authority: canonical_git_ref_then_staging_VM_boot
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: sandbox_boot_replays_retired_relational_rows_into_VM_local_objectgraph
      observed_postcondition: relational_objectgraph_replay_APIs_tables_background_loop_and_dead_helpers_deleted_store_and_sandbox_tests_PASS
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: 9fcfc978d14f5e5a9eafa216ec86609d877b6145
      implementation_commit_sha: e96655a82e6aa32088200c16ab91960492b89ffa
      push_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29188248479
      deploy_ref: activation_receipt_29188248479_sandbox_b7b1262e_at_2026-07-12T10:00:44Z
      deployed_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      acceptance_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#s2-cons-001-repair-receipt
      acceptance_contract: sandbox_boot_has_no_retired_SQL_replay_API_or_background_loop
      evidence_refs: [docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md]
      open_findings: []
      landed_commit_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      adjudication: PASS_S2_CONS_001_and_deletion_citers_repaired
      last_reconciled_at: 2026-07-12T10:13:27Z
      reconciliation_result: S2_A_complete
      close_condition: integrated_with_S2_B_and_S2_C_independently_verified_deployed_accepted_consensus_adjudicated
    - slice_id: S2-B-corpusd-wire-read-edition
      subgoal: S2
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 8
      status: landed
      dispatch_nonce: s2-wire-authority-cutover-01-nonce-01-B
      dispatch_ref: S2CorpusdWireAuthority
      agent_session_ref: agent://S2CorpusdWireAuthority
      dispatch_prompt_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#s2-b--make-corpusd-the-only-public-wire-read-and-edition-authority
      implementer_job_ref: S2CorpusdWireAuthority
      implementer_output_ref: agent://S2CorpusdWireAuthority
      verifier_job_ref: S2RepairVerifier
      verifier_output_ref: agent://S2LifecycleVerifier
      worktree_or_branch_ref: s2-b-corpusd-wire@b3da23bba9b5c4b9b7a343d4f26dc0c72173bcd4
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      lock_release_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      stage_started_at: 2026-07-12T05:21:52Z
      transition_id: s2-b-dispatch-intent-50
      expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      stage_history:
        - {status: dispatch_intent, transition_id: s2-b-dispatch-intent-50, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591, precondition: S1_complete_S2_working_fresh_inventory_recorded, postcondition: exact_corpusd_read_and_edition_cutover_slice_is_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s2-b-dispatched-51, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: 5da44349, precondition: canonical_dispatch_intent_and_live_lock_epoch_8, postcondition: S2CorpusdWireAuthority_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s2-b-returned-52, recorded_at: 2026-07-12T06:26:17Z, actor: S2CorpusdWireAuthority, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: one_matching_agent_result, postcondition: corpusd_read_proxy_route_and_local_edition_deletion_integrated_focused_tests_PASS, external_operation_id: not_applicable}
        - {status: committed, transition_id: s2-b-committed-53, recorded_at: 2026-07-12T06:26:17Z, actor: Main, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: integrated_atomic_S2_and_final_ratchet_PASS, postcondition: canonical_code_ready_for_push_and_independent_verification, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s2-b-pushed-54, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: integrated_atomic_S2_doccheck_and_ratchet_PASS, postcondition: origin_main_contains_S2_implementation, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s2-b-verifying-55, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: pushed_S2_implementation_and_exact_verifier_contract, postcondition: independent_S2IndependentVerifier_authorized, external_operation_id: not_applicable}
        - {status: blocked_incomplete, transition_id: s2-b-verification-failed-56, recorded_at: 2026-07-12T06:33:00Z, actor: S2IndependentVerifier, expected_parent_sha: 97dc05f7, precondition: independent_source_authority_review, postcondition: S2_VER_001_retained_VM_local_edition_read_gate_documented, external_operation_id: agent_S2IndependentVerifier}
        - {status: committed, transition_id: s2-b-read-authority-repaired-57, recorded_at: 2026-07-12T06:50:00Z, actor: Main, expected_parent_sha: 08803bb2, precondition: S2_VER_001_documented_before_fix, postcondition: cross_owner_runtime_read_exception_deleted_owner_scope_regression_and_ratchet_PASS, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s2-b-reverifying-58, recorded_at: 2026-07-12T06:50:00Z, actor: Main, expected_parent_sha: 08803bb2, precondition: repair_pushed_focused_tests_and_ratchet_PASS, postcondition: independent_S2RepairVerifier_authorized, external_operation_id: not_applicable}
        - {status: landed, transition_id: s2-b-landed-61, recorded_at: 2026-07-12T10:13:27Z, actor: Main, expected_parent_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50, precondition: corpusd_only_Wire_authority_deployed_product_proof_and_consensus_repaired, postcondition: S2_B_complete_and_S3_authorized, external_operation_id: github_run_29188248479}
      lock_expires_at: 2026-07-12T07:15:00Z
      mutation_class: red
      protected_surfaces: [corpusd_canonical_writes, public_wire_reads, runtime_publication_settlement, proxy_routing]
      exact_files_packages_routes_state_authorities: [internal/platform, internal/proxy, internal/runtime/universal_wire.go, internal/runtime/wire_publication.go, /api/universal-wire/stories, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [dual_read, dual_write, backfill, runtime_fallback, compatibility_alias, third_store]
      authority_edges_locked: [corpusd_only_world_wire_authority, user_computer_private_working_state_only]
      implementer_agent: S2CorpusdWireAuthority
      verifier_agent: S2RepairVerifier
      pre_mutation_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      rollback_commit_or_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      accepted_slice_dependency_refs: [S1@9dff3690]
      external_operation_id: not_applicable
      effect_authority: canonical_git_ref_then_staging_product_API
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: corpusd_publication_exists_but_runtime_local_edition_and_story_read_remain_authoritative
      observed_postcondition: corpusd_canonical_publications_serve_product_story_contract_proxy_bypasses_VM_runtime_story_route_and_local_edition_deleted
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: b3da23bba9b5c4b9b7a343d4f26dc0c72173bcd4
      implementation_commit_sha: e96655a82e6aa32088200c16ab91960492b89ffa
      push_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29188248479
      deploy_ref: activation_receipt_29188248479_sandbox_b7b1262e_at_2026-07-12T10:00:44Z
      deployed_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      acceptance_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#s2-d-deployed-acceptance-receipt
      acceptance_contract: canonical_corpusd_publications_render_existing_story_contract_without_runtime_local_edition
      evidence_refs: [docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md]
      open_findings: []
      landed_commit_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      adjudication: PASS_corpusd_only_Wire_authority_stopped_VM_feed_authenticated_browser
      last_reconciled_at: 2026-07-12T10:13:27Z
      reconciliation_result: S2_B_complete
      close_condition: integrated_with_S2_A_and_S2_C_independently_verified_deployed_accepted_consensus_adjudicated
    - slice_id: S2-C-source-captures-to-corpusd
      subgoal: S2
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 8
      status: landed
      dispatch_nonce: s2-wire-authority-cutover-01-nonce-01-C
      dispatch_ref: S2SourceCaptureCutover
      agent_session_ref: agent://S2SourceCaptureCutover
      dispatch_prompt_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#s2-c--publish-source-captures-directly-to-corpusd
      implementer_job_ref: S2SourceCaptureCutover
      implementer_output_ref: agent://S2SourceCaptureCutover
      verifier_job_ref: S2RepairVerifier
      verifier_output_ref: agent://S2LifecycleVerifier
      worktree_or_branch_ref: s2-c-source-capture@6c31805830d6596c9a1bf6fd9f5bea76d9d79e78
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      lock_release_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      stage_started_at: 2026-07-12T05:21:52Z
      transition_id: s2-c-dispatch-intent-50
      expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      stage_history:
        - {status: dispatch_intent, transition_id: s2-c-dispatch-intent-50, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591, precondition: S1_complete_S2_working_fresh_inventory_recorded, postcondition: exact_source_capture_corpusd_cutover_slice_is_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s2-c-dispatched-51, recorded_at: 2026-07-12T05:21:52Z, actor: Main, expected_parent_sha: 5da44349, precondition: canonical_dispatch_intent_and_live_lock_epoch_8, postcondition: S2SourceCaptureCutover_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s2-c-returned-52, recorded_at: 2026-07-12T06:26:17Z, actor: S2SourceCaptureCutover, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: one_matching_agent_result, postcondition: source_capture_host_publication_integrated_and_focused_tests_PASS, external_operation_id: not_applicable}
        - {status: committed, transition_id: s2-c-committed-53, recorded_at: 2026-07-12T06:26:17Z, actor: Main, expected_parent_sha: e96655a82e6aa32088200c16ab91960492b89ffa, precondition: integrated_atomic_S2_and_final_ratchet_PASS, postcondition: canonical_code_ready_for_push_and_independent_verification, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s2-c-pushed-54, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: integrated_atomic_S2_doccheck_and_ratchet_PASS, postcondition: origin_main_contains_S2_implementation, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s2-c-verifying-55, recorded_at: 2026-07-12T06:28:54Z, actor: Main, expected_parent_sha: 5d056d90674505ed241b2cd281202611bc105d0c, precondition: pushed_S2_implementation_and_exact_verifier_contract, postcondition: independent_S2IndependentVerifier_authorized, external_operation_id: not_applicable}
        - {status: landed, transition_id: s2-c-landed-61, recorded_at: 2026-07-12T10:13:27Z, actor: Main, expected_parent_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50, precondition: source_capture_corpusd_cutover_deployed_and_consensus_repaired, postcondition: S2_C_complete_and_S3_authorized, external_operation_id: github_run_29188248479}
      lock_expires_at: 2026-07-12T07:15:00Z
      mutation_class: red
      protected_surfaces: [source_ingestion, corpusd_canonical_writes, VM_lifecycle, runtime_host_proxy]
      exact_files_packages_routes_state_authorities: [cmd/sourcecycled, internal/cycle/web_capture_graph.go, internal/proxy/platform_objectgraph.go, internal/platform/objectgraph_handlers.go, internal/runtime/objectgraph_runtime.go, deployment_configuration, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [VM_boot_for_capture_write, local_fallback, dual_path, third_store, processor_activation_rearchitecture]
      authority_edges_locked: [corpusd_only_shared_capture_authority, runtime_processor_activation_separate_from_capture_persistence]
      implementer_agent: S2SourceCaptureCutover
      verifier_agent: S2RepairVerifier
      pre_mutation_sha: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      rollback_commit_or_ref: d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591
      accepted_slice_dependency_refs: [S1@9dff3690]
      external_operation_id: not_applicable
      effect_authority: canonical_git_ref_then_staging_source_ingestion
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: sourcecycled_shared_capture_projection_calls_user_computer_runtime
      observed_postcondition: sourcecycled_publishes_capture_objects_and_edges_to_host_proxy_corpusd_runtime_capture_route_deleted_no_fallback
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: 6c31805830d6596c9a1bf6fd9f5bea76d9d79e78
      implementation_commit_sha: e96655a82e6aa32088200c16ab91960492b89ffa
      push_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29188248479
      deploy_ref: activation_receipt_29188248479_sandbox_b7b1262e_at_2026-07-12T10:00:44Z
      deployed_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      acceptance_ref: docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md#final-post-repair-consensus-adjudication
      acceptance_contract: sourcecycled_publishes_canonical_capture_graph_without_user_VM_boot_or_runtime_projection_route
      evidence_refs: [docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md]
      open_findings: []
      landed_commit_sha: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      adjudication: PASS_source_capture_corpusd_only_no_runtime_fallback
      last_reconciled_at: 2026-07-12T10:13:27Z
      reconciliation_result: S2_C_complete
      close_condition: integrated_with_S2_A_and_S2_B_independently_verified_deployed_accepted_consensus_adjudicated
    - slice_id: S3-I1-dead-api-handlers
      subgoal: S3
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 11
      status: landed
      dispatch_nonce: s3-runtime-dissolution-i1-nonce-01
      dispatch_ref: S3I1Implementer
      agent_session_ref: agent://S3I1Implementer
      dispatch_prompt_ref: docs/evidence/s3-runtime-dissolution-dispatch-2026-07-12.md#s3-i1--delete-unregistered-runtime-api-handlers
      implementer_job_ref: S3I1Implementer
      implementer_output_ref: agent://S3I1Implementer
      verifier_job_ref: S3I1Verifier
      verifier_output_ref: agent://S3I1Verifier
      worktree_or_branch_ref: agent/s3-i1-dead-api@d3d1b59a2878c2a3b060271e4d8e5aedfdae3beb
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: c4173c6d
      lock_release_ref: retained_by_lock_epoch_11_for_S3_I2
      stage_started_at: 2026-07-12T10:34:34Z
      transition_id: s3-i1-dispatch-intent-62
      expected_parent_sha: af0479db
      stage_history:
        - {status: dispatch_intent, transition_id: s3-i1-dispatch-intent-62, recorded_at: 2026-07-12T10:34:34Z, actor: Main, expected_parent_sha: b1cc1e55, precondition: S2_complete_S3_working_ratchet_PASS_gopls_dead_callers_confirmed, postcondition: exact_deletion_only_slice_is_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s3-i1-dispatched-63, recorded_at: 2026-07-12T10:48:11Z, actor: Main, expected_parent_sha: c4173c6d, precondition: canonical_dispatch_intent_and_live_lock_epoch_10, postcondition: S3I1Implementer_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s3-i1-implementation-returned-64, recorded_at: 2026-07-12T11:15:05Z, actor: S3I1Implementer, expected_parent_sha: c4173c6d, precondition: isolated_branch_and_exact_deletion_lock, postcondition: corrected_deletion_commit_returned_with_non_HTTP_behavior_tests_preserved, external_operation_id: not_applicable}
        - {status: committed, transition_id: s3-i1-committed-65, recorded_at: 2026-07-12T11:15:05Z, actor: Main, expected_parent_sha: c78ece1e, precondition: focused_runtime_smoke_PASS_and_ratchet_rebased_to_lower_counts, postcondition: implementation_integrated_locally_and_durable_ledger_ready_for_push, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s3-i1-pushed-66, recorded_at: 2026-07-12T11:16:28Z, actor: Main, expected_parent_sha: 405a97bc, precondition: implementation_and_ratchet_checkpoint_committed, postcondition: canonical_main_contains_c78ece1e_and_405a97bc, external_operation_id: git_push_origin_main}
        - {status: verifier_dispatch_intent, transition_id: s3-i1-verifier-intent-67, recorded_at: 2026-07-12T11:16:28Z, actor: Main, expected_parent_sha: 405a97bc, precondition: canonical_implementation_and_lower_ratchet_baseline, postcondition: independent_verifier_scope_and_identity_ready_for_dispatch, external_operation_id: pending_S3I1Verifier}
        - {status: verifying, transition_id: s3-i1-verifier-dispatched-68, recorded_at: 2026-07-12T11:18:17Z, actor: Main, expected_parent_sha: 2bb95000, precondition: canonical_verifier_dispatch_intent, postcondition: independent_S3I1Verifier_started_against_canonical_diff, external_operation_id: S3I1Verifier}
        - {status: verified, transition_id: s3-i1-verified-69, recorded_at: 2026-07-12T11:22:00Z, actor: S3I1Verifier, expected_parent_sha: e372abce, precondition: canonical_diff_and_independent_reviewer_identity, postcondition: PASS_confidence_0_91_no_findings_source_contracts_and_ratchet_decrease_confirmed, external_operation_id: S3I1Verifier}
        - {status: ci_pending, transition_id: s3-i1-ci-rerun-intent-70, recorded_at: 2026-07-12T11:22:00Z, actor: Main, expected_parent_sha: e372abce, precondition: behavior_push_CI_29190541541_canceled_by_subsequent_ledger_push, postcondition: same_behavior_run_rerun_only_after_durable_intent, external_operation_id: pending_rerun_29190541541}
        - {status: ci_passed, transition_id: s3-i1-ci-passed-72, recorded_at: 2026-07-12T12:04:42Z, actor: Main, expected_parent_sha: 25ac3ff9, precondition: rerun_attempt_2_for_behavior_checkpoint, postcondition: all_selected_normal_and_race_gates_PASS, external_operation_id: github_actions_29190541541_attempt_2}
        - {status: deployed, transition_id: s3-i1-deployed-73, recorded_at: 2026-07-12T12:04:42Z, actor: Main, expected_parent_sha: 25ac3ff9, precondition: CI_PASS_and_deploy_job_success, postcondition: sandbox_and_gateway_active_at_405a97bc, external_operation_id: activation_receipt_29190541541_attempt_2}
        - {status: accepted, transition_id: s3-i1-accepted-74, recorded_at: 2026-07-12T12:04:42Z, actor: Main, expected_parent_sha: 25ac3ff9, precondition: deployed_identity_405a97bc, postcondition: retired_routes_404_registered_loops_200_registered_prompt_validation_400, external_operation_id: authenticated_staging_probe_2026_07_12T11_37Z}
        - {status: consensus_repair, transition_id: s3-i1-consensus-repair-75, recorded_at: 2026-07-12T12:04:42Z, actor: Main, expected_parent_sha: 25ac3ff9, precondition: panel_three_PASS_one_procedural_BLOCKING, postcondition: ratchet_citer_drift_and_stale_status_comment_repaired_before_final_panel, external_operation_id: /tmp/choir-s3-i1-consensus-20260712}
        - {status: consensus, transition_id: s3-i1-final-consensus-76, recorded_at: 2026-07-12T12:40:47Z, actor: Main, expected_parent_sha: af0479db, precondition: repaired_ratchet_PASS_comment_repaired_CI_29192061906_PASS_deployed_af0479db, postcondition: three_substantive_PASS_one_incomplete_no_confirmed_blocker, external_operation_id: /tmp/choir-s3-i1-final-consensus-20260712}
        - {status: adjudicated, transition_id: s3-i1-adjudicated-77, recorded_at: 2026-07-12T12:40:47Z, actor: Main, expected_parent_sha: af0479db, precondition: final_panel_no_confirmed_blocker, postcondition: S3_I1_all_material_findings_repaired_or_nonblocking, external_operation_id: /tmp/choir-s3-i1-final-consensus-20260712}
        - {status: landed, transition_id: s3-i1-landed-78, recorded_at: 2026-07-12T12:40:47Z, actor: Main, expected_parent_sha: af0479db, precondition: deployed_accepted_verified_consensus_adjudicated_ratchet_PASS, postcondition: S3_step_1_dead_handler_slice_complete_next_S3_step_1_slice_authorized, external_operation_id: not_applicable}
      lock_expires_at: 2026-07-12T14:04:42Z
      mutation_class: orange
      protected_surfaces: [runtime_API_package_surface]
      exact_files_packages_routes_state_authorities: [internal/runtime/api.go, internal/runtime/api_spawn_test.go, internal/runtime/api_test.go, internal/runtime/concurrent_workers_test.go, internal/runtime/failure_isolation_test.go, internal/runtime/test_helpers_test.go, internal/runtime/texture_real_llm_test.go, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [replacement_routes, wrappers, aliases, new_packages, Browser_extraction, live_execution_core, API_bootstrap]
      authority_edges_locked: [registered_product_routes_unchanged, test_helpers_do_not_keep_dead_production_APIs_alive]
      implementer_agent: S3I1Implementer
      verifier_agent: S3I1Verifier
      pre_mutation_sha: c4173c6d
      rollback_commit_or_ref: c4173c6d
      accepted_slice_dependency_refs: [S2@b7b1262e455a779ca00c8d968ef28b3fa6af9b50]
      external_operation_id: not_applicable
      effect_authority: canonical_git_ref_then_staging_sandbox
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: six_unregistered_exported_API_handlers_have_no_production_callers
      observed_postcondition: six_unregistered_handlers_and_handler_only_tests_deleted_registered_routes_byte_identical_non_HTTP_behavior_tests_preserved
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: c78ece1e
      implementation_commit_sha: c78ece1e
      push_ref: af0479db
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29192061906
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/29192061906#job-86649177418
      deployed_sha: af0479db1e2afe0fafb5c3ca017f71c2d85cbdb4
      acceptance_ref: docs/evidence/s3-runtime-dissolution-dispatch-2026-07-12.md#s3-i1-deployment-and-consensus-receipt
      acceptance_contract: six_dead_handlers_deleted_registered_run_and_prompt_surfaces_unchanged
      evidence_refs: [docs/evidence/s3-runtime-dissolution-dispatch-2026-07-12.md, agent://S3I1Implementer, agent://S3I1Verifier, /tmp/choir-s3-i1-final-consensus-20260712]
      open_findings: []
      landed_commit_sha: af0479db1e2afe0fafb5c3ca017f71c2d85cbdb4
      adjudication: PASS_three_substantive_final_verdicts_one_incomplete_output_no_confirmed_blocker
      last_reconciled_at: 2026-07-12T12:40:47Z
      reconciliation_result: S3_I1_landed_next_ordered_dead_surface_slice_authorized
      close_condition: deletion_landed_deployed_product_accepted_independently_verified_consensus_adjudicated_ratchet_decreased
    - slice_id: S3-I2-declaration-only-helpers
      subgoal: S3
      suite_run_id: choir-autoputer-completion-2026-07-11-01
      orchestrator_lock_epoch: 12
      status: consensus_repair
      dispatch_nonce: s3-runtime-dissolution-i2-nonce-01
      dispatch_ref: S3I2Implementer
      agent_session_ref: agent://S3I2Implementer
      dispatch_prompt_ref: docs/evidence/s3-runtime-dead-helper-dispatch-2026-07-12.md#exact-mutation-lock
      implementer_job_ref: S3I2Implementer
      implementer_output_ref: agent://S3I2Implementer
      verifier_job_ref: S3I2Verifier
      verifier_output_ref: agent://S3I2Verifier
      worktree_or_branch_ref: agent/s3-i2-declaration-helpers@6cb224a3b4f148f5d8e0f2f4f1b413bb35823db7
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: 2bc15174
      lock_release_ref: pending_S3_I2_landing
      stage_started_at: 2026-07-12T12:47:48Z
      transition_id: s3-i2-dispatch-intent-79
      expected_parent_sha: 464e58cc
      stage_history:
        - {status: dispatch_intent, transition_id: s3-i2-dispatch-intent-79, recorded_at: 2026-07-12T12:47:48Z, actor: Main, expected_parent_sha: f10b8d98, precondition: S3_I1_landed_ratchet_PASS_three_declaration_only_exports_confirmed, postcondition: exact_deletion_only_helper_slice_is_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s3-i2-dispatched-80, recorded_at: 2026-07-12T12:50:39Z, actor: Main, expected_parent_sha: 2bc15174, precondition: canonical_dispatch_intent_and_live_lock_epoch_11, postcondition: S3I2Implementer_started_with_recorded_nonce_and_exact_mutation_lock, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s3-i2-implementation-returned-81, recorded_at: 2026-07-12T12:58:27Z, actor: S3I2Implementer, expected_parent_sha: 2bc15174, precondition: isolated_branch_and_exact_deletion_lock, postcondition: three_file_27_line_deletion_only_commit_returned, external_operation_id: not_applicable}
        - {status: committed, transition_id: s3-i2-committed-82, recorded_at: 2026-07-12T12:58:27Z, actor: Main, expected_parent_sha: f637c5b8, precondition: focused_promptspec_and_tool_profile_tests_PASS_ratchet_rebased_lower, postcondition: implementation_integrated_locally_and_durable_ledger_ready_for_push, external_operation_id: not_applicable}
        - {status: pushed, transition_id: s3-i2-pushed-83, recorded_at: 2026-07-12T12:59:42Z, actor: Main, expected_parent_sha: 6180be79, precondition: implementation_and_ratchet_checkpoint_committed, postcondition: canonical_main_contains_f637c5b8_and_6180be79, external_operation_id: git_push_origin_main}
        - {status: verifier_dispatch_intent, transition_id: s3-i2-verifier-intent-84, recorded_at: 2026-07-12T12:59:42Z, actor: Main, expected_parent_sha: 6180be79, precondition: canonical_implementation_and_lower_ratchet_baseline, postcondition: independent_verifier_scope_and_identity_ready_for_dispatch, external_operation_id: pending_S3I2Verifier}
        - {status: verifying, transition_id: s3-i2-verifier-dispatched-85, recorded_at: 2026-07-12T13:01:01Z, actor: Main, expected_parent_sha: caef6bdd, precondition: canonical_verifier_dispatch_intent, postcondition: independent_S3I2Verifier_started_against_canonical_diff, external_operation_id: S3I2Verifier}
        - {status: verified, transition_id: s3-i2-verified-86, recorded_at: 2026-07-12T13:06:59Z, actor: S3I2Verifier, expected_parent_sha: 4a94c05f, precondition: canonical_diff_and_independent_reviewer_identity, postcondition: PASS_confidence_0_97_no_findings_exact_scope_and_ratchet_decrease_confirmed, external_operation_id: S3I2Verifier}
        - {status: ci_pending, transition_id: s3-i2-ci-rerun-intent-87, recorded_at: 2026-07-12T13:06:59Z, actor: Main, expected_parent_sha: 4a94c05f, precondition: behavior_push_CI_29193594601_canceled_by_subsequent_ledger_push, postcondition: same_behavior_run_rerun_only_after_durable_intent, external_operation_id: pending_rerun_29193594601}
        - {status: ci_passed, transition_id: s3-i2-ci-passed-89, recorded_at: 2026-07-12T13:35:02Z, actor: Main, expected_parent_sha: 195ef87b, precondition: rerun_attempt_2_for_behavior_checkpoint, postcondition: all_selected_normal_and_race_gates_PASS, external_operation_id: github_actions_29193594601_attempt_2}
        - {status: deployed, transition_id: s3-i2-deployed-90, recorded_at: 2026-07-12T13:35:02Z, actor: Main, expected_parent_sha: 195ef87b, precondition: CI_PASS_and_deploy_job_success, postcondition: sandbox_and_gateway_active_at_6180be79, external_operation_id: activation_receipt_29193594601_attempt_2}
        - {status: accepted, transition_id: s3-i2-accepted-91, recorded_at: 2026-07-12T13:35:02Z, actor: Main, expected_parent_sha: 195ef87b, precondition: deployed_identity_6180be79, postcondition: authenticated_registered_run_list_200_and_all_deploy_health_checks_PASS, external_operation_id: staging_probe_2026_07_12T13_22Z}
        - {status: consensus, transition_id: s3-i2-consensus-intent-92, recorded_at: 2026-07-12T13:35:02Z, actor: Main, expected_parent_sha: 195ef87b, precondition: deployed_accepted_verified_ratchet_PASS, postcondition: post_implementation_consensus_ready, external_operation_id: pending_agentic_consensus_S3_I2}
        - {status: consensus, transition_id: s3-i2-consensus-blocked-93, recorded_at: 2026-07-12T13:55:41Z, actor: Main, expected_parent_sha: 464e58cc, precondition: four_reviewer_panel_complete, postcondition: three_PASS_one_confirmed_receipt_citer_blocker, external_operation_id: /tmp/choir-s3-i2-consensus-20260712}
        - {status: consensus_repair, transition_id: s3-i2-consensus-repair-94, recorded_at: 2026-07-12T13:55:41Z, actor: Main, expected_parent_sha: 464e58cc, precondition: receipt_literal_created_one_new_runtime_citer, postcondition: command_wording_normalized_and_citer_count_restored_to_192_pending_final_panel, external_operation_id: not_applicable}
      lock_expires_at: 2026-07-12T15:35:02Z
      mutation_class: orange
      protected_surfaces: []
      exact_files_packages_routes_state_authorities: [internal/runtime/promptspec/promptspec.go, internal/runtime/runtime.go, internal/runtime/tool_profiles.go, docs/runtime-dissolution-inventory.yaml]
      forbidden_targets: [replacement_helpers, aliases, wrappers, new_packages, routes, config, bootstrap, live_tool_loop, Browser_extraction, promotion_candidate_mutation]
      authority_edges_locked: [registered_routes_unchanged, tool_registrations_unchanged, state_authorities_unchanged]
      implementer_agent: S3I2Implementer
      verifier_agent: S3I2Verifier
      pre_mutation_sha: 2bc15174
      rollback_commit_or_ref: 2bc15174
      accepted_slice_dependency_refs: [S3-I1@af0479db1e2afe0fafb5c3ca017f71c2d85cbdb4]
      external_operation_id: not_applicable
      effect_authority: canonical_git_ref_then_staging_sandbox
      receipt_lookup: git_history_agent_job_record_GitHub_Actions_staging_product_API
      expected_precondition: three_exports_have_declaration_only_reference_sets
      observed_postcondition: three_declaration_only_helpers_deleted_no_route_tool_registration_or_state_authority_line_changed
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: f637c5b8
      implementation_commit_sha: f637c5b8
      push_ref: 6180be79
      ci_run_ref: https://github.com/choir-hip/go-choir/actions/runs/29193594601#attempt-2
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/29193594601#job-86654222017
      deployed_sha: 6180be797ad264d345c5a2bf328c93748363df1a
      acceptance_ref: docs/evidence/s3-runtime-dead-helper-dispatch-2026-07-12.md#s3-i2-implementation-and-verification-receipt
      acceptance_contract: three_dead_helpers_deleted_no_route_tool_or_state_authority_change
      evidence_refs: [docs/evidence/s3-runtime-dead-helper-dispatch-2026-07-12.md, agent://S3I2Implementer, agent://S3I2Verifier]
      open_findings: [final_post_repair_consensus_pending]
      landed_commit_sha: pending
      adjudication: three_PASS_one_confirmed_citer_blocker_repaired_final_panel_pending
      last_reconciled_at: 2026-07-12T13:55:41Z
      reconciliation_result: consensus_blocker_repaired_ratchet_rebase_pending
      close_condition: deletion_landed_deployed_product_accepted_independently_verified_consensus_adjudicated_ratchet_decreased
  s1_runtime_exception_disposition:
    - {path: internal/runtime/config.go, symbols: [DefaultActivationBudget, LoadConfig, normalizeConfig], disposition: core, reason: bounded_activation_configuration}
    - {path: internal/runtime/runtime.go, symbols: [ExecuteActivationSync, CancelRun], disposition: core, reason: single_lifecycle_authority_budget_and_immediate_terminal_cancel}
    - {path: internal/runtime/api.go, symbols: [RegisterRoutes], routes: [/api/agent/loops, /api/agent/cancel], disposition: core, reason: connect_existing_owner_scoped_operator_handlers}
    - {path: internal/runtime/config_test.go, disposition: test, reason: activation_budget_configuration_regression}
  ratchet_artifact:
    path: docs/runtime-dissolution-inventory.yaml
    baseline_ref: 7994dfa62e3e9ba8420a5bb4810aae9be87a4ae1
    last_verified_ref: 9dff3690; agent://S1DeployVerifier; https://github.com/choir-hip/go-choir/actions/runs/29179656372
  current_artifact_state:
    runtime_dissolution: S0_inventory_and_ratchets_complete
    autoputer: not_complete
    choir_in_choir: closed
    autopaper: blocked_by_suite
  what_shipped:
    - 008a7b88cf200119c0f762cc51cfba6be3007445 grand-suite authority, registry cutover, subordinate demotions, consensus evidence, and live doccheck packet
    - 2327fcef4716aef070eb4b819296f01b44267364 S0 runtime inventory, executable ratchets, independent verification, consensus adjudication, and CI acceptance
    - 4973ee40570382c25398ea50e15148569cf351ab S1 owner-scoped list/cancel product surfaces, 60-minute activation budget, terminal-state late-write guards, passivation race repair, green CI/deploy, direct CLI acceptance, independent verification, and post-repair consensus
  what_was_proven:
    - One suite entrypoint across ACTIVE, mission graph, authority manifest, README, and doccheck live packet
    - Post-repair six-reviewer consensus cleared authority, durability, S1 exception, and S4 boundary findings
    - Live doccheck passed; full doccheck reported no errors; cmd/doccheck tests passed
    - S0 exact inventory covers 150 Go files, 461 Store calls, four conservative interface candidates, 151 citers, caller/debt/route/tool/importer/wrapper/compat surfaces, and fails closed on drift.
    - S1 restored runtime-bearing deployability and proved owner-scoped list/cancel, immediate admission release, durable terminal cancellation, deadline terminalization, late completion/passivation resistance, and direct deployed CLI cancellation.
  unproven_or_partial_claims:
    - Wire authority cutover
    - Runtime extinction
    - Audited computer through contained Choir-in-Choir operator proof
  belief_state_changes:
    - Runtime dead-code Phase 1 was not runtime dissolution; the package returned to its original production size.
    - A single executable grand suite replaces competing run-truth and autoputer sequencing spines.
  remaining_error_field:
    - Wire authority split and VM fate-sharing
    - internal/runtime god package and compatibility wrappers
    - audited-computer/operator/receipt/run-truth/self-development/containment gaps
  highest_impact_remaining_uncertainty: The exact remaining runtime-local Wire publication/read/migration callers and the smallest atomic cutover onto corpusd world-wire authority.
  next_executable_probe: Build the fresh S2 authority/caller/deletion inventory, verify existing corpusd publication/read capabilities and product lifecycle controls, then persist exact non-overlapping mutation slices before dispatch.
  suggested_goal_string: /goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
  evidence_artifact_refs:
    - docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md
  rollback_refs: []
  superseded_by: ''
  successor_goal_string: ''
```

## Forbidden Collapses

- suite document exists -> suite topology is implemented;
- member Definition complete -> suite complete;
- interrupted -> blocked;
- resumed -> rerun completed work;
- new package -> cutover complete;
- wrapper/facade -> extraction;
- deprecated alias -> deletion;
- lower runtime LOC -> no duplicated behavior;
- tests use API -> production uses API;
- consensus majority -> phase passed;
- worker says done -> orchestrator verified;
- local proof -> staging protected-surface proof;
- checkpoint -> completion;
- contained key exists -> key cannot escalate;
- external autoputer works -> Choir-in-Choir is safe;
- Autoputer complete -> Autopaper automatically authorized.
