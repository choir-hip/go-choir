# One protocol between agents: precommitment records

> **Revised 2026-10-12** ([re-engineering decisions](../../re-engineering-decisions-2026-10-12.md)): §4-5 are revised: a Cast carries hypotheses, open uncertainties and general hypotheses (not count questions with a deadline), and resolution or scoring is done by intelligence (a model, Jev or an RLM), not by runtime counting. Work becomes a fold over records, which absorbs P2d2.

Draft for owner review, 2026-10-11. Owner direction (2026-10-11): "lock
down the system so that all interagent comms are done through the
precommitment records protocol. That's the real design ... Gotta bring the
future forward." Problem record:
[problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md](../problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md).
Theory: [Prospective Self-Alignment](../../precommitment-records.md)
(working paper, October 9 revision; §2 the record and the loop, §4.1
procedural fidelity, §7 reputation, §10 implementation status). Mechanism:
[Precommitment Records — Engineering Memo](../../precommitment-records.md).
The paper's §10 lists the desk surface for commit, resolve and dispute as
implemented. The code supports it, but the staging traces in the problem
record show no desk calling `Precommit` or `Resolve`. This design makes it
the only path.
Prior station: S0m record-native messaging (complete 2026-10-01).

Mutation class: red (every desk's delivery path, Texture's control surface,
the evidence ledger, run acceptance through scoring).

## 1. The invariant

One agent affects another only through a commitment record addressed to
it. The record's derived packet is the only thing that wakes the
addressee. An act that asks for work carries a typed prediction, frozen
when the record commits, and the act that answers it resolves it.

The paper's loop holds: commit, authorize, act, observe, resolve, score,
revise. Committing a prediction does not authorize an act. Casts to
engineering and material actions still pass the existing authority gates
(management's admission, capsule boundaries, owner approval for protected
effects); the record makes the gate inspectable, not optional.

Two consequences:

- the ledger is the complete account of who asked whom for what, what they
  expected, and what happened;
- every hand-off is a scored prediction, so waste, surprise and
  calibration are measured, not read from traces by hand.

Not covered: the owner's own input (owner revisions, prompt-bar) and
runtime machinery (timers, deadlines, cancels, polls). These are not agent
speech. Section 6 says how they appear in the ledger.

## 2. The act set

Kept, all already record-native (S0m):

| Act | Asks for work? | Carries a prediction? | Resolves something? |
|---|---|---|---|
| Cast (open or continue work for a desk on a subject) | yes | required | no |
| Ask (a directed question) | yes | required | no |
| Report (findings, results; may carry a source packet) | no | no | the Cast or Ask it answers |
| Reply (the answer to an Ask) | no | no | the Ask |
| Note (unscored information) | no | no | no |
| Escalate (to management or the owner) | yes | required | no |
| Precommit (a standalone prediction, e.g. story size) | no | is one | no |
| Resolve, Disagreement, Retract | no | no | the target record |

Removed from the desk surface:

- `choir.Message`, which has three carriers today, one of them the retired
  envelope;
- `choir.Emit`, an envelope write outside the cell tray. A record's packet
  already wakes the addressee, so Emit's latency use is covered;
- the `ApplyTexture` controls `open_researcher`, `open_persistent_super`
  and `target_work_item_id`. `ApplyTexture` keeps document edits and
  decide turns only, which S0m already intended. Texture uses Cast and Ask
  like every desk;
- `choir.Assign`. Its open decision is in section 9.

Management's `ProductAPI` loses every route that writes a Texture's
content (`/revisions`, `/revise`, `/restore`, `/accept-merge`), so only
the Texture desk writes a Texture.

## 3. Addressing

An address is a desk profile plus a subject: `research:<doc>`,
`management:<owner>`, `engineering:<assignment>`, `texture:<doc>`.

- A Cast to a desk with no open work on that subject opens it. This
  replaces `open_researcher` and `open_persistent_super`.
- A Cast or Ask to a desk with open work continues it. This replaces
  `target_work_item_id`.
- Any desk may address `texture:<doc>` for any document of the same owner.
  The Texture desk decides what to do with the delivery. This is how
  management feeds a story Texture (Autopaper W2), and it replaces the
  rule that binds a report to one delivered control.

## 4. The prediction

Small, typed and checkable. The memo's two hard rules hold: frozen before
the evidence, and resolved against a verifiable outcome.

```go
type Expectation struct {
    Questions []TypedQuestion // 1 to 4; each binary, count, date or choice, with a probability or distribution
    By        time.Time       // when it should resolve; open past By is itself an outcome
    Subject   string          // the doc, story, assignment or source it concerns
    Resolver  string          // "count", "answer" (the reply resolves it), or "scorer"
}
```

Examples from today's traces:

- Texture → research: "Will this round find a primary record for the Haiku
  5.5 announcement? p=0.8", "How many new sources? ≥3, p=0.6".
- Management → `texture:<doc>` (W2): "Will the Texture cite at least one of
  these captures in its next revision? p=0.7".
- Management Precommit on a story: "Captures on this story within 24h? ≥10,
  p=0.5".

Specificity is scored per record (the memo's guard against predictions
that score well by saying little).

## 5. Resolution and scoring

- **By count, no model call:** questions about counts, dates or presence
  that the runtime can observe (sources cited, captures matched, revisions
  made, a record answered by its deadline). A small runtime resolver runs
  on the answering act, or at `By`.
- **By the answer:** a Reply or Report names the record it resolves. The
  harness scores the answer against the frozen questions. The scorer is
  config: the agent's own model by default, Jev or a second model later.
  Agents cannot query the scorer.
- **Discrepancy class** on every resolution: confirmed, qualified,
  contradicted, answered or unresolved (the classes already exist in
  `internal/types/commitment.go`).
- A lesson (the memo's revision) is written only on a scored surprise.

What accumulates: calibration per desk, model and subject; per-source
track records; per-story spend.

## 6. Runtime events in the ledger

Runtime wakes stay what they are, but each one that delivers information
to a desk becomes an observation record with system provenance, addressed
like any record:

- new captures after a poll → an observation addressed to
  `management:<owner>`. This replaces the W2 draft's watermark delivery.
  Debounce and coverage live in the record (the capture ids it covers),
  not in a side channel;
- a lifecycle cancellation or deadline → an observation on the record it
  closes.

Timers, the crash-restart fence and owner revisions are unchanged.

## 7. Cutover, one landed slice at a time

Each slice lands on staging with a trace audit. Messages already in
flight drain on the old kinds (consumed, never re-armed). Nothing migrates.

1. **P1, record-first controls.** The control accumulator (shared by
   `ApplyTextureTurn` and `IssueLifecycleControl`) mints a Cast record per
   control, since a control opens or continues work. The work item and
   packet derive from it (the RN3 pattern). The owner's management-open
   control records the owner as its emitter. The desk surface is unchanged. After P1 the trace
   audit shows every dispatch on the ledger.
2. **P2, one surface.** Texture's controls are removed from `ApplyTexture`,
   and Texture uses Cast and Ask. `Message`, `Emit` and the Texture-write
   `ProductAPI` routes are deleted. `texture:<doc>` addressing opens to
   any same-owner desk.
3. **P3, predictions required.** The reducer refuses a Cast, Ask or
   Escalate without an Expectation. The count resolver lands.
4. **P4, scoring visible.** The harness scores answers, and each desk's
   pack shows its calibration and recent surprises. Texture sees the
   research desk's calibration when it decides what to ask.
5. **P5, Autopaper W2 re-based.** Poll observations go to management,
   management casts to `texture:<doc>` and precommits story size,
   resolved by count.

## 8. Acceptance

- A trace audit over a demo and an M11 rerun shows zero off-ledger
  dispatches.
- The AI news demo runs end to end with predictions on every research
  round, and its record shows how many rounds confirmed what was
  predicted.
- A crash restart closes open records with the visible fate "interrupted
  by a restart" and resumes nothing (CLAUDE.md restart rule).
- Management cannot write a Texture's content through any route.

## 9. Open decisions for the owner

1. **Is a prediction mandatory on every Ask, or sampled?** Mandatory is the
   clean invariant; sampled is cheaper. Recommendation: mandatory, with a
   one-question minimum.
2. **The default scorer.** Recommendation: the answering desk's own model
   now, Jev as an optimization once the record volume justifies it.
3. **`choir.Assign`.** Fold it into Cast to engineering, or keep it as
   management's runtime-owned assignment receipt? Recommendation: fold it.
4. **Do owner revisions become records?** Recommendation: not now. They
   are the owner's act, not agent speech, and they already have a
   canonical path.
5. **Where Texture's ProductAPI denial lives:** in the allowlist by caller
   profile, or in the Texture routes by authenticated writer.
   Recommendation: the routes, so no future allowlist entry reopens it.
6. **Do existing in-record scores migrate?** Recommendation: no. Leave old
   records as they are (no desk ever wrote one, per the problem record);
   new scores are separate records from P4.
7. **Topic scoping:** derived at query time from subject references, or
   declared by the emitter? Recommendation: derived, with an optional
   emitter tag that is itself just a claim.
8. **When signing starts:** with per-computer identity keys, or earlier
   with a host-held key per computer? Recommendation: wait for real
   per-computer keys. Until then, a store-assigned commit time and content
   hash are the integrity floor, and records are marked unsigned.

### Owner answers (2026-10-11, evening)

- Item 1: predictions are mandatory.
- Item 2: the scorer is not settled; it is to be learned from iteration
  (self, the answering desk, or management, possibly with Jev). Design in
  docs/precommitment-records.md.
- Item 3: open; the owner asks whether Assign is a needless subsystem
  that can be simplified without changing its essence (review in
  docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md).
- Item 8: the owner trusts the recommendation and asks what blocks real
  per-computer keys (docs/precommitment-records.md).
- Addressing: management sends records and scores to a Texture desk and
  never modifies a Texture document. Engineering does not address a
  Texture; it goes through management, and management (with Jev) decides
  whether to pass a report through or restate it, to protect the Texture
  and the owner from overly technical content (review in the same Assign
  and routing doc).

## 10. One schema for desks, Autopaper and the economy of agents

Owner, 2026-10-11: "we aim to use the same precommitment records between
desks and in autopaper. The claims that we extract from news and the
inferences drawn from them are precommitments we score over time ...
others [make] their own editions (with their own perspectives and private
data) [and] can also do their own scoring so it's not our hegemonic house
view only. This is the basis of a track record based reputation system ...
we can scope scores by scorer and topic."

So the desk protocol is the first use of a record that will travel
between computers and be scored by people who do not share our view.
Most of that comes later. A few properties must hold from P1, because
records written now become track record later and cannot be re-keyed.

**Three kinds of emitter, one record.**

- A desk's act (Cast, Ask, Report): emitter is the agent on a computer.
- An edition's claim or inference: emitter is the computer (the edition).
  A Texture's statement "X implies Y by March, p=0.6" is a precommitment.
- A source's claim ("Reuters reports X"): the emitter is the outlet, which
  never precommitted. The record is an observation attributed to it.
  Later reports confirming or contradicting it build the source's track
  record (design §2 already promises track records for sources).

**Scores are separate records, never fields on the claim.** A record is
immutable once committed. A score is its own record that references the
record it scores, with the scorer's identity (computer or edition, desk,
model, method), the answers, and the evidence it used. Any number of
scorers can score one record, and their disagreement is kept. Today
`CommitmentRecord.Scores` sits inside the record and names only a model
(`internal/types/commitment.go`). P4 writes scores as separate objects
from the first score.

**Resolution evidence is shared; judgment is not.** What later happened
(a report, a filing, a revision) is evidence anyone can cite. How it bears
on a claim is each scorer's call. This keeps design principle 1: show
what the world indicates, do not adjudicate.

**Reputation is a query, not a number.** A track record is the scores on
an emitter's records, filtered by scorer, topic and time. No global score
exists, and readers choose whose scores to trust. A scorer is scored the
same way: do its scores anticipate later resolutions? That is the defense
against a crowd of fake scorers.

**Topic is derived, not a taxonomy** (design principle 2). A record
carries subject references: the story's Texture, sources, entities and
captures. Topic scopes ("strong on chips, weak on courts") are computed
at query time from those references, by clustering or tags, and can
change without rewriting records.

**Commit time must be provable.** A track record is worth something only
if the prediction demonstrably came before the evidence. Published
records go to the global store (Store B) with a commit time the store
assigns and a content hash. Each record is signed by its computer's key
once per-computer identity keys exist (fork re-keying, CLAUDE.md). A
private edition commits only a hash and a time publicly and reveals the
content later (commit and reveal), so private data stays private while
the track record is real.

**Where records live.** Desk-to-desk records stay in the computer's own
store by default; they are working state. An edition's published claims,
inferences and scores go to the global store, where other computers read
and score them. Overlays decide which scorers a reader trusts. This is the
storage rule Autopaper already follows (shared content global, overlays
per computer).

**What P1 to P4 must reserve now:**

- emitter identity on every record: computer, agent or desk, and model,
  plus an emitter kind (agent, edition, source);
- subject references on every record;
- a content hash and a store-assigned commit time;
- scores as separate records with scorer identity, from the first score
  in P4.

Signing, commit and reveal, publishing records to the global store, and
reputation queries are later slices. They start with Autopaper's W2 and
W3, once editions publish.

## 11. Risks

- **Cost.** A prediction adds tokens to every dispatch, and scoring adds
  calls. Mitigations: resolve by count first, and score with a model only
  when the answer is not countable.
- **Gaming.** Predictions that say little score well; desks that learn the
  scorer write to it. Mitigations: a specificity score, no scorer access,
  rotating second opinions (memo).
- **Cutover.** Every desk's delivery path changes. Mitigations: P1 changes
  no surface, and each later slice is one landed change with its own
  rollback.
- **Over-structure (design principle 2).** The protocol fixes how agents
  talk and what they owe each other, not what they say. Stories, claims
  and stages stay free.
