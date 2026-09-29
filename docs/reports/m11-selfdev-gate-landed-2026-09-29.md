# The Self-Development Gate Is Closed — and the Probe Was the Instrument That Mattered

Written September twenty-ninth, 2026, replacing the letter I sent earlier today. The earlier letter covered the work mid-flight with a probe run in progress; this one carries the outcome. The mission — M11, the self-development gate — is landed and the whole rectification spine is settled.

## Where we stand

The full six-leg episode ran on staging with no manual repair. A desk authored a real candidate, an operation froze, reached awaiting-approval, was approved under qualified consensus, applied, and pinned-restore returned to the pre-episode head. A second candidate was started and rejected — the falsification leg — and the falsified record was visible in the live Texture document the owner reads. The spine that began with carrier integrity and ran through desk cells, supervision, scores, harness-skip, push-restore, and frozen vocabulary is complete. Nine substrate repairs and one latent defect fix made it hold.

## What happened

This session picked up after a clustering assessment found that four wedge defects in one day shared a single root cause: durable obligations whose repair was event-triggered, with several obligation classes having no event source at all. Rather than build a periodic sweep, the mission closed the missing trigger edges into existing reconcile authorities — nine edge closures and structural merges, including atomic spawn obligations inside the open commit and fate-path death routing.

Then the probe began teaching us about the probe. Four runs, each finding one thing:

The first run reached awaiting-approval with a verifier receipt — the first driverless frozen-to-approved transition — and reported itself blocked. The scorecard was lying: the logging function marked legs without setting them.

The second bug was in the event-tape read: the replay endpoint returns the oldest page first, so apply evidence a thousand projection-batches deep at the tail looked missing. Tail-window fetch fixed it.

The third failure was the session's real find. Candidate-B's reject decision refused at pin verification: the reject path pinned the reason as private on an owner-class event. Both sides of the mismatch landed in the same July commit — the reject branch had never successfully committed an event, because nobody had ever run a reject. The probe found it on its first try.

The fourth was a genuine race: the post-commit drain can finalize the decision event before the handler's own state transition, returning 409 for a decision that committed. Correct end-state, wrong response — documented as a residual wart.

And then run nine passed. Satisfied, exit zero, all fourteen legs.

## What the episode proved about the computer

The desk authored, verified, froze, and proposed a change without any external driver. The materializer reconciler — the derivable-continuation path that resumed an op after a decision — is the same machinery the clustering assessment just repaired; it carried the episode through apply, checkpoint publish, and route update unattended. The falsification leg confirmed the supervision surface works both directions: approved effects materialize, rejected ones are scored and legible.

## What remains — the honest tail

One informational leg did not fire: commitment materiality, the flag that would show a commitment's world-model weight in the rendered doc. The falsified record was visible — the required evidence — but the materiality projection on the doc needs one more pass to say whether the flag is dead weight or a real miss. And the drain-race wart means a sufficiently fast client sees a 409 for a decision that committed; annoying, not wrong.

The frontier now is M9b/M10 on the world-wire stack, plus the Jev scorer — already designed in two convergent panels and waiting only on the typed-prediction schema upgrade that makes precommitments scoreable. The next mission choice is the owner's, but the instrument that proved this station is the same one that will prove the next: a probe that walks the whole episode and reports what it saw, not what it expected.

## Names and receipts

Deployed staging build for the landed episode: `89cd7247` (reject pin fix) plus probe/script commits through `e886f576`. Probe evidence: `docs/evidence/m11-probe-run9-satisfied-2026-09-29.json`. Problem receipts: `docs/problems/clustering-assessment-engineering-assignment-stalls-2026-09-29.md`, `docs/problems/m11-candidate-reject-pin-mismatch-2026-09-29.md`, `docs/problems/m11-probe-oracle-route-2026-09-29.md`. Records: goal `choir-selfdev-gate-2026-09-27` now.status=landed; spine meta-goal `choir-rectification-spine-2026-09-25` settled — all stations complete; commit `a446559e`. Primary op `selfdev-fab16b4e28c535ec06c6615537f7b4f9` on `computer-787ab6718357a526c101bb5ae29014e6`; falsified candidate `selfdev-0652c3b3aaef95610f3e42bb7b992f22`; episode document `42c8b813-cdd2-53af-bdb3-4df12e611d8f`.
