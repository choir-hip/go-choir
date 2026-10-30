# vmctl's retention plan matches nothing while test computers and quarantined data images hold ~70 GiB

Date: 2026-10-22, about 17:00 UTC. Found by: FP0, reading Node B read-only
after deploy R was refused by the disk floor. Status: fix on lane
`lane/fp0-vm-retention` (de3f616e6, both rules off, shadow observe), not
landed; station docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md. Mutation class
of the fix: red (VM lifecycle: vmctl deletes VM state).

Related: docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md
(the disk floor), docs/archive/problems/retention-prune-would-destroy-the-paper-2026-10-16.md
(retention's warmness guard), docs/archive/problems/node-b-disk-free-space-2026-10-09.md.

## Evidence (FP0, read-only on Node B, 2026-10-22)

- btrfs root 475 GiB. Free 139 GiB at 16:50, after an owner-approved early
  compaction of the paper (store 33.0 to 14.5 GB) and deleting a week-old Dolt
  backup (it freed only 7.2 GiB: shared extents). The deploy preflight floor
  is 90 GiB (`DEPLOY_MIN_FREE_KIB`, scripts/node-b-deploy-disk-preflight);
  deploy R was refused by it at 14:28.
- VM state holds about 150 GiB:
  - the paper 51 GiB, regrowing about 85 to 140 MiB a minute until its
    125.7 GiB cap or the 04:00 compaction;
  - the candidate fleet dir 40 GiB, **of which 32.4 GiB are two quarantined
    data images from 2026-10-07**;
  - the owner's computer 15 GiB;
  - **test computers 9.6, 12, 8.6, 8.7 and 1.4 GiB** (computer-2bd23366,
    computer-1f86c719, computer-7dcdf5e2 and FP0's two test computers);
    another user's 3.1 GiB.
- Other holders, for the record (not this lane): /nix/store 65 GiB,
  platform-dolt 36 GiB (oldgen .darc archives nothing collects), corpus-dolt
  6.3 GB, a 16 GiB swap file, builder 5.8 GB, offhost-backup 4 GB; btrfs
  metadata DUP 84% of 18.6 GiB, 38 GiB unallocated at 14:55.
- **vmctl's retention plan matches nothing.** The five test-class computers
  are registered accounts and the quarantined images are never aged out.

## What the code does today (read at f0b1ee313)

Retention prune runs in `active` mode on Node B (nix/node-b.nix:
`VMCTL_RETENTION_PRUNE_MODE=active`, ephemeral domains `example.com,example.test`,
prefixes `diagnostic-,sourcemaxx-proof-`, `EPHEMERAL_MIN_AGE=24h`), with a
dry-run shadow plan beside it. `retentionOwnershipReclaimable`
(internal/vmctl/retention_prune.go) takes an ownership only when all of these
hold: not held; the account is ephemeral by email domain or user-id prefix;
kind interactive **and** desktop `primary`; state stopped, hibernated or
failed; `LastActiveAt` at least 24 h old; and (since 10-16) the warmness class
is not protected.

The test warmness class (`WarmnessClassTest`) is derived from the same
ephemeral classifier (`refreshTestAccounts`), so a test-class computer and an
"ephemeral" account are the same set. Why none of the five matches is
**not established**. Hypotheses, none confirmed by a trace:

- H1. They are in use: FP0 proves on them several times a day, and every
  lifecycle transition (`transitionVM`, hibernate, stop, deploy refresh of an
  active computer) resets `LastActiveAt` to now, so the 24 h age is never
  reached while a computer is a working test computer.
- H2. Some are `active` (busy, or kept warm) and only stopped, hibernated or
  failed computers qualify.
- H3. An account is not on an ephemeral domain or prefix (a registered
  `playwright-state-…` account on another domain), so it is neither test
  class nor ephemeral, and FP0's "test class" reading came from elsewhere.

The plan cannot tell these apart: it lists candidates only, never the
computers it kept and why. That is the defect FP0 hit: a dry run cannot answer
"what would go, and what stops the rest".

Quarantined data images: cold recovery (`HandleColdRecover`,
internal/vmctl/cold_recover.go) renames `data.img` to
`data.img.quarantine-<generation>-<operation>` beside a
`rec-<generation>-<operation>.journal`. Two bounds exist and neither is an
age:

- `QuarantineDataImage` prunes completed quarantines of the same VM at the
  next recovery;
- the reclaim sweep's `PruneRecoveryQuarantines` keeps the newest
  `VMCTL_RECOVERY_QUARANTINE_RETAINED` (default 2) post-swap quarantines per
  VM dir.

A VM dir with two quarantines therefore keeps both forever, which fits the
candidate fleet dir's two 2026-10-07 images (hypothesis: their journals may
also be pre-swap or missing, which keeps them under any count). Nothing
records what a removed quarantine was. Also, `PruneRecoveryQuarantines`
returns at the first VM dir whose prune errors, so one stuck dir stops the
sweep for every later dir (secondary finding).

## Consequences

- About 40 GiB of test-computer state and 32.4 GiB of quarantined images sit
  on a disk whose floor already refused a deploy, with no policy that would
  ever reclaim them and no report that says so.
- Raising the existing rule's reach blindly (shorter age, more states) would
  reach FP0's working test computers or a computer a staged feature selects.

## Fix direction (this lane)

1. A test-class idle rule, its own switch (off, observe, enforce), default
   off; minimum idle default 7 days. It takes only a test-class computer
   (live tiers) that is stopped, hibernated or failed, not held, idle at least
   N days, and not named by any `VM_*_ONLY` selector by VM id or computer id.
   It never takes the paper or any platform computer, the owner's computer or
   any paid account's (premium), or any non-test account's computer. Enforce
   removes through the existing `destroyRetentionCandidate` path
   (`DestroyVMState`, then the ownership), with the same re-check at destroy
   time.
2. A quarantine age-out rule, its own switch, default off; minimum age
   default 7 days. It takes only a quarantine whose recovery finished: the
   journal's phase is `done` or its status is `active`, and the status is
   not `failed`. It keeps `swapped` and `booted` (a recovery that swapped
   but never verified), every pre-swap phase, and a missing or unreadable
   journal: any of these may be the only copy. This is stricter than
   vmmanager's count prune (`recoveryJournalPrunable` accepts `swapped`
   and later). It never takes a quarantine of a protected-class or
   selector-named computer. (Round 2 corrected this paragraph, which said
   "swapped or later" while the code said `done`/`active`: DeepSeek's
   phase-drift finding.) Each removal writes a manifest line (what, when quarantined, why:
   the recovery id, journal phase and canonical head, size) before and after
   the unlink.
3. The retention plan (and the shadow plan) reports every test-class computer
   and every quarantined image with bytes, idle age and either the match or
   the reasons it is kept, so FP0 can read what would go.

Failure modes the tests must cover (written before the code):

- T1. The paper (platform computer on a test account) is planned or removed.
- T2. The owner's (paid) computer, or a non-test account's primary computer,
  is planned or removed.
- T3. A selector-named test computer (VM id or computer id in a
  `VM_*_ONLY` list) is planned or removed; `class:` and `*` tokens do not
  count as naming a computer.
- T4. An active, held, or recently active test computer is planned.
- T5. Observe mode deletes anything; or the shadow plan, in any mode, does.
- T6. A class change between plan and destroy slips through (destroy-time
  re-check).
- T7. A pre-swap or journal-less quarantine is removed; a young one is
  removed; one of a protected or selector-named computer is removed.
- T8. A removal leaves no manifest line, or a manifest line lacks what,
  when, why or size.
- T9. The report omits a kept test computer or quarantine, or its reasons.
- T10. With both new switches off the plan's candidates and deletions are
  today's (inert deploy). The plan's config block does gain the new keys
  (`test_idle_mode`, `quarantine_mode`, both min ages, `selector_named`), so
  the JSON is functionally, not byte-, identical.

## Review round 2 (bunch76, at 7430e1f31): findings, recorded before the fixes

Reviews: DeepSeek (send back), Gemini flash (accept with edits), Sonnet
(accept with edits; blockers before either rule may be switched active).
Reproduced or confirmed by reading the code at 7430e1f31:

- R2-1 (all three; build). `inspectQuarantine` reads `syscall.Stat_t.Ctim`,
  which exists only on Linux. `go build ./internal/vmctl ./cmd/vmctl` fails
  on darwin, while the base f0b1ee313 builds there. Also, where `Sys()` is
  not a `*syscall.Stat_t`, the code falls back to the modification time,
  which is older than the rename, so it fails open (ages out early). Not
  reproducible on this Linux host (vmctl's Dolt cgo deps do not
  cross-compile); the reviewers reproduced it on darwin/arm64.
- R2-2 (all three; fail-open). Quarantines in a dir with no ownership record
  are checked only against selectors. A journal naming the paper's or the
  owner's computer in an ownerless dir is `will_remove` (DeepSeek's and
  Sonnet's probes). The scan also reads every dir under vm-state, not only
  `vm-*` and `candidate-*`, so operator-saved dirs are in scope. The
  10-07 pair in the candidate-fleet dir is the owner's pre-restore image and
  possibly unique (docs/archive/problems/node-b-disk-free-space-2026-10-09.md;
  docs/problems/fresh-realization-skips-file-hydration-2026-10-09.md:
  484 paths absent from the restored disk, check never closed). Today they
  are kept only if their dir still resolves to the owner's ownership: by
  luck, not by code. The lane's own test expected an ownerless
  `candidate-fleet` quarantine to be `would_remove`.
- R2-3 (Sonnet F2; policy). A `done` journal does not prove a quarantine is
  not the only copy: the 10-09 record shows a done recovery whose quarantine
  held 484 user paths the restored disk lacked. The rule allowed every
  non-protected class, real (primary) accounts included.
- R2-4 (DeepSeek 1, Sonnet F3). The selector protection reads only
  `^VM_[A-Z0-9_]+_ONLY$`. vmctl's environment also carries per-computer
  lists the regex misses: `CHOIR_CLAUDE_DELEGATE_COMPUTERS` (the delegate
  computer-ad1b46d4…, on an example.com account, so test class),
  `CHOIR_VISUAL_CAPTURE_COMPUTERS`, `VMCTL_JAILER_ONLY`,
  `CHOIR_CAPSULE_MIRROR_COMPUTERS`, `CHOIR_CAPSULE_MIRRORS_ONLY`. The proof
  checked the delegate at proof time only; a computer that becomes eligible
  later would be removed unreviewed.
- R2-5 (Sonnet F4, DeepSeek 11). `test_class_idle` dropped the ephemeral
  rule's `interactive` + `primary` desktop condition, so every desktop of a
  test account (a branch desktop of the paper's example.com account
  included) was eligible. And `DestroyVMState` removes the whole dir,
  including a pre-swap, failed or journal-less quarantine the age-out rule
  would keep (Sonnet probe P4). Related (DeepSeek 7): a computer stopped by
  `recover_current` was not excluded.
- R2-6 (DeepSeek 4). The record said "swapped or later"; the code says
  `done`/`active`. Corrected above to match the code.
- R2-7 (Sonnet F6, mutants). Mutants that survive the lane's tests:
  - M07, M08: the lifecycle claim is dropped or its conflict check skipped;
  - M09, M10, M31: a manifest write failure is ignored;
  - M12, M16: the quarantine removal-time rechecks are dropped;
  - M33: age is read from mtime instead of ctime;
  - M34: the regular-file check is dropped;
  - M29: a zero `LastActiveAt` counts as idle.
- Low findings, taken in this round: unparsed or non-regular quarantine
  names vanish from the report (Sonnet F8); no floor on the minimum ages
  (F9); concurrent passes write a false `failed` line for an already-removed
  quarantine (F7, part 1).
- R2-8 (found by the round-2 tests, 2026-10-22; live today). The existing
  `orphan_state_dir` rule, which runs **active** on Node B
  (`VMCTL_RETENTION_PRUNE_MODE=active`, `ORPHAN_MIN_AGE=6h`), destroys any
  `vm-*` or `candidate-*` dir with no ownership record after six hours,
  through `DestroyVMState` (the whole dir). It does not look inside. If the
  candidate-fleet dir holding the owner's 10-07 pre-restore images ever
  loses its ownership record (a moved realization, a registry reset, an id
  mismatch), the next sweep deletes those possibly unique images. Today they
  survive only because that dir still resolves to an ownership. The
  round-2 test that put an ownerless candidate-fleet quarantine beside the
  age-out rule saw the orphan rule plan the dir for destruction.
- Low findings kept as residuals: the ephemeral rule ignores selectors and
  runs first (pre-existing); the report's verdicts are not bounded by
  `MAX_DELETES`/`MAX_BYTES`; the manifest lives on the filesystem it frees
  and its new directory entry is not fsynced; a quarantine unlink racing a
  cold recovery's own count prune on the same VM (F7, part 2); the newest
  rollback quarantine of a test computer can age out (DeepSeek 5).

## Round 2 dispositions (77af30099)

- R2-1: fixed. ctime is read in retention_stat_linux.go; off Linux the age
  is zero (nothing ages out). A source test forbids Linux-only stat fields
  outside `_linux.go` files.
- R2-2: fixed. Only `vm-*` and `candidate-*` dirs are scanned. An ownerless
  quarantine is kept (`no_ownership`). The journal's computer id is
  checked: missing, protected (platform, a paid account's computer by
  stable id, a protected ownership's computer) or not the owner's own
  computer means keep.
- R2-3: fixed. Only a test-class owner's quarantine ages out.
- R2-4: fixed. Selectors are `VM_*_ONLY`, `VMCTL_*_ONLY`,
  `CHOIR_*_COMPUTERS` and `CHOIR_*_ONLY`.
- R2-5: fixed. test_class_idle takes only the interactive computer on the
  primary desktop. It keeps a computer stopped by `recover_current`, and
  any dir holding unfinished or unreadable recovery state (re-read at
  destroy time).
- R2-6: fixed in this record (the phase paragraph).
- R2-7: tests added. Each named mutant compiles and is killed.
- R2-8: fixed. The orphan rule never takes a dir holding recovery state.
- F8 and F9: fixed. Low residuals are listed in the station file.

## Review round 3 (bunch86, at dd9ae4d07): findings, recorded before the fixes

DeepSeek and Gemini flash, both accept with edits.

- R3-1 (medium). The R2-8 destroy-time recheck is untested. The R2-8 test
  creates the quarantine before the plan, so the plan-time skip removes the
  candidate and the destroy-time guard is never reached. DeepSeek's mutant
  deleting the guard survives.
- R3-2 (medium, live). R2-8 changes the live orphan rule, but it has no
  deployed proof and no report. The plan records nothing about a dir it
  now keeps: `orphan_state_dirs` counts it, `candidates` omits it, and no
  reason or bytes appear. After the deploy, FP0 cannot tell "kept by R2-8"
  from "not orphaned".
- R3-3 (medium, live, pre-existing). The active ephemeral rule
  (`ephemeral_test_primary`; example.com/example.test,
  `diagnostic-`/`sourcemaxx-proof-`, 24 h) destroys whole dirs through
  `DestroyVMState` with no recovery-state check. An ephemeral computer idle
  over a day whose dir holds a pre-swap, failed or journal-less quarantine
  loses it. This is the R2-5/R2-8 failure on a rule that runs today. Its
  destroy-time path also takes no lifecycle claim.
- R3-4 (low). Off Linux the quarantine age is zero by design (fail
  closed). So the tests that expect a quarantine to age out fail on darwin
  (3 to 5 of them, per the reviewers).
- R3-5 (low). Kept orphan dirs are invisible. A `rec-*.journal` is never
  deleted after a recovery, so an orphan dir that ever cold-recovered is
  pinned forever, `data.img` included.
- R3-6 (low). test_class_idle treats a finished quarantine as safe and
  destroys the dir, so a quarantine whose recovery finished minutes ago
  goes at once. The age-out rule would have kept it for `QuarantineMinAge`.
- R3-7 (info). Comments in retention_test_class.go, retention_prune.go and
  cmd/vmctl/main.go still say only `VM_*_ONLY`.
- Residual named by DeepSeek, for the owner: the orphan rule still removes
  an ownerless dir holding only a plain `data.img` (a computer that lost its
  ownership and never cold-recovered) after 6 h. That is the rule's purpose
  since before this lane, but the R2-8 premise (a registry reset, a moved
  realization) applies to it too.

## Round 3 dispositions (b55cf98f3, f93535204)

- R3-1: tested. A quarantine created after the plan stops the orphan
  removal, and the mutant that deletes the destroy-time recheck is killed.
- R3-2: fixed. The plan's `kept_state_dirs` gives the rule, reasons, bytes
  and release for every kept dir. Proof step 4 runs before and after the
  deploy: it lists every dir on Node B holding recovery state and checks
  that none is a candidate.
- R3-3: fixed. The live ephemeral rule keeps a dir holding unfinished or
  unreadable recovery state, a staging image, an unfinished journal, or a
  young finished quarantine. It does this at plan time and again under the
  lock and a lifecycle claim at destroy time.
- R3-4: fixed. Tests that need a quarantine to age out skip off Linux,
  with the reason.
- R3-5: fixed (reported). An orphan dir kept for recovery state is
  released only by an owner act: read it, then delete it or restore its
  ownership. A journal is never deleted, so a dir that ever cold-recovered
  stays until then.
- R3-6: fixed. test_class_idle keeps a dir with a finished quarantine
  younger than the quarantine minimum age.
- R3-7: fixed.
- Open for the owner: whether ownerless dirs holding only a plain
  `data.img` should also be kept.

## Review round 4 (bunch97, at 2ffb6c3ad): findings, recorded before the fixes

DeepSeek accepts with edits; Gemini accepts.

- R4-1 (medium, proof integrity, confirmed). Proof step 4 can pass
  without checking anything. `scripts/proofs/fp0-vm-retention.sh:118-127`
  runs under `set -uo` (no `-e`) and ignores the exit status of the plan
  read (`vmctl_get` fails on a curl error). An empty or non-JSON plan
  leaves the candidate list empty, so the only check passes. A wrong or
  unreadable `--state-dir` does the same: the remote script does
  `cd ... || exit 0` and the recovery-dir list comes back empty. An empty
  candidate list or an empty recovery-dir list also prints PASS, though
  nothing was compared. For the deployed proof of a live rule, a vacuous
  PASS cannot be told from a real one.
- R4-2 (low, DeepSeek F2). The retention lifecycle claim is released by a
  defer after the ownership row is deleted and the lock dropped, so a
  resolve in that window can see the stale claim and get a retryable
  "already in progress".
- R4-3 (low, DeepSeek F3). The station says the selftest reports 132
  passed; it reports 131 passed and 1 failed, the failure being the
  pre-existing `fp1 step 1 computes a busy share`.
- R4-4 (low, DeepSeek F4). The station gives no disposition when step 4
  fails before the deploy (the old binary lacks the ephemeral keep).
- R4-5 (info, DeepSeek F5). The owned destroy path does not re-read
  `cfg.Mode == active`, so the comment "every check is re-read" is broad.
- R4-6 (info, DeepSeek F6). `kept_state_dirs` order is not stable, so
  before/after `kept` lines do not diff line for line.
- R4-7 (low, Gemini). A dir that ever cold-recovered is pinned by its
  journal forever (R3-5 restated); an ownerless dir with only a plain
  `data.img` still goes after 6 h (the open owner question restated);
  `vmDirRecoveryKeepReasons` reads the dir under the registry lock.

## Round 4 dispositions (956efc1e1, 2094f73d2)

- R4-1: fixed. Step 4 fails when the state dir cannot be read, when it
  holds no `vm-*`/`candidate-*` dir while the plan counts some, when the
  plan read exits non-zero or is empty, and when the body does not parse
  as a retention plan. No recovery dir, or a plan with no candidates, is
  a MANUAL "nothing to check" line, never a PASS. Eighteen selftest cases
  cover it; each failed on 2ffb6c3ad. The default selftest pass now
  points step 4 at a stub state dir (it passed vacuously before).
  The tests are 956efc1e1; the fix commit's message names 289262db0,
  that commit before an unpushed amend.
- R4-3: fixed in the station (149 passed, 1 pre-existing fp1 failure).
- R4-4: fixed in the station: how to read each step 4 outcome, and what
  a FAIL before the deploy means.
- R4-6: fixed in the proof's output (kept lines sorted by `vm_id`); the
  plan's own order is unchanged.
- R4-2: declined for this round. The stale claim lasts one lock hand-off
  and costs a retryable error, no data; moving the release into the
  final lock hold changes the lifecycle protocol on a red path and wants
  its own test.
- R4-5: declined. The mode is set once at startup, so the re-read cannot
  change; not worth a red-path edit for a comment.
- R4-7: no change. The journal pin is R3-5 (reported, an owner act), the
  plain `data.img` orphan is the open owner question, and the dir read
  under the lock touches fewer than ten entries.

## Deployed proof after D2 (main 674460d15): a proof-script defect, recorded before the fix

FP0 ran `scripts/proofs/fp0-vm-retention.sh --commit 674460d15` on staging
after deploy D2 (evidence `_runs/deployD2-retention-after.txt`, outside
the repo). Steps 1 and 2 passed. Step 4 printed no candidate check (the
plan had none), only its MANUAL "nothing to check" line, so its
no-candidate rule remains unproven (corrected for R6-1). One check
failed:

```text
INFO step=0 vmctl build.commit=
FAIL step=0 vmctl runs 674460d15
```

- R5-1 (medium, proof integrity, confirmed). Step 0 reads
  `.build.commit` from the JSON body of vmctl's `/health`. That body
  (`vmctlHealthResponse`, `internal/vmctl/handlers.go` HandleHealth) has
  no build field, so the check can never pass, on any commit. vmctl's
  build identity is the `X-Choir-Build-Commit` response header that
  `server.WithBuildIdentity` (`internal/server/server.go`) sets on every
  response, with `X-Choir-Build-Service: vmctl`. The deploy reads that
  header to write vmctl's activation receipt entry
  (`.github/scripts/node-b-ci-deploy`, "Activation receipt:
  service=vmctl"). The selftest missed it because its fake curl answers
  `:8083/health` with a body that carries `build.commit`, which real
  vmctl never sends.
- R5-2 (low, proof procedure). Step 4's before/after comparison needs a
  run before the deploy. FP0 did not make one for D2, so only the after
  half exists (no risk: the active plan is observe with no candidates).
  Step 4's closing MANUAL line says "run this step before the deploy and
  after it", which reads as advice, not as a precondition.
- R5-3 (low, proof integrity, discovered with R5-1). The same wrong read
  is in `scripts/proofs/fp3-group1-wedge.sh` step 1: "vmctl health names
  T1" searches the `/health` body's strings for the commit, so it too can
  never pass on real vmctl. Its selftest passes only on the same
  wrong-shaped fake.

## Deployed-proof dispositions (d68279679, 70898c1fe)

- R5-1: fixed. Step 0 reads vmctl's build identity where vmctl carries
  it: the `X-Choir-Build-Commit` and `X-Choir-Build-Service` headers of
  `:8083/health` (`vmctl_build_identity` in `scripts/proofs/lib.sh`, the
  same header read the deploy makes for vmctl's activation receipt). It
  passes only when the service header is `vmctl` and the commit header
  matches `--commit`. The selftest's fake vmctl `/health` is now
  real-shaped (no build field in the body, identity in the headers); the
  new cases fail on another commit, a missing header and another service
  on the port, and the read-only pass failed on fd564dbbc.
- R5-2: fixed in the proof's text. Step 4's closing MANUAL line, and the
  script header, say a run before the deploy is required and why: the
  after run has nothing to be compared with otherwise.
- R5-3: fixed with R5-1. fp3 step 1 reads the same header for "vmctl's
  build header names T1"; its selftest baseline failed on the
  real-shaped fake before the fix.

The proof stays read-only. selftest: 154 passed, 1 failed (the
pre-existing `fp1 step 1 computes a busy share`). D2's step 0 result
stands as a proof-script FAIL, not a vmctl identity mismatch; the next
run of step 0 on staging is the deployed check of this fix.

## Review of the deployed-proof fix (bunch 115): findings, recorded before the fix

One review of 6476fea46 (verdict accept, yellow, with two text
corrections). Its findings:

- R6-1 (medium-low, record accuracy). The D2 section above says "Steps
  1, 2 and 4 passed". Step 4 printed no PASS or FAIL: the plan had no
  candidates, so it printed only the MANUAL "nothing to check" line,
  which says it is not a pass. Step 4's no-candidate rule is unproven on
  staging. A proof record that misreads a MANUAL line as a PASS repeats
  the error class R4-1 fixed in the script.
- R6-2 (low, stale assumption). The proof-scripts station
  (`docs/definitions/choir-launch-fp0-proof-scripts-2026-10-21.md`,
  "Assumptions FP0 should check on the first run") still says vmctl's
  `/health` reports `.build.commit`, the fact R5-1 refuted.
- R6-3 (low, test discrimination). The three negative step 0 cases
  (another commit, no header, another service) assert only
  `FAIL.*vmctl runs <wanted>`. The FAIL text does not name what was
  observed, so the cases also pass on the pre-fix script, which always
  failed. Only the positive case tells old from new.
- R6-4 (low, strictness). The old step 0 read used `-fsS`, so a non-2xx
  answer failed. The header read uses `-sS`, and vmctl sets the build
  headers on every response, so a 401 from `:8083/health` still passes
  "vmctl runs". `HandleHealth` has no non-2xx path today and the deploy
  separately requires 2xx, but the check no longer proves the endpoint
  answered.
- R6-5 (nit). fp3 step 1's wrong-service branch has no selftest case.

## Review dispositions (218e192db, b525673fb)

- R6-1: fixed in the D2 section above: steps 1 and 2 passed; step 4
  printed only a MANUAL line, so its no-candidate rule remains unproven
  on staging. (The metamission ledger's D2 entry carries the same
  wording; it is FP0's ledger, outside this lane, and is left to FP0.)
- R6-2: fixed in the proof-scripts station: vmctl's identity is the
  `X-Choir-Build-Commit` / `X-Choir-Build-Service` headers of a 2xx
  `/health`; its body has no build field.
- R6-3: fixed. Step 0's check text names the observed commit, service
  and status, and each negative case asserts them, so the cases fail on
  6476fea46 (which they did).
- R6-4: fixed. `vmctl_build_identity` also reads the status with
  `-w '\nX-Choir-Probe-Status: %{http_code}\n'`, the deploy's own read;
  step 0 and fp3 step 1 require a 2xx. New cases: a 401 with vmctl's
  headers fails both.
- R6-5: fixed. fp3 step 1 has a wrong-service case (it passed on
  6476fea46 too; it pins the branch).

selftest: 157 passed, 1 failed (the pre-existing `fp1 step 1 computes a
busy share`). The proof stays read-only; the next staging run of step 0
is the deployed check.
