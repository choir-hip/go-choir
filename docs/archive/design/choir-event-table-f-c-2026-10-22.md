# Run telemetry in its own table (F-c): design (2026-10-22)

Date: 2026-10-22. Station: FP1 follow-up, lever F-c of
[`paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22`](../problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md).
Lane `lane/fp1-event-table-design`, from `origin/main` `6b3bdcc1b`.
Round 2 (after review bunch 28): lane `lane/fp1-event-table-design-r2`,
from `origin/main` `4fca1e01b`. Every citation was re-checked on that
checkout.
Mutation class of this document: green (docs only; no code, nothing run on
any computer, no SQL against any store). Mutation class of the change it
designs: **red** (store schema, schema epoch, projection, replay, content
witness, event readers, sequence allocation). Section 6 has the ceremony.

Evidence labels, as in the problem record: **[V]** verified by reading the
code at the cited `file:line` on this checkout; **[M]** measured, in a cited
record, not re-measured here; **[H]** hypothesis or derivation, with the
observation that settles it.

## Status (2026-10-22): deploy 2 built

**Deploy 2 (the writer) is built** on `lane/fp1-event-table-d2`, from
deploy 1's `c4508ef82`, tests first, not landed. It lands only after deploy
1 has landed as its own deploy and held (Q5). Commits: the failure-mode
tests (yellow) and their two repairs (yellow); the store writer (red); the
host grant (red: vmctl); the runtime switch (red); the proof script
`scripts/proofs/fp1-event-table-deploy2.sh` (yellow).

**How the switch works (F18).** The writer stamps only when both keys hold:

- **The host names the computer.** `CHOIR_EVENT_TABLE_COMPUTERS` in
  go-choir-vmctl's Environment (`nix/node-b.nix`, empty as deployed) takes
  ComputerIDs only. `class:test` and `*` are ignored. vmctl writes
  `event-table.json` onto the named computer's root-only credential disk at
  its next cold boot, beside the visual-capture, mirror and host-fetch
  grants (`internal/vmmanager/event_table_grant.go`). A snapshot resume
  keeps the old disk. The guest reads the grant once per boot and fails
  closed (`internal/eventtablegrant`): missing, malformed, withheld, or
  naming another computer is off.
- **The owner sets `store.event_table`.** It is set through `POST
  /api/computer-settings`, stored on the tape, applied at once, and read
  again at every boot before the first event (beside `telemetry.batch`).

Who cannot turn it on: a desk or model, because `choir.ProductAPI`'s
allowlist refuses the settings route; any in-guest process, because
`server.AuthenticatedOwner` refuses loopback callers; an unauthenticated
caller; and any account owner whose computer the host did not name.
`/api/runtime/observability` shows `event_table {host_granted, setting,
writing, reason}` and `telemetry_batching.tape_ops_by_table`. Both are
absent where the switch is off, so the response is deploy 1's there.

**Turning it off** (the owner's setting off) stops new stamps at once. Rows
already in `choir_events` stay there and stay readable through the union
reads; nothing is moved back. Removing the computer from the selector also
stops stamps, but only from its next cold boot.

**How the test computer is selected.** FP0 mints it (`node
scripts/qa_test_computer.mjs --label fp1-event-table-d2`), names its
ComputerID in `CHOIR_EVENT_TABLE_COMPUTERS` (a nix commit, deployed), cold
boots it (the proof's step 5, an operator refresh), and sets the setting
with that computer's key (step 2). The paper comes later: it is named the
same way and takes the grant at a planned refresh, under Q7's conditions
(step 8).

**F17, the local journal probe:** measured on the deploy-2 code, local tape harness
(`docs/evidence/fp1-event-table-journal-probe-2026-10-22.json`; 3,000 seed
rows per table, about 2 KB payloads, the arms alternating over three
rounds) **[M]**:

| Arm | Setting off | Setting on | Ratio |
|---|---|---|---|
| Sealed batches of up to 32 ops (per op: b_ce/b_og, the gate) | 87.6 KiB | 18.5 KiB | **0.21** |
| One tape event per op, batching off (per event, the paper today) | 142.5 KiB | 74.4 KiB | **0.52** |
| The same without the trajectory index (3 trees), batched / unbatched | 88.2 / 141.4 KiB | 14.5 / 67.3 KiB | 0.16 / 0.48 |

The gate passes: b_ce/b_og is 0.21 against a bar of 0.5, better than the
predicted 0.35. The per-event ratio is 0.52 (rounds 0.49, 0.55, 0.52). It
saves about 68 KiB of about 143 KiB per batched event. The remainder is
the prepare and finalize overhead per tape event, which F-c leaves as it
is (2.6, assumption 5). Read as "journal per event, on against off, at
most 0.5", deploy 2 sits at the bar, not under it. Q2's price is about
4 KiB per op batched and about 7 KiB per event unbatched. The numbers are
directional only. Rerun after round 2 (`ae12f9db9`; the write path is
unchanged): 87.6 → 18.4 KiB per op (**0.21**), 137.9 → 70.5 KiB per event
unbatched (**0.51**), 3-tree 0.16 / 0.48.

**Review findings on deploy 1 that bear on deploy 2 (2026-10-22, before
any fix).** These were recorded first; the dispositions follow in the
commits after this one.

- **A (DeepSeek, high).** The updater's automatic recovery bypasses the
  epoch fence. `restorePrior` (`updater.go:958-988`) swaps back to the
  prior release and restarts it without `checkStateCompatibility`, which
  only Apply calls (`:291`). The epoch-1 binary's Open then rewrites the
  receipt to 1 (`store.go:718-728`). That is harmless for deploy 1. Once an
  op is stamped, it breaches the floor (section 4.5). Planned: a durable
  floor marker that only stamping binaries write and none lower; Apply and
  `restorePrior` refuse a release below it, or one that declares no epoch
  on a floored store; and Open neither lowers a newer receipt nor runs
  under a floor above its own epoch.
- **B and F (DeepSeek, flash; Sonnet measured).** `readEventTable` decodes
  JSON under the engine mutex. A 4,000-row owner read held it 281 ms, of
  which 228 ms was decode. Planned: scan under the lock, decode after it.
- **C (DeepSeek).** `ListEventsByOwner` cuts og_objects' newest by
  `updated_at` against `choir_events`' newest by `stream_seq`. Under
  timestamp/allocation inversion it can return a row that is not the newest
  by timestamp (pinned by `TestEventTableOwnerNewestCutWithSwappedTimestamps`).
  Planned: name it as a semantic change that becomes live with deploy 2.
  The table has no `(owner_id, updated_at)` index (a fifth tree, declined
  in round 2).
- **D (DeepSeek).** Every dry run reads the projection head, and every
  event read takes the engine lock a second time. Planned: the hold-time
  readings join deploy 2's proof.
- **E (DeepSeek).** F6 read the dry-run position outside the dry run's
  transaction. Deploy 1's round 1b repaired this (`0e8f72cad`, a test hook
  inside `DryRunProjectionBatch`). Planned for deploy 2: the same check
  through the writer.
- **G (Sonnet, a hypothesis not reproduced).** The union reads run two
  statements. A cursor reader (`?after=`, the WS catch-up) could skip an
  og_objects event committed between the two halves when a later
  `choir_events` event was committed before the second. Planned: a
  deterministic test with a hook between the halves, then a fix.
- **H (Sonnet).** A plain git revert shipped by CI's deploy-refresh, and a
  platform-follow offer (which declares no epoch), are refused by nothing.
  The floor must hold on every path: deploy-refresh, platform-follow,
  Apply and `restorePrior`. Planned: the guest-side fences of A cover
  platform-follow, Apply and `restorePrior`. Deploy-refresh boots whatever
  image CI built, and a revert also reverts any fence inside the reverted
  code. So the fence for that path is a CI check, landed on its own: a push
  that lowers `storeschema.Version` fails unless the commit names the
  pre-rollback check in an explicit trailer.

**Dispositions of findings A to H (round 2 of deploy 2, rebased on deploy
1's round 1b `fbb6d74a8`).**

- **A, repaired: the store floor.**
  - `storeschema` gains `store-floor.json`, beside the epoch receipt. Only
    binaries that know it write it, none lowers it, and binaries older than
    it never touch it.
  - The writer writes the floor (epoch 2, deploy 1) durably *before*
    switching on. If it cannot, the writer stays off.
  - Open floors a workspace whose `choir_events` has a row (one-row probe),
    and Reopen re-floors an adopted workspace while the switch is on.
  - The updater refuses, on Apply and on `restorePrior`, a release below
    the floor, or one declaring no epoch on a floored store. Both refusals
    come before any mutation, swap or restart. A floored computer whose new
    release fails its probe therefore stays on that release (which is at or
    above the floor) until a forward fix.
  - With no floor (deploy 1 alone) both updater paths keep their behaviour.
  - The floor keys on "the writer was switched on" (and on rows present),
    not on the epoch. The epoch did not change in deploy 2.
  - *Should an old binary refuse to rewrite a newer receipt downward?* From
    this build on, `storeschema.Write` never lowers a newer receipt. A
    binary that predates the rule still does, which is why the floor is a
    separate file. Open also refuses a floor above its own epoch, so a
    deploy-2 binary cannot run on a store a later binary floored higher.
- **B and F, repaired.** `readEventTable` scans bytes under the engine
  mutex and decodes after release (a test hook checks the mutex is free at
  decode).
- **C, accepted as a semantic change, named here.** With the writer on,
  `ListEventsByOwner` (`/api/costs`, the texture diagnosis) cuts og_objects'
  newest by timestamp against `choir_events`' newest by `stream_seq`. When
  emitters' timestamps disagree with allocation order, a row at the cut can
  differ from the newest by timestamp. That is a few rows at the end of a
  10,000-row cost window or an 80-row diagnosis. The alternative is a fifth
  tree, `(owner_id, updated_at)`, which round 2 declined (about 7 to 9 KiB
  per op), and the table's DDL is frozen. The test that pins the
  divergence stays.
- **D, kept in the proof.** Step 9 of `fp1-event-table-deploy2.sh` reads
  every `store.event_table.*` op's calls and holds, with a bar of under a
  second. A cursor read now takes the lock three times (finding G).
- **E, repaired in deploy 1** (`0e8f72cad`). Deploy 2 adds the check
  through the writer: every stamped `tape_seq` is a position the dry run
  saw inside its transaction.
- **G, reproduced and repaired.** The hypothesis holds: with a hook between
  the halves, both `ListEventsAfter` and `ListEventsByOwnerAfter` returned
  a later `choir_events` event without the og_objects event committed
  before it, and the cursor skipped it. The cursor reads now run the
  `choir_events` half, then og_objects, then `choir_events` again. From
  the second read they keep only rows at or below the highest key the
  first two saw, so the cursor never passes an event committed before one
  it returns; the next read returns both. One snapshot (one transaction
  across both halves) was not chosen: the og_objects half goes through the
  object-graph service on another connection. What remains is a gap the
  store has always had, even in one table: numbers are allocated under
  `eventMu` and committed after it, so an event can commit after one
  allocated later (heresy delta: discovered, not closed).
- **H, repaired for every path deploy 2 can reach.**
  - Apply and `restorePrior` are covered by A.
  - Platform-follow offers declare no epoch, so a floored computer refuses
    them until offers declare one (a residual for FP0 before the paper is
    floored).
  - Deploy-refresh boots whatever image CI built, and reverting the
    event-table commits removes every fence inside them. That path is
    guarded by `scripts/check-store-schema-floor.sh`, run in CI from a
    commit of its own. It fails a push that lowers `storeschema.Version`
    unless the head commit carries `Store-Floor-Override: <the pre-rollback
    check and its result>`.
  - The lane's token cannot push `.github/workflows`, so FP0 adds this
    step to the Vocabulary Gates job (which `check`, and so the deploy,
    requires), in its own commit. The patch is
    `docs/evidence/fc-deploy2-ci-schema-floor-step.patch` (`git apply`
    from the repository root):

```yaml
      - name: Run store floor guard
        shell: bash
        env:
          STORE_FLOOR_BASE: ${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha || github.event.before }}
        run: |
          set -euo pipefail
          if [ -n "${STORE_FLOOR_BASE//0/}" ]; then
            git fetch --no-tags --depth=1 origin "$STORE_FLOOR_BASE" || echo "store-floor: base $STORE_FLOOR_BASE not fetched at depth 1; the guard retries deeper and fails closed"
          fi
          scripts/check-store-schema-floor.sh
          scripts/selftest-store-schema-floor
```

  The guard fails closed (round 2 of deploy 2, bunch 148, Devin 1 and
  DeepSeek P2): a non-zero base that cannot be read after a fetch at depth
  1 and a deeper retry is a failure, not a pass. Only an all-zero base (a
  new branch) passes without a comparison. The trailer is read from the
  head commit, so the override works on `push` only.

  Every computer's floor is visible as `/api/runtime/observability`
  `event_table.store_floor` (proof step 10).

**Tests run, round 2 (local, after the VM restart at 17:35 UTC, rerun in
full).**
- Under `-race`, through `~/heavy-mirror.lock`: the store tests matching
  `EventTable|Telemetry|ComputerSetting|FlagOff|Lifecycle|OpenRefuses|EngineMutex`
  (782 s), and the agentcore event-table, settings, telemetry,
  planned-restart, ProductAPI, observability, rematerialize and restore
  tests (215 s). Both pass.
- `internal/updater` and `internal/storeschema` pass under `-race`.
- The whole `internal/store` package passes (435 s).
- `internal/vmmanager`, `eventtablegrant`, `builder` and `platform` pass.
- The repo checks pass, as does the new `check-store-schema-floor.sh`.
  `check-guest-data-fstrim.sh` still needs `nix`.

**Tests run (local).** Under `-race`: the store tests matching
`EventTable|Telemetry|ComputerSetting|FlagOff|Lifecycle` (828 s, pass), and
the agentcore settings, event-table, telemetry, planned-restart, ProductAPI
and observability tests (pass). Without `-race`: the whole `internal/store`
package once; `internal/vmmanager`, `internal/eventtablegrant`,
`internal/types` and `internal/computerevent` (pass). `scripts/check-*.sh`
pass, except `check-guest-data-fstrim.sh`, which needs the `nix` binary this
VM lacks. The flag-off golden tape test passes unchanged.

**Rollback once any op is stamped.** First turn the setting off (live).
Then revert deploy 2's red commits: deploy 1 projects and reads every
stamped op. Deploy 1 is the floor (section 4.5), now enforced: the guest
refuses to open under a lower binary, the updater refuses it on Apply and
on recovery, and CI refuses a push that lowers the epoch without the
override. Keep the CI guard's commit when reverting.

**Where the code differs from this document:**

1. *The selector (4.2).* The setting alone is not enough. A host grant is
   a second required key. With the setting alone, any account owner could
   stamp ops and so make deploy 1 their computer's rollback floor without
   the platform choosing it. This is also what makes deploy 2 inert under
   the metamission's rule ("off unless a selector names a computer").
   Section 4.2's "no environment override" holds: nothing turns it on
   without the owner's setting. The cost: the paper needs a cold boot (a
   planned refresh) to take the grant before its setting can work.
2. *Section 2.2's three conditions are four:* a bound tape, the grant, the
   setting, and the storage class.
3. *W2's stamp is decided in `AppendEvent`, with the numbers,* not at seal
   time. A record queued while the setting is on is sealed stamped even if
   the setting goes off before its seal, and the reverse.
4. *F2 across off and on is exact only with one owner per run.* The
   `choir_events` half of the per-run seed reads the run's maximum over
   every owner (3.5). So after a reseed, a second owner's numbers in a
   shared run skip ahead: a gap, never a duplicate. The differential test
   uses one owner per run, and the duplicate test keeps two.
5. *`tape_ops_by_table` counts stamped ops only* (`choir_events`), and is
   omitted until one is stamped; unstamped ops are `tape_ops` minus it.
6. *The F17 gate quantity.* The design gates on b_ce/b_og, the cost per
   op. The cost per tape event with one op each (batching off, as on the
   paper) includes the prepare and finalize overhead, so its ratio is
   higher. The probe reports both.

## Status (2026-10-22): deploy 1 built

The owner approved the section 8 defaults on 2026-10-22 ("Defaults are
fine"): Q1 K2, Q2 four trees, Q3 `choir_events`, Q4 the multi-row insert is
deploy 3, Q5 deploy 1 ships alone first, Q6 existing rows never touched, Q7
the paper after a test computer passes, Q8 `readDB` under the engine mutex,
Q9 accept and rename, Q10 delete the three methods.

**Deploy 1 (inert) is built** on `lane/fp1-event-table` from `origin/main`
`6c60685cc`, tests first, not landed. Commits: the section 5 tests (yellow),
then five red commits: the DDL, the storage class and epoch 2; the projector
route; the union readers and seeds; the witness rule, manifest class,
fingerprints and both fences; the deletion of the three methods. Deploy 2
(the writer and `store.event_table`) and deploy 3 are not built. F17 (the
journal probe that gates deploy 2) and F18 (the setting) are deploy 2's. Review round 1b (flash, DeepSeek v4.1 flash, Sonnet: accept with edits)
is folded in: the rollback text and recipe below, F14 pinned on the lock,
the stamp guard on unkeyed literals, F6 on the real dry-run position, and
the `ListEventsByOwner` semantic change named. Round 1c (DeepSeek: accept
with edits) is folded in too: the stamp guard is type-based (go/types over
every package that depends on `computerevent`, elided literals included),
the rollback step ships as a tested artifact with an apply script, the
test hook is guarded as test-only, two claims are narrowed, and the
citations are swept. Round 1d (DeepSeek, bunch 84: accept deploy 1, with
edits before the rollback recipe is relied on) is folded in: the rollback
sequence survives its own revert (`apply --from <tip>`, rehearsed end to
end), the guard covers container conversions, fails on type errors and
pins its package set, and the step names its table exactly. The guard
checks every non-test package under `internal/`, `cmd/` and `scripts/`
that depends on `computerevent`, on the host's build tags (linux on the
VM and in CI); files behind other build tags are not read, and a
`ProjectionOp` decoded from JSON can carry a `Table` without any literal.
In deploy 1 the only producer of a projection batch is
`projectionTape.appendOpsAs`.

Deploy 1's staging proof is a no-effect proof:
`scripts/proofs/fp1-event-table-deploy1.sh` (boot log; replay report at
manifest version 4 with `choir_events` neither unknown, missing nor
observed; one finished run's events identical before and after; the
`store.event_table.*` engine ops live with short holds; the paper active
and polling; a test-computer restart, and the owner restore of a
pre-deploy-1 checkpoint on it). Section 6.3 is the proof of deploy 2.

Round 1e (DeepSeek, review bunch 99, confirmed; Medium; fixed): proof step
6 `--phase recovered` cannot run. Its `manual` message
(`scripts/proofs/fp1-event-table-deploy1.sh:148`) is double-quoted, so the
shell runs `$(mktemp -d)` while building the message and then expands
`$T`, which is unbound; under `set -u` the step exits 1 before it prints
the recovery sequence or reads the replay report. Observed on `a6fa313bb`
in a dry run with fake ssh and curl: `line 148: T: unbound variable`,
rc 1. No other double-quoted string in the script carries a `$(` or a `$`
meant as literal text; the other expansions (`$T_COMPUTER`, `$body`,
`$report`) are meant. The rollback recipe itself (the sequence below and
`fc-deploy1-rollback.sh`) is unaffected: only the proof's printed copy of
it is broken. Fixed by escaping the message's `$(` and `$T`, so it prints
as text. `scripts/proofs/selftest.sh` now runs step 6 in both phases in a
dry run against fake ssh and curl and checks that the sequence prints
literally; those cases fail on `a6fa313bb` and pass after the fix.

**What a rollback below deploy 1 means now** (corrected in review round
1b; all three reviewers). A rollback below deploy 1 is **runtime-safe, not
rollback-safe**. Round 1's claim that "an updater-applied rollback below
deploy 1 is refused before mutation" is false on most paths. The epoch
fences exactly one path: the updater's `Apply` of a release that declares an
epoch (`checkStateCompatibility`, called only from `Apply`,
`updater/updater.go:291`). It does not fence:

- a plain git revert shipped by CI's deploy-refresh, which boots the
  platform image and never reads the receipt;
- a platform-follow offer that declares no epoch. The offer may carry
  `store_schema_version` and `min_store_schema_version`
  (`platform/platform_update.go:47-48`, `:235-236`); whether corpusd sets
  them is outside this repository. An offer without them takes the
  no-window branch of `checkStateCompatibility` (`updater.go:442`), and an
  offer that declares epoch 1 is refused like any other;
- the updater's own recovery: a failed health probe after an apply runs
  `restorePrior` (`updater.go:406`, `:958-988`), which swaps back to the
  prior release without `checkStateCompatibility`.

On each of these the epoch-1 binary boots over a store deploy 1 opened. Its
`Open` rewrites the receipt to 1, the empty `choir_events` stays, and its
replay manifest calls the table unknown ("observed workspace contains
tables outside the declared schema boundary"). The computer runs, nothing
is corrupted, and everything fails closed: checkpoint publication is
refused, which stops self-development apply, platform-follow's post-apply
checkpoint and owner recovery checkpoints, until one of:

1. a forward deploy to deploy 1 or later (the empty table is again not
   observed);
2. the rollback recipe below;
3. the restore of a pre-deploy-1 checkpoint, staged without the table,
   whose workspace swap removes it.

After `restorePrior` only 1 and 3 are open: the prior release is an
existing build, and no code shipped in deploy 1 can run inside it.

**The rollback recipe.** A rollback below deploy 1 is the revert of the
deploy-1 commits **plus one boot step** added to `bootstrap` after the
`retiredRuntimeTables` loop (the same pattern): drop `choir_events` only
while it is empty. A row means an op was stamped: the computer is below the
floor, the table stays, and the repair is forward (section 4.5).

The step ships as a tested artifact (round 1c, DeepSeek finding 2), not as
a snippet here:
- `internal/store/testdata/fc-deploy1-rollback/fc_deploy1_rollback.go`,
  the whole file the revert commit adds to `internal/store`. It needs only
  `database/sql`, `errors` and `log`, which the pre-deploy-1 store package
  already imports.
- `scripts/rollback/fc-deploy1-rollback.sh apply --from <tip>`, run after
  the git revert (it refuses while `choirEventsDDL` is still defined): it
  reads the file from the deploy-1 tip with `git show`, inserts the boot
  substep `runtime-schema/drop-empty-choir-events` after the
  `retiredRuntimeTables` loop (failing unless it finds exactly that one
  anchor, unchanged since `6c60685cc`), and builds `internal/store`.

**The sequence** (corrected in round 1d, DeepSeek). Deploy 1 adds the
script and the artifact, so the revert removes both before `apply` could
run from the tree. The rollback therefore takes both from the deploy-1
tip, `TIP` being the last deploy-1 commit on main:

```sh
T=$(mktemp -d)
git show "$TIP:scripts/rollback/fc-deploy1-rollback.sh" >"$T/fc-deploy1-rollback.sh"
git revert --no-edit <deploy 1's commits>
bash "$T/fc-deploy1-rollback.sh" apply --from "$TIP"
git add internal/store/store.go internal/store/fc_deploy1_rollback.go
git commit -m "rollback below F-c deploy 1: drop an empty choir_events at boot

Store-Floor-Override: pre-rollback check done at <time>: every computer reports event_table.store_floor == 0 (<how many read>)"
```

**The override trailer** (round 2 of deploy 2, bunch 148, DeepSeek P1). The
revert lowers `storeschema.Version` 2 to 1, and CI's store floor guard
(`scripts/check-store-schema-floor.sh`, section 4.5 and the deploy-2
status) fails a push that lowers it unless the head commit carries
`Store-Floor-Override`. Before the trailer, the documented commit was
CI-red. The trailer is valid only after the pre-rollback reading: read
`/api/runtime/observability` `event_table.store_floor` on every computer,
the paper included, and write it only while every one reports
`event_table.store_floor == 0`. A computer with a floor above 0 has
stamped events; deploy 1 is its floor and the repair is forward. The
script's `rehearse` mode now makes this commit and runs the guard over it:
it must pass with the trailer and fail without it. The trailer is read
from the head commit of a push, so it does not show on a `pull_request`
event.

`fc-deploy1-rollback.sh rehearse TIP BASE` runs that sequence in a
throwaway `git clone --shared` (nothing in the working tree changes): it
checks that the revert removed the script and the artifact, applies
`--from TIP`, and checks that `choirEventsDDL` is gone, that the step is in
`bootstrap` once, that the installed file is byte-identical to the
artifact at `TIP`, and that `go build` and `go vet` of `internal/store`
pass. On the lane, `rehearse 051bc0af2 6c60685cc` reverted 20 commits and
passed in 9 s. FP0 runs it again on main's landed range before relying on
the recipe.
- Three tests keep it from rotting. `TestEventTableRollbackRecipe`
  rehearses the function. On a deploy-1 store with no stamped op, the drop
  gives back a witness equal to one the deploy-1 extractor takes of the
  same store before the table existed (`WitnessContentMatches`); that is
  content-equivalent to the old extractor's witness, because the
  optional-table rule is the only difference and the table is absent. A
  second run is a no-op, and a table holding a stamped row is left alone.
  `TestEventTableRollbackArtifactIsTheRehearsedFunction` keeps the shipped
  file identical to the rehearsed function.
  `TestEventTableRollbackArtifactBuildsInTheStorePackage` runs the script's
  patch on `store.go`, checks the call lands in `bootstrap`, and builds the
  store package with it through `go build -overlay`.
  `TestEventTableRollbackRecipeNamesTheTableExactly` (round 1d) checks the
  step names the table exactly: `SHOW TABLES LIKE 'choir_events'` reads the
  underscore as a wildcard, so beside a lookalike table and with
  `choir_events` gone it would fail a boot; the step reads
  `information_schema.tables` by exact name.

Its cost at boot is one `information_schema.tables` count and, for an empty
table, one probe and one drop of an empty table; it never reads
`og_objects`.

**Decision: shipped as a tested artifact, installed only by a revert; not
in deploy 1's code.** The drop must run in the binary below deploy 1, and
deploy 1 cannot put code into an older binary. Inside deploy 1 it would
drop and recreate an empty table on every boot to no effect, and a revert
would remove it anyway. A revert must run the script: a bare revert leaves
publication refused until a forward deploy. The artifact does not help the
`restorePrior` path, as above. The proof script's step 6 rehearses the
previous build over a deploy-1 store on a test computer, and its
`recovered` phase names the sequence above.

**Open for the panel** (round 1c, DeepSeek, dissent): fencing
`restorePrior` with `checkStateCompatibility`, so that the updater's
automatic recovery cannot reach a binary below the store's epoch, is not
part of deploy 1. If the panel wants automatic recovery closed before any
op is stamped, it is a gate for deploy 2's card. DeepSeek (bunch 84) sides
with deferral: deploy 1 writes no rows, so the downgrade is content-safe
and fails closed, and a fence would also refuse the updater's own recovery
on an epoch-2 store, wedging a computer whose new release failed its
probe. Deploy 2's card owns the decision.

Once deploy 2 stamps any op, deploy 1 is the floor (section 4.5). The
recipe does not apply there: the table is not empty, and the old binary
would also hide stamped events and could hand out duplicate numbers.

**Tests first, and what was shown red at the base** (round 1b, DeepSeek).
The section 5 tests were committed before the code (`2b30186a9`), and at
the base they do not compile, because they name the API the red commits
add. A behaviour failure on the base `6c60685cc` is shown only for F5, the
characterization of the old projector, which passed there by design
(Validate passes a stamped op, and `projectBatch` puts it in `og_objects`).
No other test was run red against stubs. Later behaviour failures came
from the tests themselves: F16 failed on the first query texts (point 1
below) and the whole-package run failed on a test repair (`17736c304`).

**A semantic change that becomes live in deploy 2** (round 1b, DeepSeek).
`ListEventsByOwner` merges the newest `limit` rows of `og_objects` by
timestamp with the newest `limit` rows of `choir_events` by `stream_seq`,
then sorts by timestamp and cuts. Where a caller's timestamps run against
allocation order, which happens because emitters set `Timestamp` before
`AppendEvent` takes `eventMu`, the `choir_events` half can leave out a row
whose timestamp is newer than a row it returns. The result is then not the
newest `limit` events by timestamp. Before F-c it was, for every row
`AppendEventOG` wrote: its `updated_at` is the event's `Timestamp`
(`graph_store.go:1168-1186`). Not for the lifecycle fold's rows, whose
`updated_at` is `run.UpdatedAt` (`lifecycle.go:3331`), so those already cut
on a key other than `Timestamp` (round 1c, DeepSeek). In deploy 1
no row is stamped, so nothing changes. From deploy 2's first stamped op
this affects `/api/costs` (limit 10,000) and the texture diagnosis view
(limit 80) at the cut, by the rows whose timestamps were inverted.
`TestEventTableOwnerNewestCutWithSwappedTimestamps` pins it (section 4.4
gave the reasons a fifth index or a `stream_seq` order were declined). It
is in the heresy delta as introduced.

**Where the code differs from this document** (each is in the code and its
tests):

1. *Plan shapes (section 9's open item, F16).* Dolt reads `(run_id, seq)`
   in index order, with no TopN, only when the `ORDER BY` names the whole
   index prefix (`ORDER BY run_id, seq`, not `ORDER BY seq`). Every
   `choir_events` query does so. `SELECT COALESCE(MAX(seq), 0) ... WHERE
   run_id = ?` (section 3.5) plans as a GroupBy over every row of the run,
   not a seek, so the per-run seed is `ORDER BY run_id DESC, seq DESC LIMIT
   1`, and no row means 0 (the NULL case of round 2 handled the same way).
2. *Fingerprints (3.3 item 3).* `canonical_id` is not unique in
   `choir_events`, so a fingerprint there is named by tape position,
   `choir_events/<tape_seq>/<op_index>`, with a new `Table` field.
3. *The V1→V2 exception of the fence guard (3.6, F11)* is four functions,
   not one: `planOGMigration`, `ogResolveLiveCollision`,
   `applyOGMigration`, `revertOGMigration`. Both fences read
   `objectRowTables`.
4. *Ties.* The union reads break ties by `stream_seq` (or `seq`), so their
   order is deterministic; before, `sort.Slice` left equal keys in the
   og_objects read order. Only rows with equal sort keys can move.
5. *F5* was run on `6c60685cc` before any change (the full `projectBatch`
   put a stamped op in og_objects); the test now pins the old route,
   `projectObject`, which deploy 1 keeps.
6. *F1 after a reconstruction* and *F3* replay through `FinalizeReplayBatch`
   on a frozen tape; the store's recording CAS is not an `EventSource`, so
   `ReconstructInto` itself is not driven from the store package. *F7*
   re-finalizes and re-projects at the same position; it does not simulate
   a crash between commit and return. *F8* with the setting on is deploy
   2's; deploy 1 has a repository-wide guard that no production code sets
   `ProjectionOp.Table`.

## Answer in brief

1. **What moves.** Only the ten batched telemetry kinds, about 94% of
   `choir.event` appends on staging **[M]**. They are named by a new closed
   storage class beside the batching class
   (`internal/types/event_durability.go:31-45`), which today names the same
   ten kinds. Run-state, acknowledged and evidence events stay in
   `og_objects` exactly as today.
2. **How a row gets there.** The writer stamps the existing, unused
   `ProjectionOp.Table` field (`internal/computerevent/projection_batch.go:68`)
   with `choir_events`. The op kind is still `object_recorded` and the body
   is still the same `objectgraph.Object`. A new binary's projector routes
   on the stamp. An old binary ignores the field and upserts into
   `og_objects` as it does today. The tape stays the single authority, and
   replay is a function of the tape alone.
3. **The table.** `choir_events`. Its primary key is the tape position
   `(tape_seq, op_index)`, which is sequential and unique by construction.
   It has three secondary indexes, one per reader shape:
   `(run_id, seq)`, `(owner_id, stream_seq)` and `(trajectory_id, stream_seq)`.
   That is **4 prolly trees per insert instead of 13**.
4. **Estimate.** About 30 to 38 KiB of journal per batched op instead of
   86 to 115 KiB, about 65% less **[H]**. For one telemetry record, including
   the per-event overhead, that is about 98 KB to about 43 KB with batching
   off, and about 88 KB to about 33 KB with batching on (staging
   constants). On the paper the whole journal falls **about 20 to 35%**
   **[H]**: 1.6 GiB per 10 minutes becomes about 1.05 to 1.3 GiB. The low
   end depends on how much of a paper event's cost is the `og_objects` row,
   which is not measured. The paper's own readings decide, not this
   estimate (section 2.6). F-c alone does not end the F2 stall. With F-a
   (after the drain fix) and a multi-row insert into the new table (F-e on
   `choir_events`, deploy 3) the fall is about 40 to 50% **[H]**.
5. **Rollout.** Three deploys:
   - Deploy 1 is inert. It adds the table, the projector route, the union
     readers, the witness and the manifest, and bumps the store schema
     epoch from 1 to 2. Deploy 1 is the rollback floor once any op is
     stamped.
   - Deploy 2 adds the writer behind a per-computer setting,
     `store.event_table`, which is off by default and is an owner act on the
     tape. It runs on one test computer first.
   - Deploy 3 is the multi-row insert.

   Existing `choir.event` rows in `og_objects` are never touched. Readers
   read the union of the two tables.

## Round 2 changes

A three-reviewer panel (bunch 28: DeepSeek v4.1 flash, Gemini 3.8 flash,
Devin; MiniMax advisory) approved with changes. Each finding was checked
against the code on `4fca1e01b`. "Declined" rows give the reason and the
code. The reviewers read `96a82ae33`; where their line numbers drifted,
this document cites the current ones.

| Finding | Reviewer | Disposition | Where |
|---|---|---|---|
| F1: a rollback below deploy 1 can split the store for ever, because an old binary projects stamped ops into `og_objects` | DeepSeek | **Adopted.** Caveat, a pre-rollback check, rebuild-from-tape as the repair. One correction: at boot the old binary discards a prepared row and replays the event from the tape (`appender.go:778-787`, `:856`; `autoputer/run.go:744-746`); it does not re-finalize it. The outcome is the same | 4.5 |
| F1: decide on a `storeschema.Version` bump | DeepSeek, Devin, MiniMax | **Adopted.** 1 → 2 in deploy 1. It fences updater-applied releases only (`updater.go:456-457`); `Open` writes the receipt and never reads it (`store.go:718-728`) | 4.1, 4.5 |
| F2: `projectChoirEvent` must not refuse on `Batchable()`; add a named storage-class predicate | DeepSeek | **Adopted.** The projector accepts any structurally valid `choir.event`. New closed class `EventKind.Storage()` | 2.1, 2.2, F6, F9, F22 |
| F3: the trajectory union returns the oldest N, not the newest | DeepSeek | **Adopted in part.** The defect is real for a trajectory over 10,000 events. The proposed fix is declined: today keeps the oldest N of its newest 10,000 (`store.go:2259`, `:2274`, `:2276`), so `DESC LIMIT N` then an ascending merge would return the newest N, a different result. The union now rebuilds today's 10,000 window first. DeepSeek's extra tests are adopted | 4.4, F2, heresy delta |
| F4: the whole-journal estimate is optimistic | DeepSeek; Gemini (dissent); MiniMax | **Adopted.** 20 to 35%, and the paper's readings 1 to 6 decide | Answer 4, 2.6, 6.2, 6.3 |
| F5: `OmitWhenEmpty` as extractor config is a footgun | DeepSeek | **Adopted.** A package-level set inside `computerversion`, not a field. Both construction sites already copy their config by hand (`replay_completeness.go:489-505`, `projectionbase/witness.go:21-35`) | 3.3, heresy delta |
| F6: section 3.3 is wrong in the old-binary direction | DeepSeek | **Adopted.** `WitnessContentMatches` needs equal table counts (`selfdevprotocol/rematerialize.go:76-78`). An old binary's restore of a pre-deploy-1 checkpoint does work when its staged store has no table | 3.3, 4.5 |
| F7: `event_kind` needs a default; `CREATE TABLE IF NOT EXISTS` adds no later columns | DeepSeek | **Adopted.** `DEFAULT ''`. Later columns need `ensureColumns`, later indexes `CREATE INDEX IF NOT EXISTS`, and each one changes the witness | 2.3 |
| F8: future vocabulary migrations and the fence must cover the new table | DeepSeek | **Adopted.** A constraint plus a source guard over one list of object-row tables | 3.6, F11 |
| F9: `desk.cell` and `desk.egress` carry `ChannelID`, so the channel read narrows | DeepSeek, Devin | **Adopted.** Every run-emitted event carries the run's channel (`runtime.go:4631-4641`, `tools_desk.go:1228-1237`). The method is renamed `ListAcknowledgedEventsByChannel`. No runtime refusal: a reader cannot know a channel's kinds without reading it | 1.2, 1.6, 4.4, F10, Q9 |
| F10: one reader-inventory error: an interface declares `AppendEvent` | DeepSeek | **Adopted.** `runSubmissionStore` (`agentcore/runtime_persistence.go:20-25`); only `*store.Store` implements it | 1 |
| Q1 to Q10 positions | DeepSeek | **Adopted:** Q2 measures the empty-`trajectory_id` share; Q4 keeps the dry run on the same statement; Q5 keeps deploy 1 for a week; Q6 notes retention and the witness; Q7 gates on the rollback check and the paper's baseline. Its "F-e on `og_objects` first if the drain fix is close" is noted in section 7, not made the plan | 7, 8 |
| `SELECT MAX(seq)` returns NULL for a run with no rows | Gemini | **Adopted.** `COALESCE(MAX(seq), 0)` | 3.5, F1 |
| The dry run must handle a store with no projection head | Gemini | **Adopted.** `sql.ErrNoRows` means position 1, as finalize (`computer_events.go:171-179`) and the appender (`appender.go:663-664`) already rule | 2.4, F6 |
| The omission probe must check that the table exists | Gemini | **Adopted.** The probe runs only for a table `SHOW TABLES` returned | 3.3, F4 |
| The optional class needs a two-sided rule | Gemini | **Adopted as a clarification.** Today's schema-drift check already makes "present on one side only" ineligible (`replay_eligibility.go:220-229`) | 3.3, F12 |
| Drop `idx_choir_events_trajectory_stream` in deploy 3 | Gemini; Devin (option); MiniMax (measure both) | **Declined.** A fan-out over runs is not provably equal, and a later index change alters the table's `SHOW COLUMNS` and so the witness (`dolt_state_extractor.go:290-333`). The index set is settled before deploy 1. F17 measures a 3-tree variant locally, so the price is known | 2.3, Q2, F17 |
| Q7: F-a stays off on the paper until the drain fix | Gemini | **Adopted** | 4.2, Q7 |
| F17's journal probe should gate deploy 2, not only deploy 3 | Devin | **Adopted.** Local ratio at most 0.5 before deploy 2 goes to staging | 4.1, F17, 6.3 |
| The narrowed replay fence reads `og_objects` only and skips a missing ID | Devin; MiniMax | **Adopted.** It would fail open for a stamped op (`vocab_replay_fence.go:179-183`). The tracker records stamped ops by tape position, and the fence re-reads them from `choir_events` | 1.4, 3.6, F11 |
| Spell out the optional manifest class | Devin; MiniMax | **Adopted.** Three code sites: `Validate`, `classify`, the missing-table loop | 3.3 |
| Pin the union merge rules; deduplicate | Devin; MiniMax | **Adopted.** Each half fetches the limit; merge key per method; deduplicate by `canonical_id`. Declined: a fifth tree `(owner_id, updated_at)` and switching `ListEventsByOwner` to `stream_seq` order. The halves differ only at the cut, and only when callers' timestamps disagree with allocation order | 4.4, F2 |
| A stamp needs a bound tape | Devin | **Adopted.** One sentence | 2.2 |
| Doc errors: the legacy `events` table has 7 secondary indexes, not 2; owner reads use the write connection | Devin | **Adopted** (`store.go:360-364`, `:796-803`; `store.go:704-706`) | 4.4, Q3 |
| Q7: `store.event_table` is orthogonal to the drain bug | Devin | **Adopted** | 4.2, Q7 |
| Rollback below deploy 1 can hand out duplicate `seq` and `stream_seq`; a WebSocket client skips hidden events | MiniMax (advisory) | **Adopted.** Part of why deploy 1 is the floor | 4.5 |
| The `choir_events` DDL is frozen after deploy 1 | MiniMax (advisory) | **Adopted** | 2.3 |
| Path error: `restore_base.go` is in `agentcore` | MiniMax (advisory) | **Adopted** | 3.3 |
| Measure a 3-tree variant on staging | MiniMax (advisory) | **Declined.** It needs a second build on staging. The local probe measures it (F17) | F17 |

## 0. The problem in one paragraph

On the paper, every tape event pays its journal bytes per touched prolly
tree. An `og_objects` row touches 13 trees: the primary key and 12 secondary
indexes. This is in the record's "What one tape event writes on the guest"
section **[V]**, and here in `internal/objectgraph/dolt_store.go:20-36`
(the table and two indexes), `:219-226` (metadata columns), `:250-257`
(record columns) and `:301-328` (the other indexes) **[V]**. Each projected
op is its own statement, and each statement flushes its own root-to-leaf
path in every tree
(`prolly_table_writer.go:320-326`, cited in the record) **[V]**.

On staging, a fit over the batching gate gives about 86 KB of store growth
per op and about 12.5 KB per tape event, with r² = 0.985
(`docs/evidence/telemetry-batching-phase1-staging-2026-10-22.json`,
`journal_effect.fit`) **[M]**. So batching, which removes tape events but
not ops, trims a telemetry record's journal by only about 10%. The lever is
the number of trees an op touches. `choir.event` is the dominant kind by
volume: 58% of the paper's tape events on 10-15, against 31% for
run-memory entries **[M]**.

## 1. Readers and writers (standing question 4)

All production callers go through the concrete `*store.Store`. No interface
declares the event read methods. One interface declares the writer:
`runSubmissionStore` has `AppendEvent`
(`internal/agentcore/runtime_persistence.go:20-25`), and its only
production argument is `rt.store` (`runtime.go:1107`) **[V]**. So the
source guard in F2 scans implementations, not interfaces. The store fields
are concrete: `internal/agentcore/runtime.go:95`,
`internal/apihandler/handler.go:14` and
`internal/textureowner/handler.go:48` **[V]**.

### 1.1 Writers of `choir.event` objects

| # | Writer | Path to the tape | Kinds | After F-c |
|---|---|---|---|---|
| W1 | `Store.AppendEvent`, acknowledged branch (`internal/store/store.go:1964`, `:2100`) | `AppendEventOG` (`graph_store.go:1168-1199`) → `ogPut` → `objectgraph.Service.CreateObject` → `DoltStore.PutObject` → write validator → `interceptOG` (`projection_tape.go:61-89`) → one `object_recorded` op | every kind | Event-table kinds (today the ten batched kinds) on a tape-bound store with the setting on: the op is built directly, as the sealer builds it, then run through `PreflightWrite` and stamped `Table: choir_events`. All other kinds: unchanged |
| W2 | Telemetry sealer (`telemetry_sealer.go:109-134` `newTelemetryRecord`, which calls `buildEventObject`, `graph_store.go:1205-1239`) | the sealed batch under `authority:vm-local-projection-telemetry` (`projection_tape.go:106-109`) | batched kinds only (`recheckTelemetryOp`, `telemetry_sealer.go:486-506`) | stamped when the setting is on |
| W3 | Lifecycle fold: `projectLifecycleRun` (`lifecycle.go:3116`, through `projectLifecycleRunWithRetry`, `:3002`) puts a `buildEventObject` into its `PutBatchConditional` with an absent-condition on the event's canonical id (`:3330-3337`). The other absent-conditions in `lifecycle.go` (`:4394`, `:5037`, `:5113`, `:6209`) are `choir.lifecycle_event` objects, a different kind | `putBatch` → `evaluateConditions` against `og_objects` (`dolt_store.go:1133-1140`, `:1798-1838`) → intercept | run-state kinds: `loop.started`, `activation.passivated`, `loop.completed` and the rest. These are never batched (`event_durability.go`) | **unchanged**: stays in `og_objects`, and the condition keeps its meaning |
| W4 | Projector (`project.go:215-256` `projectObject`, one upsert into `og_objects`; `derived.Maintain` is a no-op for this kind because no `og_key_index` family covers it, `derived_keys.go:76-80`, `:297-305`) | live finalize (`computer_events.go:227`), replay finalize (`:222`), dry run (`:114`) | all | routes stamped ops to `choir_events` (section 2.2) |
| W5 | Vocabulary migration (`vocab_migrate_og.go:347`, rewrites rows in place) | not tape; a one-time V1→V2 cutover | all | does not touch `choir_events` (section 3.6) |
| W6 | Unbound store, the test seam (`projection_tape.go:22-24`) | direct SQL into `og_objects` | all | unchanged: no stamp without a tape |

`HandleInternalRunEvents` POST (maild, `internal/maild/trace_events.go:123-125`)
and every runtime emitter go through W1. No other writer exists **[V]**: the
only code that names `ogKindEvent` is in `internal/store` (`graph_store.go:22`).

### 1.2 Readers through the store's event methods

| Store method (file:line) | Production callers | Surface | Filter | Order / cursor / limit |
|---|---|---|---|---|
| `ListEventsAfter` (`store.go:2126`) | `api_run_events.go:73` (relaxed read) | `GET /api/runs/{id}/events` (`apihandler/routes.go:22`), used by `choir trace` (`cmd/choir/trace.go:327-358`), the Playwright helper (`frontend/tests/helpers/run-trace.js:11-30`), `scripts/reeng_tracer.mjs:436`, `:457`, `scripts/reeng_ttl_session.mjs:101`, `scripts/claude_delegate_probe.py:79`, `scripts/management_latency_probe.py:127`. Acceptance evidence links have the form `/api/runs/{id}/events?after=<seq-1>&limit=1` (`run_acceptance.go:343-353`) | run_id | seq ascending; `after` is a seq cursor; limit up to 1000 (+1) |
| `ListEvents` (`store.go:2120`, delegates to the method above) | `api.go:437` (consistent); `run_memory.go:437`, which is unreachable in production: it is gated on reason `continuation_selection` (`run_memory.go:432-434`) and no caller passes it | `GET /internal/runtime/runs/{id}/events` (`routes.go:56`), desk tooling | run_id | seq ascending; up to 1000 |
| `ListEventsByOwnerAfter` (`store.go:2189`) | `live_ws.go:79` (consistent), then a desktop filter (`live_ws.go:85`, `:174`) | WebSocket `/api/ws?after_seq=` (`routes.go:38`); the frontend cursor is the highest `stream_seq` seen (`frontend/src/lib/Desktop.svelte:981-1043`) | owner | stream_seq ascending, after a cursor; 500 |
| `ListEventsByOwner` (`store.go:2157`) | `apihandler/costs.go:74` (relaxed); `textureowner/texture.go:1969` (relaxed) | `GET /api/costs` (`routes.go:27`), used by `scripts/cpu_efficiency_probe.py:74`; `GET /api/texture/documents/{id}/diagnosis` (`routes.go:82`), which `TextureEditor.svelte:1396` calls with limit 80 | owner | newest first by timestamp; up to 10000 (costs) or 500 (diagnosis) |
| `ListEventsByTrajectory` (`store.go:2253`) | `run_acceptance.go:91` (consistent; it sorts again at `:94`); `texture_workflow_verifier.go:126` has **no caller** | `POST /api/run-acceptances/synthesize` (`routes.go:53`) | owner + trajectory_id | stream_seq ascending; 3000 |
| `ListEventsByChannel` (`store.go:2224`), renamed `ListAcknowledgedEventsByChannel` in deploy 1 | `system1_verb.go:176` only, pinned by the call-site audit (`telemetry_readclass_test.go:23-33`) | the `choir.System1` desk egress; the channel is `system1:<computer>` (`:81`), written by `:323` with kind `EventSystem1Judgment`, which is **acknowledged** (`event_durability.go:66`). The caller keeps only that kind (`:182-184`). Batched kinds do carry channels: every run-emitted event carries the run's `ChannelID` (`runtime.go:4631-4641`, `tools_desk.go:1228-1237`) | owner + channel_id | timestamp ascending; 10000 |
| `ListEventsByTrajectoryAfter` (`store.go:2283`) | **none** (only `telemetry_sealer_test.go:688`) | — | — | — |
| `ListEventsOG` (`graph_store.go:1242`) | **none** (only `lifecycle_test.go:315`) | — | — | — |
| `ListEventsByOwnerOG` (`graph_store.go:1263`) | **none** | — | — | — |

At the deploy-1 head (round 1c citation sweep; DeepSeek finding 6) the
`store.go` lines in this table and in sections 1.3 and 4.1 have moved: the
event methods are now `ListEvents` `:2139`, `ListEventsAfter` `:2147`,
`ListEventsByOwner` `:2188`, `ListEventsByOwnerAfter` `:2225`,
`ListAcknowledgedEventsByChannel` `:2268` and `ListEventsByTrajectory`
`:2306`; `AppendEvent` is at `:1971` with the per-run seed's
`choir_events` half at `:2040` and the stream seed's at `:2084`;
`storeschema.Write` is at `:730`; the engine-mutex sharing at `:700` and
`s.og` at `:711`. `ListEventsByTrajectoryAfter`, `ListEventsOG` and
`ListEventsByOwnerOG` are deleted. Elsewhere, sections 0 to 9 cite
`4fca1e01b`, the round-2 checkout, as their header says.

Every list method first calls `beforeEventRead` (`telemetry_reads.go:41-50`).
This is the sealer's read barrier. F-c leaves it as it is.

### 1.3 Readers inside the write path (sequence seeds)

- **Per-run `seq` seed** (`store.go:2012-2045`). On a run's first append in
  each process it reads `ogListByMetadata(choir.event, run_id, 10000)` and
  takes the maximum `seq` for that owner. With the sealer it adds
  `telemetrySeqGap`.
- **`stream_seq` seed** (`store.go:2054-2074`). On the first append in each
  process it reads `ListObjects{Kind: choir.event, Limit: 500}`, the newest
  by `updated_at` (`dolt_store.go:719-760`), and takes the maximum.

Both read `og_objects` only. **After the cutover they must read both
tables, or numbers are handed out twice** (failure F1, section 5).

### 1.4 Readers that scan `og_objects` for every kind

| Reader | What it does with `choir.event` rows | After F-c |
|---|---|---|
| Vocabulary plan and serving fence (`vocab_migrate_og.go:347`, `:871`; event fields `kind` and `phase` are skipped, `:835-838`), reached from `MigrateAndFenceServingVocabulary` (`autoputer/run.go:693`, `:765`; `projectionbase/rebuilder.go:390`; `agentcore/rematerialize.go:204`). Boot takes a fast path when nothing was replayed (`vocab_migrate.go:219-225`) | scans role leaves | the full fence pass also reads `choir_events` (section 3.6) |
| Narrowed replay fence (`vocab_replay_fence.go`): `noteVocabReplayDeposit` records the canonical id of every replayed object op (`:104-109`), and `vocabReplayStaysFenced` re-reads each id from `og_objects` only, skipping one it cannot find (`:179-183`). Missed in round 1 | re-checks the role leaves of replayed rows | **fails open** for a stamped op unless changed: its row is not in `og_objects`, so it is skipped. Section 3.6 records stamped ops by tape position and re-reads them from `choir_events` |
| `ListObjectGraphFingerprints` (`object_graph_fingerprints.go:27-60`), used **only as a diagnostic** when live and replay differ (`replay_completeness.go:352-361`, `checkpoint_restore_bindings.go:181-182`) | fingerprints every row | gains a `choir_events` fingerprint list |
| `ReadObjectSnapshot` (`dolt_store.go:767`), `snapshotResidueObjects` (`residue_import.go:240-247`) | filter on `owner_id AND computer_id`. Events carry `computer_id = ''`, because `ogPut` sets none (`graph_store.go:91-111`, `service.go:125-136`) | not affected |
| Content extractor and replay eligibility | hash and classify every table (section 3.3) | explicit wiring |
| `og_key_index` (derived keys) | no family for `choir.event` (`derived_keys.go:76-80`) | not affected |

### 1.5 Surfaces outside Go

The frontend reads events only through `/api/ws` catch-up and the texture
diagnosis. CLI, scripts and Playwright read `/api/runs/{id}/events` and
`/api/costs`. **No route, cursor or response shape changes**, so none of
them change. No script or proof queries `og_objects` for `choir.event`
(`scripts/proofs/fp1-store.sh:64` checks only the generated columns) **[V]**.
Live WebSocket delivery comes from the runtime bus, not the store
(`runtime.go:1970`), so it is not affected.

### 1.6 What the readers need: the index set

| Need | Readers | Served by |
|---|---|---|
| A run's events in `seq` order after a cursor; `MAX(seq)` for a run | `ListEventsAfter`, `ListEvents`, per-run seed | `idx_choir_events_run_seq (run_id, seq)` |
| An owner's events in `stream_seq` order after a cursor; an owner's newest N | WS catch-up, costs, diagnosis | `idx_choir_events_owner_stream (owner_id, stream_seq)` |
| A trajectory's events in `stream_seq` order | acceptance synthesis | `idx_choir_events_trajectory_stream (trajectory_id, stream_seq)` |
| The store-wide newest `stream_seq` | stream seed | the primary key read backwards (newest 500 by tape position) |
| A channel's events | System1 egress only, whose kind is acknowledged | **none**: post-cutover channel reads stay on `og_objects`, and the method's new name says it covers acknowledged kinds only (section 2.1) |

### 1.7 Deletion citers (deleted in deploy 1, not extended)

`ListEventsByTrajectoryAfter`, `ListEventsOG` and `ListEventsByOwnerOG` have
no production callers. Each is deleted and its test repaired. They are not
made union-aware. `ogEdgeEventRun` (`graph_store.go:56`) is never written.
It is noted here and left alone. Also noted and out of scope:
`VerifyTextureWorkflow` has no caller, and `compactRunEventLedger` is
unreachable.

## 2. The table

### 2.1 Which events move: the ten batched kinds only

The routing rule is `rec.Kind.Storage() == StorageEventTable`. `Storage()`
is a new closed classification in `internal/types/event_durability.go`,
beside `eventDurability`. Its default, for any kind it does not name, is
`StorageObjectGraph`. Today it names the same ten kinds as `Batchable()`:
`loop.delta`, `loop.progress`, `loop.retry`, `loop.compaction.started`,
`loop.compaction.completed`, `tool.invoked`, `tool.result`, `desk.cell`,
`desk.egress` and `texture.agent_revision.progress`
(`event_durability.go:36-45`). The rule does not depend on whether batching
is on.

Why a second class and not `Batchable()` itself: batching and storage are
two decisions. A later change to the batching table must not move rows
between tables without anyone deciding it. A test pins the two maps as
equal today, so a change to either one fails until someone decides
(round 2, DeepSeek F2). Reasons for the ten kinds:

- **Volume.** On staging, batchable appends were 94% of runtime events
  (`events_appended_batchable` 618 against `acknowledged` 39, evidence
  `8_counters.process_2_before_kill`) **[M]**.
- **Blast radius.** Run-state transitions, owner acts, email and browser
  acts, Texture revision events and System1 judgments stay where every
  reader and the lifecycle compare-and-write already expect them.
- **The lifecycle fold stays as it is.** It conditions on the event's
  canonical id being absent from `og_objects` (`lifecycle.go:3336`) and
  carries only acknowledged kinds.
- **The channel reader needs no index.** Its one caller reads an
  acknowledged kind (`event_durability.go:66`). Batched events do carry
  channels (section 1.2), so the method is renamed
  `ListAcknowledgedEventsByChannel` to say what it covers.
- **The class discipline already exists.** The sealer has five layers,
  including a forcing test over every `EventKind`
  (telemetry-batching design §3.4). The storage class copies that
  discipline: a parser test fails if a kind has no storage entry.

### 2.2 Routing: the op carries its table

`ProjectionOp` already has `Table string \`json:"table,omitempty"\``
(`projection_batch.go:68`). Nothing in production sets it. `Validate` reads
it only to refuse the two presence tables (`:194-197`). An op without it
serializes byte for byte as today **[V]**.

- **Writer (deploy 2).** The writer stamps `Table: "choir_events"` only
  when all three hold: the store is bound to a tape
  (`s.projectionTape != nil`, `store.go:1998`), `store.event_table` is on,
  and the kind's storage class is `StorageEventTable` (W1, W2). An unbound
  store has no tape position, so it never stamps (W6).
- **Projector (deploy 1).** In `projectOp` (`project.go:69-92`), case
  `ProjectionOpObject`: if `strings.TrimSpace(op.Table) == "choir_events"`,
  it calls `projectChoirEvent(ctx, tx, op.Body, pos, i)`. Otherwise it calls
  `projectObject` as today. Any other `Table` value keeps today's behaviour.
  The projector does **not** start refusing unknown `Table` values: a
  stricter `Validate` could refuse historical tapes on replay.
- **`projectChoirEvent` refuses only shapes the table cannot hold.** It
  never refuses on vocabulary that a later binary may extend, such as the
  event kind. The reason: a later binary may add a kind to the storage
  class, and a
  deploy-1 or deploy-2 binary that replays that tape must not refuse it.
  Refusing it would be replay poison, the failure the stamp was chosen to
  avoid (round 2, DeepSeek F2; an unknown kind is `Acknowledged` and not
  batchable, `event_durability.go:100-112`). It returns
  `ErrProjectionBatchInvalid`, which the dry run turns into `batchRefused`
  before the event is canonical (`appender.go:427-437`), for any op that:
  - is not of object kind `choir.event`;
  - has a body that does not decode as a `types.EventRecord`;
  - has `tombstone` or `superseded_by` set. A row here is an immutable fact
    keyed by tape position, and a later op cannot reach it by canonical id;
  - has a zero `created_at` or `updated_at`;
  - has a `run_id`, `trajectory_id` or `owner_id` longer than 255
    characters, or a metadata `kind` longer than 128. Today's `og_meta_*`
    VARCHAR(255) generated columns refuse the same rows
    (`dolt_store.go:186-190`, `:274`).

  `computer_id` and `version_id` are stored as given (both are columns).
  A metadata `kind` that is absent is stored as `''`. Neither is a refusal,
  for the same forward-compatibility reason.

Why a stamp and not the alternatives (section 7 has the full comparison):

- A new op kind would make an old binary refuse the batch at `Validate`
  (`projection_batch.go:188-190`). After a rollback it could then neither
  finalize nor replay the tape.
- Routing on kind alone in the projector would make a new binary's replay
  of an old tape put historical events into `choir_events` while the live
  store still holds them in `og_objects`. Live and replay would then differ
  for ever.
- A projector that reads the setting row inside the transaction is
  deterministic too. But it couples projection to a row's state and costs a
  read per op. The stamp makes the decision visible on the tape itself.

### 2.3 DDL

Created empty by `bootstrap` (`store.go:765`) in `schemaDDL` (`:253`) as
`CREATE TABLE IF NOT EXISTS`, with the indexes inline. Nothing scales with
`og_objects` at boot. This is the og_key_index precedent
(`docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md`,
"Design").

```sql
CREATE TABLE IF NOT EXISTS choir_events (
  tape_seq      BIGINT UNSIGNED NOT NULL,  -- computer_event_index.sequence of the projection_batch_recorded event
  op_index      INT UNSIGNED    NOT NULL,  -- the op's index in that batch
  canonical_id  VARCHAR(255)    NOT NULL,  -- unchanged content-hash id (not unique, not indexed)
  object_kind   VARCHAR(128)    NOT NULL,  -- always 'choir.event'; kept so a row is a whole Object
  owner_id      VARCHAR(255)    NOT NULL,
  computer_id   VARCHAR(255)    NOT NULL DEFAULT '',
  version_id    VARCHAR(255)    NOT NULL DEFAULT '',
  content_hash  VARCHAR(128)    NOT NULL,
  body          LONGBLOB        NOT NULL,  -- byte-identical to what og_objects.body would hold
  metadata      LONGTEXT        NOT NULL,  -- byte-identical to og_objects.metadata
  created_at    DATETIME        NOT NULL,
  updated_at    DATETIME        NOT NULL,
  tombstone     BOOLEAN         NOT NULL DEFAULT FALSE,
  superseded_by VARCHAR(255)    NOT NULL DEFAULT '',
  run_id        VARCHAR(255)    NOT NULL DEFAULT '',  -- from metadata.run_id
  seq           BIGINT          NOT NULL DEFAULT 0,   -- from metadata.seq
  stream_seq    BIGINT          NOT NULL DEFAULT 0,   -- from metadata.stream_seq
  trajectory_id VARCHAR(255)    NOT NULL DEFAULT '',  -- from metadata.trajectory_id
  event_kind    VARCHAR(128)    NOT NULL DEFAULT '',  -- from metadata.kind; '' when absent
  PRIMARY KEY (tape_seq, op_index),
  INDEX idx_choir_events_run_seq (run_id, seq),
  INDEX idx_choir_events_owner_stream (owner_id, stream_seq),
  INDEX idx_choir_events_trajectory_stream (trajectory_id, stream_seq)
);
```

Every column is a pure function of the op and its tape position. The
projector stamps no `time.Now()`, so no column needs to go into
`IgnoredContentColumns` (`replay_completeness.go:495-503`,
`projectionbase/witness.go:26`). The twelve `og_objects` columns are kept,
so a row reproduces the `objectgraph.Object` exactly: readers decode `body`
(`ogDecode`, `graph_store.go:328`), the vocabulary fence reads
`metadata`, and the diagnostic fingerprints can compare like with like. The
empty columns cost a few bytes per row.

The write is `INSERT ... ON DUPLICATE KEY UPDATE`, keyed on the tape
position. A re-projection of the same event (finalize after
`RecoverPrepared`, `appender.go:772`) is idempotent and never wedges the
computer.

`event_kind` has `DEFAULT ''` (round 2, DeepSeek F7). A metadata `kind`
that is absent is not a refusal (section 2.2). The projector writes `''`
for it, and the default keeps that meaning even for an insert that omits
the column. DeepSeek's concern was poison after the CAS. That cannot
happen while the dry run and finalize share one DDL: the dry run runs the
same insert in a rolled-back transaction (`computer_events.go:108-117`),
so a row the table cannot hold is refused before the CAS. The frozen DDL
below keeps them the same.

**The DDL is frozen at deploy 1.** `CREATE TABLE IF NOT EXISTS` does
nothing on a store that already has the table, so it can never add a
later column or index. A later column must go through
`runtimeColumnMigrations` and `ensureColumns` (`store.go:793`,
`:814-834`), and a later index through `CREATE INDEX IF NOT EXISTS`, the
pattern at `store.go:796-803`. Each such change also alters the table's
`SHOW COLUMNS` output, which enters the witness schema hash
(`dolt_state_extractor.go:290-333`; the `Key` field marks a column that
leads an index). So it makes every earlier checkpoint unverifiable under
the new binary, and it needs its own design. The index set (Q2) is
therefore decided before deploy 1, not after it.

### 2.4 The key: tape position, not `stream_seq`

`stream_seq` is not unique by construction:

- Lifecycle-folded events carry `seq` 0 and no `stream_seq` (evidence
  `duplicates_whole_chain.excluded_note`) **[M]**.
- Both counters are in-memory high-water marks seeded heuristically
  (section 1.3), and are documented as "a gap, never a duplicate", which is
  a property of the allocator and not of stored data.

The tape position is assigned by the one per-computer appender under its
mutex. `AppendNewPayload` holds `a.mu` from `appender.go:402` through
`appendLocked` and finalize (`:458`), and the head CAS rejects a gap
(`computer_events.go:183-185`). This is doctrine C8: one appender owns
semantic ordering. Both the live finalize and replay know the position,
because `finalizeBatch` reads `sequence` from `computer_event_index`
(`computer_events.go:144`). `projectBatch` gains a `pos uint64` parameter
and passes it through with the op index (`project.go:61-65`).

**Dry run.** `DryRunProjectionBatch` (`computer_events.go:97-118`) has no
sequence. It reads `computer_event_projection_heads.sequence + 1` inside its
own transaction. Because the appender holds `a.mu` across the dry run, pin,
prepare, CAS and finalize, that is the event's real position **[V]**. The
dry run's rows are then the same rows in the same tree, and the journal
deduplicates them, as it does today (record, "Correction") **[M]**. Failure
F6 tests that the provisional and final positions are equal.

A store with no projection head row (never projected) returns
`sql.ErrNoRows`. Then the provisional position is 1. That is the rule
finalize already applies to a missing head (`computer_events.go:171-179`:
the first event must be sequence 1) and the appender applies to a nil head
(`appender.go:663-664`) (round 2, Gemini). Any other read error refuses
the dry run.

If the store's head ever lagged the appender's head, the provisional
position would be low. That costs only the dry run's deduplication, never
correctness: the dry run rolls back, and the upsert never fails on a key.
An exact alternative is to pass `event.Sequence`, which the appender binds
before the dry run (`appender.go:409`, `:428-436`), through a widened
`BatchDryRunner`. It is not proposed, because it widens a `computerevent`
interface for a journal saving only.

### 2.5 Trees per insert

| Transaction | Trees today (record) | Trees after F-c, one batched op |
|---|---|---|
| Dry run (rolled back, mostly deduplicated) | 13 | 4 |
| Prepare (`computer_event_index`) | 4 | 4 (unchanged) |
| Finalize: heads, index row and status index, projection | 3 + 13 | 3 + 4 |
| **Per op in the projection** | **13** | **4** |

### 2.6 Journal estimate and its assumptions

The model is J_op ≈ T × c_tree + c_row + c_root.

- **T** is the number of trees.
- **c_tree** is the per-tree path. It is 6.7 to 7.6 KiB at about 10^5 rows,
  measured locally (record, "Local measurement") **[M]**, and about 8 to
  9 KiB at about 10^6 rows **[H]**.
- **c_row** is the row's own new bytes, about 2 to 3 KiB. On staging the
  projection payload averaged 2.97 KB per op in the first segment
  (1,954,086 B / 657 ops), base64 included **[M]**.
- **c_root** covers the table and root chunks, about 1 KiB **[H]**.

| | T | J_op, about 10^5 rows | J_op, about 10^6 rows |
|---|---|---|---|
| `og_objects` row (today) | 13 | about 90 KiB (staging fit: 86 KB **[M]**) | about 114 KiB |
| `choir_events` row | 4 | about 30 KiB | about 38 KiB |
| Saving per batched op | | about 60 KiB (about 66%) | about 76 KiB (about 67%) |

| Per telemetry record, staging-fit constants (a = 12.5 KB per tape event, b = 86 KB per op) | Today | After F-c (b ≈ 30 KB) |
|---|---|---|
| Batching off (k = 1) | 98.5 KB | about 43 KB (-57%) |
| Batching on (k = 5.2, as measured) | 88 KB | about 33 KB (-63%) |
| Batching on plus deploy 3 (one multi-row INSERT per batch into `choir_events`) | — | about 15 to 20 KB **[H]** |

Deploy 3's estimate takes a single-statement amortization of the sequential
primary key (about 7/k KiB per row) and about 0.6 of a leaf per row for each
secondary index, because rows of one run share leaves. The raw-table probe
measured -37% for six rows as one statement on 13 random-key trees (record)
**[M]**.

**Whole journal on the paper (derived)** **[H]**:

- Batched kinds are about 0.58 × 0.94 ≈ 0.55 of tape events (10-15 mix).
- They save about 65% of the `og_objects` row.
- The row's share of a telemetry record's whole cost is the open factor.
  On staging it is about 0.87 (86 of 98.5 KB, the fit). On the local
  harness it is about 0.64 (90 of 141 KiB, record "Correction"). The paper
  has batching off, so every event pays the prepare and finalize overhead
  of about 45 KiB, and its deeper trees and larger rows put a record at
  about 150 to 250 KiB (record, reading 3). With a row of about 114 KiB at
  10^6 rows, the paper's share is about 0.46 to 0.76 **[H]**.
- So the journal falls by about 0.55 × 0.65 × share: about 31% at 0.87,
  23% at 0.64, 20% at 0.55, and 16% at 0.46.
- If run-memory ops (7 trees, record) cost less than `og_objects` rows, the
  batched share of the journal is larger, by up to a few points.
- **Published estimate: about 20 to 35%.** 1.6 GiB per 10 minutes becomes
  about 1.05 to 1.3 GiB per 10 minutes, and the 1 GiB GC pass comes about
  every 12 to 15 minutes instead of about every 10. It is below 20% if the
  paper's row share is under about 0.55, or if its batched share of ops is
  under 0.55 (reading 4).

This band is wider and lower than round 1's 30 to 40%. Round 1 used the
staging row share (0.87) for the paper. DeepSeek (F4, 18 to 30%), Gemini
(22 to 25%, from the same per-event overhead with batching off) and
MiniMax (a band of 20 to 40%) all found that optimistic. **The decision
on the paper rests on the paper's own readings 1 to 6 before and after the
setting write (section 6.3, step 9), not on this estimate.** The staging
ratio b_ce/b_og stays the gate for the test computer.

Each pass still takes about 19 s (16.6 s per GiB plus 2 s, record), so F2
needs the record's stopgaps as well: smaller passes, a non-blocking GC, and
F1's fast 503. With F-a (after the drain fix) and deploy 3 the fall is
about 40 to 50% **[H]**. The next lever by volume is
`run_memory_entries`, 31% of events, with a random UUID key and 6 more
trees; the same pattern applies, in a design of its own.

**Assumptions, each settled by the staging proof (section 6.3), except
items 4 and 6, which only the paper's readings settle.**

1. A secondary index of `choir_events` costs no more per insert than one of
   `og_objects`. Its keys are narrower, so it is probably cheaper.
2. A sequential primary key costs no more than a random one per statement.
   The rightmost leaf is partly filled, so it is probably cheaper.
3. Dolt stores `body` and `metadata` in or out of the row leaf at these
   sizes in a way that adds about c_row. Not read here; this is the ±2 KiB
   in c_row.
4. The batched share of the paper's ops today is close to 10-15's 55%. It
   is not known, and the record's reading 4 gives it.
5. The per-tape-event overhead is unchanged: 12.5 KB on staging, about
   45 KiB locally.
6. The `og_objects` row is at least about 55% of a paper telemetry
   record's cost. It is not read directly. Reading 3 (bytes per event)
   bounds it.

**History.** The 28 to 29 GiB live store is history: each 100-event
checkpoint pins about 100 distinct paths per random-key table (record,
cause 3). A sequential primary key and run- or owner-clustered secondaries
put the 100 events between checkpoints into a few leaves. The live store's
growth should fall more than the journal's **[H]**. This is not estimated;
the paper's `after_journal_mib - (after_avail_mib - avail_mib)` per GC pass
(record, reading 6) measures it.

## 3. Single state authority, replay, witness, restore

### 3.1 Authority

The tape is the only authority for an event, as it is for every other
projection. `choir_events` is a projection of `projection_batch_recorded`
ops, as `og_objects` and `run_memory_entries` are. The two tables never
disagree about one event, because one op lands in exactly one of them: the
stamp decides, on the tape. That holds for every deploy-1-or-later binary;
an older binary ignores the stamp, which is why deploy 1 is the rollback
floor (section 4.5). A reader that finds an event in neither table
is reading a lost-on-crash batched record, which batching already defines
(telemetry-batching design §5.2), or a bug.

### 3.2 Projection and replay

Replay is `ReconstructInto` / `ReconstructThroughTarget`
(`appender.go:1129-1168`) → `FinalizeReplayBatch` (`computer_events.go:87-89`)
→ `finalizeBatch` with the event's own `sequence` (`:144`) →
`projectBatchForReplay` (`:222`) → `projectOp`. That is the same route with
the same stamp and position as the live finalize. A fresh replay therefore
writes the same `choir_events` rows as the live store.

The deposit upcast (`vocab_upcast_deposit.go:556-680`) rewrites `op.Body`
in place and keeps `op.Table`, so upcast replays route the same way **[V]**.
Nothing is truncated: replay writes a fresh workspace (`store.go:536`) or
an installed base plus the tail.

### 3.3 Content witness and replay eligibility

**Does the witness hash change?** Yes, from the first stamped op on.

`DoltStateExtractor` lists tables with `SHOW TABLES`
(`dolt_state_extractor.go:263-286`). It hashes each table's columns and
rows, and builds `content_root` from the sorted per-table hashes (`:212-225`).
The new table enters automatically, and stamped events leave
`og_objects`' hash. Live and replay are always extracted by the same
binary, so they agree.

Three pieces need explicit wiring:

1. **Replay manifest** (`internal/agentcore/replay_eligibility.go:44-94`).
   It is default-deny: an unlisted table is "unknown" and the computer is
   replay-ineligible (`:177-181`, `:245-246`). Ineligibility blocks
   checkpoint publish (`checkpoint_restore_bindings.go:60`, `:187`).
   Restore has its own gate: the staged store's witness must match the
   checkpoint's (`agentcore/rematerialize.go:196`). Declared tables must
   also be present in both observation sets (`replay_eligibility.go:209-219`),
   so item 2 needs a matching rule here.
2. **Old checkpoints.** `WitnessContentMatches`
   (`selfdevprotocol/rematerialize.go:66-92`; equal counts at `:76-78`) and
   `WitnessFromObservationSets` (`selfdevprotocol/restore.go:142-179`)
   require equal table sets and equal content roots. A checkpoint witnessed
   before deploy 1 has no `choir_events`. A deploy-1 binary that replays to
   that head would create the empty table, and every empty table adds one
   schema observation and one line to `content_root`. The restore would be
   refused.

   **Rule: an empty `choir_events` is not observed.**
   - *Where the rule lives.* A package-level set in `computerversion`,
     `optionalTables = {choir_events}`, read inside `Extract` beside the
     `IgnoredTables` filter (`dolt_state_extractor.go:153-161`). It is not
     a field, so no construction site can forget it. Round 1 proposed a
     field, `OmitWhenEmpty`. DeepSeek (F5) showed why that is a footgun: a
     site that forgot it would still pass live against replay, because
     both sides would include the empty table, while every pre-deploy-1
     checkpoint became unrestorable. Both sites today already copy their
     config by hand (`agentcore/replay_completeness.go:489-505`,
     `projectionbase/witness.go:21-35`). A test in `internal/store`
     asserts the set names the table `bootstrap` creates.
   - *How it is checked.* Only for a table that `SHOW TABLES` returned
     (`queryDoltTables`, `:263-286`), so a workspace without the table
     needs no probe and cannot fail one (round 2, Gemini). Then one
     `SELECT 1 FROM choir_events LIMIT 1`. No row: the table is left out of
     both the schema and the content observations, and so out of
     `content_root`, exactly as `IgnoredTables` leaves a table out.
   - *The manifest class.* One new class, `event_projection_optional`, and
     one new entry, `choir_events: event_projection_optional`
     (`replayEligibilityManifestVersion` 3 → 4). It touches three code
     sites. `Validate` must accept it (`replay_eligibility.go:119-123`).
     `classify` admits it with content, like `event_projection` (`:177-196`;
     an unknown class falls to `default` and is ineligible). The missing-
     table loop skips it when it is absent from both sets, like
     `retired_absent` (`:209-219`). Present on one side only is still
     ineligible: today's schema-drift loops already catch it (`:220-229`).
     With a deploy-1-or-later binary on both sides that case cannot arise,
     because the stamp that decides the table is on the tape (round 2,
     Gemini, Devin). The manifest version is reported in the eligibility
     JSON (`:147`) but never persisted or compared, so the bump is local.

   **What the rule buys, and what it does not** (corrected in round 2,
   DeepSeek F6). A new binary reading a store with no stamped op produces
   the same witness that an old binary produced for the same head. So a
   new binary can restore every pre-deploy-1 checkpoint. The reverse is not
   true. An old binary has no such rule. Extracting a store that holds the
   empty table, it sees one more table, so its witness no longer matches
   its own earlier checkpoints, and its manifest calls the table unknown.
   An old binary can still restore a pre-deploy-1 checkpoint when its
   staged store has no `choir_events`: a replay from genesis into a fresh
   store (`agentcore/rematerialize.go:127`), or a base published before
   deploy 1. A base published by a deploy-1 binary carries the empty table
   in its workspace, so an old binary's restore from it is refused
   (`:196`). Section 4.5 uses this.

   The same rule covers the base witness check
   (`projectionbase/verify.go:68-105`) and an old base installed under the
   new DDL (`agentcore/restore_base.go:194`).
3. **Diagnostics.** Today `compareReplayObjectGraph` names the
   `og_objects` rows that differ. Deploy 1 adds the same per-row
   fingerprints for `choir_events`, so an inequivalence names its rows.

**Pre-existing, not introduced.** Checkpoint witnesses are already a
function of the binary's schema. `SHOW COLUMNS` enters the schema hash
(`dolt_state_extractor.go:290-333`), so any added column
(`store.go:814-834`) or table makes earlier checkpoints unverifiable under
the new binary. The closing heresy delta records this as discovered. The
rule above avoids it for this change only.

### 3.4 Restore to a prior event head

Restore stages a fresh store or an installed base. It reconstructs through
the target head (`agentcore/rematerialize.go:127-164`), checks the witness
against the checkpoint (`:183-200`), and then swaps the whole workspace
directory into place (`:227-265`, per the code audit). It uses no Dolt reset.
`choir_events` rows after the target head are therefore not in the staged
workspace. Rows are keyed by tape position, so "after the head" is exactly
`tape_seq > head.sequence`. `Reopen` (`store.go:946-980`) keeps the
in-memory `seq` and `stream_seq` marks, which are at least the restored
store's maxima, so the next numbers leave a gap and never repeat. This is
today's behaviour and F-c does not change it. A process restart seeds from
both tables (section 3.5).

### 3.5 Sequence seeds (the write path's reads)

- **Per-run.** `max(today's og_objects seed, SELECT COALESCE(MAX(seq), 0)
  FROM choir_events WHERE run_id = ?)`. This is one index seek. `MAX` over
  no rows is SQL `NULL`, and scanning `NULL` into an `int64` fails, which
  would fail the append on a run's first event (round 2, Gemini); the
  `COALESCE` makes it 0. The owner filter of today's seed
  (`store.go:2024-2026`) is dropped for the table half. Runless events
  share run_id '' across owners, so the maximum can only over-estimate,
  which is a gap and safe.
- **Store-wide `stream_seq`.** `max(today's newest-500 og_objects seed,
  the maximum stream_seq among the newest 500 choir_events rows by
  primary key)`. This has the same quality as today's newest-by-time
  heuristic: tape order is allocation order except for appends in flight.
- Both keep the sealer's `telemetrySeqGap` and `seqFloor` rules
  (`store.go:2027-2041`, `:2071-2073`).

### 3.6 Vocabulary, fingerprints, residue

- **Fence.** The full fence pass (`servingOGRoleFields`,
  `vocab_migrate_og.go:871`) also reads `choir_events` role leaves, with the
  same skip map. Stamped ops exist only on V2 binaries with the write
  validator (`PreflightWrite`, `dolt_store.go:613-626`), and replayed
  deposits are upcast before they are projected. The V1→V2 row migration
  (`planOGMigration`) therefore does not rewrite the table. The fence makes
  the "never V1" claim fail closed instead of trusting it (F11).
- **Narrowed replay fence** (missed in round 1; Devin, MiniMax). Replay is
  the one writer the write guard does not see (`computer_events.go:207-213`).
  After a replay onto a fenced store, boot re-checks only the replayed rows:
  `noteVocabReplayDeposit` records each object op's canonical id
  (`vocab_replay_fence.go:104-109`), and `vocabReplayStaysFenced` re-reads
  each id from `og_objects` and skips one it cannot find (`:179-183`). A
  stamped op's row is in `choir_events`, so today's code would skip it and
  report the store still fenced. That fails open. The change:
  - `finalizeBatch` passes its `sequence` (`computer_events.go:144`) to
    `noteVocabReplayDeposit` (called at `:211`). The tracker records a
    stamped op as `(tape_seq, op_index)`, in a set of its own under the same
    `vocabReplayTrackLimit`.
  - `vocabReplayStaysFenced` re-reads those rows from `choir_events` by
    primary key, with the same fixpoint and role-field checks. A stamped
    position it cannot find returns `false`, so the full pass decides. It
    is never skipped.
  - Looking a stamped id up by `canonical_id` instead would scan the
    table, because `canonical_id` is not indexed (section 2.3).
- **Future vocabulary migrations** (round 2, DeepSeek F8). The plan
  (`planOGMigration`, `vocab_migrate_og.go:342-352`) and both fences scan
  `og_objects` by name. A later V2→V3 migration written the same way would
  leave `choir_events` rows behind, and the full fence would then fail
  closed at boot (`autoputer/run.go:693-697`, `boot.fatalf`). So this is a
  constraint on every later migration: the plan, the row rewrite and both
  fences cover `choir_events` as well as `og_objects`, and the replay
  deposit upcast covers stamped ops (it does today: it rewrites `op.Body`
  and keeps `op.Table`, section 3.2). Deploy 1 puts the two names in one
  list, `objectRowTables`, and both fences read it. The V1→V2 plan keeps
  reading `og_objects` only, because no `choir_events` row can be V1; it is
  the one named exception, and the full fence covers it. A source guard
  fails on any other `FROM og_objects` in `internal/store/vocab_*.go` that
  does not come from the list, so a later plan must read the list (F11).
- **Fingerprints.** See section 3.3, item 3.
- **Residue import** snapshots legacy direct-written rows into the tape
  (`residue_import.go:51-122`). It must never read `choir_events`, whose
  rows are already tape-derived (F21).

## 4. Migration and rollback

The doctrine rule for this pass is: no boot step scans or rewrites all of
`og_objects`, and schema changes are online and rollback-safe
(`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md:270`).
Round 2 makes "rollback-safe" precise for this change. Each deploy can be
rolled back to the one before it. Deploy 1 can be rolled back to the
previous build only until the first op is stamped; after that, deploy 1 is
the floor (section 4.5). That is the expand-then-write shape: the expand
step is stable before anything writes.

### 4.1 Three deploys

| Deploy | Contents | Effect on a computer |
|---|---|---|
| 1, "expand" (red, inert) | DDL (empty table at boot); `projectChoirEvent` and the route; `pos` through `projectBatch`; union readers (section 4.4) and both seeds; the storage class `EventKind.Storage()`; manifest class and entry; the extractor's `optionalTables` set; both fences and the `objectRowTables` list; fingerprints; `ListEventsByChannel` renamed `ListAcknowledgedEventsByChannel`; the three caller-less methods deleted; plan-shape tests; **`storeschema.Version` 1 → 2** | **None observable**: no writer stamps, the table stays empty and is not observed, and the readers' second query returns nothing. Rollback to the previous build: runtime-safe, not rollback-safe. The updater's Apply refuses an epoch-1 release; a CI revert, a platform-follow offer and `restorePrior` do not, and leave an empty table the old manifest calls unknown, so checkpoint publication is refused until a forward deploy, the rollback recipe or a pre-deploy-1 restore (status section, round 1b). See the rollback matrix |
| 2, "writer" (red) | the `store.event_table` setting in `KnownComputerSettings` (`computer_setting.go:55-61`), applied at once (the pattern of `computer_settings.go:105`) and at boot (the pattern of `telemetry_batching.go:39-56`); W1 and W2 stamp when it is on; a new counter `tape_ops_by_table` in `TelemetryStats` (`telemetry_stats.go:114-131`, `:182-205`). No epoch bump: deploy 2 is backward compatible with deploy 1 | off: byte-identical tape and store to deploy 1. On, per computer: batched kinds go to `choir_events` |
| 3, "F-e on the new table" (red) | in `projectBatch`, consecutive stamped ops of one batch become one multi-row upsert into `choir_events`. The dry run runs the same multi-row statement, so finalize writes the same chunks and the journal still deduplicates them (round 2, DeepSeek Q4) | only useful with batching on; measured on its own |

Deploy 1 must be the stable release, with at least one staging day and one
paper boot, before deploy 2 is anywhere. A rollback of deploy 2 then lands
on a binary that projects and reads the table. **Deploy 1 is the rollback
floor** once any op has been stamped (section 4.5). Keep it as a release
you are willing to roll back to for at least a week after deploy 2
(round 2, DeepSeek Q5).

**Gate for deploy 2** (round 2, Devin). The local journal probe (F17)
runs on the deploy-2 code before it ships. Go bar: b_ce/b_og at most 0.5
on the harness, the same bar staging uses. A local miss stops deploy 2
until the gap is explained. The local number is directional only; it can
stop a deploy, but only staging and the paper can pass one.

**The schema epoch: 1 → 2 in deploy 1** (round 2; DeepSeek F1, Devin,
MiniMax). The epoch's contract asks for a bump whenever a change is not
freely backward compatible for older binaries
(`storeschema/schema.go:17-26`). Round 1 said `choir_events` was freely
backward compatible. That was wrong. An old binary does run on a deploy-1
store, but its manifest calls the table unknown, so it can no longer
publish a checkpoint. After any stamped op it also hides events and can
hand out duplicate numbers (section 4.5).
- *What the bump does.* The updater refuses, before mutation, a release
  that declares epoch 1 (the builder stamps the build's epoch,
  `builder/request.go:240`) on a store whose receipt says 2
  (`updater/updater.go:441-463`, the check at `:456-457`; called from
  apply at `:291`). That fences the updater's Apply of an
  epoch-declaring release, and nothing else (corrected in round 1b): the
  updater's own `restorePrior` after a failed health probe (`:406`,
  `:958-988`) does not call the check, a platform-follow offer may declare
  no epoch, and a CI revert never reads the receipt (status section).
- *What it does not do.* `Open` writes the receipt and never reads it
  (`store.go:718-728`). No path other than the updater reads it (a grep for
  `storeschema.Read` finds only `internal/updater`). So an old binary that
  reaches the computer another way is not refused by code, and its first
  `Open` rewrites the receipt to 1. The pre-rollback check in section 4.5
  fences that path by procedure.
- `min_store_schema_version` does not change: deploy 1 migrates an epoch-1
  store by creating the table. Deploys 2 and 3 do not bump: each is
  backward compatible with deploy 1.

### 4.2 The selector

`store.event_table` is a per-computer owner setting. It is an object of
kind `choir.computer_setting`, written through the tape
(`computer_setting.go:76-96`) and set with
`POST /api/computer-settings {"key":"store.event_table","value":true}`, the
route the batching gate used. It decides only whether the writer stamps.
The projector never reads it. It goes on for one `warmness_class: test`
computer first (minted with `scripts/qa_test_computer.mjs`, as in the
gate), then the paper. Turning it off is the live rollback: new events go
back to `og_objects`, and the union reads keep every event visible. No
environment override is proposed; telemetry.batch has
`CHOIR_TELEMETRY_BATCH`. F-c does not depend on the sealer, so it can go on
the paper with batching still off
(docs/problems/host-planned-restart-skips-telemetry-drain-2026-10-22.md).

The setting is orthogonal to the drain bug (round 2, Devin). It decides
which table a projected op lands in. It does not change which events are
queued, sealed or lost: an open telemetry window is lost on a skipped drain
whether or not its ops would have been stamped. So `telemetry.batch` (F-a)
stays off on the paper until the planned-restart drain fix lands, whatever
this setting says (round 2, Gemini).

**The stream_seq lease gate** (deploy 2 round 3, 2026-10-23; D24's lease,
docs/problems/stream-seq-high-water-mark-design-2026-10-23.md section 4
"When"). Once events move to `choir_events`, a computer without a lease
would seed `stream_seq` by the exact read over both tables at every boot
(104 s on the paper before D24). So the writer stamps, and the setting turns
on, only once the lease state is `ready` with its latest lease row written
through a bound tape:

- `Store.EventTableLeaseReady` returns nil, or `ErrEventTableLeaseNotReady`
  naming the state. It reads the lease's published snapshot and one atomic
  (`lease.onTape`, set where the seed and the refill write the row), so it
  never waits on the allocator. It is the lease's state, not a copy.
- `stampsEventTable` (under `eventMu`) also needs `ready` and `onTape`;
  `EventTableWritesOn` reports the switch and the lease together.
- `POST /api/computer-settings {"key":"store.event_table","value":true}`
  answers 409, reason `stream_seq_lease_not_ready`, and stores nothing
  before that.
- A value stored before a restart arms the switch at boot, and the floor is
  written then. The writer begins when the eager seed lands; until then the
  status reason says the writer waits.
- A lease seeded before any tape was bound is an og row with no tape event.
  It does not admit the writer until a refill writes a row through the tape.

Who owns `maxStreamSeq`: the lease does (`allocateStreamSeqLocked`,
`completeSeedLocked`). Deploy 1's two-table seed is now the lease's exact
seed (`exactTableStreamSeqMax`). The event table adds no allocator and no
second seed, so the two do not disagree.

### 4.3 Existing `choir.event` rows: never touched

The rows already in `og_objects` stay there. Readers read the union. There
are three reasons.

- **Cost.** A move deletes from 13 trees and inserts into 4, about 120 KiB
  of journal per row by the record's constants **[H]**. The paper holds an
  unknown but large number of rows (section 9), so a move costs hundreds of
  GiB of journal and GC.
- **Replay.** A move that is not on the tape makes live (moved) and replay
  (not moved) differ. A move that is on the tape doubles the tape.
- **Doctrine.** The boot rule forbids it at boot, and an online move is a
  long red maintenance job for bytes that are already written.

The `og_objects` half of each union read costs what it costs today, and
its share stops growing. Removing old telemetry from either table is
retention. That is a separate owner decision: how long trace telemetry is
kept.

### 4.4 Union reads

Each method runs its `og_objects` query as today, plus one indexed query on
`choir_events`, then merges and cuts. The rules, pinned in round 2 (Devin,
MiniMax, DeepSeek F3):

- Each half fetches at least what the caller's result can need, never
  fewer than the limit.
- The merged rows are deduplicated by `canonical_id` before any cut. One op
  lands in one table, so a duplicate means a bug or a store an old binary
  touched (section 4.5). The copy kept is the `og_objects` one.
- The cut uses the method's merge key, with `stream_seq` as the tie-break.

| Method | Today's `og_objects` half | `choir_events` half | Merge and cut |
|---|---|---|---|
| `ListEventsAfter` | the run's newest 100,000 by `updated_at`, then `seq > after` (`store.go:2133-2147`) | `WHERE run_id=? AND seq>? ORDER BY seq LIMIT ?` | ascending by seq, first `limit`; the cursor and evidence-link semantics are unchanged |
| `ListEventsByOwnerAfter` | the owner's newest 100,000 by `updated_at`, then `stream_seq > after` (`:2197-2214`) | `WHERE owner_id=? AND stream_seq>? ORDER BY stream_seq LIMIT ?` | ascending by stream_seq, first `limit` |
| `ListEventsByOwner` | the owner's newest `limit` by `updated_at` (`:2162-2166`; `dolt_store.go:743`). `updated_at` is the event's `Timestamp` (`graph_store.go:1169-1172`, `:1186`) | `WHERE owner_id=? ORDER BY stream_seq DESC LIMIT ?` | descending by timestamp, first `limit`, as today |
| `ListEventsByTrajectory` | the trajectory's newest 10,000 by `updated_at` (`:2259`), filtered to the owner, sorted ascending by `stream_seq` (`:2274`), and the **first** `limit` kept (`:2276`): the oldest `limit` of a 10,000 window | `WHERE trajectory_id=? ORDER BY stream_seq DESC LIMIT 10000` (the index serves the term and the order); then the owner filter in Go, as today | first rebuild today's window: the newest 10,000 of the union by timestamp. Then filter the owner, sort ascending by `stream_seq`, keep the first `limit` |
| `ListAcknowledgedEventsByChannel` | as today | none: the post-cutover channel read stays on `og_objects` | — |

**The trajectory method** (round 2, DeepSeek F3, adopted in a corrected
form). DeepSeek is right that round 1's half, `ORDER BY stream_seq LIMIT
?`, changes the result: for a trajectory over 10,000 events it returns the
oldest rows of the whole trajectory, while today returns the oldest of the
newest 10,000. Its fix, `DESC LIMIT limit` and then an ascending merge, is
not today either. Today keeps the oldest `limit` of its window, not the
newest `limit` (`store.go:2274-2277`). For a trajectory of 5,000 events all
in `choir_events`, that fix would return events 2,001 to 5,000, and today
returns 1 to 3,000. Rebuilding the window first gives today's result
exactly, whenever timestamps follow allocation order. Whether acceptance
synthesis should read the newest events instead is a separate reader
question, recorded in the heresy delta.

**The owner method and the cut** (Devin, MiniMax). The `og_objects` half
picks the newest by timestamp, and the `choir_events` half by allocation
order. They differ only when callers' timestamps disagree with allocation
order. That happens because emitters set `Timestamp` before `AppendEvent`
takes `eventMu` (`runtime.go:4639`, `tools_desk.go:1235`), so two
concurrent emitters can swap. The difference is a few rows at the cut of a
cost total (`/api/costs`, limit 10,000) or a diagnosis view (limit 80).
Two other options were declined. A fifth tree, `(owner_id, updated_at)`,
costs about 7 to 9 KiB per op on every row to fix that boundary. Switching
the method to `stream_seq` order changes what `/api/costs` and the
diagnosis return today; that is a reader change, not part of F-c.

**Cursor methods.** For `ListEventsAfter` and `ListEventsByOwnerAfter`
today's half is capped at its newest 100,000 rows, and a cursor older than
that window skips rows today. The `choir_events` half is an exact indexed
cursor read, so the union never skips more than today does.

**Connections.** Today the owner methods and the stream seed read through
`s.og`, a `Service` over the write `DoltStore` (`store.go:704-706`). The
metadata methods and the per-run seed read the read pool
(`ogListByMetadata`, `graph_store.go:296-304`). Both hold the shared engine
mutex (`ShareEngineMutex`, `store.go:693`). Round 1 implied all event reads
were on the read pool; that was imprecise (Devin). The `choir_events` half
goes to `readDB` under the engine mutex, with labelled op names. This
needs a small exported `DoltStore` helper, because the mutex is unexported
(`dolt_store.go:65-69`). The upstream race the mutex was kept for is
recorded in the engine-concurrency design. `run_memory_entries` is read on
the write connection (`run_memory.go:95-130`), but event reads are more
frequent (WebSocket catch-up), so the read pool is the better default.

### 4.5 Rollback matrix

| From → to | Tape | Store reads | Replay and witness | Repair |
|---|---|---|---|---|
| Setting on → off | new events unstamped | union: nothing hidden | equivalent | none |
| Deploy 2 → deploy 1 | stamped ops stay; deploy 1 projects them | union | equivalent | none |
| Deploy 3 → deploy 2 | same op shape; one statement becomes N | same | same rows, so equivalent | none |
| Deploy 1 → the previous build, no op ever stamped | none. The updater's Apply refuses an epoch-1 release on this store (section 4.1); this row is reached by a CI revert, a platform-follow offer that declares no epoch, or the updater's `restorePrior` after a failed health probe (round 1b) | today's | an empty table **unknown** to the old manifest: replay-ineligible, so **checkpoint publish** is refused; the computer keeps running (og_key_index precedent, finding 3). **Restore** of a pre-deploy-1 checkpoint still works when the old binary stages a store without the table: from genesis, or from a base published before deploy 1. From a base published by deploy 1 it is refused (section 3.3, round 2 DeepSeek F6) | forward; or the rollback recipe (the revert plus a boot-time drop of an empty `choir_events`, status section); or an old-binary restore of a pre-deploy-1 checkpoint, whose workspace swap also removes the empty table. Round 1's hand `DROP TABLE` is withdrawn: it is SQL against a deployed store |
| Deploy 2 → the previous build, after stamped ops | **Not a supported rollback: deploy 1 is the floor.** The old projector ignores `Table`, so any stamped op it projects lands in `og_objects`. It projects them on any boot with a tail to replay: replay mode discards a prepared row and replays the event from the tape (`appender.go:778-787`, `:856`; `autoputer/run.go:744-746`). It also projects them in any rebuild or restore it runs | events stamped while deploy 2 ran are **invisible** to trace, WS catch-up, acceptance and costs. They are on the tape and in the table, but a WebSocket client that catches up during the window moves its cursor past them. The old seeds read `og_objects` only (`store.go:2012-2045`, `:2054-2074`), so they can hand out `seq` and `stream_seq` numbers `choir_events` already holds. Those duplicates go onto the tape and stay (round 2, MiniMax) | replay-ineligible (unknown table). Worse, once the old binary projects any stamped op, a later deploy-1 replay puts that op in `choir_events` while the live store holds it in `og_objects`. Live and replay then never agree again, and checkpoints stay refused after the forward deploy (round 2, DeepSeek F1) | forward to deploy 1 or later. If the old binary projected any stamped op, also rebuild the store from the tape on that binary: a fresh replay into a new workspace, as the projection-base rebuilder does (`projectionbase/rebuilder.go:385-395`), installed in place of the live store. Duplicated numbers cannot be repaired |

The old binary's behaviour on a stamped op is a characterization test on
today's code, written before any change (F5). Today's `projectObject`
reads only `op.Body` (`project.go:73-74`, `:215-256`), and `Validate` does
not refuse `Table` values other than the presence tables
(`projection_batch.go:194-197`) **[V]**.

**Pre-rollback check, below deploy 1 only, once any op has been stamped**
(round 2, DeepSeek F1). The default answer is "roll back to deploy 1
instead". If an older build is truly needed, all four must hold on the
computer first, read through product routes:
1. `store.event_table` is off, and at least one event has been appended
   since the off write. Then the newest `og_objects` event carries the
   store-wide `stream_seq` maximum, which is what the old seed reads.
2. No run is open, so no run continues from a `seq` that only
   `choir_events` holds. This is the idle state an ordinary refresh already
   waits for.
3. The appender is drained: no prepared row, and the store's head equals
   the platform head (`/health` `effective_release.event_head.sequence`
   stable across two readings while idle). Then boot has no tail to
   replay.
4. No rebuild, base publish or restore runs on the old binary while it is
   deployed.

If 3 or 4 was skipped, the repair is the rebuild in the last column. The
paper is never idle (an always-busy computer), so for the paper the floor
is absolute.

## 5. Failure modes, written before any code, each with its test

Harness: the store's tape harness (`internal/store/telemetry_harness_test.go`:
a real `Store`, the real appender, recording pin and CAS, `reopen`), unless
noted otherwise.

| # | Failure | Test that catches it |
|---|---|---|
| F1 | A seed reads `og_objects` only, so `seq` or `stream_seq` is handed out twice after the cutover or after a reopen | Harness: with the setting on, append batched and acknowledged events across several runs; `reopen` (a new `Store`, so the marks start empty, harness `:201-210`); append more; assert no duplicate `(run_id, owner, seq)` and no duplicate `stream_seq` across both tables. Repeat with batching on, with parallel appenders, and after `ReconstructThroughTarget`. Include a run's first append with no `choir_events` rows (`MAX` is `NULL`; the append succeeds with seq 1) |
| F2 | A reader forgets the union, or merges or limits wrongly | Differential test over all four union methods: one event script into two stores, setting off against setting on, plus a store where the setting toggles in mid-run; identical results for every cursor and limit, including `limit+1` paging in `handleRunEvents`. The script includes a trajectory over 10,000 events and over the 3,000 limit, one that spans the toggle, and an owner whose timestamps follow allocation order (the `ListEventsByOwner` boundary of section 4.4 is pinned by a separate test with swapped timestamps, which documents the difference). One event planted in both tables comes back once. Source guard: any function in `internal/store` that reads `ogKindEvent`, or calls `ListObjects` with no kind, must be on the union-aware list. It scans implementations, not interfaces (section 1) |
| F3 | Live and replay disagree: a wrong position, a wrong table, a `time.Now()` default | Harness: a mixed tape (toggled on, off, on; singles and sealed batches); `ReconstructInto` a fresh store; `replayDoltStateExtractor` of both is equivalent and `replayEligibility` is eligible |
| F4 | An old checkpoint becomes unverifiable because the empty table enters the witness | Build a witness on a workspace without `choir_events`; open it with the new DDL (empty table); the witness matches (`WitnessContentMatches`). After one stamped op, the table is observed and the old witness no longer matches, which is correct. A workspace with no `choir_events` at all extracts without error and without a probe. Every `DoltStateExtractor` built in production omits the empty table (no construction site can opt out) |
| F5 | An old binary refuses or wedges on a stamped op | **Characterization test on today's `main`, before the change**: `projectBatch` with `Table: "choir_events"` on an object op writes `og_objects` and `Validate` passes |
| F6 | The dry run's provisional position differs from the final one (extra chunks), or the dry run passes an op that finalize refuses (poison after the CAS) | Hook `DryRunProjectionBatch` (`hookedProjection`, harness `:154`) to record the position; assert it equals the finalized `tape_seq` for every stamped op. A store with no projection head gives position 1. Table-driven refusals at the dry run, never on the tape: run_id of 300 characters, a metadata `kind` of 200, zero timestamps, `tombstone` set, a non-`choir.event` object. Table-driven acceptances: a non-empty `computer_id` or `version_id` is stored; an absent metadata `kind` is stored as `''`; a kind the binary does not know is stored (F22) |
| F7 | Re-finalizing (`RecoverPrepared` after a crash) fails on the primary key | Finalize the same prepared event twice, then crash between commit and return and run `RecoverPrepared`: no error, one row |
| F8 | The lifecycle fold starts writing to the table, so its absent-condition stops meaning anything | `projectLifecycleRun` (a `ReplaceLifecycleActivationRequest`) with a `loop.started` event, setting on: the event is in `og_objects`, a second identical fold returns `ErrConcurrentStateChange`, and a source guard checks that `lifecycle.go` never sets `Table` |
| F9 | The class leaks: a kind outside the storage class is stamped, or a batching change moves rows silently | A parser test: every `EventKind` has a storage entry. A forcing test over `types.EventKinds()`: only `StorageEventTable` kinds are ever stamped, and only by a tape-bound store. A test pins the storage map equal to the batched set today, so a change to either fails until someone decides. `recheckTelemetryOp` also asserts `Table ∈ {"", choir_events}` |
| F10 | The channel read misses events | Pin `EventSystem1Judgment` as acknowledged, and pin `ListAcknowledgedEventsByChannel`'s production callers to `system1_verb.go` in the existing call-site audit (`telemetry_readclass_test.go:23-33`). A new caller fails the test and must add a channel index or a union. The rename makes the contract visible at every call site |
| F11 | The fences are blind to the table | A raw-SQL V1 role leaf in a `choir_events` row makes the full fence pass fail closed. A replayed stamped op with a V1 role leaf makes `vocabReplayStaysFenced` return false, so the full pass runs; a tracked stamped position that is missing also returns false. A source guard: every `FROM og_objects` in `internal/store/vocab_*.go` comes from `objectRowTables`, except the named V1→V2 plan |
| F12 | The manifest reports an empty or absent optional table as missing, or a non-empty one as fine when replay lacks it | `replay_eligibility_test.go`: absent from both sets is eligible; present in both with equal hashes is eligible; present in live only, or in replay only, is ineligible (schema drift); `Validate` accepts the new class |
| F13 | Restore keeps rows after the target head | Stamped events at heads h1 < h2; `ReconstructThroughTarget(h1)`: the maximum `tape_seq` is at most h1, and the witness equals one taken at h1 |
| F14 | Event-table reads race the projector on the shared engine | A `-race` test with finalize running concurrently with union reads (the pattern of `engine_mutex_live_test.go`) |
| F15 | Boot scans `og_objects`, or the DDL is slow on a large store | `Open` over many `og_objects` rows: `DOLT_HASHOF_TABLE('og_objects')` is unchanged and the open time is bounded (the pattern of `TestReopenWithManyRowsDoesNotTouchOgObjects`) |
| F16 | A reader query does not use its index | Plan-shape tests that `EXPLAIN` each `choir_events` query text, as the generated-column plan tests do (`dolt_store.go:1243`) |
| F17 | The saving is smaller than predicted | Local probe on the harness (journal per batched op, setting off against on, same seed, `doltJournalBytes`, `dolt_maintenance.go:192`). **It gates deploy 2** (section 4.1): b_ce/b_og at most 0.5 locally before staging. It also measures a 3-tree variant (no trajectory index), so Q2's price is known in KiB. It is directional only: a local miss stops deploy 2, but only staging and the paper can pass it |
| F18 | The setting is not applied at once, or is lost across a reboot | API test: `POST /api/computer-settings` takes effect without a restart; a crash boot reads it from the store (the gate's "read_at_boot") |
| F19 | A planned or crash restart adds a resumer | None is added. A source guard keeps the boot path's new statements to the DDL and the setting read (CLAUDE.md, "Restarts End Work") |
| F20 | The deleted methods have hidden callers | Compilation; the grep in section 1.2 |
| F21 | Residue import re-emits table rows as ops | The residue snapshot ignores `choir_events` (a test with rows present) |
| F22 | A binary refuses a stamped op whose kind a later binary added to the storage class: replay poison | Replay a tape with a stamped op of a kind absent from the binary's maps: it projects into `choir_events`, the dry run passes it, and replay equals live (round 2, DeepSeek F2) |
| F23 | The epoch is not bumped, so an updater-applied release from before deploy 1 lands on a deploy-1 store | `storeschema.Version` is 2. In the pattern of `updater/state_compat_test.go`: a store receipt of 2 and a release built at 1 is refused by Apply before mutation. This is the only path the epoch fences (round 1b); the others are rehearsed by `TestEventTableRollbackRecipe` and the proof's step 6 |

## 6. Mutation class, ceremony, staging proof

### 6.1 Class: red

The change touches the guest store schema and its epoch, the projector
(tape to state), replay and restore, the content witness and the replay
manifest, both vocabulary fences, the event readers (trace, WS catch-up,
acceptance evidence) and sequence allocation.

### 6.2 Ceremony

- **Conjecture delta.**
  - New: *a durable op's journal cost is paid per touched prolly tree, so
    moving batched telemetry from a 13-tree table to a 4-tree table with a
    sequential key cuts its per-op journal by about 60% and the paper's
    total by about 20 to 35%* (round 1 said about a third; section 2.6).
  - It is falsified on staging if the fit gives b_ce/b_og > 0.5. On the
    paper it is falsified if the journal bytes per tape event (reading 3)
    fall by less than 20%, after allowing for any change in the kind mix
    (reading 4).
  - If the paper falls by less than 20%, the next step is the "no
    telemetry in Dolt" design (section 7), not tuning F-c (round 2,
    DeepSeek).
  - It refines the record's ranking (F-c first) and makes F-e's value
    larger on the new table.
- **Protected surfaces.** Store schema (`schemaDDL`) and the store schema
  epoch (`storeschema.Version`); `projectOp` and `projectBatch`;
  `DryRunProjectionBatch` and `finalizeBatch`; replay
  (`FinalizeReplayBatch`); both vocabulary fences, full and narrowed; the
  restore witness compare (`selfdevprotocol`); the replay airworthiness
  manifest; `DoltStateExtractor`; `AppendEvent` and its seeds; the event
  kind classification (`types`); the four event read methods and the
  renamed channel read. Not touched: the tape event format and reducer,
  the payload format, the head CAS, auth, vmctl, gateway, deploy routing.
- **Admissible evidence class.**
  - Staging test-computer readings through product routes only:
    `/health` `effective_release.event_head.sequence`;
    `/api/runtime/observability?memory=detail` store bytes, the
    `telemetry_batching` counters and the new `tape_ops_by_table`;
    `GET .../self-development/replay-completeness`; the tape audit through
    corpusd's replay page and payload routes; WebSocket recording.
  - Local harness numbers are directional only.
  - Not admissible: local claims about the paper's journal; anything read
    over SSH; SQL against a deployed store.
- **Rollback path.** The setting off (live); then git revert of deploy 2
  (safe to deploy 1). Deploy 1 is the floor once any op is stamped. A git
  revert of deploy 1 is runtime-safe only while no op has ever been
  stamped, or after the pre-rollback check in 4.5, and it must carry the
  rollback recipe's boot step or checkpoint publication stays refused
  (status section, round 1b).
- **Heresy delta.** See the closing section.

### 6.3 Staging proof plan

The proof runs on one fresh test computer, as in the batching gate. These
are the same readings the record and the gate used. Before any of it, the
local probe (F17) has passed its go bar (section 4.1).

1. **Same build for both arms.** The gate's flag-off and flag-on intervals
   were on different builds (evidence `environment.between_builds`). Here
   both arms run on the deploy-2 build.
2. **Baseline arm.** Setting off, batching off. Run the gate's scripted
   workload (Texture create and revise, and a research turn) for at least
   10 minutes and at least 1,000 tape ops. Take a snapshot at each segment
   boundary of three readings: store bytes, the tape head, and the
   `telemetry_batching` counters (`tape_events`, `tape_ops`,
   `events_appended_by_kind`, `tape_ops_by_table`).
3. **F-c arm.** Set `store.event_table=true` (record the tape seq of the
   setting write). Run the same workload and the same snapshots.
4. **Fit.** delta_bytes = a × tape_events + b_og × og_ops + b_ce × ce_ops
   over all segments of both arms (the gate's segment method).
   - Prediction: b_og ≈ 86 KB, b_ce ≈ 30 KB, so b_ce/b_og ≈ 0.35.
   - Pass: b_ce/b_og ≤ 0.5. Report the bytes per telemetry record for both
     arms.
   - Also report the share of stamped rows with an empty `trajectory_id`
     (round 2, DeepSeek Q2). It shows how much the trajectory index serves.
5. **Correctness on the same computer.**
   - Replay completeness is `equivalent` and `eligible` at a head after the
     cutover, with stamped ops in the replayed tail.
   - The whole-chain audit finds no duplicate `seq` or `stream_seq`, and the
     gate's step-3 ordering rule holds across both tables.
   - `GET /api/runs/{id}/events` for a run that spans the setting write
     returns every event the recorded WebSocket published.
   - `POST /api/run-acceptances/synthesize` on a trajectory with stamped
     `tool.result` events cites them.
   - The replay-completeness report shows manifest version 4, and
     `choir_events` is neither unknown nor missing.
   - `/api/costs` and the texture diagnosis return events from both tables.
6. **Crash drill.** An owner lifecycle refresh (SIGKILL), as in gate
   step 4. After the boot: no duplicate numbers, runs closed "interrupted
   by a restart", setting still on.
7. **Live rollback drill.** Setting off: new batched events land in
   `og_objects`, and readers still return the whole stream.
8. **Composition.** Batching on, setting on: the fit gives the k = 5 cost
   per telemetry record. After deploy 3, the same with multi-row inserts.
9. **Paper.** After steps 1 to 8 pass and the panel agrees:
   - Take the record's readings 1 to 6 as a baseline **before** the
     setting write: two samples 600 s apart inside one GC interval. Reading
     4 also confirms `telemetry.batch` is still off there.
   - Set the paper's setting, then take the same readings after.
   - **This is the decision for the paper.** Keep the setting if bytes per
     tape event (reading 3) fall by at least 20%, after allowing for any
     change in the kind mix (reading 4). Otherwise turn it off (the live
     rollback, never a binary rollback below deploy 1) and take the next
     step named in section 6.2 (round 2, DeepSeek F4 and Q7).

The final artifact is a JSON evidence file in the shape of
`docs/evidence/telemetry-batching-phase1-staging-2026-10-22.json`.

## 7. Alternatives considered

| Alternative | What it would buy | Why F-c wins |
|---|---|---|
| **Fewer indexes on `og_objects`, for all kinds** | Every op of every kind pays fewer trees | The 12 indexes exist because their absence was the CPU problem: JSON scans at about 1.4 s per call (`dolt_store.go:212-214`; guest-cpu-is-unindexed-store-scans-2026-10-14). Dolt has no partial indexes, so a record-scoped index costs every kind (`:271-274`, `:318-328`). Dropping one saves 1 of 13 trees per op (about 7%) and regresses its reader. It is also DDL on the largest table |
| **F-e alone: multi-row upserts in `og_objects`** | -37% per row at k = 6 (raw probe, record) | Useful only with batching on, which cannot go on the paper before the planned-restart drain fix. It still pays 13 trees per statement. It composes with F-c and is stronger there (deploy 3). DeepSeek (round 2) suggests landing it on `og_objects` first if the drain fix lands before deploy 2: it has no schema, witness, reader or seed surface. That is compatible with this plan, as its own change sharing deploy 3's multi-row code. It is not made the plan, because the drain fix's date is not known |
| **A separate Dolt database for telemetry** | GC of telemetry would no longer hold the main store's write connection | It writes the same bytes. Finalize (heads, index and projection) is one transaction on one connection today (`computer_events.go:133-235`); a second database splits it, so a crash between the two leaves a head that says "finalized" over a missing row. The witness, restore and base code assume one database, `texture` (`replay_completeness.go:492`, `rematerialize` workspace swap, `projectionbase`): a second workspace means a second extractor, swap and base. It can be revisited after F-c if GC isolation is still wanted |
| **No telemetry in Dolt at all**: serve trace reads from the tape (the corpusd payload pages) or a local append-only log | Removes about 90 KiB per op altogether | The trace and acceptance would fate-share with corpusd (standing question 7). A local log is new state with no replay, witness or restore story, and acceptance cites events by id and `stream_seq` as evidence. It is the right long-term question (retention, telemetry-batching design §8), but it is not a first-pass change. F-c keeps every existing invariant (tape → Dolt projection → witness) and is reversible. If the paper falls by less than 20% under F-c (section 6.3, step 9), this is the next design, not a tuned F-c |
| **Move every `choir.event` (K1)** | About 6% more volume | It moves the lifecycle fold and its condition, the acknowledged evidence kinds and the System1 channel read. It needs a fifth tree, `(channel_id, …)`, or a condition on the new table. Open question Q1 |
| **A new op kind (`event_recorded`) instead of the stamp** | Explicit in the op kind | Old binaries refuse unknown op kinds at `Validate` (`projection_batch.go:188-190`): a rollback could neither finalize nor replay |
| **Route by kind in the projector, with no stamp** | No writer change | A new binary's replay of an old tape would put historical events into the new table while the live store holds them in `og_objects`. They are never equivalent, unless every row is moved |
| **A bounded background move of old rows** | Single-table reads | About 120 KiB of journal per moved row, and it is not on the tape (section 4.3) |
| **Primary key `stream_seq`, or `(owner_id, stream_seq)`** (3 trees) | One index fewer | `stream_seq` is not unique by construction (section 2.4). A key built from caller data can collide on replay, and a collision is either poison or a silent overwrite |

## 8. Open questions for the owner, each with a recommended default

| # | Question | Default |
|---|---|---|
| Q1 | Move only the ten batched kinds (K2), or every `choir.event` (K1)? | **K2.** About 94% of the volume, with no change to the lifecycle fold, the evidence kinds or the channel read. Named by the storage class, not by `Batchable()` (section 2.1; all reviewers agreed) |
| Q2 | Keep the `(trajectory_id, stream_seq)` index (4 trees), or serve acceptance synthesis by fanning out over the trajectory's runs (3 trees, about 7 to 9 KiB less per op)? | **Keep it, and decide it once, before deploy 1.** Trajectory derivation has fallbacks (`runtime.go:4613-4627`, `traceTrajectoryIDForRun`), so a fan-out is not provably equal. Dropping it later is not free: it changes the table's `SHOW COLUMNS` and so the witness (section 2.3). Gemini's dissent (drop it in deploy 3) and Devin's option are declined for that reason. F17 prices the 3-tree variant locally, and the staging proof reports the empty-`trajectory_id` share (DeepSeek). If the owner wants 3 trees, it must be chosen before deploy 1 |
| Q3 | Name: a new `choir_events`, or reuse the legacy empty `events` table (created at boot with a primary key and seven secondary indexes: five in `schemaDDL`, `store.go:360-364`, and two added at boot, `:796-803`; declared `empty_until_supported`, `replay_eligibility.go:50`)? | **New name.** Reusing it changes an existing manifest class and inherits its key, columns and indexes. Retiring the legacy table is a separate small change. (Round 1 said "two indexes"; Devin corrected it.) |
| Q4 | Fold the multi-row insert into deploy 2, or keep it as deploy 3? | **Deploy 3**, so each saving is measured on its own. Its dry run must use the same multi-row statement, so the journal still deduplicates (DeepSeek) |
| Q5 | Ship deploy 1 as its own release before deploy 2? | **Yes.** At least one staging day and one paper boot first. Deploy 1 is the rollback floor, so keep it as a release you are willing to roll back to for at least a week after deploy 2 (DeepSeek) |
| Q6 | Existing rows in `og_objects`: never touched, or moved or expired later? | **Never touched by F-c.** Retention, meaning how long trace telemetry is kept, is a separate owner decision with its own design. That design must account for the witness: deleting rows changes it like any other write, so a deletion must itself be a tape act, and a table emptied by retention drops out of the witness by the omission rule (DeepSeek) |
| Q7 | Enable on the paper before the batching drain fix? | **Yes**, after the test computer passes, with three conditions. `telemetry.batch` (F-a) stays off on the paper until the drain fix lands (Gemini). The paper's baseline readings 1 to 6 are taken before the setting write (DeepSeek). Any rollback goes no lower than deploy 1 (section 4.5). The setting is orthogonal to the drain bug: it does not change what an open window loses (Devin) |
| Q8 | Event-table reads on `readDB` under the engine mutex, or on the write connection as `run_memory_entries` does? | **`readDB` under the engine mutex**, with labelled ops. Today's owner reads already hold the same mutex on the write connection (`dolt_store.go:723-724`; section 4.4) |
| Q9 | Accept the narrowing of the channel read to acknowledged kinds for post-cutover rows (pinned by F10), or add a channel index (5 trees)? | **Accept, pinned, and renamed** `ListAcknowledgedEventsByChannel`. Its only caller reads an acknowledged kind. Batched kinds do carry channels (section 1.2), so the name states the contract (DeepSeek F9, Devin) |
| Q10 | Delete the three caller-less methods in deploy 1? | **Yes** (standing question 4) |

## 9. Not determined from code

- The paper's current kind mix: the batched share of its ops, and whether
  `telemetry.batch` is on there. The record's reading 4 gives both.
- The number of `choir.event` rows in the paper's `og_objects`, and its
  tree depth. They decide c_tree and the union read's `og_objects` cost.
- Whether Dolt keeps `body` and `metadata` in the row leaf or stores them
  out of band at 2 to 4 KB. This is the ±2 KiB in c_row.
- The per-statement journal cost of a sequential-key tree against a
  random-key one. Assumed no higher; the local probe (F17) measures it.
- Whether Dolt answers `ORDER BY stream_seq DESC LIMIT n` on
  `(owner_id, stream_seq)` and `MAX(seq)` on `(run_id, seq)` as seeks.
  F16's plan tests decide.
- Whether any checkpoint taken before deploy 1 will ever be restored on
  staging or the paper. The section 3.3 rule makes it possible, but the
  need is not known.
- Whether anything outside this repo reads a guest store's `og_objects`
  for `choir.event`. Nothing was found in code.
- The `og_objects` row's share of a paper telemetry record's cost, which
  sets the low end of the 20 to 35% band (section 2.6). Reading 3 bounds
  it.
- Whether Dolt reports a column that leads a secondary index in
  `SHOW COLUMNS`' `Key` field, as MySQL does (`MUL`). If it does, any later
  index change on `choir_events` changes the witness (section 2.3). The
  design assumes it does; a one-line check on the harness settles it.
- Which host paths can put an older guest binary on a computer without
  the updater, and so without the epoch gate. The code shows only that
  nothing outside `internal/updater` reads the receipt (section 4.1).

## Heresy delta

- **Discovered**
  - Checkpoint witnesses are a function of the binary's schema. Any added
    table or column makes earlier checkpoints unverifiable under the new
    binary, because `WitnessContentMatches` requires equal table sets and
    `SHOW COLUMNS` enters the schema hash.
  - The `seq` and `stream_seq` seeds are heuristics over `og_objects`
    ordering.
  - Three event methods have no production caller:
    `ListEventsByTrajectoryAfter`, `ListEventsOG` and `ListEventsByOwnerOG`.
  - `compactRunEventLedger`'s event read is unreachable, and
    `VerifyTextureWorkflow` has no caller.
  - The legacy `events` table is still created at every boot, with a
    primary key, seven secondary indexes and no readers (round 1 said two).
  - `ListEventsByChannel` has a single caller, System1.
  - Round 2: `ListEventsByTrajectory` returns the oldest 3,000 events of
    its newest-10,000 window (`store.go:2259`, `:2274-2277`). For a
    trajectory over 3,000 events, acceptance synthesis reads its oldest
    events, not its newest. F-c keeps this; whether acceptance should read
    the newest is a reader question of its own.
  - Round 2: the narrowed vocabulary replay fence skips any replayed id it
    cannot find in `og_objects` (`vocab_replay_fence.go:179-183`). It
    would fail open for any object row a replay writes to another table.
    `choir_events` is the first such table, and F-c handles it.
  - Round 2: the store schema receipt is written by every `Open` and read
    only by the updater (`store.go:718-728`; `updater.go:441-463`). An
    older binary that reaches a computer by another path is not refused,
    and it rewrites the receipt.
  - Round 2: the two `DoltStateExtractor` construction sites copy the same
    `IgnoredTables` and `IgnoredContentColumns` by hand
    (`agentcore/replay_completeness.go:489-505`,
    `projectionbase/witness.go:21-35`). A third site would be easy to get
    wrong.
- **Introduced (by the design, once built)**
  - A stamped op's projection depends on whether the binary honours
    `ProjectionOp.Table`: an old binary puts it in `og_objects`. Hence the
    floor: once any op is stamped, a binary rollback goes no lower than
    deploy 1 (a one-way door, fenced by the epoch only for the updater's
    Apply of an epoch-declaring release, and by the pre-rollback check
    otherwise; round 1b).
  - Below deploy 1, even with no stamped op, a rollback is runtime-safe but
    not rollback-safe: the old binary refuses to publish checkpoints over
    the empty table until a forward deploy, the rollback recipe or a
    pre-deploy-1 restore (round 1b).
  - `ListEventsByOwner` can return rows that are not the newest by
    timestamp where timestamps run against allocation order, from deploy
    2's first stamped op (round 1b, DeepSeek; pinned).
  - Event reads are a union of two tables until retention.
  - `ListAcknowledgedEventsByChannel` (renamed) covers acknowledged kinds
    only for post-cutover rows (pinned).
  - Two event classifications, durability and storage, pinned equal by a
    test.
- **Repaired:** none by this document. Once landed: the 13-tree write
  amplification of batched telemetry, and the narrowed fence's blind spot
  for rows outside `og_objects`.
