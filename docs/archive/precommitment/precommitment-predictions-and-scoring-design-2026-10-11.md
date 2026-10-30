# Predictions required and scoring: design for protocol stations P3 and P4

> **Revised 2026-10-12** ([re-engineering decisions](../../re-engineering-decisions-2026-10-12.md)): Runtime scoring is withdrawn: scoring needs intelligence (Jev, a model or an RLM, often from several perspectives). The prediction shape is replaced by hypothesis-shaped Casts.

Draft for owner review, 2026-10-11. Stations P3 (predictions required) and
P4 (scoring) of
[definitions/choir-precommitment-protocol-metamission-2026-10-11.md](../definitions/choir-precommitment-protocol-metamission-2026-10-11.md).
Parent design:
[precommitment-protocol-design-2026-10-11.md](../../precommitment-records.md)
(§4 the prediction, §5 resolution and scoring, §7 stations, §9 decisions,
§10 one schema). Mechanism:
[Precommitment Records — Engineering Memo](../../precommitment-records.md).
Mutation class for both stations: red (lifecycle reducer, desk surface,
evidence ledger, scoring).

Owner, 2026-10-11: "Predictions mandatory." "Scorer not sure. Maybe self
scoring, maybe answering desk, maybe mgmt possibly with jev. Gotta figure
out from iteration, trust your judgment." "Panel is good for uncertainty,
not ceremony." And earlier: the same records carry Autopaper claims and
inferences, scored over time by others, "not our hegemonic house view
only", with scores scoped by scorer and topic.

## Summary

**What I found.** The code already has most of the pieces: typed
questions, a mechanical resolution on reply, a Jev transport that the
runtime already calls for self-development judgments, and an acting pack
that is score-free by design. Five things P3 and P4 have to change or
respect, all verified in code:

1. Casts are directive records, and directives are dropped from every
   scoring view and from the desk pack's query.
2. Ask records carry a question with no probability.
3. The packet delivered for a Precommit copies the asker's probabilities
   into the addressee's notes. This breaks blind reading the moment anyone
   uses it.
4. The reply's mechanical resolution scans up to 10,000 records per
   report.
5. Records carry no model id, so calibration per model is impossible
   today.

Real traffic (the P2a demo) shows eight dispatches in 27 minutes sharing
two work items. Research's answers are therefore not linked to the cast
they answer, and the count resolver has to recover that link.

**The main design idea.** Separate the *answer* from the *score*. Scoring
a resolved prediction is arithmetic: a proper scoring rule over the frozen
probability and the answer. The only judgment is "what was the answer".
So the question of who scores becomes the question of who supplies
answers, and there are cheap sources:

- the runtime counting what happened (no model call);
- the answering desk filling in typed answers in its reply, without seeing
  the asker's probabilities (no extra call);
- Jev, run by the harness on management's authority (about $0.0001 per
  record);
- the asker's own optional resolution (no extra call).

Each source writes its own score record. Count-resolved outcomes are the
ground truth used to score the scorers.

**How sure I am.** High on the record and reducer design, because it is
grounded in the code paths named below. Medium on which scorer will win,
which is the point of the experiment. Lower on the count vocabulary's
coverage. My guess is that 60–80% of the questions desks ask in the AI
news demo can be written as countable predicates. That guess is a
hypothesis, and P3's demo measures it.

**What I recommend.**

- **P3.** A mandatory `expect` argument on Cast, Ask and Escalate: one to
  four yes/no questions, each either a count predicate from a closed
  vocabulary or a free question with a stated criterion, plus a deadline.
  It is validated inside the cell, so the desk can fix it in the same
  cell, and again by the reducer.
- **P3, Notes.** A Note can no longer open or start work, so it cannot
  become the unscored way around the rule.
- **P3, resolver.** A count resolver that runs with no model call,
  produces idempotent results and fits the crash rule.
- **P4, starting default.** Count wherever a question is countable. Jev,
  on management's authority and run by the harness, for the rest. The
  answering desk's typed answers and the asker's self-resolutions are
  recorded beside them but do not govern.
- **P4, scoring the scorers.** Jev also scores a 25% sample of countable
  questions without seeing the count, so its accuracy is measured.
- **P4, panel.** An agentic-consensus panel audits only the disagreements,
  as the owner said: for uncertainty, not ceremony.
- **P4, management's model.** Management's own model stays off as a
  per-record scorer. It costs a desk turn on the ChatGPT account that
  already runs out.
- **P4, what desks see.** Each desk sees only its own calibration, from
  count-resolved questions, and never a model scorer's judgment of its own
  predictions.
- **Specificity.** Specificity is a skill score against a base rate: a
  prediction that copies the base rate scores zero. An answer of
  "unresolvable" costs as much as a wrong answer.
- **The bet worth making.** The best scorer is no scorer. Most effort
  should go into making questions countable, not into building better
  judges.

## 1. What exists today

### 1.1 The record, the acts and the views

- `TypedQuestion` (question, answers, probabilities, materiality weight)
  and `CommitmentPrediction` (hypothesis, assumptions, alternatives,
  questions, committed time) exist
  (`internal/types/commitment.go:75-122`). `CommitmentScore` sits inside
  the record as `Scores []CommitmentScore` and names only a scorer model
  (`commitment.go:138-146`, `:198`). Design §9 item 6 already says not to
  migrate it. New records should leave it empty for good.
- **Precommit** requires a question, a distribution and a resolver desk
  (`internal/agentcore/rlm_reduce.go:233-247`). It has no deadline and no
  subject key.
- **Ask** mints a precommit-kind record whose `Precommit` has a question
  and a resolver but no distribution (`rlm_reduce.go:1113-1129`). So an
  Ask is "a staked request" in name, but nothing is staked.
- **Cast** mints a directive record (`rlm_reduce.go:1133-1135`). Texture's
  Cast, Ask and Note fold into turn controls
  (`internal/agentcore/texture_dispatch_fold.go:98-131`). The control
  accumulator mints each record as a directive of subtype cast, with
  `staged_act` in its subject references
  (`internal/store/lifecycle_control_issue.go:352-383`).
- **Directives are excluded from every scoring view.** `ResolveCommitments`
  skips them (`internal/types/commitment_views.go:252`), and since P1 the
  desk pack's query excludes them before the window
  (`ListCommitmentClaimRecords`, `internal/store/commitment.go:130`). A
  Cast with a prediction would be invisible to calibration unless P3
  changes this.
- **Reply resolution is mechanical.** When a report links to an open
  precommit-kind record, `CommitLifecycleAct` appends a resolution with
  verdict `answered` in the same transaction
  (`internal/store/lifecycle_commit_act.go:136-183`). It is an "arrival"
  resolution and carries no material verdict. To find out whether the
  target is already resolved, it lists up to 10,000 records
  (`lifecycle_commit_act.go:144`). That is linear in ledger size, and past
  10,000 records the check silently misses older resolutions. The count
  resolver must not copy this pattern.
- **Blind reading is broken for Precommit.** `precommitPacketForRecord`
  writes `distribution:<answer>=<p>` notes into the packet the addressee
  receives (`rlm_reduce.go:1730-1732`). The memo's blind-reading option
  and the anti-anchoring guard both need the addressee *not* to see the
  asker's probabilities. This is harmless today only because no desk calls
  Precommit (problem record). It must change before predictions become
  mandatory.
- **The acting pack is score-free by construction.** It carries no score,
  distribution or disagreement fields (`commitment_views.go:84-112`). This
  boundary was owner-ratified in the 2026-09-29 Jev/management design
  ("ActingPack never carries commitment scores",
  `definitions/choir-signal-management-scorer-jev-2026-09-29.md`). Design
  §7 P4 says "each desk's pack shows its calibration". These two can be
  reconciled (§4.7).
- **Records carry no model id.** P1's staging audit record shows
  `"model_id": ""` on every control record (session evidence
  `scratchpad/p2a/audit-raw.json`). The memo says biases differ by model,
  so packs are per model. Without the model id, per-model calibration
  cannot be computed.

### 1.2 Jev today

- Transport landed (M4, `docs/archive/evidence/m4-jev-transport-roundtrip-2026-09-30.md`).
  It runs as gateway `POST /provider/v1/judgments` with the model pinned
  server-side (`typesafe/jev-1.13-20260917`). Question types are `noul`,
  `choice` and `score` (`internal/gateway/jev.go:152`). The measured cost
  of a 331-token request was $0.0000139. The rate limit is 60 per minute
  per computer.
- The guest already calls Jev from the runtime, not from a desk: it judges
  frozen self-development candidates with three yes/no choice questions
  (`internal/agentcore/selfdev_candidate_judgment.go:40-70`, `:185`). This
  is the pattern for a harness-run scorer that desks cannot query.
- The 2026-09-29 owner decision was that the scorer is management, with
  Jev as its default method and an escalation to its own model on low
  confidence (`orientation-jev-supervision-metamission-2026-09-29.md` §B).
  That station (M5) is `blocked_incomplete`. Today's owner words reopen
  the choice. This design keeps "management owns scoring, Jev is its
  default method" as the identity on the score record. It moves the call
  into the harness so that no management turn is spent per record.

### 1.3 Real traffic

The problem record counts zero `Precommit`, `Resolve`, `Ask` or `Reply`
calls in two days of staging traces
(`problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md`).

The P2a deployed demo (AI news with a follow-up, staging a2bcf724,
19:19–19:48Z; session evidence `scratchpad/p2a/`) gives the shape of the
traffic P3 will carry:

- Eight agent dispatches to research: seven Casts and one Note. All went
  to the same research desk across **two work items**, one per owner
  request (`e576fa59…` for the follow-up).
- Eleven research updates queued. Six were applied and five rejected. The
  last rounds' reports carried no sources because research's search timed
  out at the guest
  (`problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md`).
- Round one reported "Confirmed primary source: Anthropic's official page
  documents Claude Haiku 5.5".
- Casts at 19:25:59, 19:29:09 and 19:30:09 each asked again for "primary
  confirmation of the Mistral Large 4 item". The research run that
  completed at 19:41:48 reported "Confirmed: Mistral announced Mistral
  Large 4 on 6 October 2026 in its own newsroom". Matching that report to
  the cast it answers comes from the timeline, not from a link on the
  records. That missing link is itself a finding (§3.7).
- Texture's Note (19:46:56) told research to "stop sending status-only
  packets", and it continued research's work. Under P3, a Note that wakes
  research to do another round would be a request for work with no
  prediction.

Questions that would have resolved by count in this demo:

| Question | Count predicate | Outcome in the demo |
|---|---|---|
| Primary source for Haiku 5.5? | `domain_match` hosts anthropic.com | yes |
| Mistral confirmed on its own site? | `domain_match` hosts mistral.ai | no, no, then yes (by timeline) |
| Any new source? | `new_sources` ≥ 1 | 0 in the last rounds |
| Will the report be incorporated? | `incorporated` | 6 of 11 applied |

## 2. The principle: separate the answer from the score

A resolved prediction is scored by a proper scoring rule (Brier to start)
over two things: the frozen probability `p` and the outcome `o`. Once `o`
is known, the score is arithmetic. All the judgment is in finding `o`. So:

- A **score record** stores one answer source's outcomes for a record's
  questions, with that source's identity and evidence. It does not store a
  Brier number. Brier, calibration and skill are views computed at query
  time, so the scoring rule can change later without rewriting records.
  The memo says "Scores are views".
- Any number of answer sources write score records for the same target.
  Disagreement between them is kept, never collapsed.
- The **governing answer** for a question, used for a desk's calibration,
  is a query parameter: whose answers do I trust, for which question type.
  It is not a field. This is the same mechanism §10 needs across
  computers: a reader of another edition picks which scorers to trust.

This removes most model calls. The runtime can count. The answering desk
already knows what it found, and it can say so in typed form without
seeing what the asker expected.

## 3. P3: predictions required

### 3.1 The shape a desk writes

```go
// The third (Ask, Escalate) or fourth (Cast) argument.
choir.Ask("research", "Is Mistral Large 4 confirmed on Mistral's own site?",
    map[string]any{
        "by": "45m",
        "q": []map[string]any{
            {"text": "Mistral's own site confirms Large 4",
             "count": "domain_match", "hosts": []string{"mistral.ai"}, "min": 1, "p": 0.55},
            {"text": "At least 2 new sources", "count": "new_sources", "min": 2, "p": 0.6},
        },
        "why": "a single outlet ran it; vendors usually post launches the same day",
    })
```

Stored form (an additive sub-object on any act record):

```go
type CommitmentExpectation struct {
    Questions []ExpectQuestion `json:"questions"`       // 1 to 4
    By        string           `json:"by"`              // RFC3339, stamped from a duration at stage time
    Why       string           `json:"why,omitempty"`   // one line, optional
    // Runtime-stamped at commit; never model-authored:
    Baseline  map[string]string `json:"baseline,omitempty"` // doc head revision id, work item id, round ordinal, model id
}

type ExpectQuestion struct {
    ID        string   `json:"id"`                  // q1..q4, assigned at stage time
    Text      string   `json:"text"`
    Count     string   `json:"count,omitempty"`     // predicate from the closed vocabulary (§3.3)
    Min       int      `json:"min,omitempty"`       // count: yes means observed >= Min
    Hosts     []string `json:"hosts,omitempty"`     // domain_match only
    Refs      []string `json:"refs,omitempty"`      // cited_next_revision: the sources or captures in question
    Criterion string   `json:"criterion,omitempty"` // judged questions: the observation that makes it yes
    P         float64  `json:"p"`                   // probability the answer is yes
    // Runtime-stamped: the outcome is the emitter's own later act.
    SelfControlled bool `json:"self_controlled,omitempty"`
}
```

Design §4 lists binary, count, date and choice questions. P3 ships yes/no
questions only, with counts as thresholds ("count ≥ Min"). A date question
is a yes/no question with a deadline ("by Friday"). A choice question can
be added when a desk needs one; Jev's `choice` type already supports it.
Keeping one question type keeps the authoring, the scoring and the Jev
mapping trivial.

Validation, applied the same way at the verb and in the reducer:

- 1 to 4 questions. Each has `p` in [0.02, 0.98]. Exact 0 and 1 are
  refused, because a dogmatic forecast cannot be recalibrated (the memo's
  Jev finding applies equally to desks).
- Each question has either a known `count` predicate with valid
  parameters, or a non-empty `criterion`.
- At least one question is not a liveness predicate. `answered_by` alone
  is refused (§3.3).
- `By` falls between 2 minutes and 30 days after commit. If omitted it
  defaults by addressee: research 45 minutes, management 2 hours,
  engineering 24 hours, `texture:<doc>` 2 hours. The defaults are tunable
  and recorded on the record as frozen values.
- `Why` is optional. The memo calls the reasons "the unit of learning",
  but making them mandatory doubles authoring cost before we know they
  are read. P4's data can promote them: if surprises with a `why`
  produce better revisions, require it.

### 3.2 Which acts must carry one

| Act | Expectation | Note |
|---|---|---|
| Cast (any desk, including Texture's folded Cast) | required | opens or continues work |
| Ask (including Texture's folded Ask) | required | |
| Escalate | required | e.g. "search restored within 1h? p=0.3" (`search_ok` predicate, later) |
| Precommit | is one | `{question, distribution, resolver}` decodes as a one-question expectation; old bodies stay readable |
| Note | not allowed to open or start work | see below |
| Report, Reply, Resolve, Disagreement, Retract | none | they answer or close |
| Owner acts (prompt-bar, owner revisions, owner management-open) | exempt | not agent speech (`EmitterKind owner`) |
| Runtime observations (polls, deadlines) | exempt | system provenance |

**Notes.** In the P2a demo, Texture's Note folded into `open_researcher`
and continued research's work (`texture_dispatch_fold.go:116-119`). If
predictions are mandatory on Cast but Notes still wake a desk to work, the
cheapest strategy for any desk is to stop casting and start noting. P3
should change the Texture fold so that a Note is delivered to the
addressee's open work without counting as a round and without arming a
new activation. The addressee reads it at its next wake. A Note to a desk
with no open work on the subject is refused, with a message naming Cast.
Other desks' Notes open no work item. They pass `CanMessage`
(`rlm_reduce.go:1876-1882`), but their directive packet still wakes the
addressee (`wakeUpdatedCoagent`, `rlm_reduce.go:1919-1924`). So the rule
for every desk is the same: a Note's packet is information for the
addressee's next turn, and the addressee's prompt must not treat it as a
work request. Only the Texture fold opens work from a Note today, so the
code change is local to that fold. The prompt change covers every desk. The guard is to track each desk's Note-to-Cast ratio as
a Goodhart detector (§5).

### 3.3 The count vocabulary

Every count predicate is computed from runtime-observed state. None is
model-claimed. Each lists the observations it reads, so any reader can
recount from the evidence references.

| Predicate | Counts | Observed from | Resolves when |
|---|---|---|---|
| `answered_by` (liveness) | an answering report or reply linked to the record before `By` | commitment records | answer or `By` |
| `new_sources` | distinct source URIs in the answering packets that are not in the subject document's sources at commit time | packet `Sources[].Target.URI` (`internal/types/evidence.go:83`), the head revision's `Provenance.Sources` named by `Baseline.doc_head` (`internal/types/texture_provenance.go:25-41`) | answer, or `By` |
| `domain_match` | answering-packet sources whose URI host equals or is under one of `Hosts` | packet sources | answer, or `By` |
| `incorporated` | the answering packet was applied (not rejected) by the Texture | the `update_applied` and `update_rejected` lifecycle events | the Texture's disposition, or `By` |
| `cited_next_revision` | `Refs` (default: the answer's source URIs) that appear in a revision committed after the answer and before `By` | revision `SourceEntities` and `Provenance.Sources` | a revision, or `By` |
| `revisions_by` | revisions of the subject document committed before `By` | revision rows | `By` |
| `assignment_state` | the engineering assignment reached a named state (verified, accepted) before `By` | assignment record | state change, or `By` |
| `captures_matched` (P5) | captures matched to a story before `By` | `choir.web_capture` objects (Autopaper W2) | `By` |

The vocabulary is closed in code. Adding a predicate is a small reviewed
change, because the predicates are the ground truth that everything else,
scorers included, is measured against.

`domain_match` matters most for the AI news demo. It turns "found a
primary source?" from a judgment into a count, as long as the desk names
the host it expects. A desk that does not know the host asks a judged
question instead and pays for it in specificity (§4.6).

**Self-controlled questions.** When a predicate's outcome is the emitter's
own later act (for example Texture predicting `cited_next_revision` on its
own document), the runtime sets `SelfControlled`. Such questions are
recorded and resolved, but excluded from calibration and from scorer
ground truth. A desk can always make its own prediction come true, which
is the memo's "self-fulfilling predictions" caution.

### 3.4 Examples from the AI news demo

- **Texture → research, first round** (`Cast`, `by` 45m):
  - "A primary source for Claude Haiku 5.5": `domain_match` anthropic.com
    ≥ 1, p=0.8.
  - "At least 4 new sources": `new_sources` ≥ 4, p=0.6.
- **Texture → research, follow-up rounds** (the 19:25–19:30 Casts):
  - "Mistral's own site confirms Large 4": `domain_match` mistral.ai ≥ 1,
    p=0.5.
  - "At least 2 new sources on policy, chips or funding": `new_sources`
    ≥ 2, p=0.5.

  Asked three times, the first question would have resolved no, no and
  then (by the timeline) yes. Texture's pack would have shown the miss
  before the third ask.
- **Texture → research, last round** (19:46:01): "At least 1 new source":
  `new_sources` ≥ 1, p=0.7. It resolves to 0, which is exactly the "status
  only" waste Texture noticed by hand and tried to fix with a Note.
- **Texture → management** (`Cast` with an `execution_request` packet):
  "The assignment is verified within 24h": `assignment_state` verified,
  p=0.6. "The change is limited to the objective": judged, with a
  criterion.
- **Management → `texture:<doc>`** (feeding a story, P2d/P5): "Texture
  cites at least one of these captures within 2h": `cited_next_revision`
  with `Refs` set to the capture ids, ≥ 1, p=0.7.
- **Management Precommit on a story** (P5): "At least 10 captures on this
  story within 24h": `captures_matched` ≥ 10, p=0.5.
- **Management → engineering**: "The capsule passes verification within
  24h": `assignment_state` verified, p=0.7.
- **Research → management Escalate** (search rate-limited): "Search works
  again for this desk within 1h": a judged question to start (criterion:
  a search from research returns results). It becomes `search_ok` once
  the search plane exposes per-desk outcomes.

### 3.5 Where the expectation lives, and what the addressee sees

**On the act record itself**, as `Expectation *CommitmentExpectation`,
frozen in the same object and the same content hash as the act. The
alternative was a sibling precommit record minted in the same commit. I
rejected it for three reasons:

- two records for one act must stay paired across every derived view and
  every export;
- Texture's acts are minted by the accumulator, not by
  `commitLifecycleActIntent`, so the pairing would need to be built twice;
- §10's commit-time proof is cleanest when the prediction and the request
  share one hash.

Consequence: views decide by `Expectation != nil`, not by record kind. A
new view (`ResolveExpectations`) joins expectation-bearing records of any
kind with their score records. The existing claim views
(`ResolveCommitments`, accruals, materiality) stay as they are, so P3
changes nothing a desk or the owner already sees. That is the same
discipline P1 used.

**The addressee sees the questions, never the probabilities.** The packet
delivered for a Cast or Ask lists each question's id, text, predicate or
criterion and the deadline. These are the request, and research needs to
know what it is being asked. The `p` values and the `why` stay off the
packet. P3 also removes the distribution notes from
`precommitPacketForRecord` (`rlm_reduce.go:1730-1732`). The answering
desk can then supply typed answers blind (§4.3).

The record also gets its `ModelID` from the emitting run (P1 left it
empty), plus a `Baseline` the runtime stamps: the document head revision
id at commit (for `new_sources`), the work item id and round ordinal (for
linking answers, §3.7), and the emitting model.

### 3.6 How the reducer refuses a missing one

Defense in two layers, with one authority:

1. **At the verb, in the cell.** The `expect` argument becomes part of the
   signature: `Cast(desk, objective, spec, expect)`, `Ask(toDesk,
   question, expect)`, `Escalate(toDesk, issue, expect)`.
   - A model that omits it gets a Yaegi arity error. A model that passes a
     malformed one gets an error returned inside the same cell, the
     pattern `ReportPacket` already uses ("checks the packet at the call
     and returns the error in the same cell", `rlm_research_runtime.yaml`).
   - The model can fix it without losing the cell. This matters for
     Texture, where a reducer refusal fails the whole cell, edit included
     (the P2a fold rule).
   - The error text names the field and gives a one-line valid example.
   - `StagedIntent` gains `Expect string` (JSON), like its existing
     `Precommit` field.
2. **In the reducer, as the authority.** `validateSemanticActIntent`
   (`rlm_reduce.go:161`) checks the expectation for `IntentCast`,
   `IntentAsk` and `IntentEscalate` with the same validator. The Texture
   fold (`texture_dispatch_fold.go:98-131`) copies the expectation into
   the control. `TextureTurnControl` (`internal/types/lifecycle.go:184`)
   gains `Expectation`, and `mintControlRecord`
   (`lifecycle_control_issue.go:352`) writes it onto the record. A fold
   that receives a Cast or Ask without one refuses it.

**Replay.** A tray staged before the deploy has no expectation, and
`recoverPartialActCommit` must still replay it. This is P2c's
drain-on-old-kinds rule. The tray carries a schema version stamped at
stage time. Version-1 trays skip the expectation check, and the branch is
removed one release later as a named residual. Adding `Expectation` to
the control changes Texture's turn command digest, so the replay goldens
change with this commit. That is expected, not a regression.

**Refusal is measured.** P3's acceptance counts in-cell expectation errors
per desk per demo. A desk that keeps failing is a prompt or vocabulary
problem, not a reason to relax the rule.

### 3.7 The count resolver

- **What it writes.** One score record per target with method `count` and
  scorer `system:count` on this computer. It holds a per-question outcome
  (yes, no or interrupted), the observed count, and the evidence
  references counted (URIs, revision ids, event ids). Its id is
  deterministic: `<target>:score:count`, with the not-exists condition
  `CommitLifecycleAct` already uses. A replay mints nothing.
- **When it runs.**
  - Some predicates resolve on arrival: `answered_by`, `domain_match` and
    `new_sources` on the first answering report.
  - The others resolve on a later event or at `By`: `incorporated` on the
    Texture's disposition; `cited_next_revision` and `revisions_by` on a
    revision commit or at `By`; `assignment_state` on a state change or
    at `By`.
  - The resolver is a pure function of stored state, called after those
    commits and by a sweep for records past `By`. It needs no model.
  - **Hypothesis to settle during implementation:** whether arrival
    resolution can ride inside `CommitLifecycleAct`, as the mechanical
    `answered` resolve does. Keeping it out of the trajectory transaction
    avoids lengthening the trajectory lock, so the default is a
    post-commit step that is idempotent by id.
- **Linking answers to the cast they answer.** P2a showed that follow-up
  rounds share one work item, and producer reports bind to the work item,
  not to a cast record (`commitLifecycleReportActIntent`,
  `rlm_reduce.go:1962-2114`). Today nothing on a report names the round it
  answers. The runtime stamps it:
  - When research's run is activated by a delivered control, the run's
    binding carries that control's `source_record_id`.
  - Research's next producer report gets `RelatedIDs` set to that record,
    as a system-derived field, not a model claim.
  - Fallback when no binding exists: the expectation's answer window is
    every report on the same work item after the record and before the
    next expectation-bearing act on that work item, or `By`.

  The trace audit in acceptance checks this join one to one, as P1's audit
  did for controls.
- **Crash restarts.** CLAUDE.md requires that a crash restart close open
  work with "interrupted by a restart" and resume nothing. The resolver is
  not work: it wakes no desk and only records what happened. So it is not
  an armed obligation and needs no planned-restart marker. A record whose
  work item was closed by a crash restart resolves each question as
  `interrupted`. Interrupted outcomes are excluded from calibration and
  from scorer ground truth, because they are the platform's failure, not
  the world's answer.
- **Indexes.** The resolver and the mechanical `answered` resolve look up
  score and resolution records by target. P3 adds a body index on the
  target id (`$.score.target_id`, `$.related_ids`) and replaces the
  10,000-record scan at `lifecycle_commit_act.go:144` with a direct
  lookup. Otherwise the cost of resolution grows with ledger size, and
  Autopaper's claim volume makes that fatal.

## 4. P4: scoring

### 4.1 The score record

A new record kind `score`. It is a `choir.commitment_record`, so it gets
the same store, hash, commit time and §10 emitter fields as every other
record:

```go
type CommitmentScoreRecord struct {
    TargetID      string                 `json:"target_id"`
    Scorer        ScorerIdentity         `json:"scorer"`
    Answers       map[string]ScoreAnswer `json:"answers"`         // by question id
    SawPrediction bool                   `json:"saw_prediction"`  // could this source see the asker's p?
    ScoredAt      string                 `json:"scored_at"`
}

type ScorerIdentity struct {
    ComputerID string `json:"computer_id"`
    Desk       string `json:"desk,omitempty"`     // management, research, texture, ...
    AgentID    string `json:"agent_id,omitempty"`
    ModelID    string `json:"model_id,omitempty"` // pinned version
    Method     string `json:"method"`             // count | answerer | self | jev | llm | management | panel | owner
}

type ScoreAnswer struct {
    Outcome  string   `json:"outcome"`            // yes | no | unresolvable | interrupted
    P        float64  `json:"p,omitempty"`        // the source's confidence in yes (Jev's distribution; 1/0 for count)
    Observed int      `json:"observed,omitempty"` // count predicates
    Evidence []string `json:"evidence,omitempty"`
}
```

- **Ids.** `<target>:score:<method>:<scorer agent or model>`. One score
  per scorer per target. A rescoring by a newer model version is a new
  identity, because the model id differs.
- **Never a field on the claim.** `CommitmentRecord.Scores` is never
  written for new records. The existing `Disagreement` act stays for
  desks that dispute a resolution. Disagreement between scorers is
  derived: two score records with different outcomes for the same
  question.
- **§10, other computers.** An edition on another computer that scores
  our claim writes the same record with its own `ComputerID`. Nothing in
  the shape is specific to one computer.

### 4.2 The answer sources

| Source | Sees the asker's p? | Extra model calls | Bias | Role at start |
|---|---|---|---|---|
| **count** (runtime) | n/a | 0 | none beyond resolver bugs | governs every countable question; ground truth for scoring scorers |
| **answerer** (the desk that replied; typed `answers` on its Reply or Report) | no (the packet strips p) | 0 (a few output tokens) | flatters its own work ("yes, I found the primary source") | recorded; checked against count where checkable; candidate to govern judged questions |
| **jev** (harness-run, scorer identity: management desk, method jev) | no | 1 per record with judged questions, all questions batched; plus a 25% blind sample of countable records | literal reading; per-type miscalibration (memo: choice overconfident, noul underconfident) | **governs judged questions at start** |
| **self** (the asker's own `Resolve` with typed answers, optional) | yes, its own | 0 (rides the cell that reads the answer) | flatters its own forecast | recorded; never governs; the self-favor gap is the oversight number |
| **management** (management's own model, a desk turn) | configurable | 1 desk turn (tens of thousands of input tokens) | shares a provider with research | **off**, enabled only for an experiment window |
| **panel** (agentic-consensus, several frontier models, offline) | no | many, but only on a sample | diverse models, so blind spots are less correlated | audits disagreements; supplies gold labels for judged questions |
| **owner** | optional | 0 model calls; owner time | n/a | gold labels on a handful of records, only when the owner wants to |

Why the answering desk can answer blind: the packet lists the questions,
and research has to know them anyway to do the work. A Reply or Report
gains an optional `answers` map (`{"q1": {"outcome": "yes", "evidence":
["https://mistral.ai/news/..."]}}`). It is minted as an `answerer` score
record in the same commit as the report. Where a claim is checkable, the
runtime checks it. A claimed `domain_match` must point at a URI that is
actually in the packet's sources. A mismatch is itself recorded, as the
answerer's honesty track record.

Why Jev and not management's model at start:

- Jev is a different model family from every desk model, so its errors
  are less correlated with the asker's.
- It is about four orders of magnitude cheaper per record.
- It cannot be in-context gamed by a desk, because no desk calls it.
- The runtime already calls it from guest code.

Management's own model would spend a turn on the ChatGPT account that
already stops research and management
(`problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md`).
Recording Jev's score as "management, method jev" keeps the 2026-09-29
decision that scoring belongs to management. Management *reads* scores of
other desks' predictions as input to spend and routing (supervision), and
can take over scoring for any question type the experiment says Jev
handles badly.

**The Jev call** maps each judged question to a `choice` question with
criteria taken from the desk's `criterion` and choices yes, no and
unresolvable. A fixed specificity question is added: "Could two careful
readers disagree about whether this was met?" The state given to Jev is
the question text, the criterion and a capped observation excerpt (the
answering packet's claims and sources, at most about 3,000 tokens). It
never includes the asker's p. It is one call per record, made by the
harness after the answering report commits, queued under the 60-per-minute
bucket.

### 4.3 Starting default

1. If a question is countable, the count governs.
2. If not, Jev governs; if Jev is unavailable, the answerer's typed answer
   governs, marked as provisional.
3. The answerer's and self answers are always recorded when given.
4. Jev also scores a random 25% of countable records without being shown
   the count (shadow scoring).
5. Management's model stays off. The panel audits weekly at most 30
   records, chosen from disagreements.

Governing order today: count > owner > panel > jev > answerer > self. The
order is a view parameter, so the experiment changes it without touching
records.

### 4.4 The experiment that changes the default

A scorer is good if its answers match ground truth. There are three
sources of ground truth, each with a known weakness:

- **E1, shadow scoring on countable questions.** Jev answers countable
  questions without the count, and the answerer's typed answers on
  countable questions are compared with the count. This measures accuracy
  and calibration per scorer per question type. Weakness: countable
  questions are easier than judged ones, so E1 flatters every scorer.
- **E2, judged questions that become countable later.** Some judged
  questions have a countable twin. "Is this a primary record?" is checked
  later by `domain_match` once the expected host is known. "Will this
  claim hold?" is checked by a later confirming or contradicting report.
  For Autopaper this is the main source of truth, and it arrives late.
  Weakness: sparse at first.
- **E3, panel audit of disagreements.** When Jev and the answerer, or Jev
  and self, disagree on a judged question, the agentic-consensus panel
  labels a sample. Weakness: the panel is models too, and it costs a
  developer budget. This is what the owner meant by "panel is good for
  uncertainty, not ceremony": it looks only where scorers disagree.

Decision rules, applied per question type once there are at least 150
ground-truth items for it (enough for about ±5 points of accuracy at 95%):

- **Answerer accurate.** The answerer agrees with ground truth on 95% or
  more of items, and its yes-bias is under 5 points. Then the answerer
  governs judged questions of that type. Jev drops to a 20% audit sample
  and keeps measuring drift.
- **Jev weak.** Jev's accuracy is below 85% on a type. That type routes to
  a second LLM scorer (method `llm`) or to management's model for a trial
  window. Repeat E1/E3 on the new scorer.
- **Self-scores.** Self-scores never govern. If the self-favor gap stays
  under 5 points for a desk and model, self-resolutions become the cheap
  learning signal in that desk's own revisions. The memo: "a learning
  signal even though it is not an oversight signal". A self-favor gap is
  self answering yes where ground truth says no, minus the reverse.
- **Count already covers most questions.** If 70% or more of all questions
  are countable after two weeks of demos, stop investing in judges.
  Expand the predicate vocabulary instead. This is the outcome I expect.

Timeline guess: the AI news demos produce about 8 dispatches and about 15
questions per half hour. At a few demos a day, E1 reaches 150 items per
main predicate in about one to two weeks. Judged question types will take
longer. This is a hypothesis to correct with the first week's counts.

### 4.5 Specificity: vague and trivial predictions must not score well

Three guards, all arithmetic except one Jev question.

1. **Skill against a base rate, not raw Brier.**
   - For each predicate and threshold, the view keeps a base rate `b`:
     the observed yes-frequency over this desk's resolved questions on
     this computer, smoothed toward 0.5 until there are 20 items.
   - The record's skill is `(b - o)² - (p - o)²`.
   - A desk that copies the base rate earns zero. A confident correct
     prediction earns positive skill, and a confident wrong one negative.
   - "Will research answer within 45 minutes? p=0.97", when research
     always answers, earns about zero.
   - Base rates are shown to the desk (§4.7). Knowing them improves
     calibration but cannot earn skill.
2. **Unresolvable counts as wrong.** If the governing source answers
   unresolvable, the question scores as if the outcome went against the
   side the desk favored. A vague question costs as much as a wrong one,
   so desks have a reason to write countable questions or crisp criteria.
3. **The specificity question.** For judged questions, Jev's "could two
   careful readers disagree?" answer is recorded per question. Each desk's
   rate is tracked. It is reported, not folded into the score, until E3
   shows it predicts the panel's "unresolvable" labels.

The liveness rule in §3.1 removes the cheapest trivial prediction. The
skill score handles the rest. Materiality weights (the memo's
likelihood × impact × relevance) are not part of P4. They come back when
critics exist that can find omitted consequences.

### 4.6 What a desk sees

The acting pack stays free of scorer judgments. Desks see
**observations**: their own count-resolved outcomes, arithmetic over them,
and base rates. This is the memo's "feed observations and discrepancies;
keep independent scores out of the agent's context". It keeps the
2026-09-29 rule that the acting pack never carries commitment scores,
read as "never carries a scorer's judgment". So:

- `ActingPack` gains a `Calibration` block, derived per read and never
  stored. It holds, for this desk and its current model, the last 14
  days of count-governed questions:
  - per predicate: n, mean stated p, observed frequency, base rate, skill;
  - the count of interrupted and unresolvable outcomes;
  - the last three surprises (`|p - o| ≥ 0.6`), each with its record id,
    question, p and outcome.
- Judged-question outcomes (Jev, answerer) are not in the acting pack.
  The memo's other option, "sample and delay", can be tried later as an
  experiment: a weekly digest of judged outcomes. Until then they feed
  supervision only.
- The block is rendered as five lines or fewer in the turn prompt next to
  the dispatch instructions. Texture's turn prompt and management's turn
  prompt get it, in addition to `choir.Pack()`. Rendering it matters
  because a desk only sees the pack if it calls `choir.Pack()`
  (`internal/yaegikernel/choir.go:827`), and the prediction is authored at
  the moment of dispatch. Whether desks read it either way is a
  hypothesis; P4's acceptance measures it.

Example, as Texture would see it:

```
Your predictions (count-resolved, 14 days, model gpt-5.6-luna):
  new_sources>=k  n=9  said 0.71  happened 0.33  base 0.40  skill -0.9
  domain_match    n=5  said 0.78  happened 0.60  base 0.55  skill +0.2
Research on this document: 7 rounds; 3 added a new source; the last 2 added none.
Surprise: control:bd5f… "at least 1 new source" p=0.70 -> 0 (19:46).
```

Design §7 P4 says Texture should see research's calibration "when it
decides what to ask". Research seldom predicts. What Texture needs is
research's *yield* on this document, and that is the second line above,
derived from Texture's own count-resolved expectations. It is the number
that would have shortened S1h's six repeated rounds and the P2a demo's
last three.

**Supervision surfaces** carry everything: all score records, the
disagreements, the self-favor gap per desk and model, scorer accuracy per
question type, and Note-to-Cast ratios. These are the owner's Texture
evidence feed and management's spend inputs. That is where judged scores
belong.

### 4.7 Cost per record

| Step | Model calls | Tokens / cost |
|---|---|---|
| Authoring the expectation | 0 extra | about 80–150 output tokens per Cast or Ask. The P2a demo's eight dispatches would add about 1k tokens, against research's 100k+ input tokens per run (`orientation-jev-supervision-metamission-2026-09-29.md` §A) |
| Count resolution | 0 | indexed store reads |
| Answerer's typed answers | 0 extra | about 30 output tokens |
| Self-resolution | 0 extra (optional) | about 30 output tokens in a cell that runs anyway |
| Jev on judged questions | 1 per record | about 1–3k input tokens; about $0.00005–0.0002 by the M4 receipt's rate |
| Jev shadow on 25% of countable records | 0.25 per record | same |
| Management's model (off) | 1 desk turn | tens of thousands of input tokens; competes with real work on a rate-limited account |
| Panel audit | offline | at most 30 records a week |

At demo volume (hundreds of dispatches a day across staging), Jev costs
cents per day. The constraint that binds first is Jev's 60-per-minute
bucket once Autopaper's claims are scored. That needs a queue, batching,
and count-first resolution, which W2/W3 must plan for.

## 5. Failure modes and guards

The theory paper's H5 lists the manipulation modes a scoring system must
survive (`prospective-self-alignment-2026-10-09.md` §8). Each one maps to
a guard here:

| Failure mode | What it looks like | Guard |
|---|---|---|
| Trivial predictions | "research will answer, p=0.97" | liveness-only expectations refused (§3.1); skill against the base rate is about zero (§4.5) |
| Vague predictions | "research finds something useful" | judged questions need a criterion; unresolvable scores as wrong; specificity rate tracked per desk |
| Hedging at 0.5 | every p near 0.5 | Brier is strictly proper; skill is negative wherever the base rate is informative; sharpness reported |
| Choosing easy questions | only asking what is near certain | skill against base rate; per-predicate breakdown shows the question mix; residual: no materiality weighting yet |
| Avoiding predictions | moving dispatch to Notes | Notes cannot open or start work (§3.2); Note-to-Cast ratio tracked per desk; trace audit counts dispatches over time, because a desk that stops committing has not stopped failing (paper §8) |
| Self-serving self-scores | the asker says confirmed, ground truth says no | self never governs; the self-favor gap per desk and model is the memo's "most useful oversight number" |
| Self-serving answerer | research claims it found the primary source | checkable answers checked by count; answerer agreement with ground truth is its track record; answerer blind to p |
| Padding the count | research returns many weak sources to clear "≥3" | `new_sources` counts distinct URIs not already cited; `incorporated` and `cited_next_revision` tie the count to downstream use |
| Self-fulfilling predictions | Texture predicts its own next revision | `SelfControlled` stamped by the runtime; excluded from calibration and ground truth |
| Scorer gaming (in-context RL on the scorer) | desks learn to phrase questions Jev reads favorably | no desk calls a scorer; no model-scorer judgment of a desk's own predictions in its acting context; scorers rotated and compared (E1–E3) |
| Anchoring and collusion between desks | the answerer gives the asker what it expected | p and why stripped from the delivered packet (fixes `rlm_reduce.go:1730`); residual: thresholds in the question text hint at expectations, accepted |
| Bad ground truth | a resolver bug mislabels outcomes and poisons everything measured against it | closed vocabulary reviewed in code; every count record carries the evidence it counted, so anyone can recount; the panel audit sample includes a few count records |
| Misattributed answers | a report credited to the wrong round | runtime-stamped `RelatedIDs` from the activating control; window fallback; one-to-one join in the trace audit |
| Platform failures scored as forecasting errors | a crash or search timeout makes predictions "wrong" | crash-interrupted questions excluded. Residual: a search timeout is a real outcome for the asker but also a platform defect. Report it separately when the answer packet carries a runtime error note |
| Cost or rate blowup at Autopaper volume | thousands of claims per day | count first; Jev queued under its bucket; sampling for countable shadows |
| Refusal loops | desks fail cells repeatedly on the new argument | in-cell validation with an example; refusal rate is part of P3 acceptance |

## 6. Consistency with §10 (one schema across computers)

- Score records carry scorer identity (computer, desk, model, method), so
  "scores scoped by scorer" is a filter, not a schema change.
- Topic scoping stays derived from subject references at query time
  (design §9 item 7). Expectation questions add predicate and host, which
  are useful topic features. "Strong on chips" can be computed from
  `domain_match` hosts and subject entities.
- `count` is also a scorer identity scoped to a computer. Another
  edition's count over its own captures can differ from ours, and both are
  kept. Ground truth is "count on the computer that observed it", not a
  global fact.
- Scorer reputation is the same query applied to score records: how often
  did this scorer's answers match the answers later governed by count,
  panel or owner. This is the design's defense against a crowd of fake
  scorers, and P4 is its first use.
- Commit time: the expectation is inside the act record's content hash,
  so its commit time is the act's. Signing and commit-and-reveal stay
  later slices.

## 7. Station shape

**P3, predictions required** (depends on P2; P2d's addressing is needed
only for the management → `texture:<doc>` example):

- **P3a, schema and blind delivery.**
  - Add `CommitmentExpectation` to records and `Expectation` to
    `TextureTurnControl`.
  - Strip the distribution notes from `precommitPacketForRecord`. The
    delivered packet carries questions without p.
  - Stamp the emitting model id and the `Baseline`.
  - Add the target-id index; replace the 10,000-record scan.
  - Acceptance: unit (store, real Dolt). The packet test asserts that no p
    reaches the addressee.
- **P3b, mandatory at the verb and the reducer.**
  - Verb signatures and in-cell validation; reducer validation; the
    Texture fold carries the expectation; accumulator writes it.
  - Notes stop opening work.
  - Version-1 tray replay branch.
  - Prompt rewrites for every desk that casts or asks: Texture's prompts,
    management's and research's runtime overlays, with the worked example
    in `run_system.yaml`.
- **P3c, count resolver.**
  - Predicates `answered_by`, `new_sources`, `domain_match`,
    `incorporated`, `cited_next_revision`, `revisions_by` and
    `assignment_state`.
  - Answer linking through the activating control.
  - Interrupted handling; deterministic ids.
- **Deployed acceptance** (AI news demo with follow-up, fresh test
  computer):
  - every agent-emitted dispatch record after the deploy carries an
    expectation;
  - every expectation has at least one count score record, or an
    `interrupted` or judged-pending state, joined one to one;
  - in-cell expectation errors are counted and reported;
  - the fraction of countable questions is measured. This is the main
    number for the coverage hypothesis.

**P4, scoring visible:**

- **P4a, score records and answer sources.** The `score` kind, answerer
  typed answers on Reply and Report, self answers on Resolve, and the
  Jev scorer run by the harness on management's authority with the
  25% shadow.
- **P4b, calibration views and the pack.**
  - `ResolveExpectations`, `Calibration` (per desk and model).
  - The acting pack's count-only block and the rendered prompt lines.
  - The supervision view with everything else.
  - A static test that `ActingPack` carries no field derived from a
    non-count scorer, extending the M5 acceptance item.
- **P4c, the experiment's instruments.**
  - Scorer accuracy per question type against ground truth, and the
    self-favor gap.
  - The panel audit export: offline, a scratchpad JSON, not a ledger
    write. Panel labels enter the ledger as `panel` score records only if
    the owner approves (§8 question 6).
- **Deployed acceptance:**
  - demo plus an M11 rerun;
  - score records are separate objects;
  - Texture's prompt shows its calibration block with real numbers;
  - Jev receipts show cost per record;
  - a supervision read shows Jev versus answerer agreement on the demo's
    judged questions.

**Rollback:** git revert per slice. Records already written stay
(append-once). Reverting P3b without P3a leaves expectations optional and
still recorded.

## 8. Open questions for the owner

1. **Starting scorer.** Count, then Jev on management's authority (harness
   run, no management turn), with answerer and self recorded beside it,
   and management's own model off until the experiment asks for it.
   Acceptable as the default?
2. **Notes.** A Note may no longer open or start work, so mandatory
   predictions cannot be avoided by noting instead of casting. Texture's
   "stop sending status-only packets" Note would still be delivered, read
   at research's next wake. Agree?
3. **What desks see.** Count-resolved calibration only in the acting pack.
   Jev's and the answerer's judgments of a desk's own predictions stay in
   supervision. Is that the right reading of the 2026-09-29 rule "the
   acting pack never carries commitment scores"?
4. **Unresolvable costs as much as wrong.** It is harsh on purpose, to
   push desks toward countable questions. Too harsh?
5. **Base-rate scope.** Per desk per computer (private, slow to fill), or
   pooled across computers you own (faster, mixes contexts)? I recommend
   per computer now and pooling as a later experiment.
6. **Panel labels.** Should the panel's audit labels stay offline evidence
   for the decision rules, or enter the ledger as `panel` score records,
   which other editions could later see?
7. **`Why` field.** Optional now and promoted to required if P4's data
   shows surprises with reasons lead to better revisions. Or required
   from the start, as the memo's "predictions carry their reasons" would
   have it?
