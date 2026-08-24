# Recovery Consensus Review — boot/replay-contract-first route

Date: 2026-08-23
Mutation class: red (review of red-class recovery Definition)
Subject: `docs/definitions/choir-durable-substrate-recovery-2026-08-23.md` (recover `computer-03335285269bdba4f94377e56879f9e6` to head 132,436)

## Rounds & panel

Three agentic-consensus rounds (convergent) on the recovery plan/Definition. Raw outputs are
gitignored under `.agentic-consensus/recovery-definition-review-20260823-{r1,r2,r3}/`; this
doc is the durable adjudicated record.

- r1 (default 180s panel): `devin` ✅, `claude` ✅ (lean prompt) — verdicts REJECT naive
  ProjectionBase-rebuild-as-primary.
- r2 (4 of 12 succeeded): `devin` ✅, `omp-muse-spark` ✅, `omp-hy3` ✅, `opencode` ✅.
- r3 (1200s default from 7109b070, 7 of 12 succeeded): `devin` ✅, `omp-nemotron-3-ultra` ✅,
  `omp-muse-spark` ✅, `omp-x-preview-f` ✅, `omp-cursor-grok46` ✅, `cursor` ✅, `opencode` ✅.
- Failed/timed-out (env): `codex`, `omp-gpt56-sol`, `omp-gpt56-luna`, `omp-gemini37` (codex
  usage limit); earlier `omp-deepseek-v4-flash-free` unavailable; `hy3` failed once late.

## Adjudicated findings

1. **Root cause is a boot/replay contract defect, not a missing snapshot.** Verified in
   source: `vmmanager/manager.go:265` `BootReadyTimeout` (default 20s) hard-kills firecracker
   on expiry (`:769-776`); `autoputer/run.go:222` runs full-tape `appender.Reconstruct`
   before the HTTP server opens; `store/computer_events.go:182` + `appender.go:677` defer
   the Dolt checkpoint to one commit after the whole chain; `appender.go:588` resumes only
   from `localHead.Sequence`. Signature matches epoch 361 → 707 with zero progress.
   **Caveat (open):** staging may override 20s → 30m (`nix/node-b.nix:544`); the LIVE value
   must be read from the running vmctl process, not the code constant.
2. **The offline ProjectionBase rebuilder (6185910a) is the wrong substrate for this
   recovery.** `projectionbase.DiskEventSource` linear-reduces the flat `computer-event/`
   dir and fails at seq 2 (multiple `key_revoked` @ seq 2); production replay reads the
   canonical chain from corpusd Dolt `computer_event_append_receipts`
   (`event_replay.go:14-27`); the rebuilder fabricates `TransitionInput` for every kind
   (`event_replay.go:92-101` kind-switches in production), does zero real receipt
   verification, and computes the content witness as a self-comparison
   (`rebuilder.go:134`). Deferred to a separate successor Definition.
3. **Owner-ratified route (2026-08-23): boot/replay-contract fix FIRST.** Consensus
   approved this unanimously across the 7 r3 agents and claude/devin r1.
4. **Phase-0 must be a decision table, not a yes/no.** Open probe questions: guest store
   path (`RUNTIME_STORE_PATH=/mnt/persistent/state` in nix vs default `/tmp/...`); does the
   projection working set survive a firecracker SIGKILL (does
   `computer_event_projection_heads.sequence` advance across boots); live `VM_BOOT_READY_TIMEOUT`;
   guest `MachineMemSizeMib` (default 512 vs vmctl 4096, 756 MiB payload corpus — OOM risk);
   boot path (recover_current quarantines + stages a BLANK sparse image + key-only copy →
   explains 'zero progress' too) vs ordinary `BootVM`; leftover `computer_event_index`
   `status='prepared'` rows; free space.
5. **Phase-1 must split liveness from readiness.** Progress heartbeat (`503 ReplayInProgress
   seq=N`) during Reconstruct; product `/health` 200 only after head+witness match;
   `waitForGuestReady` must not mark Running on progress alone; kill gates on replay STALL
   (no seq advance for N s), not wall clock. (Devin preferred a timeout bump; hy3/x-preview-f/
   grok46/cursor preferred the guest-side liveness split — the split is the adjudicated
   choice because it preserves `not_done_when` "route before ReplayCompleteness".)
6. **Resumable durability is conditional on Phase-0.** `finalizeBatch` already SQL-commits
   per event and skips `commitDoltCheckpoint` in replay; `Head()` reads the SQL working set.
   If the working set survives SIGKILL, periodic `DOLT_COMMIT` is optional; if it dies or a
   single pass exceeds the budget, add periodic durable checkpoints (cadence from measured
   events/sec — 60s/5-10k, never per-event; advance `after` only after a successful durable
   commit). Note: at ~3.8 ev/s estimated, 132k events ≈ 10h, so resumability or a single
   long boot is required either way.
7. **RecoverPrepared is a live-append recovery, not a replay resume.** A kill between
   `Prepare` and `Finalize` at seq ≪ 132,436 hits `ErrNeedsProjectionRepair` on the next
   boot. Replay must discard prepared rows already ≤ localHead on the canonical tape.
8. **Quiesce is a mechanism, not a hope.** Assignment/capsule admission must be fenced until
   head+witness+route CAS; effects OFF is necessary but not shown sufficient.
9. **Acceptance additions:** fleet healthy-boot regression; `boot_contract_fix_proven`
   receipt = resume-across-kill demo (seq advances 500 → 1000 across a SIGKILL), not merely
   "stayed up past 20s"; bounded quiesce observation.

## Open questions carried into Phase-0 (read-only probes)

1. Guest store path and `state/` layout (data.img root shows `state` empty-ish, `state.texture/texture` present).
2. Does the projection working set persist across a killed boot (does SQL head advance)?
3. Live `VM_BOOT_READY_TIMEOUT` on staging (20s vs 30m nix override).
4. Guest RAM for this VM (512 vs 4096) and RSS during replay.
5. Were the 707-epoch boots cold-recover wipes or ordinary boots (rec-*.journals).
6. Are there leftover `prepared` rows / what is the current local head.
7. Free space.

## Consensus verdict

Route approved (boot-contract-fix-first; ProjectionBase deferred). Definition updated to be
decision-complete per findings 1-9. Confidence: high on route/root cause, medium on the
empirical Phase-0 branch outcomes.

## Round 4 (1200s, probe-informed, 7/12 succeeded) — 2026-08-23

Panel: `devin` ✅, `omp-nemotron-3-ultra` ✅, `omp-muse-spark` ✅, `omp-x-preview-f` ✅,
`omp-cursor-grok46` ✅, `cursor` ✅, `opencode` ✅; `codex`/`gpt56-sol`/`gpt56-luna`/`gemini`
failed (codex usage limit), `hy3` failed late.

### Adjudicated corrections (all agents converged)

1. **The 0-byte `/mnt/persistent/state` file is INTENTIONAL, not a broken store.** `Open`/
   `OpenFresh` write a 0-byte marker at `dbPath` (`internal/store/store.go:669-679,819-824`);
   the real Dolt working set is `state.texture/texture` (`deriveTextureWorkspacePath`,
   `texture.go:253-266`). The persistence failure is the EMPTY `.dolt` repo (no commits after
   a successful open), not the file type. "Fix the store open" was a misdiagnosis; converting
   the marker to a directory or deleting it breaks `Open`/`OpenFresh`, and `Open` does
   `RemoveAll(workspace)` when the marker is missing — both FORBIDDEN.
2. **Prime suspects for the empty repo:** (a) deployed guest image predates the
   `RUNTIME_STORE_PATH=/mnt/persistent/state` wiring (`nix/autoputer-vm.nix:727`) and used
   `DefaultStorePath /tmp/go-choir-m3/runtime.db` instead — every boot's progress died with
   the VM; (b) rec-2's `ReplaceWorkspace` swap moved away prior progress. Must verify deployed
   guest image provenance vs repo HEAD before coding.
3. **Multi-boot resumable replay is mandatory** — ~6.8k events per 30m pass at ~3.8 ev/s → ~20
   boot cycles for 132,436 events. Single 10h boot or 10x throughput is NOT the small path.
4. **Liveness listener must bind BEFORE Reconstruct** — currently `run.go:225` reconstructs
   before `:425` `s.Start()`, so `503 ReplayInProgress` cannot be served; host
   `waitForGuestReady` (`manager.go:1882-1895`) treats only HTTP 200 as ready. Guest
   listener-before-replay + host: 503+seq = live, 200 = ready, stall = kill.
5. **`bootstrapCtx` 30m must not cancel `Reconstruct`** — host stall-gate without it is
   theater. Either unbind or treat 30m as the resume quantum.
6. **`RecoverPrepared`:** disagreement — opencode argues the `ErrNeedsProjectionRepair` arm
   is unreachable in pure replay (replay never CASes); grok46/cursor argue a prepared row at
   seq ≪ 132,436 trips it. Safest: replay-safe discard, never CAS on the replay path; prove
   with a disk-backed test. Also note the marker-loss wipe hazard.
7. **SQL working set vs Dolt history durability:** opencode — replay `FinalizeReplayBatch`
   already `tx.Commit()`s per event, which persists across process kill, so periodic
   `DOLT_COMMIT` may be HARDENING not the enabling fix; cursor — verify that a reopen actually
   restores `computer_event_projection_heads` from the last Dolt commit. Settle with a
   disk-backed kill experiment before treating DOLT_COMMIT as load-bearing.
8. **Ship vehicle:** guest binary/rootfs rebuild (`nix/autoputer-vm.nix`) AND host vmctl/nix
   (`nix/node-b.nix`). `data.img` is NOT part of the rebuild. Not host-only, not image-only.
9. **Additional must-address:** stall N (120-300s, measure first); checkpoint cadence ≪ 30m
   (60s or ~200-500 events; 5-10k = 22-44m, too late); gate periodic Dolt GC during replay
   (`run.go:96-100`); lifecycle receipts append AFTER `Reconstruct` (`run.go:228-259`) and can
   move the head past 132,436 — fence until head+witness+route CAS or accept ≥target;
   free-space check; quiesce fence authority unnamed (do not reuse recover_current's
   `RecoveryFencingToken` on ordinary BootVM); multi-boot driver choice (manual ~20 cycles vs
   automated boot loop); pre-run protective copy of `data.img` + marker.

### Open questions carried forward (owner/measure decisions)

See the definition `now.question` and `blocker_or_risk`; the implementation gate list is: (1)
deployed guest image provenance vs RUNTIME_STORE_PATH wiring; (2) SIGKILL survival of the SQL
working set (determines whether periodic DOLT_COMMIT is load-bearing); (3) measured ev/s with
payload fetch + checkpointing (cadence, stall N, cycle count); (4) free space; (5) ship
vehicle; (6) quiesce fence authority; (7) bootstrapCtx as resume quantum; (8) RecoverPrepared
scope; (9) lifecycle-receipt fencing; (10) Dolt GC gating; (11) multi-boot driver; (12)
protective copy/rollback.

## A1 answered + owner decisions (2026-08-23)

**A1 (empty store) ANSWERED:** the deployed guest image `nixos-system-go-choir-autoputer-26.05.20260409.4c1018d`
(built 2026-04-09, still referenced by the last boot's fc-config.json) PREDATES the
`RUNTIME_STORE_PATH=/mnt/persistent/state` wiring, which was added 2026-04-20 (git blame
`d4a5f160` in `nix/sandbox-vm.nix:727`). The guest has therefore always run its store at the
default `/tmp/go-choir-m3/runtime.db` — every boot's replay progress died with the VM. That
fully explains the empty `state.texture` repo and zero progress across epochs 361→707, and it
means NO checkpoint code can work until the guest is rebuilt with the wiring. The guest image
is also ~4.5 months stale vs repo HEAD (many refactors since) — the Phase-1 guest rebuild
replaces it wholesale; canonical chain and persistent files are version-agnostic.

**Owner decisions (B5–B13), recorded:**
- B5: ship guest image rebuild + host vmctl/nix TOGETHER (guest rebuild is now mandatory per A1).
- B6: host-side gate — vmctl refuses route CAS + assignment admission until head+witness verified.
- B7: 30m = one resume quantum — checkpoint at the boundary, exit cleanly, boot again.
- B8: discard half-finished/prepared records on the replay path and re-apply from the log; NEVER CAS on replay.
- B9: finish = head >= 132,436 with intact witness (lifecycle appends after replay allowed).
- B10: checkpoint cadence ~60s / 200-500 events; stall timeout 120-300s — finalized after A3 measurement.
- B11: Dolt GC disabled during recovery.
- B12: agentic driver — the agent drives the ~20 x 30m sessions with receipts.
- B13: NO protective backup; on non-convergence, ABANDON this recovery and start anew (fresh computer; owner ratifies recreation). Canonical chain never rewound; "no data lost" preserved platform-wide, explicitly relaxed for 0333528 by owner choice.
