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

## Defect chain (updated 2026-10-06 panel review + trace forensics)

**Bug 1 — mechanism REVISED: the `initial_dispatch` wake exists; the
discard hypothesis is refuted for this path.** The consensus panel
(10/10 agents, `.agentic-consensus/agentic-consensus-20261006-midcourse/`)
independently flagged: `rt.activate` dispatches with
`context.Background()` (`runtime.go:311`), which carries no
`EmissionsFromCtx` buffer — the `initial_dispatch` bypasses the emission
buffer entirely and appends durably via `actorRT.Send` → `log.Append`
(`adapter.go:401-409`, `actor.go:150-175`). An `ErrEpochConflict` at
`Commit` therefore *cannot* have eaten it (and a conflict itself needs a
second writer the single-dispatcher `d.running` serialization precludes —
only a second process on the same `*-actor.db`). Console forensics on the
owner VM confirm no `runtime: activate dispatch for run ...` error was
logged at 17:24 — the `Send` succeeded; the wake row existed.

The surviving mechanism (trace-consistent, not yet row-verified — guest
SQLite is not host-reachable): the mint's `initial_dispatch` entered the
management mailbox behind a `redrive-11` deferral storm — console.log.2
shows `dispatcher: deferred` churning stale occurrence wakes every ~4s
post-mint, plus `handler: unknown update kind "delivery_failed"` poison
events — and either starved or deferred under `ErrDeferUnprocessed`
(handler.go:398 maps any `ExecuteActivationSyncChecked` error to deferral;
the live-occurrence handler's `slot occupied` deferral is by-design while
462d30ea held the slot). A **durable re-drive authority already exists**:
`actorWakeOutboxFromObject` derives an `initial_dispatch` wake for every
pending fresh mint atomically at commit (`lifecycle.go:772-805`), re-driven
by `sweepActorWakeOutbox` every 500ms — and it still did not deliver, which
narrows the cause to (a) the outbox row marked `projected` into the wiped
guest-local actor log (re-image wipes `*-actor.db` while Dolt persists the
projection flag — mark ordering at `runtime.go:2575-2594` dispatches before
marking, so the only loss window is a process death between append and
mark), or (b) the sweep's dispatch erroring identically to the direct Send.

The original epoch-conflict theory additionally predicted Bug 2's trigger
window correctly; that part stands.

**Bug 2 — watchdog no-op on zero bound packets** (unchanged):
`armFreshMintManagementResumeWatchdog` schedules
`fresh_mint_management_resume_deadline` at mint+10min. When it fired
(~17:34), `redriveStrandedFreshMintManagement` found **zero bound packets**
(the escalate packet is not a lifecycle control) and returned `false`
without releasing the slot. The watchdog consumed its durable occurrence;
the run remained `pending`. No further watchdog exists.

Combined effect: a fresh-mint Management run whose `initial_dispatch` never
delivers (mechanism above) and whose bound-packet set is empty becomes a
slot-holding zombie. The desk's obligation stays pending, all subsequent
live occurrences defer on `slot occupied`, and the probe legs time out.

## Fix shape

1. `redriveStrandedFreshMintManagement`: when `packets` is empty, fail-release
   the run via `terminalizeRunCanonical` (reason
   `management_fresh_mint_no_bound_packets`), freeing the slot. **Shipped:
   `01199fb1`.** The desk obligation stays pending; the next live
   occurrence mints fresh.

2. Emission-discard instrumentation (panel consensus, 9/10): log the drained
   `emitted`/`incorporated` counts on the `ErrEpochConflict` and
   `ErrDeferUnprocessed` paths in `dispatcher.go` — both are silent today,
   which is why the discard was guessable rather than observable. Do **not**
   implement emit-before-CAS (panel verdict 9/10 against): it breaks
   fenced-commit atomicity ("a stale activation's emissions die with it",
   dispatcher.go:55-57) and re-delivers non-deterministic emissions
   (`actorDispatchUpdateID` mints a random uuid on empty content).

3. Outbox/actor-log durability gap (deferred investigation): if mechanism
   (a) above is confirmed — `projected` outbox flag in Dolt outlives the
   wiped guest-local `*-actor.db` — the substrate fix is deriving
   unprojected-on-boot re-dispatch from the wake row, or persisting the
   actor log on the durable volume. Not fixed here.


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

**Fix-path exercise status (panel review 2026-10-06):** the post-fix probe
observed **zero mints**, so neither `slot occupied` nor the zero-packet
fail-release was exercised — the desk never authored
`open_persistent_super`, and no outbox or slot-occupied event arose to test
the repair. Console forensics: `slot_occupied` deferrals=0 on the fixed
boot segment (vs 41 pre-fix); `reconcile_terminal_run_outcomes` completed.
Deployed behavioral verification is still owed: the next probe must mint a
fresh run then strand it to observe the fail-release.

**Consensus panel outcome + reorientation**
(`.agentic-consensus/agentic-consensus-20261006-midcourse/`, 10/11 agents
returned, convergent mode):

- **A (emission discard):** reject emit-before-CAS 9/10; refuted as *this*
  incident's mechanism (see defect chain). Reduce to instrumentation.
- **B (degraded-ownership terminal for bearer routing):** fix belongs in
  vmctl `reconcileLookupReadiness` — a non-constructive `degraded→active`
  re-check on the same realization (no wake/assign/restart). Red surface;
  panel says schedule now — every bearer-token deployed proof currently
  fails silently on it. Named residual.
- **C (station order):** build a deterministic owner-side control surface
  that mints persistent Management without texture-desk agency — the desk's
  `open_persistent_super` authoring is a model-behavior dependency with no
  convergence date and every SMG leg sits behind it.
- **D (attribution):** "texture agency" for the post-fix zero mints stays a
  **hypothesis** — `controls[]` emission isn't console-observable; only the
  desk's turn-commit receipt distinguishes never-emitted from
  emitted-and-discarded.
- **Probe-hardening findings (unfixed):** `/api/trajectories` polling is
  status-blind (missing events read as `bound_report=false`,
  indistinguishable from routing/auth failure); `work_disposition`
  substring over-matches as a bound report; "report before bound report" is
  not a genuine unbound test (controls bind pre-activate).
- **Fresh-disposable stale release (corrected 2026-10-06):**
  `storedisk.erofs` is a shared host-level guest store disk resolved at
  boot — a mint boots whatever image is current *at boot time*.
  `computer-ee6cb12d` booted `im5qd1rfi` because it minted at 19:01Z before
  the `01199fb1` deploy's image build finished (image lands ~15-20min
  post-commit). Not a pin defect — a deploy-race: mints before image
  replacement completes get the old build. Gate: verify guest `/health`
  `build.commit` matches the deploy before running legs on a fresh
  disposable (folded into the SMG probe).
