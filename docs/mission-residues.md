# Mission Residue Log

Residues are work deliberately left behind by a mission: blocked drills,
follow-ons, and deferred decisions that a later mission must circle back to.
  A residue is closed only by its named revisit trigger firing and the evidence
  being recorded here. Nothing here is waived by silence.

## Open

- **R1 — Mission-0 live failure-injection drill (blocked).** No scoped
  owner CLI/API control exists to inject base download/unpack/install
  failures into a live computer realization without host-level intervention.
  Local fault injection and interruption resumption are proven
  (`install_test.go`, `restore_base_test.go`); the production-boundary
  drill is recorded, not substituted. Source:
  `docs/evidence/choir-rlm-restore-zero-deployed-proof-2026-09-09.md`
  section on blocked prerequisites; mission-0 `now.blocker_or_risk`.
  Revisit: when a scoped product control for live fault injection exists;
  owned by whichever mission charters it. Mission-1 scope: neither
  implements nor waives (owner decision 2026-09-09).
- **R2 — Keep watermark within 10000 of head (S1-B).** Follow-on from
  mission-0 completion: periodic near-head W publication. Source:
  mission-0 `now.next_action`. Revisit: before any restore whose tail
  approaches that bound.
- **R3 — Proxy boot-screen during resolve lock (optional).** Follow-on
  from mission-0 completion; cosmetic. Revisit: next proxy/UX pass.
- **R6 — Cutover final disposition deferred (owner direction 2026-09-09).**
  Mission 1 closes the cutover's withheld settlement acceptance and retains
  the cutover Definition as remainder holder; full supersession by a named
  successor is deferred while the program continues in future missions.
  Revisit: a later mission names the complete successor and redirects the
  cutover atomically.
## Closed

- **R4 — Mission-1 correction superseding tuple.** Closed 2026-09-09 by settlement-gate item-3 repair (commit `965e26a7`): `CoSuperSupersedeTuple` required for attempt > 1, forbidden on attempt 1, validated in-store; slot conflict enforced on differing propositions without tuple.
- **R5 — Mission-1 narrow admission grammar.** Closed 2026-09-09 by settlement-gate item-4 repair (commit `18498447`): sequential turn execution for `capsule_go_eval` and `record_assignment_result`; Stage 1 pre-dispatch refusal (>= 2 terminals, > 1 eval, reversed order, forbidden companions) with none-run semantics; Stage 2 tray collision skip; toolloop decoupling for admitted shapes (a) and (b).

