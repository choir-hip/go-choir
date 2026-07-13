---
name: definition
description: >-
  Use when work needs compact executable semantic authority: defining or
  revising a mission, object, invariant set, authority boundary, evidence
  class, assurance policy, completion semantics, rollback policy, or forbidden
  collapse before or during long-running agentic execution. Produces or updates
  a mission-definition document directly executable with `/goal path.md`,
  with one canonical state capsule, referenced evidence, adaptive review, safe
  concurrency, and generated non-authoritative views.
---

# Definition

Definition is the high-level semantic authority layer for agentic work.

It exists because long-running agents fail when the governing words are weaker
than the execution pressure. They turn checkpoints into completion, artifacts
into proof, plans into authority, tests into universal truth, toy evidence into
program validation, and motion into progress.

Definition makes the mission executable by defining what the mission's words are
allowed to mean, how those meanings are observed, who has authority, which
claims evidence can support, what must happen next, and when execution must stop.

The output is a **mission-definition document**. When a compatible harness is
called with:

```text
/goal <document>.md
```

it must execute the mission defined in that document. It must not summarize the
document, admire it, checkpoint early, or create a separate control language.

## Core Thesis

A definition is not a gloss. A definition is operational authority.

- A plan lists intended actions.
- A conjecture names a claim to test.
- A gradient names an optimization landscape.
- A definition says what the mission, objects, claims, evidence, authority,
  states, and completion conditions are allowed to mean.

The mission-definition document can contain plans, conjectures, gradients,
variants, ledgers, and reports, but those are subordinate projections. The
controlling object is the **definition kernel plus one canonical state
capsule**. Evidence archives and reports are referenced projections, not second
state authorities.

## Scope

Definition supersedes legacy mission-control formats. Express useful gradients,
conjectures, observer shifts, realism parameters, evidence, rollback, and
resumption as nodes or policies under this single authority. Do not preserve a
parallel legacy control language. Do not use Definition for ordinary coding
tasks whose vocabulary, authority, and completion semantics are already stable.

## Non-Definitions

Definition is not a document generator, normal plan, model vote, report that
replaces execution, license to cross a broken invariant, or excuse to delay a
valuable in-bound probe. Keep a node only when it changes action, verification,
route, scope, authority, claim, rollback, or stopping semantics.

## Mission-Definition Document

A mission-definition document is the source program for a `/goal` run.

If the user supplies an existing document, compile it in place. Preserve the
author's source text where possible, but add or update the Definition sections
needed to make execution unambiguous. Do not create a parallel control document
unless the source document explicitly requires a split.

### Authority Layout

Keep one semantic authority while separating storage roles:

1. **Definition kernel** — comparatively stable purpose, graph, invariants,
   authority boundaries, evidence rules, assurance policy, rollback, and
   completion semantics.
2. **State capsule** — the sole canonical current-state projection: active
   node/slice, settled predecessors, blockers, locks, artifact and deployment
   identity, evidence pointers, remaining uncertainty, and next probe.
3. **Evidence archive** — append-only external artifacts containing full
   transition histories, commands, traces, reviewer outputs, and receipts.
4. **Generated views** — HTML, reports, dashboards, and summaries derived from
   the kernel, state capsule, and evidence index.

The kernel and state capsule live in the mission-definition document. Evidence
may live elsewhere when referenced by immutable path, URI, commit, digest, or
artifact identity. Generated views must declare source document, source commit
or digest, generation time, and freshness. They are never editable authority.

Every current-state fact has exactly one canonical field. Do not maintain a
detailed ledger and a separately hand-written current summary. Generate or
delete duplicate projections. A contradiction between projections is a
blocking conformance failure.

The mission document is a control plane, not an event warehouse. Keep the
active frontier expanded. Collapse settled slices to compact receipts containing
status, artifact/commit identity, evidence refs, rollback refs, and invalidation
triggers; move their full histories to the evidence archive.

When authoring, migrating, or changing the kernel/schema, read
[`references/mission-schema.md`](references/mission-schema.md). Routine
execution and resumption need not load it. Use only load-bearing sections;
open missing load-bearing meanings as nodes rather than silently inferring them.

## `/goal <document>.md` Semantics

When a compatible harness receives:

```text
/goal <document>.md
```

it must interpret this as:

```text
Read the mission-definition document as semantic authority. Execute it
autonomously until its completion semantics are satisfied with named evidence,
or until a sharply evidenced escalation/blocker/supersession condition is met.
```

The harness must:

1. read the document kernel, canonical state capsule, and declared authority
   sources using the delta-first resumption policy below;
2. reconcile current artifact state with the state capsule;
3. open definition nodes for missing or contested load-bearing meanings;
4. resolve leaf definitions through the critical process;
5. choose the next executable probe or construct inside the authority boundary;
6. state the active definition/conjecture being tested before mutation;
7. execute, verify, and scope the resulting claim to its evidence class;
8. update working state when the move creates a semantic event; commit only at
   a Git durability boundary in the Define/Implement rhythm below, coalescing
   related capsule and evidence changes;
9. continue until completion, blocked escalation, or supersession.

The harness must not stop because a phase boundary, checkpoint, review packet,
passing focused test, or worker claim exists. Those are evidence candidates, not
completion.

## Definition Graph

A definition graph contains stable typed nodes with source, status, meaning,
non-definition, observables, execution effects, settlement authority, and
invalidation triggers. Use the authoring schema when creating or changing node
structure.

## Determined State

Determined state is the current semantic authority snapshot.

A claim belongs to determined state only if it is:

1. **user-stated authority**;
2. **observed fact** from tools, files, commands, traces, artifacts, or systems;
3. **settled definition** with no live contradiction;
4. **operational preference** explicitly stated by the owner.

A claim does not belong if it is merely plausible, stylish, repeated, or asserted
by a model.

Keep settled, contested, and open claims explicit using the authoring schema.

## Canonical State Capsule

Maintain exactly one compact operational snapshot using the authoring schema.
It is the only hand-maintained current-state projection. Generate summaries,
checkpoint prose, dashboards, and status views from it. Fail deterministically
on contradictory statuses, stale artifact/deployment identities, dangling
evidence, multiple active authorities, or stale generated views.

## Critical Process

Resolve nodes by opening ambiguity, differentiating meanings, criticizing reward
hacks, operationalizing observables, probing/constructing, settling, and
monitoring. When the route is unclear, proxy risk exists, evidence scope is
contested, or formalization may be needed, read
[`references/semantic-methods.md`](references/semantic-methods.md).

## Mission Execution Loop

Definition uses receding-horizon execution, but the loop operates over the
definition graph rather than over a separate mission format.

Each control interval:

1. **Select** the live node or conjecture whose settlement most reduces mission
   uncertainty or unlocks execution.
2. **State** what the current observer can and cannot see; name any blind spot.
3. **Choose** one move:
   - `define`: make a missing meaning executable;
   - `probe`: test a claim under current observer;
   - `shift`: change observer, vocabulary, domain, instrument, or prover;
   - `construct`: mutate the artifact under invariants;
   - `verify`: check an artifact or claim;
   - `settle`: promote/weaken/falsify/supersede/escalate.
4. **Bound** the mutation radius and rollback surface.
5. **Execute** the move.
6. **Update** working state if the move produced a semantic event; cross a Git
   durability boundary only when the boundary protocol requires it.
7. **Continue** unless completion, supersession, or hard escalation is reached.

If the route is clear and low-risk, batch foreseeable constructs in one interval.
The tripwire is surprise: any unexpected evidence returns execution to a full
select/state/choose/bound loop.

### Semantic Events And Git Durability Boundaries

Do not equate semantic events with Git commits:

- A **semantic event** changes a node, claim, belief, next probe, assurance
  profile, or artifact state.
- A **working-state write** updates the capsule or evidence index on disk.
- An **evidence receipt** is a durable external or content-addressed fact such
  as a job, CI, deploy, trace, or reviewer result.
- A **durability boundary** is a Git commit that makes a coherent set of events
  and receipt references resumable.

Apply semantic events and working-state writes immediately, but accumulate them
until a required durability boundary. A changed belief, next probe, worker
return, verifier dispatch, CI status, deploy receipt, consensus result, report
refresh, or lock heartbeat is not by itself a reason to commit.

For ordinary surprise-free work, use a natural two-beat rhythm. Consensus is an
assurance operation on a proposed boundary, not a third beat:

1. **Define.** Prepare one code-free boundary closing the prior slice and
   defining the next problem/evidence, authority, rollback, mutation boundary,
   and dispatch. When assurance requires consensus, freeze, review, adjudicate,
   revise, then commit the accepted Define before Implement. This satisfies
   problem-documentation-first.
2. **Implement.** Prepare code, tests, generated artifacts, capsule changes,
   local evidence, and worker identity. Run checks and independent verification;
   when assurance requires consensus, freeze, review, adjudicate, repair, then
   commit. A confirmed new platform-behavior problem instead requires the next
   commit to be a code-free Define boundary authorizing repair.

Bind review to a frozen candidate identity containing base revision, path scope,
content digest, and evidence refs. Use a content-addressed patch/bundle, read-only
snapshot, or isolated candidate commit; the latter is review substrate, not
canonical mission state. Freeze scoped mutation. Material semantic change
invalidates review; skip deterministic formatting/generated refresh reruns only
with a content-neutral rationale.
The accepted boundary must bind candidate identity, consensus/evidence refs,
adjudication, and no-rerun rationale in included state or commit metadata. After
commit, gather external receipts and normally fold them into the next Define.
Post-boundary observation does not itself require consensus; use another panel
only when unavailable evidence or surprise can change the graph, evidence class,
authority, mutation boundary, escalation, route, or stopping condition. On
stop/handoff without durable receipts, emit a final Define beat with terminal
state, evidence, and rollback anchor.

Do not create separate commits merely for dispatch intent, dispatch receipt,
worker return, verifier dispatch, CI status, deploy receipt, consensus result,
report refresh, or lock heartbeat. If an attempt produces no implementation,
keep unresolved deliberation in working state, then combine its result, blocker,
adjudication, and redefinition into one Define beat once the disposition or
escalation is known.

The expected long-run rhythm is approximately one state/Definition commit per
implementation commit, with only initial and terminal bookends. Treat sustained
departure from that rhythm as a sign that the mission is over-modeling its own
execution; simplify the Definition before adding enforcement machinery.

Archive detailed histories outside the capsule and reference them at the next
beat. Compact settled entries without losing evidence, rollback, or invalidation
refs.

### Safe Concurrency

Declare mutation lock domains, observation domains, integration authority, and
external-effect domains. Serialize mutations sharing state, an authority edge,
canonical journal parent, deployment routing, rollback surface, or protected
external effect.

After a frozen candidate identity exists, fan out independent source checks,
bounded second opinions, and read-only preparation. After its canonical
boundary, fan out CI and external verification. Separate worktrees may prepare
genuinely disjoint patches, but one integration authority lands them. Any
candidate drift, surprise, shared dependency, dissent, or ratchet drift
dissolves the batch and returns to serial control.

## Conjectures, Progress, Realism, And Evidence

Treat conjectures as graph nodes, define a variant for long missions, preserve
topology across simplified domains, and scope every nontrivial claim to its
evidence class. Read the semantic-methods reference when using these mechanisms;
routine execution on already settled definitions need not reload it.

## Evidence Ledger

For each promoted or settled claim, record its node, evidence class, exact
source/observation, artifact, result, uncertainty, and promotion relevance using
the authoring schema. The ledger records proof reach; it is not a success
substitute or a reason for its own Git commit.

## Authority And Human Escalation

Escalate to the human only for group-level decisions:

- purpose or identity changes;
- authority-boundary changes;
- unsafe/destructive or high-blast-radius mutations;
- paid/long-running compute beyond already granted policy;
- conflicting values or taste calls;
- irreversible actions without accepted rollback;
- definitions whose settlement would authorize risky mutation.

Do not escalate every leaf definition. Resolve leaf definitions through the
critical process when they stay inside established authority.

Name the node, unresolved issue, options and execution consequences, and a
recommendation using the authoring schema.

## Assurance Profiles And Second Opinions

Attach an assurance profile to each active mutation or settlement class. Map
panel tiers onto project-specific mutation classes and ceremony; consensus
supplements those rules and never replaces them or creates a competing risk
taxonomy. Record risk, novelty, evidence floor, independent-verifier
requirement, panel tier/diversity, hard timeout/budget, and escalation triggers
using the authoring schema.

Default toward executable ratchets plus an independent verifier for routine,
low-risk work; use a compact diverse panel at a compatible batch checkpoint.
Use broader panels for behavior change and full adversarial panels for novel,
concurrent, lifecycle, migration, authority-transfer, irreversible, or protected
work. Escalation triggers override the lower tier immediately.

Use second opinions only if they can change the graph:

- chosen definition;
- split/merge/narrow/widen decision;
- execution effect;
- verifier or evidence class;
- formalization requirement;
- escalation boundary;
- stopping condition;
- downstream route.

Second opinions are not votes. The orchestrator adjudicates and updates the
graph. One substantiated minority blocker outweighs an unsupported majority
PASS.

Before requesting one, record the node, unresolved question, expected decision
impact, why internal deliberation is insufficient, tool/model family, compute
tier, hard timeout/budget, and bounded output shape using the authoring schema.

Record completion status, latency, exact model/version when available,
input/output/cache tokens, estimated cost, output size, timeout, and adjudicated
finding disposition: unique confirmed, duplicate, false positive, or unresolved.
Measure diversity by model family and finding yield, not CLI count.

Maintain dated reviewer health receipts. Quarantine repeated stalls or empty
outputs, probe them later under a bounded timeout, and expire unexplained
exclusions. After several comparable panels produce unanimous no-new-finding
results, reduce that class's panel. A confirmed unique finding, dissent, or
evidence surprise expands the next comparable panel.

## Mission Efficiency And Learning

At phase gates, learn from telemetry the tools already produce: critical-path
time, model/tool compute, CI time, rework, unique and escaped defects, rollbacks,
Definition size, and the observed Define/Implement rhythm. Do not create
per-slice tracking work, documents, or commits solely to measure efficiency.
Use the retrospective to adjust assurance, batch size, context, and concurrency;
never lower an evidence floor merely to improve a metric.

## Mission Report Policy

Broad mission-definition runs should maintain an owner-readable report when the
run changes durable system state, doctrine, deployed behavior, or long-running
training/execution state.

The report should explain:

```text
mission goal and artifact
invariants preserved or violated
major decisions and route changes
what shipped
verification evidence
what was proven vs merely attempted
residual risks
rollback refs
next mission or next executable probe
```

Do not dump logs. Link evidence artifacts. Generate the report from the state
capsule and evidence index where practical; display source identity and
generation time. An HTML dashboard is a useful view, never a write path.

PDF export is optional unless the mission document or owner requests it.

## Checkpoint And Resumption State

The canonical state capsule is the resumable state. Do not duplicate it in a
second hand-maintained checkpoint block. A checkpoint report may be generated
from the capsule and evidence index.

Use delta-first resumption:

1. On first compilation, read the full kernel, state capsule, and required
   authority sources.
2. Record their immutable identities and a digest of compiled semantics.
3. On resumption, verify those identities, reconcile the artifact and external
   effects, and load the active frontier plus referenced evidence.
4. If the kernel or authority sources changed, inspect the semantic diff and
   reopen affected nodes; perform a full reread when the diff cannot be safely
   localized.
5. Do not reread archived closed-slice histories unless an invalidation trigger,
   contradiction, audit, or rollback makes them relevant.

A checkpoint is not completion. If a safe executable probe with positive
expected information or artifact value remains inside the authority and
assurance budget, execute it instead of presenting the checkpoint as success.

## Completion Semantics

Completion means the document's own completion semantics are satisfied with
named observables and evidence.

Use statuses:

```text
working
complete
checkpoint_incomplete
blocked_incomplete
superseded
```

- `complete`: stopping condition satisfied with scoped evidence.
- `checkpoint_incomplete`: useful progress landed, but stopping condition is
  not satisfied. This is not success.
- `blocked_incomplete`: progress is blocked after root-cause probes and
  cognitive transforms, with exact blocker and required authority/prerequisite.
- `superseded`: learning changed the mission identity enough that continuing
  would optimize the wrong object.

Before any non-complete exit, verify that no safe, materially valuable probe
remains inside the authority and assurance budget. Repeating a probe class
without new information is motion theater. After three comparable failed or
non-converging probes, require a changed observer, substrate-level route,
formalization seam, supersession, or human escalation before another attempt.

## Forbidden Collapses

Never collapse artifact existence into validity, plans into execution, focused
tests into universal proof, checkpoints into completion, model agreement into
authority, formal specs into implementation conformance, local smoke into
deployed proof, or worker claims into completion. Read the semantic-methods
reference when a new collapse or proxy risk appears.

## Definition Operators

Use the operators in the authoring schema. Each must produce an observable
result. Accumulate their state changes into the next natural Define or Implement
beat. Make no silent semantic changes.

## Conformance Checklist

A run conforms to Definition if:

- [ ] It names the active mission-definition document.
- [ ] It treats `/goal <document>.md` as executable authority, not passive context.
- [ ] It identifies the real artifact/object of work.
- [ ] It separates purpose from non-purpose.
- [ ] It names authority sources and boundaries.
- [ ] It maintains a definition kernel and exactly one canonical state capsule.
- [ ] It has no contradictory hand-maintained current-state projections.
- [ ] It keeps only the active frontier expanded and archives settled histories.
- [ ] It attaches observables and execution effects to settled nodes.
- [ ] It scopes claims to evidence classes.
- [ ] It preserves topology when simplifying.
- [ ] It uses conjectures as definition nodes when truth affects execution.
- [ ] It uses variants/progress measures for long runs.
- [ ] It executes safe, materially valuable in-bound probes instead of stopping at checkpoints.
- [ ] It records evidence and rollback/resumption state.
- [ ] It distinguishes semantic events, working-state writes, evidence receipts, and Git durability boundaries.
- [ ] It follows a natural Define/Implement rhythm and folds prior closure into the next Define beat.
- [ ] It does not turn routine orchestration receipts into standalone commits.
- [ ] It assigns assurance and second-opinion cost proportional to risk and novelty.
- [ ] It records reviewer health, resource use, and adjudicated finding yield.
- [ ] It measures mission cost and latency without weakening the evidence floor.
- [ ] It pipelines read-only work while serializing shared-authority mutation.
- [ ] Its generated reports and dashboards declare provenance and are non-authoritative.
- [ ] It escalates only group-level or sharply evidenced hard blockers.
- [ ] It does not claim completion until the document's completion semantics are satisfied.

## Suggested Invocation

```text
Use Definition. Treat <document>.md as executable semantic authority, not a plan, transcript, or report. Compile its stable definition kernel and exactly one canonical state capsule; keep detailed evidence in referenced archives and generate non-authoritative views from the capsule. On resumption verify source identities and load the semantic delta plus active frontier. Define missing terms, boundaries, invariants, evidence classes, assurance policy, and completion semantics, then execute materially valuable probes through the graph. Use a natural Define/Implement rhythm: fold prior closure and next authority into one state boundary, then land implementation, tests, capsule changes, and local evidence together; do not commit routine orchestration receipts separately. Compact settled histories, risk-tier second opinions, meter reviewer cost and finding yield, parallelize read-only work after immutable identities, and serialize shared-authority mutation. Escalate on group-level authority changes, unsafe actions, protected surfaces, or evidenced non-convergence. Stop only when completion is proven or the mission is honestly blocked or superseded.
```
