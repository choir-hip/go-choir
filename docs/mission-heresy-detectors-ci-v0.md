# Mission - Heresy Detectors CI - v0

Status: successor paradoc stub.

Source: `docs/heresy-detectors.md` and `docs/choir-doctrine.md`.

## Parallax State

status: open_handoff

mission conjecture: if the doctrine heresy detectors become executable with
typed allowlists and fail-on-unaccepted-increase semantics, then future missions
will preserve truth-from-facts and deletion pressure instead of self-scoring
around detector evidence.

deeper goal (G): make heresy accounting durable and reviewable without turning
new discoveries into false regressions or false repairs.

witness/spec (A/S): structured detector manifest, baseline ledger generation,
allowlist classes, and a CI or local docs check that fails only on unaccepted
introduced heresy.

invariants / qualities / domain ramp (I/Q/D): counts are evidence, not ontology;
historical evidence and doctrine detector text must be allowlisted; discovery,
introduction, and repair remain separate; do not block agents from naming new
heresies.

variant (ranking function) V: detector families without structured manifest +
baseline counts without classification + missing fail-on-increase check +
missing generated heresy ledger.

authority / bounds: yellow process/test change; no runtime behavior change.

mutation class / protected surfaces: yellow; protected surfaces include doctrine
reward function and CI/process gates.

evidence packet: generated baseline, sample accepted increase, sample rejected
increase, docs/process check result, rollback path, heresy delta.

heresy delta: should repair H020/H021 enforcement weakness without introducing a
fake-clean-story incentive.

next move: convert `docs/heresy-detectors.md` into machine-readable manifest and
write the smallest script that reports count deltas without failing.

ledger file: `docs/mission-heresy-detectors-ci-v0.ledger.md`.

settlement: not claimed.
