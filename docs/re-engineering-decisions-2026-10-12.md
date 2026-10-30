# Re-engineering decisions, 2026-10-12

Status: owner-agreed design direction, written after an iterative review
of the master report
([reports/master-report-protocol-research-autopaper-2026-10-11.md](archive/reports/master-report-protocol-research-autopaper-2026-10-11.md))
and six read-only studies
([reports/re-engineering-studies-2026-10-12/](archive/reports/re-engineering-studies-2026-10-12/README.md)).
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
   ([problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md](problems/crash-boots-redrive-applies-2026-10-13.md)).
7. **Language.** Provider failure is normal: say "M of N search providers
   active". "Degraded" means users get worse searchability or results.

## 2. Work is records: conversations, rounds, hypotheses, cursors

Revised the same day after the divergent and lateral panels (§12). The
work-item abstraction (mutable rows with a status column, doing six
unrelated jobs, study B §1) and the delivery packet (a second mutable
object per record) are replaced by append-only precommitment records and
one cursor per inbox.

- **Conversation.** Communication is single-threaded from the sender's
  side: one conversation per (sender desk, recipient desk, subject), for
  example Texture → research about document X. Every Cast the sender
  sends adds to that one conversation (new hypotheses, revised
  probabilities, steering, withdrawals). The sender never manages thread
  ids. Parallelism is the recipient's responsibility: research spreads
  the work over sub-RLMs; management and engineering spread engineering
  work over capsules. This is the RLM pattern: each desk is an RLM, each
  RLM may have sub-RLMs. Depth stays shallow for now and scales toward
  large swarms as models are post-trained on the pattern; that scale is
  what makes Autopaper work on global current events.
- **The unit of progress is the hypothesis** (idea-level supervision, not
  action-level supervision). Hypotheses have stable ids within a
  conversation (H1, H2, ...). A later Cast may add H4, revise H2's
  probability or withdraw H3. Each hypothesis is open, settled (with an
  outcome) or dropped (with a reason). Reports name the hypotheses they
  bear on; scoring and calibration are per hypothesis.
- **Rounds.** A conversation has rounds of work. The recipient closes the
  open round with a final Report that says what it settled, what it
  dropped and why it stopped. The next Cast after a close opens a new
  round in the same conversation. Records are append-only, so a living
  story's conversation can run through many rounds.
- **Status is a fold** over the records, never a mutable column: an open
  round is one with no closing record; the work list is a query;
  authority to act is an open round or an unread record addressed to me.
- **Closing is not reading.** A final Report closes the recipient's work;
  the sender may not have read it yet, and that evidence stays waiting
  for it. An Ask and its Reply inside a round do not close the round.
- **A crash closes the open round** ("interrupted by a restart", a system
  record with a deterministic id). Its hypotheses stay on record, so the
  next round picks them up deliberately, never by resuming. A
  planned-restart marker skips this.
- **Delivery by cursor, as in Kafka.** Every record addressed to an inbox
  (for example `texture:<doc>`) gets the next sequence number in that
  inbox. Each reader keeps one cursor: "I have read through #57". A desk
  wakes when records exist past its cursor. The cursor advances in the
  same transaction as the work that read the records (for Texture, the
  revision commit), so a record is never read twice or skipped. Packets
  and their delivery status machine are deleted; after a crash nothing is
  stranded, and unread records wait past the cursor for the next turn.
- **Sub-RLMs are the recipient's own business.** Their work reaches the
  sender only through the recipient's Reports.
- **Texture's inbox** is every record addressed to `texture:<doc>` past
  its cursor. This replaces the rule that one Texture run binds one work
  item, so **P2d2 is part of this cutover**; the directive inbox design
  (shape D) is not built.
- Records need: hypothesis ids and their states, a final flag on Reports,
  a system record for crash closure, the subject on desk-authored
  addresses, per-inbox sequence numbers, and indexed columns for the fold.

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
- Reports tag findings with the hypothesis ids they bear on, or mark
  them as outside every listed hypothesis. Belief updates are records by the
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
  inference). Desks lose the blocking search call (§12).
- Concurrency lives on the host. Model-authored goroutines must not call
  `choir.*` network verbs until the transport correlates by request id
  ([problems/parallel-choir-calls-cross-wire-results-2026-10-12.md](archive/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md)).
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
*Revised in §13.2: an article version is a Texture version, so version
numbers are Texture revision numbers.*
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

## 10. Order of work: three cuts

Revised after the panels (§12): a radical cutover, not six gradual
stations. No shadow folds, dual-writes, interim closers, standing anchors,
one-release keeps or feature flags.

0. **Cut 0, eyes** (small): trace export through the product API (no SSH),
   an end-to-end tracer and invariant checker, minting fresh test
   computers, the request-id fix for parallel `choir.*` calls, one REPL
   owner per desk, and the register of what survives the cut.
1. **Cut 1, authority**: records as the only work state (conversations,
   rounds, hypothesis ids, cursors; work items and packets deleted), crash
   closure as records, Texture's inbox (P2d2), engineering only through
   management (path B, Assign and the restart recast deleted),
   sub-researchers on records, and all four desk prompts written once for
   this system.
2. **Cut 2, evidence to publication**: async search and the research desk,
   Texture's write verb, captures in the block model with quote checks,
   per-version permalinks, living stories (W2) and the reader: Autopaper
   live. Then the model comparison.

## 11. Superseded or revised

- `definitions/choir-reeng-r1` through `r6` (drafted at 03499ef9): deleted;
  superseded by the three cuts (§10, §12). Their code-cited findings are
  carried into the cut files.
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

## 12. Cutover decisions (after the divergent and lateral panels)

Owner, 2026-10-12: "a more radical cutover, less gradual. We don't have
any backwards compatibility needs." Two 10-model panels (divergent with
ten lenses, lateral) reviewed the whole metamission; 8 of 10 answered in
each (raw outputs in `.agentic-consensus/reeng-{divergent,lateral}-2026-10-12/`,
session diagnostics, not evidence). All 16 answers agreed that the staged
drafts protected old state the owner does not need. The owner settled
conversations and cursors (§2) and delegated the rest to the agent's
judgement ("most of this stuff doesn't move me either way / I trust your
judgement"). Decided:

1. **What survives the cut.** Published versions, and the owner's
   computer and Textures, untouched until the cut is proven. Staging test
   computers are disposable. Old execution state is archived as history,
   never migrated. Before the first public Autopaper permalink, the record
   schema, block ids and reference shape may still change freely; at that
   permalink they freeze. Compatibility begins at the permalink.
   *Revised in §13.1: nothing survives, including the owner's computer
   and Textures; only the freeze at the first Autopaper permalink stands.*
2. **The cutover deploy counts as a crash.** Everything open closes with
   a visible fate; nothing resumes.
3. **Rollback is forward-fix** after Cut 1, with a tagged last-good commit
   and a state snapshot. A git revert alone cannot read new-format records.
   *Revised in §13.1: no state snapshot; the tag stays.*
4. **One breakage log per cut.** During a cut window, each newly found
   problem is appended to `docs/problems/reeng-cut<N>-breakage-<date>.md`
   in the same commit as its fix, and the log is reviewed once per cut.
   This is an explicit, time-boxed exception to "documenting a problem
   comes first" (CLAUDE.md) for the cut windows only; a problem that is
   not fixed in the same commit gets its own problem doc as usual.
5. **Redesign tripwire.** If the same layer fails three tracer runs, or a
   fix needs new mutable state that acts as an authority, stop patching:
   write a structural assessment and redesign that layer.
6. **Desks lose the blocking search call**; async search is the only
   search surface for desks.
7. **Support judgement** (does a source support a claim) is done by
   management or Jev, never by research, which found the evidence, and
   never by Texture, which wrote the claim.
8. **Living documents don't settle; requests finish.** Document
   settlement stops being an execution authority.
9. **After a crash, Autopaper restarts by a deliberate management act**,
   never by a boot-time resumer. *Revised in §13.3: no restart policy
   yet; the owner restarts it by hand.*
10. **One REPL owner per desk.** The session manager today starts a second
    REPL when a desk's session is busy with another run's cell
    (`internal/agentcore/tools_desk.go:118-131`). A persistent desk has one
    execution owner; parallel intelligence is sub-RLMs, parallel I/O is
    the host.
11. **Sub-researchers move onto records in Cut 1.** Today `choir.Research`
    waits for all children, gives each a 10-minute window and 75% of the
    remaining time, and discards their events
    (`internal/agentcore/research_children.go:44-46, 167, 210`).
12. **Boot-time actors are deleted, not gated:** the engineering restart
    recast, spawned-work replay, terminal settlement repair, the Texture
    wake re-arm on open work, and wire-publication claim recovery, which
    cancels trajectories at every boot (`internal/agentcore/wire_publication.go`).
13. **Prompts are written once, for the system after the cut**, with each
    desk's verb list generated from its real module set and only role and
    judgement written by hand. R1's truth pass on today's system is
    dropped, and so is the baseline demo of the system being deleted.
14. **Rejected panel suggestions** (they reintroduce caps or reminders): a
    maximum of three hypotheses per Cast, an automatic "no new findings"
    notice after two quiet turns, and a hash-cycle detector on Casts.

Open, measured in Cut 0: how long a push takes to reach a healthy fresh
test computer (it bounds the pace of every cut), and whether the one-hour
lifetime of the broker handle breaks long-lived desk sessions.

## 13. Owner rulings and the convergent review (2026-10-12, later)

**Owner rulings**, after reading the cut drafts:

1. **Nothing is preserved before Autopaper goes live.** The owner's
   computer is not held at a build; it takes each cut like a test
   computer. "I know we're taking a risk, but I want to take that risk. I
   like the forcing function." No Texture export: "none of the data is
   valuable until we actually go live with AutoPaper." Pre-cut
   publications need not keep resolving. Compatibility begins at the
   first public Autopaper permalink (§12.1's freeze stands). The state
   snapshot in §12.3 goes; the tagged last-good commit stays.
2. **An article is a Texture, and an article version is a Texture
   version.** Published version numbers are Texture revision numbers, not
   a second count. Stories work in public from their first revision; a
   story is publication-ready nearer v5 to v10 than v2.
3. **No restart policy yet.** After a crash Autopaper stays down until
   the owner restarts it by hand. A restart policy is a later station.
4. **Production follows the ingestion rhythm.** Six hours of polling
   should yield many stories, not three: stories are opened and revised
   at roughly the cadence captures arrive. The acceptance observes this
   per poll interval; no quota is added (§1.2). A poll brings many
   captures at once, so one interval can open several stories. The point
   is to cover the consequential stories as they emerge and evolve. Early
   on, with little corpus, most captures open new stories; over time the
   share of new stories falls as more captures revise and cite existing
   ones. That falling ratio is expected, not a defect.

**Convergent panel** on the metamission and the three cuts, frozen at
2d2b864e. Six of eight answered (codex, claude opus, gpt-6.1-sol,
gemini-3.8-flash, deepseek-v4.1-flash, glm-5.3-flash; gpt-6-luna and
muse-spark timed out). All accepted the metamission. Codex sent back all
three cuts; gpt-6.1-sol sent back all three; gemini and deepseek sent
back one or two; claude and glm accepted with amendments. Raw outputs are
session diagnostics in `.agentic-consensus/reeng-cuts-convergent-2026-10-12/`.
Adopted, each checked against the code:

- **Cut 0** is red for the trace export (Trace/evidence) and carries the
  red ceremony. The export marks each layer present, empty or unserved
  and is owner-scoped. The tracer drives parallel tagged calls and
  overlapping cells on one desk. Cut 0 owns the broker-handle fix if the
  70-minute session confirms it. Cut 0 records the run-acceptance and
  per-turn baselines Cuts 1 and 2 compare against. Tool calls in one turn
  do run in parallel (toolregistry/batch_executor.go:52-60), correcting
  the draft.
- A per-computer maintenance hold exists in vmctl
  (internal/vmctl/ownership.go:181-205; the deploy skips held computers,
  ci.yml:1434). It goes unused under ruling 1.
- **Cut 1:** eligibility is separate from activation. A turn spends the
  wakes inside its range; later wakes stay pending; after a commit the
  controller re-checks; a failed turn re-arms nothing; a planned resume
  activates a desk even when head equals cursor. An owner revision
  commits an owner-emitted record pointing at it, so every inbox sequence
  belongs to a record. The self-development operation stays as the
  effect journal of a candidate bundle. Verify binds to the operation id
  named in engineering's Report, and a deployed run goes from Freeze
  through restore. Engineering keeps Ask and Reply to management and
  loses Cast. The autonomous wire-publication path is a work-item
  consumer and is deleted. Four boot phases are named as kept. The cursor
  is a map per inbox, and the marker gains a work_state_epoch.
  `CancelAct` and Retract are deleted under the §12 delegation, not by an
  explicit owner statement. The packet citer count is corrected to 40
  files and 343 lines; the work-item count of 62 files and 1,090 lines
  stands.
- **Cut 2:** a version's immutable payload (text, manifest hash,
  signatures, resolved blocks) is separate from its live nav and check
  overlay. Search arrivals reach a running cell's store as they land.
  PDFs become blocks, as §7 already said. ReadDoc stays as the one
  document read verb for Textures and captures. The crash-closure record
  never wakes anyone. The tracer's controlled page lives on a
  `tracer-fixtures` branch.

