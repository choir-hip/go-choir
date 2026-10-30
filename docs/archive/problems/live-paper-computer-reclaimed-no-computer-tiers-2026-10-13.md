# The Live Paper Computer Was Reclaimed: There Are No Computer Tiers

Date: 2026-10-13. Found while recording the paper computer's publications
before C1e B1. Status: fixed at 188f98c3 (computer tiers), deployed and verified 2026-10-13 23:51-23:54 UTC.

## What happened

At 21:00:10 UTC, Node B's vmctl pressure reclaim hibernated the Autopaper
paper computer (`computer-1355b83bced1613eb048ee28d66688a8`, VM
`vm-1669020adcece51c2f5ff9b6df399ef5`).

vmctl log, in order:

1. From 20:50 to 20:54: `pressure reclaim active decision=observe ...
   active=5 eligible=0 protected=5`.
2. From 20:56: `eligible=1 protected=4`. The paper computer lost its only
   protection: it was idle between its 10-minute polls, with no in-flight
   runs.
3. At 21:00:10: `decision=reclaim reason="host pressure crossed active
   threshold" ... eligible=1`, then `destructive kill ...
   cause=hibernate`, then `hibernated VM ... reason=pressure`.

Hibernation kills the guest process, so the next wake is a crash boot,
which closes the schedules. API keys never wake a computer
(`proxy/api_key_computer_authority.go:164-183`), so nothing has woken
it since. Product reads answer 502 `failed to resolve user autoputer`.
At 23:28 UTC Node B had 21 of 31 GB available. Published stories stay
public, because public reads never reach the paper computer.

Earlier the same afternoon, two deploy refreshes (19:49 and 20:40 UTC)
were deferred because the computer was busy. Up to the reclaim, it was
live and working.

## Why: protection is per moment, not per computer

`protectedReclaimReasons` (`internal/vmctl/pressure_reclaim.go:628-654`)
protects a computer only if one of these holds:

- it is held;
- its warmness class is `premium_always_on` or `critical_protected`;
- it has recent activity;
- its guest is busy.

The warmness class comes from `warmnessClassForOwnership`
(`internal/vmctl/warmness_policy.go:86-101`). It is
`premium_always_on` only for user ids in the operator env file
`VMCTL_ALWAYS_ON_USER_IDS` (`/var/lib/go-choir/vmctl-priority.env`, one
id today). Otherwise it is the ownership's persisted `WarmnessClass`,
which nothing in the code ever sets. Failing both, it is `primary`.

So:

- There is no account tier. The "always on" list is a per-UUID operator
  list, outside the repo, holding one account.
- There is no computer class for a platform computer like the paper
  computer. The only platform computer vmctl knows is the retired World
  Wire platform owner (`isPlatformOwnership`,
  `internal/vmctl/ownership.go:1010-1016`).
- A computer that runs on a schedule is unprotected between runs.

## Owner ruling (2026-10-13, about 23:40 UTC)

> The policy of allowing one always on user is false. There should be a
> tier for paid users. That is the tier that my account, which is called
> the owner account, should be on. It shouldn't be an owner account. It
> should just be a normal paid tier, uh, always on. And the auto paper
> computer should also always be always on, always. Uh, it's even higher
> tier than the user computers.

Read as policy:

1. Always-on is a property of a paid account tier, not of a list of ids.
2. The owner's account is an ordinary paid account. No owner special
   case.
3. Autopaper computers are a platform tier above every user tier. They
   are always on.

## Standing questions this touches

- Single state authority. The tier must live in one durable place, not
  in an env file beside the code.
- Deletion citers. The env list and its `AlwaysOnUserIDs` config go once
  the tier exists.
- Restart durability. A tier must survive vmctl restarts and deploys.
- No-SSH operability. Setting a tier must not require editing a file on
  Node B.

## Consequence for capacity

If paid and platform computers are never reclaimed, memory admission has
to refuse lower tiers first. vmctl already exempts protected classes in
`admitHostMemory` (`internal/vmctl/host_capacity.go:249-260`). Test
computers minted by tracers become the first thing reclaimed. Free-tier
behavior is unchanged. This is the opening station of the resource
efficiency and cost mission the owner asked to run tonight.

Mutation class of the fix: red (vmctl, VM lifecycle).

## Resolution (188f98c3, CI 38485963385, deployed 23:51:38 UTC)

- Tiers live in `internal/computertiers/computer-tiers.json`, embedded in
  vmctl. It lists one paid account (the owner's, now an ordinary paid
  account) and one platform computer (the paper computer). A tier changes
  by a reviewed commit and ships with the deploy. The env list,
  `AlwaysOnUserIDs` and the `vmctl-priority.env` hook are deleted. The
  file itself is still on Node B but is no longer read.
- vmctl at start logged `paid_user_count=1 platform_computer_count=1`. At
  23:51:48 it logged `warmness policy resumed 1 always-on desktop VM(s)`,
  waking the paper computer. `/internal/vmctl/list` shows it `active` as
  `critical_protected`, and the owner's computer as `premium_always_on`.
- Waking it was a crash boot, so its 14 schedules closed
  `interrupted_by_restart` (23:51:47). The owner act (POST
  `/api/autopaper/sources/schedules`, under the metamission's standing
  ruling) restarted all 14 at 15-minute intervals. All 14 polled at
  23:53:55.

Residuals:
- The always-on sweep now resumes the platform computer whenever it is
  stopped. If it crashed on every boot, the sweep would retry on every
  pass. That is the restart station's L1b, the per-computer boot ledger,
  which bounds automatic boots per episode. Until L1b lands, the bound is
  vmctl's existing recovery admission.
- After any crash boot, Autopaper still needs the owner act to restart
  its schedules. That is by design: a crash never resumes work.
- Each interactive guest is shaped at 8 GiB on a 31 GiB host
  (`nix/node-b.nix`, `VM_INTERACTIVE_MEM_MIB`), so a few active computers
  reach the pressure threshold. Memory per computer is the resource
  mission's next question.
