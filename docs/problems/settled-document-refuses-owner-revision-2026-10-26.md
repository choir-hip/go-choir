# A settled document refuses the owner's next revision (F1)

Recorded 2026-10-26 01:16Z by FP0. Status: open, documented before any fix.
Mutation class of a fix: red (Texture lifecycle, canonical writes).

## What the owner sees

The owner asks for something in a Texture document, the work finishes, and
the owner asks for the next step in the same document. The revise is
refused with 409 "durable lifecycle state is unavailable or terminal". The
only way on is a new document. The owner's direction (2026-10-26 00:5x):
"when it makes sense, build on top of existing documents". Today a document
cannot be built on once its work is done.

## Evidence

1. PD3 S1 (2026-10-25 11:3x, b@c): document B settled after its third owner
   turn and refused the fourth (docs/evidence/pd3-s1-acceptance-2026-10-25.md,
   finding F1). The proof script switched to fresh documents.
2. T1a Reader round 2 (2026-10-26, b@c, computer-6c50d6e5): document
   98d0a421, trajectory 4e7fd334-8cc8-5a5d-8eca-8d00a719941f. The
   self-development candidate selfdev-3d55d4ff reached awaiting_approval at
   01:00:29; the trajectory settled at 01:00:40 (`trajectory.status
   settled`, `settled_at 01:00:40.538Z`, the only work item `completed`,
   both engineering co-super assignments terminal). FP0 rejected the
   candidate at 01:08:45 and sent the four fixes by `texture revise` on the
   same document: 409, the message above. Round 3 went into a fresh
   document (49807843), so the carried tree, the trace and the engineer's
   context of round 2 did not carry.

## Where it happens

- `internal/textureowner/texture_agent_revision.go:102-105`: the owner
  revise refuses unless `snapshot.Trajectory.Status == live`.
- `internal/store/store.go:1598` `ReconcileLifecycleSettlementForTerminalRun`
  settles the trajectory when a terminal run carries the settlement trigger
  and `lifecycleSettlementReady` holds; `internal/store/lifecycle.go` (about
  line 5263) writes `settled`, a terminal status.
- The 2026-10-09 fix (docs/problems/texture-settled-work-refuses-owner-revise-2026-10-09.md)
  reopens settled Texture work on a live trajectory and named this case as
  its residual: "Whether a document trajectory should ever settle while its
  owner keeps it is a Texture contract question (living documents, Gate 3)."

## Why it matters now

- It blocks the T1a iteration loop: every rejected candidate needs a new
  document, so a round loses the carried tree and starts cold.
- The conductor's continue-a-document routing (the routing question the
  owner raised 2026-10-26) needs the same thing: a prompt routed to an
  existing document must be able to start new work there.

## Open question (for the design, not settled here)

A trajectory is one episode of work; a document outlives its episodes. The
candidate shapes: (a) an owner revise on a settled document starts a new
trajectory bound to the same document, carrying its head and its links;
(b) a settled trajectory reopens to live on owner input; (c) document
trajectories never settle while the owner keeps the document. Settled is
terminal elsewhere in the store and in the doctrine's restart rules, which
weighs against (b).
