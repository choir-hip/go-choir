# PSA, Precommitment Records and ICRL in the REPL: Status and Design (2026-10-24)

Status: **design, green.** No code, no deploy, no staging access. Written for
the owner's design day on "PSA / pre-commitment records / ICRL in the REPL"
(owner brief, 2026-10-24 about 05:45 UTC). The owner expects that "we may not
have fully integrated PSA/precommitment records and in-context reinforcement
learning" yet. This document says how far that is true. It inherits
[Choir Doctrine](../choir-doctrine.md) and
[precommitment-records.md](../precommitment-records.md), the normative
mechanism document. It is not doctrine.

Read at `origin/main` 911be16b1. Every claim about code is marked **verified**
with file:line, or **hypothesis**. Claims about what ran on staging come from
the evidence documents named, not from staging.

---

## 0. Summary

- **More is built than the 2026-10-14 status says.** Three precommitment
  loops exist in code, all committed on 2026-10-14 or later. None of them
  feeds the others, and none closes the in-context learning loop in
  production today:
  1. **Engineering's rule-settled precommitments.** Engineering calls
     `choir.Precommit` in its capsule REPL, and the runtime settles each claim
     from receipts: the Freeze, management's Verify, the assignment's end. In
     tonight's drill, engineering really used this in both rounds.
  2. **The desk calibration digest** (the ICRL experiment) for management,
     research and Texture. It is built, but the switch `calibration.feedback`
     is off by default and off by owner ruling. No desk has read a digest.
  3. **Calibration level 2.** `/api/precommit` and `choir precommit ...` are
     for agents outside the desks (Claude Code, the conductor). They are built
     and nobody uses them routinely.
- **The ICRL loop is open for every desk.** Engineering's `choir.Pack()` is
  always empty in the capsule, although its prompt says the pack "shows how
  each settled" (verified, §2.3). Engineering is not one of the digest's
  desks, and the say-do view `RulePrecommitSayDo` has no caller. The other
  desks' digest is switched off.
- **The smallest end-to-end slice** is engineering, because its outcomes can
  be checked by rule, as the 2026-09-10 memo said to start. Wire the say-do
  record that already exists into engineering's pack and a digest turn. Add
  two settlements grounded in the owner's decision and the apply
  (`owner_accepts` and `applies`). Let management's Verify state a
  probability that the same owner decision settles. Section 3.
- **Update architecture interacts concretely in one place.** The release
  record already carries engineering's settled precommitments
  (`internal/release/record.go:54`). Tonight's app rewrites also hit three
  update and divergence consequences that a commitment written at apply time
  could have named and rules could settle. Package subscriptions as standing
  precommitments, and a computer learning which updates to take, are
  speculative until S7 exists. Section 4.

---

## 1. What the terms mean here

**Precommitment record.** "A typed, time-stamped record, immutable once
committed, of a hypothesis, its assumptions, the alternatives considered, and
a prediction with probabilities, bound to an actor, task and time and
falsifiable against a later observation" (precommitment-records.md §1.1).
There are two hard requirements: the prediction is committed before the
agent can see the evidence that settles it, and it settles against an
outcome that can be checked. The loop is "commit, authorize, act, observe,
resolve, score, revise, retrieve". Authorization is a separate step: "an
accurate promise to do an unauthorized thing does not make it acceptable".
In code it is a *commitment record* of kind `choir.commitment_record`
(§1.5). The document's own caveat: "pre-outcome ordering is not proven"
(§8.1). A record carries its commit time, but the system does not show that
the agent lacked access to the settling evidence.

**PSA (prospective self-alignment).** The research program: "whether conduct
and consequences stay coupled to the prospective representation under which
a system was allowed to act" (§1.5). *Self-alignment* means the agent's own
earlier commitments become things to assess, not that the agent decides what
is permitted. PSA keeps four measurable surfaces apart: epistemic
(calibration), material (coverage of consequences), instrumental (outcome)
and procedural (say-do fidelity). It treats a fifth, normative, as governance
rather than measurement (§3.1). Its own caveats: "PSA is a research program,
not a demonstrated guarantee of alignment, honesty or safety" (§0, and the
archived paper's abstract). Not every alignment problem reduces to
capability: "hidden objectives, adversarial action, measurement gaps and
illegitimate authorization stay live" (§3.2). None of hypotheses H1 to H5
has been run (§8.1, §8.4).

**ICRL (in-context reinforcement learning).** The owner, 2026-10-14: "the key
value of PSA is storing records in the desk's Yaegi REPL, with context packs
and on-demand retrieval; it serves as in-context reinforcement learning,
where feedback to models enables self-learning without modifying weights"
(§1.2; [owner notes §1](../reports/owner-notes-2026-10-14.md)). The working
hypothesis is that "in-context RL is to in-context learning as RL is to
self-supervised pretraining". The document calls the evidence "thin but
consistent". It names the same mechanism as the driver of long-horizon reward
hacking, and so makes context design "a safety decision": feed the desk
observations and discrepancies, and keep independent scores out of its
context, or sample and delay them (§1.2, §4.7). The archived memo
([memo-predictive-icrl-delta-2026-09-10](../archive/precommitment/memo-predictive-icrl-delta-2026-09-10.md)
§1) adds two warnings this design follows. Rest the headline claim on
classes of prediction that can be settled objectively (code, tests, dated
events), not on judging reading. Prospective prediction also anchors: it can
compound confirmation bias.

The owner has since tied it together (2026-10-23, metamission ledger 04:40):
"precommitment records/psa ... thats the key bridge from autoputer and
self-dev to autoradio. and its the essential core scientific idea of choir".
Doctrine says the same: "Self-development is only real when built on
precommitment scoring" (choir-doctrine.md, product order).

---

## 2. What is built (verified from code)

### 2.1 The record and its storage

- The typed record, the act kinds and the precommit body are in
  `internal/types/commitment.go` (579 lines). Records are minted once, under
  a deterministic id, on the object graph and tape
  (`internal/store/commitment.go`). Verified that the files exist. For the
  not-exists condition this document relies on precommitment-records.md
  §2.2.
- Engineering's rule settlement uses one resolve id per precommit,
  `RuleResolveRecordID` = `<id>:resolve:rule`, so the first settlement wins
  (`internal/types/commitment_rules.go:478-481`, failure mode R4 at :41-43).
  Verified.
- The record still has an in-record `Scores` slice, against the decision
  that scores are separate records (precommitment-records.md §8.3 item 5).
  Rule settlements follow the decision: they are separate resolve records
  (`commitment_rules.go:20-24`). Verified.

### 2.2 Desk verbs in the REPL (the in-cell module surface)

| Desk | Precommit | Resolve / Disagreement | Report | Evidence |
|---|---|---|---|---|
| management | yes | yes | yes | `internal/deskmanifest/manifest.go:208-209` |
| research | yes | yes | yes | `manifest.go:239-240` |
| engineering (capsule) | yes | no | yes | `manifest.go:258` |
| Texture | **no** | no | **no** (has `Write` with `pSupported`) | `manifest.go:226` |
| management task | no | no | no | `manifest.go:276` |

- `choir.Precommit(any)` stages a typed prediction, and the reducer authors
  the record when the cell returns (`internal/yaegikernel/choir.go:1391-1401`;
  surface text at `internal/yaegikernel/surface.go:65`). Verified.
- `choir.Pack()` returns the acting desk's commitment pack: its own acts and
  acts addressed to it, joined with their resolutions and discrepancies.
  There is no score field "by construction" (`choir.go:1289-1301`;
  `internal/types/commitment_views.go:486-522`). It includes the digest when
  one is set (`surface.go:51`). Verified.
- The pack is filled only on the host desk-cell path
  (`internal/agentcore/tools_desk.go:560`, built at :934-956, which lists the
  last 256 claim records on the computer and keeps 16 items). Verified.
- **Engineering's pack is always empty.** The capsule request has no pack
  field (`internal/capsule/types.go:155-172`: Source, Cwd, AllowedPackages,
  TimeoutMS, Inbox, Updates, Records, Jobs). The capsule broker builds its
  frame without one (`cmd/capsule-broker/session_worker.go:227`). With no
  pack, `choir.Pack()` returns an empty pack (`choir.go:1297-1299`). Yet
  engineering's prompt says "choir.Pack shows how each settled"
  (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:63`).
  Verified in code. That a live engineering cell's `choir.Pack()` returns
  zero items is a hypothesis: no run event read tonight shows a Pack call.
- **There are no record query verbs.** No REPL verb lists a desk's own open
  or settled precommitments, or a track record by claim kind.
  `choir.Records()` returns inbox records, not commitments (surface.go). This
  matches precommitment-records.md §8.1 "Not built: record query verbs in the
  REPL". Verified by the absence of any such verb in
  `deskmanifest/manifest.go` and `surface.go`.

### 2.3 Engineering's rule-settled precommitments (built 2026-10-14, used tonight)

- **The vocabulary** is a closed set of six kinds: `tests_pass`,
  `freeze_within`, `paths_confined`, `verify_pass`, `completes` and
  `visual_captured` (`commitment_rules.go:72-104`). "Adding a kind is a
  reviewed change" (:73-74). Verified.
- **Validation.** Answers are exactly yes and no, and p(yes) lies in
  [0.02, 0.98] (:140-165, constants :119-120). Only engineering may carry a
  rule. A rule precommit may not share a cell with Freeze or Complete, and is
  refused after the candidate froze
  (`internal/agentcore/engineering_precommit_rules.go:63-87`; R1 and R2 at
  `commitment_rules.go:28-36`). The reducer stamps the assignment id and the
  opening time, so the model cannot move the clock (R1). Verified.
- **Settlement points.** A fresh successful Freeze settles `tests_pass`,
  `freeze_within`, `paths_confined` and `visual_captured`
  (`engineering_precommit_rules.go:197-224`). Management's Verify settles
  `verify_pass` (`internal/agentcore/rlm_reduce.go:1584-1589`). The terminal
  disposition settles `completes` and whatever is still open
  (`engineering_precommit_rules.go:230-246`; rules at
  `commitment_rules.go:439-476`). Settlement is best effort and never fails
  the act it observes (R6). Verified.
- **Prompting.** Engineering is told to precommit in its first cell, one
  claim per question (`rlm_engineering_runtime.yaml:63`). Verified.
- **Views.** `RulePrecommitSayDo(records, "engineering:")` computes Brier
  over resolved claims and a strict Brier, per rule kind
  (`commitment_rules.go:612-616`). **It has no non-test caller.** Verified
  by grep over `internal/` and `cmd/`. The say-do record is computed nowhere
  that a desk or the owner reads.
- **Release record.** The release record carries engineering's
  precommitments with p(yes) in thousandths and the settlement
  (`internal/release/record.go:54`, :91-100). They are read from the ledger
  for an applied operation (`internal/agentcore/api_release.go:136-177`).
  Verified.

### 2.4 The desk calibration digest: the ICRL experiment (built 2026-10-14, switched off)

Design and pre-registration:
[icrl-calibration-experiment-2026-10-14](../reports/icrl-calibration-experiment-2026-10-14.md).

- **Observations** are collected from the tape
  (`internal/agentcore/calibration_feedback.go:152-284`):
  - management's (and Texture's) Cast hypothesis probabilities, settled by
    a counterpart Report or by an independent "calibration score record";
  - research's final-Report `p_complete`, settled by a scorer's coverage;
  - Texture's `pSupported`, settled by Jev's support verdict.

  Verified.
- **Desks.** Only management, Texture and research have a digest. A thread
  that maps to any other desk, engineering included, returns ""
  (`calibration_feedback.go:78-87`). Verified.
- **The arithmetic** is Brier, ECE over five bins and mean p against
  observed. A 10% held-out set is never shown, nothing is shown under n = 20,
  and no bin is shown under n = 5 (`internal/calibration/calibration.go:63-235`).
  The digest text is stable when nothing changed (:254-292). Verified.
- **Placement.** A user turn headed `Calibration digest:` goes after the
  history, never in the system prompt, so the cached prefix holds. It is
  re-sent only when the text changes (`calibration_feedback.go:360-407`). The
  same digest is in `choir.Pack().Calibration` (`tools_desk.go:952-954`).
  Arms are fixed per desk thread by hash (`calibration.go:98-107`). Verified.
- **The switch.** `calibration.feedback` defaults to false
  (`internal/store/computer_setting.go:31-36`, :66). The owner ruled on
  2026-10-14: "calibration feedback stays off"
  ([owner notes §19](../reports/owner-notes-2026-10-14.md), line 332). The
  forecast-act design (2026-10-20) keeps it off until the H-ICRL readout.
  The only evidence file is the baseline
  (`docs/evidence/icrl-calibration/baseline.json`). No phase-0 or phase-1
  result is in the repo. Hypothesis: the experiment's settlement writer
  (`scripts/icrl_calibration.py settle`) was never run on a schedule. No
  ledger entry records it.
- **Pressure on resources.** A digest snapshot lists up to 20,000 commitment
  records, plus citation checks and verdicts, once per 30 minutes per
  computer (`calibration_feedback.go:37-41`, :293-322). This is noted under
  the resource rule. It costs nothing while the switch is off (:342).
  Verified.

### 2.5 Calibration level 2: agents outside the desks (built 2026-10-14)

- **Routes.**
  - `POST/GET /api/precommit/claims`, `POST .../claims/{id}/settlements` and
    `GET /api/precommit/calibration` (`internal/calibration/handler.go:96`).
    They are mounted at `internal/apihandler/routes.go:24` and gated by the
    proxy as `precommit:read` and `precommit:write`
    (`internal/proxy/handlers.go:591-594`). The scopes can be minted
    (`internal/auth/handlers.go:1565-1566`).
  - These routes are deliberately not on management's ProductAPI allowlist:
    "desks write their own records through their in-cell module surface"
    (`internal/agentcore/api_precommit.go:9-15`).

  Verified.
- **Records.** A claim is a precommit record and a settlement is a resolve
  record linked by ParentID. Both carry a `CommitmentCalibration` body, and
  no new kind is registered. Writes are append-only, and a replay is a no-op
  (`internal/calibration/doc.go:1-22`). Verified.
- **Scorers.** The governing settlement ranks rule over owner over
  independent over self, and disagreement is kept
  (`internal/calibration/math.go:31-65`). The self-score gap is reported
  (:88-96). Verified.
- **Context text.** `ContextText` renders "the plain-text track record an
  agent pastes into its context" (`math.go:408-411`). This is level 2's
  version of the ICRL loop, done by hand. Verified.
- **CLI and MCP.**
  - `choir precommit add|settle|list|calibration|import-ledger`
    (`cmd/choir/commands.go:659-764`; `cmd/choir/precommit.go:18-21`).
  - `import-ledger` turns the level-1 Markdown ledger
    (`docs/calibration/claude-ledger.md`, 19 rows) into claims. It marks
    settled rows `reconstructed`, because they were written after the fact
    (`precommit.go:145-249`).
  - The MCP server tells agents to call `calibration_summary` "before
    stating new probabilities" (`cmd/choir/mcp.go:32`).

  Verified.
- **The three tallies are disjoint.** The desk digest does not read level-2
  claims. `collectCalibration` reads Cast hypotheses, research Reports and
  Texture citation checks only. Level 2 does not read engineering's rule
  settlements. Nothing reads `RulePrecommitSayDo`. Verified by reading
  `calibration_feedback.go:152-284` and `internal/calibration/handler.go`.

### 2.6 Settling inside the protocol, and the owner's view

- **F-new-1 still holds.** A Cast hypothesis can be settled only by the
  counterpart's Report.
  - Texture has no Report verb (`manifest.go:226`).
  - Research's `ReportPacket` decodes into `SourcePacket` with
    `DisallowUnknownFields` (`internal/yaegikernel/report_packet.go:24-27`),
    and `SourcePacket` has no `settled` field
    (`internal/types/evidence.go:60-86`).
  - So no desk can settle a hypothesis inside the protocol.

  Verified for the verbs and the packet. The counterpart-only rule in
  `store/inbox.go` is taken from the ICRL document's §6.
- **Management's Precommit has no settler.** A non-engineering Precommit may
  not carry a rule (`engineering_precommit_rules.go:69-73`), and nothing
  else settles management's free-form precommits. Verified for the rule
  restriction. That nothing else settles them is a hypothesis, based on
  grep: no other settlement writer was found.
- **The owner's view in Texture.** The commitment materiality projection
  (falsified first, then overdue, then open claims) is rendered as
  evidence-source entities in a Texture doc
  (`internal/textureowner/texture_evidence_sources.go:797-830`, called at
  :706). Verified. The score-carrying `BuildSupervisionPack`
  (`commitment_views.go:528`), `AccrualByAgent` and `AccrualByDoc` have no
  non-test caller. Verified by grep.
- **The learning-claim gate** (refuse a learning claim that cites no scored
  record) is wired into capsule verification
  (`internal/agentcore/tools_capsule.go:468-486`). Verified.

### 2.7 Table

| Capability | Designed in | Built | Evidence |
|---|---|---|---|
| Typed record, append-once on the tape | precommitment-records §2 | yes | `types/commitment.go`; `store/commitment.go` |
| `choir.Precommit` in the REPL | §4.5 | yes, for management, research and engineering; **not Texture** | `manifest.go:208,239,258,226`; `choir.go:1391` |
| Hypotheses on Casts (kind, criterion, `resolves_by`) | §4.4; L3 | yes (L3-1, 2026-10-15) | management overlay line 23; roadmap re-cut §6 |
| Settlement of hypotheses inside the protocol | §4.4 | **no** (F-new-1) | `manifest.go:226`; `evidence.go:60`; `report_packet.go:26` |
| Off-computer scorer writing calibration score records | ICRL §2.3 | yes (route and script); not run on a schedule (hypothesis) | `routes.go:34-35`; `scripts/icrl_calibration.py` |
| Rule-settled engineering say-do | §3.1 procedural; commitment_rules | **yes**, used tonight | `commitment_rules.go:72-104,439-476`; drill bundle (§5) |
| Rule kinds for the owner's decision, the apply, surviving a refresh | not designed | no | `commitment_rules.go:100-104` |
| Say-do view (`RulePrecommitSayDo`) | commitment_rules | partial: function built, **no caller** | `commitment_rules.go:612-616` |
| Desk digest (ICRL), management, research and Texture | ICRL experiment | yes; **switched off** by default and by owner ruling | `calibration_feedback.go:78-87,340-407`; `computer_setting.go:66`; owner notes §19 |
| Engineering digest or track record in context | §4.7 "a desk sees its own settled track record" | **no** | `calibration_feedback.go:78-87`; `capsule/types.go:155-172` |
| `choir.Pack()` for engineering | engineering overlay | **no** (always empty; the prompt says otherwise) | `session_worker.go:227`; `rlm_engineering_runtime.yaml:63` |
| Record query verbs in the REPL | §4.7, §8.1 | no | surface.go |
| Automatic pack injection by corrective value | §4.7 | no; stays off "until held-out evidence supports it (H2)" | precommitment-records §4.7 |
| Level-2 API, CLI, MCP for outside agents | choir-mcp-precommitments | yes | `calibration/handler.go`; `commands.go:659-764` |
| Level-1 Markdown ledger import | level 2 | yes (dry run by default) | `precommit.go:261-333` |
| Release record carries settled precommitments | selfdev-update-architecture §1 | yes | `release/record.go:54,91-100`; `api_release.go:136-177` |
| Adopter reads a publisher's say-do when deciding | §4.6 reputation | no (hypothesis: no reader) | none found |
| Supervision view in Texture | §1.3; CLAUDE.md | partial: materiality projection only | `texture_evidence_sources.go:706,797` |
| Score-carrying supervision pack, per-agent box score | §5.5 | function built, no caller | `commitment_views.go:528,367,378` |
| Materiality coverage, critics, neutral probe | §3.1, §2.4 | no ("theory only" for the probe) | precommitment-records §2.4 |
| Signing, `commit_hash`, publication of commitments | §6 | no | precommitment-records §6 |
| Historical back-tests (H0), claims layer, public box scores | §5.5-5.7 | no (offline E0 and E6a ran) | `docs/reports/claims-e0-e6a-results-2026-10-14.md` |
| Package subscriptions, update decisions as precommitments | none | no | §4 below |

**Status drift.** precommitment-records.md §8.1 (dated 2026-10-14) and the
roadmap re-cut draft §6 (lane `fp0-roadmap-recut`, d215cf6ef) both list
"settlement and scoring" as not built. Neither mentions engineering's rule
settlement, the switched-off digest or level 2. A §8.1 refresh is a green
follow-up. This document does not edit the normative doc.

---

## 3. The gap to "PSA in the REPL", and the smallest end-to-end slice

### 3.1 What a desk needs, piece by piece

| Piece | Exists? | Where it would live |
|---|---|---|
| (a) Record a claim with p and a settlement condition at decision time | yes for engineering (rule kinds) and management (hypotheses); Texture has only `pSupported` | `choir.Precommit` in the cell, minted by the reducer on the commitment ledger |
| (b) A settler that is not the claimant, grounded in tape evidence | engineering: rules from Freeze, Verify and terminal; Texture: Jev; management and research: an off-computer scorer only | the runtime at the transition that produces the evidence (the rule pattern), writing a resolve record by `system:rule` |
| (c) Settlement grounded in the outcome that matters | **no**: `verify_pass` grounds in management's Verify, which tonight passed two candidates the conductor rejected (§5.2) | new rule kinds settled at the owner or standing-consensus decision and at materialization |
| (d) The desk's next context shows its calibrated record for that kind of decision | **no** for engineering; switched off for the others | an aggregate digest turn plus `choir.Pack().Calibration`, by the ICRL placement rules |
| (e) On-demand retrieval of the desk's own records | no | a read-only REPL verb (proposed `choir.TrackRecord(kind)`), aggregate only |
| (f) The owner sees open, settled and falsified commitments per desk | partial (materiality projection) | a Texture supervision doc built from the ledger; the release record for apply-time claims |

### 3.2 Context design: the line that has to be drawn

The 2026-09-29 boundary keeps scores out of the acting pack. The 2026-10-14
intent says a desk sees its own settled record. precommitment-records.md §4.7
reconciles the two: show observations and discrepancies, keep a scorer's
judgment out. Engineering's rule settlements sit on the safe side of that
line. They are observations from receipts ("the test receipt exited 1"; "the
assignment ended blocked"), not a model's judgment. `commitment_rules.go:67-70`
(R12) already records that a rule settlement carries a discrepancy and an
observation "which the acting pack already shows". The proposal therefore
shows engineering aggregate rule-settled tallies only: n, the share that
held, mean p and Brier per rule kind. It never shows a scorer's verdict
against an item, and it uses the digest's existing guards (n of at least 20,
a held-out 10%, arms). It is still a new desk-facing feedback channel, and
the owner ruled "calibration feedback stays off". Turning it on is the
owner's decision (§6, question 1). Building it behind a switch that defaults
to off is not.

### 3.3 The smallest end-to-end slice (proposed; nothing below is built)

**E-ICRL-0: engineering sees its own say-do record.** This is plumbing
between existing parts. Orange: runtime context; rollback is a setting.

1. **Compute it.** The runtime calls `RulePrecommitSayDo(records,
   "engineering:")`, which exists and has no caller, from the same 30-minute
   snapshot as `calibrationSnapshotFor`. It renders a text in the style of
   `calibration.Render`: per rule kind, "you said 0.88, 0.40 held (n = 25)".
   The held-out rule and the minimum n are those of `calibration.Compute`.
2. **Carry it.**
   - Add an optional `Pack *types.ActingPack` to `capsule.GoEvalRequest`
     (`capsule/types.go:155`).
   - Pass it into the broker's frame (`session_worker.go:227`) and fill it in
     `tools_capsule.go` next to `req.Records` (:631-635).
   - Engineering's `choir.Pack()` then shows its acts and the digest, as the
     prompt already claims.
   - Also add the digest user turn at activation start through the existing
     `calibrationDigestInjector` pattern (`calibration_feedback.go:394-407`).
3. **Gate it.** Use a new setting, `engineering.track_record`, default off.
   It is separate from `calibration.feedback`, so the paper's management
   experiment stays uncontaminated. Arms by assignment thread
   (`calibration.Arm`). Rollback is setting the switch false.
4. **Accept it.** A deployed acceptance on a test computer:
   - seed at least 20 settled rule claims (seven drill assignments at about
     three claims each, about 31 minutes per drill: one night);
   - run one more assignment and read its run events;
   - show the digest turn, a first-cell `choir.Pack().Calibration` that is
     not empty, and the precommit probabilities of that assignment;
   - the receipt is a bundle like the drill's.

   This proves the loop closes. It does not prove learning. That needs the
   H-ICRL arms over many assignments.

**E-ICRL-1: settlement grounded in the outcome that matters.** This writes
ledger records on the self-development path, best effort, behind a switch.
By CLAUDE.md that is red-additive, review tier 1.

5. **Add two rule kinds to the closed vocabulary.**
   - `owner_accepts`: the operation's decision. Accepted settles yes and
     rejected settles no. The record names the decider (owner act or standing
     consensus) and points at the decision event.
   - `applies`: the operation reaches `applied`. Failed, degraded or rolled
     back settles no. Reaching `restored` after `applied` is still yes; a
     later `survives_refresh` kind is out of scope here.
   - They settle at the transitions that already exist: the owner's decision
     (`decideSelfDevelopment`, `internal/agentcore/api_self_development.go:310`;
     whether the standing-consensus poster takes the same path is a
     hypothesis) and the applied
     materialization (`self_development_materializer.go:473`; states at
     `internal/selfdev/operations.go:17-31`).
   - The same R4 one-settlement-id rule applies, and settlement never blocks
     the transition (R6).
   - Engineering's overlay adds the two kinds to its first-cell precommit
     list.
6. **Management's Verify carries a probability.** `choir.Verify` gains an
   optional `pAccept`. The reducer turns it into a rule precommit of kind
   `owner_accepts`, bound to the operation and settled by the same decision.
   Today only engineering may carry a rule (`engineering_precommit_rules.go:69-73`).
   Binding happens in the reducer, so R1 (the reducer stamps ids and clocks)
   still holds. This gives management a say-do record as a verifier, which
   tonight showed is the weak link (§5.2).

**Owner surface.** No new UI. The release record already carries the
precommitments. Two things are proposed:

- Make the supervision Texture doc revise at the decision and at the apply,
  quoting the settled claims. This is the gap in the lane problem record
  `texture-supervision-doc-stops-at-the-decision-boundary-2026-10-24`.
- Expose the say-do view through `GET /api/calibration` beside the digests.

**Not in the slice:**

- in-protocol settlement (F-new-1);
- record query verbs beyond `Pack`;
- retrieval by corrective value;
- the paper's management digest experiment;
- materiality coverage.

Each is named in §6 or left to precommitment-records §8.

### 3.4 Why engineering first, and not management or the conductor

- **The outcome can be checked by rule.** Tests, freezes, decisions and
  applies are receipts. This follows memo P1: rest the claim on classes that
  can be settled objectively.
- **It is already in use.** In tonight's drill, engineering precommitted in
  both rounds without being asked twice (§5.1). The only missing piece is
  the feedback.
- **It is the doctrine's binding constraint.** "Self-development is only
  real when built on precommitment scoring."
- **Its failure modes are already written down** (R1 to R12).
- **Management's hypotheses need an outside scorer** (F-new-1), and the
  owner has kept that loop off.
- **The conductor is an outside agent.** It can use level 2 today with no
  code (§5.2). That is a practice change, not a slice.

---

## 4. Interaction with update architecture

What is settled (owner notes §13, §15, §16;
[selfdev-update-architecture-2026-10-15](../reports/selfdev-update-architecture-2026-10-15.md);
[lu-guest-updates-design-2026-10-14](../reports/lu-guest-updates-design-2026-10-14.md)):

- Only source is published, and the host is the build authority.
- Computers diverge, so a host-built binary for A is not valid for B.
- Apps travel as packages that the host builds once. Core platform changes go
  upstream through the owner's merge and a deploy.
- Platform updates are pulled with a deadline, in rings, with probation and
  rollback.
- S7 packages: the manifest is on the paused branch
  `origin/feat/s7-0-app-packages`, and package presence is a fold of
  `choir.app_package_event` on the tape
  ([app-package-carries-app-desk §1](app-package-carries-app-desk-2026-10-21.md)).

### 4.1 Concrete now (code exists, or tonight's evidence)

- **Release records already carry say-do.** An applied operation's release
  record holds engineering's rule-settled precommitments
  (`release/record.go:54`; `api_release.go:136-177`). Verified. When a
  release is published to the host and another computer considers adopting
  it, the publisher's claims about that change, and how they settled, travel
  with it. That is the §4.6 reputation idea on the one object that already
  moves between computers. No adopter reads them today (hypothesis: no
  reader found).
- **Tonight's update and divergence consequences could all have been
  stated, and rules could settle them.** Each was found by a person reading
  code after the fact:
  1. After the 05:02 platform refresh, a frontend-only Files candidate had to
     build `bin/autoputer` from the old base (`0cbc52edc`): "the running
     binary was built from 824b8410 but the capsule source is pinned to
     0cbc52edc" (apprewrite log, Files).
  2. Applies on one computer do not compound. The Files candidate carried
     the pristine Slides CSS, so approving it would have reverted Slides.
     Problem record on lane `fp0-apprewrite` 6cb4b3da4.
  3. A computer with a layered `bin/autoputer` stops receiving guest fixes
     (`docs/problems/guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md`).

  These are material consequences in §3.1's sense: each would flip an
  approval. A commitment written at apply time could predict them ("after
  this apply, the next candidate's base includes it"; "platform updates
  still reach this computer"). The receipts to settle them exist:
  `source_patch_base_commit` on the next Freeze, and the effective-release
  marker after the next refresh. Proposed rule kinds: `next_base_includes`
  and `survives_refresh`. Concrete in mechanism. Their value is a hypothesis.
- **Rule settlement at a probation hold.** LU3b designs a probation hold
  after a refresh, with the previous image booted once if the new one fails.
  That hold is a settlement: "this computer is healthy N minutes after taking
  build B" settles yes or no from vmctl's health receipt. Writing it as a
  record would give each build a fleet-wide say-do record by ring. Design
  hypothesis: LU3b is not built.

### 4.2 Speculative (no substrate yet)

- **Package subscriptions as standing precommitments.** CLAUDE.md already
  separates a standing intent from its occurrences: an Autopaper schedule
  survives crashes, and each tick is new work. A package subscription fits
  the same shape. The standing record says "versions of P from publisher Q
  install and pass their checks on this computer, p". Each new version's
  install settles one occurrence. The per-(package, publisher) record is a
  query, not a number (§4.6). This needs S7 install events, which exist only
  on a paused branch.
- **A computer learning which optional updates to take.** Management
  decides take or defer for an optional stock update or package version,
  with a precommitment ("taking U leaves my computer healthy and my applied
  changes intact"). Probation and the next Freeze's base settle it. Across
  the fleet, settlements in Store B become the evidence other computers
  read. Because computers diverge, another computer's "U applied cleanly" is
  evidence, not proof, for mine. Reputation would have to be conditioned on
  how close the bases are. That is a research question.
- **The normative limit.** A precommitment is a prediction, never the
  authority (§1.1: authorization is its own step). Host security updates are
  not optional. A computer's well-calibrated "deferring this is fine" must
  never defer one. Precommitments on security updates measure, and policy
  governs. The same holds for the CLAUDE.md rule that always-busy computers
  take a planned, marker-backed refresh.

---

## 5. Evidence from tonight

### 5.1 The self-development drill (staging, 03:02-03:39 UTC, PASS 19/19)

Source: [selfdev-drill-2026-10-24](../evidence/selfdev-drill-2026-10-24.md).
The precommitments were read from the local bundle
`~/.config/choir-qa/traces/fp0-selfdev-1024a/bundle-a1.json` (outside the
repo; run events only, no ledger records).

| Round (assignment) | Claim | Stated p(yes) | Expected settlement by rule (hypothesis) | Brier |
|---|---|---|---|---|
| 1 (`2b214c9c…`, ended blocked) | `tests_pass` | 0.88 | no: no Freeze before the end (`commitment_rules.go:454-468`) | 0.774 |
| 1 | `verify_pass` | 0.80 | no: no completed candidate (:469-473) | 0.640 |
| 1 | `completes` | 0.95 | no: ended blocked (:449-453) | 0.903 |
| 2 (`2e1a17b3…`, completed, froze) | `tests_pass` | 0.80 | yes: the cited test job passed | 0.040 |
| 2 | `completes` | 0.85 | yes | 0.023 |

- **Engineering precommitted unprompted by the drill, in its first cell,
  both rounds.** Mechanism (a) works on the product path. Verified from the
  bundle's tool calls. The settlement records themselves were not read: the
  bundle holds run events, not ledger records. The outcomes in the table are
  what the code would write.
- **Round 2 could not see round 1's settlements.** Its pack was empty (§2.3)
  and engineering has no digest. Round 2 did state lower probabilities (0.80
  and 0.85, against 0.88 and 0.95). If that came from anywhere, it came from
  the re-cast text or the carried patch, not from records. Hypothesis: the
  re-cast was not read for this.
- **Round 1 failed on a say-do gap the rules do not cover.** The desk read
  `choir.Help("Poll")` ("never waits: end your turn") and then polled 30
  times. That is a procedural fidelity failure. The precommitment it would
  have needed, "I will end my turn while a job runs", is not something a
  rule kind can state. It is measurable from the trace: compare Poll calls
  against the job's state. Problem record
  `docs/problems/engineering-busy-polls-its-own-job-2026-10-24.md`.
- **What the loop would add here.** With E-ICRL-0, round 2 would have read
  "you said 0.95 that you complete; 0 of 1 held" in its first cell. That is
  below n = 20, so the real digest would show nothing; by design one round
  teaches nothing. Over a night of assignments it would show something like
  "`completes`: you said 0.9, 0.6 held". That is the claim H-ICRL would test.

### 5.2 The app rewrites on b@c's computer (04:06 onward)

Source: `/Users/wiz/Dev/lanes/_runs/apprewrite/log.md`, read only.

- **The verifier the rules trust was overruled twice.** Slides attempts 1
  and 2 reached "frozen and verified" (Texture revision at 04:15:07 and
  04:20:04). The conductor then rejected both: a stray `.baseline` file and
  nested duplicate divs in the first; a rewrite minified into 21 lines in the
  second. Any `verify_pass` engineering committed would have settled **yes**
  against management's Verify (`rlm_reduce.go:1584-1589`), while the decision
  that mattered was **no**. Settling engineering's record against the
  weakest judge teaches it to satisfy that judge, which is the Goodhart
  shape §1.2 warns about. This is the case for `owner_accepts` (E-ICRL-1
  step 5) and for scoring management's Verify itself (step 6). Whether these
  engineering runs precommitted is unknown: their run events are not in the
  run directory.
- **Engineering's own claims about its candidates.** The combined operation
  (05:28) shows the pattern:
  - Round 1 polled about 60 times, compacted at 122k tokens and Completed
    blocked.
  - Round 2 Completed blocked after 33 seconds "for scope".
  - Both would settle `completes` as no if precommitted.
  - A desk that could see "you said 0.9 that you complete; 0.4 held" might
    commit honestly lower, or escalate earlier. That is a hypothesis H-ICRL
    tests, not a finding.
- **Where the conductor itself could have precommitted, today, with no
  code.**
  - The conductor is an outside Claude Code agent, so level 2 is its
    surface: `choir precommit add` with `--criterion`.
  - Two examples with natural settlers: "approving Files leaves the applied
    Slides intact, p" settles false from the frozen tree's CSS hash; "Slides
    attempt 3 applies and serves the new chunk within 5 minutes, p" settles
    true at 05:04.
  - The conductor's approve and reject decisions are themselves forecasts of
    the owner's judgment. The owner's morning review could settle them.
  - The owner raised the bar at 05:47 ("redesign, not restyle"). By that
    later bar, the approved Slides would be "accepted under the old bar,
    qualified under the new". That is exactly the case for keeping the
    record instead of overwriting it.
- **Texture supervision stops at the decision boundary** (lane problem
  records d7af8770a and ce5eb295a). The doc stayed at "awaiting owner
  decision" through the approval and the apply. Settled `owner_accepts` and
  `applies` records are the ledger events Texture would render. This
  matches CLAUDE.md's "the commitment ledger is the durable substrate ...
  Texture renders that ledger into the owner-facing supervision surface".

---

## 6. Open questions for the owner

1. **May engineering see its own aggregate rule-settled record?** The
   proposal is an aggregate digest only: observations from receipts, no
   scorer, n of at least 20, a held-out 10%, arms, behind a switch that
   defaults to off. This is the 09-29 boundary against the 10-14 intent, and
   the 10-14 ruling "calibration feedback stays off" covers the other desks.
   Does it cover this one?
2. **Is the owner's decision a settler?** The owner's act is not agent
   speech. A rule settlement would record it, with system provenance, as the
   outcome of `owner_accepts`. Under "autonomous first", should the standing
   consensus decision count the same, or be a separate kind?
3. **Should management's Verify carry a probability that the owner's
   decision scores?** This would be the first scored verifier on the
   self-development path.
4. **One track-record view, or three?** Today the level-2 claims, the desk
   digest and engineering's say-do are disjoint. Merge them into one query
   (filter by agent, desk, kind, scorer), or keep them separate until the
   experiments read out?
5. **The paper's H-ICRL experiment.** Run phase 0 now (settle on a
   schedule, switch still off), or keep it parked and spend the design day
   on engineering, where outcomes can be checked by rule?
6. **F-new-1.** Repair settlement inside the protocol (is a peer desk an
   independent settler?), or rely on rules and off-computer scorers for now?
7. **Update and apply consequences as rule kinds** (`next_base_includes`,
   `survives_refresh`). Add them with E-ICRL-1, or after the
   applies-do-not-compound substrate fix?
8. **Should FP0 and conductor agents record their own review and approval
   predictions** with `choir precommit` from today? This is a practice
   change, with no code.

---

## Ceremony

- Mutation class: green. This document only. The slices it proposes are
  orange (E-ICRL-0) and red-additive (E-ICRL-1), and each would be classed
  and reviewed on its own.
- Heresy delta:
  - **discovered:** engineering's prompt claims `choir.Pack` shows its
    settlements, but the capsule frame carries no pack; `RulePrecommitSayDo`
    is uncalled; three disjoint tallies; `verify_pass` grounds in the
    verifier the owner side overrode; precommitment-records §8.1 is stale on
    what is built;
  - **introduced:** none;
  - **repaired:** none.
- Problem documentation first: the empty engineering pack is a new problem.
  It is recorded here, in §2.3. A fix needs its own problem record before
  any code.
