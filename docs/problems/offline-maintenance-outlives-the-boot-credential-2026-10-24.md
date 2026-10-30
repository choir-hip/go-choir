# A long offline maintenance outlives the boot credential, and the paper stays down an hour (2026-10-24)

Status 2026-10-30: the credential-lifetime issue is live. The Dolt-store offline compaction framing and exposure section are historical (the Dolt stores are deleted). Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-24, 05:1x UTC. Found by: FP0. Class of this record: green. Class of
the fix: red (the boot path and the credential envelope). Status: recorded; not
fixed. There is no recurrence risk at the next 04:00 timer (see "Exposure").

## What happened

- 04:00:01: the nightly store compaction timer (go-choir-store-compaction, a
  planned refresh with an `OfflineMaintenance` hook) killed the paper
  (vm-1669020a…, critical_protected) and compacted its data image offline.
- The compaction worked: `Squashed:true CommitsBefore:7421 CommitsAfter:2
  BytesBefore:55143180884 BytesAfter:18854731052`, with root
  `7k26ng74rgq1u9aca2gg50gtsgm7hrfv` before and after, in 11 min 27 s. Memory
  peak 10 G, at its MemoryMax. 113.4 G read, 43.1 G written. The 10-20 run on an
  8.5 GB store took 47 s.
- 04:11:28: the VM booted. The guest's runtime opened its store in 431 ms, then
  failed: `autoputer: acquire or recover computer event credential: credential
  envelope: invalid bootstrap lifetime (transient; restarting)`
  (console.log.1). It restarted in a loop. The paper made no model call after
  04:00:03, and the paper liveness probe alerted from 04:34:27 onward.
- 04:41:30: `destructive kill … cause=boot-readiness-failed` once vmctl's
  30-minute readiness window ran out. The VM was marked failed. The compaction
  outcome was `failed` ("guest did not become healthy … within 30m0s").
- 05:02:28: D33's deploy refresh rescued it ("booted onto the current deploy as
  a crash boot (no planned-restart marker)"). Its lease seed took 2183 ms and
  it was ready. The paper was out from 04:00:01 to about 05:03, about an hour.
  Because the second boot was a crash boot, the work open at 04:00 closed as
  interrupted, by the restart rule.

## Cause (verified in source)

- vmctl mints the guest's `ComputerCredentialEnvelope` when it builds the
  realization config (`internal/vmctl/ownership.go`, at
  `issueComputerCredentialEnvelope`, about :1467). It mints it before the
  refresh kills the VM and runs `OfflineMaintenance`
  (`internal/vmmanager/manager.go` :1717-1719).
- The guest's `VerifyBootstrap` (`internal/platform/credential_envelope.go`
  :71-81) refuses an envelope at or past its `ExpiresAt`. A lifetime is capped
  at `maximumCredentialEnvelopeTTL = 15 * time.Minute`.
- Kill, then 11.5 minutes of compaction, then boot: the envelope was too old
  by the time the guest read it. A 47-second compaction never showed this.
- The guest classes the refusal as transient and restarts. Nothing re-mints
  the envelope. So the loop cannot end on its own: an expired envelope stays
  expired.
- A failed critical_protected computer stayed down until the next deploy
  re-armed its retry ("deploy target … re-arms the automatic retry of protected
  computer"). Nothing outside a deploy brought the paper back.

## Exposure

The sweep selects images whose store file is over 32 GiB
(`min_file_bytes 34359738368`). The paper's store is now 18.9 GB and grows
about 6 GB a day (record guest-store-compaction-scheduled-2026-10-20). So it
should not qualify again for about two days. The owner's computer and others
qualify only as platform classes. Any OfflineMaintenance hook longer than the
envelope's remaining life fails the same way.

## What a fix has to do (not decided)

1. Mint the envelope after the offline maintenance, just before the boot, or
   re-mint it when the hook returns. The realization and epoch stay the same.
   The idempotency key `guest-credential:<realization>:<epoch>` replays the
   old envelope, so a re-mint needs a fresh key or a reissue path, and the
   trace must name it.
2. The guest should treat an expired bootstrap envelope as fatal for that
   boot and say so, not as transient. Then vmctl's readiness wait fails in
   minutes, not in 30.
3. A failed critical_protected computer should be retried without waiting
   for a deploy, under the existing restart bounds.
4. The compaction unit's 10 G MemoryMax was reached on a 55 GB store. Check
   whether it would fail on a larger one.

## Heresy delta

discovered: the boot credential is minted before offline maintenance, so a
long hook makes the computer unbootable; an expired envelope loops as
transient; a failed protected computer waits for a deploy. introduced: none.
repaired: none.

## Design of the fix (2026-10-24, FP0 lane fp0-envelope-fix)

Class: red (the boot path and the boot credential). No store schema, no
credential format change, no boot-time resumer.

Two facts from the code refine the record above. The issue handler asks
corpusd for a 10-minute life (`HandleComputerCredentialIssue`,
`event_handlers.go`), not 15: 15 is only the cap `VerifyBootstrap` accepts.
So any hook longer than about 10 minutes fails this way. And a same-key
request after expiry is not replayed: corpusd re-derives the expiry from the
recorded issue time and refuses it (`mintComputerCredentialEnvelope`), so
asking again under `guest-credential:<realization>:<epoch>` cannot heal it.

**Shape.** vmctl still mints the boot envelope where it does today, before
the kill, under the same key; that keeps "fail before the kill" when corpusd
is down. When a refresh carries an `OfflineMaintenance` hook, vmctl also
hands vmmanager a re-mint closure (`PostMaintenanceCredential`, `json:"-"`,
never stored in an instance config, like the hook). vmmanager calls it after
the hook returns and before `bootVM` writes the credential disk, and boots
with what it returns. The closure asks corpusd under
`guest-credential-maintained:<realization>:<epoch>`, so the lifecycle
receipt names the re-mint, the stale envelope is not replayed, and a second
call for the same realization replays the post-hook envelope instead of
issuing another one. The realization ID and epoch stay the same: the epoch
was reserved and persisted before the kill (`ReserveBootEpoch`), the guest
binds the envelope to that realization, and vmctl publishes it after the
boot; a new epoch inside vmmanager would mean reserving under the per-VM
lock and gains nothing, since the realization has not booted yet.

**Which outcomes mint.**

- No hook (every plain refresh, boot, recover): the closure is not set and
  never called. Issuance is exactly as today, one request.
- Hook returns nil, or an ordinary error (the manager boots the image the
  hook left): re-mint, then boot. A failed compaction that ran 11 minutes
  needs a fresh credential as much as a good one.
- Hook returns `ErrOfflineMaintenanceUnbootable`: no boot, so no re-mint.
- Re-mint fails (corpusd down after the hook): log it and boot with the
  pre-hook envelope. That is never worse than today: a short hook still
  boots, a long one fails as it did.

**Failure modes, each a test written before the fix.**

1. A hook that outlasts the envelope's life boots with an unexpired
   envelope (vmctl, a fake corpusd on a test clock that the hook advances
   11.5 minutes, with corpusd's replay and expiry rules).
2. A refresh without a hook makes one issuance under the boot key and
   carries no re-mint closure.
3. Calling the re-mint twice for one realization replays: same key, same
   envelope, one issuance record.
4. An unbootable hook does not call the re-mint; an ordinary hook failure
   does (vmmanager, the maintenance step on its own).
5. A failed re-mint keeps the pre-hook envelope (vmmanager).

**Item 2, taken: an expired envelope is fatal for that boot.**
`VerifyBootstrap` returns a sentinel (`ErrCredentialEnvelopeLifetime`, same
text as today) and the guest's credential step calls `boot.fatalf` for it
instead of `retryf`. An envelope's lifetime is fixed when it is signed and
the guest has no clock sync that could move its clock back, so a restart
cannot fix it; the guest now serves a typed `startup_failed` refusal at
once and vmctl ends the boot in seconds instead of 30 minutes. vmctl keeps
`startup_failed` prompt but not durable (`RecordGuestRefusal`), so the next
start with a fresh envelope is not blocked. Test: the sentinel is what an
expired envelope returns, and the guest's classifier treats it, and only
it, as fatal.

**Residuals (not in this lane).** Item 3: a failed critical_protected
computer waits for a deploy to retry. Item 4: the compaction unit's 10 G
MemoryMax on a larger store. Corpusd's server-side expiry check at exchange
("credential envelope: expired", guest clock behind the host's) still reads
as transient to the guest. The re-mint makes a maintenance refresh issue two
envelopes for one realization; the first expires unconsumed, and consuming
the second advances the revocation epoch past it.
