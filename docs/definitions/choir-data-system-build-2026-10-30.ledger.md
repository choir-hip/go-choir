# Ledger: choir-data-system-build-2026-10-30

Append-only. One terse entry per pass. Never edit earlier entries.

- 2026-10-30 ~06:15 UTC (FP0). Goal authored on the owner's word ("start
  building this ... iron out the kinks first"). The selection goal was
  superseded. Frame review round 1 at 3f4dafb7e:
  - Devin: accept_with_notes. Five paragraph-scale fixes, F1 to F5: the PSA
    proof bound, replay semantics against the §12 stake, a closed effect
    set, outbox backpressure, and fence before stop.
  - Sonnet: send_back. Seven blocking findings, B1 to B7:
    - B1, custody and audit overclaim; the DEK sits on disk.
    - B2, what the host reads; the class is writer-declared.
    - B3, PSA cadence, signing and per-object keys.
    - B4, the effect set and host-mediator barrier; file-sync loss.
    - B5, refusal, writer epoch, fsync and receipt durability; semantic
      kinds stay synchronous.
    - B6, class-pure events, chain integrity, restore composition and
      release.
    - B7, deletion safety: pending uploads, refcount, quarantine.
  - DeepSeek: still running.

  Output is in .agentic-consensus/frame-r1/out/. The overlap is high, and
  the next move is adjudication plus frame revision 2.
