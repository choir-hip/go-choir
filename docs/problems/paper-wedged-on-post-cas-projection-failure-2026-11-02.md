# The paper wedged for two hours on a post-CAS projection failure (2026-11-02)

Status: mitigated by a restart at 08:23Z; cause unknown; open.

## What happened

- The paper (computer-1355b83b…, vm-1669020a…) made its last model call at
  06:20:02Z. From then on, every event append was refused with
  `computer event projection repair required`
  (`internal/computerevent/appender.go`: `sameHead(platformHead, embeddedHead)`
  false). The platform head had reached 5819266 (corpusd compaction
  `through=5819266` at 06:20:06Z), but the paper's embedded head stayed at
  5819265 (`/health` event_head.sequence).
- The effects:
  - the management desk's inbox wakes dead-lettered;
  - file root citations were deferred;
  - the source schedule sweep failed 4 times in a row (`re-arm rss:hn_best: …
    append projection batch: computer event projection repair required`) and
    backed off to hourly.
- `/health` stayed `ready` the whole time. The paper liveness probe alerted
  from 06:54Z ("no inference call for 30 min"), but nothing acts on that alert.
- At 08:23Z an operator refresh (`POST /internal/vmctl/refresh`, markerless,
  so it counts as a crash and work is interrupted_by_restart) rebooted the
  paper. The boot replay applied event 5819266 cleanly in 55 ms (`replay
  applied 1 events through seq=5819266 … finalize 11ms`). By 08:24Z the head
  was 5819551.

## What this tells us

- The failure was transient, not deterministic: the same event replays cleanly
  through the same code (81f09e677) and the same split schema. A schema or
  constraint fault in the new header/body tables would have failed the replay
  too, so the split is not implicated by this evidence. It is not ruled out
  either: the original error text is lost.
- The original `finalize embedded projection: <cause>` line is gone. The
  host keeps only about 500 KB of the guest console
  (`/var/lib/go-choir/vm-state/<vm>/console.log`), which reached back only to
  07:01Z when it was read at 08:13Z. That limits what can be observed.
- One candidate is the guest memory ceiling. At 06:20:14Z vmctl inflated the
  balloon to target 1519 MiB with used 3079 MiB, reclaiming 1510 MiB; it ran
  every minute at about that scale through the window. This is a hypothesis
  with no trace confirming it.

## Substrate gap

A post-CAS finalize failure wedges a running computer until the next boot.
The only repair is the boot replay. The appender has the replay path
(`Reconstruct`/`RecoverPrepared`), but nothing in a running computer invokes
it when `ErrNeedsProjectionRepair` first fires. `/health` does not report the
wedge, so the deploy and liveness tooling see a ready computer.

## Next probes and candidate fixes (not yet done)

1. Keep the cause. Log the classified finalize error to a durable place,
   such as a store_meta or a host-visible counter, or raise the console
   retention.
2. Surface the wedge in `/health`, for example as `projection_repair_required`
   with local and platform heads, so the probe and vmctl can see it.
3. Self-repair in place: when the embedded head trails the platform head
   while running, run the replay path for the missing tail under the store
   lock, the same code the boot runs. Fall back to a restart only if that
   fails.
4. Correlate the balloon target with SQLite errors on d@e. This is an
   adversarial drill, and it runs on d@e only.
