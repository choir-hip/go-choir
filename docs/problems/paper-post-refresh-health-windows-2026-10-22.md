# The paper's health has two failing windows after a planned refresh (2026-10-22)

Date: 2026-10-22, 18:50 UTC. Found by: FP0, after deploy V's planned
refresh of the paper. Status: open, problem record before any fix.
Mutation class of a fix: red (guest boot path and the paper's
availability).

## Evidence

Deploy V refreshed the paper (vm-1669020a…, critical_protected, busy) at
18:30:28. Its new address was 10.201.173.2. The guest started its server
at 18:30:39 after a 3.6 s store open. vmmanager checks /health every 15 s
with a 3 s timeout and logs only failures
(`internal/vmmanager/manager.go:443-444`, `1597-1607`, `2749-2756`):

- "booted VM" at 18:31:01 means a 200 was seen at about 18:30:40.
- Window A: failures at 18:31:00, 18:31:15, 18:31:30, 18:31:45, 18:32:00.
  It matches the boot phase `passivate_interrupted_activations`
  (18:30:42 to 18:32:04, 81.8 s); the console is silent from 18:30:46 to
  18:32:01 inside it.
- No failures from 18:32:15 to 18:33:30.
- Window B: one failure at 18:33:45, and an operator curl at 18:33:44
  that timed out at connect (10 s), not a refusal or a slow answer. 200 by
  about 18:34:25.

Read-only host checks at 18:49: no guest firewall rebuild between 18:33:20
and 18:34:10 in the vmctl journal, no other computer failed health at
18:33:4x, no conntrack messages in dmesg, host CPU pressure zero.
`boot-timeline.json` has no `first_healthy` field.

## What it is not

- Deploy E's fix holds: the wake-outbox list took 2.65 s this boot
  (3m45s before E).
- The host firewall rules for the new tap are installed before
  Firecracker starts (`manager.go:3616-3660`); the guest answered at the
  new address within a second of its listener.
- /health reads memory and three store reads under a 1 s budget and
  answers 503 `store_stalled` on a miss (`internal/agentcore/api.go`); it
  takes no engine lock and cannot cause a connect timeout.

## Open

1. Window A: the passivation phase grows deploy by deploy (48, 52, 81 s).
   During it one run step held about 75 s. Why passivation takes this
   long, and whether /health fails as 503 or as a timeout then (vmmanager
   discards the status).
2. Window B: a connect timeout on a listening guest. Hypotheses: the guest
   stalled (memory pressure: the daemon's limit plus the desk cgroup's
   memory.max add up to all of guest RAM; or vCPU starvation), or its
   accept queue overflowed. No route reads `ListenOverflows`.
3. The derived-keys validation pass re-reads every row of each family
   (46,381 commitment_record rows) after every process start, during the
   post-boot burst, because its validated state is memory-only. It is not a
   boot-path scan (a goroutine after `rt.Start`) but it competes with the
   burst.
4. Side finding: vmctl learns the new address only after the boot wait, so
   the guest's calls to vmctl are refused until then (a memory-lease
   release at 18:30:39 was refused; a report got through on retry at
   18:31:12).

Next observation: during the paper's next planned refresh (KF step 5),
probe /health every 2 s with `curl -w '%{http_code} %{time_connect}
%{time_total}'`, and read the guest's `/proc/net/netstat` ListenOverflows
and `/proc/pressure/memory` through an existing host-only route if one
exposes them.
