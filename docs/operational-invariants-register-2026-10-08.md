# Operational Invariants Register (draft)

Date: 2026-10-08. Status: **draft for owner review** — proposed as a new
operational invariant class beside the doctrine's derived semantic invariants
(`docs/choir-doctrine.md` § Derived Architectural Invariants). Not doctrine
until promoted. Doctrine wins on conflict.

## Why this exists

The doctrine states Choir's **semantic** invariants precisely (I1…: writer
classes, desk authority, no parent/child control). Its **operational**
invariants — liveness, bounded recovery, bounded resources, deploy identity,
attributable failure — were never written down. They were discovered one
outage at a time: 138 problem docs since July. Almost none of those docs
say the tape, the projection model or the desk ontology is wrong; they say an
implied operational property had no owner. This register names those
properties, their enforcing component, their alarm and their proof, so each
mission is written against an invariant instead of a symptom.

## Method and evidence

- Seven parallel triage agents read every `docs/problems/*.md` in full and
  recorded cluster, revealed invariant, status, enforcer and fix refs:
  [`evidence/problem-doc-triage-2026-10-08.jsonl`](evidence/problem-doc-triage-2026-10-08.jsonl).
- **Status reflects doc text, which is often stale.** Spot checks found fixes
  recorded in commits, `ACTIVE.md` or station reports but never back-written
  to the problem doc (e.g. `sa2-appendevent-unbounded-scan` fixed in
  `475902d7`; `smg-management-open-invalid-transition` fixed and
  deployed-verified in `e9cd9fed`; `s0-tap-egress-unfiltered` closed by S1a).
  Of 85 docs triaged open/unclear, 36 are cited by non-docs commits:
  [`evidence/problem-doc-open-xref-2026-10-08.tsv`](evidence/problem-doc-open-xref-2026-10-08.tsv).
  Treat per-doc status as an upper bound on openness until reconciled (P1).
- Doc-level counts by triage: liveness 41, recovery 24, release 20,
  resources 17, contracts 14, selfdev 9, security 8, observability 4,
  worldwire 1. Status as written: 80 open, 35 fixed-unverified,
  18 fixed-verified, 5 unclear.

## Columns

- **Enforcer:** the component that makes the invariant hold by construction,
  or `unowned`. A pile of per-path patches is `fragmented`, not owned.
- **Alarm:** what pages before the invariant is violated. Most are `none`.
- **Proof:** the staging/disposable acceptance that demonstrates it.
- **Goal:** 1 = stable automatic computer, 2 = supervised self-development,
  3 = World Wire. Most operational invariants serve all three; listed is
  where they first bind.

## Liveness — durable work always progresses or ends visibly

**O1 Obligation terminality.** Every durable obligation (wake, control
packet, work item, directive, desk update, assignment) has exactly one live
driver and reaches a recorded terminal fate — consumed, discharged, refused,
or visibly dead-lettered — within a bounded number of attempts. Processing an
obligation without changing its state is forbidden.
Evidence (~25 docs): `sa1-wake-outbox-rearm-storm`,
`s0m-management-live-occurrence-storm`, `texture-desk-activation-contract-respawn-loop`,
`m11-engineering-desk-deferral`, `m11-texture-required-write-loop-starves-selfdev`,
`passivated-run-wake-consumed-before-reactivation`, `sa-management-mint-no-start-slot-deadlock`,
`texture-incorporate-deadlock-settled-producer`, `engineering-run-death-leaves-bound-assignment`,
`clustering-assessment-engineering-assignment-stalls`.
Enforcer: **fragmented** — many per-path fixes in `internal/store/lifecycle*`;
`continuous-texture-supervision-implementation-inventory` already names the
target ("one exact authority per item"). Alarm: none. Goal: 1.
**This is the largest unowned invariant and the top architecture item.**

**O2 Tape-derived continuation.** Every continuation is derivable from the
canonical tape, never from process-local timers, in-memory wakes or
out-of-band channels; boot restores state and re-derives obligations without
starting in-flight work.
Evidence: `root-cause-wrong-path-cluster`, `kernel-cutover-wake-gap-analysis`,
`s0m-desk-run-dispatch-stall`. Enforcer: unowned. Alarm: none. Goal: 1.

**O3 Reject at commit, never dead-letter silently.** An addressed message,
directive or report that cannot be delivered is refused at commit; one
undeliverable item never poisons a listing or page for its consumer.
Evidence: `s0m-channel-mail-texture-dead-letter`, `s0m-directive-engineering-desk`,
`sa-delegated-report-poisons-management-listing`, `s0m-report-desk-target-resolution`.
Enforcer: fragmented. Alarm: none. Goal: 1.

**O4 One active run per agent.** Concurrent activations of one agent converge
to a single active run; a terminal run is never an agent's ActiveRunID.
Evidence: `smg-management-open-invalid-transition`,
`s0m-freed-control-stale-activerunid-blocks-rebind`. Enforcer: partial
(`e9cd9fed`, S0m fixes). Alarm: none. Goal: 1.

**O5 Never interrupt live work.** A computer with in-flight runs or owed
mutations is never hibernated or reclaimed.
Evidence: `vmctl-idle-sweep-hibernates-busy-guest`,
`vmctl-pressure-reclaim-mid-respawn-gap`. Enforcer: unowned (doc text).
Alarm: none. Goal: 1.

## Recovery — any computer reaches its head, boundedly and observably

**O6 Bounded recovery.** Reaching the canonical head replays at most
`MaxRecoveryTailEvents`; bases are refreshed by cadence; over-bound starts are
refused with a typed, durable condition and a repair job.
Evidence: `sa-projection-base-watermark-never-refreshed`. Enforcer:
`cmd/checkpointd` + `internal/vmctl/recovery_admission.go` (2026-10-08).
Alarm: job alerts `warning_tail`/`urgent_tail`/`repeated_failure` (read-time
only; no pager). Proof: staging cadence + incremental + retained resume
done; admission refusal demo pending. Goal: 1.

**O7 History-independent cost.** Boot, recovery and per-write paths cost in
proportion to pending work, never total history: indexed pending reads only,
no whole-graph, body or tape scans.
Evidence (~9): `held-computer-boot-terminal-outcome-scan-crash`,
`held-computer-boot-work-item-sweep-snapshot-crash`,
`held-computer-super-rewarm-*` (×4), `s0-vocab-rescan-fires-on-any-replay`,
`sa2-appendevent-unbounded-scan`. Enforcer: fragmented (per-path fixes).
Alarm: none. Proof: none general — a boot-cost budget test on an owner-scale
store would make this enforceable. Goal: 1.

**O8 Readiness is earned.** A computer reports active only when genesis
exists, credentials verify against the current signer, the guest serves, a
recoverable base exists and the frontend baseline is staged; one failed probe
never degrades it.
Evidence: `s0b-registration-computer-missing-genesis`, `vm-restart-no-rebind-8085`,
`genesis-computer-surface-underivable-spa`, `candidate-realization-readiness-kill-loop`,
`held-computer-single-health-fail-degrades`, `fresh-computer-genesis-missing-prompt-bar-500`.
Enforcer: partial. Alarm: none. Goal: 1.

**O9 Every start and apply terminates observably.** A start, resume,
platform update or release apply reaches running, a typed refusal or a
recorded failure — never hangs, wedges a pending transition, or crash-loops.
Held is a benign state.
Evidence: `retained-computer-lifecycle-start-timeout`,
`held-computer-boot-crash-loop-and-resolve-race`, `s2-postswap-restart-loop-kills-vm`,
`s2-refused-apply-wedges-pending-transition`, `platform-update-stranded-tail-and-baseless-checkpoint`,
`s2-stranded-retired-realization-permanent-wedge`. Enforcer: partial (S2,
typed admission). Alarm: none. Goal: 1.

## Placement — a realization is a disposable cache

**O21 A realization holds no unique state.** Every piece of a computer's
persistent state has exactly one durable home outside any realization — tape
(what happened), content-addressed store (bytes), escrow (keys), projections
(derived) — and a realization is a cache built from them. Recovery, resume,
host move, hosted↔desktop placement and forks are one operation. Owner
ratified the principle 2026-10-08; it restates the
[computer ontology](computer-ontology.md) ("a realization may be replaced
without changing ComputerID") as an enforceable property.
Evidence: `fresh-realization-missing-privacy-key-blind-boot` (privacy key
only on the realization disk), `sa-projection-base-watermark-never-refreshed`
(local store was the only fast restore path). Inventory and findings:
[state-homes-inventory-2026-10-08.md](state-homes-inventory-2026-10-08.md)
(privacy key, capsule artifacts, 15-minute file window; updater `current` and
signer trust unverified). Enforcer: unowned. Alarm: none. Proof: lose-the-disk
disposable — destroy the data disk, realize again, identical effective head,
witness, files, artifacts and release with no operator step. Goal: 1 (and the
desktop app's hosted↔local move).

## Resources — everything that grows is bounded and owned

**O10 Declared bound per growth surface.** Every store, journal, image,
corpus, scratch directory and outbox has a declared bound and a reachable,
liveness-checked reaper that can actually reclaim.
Evidence: `s0-storage-lifecycle-gaps`, `guest-dolt-journal-and-host-image-leak`,
`world-wire-corpus-resource-burn`, `node-b-deploy-disk-headroom`,
`held-computer-persistent-image-critically-full`, `s0-gc-og-wrong-store-deletion`.
Enforcer: partial — artifact GC exists but runs **dry-run** on Node B;
checkpoint scratch unswept. Alarm: deploy disk preflight only. Goal: 1, 3.

**O11 Memory budgets; retryable work dies first.** Every process and guest
has a memory budget; under host pressure the kernel kills retryable workers
before serving stores or guest VMs.
Evidence: `platform-dolt-oom-realization-cluster`, `guest-vm-embedded-dolt-memory-starved`,
`texture-revision-read-flap-during-oom`. Enforcer: partial (corpus-dolt
cgroup caps; checkpointd `OOMScoreAdjust=1000`). Alarm: none. Goal: 1.

**O12 No lock across I/O or long work.** Control-plane and store locks are
never held across network calls, exec, boot waits or sweeps; reads never queue
behind whole-computer scans.
Evidence: `s0-fetch-guest-timeline-deadlock`, `texture-latency-layered-evidence`
(engine mutex), `gc-sweep-holds-service-write-lock` (2026-10-08 — a
regression introduced by the recovery work, failing fresh boots during
hourly GC). Enforcer: **unowned** — no lint, test or review rule; the latest
instance shipped through unit tests and an 8-model review. Alarm: none.
Goal: 1.

## Release — what runs is known, bound and reversible

**O13 Deploy identity.** "Deployed" means every component — host service,
guest runtime, frontend, oneshot worker — reports the deployed commit; health
gates are commit-bound; deploy decisions diff deployed state against main.
Evidence: `app-layer-push-health-gate-not-commit-bound`,
`s0m-guest-runtime-deploy-gap`, `s0-deploy-refresh-skips-autoputer-internals`,
`ci-docs-only-head-strands-runtime-deploy`, `deploy-refresh-expected-commit-pointer-followers`.
Enforcer: CI deploy job (partial; checkpointd manifest probe added
`11ee8b50`). Alarm: deploy failure. Goal: 1.

**O14 Release integrity.** A release's declared commit equals its binary;
state-schema compatibility is checked before mutation; the exec pointer swaps
atomically; rollback reverts the executed binary.
Evidence: `s2-release-provenance-unbound`, `s2-release-state-compat-gate-missing`,
`s2-layering-entrypoint-global-not-atomic`, `s2-runtime-exec-still-baseline`.
Enforcer: S2 (closed 2026-10-05). Proof: S2 deployed matrix. Goal: 2.

## Security — authority is bound, never asserted

**O15 Transport-bound authority.** Internal endpoints authorize by
transport-bound identity, never network position or a self-asserted header.
Evidence: `s0-guest-reaches-host-internal-authority` (fixed),
`s0-internal-surface-forgeable-caller` (open per doc — `X-Internal-Caller`
is still the host-internal convention). Enforcer: partial (S1a). Goal: 1.

**O16 Credentials never in readable channels.** Gateway and runtime
credentials never appear in kernel command lines, launch configs or anywhere
guest child processes can read.
Evidence: `s0-gateway-token-on-kernel-cmdline`, `s1-gateway-token-on-kernel-cmdline`,
`s1-non-root-runtime-child-uid-boundary`. Enforcer: open (S1 remainder).
Goal: 1.

**O17 Capsule confinement.** Capsule egress is policy-mediated and recorded;
a capsule's authority (package allowlist, verbs) is broker-owned and bound to
its activation, never widened by model-authored code.
Evidence: `s0-tap-egress-unfiltered`, `capsule-go-eval-security`,
`capsule-teardown-cgroup-not-empty`. Enforcer: partial (S1a network
isolation). Goal: 2.

## Self-development — every change episode ends legibly

**O18 Self-development operations terminate with an observable result.** A
self-development operation ends terminal or refused once its turn ends; a
"completed" implementation implies an observable subject change; owed
verification stays openable; acceptance witnesses drive only owner-reachable
paths; source reaches the builder as a verifiable patch.
Evidence: `s0-selfdev-executing-wedge`, `m11-impl-completed-no-subject-change`,
`engineering-verification-chain-dead`, `m11-probe-oracle-route`,
`m11-recast-desk-terminal-report-uncommittable`, `s2-selfdev-runtime-changes-binary-carried`.
Enforcer: partial. Goal: 2.

## Contracts — the model-facing surface is stable and self-describing

**O19 Self-describing verb contracts.** Every verb a desk is told to call has
its exact signature in its overlay; decode errors enumerate valid fields;
deterministic rejections return for re-composition, never identical replay.
This is what lets stronger models drop in without core change.
Evidence: `s0m-report-verb-signature-undocumented`,
`texture-desk-applytexture-contract-failure`, `texture-desk-decision-kind-wait-rejected`,
`m11-tools-actuator-strands-choir-required-desk`, `s0m-desk-no-authoring-act`.
Enforcer: unowned (prompt text). Goal: 1, 2.

## Observability — every failure is attributable from the host

**O20 Attributable failure.** Guest serial and service stderr are captured to
bounded, host-readable sinks; every refusal returns its specific reason;
post-commit audit failure never changes a committed result.
Evidence: `guest-stderr-unreachable-after-cold-boot`, `s0-guest-serial-not-captured`,
`s2-updater-refusal-reason-observability`, `texture-audit-post-commit-boundary`.
Enforcer: partial. Goal: 1.

## Process invariants (SDLC)

**P1 Problem docs carry a lifecycle.** A fix commit updates its problem doc's
status (open → fixed-unverified → fixed-verified/superseded) in the same
landing. Today 36 of 85 "open" docs are cited by fix commits; status lives in
`ACTIVE.md` and reports instead.

**P2 Lock scope is reviewed as an invariant.** Any change that adds or widens
a mutex states what it protects and what else contends (O12). The 2026-10-08
regression passed unit tests and an 8-model frozen-candidate review.

## What this says about the three goals

- **Goal 1 (stable automatic computer)** is mostly O1, O2, O7, O12 — all
  unowned or fragmented — plus finishing O6, O8, O9, O10, O11. O1 is a single
  architectural owner for durable-obligation fate; O7 and O12 want an
  enforceable budget test and a lock-scope rule rather than more patches.
- **Goal 2 (self-development)** additionally needs O18 and O19, and leans on
  O14 and O17, which S1a/S2 largely delivered.
- **Goal 3 (World Wire)** adds O10 for the corpus and the store separation
  in `world-wire-rearchitecture-2026-10-05.md`; the rest is goals 1 and 2
  applied to a platform-owned computer.

## Next steps

1. Owner review of this list: merge, split, rename, and decide promotion into
   doctrine.
2. Reconcile per-doc status (P1) against commits and station reports;
   back-write status into each problem doc.
3. For each unowned/fragmented invariant, name the owning component, its
   alarm and its staging proof — those become the missions.
