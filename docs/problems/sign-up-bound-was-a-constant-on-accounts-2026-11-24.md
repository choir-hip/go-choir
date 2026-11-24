# The sign-up bound was a constant on accounts, with an exemption bolted on (2026-11-24)

Structural assessment after three review rounds of `lane/tester-capacity`
(contract rule 7: three rounds without convergence, stop patching, change the
frame). The same two supports carried every finding.

## Problem

The lane was to let outside testers sign up without filling the host. It
bounded sign-ups by `AUTH_REGISTRATION_CAP`, a number of accounts holding a
passkey (22, from `4 + (154 - 100) / 3`), and exempted the operator's own
accounts (CI, drills) by a marked row.

## Evidence: three rounds, each about the last round's fix

- **Round 1**: the exemption was by address (`example.com`, `example.test`);
  anyone could type one. Fix: a secret header (`X-Choir-Operator`) that marks
  the row, a migration that marks the legacy rows, a day-old sweep of
  unfinished rows.
- **Round 2**: the mark lived on an email row that strangers can touch: an
  unfinished operator row was inherited by the next person; the sweep could
  delete a row mid-ceremony; the lookup and the write normalised addresses
  differently; the migration marked rows by passkey. Fix: clear the mark on
  resume, refresh the row, mark only rows with a passkey.
- **Round 3**: those fixes are separate calls on a shared row
  (lookup, clear, cap check, refresh, create, mark), so a stranger and the
  operator racing on one address leak the exemption past a full cap or strip
  it mid-ceremony (seat 1, blocking); the migration still marks any passkey
  holder on the legacy domains, so anyone who registered there before the
  deploy is exempt forever (seat 2, blocking); and the cap of 22 assumed 3 GiB
  a computer where 8.4 GiB was measured, so 18 new computers exhaust the
  100 GiB reserve before the cap is reached (seat 1, blocking).

## Belief: two supports

1. **A user row that exists for an address before anyone finished.** Every
   mutation (mark, clear, refresh, sweep, resume) was a guard around a row a
   stranger can touch. `lane/native-passkey` removes the row: the address is
   proved by mail first, and the account, its passkey and the proof are made
   in one transaction at `register/finish`. On that base there is nothing to
   sweep, refresh, clear or race.
2. **A constant on accounts standing in for what is scarce.** What is scarce
   is the host's disk, for computers. An account costs nothing until it has a
   computer, so a count of accounts is a proxy, and every proxy needed an
   exemption to be bearable (the operator's CI creates accounts too) and a
   per-computer estimate that was wrong (3 GiB against 8.4 measured). A
   bound that is the measured room needs no exemption: the operator's
   accounts take places like everyone's, and when the host is full the
   operator's account is refused like anyone's.

## Change of frame

The bound is the room itself (`internal/hostroom`): sign-up is open when the
free disk, less the 100 GiB reserve (60 GiB deploy floor and 40 GiB for the
paper's growth), less what the accounts already admitted will still take, is at
least one footprint. What they will still take (`Plan.Held`): a whole footprint
for each account that holds a proved passkey and has no computer yet, and, for
each such account whose computer exists, the gap between its footprint and
the disk its computers hold now. vmctl publishes that disk as everything under
each computer's state directory (the data image and, while it sleeps, the
memory snapshot), summed over the account's computers, each file measured by
the snapshot budget's measure (`vmmanager.FileDiskBytes`: bytes on disk as btrfs
holds them; Node B's state is `compress=zstd`, where `st_blocks` counts a
compressed 4 GiB memory file at 4 GiB). A computer made today
holds 0.5 to 0.7 GiB; it grows to the footprint; releasing its promise the
moment it exists would hand the room back while the disk is still empty, and
counting the data image alone would let a wake (which deletes the snapshot)
open a place that the next hibernate takes back. So neither opening a computer
nor waking or hibernating one makes room
(`TestTenNewComputersOpenAndTheRoomDoesNotGrow`, at the hostroom and the auth
level; `TestAWakeAndAHibernateDoNotOpenAPlace`; and on a compressing disk
`TestAHibernateDoesNotOpenAPlaceOnACompressingDisk`).

**Which way a mismeasure errs.** The owners file is subtracted from the
footprint (`Held = footprint - disk`), so a disk figure that is too HIGH opens a
place; a figure too low only holds more back. The disk is therefore counted only
as far as it is measured exactly: each data file by `vmmanager.FileDiskBytes`
and counted only when the method is `btrfs_extents`, the snapshot from the
`disk_bytes` its writer recorded in `meta.json` and counted only when that was
`btrfs_extents` too. Anything else (the FIEMAP bound, which can run high; the
`st_blocks` fallback, which counts a compressed 4 GiB memory file at 4 GiB; an
error) counts 0, so the computer holds its whole footprint back
(`TestOnlyTheExactMeasureCounts`, `TestAFallbackMeasureCannotOpenAPlace`). On
Node B vmctl holds `CAP_SYS_ADMIN` (`nix/node-b.nix`, the unit's capability
bounding set), so production runs the exact measure; the read-only probe of
2026-10-22 recorded `disk_bytes_method=btrfs_extents` equal to the tree search
(`docs/security-density-architecture-checklist-2026-10-21.md`, RES-14). The
deployed proof checks the method on every snapshot and the journal for a
fallback. Each computer's figure is cached and recomputed on its state change
or on the 30 s tick, and a publish that a registry change triggers reuses it.
The file is rewritten within 2 s of every write of the registry's persistence,
and every 30 s. Before the snapshot is consumed by a wake, before the retention
prune and the test-idle prune destroy a computer's state, the file already says
so (`beginDiskRelease`: the disk is taken out of the computer's figure and the
file is written, then the delete happens), so the free reading rising cannot
outrun it (`TestAWakeTellsTheFileBeforeItConsumesTheSnapshot`,
`TestTheTestIdlePruneTellsTheFileBeforeItDestroysState`). Three paths delete
disk without that pre-publish, and each can leave the file ahead of the free
reading for up to the 2 s gap (a stop or a removal of a hibernated computer
through `discardStoppedSnapshotLocked`, which holds the registry lock; the
boot-time orphan sweep, before which the first publish no longer happens; and a
snapshot discarded by the manager itself). The worst of those is one
snapshot per stop or removal, summed over those that fall in the same publish
gap (a burst of stops, a deploy refresh of several idle computers, is N
snapshots, not one) for up to about 2 s, which still fits the slack the bound
normally leaves (at most the sum of the snapshots over 15 GiB places);
moving the discards out of their locked sections is not small and is not done.
Waking a computer never makes room only for snapshots counted exactly: a
snapshot whose `meta.json` carries no `btrfs_extents` reading counts 0, which
holds more back, never less. Every file counted as 0 for want of an exact
measure is reported in the owners file (`inexact_files`) and the deployed proof
fails while any is nonzero, the data images included.

- The footprint is 15 GiB, the largest measured total disk of one computer
  rounded up. The measurement is
  `docs/problems/computer-disks-hold-self-development-leftovers-2026-11-24.md`:
  drill holds 13,297 MB of data image (13.6 GB re-read after a wake) plus a
  1.5 GB memory snapshot while hibernated, about 14 GiB; b@c holds 8,865 MB
  (8.4 GiB in `docs/definitions/choir-storage-efficiency-2026-11-22.md`, the
  same computer a day earlier). 9 GiB, the first choice, was below the drill
  measurement. It is the worst measured, not a tester's: no tester's computer
  has been measured after weeks of use, so the bound probably under-admits
  until one is. When the owner's capacity reading shows the testers' own
  computers, replace it with their measured figure (a lowered footprint raises
  the places and is safe only against that measurement; the parsers refuse less
  than 15, so lowering it means editing `hostroom.MinFootprintGiB` in the same
  change). Both parsers refuse a
  footprint under 15 GiB and a reserve under the 60 GiB deploy floor
  (`hostroom.MinFootprintGiB`, `MinReserveGiB`), so a typo cannot spend the
  host: such a plan closes sign-up.
- Only accounts whose address was proved count (`users.email_verified_at` not
  null). Accounts from before the proof rule (27 hold a passkey with no
  proof in production) cannot sign in or open a computer until recovery
  proves them, so they hold no place; counting them would close sign-up on
  deploy.
- The decision is made inside the register/finish transaction, which reads the
  accounts that hold a proved passkey through the same transaction that inserts
  the new one; the single SQLite connection serialises finishes, so of twelve
  racing for the last place one wins
  (`TestTheLastPlaceIsDecidedInsideTheTransaction`). The mail and begin steps
  check the same number early so a person is not sent through a ceremony that
  will be refused. A refusal spends nothing: begin checks the room before it
  spends the mailed code, and a refusal at finish puts the pending challenge
  back, so the same code and the same passkey answer work once there is room
  (`TestARefusedBeginKeepsTheMailedCode`, `TestARefusedFinishKeepsThePendingChallenge`).
- The two readings are taken when the decision is made: the free disk by
  `statfs` on the directory auth.db lives in (the same btrfs root as
  `/var/lib/go-choir/vm-state`, `nix/disks.nix`), and the file vmctl writes every
  30 seconds naming the accounts that have a computer and the disk each holds.
  A missing, torn, stale (over 3 minutes) or future-dated file, or a failed
  `statfs`, closes sign-up with a plain "try again later". Never open on a
  guess.
- There is no unbounded mode. No room plan, or one that did not load whole (a
  part missing, a malformed number, a zero footprint), closes sign-up (503) in
  every environment while the rest of the service runs; development servers and
  tests give a plan (`TestNoRoomPlanClosesSignUpEverywhere`). This is a
  correctness refusal, not a switch.
- The owner reads the figures; a stranger reads only whether sign-up is open.
  `/api/pulse/summary` carries `capacity.room.{available, open}` and nothing
  else of capacity, and its storage block carries counts only (no vm-state
  bytes, no filesystem figures); `/api/platform/operator/capacity` (platform operator,
  passkey session) carries memory, disk, computers and the room's accounts,
  promised accounts, held bytes, footprint, reserve and places, and the storage
  figures. Pulse reads back the same owners file, with the same age, that auth
  reads, so a file auth cannot use is "no reading" there too, never open.
- The operator exemption is deleted: the `X-Choir-Operator` header, the
  secret, the `users.operator` column's use (the column was never deployed),
  the migration, the sweep, `RefreshSignUp` and `ClearUnfinishedOperatorMark`.
- The waitlist keeps the normalised address, first and last time seen, a count
  and an unkeyed SHA-256 of the source (an IPv4 address, or the /64 of an IPv6
  one, `appSource`'s rule); at most 5 rows a source and 1000 in all; deleted 30
  days after the last ask by the auth sweeper whether or not anyone is being
  refused; a person the list could not keep is told so, not that they are
  listed.
- A wake the host cannot hold answers 503 busy with no figures on every route
  (proxy, bearer including its `cause`, the compute panel). Make-room is on in
  production, and the truth is: a wake make-room can help hibernates idle
  computers (at most three, each bounded by the settle wait) and is admitted; a
  wake it cannot help is refused with nobody hibernated, the refused computer
  untouched and no reservation left
  (`TestAWakeMakeRoomCannotHelpIsRefusedPromptlyAndLeavesNothingStarted`,
  `TestAWakeMakeRoomCanHelpHibernatesAtMostThreeIdleComputers`).

## What is not built, and the residual

- **An account with two computers, one unreadable.** An unreadable state
  directory holds 0, which holds a whole footprint back for that computer; for
  an account with two computers the readable one can already exceed a
  footprint (so the account holds nothing back) while the unreadable one's
  growth is not reserved, at most one footprint, and the account counts as
  opened. Bounded; not built.

- **Operator bypass.** CI and drills create accounts through the real sign-up;
  on a full host they are refused like anyone. If CI must create accounts on a
  full host, the smallest safe form would be bound to the consumed challenge,
  never to an email row: the operator's tool would ask the auth service (over
  the host-internal credential, not a public header) to mint a single-use
  token tied to one `pending_registrations` challenge, and the finish would
  skip the room check only for that challenge. Not built; the owner has not
  said CI needs it.
- **Who can write the owners file.** vmctl, as root, and the VMMs it
  launches directly (the paper is not jailed) inherit its read-write mount of
  `/var/lib/go-choir`: a guest escape from an unjailed VMM could forge the
  file and open sign-up past the bound. This is a host-trust assumption of the
  same size as the rest of that mount; the alternative (an authenticated vmctl
  endpoint, or an HMAC with a key only auth and vmctl load) is not built.
- **Unproved until deployed:** that the auth service's sandbox can read
  `/var/lib/go-choir/host-room/computer-owners.json` that vmctl writes (if it
  cannot, sign-up is closed and Pulse shows it; that failure is closed, never
  open), and that the free disk read by auth's `statfs` equals vmctl's state
  directory's (one filesystem, `/var/lib/go-choir`).
- **Accounts with a computer already** (the owner's, the paper's): the file
  lists every user that has an ownership, so they hold no promise; their disk
  is in the free reading.
- **A promised account that never opens its computer** holds a place
  indefinitely. That is conservative (it keeps one footprint back) and is the
  owner's to clean up by deleting accounts.

## Remaining error

The footprint is a worst measured figure, so the bound probably under-admits
until a tester's real footprint is read; the first honest reading of it is the
Pulse "promised" and disk figures after the first week of testers.

## Security-Effect sentences for the `host:internal/vmctl/` files

One per file the surfaces list (`nix/guest-security-surfaces.txt`) matches in
this change, as two seats checked them against the code and the third round of
review corrected them. The vmctl commit's trailer carries all seven, true of
that commit as it stands. Each begins `none;` (the gate's pattern is
`^Security-Effect: none; .{20,}$`). `internal/vmmanager/snapshot_disk_linux.go`
(a zero extent reading with no allocated block is trusted as the exact zero) is
not on the list; the host-side vmmanager entries that are (`hostfirewall.go`,
`guest_egress.go`, `guest_reach.go`) are untouched.

- `internal/vmctl/ownership.go`: "Security-Effect: none; host-side. Adds mem_size_mib to VMOwnership and records 4096 (capped by the interactive size; 0 for exempt, platform or paid classes) once at creation, passed to VMManagerConfig.MachineMemSizeMib which already sizes a guest; adds the sign-up room plan and the owners file path to OwnershipRegistry. No guest-reachable request or action sets the recorded size or the plan."
- `internal/vmctl/host_admission.go`: "Security-Effect: none; the memory admission estimate reads the computer's own recorded size instead of the host's interactive size. Host-side, no guest input."
- `internal/vmctl/warmness_policy.go`: "Security-Effect: none; adds a host-side set of operator proof computer ids read from the host's tier configuration, trimmed like the other sets. No guest input."
- `internal/vmctl/pulse.go`: "Security-Effect: none; host-side only. Adds the capacity block to the internal vmctl pulse summary; of that block the public /api/pulse/summary serves only capacity.room.{available, open}, and of storage only the manual-recovery snapshot count and the existing expensive-nix-store-walk flag (the summary's existing account and computer aggregate counts remain); the proxy serves the full reading only to the platform operator (a passkey session or an unbound operator api key), and vmctl only to host-internal callers. Also writes /var/lib/go-choir/host-room/computer-owners.json within a 2 s coalescing gap of every write of the ownership registry's persistence (the signal is raised as that write begins, so a write that fails also triggers it), plus the length of a publish already in flight, before a wake consumes a snapshot and before the retention prune and the test-idle prune destroy a computer's state (not before a stop or removal of a hibernated computer, which discards its snapshot under the registry lock), the first time only after the boot-time orphan snapshot sweep, and every 30 s; each regular file outside the computer's snapshot directory is counted by vmmanager.FileDiskBytes only when the method is btrfs_extents, and the memory snapshot from the disk_bytes its meta.json records only when that reading was btrfs_extents, else 0 (a count reported as inexact_files); auth reads it to decide sign-up; writable by root vmctl and by unjailed VMMs inheriting its /var/lib/go-choir mount, not by jailed guests; no request writes it."
- `internal/vmctl/data_image.go`: "Security-Effect: none; host-side. The computer's disk figure counts a regular file in its state directory only when vmmanager.FileDiskBytes returns the exact btrfs extent reading, reads a sleeping computer's memory snapshot from the disk_bytes its meta.json recorded (also only when exact), and counts anything else 0 while adding to the account's inexact_files count (a fully sparse file reads as the exact btrfs_extents zero, so it is counted exactly and is not inexact); the figure is published only in /var/lib/go-choir/host-room/computer-owners.json and the internal/operator capacity reading, and reaches a guest only as the public room.available/open flag. No guest input."
- `internal/vmctl/retention_prune.go`: "Security-Effect: none; host-side. The retention prune takes the computer's disk out of the owners file and writes it (beginDiskRelease, diskReleaseWhole) before it calls DestroyVMState, then recomputes and republishes whether the destroy failed or not; the orphan-state-dir branch destroys a directory the owners file never counts, because it has no ownership. A guest cannot choose what is pruned. No guest input."
- `internal/vmctl/retention_test_class.go`: "Security-Effect: none; host-side. The test-idle prune takes the computer's disk out of the owners file and writes it (beginDiskRelease, diskReleaseWhole) before it calls DestroyVMState, then recomputes and republishes; the rule only takes computers in the host's test warmness class (test computers from the tier file; test accounts from the retention program's ephemeral email domains and user-id prefixes, read from the auth database). A guest cannot request the prune. No guest input."
