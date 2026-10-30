# A crashed computer stays down until its owner runs `computer start`

Observed 2026-10-30 from the code, on main at 3b1930690.

## What happens

A guest crashes. The next vmctl lookup finds it unhealthy past the route
grace and latches the ownership `degraded`
(`reconcileLookupReadiness`, internal/vmctl/ownership.go). A bearer-key
request then meets `wakeableAPIKeyState`
(internal/proxy/api_key_computer_authority.go, added by 58311f268 on
2026-10-26), which lists `degraded` and `failed` as states a request does not
wake, and answers `503 computer_waking_failed state=degraded`. It keeps
answering that until the owner runs `choir computer start`.

## Why this is wrong

CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)": a crash
restart never resumes work (boot closes every open obligation with the fate
"interrupted by a restart"), and there must be no crash-restart loops. The
owner never asked that a crashed computer stay down. The refusal came from the
agent's drafting of the bearer-wake decision
(docs/problems/api-key-request-to-a-hibernated-computer-gets-502-2026-10-26.md:
"A state that cannot be woken by a request (booting, stopping, degraded,
failed)"); the owner's decision was only that a bearer request wakes its
computer.

The browser path was already different: vmctl's resolve cold-boots a degraded
or failed computer (a new epoch, no planned-restart marker). So a crash healed
for a page load and not for the CLI, and nothing bounded either: a computer
that crashes on every boot was restarted by every request, forever.

## Every place that refused a degraded or failed computer

- proxy bearer wake, `wakeableAPIKeyState`: refused. Now wakes.
- proxy compute-panel wake, `runComputeRecovery` (compute_status.go): left a
  degraded or failed computer running and told the user to reload. Now
  resolves it, like the reload does.
- vmctl `resolveDesktopContext` (cookie path, platform ensure): already
  started both; it had no bound and no look at a guest that still answers.
- vmctl always-on sweep (`alwaysOnWarmEligibleLocked`), platform warm
  (`WarmUniversalWirePlatformComputer`): start only stopped or hibernated
  computers. Unchanged; see residuals.
- vmctl deploy refresh and rescue: refresh only active computers; rescue
  only failed or failure-stopped always-on computers. Unchanged.

## Decision

Wake path: the next request, through the wake paths that exist. vmctl's only
crash detector is the lookup health check, which is itself driven by a
request, and resolve already cold-boots a crashed computer, so the least new
machinery is to let the bearer and compute-panel wakes reach that resolve. No
new restart driver, no new sweep.

The bound lives in vmctl, which owns the lifecycle, on the ownership row (so it
is durable): `crashes`, each with a time, realization epoch, kind and cause.
Crashes are recorded where vmctl learns of an unplanned exit: the lookup latch
(active to degraded), a resolve that finds an active computer's guest process
exited, a wedge stop, and a restart whose guest did not come up (a crash during
boot). A planned restart (refresh, rescue, recover, hibernate, owner stop, vmctl
restart) holds a lifecycle claim or changes the state first, and is never
recorded. A realization is one crash however often it is seen unhealthy.
Refusals that spawned nothing (memory pressure, another start in flight),
durable recovery refusals, and host or control-plane failures (no credential, no
epoch) are not crashes: an outage of one must not hold every computer restarted
during it.

`crashLoopMaxCrashes = 3` inside `crashLoopWindow = 30m` (internal/vmctl/
crash_loop.go) set the existing durable maintenance hold with
`HeldBy = "vmctl:crash-loop"` and reason `crash loop: 3 crashes in 30m`. Every
automatic path already refuses a held computer; resolve now answers a
crash-held one with a `crash_loop` refusal (503, the reason, Retry-After)
instead of an old URL. Lookup and `computer status` show `held`, `hold_reason`
and `held_by`.

The owner's `computer start` (also restart and recover) is marked
`owner_start` on the resolve; vmctl clears the crash history and a hold it set,
never an operator's maintenance hold. `Unhold` of a crash-loop hold clears the
history too.

An automatic restart of a degraded computer whose guest still answers /health
and reports a self-development apply or a live capsule is refused (the readings
a planned host stop uses), never forced.

## Failure modes pinned (internal/vmctl/crash_loop_test.go and the proxy tests)

A restart loop; a planned restart counted as a crash; the owner start not
clearing the hold; two wake paths racing into two restarts; a crash during
boot retried without bound; the hold lost on a vmctl restart; the window never
sliding; a restart mid-apply; a held computer answering with an old URL or
hanging its request (resolve left a nil `pendingWaiters` entry for a held
computer, which the next resolve waited on: fixed with the hold).

## Rollback

Revert the commit. The `crashes` field is ignored by the old binary. A computer
held by the bound at that moment reads as an ordinary maintenance hold until
`computer recover` (unhold) clears it.

## Residuals

- An always-on computer that crashes with no request to find it is not warmed
  by the sweep, which only starts stopped or hibernated computers. The
  platform computer is found by public reads; others wait for their next
  request. A crash probe in the sweep would be new machinery.
- A degraded latch can follow a live but slow guest; resolve has always
  recovered such a guest in place. The mid-apply refusal narrows that;
  the rest is the pre-existing cookie-path behavior, now shared by the bearer
  path.
- A crash-held computer whose process is still alive stays alive and
  unreconciled until the owner starts it (the maintenance-hold behavior).
