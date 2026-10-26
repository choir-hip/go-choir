# Density station D: the problem per item and its failure modes, written before each item's code (2026-10-26)

Companion to `docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md`
(FP0's investigation of the 11:49 refusal; landing with
`docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md`).
Goal file: `docs/definitions/choir-density-2026-10-26.md`. Each section is
committed before the tests and code of its item (problem documentation
first). Mutation class of this document: green. Every item it describes is
red.

## D1. A refused start never makes room

### Problem

`admitHostMemoryWith` (`internal/vmctl/host_admission.go`) compares one
MemAvailable sample against `floor + headroom + reserved + need` and returns
a `host_memory_pressure` refusal (503, Retry-After 60). Nothing on that path
hibernates anything. The two paths that hibernate run on their own timers:
the idle sweep (`StopIdleVMs`, 30 minutes, and never a primary computer
while the host is under capacity) and sustained pressure reclaim (only
under the 4779 MiB floor with PSI avg60 over 5.0 for four samples). Between
the floor and the admission line, starts are refused and nothing is
reclaimed, so a person stops a computer by hand. Today 11:49 b@c was 448 MiB
short and FP0 stopped d@e. Fourth recorded recurrence (10-13, 10-22 03:16
and 05:25, 10-26 03:1x).

### Callers

- `startExistingVM` (`ownership.go`): resolve of a stopped or hibernated
  computer, the explicit resume, deploy rescue retries, platform warms. No
  registry lock held; the starter holds its own lifecycle claim.
- `rescueBoot` (`ownership.go`): deploy rescue. No registry lock held.
- the new-computer branch of `resolveDesktopContext` (`ownership.go`):
  calls `admitHostMemoryLocked` **while holding `r.mu`**, before any
  ownership or waiter is registered.

### Design (proposal; the decision recorded in the goal file's `now`)

On a `host_memory_pressure` refusal with `VMCTL_ADMISSION_MAKE_ROOM=active`:

1. Take the single make-room slot (one decider; a second refused start
   waits for the first, then re-admits on a fresh sample before deciding
   anything itself).
2. Rank victims: active, not held, no lifecycle claim, not the starter, not
   a protected class (premium_always_on, critical_protected,
   public_platform), warmness priority at or below the starter's (a test
   start never evicts a primary computer), the pressure-reclaim idle rule
   (a test computer yields to a real one with no idle time; otherwise idle
   at least `VMCTL_PRESSURE_RECLAIM_MIN_IDLE`; a test victim of a test
   starter must be idle past the admission settle window), lowest priority
   first, longest idle first. The guest busy probe (fail closed: only a 200
   `/health` with no runs, no pending desk mutations and no active
   self-development operation is idle) runs last, only on the candidates in
   rank order.
3. If the eligible victims' resident memory (latest RSS observation, else
   the recorded peak) cannot cover the shortfall, refuse at once without
   hibernating anything, naming the shortfall and why each skipped computer
   was skipped.
4. Hibernate victims one at a time through the ordinary hibernate path
   (`hibernateVMForDesktopWithReason`, reason `make_room`), each followed
   by a re-admission on a fresh sample, polled for a bounded settle wait;
   stop at the first admit. At most `VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS`
   (default 3) per start.
5. Record the decision in `admission.recent` (vmctl health):
   `admit_after_make_room` with the hibernated VM ids, or
   `refuse_no_room` with the reason.

The new-computer branch releases `r.mu` before making room, then re-takes
it and re-checks that no other request registered the same computer
meanwhile; if one did, the reservation is released and the resolve starts
over.

### Failure modes (each pinned by a test before the code)

- **M1** a start hibernates a computer that is working (a run, a pending
  desk mutation, a self-development operation): the busy probe must answer
  idle, and an unanswered probe counts as busy.
- **M2** a start hibernates the paper, a public_platform computer, an
  always-on (premium) computer, or a held computer.
- **M3** a start hibernates a computer of a higher class than its own (a
  test start evicting a primary computer).
- **M4** two refused starts both pick the same victim (a double hibernate)
  or both evict for the same shortfall (over-eviction).
- **M5** the victim is the starter itself, or a computer whose own start is
  in flight (booting, or holding a lifecycle claim).
- **M6** make-room evicts when no eligible set could cover the shortfall.
- **M7** make-room hibernates without bound, or a start waits without
  bound for memory to show up.
- **M8** a refusal does not say why nothing was eligible.
- **M9** a race with the idle sweep or pressure reclaim hibernates one
  computer twice or fails the start; a victim the sweep already took is
  skipped and the re-admission re-samples.
- **M10** the new-computer branch holds `r.mu` across a hibernate (every
  lookup stalls for the snapshot write), or mints a second computer for
  the same user after re-taking the lock.
- **M11** the admitted start's reservation is lost or double-counted after
  making room.
- **M12** protected starts change (they are admitted without making room,
  as today), or guest-size mode changes.
- **M13** rollback: `VMCTL_ADMISSION_MAKE_ROOM=off` restores today's
  refusal exactly.
- **M14** restart semantics: make-room runs only inside a start; a vmctl
  restart resumes and hibernates nothing.

Added by review 302 (Devin and DeepSeek on abcf4f577), each pinned by a test
before its code:

- **M15** a victim that became working, held, recently used or unroutable
  between plan time and its own hibernate is still hibernated: the plan is
  re-read from the live registry and the victim probed busy immediately
  before each hibernate.
- **M16** a guest that answers busy at the hibernate fence is killed (the
  kill path) instead of left running; it is now a skip for every caller
  (`docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md`).
- **M17** the victim set is chosen once for the first shortfall, and another
  path's in-flight hibernate counts as this start's room, so a start refuses
  after evicting while eligible victims remain, with false refusal text. Now
  one victim at a time, the shortfall recomputed from a fresh sample after
  each, pending memory only waited for, never counted; a refusal is on a
  fresh sample.
- **M18** a typo or an observation word turns make-room on: the code default
  is off and only `active`, `on`, `enabled`, `true`, `1` enable it.
- **M19** a start superseded after it hibernated victims leaves no record
  (`abandon_make_room` names them).
- **M20** unbounded env overrides (capped at 10 victims and 60 s) and a
  cancelled start that keeps evicting (the start's context stops it and
  reaches the route guard).

### Protected surfaces, evidence, rollback

Protected: vmctl admission, hibernate (VM lifecycle), the idle sweep and
pressure reclaim's shared ranking. Admissible evidence: the unit tests
above for the decision and concurrency; staging (five test computers
started in a row on a host that cannot hold them, no hand stop,
`admission.recent` read from vmctl health) for the claim. Rollback:
`VMCTL_ADMISSION_MAKE_ROOM=off` in `nix/node-b.nix`, or a revert of the
code commit.

### Residuals noticed (not this item)

- Sustained pressure reclaim does not exclude public_platform computers
  (`protectedReclaimReasons` names premium_always_on and critical_protected
  only); the idle sweep does. Make-room excludes all three.
- Protected starts (`admit_protected`) are admitted under the floor without
  making room; the backstop is pressure reclaim.
