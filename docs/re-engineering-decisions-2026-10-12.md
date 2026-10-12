# Re-engineering decisions, 2026-10-12

Status: owner-agreed design direction, written after an iterative review
of the master report
([reports/master-report-protocol-research-autopaper-2026-10-11.md](reports/master-report-protocol-research-autopaper-2026-10-11.md))
and six read-only studies
([reports/re-engineering-studies-2026-10-12/](reports/re-engineering-studies-2026-10-12/README.md)).
The owner asked for agreement before any code change ("extremely high
stakes decision making"). This record holds what was agreed. Where an
earlier design or a study proposes something different, this record wins
until a later owner decision replaces it.

Mission framing: once these decisions are carried into stations, an
autonomous run crosses Gates 1-3 (ACTIVE.md roadmap) and makes Autopaper
live. Deferred missions come after.

## 1. Standing principles

1. **Affordance invariant.** Every desk's prompts and tool descriptions
   describe its actual affordances: no stale tool names, no instructions
   for things that do not exist, nothing real left undescribed. The
   studies found this broken on every desk (study G §1, study D §2, study
   C §1). Sweeping it is the first station.
2. **Intelligence lives in models, not in the runtime.** No budgets, no
   round caps, no runtime-owned research job, no runtime scoring. Agents
   keep going until new information stops paying for itself, and they
   report progress through records as they go. The runtime keeps only
   substrate: delivery, idempotency, authority boundaries, schema
   well-formedness, crash safety, quotas and infrastructure limits.
3. **Prefer deletion.** Nearly every fix the studies found is a deletion:
   stale prompts, stubs, caps, budgets, dead code. Delete before adding.
4. **Circuit breakers when needed, not before.** No preemptive guards
   against pathological loops. Add one when a real loop is observed.
5. **The 60-minute run limit is a prototyping safety constraint**, not
   design. Demo and eval runs should finish well inside it once the desks
   work. Real projects and the continuously running Autopaper may take far
   longer and may span many runs; how that splits is not a design concern
   now. Do not build behaviour on top of the limit.
6. **Restart rule unchanged** (CLAUDE.md): a crash closes work, a planned
   restart may resume it. The code breaks it today in both directions
   ([problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md](problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md)).
7. **Language.** Provider failure is normal: say "M of N search providers
   active". "Degraded" means users get worse searchability or results.

## 2. Work is records

The work-item abstraction (mutable rows with a status column, doing six
unrelated jobs, study B §1) is replaced by precommitment records.

- A Cast opens an obligation. Reports in its thread are progress. A final
  Report, a Retract, or a system fate record closes it.
- Status is a fold over append-only records, not a mutable column: open
  work is a Cast thread with no closing record; the work list is a query;
  authority to act is an open Cast (or an unconsumed record) addressed to
  me.
- A crash boot appends "interrupted by a restart" to every open thread.
  A planned-restart marker skips this.
- Texture's obligation is its inbox: every record addressed to
  `texture:<doc>`. This replaces the rule that one Texture run binds one
  work item, so **P2d2 becomes a stage of this migration**. The directive
  inbox design (shape D) is not built.
- Records need: a final flag on Reports, a system fate record, a
  store-stamped thread id, the subject on desk-authored addresses, and
  indexed columns for the fold (study B §2-3).

## 3. Casts carry hypotheses and uncertainties

A request for work is not "find X; I expect a primary source, p=0.8". It
states what the caster believes and where it is unsure, which leaves room
for learning and tells the worker how hard to work.

- **Hypotheses**, each with a probability, how much it matters, and what
  would change the caster's mind.
- **Open uncertainties** with no hypothesis yet, weighted by how much they
  matter.
- **General hypotheses** (§3.1), such as "there are important things I
  haven't mentioned", each with its own probability.
- Reports tag findings with the hypothesis they bear on, or mark them as
  outside every listed hypothesis. Belief updates are records by the
  caster or a third party, citing the Reports.
- Effort follows uncertainty times stakes. Diminishing returns are
  measurable: further work stops moving any hypothesis that matters.
- The record type already has hypothesis, assumptions, alternatives and
  weighted-question fields (`internal/types/commitment.go`); Casts fill
  none of them today. The count-questions-plus-deadline Expectation in the
  protocol design §4 is replaced by this shape.

### 3.1 General hypotheses: a space to sample from

General hypotheses are open-ended but falsifiable claims that apply to
almost any inquiry. They are crystallized intelligence offered to the
desks, **not rules**: a prompt carries a diverse list as a space to sample
from, and a caster picks the few that fit. They complement the cognitive
transforms in `skills/cognitive-transform-portfolio` (transforms are moves
that change the next action; general hypotheses are claims about the
world that the next action can confirm or refute).

Each needs an operational meaning of its key word before it can resolve.
For "important": a finding that changes the caster's lede or structure, or
moves a stated probability by at least a set amount, judged by the caster
or a third party, never by the reporter. Scoped versions ("important
things about who funded this") are sharper and still open-ended.

A first seed list, to be grown and pruned by use:

- **Omission:** there are important things I haven't mentioned.
- **Framing:** the question is mis-framed; the real story is adjacent to it.
- **Recency:** something has changed since my newest source.
- **Primary record:** a primary record exists that would settle this
  (filing, transcript, changelog, dataset, court docket).
- **Counter-evidence:** credible disconfirming evidence exists that I
  haven't seen.
- **Common origin:** sources that look independent trace back to one
  origin.
- **Incentive:** a party with a stake shaped the account I have.
- **Silent party:** someone materially affected hasn't been heard from.
- **Number error:** a key figure is off by a unit, a denominator or an
  order of magnitude.
- **Definition drift:** a key term means different things in different
  sources.
- **Mechanism:** the stated cause is a correlate; the mechanism is
  different.
- **Scope:** the claim holds narrowly but is reported broadly, or the
  reverse.
- **Second order:** the most important consequence is downstream of the
  headline.
- **Base rate:** this is less unusual than it looks.
- **Prior coverage:** this was reported before, earlier or under another
  name.
- **Other languages:** sources in another language tell a different story.
- **Correction:** a source I rely on has been corrected or retracted.
- **Timeline:** the dates in my sources don't line up.
- **Expert split:** experts disagree more than coverage suggests.
- **Hype:** the claim is mostly marketing; measured performance is weaker.
- **Non-event:** nothing material happened.

For engineering and management (examples): the failure is in a substrate,
not the symptom; a replacement exists but is not wired in; the test
passes for the wrong reason; the prompt describes an affordance that
doesn't exist; the trace contradicts the behaviour I'm attributing.

Over many Casts, how often a general hypothesis comes true against its
stated probability measures how well a desk knows what it doesn't know.

## 4. Research desk

Research is a simple RLM (root and sub-RLMs) that searches, reads,
reports and reduces uncertainty, driven to get to the bottom of things.
It owns its work; the runtime does not.

- **Verbs:** async Search, Read (fetch into a capture), Report, Research
  (spawn a sub-RLM), Ask, Reply, Note. Nothing retired is listed.
- **Report as soon as understanding changes.** No forced status reports.
  A cell may report and search again in the same cell.
- **Claims carry epistemic status** (confirmed, reported, inferred,
  speculative, refuted). Calibrated speculation and rumour are welcome
  when framed as such. No claim quotas.
- **Research ends its own work** with a final Report that says why it
  stopped. It stops at diminishing returns *relative to the request*:
  new information is always available, so the goal that bounds the work
  comes from the Cast's hypotheses and stakes.
- **No reminders.** Research knows its thread is open; Texture can always
  ask a follow-up, before or after research closes. Both are intelligent.
- **Delete:** the 4-round cap, the per-activation call cap, the forced
  status report, the `update_coagent` leftovers in search results, the
  override that keeps research reports open, retired verbs, and
  sub-researchers inheriting the parent's prompt (study C).
- **Sweep:** `update_coagent` and every other dead name across prompts,
  tools, code and tests.

## 5. Async search

- `choir.SearchStart(q)` returns a stub at once ("s3 started: 4
  providers"). Each provider's results land in a persistent, lock-guarded
  store in REPL state as they arrive (`choir.Searches.Get(id)`), and the
  RLM decides when and how to read them: with code, filters, sub-RLMs.
- The only push is one count line on each cell result ("s3: 3/4 answered,
  +2 new"). `choir.SearchWait(id, ms)` lets the model block briefly if it
  chooses.
- The gateway gains a streaming mode that starts every eligible provider
  at once and sends each provider's results as they arrive (the router
  already completes per provider; the guest link already streams for
  inference). The existing search call stays.
- Concurrency lives on the host. Model-authored goroutines must not call
  `choir.*` network verbs until the transport correlates by request id
  ([problems/parallel-choir-calls-cross-wire-results-2026-10-12.md](problems/parallel-choir-calls-cross-wire-results-2026-10-12.md)).
- This replaces the deadline-ladder station; the 30-second tie it
  documented disappears when nothing waits on the slowest provider.

## 6. Texture

Re-engineer Texture before any model comparison.

- Turn time is dominated by one large write per turn (76% of model time in
  the measured demo), not by cell count (about 5 per turn; the "~60"
  figure was a misreading of a 12-turn total). **Write less**: changed
  blocks only.
- Each turn's message carries everything needed: the current document as
  short-alias blocks and the new records with their sources and quotes.
- One write verb with block operations and inline citations; a full
  rewrite keeps unchanged blocks' ids and citations. `Wait`, `Done` and
  `Reject` replace the decide operation.
- Prompt core about 3.5k characters, aligned with the document model
  (§7). Fix the 26 mismatches study D lists by deleting, not annotating.
- Texture integrates; it does not translate engineering. Owner-facing
  Textures hide programming jargon; diffs sit behind a clickable
  transclusion citation.
- Model comparison (deepseek-v4.1-flash, gpt-6-Luna, glm-5.3-flash,
  muse-spark-contributor, other opencode-go models) and the
  reasoning-effort test come after the re-engineering.

## 7. One document model

- Captured content (web pages, PDFs, feeds) lives in the **same block
  model** as Texture documents: one model, two kinds of author.
- Content-addressed block ids (`cid`, a hash of kind and normalized text)
  for citing; Texture blocks keep an editing id and gain a `cid`.
- One reference shape for citations and transclusions:
  `{doc, version, cid, hash, quote}`. A story transcludes source blocks
  the way one Texture transcludes another.
- Captures keep their raw bytes and a structured block list; a changed
  re-capture is kept, not dropped as a duplicate.
- **Citation checking is iterative and memoized.** Users see the first
  version at once. Quote matching (mike's cascade: exact, normalized,
  punctuation-tolerant) is deterministic code; whether a source supports a
  claim is a model judgement, run in the background by a desk other than
  the author. Results are keyed by the citing block's and the cited
  block's content hashes, so an unchanged pair is never re-checked.
  Passing checks attach to the existing version; failures go to Texture,
  which writes the next version. Texture stays the sole writer.
- Copyright is set aside for now; functional utility first.
- Order: B (quote verification) → A (content-addressed capture blocks)
  → C (transclusion); D (alignment across re-captures) deferred.

## 8. Publishing

Each published version of a story has its own immutable permalink
(`/v<n>`; v7's link always shows v7). Chevrons move between versions; a
"new version available" control jumps to the latest; the bare story link
shows the latest in place. Versions are numbered by publish (v1, v2, v3).
A version's transcluded source blocks are pinned to it. Supersedes the
one-permalink shape in the W3a draft and the app design §4.6.

## 9. Routing, scoring, keys

- Management sits between Texture and engineering. Engineering stays
  technical and never addresses Texture. **Jev is part of management**: it
  passes simple reports straight through, or management reasons and
  writes the report to Texture. Jev never holds (no hold exists in code
  today; keep it that way).
- Path B (an owner edit to an engineering-bound document goes straight to
  engineering, and engineering reports straight back) dissolves into
  owner → Texture → management → engineering, deleting the
  crash-resuming restart recast with it.
- **Assign is deleted** (an in-memory stub that persists nothing).
- **Self-development recedes as a code-level concept**: it is integral and
  diffuse. The guest and host update machinery stays under plain names.
- **Scoring needs intelligence**: Jev, a model, or an RLM, often from
  several perspectives. The runtime does not score.
- **Signing keys** per computer, per desk, perhaps per sub-RLM: deferred
  as a non-functional requirement.

## 10. Order of work

1. Truthful prompts and deletions on all four desks (affordance sweep,
   dead names, stubs, caps, budgets, Assign). Orange.
2. Restart rule in both directions. Red.
3. Records as work, with P2d2 as the Texture inbox stage. Red.
4. Async search and the research redesign with hypothesis-shaped Casts.
   Orange and red.
5. Texture re-engineering, then the model comparison. Red.
6. One document model and per-version permalinks, then the Autopaper
   reader and living stories. Red.

## 11. Superseded or revised

- `definitions/choir-research-job-2026-10-11.md`: deleted (runtime-owned
  job and budgets rejected).
- `definitions/choir-search-deadline-ladder-2026-10-11.md`: deleted
  (async search replaces it).
- `definitions/choir-autopaper-w3a-reader-2026-10-11.md`: revised to
  per-version permalinks.
- `precommitment-protocol-design-2026-10-11.md` §4-5: the Expectation
  shape and runtime count-resolution are replaced by §3 here.
- `precommitment-predictions-and-scoring-design-2026-10-11.md`: runtime
  scoring withdrawn.
- `protocol-review-assign-and-engineering-routing-2026-10-11.md`: Jev
  has no hold; Assign deleted.
- `texture-directive-inbox-design-2026-10-11.md`: shape D not built.
