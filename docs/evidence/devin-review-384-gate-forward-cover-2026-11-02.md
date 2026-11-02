# Devin review 384: the gate's forward cover (rule 7), 2026-11-02

Seat: Devin (one seat; a gate-file change). Candidate: fb9f48d35 (scripts/check-guest-security-epoch, scripts/selftest-guest-security-epoch). Owner approved the mechanism 2026-11-01 ("Do do #1"). Verdict: **approve**, confidence medium-high.

Verified by the seat: covers resolve by exact full sha; the target must be an earlier commit of the range; a "none" cover never satisfies a gate-file commit; the epoch rules and CI-wiring checks run outside the cover path; trailer parsing is confined to the trailer block, and a malformed cover line fails closed; bash 3.2 idioms are safe; running both copies in CI only over-rejects.

Findings and adjudication:
1. Non-blocking: the first cover won, so a wrong-kind cover made a later right-kind cover useless and re-wedged main. **Adopted**: any cover of the right kind counts; selftest c4b.
2. Non-blocking: a malformed cover line is itself uncoverable. **Rejected, documented**: loud strictness is kept; the pre-push hook runs the gate before the push, so a malformed cover is caught before it is on main.
3. Nit: covers only work once the deployed base's copy has rule 7. **Adopted** as a header note.
4. Nit: "earlier" is rev-list order, not DAG order; this fails closed. **Accepted**, no change.
5. Nit: selftest gaps. **Partly adopted**: duplicate covers on one sha (c4b).
6. Nit: stale comment. **Adopted**.

Selftest: 64 cases pass under bash 5.3 and macOS /bin/bash 3.2.
