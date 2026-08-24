# Recovery guest OOM crash-loop from startup Dolt GC; B11 deferral gap — 2026-08-24

## Evidence (staging, node-b)

Real recovery boot #1 (02:36:07Z, epoch 710): replay resumed from the faithful
workspace (committed 83,122) and advanced ~82 ev/s to seq 99,765 / committed
99,622 (last workspace write 02:46:32Z — `noms/vvvv…` 5.34 GiB + `journal.idx`
109 MiB), then went SILENT: no writes, no errors, no fatal, health still 503
`replaying`; vmctl's stall gate failed the start at 02:48:33Z ("no sequence
advance for 2m0s, seq=99765") and the manager killed the VM. Platform serves
the exact stalled page instantly (99,701..100,000, 300 events, 1.25 MiB, 3s —
all `projection_batch_recorded`), so the platform side is healthy.

Recovery boot #2 (epoch 711): the autoputer crashed in a DETERMINISTIC KERNEL
OOM crash-loop:

```
store: persistent disk high-water warning: used=7 GiB total=31 GiB
avail=24567 MiB (8 GiB default cap); running dolt gc (used crossed 7 GiB milestone)
Out of memory: Killed process 1908 (autoputer) total-vm:2725656kB, anon-rss:1095260kB ...
Out of memory: Killed process 1947 ... (repeat every ~20s: 1908, 1947, 1987, 2025, 2065, 2102, 2144, 2184)
```

The guest is 4096 MiB; the Dolt chunk store is 5.6 GiB; the startup milestone
GC (`store.MaybeRunDoltGC`, run.go:132) runs BEFORE the tape replay and its
memory demand (~2.5 GiB VM from anon-rss 1.09 GiB at kill) exceeds the guest
budget → OOM kill → systemd `Restart=on-failure` → GC again → OOM again.

## Root cause: B11 is not fully implemented

B11 (owner decision, definition): **Dolt GC OFF during recovery**. The
deployed code defers the PERIODIC GC to post-replay (`startPeriodicDoltGC`
after the reconstruct), but `MaybeRunDoltGC` still runs at STARTUP (run.go
`startup phase=dolt-maintenance`) — before the replay — and it is memory-
unbounded against a 5.6 GiB chunk store in a 4 GiB guest. The disk crossing
the 7 GiB milestone happens exactly when the workspace accumulates the first
recovered batch (5.6→7 GiB), which is WHY boot #2's startup check fires after
boot #1 made progress.

The boot #1 silent stall at ~100k is NOT yet fully evidenced (guest console
had no lines 02:43-02:48; the checkpoint commit at the 5.6→7 GiB crossing is
the prime candidate — same substrate family: Dolt maintenance cost against
the guest's memory/IO budget). Working hypothesis: the startup/checkpoint
Dolt maintenance is the common cause; the GC-deferral fix will be tested
against the same 100k stretch and will either clear the stall or isolate a
separate checkpoint-cost problem.

## Protected surfaces / classification

- Red: guest runtime start path on the recovery boundary; Dolt store
  maintenance; the recovery replay substrate.
- No platform tape impact: head remains 132,539/acc54c39…. The workspace is
  durable at committed 99,622; every reopen was `fresh=false` (store
  integrity survived the mid-GC kills).
- Quarantine preserved: data.img untouched; quarantine images untouched.

## Fix shape

Defer the startup milestone GC to AFTER the replay completes — i.e., the
guest (computer-event-authority) path runs NO Dolt GC before the tape head is
reached; the host/local-dev path (no credential file → no replay) keeps the
startup maintenance. This is the literal B11 contract.

## Not a regression

- Pre-dates this mission's recovery work? No — the OOM loop first appeared
  with the rebuilt image + the recovered workspace crossing the 7 GiB
  milestone; the milestone GC is the B11-area code but the memory profile on
  the 4 GiB guest is a NEW discovery. Counted as discovered. Not repaired
  until the deferral + a successful recovery boot past the 100k stretch.
