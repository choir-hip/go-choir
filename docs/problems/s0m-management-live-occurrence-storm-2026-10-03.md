# S0m finding: persistent-Management live-occurrence storm — third live-lock in the reconcile/redrive path

Written 2026-10-03. Mutation class: red (protected surface — persistent
Management reconcile + lifecycle control re-drive).

## Symptom

On guest `ebfd2e98` (staging), after the stranded-bound probe killed research
carrier `d8a17dee`, the freed control `1ed83383` re-entered `pending`/unbound
and stayed there for 12+ minutes (148 probe polls). During the same window the
guest journal shows `persistent Management live occurrence received → bound
run=cf5a70ca → terminal` repeating dozens of times across many trajectories
(`07aece4`, `24693e8`, `91492b4`, `5242ca0`, `3fa254b`, `d6f1b0e`, `d8ccd11`…).
The released research packet never re-bound; the desk re-drive is being
consumed by a Management occurrence churn.

## Root-cause cluster (3 live-locks, one subsystem)

This is the **third** live-lock recorded in the persistent-Management /
lifecycle-control reconcile path:

1. `886e5ce1` — *exhausted-recast live-lock*: a transient error returned by an
   exhausted restart recast caused the dispatcher to re-deliver the reconcile
   occurrence forever, saturating the guest and starving every other desk
   reconcile. Fixed by returning clean so the occurrence is incorporated.
2. `3b0a1ed2` + `b7f59cc9` — *reactivated-resident* and *producer-report
   authority* live-locks: a stranded reactivated resident and an
   `ErrEngineeringAssignmentInvalid` deferred forever, re-driving reconcile.
3. **This** — *live-occurrence storm*: `bindTerminalRunOutcome` fires
   `wakeUpdatedCoagent` for every freed control; the persistent-Management
   branch keeps receiving+binding+terminalizing occurrences for the same
   resident run without ever discharging the obligation that triggered them.

Common shape: a durable obligation is re-delivered by a re-drive that never
reaches the state transition that discharges it, so the re-drive loops. Each
prior fix returned a *terminal* verdict for one trigger class; the storm
recurs on the next unhandled trigger class. This is a substrate defect — the
re-drive loop has no convergence invariant ("each wake must either discharge
the obligation, rebind it, or score it exhausted") — not three independent
bugs.

## Evidence

- Guest journal `go-choir-vmctl` 2026-10-03 01:55–02:20: repeating
  `persistent Management live occurrence received/bound/terminal` for
  `run=cf5a70ca`.
- Trajectory `ffc400c2-d70e-58b3-b00c-c2a0917af6b9` (staging): control
  `1ed83383` `disposition=pending`, `delivered_to_loop_id` cleared, never
  re-bound; work item `00cccb50` (research) still `open`.
- Probe evidence `docs/evidence/s0m-stranded-bound-2026-10-03.json`:
  `carrier_cancelled` (200) → 148× `claim_released_pending` → timeout at
  recover window, `final_update.disposition=pending`.

## Correction

Earlier commit `1e285a3e` attributed the bind failure to a "steering gap —
the cell chose apply over open_researcher." The trace disproves that:
`e9bdff26` turn seq 3 (`texture_turn_committed` 2026-10-03T01:52:30) reason =
"atomically open a research desk to confirm that date"; `work_opened` minted
`research:8764897c`; `control_queued` + `control_delivered` `1ed83383`. The
instruction was not missed — the whole Ask→control→bind→deliver chain worked.
The first probe's bind window simply closed before the (backlog-latent) turn
landed. The residual is the rebind-after-release storm documented here.

## Next boundary

Stop patching the re-drive at the trigger level. Define a convergence
invariant for the lifecycle-control re-drive: every freed/pending control
obligation must, on wake, either (a) bind to a live carrier, (b) discharge
(consumed/replied), or (c) score `delivery_attempts_exhausted` — never loop a
Management occurrence without changing obligation state. A substrate-level
fix owns this: an obligation-state-aware dispatcher, or a bounded re-drive
budget per obligation, not another per-trigger terminal return.
