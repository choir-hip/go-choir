# "Updates available": an owner-started planned restart onto the new release (2026-11-09)

Owner, 2026-11-09: "B at C hasn't picked up the platform updates that I see on
my [own] account. And I guess there should be some kind of a pop-up that
appears on a computer while it's running, saying like, restart this computer
to get the updates. And you can click on that if you want to. Updates
available."

Class: red (vmctl deploy refresh, deploy routing, the proxy lifecycle route).
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

1. **vmctl:** the deploy-refresh request takes an optional `owner_busy: true`
   (a request field honoured only from the proxy's owner route; the sweep and
   deploys keep the class policy). With it, the refresh marks the guest with
   `allow_busy=1` (the existing query parameter). The target is vmctl's
   current release (currentGuestRelease), never caller-chosen. It runs through
   trackedDeployRefresh, so the ring gate, held, divergent and one-in-flight
   rules, the marker write, the restart and the pending bookkeeping are
   unchanged. vmctl passes the guest's structured refusal (reason code,
   counts) back instead of only its text.
2. **Guest refusals stay exactly as they are** (they are correctness, not
   switches): a self-development apply, a live engineering capsule, a
   snapshot in flight, a candidate awaiting its decision, a hibernate fence.
   With allow_busy, running runs no longer refuse: the marker makes it a
   planned restart and their work resumes at boot
   (CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)").
3. **Proxy:** `POST /api/computers/{id}/lifecycle/update` beside start, stop,
   restart and refresh (internal/proxy/computer_lifecycle.go), the owner's
   session, an idempotency key and the lifecycle receipt; it calls the vmctl
   owner refresh. 200 `{outcome: refreshed}` or 409 `{outcome: deferred,
   reason, live_capsules, ...}`.
4. **Signal:** the guest exposes `update_available: {target, since}` on
   `/api/runtime/observability` (and the obligations surface) from the
   refresh-hold state vmctl already sets when it asks and is refused
   (noteRefreshAsked, refresh_hold.go). The ring gate decides whether vmctl
   asks at all, so a computer whose ring has not opened shows no notice.
5. **Frontend:** a persistent desktop notice (a chip in the prompt bar's
   status area or the top of the desk menu, not a transient toast):
   "Updates available — Restart to update". Click: confirm ("Running work
   resumes after the restart"), POST, then either "Restarting…" until the live
   channel reconnects and the build commit changes (then reload the page if
   the frontend build changed), or the refusal in plain words ("An
   engineering sandbox is running; it can be paused for the restart once
   sandbox pause lands" / "A self-development change is waiting for your
   decision").

## Follow-on: pause a live sandbox

A live engineering capsule is the commonest refusal. The owner asked that a
sandbox can be paused and resumed safely; that is its own design
(investigation running), after which "Restart to update" pauses live
capsules instead of refusing.

## Failure modes (tests first)

1. Owner click on a computer that is already on the release: 200 no-op
   ("up to date"), no restart.
2. Click while a capsule is live / apply in flight / candidate awaiting:
   409 with that reason; nothing restarts; the notice stays.
3. Click with running runs only: planned restart with the marker; runs
   resume at boot (platform_update_resume), not closed as interrupted.
4. A non-owner (another user's session, an API key without the owner
   scope) cannot trigger it.
5. The ring gate holds: a click cannot release a ring that has not opened.
6. Two clicks: one restart (idempotency key, one-in-flight).
7. The notice appears only when vmctl has a pending refresh for this
   computer, and disappears after the restart.
