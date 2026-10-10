# Every capsule spawn fails: the broker socket path is too long on the data disk (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `red` (capsule
spawn, engineering execution). Introduced by c15e90c4 (capsule writable
state moved to the data disk), which this session landed.

## Evidence

Minesweeper demo on staging at 37498792 (test computer
demo-ms-2026-10-10b, vm-8cac78e5…; trajectory 08d1df18…, read through
`/api/trajectories/<id>` into the session scratchpad traces/ms8):

- Management run 5c11cfbf cast engineering at 22:45:47 and parked
  (`awaiting_delegated_cast`) at 22:45:52, as intended by 744bd55b.
- Assignment `delegated-sha256:bbb6665…` was cancelled at 22:46:04 with
  `capsule create parent broker listener: listen unix
  /mnt/persistent/capsule-state/boot-84c9b456-…/capsule-7492efb9-…/root/run/capsule/broker.sock:
  bind: invalid argument`. No engineering run ever started.

## Cause (from source)

A Unix socket path must fit `sun_path`, 108 bytes on Linux; a longer path
fails `bind` with EINVAL. The capsule executor binds two listeners under the
capsule root (`internal/capsule/executor.go`, the broker socket and the
host-side socket). With state on tmpfs the root was short. On the data
disk the broker path is 145 bytes:
`/mnt/persistent/capsule-state/` (30) + `boot-<boot id>/` (42) +
`capsule-<id>/` (45) + `root/run/capsule/broker.sock` (28).

The unit tests use short temporary directories, so they never reach the
limit; no test pins the deployed path length.

## Belief state

Before: c15e90c4 moved capsule state without changing spawn. After: since
that deploy, every engineering capsule spawn on staging fails, so no
delegated cast can run. Remaining error field: whether the egress proxy
socket has the same defect (it is bound at the same depth).

## Fix direction

Bind each listener through a short path that names the same directory:
open the socket's directory and bind `/proc/self/fd/<fd>/<name>`. The
socket file lands in the same place, and the capsule side keeps connecting
at its own short in-root path. A test binds under a directory whose full
path exceeds 108 bytes.

## Second defect seen in the same run: the cancellation never reaches the turn

The cancel report (`assignment-report:cancel-report:sha256:2b53075f…`,
engineering producer_report, kind `blocker`) was delivered to the parked
management run (`delivered_to_loop_id` 5c11cfbf) and is still `pending`.
The run resumed at 22:46:04, but its first provider call carried no new
user message (message roles end in `assistant`, last user text empty).
The model restated its earlier summary, made no cell call, and the turn
completed, so Texture never learned the cast failed.

Cause (from source): `ListLifecycleControlsDeliveredToRunPage`
(`internal/store/lifecycle_control_delivery.go`) authenticates each
producer report by loading `GetLifecycleRun(update.SourceRunID)`. A cast
cancelled at spawn never had a run, so the runtime-written cancel report
has no `source_run_id`; the lookup fails with not-found and the whole page
errors. `pendingCoagentUpdatesForRun` maps not-found to "nothing pending",
so the resumed turn gets no message at all, and one runless report hides
every other packet delivered to that run. This is the shape of
sa-delegated-report-poisons-management-listing-2026-10-06: a valid
delegated report poisoning the listing.

Fix direction: a producer report with no source run is one the runtime
wrote for an assignment that never ran. It still authenticates through its
producer work item (trajectory, assigned agent, engineering authority) and
the delegated parent join back to the consuming run; only the run check is
skipped.
