# The Long Walk to a Real Self-Development Episode

Written September twenty-ninth, 2026. This report covers one working session — about eleven hours — on the M11 mission: landing the first fully driverless self-development episode on staging. A probe run is in flight as I write; its outcome is noted at the end.

## Where we stand

The substrate repairs are in and they work. The episode pipeline — propose, verify, approve, apply, falsify, restore — has now run end to end on staging twice without manual repair. What remains is a probe that keeps discovering its own bugs on each pass, plus one real product defect it exposed along the way.

## What happened

This session picked up after a clustering assessment found that four wedge defects in one day shared a single root cause: durable obligations whose repair was event-triggered, with several obligation classes having no event source at all. Rather than build a periodic sweep, the mission closed the missing trigger edges into existing reconcile authorities.

Nine edge closures and structural merges landed in a commit chain: cancellation now fails the bound operation at one choke point, a desk reconcile kick fires when an assigned run leaves pending, the verify-fail branch is gated to frozen operations only, delegated spawn resume is serialized under the open mutex, the bind-to-activate crash gap is closed, restart recasts cap at three attempts, the spawn obligation is written atomically inside the open commit, terminal-run death routes through the fate path, and three write-only claim projections are deleted.

Then the probe began teaching us about the probe.

The first run reached "awaiting approval" with a verifier receipt — the first driverless frozen-to-approved transition — and reported itself blocked. The predicate bug turned out to be in the harness, not the computer: a logging function that marked legs was appending to a file without ever setting the flags the predicate read. The episode had run; the scorecard was lying about it.

The second bug was subtler. The probe reads computer events through a replay endpoint that returns the oldest page first. Apply evidence — materialization applied, checkpoint published, route updated — lives at the tail, behind more than a thousand projection-batch records. The probe was reading the beginning of the tape and concluding the end never happened. A tail-window fetch fixed it.

The third failure was real and worth the whole exercise. Candidate-B's reject decision refused at pin verification: the reject path attached the rejection reason as a private payload on an owner-class event, and the pin authority requires the two to match. Both sides of that mismatch were introduced in the same July commit — "freeze disabled audited cutover" — which means the reject branch had never successfully committed an event through the pinned-event path. Nobody had ever run a reject. The probe found it on its first try. The fix pins the reason as owner-class, which it is; the decision record belongs to the owner anyway.

The fourth failure was a race so narrow it took two probe runs to see. When the probe posts an approve decision, the event commits and the post-commit observer fires the materializer drain, which finalizes the same decision the handler is still finalizing. Whichever of the two writes the operation record second loses the race to ErrConflict and the handler returns 409 "state is accepted" — even though the decision the client asked for is exactly the decision that committed. The durable operation record is correct; the HTTP response lies about which path got there first. The probe now treats that 409 as what it is — a finalized decision whose response raced its own recovery — and the underlying rescue wart is documented for a future pass.

## What remains

The probe is running again with the reject fix and the race tolerance in place. The legs it still owes: a clean approve, apply events confirmed at the tail, candidate-B rejected, and the restore leg. If it passes, M11 is done and the goal file closes. If it fails, it will fail at a leg we have not seen before — the pattern this session established is that each probe pass advances exactly one defect deeper, and the defects it has been finding have moved steadily from the harness toward the substrate.

## The one thing worth remembering

The probe is not a test of the system. It is a second system, running the same code paths as a user, and it kept finding that the harness's own measurement was the first thing to break. Three of the four failures this session were the observer lying about the observed. The fourth — the reject pin mismatch — is the kind of bug that lives in production for two months because no user had ever pressed that button. A probe that walks the whole episode is the only instrument that finds it.

If I were holding the phone: the substrate work is sound, the probe is almost honest, and the next run should tell us whether M11 closes or finds its fifth defect. Either answer is information.

## Names and receipts

Commits this session: `067cdd95` through `e886f576` on main. Deployed build on staging at time of writing: `89cd7247` (reject pin fix). Probe script: `scripts/m11_selfdev_episode_probe.mjs`. Problem receipts: `docs/problems/clustering-assessment-engineering-assignment-stalls-2026-09-29.md`, `docs/problems/m11-candidate-reject-pin-mismatch-2026-09-29.md`. Probe evidence: `/tmp/m11_probe_run5.json` through `run8.json`; run 9 in flight.
