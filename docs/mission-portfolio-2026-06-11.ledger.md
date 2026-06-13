# Mission Portfolio Ledger

## 2026-06-12 — Architecture-First Recut

Claim/scope: the portfolio should optimize for durable actor architecture and
old-code deletion before product-facing success. Product surfaces are useful
as falsifiers only after the substrate can carry their meaning. Scope is docs
and mission sequencing; no runtime behavior change.

Move: shift observer from product path to architecture spine. Rewrote the
portfolio Parallax State so M1-M4 are the core cutover spine, M5 is a
post-spine falsifier, M6/M8 are promotion substrate, and M7 is review UI on
top of real promotion/rollback.

Expected ΔV: 0 for implementation, +observer evidence. The portfolio should
not claim architectural descent from a docs recut, but future passes should
avoid descent-free product detours.

Actual ΔV: 0. Current portfolio V is 7: M2, M3, M4, M5, M6, M8, and M7 remain
unsettled. M9 and M1 stay done; M5 remains deferred until after M2-M4.

Receipt:
- Updated `docs/mission-portfolio-2026-06-11.md` with an
  architecture-first revision section.
- Updated the recommended order to M9 -> M1 -> M2 -> M3 -> M4 -> M5 -> M6
  -> M8 -> M7, with M10/M11/M12 side tracks only when they do not distract
  from the spine.
- Marked mission kinds: spine, falsifier, promotion substrate, review
  surface, side track.
- Reframed M4 around removing residual RunContinuation record/API/event
  surfaces because M1a already deleted the synthesis decision layer.

Open edge: execute M2 next and prove old messaging mechanisms are deleted or
carried only as named temporary shims inside one landing batch.
