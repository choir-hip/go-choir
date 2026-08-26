# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Active Definition — Substrate Cleanup and Cutover

[`definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md`](definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md)
is the owner-priority executable `/goal` as of 2026-08-25. Ratified by unanimous
agentic consensus (12 models), it executes the clean architectural cutovers before
substrate overhauls:
(1) Delete mutable dynamic binary updater fallback (`/mnt/persistent/choir-updater/current`),
enforcing that guest MicroVMs execute strictly ONE immutable Nix store binary;
(2) Remove `RUNTIME_DOLT_GC_DISABLED = "1"` to restore standard embedded Dolt GC for normal computers;
(3) Delete `choir.refresh_runtime=1` kernel parameter and wrapper deletion dance;
(4) Seal historical computer `computer-03335285269bdba4f94377e56879f9e6` under permanent
host hold as an immutable evidence artifact (retiring 133k linear replay as a release gate);
(5) Formally suspend self-development effects until clean substrate overhauls (Tracks K, F, M,
Assurance) and the Yaegi Actor Kernel are deployed;
(6) Provision fresh overhaul test computer on staging.

## Suspended Definition — Scheduling Contract and Candidate Proof

[`definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`](definitions/choir-scheduling-and-candidate-proof-2026-08-21.md)
is **suspended pending substrate overhauls**. Attempting to author and prove
self-development candidates on an uncompacted, fragile staging computer with 200-iteration
shell loops was an inverted dependency. It will be re-opened on a fresh, snapshot-backed,
Yaegi-powered computer after the substrate overhauls and actor kernel land.

## Sealed Operation — Stabilize and Hold 0333528

[`definitions/choir-0333528-stabilize-and-hold-2026-08-24.md`](definitions/choir-0333528-stabilize-and-hold-2026-08-24.md)
is **settled and sealed**. Computer `computer-03335285269bdba4f94377e56879f9e6` is
permanently held host-side (`held=true`) and guest-fenced (`RUNTIME_MAINTENANCE_HOLD=1`),
serving on `:8085` (HTTP 200) with canonical head advancing past 133,319. It serves as
an immutable historical evidence artifact.

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
[`definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md`](definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md).

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
[`definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md`](definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md).
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

## Ratified Successor Definitions — Sequenced, Not Yet Executable

[`definitions/choir-durable-substrate-overhauls-2026-08-23.md`](definitions/choir-durable-substrate-overhauls-2026-08-23.md)
is the owner-ratified immediate successor for the 4-track substrate overhauls (Track K key
escrow, Track F file-CAS + periodic event watermarks, Track M host MTA spool +
guest Maildir, and Assurance daily restore drills). It is sequenced directly after
`choir-substrate-cleanup-and-cutover-2026-08-25.md` completes and executes on a fresh
staging computer.

[`definitions/choir-private-go-actor-kernel-2026-08-12.md`](definitions/choir-private-go-actor-kernel-2026-08-12.md)
is the owner-ratified successor for the RLM authoring upgrade. It is sequenced
after the Durable Substrate Overhauls. It converges Researcher and CoSuper on one
durable-actor kernel with private disposable model-authored Go activations inside
isolated guest capsules.
## Draft Successor Definitions — Not Executable

Draft successors are blocked hypotheses, not schedules or implementation
authority. Their source Definitions and three registries retain constraints:
`definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md`
and `definitions/choir-in-choir-computer-control-draft-2026-07-18.md`.
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

Invoke the active cleanup and cutover Definition through
`/goal docs/definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md`.
After cleanup completes, invoke the Durable Substrate Overhauls through
`/goal docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md` (Tracks K, F, M, Assurance),
followed by the Private-Go Actor Kernel through
`/goal docs/definitions/choir-private-go-actor-kernel-2026-08-12.md`.
Self-development candidate proof resumes only after the substrate and actor kernel are deployed.

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
