# A capsule frontend build stalls b@c's guest on disk IO and starves its runtime (2026-10-27)

Status: open; the cause is not confirmed. Mutation class of any fix:
unknown. Documented before any fix.

## Evidence (UTC, host journal and the round's trace export)

- **00:30:06.** b@c (computer-6c50d6e5…, VM candidate-fleet-bf04bbc7…)
  takes a planned refresh onto train 29 (cba66873b) while idle, between
  the two attempts of the citation round (trajectory 150b7de5…).
- **00:36.** The second attempt starts in a fresh capsule. Engineering
  (DeepSeek) works through about 215 loop iterations, reaching about 223k
  input tokens.
- **01:08:57.** Engineering starts a capsule job: `npm run build` of the
  frontend, using the shared node_modules when the package files match,
  15-minute limit.
- **01:09:08.** b@c's runtime health check fails for the first time.
  Failures repeat every 15 s.
- **01:10:28.** vmctl kills the VM (`cause=recover`) and cold boots it
  (epoch 48, healthy at 01:10:42). Under the restart rule the open work
  closes and does not resume: the assignment is cancelled
  `interrupted_by_restart`, and the capsule is revoked.
- **Host signals.** vmctl's pressure reclaim saw intermittent host IO
  pressure (01:00:57, 01:04:54, 01:08:57) and no memory pressure.

## Ruled out (read in code)

- **Train 29's job-memory deletion.** The removed lease hook acted only
  for jobs that declared memory_mib, and with hotplug off it admitted them
  unchanged. This job declared none.
- **Balloon statistics newly on for b@c.** The balloon is reporting only:
  amount 0, deflate_on_oom true.

## Hypotheses (not confirmed)

- **H1, guest memory.** The build exhausted guest memory. The capsule
  budget is 3/4 of MemTotal, so the runtime and its seven loops share the
  last quarter, and the guest thrashed without an OOM kill long enough to
  miss health checks.
- **H2, guest CPU.** The build pegged the guest's vCPUs, and the runtime's
  health handler starved past the check timeout.
- **H3, host IO.** Host IO pressure (other computers, the paper backfill)
  stretched the build's disk IO, which made H1 or H2 worse.

The first attempt's build on the same computer passed, before the refresh,
so this is not reproducible on every build.

## Next observation

Repeat the build with instruments: the capsule records `/proc/pressure/*`
and `free -m` every 5 s during the build to a file. Prefer
`nice -n 10` and a bounded Node heap. FP0 sends this as the third attempt
of the citation round.

## Update 02:5x: the third attempt, instrumented (learning from the failed runs' traces)

The third attempt ran every build under `nice -n 10` with a 2 GiB Node
heap, and it sampled `/proc/pressure/*` and `free -m` inside the capsule
every 5 s. The samples are in the round's trace (`choir trace export`, the
tool results of engineering assignment 31a8b049…).

- **Memory is not the cause.** Memory pressure stayed 0; about 3 GB of
  7.9 GB was used, with 2.7 to 3 GB free and no swap. H1 is weakened.
- **CPU is partly busy.** Some pressure was 32 to 40%.
- **Disk IO is the stall.** IO pressure was some 61 to 65%, full 43 to
  63%. Every runnable task in the guest waited on disk for about half the
  time.
- **The runtime.** Its health check failed 75 times from 02:21 to 02:43.
  vmctl marked b@c degraded at 02:42:46 and re-promoted it at 02:43:47
  without a restart (the lower priority helped). The build hit its time
  limit and no candidate was frozen.
- **The host is not the bottleneck.** Host IO pressure was about 4%, and
  the VM's cgroup io.pressure was 0. Firecracker was not waiting on the
  host disk; the guest waited on its virtio disk.
- **A regression.** The first attempt's build on b@c (00:18 to 00:19, before
  the 00:30 refresh onto train 29) took about one minute. Both builds since
  (01:09, then 02:21 to 02:43) ran over 15 minutes and stalled on IO.

### Ruled out

- Firecracker drive rate limiters: none on any drive.
- D6 cache return: never touched b@c.
- Balloon free-page reporting: d@e has had it since train 22, and its
  capsule builds passed. The only balloon difference on b@c is 5 s stats
  polling.
- Host disk contention.

### Remaining hypotheses (not confirmed)

- **H4.** Something in train 29's guest image or capsule executor slowed
  capsule file IO. The capsule_executor_linux and job_memory deletions
  are the guest-side changes to read first.
- **H5.** b@c's data image. 15 GB is allocated (10,644 extents, against
  d@e's 5 GB and 2,496 extents), with the Sync IO engine on a
  copy-on-write btrfs file. It can be slow, but it was the same file at
  00:18.
- **H6.** The store image's page cache was cold after the refresh, so the
  build read node_modules from disk. But a cold cache costs once, not 15
  minutes twice.

### Changes this suggests (owner, 02:5x: "maybe we need to make some changes")

1. **A capsule job never starves the runtime.** Put capsule jobs in a
   guest cgroup with lower cpu.weight and io.weight than the runtime, and
   make the health handler answer without disk IO. A busy build is then
   slow, never a "computer down". Whatever the cause, this is the
   substrate fix.
2. **Measure, don't guess.** Record a timing receipt for every capsule
   job (wall time, IO bytes, pressure peaks), so a 1-minute build becoming
   15 minutes is a visible regression in the next round's context.
3. **Learn from failed attempts.** Management should read the previous
   attempt's cause before it re-dispatches. The 01:10 restart cause was in
   the trace, but attempt 3 was sent without it until FP0 added it. This
   is the precommitment and ICRL thread.

## Update 03:1x: H4 is weak; a new suspect, the guest's trim timer

A read-only code read of the diff from train 28 (f6bd05924) to train 29
(cba66873b) found no change to the guest image: no change to
`nix/autoputer-vm.nix`, the flake, the kernel config, mount options,
`internal/capsule`, the workspace overlay, the shared node_modules,
cgroups, fsync or the store's write path. The guest Go changes are a
comment, an observability field, and the job-memory lease path (already
ruled out). The b@c nix lines for balloon stats and visual capture were
no-ops for b@c, which was already named. So **H4 is weakened**: the
release did not change how a capsule does IO.

What the refresh did do was cold boot the guest. That gives a new
hypothesis:

- **H7, the trim timer.** `choir-data-fstrim` (`nix/autoputer-vm.nix`
  about line 862) runs 10 minutes after boot, then hourly, and trims
  `/mnt/persistent` (discard has been on for every computer since before
  train 28). On a fresh mount ext4 has forgotten what it already trimmed,
  so the first trim covers the whole free space. The host side is a
  copy-on-write btrfs file with 10,644 extents, where each discard is a
  hole punch. A long trim could hold the guest's virtio queue, and its
  `IOSchedulingClass=idle` protects only guest IO priority, not the
  device queue. **Against it:** a trim at about 00:40 or 01:20 (after the
  01:10 cold boot) would need to last tens of minutes to cover the build
  windows (01:09; 02:21 to 02:43). Not confirmed.

The next observation, cheapest first:
1. The guest journal for `choir-data-fstrim` since 00:30 (start, stop,
   `fstrim -v` bytes), and the data drive's `discard_max_bytes`.
2. The host journal at 00:30: cold boot or snapshot resume (whether the
   hotplug removal changed b@c's launch config).
3. A build with the trim timer stopped, against one with it running.

Change 1 above (capsule jobs below the runtime in cpu and io weight;
health without disk IO) stays the substrate fix whatever the cause.

## Update 05:3x: H9, a refused-egress log flood (strongest so far)

b@c's surviving guest console (02:37 to 02:58; earlier files rotated
away) shows the build's npm retrying connections to `registry.npmjs.org`
and the capsule's egress proxy refusing every one ("host is not on the
npm ecosystem allowlist; ask management for a wider grant"):

- **52,434 refused records,** all for registry.npmjs.org: 11,243 lines in
  the 02:37 minute and 41,178 in the 02:40 minute. About 22 MB of console
  output was written from 02:40:06 to 02:40:46, so the log rotated four
  times in 40 seconds.
- **14 kernel RCU stall messages** in the same files ("GPs behind …
  detected by"). The guest kernel itself was starved.

Each refused record does three writes (`internal/capsule/executor.go:789`):
a `log.Printf` line that the runtime unit sends to the journal and the
serial console (`StandardOutput = "journal+console"`,
`nix/autoputer-vm.nix`), and an open, append and close of the capsule's
egress ledger file on `/mnt/persistent`. A 52,000-connection retry loop
is then about 52,000 file opens and appends, 52,000 journal writes and
52,000 synchronous serial-console lines. That matches what was measured:
guest IO pressure "full" 43 to 63%, memory pressure 0, and the host
nearly idle (a serial console and a guest journal are guest-side waits).

The paper's console has no egress lines and no RCU stalls, so its
health flaps have a different cause
([derived-key-validation-reruns-every-boot-2026-10-27.md](derived-key-validation-reruns-every-boot-2026-10-27.md)).

Still unexplained: why the 00:18 build did not retry against the registry
and the later builds did. Candidates: the capsule's npm mirror grant
(`capsuleMirrorsLocked`) reading differently after the cold boot, or the
shared node_modules not matching the package files in the later attempts,
so npm went to the network. Not read yet.

Fix candidates (red, capsule egress):
1. Aggregate refusals: log the first refusal per host and capsule, then a
   count every N seconds, never one line per connection.
2. Keep egress records off the serial console: journal only, or the
   ledger only.
3. Batch ledger writes: keep the file open, or buffer.
4. Make a refused registry fail the build fast (npm's fetch-retries,
   offline mode when the mirror is not granted), so a missing grant reads
   as "needs a grant" in seconds, not a 15-minute stall.

H7 (the trim timer) and H8 (the derived-key pass) are no longer needed to
explain this stall. Neither is ruled out as a contributor.

## Update 07:0x: what sent npm to the registry (read in the round's trace)

The stalled job is job-31e0c2d7e10c (attempt 3, started 02:20:56, stopped
02:43:26, trajectory 150b7de5…). Its script is the frontend build from
engineering's runtime prompt
(`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:58`):

```
if cmp -s package.json $d/package.json && cmp -s package-lock.json $d/package-lock.json && test -d $d/node_modules; then
  ln -s $d/node_modules node_modules      # the locked deps from the Nix store, no network
else
  npm ci --no-audit --no-fund             # needs the registry
fi
```

with `d=$CHOIR_FRONTEND_DEPS`. The job's tail at 02:23:48 shows a Node
process leaking socket listeners (`MaxListenersExceededWarning … 11 socket
listeners added to [ClientRequest]`), which is npm retrying. So the `cmp`
check failed and the build fell to `npm ci`, in a capsule with no grant
for the registry: the 52,434 refused connections of H9.

Neither package file changed in the release: their last change is
448b40150 (2026-10-15), already in train 28. So the mismatch is between
the capsule's tree and the guest's `$CHOIR_FRONTEND_DEPS`, and it began
between 00:18 (attempt 1 linked the deps) and 01:09. Candidates:

- the capsule's base tree is b@c's own source (U4 source projection is on
  for b@c), and its package files differ from the platform's, for example
  from an applied self-development candidate;
- or the refreshed guest's `$CHOIR_FRONTEND_DEPS` carries package files
  that differ from the platform tree.

The next observation is one cell in the next capsule:
`cmp package.json $d/package.json; cmp package-lock.json $d/package-lock.json; ls -l $d`
from /workspace/platform/frontend. Train 33 (036a216eb) fixes the flood, so
a refused `npm ci` is now one log line and a failed build, not a stalled
guest. The prompt's rule still applies: a change to the package files needs
a second round with the ecosystem grant.

## Probe result (13:00, citation attempt 6)

Attempt 6 started from the clean booted base (b@c on train 34, the applied
stack now upstreamed) and engineering's first cell recorded both `cmp`
comparisons against `$CHOIR_FRONTEND_DEPS` passing. So the package files
match the guest's frontend deps on the clean base; the mismatch of attempts
2 and 3 came with the stale stack tree (an old base plus the applied patch),
which the patch-identity containment (train 34) now retires.
