# Idle test computers keep old builds: root cause (2026-10-11)

Follow-up to `idle-test-computers-keep-old-builds-after-deploy-2026-10-11.md`.
Research only; nothing was changed, stopped or restarted. Mutation class of
any fix: red (VM lifecycle, deployment routing).

## Summary

**What I found.** The test computers are not working. vmctl itself
classifies them as idle and not busy on every 2-minute sweep. They stay
on because of a deliberate staging setting, `VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity`
(`nix/node-b.nix:753-756`, in place since 4ae89ab2, 2026-05-19). In that
mode the idle sweeper never hibernates an ordinary ("primary") computer
unless the host is under memory, CPU or IO pressure
(`internal/vmctl/warmness_policy.go:187`). Every QA and user computer is
"primary" by default (`warmness_policy.go:100`). Staging has had no
pressure since 2026-10-10 22:34:55Z, so since then nothing has idled. The
sweeper has logged zero idle hibernations since the journal begins
(2026-10-05). All 56 hibernation batches in that window came from pressure
reclaim.

**Real work or bug?** Neither, exactly. No work keeps them up, and the
busy/idle classification is correct. The bug is a gap between two
policies. The 2026-10-09 deploy decision assumes busy computers update "at
their next idle moment through the existing idle sweep"
(`deploy-restarts-busy-computers-2026-10-09.md`). But the May keepalive
policy turns that sweep off for primaries while the host has spare
capacity. Pressure reclaim hid the gap until yesterday: it hibernated idle
test computers almost every day, and they booted on the new base the next
time they were used.

**Consequence.** No active, mutable computer, user computers included,
ever takes a new build on its own. Deploys restart none of them (by
design since f6001aea). The idle sweep stops none of them while the host
has capacity. Builds arrive only through an explicit
`POST /api/computers/<id>/lifecycle/restart`, the `refresh_active_computers`
CI dispatch, or pressure reclaim. The owner's computer is the separately
preserved, immutable constructed computer, which deploys never touch by
design. CLAUDE.md's sentence "they update idle computers and leave busy
ones to their next idle moment" (`CLAUDE.md:89`) is not true on staging
today.

**How sure.** High for the cause: code, staging configuration and vmctl
logs agree, and the only candidacy rule that tells the idle sweep apart
from the pressure plan is the keepalive rule. The pressure plan's
`eligible=3` count also passes the 30-minute idle check and the guest busy
probe. I did not read vmctl `/health` or `ownerships.json` directly
(logs-only rule), so the per-computer `last_active_at` values are
inferred, not read.

**Recommendation.** Do not turn keepalive off. A hibernate here is a cold
stop with no planned-restart marker, so it would close Autopaper source
schedules and other open obligations as "interrupted by a restart". It
would also make idle `@example.com` QA computers deletable by retention
24 h later. The right fix is a new "update when idle" step: restart an
idle computer that has a pending update as a planned restart, and keep it
on. That step needs a host-requested planned-restart marker, which does
not exist yet. It fits naturally with the hibernation and wake work, so
defer it there. Meanwhile, correct CLAUDE.md's claim, and have deployed
acceptance demos check the guest build and restart the test computer
explicitly.

## Evidence

### 1. What the idle sweep checks (code)

`StartIdleSweeper` runs every `VMCTL_IDLE_SWEEP_INTERVAL` (2 m). Each
sweep runs process reconcile, always-on warming, the pressure-reclaim plan,
retention prune, and then `StopIdleVMs` (`internal/vmctl/ownership.go:905-935`).
`StopIdleVMs` → `CheckIdleOwnerships` → `idleOwnershipCandidates`
(`internal/vmctl/warmness_policy.go:158-216`) skips an active ownership
when any of these hold, in order:

1. it is held (maintenance hold);
2. `LastActiveAt` is zero;
3. `now - LastActiveAt <= idleTimeout` (30 m);
4. the guest busy probe says busy (`warmness_policy.go:180`). The probe
   is `GET <computer_url>/health`, and it is busy when `running_runs > 0`,
   `desk_pending_mutations > 0` or `selfdev_active_operations > 0`, or when
   the guest does not answer, answers non-200, or answers unreadably
   (`ownership.go:2893-2930`);
5. its warmness class is protected (public_platform, premium_always_on,
   critical_protected);
6. **its class is `primary`, the keepalive mode is `under-capacity`, and
   the host is not under pressure** (`warmness_policy.go:187`).

What resets the idle clock: `LastActiveAt` changes only in vmctl
ownership paths. The main one is `resolveDesktopContext`
(`ownership.go:1667, 1680`), which the proxy calls for every product API
request (it caches routes for only 2 s, `internal/proxy/handlers.go:1192`,
`defaultRouteCacheTTL = 2s`). Boot, resume, refresh and recover also set
it. A vmctl restart's reattach keeps it (`ownership.go:683`). Several
things never touch it: guest-originated traffic (gateway inference, event
credentials), open desktop streams after the first resolve, guest
heartbeats, vmctl's own health checks, source schedules (`source_poll_due`
wakes), wake outbox retries and persistent research desks. So a harness
that polls `/api/texture/...` every 20 s keeps a computer "recently
active", but with no harness nothing resets the clock.

`researcher_count` is not an activity signal. It is the configured number
of research slots (`internal/agentcore/api.go:959`,
`ResearchCount: h.rt.cfg.ResearchCount`), and the busy probe never reads
it. **Hypothesis 1 of the first doc (live research desks keep it busy) is
false.**

### 2. Staging configuration (code + log)

`nix/node-b.nix:753-756`:

```
# Keep personal computers resident while the host is under capacity.
"VMCTL_IDLE_TIMEOUT=30m"
"VMCTL_IDLE_SWEEP_INTERVAL=2m"
"VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity"
```

The binary's default is also `under-capacity` (`cmd/vmctl/main.go:546`).
The vmctl restart at 18:35:23Z logged it:

```
18:35:23 vmctl: idle timeout set to 30m0s
18:35:23 vmctl: warmness policy primary_keepalive_mode=under-capacity always_on_user_count=1
18:35:23 vmctl: route-gated idle sweeper interval set to 2m0s
```

The policy is intentional. `docs/archive/vm-priority-policy.md:94-96`
says: "Under normal capacity, primary computers are not idle-reclaim
candidates. This prevents returning real users from paying avoidable
cold-start latency merely because a coarse idle timer fired." Commit
4ae89ab2 (2026-05-19) changed staging from `off` to `under-capacity`.

### 3. The sweeper did consider them, and found them idle and not busy (log)

The pressure-reclaim plan is computed on the same sweep and applies the
same checks except keepalive: protected class, `LastActiveAt` within
`VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m`, guest busy
(`internal/vmctl/pressure_reclaim.go:628-654`). It logged this every 2 m
from 18:35:23Z through at least 21:11Z:

```
18:35:23 pressure reclaim active decision=observe reason="host pressure below reclaim threshold" active=4 eligible=3 protected=1 pressure=false
19:21:23 ... active=5 eligible=3 protected=2 pressure=false   (demo-p2a booted 19:19:35, recent_activity)
21:11:23 ... active=6 eligible=4 protected=2 pressure=false
```

At 18:35Z the four active VMs were the three test computers
(`vm-82cf0bb7…` demo-arxiv-2026-10-10c, `vm-c31cb451…` demo-arxiv-2026-10-10,
`vm-8cac78e5…` demo-ms-2026-10-10b) and the owner's constructed computer
`candidate-fleet-e15cb89f…`. The docs identify that computer as the
owner's G4-frozen computer (`docs/archive/evidence/effects-red-cosuper-refresh-still-running-2026-08-17.md`).
It is the one protected. So all three test computers were idle for more
than 30 m by vmctl's own clock, not busy by the guest probe, and not
protected. The idle sweep skipped them only because of rule 6.

No log line reports an idle-sweep skip, so this attribution is by
elimination. It is still tight: rules 1 to 5 match the pressure plan's
checks (held → `maintenance_hold`, zero `LastActiveAt` →
`unknown_last_active`, idle < 30 m → `recent_activity`, busy →
`guest_busy`, protected class), and the plan counted these three as
eligible.

### 4. The guests were doing nothing (log)

Gateway journal: the last inference request from `vm-82cf0bb7…` was at
17:20:11Z (a chatgpt circuit-open failure). `vm-c31cb451…` and
`vm-8cac78e5…` made no gateway calls between 13:00Z and 21:00Z. The proxy
does not log per-user requests, so harness traffic after the demos ended
can't be counted directly. The pressure plan's idle ≥ 30 m shows that no
proxied request resolved these computers in the 30 minutes before each
sweep.

### 5. The idle sweep has hibernated nothing this week; pressure did all of it (log)

Since the vmctl journal begins (2026-10-05 02:52Z):

- `idle sweeper hibernated`: **0** lines.
- `pressure reclaim hibernated`: 56 lines (Oct 05: 15, Oct 06: 20,
  Oct 07: 8, Oct 09: 7, Oct 10: 6).
- `pressure=true` sweeps: Oct 05: 589, Oct 06: 113, Oct 07: 58,
  Oct 08: 48, Oct 09: 143, Oct 10: 106, **Oct 11: 0**. Last at
  2026-10-10 22:34:55Z.

Example: at 2026-10-10 20:47:07Z pressure crossed the threshold and
reclaim hibernated four VMs, including `vm-c31cb451…`
(demo-arxiv-2026-10-10), with `reason=pressure`. Until yesterday,
pressure was the mechanism that made idle test computers pick up new
builds. Today the host has headroom, so nothing does.

### 6. How these computers actually got new builds (log)

Every boot of the three test computers since 2026-10-10 22:41Z followed an
explicit owner-path restart, `lifecycle request POST /internal/vmctl/stop
caller="proxy.lifecycle.restart"`, then `started retained VM … on resolve`.
That is the `POST /api/computers/<id>/lifecycle/restart` product route
(`internal/proxy/computer_lifecycle.go:147-156`). Examples for
demo-arxiv-2026-10-10c: 02:03:55, 03:07:47, 08:55:50, 10:00:54, 10:13:10,
11:33:12, 13:05:03, 15:12:31 and 17:06:21Z. The QA agents restarted the
computers by hand after deploys. Deploy runs 38163884955 (a2bcf724) and
38170958509 (b172895b) logged "Update pending, not restarted" for each
test computer and "Preserving immutable constructed computer
candidate-fleet-e15cb89f…" for the owner's computer.

## Hypotheses from the first doc, resolved

1. *Live research desks count as busy*: **false**. The busy probe reads
   only running runs, pending desk mutations and active self-development
   operations. `researcher_count` is a configured constant.
2. *Route-gated sweeper plus a live route*: **false as stated**. "Route-gated"
   means a hibernate must pass the ComputerVersion route authority
   (`authorizeLifecycleRoute`, `ownership.go:894`). It is a permission
   check, not an activity signal. The sweeper never got as far as asking,
   because keepalive filtered the computers out first.
3. *Deploys update only stopped computers; there is no later idle
   update*: **true, and the root of it.** Deploys rebuild the base and
   restart nothing (`.github/workflows/ci.yml:1440-1448`, f6001aea). A
   stopped or hibernated computer boots on the new base at its next
   resolve. The promised "next idle moment" path is the idle sweep, which
   keepalive disables for primaries. Slices 1 to 3 of
   `deploy-restarts-busy-computers-2026-10-09.md` (focus signal,
   idle-only restart, update prompt) have not landed: `last_owner_input_at`
   does not appear in the code.

## Consequence for deploys

- **QA test computers** keep their boot-time build until someone restarts
  them, the host comes under pressure, or `refresh_active_computers` runs.
  A deployed acceptance demo on an existing test computer tests old code
  unless it restarts the computer first.
- **Ordinary user computers** (any non-held, mutable, primary computer)
  behave the same: while the host has capacity, a computer someone keeps
  open never updates, and neither does one nobody uses. Only computers
  that are already hibernated or stopped get the new build, at their next
  visit. Guest-to-host skew is unbounded, which violates consideration 3
  of the 10-09 decision ("the update window must be bounded").
- **The owner's computer** is the immutable constructed computer, and
  deploys preserve it by design (G4 freeze). It updates only through its
  own product update or restore path, which is a separate question.
- **CLAUDE.md:89-90** promises behaviour that does not exist on staging.
  That is a doctrine/runtime divergence worth correcting in words before
  any fix.

## Options

**A. Turn keepalive off on staging (`VMCTL_PRIMARY_KEEPALIVE_MODE=off`).**
A one-line config change. Idle primaries would hibernate after 30 m and
boot on the new base at the next visit. Not recommended:

- "Hibernate" today is a VM stop with no memory snapshot
  (`vmmanager/manager.go:927`, per the 10-09 doc), and no host path writes
  a planned-restart marker. Only the guest's own platform-update apply
  does (`internal/agentcore/planned_restart.go:157`). So every idle
  hibernate boots as a crash. Active Autopaper source schedules close as
  "interrupted by a restart" (`internal/agentcore/source_schedule.go:20-24`),
  and the busy probe does not count pending `source_poll_due` wakes. Turning
  keepalive off would silently kill living-document schedules after every
  30 minutes of owner absence. That contradicts "a computer stays on while
  it is working" and the purpose of the persistent tier.
- QA computers are `@example.com`. Retention prune deletes a stopped or
  hibernated ephemeral primary whose `LastActiveAt` is 24 h old
  (`internal/vmctl/retention_prune.go:386-405`, `VMCTL_RETENTION_EPHEMERAL_MIN_AGE=24h`).
  Keepalive is currently what keeps idle QA computers from being deleted.
- It brings back the cold-start cost the May policy was written to avoid.

**B. Fix now: update-when-idle as its own sweep step (red).** Add a step
to the sweeper, separate from hibernation. For an active, mutable,
non-held computer that has a pending update (booted base ≠ current base;
vmctl knows both), is idle by the existing clock and busy probe, and has
no recent owner input, do a planned refresh: ask the guest to write the
planned-restart marker, then `RefreshVMForDesktop`. The computer stays on,
and its schedules and wakes resume after the boot. Requirements: a
host-to-guest "prepare planned restart" call (new guest endpoint), pending-
update detection in the ownership record, a per-computer rate limit, and
a receipt. Slice 1's focus signal is still missing, so "idle" would mean
API-idle, not input-idle. That is acceptable for a computer nobody has
called for 30 minutes. Cost: a new red lifecycle path. Its "boot
fresh, resume via marker" design is the same one hibernation and wake
need.

**C. Defer alongside hibernation and wake (recommended).** Real
hibernation needs the same three things: a pending-update check at wake
("boot fresh instead of resuming memory"), a host-initiated planned-restart
marker, and an idle definition that counts scheduled wakes. Build them
once there. Interim steps, all green or yellow:

- Correct CLAUDE.md:89-90 to describe staging today: deploys restart no
  active computer, and active primaries update only on explicit restart,
  refresh dispatch or pressure reclaim.
- Have deployed acceptance demos compare the test computer's
  `/health` `build.commit` with the deployed SHA and call
  `POST /api/computers/<id>/lifecycle/restart` when it is idle and stale.
  The QA agents already do this by hand (evidence 6).
- Have the deploy job list each pending computer's booted build, so the
  skew is visible in the receipt.

## What to verify next

1. Read vmctl `GET /health` on node-b (localhost:8083, read-only; this
   research did not, under its logs-only rule). Expected: `warmness.idle_eligible_by_class`
   is empty (keepalive applies), while `reclaim.inventory.eligible` and
   `reclaim.candidates` list the test computers as `warmness_class=primary`
   with `idle_seconds > 1800` and no `protected_reasons`. That confirms the
   attribution directly.
2. Check whether `/var/lib/go-choir/vmctl-priority.env` (the
   `EnvironmentFile` override) sets `VMCTL_PRIMARY_KEEPALIVE_MODE`. The
   startup log says `under-capacity` either way.
3. Confirm the owner's computer is the `always_on_user_count=1` entry (the
   `premium_always_on` class) and not only protected as an immutable
   construction.
4. Before choosing B or C: does any QA or demo computer carry active
   Autopaper source schedules? If so, any host restart, including the
   manual restarts QA uses today, already closes those schedules as
   crashes. That is a live issue regardless of this decision.
5. The 2026-09-28 doc `vmctl-idle-sweep-hibernates-busy-guest-2026-09-28.md`
   blamed the idle sweep for hibernating at about 31 minutes. Under
   keepalive that was more likely pressure reclaim, with the same 30 m
   `MIN_IDLE` (hypothesis, not checked; the journal no longer covers
   September).

## Open questions for the owner

1. Should staging and production keep primaries warm while the host has
   capacity (the May policy)? Or should an idle computer give up memory
   once hibernate and wake preserve its schedules?
2. Is a planned refresh of an idle computer with a pending update
   acceptable without a prompt (option B)? Or must every update of a
   computer you are not watching wait for the "Update now / In 5 minutes"
   prompt (slice 3)?
3. What bound do you want on guest/host skew for a computer that is never
   idle (the 10-09 "never idle for several days shows a notice")?
4. Should QA test computers get their own warmness class (for example
   `candidate`) so they idle and update independently of the policy for
   real users, as long as retention does not delete them?
