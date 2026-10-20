# Related stories are thin: no subject nodes in the graph, and no desk can ask System 1 a question (2026-10-20)

Status: documented; design in
[related stories, phase 3](../reports/related-stories-phase3-research-review-2026-10-20.md);
the verb (design §3.1, order step 3) follows in later commits. Continues
[a published story is a dead end](related-stories-dead-end-2026-10-20.md).
Mutation class of the first fix: red (gateway calls, research desk surface).

Owner, 2026-10-20 (notes §28, end of the section, reports/owner-notes-2026-10-14.md):
"there should be more connections between articles, more links from one to
the other with the related stories feature. We should think about how to
review all the existing articles and make sure that they are correctly
connected in the object graph when they touch on similar topics, events, and
entities. Let's think about how we can effectuate this with a desk like the
research desk." Then: "We are not yet using the embeddings, although we're
looking to do that eventually, but I want to really use the object graph
effectively first." "Jev can also do a lot of this classification. maybe we
should give research the ability to use Jev in addition to management." Told
that no desk calls Jev today: "Even if management isn't using Jev as a tool,
then research should."

Naming ruling (owner, same day, relayed by the coordinator): the desk verb is
`choir.System1`, not `choir.Jev`. System1 is the fast intuitive judge: it
returns a distribution over the choices it is given. The desk's own reading
and reasoning is System 2, and decides. Jev stays the name of the current
backing model and transport (`internal/gatewayruntime/jev.go`); the verb does
not expose that name, so the backing judge can change without the desks
changing.

## Problem

Three facts, one cause: the graph holds no subject nodes, so the only path
between two stories is a shared capture, and the one judge that could widen
the paths is out of every desk's reach.

### 1. Links exist only through shared captures

Front 60, after the live-page fix (problem doc
[related-stories-dead-end](related-stories-dead-end-2026-10-20.md), section
"After the live-page fix"): 9 stories linked, 15 pairs, 14 right. That is 9 of
60, 15% coverage. The first version of the same probe gave 12 of 60 (20%) with
10 wrong pairs; removing live blogs bought precision (about 8-13% wrong) at the
price of coverage. The signal is thin: a published version records the captures
it cites (`cites_capture` edges), and two stories are related when they share
one, by exact id or by recapture of the same canonical key. A story about the
same subject that cites different pages has no path to its sibling.

### 2. The graph has no entity or topic-thread nodes

The object kinds in the graph for a story are the document, its revisions,
the captures it cites and (phase 2, waiting on its rerun) world events. There
is no `choir.entity` and no `choir.topic_thread`, and no edge a reviewer could
write that says "this story is centrally about X" or "this story belongs to
the running subject Y". Design §2 lists the same absence: "No entity or thread
objects exist." A reviewer cannot attach a story to a subject that has no
node, whoever the reviewer is.

### 3. No desk can ask System 1 a question

Jev, the gateway-pinned choice judge, is called only by runtime-side judges
that run outside any desk's turn:

- the intake labeller: `callIntakeJudge` and `labelOneCapture`,
  `internal/agentcore/intake_labels.go:383` and `:235`, which calls
  `callJudgeWithin` (`:389`) on the `intake_labels` scope;
- the event layer: `internal/agentcore/event_layer.go:386`, the same scope;
- the support judge: `askJevForSupport`,
  `internal/agentcore/support_judge.go:221`, called at `:285` on the shared
  scope, with its cost written by `recordJevCost` (`:351`).

All three go through the one `CandidateJudge` the runtime holds
(`internal/agentcore/runtime.go:214`, bound at `internal/autoputer/run.go:613`
to `gatewayruntime.NewJudgmentClient`, `internal/gatewayruntime/jev.go:40`),
which posts to the gateway's `/provider/v1/judgments`. The model is pinned in
the gateway, not selectable by the caller (`internal/gateway/jev.go`).

The desks see Jev only as output. Management reads Jev's intake labels on its
captures records as "advice, not a decision" (management overlay,
`internal/runtimeprompts/overlays/rlm_management_runtime.yaml`, the "Intake
labels" bullet); research sees them as topic and outlet hints in
`SearchWireCorpus` results (`internal/researchtools/researchtools.go:34`).
Neither desk has a cell verb that sends Jev a question of its own
(`deskModuleSets`, `internal/yaegikernel/choir.go:320`, names none). So a
desk that wants a cheap, calibrated classification over a closed set ("same
incident, later development, same running subject, shares an entity only
incidentally, unrelated") must spend its own model's reasoning on it, or skip
it.

## Why it matters

Reviewing about 1,000 stories against their candidates is on the order of
10,000 closed pair questions (design §6 R5). Done by a desk model that is a
large cost and a slow loop; done as one System 1 distribution per question it
is the job Jev was built for. Without the verb the research desk cannot do the
review at all within budget; without the subject nodes, a reviewer has nothing
to attach to. The two are separate problems with separate fixes, so each gets
its own record: this doc covers the verb (order step 3) first, because it needs
no schema; the nodes, edges and `choir.Relate` are order step 4 and get their
own change.

## Evidence

- Coverage numbers: `related-stories-dead-end-2026-10-20.md`, sections "Staging
  finding" and "After the live-page fix" (front 60: 12 of 60 before the
  live-page fix, 9 of 60 after; 38 pairs over the top 300, about 13% strict
  wrong).
- Code citations above are from `b09328bfc`.
- Heresy (design §9): desks re-derive with their own model what Jev answers
  more cheaply. Discovered here; not yet repaired.

## Belief state and remaining error

Believed: giving research one bounded, tape-recorded `choir.System1` verb
makes the review affordable and auditable; the distribution, not a decision,
reaches the cell, so the desk keeps the decision. Not yet known: whether Jev's
distributions on the pair-relation questions are calibrated enough to take
without reading closely (design R4; the blind audit decides that, not unit
tests), and how many calls a run and a day need (the caps ship off and are set
by the owner from config).

## Fix order

1. This record (green).
2. "Ways the verb could fail" (green), before code.
3. `choir.System1` for the research desk only, with bounds, budget caps off by
   default, and a tape record that replay reads instead of calling Jev again
   (red). Management does not get the verb in this change (owner ruling).
4. Entity and thread kinds, edges, `choir.Relate`, export (later, red).
