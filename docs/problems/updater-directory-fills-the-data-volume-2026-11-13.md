# The updater's directory fills a computer's data volume

Recorded 2026-11-13T20:05Z. Status: fixed on lane/updater-prune, in review (not landed, not deployed, not proven on staging).

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

## Fix and named consequences (2026-11-13)

Owner ruling: every computer will look like this one, so the updater's root is
bounded, not conservatively pruned. After a settled apply it holds the current
release, the one rollback predecessor, the store objects those two root, and
inputs of an operation that is in flight. The rule is written once, in the
header of `internal/updater/prune.go`. Apply drops its own source once staged or
refused; the prune runs in chunks after a verified apply and at updater start;
the runtime collects bundles and host builds no operation can read
(`internal/agentcore/selfdev_bundle_gc.go`); a restage journals where it came
from so the predecessor survives it.

Consequences, named because they are the price of the bound:

- **Zero cushion.** A tape restore, or the revert of an old operation, to a
  release that is neither current nor its predecessor refuses before it
  changes anything (`verifyPinnedFrontend`: "served SPA is underivable"; the
  revert answers 409). Nothing re-fetches a pinned release from the host.
  Re-fetching on demand is a follow-up; until then those checkpoints are not
  restorable to their binary.
- **Operation evidence.** A settled operation's frozen bundle (source patch,
  screens) goes unless its release is current, its predecessor or a reachable
  pre-migration release. Reads answer "no longer retained on this computer".
  The bundle digest stays in the operation record; the bytes are not on the
  tape as far as this investigation established.
- **Journals and locks grow in count.** `operations/` gains one journal and one
  empty lock per apply key, and one journal per restage (a few KB each). They
  are not pruned: a terminal journal is what stops a replayed key from
  re-applying an older release, and only the runtime knows when an operation
  can no longer replay. Bounded in bytes per update, not in count.
- **Other state under the root is not covered:** `package-migration/`,
  `package-drafts/`, `bundle-finalizations/`, retained candidates, rebase and
  carried-patch files, `.overlay-work`, `.inputs-*` staging, `layering-diag.log`.
  Not measured here; the data_volume section shows them.
- **Cost of the prune.** After each chunk the keep set is recomputed from the
  journals (about 200 small files). Manifests are read without re-hashing.
- **A restore on a full volume refuses.** The tape restore, base restore and
  package migration take the updater's restore hold; when it cannot be taken
  they refuse before changing anything. The boot surface still reads a
  `current` that verifies.
- **`store/` is the upper layer of the `/nix/store` overlay** in the autoputer's
  private mount namespace (`nix/autoputer-vm.nix`); the updater deletes in the
  upper directory path, as `MaterializeClosure` already creates and removes
  entries there. Overlayfs documents changes to a mounted upper layer as
  undefined; only objects no surviving release roots are removed.
- **Candidate screenshot builds.** A closure materialized into `store/` for a
  shot is unrooted and goes after ten minutes; a retried shot materializes it
  again from the host build, which stays while its operation is open.
