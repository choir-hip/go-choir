# A request bound during a refresh boot pins the stack on its old base (problem 30)

Mutation class: green (this doc); the fix is red (source projection and the
engineering bind) and lands separately.

A recurrence of problem 28 after its fix (043b78806, "an assignment takes its
base at bind"). Same symptom, a different cause.

## Evidence

Pool arm, computer-4d38420cf85ac490a36baa0ef5d7a67a; operation
selfdev-94b5523702fb0d8b311cadbda31fb779; assignment-19dc12564c1352b4bd59eb00;
trajectory eb81e65d-0495-5aa7-a73b-12b2a94af29a. The apply was refused:

    updater refused apply: updater: base_moved_since_freeze: release base
    commit 70160304a75fd6338945f8db2c05ca3e785f10b9 does not match booted base
    commit 3c0af9920fa296db23b7f78f8ee5908c3492028b

Read with the pool key through `choir self-dev show` and
`GET /api/runtime/observability` (recent_log, effective_release). The guest
log, 2026-11-03:

    21:45:06 engineering admission wake ... assignment-19dc1256... (the planned-boot resume)
    21:45:07 source projection (boot): stack_base_moved base=70160304a75f head=35b5cba86650
             booted=3c0af9920fa2 op=selfdev-c2624f40... the stack is based on 70160304a...
             but this computer booted 3c0af9920...; that commit is not in the guest
             source repository, so the stack cannot be shown to be upstreamed
    21:45:07 runtime: engineering assignment assignment-19dc1256... binds on the source the
             computer projects now: capsule-source-git:35b5cba866506894585621f29dd9afbbbbfa537a:...
             (its Cast read capsule-source-git:6210bd8581a81da44f27fb3b9be1a2dca272eb69:...)
    21:45:17 saga phase=spawn took=9.644s; phase=bind; phase=activate
    21:45:22 source base fetch (attempt 1): fetched commit=3c0af9920fa2 in 4.858s
    21:45:22 source projection (base_fetch): stack_rematerialized base=3c0af9920fa2
             head=1c46943d0730 ... grafted byte-identically onto the booted base 3c0af9920...

The bound assignment's binding (trajectory snapshot) names
`capsule-source-git:35b5cba866506894585621f29dd9afbbbbfa537a`. The source
projection now reports `stack_rematerialized`, HEAD 1c46943d on 3c0af9920,
served base 70160304a (the applied stack, selfdev-c2624f40, frozen while
70160304a was booted).

## Cause (confirmed from the trace and the code)

1. The stack's top operation was applied on base 70160304a. Its projection
   commit 35b5cba carries `Choir-Stack-Base: 70160304a`.
2. vmctl's planned refresh booted 3c0af9920. Its objects were not yet in the
   guest repository: the boot's base fetch waits a random jitter of up to
   20 s first (source_base_fetch.go). With the booted base missing,
   `reconcileStack` cannot graft and reports `stack_base_moved`, leaving HEAD
   at 35b5cba.
3. The planned-boot resume of the queued request ran in that window.
   Problem 28's bind (`sourceAtBind`) reconciles, then reads HEAD, but does
   not look at the reconcile's state. It bound on 35b5cba. The Cast gate
   refuses this same state (`Result.Refusal`); the bind had no gate.
4. Fifteen seconds later the fetch landed and the stack was grafted onto
   3c0af9920 (HEAD 1c46943d), but the capsule was already pinned to 35b5cba.
5. Freeze (`composeFreezePatch` → `sourceproj.Cumulative`) composes the
   patch from the pinned commit's stack base: 70160304a. The updater compares
   that with the booted base and refuses. That came about 40 minutes later.

The hypotheses in the brief, checked against the evidence:
- "the projection grafts the candidate onto the stack's base": partly. The
  graft is right once the base is fetched, but the bind ran before it.
- "base-at-bind reads a cached base": no. It read HEAD live, after the
  reconcile, and HEAD was the ungrafted projection commit.
- "the Freeze base comes from the stack": yes, by design (cumulative patch),
  and correct for a grafted HEAD. The stack base was wrong only because the
  pin was.

The Cast's own source, 6210bd85, would have failed too: it was read while
5ee5f39dd was booted.

## Belief state and remaining error

- The bind needs the same source gate as the Cast. A state that settles by
  waiting (booted base not fetched yet, apply pending) should leave the
  request waiting under admission's retry. A state that will not settle
  (`stack_base_moved` with the base present, a fetch that gave up,
  unrebuildable) should end the request at bind with the stable reason.
- Defence in depth: the source a request binds on must have the booted base
  as its stack base (or be the booted base itself). That is the invariant
  the updater checks at apply.
- Not addressed: a frozen candidate whose pin is on the old base cannot be
  re-based at apply. No rebase-at-apply exists. The way back is a new round
  on the same trajectory, which offers the failed candidate's patch as
  carried work, checked against the new base.
- Residual: the planned-boot resume fires about 1 s after boot, which is
  always inside the fetch's jitter window. With the fix, a stacked computer's
  first bind after a refresh waits one or two admission retries (10 s, 20 s).
