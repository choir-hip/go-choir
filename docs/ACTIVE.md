# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Completed Definition — Private Programmable Go Actor Kernel

[`definitions/choir-private-go-actor-kernel-2026-08-12.md`](definitions/choir-private-go-actor-kernel-2026-08-12.md)
completed 2026-08-27 (deployed commit `53f80af4`). It establishes private,
interpreted Go activations via a process-per-activation Yaegi sidecar inside
disposable guest-local capsules with unified Bash/Go broker routing, opaque
handles, durable continuity across forced activation death, immutable Texture
transclusion of host-selected salient receipts, and verified genesis surface derivability.
## Completed Definition — Durable Substrate Overhauls

[`definitions/choir-durable-substrate-overhauls-2026-08-23.md`](definitions/choir-durable-substrate-overhauls-2026-08-23.md)
completed 2026-08-26 (commits `f0e68b0a`..`e65e91c4`). All four tracks are
implemented and verified: (1) Track K key escrow with 2-of-N quorum gate and
WebAuthn PRF wrapping (deployed & proven on staging); (2) Track F encrypted 4MiB
chunk file-CAS, Merkle manifests, tape citations (`file_root_committed`), sync barrier,
atomic boot hydration, and ProjectionBase materializer; (3) Track M host fsync'd MTA
spool, async LMTP drain, and guest Maildir; (4) Assurance & Scale self-describing
recovery capsules, automated restore drill runner, and background blob integrity scrubber.
## Completed Definition — Substrate Cleanup and Cutover

[`definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md`](definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md)
completed 2026-08-26 (commit `a12532d2`, staging `c3314c59`). The guest MicroVM
closure now executes strictly ONE immutable Nix store binary
(wrapper `safvdbs8...` has zero `choir-updater/current` fallback references),
standard embedded Dolt GC is restored (no `RUNTIME_DOLT_GC_DISABLED`), and the
`choir.refresh_runtime=1` deletion dance is deleted (`VMConfig.RefreshRuntime`
removed). Boot-proven on staging: a hibernated test computer resumed onto the
new guest image on month-old persistent disk and reported `deployed_commit
c3314c59` ready. Self-development effects remain formally suspended. The Node B
deploy disk-preflight floor was calibrated 120 GiB -> 100 GiB with a
problem-documentation-first receipt
([evidence](evidence/node-b-deploy-disk-preflight-floor-2026-08-26.md));
host disk expansion remains an owner decision.

## Completed Recovery — Account & Mail (`yusefnathanson@me.com` / `000@choir.news`)

Account `yusefnathanson@me.com` (`5bd6de97-3b58-408c-bf89-c42c81b083de`) is restored:
the route-bound 0333528 computer is active at epoch 804 under the host and guest
maintenance holds, authenticated shell bootstrap returns HTTP 200, and the owner
mailbox exposes the Ryan message. Guest Maildir synchronization is not claimed.
Evidence: [`evidence/account-recovery-yusefnathanson-2026-08-27.md`](evidence/account-recovery-yusefnathanson-2026-08-27.md).

## Completed Definition — Substrate & Scheduling Readiness

[`definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md`](definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md)
completed 2026-09-03 (deployed commits `2bf93be7`..`bf6c51c0`). Target achieved:
live-trigger-only Super wakes, FIFO selection under computer-scoped arrival ordinals
across 4 sequential cycles without supersession, boot-does-not-schedule assertion
verified across epochs, settlement (tombstone) of the nine 08-19 cancel producer
reports via dedicated store reducer with claim-scan retired, exact-run resume structurally
isolated, terminal-event probe negative and positive verified, and normal-boot stability
without `RUNTIME_MAINTENANCE_HOLD` on staging `computer-03335285269bdba4f94377e56879f9e6`
(pre-A checkpoint `99949fe2`). Deployed evidence:
[`evidence/effects-red-substrate-scheduling-readiness-complete-evidence-2026-09-03.md`](evidence/effects-red-substrate-scheduling-readiness-complete-evidence-2026-09-03.md).
Effects remain OFF.

## Active Definition — RLM Session Interpreter Cutover

[`definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md`](definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md)
is **unblocked and active** following Definition 1 completion. Target: persistent Session worker per
activation in `cmd/capsule-broker` and `internal/yaegikernel`, prebound `choir` modules,
SIGKILL reaped <500ms, ambient JSON tool removal from CoSuper prompt, live sealed proof on staging.
## Queued Definition — Supervised Self-Development on RLM

[`definitions/choir-supervised-self-development-on-rlm-2026-09-02.md`](definitions/choir-supervised-self-development-on-rlm-2026-09-02.md)
is **paused pending Definition 1 and Definition 2 deployed acceptance**.
Target: Candidate change A solitaire implementation authored via RLM session cells, 5-ref freeze,
qualified consensus under `reversible-selfdev-v1`, promotion, live play verification, falsification with B,
and restore to pre-A checkpoint `99949fe2`.

## Superseded Definition — Scheduling Contract and Candidate Proof

[`definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`](definitions/choir-scheduling-and-candidate-proof-2026-08-21.md)
is **superseded** by the 3-Definition autonomous engineering sequence (Definition 1 for substrate/scheduling,
Definition 3 for candidate A solitaire proof).
## Sealed Operation — Stabilize and Hold 0333528

[`definitions/choir-0333528-stabilize-and-hold-2026-08-24.md`](definitions/choir-0333528-stabilize-and-hold-2026-08-24.md)
is **settled and sealed** as a historical operation. Its hold statement below
is superseded: the hold was lifted 2026-09-03 during outage recovery
(see `docs/evidence/effects-red-computer-unresolvable-after-refresh-2026-09-03.md`
and `docs/evidence/effects-repair-verification-2026-09-03.md`), and the computer
has since served active staging duty across realizations into epoch 876 with
Effects OFF. Do not treat this section as a standing hold directive.
(Original 2026-08-24 record: held host-side with guest fence, serving `:8085`,
canonical head past 133,319, as immutable evidence artifact.)

## Completed Pre-Flight & Historical Recovery

[`definitions/choir-durable-substrate-preflight-2026-08-24.md`](definitions/choir-durable-substrate-preflight-2026-08-24.md)
completed 2026-08-24. All four pre-flight areas are settled and verified:
(1) Dolt 2.0 embedded-driver upgrade verified; (2) `candidate-fleet-d03dacaa...` invalid-genesis
loop resolved; (3) active-guest Dolt GC policy verified with 5 GiB safe-guard; (4) live computer
`computer-03335285269bdba4f94377e56879f9e6` liveness restored.

[`definitions/choir-durable-substrate-recovery-2026-08-23.md`](definitions/choir-durable-substrate-recovery-2026-08-23.md)
is settled historical recovery evidence. 0333528 was recovered to canonical head 132,436
via the fixed boot/replay contract. Offline ProjectionBase rebuild was deferred to Track F.
## Completed Substrate — Tape-Based Recovery

[`definitions/choir-tape-recovery-2026-08-13.md`](definitions/choir-tape-recovery-2026-08-13.md)
completed 2026-08-15. Staging `4ac90583` paid all six required
receipts, including `serving_join` (unsigned host shell ≠ retained
SPA ≠ secondary SPA after vmctl resolve) and
`capability_renewal_pass` across restore without a subsequent start
(epoch 268, `store_closed: false`). Independent review of checkpoint
+ rematerialization + serving join returned ACCEPT 2026-08-15. It is
settled evidence and the restore substrate, not an executable
entrypoint. Do not rematerialize or invent `choir computer create`
to reopen it.

## Superseded Effects Definition — Historical Evidence

[`definitions/choir-supervised-self-development-effects-2026-08-11.md`](definitions/choir-supervised-self-development-effects-2026-08-11.md)
is superseded historical evidence. Its policy, email, and restore reasoning remain
citable historical evidence; it is not an executable entrypoint. The tape-recovery Definition
owns restore substrate receipts. Active execution lives solely in
[`definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md`](definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md)
(activated 2026-09-03 following Definition 1 terminal receipt; Definition 1 is completed evidence, not an entrypoint).

The scope-disjoint
[`choir-instruction-substrate-prune-2026-08-11.md`](definitions/choir-instruction-substrate-prune-2026-08-11.md)
completed 2026-08-12: 106/106 beads dispositioned, doccheck signal repaired,
instruction packet pruned with invariant conservation. It is settled evidence,
not an entrypoint; do not re-open the retired beads store.

The active Definition had one owner-ratified pre-effects subordinate contract:
[`definitions/choir-sandbox-autoputer-rename-2026-08-11.md`](definitions/choir-sandbox-autoputer-rename-2026-08-11.md).
It owned the single clean naming cutover before effects work resumed: service
surfaces became `autoputer`, persistent computer identity surfaces became
`computer`, and no compatibility path was permitted. The cutover is complete:
the retained pre-drop replay diff is evidence for the active Definition, staging
state was recreated afterward, and the renamed product path passed deployed
acceptance. It is settled evidence, not a second entrypoint or product schedule.

The latest staging runtime/proxy deployment observed by the rename acceptance is
`3cd12d1452ad1d06b5df57cf9183313568f60cb5`; `/health` reported proxy status
OK and vmctl status OK on 2026-08-12. The earlier `914f7a5d976a` frontend/proxy
capture is historical host/source identity only. Retained guest proof and all
effects remain OFF. The residual epoch `8253` record is historical evidence,
not an open unknown on any live Definition.

Historical handoff, guest-prefix, mailbox, and terminal-boot receipts remain in
[the joined runtime review](evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md)
and [its requirement audit](evidence/continuous-texture-supervision-requirement-audit-2026-08-08.md),
plus the disposed Mission 0 direct-key ceremony at
[`continuous-texture-supervision-direct-key-ceremony-2026-08-09.md`](evidence/continuous-texture-supervision-direct-key-ceremony-2026-08-09.md)
(do not execute it as a live gate; no headless retry, substitute identity,
recovery bypass, SSH, or weaker authorization is admissible).
They are historical evidence, not rollback or live schedule; effects remain OFF.
The active executable slice and `next_action` live solely in
[`definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md`](definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md).
The tape-recovery restore proof is paid (complete 2026-08-15).
Completed Definitions are historical evidence, not executable entrypoints; full
claims and receipts remain in their source files and `mission-graph.yaml`:
`choir-tape-recovery-2026-08-13.md` (whole-computer restore substrate);
`choir-coherent-computer-convergence-2026-07-21.md` (durable-work kernel);
`choir-cli-self-development-2026-07-16.md` (incomplete construction);
`choir-audited-autoputer-construction-2026-07-15.md` (audited construction and
D-ROUTE); `choir-autoputer-completion-2026-07-14.md` and
`choir-autoputer-completion-2026-07-13.md` (runtime evidence); and
`og-dolt-heresy-completion-2026-07-08.md` (settled storage/D-ROUTE/H031).
None is executable unless explicitly promoted in the current registry.

## Superseded — Scheduling Contract and Candidate Proof

[`definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`](definitions/choir-scheduling-and-candidate-proof-2026-08-21.md)
is superseded by the 3-Definition autonomous engineering sequence (2026-09-02):
substrate/scheduling scope lives in Definition 1
([`choir-substrate-and-scheduling-readiness-2026-09-02.md`](definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md));
candidate proof scope lives in Definition 3
([`choir-supervised-self-development-on-rlm-2026-09-02.md`](definitions/choir-supervised-self-development-on-rlm-2026-09-02.md)).
It is not an executable entrypoint.

## Draft Successor Definitions — Not Executable

Draft successors are blocked hypotheses, not schedules or implementation
authority. Their source Definitions and three registries retain constraints:
[`definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md`](definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md)
and [`definitions/choir-in-choir-computer-control-draft-2026-07-18.md`](definitions/choir-in-choir-computer-control-draft-2026-07-18.md).
Neither authorizes implementation, host access, raw vmctl, SSH, shared
credentials, candidate VMs, or promotion without separate owner ratification.

## Supporting Maintenance

Supporting maintenance Definitions retain their evidence and status:
`choir-seam-repair-2026-07-10.md`, `choir-autopaper-activation-2026-07-10.md`,
`choir-autoputer-completion-suite-2026-07-11.md`,
`choir-run-truth-suite-2026-07-11.md`, and
`documentation-authority-reduction-2026-07-09.md`. They are settled,
superseded, or historical as stated by their source Definitions, not entrypoints.

## Invocation

Invoke the active Definition 2 via
`/goal docs/definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md`.
RLM session interpreter cutover executes on staging
`computer-03335285269bdba4f94377e56879f9e6` following Definition 1 completion.

## Unowned External Work

No Definition owns runtime dissolution, broader Wire work, external capsules,
or ComputerVersion optimization. The sequenced private-Go actor-kernel
Definition owns actor activation extraction only after its predecessor closes.
Other new work requires current evidence, owner ratification, and registry
promotion.

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is discovery metadata; Definitions
own state, not Git history.

## Settled Deploy Receipt

The former `running_runs: 1` blockage is settled historical evidence at
`9dff369044c2147140782958de3e91971caed6bc`; see
`docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md`. Do not rerun its
topology; document any recurrence as a new problem with promoted authority.
