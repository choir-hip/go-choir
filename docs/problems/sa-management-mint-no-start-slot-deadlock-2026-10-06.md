# Persistent Management mints a run that never starts and holds the slot forever

**Status** confirmed on staging, root cause traced, fix committed
(2026-10-06). First write of a behavior-changing fix for the stranded-run
deadlock; this doc is the required problem-first record ahead of the fix
commit.

## Finding

2026-10-06, SMG acceptance probe on disposable `computer-03335285269bdba4f94377e56879f9e6`
(user `8d444018-22d5-4e96-b7e3-02a296b29a36`, registered via passkey; computer
minted `active` after explicit `bootstrap-chain` POST per the S0b genesis
problem).

The probe's escalate-tray leg POSTed a coagent update that minted persistent
Management run `462d30ea-a33a-41e2-9e55-6295bc2f31f4` at 17:24:16Z. The
runtime logged `persistent Management live occurrence bound run=462d30ea…`
— the reconcile minted it. Every later live occurrence then deferred with
`persistent Management slot occupied by run 462d30ea…`.

At 17:51Z the run was still `state=pending`, `updated_at=17:24:16` —
**27 minutes minted, never executed**. Console log shows zero
`initial_dispatch` delivery lines, zero `ExecuteActivation` lines, zero
non-slot deferrals for that mailbox. The minted `initial_dispatch` was lost
(or dropped before `dispatchReady`); the run holds the slot while remaining
permanently `pending`.

## Defect chain (two bugs, one stranded outcome)

**Bug 1 — lost `initial_dispatch` (root: epoch-conflict emission discard,
hypothesis)**: `reconcilePersistentManagementActorLocked` mints the run and
calls `rt.activate(rec)` → `dispatchActor` → `a.dispatchAtMode`. Because the
mint happens *inside* a dispatcher activation (the handler for the incoming
`sha256:` live occurrence), `EmissionsFromCtx` buffers the update — it
becomes durable only at `d.log.Commit`. `Commit` on `ErrEpochConflict`
returns with **no log line and no retry** (`internal/actor/dispatcher.go:445-449`),
discarding the buffered `initial_dispatch` while the minted run — already
durable via its own writes — stays `pending`. Nothing marks this run
terminal, so it occupies the persistent Management slot forever.
(Hypothesis: epoch conflicts are the only silent emission-discard path;
deferred activations also drop `evEmitted` at dispatcher.go:382-409, but a
deferred handler never reaches the "bound run" log line that was observed.)

**Bug 2 — watchdog no-op on zero bound packets**: `armFreshMintManagementResumeWatchdog`
schedules `fresh_mint_management_resume_deadline` at mint+10min. When it
fired (~17:34), `HandleFreshMintManagementResumeDeadline` →
`redriveStrandedFreshMintManagement` → `listPendingLifecyclePacketsDeliveredToRun`
returned **zero bound packets** — the escalate packet is not a lifecycle
control, so nothing was bound at mint — and the function returned `false`
without releasing the slot. The watchdog consumed its durable occurrence;
the run remained `pending`. No further watchdog exists.

Combined effect: a fresh-mint Management run whose `initial_dispatch` is
lost and whose bound-packet set is empty becomes a permanent slot-holding
zombie. The desk's obligation stays pending, all subsequent live
occurrences defer on `slot occupied`, and the probe legs time out.

## Fix shape (for the landing commit)

1. `redriveStrandedFreshMintManagement`: when `packets` is empty the run
   has no recoverable bound obligation. Rather than a silent `return false`
   (which keeps the slot held by a dead run), transition the run to a
   terminal state (`failed`) via `terminalizeRunCanonical` with reason
   `management_fresh_mint_no_bound_packets`, freeing the persistent slot.
   The desk obligation remains pending; the next live occurrence mints a
   fresh run against it — self-healing rather than permanent deadlock.

2. (Already covered by the existing boot-passivation path — a runtime
   restart passivates `pending` runs and clears the slot; the fix above
   makes the watchdog do the same without requiring a restart.)

The initial_dispatch loss itself is under separate observation; the
fail-close on zero bound packets bounds the damage of any future
dispatch loss without needing to detect it directly.

## Probe evidence

`docs/evidence/smg-rlm-acceptance-owner-clean-2026-10-06.json` — probe on
computer `03335285`, escalate leg submitted 17:24, timed out at 45min
deadline with `bound_report=false`, `mgmt_signal=false`.

## Mutation class

`red` — protected surface: persistent Management slot lifecycle and run
state authority. Rollback path: revert the `len(packets) == 0` terminal
release branch to the prior `return false`; the run then strands again
(the known defect), which is the pre-fix behavior.

## Standing questions

- Decision provenance: defect observed on staging; fix is an orchestrator
  change to runtime controller code; no user-facing authority shift.
- Settled-decision conformance: conforms to the persistent Management
  single-slot contract; the fail-release frees the slot rather than
  violating it.
- Single state authority: the run record is the sole authority;
  `terminalizeRunCanonical` writes through the canonical lifecycle reducer
  when the run is lifecycle-bound.
- Artifact-verified success: probe legs 1–3 on a fresh disposable must
  show `bound_report=true`, `cancel_verb=true`, `unbound_refusal=true`
  after the fix deploys.

## Deploy + verification status (2026-10-06)

`01199fb1` built into the guest image (`guest/build.json` commit match;
`autoputer=/nix/store/qs4irlsyzy…` carries the fix string) and reached the
owner computer via the sanctioned manual path
`choir computer refresh --computer computer-03335285…` (vmctl epoch 1107,
guest /health `build.commit=01199fb1`, `deployed_at=19:10:28Z`). Two
substrate wedges surfaced on the way: a `degraded` ownership state is
terminal for API-key routing (`api key computer realization unavailable`
until explicit `choir computer start`, epoch 1108) — the bearer path never
re-resolves by design, so health recovery does not self-heal routing.

**Fix verification outcome**: SMG probe on the fixed runtime
(`docs/evidence/smg-rlm-acceptance-fixedruntime-2026-10-06.json`) —
`prompt_bar_submit` OK, but no persistent Management mint arrived in 40
min: the texture desk never authored `open_persistent_super` (the S0m
agency edge). The mint-no-start defect is therefore not exercised on the
fixed runtime yet; verification requires a live escalate→mint (texture
agency, deterministic owner control, or a rerun once agency improves).

**Fresh disposable note**: `computer-ee6cb12d…` minted post-deploy at
19:01Z still boots the pre-fix wrapper (`im5qd1rfi…`); new computer mints
do not ride the rebuilt guest image until the mint path re-resolves the
guest release — another propagation gap inside the same class.
