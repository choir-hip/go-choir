# FP0 deleted a quarantine image recorded as possibly unique owner files (2026-10-22)

Date: 2026-10-22, 18:16 UTC. Found by: FP0, when the VM retention lane's
round 2 review (bunch 76) cited the 10-09 records. Status: open. The
deletion cannot be undone; recovery depends on older file roots in the
content store. Mutation class of the act: black (irreversible deletion of
owner data), done under an owner yes that was given on incomplete
information.

## What happened

- 16:4x. FP0 asked the owner, among one-time disk reclaims: "(a) Two
  quarantined disk images from October 7 in the candidate fleet's folder:
  32.4 GiB." The question did not say what the repo records about them.
- 17:05. Owner: "YES to ALL three".
- About 17:06. FP0 checked that no process held the files open and that the
  computer was running on its live `data.img`, then deleted both:
  `vm-state/candidate-fleet-e15cb89f25d963c220319b7b/data.img.quarantine-1-39ddc7f2…`
  (32 GiB allocated) and `…/data.img.quarantine-1-e3dacdac…` (406 MiB).
  Free space rose by about 33 GiB, so neither shared extents with another
  file on Node B.

## What the repo already said about them

- `docs/problems/node-b-disk-free-space-2026-10-09.md:20`: the 39ddc7f2
  image is the "owner pre-restore image (10-07)" and "Holds 484 paths
  absent today: possibly unique"; line 125 keeps both images "pending the
  484-path check".
- `docs/problems/fresh-realization-skips-file-hydration-2026-10-09.md`
  ("Owner computer: files present before the 2026-10-07 restore are absent
  now"): a read-only path-and-size comparison found 484 paths in the
  39ddc7f2 image absent from the current disk (`files/Source` 432,
  `files/.choir` 37, `files/System` 2, and at least 5 `*.texture`
  documents), and 3 `files/Source` paths in the e3dacdac image. It says:
  "both quarantine images are retained as recovery sources. The owner
  directed 'delete images not required for recovery'; these are required."
  The check that would have closed this (the platform file-root chain) was
  never run, and no list of the 484 paths was saved.

The computer is `computer-03335285269bdba4f94377e56879f9e6` (vm
`candidate-fleet-e15cb89f…`, user 5bd6de97…, primary desktop, class
`premium_always_on`), per vmctl's list at 18:14.

## What no longer exists

- No btrfs snapshot on Node B is newer than 2026-04-10.
- The off-host backup client (`scripts/off-host-backup.sh`, added
  2026-10-19) clones only each computer's live `data.img`, never a
  quarantine file, and started after 10-07.
- The image's extents were exclusive, so no reflink copy remains.

## What may still hold the files

The 10-09 record: "The owner's files are still in older roots in the
content store." If the computer's file-root history still lists roots from
before 2026-10-07 11:46 UTC and their chunks were not collected, the files
can be read back from there. A read-only investigation of how to list and
read those roots is running (host loopback routes only; no SQL against
production Dolt; nothing on the guest).

## Cause (FP0's process)

CLAUDE.md: "Before deleting or overwriting, look at the target." FP0 looked
at the files (open handles, the live image) but not at the repo's record of
them. A `git grep` of the file name would have found both records in
seconds. The owner's yes covered a description that left out the one fact
that mattered.

## Fix to the process

Before any deletion of state FP0 did not create: `git grep` the target's
name, hash prefix and directory in `docs/`, and put every record that
mentions it, with its claim, in the question to the owner. A record that
says "required for recovery" or "possibly unique" blocks the deletion until
the owner answers that specific claim.

## What the image held (read-only investigation, 18:25 to 18:40 UTC)

The platform keeps every file-root manifest (paths, sizes, sealed chunk
digests) for good: corpusd's `computer_file_roots` and the
`file_root_committed` events on the tape (`internal/platform/file_cas.go`,
`internal/autoputer/file_sync.go`). Read through corpusd on host loopback
(`/internal/computers/files/roots`, `/internal/computers/events/replay`,
`/internal/computers/files/root`); nothing on the guest, no SQL.

- The last root before the 10-07 restore was committed at 07:47
  (sequence 571764, root `c52da9ef965d…`): 2,152 files, 92,937,359 bytes,
  the same counts as the 10-09 comparison of the deleted image. The next
  root is 10-08 00:03 (`04e78177702c…`, 2,385 files).
- 485 paths of the 07:47 root are absent from the first post-restore root,
  and 477 from today's root (18:14, `80d6f1387077…`). So the paths were
  lost at the restore, not deleted later (the 10-09 record's open
  question).
- The 477: 432 `Source/platform` (an older checkout of this repo: all 267
  loose git objects exist in the repo; 143 of 145 working files match a git
  version's size exactly, the other two are July docs a few bytes off; the
  rest is the clone's packs, refs and logs), 30 `.choir/zot` (the registry
  removed in deploy D), 2 `System/model-policy-overlays/*.toml` (450 and
  475 bytes), and 13 `*.texture` shortcut files (155 to 337 bytes each;
  pointers to documents in the guest's Texture store, not the documents).
- Contents are not recoverable from file roots: the artifact GC has
  collected chunks outside a 7-day window since 2026-10-14, and any
  surviving chunk is sealed with the computer's key (a two-approval unwrap).

Likely unique loss: the two overlay files, and the shortcuts if their
documents are gone from the Texture store (the owner can check by title).
Path lists: FP0 scratch `fileroots/missing.txt` and Node B
`/root/fp0-fileroots/` (manifests, 0700).

The computer is the owner's premium always-on primary
(`computer-03335285…`). vmctl lists `vm-4a9afa23` (computer-ad1b46d4…)
as class `test`; both stay excluded from proofs.
