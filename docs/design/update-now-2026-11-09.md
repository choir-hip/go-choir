# "Updates available": an owner-started planned restart onto the new release (2026-11-09)

Owner, 2026-11-09: "B at C hasn't picked up the platform updates that I see on
my [own] account. And I guess there should be some kind of a pop-up that
appears on a computer while it's running, saying like, restart this computer
to get the updates. And you can click on that if you want to. Updates
available."

Class: red (vmctl deploy refresh, deploy routing, the proxy lifecycle route).
Revision r1 closes Devin's review (SEND_BACK, four blocking findings; see
"Adjudicated review findings").

Naming: **owner update** is the new planned, marker-backed act in this doc.
It is not the existing `refresh` lifecycle action, which goes to
`/internal/vmctl/refresh`, the markerless explicit refresh that boots as a
crash boot (handlers.go:895-899) and closes open work. A builder must wire
the owner update to the deploy-refresh path (`trackedDeployRefresh`), never
to `/internal/vmctl/refresh`.
Rollback: revert; the sweep and ring policy are unchanged, so without the
route computers update as today.

## Why b@c lags

vmctl refreshes a computer only when its guest says it is not busy
(internal/agentcore/planned_restart.go:548 plannedRestartBusy); the retry
sweep asks every minute (internal/vmctl/deploy_refresh_retry.go:468). A
computer that always has a run going (b@c running back-to-back theme rounds)
never answers "not busy", so it stays on its old release until a gap. Only
critical_protected and public_platform computers refresh while busy
(deploy_refresh.go:249).

## Design: reuse the deploy refresh; add an owner door and a visible signal

1. **vmctl owner update.** The deploy-refresh request takes an optional
   `owner_busy: true`, honoured only from the proxy's owner-update route (the
   sweep and deploys keep the class policy, `deployRefreshAllowsBusy`,
   deploy_refresh.go:248). This is a deliberate policy extension: today only
   `critical_protected` and `public_platform` computers refresh while busy;
   explicit owner intent extends that to any computer the owner holds. With
   it, `allowBusy` (deploy_refresh.go:415) is true and the guest is marked
   with `allow_busy=1` (deploy_refresh.go:99). It runs through
   `trackedDeployRefresh` (deploy_refresh_retry.go:452), so the ring gate
   (deploy_refresh.go:455), held, divergent and one-in-flight rules, the
   marker write, the restart and the pending bookkeeping are unchanged. vmctl
   passes the guest's structured refusal (reason code, counts) back.
2. **Target: the ring-intended release, not the current one.** The owner
   path resolves the target as the sweep's enroll does: `guestReleaseFor(
   computerID)` (ring_walk_handler.go:86), the release a boot of this
   computer launches now, never `currentGuestRelease()`
   (deploy_refresh_retry.go:185) and never caller-chosen. For a ring-held
   computer that is the held release, so the target equals what it serves
   and the already-current step below answers "up to date"; for a computer
   whose ring has not opened, `ringGate` (deploy_refresh.go:455,
   ring_walk_handler.go:102) answers 409 "ring waits". Either way an owner
   click never restarts onto the release the computer already serves, and
   never releases a ring that has not opened. `restart.target` and the marker
   record the release actually intended.
3. **Already current: a no-op at the top of the owner path.** Before the
   gate, the marker or any pending bookkeeping, the owner path reads the
   served commit (`servedCommit`, deploy_refresh_retry.go:572, the guest
   /health `build.commit` the sweep uses at :519-524). `deployRefreshWith`
   has no such check (the sweep's is outside it), so this is new work.
   Served == target returns `{outcome: current}` with 200, no marker, no
   restart, no pending entry created, and forgets any stale one. A served
   commit that cannot be read is not "current": the path continues.
4. **Guest refusals stay exactly as they are** (correctness, not switches):
   a self-development apply, a live engineering capsule, a snapshot in
   flight, a candidate awaiting its decision, a hibernate fence. With
   allow_busy, running runs no longer refuse: the marker makes it a planned
   restart and their work parks and resumes at boot through `bootResumesWork`
   and `passivateInterruptedActivations` (class-agnostic; management-task
   runs are intentionally not resumed). `platform_update_resume` is the
   self-development update tail, not the run resume. When sandbox pause
   lands (docs/design/sandbox-pause-resume-2026-11-09.md), the live-capsule
   refusal becomes a pause and resume instead.
5. **Proxy route and registration.** `POST /api/computers/{id}/lifecycle/update`
   beside start, stop, restart, refresh and recover. The `update` action must
   be registered at every site, or the build fails or wedges boot:
   - `parseComputerLifecyclePath` (proxy/computer_lifecycle.go:25), the path
     allowlist;
   - `isOwnerVMLifecycleAction` (platform/lifecycle_control.go:45), or the
     durable intent is rejected and every click 503s;
   - `OwnerVMLifecycleAdvancesEpoch` (lifecycle_control.go:54), since the
     restart boots a new epoch.

   **Host-side audit only.** `update` is NOT added to the
   `PendingLifecycleControls` list (lifecycle_control.go:151), the CAS
   `allowedActions` (platform/computer_events.go:313) or the guest allowlist
   (agentcore/lifecycle_receipts.go:29). A receipt the guest refuses is
   boot-fatal (`boot.fatalf`, autoputer/run.go:895) and would crash-loop the
   computer; `recover` is the precedent. (`refresh` is in the SQL list but not
   the guest list, a latent mismatch not to copy.) A test asserts that an
   update receipt is never returned by `PendingLifecycleControls`.
6. **Authz: `authorizeOwnerAct`** (proxy/owner_acts.go:25), as restore and
   rematerialize use (computer_lifecycle.go:350-353), not the siblings'
   `computer:lifecycle` scope. The act restarts the owner's computer while
   runs are in flight; agent-minted keys can hold `computer:lifecycle` but
   never `owner_acts`, so an agent cannot restart its own computer under
   itself. The owner's browser session passes; `admin` alone does not.
7. **Response contract.** Like the siblings: durable intent, then 201 with the
   lifecycle receipt (computer_lifecycle.go:218), whose body carries
   `outcome`: `refreshed`, `current` (already up to date), or `deferred` with
   `reason` and counts, which the proxy returns as 409 `{outcome: deferred,
   reason, live_capsules, ...}`. Refusals and no-ops write no restart. The
   vmctl client gains a deploy-refresh method that returns the structured
   body on 409 rather than an error. As with restart, a proxy timeout
   (`vmctlOpContext`, 60 s) does not cancel the vmctl act; the frontend treats
   a timeout as "Restarting..." and reads the outcome from the build commit.
8. **Second click.** The same idempotency key dedupes a retry. A different
   key while the first is in flight gets 409 `{outcome: deferred, reason:
   "refresh in flight"}` (`deployRefreshInFlightReason`): one restart; the
   chip shows "Restarting..." and stays disabled.
9. **Signal: host-side, from vmctl's pending tracker.** `update_available:
   {target, since}` is read by the proxy from vmctl, not from the guest. The
   tracker (`deployRefreshTracker.pending`, persisted by `saveLocked`,
   deploy_refresh_retry.go:398) holds exactly the computers a deploy or
   sweep deferred. A vmctl read (user, desktop) returns available when the
   computer has a pending entry, `ringGate` answers "" for its target, and
   the served commit differs from the target; `target` is the ring-intended
   release, `since` is `FirstDeferredAt`. A ring-held computer shows no
   notice; a ring rollback makes `target` an older commit, and the chip
   reads "Update available" with the target shown, not "newer". The signal
   rides the obligations surface (`restart{kind,reason,target}`,
   obligations_surface.go:20-34, owner-authed) or the live channel push, not
   observability polling. The target is the release commit, already exposed
   by `restart.target`, not a secret. The guest-side refresh hold
   (`noteRefreshAsked`, refresh_hold.go:74) is rejected as the source: it is
   in-memory, dies with any restart, and is set only after hibernate-fence
   admission (planned_restart.go:735-749), so a fenced, still-pending
   computer would show no notice, and a post-crash computer none until the
   next sweep ask (about one sweep interval).
10. **Frontend:** a persistent desktop notice (a chip in the prompt bar's
    status area or the top of the desk menu, not a transient toast): "Updates
    available - Restart to update". Click: confirm ("Running work resumes
    after the restart"), POST with a fresh idempotency key, then by outcome:
    `refreshed` shows "Restarting..." until the live channel reconnects and the
    build commit changes (reload the page if the frontend build changed);
    `current` clears the chip ("Already up to date"); `deferred` shows the
    refusal in plain words ("An engineering sandbox is running; it can be
    paused for the restart once sandbox pause lands" / "A self-development
    change is waiting for your decision" / "Waiting for its turn in the
    rollout").

## Failure modes (tests first)

1. Owner click on a computer already on the release (including a ring-held
   one serving its held release): 200 `current`, no marker, no restart, no
   pending entry created.
2. Click while a capsule is live / apply in flight / candidate awaiting:
   409 with that reason; nothing restarts; the notice stays.
3. Click with running runs only: planned restart with the marker; runs park
   and resume at boot (`passivateInterruptedActivations`), not closed as
   interrupted.
4. A non-owner (another user's session, an API key without `owner_acts`,
   including one with `computer:lifecycle`) cannot trigger it.
5. The ring gate holds: a click on a computer whose ring has not opened gets
   409 "ring waits", and one on a ring-held computer is `current`; neither
   restarts it.
6. Two clicks: same key, one receipt; different key while in flight, 409
   "refresh in flight"; one restart either way.
7. The notice appears only when vmctl has a pending entry for this computer
   whose ring is open and whose served commit differs; it disappears after
   the restart, survives a vmctl restart (persisted tracker), and shows for a
   hibernate-fenced computer.
8. An `update` receipt never reaches the guest: `PendingLifecycleControls`
   excludes it and a boot after an update does not hit the reconcile refusal.
9. `update` is parsed, accepted as an owner lifecycle action and advances the
   epoch; the owner route never calls `/internal/vmctl/refresh`.
10. A crash after the marker is consumed boots markerless and closes work (no
    resume loop); the owner route adds no boot-time resumer.

## Adjudicated review findings (Devin, SEND_BACK)

- B1 registration sites (parser, isOwnerVMLifecycleAction, AdvancesEpoch;
  guest-visible receipt is boot-fatal): adopted, item 5, host-side audit only.
- B2 target resolver (`currentGuestRelease` restarts a ring-held computer onto
  its own release): adopted, item 2, `guestReleaseFor`.
- B3 already-current no-op missing from `deployRefreshWith`: adopted, item 3,
  top of the owner path, no pending entry.
- B4 `update_available` in-memory with a hibernate-fence gap: adopted, item 9,
  host-side from the pending tracker; the guest-side gaps are recorded as the
  reason for rejecting that source.
- Clarification: authz gate: adopted, item 6, `authorizeOwnerAct`.
- Clarification: response shape: adopted, item 7, 201 plus receipt.
- Clarification: second click: adopted, item 8.
- Correction: `platform_update_resume` cites the wrong mechanism: adopted,
  item 4. Correction: `noteRefreshAsked` is guest-side: adopted, item 9.
- Policy extension of the busy-refresh class exception to any owned computer:
  adopted, stated in item 1.
- `vmctlOpContext` 60 s timeout does not cancel the act: adopted, item 7.
- "vmctl owner refresh" collides with `/internal/vmctl/refresh`: adopted, the
  Naming paragraph and "owner update" throughout.
- Ring rollback shows an older target: adopted, item 9.
- Observability is heavy for notice polling: adopted, obligations surface or
  live push, item 9.
- Reconciling the `refresh` SQL/guest allowlist mismatch: deferred, out of
  scope; recorded so the update receipt does not copy it.
