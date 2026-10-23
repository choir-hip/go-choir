# Node B: the Nix store and host disk policy (2026-10-22)

Status: policy built on `lane/fp0-nix-store-policy`, revised after review
round 2 (bunch 74); not landed. Deploy 1 is the commit group ending at the
"deploy 1 head" named in the now card below; deploy 2 is the one commit
after it. Problem first recorded in 044693eeb. Lane
`lane/fp0-nix-store-policy`. Companion to
[node-b-disk-fills-from-the-paper-data-image-2026-10-22.md](node-b-disk-fills-from-the-paper-data-image-2026-10-22.md)
(lane `lane/fp0-disk-pressure`, the paper's image and the compaction
pressure trigger), whose "Deploy floor" section this record takes up.

## Evidence (FP0, read-only on Node B, 2026-10-22)

- btrfs root 475 GiB. Free 139 GiB at 16:50, after an owner-approved early
  compaction of the paper (its store 33.0 to 14.5 GB) and deleting a
  week-old Dolt backup, which freed only 7.2 GiB (shared extents).
- Deploy R was refused at 14:28 by the deploy disk preflight
  (`DEPLOY_MIN_FREE_KIB` = 90 GiB in `scripts/node-b-deploy-disk-preflight`).
- Holders:
  - VM state about 150 GiB: the paper 51 GiB, regrowing about 85 to
    140 MiB/min until its 125.7 GiB cap or the 04:00 compaction; the
    candidate fleet dir 40 GiB, of which 32.4 GiB are two quarantined data
    images from 2026-10-07; the owner computer 15 GiB; test computers 9.6,
    12, 8.6, 8.7, 1.4 GiB; another user 3.1 GiB.
  - `/nix/store` 65 GiB. nix.conf: `min-free` 80 GiB, `max-free` 100 GiB,
    `auto-optimise-store` false, `keep-derivations` true. 51 auto gcroots,
    including 7 prebuild-results and builder releases under
    `/var/lib/go-choir/builder/releases/*/out`; 4 system generations. A
    guest release is about 3.9 GiB (storedisk.erofs 3.5 GiB, already LZ4
    compressed by mkfs.erofs, `nix/autoputer-vm.nix` storeDiskErofsFlags);
    the store holds 11 of them with little sharing (RES-13 reading,
    `docs/definitions/choir-launch-fp10-res-13-22-2026-10-21.md` R7).
  - platform-dolt 36 GiB, almost all `noms/oldgen` `.darc` archives; auto
    GC every ~30 min appends to oldgen (a 25.7 GB file at 11:12, 6.2 GB at
    14:54) and nothing collects oldgen. corpus-dolt 6.3 GB. (Out of this
    lane's scope; named so the totals add up.)
  - swap file 16 GiB (removal waits on tonight's 02:37 backup proof);
    builder 5.8 GB; offhost-backup 4 GB.
  - btrfs metadata DUP, 84% of 18.6 GiB used; 38 GiB unallocated at 14:55.

## What the repo does today (read from main at f0b1ee313)

- `nix/node-b.nix` `nix.settings`: `auto-optimise-store = false`,
  `min-free = 80 GiB`, `max-free = 100 GiB`. `nix.optimise.automatic`
  is already on, weekly (`Sun 03:30`); if it has been running, the store
  older than last Sunday is already hard-linked and only paths added since
  are not (FP0 checks `journalctl -u nix-optimise`).
- System generations are pruned to `+4` in three places that must agree:
  the 4-hourly `go-choir-nix-gc` unit, the daily `go-choir-disk-gc` sweep
  and the deploy preflight.
- Nothing prunes GC roots outside the system profile. The roots that hold
  store paths are:
  - `/var/lib/go-choir/prebuild-results/{frontend,nixos,service-*}-result`
    (ci.yml prebuild): one link per attribute, overwritten each prebuild,
    so bounded. The service links are the only roots of a service-pointer
    deploy's closure: `/var/lib/go-choir/services/<name>` is a `cp -a` copy
    whose binaries still name store paths (the ELF interpreter, ICU), and a
    service-pointer deploy does not switch the system, so when the service
    was built at a newer nixpkgs those paths are rooted by nothing else.
  - `/tmp/go-choir-*-result` (ci.yml deploy build): same shape and the same
    role as the prebuild links.
  - `/var/lib/go-choir/builder/releases/*/out`: written by ci.yml's
    app-layer release push (`release_dir=/var/lib/go-choir/builder/releases/${DEPLOY_COMMIT}`,
    `choir-builder --out "$release_dir/out"`) from e527c1698 (2026-10-04)
    until a47122de0 (2026-10-09) dropped it, and by
    `scripts/s2_station_acceptance.sh` (`REL_ROOT`), deleted on 2026-10-12
    (b9f195b1c). So every root there is at least 13 days old (corrected in
    review round 2, S-F4; the first version named only the probe). Nothing
    in the repo reads or writes that tree now. The live self-development builder is corpusd's host-build service
    (`internal/hostbuild`), which builds under
    `/var/lib/go-choir/host-build/work/<job>/result`, exports the app-layer
    closure into the blob store and removes the work dir (and its result
    root) when the job ends; a guest replays the exported closure into its
    own private store. So no live computer's release depends on a host
    store path rooted under `builder/releases`.
- The deploy floor (90 GiB) sits above `min-free` (80 GiB) plus a deploy's
  footprint. That is deliberate: on 2026-09-17
  ([archive mission, item 2](../archive/mission-mail-attachments-and-app-cutover-2026-09-17.md))
  `min-free` was 120 GiB and the floor 90 GiB, so every deploy build ran
  with free space under `min-free`; nix's auto-GC fired inside `nix build`,
  deleted the build's own not-yet-rooted inputs ("not a valid path") and
  the deploy looped. Lowering only the floor brings that back.
- No alarm fires on host free space or btrfs unallocated space.
  `go-choir-btrfs-headroom` balances once a day when unallocated is under
  30 GiB and says nothing otherwise; the deploy preflight speaks only when
  a deploy runs.

## The problem

1. **The floor blocks landings while the store keeps headroom it does not
   need.** With 90 GiB as the deploy floor and 80/100 GiB as nix's GC
   band, the host needs ~90 GiB idle to land anything, and the VM state
   (the paper's regrowth) eats into that by the hour. The floor cannot fall
   alone (2026-09-17); `min-free`/`max-free` must fall with it, in order.
2. **Store retention is unbounded outside the system profile.** Stale
   S2-probe roots under `builder/releases` keep releases alive that
   nothing uses. The generation count is written three times.
3. **New store paths are not deduplicated until Sunday.**
   `auto-optimise-store` is off; each deploy adds duplicate files (service
   packages, frontend bundles) that wait up to a week for `nix.optimise`.
4. **No warning before the floor.** The first sign of a low disk is a
   refused deploy (14:28) or ENOSPC in a guest; the btrfs metadata ENOSPC
   of 2026-10-16
   ([node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md))
   showed 135 GB free in df.

## Not this lane

The paper's image growth and compaction (lane `fp0-disk-pressure`), Dolt
oldgen collection, the swap file, and the quarantined candidate images
(32.4 GiB: an owner decision; they are evidence from 2026-10-07).

## Belief state and remaining error

- Believed: the store's reclaimable share outside the four system
  generations is the `builder/releases` roots plus whatever the 7 prebuild
  and `/tmp` result links pin that the current system does not. Not
  measured: which of the 51 auto roots pin which of the 11 releases. FP0's
  read (`nix-store --gc --print-roots`, `nix path-info -S`) settles it.
- Believed: a one-time `nix-store --optimise` saves little (single-digit
  GiB), because the weekly optimise already ran and the dominant files
  (storedisk.erofs, one per release, differing by build commit) never
  dedupe at file granularity. Not measured.
- Unknown: a deploy's real disk footprint (no `df` before and after in any
  log; bounded 4 to 8 GiB by the guest image).

## The policy (built; revised in review round 2)

### 1. Dedup: `auto-optimise-store = true`, and a one-time optimise

Nix then hard-links each new path's files into `/nix/store/.links` as the
path is added, instead of waiting for Sunday's `nix.optimise` (kept as the
backstop). Cost: one hash per new file at build or substitution time, a few
seconds per deploy.

Estimate, from what the store holds: about 43 of the 65 GiB are 11 guest
releases, and their 3.5 GiB `storedisk.erofs` files differ by build commit,
so they never dedupe at file granularity. The other ~22 GiB (Go binaries,
frontend bundles, toolchains, the system closure) is where duplicates are;
a general NixOS store frees 20 to 35% on its first optimise. If the weekly
optimise has been running, only the paths added since Sunday are
candidates: **about 1 to 5 GiB**. If it has not (its journal says), **up to
5 to 8 GiB**. One more cost: every file gains a `.links` entry, so btrfs
metadata grows by roughly 100 to 300 MiB (metadata is at 84%; 38 GiB is
unallocated, so this fits).

FP0's plan, after deploy 1 is live (so new paths are already handled):

```sh
# a. did the weekly optimise run?
systemctl list-timers nix-optimise.timer
journalctl -u nix-optimise.service --since -15d | tail -n 20
# b. measure first (read-only; hashes only files whose size another inode shares)
nice -n 19 ionice -c3 python3 -I /opt/go-choir/scripts/nix-store-dedup-estimate.py
# c. only if (b) says >= 2 GiB, and btrfs unallocated >= 30 GiB, and no deploy is running:
systemd-run --unit=nix-optimise-once -p Nice=19 -p IOSchedulingClass=idle \
  -p CPUQuota=100% nix-store --optimise
journalctl -u nix-optimise-once -f    # ends with "N MiB freed by hard-linking M files"
df -BG / ; btrfs filesystem usage -b / | grep -E 'unallocated|Metadata'
```

### 2. Retention (`scripts/node-b-nix-retention`; ships in observe)

One script now decides; the preflight, the 4-hourly `go-choir-nix-gc` and
the daily `go-choir-disk-gc` call it and then `nix store gc`.

| Root | Keep | What it is for |
|---|---|---|
| System generations | nix's own `nix-env --delete-generations +N`: every generation newer than the current one, the current one and the N-1 below it. Observe: **+4** (exactly the old units); enforce: **+3** | the current system; the previous one, the deploy rollback target named by `deploy-receipt-previous.json`; one spare for a two-step rollback (a bad deploy, then a bad fix). After a manual rollback the rolled-back system's predecessors stay (review round 2, S-F1: the first version kept "the newest N by number", which after a rollback deleted them). The booted system is rooted by `/run/booted-system` on its own. Enforce saves one guest image plus the host delta, ~4 GiB. |
| `builder/releases/*/out` links | newest **2**, and any younger than **7 days**; the rest unrooted (symlink only; the directory, receipt and closure blob stay) | Nothing now. ci.yml's app-layer push (2026-10-04 to 10-09) and the S2 probe (deleted 2026-10-12) made them, so all are older than 7 days and the keep count is the guard in practice. The self-development builder is corpusd's host-build, which roots its work only for the build and drops it; guests replay the exported closure into their own store, so no live computer's release needs these. |
| `prebuild-results/*-result` | all (one per attribute, overwritten each prebuild) | deploy latency; the `service-*` links are the only roots of a service-pointer deploy's closure (the pointer is a copy whose binaries name store paths: ELF interpreter, ICU). Removing them could break the next restart of a host service. |
| `/tmp/go-choir-*-result` | all (same shape) | the deploy's own build roots; same service-pointer role. |
| `host-build/work/*/result` | not touched | corpusd removes them; a crashed corpusd's are removed at its next start. |

Never removed by the script, whatever the mode: anything nix's `+N` keeps
(the current generation, every newer one, the N-1 below current), the
booted system, a store path the current guest image
(`/var/lib/go-choir/guest`) or either system is, any non-symlink, any link
outside the store, any `..` path, any link whose `stat` fails. A builder
root that is not a plain absolute path below `/` refuses the builder step
(an empty value means the default path). A listing without a current
generation deletes nothing.

`observe` (deployed) runs exactly the legacy `nix-env --delete-generations
+4`, logged as `deleting system generations (nix-env +4): … (current N)`,
and logs `observe: would delete system generations: …` (what +3 adds) and
`observe: would unroot …`; `enforce` is a later one-line change in the two
units (`GO_CHOIR_NIX_RETENTION_MODE=enforce`) and the preflight's default.

### 3. The coupled numbers, and why they move together

| | before | deploy 1 | deploy 2 |
|---|---|---|---|
| nix `min-free` | 80 GiB | **24 GiB** | 24 GiB |
| nix `max-free` | 100 GiB | **40 GiB** | 40 GiB |
| deploy floor default (`DEPLOY_MIN_FREE_KIB`) | 90 GiB | 90 GiB | **60 GiB** |
| preflight guard | none | floor ≥ live `min-free` + 10 GiB; unreadable → 90 GiB | floor ≥ live `min-free` + 36 GiB (the floor's terms); unreadable → 116 GiB (round 3) |
| alarm: warning | none | free < 60 GiB, unallocated < 20 GiB, metadata unreadable | same |
| alarm: error | none | free < 24 GiB, unallocated < 10 GiB, a failed reading | same |

nix runs a GC inside a build whenever free space falls under `min-free`, and
that GC deletes the build's own unrooted inputs ("not a valid path"; the
2026-09-17 deploy loop with `min-free` 120 and the floor 90). So a deploy
that starts at the floor must end above `min-free` with everything that can
overlap it:

    60 ≥ min-free 24 + deploy 8 + guest store compaction 14 + paper regrowth 10 + margin 4

The compaction's peak new allocation is about its compacted size (13.5 GiB
today, ledger 16:34) and nothing stops it overlapping a deploy (DS-2); the
paper regrows at 85 to 290 MiB/min, ~10 GiB over a 30-minute deploy. The
first version's 48 GiB covered the footprint only (48 − 8 − 13.5 = 26.5,
2.5 GiB above min-free). 56, the sum of the terms with no margin, was
round 2's figure; round 2b (DS2b-3) adds a 4 GiB margin: **60 GiB**. 60
also keeps `floor − footprint ≥ max-free`.

**When a term runs over.** The guard is the margin, and step 0 is the
check on the margin's main unknown. If one deploy's footprint exceeds
8 GiB, or a compaction peak exceeds 14 GiB, or the paper outruns 10 GiB
during a deploy, by up to 4 GiB in total, the build still ends above
`min-free` and nothing happens. Beyond that, free space can cross 24 GiB
mid-build; nix's auto-GC then runs inside the build, deletes its
not-yet-rooted inputs, and the build fails with "not a valid path" (the
2026-09-17 failure). That failure is loud and not destructive: the switch
never happens, the running system and the previous generation are
untouched, the 4-hourly GC would have collected the same unrooted paths,
and the alarm's error line fires under 24 GiB. Recovery is to free space
(or wait for the compaction to finish) and re-run the deploy. The terms
that can run over are measured, not assumed: step 0 measures the deploy
footprint and holds deploy 2 if it exceeds 8 GiB; the compaction peak is
the paper's compacted size, which vmctl logs at each compaction
(`file_bytes_after`); the paper's rate is FP0's reading of its image growth.
If any measurement exceeds its term, raise the floor by the excess before
deploy 2 lands, together with the matching constant in
`scripts/test_node_b_disk_policy.py`.
`scripts/test_node_b_disk_policy.py` checks both inequalities. A flake.lock
or nixpkgs bump can bring ~20 GiB of new host closure (S-F3): give such a
deploy ~70 GiB free by hand.

**Order is enforced by code now** (S-F2, DS-1). The preflight runs from the
new checkout before the switch, so it reads the `min-free` nix runs with
*now* from `/etc/nix/nix.conf` and never takes a floor under it plus
36 GiB, the same terms as the floor (round 3, R3-F2; deploy 1's preflight
used 10 GiB, the margin the 90/80 pair ran with); without a readable
numeric `min-free` it fails closed to the old band's 80 + 36 = 116 GiB.
Under the old 80 GiB band, deploy 2's preflight asks 116 GiB; deploy 1's
asks 90. So deploy 2 is safe whether it lands after deploy 1, with it, or
outlives a revert of it; landing them in order still matters for the
footprint gate below. Rollback needs no order either: reverting deploy 1
puts `min-free` back to 80 and deploy 2's guard lifts the floor to
116 GiB at the next deploy (revert deploy 2 too to return to 90).
`nix/node-b.nix`'s comment still says "plus 10 GiB" and "back to 90 GiB";
it is left for the next host change so deploy 2 stays preflight-only (the
guard it describes is now stricter, not looser). At the paper's rate, 60 GiB buys landings, not runway; the pressure
trigger and discard are the runway.

### 4. btrfs `compress=zstd` for `/nix/store`: evaluated, not implemented

- **Not as a mount option.** btrfs applies `compress` filesystem-wide (the
  first mount's options win; a per-subvolume mount cannot differ). It would
  compress every new write to the VM data images (the paper's 51 GiB of
  random writes), the Dolt stores and the swap file's neighbours: CPU on
  every guest write, 128 KiB compressed extents that fragment random-write
  images, and a change to the reflink-based store compaction's I/O.
- **Per directory instead:** `btrfs property set /nix/store compression zstd`
  (new files and directories under it inherit the property). `/nix/store`
  is mounted read-only, so it is set from a private namespace:
  `unshare -m sh -c 'mount -o remount,rw /nix/store && btrfs property set /nix/store compression zstd'`.
- **Savings estimate:** the 11 `storedisk.erofs` files are already LZ4
  (`storeDiskErofsFlags = [ "-zlz4" ]`), so zstd gains little on ~38 GiB of
  them (btrfs's heuristic skips most incompressible extents); the other
  ~22 GiB typically compresses 40 to 50% with zstd. **About 9 to 12 GiB**
  once the store has turned over. Measure before deciding (read-only, idle
  I/O, ~10 min): `nice -n 19 ionice -c3 sh -c 'tar -C / -cf - --exclude=nix/store/.links nix/store 2>/dev/null | zstd -3 -T2 | wc -c'`
  (a stream compresses better than btrfs's 128 KiB extents, so read it as
  an upper bound); after enabling, `compsize /nix/store` gives the truth.
- **CPU:** zstd level 3 compresses at roughly 150 to 300 MB/s per core and
  decompresses above 1 GB/s. A deploy writes ~5 to 8 GiB of new paths: about
  30 to 60 core-seconds per deploy on a 6-core host. Reads decompress only
  on a page-cache miss (guest boots read `storedisk.erofs` once into cache).
- **No one-time `btrfs filesystem defragment -r -czstd`.** Defragment
  rewrites extents and **unshares reflinks**: vmctl's jail `.shared` copies
  are reflinks of the store's image files (`internal/vmmanager/jail_shared.go`),
  so defragmenting those images would turn each shared extent into a second
  physical copy and *raise* usage by up to 3.5 GiB per live copy; it also
  churns metadata at 84%. The store turns over within about a day at the
  current deploy rate (old generations are collected every 4 hours), so the
  property alone compresses most of it without a defragment. If the
  long-lived paths matter later, defragment only files that are not images:
  `find /nix/store -type f ! -name '*.erofs' ! -name '*.ext4' ! -name initrd ! -name vmlinux -size +64k -print0 | xargs -0 btrfs filesystem defragment -czstd`
  (after `unshare` + remount rw, with btrfs unallocated ≥ 30 GiB).
- **Recommendation:** measure with the tar sample; if it shows ≥ 8 GiB,
  set the per-directory property (FP0 by hand first, declarative later as an
  idempotent oneshot), and no defragment. Risk is low: the property is
  reversible (`btrfs property set /nix/store compression ""`; existing
  compressed extents stay readable), and nix never writes into an existing
  path.

### 5. Host disk alarm (`go-choir-host-disk-alarm`)

A timer, not vmctl's daily boot-state line: a disk can lose 50 GiB in six
hours at the paper's rate, and the reading is a host fact that should not
depend on vmctl running. Every 15 minutes, one journal line; the priority is
the alarm (`journalctl -u go-choir-host-disk-alarm -p warning` shows alarms
only; the unfiltered `journalctl -u go-choir-host-disk-alarm` shows every
reading, so a missing quarter hour there means the timer did not run). The
free warning equals deploy 2's floor (60 GiB), so after deploy 2 a warning
means the next deploy is refused. Between the deploys the floor is still
90 GiB and a deploy refused at 60 to 90 GiB gets no alarm warning, only the
preflight's own refusal (DS2b-4: accepted for the hours between the two
deploys rather than teaching the alarm to read the live floor). The unallocated warning sits under `go-choir-btrfs-headroom`'s 30 GiB
balance line. Metadata use is printed, not thresholded (unallocated space
is the metadata headroom signal), but an unreadable metadata line is a
warning (DS-4). Limits (S-F8): it is a journal line, nothing pages, and a
hung `btrfs` is killed at 1 minute by systemd with only systemd's own
timeout line.

## FP0: what to eval (on the Mac, at the deploy 2 head)

```sh
c=.#nixosConfigurations.go-choir-b.config
nix eval $c.nix.settings.min-free                 # 25769803776
nix eval $c.nix.settings.max-free                 # 42949672960
nix eval $c.nix.settings.auto-optimise-store      # true
nix eval --raw $c.systemd.timers.go-choir-host-disk-alarm.timerConfig.OnCalendar   # *:0/15
nix eval --json $c.systemd.services.go-choir-host-disk-alarm.serviceConfig.Environment  # ...WARN_FREE_GIB=60...
nix eval --json $c.systemd.services.go-choir-nix-gc.serviceConfig.Environment      # ["GO_CHOIR_NIX_RETENTION_MODE=observe"]
nix eval --json $c.systemd.services.go-choir-disk-gc.serviceConfig.Environment     # ... "GO_CHOIR_NIX_RETENTION_MODE=observe"
nix eval --raw $c.system.build.toplevel.drvPath   # the whole host evaluates
bash scripts/test-node-b-nix-retention.sh && bash scripts/test-node-b-host-disk-alarm.sh \
  && bash scripts/test-node-b-deploy-disk-preflight.sh && python3 -I scripts/test_node_b_disk_policy.py
```

At the deploy 1 head the same evals hold; `test_node_b_disk_policy.py`
fails exactly its two floor = 60 GiB checks there (n3, n5), which deploy 2
satisfies. Deploy 2 does not touch `nix/node-b.nix`.

## Deploy classes (`.github/scripts/deploy-impact-classify`, S-F7)

- **Deploy 1 is a host-OS deploy.** 1a729b00e (in the deploy 1 group,
  committed under `.github/scripts`) maps `scripts/node-b-nix-retention`
  and `scripts/node-b-host-disk-alarm` to `mark_host_os_only`, as
  `scripts/node-b-sync-service-pointers` already is, and the
  `scripts/test-node-b-*.sh` tests to "ignored". The deploy 1 range
  classifies as `deploy_host_os=true deploy_vmctl_restart=true
  deploy_active_vm_refresh=false deploy_frontend=false
  deploy_host_service=false`: a host switch and a vmctl restart, no guest
  image rebuild, no computer refresh. Without 1a729b00e the two scripts
  were "unknown deployed path" and deploy 1 was a full deploy (a guest image
  rebuild and a refresh of every active computer, the paper included).
- **The CI test step is a patch, not a commit:**
  `docs/evidence/node-b-disk-policy-ci-wiring-2026-10-22.patch` (tracked in
  the deploy 1 group; workers cannot push `.github/workflows`). FP0 commits
  it from the Mac with or after deploy 2, because
  `test_node_b_disk_policy.py` checks the 60 GiB floor:
  `git apply docs/evidence/node-b-disk-policy-ci-wiring-2026-10-22.patch`.
  It adds a "Run Node B disk policy tests" step to the Vocabulary Gates job;
  the CI and deploy workflow contract tests pass with it. (Round 2 left this
  and the classifier as one uncommitted `ci-wiring.patch` in the worktree
  root, which the branch did not carry: DS2b-2.)
- **Deploy 2 is no deploy.** It changes only
  `scripts/node-b-deploy-disk-preflight`, which the classifier ignores, so
  no deploy runs for it; the 60 GiB floor takes effect at the next deploy
  that runs for any reason (the preflight is read from that checkout). It
  no longer edits `node-b.nix`, so it causes no vmctl restart.
- `auto-optimise-store` (S-F9): every `nix store gc` now ends with a pass
  over a fuller `.links`; the prebuild job runs one before each prebuild,
  so watch its duration (the prebuild log prints elapsed time).

## FP0: staging proof

Deploy 1 (needs 90 GiB free under the old floor; land it promptly, since the
paper regrows 5 to 16 GiB an hour):

0. **Footprint, the gate for deploy 2** (S-F3, DS-7). Free space as btrfs
   sees it, not `du`, right before the prebuild starts and right after the
   switch and refresh finish:
   `df -B1 --output=avail /; btrfs filesystem usage -b / | grep -E 'Free \(estimated\)|unallocated'`.
   Also note the paper's growth over the same window
   (`stat -c %b*%B` of its data image, or vmctl's file_bytes) and whether a
   store compaction ran. Footprint = (free before − free after) − paper
   growth. Deploy 1 is now host-only, so add the size of one guest image to
   it (`nix path-info -S $(readlink -f /var/lib/go-choir/guest)`, ~3.9 GiB):
   that is the footprint of a full deploy. **Deploy 2 proceeds only if that
   sum is ≤ 8 GiB.** If it is larger, deploy 2's floor becomes
   24 + footprint + 14 + 10 + 4 (rounded up), and the test's footprint
   constant and the preflight's `overlap_kib` term with it, before it
   lands.
1. The CI deploy log's preflight line:
   `live_min_free_kib=83886080 min_required_kib=94371840` (the guard reads
   the old band and keeps 90 GiB).
2. **A hold for deploy 2** (DS2b-7: the preflight's guard reads the file,
   not the running daemon). After the switch:
   `grep -E '^(min-free|max-free|auto-optimise-store)' /etc/nix/nix.conf`
   → `25769803776`, `42949672960`, `true`; `systemctl show nix-daemon -p ActiveEnterTimestamp`
   after the switch. Deploy 2 waits until the daemon's start time is after
   the switch; a file at 24 GiB with a daemon started before the switch is
   still running 80.
3. `systemctl start go-choir-host-disk-alarm.service; journalctl -u go-choir-host-disk-alarm -n 2 -o short-iso`
   → `host disk ok: free N GiB, btrfs unallocated M GiB, metadata P% of S GiB`
   (or a `<4>` line naming 60 GiB if free is under it), N and M matching
   `df -BG /` and `btrfs filesystem usage -b / | grep unallocated`.
4. Retention in observe, by hand (it runs the same `nix-env +4` the old
   units ran): `/opt/go-choir/scripts/node-b-nix-retention` →
   `mode=observe keep_generations=3 builder_root=/var/lib/go-choir/builder/releases …`,
   `(nix-env +4)` or `no system generation to delete`, one `root … (class)`
   line per auto root, `observe: would unroot …` lines; afterwards
   `find /var/lib/go-choir/builder/releases -type l | wc -l` unchanged.
5. Attribution, for the enforce decision:
   `for p in /nix/store/*-go-choir-guest-image*; do echo "$p"; nix-store -q --roots "$p"; done`,
   `find /var/lib/go-choir/builder/releases -type l -printf '%TY-%Tm-%Td %p -> %l\n'`
   (all should predate 2026-10-13) and `nix path-info -Sh` of their targets.

Deploy 2 (the next deploy after it lands, with step 0 passed):

6. The CI deploy log: `live_min_free_kib=25165824 min_required_kib=62914560`.
7. auto-optimise on new paths: `stat -c '%h %n' $(readlink -f /var/lib/go-choir/guest)/storedisk.erofs`
   on a guest image built after deploy 1 → link count ≥ 2.
8. The first deploy that starts with free space between 60 and 90 GiB:
   builds complete, no `not a valid path` in the log, and
   `journalctl -u nix-daemon --since '<deploy start>' | grep -iE 'deleting|freed'`
   shows no auto-GC during the build. Record the post-switch free space.
9. Alarm under pressure (whenever it happens): a `-p warning` line names
   `below 60 GiB (the deploy floor)` before any refused deploy; a refused
   deploy's log ends with the alarm's last line.

## Open for the owner

- Enforce retention (nix `+3`; unroot builder releases past the newest 2
  and 7 days) after observe shows what it would remove.
- The btrfs compression property on `/nix/store` after the tar sample.
- Not this lane, named so it is decided: the two quarantined candidate data
  images (32.4 GiB, evidence from 2026-10-07).

## Residuals

- The deploy footprint is unmeasured until step 0.
- The prebuild's `nixos-result` pins one extra guest image whenever the
  prebuilt commit is not the deployed one; kept (bounded at one).
- Which of the 51 auto roots pin which of the 11 releases (step 5).
- No deploy-in-flight signal between the preflight and the compaction;
  the floor reserves for the compaction instead of coordinating with it.
- `DEPLOY_MIN_FREE_KIB` has no break-glass path in ci.yml; a floor change
  is a commit (the guard keeps it from going under live min-free + 36).
- The guard trusts `/etc/nix/nix.conf`, not the running daemon (DS2b-7,
  R3-F3); step 2 is the hold for the first deploy after the min-free
  change. Why the file is the source: the locked nixpkgs (fe1324228)
  restarts nix-daemon when the file's store path changes at a switch
  (`restartTriggers = [ config.environment.etc."nix/nix.conf".source ]`
  with `stopIfChanged = false`, nixos/modules/services/system/nix-daemon.nix
  lines 303 and 336), and ci.yml exits the deploy when the switch fails
  twice. nix has no call that returns the daemon's settings: `nix config
  show` (Nix 2.34, src/nix/config.cc) prints the client's own
  `globalConfig`, read from this same file. What is left: connection
  workers forked before a restart keep the old value until they exit
  (nix-daemon.nix's own comment, lines 328 to 332), and a deploy whose
  switch failed leaves file and daemon apart until the next switch, which
  runs after the next preflight.
- The retention log's deletion list is a model of nix's `+N` at the moment
  of the listing (DS2b-6); a concurrent `nix-env --set` can make the log
  differ from what nix deletes, which is always nix's own rule. An old
  booted *generation link* can be deleted; the booted store closure stays
  rooted by `/run/booted-system`.

## Now card (2026-10-22, round 3)

- Station: FP0's Nix store and host disk policy lane. Mutation class: red
  (host Nix config, deploy routing via the preflight and the classifier).
- Deploy 1 group (everything up to the deploy 1 head): 044693eeb (problem),
  2a0cde15d (tests), ddd5a3c07 (change), 678c4c10c (policy docs),
  d3e8b5cd3 (round 2 findings), 473f8a451 (tests), 0284b76db (fixes),
  cc25aed8c (docs), 326a2860b (round 2b findings), a9937a8a5 (tests),
  997bc4e89 (preflight fail-closed, alarm 60), 1a729b00e (classifier), and this docs
  commit (17ef1dccf, the deploy 1 head). Deploy 2: every commit after it,
  preflight and its test only (0aa462439 floor 60 GiB; round 3: findings,
  test, 27566d828 fix, this record). The ci.yml step:
  `docs/evidence/node-b-disk-policy-ci-wiring-2026-10-22.patch`, for FP0.
- Tests at the deploy 2 head: retention 48/48, alarm 17/17, preflight
  27/27, policy 8/8, `.github/scripts/deploy-impact-classify-test` ok. At
  the deploy 1 head the same but policy 6/8 (the two floor checks).
- Next: FP0 evals, re-review of the delta, lands deploy 1 (host-only),
  proves steps 0 to 5 with step 2 as a hold; deploy 2 only if step 0's
  footprint plus one guest image is ≤ 8 GiB; then steps 6 to 9; then the
  enforce decision.

## Review round 2 findings (bunch 74: Gemini flash, DeepSeek, Sonnet), recorded before the fixes

Recorded as found; dispositions follow in the commits after this one.

- **S-F1 / DS-3, observe is not the legacy `+4` after a rollback.**
  `nix-env --delete-generations +N` keeps every generation newer than the
  current one, the current one and N-1 older; the script kept the newest N
  by number plus current. Equal whenever current is newest (every automated
  deploy); after a manual rollback to generation 5 of 1..8, real nix deletes
  only 1, the script's observe deletes 1 2 3 4 (Sonnet, real nix-env on a
  throwaway store). Enforce has the same flaw: it deletes the generation
  just below a rolled-back current, the next rollback target.
- **S-F2 / DS-1, the floor-after-min-free order is enforced by people.**
  The preflight reads only free space. Both commits in one deploy, a revert
  of deploy 1 alone, or `nixos-rebuild switch --rollback` to a
  pre-deploy-1 generation leaves the live `min-free` at 80 GiB under a
  48 GiB floor: the 2026-09-17 loop.
- **DS-2, the floor ignores the guest store compaction.** A compaction's
  peak new allocation is about its compacted size, 13.5 GiB today
  (ledger 16:34, disk-pressure round 2), and nothing stops it overlapping a
  deploy ("no deploy-in-flight signal"). 48 − 8 − 13.5 = 26.5 GiB, 2.5 GiB
  above `min-free`, minutes of the paper's regrowth (85 to 290 MiB/min).
- **S-F3 / DS-7, the 8 GiB footprint is unmeasured** and nothing makes its
  measurement a gate for deploy 2. A flake.lock bump can bring ~20 GiB.
- **S-F4, provenance.** `builder/releases` was also written by ci.yml's
  app-layer release push from e527c1698 (2026-10-04) to a47122de0
  (2026-10-09), not only by the S2 probe. The conclusion (no live writer
  or reader) holds; every root is at least 13 days old, so in production
  the keep count, not the age, is the only guard.
- **DS-6, retention hardening.** An empty, relative or `/` builder root
  makes every auto root a builder release; a failed `stat` reads as mtime 0
  (infinitely old).
- **DS-4 / DS-5, alarm.** An unparsable metadata line yields an all-clear
  `<6>`; the comment "a missing line means the timer did not run" is wrong
  for `-p warning`, which hides `<6>`.
- **S-F5, test gaps** (surviving mutants): the builder keep count is never
  the deciding guard; the default protect list is never exercised; no
  98/99/100 generation case; the alarm's exact thresholds and the exit code
  of its error and df-failure paths are unchecked.
- **S-F6**, none of the three test scripts run in CI.
- **S-F7, classifier blast radius.** The two new scripts are "unknown
  deployed paths", so deploy 1 is a full deploy: host OS, a guest image
  rebuild and a refresh of every active computer (the paper included, on
  the marker path). Deploy 2's node-b.nix comment edit restarts vmctl.
- **S-F8 / S-F9 / S-F10**, the alarm is a journal line only; auto-optimise
  makes every `nix store gc` walk a full `.links` (prebuild latency to
  watch); the estimator reads size-colliding images through the page cache
  and uses `os.path.stat`.

## Review round 2 dispositions

| Finding | Disposition |
|---|---|
| S-F1 / DS-3 | Fixed (0284b76db): both modes call nix's own `+N`; tests R12 (Sonnet's 1..8/current 5 case, a rolled-back 102), R13, R14 failed first on the old code. |
| S-F2 / DS-1 | Fixed (0284b76db): preflight guard on the live `min-free`, fail closed to 90 GiB; `scripts/test-node-b-deploy-disk-preflight.sh` P1 to P3 failed first. |
| DS-2 | Fixed (deploy 2): floor 56 GiB reserves the compaction peak and regrowth; test N7. Raised to 60 in round 2b. |
| S-F3 / DS-7 | Docs: proof step 0 measures deploy 1's footprint by btrfs free space and gates deploy 2. |
| S-F4 | Corrected here and in the script header. |
| DS-6 | Fixed: a non-plain builder root refuses; a failed stat keeps the link (R15, R16). An empty value already meant the default path (`${VAR:-…}`), never `/*`; the test asserts that. |
| DS-4 / DS-5 | Fixed: unreadable metadata is a warning (A8); comments name both journal views. |
| S-F5 | Tests: R17 (keep count when all releases are old), R18 (shipped defaults), R14, A6/A7 (exit codes, exact thresholds). |
| S-F6 | Round 2b: `docs/evidence/node-b-disk-policy-ci-wiring-2026-10-22.patch` (tracked), for FP0 to commit under `.github/workflows`. |
| S-F7 | Recorded under "Deploy classes"; deploy 2 no longer touches `node-b.nix`; the patch also maps the scripts to host-OS-only. |
| S-F8 | Recorded (alarm limits). Refused deploys print the alarm's last line. |
| S-F9 | Recorded: watch `nix store gc` duration in the prebuild. |
| S-F10 | Fixed: 1 GiB hash cap, page cache dropped after each hash, `stat.S_ISREG` (E4). |
| Flash: mandatory sequential deployment | Kept as the plan; no longer load-bearing for safety (the guard), still the footprint gate. |

## Review round 2b findings (bunch 80: Gemini flash ACCEPT, DeepSeek accept with edits), recorded before the fixes

- **DS2b-1, the preflight fails open on a non-numeric `df` reading.** A
  `df` that exits 0 with a non-numeric last line makes both `[` tests
  error, the `if` is false and the script exits 0: a deploy proceeds with
  no free-space check (DeepSeek reproduced `root_available_kib=garbage` →
  rc 0). Both reads (before and after the reclaim) have it. Pre-existing on
  main, but it is the red gate.
- **DS2b-2, `ci-wiring.patch` was cited but not delivered.** It sat
  uncommitted in the lane worktree's root, so a reviewer of the branch could
  not see it and FP0's `git apply` steps could not run from the branch.
- **DS2b-3, 56 GiB is exactly the sum of its terms** (24 + 8 + 14 + 10); the
  only slack is rounding (~1.8 GiB). A footprint over 8 GiB, a compaction
  peak over 14 GiB, a deploy over 30 minutes or the paper above 290 MiB/min
  crosses min-free; step 0's footprint gate is manual and gates only
  deploy 2.
- **DS2b-4**, between the deploys the alarm warns at 56 while the floor is
  still 90; a deploy refused at 56 to 90 gets no warning.
- **DS2b-5**, the now card's deploy 1 list omits 678c4c10c, 044693eeb and
  2a0cde15d.
- **DS2b-6**, the retention log's deletion set is a snapshot model (a
  concurrent `nix-env --set` can make it differ from what nix's `+N`
  deletes; the deletion itself is nix's). An old booted *generation link*
  can be deleted; the booted closure survives through `/run/booted-system`.
- **DS2b-7**, the guard trusts `/etc/nix/nix.conf`, not the running daemon:
  a switch that writes the new file but fails to restart `nix-daemon`
  leaves file 24 / daemon 80, and the guard computes the lower floor.

## Review round 2b dispositions

| Finding | Disposition |
|---|---|
| DS2b-1 preflight fail-open on garbage `df` | Fixed: every numeric read (free space before and after the reclaim, `DEPLOY_MIN_FREE_KIB`, live `min-free`) fails closed; tests P6 (garbage, empty, garbage after the reclaim) and P7 failed first (a9937a8a5). |
| DS2b-2 `ci-wiring.patch` not delivered | Fixed: the classifier part is commit 1a729b00e in the deploy 1 group (its test failed first in a9937a8a5); the ci.yml part is tracked at `docs/evidence/node-b-disk-policy-ci-wiring-2026-10-22.patch` for FP0. The untracked root copy is gone. |
| DS2b-3 56 = the exact sum | The margin is the guard: floor 60 GiB (4 GiB over the terms); step 0 checks the main unknown and holds deploy 2; "When a term runs over" says what a larger term does (a failed, non-destructive build) and how each term is measured. |
| DS2b-4 alarm 60 vs floor 90 between deploys | Accepted for the hours between the two deploys; stated in section 5. |
| DS2b-5 now card list | Fixed. |
| DS2b-6 log is a model; booted generation link | Stated in residuals. |
| DS2b-7 guard trusts the file | Step 2 is now a hold for deploy 2; residual stated. |
| Flash (bunch 80) | ACCEPT; no edits asked. |

## Review round 3 findings (bunch 95: DeepSeek, confirmed), recorded before the fixes

All three are in `scripts/node-b-deploy-disk-preflight` at the deploy 2
head (0aa462439).

- **R3-F1, the numeric reads have no length cap, so a big number fails
  open.** The `^[0-9]+$` checks (lines 25, 30, 50, 84) admit any length.
  A 20-digit `DEPLOY_MIN_FREE_KIB` passes the check, then makes every `[`
  that compares it error (status 2); an `if` whose test errors is false, so
  the guard (line 38) never raises it, and after the reclaim the refusal
  (line 88) is skipped: exit 0 and the deploy proceeds. The same holds for
  a 20-digit `df` reading. A `min-free` of `9223372036854775808` passes the
  check and `$((live_min_free / 1024))` wraps negative in bash's 64-bit
  arithmetic, so the guard computes a floor under the default and admits
  the deploy at 60 GiB whatever nix runs with. A leading zero makes
  `$(( ))` read the value as octal. Any arithmetic or test error must
  refuse the deploy.
- **R3-F2, the guard's margin is a fixed 10 GiB, not the floor's terms.**
  The default floor is min-free + 36 GiB (deploy 8 + compaction 14 + paper
  regrowth 10 + margin 4, section 3), but the guard keeps only live
  min-free + 10 GiB. For a live min-free between 24 and 50 GiB the guard
  floor is at or under the default 60, and from 50 GiB up it is min-free
  + 10: in both ranges a deploy can start with less than 36 GiB above
  min-free, so the overlapping work no longer fits and nix's auto-GC can
  run inside the build (2026-09-17). Example: min-free 40 GiB, floor 60,
  65 GiB free is admitted with 25 GiB above min-free.
- **R3-F3, the guard reads `/etc/nix/nix.conf`, not the running daemon's
  value** (the same gap as DS2b-7, now asked for in code). Either read the
  daemon's effective value with the file as a fallback that fails closed on
  disagreement, or record why the file is authoritative here, with the
  evidence.

## Review round 3 dispositions

| Finding | Disposition |
|---|---|
| R3-F1 numeric reads unbounded | Fixed (27566d828): every compared number is 1 to 15 digits and read as decimal (`10#`), and a `[` that errors refuses the deploy (`at_least`/`refuse`) at the guard and at both free-space checks. Test P8 (20-digit floor and `df` readings, min-free 2^63, leading zeros) failed first (9e0bee653). |
| R3-F2 guard margin fixed at 10 GiB | Fixed (27566d828): the guard is live `min-free` + 36 GiB, the floor's own terms, so the overlap fits for every `min-free`; the fail-closed floor is the old band's 80 + 36 = 116 GiB. Test P9 (min-free 40 GiB admitted 65 GiB free) and P4 (a hand-set 56 under min-free 24 kept) failed first (9e0bee653). |
| R3-F3 file, not daemon | Kept the file as the source, with the evidence in the script's comment and in Residuals: NixOS restarts nix-daemon when the file changes at a switch, a failed switch fails the deploy, and `nix config show` reads the same file in the client, so it adds no daemon reading and no disagreement to fail closed on. Step 2 stays the hold. |

## Owner ruling: retention to enforce (2026-10-23)

Owner, 02:3x UTC: "Yes, do nix retention change from observe to enforce."
D6's observe run on Node B (02:12, by hand) would unroot 25 builder
releases, 13 to 18 days old, all older than 2026-10-13, and keep 30 links
otherwise unchanged; the newest 2 releases and anything younger than 7 days
stay. Enforce also keeps 3 system generations instead of 4 (the current
system and two rollback targets). The change sets
`GO_CHOIR_NIX_RETENTION_MODE=enforce` in go-choir-nix-gc, go-choir-disk-gc
and the deploy preflight's default. Rollback: revert that commit (observe
again; unrooted paths that the GC already freed rebuild from the binary
cache or the flake). Proof: the next go-choir-nix-gc run logs `mode=enforce`
and its `unroot` lines; `find /var/lib/go-choir/builder/releases -type l | wc -l`
falls to the kept set; free space before and after.
