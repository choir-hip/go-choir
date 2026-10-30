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
- The 2026-10-09 fix (docs/problems/settled-document-refuses-owner-revision-2026-10-26.md)
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

## Design (2026-10-26, lane/f1-continue; shape (a))

**Shape.** An owner revise on a document whose trajectory is **settled**
starts a new live trajectory bound to the same document, then commits the
owner revision on it exactly as the live path does. The settled trajectory
is never written: settled stays terminal (against (b)), and documents keep
settling, so the restart rules and settlement are unchanged (against (c)).
A **cancelled** trajectory keeps refusing: cancel is an explicit stop
(owner or management), and continuing it would undo that act; the owner
can start a new document. No existing continue/reopen/successor mechanism
existed (searched `continue`, `reopen`, `successor`, `next_trajectory`,
`continue_doc_id`); the nearest, the 2026-10-09 reopen
(`reopenTextureWorkForOwner`), is for a live trajectory and is kept.

**Files.**
- `internal/store/lifecycle_continue.go`: `ContinueLifecycleDocument`
  (one conditional commit) and `LifecycleTrajectoryLineage`.
- `internal/types/lifecycle.go`: `ContinueLifecycleDocumentRequest`,
  `TrajectoryRefContinues` (`continues_trajectory_id`).
- `internal/textureowner/texture_agent_revision.go`:
  `continueSettledDocumentForOwner`, called from
  `handleLifecycleOwnerRevision` when the trajectory is settled, the prompt
  is non-empty and the revise is against the current head. Ids come from
  the settled trajectory (`textureContinueIdentity`), so racing revises send
  one identical command and all but one replay it; a loser of any other
  kind re-reads the document and joins whatever live trajectory it moved
  to. The response gains `trajectory_id`.
- `internal/agentcore/carried_patch.go`: the carried-patch history reads
  the lineage, so a rejected or unfinished round on the settled trajectory
  is offered to the next trajectory's engineer.
- `internal/store/lifecycle.go`: `ArchiveLifecycleArtifact` refuses a
  trajectory the document moved on from (a race could otherwise archive the
  live document through its settled predecessor).
- `frontend/src/lib/TextureEditor.svelte`: after a revise, the editor
  follows `trajectory_id` to the new trajectory's stream.

**The commit.** Conditions: the settled trajectory unchanged, the document
unchanged and still pointing at it with the expected head, the head
revision, the desks, the new trajectory/work/event/receipt absent. Writes:
the new trajectory (live, the predecessor's kind and settlement rule,
subject refs `doc_id`, `artifact`, the rule's required refs and
`continues_trajectory_id` only), one open Texture work item (the reopen's
constant objective), the document (pointer moved), `texture:<doc>` and
`research:<doc>` rebound (their object metadata `trajectory_id`) when they
belong to the settled trajectory, a `trajectory_started` event
(`[doc_id, head]`) and a `start` receipt. Command and event kinds are the
existing frozen ones: **no new lifecycle vocabulary, no schema change, no
new object kind.**

**What carries.** The document and its whole revision chain (doc-keyed);
the head, by reference (the owner revision's parent is the settled head and
its content is the head's); the Texture desk's identity, mailbox and
memory (keyed by agent id, not trajectory); the `texture:<doc>` inbox and
its records and rounds (doc-keyed); the research desk; through the lineage,
the engineering rounds of earlier trajectories (frozen candidates whose
operation was rejected or failed, and kept unfinished patches, by the
carry's existing rule). In the T1a case: the revise reaches Texture on the
new trajectory; Texture's Cast to management plans the engineering
assignment from `document.TrajectoryID` (`planDelegatedCast`), so the
assignment binds to the new trajectory, and its carry offer reads the
settled trajectory's rejected candidate selfdev-3d55d4ff with its rejection
reason.

**What does not carry.** Work items, worker updates, controls, runs,
lifecycle events and engineering assignments stay the settled trajectory's
own; the new trajectory's event cursor starts at 1 (a client reconnects on
the new trajectory); per-episode subject refs are dropped; nothing is
reopened. Late evidence for the settled trajectory arriving after the
document moved on is still admitted (review 260): `QueueLifecycleUpdate`'s
late branch and `texture_evidence_embed.go` accept a trajectory in the
bounded lineage (`LifecycleTrajectoryLineage`) of the document's current
trajectory, and the embed commits on the current one. The live branch of
`QueueLifecycleUpdate` and the other pointer checks still require
`document.TrajectoryID == req.TrajectoryID`.

**Failure modes (each has a test).** The 409 itself; the settled trajectory
rewritten; the pointer or the desk left behind; the head not carried; no
Texture target or wake; a retry or a race minting two trajectories; a
cancelled or stale-head revise continuing; the settled snapshot no longer
reading; the chain not extending; the archive race; the carry losing the
earlier round; a lineage that follows another document's trajectory, loops
or ignores its bound; a desk run still owning the activation (refused,
since it would rebind the desk back when it ends); a late update or an
evidence embed for the settled trajectory after the continue (admitted by
lineage). Tests: `internal/textureowner/texture_settled_continue_test.go`
(C1-C11), `internal/store/lifecycle_continue_test.go` (S1-S10),
`internal/agentcore/carried_patch_lineage_test.go` (L1-L3),
`internal/textureowner/texture_evidence_embed_test.go` (the embed after a continue).

**Residuals.**
- An open document stream in another tab stays on the settled trajectory
  until it reconnects (the revising editor follows the response).
- The proxy's Research more door recomputes an owner revision id from the
  document's trajectory before posting the revise
  (`internal/proxy/platform_expand.go`); it reads stories only, which never
  settle, so it is unaffected, but a future caller doing the same on a
  settled document would compute the id against the old trajectory.
- `GET /api/trajectories` now lists two trajectories for a continued
  document; nothing in `internal/` groups by document, client grouping was
  not checked.

**Rollback.** One `git revert` of the fix commit. Nothing persisted needs
migration: trajectories already continued stay valid, readable live or
settled trajectories with an extra subject ref; after the revert their
documents simply refuse the next revise on settlement again.

**Review tier.** Tier 2 (Texture canonical writes: the document pointer and
desk bindings move in a new reducer transition).

## Staging finding, 2026-10-26 04:48 UTC: the continue lands, Texture never starts (F1-S1)

Deployed in b4b4e551b (services active 04:43). On b@c, FP0 revised the settled
round 2 document 98d0a421 (trajectory 4e7fd334, settled 01:00:40) at 04:48:03
with a note-only direction. The revise answered 200 and continued the
document: new trajectory 909a5aee (subject_refs.continues_trajectory_id
4e7fd334), owner revision 93ec6437, artifact_head_advanced 04:48:04, and one
open work item 5b93d8cc assigned to texture:98d0a421. That half works: before
the fix the same revise answered 409.

No Texture run starts. From 04:48:04 the dispatcher defers both wake
occurrences for texture:98d0a421 (d633ca0e, afdd1c60) every second, 19 times by
04:54:42, with the cause "actorruntime: reconcile Texture coagent wake: start
reconciled Texture revision: replace durable activation: lifecycle invalid
transition". The trajectory's event list holds only trajectory_started and
artifact_head_advanced; no lifecycle_work_assigned delivery and no run. The
document is unchanged at 04:55.

Hypothesis, not yet read in code: the desk's durable activation still belongs
to the settled trajectory's last run, and replacing it with an activation on
the continued trajectory is a transition the lifecycle reducer refuses (the
continue moved the desk binding but not the activation record, or the
replace path checks the activation's trajectory). The unit tests built the
continue and the revise handler but never ran the actor's coagent-wake
reconcile against a continued document.

Effect: an owner revise of a settled document is accepted and silently never
acted on; the deferral retries every second without end. Severity: the F1
product promise is not kept; no data loss.

## Staging proof, 2026-10-26 05:49 UTC

Deployed 39a1be5c5 (train 3, with e51c99113; deployed 05:48:47; b@c reports
that build). The activation deferred since 04:48 (F1-S1) did not resume: at
05:49 trajectory 909a5aee was live, its work item open, its activation
passivated, its lifecycle two events long (started, head advanced), and the
head still the 04:48 owner revision 93ec6437. A fresh owner revise of the
settled document 98d0a421 (05:49:47, owner revision 00966d98) continued on
909a5aee and Texture committed revision 79a4e04a at 05:49:54, seven seconds
later: the requested sentence appended, nothing else changed, metadata
`trajectory_id` = 909a5aee. F1 is proved on the product path. Residual: the
04:48 occurrence was closed without a run and without a visible fate on the
document (observed only; whether the 05:48 boot carried a planned-restart
marker was not checked). The owner revision's own label (review 277 LOW) is
fixed on lane/f1-label (review 280), not yet deployed.


## Merged from docs/problems/texture-settled-work-refuses-owner-revise-2026-10-09.md (2026-10-09)

**After Texture settles its work, the owner cannot revise the document (2026-10-09).**
Found by the Texture acceptance suite, third run on staging (build 7bc8f374; receipt `evidence/texture-acceptance-2026-10-09T18-59-43-320Z.json`), fresh disposable account `1d20b772…` (`computer-7799ad0b…`, VM `vm-549ec1d9…`), document `1ae32977…`. Mutation class of the eventual fix: red (lifecycle work items, owner revision intake). Problem first; no fix in this commit.

(Full text in git history at the path above.)
