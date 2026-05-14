# MissionGradient: Pressure-Aware Computer Lifecycle

**Status:** active implementation
**Created:** 2026-05-14

## Real Artifact

Optimize the deployed user-computer lifecycle controller:

```text
browser/session intent
-> authorized computer resolution
-> warm, resume, recover, or boot decision
-> gateway credential reconciliation
-> observable desktop boot progress
-> pressure-aware reclaim with preserved state and rollback
```

The artifact spans `vmctl`, `vmmanager`, proxy bootstrap behavior, staging
runtime configuration, lifecycle observability, and the desktop loading UI. It
is not just a larger idle timeout, a prettier spinner, or a local-only benchmark.

The low-resolution system is still the real deployed app on
`https://draft.choir-ip.com`: a new account boots, a returning account resumes,
bootstrap progress is visible, and reclaim decisions are explainable. Higher
resolution adds pressure-based eviction, snapshot/resume paths, concurrent users,
background/candidate computers, and longer-lived live work.

## Prior Art And Local Learnings

The mission must begin with a short research pass and fold durable findings into
this document or a follow-up architecture note. Use primary sources where
possible:

- Firecracker snapshotting: snapshots capture guest memory and microVM state;
  restored network/vsock connections are not guaranteed, and snapshot files are
  trusted host artifacts that need lifecycle security.
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md>
- AWS Lambda SnapStart: cold-start reduction comes from initialized
  Firecracker-backed snapshots, but uniqueness, entropy, secrets, and network
  connections require after-restore care.
  <https://docs.aws.amazon.com/lambda/latest/dg/snapstart.html>
- AWS Lambda lifecycle: restore has its own phase, timeout, hooks, and
  `RESTORE_REPORT` observability.
  <https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html>
- Fly Machines autostop/autostart: useful knobs include stop vs suspend,
  autostart pairing, minimum running machines, and the warning that large
  per-user environment fleets need explicit lifecycle management rather than a
  generic proxy loop.
  <https://fly.io/docs/launch/autostop-autostart/>
- Linux PSI: `/proc/pressure/{cpu,memory,io}` gives short, medium, and long
  pressure windows plus pollable pressure triggers.
  <https://kernel.org/doc/html/v6.0/accounting/psi.html>
- Kubernetes node-pressure eviction: pressure systems should monitor memory,
  disk, and PID scarcity, reclaim lower-value resources first, and account for
  polling lag and OOM races.
  <https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/>
- systemd cgroup resource control: `MemoryHigh` is the main throttling mechanism
  and `MemoryMax` is the last line of defense for a unit.
  <https://www.freedesktop.org/software/systemd/man/devel/systemd.resource-control.html>

Local learnings to preserve:

- Returning accounts must not depend on clearing cookies, local storage, or a
  fresh browser profile.
- A failed resume path can strand an existing active computer unless the
  ownership registry and VM manager converge on recoverable lifecycle state.
- VM processes can survive control-plane restart; reattachment and gateway
  credential reconciliation are part of the lifecycle, not cleanup trivia.
- The frontend must expose causal boot progress and keep logout reachable during
  load and failure.
- Staging currently uses `VMCTL_IDLE_TIMEOUT=6h` and
  `VMCTL_IDLE_SWEEP_INTERVAL=5m` as a coarse safety valve. This is temporary
  until pressure-aware reclaim exists.

## Implementation Slice: Dry-Run Pressure Policy

The first implementation slice introduces pressure-aware reclaim in observation
mode only:

```text
host pressure sample
-> active VM inventory
-> protected-work reasons
-> ranked reclaim candidates
-> vmctl/proxy health summary
```

No VM is hibernated by pressure policy in this slice. The existing idle timeout
path remains the only automatic hibernation path. Staging enables:

```text
VMCTL_PRESSURE_RECLAIM_MODE=dry-run
VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m
VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_MIB=2048
VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT=15
VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG10=1.0
VMCTL_PRESSURE_MAX_CPU_SOME_AVG10=90.0
VMCTL_PRESSURE_MAX_IO_SOME_AVG10=5.0
VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES=5
```

Dry-run telemetry reads `/proc/meminfo`, `/proc/pressure/{memory,cpu,io}`,
state-dir filesystem headroom, and process-ID headroom. Candidate ranking favors
worker/background computers before published branch desktops before primary
interactive desktops. Protection currently covers recent activity, unknown
last-active state, and worker purposes that look like verifier, promotion,
rollback, or publication work.

This is intentionally narrower than active reclaim. The next realism axis is to
connect protected-work checks to live prompt/run/file/verifier/promotion state,
then enable active reclaim behind a fast rollback knob.

## Invariants

- The product object is a persistent user computer. `sandbox` remains an
  implementation/service name only.
- Each authenticated user/desktop pair resolves to one isolated active computer
  route at a time. Different users do not share mutable private computers.
- Public logged-out desktop viewing must not hydrate or mutate a private active
  computer.
- Reclaim, stop, hibernate, resume, recovery, and snapshot restore preserve
  durable user state and do not lose foreground updates.
- Crash recovery and resume do not duplicate canonical effects. Epoch semantics
  remain explicit: resume preserves epoch; fresh boot/recovery increments epoch.
- Provider credentials never enter guest files, environment, process arguments,
  snapshots, memory images intended for reuse, or browser-visible state.
- Gateway credentials are VM identity credentials only; they must be reconciled
  after restart, resume, recovery, or route switch.
- A live foreground user session, in-flight prompt, LLM call, file write,
  verifier run, promotion, or publication action is protected from idle reclaim.
- Background/candidate computers are lower priority than the visible active
  computer unless the candidate is inside an explicit verifier/promotion window.
- Reclaim policy must be based on measured capacity or pressure, not only a
  short fixed idle timer.
- Loading UI must reflect real causal state when available: resolving computer,
  booting VM, reconciling credential, waiting for guest health, restoring
  desktop state, connected, retrying, or failed.
- Logout and auth recovery stay reachable during loading and failure.
- Platform behavior-changing work remains staging-first:
  commit -> push main -> monitor CI/deploy -> verify deployed identity -> run
  deployed product-path proof.

## Value Criterion

Minimize returning-user time to interactive desktop and black-screen time while
preserving isolation, credential boundaries, durable state, and host stability.

Optimize:

- p50/p95 time from page load or login to `[data-desktop-ready="true"]`;
- percent of returning sessions served by warm or fast-resumed computers;
- number and duration of bootstrap 502/503 retries visible to the user;
- host memory, CPU, IO, disk, and PID headroom under normal staging load;
- accuracy of lifecycle state shown in UI, logs, and health/trace records;
- recovery rate from failed/pending/stale VM manager state without manual
  intervention.

Penalize:

- bypasses of auth, vmctl, gateway, or product APIs;
- shared mutable private state;
- credential exposure through snapshots, logs, guest env, or browser state;
- killing live user work to satisfy a resource metric;
- hiding uncertainty behind an infinite spinner or cosmetic boot animation;
- tests that pass only by mocking away proxy/vmctl/gateway behavior;
- local-only proof for staging VM lifecycle claims.

## Homotopy Parameters

Increase realism along these continuous axes:

- Users: one new account -> one returning account -> multiple returning accounts
  -> concurrent active users plus background/candidate computers.
- Lifecycle: warm running VM -> stopped/resumed VM -> recovered failed VM ->
  hibernated/snapshotted VM -> restored route after control-plane restart.
- Policy: fixed timeout -> measurement-only pressure policy -> dry-run reclaim
  decisions -> active pressure-aware reclaim -> pressure-aware snapshot/resume.
- Resource signal: manual memory reading -> host `/proc` metrics -> PSI windows
  -> cgroup/systemd accounting -> lifecycle events correlated with user latency.
- UX feedback: static boot console -> state-specific progress -> progress tied
  to bootstrap/retry/lifecycle events -> visible remediation on failure.
- Work protection: no in-flight work -> prompt submission -> LLM call ->
  file/upload write -> verifier/promotion/publication action.
- Proof: focused unit tests -> local integration where useful -> deployed
  staging proof with build identity -> repeated canary-style return visits.

## Dense Feedback Channels

Use feedback that exposes the error field:

- vmctl unit tests for resolve/resume/recover/reclaim state transitions,
  singleflight behavior, epoch semantics, and gateway credential reconciliation.
- vmmanager tests for orphan detection, process identity, snapshot/resume safety,
  and cleanup refusal when ownership is ambiguous.
- Policy tests using synthetic VM inventories and host pressure samples to prove
  eviction ordering and protected-work exclusions.
- Security tests or assertions proving provider credentials are absent from
  guest env, persistent dirs, process args, logs, and reusable snapshots.
- Frontend tests proving boot console visibility, logout reachability, retry
  status, and no black screen during slow bootstrap.
- Staging health/build identity checks for proxy and runtime service commit.
- Staging Playwright proof for public desktop, new-account boot,
  returning-account resume, simulated bootstrap delay, logout, and recovery from
  an intentionally hibernated or stopped computer.
- Lifecycle logs or trace events with `user_id`, `desktop_id`, `vm_id`, epoch,
  decision reason, pressure sample, protected-work state, and route outcome.
- Baseline/after measurements for time-to-desktop-ready and warm-hit ratio.

## Forbidden Shortcuts

- Do not fix returning-account failures by telling users to erase cookies or
  local browser state.
- Do not replace private active computers with a shared mutable VM.
- Do not allocate private mutable computers for signed-out public viewing.
- Do not simply set the idle timeout to infinity and call the mission complete.
- Do not kill or hibernate live user work without an explicit protected-work
  check and durable failure/recovery record.
- Do not store provider credentials in guest images, VM data dirs, snapshots, or
  browser-visible bootstrap payloads.
- Do not use browser-public internal routes such as `/api/agent/*`,
  `/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints
  for acceptance.
- Do not edit tracked files directly on Node B as the source of truth.
- Do not claim Firecracker snapshot readiness without proving network,
  credential, uniqueness, entropy, and state-restoration behavior.
- Do not make the boot console a fake progress bar that advances independently
  of real lifecycle events once those events are available.

## Rollback Policy

Every implementation step must be revertable by git SHA. For behavior-changing
work, record:

- pushed commit SHA;
- GitHub Actions run and staging deploy job;
- `/health` proxy and runtime deployed commit identity;
- deployed acceptance command and result;
- any created test users/desktops/VM IDs;
- lifecycle policy configuration used for the proof;
- VM state affected by forced stop/hibernate/recovery tests;
- rollback knobs, including disabling pressure-aware reclaim, restoring the
  previous idle timeout, and reverting UI lifecycle event consumption.

Pressure-aware reclaim should first ship in observation or dry-run mode if the
implementation materially changes eviction behavior. Active reclaim must have a
fast configuration rollback.

## Learning Side-Channel

Classify discoveries during the mission:

- Tactical learning: update implementation, tests, or lifecycle telemetry
  directly.
- Target-level learning: update this mission doc or split a follow-up mission
  if research shows that snapshots, cgroups, PSI, or boot UX should be staged
  differently.
- Invariant-level learning: stop and escalate before changing privacy
  isolation, credential placement, active-computer ownership, promotion
  semantics, or public/private route boundaries.

Durable learnings should land in:

- this mission document;
- `docs/current-architecture.md` if the lifecycle model changes;
- `docs/runtime-invariants.md` if new resource, reclaim, credential, or
  observability invariants become operating rules;
- regression tests and staging proof notes.

## Stopping Condition

The mission is complete when staging proves:

- returning existing accounts reach the desktop without cookie clearing;
- new accounts and logged-out public desktop still work;
- slow or failing bootstrap shows truthful progress/error state with logout
  reachable;
- the lifecycle controller records measured warm/resume/recover/boot decisions;
- pressure-aware policy exists at least in dry-run with observed host pressure,
  VM inventory, protected-work checks, and proposed eviction order;
- active reclaim, if enabled, chooses lower-value idle resources first and
  preserves durable state;
- no provider credential leaks into guest or snapshot surfaces;
- deployed health reports the expected commit;
- residual risks and the next realism axis are named plainly.

## Short Goal Prompt

Use MissionGradient. Complete
`docs/mission-pressure-aware-computer-lifecycle-v0.md` by researching prior art
and optimizing the deployed user-computer lifecycle for security, performance,
and UX under its invariants and verification criteria. Preserve topology, avoid
forbidden shortcuts, prove behavior on staging for platform changes, and
stop/escalate on invariant-level surprises.
