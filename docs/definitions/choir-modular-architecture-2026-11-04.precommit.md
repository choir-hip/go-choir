# Precommitment ledger: modular architecture (opened 2026-11-04 19:13 UTC)

Companion to
[choir-modular-architecture-2026-11-04.md](choir-modular-architecture-2026-11-04.md).
It applies the precommitment-records practice to the mission that builds it:
we score our own predictions as we go, with the same mechanism the mission
gives the desks. The mechanism and scoring rules are in
[docs/archive/learning/precommitment-records.md](../archive/learning/precommitment-records.md), sections 2 and 5.
The format follows the data architecture ledger
(`choir-data-architecture-selection-2026-10-28.precommit.md`). This file
applies them by hand until desk records carry the work.

Owner, 2026-11-04: "we need to recurse. In a prior mission ... we had a
precommitment ledger. We need that for this too, so we can score ourselves
as we go." Terminology (owner): PSA is the theory and science; precommitment
records are the practice and engineering. This ledger is practice. It is also
evidence for the paper the owner will write at external launch, so records
are committed before their evidence exists and never edited.

## Rules

1. A record is committed before the evidence that resolves it exists, and
   committed text is never edited. A revision is a new record that cites the
   one it revises. Commit times come from `date -u`; the git commit that adds
   a record is its timestamp witness.
2. Every record carries: a claim with its assumptions; a probability, or a
   point estimate with an 80% interval; a falsifier; the resolving
   observation; what will be hard to measure, and the mitigation.
3. The committer does not settle its own records. FP0 (the orchestrating
   agent) commits. The settler is a rule (a number read from an artifact),
   Devin, or the owner, and the settlement cites the artifact. FP0 may
   self-score beside the independent score; self-scores never gate.
4. Scoring: Brier and log score for probabilities, with skill against a stated
   base rate where one exists; coverage and the interval score for 80%
   intervals; log of observed over predicted for point estimates. Calibration
   is reported with sharpness.
5. Resolution classes: confirmed, qualified, contradicted, unresolved,
   interrupted (a crash or platform failure; excluded from calibration).
6. A discrepancy names the assumption that failed, and the learning entry says
   what changes; a later record checks that it did.
7. Re-predict at every phase boundary (roadmap phases 0 to 8), citing what was
   learned.

## Records, round 0 (committed 2026-11-04 19:13 UTC)

Context at commit: the consolidated plan is being written; the gVisor
memory, layering and capsule experiment on choir-agents (x86, systrap,
rootless) is running and has not reported. Earlier x86 numbers: warm desk
cell 1.8x native, allocation loop 2.1x, open+read+close 9x, start about
30 ms. Every record starts unresolved.

### gVisor (settled by rule from the running experiment's report)

**R0-01.** A warm `go build` of a mid-size pure-Go package under runsc
systrap with an in-memory root overlay, over native.
- Estimate: ratio 1.6, 80% interval [1.2, 2.5].
- Grounds: builds are file- and mmap-heavy (9x and 10x per operation) but
  mostly compute; the build cache dominates warm builds.
- Falsifier: above 2.5.
- Hard to measure: the overlay choice changes the answer. Mitigation: settle
  on the in-memory overlay; record the others beside it.

**R0-02.** A cold `go build` of the same package, over native.
- Estimate: ratio 2.0, 80% interval [1.3, 3.5].
- Falsifier: above 3.5.

**R0-03.** The Go toolchain and git run unmodified under runsc rootless
systrap (build, test, status, commit all succeed).
- p = 0.80.
- Falsifier: any of them fails for a sandbox reason.

**R0-04.** gVisor's own rootfs overlay can replace the capsule's overlayfs
and namespace setup well enough that the daemon could drop CAP_SYS_ADMIN for
capsules.
- p = 0.40.
- Assumptions: rootless runsc creates its own user namespace; the capsule's
  writable overlay can be the sandbox's overlay.
- Falsifier: the report shows a capsule need that still requires the
  daemon's mount or namespace privilege.
- Hard to measure: "could drop" is a design judgment. Mitigation: Devin
  settles from the report and `internal/capfloor/capfloor.go`.

**R0-05.** Steady resident memory of one idle runsc sandbox (sentry plus
gofer, workload excluded).
- Estimate: 45 MB, 80% interval [25, 90].
- Falsifier: above 90 MB.
- Hard to measure: shared pages. Mitigation: settle on PSS; record RSS beside.

**R0-06.** Marginal PSS of each additional sandbox at 8 concurrent.
- Estimate: 35 MB, 80% interval [15, 70].

**R0-07.** runsc runs inside a Landlock plus seccomp confinement equivalent to
the desk worker's (order (a) in the experiment), with only additions to the
allowed set.
- p = 0.50.
- Falsifier: it cannot run inside without removing a restriction.

### Plan and review

**R0-08.** Devin's first review of the consolidated plan returns ACCEPT.
- p = 0.40 (base rate this week: the modular design was accepted on its
  second round, the desks design was sent back on its first).
- Settler: the review output.

**R0-09.** Blocking findings in that first review.
- Estimate: 2, 80% interval [0, 5].

### Build

**R0-10.** Phase 1 (PSA-only messaging, raw channels deleted, protocol lock
1.0.0) is on main with CI green, measured from the owner's sign-off.
- Estimate: 1.5 days, 80% interval [0.5, 4].
- Hard to measure: sign-off time. Mitigation: the owner's message timestamp.

**R0-11.** Enforcing the writer gate (observe to enforce) on d@e surfaces at
least one writer not in the custody inventory.
- p = 0.65.
- Grounds: 52 WriterRuntime registrations and an observe-only gate that has
  never refused anything.

**R0-12.** The reader gate on d@e surfaces at least one cross read not among
the 23 inventoried.
- p = 0.75.
- Grounds: the inventory sampled agentcore store calls rather than
  enumerating them.

**R0-13.** At Phase 4 close, the desk-name grep gate reports zero references
outside the kernel's allow list.
- p = 0.75.

**R0-14.** Phases 1 to 6 landed and self-development resumed, measured from
sign-off.
- Estimate: 14 days, 80% interval [7, 30].
- Hard to measure: scope will move. Mitigation: settle on the phase list as
  of sign-off; record scope changes as revisions.

### Trust

**R0-15.** Replaying settled precommitment records from d@e and the arms, the
per-class lower-credible-bound ordering predicts the next contradicted or
fidelity event better than chance.
- Estimate: AUC 0.65, 80% interval [0.52, 0.80].
- Falsifier: AUC below 0.55, or below a success-rate baseline.
- Hard to measure: few settled material events exist today. Mitigation: report
  n; if n is under 30 events, resolve as unresolved, not as a pass.

## Settlements (2026-11-04 ~20:40 UTC; FP0 writes, the settler is the rule reading docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md unless named)

**R0-01, settled contradicted.** Warm `go build` 0.45 s against 0.1 s native: ratio about 4.5, outside [1.2, 2.5]; log ratio ln(4.5/1.6) = +1.03. Assumption that failed: that a warm build is dominated by compute. A 0.1 s warm build is dominated by cache stat and open calls, the 9x-per-operation class.

**R0-02, settled confirmed.** Cold `go build` 16.3 to 17.1 s against 11.1 s: ratio about 1.5, inside [1.3, 3.5]; log ratio ln(1.5/2.0) = -0.29.

**R0-03, settled confirmed.** Go build, test, git status and commit all ran unmodified. p = 0.80; Brier 0.04.

**R0-04, unresolved, for Devin.** The evidence says gVisor's overlay and namespaces replace the capsule's setup, with cgroup limits, CAP_NET_ADMIN for egress, and result extraction open. Whether the daemon "could drop CAP_SYS_ADMIN for capsules" is Devin's judgment against `internal/capfloor/capfloor.go`.

**R0-05, settled confirmed.** Fixed cost of a sandbox about 49 MiB (sentry 37, launcher 8, gofer 7), inside [25, 90]; log ratio ln(49/45) = +0.09.

**R0-06, void as stated.** "Marginal PSS of each additional sandbox" did not say whether the workload counts: 37 MiB with the worker (inside [15, 70]), about 14 MiB without (just outside). The record is unsettleable as written. No replacement is filed; the question it served (per-desk or shared sandbox) is answered by the evidence directly.

**R0-07, settled contradicted.** runsc cannot run inside Landlock (it denies `mount`), and inside the cell seccomp filter only with about 180 added syscalls. p = 0.50; Brier 0.25. The working order is the reverse (runsc outer), which the record did not consider.

Calibration so far: 80% intervals 2 of 3 covered (R0-02, R0-05; R0-01 missed); probabilities R0-03 and R0-07, mean Brier 0.145.

### Learning entries

- **R0-01 (contradicted).** Short operations are syscall-bound even when the job is "compute". Change: a slowdown prediction names the operation's dominant cost class (compute, metadata, IPC) from a native profile before predicting. Checked by: the capsule pilot's predictions.
- **R0-06 (void).** The quantity was underspecified. Change: every point estimate states its unit boundary (what is included and excluded). This is improvement 3 of the ledger review (explicit voids, counted).
- **R0-07 (contradicted).** FP0 predicted one layering order without listing the alternative. Change: when a design choice has a small known set of options, the record covers each option, not the first one considered.

**Correction (2026-11-04 19:35 UTC).** The settlements heading above says "~20:40 UTC"; the settlements were written at about 19:32 UTC (commit 1876a56da is the witness). The heading is left as committed.

## Settlements (2026-11-04, settler: Devin's review output, scratchpad review-plan/devin.out, plan at 611b526af)

**R0-08, settled confirmed.** Devin's first review of the consolidated plan returned ACCEPT. p = 0.40; Brier 0.36; log score ln(0.40) = -0.92. The base rate the record cited (one accept, one send-back this week) undersold a plan that had already absorbed two earlier review rounds on its component designs.

**R0-09, settled confirmed.** Zero blocking findings (seven non-blocking amendments), inside [0, 5]; the point estimate of 2 was high.

Calibration so far: 80% intervals 3 of 4 covered (R0-02, R0-05, R0-09; R0-01 missed); probabilities R0-03, R0-07, R0-08, mean Brier 0.22.

### Learning entry

- **R0-08 (confirmed at 0.40).** A plan assembled from designs that each passed review carries their acceptance forward. Change: a review-outcome prediction states how many earlier rounds the reviewed content already passed and conditions on it. Checked by: the goal file's review record in round 0b.

## Records, round 0b (committed 2026-11-04, before the goal-file review and before Phase 1)

Format change from round 0 (the ledger review lessons): each record names its
materiality (high, medium, low: how much a wrong answer would change what we
do), its trigger (the event that makes it settleable), and its settler. A
record whose premise dies is voided explicitly, with the reason. Point
estimates state their unit boundary. Where a choice has a small known set of
options, the record covers each (R0-07 lesson).

**R0b-01.** Devin's review of the rewritten goal file returns ACCEPT.
- p = 0.70. Grounds: the plan it points to was accepted and absorbed every amendment (R0-08 lesson: condition on rounds already passed); the goal file itself is new text, never reviewed.
- Materiality: low. Trigger: the review output. Settler: the review output.

**R0b-02.** Phase 1 (MA1.1 to MA1.7) is on main with CI green, from the owner's MA0.5 sign-off.
- Estimate: 1.5 days, 80% interval [0.5, 3]. Unit: wall-clock from the sign-off message to the CI-green commit that closes the last Phase 1 slice. Grounds: base rate, FP0 timelines ran about 10x pessimistic on the data system mission; R0-10 (filed before the plan grew MA1.5 to MA1.7) said 1.5 [0.5, 4].
- Materiality: medium (sequences the owner's calendar). Trigger: the closing CI run. Settler: rule (timestamps).

**R0b-03.** MA1.2's differential (the manifest-derived resolver against the old route switch on the same inputs) finds at least one route the old switch carried and the plan's list of non-manifest rules does not name.
- p = 0.55. Grounds: Devin found four unlisted rules by reading; reading misses what a differential sees.
- Materiality: high (a missed route silently drops messages). Trigger: the differential test's first run. Settler: rule (the test's output).

**R0b-04.** MA1.1 deletes at least 600 net lines of Go (insertions minus deletions, excluding tests).
- Estimate: 900 net deleted, 80% interval [500, 1600]. Unit: `git diff --numstat` over `*.go` not `*_test.go`, the MA1.1 commits only.
- Materiality: low. Trigger: MA1.1 landed. Settler: rule.

**R0b-05.** The first d@e run after MA1.3 lands produces a request record carrying `p_success` and provenance that reads back through the product API without a code fix.
- p = 0.60. Grounds: new tape kind plus a read path; first deploys of new record kinds needed a fix about half the time in the data system mission.
- Materiality: medium. Trigger: MA1.3's deployed acceptance. Settler: rule (the API response).

**R0b-06.** Under the asynchronous settler (MA1.5), a research fan-out on d@e completes with the judge stalled, and the judge's settlement arrives later and is recorded.
- p = 0.75.
- Materiality: high (fate-sharing between research and the judge would stall the paper). Trigger: MA1.5's stalled-judge test and its d@e acceptance. Settler: rule.

**R0b-07.** Per-run inert sub-calls (MA1.6 detector) on d@e's first ten research runs after MA1.6.
- Estimate: 20% of sub-calls inert, 80% interval [5%, 45%]. Unit: sub-RLM casts the detector marks inert, over all sub-RLM casts, in those ten runs.
- Materiality: medium (tells us whether inertness is a tuning problem or a design problem). Trigger: ten research runs completed after MA1.6. Settler: rule (the detector's counts).

**R0b-08.** Phase 1 needs at least one rollback or hotfix on staging (ring-expedited releases included).
- p = 0.50.
- Materiality: medium (calibrates the expedite policy). Trigger: Phase 1 closed. Settler: rule (deploy history).

**R0b-01, settled confirmed.** Devin's goal-file review returned ACCEPT, 0 blocking findings, 8 non-blocking amendments (folded). p = 0.70; Brier 0.09.

## Settlements (2026-11-04 ~22:40 UTC; settler: rule)

**R0b-04, settled contradicted (interval).** MA1.1 deleted 1632 net non-test Go lines (61 added, 1693 deleted; `git diff --numstat` over `*.go` not `*_test.go`), outside [500, 1600]; log ratio ln(1632/900) = +0.60. Assumption that failed: the slice's own list was the whole deletion; the receive path pulled orphans with it (lifecycle message intents, receipt drains, the emit-notice dedupe).

**R0b-03, settled qualified (settler: the MA1.2 differential, commit ab8d3699c body).** The differential over 9996 inputs found no route the old switch carried outside the plan's list (the claim as worded: contradicted, p 0.55, Brier 0.30). It did surface rules the list did not name: subject scoping, a management task needs a subject on its sender id, the dual spelling of management-task, and unroutable addresses falling through to the packet path. Learning: a record about "routes" should have said "routes or routing rules"; the unit boundary was again underspecified (R0-06 lesson, second instance).

**R0b-05, settled confirmed (2026-11-05 ~01:25 UTC; settler: rule, the API response in docs/evidence/ma1-3-de-request-record-2026-11-05.json).** The first d@e run after MA1.3 deployed (e385fd349; trajectory f0044974, settled) wrote a Texture Cast to research carrying p_success 0.78, weight 0.62, settlement recipient, a frozen materiality procedure (request.cast.v1, I1, hashed) and reducer-stamped provenance with origin desk_run, read back through GET /api/inboxes/research:<doc>/records with no code fix. p = 0.60; Brier 0.16. A CI-only test fix (e385fd349, a Linux test still calling a verb engineering lacks) preceded the deploy; it touched no read path.

**R0b-08, settled confirmed (2026-11-05 ~04:10 UTC; settler: rule, deploy history).** Phase 1 took three hotfixes on staging before it closed, all ring-expedited: 97018316f (an expired CI switch failed every commit at the date rollover), e385fd349 (a Linux-only capsule-broker test called a verb engineering lacks), 897662321 (a timer-synchronised fake judge flaked under -race). No rollback. The claim is decided before the Phase 1 close because it is "at least one". p = 0.50; Brier 0.25. Learning: two of the three were Mac-only builder testing; the Linux vet before every push is now standing practice.

**R0b-07, unresolved (missing oracle).** No sub-RLM cast has been written on d@e since MA1.6 deployed: GET /api/subcalls returns an empty ledger, because no prompt teaches choir.Research and Texture fans out by desk-to-desk Casts instead (ledger 2026-11-05T03:45Z). The detector's counts on those Casts are the Phase 1 baseline below, not this record's unit.

## Phase 1 inertness baseline (2026-11-05 ~04:10 UTC; d@e, GET /api/subcalls, before 222d946f8 shows desks their own count)

Typed precommitments flagged inert, by desk: Texture 44 of 44 (two prompt versions, 5 and 39), management 1 of 1, engineering 1 of 1; every flag is no_typed_question. Delegation Casts: Texture 89, management 42, none marked inert, though the no_typed_question flag fires on them too (Texture 91 flags, management 25). The legacy "agent" desk shows 1024 precommitments, none inert, model none. Judge agreement sampled 0. Read: on the desk-to-desk path every typed request is inert by the rule detector, which is a prompt and protocol fact (the desks never state a typed question), not a tuning margin; the delegation flag-versus-inert split is by design: a delegation is inert only when its weight is below the floor (internal/types/inert_rates.go:136), while by_flag tallies every flag, so no Cast tonight carried a weight under the floor. Raw: scratchpad subcalls-de.json, not committed.

**R0b-02, settled contradicted (interval) (2026-11-05 04:31 UTC; settler: rule, timestamps).** Phase 1 closed with MA1.4 (3ff42209e, 4b8cf26ad; CI 41967088720 green at 04:31:14Z, staging serves 4b8cf26ad). From the owner's sign-off (a209cb739, 21:53:05Z) that is 6 h 38 min, 0.28 days, below the interval [0.5, 3]; log ratio ln(0.28/1.5) = -1.69. Phase 2 (MA2.1 to MA2.5) closed earlier in the same span. Assumption that failed: the base rate carried human-gated review latency; with Sonnet builders in parallel, review concurrent with landing, and runtime checks as the verifier, the critical path was CI and deploy time (about 12 to 15 minutes a slice) plus three review rounds on MA1.4. Learning for round 1: estimate Phase 3 from slices times the CI-plus-deploy cycle plus review rounds, not from the human base rate; MA1.4's three Devin rounds (one mechanism) are the expensive tail.

## Round 1: Phase 3, custody and labels (committed 2026-11-05 04:45 UTC, before MA3.1 starts)

Learned from rounds 0 and 0b, and applied here. Durations come from the measured cycle (one slice is about 12 to 15 minutes of CI and deploy, a builder is 10 to 20 minutes, and a review round is about 10 minutes) and not from the human base rate (R0b-02, log ratio -1.69). The unit is spelled out where "routes" against "routing rules" bit before (R0b-03, R0-06). Review rounds cluster on one mechanism (MA1.4 took three). R0-11 and R0-12 stand and are not restated.

**R1-01.** Phase 3 (MA3.1 to MA3.10) is on main with CI green, measured from the commit of this round.
- Estimate: 1.2 days, 80% interval [0.4, 3]. Unit: wall-clock from this round's commit to the CI-green commit that closes the last Phase 3 slice. Grounds: ten slices, eight of them red tier 2 (two seats), against Phase 1's seven slices in 6.6 hours. MA3.2 waits on a recorded d@e day of zero counters for built-in kinds, which is a floor of about 24 hours unless the counters are read as zero sooner on a shorter recorded window that the slice states.
- Falsifier: outside the interval. Hard to measure: the MA3.2 day. Mitigation: the slice records its window's start and end.
- Materiality: medium. Settler: rule (timestamps).

**R1-02.** MA3.1's registry load finds at least one kind claimed by two manifests or by none (a load failure the slice must resolve) on its first run.
- p = 0.70. Grounds: 52 WriterRuntime lines, OwnedKinds declared only for Texture and research evidence today, and service writers (corpusd, autoradio, store) with no manifest.
- Falsifier: the first load passes clean. Settler: rule (the first test run's output, cited in the slice's commit).

**R1-03.** At least one Phase 3 slice ships a fix after deploy that d@e acceptance found, not CI or review.
- p = 0.55. Grounds: Phase 1's d@e runs found the stranded round (old) and the unexercised sub-call path, but no fix-after-deploy for a new slice; custody enforcement touches paths CI fixtures do not drive.
- Falsifier: no post-deploy fix in Phase 3. Settler: rule (commit history: a fix commit citing a d@e trajectory).

**R1-04.** Total Devin and DeepSeek review rounds across Phase 3 slices.
- Estimate: 18, 80% interval [12, 28]. Unit: completed review outputs (one seat on one frozen candidate is one round), Phase 3 slices only. Grounds: Phase 1 and 2 took about 22 over twelve slices with a mix of tiers; Phase 3 is mostly tier 2.
- Settler: rule (count of review-*/ outputs named in the ledger).

**R1-05.** After MA3.3, at least one existing desk behaviour on d@e (Texture revise, research, management intake, engineering apply) fails until a custody resolution (MA3.7) or a lease (MA3.4) lands.
- p = 0.60. Grounds: the plan sequences MA3.7 after MA3.3 and MA3.4; services convert in the order of section 8.4, so the reader gate may refuse a path whose resolution has not landed. Hard to measure: an unexercised path refuses silently. Mitigation: the reader gate's refusal counter, read through the product API, over the acceptance runs.
- Falsifier: every acceptance run after MA3.3 settles with the refusal counter at zero for desk runs. Settler: rule (counter and trajectory states).

**R1-06.** MA3.5's labels: a d@e run that reads an owner-private capture carries the label into every act it commits, on the first deployed run, with no fix.
- p = 0.50. Grounds: R0b-05 read back first time (new record field), but labels propagate through a per-run accumulator across cells, a new mechanism with more seams.
- Settler: rule (the run's records read through the product API).

**R1-07.** The judge settles at least one desk-to-desk Cast on d@e during Phase 3.
- p = 0.25. Grounds: today the judge only settles sub-call Casts and none are written on d@e; this needs owner question (d) answered or a prompt that teaches choir.Research. Recorded to keep the missing oracle visible.
- Settler: rule (GET /api/subcalls settlement_by_class non-empty).

**R0-12, settled confirmed (2026-11-05 ~13:26 UTC; settler: rule, GET /api/runtime/observability on d@e).** After MA3.3 deployed (24a3ad5c2 on d@e from 13:26Z) the reader gate counted `choir.computer_setting|desk:texture|wrong_reader` 6: Texture reads computer settings, which section 8.2 gives to the shell and the owner, and which is not among the 23 inventoried cross reads. The MA3.3 suites had found the same read at test level (and management and research reading settings, and Texture reading content_item via sourceEntityFromEvidenceRef). p = 0.75; Brier 0.06. Learning: the inventory sampled call sites by desk; settings reads sit in shared helpers (`textureSettingOn`, `computerSettingOn`) that no desk-scoped sample reaches. Undeclared readers on d@e (files 28, settings 2, source schedules 2) are a separate class, reads with no actor, for MA3.7.
