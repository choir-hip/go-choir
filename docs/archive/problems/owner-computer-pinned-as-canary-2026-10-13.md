# The owner's computer is pinned as a canary and takes no deploy

Found 2026-10-13 about 14:55 UTC, after the owner asked whether their
account was being updated (it shows the deleted Universal Wire app and no
Autopaper content). Read-only checks on Node B:

- vmctl list: computer-03335285… is active on VM
  candidate-fleet-e15cb89f25d963c220319b7b (epoch 1189), warmness
  premium_always_on, divergence_status canary, construction_version
  code:sha256:499bee7b….
- Its guest /health reports build 86770ae7, built 2026-10-10 16:16 UTC.
  The current guest image is 761d94c5.
- vmctl's journal shows every deploy today (09:38, 10:30, 10:47, 11:24,
  12:12, 13:39, 14:31, 14:51) re-attaching the same VM process; none
  updated it.

Cause: a self-development candidate fleet realized the owner's computer
as a constructed computer version on 2026-10-10. vmctl classifies a
constructed version with no explicit status as "canary"
(internal/vmctl/handlers.go:910-915), and a canary is "pinned ... never
touched by automatic refresh" (internal/vmctl/ownership.go:143). So the
owner's computer has taken none of C0, C1 or C2. The metamission records
"the owner's computer takes every cut (accepted risk, §13.1)"; that has
not been true since 2026-10-10.

Open decision (the owner's computer, a red surface): move it back to
tracking the platform (vmctl's apply-platform-follow route, or a planned
restart onto the current image), which closes or resumes its open work
under the restart rule; and decide whether a constructed version should
be pinned by default at all.

## Corrections (2026-10-13 15:10 UTC, from docs/reports/owner-computer-unpin-plan-2026-10-13.md)

- The computer has been a constructed computer version since its route slot
  was committed on 2026-07-17, not since 2026-10-10; its 2026-10-10 boot
  was a manual owner-approved stop and resume.
- The "canary" label is display only (derived in handlers.go:908-915; no
  code reads divergence_status since a47122de). What keeps the computer
  off the platform: deploys restart no active computer unless CI is
  dispatched with refresh_active_computers (ci.yml ~852); that dispatch's
  selection skips snapshot_kind constructed-computer-version; and
  premium_always_on is never idle-stopped. A one-off refresh does not
  change any of these, so the computer falls behind again at the next
  deploy until CI's selection changes.
- The refresh onto the current image is a crash boot by design: only the
  guest writes a planned-restart marker, and D7's work-state epoch treats a
  jump from 86770ae7 as a crash. Parked runs (295) and open rounds close as
  interrupted by a restart. The owner confirmed this before the refresh.

## Repair (2026-10-13, red; landed locally, not yet staging-proven)

Deploys now refresh active computers through a host-requested planned
restart:

- The guest serves `POST /internal/runtime/planned-restart {"target"}`
  (host-peer only). It writes the marker (reason `platform update`, its own
  work-state epoch) and answers 200 once the marker is durable. It answers
  409 `outcome busy` while runs are running or a self-development operation
  is mid-transaction. `DELETE` clears the marker.
- vmctl serves `/internal/vmctl/deploy-refresh`. GET returns the selection:
  every active interactive computer, constructed and `premium_always_on`
  included, with held ones skipped. POST asks the guest for the marker and
  then refreshes. Busy or unreachable guests get 409 `deferred`. A guest
  built before the endpoint (404) is refreshed only if its `/health` reports
  no work, and that boot is a crash boot. A failed refresh clears the marker.
- CI uses this path on every deploy that sets `deploy_active_vm_refresh`.
  Deferred computers are listed in the activation receipt
  (`active_computers.deferred`) and retried at the next such deploy. The
  manual input `refresh_active_computers=true` also force-refreshes deferred
  computers through `/internal/vmctl/refresh`, which is still a crash boot.
- D7 is unchanged. A marker from another work-state epoch is still a crash
  boot, so a deploy that bumps the epoch closes work instead of resuming it.

Residuals:

- A deferred computer waits for the next deploy that changes the guest. No
  idle sweep retries it.
- A run admitted between the guest's busy check and the kill is parked and
  resumed once by the planned boot, not refused.
- The first deploy carrying this change reaches guests that predate the
  endpoint, so those idle computers take a crash boot one last time.

## Decision recorded (2026-10-13)

bcc556de reverses the 627e7420 (2026-07-17) rule that a global deploy never
refreshes a constructed computer version. Authority: the owner's request
that their computer stay current ("Is my owner account not getting
updated?", then "Update it now"). Divergent and held computers are still
skipped; busy computers are deferred, never killed. The first deploy
carrying the guest endpoint is a crash boot for idle computers (no running
guest has the endpoint yet); from the next deploy on, a deploy refresh is a
planned restart that resumes work once.
