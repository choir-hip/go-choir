# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Active Definition — Scheduling Contract and Candidate Proof

[`definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`](definitions/choir-scheduling-and-candidate-proof-2026-08-21.md)
is the owner-priority executable `/goal` as of 2026-08-21. It supersedes the
2026-08-20 candidate-proof Definition (whose finish.acceptance it inherits
unchanged) and the 2026-08-11 effects definition. Proof target is
**policy-governed autonomy**: the ratified scheduling contract (one live
assignment per computer; computer-scoped arrival ordinals; FIFO among
non-expired requests; expiry/deadlines; retryable refusal) proven end-to-end,
then capsule-authored candidate change A under effect-specific qualified
consensus, E2 correction, and acceptance-fenced restore through the completed
tape-recovery substrate (platform/cycle OUT of restore; computer-surface
frontend is per-computer). This Definition does not include any email leg.
Effects remain OFF until this Definition's decision-policy gates pass. Active
execution: scheduler contract, texture authoring repair, then in-VM candidate
authoring inside guest capsules, bundle freeze with five refs bound to
capsule-exec receipts, qualified consensus under reversible-selfdev-v1,
promotion, live play verification, falsification with B, and restore to pre-A
checkpoint 99949fe2.

### Subordinate Operational Contract — Recovery of Stopped Computer 0333528

[`definitions/choir-durable-substrate-recovery-2026-08-23.md`](definitions/choir-durable-substrate-recovery-2026-08-23.md)
is the owner-ratified subordinate operational contract executing as of
2026-08-23. It unblocks the retained computer `computer-03335285269bdba4f94377e56879f9e6`
(stopped, epoch file 707, no live firecracker — stale pid) so the scheduling Definition can proceed. It supersedes the
predecessor [`definitions/choir-host-orchestrated-recovery-2026-08-22.md`](definitions/choir-host-orchestrated-recovery-2026-08-22.md).
Delivery: 0333528 recovered to canonical head 132,436 via the fixed boot/replay contract — resumable replay (periodic Dolt checkpointing; `Reconstruct` resumes from `localHead.Sequence`) and no hard-kill at the 20s `BootReadyTimeout` — converging to active, quiesced state on staging. The verified root cause is a boot/replay contract defect (20s hard-kill before the single deferred Dolt commit), not a missing snapshot. The offline `ProjectionBase` rebuild substrate is deferred to a **separate successor Definition** (owner-ratified 2026-08-23); this Definition does not publish a `ProjectionBase`.

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
is superseded by the candidate-proof Definition above. Its policy, email, and
restore reasoning remain citable historical evidence; it is not an executable
entrypoint and does not own current next_action. The tape-recovery Definition
owns restore substrate receipts; the candidate-proof Definition owns the current
candidate proof. Reasoning and retired approaches live in the
[supplement](definitions/choir-supervised-self-development-effects-2026-08-11-supplement.md).
`choir-self-development-roadmap-2026-08-11.md` is a **historical migration
receipt**, not a live schedule. CTS remains citable evidence, not an entrypoint.

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
They are historical evidence, not rollback or live schedule; effects remain OFF
until the candidate-proof Definition's decision-policy rehearsal gates pass. The
current executable slice and `next_action` live only in the
candidate-proof Definition. The tape-recovery restore proof is paid (complete
2026-08-15).

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

## Ratified Successor Definition — Sequenced, Not Yet Executable

[`definitions/choir-private-go-actor-kernel-2026-08-12.md`](definitions/choir-private-go-actor-kernel-2026-08-12.md)
is the owner-ratified successor for the RLM authoring upgrade. It is sequenced
after the active supervised-self-development-candidate-proof Definition proves
the effect-policy autonomy window, reversible restore, irreversible consequence
receipts, and completion cutover. It does not create a concurrent product
schedule and must not begin effect-bearing implementation while its predecessor
is working.

The successor converges Researcher and CoSuper on one durable-actor kernel with
private disposable model-authored Go activations. Restricted profiles receive
Go only; an effects-capable implementation assignment additionally receives
direct Bash through the existing capsule broker. The first acceptance target is
a CoSuper vertical slice with dynamic delegation, citable command/message
receipts, forced activation death, cross-model durable recovery, Texture
transclusions, and an exact externally governed effect bundle. The same kernel
must then refuse Bash and general execution under a Researcher profile.

[`definitions/choir-durable-substrate-overhauls-2026-08-23.md`](definitions/choir-durable-substrate-overhauls-2026-08-23.md)
is the owner-ratified successor for the 4-track substrate overhauls (Track K key
escrow, Track F file-CAS + periodic event watermarks, Track M host MTA spool +
guest Maildir, and Assurance daily restore drills). It is sequenced after the
subordinate recovery Definition unblocks `computer-0333528...` on staging.

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

Invoke the owner-priority scheduling-and-candidate-proof Definition through
`/goal docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`.
Its `now.*` and `finish.*` own the current product schedule. Tape-recovery
completed 2026-08-15 and is settled evidence, not an entrypoint. After the
scheduling Definition reaches `complete` and the registries promote the
successor, invoke
`/goal docs/definitions/choir-private-go-actor-kernel-2026-08-12.md`.
The successor is owner-ratified architecture authority but is not concurrently
executable. Historical CTS, convergence, performance, Choir-in-Choir, and
Autopaper material is evidence or blocked hypothesis, never an entrypoint.
The OG/Dolt/heresy and computer contracts retain D-ROUTE, H031, lifecycle,
route-projection, and single-event-chain authority; this Definition adds no
third semantic store.

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
