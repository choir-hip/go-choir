---
name: throughline
description: >-
  Use when work needs an executable /goal file for a long-running, uncertain,
  or product-changing mission: a concrete outcome, observed starting state,
  final artifact, proof, authority boundary, rollback, a value criterion and
  realism axis, a typed conjecture, and a compact current-state card. Compiles
  MissionGradient's value/homotopy discipline and Parallax's conjecture/shift
  discipline into one goal-file format that can be run with `/goal path.md`.
tags: [definition, goal-file, conjecture-learning, homotopy, long-running-agents]
related_skills: [cognitive-transform-portfolio, agentic-consensus]
---

# Throughline: Goal Files

Throughline makes `/goal <file>.md` a run command for a real outcome over an
uncertain route.

A goal file says, in order: what exists now, what will exist when the work is
done, how that result will be proved, what may be changed, what the mission is
actually optimizing, what it currently believes and how it would know, and what
one safe action comes next. It is executable authority, not a plan transcript,
a model vote, or a status report.

The point is reliability in service of the product. A goal should make it
easier to build and inspect Choir, not make the process more elaborate than the
artifact it promises.

## What `/goal` Means

When a compatible harness receives:

```text
/goal <file>.md
```

it must reconcile the observed starting state, build or investigate the
promised artifact within the stated boundaries, collect the named evidence, and
continue until the goal is complete, honestly blocked, or superseded. It must
not stop at a checkpoint, a passing local test, a candidate, a reviewer claim,
or a polished document.

The goal file is the one hand-maintained authority for its current state.
Evidence archives, CI/deploy receipts, review outputs, and HTML views are
referenced projections. They never silently overrule the goal file.

Only owner-stated authority, observed facts, settled decisions without a live
contradiction, formal checks within their stated scope, and explicit in-bound
operating preferences may authorize execution. A model, reviewer, or repeated
claim can supply evidence or a proposal; it is not authority by repetition.

## Start With Reality

Every goal opens with an **initial-state receipt**. Capture what is observed,
not what would be convenient:

- canonical branch/ref and relevant deployed identity;
- every dirty worktree or candidate in scope, with paths, owner, disposition,
  and recovery handle;
- the current product/repository artifact and known failures;
- existing settled decisions and unknowns that can change the next action.

Do not touch unclassified dirty work. A dirty `main` can be an intentional
candidate, user WIP, or a recovery surface; record which before working near
it. If a fact is unavailable, say `unknown` and make reconciliation the next
action. During that read-only reconciliation, mutation class may be `unknown`;
no implementation is authorized until it is classified. Never fabricate a
clean baseline or a candidate identity.

The start receipt is immutable except for a dated correction that preserves the
original observation and explains why it was wrong. It is not a running log.

## Define The Finish Before The Method

Write the goal around the thing a person, external agent, or product can use or
inspect when it succeeds:

1. **Deliver** — one plain-language user or product outcome.
2. **Finish** — the exact artifact or durable state that must exist.
3. **Acceptance** — the product path, command, or observation that proves a
   scoped claim about that artifact.
4. **Rollback** — the reversal/refusal path if the change fails after landing.
5. **Non-goals and constraints** — only boundaries that can change the
   delivery, safety, or authority.

For source or platform-behavior change, `finish` also names the required
landing path: pushed source identity, CI, deployment/staging identity, and
deployed product-path acceptance. A docs-only goal may explicitly mark that
path not applicable; it may not silently substitute a local check for it.

Internal restructuring is valid only when it directly supports this finish
line. Do not make "move packages," "write documentation," "get consensus," or
"reduce a count" the mission's final artifact unless that is itself the
user-visible product outcome.

Weak measures — LOC, a structural ratchet, panel agreement, latency, token use,
number of active agents — may steer where to inspect next. Each must state its
baseline, the decision it can inform, and what it cannot prove. A weak measure
never advances `complete`, settles an authority question, or turns a candidate
into an accepted artifact.

## Value And Homotopy

A goal says what is wanted. A **value criterion** says how to decide whether
the artifact is getting closer. Two one-liners, mandatory:

- `better_means` — the divergence being reduced. Not "build X" but "minimize
  <measurable gap> while preserving <invariants>." This is the loss function.
- `goodharting_would_be` — what the metric looks like if faked. Name the cheap
  proxy that would satisfy the letter and miss the goal. If you cannot state
  both, the mission is not ready.

**Homotopy, not ladder.** Define one real system parameterized along a
continuous `realism_axis` from low to high resolution. Simplify by reducing
resolution while preserving topology — same interface family, state
transitions, authority boundaries, event semantics, verifier meaning. A
simplification that cannot continuously deform into the full system is a
different object, not a useful rung. A low-resolution version is valid only as
a projection of the real system; a fake island is not progress.

The full multi-term functional lives here as prose rationale, not in the
schema. The schema carries only `realism_axis` and the two value one-liners.

## The Conjecture

The mission document claims that completing the artifact will actually advance
a deeper goal. State the bridge conjecture:

```text
If A satisfies S under I/Q over D, then G advances.
```

Treat that bridge as suspect until evidence supports it — many missions fail by
achieving the stated objective while missing the deeper goal.

Every load-bearing belief becomes a conjecture:

```text
CONJECTURE = (CLAIM, TEST, EDGE, ΔO, SCOPE)

CLAIM   what might be true
TEST    how the current observer would know
EDGE    how the claim could survive falsely without detection
ΔO      the smallest observer upgrade that would shrink the edge
SCOPE   the domain over which the claim may be asserted if the test passes
```

EDGE classes — each has a different fix:

```text
independence    the mission's assumptions don't bear on it -> adopt a new evidence source
resource        a proof exists but exceeds the budget      -> more budget, or a smaller claim
missing_oracle  no instrumentation/permission/tool sees it -> the usual observer upgrade
frame_lock      the refutation can't be stated in the
                current vocabulary                          -> extend the vocabulary; the
                                                               dangerous class (evidence gets
                                                               reinterpreted to fit the expressible)
```

Statuses: `proposed | active | testing | supported | weakened | falsified |
superseded | promoted_to_assertion`. An assertion is a supported conjecture
with receipts and an explicit scope; when a premise dies, it reverts — visibly.

The line that matters: **a prediction scored but never edged is decoration; a
conjecture whose edge shrank is what compounds.** A conjecture record is useful
only if it changes the next discriminator, probe, verifier, action route,
assertion scope, or stopping condition. Filling fields without changed behavior
is conjecture paperwork — the named failure mode of this format.

## The Compact Goal File

Use the authoring schema in
[`references/mission-schema.md`](references/mission-schema.md). Keep the file
short enough that a fresh agent can find its finish line and current action
without reading history. Its load-bearing blocks:

```text
start       immutable observed baseline and protected WIP
finish      promised artifact, proof, rollback, landing
value       better_means + goodharting_would_be
homotopy    realism_axis
boundaries  authority, mutation class, invariants, exclusions
now         the one mutable current-state card (status, slice, candidate,
            conjecture, decision, belief, blocker, next_action)
receipts    compact refs for closed boundaries
```

## The Circuit

One pass per control interval. Same circuit at every scale; only budgets
differ.

```text
1. CLAIM     What conjecture currently decides this mission?
2. POSITION  From where am I looking? "From here I can see X cheaply;
             I cannot see Y at all." Name the edge class.
3. MOVE      one of four — name the conjecture it will decide, or the
             observer evidence it buys:
               probe      test a conjecture under the current observer
               shift      move the observer (see catalog in Stuck)
               construct  build or extend the witness
               settle     decide the conjecture, or accept-and-name the edge
             Each move produces a strong, clear, definitive statement about
             the system. "The code works" is not a statement; "corpusd returns
             bare arrays for revision lists, breaking the Texture editor's
             revision selection" is.
4. BOUND     smallest substrate that can carry the move; stay inside the
             authority envelope; mutations reversible (candidate/capsule when
             risky). Batch when the route is unambiguous (see below).
5. UPDATE    Rewrite `now` in place; append one terse entry to the ledger
             file; record the conjecture verdict and actual ΔV against
             expected. A move that changed nothing is evidence about the
             OBSERVER, not the world.
6. EXIT?     conjecture decided | superseded | edges accepted and named |
             obligation only another authority can discharge | budget
             insolvent -> re-plan or hand off.
```

**Belief state** is three lines in `now`: `believed_state`, `main_uncertainty`,
`next_observation`. It is the compact resumable picture, not a second ledger.

**Batching.** When the route ahead is unambiguous — a planned sequence of
bounded constructs whose shape is already decided — one pass may plan and
execute the whole batch: name the k constructs, the predicted total ΔV, and the
per-construct check, then run them back-to-back with focused verification only.
The tripwire ends the batch early: any surprise, any deviation of actual
evidence from predicted ΔV, returns to a full circuit pass.

**Ledger.** Move history goes to a companion append-only ledger file
(`docs/<mission>.ledger.md`), written every pass, never re-read in full —
consult it only when auditing or when `now` has lost a thread.

## Stuck

Three forcing rules keep a stuck mission from grinding:

- **The forcing rule.** If the last two moves changed nothing — no ΔV, no new
  observer evidence — the next move is a SHIFT. Probing harder from a fixed
  position cannot escape that position's blind spot.
- **The learning rule.** Repeated obstacles are evidence about the conjecture,
  not just the route. When the same class of obstacle recurs, reconsider the
  bridge `A satisfies S => G`: the witness may be wrong, the spec a proxy, the
  domain may not embed, or the observer may lack the predicate that would
  reveal the real goal. Update, weaken, split, or supersede before grinding.
- **The architectural-mode rule.** A move that changes Choir from agentic to
  workflow, trajectory/work-item to run-tree, evidence contract to smoke proxy,
  or promotion protocol to shortcut behavior requires an explicit conjecture
  delta before construction. Do not let a probe precondition silently become
  architecture.

**Surprise classification.** Tactical (route-level, next probe inside
authority), target-level (the objective/spec shifts; update the goal and
continue), invariant-level (a protected invariant is threatened; stop or
escalate with exact evidence and the smallest safe next probe).

**Shift catalog** — when the forcing rule fires, change the observer, not just
the effort:

- *instrument* — add a measurement that makes the invisible visible;
- *vantage* — look from a different layer, caller, or boundary;
- *vocabulary* — extend what the mission can state (the frame_lock escape);
- *domain* — shrink or move the domain to where the claim is decidable;
- *prover* — change the verifier/evidence source;
- *inversion* — prove the negation, or assume the conclusion and work back.

## Review And Landing

Use deterministic checks first. Add independent review or agentic consensus
only when it can change a real decision: candidate scope, product behavior,
evidence floor, rollback, authority, or stopping condition. Do not send a
moving target to a panel and do not use a panel merely to narrate progress.

Bind review to a frozen candidate identity: base ref, scoped paths, digest, and
available evidence. The result is an evidence receipt with an adjudicated
outcome (`accept`, `repair`, `reject`, or `escalate`), not a vote and not its
own commit. A reproducible minority blocker outweighs an unsupported majority
pass.

**Consensus is a completion gate.** Before a behavior-changing mission may
settle `complete`, run the agentic-consensus panel on the frozen landed
candidate (diff identity + deployed evidence). The gate is not advisory:

- **Approve** → record the `consensus_review` receipt and settle.
- **Send back / does not approve** → completion is blocked. Close the named
  gap (tighten the proof, repair the defect, or accept-and-name the edge),
  re-freeze the new candidate identity, and re-run the panel. Iterate until
  the panel approves or a named edge is explicitly accepted — never settle
  `complete` on a rejected candidate, and never re-run the same frozen
  identity hoping for a different verdict.
- A convergent suggestion that is not a blocker is still adjudicated: adopt
  it, defer it with a named successor, or reject it with a reason. Do not
  silently drop panel findings.

The panel reviews evidence; it does not create it. A SEND BACK that names a
real gap is a finding about the proof or the artifact, not a failed vote.

Reliability needs independence, not agent count. Vary the relevant failure
surfaces: model family/version, context or memory lineage, tool/search source,
and reviewer obligation (builder, falsifier, verifier).

For source or platform behavior changes, complete the repository landing loop:
commit and push, CI, deploy/staging identity, then deployed acceptance. Record
those identities and results in the terminal receipt; do not call a local test
or a deployment SHA the final proof.

## Settlement

Use explicit mission statuses:

- `complete`: the stopping condition is satisfied with named evidence.
- `checkpoint_incomplete`: useful uphill progress landed, but the stopping
  condition is not satisfied. A resumable handoff state, not success.
- `blocked_incomplete`: a named blocker prevents progress after root-cause
  probes and shifts. Must include the smallest safe next probe or the external
  authority required to continue.
- `superseded`: target-level or invariant-level learning changed the mission
  identity enough that continuing would optimize the wrong artifact.

Do not phrase `checkpoint_incomplete` as "completed" or "done." Say plainly:
"Mission incomplete; checkpoint landed." The default action when the stopping
condition is unsatisfied is to continue, redirect, or delegate the next safe
executable probe — stop only when continuing would exceed authority, violate an
invariant, become unsafe, wait on an external with no useful parallel work, or
repeat already-falsified probes.

## Via Negativa

Do not:

- create fake stages or fake APIs;
- create fake work, placeholder implementations, parallel mock systems, or code
  detritus that future agents will imitate as precedent;
- optimize a checklist instead of the artifact;
- treat code existence as behavioral proof;
- hide uncertainty, or assert past the reach of the evidence;
- let a worker verify its own work;
- turn an actionable blocker into a final answer while authorized probes remain;
- grind a fixed position after two zero-moves (the forcing rule);
- grind repeated obstacles without reconsidering the bridge conjecture;
- let a weak measure advance `complete`;
- default to the smallest honest step when a bigger bounded move decides more;
- build a cathedral — a parallel elaborate structure beside the real object.

## Conformance Check

A goal file conforms when it:

- opens with an immutable `start` receipt classifying every dirty worktree;
- defines `finish` around a usable artifact with acceptance, rollback, and a
  landing path for behavior change;
- states `value.better_means`, `value.goodharting_would_be`, and
  `homotopy.realism_axis`;
- carries the bridge conjecture and its current verdict in `now.conjecture`;
- keeps `now` as the sole mutable card — status, slice, candidate, conjecture,
  decision, belief, blocker, next_action all describing one reality;
- forces a shift after two zero-moves;
- records closed boundaries in `receipts`, not as a running log;
- never lets a weak measure advance `complete`.
- never settles `complete` on a candidate the consensus panel sent back —
  the gate iterates (close the gap, re-freeze, re-run) until approval or an
  explicitly accepted edge.
