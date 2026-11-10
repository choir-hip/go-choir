---
definition_version: 4

# The overnight Autopaper mission (owner, 2026-11-09 evening: "we could bunch
# that together with the next auto paper mission, and that can be our
# overnight mission for the night"; "I really want to start the overnight
# mission before I go to bed"). It merges what remains of
# choir-autopaper-improvement-2026-11-09.md with the next Autopaper mission
# (docs/design/autopaper-next-mission-handoff-2026-11-09.md), and carries the
# side lanes the owner named the same evening. The handoff is this file's
# scope document: sections 2 to 6 of it are binding here and are not repeated.
readiness: executable

review:
  reviewer: none (authored at the owner's order under time pressure; the
    handoff it executes was reviewed and revised on 2026-11-09; each red lane
    gets its own review, see boundaries)
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - {id: W0_contracts_and_audit, path: "this file, wave 0", readiness: executable, status: pending, depends_on: []}
    - {id: W1_first_lanes, path: "this file, wave 1", readiness: executable, status: pending, depends_on: [W0_contracts_and_audit]}
    - {id: W2_second_lanes, path: "this file, wave 2", readiness: executable, status: pending, depends_on: [W1_first_lanes]}
    - {id: W3_materiality_clock, path: "this file, wave 3", readiness: executable, status: pending, depends_on: [W1_first_lanes]}
    - {id: X_side_lanes, path: "this file, side lanes", readiness: executable, status: pending, depends_on: []}
    - {id: E_extra_credit, path: "this file, extra credit", readiness: drafted, status: pending, depends_on: [W2_second_lanes]}

start:
  captured_at: 2026-11-10T03:00:00Z
  source:
    canonical_ref: origin/main dac243b35
    deploy_identity: staging 0ebab72a7 (operator path, model switching M1, Devin provider); d@e still on 03300ad5f behind a failed rebase shot review; b@c on 0ebab72a7
  worktrees:
    - {path: /Users/wiz/Dev/go-choir, status: dirty, class: user_wip, owner: owner, touch: forbidden, recovery: "one untracked file .env.bak-bedrock-20261109124037 (secrets); never commit here, never read or print it"}
    - {path: .claude/worktrees/agent-a5f5a23b45e479bef (shot-reuse-build 6b076c2a9), status: clean, class: goal_candidate, owner: previous thread, touch: goal_owned, recovery: "Sonnet seat ACCEPT; Devin seat pending at $S/review-shotbuild/devin.out; widen the compared paths before landing (side lane X1)"}
    - {path: .claude/worktrees/agent-a98db2eb4b80ffb2d (mgmt-opening-repair 986891d1e), status: clean, class: goal_candidate, owner: previous thread, touch: goal_owned, recovery: "Devin round 1 REPAIR answered; round 2 pending at $S/review-mgmtopen2/devin.out (side lane X2)"}
    - {path: branch integrate-1109 (delegate-first 506bf659d, 02dbe83a8), status: clean, class: goal_candidate, owner: previous thread, touch: read_only, recovery: "superseded in intent by the coding-agent desk (X4); do not land; carries a built binary"}
    - {path: stash 'devin-provider-wip' and two older stashes, status: unknown, class: user_wip, owner: owner, touch: forbidden, recovery: "leave; one contains the env backup"}
    - {path: about 210 builder worktrees under .claude/worktrees, status: clean, class: generated_temp, owner: earlier builders, touch: read_only, recovery: "their commits are on main; removal is a later cleanup, not this mission"}

finish:
  deliver: >-
    By the time the owner re-engages on 2026-11-10: the paper's readers can
    edit their section list, edit and revise their own documents, respond to
    a story with an op-ed that the paper's desk answers, and see what
    happened in a changelog; the paper writes fewer short, duplicate and
    non-English stories; research can find the paper's own earlier coverage;
    and the paper runs within its model and search budgets.
  artifact: >-
    The handoff's section 6 "Done for pass 1" items 1 to 8 proved on staging
    with two identities on two computers (the paper's owner path, and a reader
    on d@e), item 9's audit baseline and first rerun recorded, plus W3's
    relies-on links being written and the pre-registration committed.
  acceptance:
    - {action: "handoff section 6 items 1 to 8, each as a deployed product-path run on choir.news with a reader identity on d@e", proves: "the pass-1 product works across two computers by publication and adoption", evidence_class: deployed proof}
    - {action: "the S1 audit script run twice, at least 6 hours apart, output under docs/evidence/autopaper-source-audit-2026-11-10/", proves: "the baseline lane Q is measured against, and the read-versus-cited share per source", evidence_class: deployed observation}
    - {action: "gateway log and spend report over the night", proves: "the paper stayed within Bedrock $20/day and its searches within the live provider's daily allowance", evidence_class: deployed observation}
    - {action: "a research run on d@e that finds and reads an earlier published story on its subject through the ranked search", proves: "W3's retrieval slice", evidence_class: deployed proof}
  rollback: >-
    Each lane is its own commit with a one-line rollback (revert). A published
    op-ed, a recorded reliance link and a changelog event are tape records and
    are not unwritten; a lane that writes a new record kind is proved on d@e
    before it reaches the paper.
  landing:
    required: true
    environment: staging (choir.news)
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the distance between the handoff's section 6 list and what a
    second person on a second computer can actually do on staging, while the
    paper keeps publishing within budget and every new record goes through
    the desks' own boundaries (one revise call into Texture, publication and
    adoption between computers).
  goodharting_would_be: >-
    Lanes merged and unit tests green with no two-identity run on staging; a
    proof run as the owner answering the owner's own story; the quality audit
    improved by publishing fewer stories; a changelog that shows events no
    desk produced.

homotopy:
  realism_axis: >-
    One identity on d@e with fakes, then two identities on two test
    computers, then a reader on d@e against the real paper, then the same
    over a full night of the paper's real traffic.

boundaries:
  mutation_class: red
  authority_sources:
    - 'owner statements of 2026-11-09 quoted in the handoff, in this file and in docs/design/coding-agent-desk-over-mcp-2026-11-09.md section 11'
    - 'CLAUDE.md (Fable orchestrates and designs; Sonnet builds, investigates and reviews; every landing commit carries Ring-Expedite; No Blocking Asks)'
    - 'docs/choir-doctrine.md'
    - 'this file''s defaults where nobody ruled'
  must_preserve:
    - 'the keep set; the restart rules (a crash closes work, nothing resumes without a planned-restart marker)'
    - 'never act on the paper''s computer (computer-1355b83bced1613eb048ee28d66688a8) or the owner''s computer except through product paths the owner uses; reading is allowed; operator acts on the paper are limited to the model policy verbs'
    - 'adversarial drills only on d@e (computer-59c902c5d46caecc0a180a8557164e72); b@c is never deleted; computer-7cc96dc670daf6eccd65919e9f70162a is a real user'
    - 'no secrets, keys or email addresses in the repo or in output; example.com in tests'
    - 'Bedrock $20 a day (the gateway enforces it; the owner allows $40 only on request); River $1 a day; self-development on Synthetic DeepSeek then River'
    - 'no runtime switches; desks talk only through precommitment records; the host never sends research material straight to Texture'
    - 'the paper never merges a reader''s text, and never asks for human approval'
  excluded:
    - 'images: improvement-goal stations S2 to S5 and the rights signals (D7) are deferred to a later mission; do not make a Texture embed publishable'
    - 'the secrets vault and provider vault (owner, 2026-11-09: "We''ll do the whole shebang later", sealed in transit when built)'
    - 'the Mac app and device key work (tabled until later in the week)'
    - 'Autoradio and the second factor'
    - 'the four undecided remote branches; the weekly nixpkgs bump'
    - 'the model-switching fleet document (M2) unless every wave is done'
  protected_surfaces:
    - 'Texture canonical revisions (lanes C1, C2), tape records and the behavior ledger (H1), publication and adoption on the Autopaper object graph, the candidate-shot gate (X1), management''s task path (X2), settlement of open Texture work (X3)'

now:
  status: working
  slice: "W0: the three contracts written and the S1 audit script's first run, with side lanes X1 and X2 landing beside it"
  source_ref: origin/main dac243b35
  deploy_identity: staging 0ebab72a7
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C1_one_revise_call_carries_the_night
    claim: >-
      If every write to a Texture goes through one revise call with sources,
      and results cross computers only by publication and adoption, then
      comments, forks, op-eds, the desk's response and the changelog are all
      consequences of parent links, and the wide pass can be built by
      independent builders without merge friction.
    test: >-
      Wave 1 lanes built in parallel against the W0 contracts merge with
      conflicts only in files a lane does not own, and the handoff's items 3
      to 6 pass on two computers without a lane-specific side channel.
    edge: missing_oracle
    delta_o: >-
      A two-computer drill on d@e and a second test computer run after each
      lane lands, before the next lane builds on it.
    scope_if_supported: the pass-1 Texture writing features on staging
    status: active
    evidence_refs: []
  decision:
    what: >-
      Merge the improvement goal's remainder into the handoff's mission; carry
      the S1 audit as wave 0 and its small source fixes into lane Q; defer
      images.
    kind: purpose
    status: settled
    evidence_ref: owner, 2026-11-09 evening, this thread; audit of the improvement goal (only D13 landed)
    owner_ratification_ref: owner statement quoted in the header
  belief:
    believed_state: >-
      None of the handoff's step 0 contracts exist. The improvement goal's
      stations are not started except the stale-queue rule. The paper makes
      about 900 model calls and 300 searches an hour; one search provider
      answers; swe-2-flash leads the default pool but the paper had not been
      seen on it at capture time.
    main_uncertainty: >-
      Whether the paper's computer has taken the build that leads with
      swe-2-flash and how Devin's rate limits behave at 900 calls an hour;
      and whether the publish and adopt contract can be met with the object
      graph as it stands or needs a new record kind.
    next_observation: >-
      The gateway log for vm-1669020a: share of calls on devin, quota holds,
      Bedrock dollars per hour.
  blocker_or_risk: >-
    Search supply: the paper issues about 7,200 searches a day against about
    3,300 a day of allowance on the one provider that answers, with almost no
    cache hits. Lane A measures and fixes the demand side first.
  next_action: >-
    Read the handoff in full, then write the three W0 contracts as one doc
    and start the S1 audit script; in parallel read the two pending Devin
    reviews and land X1 and X2.

receipts: []

weak_measures:
  - {name: paper model calls per hour by provider, kind: telemetry, baseline: "about 975 an hour, 30% on Bedrock Haiku at 2026-11-10T00:50Z", desired: "Bedrock share falls; total does not rise", decision_use: "whether to change the pool lead or cut calls", cannot_prove: "that the calls are useful work"}
  - {name: paper searches per hour, kind: telemetry, baseline: "about 300 an hour, 7 cache hits in 772", desired: "under 140 an hour", decision_use: "lane A's dedupe and cache work", cannot_prove: "research quality"}
  - {name: quality audit counts, kind: weak_signal, baseline: "2026-11-09 audit: 78% under 300 words, 74 duplicate groups, 32 non-English, 52% no section", desired: "new stories only: lower on each", decision_use: "lane Q's second pass", cannot_prove: "that a story is good"}
---

# The overnight Autopaper mission (2026-11-09)

Run it with `/goal docs/definitions/choir-autopaper-overnight-2026-11-09.md`.
Ledger: [choir-autopaper-overnight-2026-11-09.ledger.md](choir-autopaper-overnight-2026-11-09.ledger.md).

`$S` in the start receipt is the previous thread's scratchpad:
`/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad`.
It holds the review outputs named there, the landing scripts (`allgates.sh`,
`arwatch-ap1.sh`, `deployed.sh`) and the corrected spend script
(`spend2.sh`, also `scripts/spend-report` in the repo).

## How to run it

One wide build pass, a QA pass, then a second pass
([handoff](../design/autopaper-next-mission-handoff-2026-11-09.md) section 2).
Fable orchestrates and writes the contracts and designs. Sonnet builders take
one lane each in their own worktrees, with the file owners the handoff names.
Runtime checks are the main verifier: a drill on d@e, then deployed acceptance.
Review: one Devin round for lanes C1, C2 and H1 and for every red side lane;
two seats (Devin and a Sonnet seat) where a defect would be silent or
irreversible; the standing tiers elsewhere. Every landing commit carries
`Ring-Expedite`. Never wait idle on CI or a deploy.

The owner is asleep. Nothing here needs him: where a decision has no obvious
answer, take the default written below or the conservative option, record it
in the ledger as a named residual, and continue.

## Wave 0: contracts, measurement, the audit

1. **The three contracts** (handoff section 3, step 0, items 1 to 3), written
   as one design doc and reviewed by one Devin seat before any wave 1 builder
   starts: the revise call, publish and adopt between computers, and the
   changelog events. Decide in it which computer runs a reader's section
   questions and where that cache lives.
2. **Jev measurement** (step 0 item 4), from logs and a d@e probe: latency
   and accuracy at 1, 8, 32 and 64 questions a call, and the deployed judgment
   body cap. The per-organization limits need the provider consoles; record
   them as unknown if they cannot be read without the owner.
3. **The S1 source audit** (the improvement goal's station S1, items 1 to 4
   and 6): a read-only script over the paper's published stories and, where
   the product exposes it, captures: per source, captures a day, share read,
   share cited, freshness, failure rate, language; per story, single-source
   dependence and capture-to-publish time. First run tonight, second run at
   least six hours later. It reuses
   `docs/evidence/autopaper-quality-audit-2026-11-09/`.

## Wave 1

Lanes A (Jev parallelism), B (sections as an editable list), C1 (edit and
revise in place, comments, forks), G1, H1 (the behavior ledger) and Q (article
quality), as the handoff's section 4 defines them. Folded in from the
improvement goal:

- into Q: the language field fix (`rss.go` still stamps the source's first
  language on every item), the two failing feeds (venturebeat, cisa), and the
  S1 report's fix list;
- into A: **Jev parallelism applies to every Jev call site, not only the
  section questions** (owner, 2026-11-09 evening: "the Jev parallelism
  should apply for every time we're calling Jev. We should be making many
  calls at once. And I think it's probably the more we call Jev, the better,
  because that's going to be a key part of our learning loop that we're
  going to be calibrating how to call it better and how to calibrate our
  thresholds for how to use it"). Wave 0's measurement lists every call
  site; lane A makes each one issue its questions concurrently within the
  measured limits, and every Jev call records its question, answer, model,
  cost and latency on the tape so thresholds can be calibrated from it.
- into A: search demand. Find which research step issues the paper's
  searches, dedupe and cache them, and give the one live provider a daily
  budget so the month's allowance lasts
  ([cluster record](../problems/paper-model-and-search-plane-cluster-2026-11-09.md)).
  Classify serper's 400 and Brave's monthly 429 as quota.

## Wave 2

Lanes C2 (op-eds, the gate, the desk's response), D (prompt bar), E (the
changelog app), F (asking and investigating), G2 and H2 (algorithm² proposals
from the ledger).

## Wave 3: start the materiality clock

From [autopaper-learning-materiality-2026-11-04.md](../design/autopaper-learning-materiality-2026-11-04.md),
as the owner ruled on 2026-11-09:

1. **Search the paper's own archive properly.** Owner: "we should be doing
   BM25 ... We should make it work better. No embeddings." Published-story
   search becomes BM25-ranked partial match over the whole archive (today it
   scans the newest 2,000 of 5,736 and zeroes any story missing one query
   word, `internal/platform/publication_pieces.go`). Research's overlay tells
   it to find and read the paper's own earlier stories on a subject. This is
   the same defect as lane Q's duplicate detection: one fix, built in lane Q's
   files, proved by a research run on d@e.
2. **Record the reliance link.** When a published story cites an earlier
   published story, write a directed, typed `relies_on` link where the cited
   captures are already visited. Data only; nothing ranks by it; the score is
   never shown to a story-writing desk.
3. **Pre-register** hypotheses AL-H1 to AL-H6 from the note's section 11 as a
   committed document before any link data is read. Settlement horizon 28
   days; weights frozen at commit.

Not in this wave: Texture citing the paper's own stories in its prose, the
hierarchical model, reader-signal custody.

## Side lanes (independent of the waves; run them beside wave 0 and 1)

- **X1. Screenshot verdict reuse** (red, two seats). Built at 6b076c2a9. Read
  the Devin seat; widen the compared paths with `internal/appdesk/firstparty.json`,
  `internal/appdesk/required.json`, `internal/pkgruntime` and `styles/` (the
  Sonnet seat's finding 1); land; prove on d@e that a Go-only deploy costs no
  model call and swaps. Read the logged status histogram of a healthy boot and
  fix the liveness rule from that evidence if a benign 4xx appears. d@e is
  stranded on an old base until this or a judged round passes.
- **X2. The management task opening** (orange, one seat). Round 2 pending;
  land on ACCEPT; prove on d@e with the steps in
  `docs/problems/management-task-opening-defects-2026-11-09.md`; then measure
  the paper's calls per task before and after.
- **X3. Work nobody serves** (red, two seats). The substrate fix from
  [unsettled-obligations-cluster-2026-11-09.md](../problems/unsettled-obligations-cluster-2026-11-09.md):
  ask `unattendedTextureRequest` after a Texture turn that commits a Wait, and
  at a markerless crash boot for open Texture work with no live run; close and
  notify management; never resume. Then scope the late-before-screens check to
  diffs that need a pair, and log the dispatcher's skip cause.
- **X4. The coding-agent desk over MCP** (red). Build per
  [the design](../design/coding-agent-desk-over-mcp-2026-11-09.md) with the
  owner's rulings in its section 11: straight to the standing desk; the Go
  engineer stays as the fallback; Jev routes each interim update to Texture or
  to management; `desk_complete` needs the runtime-run checks and Jev's read of
  the change; the owner's Claude login; no spending cap; progress reported at
  semantic steps. Start with the failure list and step 0's probes. A real paid
  delegate round is allowed only on d@e. It need not finish tonight; land what
  is proved and leave the rest as a named station.
- **X5. Partly built designs, one hour each at most.** Operator verbs O4
  (schedules) and O5 (restart) behind the gate that landed tonight; confirm by
  a code read whether a failed health check rolls a ring back automatically,
  and write a problem record if it does not.
- **X6. Explorations taken further** (owner: "those don't need to land unless
  they are logically convergent, but we can just take them further"). One
  Sonnet agent each, results as evidence docs, no product code: the paper's
  tape synced to a Mac for a day, in bytes and lag (resource fabric); Node B's
  sealed segments re-packed offline as 64 KiB blocks (small-object storage);
  Brier scores over existing desk claims on finished tasks (tasks precommit);
  a vsock echo spike on d@e; the `LeafRunner` seam as a mission proposal;
  branching tape's first three do-now items as a mission proposal.

## Extra credit, only if every wave is proved

The retired-name rename (owner: "Put that on the docket ... extra credit for
auto paper if we finish up before I re-engage tomorrow"): World Wire, Universal
Wire and sourcecycled become Autopaper in code
(`docs/autopaper-sw-design-2026-10-09.md` section 9). Stored names on tapes and
in the object graph are a migration, not a rename; change identifiers and
labels only, and write down what stored names remain.

## Standing checks through the night

Each hour, read only: the paper's calls by provider and its share on
swe-2-flash, Bedrock dollars against $20, searches per hour, and whether the
paper is publishing. The hourly job in the previous thread dies with that
thread; start one here. A Bedrock projection over $20, the paper silent for 20
minutes, or Devin refusing more than half the paper's calls is a reason to act
through the model policy verbs on the paper, and nothing else.

## Proofs owed from earlier work

Take them when a wave is waiting on CI: `choir policy lead` and rollback on
d@e (model switching M1); a live Bedrock call with both cache lifetimes; a
research round on d@e using `Search`, `ReadAll` and `Cut` in one cell; the
prompt size per desk against the 100,000-token bound.

## Report

When the owner re-engages: a spoken letter (the reporter skill), then the
final evidence list CLAUDE.md asks for. Name every expedite.
