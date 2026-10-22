# vmctl's retention plan matches nothing while test computers and quarantined data images hold ~70 GiB

Date: 2026-10-22, about 17:00 UTC. Found by: FP0, reading Node B read-only
after deploy R was refused by the disk floor. Status: fix on lane
`lane/fp0-vm-retention` (de3f616e6, both rules off, shadow observe), not
landed; station docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md. Mutation class
of the fix: red (VM lifecycle: vmctl deletes VM state).

Related: docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md
(the disk floor), docs/problems/retention-prune-would-destroy-the-paper-2026-10-16.md
(retention's warmness guard), docs/problems/node-b-disk-free-space-2026-10-09.md.

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
  possibly unique (docs/problems/node-b-disk-free-space-2026-10-09.md;
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
