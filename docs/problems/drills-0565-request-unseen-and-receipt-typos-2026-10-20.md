# Drills on 0565b9620: engineering never looks at the request, and a retyped receipt ref blocks Complete (2026-10-20)

Staging deploy 0565b9620 carries the Cast request fix (486ffbc47), the
release witness (S6-0) and visual evidence (d7f6ecae3). Two drills ran on
cold-booted test computers from 11:36 UTC and were stopped at 11:58 UTC
once the traces below were read.

## Evidence

**Claude delegate probe** (computer-ad1b46d4…, trajectory 60af8acd-…,
engineering run `run:assignment-d764fafb0802f6f8b51ec526`):

1. Management's Cast objective now defers to the request: "Carry out the
   bounded Claude delegate probe exactly as specified, then verify the
   resulting file contents in your own cell…". The management overlay
   change took effect.
2. Engineering's first cell printed `r.Kind, r.From, r.Objective, r.Body`
   for each record. Body is the spec `{"parent": …}`. It never printed
   `r.Request`, so whether the request was present is not observable from
   the trace. The engineering overlay names the field once, mid-line, in a
   long bullet (rlm_engineering_runtime.yaml line 53).
3. Engineering briefed the delegate with "exactly as specified by the
   authenticated management assignment" and `paths ["."]`. Delegate run
   `d20261020T113744-ecebc039`: done, success, 6 turns, $0.070, no edits;
   it asked for the missing stamp and path.
4. Engineering's `find /workspace/platform -newermt <start>` listed
   `.env*`, `.npmrc`, `.yarnrc*`, lockfiles and `bunfig.toml`, none written
   by the delegate (they look like the capsule's secret masks). A
   time-based change scan is misleading in the capsule.
5. Every `choir.Complete` was refused: `reduce: persist tray-1: executor
   receipt unavailable`. The first cited
   `capsule-go-eval:sha256:0c1cb7ecb4b77cac69606e660abad57bbbbe7014a84a3ca9…`;
   the real ref is `…bbbbe7014a14a84a3ca9dcd761e53105b` (three characters
   dropped). The overlay's terminal-fate retry then looped
   (`engineering_overlay_terminal_fate_required`), and the run stayed
   `running`.

**Visual evidence drill** (computer-7011602c…, trajectory 6adce3d3-…,
engineering run `run:assignment-d9c128f9c2f2c3a87ee8597c`): engineering
printed `r.Kind, r.Body` (the `{"parent": …}` spec), asked management
"Cast lacks page spec, route and check commands", and called Complete
blocked. The tracer was stopped before management answered.

## Classification

- **Request unseen (substrate-adjacent).** The fix put the request in a
  separate field. Desks print Body. A field the desk must know to ask for
  is the same failure as a parent it cannot read, one step later. Third
  related occurrence after P5 #1 and the 10:42 probe.
- **Silent paths in the request view.** `engineeringRequestView` returns
  with neither request nor error when the assignment lookup fails or the
  record is not the opening Cast, so a missing request is invisible.
- **Receipt ref typos (repeat).** Second recorded occurrence after
  `selfdev-go-drill-jobs-0101-2026-10-15.md` A1. The refusal does not
  name the ref, so the desk cannot see its typo and retries the same
  mistake.

## Fix (next commit)

1. The opening Cast's Body, as engineering's view shows it, carries the
   request (or `request_error`) inside the spec JSON, so printing Body
   shows it. The silent paths set `request_error`.
2. An unknown receipt ref is refused naming the ref, and when a receipt
   this executor issued shares a long prefix, the refusal names it ("did
   you mean").

Class: red (engineering inbox view, Complete/Freeze evidence path).
Rollback: revert. Proof: the same two drills on the next deploy.
