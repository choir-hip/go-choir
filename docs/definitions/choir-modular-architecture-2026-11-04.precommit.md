# Precommitment ledger: modular architecture (opened 2026-11-04 19:13 UTC)

Companion to
[choir-modular-architecture-2026-11-04.md](choir-modular-architecture-2026-11-04.md).
It applies the precommitment-records practice to the mission that builds it:
we score our own predictions as we go, with the same mechanism the mission
gives the desks. The mechanism and scoring rules are in
[docs/precommitment-records.md](../precommitment-records.md), sections 2 and 5.
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
