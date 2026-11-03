# A build refused for host memory cancels its assignment (problem 24, 2026-11-03)

Status: open. Mutation class of the fix: red (engineering assignment
lifecycle: a new waiting path for an open request, its bound and its restart
semantics).

## Evidence

- On the deepseek test arm, a Slides round assignment opened at 14:39:44Z
  and was cancelled in the same second with the reason "host memory refused
  the build capsule: host memory available 10216 MiB; 2161 MiB reserved for
  starts in flight and live builds; this build needs 2810 MiB above the
  4779 MiB floor and 1024 MiB headroom".
- Two other arms' build capsules were live on the host at the time.

## What happened

The spawn saga (`spawnBindActivateAssignment`,
`internal/agentcore/engineering_assignment_runtime.go`) treats every spawn
error as fatal and cancels the durable open (`cancelOpen`). A build-class
spawn the host cannot cover returns `capsule.HostMemoryRefusal` from
`Executor.Spawn` (S4, `internal/autoputer/capsule_host_memory.go`), so it
too cancels the assignment. The design said so on purpose
(docs/design/capsule-heavy-work-scheduling-2026-11-03.md, §Refusal).

## Why it is a problem

The refusal is transient: it clears when another computer's build ends.
On a busy host every refusal needs a person to re-Cast, and
self-development is meant to run on its own. The job level already waits
(job state `waiting`, bounded at 30 minutes); the spawn level does not.

## Existing mechanism

The engineering admission queue (`internal/agentcore/engineering_admission.go`)
already keeps a request open and unbound with a retryable refusal, re-arms
its spawn wake with backoff, expires it at the assignment deadline, closes it
as interrupted at a crash boot, and resumes it once at a planned boot. A host
memory refusal fits that path; it needs its own 30-minute bound and a visible
"waiting for host memory" record.
