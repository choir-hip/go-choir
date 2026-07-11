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
  current_subgoal: S0
  last_completed_subgoal: B0
  definition_gate:
    status: passed
    consensus_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
    adjudication_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
  suite_run_id: choir-autoputer-completion-2026-07-11-01
  canonical_journal_ref: refs/heads/main@origin
  journal_expected_parent_sha: 1392a724d4381f3f4d9ca41478e8395acf87154b
  orchestrator_lock:
    holder: Main
    epoch: 3
    expires_at: 2026-07-11T23:11:54Z
    expected_parent_sha: 5cfcdad32c395ec587aae8599306e458204736cb
    lock_transition_id: s0-lock-acquisition-03
  suite_authority_sha: 008a7b88cf200119c0f762cc51cfba6be3007445
  subgoal_status:
    B0: {status: complete, started_at_sha: 27db14c36c482e321b56a056f6ce5e0accb338a4, completed_at_sha: 008a7b88cf200119c0f762cc51cfba6be3007445, evidence_refs: [008a7b88cf200119c0f762cc51cfba6be3007445, docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md], rollback_refs: [27db14c36c482e321b56a056f6ce5e0accb338a4], blockers: []}
    S0: {status: working, started_at_sha: 008a7b88cf200119c0f762cc51cfba6be3007445, completed_at_sha: '', evidence_refs: [], rollback_refs: [008a7b88cf200119c0f762cc51cfba6be3007445], blockers: []}
    S1: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S0]}
    S2: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S1]}
    S3: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S2]}
    S4: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S3]}
    S5: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S4]}
    S6: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S5]}
    S7: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S6]}
    S8: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S7]}
    S9: {status: waiting_on_predecessor, started_at_sha: '', completed_at_sha: '', evidence_refs: [], rollback_refs: [], blockers: [S8]}
  active_phase_checkpoint:
    subgoal: B0
    status: passed
    deployed_sha: not_applicable_green_authority_landing
    ci_ref: docs_only_workflow_for_008a7b88cf200119c0f762cc51cfba6be3007445
    staging_ref: not_applicable_green_authority_landing
    product_proof_refs:
      - 008a7b88cf200119c0f762cc51cfba6be3007445 pushed to origin/main
      - scripts/doccheck -mode live (passed)
      - scripts/doccheck -mode full (report-only, no errors)
      - go test ./cmd/doccheck (passed)
    consensus_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
    open_findings: []
    adjudication_ref: docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md#definition-gate-result
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
      orchestrator_lock_epoch: 3
      status: verifying
      dispatch_nonce: s0-runtime-inventory-ratchet-01-nonce-01
      dispatch_ref: S0RatchetImplementer
      agent_session_ref: agent://S0RatchetImplementer
      dispatch_prompt_ref: docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md
      implementer_job_ref: S0RatchetImplementer
      implementer_output_ref: agent://S0RatchetImplementer
      verifier_job_ref: S0RatchetVerifier
      verifier_output_ref: agent://S0RatchetVerifier
      worktree_or_branch_ref: s0-runtime-inventory-ratchet-01@6a9c34954a91bb34a427ca5b55a8239515466a97
      declared_reconciliation_substrates: [canonical_git_ref, agent_job_record, agent_output_artifact, isolated_worktree_or_patch]
      mutation_delivery_mode: isolated_worktree_or_patch
      direct_shared_worktree_allowed: false
      direct_shared_worktree_justification: not_applicable
      lock_acquired_ref: 1a9a90b63f6541fcb8d96502e85a158b8446d14e
      lock_release_ref: pending_slice_close
      stage_started_at: 2026-07-11T21:11:54Z
      transition_id: s0-runtime-inventory-ratchet-repair-returned-06
      expected_parent_sha: 1392a724d4381f3f4d9ca41478e8395acf87154b
      stage_history:
        - {status: dispatch_intent, transition_id: s0-runtime-inventory-ratchet-dispatch-intent-01, recorded_at: 2026-07-11T21:11:54Z, actor: Main, expected_parent_sha: 1a9a90b63f6541fcb8d96502e85a158b8446d14e, precondition: S0_working_and_lock_epoch_3_held, postcondition: dispatch_prompt_and_exact_mutation_lock_are_canonical, external_operation_id: not_applicable}
        - {status: dispatched, transition_id: s0-runtime-inventory-ratchet-dispatched-02, recorded_at: 2026-07-11T21:14:41Z, actor: Main, expected_parent_sha: f72a141ef0f97fbec6521831dc3f5836b9526631, precondition: canonical_dispatch_intent_and_live_lock_epoch_3, postcondition: implementation_agent_started_with_recorded_nonce, external_operation_id: not_applicable}
        - {status: implementation_returned, transition_id: s0-runtime-inventory-ratchet-returned-03, recorded_at: 2026-07-11T21:23:47Z, actor: Main, expected_parent_sha: eca2f134cca65c85a02971af8f7e1140b7fc7f44, precondition: exactly_one_matching_agent_result_for_dispatch_nonce, postcondition: isolated_commit_recorded_for_integration, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-verifier-intent-04, recorded_at: 2026-07-11T21:28:23Z, actor: Main, expected_parent_sha: d2cde593b2b6e7b1ab407e74e713eee5534b8c42, precondition: corrected_implementation_integrated_and_orchestrator_smoke_passed, postcondition: independent_verifier_assignment_is_canonical, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-verifier-failed-05, recorded_at: 2026-07-11T21:31:09Z, actor: S0RatchetVerifier, expected_parent_sha: 5629347ba0a5c344341c4f2220f6ebb4ab10450a, precondition: independent_read_only_verification_of_canonical_slice, postcondition: two_blocking_findings_recorded_for_repair, external_operation_id: not_applicable}
        - {status: verifying, transition_id: s0-runtime-inventory-ratchet-repair-returned-06, recorded_at: 2026-07-11T21:38:51Z, actor: Main, expected_parent_sha: 1392a724d4381f3f4d9ca41478e8395acf87154b, precondition: both_blocking_findings_have_targeted_regressions_and_local_focused_pass, postcondition: repaired_commit_integrated_and_ready_for_independent_reverification, external_operation_id: not_applicable}
      lock_expires_at: 2026-07-11T23:11:54Z
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
      observed_postcondition: repair_commit_6a9c34954a91bb34a427ca5b55a8239515466a97_integrated_as_1392a724_with_local_focused_pass
      external_operation_idempotent: true
      implementation_sha_or_dirty_snapshot: 6a9c34954a91bb34a427ca5b55a8239515466a97
      implementation_commit_sha: 6a9c34954a91bb34a427ca5b55a8239515466a97
      push_ref: pending_push_for_independent_reverification
      ci_run_ref: not_applicable_until_integration
      deploy_ref: not_applicable_yellow_slice
      deployed_sha: not_applicable_yellow_slice
      acceptance_ref: artifact://104
      acceptance_contract: go_test_cmd_runtime_ratchet_and_baseline_invocation_pass_with_regression_fixtures_failing
      evidence_refs: [docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md]
      open_findings: [S0-RAT-001_missing_real_production_caller_validation, S0-RAT-002_citer_identity_truncates_suffix_after_240_bytes]
      landed_commit_sha: pending
      adjudication: pending
      last_reconciled_at: 2026-07-11T21:38:51Z
      reconciliation_result: both_verifier_findings_repaired_locally_pending_independent_reverification
      close_condition: independently_verified_inventory_and_ratchet_landed_then_S0_consensus_adjudicated
  s1_runtime_exception_disposition: []
  ratchet_artifact:
    path: ''
    invocation: ''
    baseline_ref: ''
    last_verified_ref: ''
  current_artifact_state:
    suite_definition: authority_persisted_at_008a7b88cf200119c0f762cc51cfba6be3007445
    runtime_dissolution: not_started
    autoputer: not_complete
    choir_in_choir: closed
    autopaper: blocked_by_suite
  what_shipped:
    - 008a7b88cf200119c0f762cc51cfba6be3007445 grand-suite authority, registry cutover, subordinate demotions, consensus evidence, and live doccheck packet
  what_was_proven:
    - One suite entrypoint across ACTIVE, mission graph, authority manifest, README, and doccheck live packet
    - Post-repair six-reviewer consensus cleared authority, durability, S1 exception, and S4 boundary findings
    - Live doccheck passed; full doccheck reported no errors; cmd/doccheck tests passed
  unproven_or_partial_claims:
    - Fresh S0 runtime inventory and ratchet baseline
    - Deploy restoration
    - Wire authority cutover
    - Runtime extinction
    - Audited computer through contained Choir-in-Choir operator proof
  belief_state_changes:
    - Runtime dead-code Phase 1 was not runtime dissolution; the package returned to its original production size.
    - A single executable grand suite replaces competing run-truth and autoputer sequencing spines.
  remaining_error_field:
    - staging deploy blocked by one active run
    - Wire authority split and VM fate-sharing
    - internal/runtime god package and compatibility wrappers
    - audited-computer/operator/receipt/run-truth/self-development/containment gaps
  highest_impact_remaining_uncertainty: Fresh S0 disposition and caller graph
  next_executable_probe: Reconcile repository and staging, then establish the S0 runtime inventory, caller graph, citer dispositions, and executable ratchets.
  suggested_goal_string: /goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
  evidence_artifact_refs:
    - docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md
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
