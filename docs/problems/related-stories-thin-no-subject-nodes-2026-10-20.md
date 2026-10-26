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
  `internal/agentcore/intake_labels.go:382` and `:236`, which calls
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

## Ways the verb could fail (before code)

The verb is `choir.System1(questions)`: a research cell sends closed choice
questions, the host sends them to the one `CandidateJudge`, records the call
on the tape and returns each question's distribution. Each failure below is
named before the code, with the guard that the tests pin.

- **F1 Unbounded spend.** A loop in a cell, or a retry storm, calls the
  judge thousands of times. Guard: a per-call bound (12 questions, 2-6
  choices each, bounded text), a per-run cap and a per-computer daily cap on
  calls, both from runtime config and both off by default (0): with either
  unset the verb returns an error naming the config. Attempts count against
  the caps, not only successes, and a "requested" record is written before
  the call so a crash mid-call is still counted.
- **F2 Smuggled evidence.** A cell pastes text it never read into a question
  and the judge treats it as a source. Guard: `context_refs` name captures
  (canonical capture ids) or story versions (`story:<revision_id>`) and the
  runtime resolves their text itself, bounded, from the owner's stores; a
  ref that does not resolve refuses the whole call before any spend. The
  question text is the desk's own wording, sent in a separate field from the
  context, bounded, and the instruction to the judge says only
  `state.context` is evidence.
- **F3 Distribution collapsed into a decision.** The verb returns "the
  answer" and the desk takes it. Guard: the verb returns the probabilities
  per choice exactly as the judge gave them, no thresholding, no argmax
  field, no confidence gate; a choice the judge gave no probability for is
  absent, never filled in. The decision stays in the cell (System 2).
- **F4 No tape record.** The question, the answer and the cost live only in
  the cell's memory; no audit, no replay. Guard: a `system1.judgment` event
  (acknowledged durability, not batched) per call carrying the request
  digest, the question ids, the distributions, the model as reported, the
  usage and cost, and the transport's response id when it has one; the
  cost also goes to the run's cost line (`recordJevCost`) so `/api/costs`
  sees it.
- **F5 Replay double spend.** A cell that runs again (a retry in the same
  run, a model re-sending the same cell, a resumed run after a planned
  restart) calls the judge again and pays twice. Guard: the request digest
  covers the normalized questions and the content hash of every resolved
  context text; before any spend the host looks the digest up among the
  run's recorded answers and returns the recorded distributions, flagged
  `replayed`, without a call and without counting toward the caps. Because
  the context is hashed, a changed text is a new question, not a stale
  answer.
- **F6 Crash restart.** The computer restarts between "requested" and the
  answer. Guard: nothing resumes (restart rule); the request record stays,
  counts toward the caps, and has no answer, so a retried cell is new work
  and calls the judge once more. A crash costs at most one call.
- **F7 Transport errors.** The gateway is down, rate limited, disabled, the
  call times out, or the answer does not parse or misses a question.
  Guard: an error to the cell that says what failed, a bounded timeout, a
  failure record on the tape (counted as an attempt, never replayed), and no
  partial success is guessed at; a question the judge did not answer is
  reported as unanswered, never filled.
- **F8 Validation.** Empty list, empty or oversized text, duplicate ids,
  ids that are not simple names, fewer than two or more than six choices,
  duplicate or empty choices, too many context refs. Guard: all refused
  before any resolution or spend, with the first offending field named.
- **F9 Wrong desk sees the verb.** Management, engineering, Texture, a
  research child or a management task gets `choir.System1`. Guard: the verb
  is in the research module set only (owner ruling: management does not get
  it in this change); the session handle of every other role lacks the
  broker action; and the host refuses the action unless the session role is
  `research`, independent of the worker. Tests pin the exports of all five
  other roles, the handle, and the host check.
- **F10 Injection through context.** A capture's text tells the judge to
  answer a certain way. Guard: context is bounded and passed as data under
  a named field; the judge's answer is only a distribution the desk weighs
  and the desk's overlay says story text is data; one call's answers carry
  no authority over a graph object (there is no write verb in this change).
- **F11 Shared rate bucket.** The verb draws on the shared judgment bucket
  with the support judge. Guard: the caps keep the volume small; if the
  bucket starves the support judge in staging, a `system1` scope is a
  gateway change and its own record. Not changed here.

## Built: choir.System1 (step 3, not yet deployed)

- Cell side: `internal/yaegikernel/system1.go` (the one validation, the bounds,
  `ChoirScope.System1`); the verb is in the research module set only
  (`deskModuleSets`, `internal/yaegikernel/choir.go`), and only the research
  session handle carries `ActionSystem1`.
- Host side: `internal/agentcore/system1_verb.go`, reached from `deskEgress`
  (`internal/agentcore/tools_desk.go`) with the session role, refused unless it
  is `research`. It reuses the one `CandidateJudge` and `callJudgeWithin`, the
  shared parsing type `jevDistribution`, and `recordJevCost` (now
  `recordJevCostFor`, same event, agent and purpose named by the caller).
- Bounds: 12 questions per call, 2-6 choices of 80 characters, question text
  400 characters, 4 context refs per question and 16 distinct per call, each
  resolved text 1000 characters (the intake labeller sends 600 per capture
  summary), judge call timeout 60 s.
- Config (runtime environment, positive integers, both required, default off):
  `CHOIR_SYSTEM1_MAX_CALLS_PER_RUN`, `CHOIR_SYSTEM1_MAX_CALLS_PER_DAY`. The
  day is the UTC day and the count is per computer. `nix/node-b.nix` is not
  changed here; enabling means setting both in the guest runtime's environment.
- Tape: `system1.judgment` events (`types.EventSystem1Judgment`, acknowledged)
  on channel `system1:<computer>`: `requested`, then `answered` or `failed`.
  Replay reads the `answered` record of the same run with the same request
  digest.
- Not done, by decision: management does not get the verb (owner ruling); the
  gateway rate scope stays the shared one (F11); entity and thread nodes,
  `choir.Relate` and the export are step 4.

## Fix order

1. This record (green).
2. "Ways the verb could fail" (green), before code.
3. `choir.System1` for the research desk only, with bounds, budget caps off by
   default, and a tape record that replay reads instead of calling Jev again
   (red). Management does not get the verb in this change (owner ruling).
4. Entity and thread kinds, edges, `choir.Relate`, export (later, red).

## Re-measured 2026-10-26 (owner: "a lot of the top stories are missing their related stories section, even though they definitely have related stories")

Read-only, public GETs of https://choir.news/api/platform/publications/front?limit=60
and the related route for each story, about 00:50 UTC (Sonnet
investigation; report kept outside the repo). 14 of the 60 front stories
have any related link (23 percent), 6 of the top 20. Every answer was
partial false with no errors, so the budget, kill switch and cache are not
involved; it is not a rendering bug or a recent regression (the 10-20
measurement after the live-blog fix was about 15 percent). Coverage tracks
cited captures: 1 of 34 stories citing 2 captures, 6 of 13 citing 3 or 4,
7 of 13 citing 5 or more. The paper now writes most stories from two
sources, so shared-capture overlap is rare. Three same-subject pairs share
no capture and have no link either way: the two Vucic-concedes stories,
the two "20,000 California ballots" DOJ stories, and the El-Sayed
metal-detector and El-Sayed/Rogers debate stories. The walk
(internal/platform/publication_related.go relatedWalk, relatedCandidates)
has no event, entity, topic or related_story path; phase 3 (the research
desk's System 1 linking, design above) was never built, and choir.System1
now exists (research.system1, on for the paper). Fix shape unchanged: a
related_story edge written by research's System 1 review over graph
candidates, read by relatedCandidates as extra candidates behind a data
switch, with the precision check (at most 10 percent wrong over 40 or more
pairs); do not lower the threshold or the hub cutoff.
