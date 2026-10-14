# Goal Files as Precommitments: Design and First Retrospective Numbers

Date: 2026-10-14. Mutation class: **green** (this report, an offline
script `scripts/goal_precommitments.py`, and its generated ledger
`docs/calibration/goal-ledger.md`; no runtime behavior changes).

## Bottom line

- A throughline goal file already makes claims that can be scored. Its
  acceptance list says what will be proved, its bridge conjecture says the
  work will advance a deeper goal, and its receipts say what happened. What
  it lacks is a stated probability frozen before the work starts. This
  report proposes where that probability goes, when it freezes, and who
  settles it.
- **No goal file in the corpus records a probability.** That is 0 of 78
  files and 0 of 295 acceptance criteria. So no planner track record exists
  yet. What the corpus can give is a base rate.
- The base rate, settled by fixed rules from each file's own status and
  receipts: **21 of 26 resolved goals met their finish (0.81, 95% interval
  0.62 to 0.91)**. The climatology baseline, which forecasts that rate for
  every goal, scores **Brier 0.168 out of sample** (0.155 in sample). A
  planner who writes probabilities has to beat about 0.17 to show any skill.
- Four of the five goals that did not land were superseded rather than
  failed. Leave supersession out and the landing rate is 21 of 22.
- Bridge conjectures are the unscored half. Of the 21 met goals, 9 still
  carry the bridge as `active` and none was ever settled by someone other
  than its author. No conjecture in the corpus is marked `falsified`.
- 8 of 78 files are machine-unreadable: 6 YAML errors and 1 file with no
  frontmatter. Four of those are live authorities: the precommitment
  protocol metamission, the re-engineering metamission, re-engineering
  Cut 1, and ledger station L1. The rest of the corpus could be checked by a script today.

## 1. Why goal files

Engineering precommitments, being added in parallel
(`internal/types/commitment_rules.go`), score what a desk says it will do
inside one assignment. They use closed rule kinds (`tests_pass`,
`freeze_within`, `paths_confined`, `verify_pass`, `completes`), a yes/no
distribution with p(yes) in [0.02, 0.98], and settlement by rules from
receipts, never by a model.

Goal files are the planning side of the same say–do loop. Each one says,
before the work: this artifact will exist, this action will prove it, and
building it will advance something deeper. The owner's 2026-10-14 notes
put precommitment (prospective self-alignment, PSA) at the core. They also
name throughline and its predecessors as precursors of PSA. Scoring goal
files closes that lineage: the planner gets the same feedback the desks
get.

## 2. Recording a goal as precommitments

Three kinds of claim, each a yes/no question with a probability:

| Claim | Question (yes = ...) | Who settles | When |
|---|---|---|---|
| Acceptance criterion i | the acceptance action passes and proves its `proves` text by the goal's landing (terminal receipt) | a rule where a command, CI, deploy or consensus receipt exists; otherwise the owner or a panel | at the terminal receipt |
| Lands | `now.status` reaches `complete` with a terminal receipt by `precommit.due` | rule (status + receipt) | at `precommit.due`, or earlier at the terminal receipt |
| Bridge conjecture | the deeper goal G advances as the bridge claims | owner or panel, never the author | at `value.bridge_due`, usually after a later station has used the artifact |
| Within budget (optional) | landed within the stated budget | rule over receipts' timestamps or token ledger | at the terminal receipt |

### Proposed YAML shape

```yaml
precommit:                         # new block; filled at authoring, frozen at promotion
  committed_at: 2026-10-14T18:00:00Z
  author: {agent: <agent or person id>, model_id: claude-opus-5-5}
  frozen_ref: <digest; equals review.frozen_ref at promotion to executable>
  due: 2026-10-28                  # past this, an unlanded goal settles "no" (strict)
  lands_p: 0.70                    # p(yes) that the goal is met by `due`
  within_budget_p: 0.50            # optional
  revisions: []                    # append-only; see the freeze rule

finish:
  acceptance:
    - action: <product path, command, or observation>
      proves: <scoped claim>
      evidence_class: deployed proof
      p: 0.80                      # p(yes): passes and proves by landing
      settles_by: rule:acceptance_command   # rule:<kind> | owner | panel

value:
  better_means: <the loss function>
  goodharting_would_be: <the faked metric>
  bridge_p: 0.60                   # p(yes) that the bridge holds
  bridge_scorer: owner             # owner | panel; never the author
  bridge_due: 2026-11-30
```

Every `p` is p(yes) in [0.02, 0.98], the same bound engineering uses: a
dogmatic 0 or 1 cannot be recalibrated. The distribution is implicit,
`{"yes": p, "no": 1 - p}`.

`settles_by` names the scorer that may settle the criterion. Rule kinds
for goals form a small closed set, as engineering's do:

| Rule kind | Settles "yes" when |
|---|---|
| `ci_green` | the CI run for the terminal receipt's `source_commit` succeeded |
| `deploy_identity` | staging health reported that commit after the deploy |
| `acceptance_command` | the named acceptance command exited 0 against the named environment, with its output ref |
| `consensus_approved` | the completion panel on the frozen landed candidate approved |

A criterion whose evidence class is human inspection or a judgment settles
`owner` or `panel`. Adding a rule kind is a reviewed change, because rules
are the ground truth other scorers are measured against.

### The freeze rule

- Probabilities may change freely while the file is `intent`, `drafted`
  or `reviewed`.
- They **freeze at the promotion to `executable`**. The
  `review.frozen_ref` digest covers the whole frontmatter, p fields
  included, and `precommit.frozen_ref` repeats it.
- After the freeze, edits **append, never rewrite**. A changed belief
  becomes an entry in `precommit.revisions` with `at`, `field`, `p` and
  `reason`. The frozen p is what gets scored. A revision is scored
  separately as a later, cheaper forecast, made at its own time.
- An acceptance item added after the freeze has no frozen p. It is
  recorded and settled but excluded from the author's calibration. One
  dropped after the freeze settles `unresolved` and counts as "no" in the
  strict score.
- A checker can enforce this from git alone: the p fields at the
  promotion commit must equal the current ones, minus appended revisions.
  `--check` in the ledger script is the place to add that check later.

### Settlement at landing

The terminal receipt gains a settlement list, one entry per scorer per
claim:

```yaml
receipts:
  - id: <station>-terminal
    boundary: terminal
    ...                            # identity, proof_refs, landing as today
    settlement:
      - claim: acceptance[1]
        outcome: met               # met | not_met | unresolved
        evidence_ref: <CI run, deploy identity, acceptance output>
        scorer: system:rule        # rule kind named in settles_by
      - claim: acceptance[1]
        outcome: met
        scorer: self:claude-opus-5-5   # the author's self-score, written first
        self_p: 0.95
      - claim: lands
        outcome: met
        scorer: system:rule
```

The rules:

1. **The author self-scores first.** The executing agent writes its own
   verdict and a p for each claim *before* it reads any independent
   settlement. This is the E6a loop's step 2. Calibrated self-scoring is
   a capability, and it gets scored too.
2. **Rules settle where a receipt exists.** A CI run, deploy identity,
   acceptance command output or panel verdict settles its criterion
   mechanically. No model reads the evidence for a rule-settled claim.
3. **The bridge is settled by someone else.** That is the owner or a
   panel, at `bridge_due`, under its own scorer identity. A panel scorer
   records its member models.
4. **Disagreements are kept.** When the self-score and an independent
   scorer differ, both entries stay. The difference is the self-score
   gap, a monitored number per author and model.
5. **Unresolved is honest, not free.** Superseded goals settle their
   criteria `unresolved`, not `not_met`: supersession is not refutation.
   The strict score still counts unresolved as o = 0, and the
   resolved-only score drops them. Both are reported side by side, as E6a
   does.
6. **Score the scorer.** Each scorer (rule, self, owner, panel) has a
   track record. Rules are the reference. A panel's verdicts are compared
   with the owner's on the claims both settle.

## 3. Mapping onto commitment records

`internal/types/commitment.go` already has the vocabulary. A goal file
could be mirrored into the ledger one way (repo to ledger) without new
types:

| Goal file | Commitment record |
|---|---|
| `finish.acceptance[i]` with `p` | `CommitmentPrecommit{Question, Distribution: {"yes": p, "no": 1-p}, Resolver}`. `Question` is "<action> proves <proves> by landing". `Resolver` comes from `settles_by`. A `rule:` scorer maps to the new `Precommit.Rule` field with resolver `rule`; its settlement is a resolve record by the scorer agent `system:rule` |
| `precommit.lands_p` | a second `CommitmentPrecommit` on the same goal |
| `value.bridge_p` | its own `CommitmentPrecommit`, with `Resolver` set to the owner or panel |
| `precommit.committed_at`, `author` | `Provenance.CommittedAt`, `AgentID`, `ModelID`. `Prediction.CommittedAt` must precede the observation, as the type requires |
| `review.frozen_ref` | `Provenance.ContextRef` (the digest the prediction was made under) |
| goal id | `ParentID` for every claim of one goal. `SubjectRefs` names the goal file |
| a `settlement` entry | `CommitmentResolve{Verdict, EvidenceRefs}`, linked by `ParentID` to the precommit |
| a differing second scorer | `CommitmentDisagreement{ScorerVerdict, ResolverVerdict, EvidenceRefs}` |
| `unresolved` | `DiscrepancyUnresolved` |
| Brier, calibration | a derived view, never stored, like `RulePrecommitSayDo` |

**What is the same:** yes/no questions, the [0.02, 0.98] bound, frozen
before evidence, settled by rules where receipts exist, several scorers
per claim with disagreement preserved, scores as views, strict and
resolved-only side by side.

**What differs:**

- **Authority.** A goal file is a repo document. Its authority is a git
  commit and a digest, not a record on a computer's tape. A mirror is a
  projection; the file stays the source.
- **Freeze enforcement.** The runtime refuses a late engineering
  precommit, for example one staged after Freeze. A goal file's freeze can
  only be checked after the fact, from git history.
- **Author.** Goal files are written by coding agents and the owner
  outside any computer, so there is no `ComputerID`. A mirror needs an
  `EmitterKind` for a repo planner. None exists yet.
- **Settler.** The agent that executes a goal also writes its receipts.
  Engineering's rule scorer reads receipts the runtime produced. A goal's
  receipts are hand-written, so rule settlement here is only as good as
  the receipt's refs. A rule should re-read the CI run or deploy identity
  the receipt cites, not trust its prose.

## 4. First retrospective numbers

Generated by `python3 -I scripts/goal_precommitments.py` (rules
`goal-ledger-rules/1`, date 2026-10-14). The full per-goal table is in
`docs/calibration/goal-ledger.md`. Every rule is in the script's
docstring and summarized here.

### Settlement rules in brief

- **Goal outcome** comes from `now.status` (first word) and receipts only:
  - **met**: a success status (`complete`, `landed`, `settled`, `closed`)
    plus at least one terminal-class receipt. Rules R1 to R8 name what
    counts: a terminal boundary or kind, a non-placeholder
    `deployed_acceptance`, an approved completion panel, a settled
    `outcome` receipt, a `finish.acceptance` receipt with proof refs, and
    v3-style string receipts labelled terminal, settled or
    landed-and-deployed.
  - **claimed**: a success status with no such receipt.
  - **not_met**: `blocked_incomplete`; or bare `blocked` on a file that
    reached reviewed or executable; or `superseded` with no terminal
    receipt.
  - **partial**: `checkpoint_incomplete` with receipts.
  - **open**: working or drafting statuses, and `checkpoint_incomplete`
    with no receipts.
  - **unscorable**: unparseable files and status words outside the
    vocabulary.
- **Criteria:**
  - A criterion is met individually only when a positive receipt names it,
    by "acceptance action N" or by quoting its action or proves text.
  - Otherwise a met goal's criteria are met as a block
    (`met_by_goal_receipt`). The exception is a landing-required goal with
    no deployed-evidence receipt: its deploy-class criteria stay
    unresolved.
  - A blocked goal's criteria are not met.
  - A superseded goal's criteria are unresolved.

### Counts

| Measure | Value |
|---|---|
| Goal files | 78 |
| Parsed | 71 (6 YAML errors, 1 with no frontmatter) |
| Goal outcomes | met 21, claimed 4, partial 0, not met 5, open 40, unscorable 8 |
| Acceptance criteria (parsed files) | 295 |
| Criteria settled | met 81 (79 as a block from the goal receipt, **2 named by a receipt**), not met 6 |
| Criteria unresolved | 208: open 161, superseded 19, claimed 18, unscorable 10 |
| Files or criteria with a stated probability | **0** |
| Files with a bridge conjecture claim / with `value.better_means` | 69 / 71 of 71 parsed |

### Base rates and the climatology baseline

Outcome o = 1 for met. The populations are:

- **resolved**: met against not met;
- **strict**: not met, partial and claimed all count as 0;
- **lenient**: claimed counts as met.

The climatology forecast gives every goal the base rate. Leave-one-out
gives each goal the rate of the others. The family column gives each goal
the rate of its own family's other goals.

| Population | n | met | Base rate (95% Wilson) | Brier in sample | Brier leave-one-out | Brier family, leave-one-out | Brier at p = 0.5 |
|---|---|---|---|---|---|---|---|
| resolved | 26 | 21 | 0.81 (0.62–0.91) | 0.155 | 0.168 | 0.143 | 0.250 |
| strict | 30 | 21 | 0.70 (0.52–0.83) | 0.210 | 0.225 | 0.273 | 0.250 |
| lenient | 30 | 25 | 0.83 (0.66–0.93) | 0.139 | 0.149 | 0.124 | 0.250 |

How to read it: a planner who wrote "p = 0.8, this lands" on every goal
would score about 0.17. Skill is 1 − BS / 0.168. Knowing the family helps
a little on the resolved population (0.143) but hurts on the strict one
(0.273, worse than a coin). The families are too small to carry
information yet.

By slice (resolved rate; the open and unscorable files are excluded from
the rate):

| Slice | met / not met / claimed / open | Resolved rate |
|---|---|---|
| definition_version 3 (2026-09, before the readiness ladder) | 13 / 0 / 2 / 1 | 1.00 |
| definition_version 4 | 8 / 5 / 2 / 39 | 0.62 |
| readiness `reviewed` | 3 / 3 / 2 / 18 | 0.50 |
| readiness `executable` | 3 / 0 / 0 / 7 | 1.00 |
| readiness `drafted` | 2 / 2 / 0 / 12 | 0.50 |
| family `signal` (2026-09-29 wave) | 2 / 3 / 0 / 3 | 0.40 |
| family `appdev` | 4 / 0 / 1 / 16 | 1.00 |
| metamission spines (7) | 0 / 1 / 1 / 5 | 0.00 |
| month 2026-09 / 2026-10 | 15 / 3 / 2 / 6 and 6 / 2 / 2 / 34 | 0.83 / 0.75 |

### Bridge conjectures

| Bridge status (hand-maintained) | met goals | all parsed goals |
|---|---|---|
| supported | 10 | 14 |
| active | 9 | 12 |
| resolved | 2 | 2 |
| proposed | 0 | 33 |
| testing, weakened, partially weakened, none | 0 | 10 |
| falsified | 0 | 0 |

Nine goals landed with their bridge still `active`. The bridge is where
"achieved the objective, missed the goal" would show, and nobody settled
it.

### What surprised me

- **Receipts almost never name the criterion they prove.** Only 2 of 81
  met criteria were named by a receipt. Everything else is inferred from
  the goal's terminal receipt. Today, per-criterion calibration could not
  be scored even with probabilities.
- **The status vocabulary drifted.** The schema has 5 statuses. The
  corpus uses 13 words, among them `settled`, `landed`, `closed`,
  `deferred`, `in_progress`, `drafted`, and one free-text status
  ("autopaper live; residual acceptance items open").
- **`checkpoint_incomplete` is used as "parked".** All 10 files with that
  status have zero receipts. Nine are appdev stations the metamission set
  aside; the tenth is `choir-signal-research-hillclimb`. The skill defines the status as "useful uphill progress landed".
  The rules count them as open, not partial.
- **Not landing mostly means superseded.** Four of the five not-met goals
  are supersessions. Only `choir-signal-management-scorer-jev` is
  `blocked_incomplete`.
- **Four goals claim success with no receipt a rule can read.**
  `choir-appdev-s0-reality-boot-timeline`,
  `choir-carrier-substrate-integrity` and `choir-reeng-cut0-eyes` (both
  with `receipts: []`), and `choir-selfdev-gate` (only a charter receipt).
  Their landings may be recorded elsewhere, for example in the
  rectification spine. The goal file itself does not show them.
- **Live authorities do not parse.** `choir-precommitment-protocol-metamission`,
  `choir-reeng-metamission`, `choir-reeng-cut1-authority` and
  `choir-ledger-l1-restart` fail YAML on unquoted `: ` inside plain
  scalars. `choir-ontology-kernel` has a one-space indent error.
  `choir-appdev-s0m-record-native-messaging` is pure YAML with no closing
  `---`; the script reads it under rule P1b.
  `choir-ontology-kernel-migration-design` is a design doc stored in
  `docs/definitions/`. The docs truth checker does not parse goal-file
  frontmatter.

## 5. Limits

- **This is a baseline, not calibration.** No one stated a probability,
  so there is nothing to calibrate. The base rate says how often goals
  landed. It says nothing about whether any planner knew which ones would.
- **Survivorship.** Git history shows 148 goal files ever added under
  `docs/definitions/`, of which 70 were deleted. Most went on 2026-09-22
  and 23 in the schema consolidation, and 8 on 2026-10-12, among them
  re-engineering R1 to R6. Abandoned plans are likely over-represented
  among the deleted, so the surviving base rate is probably inflated.
  That is a hypothesis; the deleted files were not scored.
- **Status fields are hand-maintained.** The rules read what the authors
  wrote. A file marked `complete` by mistake scores as met if it also has
  a terminal-class receipt.
- **Receipts rarely map 1:1 to criteria.** The block rule
  (`met_by_goal_receipt`) credits every criterion of a met goal. Some of
  those criteria may in fact have been dropped or substituted.
- **The deployed-evidence test is broad.** It matches "staging",
  "deployed" and "choir.news" in receipt text, because it only gates the
  landing exception. A receipt saying "not yet deployed" would pass it.
- **Open goals have no due date.** Without `precommit.due`, 40 open goals
  cannot be called overdue. The strict population therefore leaves them
  out rather than scoring them "no".
- **Small n.** 26 resolved goals. The intervals above are wide, and the
  version 3 / version 4 contrast may come from v3 files being written up
  after landing, not from better planning. That is a hypothesis.

## 6. Proposed amendment to the throughline skill

This is a proposal for review. The skill file is not edited by this
change.

```diff
 ## Define The Finish Before The Method
 ...
+## Precommit The Finish
+
+Every acceptance item carries `p`: the author's probability that the
+action passes and proves its claim by landing, and `settles_by`: the rule
+kind (`ci_green`, `deploy_identity`, `acceptance_command`,
+`consensus_approved`) or scorer (`owner`, `panel`) that settles it. The
+`precommit` block carries `committed_at`, the author's agent and
+`model_id`, `due`, and `lands_p`. `value.bridge_p` is the author's
+probability that the bridge holds, settled by `value.bridge_scorer`
+(owner or panel, never the author) at `value.bridge_due`. Every p is
+p(yes) in [0.02, 0.98].
+
+**Freeze.** Probabilities freeze at the promotion to `executable`;
+`review.frozen_ref` covers them. After the freeze, a changed belief is an
+appended `precommit.revisions` entry; the frozen value is never
+rewritten, and it is the value scored.
+
 ## Settlement
 ...
+The terminal receipt carries `settlement`: one entry per claim per
+scorer, with `outcome` (met | not_met | unresolved), `evidence_ref` and
+`scorer`. The author writes its self-score (outcome and `self_p`) before
+reading any independent settlement. Rules settle every criterion whose
+`settles_by` is a rule kind, by re-reading the cited CI, deploy or
+acceptance output. Superseded claims settle `unresolved`. Scores are
+derived views; they are never written into the goal file.
+
 ## Conformance Check
 ...
+- gives every acceptance item a `p` and `settles_by`, and the bridge a
+  `bridge_p` with a scorer other than the author, frozen at promotion
+  to `executable`;
+- settles each claim in the terminal receipt by naming it (rule,
+  self-score, and any owner or panel verdict kept side by side).
```

The minimal template in `references/mission-schema.md` would gain the
same fields: `precommit` at the top level, `p` and `settles_by` on each
`finish.acceptance` item, `bridge_p`, `bridge_scorer` and `bridge_due` in
`value`, and `settlement` on terminal receipts.

### Goodharting note

Calibration alone can be gamed in the planning seat:

- **Sandbagging.** The author writes low p on criteria it knows are easy,
  then "beats" its forecasts.
- **Trivial criteria.** The author writes acceptance actions that are
  certain to pass ("the health endpoint returns 200") and states a high,
  well-calibrated p. The Brier score is excellent and the finish proves
  nothing.
- **Owning the bridge.** If the author settles its own bridge, the claim
  that matters most is graded by the party with the most to gain.

The fixes:

- **Score skill against the base rate**, not only calibration. A planner
  who forecasts the climatology rate is perfectly calibrated and has zero
  skill. Report 1 − BS / BS_climatology per author, next to the
  reliability term.
- **Weight criteria by what they prove.** A criterion whose `proves` is a
  restatement of its `action` should fail review. This is the existing
  `goodharting_would_be` discipline applied to each criterion.
- **Keep the bridge scored by someone else**, with that scorer's own
  track record. The landed-candidate consensus gate already exists; the
  bridge verdict at `bridge_due` is a second, later gate on purpose.
- **Watch the self-score gap**, the author's self-score minus the
  independent outcome, per author and model. A planner who learns to
  please the scorer rather than predict the world shows up here first.

## 7. Mutation class and residuals

- **Class: green.** Docs plus an offline script with no runtime, CI or
  deploy effect. Protected surfaces touched: none. No landing loop: the
  change is docs plus a script nothing runs.
- **Heresy delta:**
  - *discovered*: 8 machine-unreadable goal files (4 live). Status
    vocabulary drift. `checkpoint_incomplete` used for parked stations.
    Four success claims without a readable terminal receipt.
  - *introduced*: none.
  - *repaired*: none. The goal files were not edited.
- **Residuals:**
  1. Fix the YAML in the 6 unparseable files and decide whether
     `choir-ontology-kernel-migration-design` belongs in
     `docs/definitions/`.
  2. Owner or panel decision on the amendment above. Then the first goal
     authored with p fields, so a real track record starts.
  3. Wire `--check` into the docs-only truth workflow, report-only at
     first, and add the freeze check (p fields at the promotion commit
     equal the current ones, minus revisions).
  4. Score the 70 deleted goal files from git history, to measure the
     survivorship bias in the base rate.
  5. Mirror into the ledger (section 3) only after an `EmitterKind` for
     repo planners is agreed.

## Reproduction

```text
python3 -I scripts/goal_precommitments.py                 # regenerate the ledger
python3 -I scripts/goal_precommitments.py --check         # exit 1 if stale
python3 -I scripts/goal_precommitments.py --json out.json # full extraction
```
