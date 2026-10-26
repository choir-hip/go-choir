# A guest that answers "busy" to a hibernate is killed, not spared (2026-10-26)

Status: documented; fix not built. Found by review 302 (both seats, Devin and
DeepSeek) of density D1 (lane/density, abcf4f577). Class of the fix: red (VM
lifecycle), review tier 2.

## Problem

`captureSnapshot` (`internal/vmmanager/snapshot_lifecycle.go:189-198`) fences
the guest with `guestHibernatePrepare`. A busy guest answers 409 and reopens
admission itself; the fence returns reason `guest_busy`
(`snapshot.go:91`). `hibernateVMSnapshot` treats every non-empty reason the
same way: "snapshot hibernate ... not possible; using the kill path"
(`snapshot_lifecycle.go:86-107`). So the one answer that means "I am working,
leave me running" ends in `hibernateVMKill`, a cold boot at the next wake, which
is a crash boot that closes the guest's work as interrupted.

The guest's refusal is correct and the host's handling inverts it. The restart
rule ("a computer stays on while it is working") is violated whenever this
fires.

## Reach

- The idle sweep probes `/health` busy and then hibernates, so the window is one
  probe round trip; Node B's vmctl journal shows no kill-path hibernate in the
  last 7 days (26 hibernate lines, none "using the kill path"). Latent, not
  observed.
- Density D1 (make-room) probes all victims at plan time and hibernates them up
  to 40 to 50 seconds later; that widens the window enough to matter (review
  302 F1).
- The capsule blindness of the busy probe
  (idle-hibernate-blind-to-live-capsules-2026-10-26.md) is a separate gap: there
  the guest does not answer busy at all.

## Fix shape

1. `guest_busy` from the fence aborts the hibernate and leaves the VM running,
   returning a distinct error that every caller (idle sweep, pressure reclaim,
   make-room) counts as a skip, never a kill. Other reasons (disk budget, a
   failed pause or snapshot) keep today's kill path.
2. Make-room re-reads ownership and re-probes busy immediately before each
   victim's hibernate (D1's own fix, review 302).
3. Test: a guest that answers 409 at prepare stays running and healthy, and the
   caller records a skip.
