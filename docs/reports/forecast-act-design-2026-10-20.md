# The forecast act: a separate, rule-settled learning loop (design, 2026-10-20)

Design only: no code changed, no staging state touched. Mutation class of
this document: green; the class of the work it describes is in section 11.

Owner ruling, 2026-10-20 ([owner notes](owner-notes-2026-10-14.md) §27, item
19): "The learning loop: a separate forecast act on scheduled, checkable
events, settled by rules first; a desk sees aggregates of its own record,
not item verdicts; no token-budget learner yet."

Reads behind this: [precommitment records](../precommitment-records.md), the
[ICRL experiment](../archive/reports/icrl-calibration-experiment-2026-10-14.md), the
[three-day letter](choir-last-three-days-2026-10-17.md), the
[op-ed design](op-ed-section-design-2026-10-20.md),
witness (b),
the [source coverage gaps](source-coverage-gap-analysis-2026-10-20.md), the
[L3 station](../archive/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md)
(which this design narrows) and
[the signal is sparse](../signal-is-sparse-not-the-learner-2026-08-01.md),
whose point is that correction must come from outside the learner: here, a
settler the desk does not control, on events it does not influence.

## 1. Summary

Today the paper's desks mostly predict their own desks. A story opens with
hypotheses whose probability is, by prompt, the chance our research desk
will confirm them; 32 of 38 sampled were such claims. Nothing in the ledger
predicts the world and waits for the world to answer, so nothing a desk
could learn from is about the world.

This design adds a **forecast**: a separate act with its own record. It
says a named, scheduled, checkable event will come out a stated way, with a
probability, bound to a calendar entry the system, not the desk, put there.
It is committed before the outcome can be observed, cannot be edited,
cannot be settled by its author, and is settled by a deterministic resolver
over a primary source wherever one exists. Jev settles only what no rule
can. A desk later sees aggregates of its own settled forecasts, never which
one was right.

Recommendation: build in the order calendar, act, witness, rule settler,
owner view, and desk feedback last, after the running calibration
experiment reads out. Start with statistics releases, central bank
decisions and the Taiwan election of 28 November, whose sources are public
and rule-checkable. Stories and op-eds use the same act.

## 2. Current state, with evidence

**What a story's hypothesis is.** Management opens a story by casting to
`texture:new` with hypotheses. Each is a `CastHypothesis`: statement,
probability and, since L3-1 (0fe4cead0), optional `kind` (`assertion` or
`forecast`), `criterion` and `resolves_by`
(`internal/types/commitment.go:337-359`). Admission checks shape
(`checkHypothesisKind`, `internal/store/inbox.go:218`) and time
(`checkHypothesisClock`, `inbox.go:251`): a forecast needs a criterion and a
`resolves_by` after the commit and at most seven days later
(`HypothesisForecastHorizon`, `commitment.go:359`). Kind is optional; a
record without one is legacy.

**Why 32 of 38 were "research will confirm" claims.**
1. The prompt defines the number that way. Management's overlay says a
   hypothesis's probability "is your calibrated chance that independent
   research will confirm it"
   (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:23`). A
   desk told to estimate research confirmation writes what research can
   confirm. The same line offers "forecast" only as an alternative.
2. A hypothesis is a work-routing device: a Cast carries it so the
   recipient knows what the caster believes and how hard to look
   (precommitment records §4.4). Its natural content is "this is true now",
   a verification task.
3. Nothing rewards a dated, checkable event. The L3 sample
   (`start.corrections`) found 6 of 38 forward-looking, 1 dated, 18 of 38 at
   p >= 0.85; all 8 opens after L3-1 deployed were `assertion` (L3
   definition, deployed smoke).

**Who settles, and why it is not independent.** Only the side that did not
open a hypothesis may settle it (`inbox.go:328`, `settleable`): Texture
settles management's, research Texture's. In practice neither can: Texture
has no Report verb, and `ReportPacket` decodes with `DisallowUnknownFields`
into a type with no `settled` field (`internal/yaegikernel/report_packet.go:26`;
ICRL §6, F-new-1). The ICRL experiment settles them off the computer through
the owner route `POST /api/calibration/settlements`
(`internal/agentcore/calibration_api.go`) with a frozen judge prompt. That
measures research, not the world.

**What exists to reuse.**
- Rule settlement. Engineering's `choir.Precommit` carries a rule from a
  closed vocabulary; the runtime settles it from receipts with a `resolve`
  record by scorer `system:rule`, one deterministic id, a not-exists write,
  best effort (`internal/types/commitment_rules.go`, R1-R12;
  `internal/agentcore/engineering_precommit_rules.go:62`). The records are
  ledger-only: no addressee, inbox or wake.
- An aggregate-only digest: per desk, bins, no per-item outcome, a 10
  percent held-out set, a minimum n (`internal/calibration/calibration.go:101,256`;
  `internal/agentcore/calibration_feedback.go:152`), gated by
  `calibration.feedback`, default off (`internal/store/computer_setting.go:36,52`).
  Its claim types are `hypothesis`, `completed`, `citation_support`
  (`internal/types/calibration_digest.go:15`); forecast Brier would pool into
  hypothesis Brier today.
- Source dating fields (`VintagePolicy`, `LookaheadStatus`, `ReleaseDate`,
  `internal/sources/types.go:128-130`); whether any live source fills them
  was not checked.

**What does not exist.** A calendar; a forecast verb; a world-event
resolver; witness (b) (a dedicated key was ruled, §27 item 9; not built); a
commit hash or signed receipt on records (precommitment records §6); any
settled forecast on the paper (the L3-1 smoke found none).

## 3. The forecast act

### 3.1 Why not a hypothesis kind

The kind `forecast` exists and one hypothesis in 38 used it. Growing it
fails three ways: "probability" would carry two definitions in one field,
and the digest pools them; a Cast hypothesis is addressed, so its
counterpart is its settler, the dependence we are removing; and it can be
revised or withdrawn (`CastRevision`, `CastWithdrawal`), where a forecast
must not be editable at all. So a forecast is a ledger-only precommit-kind
record with its own typed body and verb. The hypothesis kind stays in the
type, because committed records must still fold; new Casts stop using it
(question 5).

### 3.2 Schema

One forecast is one binary question: a `CommitmentForecast` body inside the
existing record envelope (emitter, computer, model id, run, subject
references, store-assigned commit time).

| Field | Req. | Meaning |
|---|---|---|
| `claim` | yes | One sentence that will be true or false: "BLS reports September CPI-U year over year at or above 3.2 percent." |
| `event_id` | yes | A calendar entry (section 4) in state `scheduled`. A desk cannot invent an event. |
| `probability` | yes | That the claim is true, in [0.02, 0.98] (the rule-precommit bound, R3). |
| `resolution` | yes | Frozen: `resolver` (closed vocabulary), parameters, `source_class`. Hashed at commit. |
| `resolves_by` | yes | RFC 3339, after the commit, within seven days (the existing clock check), not before the event time. |
| `attached_to` | no | A story (`texture:<doc>`) or an opinion piece (publication id); also a subject reference, from which domain is derived. |
| `reference` | system | Stamped at commit, never by the desk: the resolver kind's base rate, and a market price where a `polymarket` source lists the question. |
| `commit_hash` | system | sha256 of the canonical body, with the first event-head receipt that carried it (precommitment records §6, step 1). |

Resolvers are a closed vocabulary; adding one is a reviewed change, as with
`PrecommitRuleKind`:
- `numeric_release`: a value in a named series, first-release vintage,
  against a threshold or band in stated units. Revisions never reopen it.
- `official_result`: a named outcome in an official decision or result
  document, from an enumerated set (a bank holds, raises or cuts; a bill
  passes; a named candidate leads the certified count).
- `occurrence`: a document of a stated class appears in a named primary
  source by the deadline (a Federal Register entry, an advisory).
- `judged`: no rule fits; Jev and a second model settle a frozen criterion.

Source classes follow the coverage analysis's `source_standing` list (§4
there): `primary_government`, `primary_statistics`, `primary_central_bank`,
`primary_election_authority`, `primary_legislature`, `primary_court`,
`primary_company`, `wire`, `outlet`. A rule resolver accepts only a primary
class; `judged` accepts a primary capture or two captures from distinct
outlets counted by origin. Binary only in v1: a multi-candidate election
becomes one forecast per template threshold, with a per-event cap.

### 3.3 Outcomes

A settlement is `yes`, `no`, `void`, `source_failure` or `unresolvable`.
`void` (event postponed past the deadline or cancelled) and
`source_failure` (the platform could not get or parse the source) are
excluded from scores and counted in coverage: a platform fault must never
read as forecasting error (precommitment records §5.4). `unresolvable`
counts against the writer in strict Brier, beside resolved-only as E6a did.

### 3.4 Who authors, through which surface

The surface is the desk's in-cell `choir` module. Two new verbs, staged in
a cell and folded by the reducer:
- `choir.Calendar(window)`: read-only; entries due within seven days with
  each resolver template and what this desk already forecast.
- `choir.Forecast(spec)`: stages the act. The reducer validates, stamps
  `reference` and commit time, mints the record, and refuses with a message
  naming the field (the L3-1 pattern).

Authors:
- **Management** and its delegated tasks. It already holds `Precommit` and
  reads the stories (`internal/yaegikernel/choir.go:330`).
- **Texture, for an op-ed it is writing**, in the same turn, so a house
  piece cannot print a forecast that was not recorded (op-ed design §4.7).
  Texture's module set (`choir.go:369`) gains the two verbs. A forecast is
  a ledger record, not a document write: Texture remains the sole writer of
  canonical revisions. The op-ed's forecast box is drawn from the records.
- **Research does not.** It has read-only world-evidence authority; keeping
  it out of future claims means the desk that finds evidence has no stake in
  the outcome. Management may Ask it for evidence first.
- **Engineering does not.** It has its own rule-settled precommits.

A forecast is not a Cast: no addressee, inbox sequence or round, and it
wakes no one.

## 4. Where forecasts come from: the calendar

A forecast needs something scheduled to be right or wrong about. The system
maintains a **calendar of scheduled events**, a small shared object kind
(`choir.scheduled_event`; a new kind must be registered or the write gate
refuses it). An entry holds `event_id`, title, domain, `scheduled_at` (UTC,
source time zone kept), the announcing capture, the authority, a
`resolver_template` (applicable resolvers, the series or document that will
settle it, unit, vintage rule) and a state (`scheduled`, `rescheduled`,
`cancelled`, `occurred`, `settled`). Desks read it; only ingestion writes
it; a date change is a new write superseding the old. The announcing
capture needs its own first-seen witness, so a desk cannot claim an event
was scheduled after the fact.

### 4.1 Sources, tied to the coverage analysis

The first calendar is hand-curated, then polled. Rows are the
[coverage analysis](source-coverage-gap-analysis-2026-10-20.md) §5 ranks.

| Calendar | Primary source | Coverage row, wave | Resolver |
|---|---|---|---|
| US statistics (CPI, jobs, GDP, retail sales, energy stocks) | BLS, BEA, Census, EIA schedules and releases | 22, wave 1 feeds. Whether items carry the headline value or only the announcement was not checked; if not, the wave 2 dataset connector is needed | `numeric_release` |
| Central bank decisions | Fed and ECB `official:` registry rows (registered, unscheduled); Taiwan CBC feed is 3.4 MB, over the 2 MiB read cap | Fed, ECB wave 1; CBC row 5, wave 2 | `official_result` |
| Taiwan local elections, 28 November | CEC dataset on data.gov.tw, published about seven days after the vote, so election night itself is not this source | 9, wave 2 | `official_result` |
| Legislative votes | Legislative Yuan API `/meets`, `/votes` | 17, wave 2 (JSON connector) | `official_result` |
| Advisories, rules, filings, revenue | CISA, Federal Register, BIS; TWSE (terms unread); SEC 8-K | 15, 6, 18, 3 | `occurrence`, `numeric_release` |

Court rulings have no schedule and earnings need investor-relations
calendars; both wait. The first three rows suffice for phase 2. Each source
needs its licence read before it is polled (§27 item 11).

### 4.2 What the calendar cannot do

Scheduled events cover macro statistics, elections, legislatures and
central banks, not most stories (a leak, an outage, a merger rumour). A desk
calibrated on releases has not shown it is calibrated on unscheduled
judgment, so the design claims only what it measures; transfer is its own
hypothesis (H4, precommitment records §8.4). A daily tick lists events due
within seven days with no forecast; management forecasts those it has
context for, and leaving the rest is visible in coverage.

## 5. Anti-gaming requirements

Each extends a guard in precommitment records §5.4.

1. **Committed before the outcome is observable.**
   - Clock: `resolves_by` after the commit and within seven days; the
     event's `scheduled_at` later than the commit by a configurable minimum
     lead, checked against the store's commit clock.
   - Evidence order: evidence counts only if **witness (b)** places the
     capture's first write after the forecast, i.e. the forecast is a strict
     ancestor of the anchor head corpusd read before the first insert. A
     capture with no witness, or a publisher time before the commit, is
     inadmissible: timestamps may exclude evidence, never admit it.
     Unprovable order fails closed to `source_failure`. Witness (b) is a hard
     precondition of **scoring**, not of committing.
2. **No edits after commit.** Minted once under a deterministic identity
   with a not-exists condition; no revise or withdraw verb. The reducer
   refuses a second forecast by the same desk on the same event and
   resolution spec; the first is scored. Not updating before the event
   loses information in v1, deliberately: it removes a hedge-by-updating
   route. Only the system marks a forecast `void`, as a settlement.
3. **No settling by the authoring desk.** A forecast has no addressee, so no
   counterpart settles it by Report. Management and research hold `Resolve`
   and `Disagreement`; the reducer must refuse both on any target carrying
   a forecast body, whatever the named `resolver`. Whether it today refuses
   a `Resolve` from a desk other than the named resolver was not verified,
   so the refusal is explicit and tested. Settlement is written only by the
   runtime settler (rules) or a scorer identity with no desk tools.
4. **The event and resolver are not the desk's to define.** Calendar entry
   and template are system-owned; the desk chooses a threshold inside the
   allowed form, not the source, series, vintage or comparator. A settlement
   records resolver kind and version, capture ids and `doc_hash`, the
   extracted value with its matched span, and the comparison; a rerun writes
   nothing.
5. **Selection is the system's.** The count of in-domain calendar events
   not forecast sits beside accuracy, as the op-ed design does for pieces
   without forecasts.
6. **Trivial forecasts earn nothing.** Skill is measured against the
   stamped reference, so "the FOMC will meet" scores about zero; a per-desk
   daily cap (setting, default 10); a forecast whose reference base rate is
   above 0.95 or below 0.05 is flagged `low_information` and kept out of
   headline calibration.
7. **No self-fulfilling events**: the calendar admits none whose outcome is
   the paper's own act. **Scorers are blind**: a Jev question never carries
   the author or the probability, and independent scores stay out of the
   acting context (precommitment records §1.2, §4.7).

## 6. Settlement: rules first, then Jev

**Where it runs.** A settler sweep in the computer's runtime, on the cadence
the source-schedule tick uses, not a new timer. It finds open forecasts
whose event has occurred or whose `resolves_by` has passed, tries to settle
them, is best effort and never blocks another act (R6). It is not a
boot-time resumer: a forecast is a ledger fact about the world, not an
obligation (run, wake, control, directive, assignment, work item), so the
restart contract that closes open obligations does not apply. The sweep is
idempotent (deterministic settlement id, first write wins); a crash
mid-sweep means the next tick does the work. Because this reading touches
CLAUDE.md's restart rule, it goes to the review panel (F11).

**Rules.** For the three rule resolvers the settler reads witnessed captures
of the named primary source made after the commit, through the capture path
the paper already uses, applies the template's pinned extraction, compares,
and writes one `resolve` record by scorer `system:rule:<resolver>` with the
evidence refs and every witness it relied on, so scoring survives a later
chain truncation (L3 definition, `finish.artifact`). Two failure edges are
named: extraction matching the wrong number (pinned series, first-release
vintage, matched span in the record) and a source absent or blocked
(`source_failure`, retried).

**Jev, and a second model, for the rest.** A `judged` question is typed
(`noul`, `choice`, `score`), carries only the frozen claim, criterion and
witnessed captures, and goes to Jev and a model of a different family.
Agreement settles. Disagreement settles nothing: two score records stay, the
forecast stays `unresolved`, and it is listed for audit, not averaged
(precommitment records §5.2). Jev is literal and miscalibrated per type
(§5.3), so a judged settlement feeds calibration and governs nothing. If
judged forecasts mostly need a human, stop allowing them rather than
staffing them.

**Audit.** Weekly, the owner checks a seeded sample of rule settlements
against the primary source. The label scores the settler ("score the
scorer"), and a resolver with a bad audit has its own off switch.

## 7. Calibration feedback: aggregates only

**What a desk may see.** A periodic summary in its prompt, built by the
existing digest (`calibration.Compute`, `Render`) with a new `forecast`
claim type. Per desk, and per domain where n allows: Brier and skill against
the reference; reliability and resolution together, so a desk cannot look
calibrated by becoming vague (ICRL condition 3, P12); mean stated
probability against observed frequency, the direction, a few coarse bins;
n settled, open, void or failed; and the count of in-domain events it did
not forecast. No statement, event id, date or per-item outcome, the
precedent the owner set when the acting pack gained aggregate tallies (ICRL
F-new-2).

**Two leaks to close.** *Differencing:* if the summary updates after every
settlement at small n, a desk can subtract two summaries and read the last
verdict. The block updates only after ten new settlements or a weekly
boundary, whichever is later; nothing shows below n of 20 (ICRL P5); bins
under 5 are omitted; a 10 percent held-out set is excluded. *Domain cuts
that become item lists:* a domain row obeys the same floor.

**Placement and gating.** Same placement as the ICRL digest: a user turn
after history, never the system prompt or tool catalog, so the cached
prefix stays byte-identical (ICRL P1, P2); refresh at most daily.
`calibration.feedback` stays off: the owner kept it off (owner notes §19,
"calibration feedback stays off"), and the ICRL design turns it on only
after phase 0 (instruments only, 48 hours or 300 settled management
hypotheses, whichever is later). This design adds a second condition: the
forecast block is **not shown to any desk** until the H-ICRL experiment on
management hypotheses has read out, because a second feedback stream
mid-experiment would contaminate its arms. Until then the aggregates are
computed for the owner only, in the read-only `GET /api/calibration` view. A
separate setting, `forecast.digest`, default off, gates the desk-facing
block so the two switches cannot be confused.

**A design limit.** ICRL randomises by thread, and its management unit is
the persistent `management:<owner>` desk, one unit, so forecasts written by
it cannot be randomised. If per-event management tasks write them, each is
a thread and `calibration.Arm` applies; pre-register a forecast analogue of
ICRL §3 before the digest is ever on. Otherwise the evidence is a
before-and-after series on one desk, which is weak, and reports must say so.

**Not built.** A token-budget learner, per the owner's ruling; precommitting
the value of a turn (a free-form value has no ground truth); and retrieval
of past misses into context, which waits until one independent loop closes.

## 8. Public display

- **Op-eds** show forecasts as text from the records, labelled "open,
  unaudited" (op-ed design §4.3 point 4), with no score. The count of
  forecasts against pieces with none may show; a count is verifiable.
- **Stories** show no forecast in v1.
- **A public score needs the foundations** (precommitment records §6, step
  2): a commit hash per forecast published when made with a platform-signed
  receipt, witness (b), and platform heads anchored externally so
  backdating is bounded. Until then no Brier, accuracy or ranking appears
  anywhere. After, the first view is a box score with n, settled, void and
  source-failure counts and "n too small" under the sample bar; never one
  trust number, never a ranking of people.

## 9. Phased plan

Each phase has its own stop and starts after the previous one has run on
staging.

0. **Read, and fix the calendar; no code (green).** The letter's first
   station (kind mix, forecast share, who settles), plus 20 to 40
   hand-written calendar entries for the next 30 days from the first three
   sources, each with resolver template, verified primary source and licence
   note. This is the schema's review artifact and counts events per week
   (F13).
1. **The act, no settlement (red, flag off).** The body, the two verbs, the
   calendar kind, a prompt paragraph defining forecast probability
   separately, overlay line 23 revised so Cast hypotheses are assertions
   only, the refusals in 5.2 and 5.3, and `forecast.act`. On for the paper
   only; observe volume, padding, refusals, cost. One prompt edit, so the
   cache cold-starts once.
2. **Witness (b), then the rule settler (red).** The dedicated-key witness
   (W1-W8 in the problem document), then the sweep and three rule resolvers
   on the first three calendars.
3. **Owner view (orange).** Denominators by desk, domain, resolver and
   outcome class; coverage; audit sample; disagreement list.
4. **Judged settlement (orange).** Only if rule coverage is too narrow.
5. **Desk feedback (orange, flag off).** After the H-ICRL readout, a
   pre-registration and the owner's yes.
6. **Public display (orange; red for any printed score).** Op-ed box from
   records; a first box score only after the foundations.

This keeps the owner's order for PSA (scoring first, precommitment records
§8.2) and narrows the first loop to scheduled events. L3's acceptance ("ten
or more forecasts committed before their evidence, half settled by a
settler other than the committing desk") is met by forecast-act records,
not Cast hypotheses of kind `forecast`; repoint its acceptance rows when
phase 1 lands.

## 10. Failure modes, written before any code

- **Padding and cherry-picking (F1, F2).** Probability bound, reference
  skill, `low_information`, daily cap, the not-forecast count.
- **Wrong extraction or revised data (F3).** Pinned series, first-release
  vintage, matched span, weekly audit, per-resolver off switch.
- **Old evidence admitted (F4).** Witness (b), fail closed; W1-W8 apply.
- **The event moves (F5).** Past `resolves_by` is `void`; inside it keeps
  the forecast.
- **Platform fault read as a bad forecast (F6).** `source_failure`, cutoff,
  coverage. Known risks: an incomplete TLS chain (stat.gov.tw), blocked
  agents, feeds over the 2 MiB cap.
- **Leaky summary; calibration by hedging (F7, F8).** Update rule, n floor,
  held-out set; resolution and skill beside reliability.
- **Contaminating the ICRL experiment (F9).** Desk block off until its
  readout; owner view separate.
- **Two meanings of "probability" (F10).** Separate paragraphs and verbs;
  the Cast line stops offering `forecast`.
- **Settler versus the restart rule; cost (F11, F12).** Forecasts are facts,
  not obligations; the sweep is idempotent and not a resumer; panel review.
  A forecast is a model turn: batch per daily tick, ride a call that already
  read the story, record cost per forecast in phase 1.
- **A thin calendar (F13).** Under about 15 events a week from the first
  three sources: say so and stop before building.
- **Over-claiming transfer, stale sources, op-ed collision (F14).** Reports
  say "on scheduled releases" only; the registry freshness gate and a
  licence check at registration (coverage analysis §7 item 6); question 3.

## 11. Mutation classes, protected surfaces, rollback

- **Phase 0, green.** Delete the files.
- **Phase 1, red.** Protected surfaces: the commitment record type (a
  compatible extension of the Cut 2 freeze, needing a freeze-receipt entry),
  the reducer, Texture's module set (not its revision path). Rollback:
  `forecast.act` off; new forecasts are refused, existing records stay.
- **Phase 2, red.** Store B capture writes and host signing (the witness),
  Trace/evidence (settlements). Rollback: resolvers and sweep off by
  setting; the witness is additive and never fails a capture write; minted
  records are never edited, only marked withdrawn by a new record.
- **Phases 3 to 6, orange** (red for any printed score). A read-only API,
  Jev routing, every desk's context, publication. Rollback: revert;
  `forecast.digest` off, live, no deploy; hide the box.

Heresy delta: **discovered**, that the paper has no forecast about the world
and that no hypothesis can settle in protocol (ICRL F-new-1); **introduced**,
none; **repaired**, none yet. Typed conjecture: scheduled-event forecasts
settled by rule give a desk a Brier signal that is independent of the desk
and cheap. Falsified if, after phase 2, fewer than half of committed
forecasts settle by rule, or the weekly audit finds a rule error rate above 5
percent. Smallest observer upgrade: the audit sample.

## 12. Acceptance proof

Deployed proof, in phase order; failure modes first, end-to-end tests, each
leaving a repeatable artifact (CLAUDE.md).

- **Phase 1.** A forecast against a real calendar entry shows in the ledger
  with commit sequence and `commit_hash`, in no inbox, waking no desk. Each
  refusal fires naming its field: unknown event, probability 1.0,
  `resolves_by` past seven days, a second forecast on the same event and
  spec, a `Resolve` from management or research on a forecast, any edit.
  The management system prompt is byte-identical with `forecast.act` off
  and on, apart from the intended paragraph.
- **Phase 2 (the L3 floor, narrowed).** At least ten forecasts committed
  before their events, at least half settled by rule, each settlement
  carrying scorer `system:rule:*`, evidence capture ids with
  first-creation witnesses, the extracted value and span, and the resolver
  version. A settlement with an unwitnessed capture, or a publisher time
  before the commit, is refused and recorded `source_failure`. Rerunning the
  sweep writes nothing; a restart between commit and settlement leaves the
  forecast intact, settled once, and not closed as "interrupted by a
  restart". The owner audits ten rule settlements against the primary
  source with zero mismatches.
- **Phase 5.** The system prompt is byte-identical with the setting off, on
  and control, and on and digest; a control-arm desk cannot read forecast
  outcomes through `choir.Pack()` or `choir.Records()`.

## 13. Questions for the owner

Each is yes or no. My recommendation is yes to all; the reason is given
where it is not obvious.

1. **A new act**, ledger-only, unaddressed and uneditable, not a hypothesis
   kind.
2. **Calendar-bound only**: a forecast must name a system calendar entry.
3. **Op-ed forecasts follow the calendar.** The op-ed design's "every house
   op-ed carries one to three forecasts" becomes "when a calendar event
   fits; otherwise none, and the page shows the count of pieces with none".
   A forced forecast with no checkable event is the padding this design
   exists to avoid.
4. **Authors**: management (and its tasks) and, for op-eds, Texture; not
   research or engineering.
5. **Retire hypothesis kind `forecast` in new Casts** after phase 1 has run
   a week; old records keep folding.
6. **First calendars**: US statistics, Fed and ECB decisions, then the
   Taiwan election of 28 November.
7. **Witness (b) before scoring**: forecasts may be committed earlier, none
   is scored until the dedicated-key witness exists.
8. **One forecast per desk per event**, no update before the event, for v1.
9. **Jev only where no rule fits**, with a second model and disagreement
   kept; built only if phase 2 shows rules too narrow.
10. **Forecast feedback waits for the ICRL readout**, behind its own
    `forecast.digest` setting; summaries update in batches (ten new
    settlements or a week, n floor 20).
11. **No printed score before receipts**: none until commit hashes with
    signed receipts, witness (b) and external anchoring exist.
12. **Stop on a thin calendar**: fewer than about 15 events a week from the
    first three sources means stop and report.
13. **Daily cap of 10 forecasts per desk**, a setting, until cost per
    forecast is measured.
