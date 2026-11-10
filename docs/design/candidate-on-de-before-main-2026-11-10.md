# A candidate runs on d@e before it is on main (2026-11-10)

Status: design, not built. Mutation class of the build: red (vmctl, the
deploy gate). Rollback of this document: revert.

Asked for by the owner on 2026-11-10 ("The branch build is related to
progressive delivery ... What's the solution?"), following
[the method audit](sdlc-audit-2026-11-10.md), item 3. It builds on
[ring-ordered deploys](ring-ordered-deploys-2026-11-03.md) and changes none
of its rules.

## The problem

Rings were gated on time. The soak floors are 10 minutes for d@e, then 15
minutes, 4 hours, 4 hours, 2 hours and 1 hour (`internal/vmctl/ring_walk.go:86-93`),
so a good change took most of a day to reach every computer. The owner put
every release on expedite. An expedite opens every ring at once, so a bad
change now reaches the paper as fast as a good one. On 2026-11-10 a change
meant to reach d@e first was on the paper within two minutes. The time
floors are now zero (owner, 2026-11-10; amendment in
`ring-ordered-deploys-2026-11-03.md`), so rings advance on health alone and a
release walks d@e first in about 6 to 12 minutes without an expedite.

Two different things were being bought with one mechanism:

| Want | What buys it | Cost |
| --- | --- | --- |
| A change is tried on one safe computer before any other gets it | a proof on d@e | minutes |
| Slow failures (a leak, slow corruption) are caught before the paper | a wait on the clock | hours |

Before launch the first matters and the second does not. So the first moves
to before the merge, gated on a proof, and the second stays in the ring walk,
expedited until the owner says otherwise.

## The flow

1. **Candidate.** A commit that is a descendant of `origin/main` (a lane or a
   train, rebased) is named to the deploy gate. Node B builds its guest image
   only and materialises it as a guest release beside the current one. The
   current-release link does not move, and host services are not switched.
2. **Pin and refresh.** vmctl records a candidate pin: d@e runs that release
   id. d@e takes a planned, marker-backed refresh onto it, with the usual
   refusals.
3. **Proof.** The candidate's tracked proof scripts run against d@e, with the
   ring walk's health guards watching d@e.
4. **Promote.** On a pass, `main` is fast-forwarded to that same commit. The
   main deploy builds the same derivations, which are already in Node B's
   store, switches the host, and walks the rings expedited as today. d@e is
   already on the target and counts as healthy at once. The candidate pin
   clears when the current release equals the pinned one.
5. **Reject.** On a failure, the pin is cleared and d@e takes a planned
   refresh back to the current release. `main` never moved.

## What it costs

Measured on run 42866362750: the prebuild takes about 7 minutes and the
deploy job about 14.

| Path | Today | With a candidate |
| --- | --- | --- |
| Good change, time to every computer | about 15 min | about 8 min for the candidate, the proof, then about 15 min |
| Bad change | on every computer in about 15 min, then a revert and another 15 | never on main; d@e returns in a minute |

So a good change pays about ten minutes more. Trains spread that over
several lanes.

## What exists and what is missing

Exists:
- A guest release is a directory named by the store hash of the guest image,
  and several can coexist (`internal/vmmanager/guest_release.go:3-5`).
- Per-computer release choice goes through a pin interface, which the ring
  walk implements (`internal/vmmanager/release_pin.go:20-29`;
  `bindCurrentRelease`, `guest_release.go:264-290`).
- `POST /internal/vmctl/deploy-refresh` refreshes one computer, marker-backed
  (`internal/vmctl/deploy_refresh.go:174-185`).
- The guest image embeds its build commit, so the same commit gives the same
  store path.

Missing:
1. A gate verb that builds a commit which is a descendant of main and not yet
   on it. Today the gate refuses anything that is not an ancestor
   (`scripts/node-b-ci-gate:19-21`).
2. Materialising that release without moving the current link.
3. A second pin source: a candidate pin naming a release id, accepted only
   for ring 0 computers.
4. The ring walk ignoring a candidate release as a target or as `from`.
5. The deploy receipt left alone by a candidate, so the expedite range and
   the security epoch gate's "deployed base" still describe main.

## Rules

- A candidate pin may name only a ring 0 computer. It is not a switch: it is
  a release d@e runs, as the ring design already lets computers differ.
- One candidate at a time. A second waits or joins the train.
- A candidate that raises the security epoch or carries a one-way migration
  is refused; those are proved on d@e by the existing rule and merged first.
- A candidate pin older than two hours is cleared and d@e is returned, so an
  abandoned candidate cannot hold d@e.
- The crash rule holds: the refresh onto and off a candidate is planned and
  marker-backed. A crash on the candidate closes work, and the guards return
  d@e.

## What it does not cover

- **Host services.** The gateway, vmctl and the proxy are shared by every
  computer. A candidate tests the new guest against the old host. A change to
  a host service is covered only after the promote, as today. The guest ABI
  fields guard the mismatch.
- **Load.** d@e does not carry the paper's traffic. Behaviour that depends on
  load is measured on the paper after landing.
- **Slow failures.** Those are what time soaks are for. They return when the
  owner ends the expedite period.

## Failure list, written before any code

- The candidate's build fails: nothing is pinned.
- The refresh is refused (capsule live, apply running): the candidate waits;
  it does not force.
- d@e crashes on the candidate: guards clear the pin and return it.
- vmctl restarts while a pin is held: the pin must be durable, or be dropped
  and d@e returned at start. Choose durable, beside the walk state.
- `main` moves while the candidate is proved: the promote is no longer a
  fast-forward; the candidate is rebased and proved again.
- The promote's build is not a store hit because an input differed: the
  deploy rebuilds and d@e is refreshed again. Slower, not wrong.
- Retention deletes the candidate release: pinned releases are kept already.
- A candidate release is mistaken for the current one by the walk: test that
  a walk never starts from or toward a candidate id.

## Build order

1. The gate verb and materialisation, with no pin: the release appears and
   nothing runs it.
2. The candidate pin and the refresh of d@e, with reject.
3. The promote path and the store-hit check.
4. `scripts/land --candidate` to drive it.

Each step is proved on d@e before the next. Review: two seats, because a
defect in pinning would be silent.
