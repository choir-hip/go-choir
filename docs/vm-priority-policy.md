# VM And Computer Priority Policy

**Status:** current policy and roadmap
**Last updated:** 2026-05-20

This document defines how Choir decides which VM-backed computers stay warm,
which may hibernate, and what a future paid or reserved 24/7 uptime tier must
mean.

Product language should say **computer**. Implementation language may say
**VM**, **vmctl**, **Firecracker**, or **sandbox** when describing the substrate.

## Current Policy

The deployed staging policy is:

```text
VMCTL_IDLE_TIMEOUT=30m
VMCTL_IDLE_SWEEP_INTERVAL=2m
VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity
VMCTL_PRESSURE_RECLAIM_MODE=active
VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m
VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES=5
VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_MIB=32768
VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_PERCENT=10
VMCTL_STALE_STATE_MIN_AGE=6h
VMCTL_STALE_STATE_MAX_DELETES=5
```

Node B also loads optional operator priority overrides from:

```text
/var/lib/go-choir/vmctl-priority.env
```

That file may set:

```text
VMCTL_ALWAYS_ON_USER_IDS=<auth user UUID>,<auth user UUID>
```

Use auth user UUIDs, not email addresses. The auth database knows both, but the
vmctl ownership registry is keyed by authenticated user id.

## Warmness Classes

vmctl classifies ownership records into warmness classes:

| Class | Meaning | Current behavior |
| --- | --- | --- |
| `premium_always_on` | Explicitly configured always-on primary computer | Protected from ordinary idle and pressure reclaim; hibernated/stopped published primary is resumed by the warmer |
| `critical_protected` | Worker for verifier, promotion, rollback, or publication work | Protected while recent; stale critical workers become reclaimable after the critical protection window |
| `public_platform` | Future public/default platform computer lane | Modeled for priority ordering; not yet the main deployed route policy |
| `primary` | Ordinary active user computer | Kept warm while host is under capacity; may be reclaimed only after lower-priority idle resources are exhausted under pressure |
| `candidate` | Candidate/background user computer | Lower retention priority than primary; expected to hibernate when idle or under pressure |
| `worker` | Ordinary worker VM | Lowest retention priority; expected to hibernate first when idle or under pressure |

Classification order matters:

1. If the owner user id is in `VMCTL_ALWAYS_ON_USER_IDS`, the ownership is
   `premium_always_on`.
2. If an ownership has an explicit valid warmness class, vmctl uses it.
3. Worker VMs with verifier/promotion/rollback/publication purpose markers are
   `critical_protected`.
4. Other worker VMs are `worker`.
5. Non-primary interactive desktops are `candidate`.
6. Primary interactive desktops are `primary`.

## Reclaim Order

Warmness priority is ordered so lower numbers reclaim first:

```text
worker < candidate < primary < public_platform < premium_always_on < critical_protected
```

The current priority values are:

```text
worker: 5
candidate: 10
primary: 20
public_platform: 30
premium_always_on: 90
critical_protected: 100
```

Under normal capacity, primary computers are not idle-reclaim candidates. This
prevents returning real users from paying avoidable cold-start latency merely
because a coarse idle timer fired.

Under pressure, vmctl ranks active VMs by protection, priority, and idle time.
It hibernates only a bounded number of unprotected idle candidates per sweep.
If lower-priority resources exist, ordinary primary computers are skipped before
pressure reclaim considers them. Recent activity and unknown last-active state
remain protected.

Candidate computers and workers are intentionally hibernation-friendly. They
are mutation contexts and should not consume scarce capacity ahead of a real
user's primary desktop.

## Stale VM-State Reclaim

Hibernation frees CPU and memory pressure, but it does not free the sparse
per-VM state directories under `/var/lib/go-choir/vm-state`. Large
Choir-in-Choir portfolio runs can therefore leave many stopped or hibernated
candidate/worker `data.img` directories behind even after active pressure
reclaim has done its job.

The current hardening policy treats low state-dir free space as pressure. When
state-dir pressure is present, vmctl may delete a bounded number of stale
terminal VM-state directories per sweep:

- eligible states: `stopped`, `hibernated`, or `failed`;
- eligible kinds: worker VMs and unpublished non-primary candidate computers;
- minimum age: `VMCTL_STALE_STATE_MIN_AGE`;
- sweep bound: `VMCTL_STALE_STATE_MAX_DELETES`;
- protected: active, booting, degraded, stopping, primary, published,
  premium-always-on, recent, unknown-last-active, and recently critical
  verifier/promotion/rollback/publication work.

This is a substrate cleanup policy, not a product deletion action. It must only
delete disposable producer machine state after package/source/adoption evidence
has moved into durable product ledgers. Owner review should use
AppChangePackage and adoption refs, not stale source VM disks.

## Always-On Semantics

`premium_always_on` is a product promise shape, not just a boolean.

Current implementation:

- configured always-on primary computers are protected from ordinary reclaim;
- if an existing published primary ownership for a configured user is stopped or
  hibernated, the idle sweeper's warmer resumes it;
- the warmer does not create brand-new ownership records;
- the warmer does not warm candidate desktops;
- the warmer does not warm worker VMs;
- health exposes only aggregate class counts and policy names.

Current limitation:

- the source of truth is an operator environment file, not a product account
  tier table;
- changes require service environment update and vmctl restart/reload;
- the configuration uses auth user UUIDs, not emails;
- there is no capacity admission controller for overselling always-on slots yet.

## Health And Privacy

Browser-public health may expose aggregate lifecycle state such as:

```text
warmness.policy.primary_keepalive_mode
warmness.policy.always_on_user_count
warmness.by_class
warmness.active_by_class
reclaim.inventory
reclaim.decision
```

It must not expose user ids, emails, VM ids, desktop ids, prompt text, gateway
tokens, credentials, or private file paths.

## Future Policy

The near target is to move priority policy from operator files into product
state while preserving the same semantics:

1. Store uptime entitlements in platform Dolt, keyed by account/user/computer.
2. Keep vmctl's aggregate health redacted.
3. Add a safe hot-reload or reconcile path for always-on entitlements.
4. Add capacity admission so paid 24/7 slots cannot exceed the host or fleet
   budget without an explicit degraded-state decision.
5. Use Node A and Node B together for migration, route handoff, and reduced
   deploy/cold-start downtime.
6. Give users honest UX status: warm, waking, under pressure, degraded,
   migrated, or awaiting capacity.
7. Add alerts for unexpected reclaim of primary or always-on computers.
8. Add route-level rollback so a bad VM lifecycle or deploy change can return
   the user to the prior active computer.

Longer term, the platform should treat warmness as a scheduling contract over
computers:

```text
entitlement + recent activity + work criticality + host pressure + migration options
-> keep warm | resume | hibernate | migrate | reject new work | escalate
```

## Operational Rules

- Do not use a short idle timeout as the main product policy.
- Do not reclaim a primary user computer while lower-priority idle workers or
  candidates are available.
- Do not treat candidates as 24/7 resources unless a specific mission reserves
  them and records why.
- Do not model paid uptime as a UI flag without capacity, keepalive, reclaim,
  migration, and verification semantics.
- Do not put raw user identifiers into public health or Trace summaries.
- Do not bypass git/CI/deploy for tracked platform policy changes.

## Code Anchors

- `internal/vmctl/warmness_policy.go` - warmness classes, keepalive mode,
  ranking, idle-candidate filtering.
- `internal/vmctl/pressure_reclaim.go` - pressure candidate ranking and
  protected reclaim reasons.
- `internal/vmctl/ownership.go` - idle sweeper, hibernate/resume behavior,
  always-on warmer.
- `cmd/vmctl/main.go` - environment parsing for lifecycle policy.
- `nix/node-b.nix` - staging Node B vmctl service policy and runtime priority
  environment file.
