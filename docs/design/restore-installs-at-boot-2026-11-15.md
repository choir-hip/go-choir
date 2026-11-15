# A restore installs at boot

Date: 2026-11-15. Class of this document: green (design and failure list, no
code). Station: lane/restore-boot-install, from lane/restore-act at
`06b742853`. It answers
`docs/problems/restore-act-three-review-rounds-structural-assessment-2026-11-15.md`
(on main): three review rounds of an in-process swap did not converge, so the
swap leaves the running process.

Citations are `file:line` at `06b742853` (the lane tip) unless a commit is
named; the assessment says "run.go:213" for the compact-rebase line on main, and
at the lane tip the same call is `internal/autoputer/run.go:220`. Anything I
did not read in the code is marked **inference**.

## 0. What reading the code found first

The mechanism asked for is sound. One thing the code shows makes it useless
alone, and it comes before the design because it decides whether the design is
worth building:

**The boot that installs the restored store then replays it forward to the
canonical head, and ends where it started.** The replay phase calls
`appender.Reconstruct` (`internal/autoputer/run.go:860`), which replays the
chain to the host's head, not to a target. Its plan is made by
`materializeProjectionBaseIfNeeded` (`run.go:436`) with a store that holds
events: `PlanRecovery(empty=false, localSeq, chainExists, 0, targetSeq=H)`
(`internal/autoputer/projection_base.go:140-148`), "a store that holds events
resumes from its own head". A store restored to checkpoint head T therefore
resumes from T and replays (T, H] at that boot. Two consequences:

1. The restore does not stick. After the replay the store is the pre-restore
   state plus the `restore_requested` event. The release `current` pointer is
   the only part that stays restored. This is the 2026-10-30 problem
   (`docs/problems/restore-is-neither-forward-nor-durable-2026-10-30.md`), now
   deterministic: it happens at the install boot instead of at the next append
   (`internal/computerevent/catchup.go:1-60`, the in-place catch-up), and the
   in-process flip has the same end state, later.
2. A boot refuses when H − T exceeds `MaxRecoveryTailEvents` = 720,000
   (`internal/recoveryplan/recovery_plan.go:22`, refusal at `:131-134`, surfaced
   as a boot refusal at `run.go:442-444`). A checkpoint more than 720,000 events
   behind the head, restored, makes the next boot refuse: the computer is down.
   (The act's plan refuses a target with no snapshot inside the bound relative
   to the target, `restore_snapshot.go` `planRestoreTarget`; nothing bounds the
   distance from the target to the head.)

What would make a restore stick is the design already written and never built:
a forward `restore_applied` event, an epoch root whose effect is "state :=
reconstruct(T) ⊕ carried", so that after the install local head = canonical
head = R and the boot replays only what was appended after R
(`docs/design/computer-history-restore-audit-2026-10-30.md:208-233`, steps
4-5 at `:598-600`). That is a new event kind, a reducer rule and a narrow append
path: a red change of its own, outside this lane's authority.

So: **the boot step can carry the swap; it cannot carry the meaning.** Section
2 designs the swap as asked, assuming a forward event R exists, and labels each
place that assumption is used (P1). Open question 1 is whether to build P1
first. The conservative default is that nothing in this document is built until
the owner rules on it.

## 1. What boot does today, from the code

### 1.1 Order, and what could need the store or the disk first

1. `go-choir-updater-prune` (a oneshot, ordered before the updater daemon and
   the runtime): `choir-updater prune --root /mnt/persistent/choir-updater
   --budget 60s` (`nix/autoputer-vm.nix:905-927`). It is the only process in the
   updater root while it runs. It keeps `current`, the Prior of the newest apply
   and of the newest restage, journal-named releases and a rollback of a package
   migration, at most three releases, and deletes the rest
   (`internal/updater/prune.go:20-61`). It says what this costs: a restore to a
   release that is neither current nor a predecessor finds it pinned-absent and
   refuses (`prune.go:56-61`).
2. The runtime unit's `ExecStart` is a shell script
   (`nix/autoputer-vm.nix:135`, `:1246`). It reads
   `$CHOIR_UPDATER_ROOT/current/layering-entrypoint` and execs that release's
   binary instead of the base one (`:153-171`), with a base-binding check
   (`:172-187`) and a boot-loop guard (`:188-`). **The binary that runs the Go
   boot is chosen by `current` before any Go code runs.**
3. `Run()` (`run.go:171`): config, cgroups, the HTTP server and its startup
   failer (`:172-199`), `MkdirAll` of the store directory (`:201-204`). Nothing
   in `:171-205` opens the store or reads the updater root (read from the
   lines; **inference** that no callee does).
4. At the lane tip: the restore repair (`run.go:212-218`, my
   `RepairRestoreAtBoot`, the thing this design replaces), then the compact
   rebase (`:219-229`), the header/body split gate (`:230-241`), `store.Open`
   (`:243`) with the pre-rebase fallback (`:244-253`).
5. After the store opens: credentials and the privacy key, then
   `materializeProjectionBaseIfNeeded` and the recovery plan (`:436`), the
   marker consumption (`:632`), `selfDevelopmentUpdaterOption` (`:568`), which
   is the first place Go reads `current` (`runtime.go:1201-1210` at the tip,
   `ReadCurrentManifest`), and `runReplayPhase` (`:849-925`).

### 1.2 The planned-restart marker

- Path: `<storePath>.planned-restart` (`planned_restart.go:127-133`).
- Writers: the guest's planned-restart endpoint for a deploy refresh, reason
  "platform update" (`planned_restart.go:851-864`, reason constant `:499`;
  the host then kills the VM, `:841`), and the restore restart, reason
  `self_development_restore` (`self_development_restore_restart.go:14-17`,
  `:81-92`). Both go through `writeHostPlannedRestartMarker`
  (`telemetry_batching.go:125`), which drains telemetry and holds synchronous
  mode; the file is written with fsync, rename and directory sync
  (`planned_restart.go:136-169`).
- Peek: `PlannedRestartMarkerPending` (`:86-88`, `:61-80`) is true only for a
  marker present and no older than `plannedRestartMaxAge` = 15 minutes
  (`:28`); a marker that cannot be read counts by its file time. It consumes
  nothing. The compact rebase calls it at `run.go:220`.
- Consumption: `ConsumePlannedRestartMarker` at `run.go:632`, after the store
  open and the credential exchange, judged at process start so slow phases do
  not age it (`:625-631`). It removes the marker before the caller resumes
  (`planned_restart.go:184-224`); a marker that cannot be removed, is
  unreadable, has another work-state epoch (`:215-218`) or is stale is a crash.
- **No marker is a crash** (`planned_restart.go:20-23`, `run.go:637`): nothing
  resumes, open obligations are closed "interrupted by a restart".

### 1.3 The compact rebase: how it swaps and what a crash leaves

`store.RebaseOntoCompactCopy(dbPath, planned)` (`internal/store/compact.go:590`).
It touches `state.db` and its `-wal`/`-shm` only; the marker and the workspace
are not renamed (it writes the lease-untrusted marker into the workspace,
`:683`, `:890`).

- Every boot first finishes or undoes a swap a crash interrupted: `state.db`
  absent and `state.db.precompact` present means a crash between the renames;
  with an intact staged copy it finishes, otherwise it undoes (`:595-614`).
  This runs on any boot, planned or not: it moves files to a state a previous
  boot began, and replays and fetches nothing.
- Only then, if `planned`: it checks the staged copy (sidecar, layout, tail,
  header, `:615-679`), carries the two direct-SQL tables from the original
  (`:657-661`, table list `:716`), then swaps with renames:
  `state.db → .precompact` (`:687`), `moveWAL` (`:690`, `:877-887`),
  `finishSwap` staged → `state.db` (`:693`, `:889-899`, with a directory sync),
  and writes the layout marker (`:696`). Each failure undoes (`:691`, `:694`).
- The original is the rollback copy until the boot replay is done: it is
  removed by `removePrecompact` (`:915-925`), called from
  `StartTelemetryRetirement`, "once the replay is done and the runtime is up"
  (`telemetry_retire.go:281-295`). If the swapped-in file will not open the boot
  puts the original back (`RestorePrecompact`, `compact.go:844-857`,
  `run.go:244-253`).
- A crash mid-swap leaves a state the same function resolves on the next boot,
  from files, with no record. That works because it moves one file and the
  fact is unambiguous (the state file absent plus a rollback present).

### 1.4 Where `current` is read at boot

(a) By the shell `ExecStart` before the Go boot, to choose the binary
(§1.1.2). (b) By `WithSelfDevelopmentUpdater` after the store is open
(§1.1.5). (c) By the updater's `RestartCurrent`, which restarts the runtime
unit and so re-runs (a) (`internal/updater/client.go:312-330`). The restage of
`current` is `updater.RestagePinnedRelease` (`updater.go:987-1019`): it checks
the release's pinned manifest and the store floor, journals the swap so the
release it leaves stays the prune's predecessor (`:1015`, `:1025-`), and swaps
the pointer atomically.

## 2. The mechanism

### 2.1 In one paragraph

The act replays into a staged store exactly as today and verifies the witness
there. It then writes a durable **intent** (the act's record, phase `staged`),
asks for a planned restart through the existing restore-restart path, and ends
its in-process life: nothing live is touched. At the next boot, before the store
opens, one step reads the record, and if a planned marker is pending it renames
the old store into a quarantine directory, renames the staged store into place,
sets `current` to the restored release, records `installed` durably, and lets
the boot open the store it finds. That step is the only place a store or a
release pointer is swapped, and it only goes forward.

### 2.2 The facts that remain (the act's record)

The accepted-act surface is unchanged: 202 with an act id, the record and its
durable writer (`restore_act.go:307-341`), the status route, one act at a time,
the refusal beside live work, the CLI, `/health`. In the record, the install
phases become three, named by what they say about the files, not by what
happened in a process:

| Phase | Meaning | Written |
| --- | --- | --- |
| `staged` | the staged store is complete, verified and sealed; nothing live is changed; the restart is requested | by the act, durably, once, after the witness matched and the staged store was closed and its lease marker written |
| `installing` | the boot has decided to install and will finish it whatever happens | by the boot step, durably, before its first rename |
| `installed` | every rename is done and synced and `current` is the restored release | by the boot step, durably, before the store opens |

`prepared`, `release_restaged`, `flip_started`, `flip_completed`, the corrective
fact, and `restored` as a special terminal state disappear. The record keeps:
the act id, kind, computer, checkpoint digest and target head, the estimate and
progress, the prior and restored release digests, the quarantine path, the list
of renames (written once, at `installing`), the refusal, `live_state`, and the
result report (computed before the intent; it is a statement about the staged
store, which is why it can be written before the install).

**Why three phases and not two.** The install is several renames (marker,
workspace, `state.db`, `-wal`, `-shm`, each way). A crash between two of them
needs a verdict from the record alone. With only `staged` and `installed` the
verdict would have to come from which files exist, which is the inference the
three review rounds kept finding holes in. `installing` is the one added fact:
"the boot began; every boot rolls this forward". It exists because the
installation is not one rename; if the store layout allowed a single rename
(for example both realizations as sibling directories and one pointer) there
would be two. It does not.

### 2.3 Who requests the restart, and the cases where it does not come

The act calls `requestRestoreRestart` (`self_development_restore_restart.go:138-173`):
if the computer is busy (`restoreRestartBusy`, `:121-130`: running runs, a live
capsule or apply, a streaming snapshot) it waits, polling every 15 s for up to
one hour (`:57-60`, `:189-`); when idle it writes the marker (reason
`self_development_restore`, `:81-92`), waits 2 s for its HTTP answer, and calls
the updater's `RestartCurrent` (`:175-187`). A marker that cannot be written or
a restart that fails clears the marker (`:94-98`, `:182`), so a later crash is
never read as planned.

The act's state while it waits is `awaiting_restart`, with the deferred reason in
`restart_deferred`. Cases:

- **The restart comes** (the planned boot): §2.4.
- **The restart never comes** (busy past the hour, updater unavailable, marker
  unwritable). The intent stays, `restore_in_flight` stays true so no snapshot
  capture starts, the status says "awaiting restart, not requested: <why>". Asking
  again is the same request: re-posting the restore with the same idempotency
  key returns the act (`restore_act.go:520-`) and, new, re-asks for the restart
  (**the one new behavior on the existing route; no new route, no switch**).
  Until then the staged store (a store-sized directory) stays on the volume,
  counted by the floor (§2.8).
- **A crash restart comes first** (any boot with no marker: host kill, OOM,
  deploy VM restart without a marker, a manual stop and start). The contract
  says nothing resumes. The boot step finds the intent with no pending marker:
  it closes the act `interrupted` (live state pre_restore, nothing was moved),
  durably, then removes the staged store. The owner re-submits and pays the
  replay again. Stated cost: a crash in the waiting window discards up to an
  hour and a half of replay. The alternative (installing under no marker)
  would be a boot-time resumer that runs without a marker, which the contract
  forbids (`planned_restart.go:20-23`); I do not choose it.
- **A deploy's planned restart comes first** (marker reason "platform update").
  Any pending planned marker installs the restore; the restore does not need
  its own marker, because while an act is open `restore_in_flight` is true and
  the planned-restart endpoint refuses `allow_busy` and ordinary requests alike
  (`planned_restart.go:571` `unsafe()`, the refusal at `:813-838`).
  Acceptance also refuses while a marker is pending (new, §2.7), so a restore
  cannot be accepted into a restart already planned. The boot installs, the
  deploy proceeds as it would. **Inference:** this ordering is safe for the
  deploy because the install finishes before `store.Open`; I have not read the
  deploy's own boot resumes against a changed store.

### 2.4 The boot step, in order

A new function beside the compact rebase, `installRestoreAtBoot(storePath,
updaterRoot, plannedPending)`, called at `run.go:220` in place of
`RepairRestoreAtBoot` and before `RebaseOntoCompactCopy`. Every sub-step is
idempotent; the order is the whole design.

0. Read the acts directory strictly (`restore_act.go:1195-1217`), validate every
   record's paths lie inside the data volume (`:1223-1249`). Anything unreadable
   beside a staging or quarantine directory is **unresolved**: nothing is
   touched, the guest refuses new restore acts and `/health` names it (the
   existing behavior, `restoreUnresolvedNow`, `:737-775`). Then, for the one open
   record, if any:
1. Phase `installing` → go to 4, **whatever the marker says.** A half-made
   install is rolled forward on any boot, planned or not, exactly as the compact
   rebase finishes its own interrupted swap on any boot (`compact.go:595-614`).
   This moves files to a state a previous boot decided on; it replays, fetches
   and reads no staging.
2. Phase `staged`, **no planned marker pending** (`PlannedRestartMarkerPending`
   false) → close the act `interrupted` (durable), then remove the staged store.
   Done; the boot goes on and opens the untouched live store.
3. Phase `staged`, planned marker pending → check cheaply and fail closed:
   the staged files exist and the state file's header is intact (the check the
   compact rebase uses, `compact.go:813-839`, `checkStagedHeader`: **inference**
   that it is reusable for a restored store's `state.db`), the restored release is
   still pinned (`updater.ReadPinnedManifest`, the check `verifyPinnedFrontend`
   does, `rematerialize.go:875-894` at the tip), and the old store exists. A
   failure here closes the act `failed` (named: `release_pruned`,
   `staged_damaged`, ...; live state pre_restore, durable), removes the staged
   store, and the boot goes on untouched. The release can be pruned here: the
   prune runs at this very boot before the runtime (§1.1.1), and accept-time
   pinning only proves the release existed when the act was accepted. Nothing
   else is re-verified: the witness was checked on the staged store before the
   intent; re-hashing a 46 GB store at boot is not "cheap".
   Then write `installing` durably, with the rename list (only renames whose
   source exists), and continue.
4. Roll the renames forward, in the listed order: old marker, workspace,
   `state.db`, `-wal`, `-shm` into the quarantine directory (created now, beside
   the store); then the staged ones into place. Each rename is made only if its
   source exists and its destination does not (a repeat after a crash skips the
   ones done; the list is the recorded one, so nothing is moved by inference from
   a source that never existed). Then sync the data, quarantine and staging
   directories. A rename that fails (an I/O error) is not undone: the record
   stays `installing`, the boot logs and **refuses to open the store** (§2.6),
   so a repeat boot retries. There is no undo path in this design.
5. Set `current` to the restored release: `RestagePinnedRelease` with the
   restored workspace's floor (`updater.go:987`). Idempotent: already current is
   a no-op. A failure keeps `installing` and refuses the boot as in 4.
6. Write `installed` durably (the record's terminal state `succeeded`, with the
   result report and the forward-durability notice, `restore_act.go:93`). Remove
   older quarantine directories (an earlier restore's) now that this outcome is
   durable. Remove the empty staging directory.
7. If the restored release execs a different binary than the one running
   (`restoredExecDiffers`, `self_development_restore_restart.go:29-42`, using the
   running manifest the boot has), write a planned-restart marker (reason
   `self_development_restore`) and exit for the updater/systemd to start the
   next boot into `current` (§2.5). Otherwise go on.
8. The compact rebase runs with `planned = false` when a restore was installed
   this boot or an intent is open (§2.7), then the split gate, then `store.Open`,
   credentials, the recovery plan, and the replay phase as today.
9. After `replay_done` (`run.go:911`), the quarantine of this install is removed
   in the same place the compact rebase's rollback copy is (the call that runs
   "once the replay is done and the runtime is up", `telemetry_retire.go:281-295`).

### 2.5 The binary: why the boot step sets `current`, not the act

`current` chooses the binary before Go runs (§1.1.2). If the act restaged `current`
before asking for the restart (as the in-process flip does,
`rematerialize.go:285`), the restart would exec the restored release's binary,
which is an older release and may predate this boot step: it would open the
unswapped live store under the restored release's pointer, the mix the reviews
refused. So the **act does not touch `current`**; the boot step (running in the
binary that wrote the intent and so contains the step) swaps the store and sets
`current` itself. If the restored release execs a different binary, step 7 asks
for one more planned restart so the next boot runs it. That second restart is a
normal planned boot with a marker and an `installed` record: it resumes nothing.
Cost: a restore that changes the exec restarts twice.

Consequence for the prune: the restored release is neither current nor changed
until step 5, and the boot prune (§1.1.1) runs before the step. Step 3's pinned
check is what catches the race; accept-time pinning is necessary, not sufficient
(§2.4.3).

### 2.6 Refusing to boot, and what "boot" does when the step cannot finish

A boot whose step fails in 4 or 5 does not open the store: it exits non-zero
with the act's name, the way `boot.fatalf` does for an unopenable store
(`run.go:255-257`); systemd `Restart=on-failure` (`nix/autoputer-vm.nix:1247`)
runs the step again, which resumes at `installing`. This is bounded by the
runtime unit's own restart rate (inference: the unit has `RestartSec = 1`
and no start limit set in the lines read; whether systemd's default start-limit
applies I did not read). A persistent failure (a full disk, a read-only volume)
leaves the computer down with `installing` on disk and a log line naming the
act and the failing rename, which is a visible failure instead of a silent mix.
The operator action is §2.10. This is the cost of having no undo: the
alternative (opening an old or empty store after a half install) is the mix.

### 2.7 Interplay with a staged compact copy and with a planned restart

- **Compact copy.** Its swap is `state.db` only and it carries two tables from
  the original (`compact.go:657-661`, `:716`). If it ran first, a restore install
  would replace the file it just swapped in, and the boot's open fallback
  `RestorePrecompact` (`run.go:248`) could put the pre-compact original over the
  restored store. So when an intent is open or an install happened this boot, the
  step passes `planned = false` to `RebaseOntoCompactCopy`: it then only finishes
  or undoes a crash-interrupted swap (`compact.go:595-615`) and leaves the staged
  copy. The next planned boot judges that copy against the restored store and
  discards it, because it is past the store (`compact.go:560-561`) and a restore
  voids the file-identity anchor (`:547-549`, documented there). **Order matters
  once more:** a crashed compact swap (state file absent, rollback present) must
  be finished by `RebaseOntoCompactCopy`'s first branch *before* the restore
  install renames `state.db`; so the restore step runs after that call returns
  with `planned=false`, not before it. (The call order at `run.go:220` is then:
  compact crash-finish only, restore install, store split, open.)
- **The two direct-SQL tables.** The compact rebase carries `co_super_slots` and
  `agent_evidence` from the original because the tape does not hold them
  (`compact.go:700-716`). A restore replays only the tape, so they come from the
  staged store, empty. A computer holding rows in them is not replay-eligible
  (`replay_eligibility.go:69`, `:90`: `ReplayEmptyUntilSupported`); acceptance
  does not read eligibility (§6). Stated, not changed.
- **A deploy's planned restart**: §2.3.
- **Acceptance while a marker is pending** is refused (409 `restart_already_planned`),
  so an act is never accepted into a restart that will happen before its replay
  finishes.

### 2.8 The quarantine of the old store

Today every successful restore keeps a store-sized `rematerialize-quarantine-*`
that nothing prunes and the disk floor does not count (reviews of rounds 2 and 3).
Here: the old store is renamed, not copied (no bytes at the swap); it is the
rollback of this install and is removed **after the boot replay is done**, the
same lifecycle as the compact rebase's `.precompact` (`compact.go:915-925`,
`telemetry_retire.go:293-295`), so its lifetime is one boot. Earlier quarantines
(from before this change, or left by an install whose replay never finished) are
removed at step 6 once this outcome is durable. After the replay the owner has no
pre-restore copy: the tape holds everything the pre-restore store held except the
two direct-SQL tables (§2.7), and "undo is another restore" (audit design, the
forward-restore paragraph, `computer-history-restore-audit-2026-10-30.md:208-233`).
This is the owner's decision (open question 4), with the conservative default
above.

Disk accounting: peak use is live + staged during the replay (what the floor
already requires at acceptance: `restore_act.go:660-683`, live `state` and
`-wal` plus 2 GiB), and old + restored from the swap to the end of the boot
replay: the same two stores. The floor changes in one way: it adds the workspace
directory and any quarantine already on the volume to what it counts (the round-3
review's caveat: it counted only `state` and `-wal`). **Inference:** this is the
only floor change needed; I have not measured a workspace.

### 2.9 What the CLI and the status say

States the status route returns: `accepted`, `fetching_base`, `replaying`,
`verifying`, **`awaiting_restart`** (intent written; `restart_requested`,
`restart_deferred`), then, after the restart, a closed record: `succeeded`
(installed) or `interrupted` / `failed` (nothing was moved, `live_state:
pre_restore`), or, with the boot step stuck, `installing` with `live_state:
unresolved`. `restored` as a special state is gone: every success is the same
path. The CLI keeps its poll, already tolerant of 502/503 while the guest
restarts (`cmd/choir/computer_restore.go`, `restoreStatusTransient`), prints
`awaiting_restart` and "the computer restarts to finish the restore" once, and
ends at the first closed state; it reads the closed record from the data volume
after the boot, which the status route already does for any ended act
(`restore_act.go:935-981`). Exit 0 only for `succeeded`.

### 2.10 What an operator does when it is unresolved

`/health` lists `restore_unresolved`; the guest refuses new acts. For `installing`
(the boot step stuck): read the record's rename list; move each listed
destination's pair so that every staged item is in place and the old store is in
the quarantine directory, or the reverse if the owner wants the old store, make
`current` the matching release, set the record's `state` to `succeeded` or
`interrupted` and `live_state` to match, restart. A restart re-runs the step,
which finds nothing to do. Nothing is deleted while any record is unresolved.
(The existing runbook, "The runbook for an unresolved act" in the problem record,
describes the same steps for the in-process flip; it is replaced, not extended.)

## 3. What is deleted from lane/restore-act, and what stays

All paths are at `06b742853`.

**Deleted (code).**
- In `rematerialize.go`: the whole install section from `rt.currentReleaseMu.Lock()`
  at `:213` to the success return at `:387`: the `facts` struct and its deferred
  failure handler, `storeClosed`/`leaveAsItIs`, the quarantine `Mkdir`, the
  `flipSteps` list building, `applyRenamesObserved` and its callback, the
  `syncFlipDirs` call, the corrective `flip_started`, the report assembly after
  the flip, `restoreReopenFault`, `store.Reopen`, `RebindProjection`,
  `snapshots.forget`, `selfdev.NewStore`, `restoreExecChanged` and
  `restoreRestaged` there. The engine ends at "staged store closed, lease marker
  written, intent written" (`:202-210`).
- In `restore_act.go`: `undoInstall` (`:1107-`), `installFacts` (`:1080-1090`),
  `errRepairStopped`, `repairRestoreAct` (`:1165-1193`), `ensureReleaseOf`
  (`:1251-`), `errRestoredRestartRequired` (`:217`), `restoreStateRestored`
  and its branch in `runRestoreAct` (`:850-858`), `restoreInstallPhaseKnown`'s
  four old phases, the flip list in the record (`RestoreActFlipStep` stays only as
  the boot's recorded list), the `leaveAsItIs`/unresolved paths of the in-process
  engine, `RepairRestoreAtBoot`'s undo branch.
- In `rematerialize.go`: `applyRenamesObserved`, `undoRenames`, `renameStep`'s
  in-process use (the type moves to the boot step).
- Seams: `restoreReopenFault`, `restoreInstallCrash` points `mid_flip`,
  `before_flip_completed`, `after_flip_completed`, `after_reopen`,
  `after_undo`, `before_flip_started`; `restoreRepairStepHook`'s `undo` and
  `release` steps (`restore_act_seams.go`).

**Deleted (tests, `restore_act_install_test.go`, `restore_act_test.go`).**
`TestRestoreCrashPoints` (its in-process points), `TestRestoreCrashPointLiveWalAtFlipStarted`,
`TestRestoreEnginePanicsAreSettledByTheRoutine`, `TestRestoreDoubleWriteFailureIsUnresolvedNotTerminal`,
`TestRestoreReopenFailureIsRestoredNotFailed`,
`TestRepairIsUnresolvedWhenThePriorReleaseCannotBePutBack`,
`TestRepairIsIdempotentAndSafeWhenInterrupted` (rewritten for the boot step),
and the `flip_started`/`flip_completed` rows of
`TestRestoreInstallRecordWriteFailureTouchesNothing`.

**Kept unchanged.** The act surface: `acceptRestoreAct` and its validations, the
one-at-a-time slot and same-key return, the durable record writer
(`writeRestoreActRecordDurable`, `syncDirectory`, `restore_act.go:307-351`),
`finish`'s record-then-clean order (`:873-910`), the status route and strict
reader and path validation (`:935-1017`, `:1195-1249`), `restoreUnresolvedNow`,
`restoreLiveWork` (accept check), the disk floor and estimate, the
`RestoreInFlight` unsafe-work wiring and `/health` fields, the replay observer,
`product_api` refusing the status route, the CLI and its act states, the
`/health` and vmctl busy readings, `requestRestoreRestart` and its idle wait
(`self_development_restore_restart.go`), the dry-run `ReconstructThroughTargetObserved`,
`reeng_tracer.mjs`'s act reading, and every test of those.

**Added.** `installRestoreAtBoot` (about the size of `RebaseOntoCompactCopy`'s
crash-finish branch plus a list of renames; **inference** from its structure, not
a measurement), the `staged`/`installing`/`installed` facts, a pending-marker
check in acceptance, the same-key re-ask of the restart, the quarantine removal at
replay-done, the floor's two additions.

## 4. Failure list (tests written from it before code)

Tests go through the real boot function and a SIGKILL of a child process at each
named step, as the lane's crash tests do (`restore_act_install_test.go`).

Act side:
1. The act ends at `awaiting_restart` with the staged store sealed, the live store
   untouched and `current` unchanged; nothing in the live store, the release or the
   quarantine is touched by the act, even on a panic after the intent.
2. The intent write fails (ENOSPC injected): the restore fails with nothing live
   touched and the staged store is cleaned by the end of the act.
3. A restart that cannot be requested (busy, marker unwritable): `awaiting_restart`
   with the reason; re-posting the same key re-asks and returns the same act.
4. Acceptance is refused while a planned marker is pending.
5. A crash restart (no marker) with `staged`: the act is closed `interrupted`
   (pre_restore), the staged store removed, the live store opens untouched.

Boot step (each: a child killed at the point, then the next boot, a marker or not as
stated):
6. Planned boot, `staged`, everything intact: installed; the restored store is
   open; `current` is the restored release; the record is `succeeded`; the old
   store is in quarantine; the boot replay stops at the restored head
   (**P1**: with the forward event, the head is R).
7. Kill before `installing` is written: next boot (marker or not) → as 5 or 6
   from `staged`.
8. Kill after `installing`, before any rename: next boot with **no marker** rolls
   forward to `installed`.
9. Kill between each pair of renames (all ten positions, including the skipped
   `-wal`/`-shm`): next boot with no marker completes the install; the restored
   store opens; nothing is moved twice; the old store is whole in quarantine.
10. Kill after the last rename before the directory sync; after the sync before
    `current`; after `current` before `installed`: each rolls forward to
    `installed` with `current` equal to the restored digest.
11. Double crash: kill during the roll-forward of 9, then again during the next
    boot's roll-forward: the third boot completes.
12. Kill after `installed` before the open; after the open before the replay
    finishes: the record stays `succeeded`, the quarantine stays until a boot
    whose replay completes, then it is removed.
13. The restored release was pruned at this boot (`staged`, planned): the act is
    `failed: release_pruned`, pre_restore, the live store untouched.
14. A damaged staged `state.db` (header) at `staged`: `failed`, untouched.
15. A rename error (read-only destination) at step 4: the record stays
    `installing`, the boot exits non-zero, a repeat boot retries; no empty store
    is created at the original path; after the fault clears the install completes.
16. A corrupt or unreadable record beside a staging directory: unresolved, nothing
    deleted, restore acts refused, `/health` names it.
17. A record naming a path outside the data volume: unresolved, nothing renamed.
18. A staged compact copy and an intent: the compact step runs `planned=false`,
    its copy is left, and the next planned boot discards it as past the store.
19. A crashed compact swap (state file absent, rollback present) and an intent:
    the compact crash-finish runs first, then the restore install; the final store
    is the restored one.
20. A deploy-refresh marker pending at the install boot: the restore installs,
    then the boot continues as a planned refresh boot.
21. Restored release execs another binary: after `installed` the step writes the
    restore marker and exits; the second boot (marker) finds no intent and opens;
    a crash instead of the second boot finds no intent and opens.
22. The restored head survives the boot replay (the test that fails today because
    `Reconstruct` replays to the canonical head): needs **P1**.
23. A restore to a checkpoint more than `MaxRecoveryTailEvents` behind the head is
    refused at acceptance, not at the next boot (**new**: the up-front check the
    plan lacks, `recovery_plan.go:131-134`; needs a decision with P1).
24. The disk floor counts the workspace and an existing quarantine.
25. Status and CLI across the restart: 502/503 tolerated, `awaiting_restart` shown
    once, the closed record read after the boot, exit codes per state.

## 5. Class, surfaces, rollback, proof, cost

**Mutation class: red.** Protected surfaces: the boot's store open path and the
files it renames (the store, the workspace, the state file and its WAL), the
`current` release pointer and the planned-restart marker (consumed at
`run.go:632`), restore and rematerialize, the act record, the restart request,
and (through `/health`) vmctl's idle reading. It also touches checkpoint and
route projection indirectly (the restore's release identity). Two seats, and a
re-review after a blocking finding. This document is green.

**Rollback.** Revert. Order by order: a new guest reverted while an act is open
leaves a `staged` record and a staged store that the older binary ignores (it has
no boot step): remove `rematerialize-staging-*` and the record by hand; nothing
else moved. A guest reverted while `installing` is on disk is the dangerous case:
the old binary opens whatever sits at the original path, possibly none; the
operator runs the §2.10 steps before restarting the old binary. Rolling the host
(proxy, vmctl) back needs nothing (no host change in this design beyond what the
lane already carries).

**Proof, d@e only, after landing, before the paper** (commands to be refined with
the lane; nothing here runs now):
1. `choir computer restore --computer D --checkpoint-file ckpt.json --no-wait`:
   202, `awaiting_restart` reached after the replay (about 283-363 s for 5,300
   events), the act's record under `restore-acts/`, the live store and
   `current` unchanged (`choir self-dev head`).
2. Wait for the restart: the computer restarts once (twice if the restored
   release execs another binary), the status reads `succeeded`, `choir self-dev
   head` shows the restored head (with **P1**: R and no replay of (T, H]; without
   it the head returns to H, which is the failing test 22).
3. Kill drill: during the roll-forward window, `choir computer restart` is a crash
   boot. Read the act: `installing` completes, or, killed before `installing`,
   `interrupted` with the live store untouched.
4. Planned-restart exclusion: `choir computer refresh` during `awaiting_restart`
   installs the restore at that boot.

**Cost, plainly: every restore restarts the computer**, once, and twice when the
restored release changes the exec. Today a restore replaces the store under the
running process and restarts only when the exec changes; so for a restore that
did not restart before, this is a new restart: sessions, running runs (closed as
at any restart) and the idle wait (up to an hour for a busy computer, §2.3) are
paid. A busy computer may never reach idle: open question 3.

## 6. Open questions, with the conservative default

1. **Forward semantics (P1).** The boot replays the installed store to the
   canonical head, so without a forward `restore_applied` the restore is undone at
   the install boot and may refuse the boot (§0). Build the forward event first?
   Default: yes, or build nothing. Without it this design installs a release
   pointer and a store that the same boot reverts, which I would not ship. This is
   the largest open item, and it is the owner's or a separate lane's, not this
   lane's.
2. **Does the boot step stay small?** It is one function with three persisted
   states and one direction (forward), closer to the compact rebase's crash-finish
   than to the in-process flip. What it cannot carry is P1. Default: judge the size
   after the failure-list tests exist.
3. **A busy computer** (the paper is always busy,
   `docs/problems/always-busy-computer-never-updates-2026-10-15.md`): the restore
   restart waits for an idle moment for an hour and then gives up
   (`self_development_restore_restart.go:57-60`), unlike a deploy refresh, which the
   host can ask with `allow_busy`. Default: the same rule as today (wait, report,
   re-ask by re-posting); never cut running runs without the planned-restart
   machinery. The owner may rule that an explicit restore is an owner act that
   interrupts runs.
4. **The pre-restore store after the boot replay.** Default: removed at replay-done
   (§2.8), the compact rebase's precedent; "undo is another restore". The owner may
   want the old store kept for a day; that needs a prune rule with a source, and I
   have none.
5. **Acceptance and replay eligibility.** A computer with rows in `co_super_slots`
   or `agent_evidence` is not replay-eligible and a restore would drop them
   (§2.7). Acceptance does not read eligibility. Default: unchanged; named.
6. **The tail bound at acceptance** (test 23): refuse a target whose distance to the
   head would refuse the next boot. Default: refuse, with the numbers.
7. **Marker age.** The restore's marker is written just before the restart and the
   15-minute age (`planned_restart.go:28`) is ample; a restart the updater delays
   past it would read as a crash and drop the intent. Default: unchanged.
8. **The boot-refusal loop** (§2.6): whether systemd's start limit stops the loop is
   not read. Default: a persistent failure is visible in the log and `/health` is
   not reachable; the operator runs §2.10.
