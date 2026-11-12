---
name: throughline
description: >-
  Use when work needs an executable /goal file for a long-running, uncertain,
  or product-changing mission: a concrete outcome, observed starting state,
  final artifact, proof, authority boundary, rollback, a value criterion, a
  typed conjecture, parallel stations, owner decisions with defaults, and a
  compact current-state card. Run with `/goal path.md`.
tags: [definition, goal-file, conjecture-learning, long-running-agents]
related_skills: [cognitive-transform-portfolio, agentic-consensus]
---

# Throughline: goal files

A goal file makes `/goal <file>.md` a run command for a real outcome over an
uncertain route. It says what exists now, what will exist when the work is
done, how that is proved, what may be changed, what the mission is optimising,
what it believes, and what comes next. It is executable authority, not a plan
transcript or a status report.

This is version 5 (2026-11-12). It is shorter than version 4 because the
agents that run it no longer need a per-step procedure. The fields are in
[references/mission-schema.md](references/mission-schema.md). The reasoning
behind the format, and the version 4 text, are in
[references/rationale-v4.md](references/rationale-v4.md); read it when a rule
here seems arbitrary. `scripts/check-goal-files` enforces the checkable rules.

## What `/goal` means

The agent reconciles the start state, builds the promised artifact inside the
boundaries, collects the named evidence, and continues until the goal is
complete, observing, honestly blocked or superseded. It does not stop at a
checkpoint, a passing local test, a candidate or a polished document.

The goal file is the one hand-maintained authority for the mission's state.
Receipts, review outputs and views are projections of it. Only an owner
statement, an observed fact, a settled decision or a formal check authorises
action. A model's or reviewer's claim is evidence, never authority.

## The parts that carry the weight

1. **Start receipt.** What was observed, not what would be convenient: the
   canonical ref, the deployed identity, every dirty worktree with its owner
   and whether it may be touched, the known failures, the unknowns. Write
   `unknown` when a fact is missing. Immutable after authoring, apart from a
   dated correction.
2. **Finish.** One plain outcome, the artifact that will exist, the acceptance
   list, the rollback and the landing path. Each acceptance item names the
   action, the scoped claim it proves and its evidence class. A source or
   behaviour change is not done at a local test: its finish is the pushed
   commit, CI, the deploy and the deployed proof.
3. **Value.** `better_means` is the gap being reduced, not "build X".
   `goodharting_would_be` is what success looks like if faked. A mission that
   cannot state both is not ready. Simplify a hard system by lowering its
   resolution along `realism_axis`, keeping its interfaces and authority
   boundaries; a mock that cannot grow into the real thing is not a step.
4. **Conjecture.** Building the artifact is a bet that the deeper goal
   advances. State the bet: the claim, the test, the edge (how the claim could
   survive falsely: `independence`, `resource`, `missing_oracle` or
   `frame_lock`), the smallest observer upgrade that shrinks the edge, and the
   scope it may assert if supported. Enter the same claim, with a probability
   and a settle-by time, in the calibration ledger and record its id here. A
   conjecture is worth writing only if its verdict changes the next move.
5. **Boundaries.** The mutation class, the authority sources in the owner's
   words, what must be preserved, what is excluded and the protected surfaces.
6. **Now.** One mutable card: status, slice, source ref, deploy identity,
   candidate, conjecture, belief (believed state, main uncertainty, next
   observation), blocker or risk, next action. Every field describes the same
   moment. Rewrite it in place. History goes to the companion ledger file,
   one terse entry when the state changes.

Weak measures (free space, line counts, panel agreement, latency) may steer
where to look. Each states what it can inform and what it cannot prove. None
advances `complete`.

## Stations

A mission of several changes lists them as `stations`: an id, a name, the
scope, the mutation class, the review it needs, what it depends on, the lane
or commit carrying it and a status (`pending`, `working`, `landed`, `proved`,
`superseded`). Stations with no dependency between them run in parallel, each
in its own lane with its own builder. Each red station gets its own review
and its own deployed proof; do not batch red stations into one landing to save
a deploy. `now.slice` names the stations in motion.

A mission too large for one file is a metamission: a goal file whose stations
each point at their own goal file. It never absorbs station detail.

## Owner decisions

A question only the owner can settle goes in `decisions_for_the_owner` with an
id, the question, why it matters, the options and a **default**. The default
is the conservative option and is what the mission does until the owner
speaks, so no decision blocks the work. The owner's answer goes in
`owner_rulings` with the date and the owner's words; when the words need
reading, record the reading beside them. A ruling is authority; a default is
not, and anything irreversible waits for the ruling.

## Working

- Before each move, know which claim decides it and what result would change
  the plan. A move that cannot change the plan is not worth making.
- When two moves in a row change nothing, stop pushing from the same position:
  add a measurement, look from another layer, shrink the domain, or try to
  prove the opposite
  ([cognitive-transform-portfolio](../cognitive-transform-portfolio/SKILL.md)).
- When the same kind of obstacle returns, doubt the conjecture before the
  route. The artifact may be the wrong one.
- A change that alters the architecture's kind (agentic to workflow, evidence
  to a smoke test, a protocol to a shortcut) needs a conjecture change in the
  file before any code.
- A surprise is tactical (continue), target-level (update the goal and
  continue) or invariant-level (stop, or escalate with the evidence and the
  smallest safe probe).

## Review and landing

The review tiers, the seats and the landing loop are in the repository
contract (`CLAUDE.md`, "Mutation classes", "Review", "Delivery"). This skill
does not restate them, because a second copy drifts. What it adds:

- Bind a review to a frozen candidate: base ref, paths, digest. The result is
  a receipt with a verdict, not a vote.
- A send-back blocks completion. Close the gap, freeze the new candidate and
  review again. Never re-run the same candidate hoping for another verdict.
- A finding that does not block is still adjudicated: adopted, deferred to a
  named successor, or rejected with a reason.
- The goal file itself is reviewed before it authorises red work
  (`readiness: reviewed`), unless the owner's own statement is the authority.

Readiness is how far the file is from execution: `intent`, `drafted`,
`reviewed`, `executable`. A file that is not executable is still worth
keeping as the record of an intention.

## Settlement

- `working`: construction, repair or a decision remains.
- `observing`: everything the mission builds has landed, deployed, passed its
  deployed proof and its review, and the only acceptance left is readings over
  time that no work can bring forward. It needs `now.observation`: the
  readings and where they are recorded; a reader that runs with no live
  session (a host unit, a scheduled tick, a dated routine); the window's end
  date; the pass line for each reading; and a tripwire that returns the
  mission to `working` at once. `/goal` may stop, saying "built; under
  observation until <date>", and the live station may move on. The window is
  extended once at most.
- `complete`: the acceptance list is satisfied with named evidence.
- `checkpoint_incomplete`: progress landed; the finish is not reached. Say
  "mission incomplete; checkpoint landed", never "done".
- `blocked_incomplete`: a named blocker remains after real probes. Name the
  smallest safe next probe or the authority needed.
- `superseded`: the mission's identity changed; continuing would optimise the
  wrong artifact.

When the finish is not reached, the default is to continue. Stop only when
continuing would exceed authority, break an invariant, be unsafe, wait on an
outside party with no useful parallel work, or repeat a falsified probe.

## Do not

- build fake stages, placeholder implementations or parallel mock systems;
- treat code existing as behaviour proved, or a local test as deployed proof;
- let a builder verify its own work;
- let a weak measure, a one-time release or a lowered bar count as the finish;
- turn a blocker into a final answer while an authorised probe remains;
- use `observing` to park unfinished work;
- keep a second copy of the mission's state anywhere;
- land a goal file without running `scripts/check-goal-files`.
