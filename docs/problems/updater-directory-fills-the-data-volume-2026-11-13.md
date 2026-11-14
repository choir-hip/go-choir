# The updater's directory fills a computer's data volume

Recorded 2026-11-13T20:05Z. Status: cause located; fix built as a boot-time prune (lane/prune-at-boot), not landed.

## Problem

A long-lived computer's data volume fills until it is under the 4 GiB
capsule floor. d@e (computer-59c902c5d46caecc0a180a8557164e72) had 1.22 GiB
available of 31.2 GiB.

## Evidence

The `data_volume` section of `GET /api/runtime/observability`
(`?data_volume=detail`, landed in d6adf5c97, deployed 20:02Z), read on d@e
at 20:02:48Z:

| Path under /mnt/persistent | Allocated |
| --- | --- |
| choir-updater | 20.10 GiB, 9,034 files |
| choir-updater/incoming | 10.02 GiB, 4,357 files |
| choir-updater/store | 6.56 GiB, 2,740 files |
| choir-updater/releases | 3.52 GiB, 1,402 files, 36 or more release directories of about 0.1 GiB each |
| state.db | 1.02 GiB |
| capsule-artifacts | 0.89 GiB (walk truncated at 52,528 files) |
| files | 0.44 GiB |

Everything outside `choir-updater` sums to about 2.5 GiB.

## Belief

The updater keeps every staged download (`incoming`), every store object
and every release directory it has ever applied; nothing prunes them. Each
platform update adds roughly 0.1 GiB of release and more of incoming and
store, which matches the about 2 GiB per day growth measured on the host
image. The paper's image grows the same way.

## Remaining error

- Not read from the updater code yet: whether `incoming` is meant to be
  emptied after a verified apply, and which store objects the current and
  rollback releases need.
- The paper has not been read with the new section; it runs the same
  updater, so the same shape is expected but not observed.
- The host image does not shrink when guest files are deleted unless the
  guest trims; the freed space is certain inside the guest, likely on Node B.

## Fix direction

Prune in the updater itself, after a verified apply: empty `incoming` of
everything not in flight, keep the current release and its rollback
predecessor, and collect store objects no kept release or gc-root names.

## Built: the prune runs at boot, when nothing else does (2026-11-14)

`lane/updater-prune` is abandoned
([three-rounds assessment](updater-prune-three-rounds-structural-assessment-2026-11-14.md)).
`lane/prune-at-boot` replaces the post-apply prune with
`choir-updater prune --root ... --budget 60s`, run by the oneshot
`go-choir-updater-prune` (nix/autoputer-vm.nix), ordered before
`go-choir-updater.service` and `go-choir-autoputer.service` and required by
neither. Premises read before building:

- Writers of the updater root: the updater daemon (Apply, ImportBaseline,
  RestartCurrent) and the runtime and its workers (incoming bundles and host
  builds, RestagePinnedRelease, StageBaselineRelease, package-migration
  patches), and the runtime's exec wrapper (`bootguard`, `.overlay-work`,
  `layering-diag.log`). All start after the prune. The restart, recovery and
  cleanup path units write only under `/run`. The data volume is not mounted by
  the host while the guest runs.
- Every release reaches a computer by a runtime apply, and an apply restarts
  only `go-choir-autoputer.service` while the updater daemon stays up, so a
  prune ordered before every runtime start would see that apply unsettled and
  skip, and could overlap a live daemon. Boot only. A deploy refreshes the
  computer, so the directory is pruned once per deploy, at the rate it grows;
  self-development applies between boots wait for the next boot.
- Deleting a 20 GiB, 9,000-file tree on the build VM's ext4 virtio-blk took
  0.5 s (`rm -rf`, warm directory cache). The 60 s budget is generous; the
  unit's `TimeoutStartSec` is 120 s and nothing requires it.

Review round 1 changed the design in three places. The keep rule now also
counts an installed-verdict fence (a settled-forward apply, journal Phase
`arbitrated`, Outcome `refused`, Verdict `installed`) as installing `current`,
so the predecessor its revert reads survives the next boot. The prune refuses,
exit 0 with the reason in its line, when the daemon's socket
(`/run/choir/updater.sock`), its started marker (`updater-unit-entered`) or the
runtime's (`runtime-unit-entered`) exists, and the unit is `RemainAfterExit`,
`RefuseManualStart` and conditioned on those three paths, so a later `systemctl start multi-user.target` or manual start
cannot run it beside a live daemon. A full volume: leftovers of earlier runs
are deleted before anything is renamed, and a rename that fails with ENOSPC
frees that entry in place (a release's manifest first, so a half-deleted
release reads as a corpse the next run removes at once; syncing and retrying the
rename after each MiB freed). **Superseded by round 2:** a store object is
never freed in place (it has no marker, and `MaterializeClosure` reuses an
existing object), so the residual this sentence named no longer exists; a
store rename that fails for want of space is counted and left for a boot with
room.

Review round 2 (9843b93b1): a store object is never freed in place (no marker,
and `MaterializeClosure` trusts an existing object), so a store rename that
fails for want of space is counted (`enospc_skipped=N` in the line) and left for
a boot with room; incoming/ and releases/ are deleted before the store is
touched. The budget bounds the hide phases too, directories are synced once
per phase (a lost rename is re-decided at the next boot), a missing or
non-directory root is a skip (exit 0), a skipped line carries no counters, and
the runtime marks itself entered (ExecStartPre writes the same marker the
updater unit writes) so a failed updater unit cannot leave the runtime live
with the boot prune still runnable. `scripts/updater-prune-unit-nix-test.sh`
runs in `scripts/gates` and in CI's `guest-floor-commit-free` job (nix
installed).

Every writer of a journal result, and what the keep rule does with it
(`current` may equal the journal's target afterwards where marked):

| Phase | Outcome | Writer | current == target possible | Keep rule |
| --- | --- | --- | --- | --- |
| refused | refused | `Apply` `refuse()`, before any mutation | rarely, but no prior is recorded | nothing to keep |
| prepared, pointer_swapped, restart_requested, recovering | none | `Apply` mid-flight, `restorePrior` | yes | dead journal: prior and target kept |
| completed | applied | `Apply` | yes | prior kept (newest applied or installed fence) |
| completed | failed | `Apply` after a successful restore | no, current is the prior | none |
| completed | restaged | `RestagePinnedRelease` | yes | prior kept (newest restage) |
| fenced | refused, verdict nothing_installed | `Fence` | no | none |
| arbitrated | refused, verdict installed | `Fence` | yes | prior kept (shares the applied slot) |
| arbitrated | refused, verdict unknown | `Fence` | no at fence time | none |
| arbitrated | refused, no verdict (fenced before the field) | `Fence` | yes if it installed | kept when target is current (errs towards keeping one release) |

Residuals:

- A dead journal (no result, key free) pins the two releases it names. The
  runtime's crash-boot settlement fences a dead self-development journal, so
  that one pins for one boot; a platform update left pending on purpose is
  never fenced and pins until its pending state is resolved. The prune line
  reports `dead_journals=N`.
- One corrupt or unreadable journal stops the prune (fail closed: what it names
  is unknown), on every boot, until it is repaired; the report line names it
  (`skipped (journal <name> cannot be read)`).
- Not pruned, with their size and rate (estimates from the code, not measured on
  a computer): `carried/<sha>/source.patch` is at most 2 MiB
  (`capsule.CarriedPatchMaxBytes`) and one per distinct carried patch of a
  rejected or failed candidate; `package-drafts/<operation>.json` holds a
  package operation's tree, review, parent and source patches, so it is the
  size of those patches, one per package operation. Both belong to operations
  only the runtime's operation store can judge (the same reason bundles are the
  runtime's), so deleting them by age here would be a guess; they grow with
  engineering rounds, megabytes each, well below the 0.1 GiB per release.
  `operations/*.json` (small, one `restage-*` per boot follow or restore),
  `bootguard`, `.overlay-work` and `layering-diag.log` (written by the
  runtime's exec wrapper) and a `.journal-*` temp left by a crash inside
  `writeJournal` (a few hundred bytes per crash) are not pruned either.
- Between boots a self-development apply adds one release directory (about
  0.1 GiB), its store objects (about 0.2 GiB, from the 6.56 GiB over 36 releases
  on d@e), and no payload in incoming for an apply that staged (apply drops its source), but a failed stage or an unreadable store floor keeps its payload (collected at the next boot once it is past the 10-minute window), and a failed closure replay keeps its payload for diagnosis (its release is staged, so the next boot collects it at once); a
  computer that applies N times between two boots holds N times that until the
  next boot.
- Maximum releases kept with no dead journal: three (`current`, the Prior of
  the newest apply or installed fence that installed it, the Prior of the
  newest restage that installed it). A swap-back that cannot journal
  (`RestagePinnedReleaseBestEffort`) loses its predecessor at the next boot.

Review round 3 (62c8cb18e, delta e27661f69): a failed stage keeps its source (a transient
failure such as ENOSPC must not delete the only local copy of a downloaded
payload; the boot prune collects it past its window), while a staged release
and a deterministic pre-staging refusal (base mismatch, state incompatibility)
drop it. The runtime has its own marker, `/run/choir/runtime-unit-entered`, in
the boot prune's conditions and the binary's refusal; the updater's
`updater-unit-entered` keeps its meaning for `internal/updater/client.go`. The
CI classifier selects the unit test (`guest_unit`) when any `nix/` file, the
script or the workflow changes. The classifier's own self-test already failed on
main (three scripts named by Go code or a deployed file were classified
docs-like) and is fixed here. The line also counts in-place deletions the
budget cut (`inplace_unfinished`).

Delta review: the deploy classifier now ignores the boot prune's test script and
`scripts/chaos/*` (CI tooling; the README stays docs-like), so none of them
triggers a staging deploy; `scripts/node-b-growth-meter` is read into the host
system closure by `nix/node-b.nix` and still deploys the host OS (asserted), and
`scripts/off-host-restore-drill.sh` is operator tooling (ignored, asserted).
Apply drops its source only after a deterministic refusal (store floor, schema
window, base commit, base-image digest mismatch) or a successful stage; the three exits that keep it are
named above.
