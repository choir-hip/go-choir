# vmctl idle sweep hibernates a computer with in-flight runs

**Date:** 2026-09-28
**Class:** red (VM lifecycle — hibernation decides when a guest's execution
context is suspended; protected surface per the mutation classes)
**Status:** open — documented first per the problem-documentation-first
invariant; fix commit follows.

## Evidence

Episode probe `M11_SELFDEV_EPISODE_1790581274029` (and every prior probe
today): the bound engineering desk run `run:assignment-28abe1fc-…` was at
tool-loop iteration 129 and actively issuing gateway inference calls
(gateway journal shows `inference request from autoputer vm-455f5a…`
through 08:20) when vmctl hibernated `computer-aee82d4e…` at 08:12:46.
`last_active_at` at suspension: 07:41:24 — the op-creation timestamp.
The computer was 31 minutes into a desk run and had never been idle.

Episode probe `M11_SELFDEV_EPISODE_1790587431621` repeated identically:
`computer-433f40ed…` hibernated at 09:55:36, again exactly ~31 minutes
after `primary_started`, again mid-desk-run at iteration 88+.

Probe `M11_SELFDEV_EPISODE_1790580270598` likewise (VM hibernated while the
engineering run churned), and its op `selfdev-936cdeea…` remains pinned
`executing`.

## Root cause

`OwnershipRegistry.CheckIdleOwnerships` → `idleOwnershipCandidates`
(`internal/vmctl/warmness_policy.go:158`) decides idleness from a single
signal: `own.LastActiveAt`, the host-side ownership timestamp. It is bumped
only by ownership-resolution paths — create/resume/desktop-context resolve
(`ownership.go:616`, `1415`, `1508`, `1931`…). The probe's authenticated API
polls do not pass through those paths; guest-side work (tool-loop cells,
gateway inference, run progress) never reaches the ownership record at all.

So a computer running a 40+ minute engineering assignment looks idle to the
sweeper the moment the bootstrap/last-resolution timestamp ages past
`idleTimeout` (~30 min in deployed config). Every M11 episode today has been
suspended mid-desk-run by exactly this gap.

`run busy` is already observable: `GET /health` on the guest reports
`running_runs` (`api.go:1016-1022`). The sweep simply never consults it.

## Scope of the fix

`idleOwnershipCandidates` needs a busy check before candidacy: for an active
ownership whose `LastActiveAt` exceeds the idle timeout, GET the guest's
`/health` (short timeout, ownership `computer_url`) and treat
`running_runs > 0` as not idle. A guest that fails to answer is already a
hibernation candidate for other reasons; on read failure keep the existing
idle decision (fail-open on the sweep, fail-closed on activity claims).
Deliberately no guest-side changes, no new heartbeat protocol, no probe
workaround — the busy signal already exists.

Failure mode if unchanged: every desk leg longer than `idleTimeout` is
suspend-and-restart. The restart-recast path recovers (attempt 2 observed
relaunching cleanly post-hibernation) but doubles episode latency and puts
the probe's 60-minute `awaiting_approval` budget under the combined desk
restart + execution time — which is why tonight's probe runs keep timing
out `blocked` instead of reaching the verdict.

## What proves closure

`scripts/m11_selfdev_episode_probe.mjs` runs to `awaiting_approval` without
the episode VM hibernating mid-desk-run; a unit test pins the busy-check:
ownership with `running_runs>0` on a live guest is never an idle candidate,
ownership whose guest is unreachable still hibernates.
