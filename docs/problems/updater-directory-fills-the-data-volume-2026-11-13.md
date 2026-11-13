# The updater's directory fills a computer's data volume

Recorded 2026-11-13T20:05Z. Status: cause located, fix not yet built.

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
