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
- **R7 — Engineering-proof successor owns tool/operation retirement (open).**
  Mission 2 proves vocabulary only; the five overlay JSON tools and four
  legacy capsule operations plus replay-returns-original-receipts belong to
  the Engineering-proof successor mission: overlay JSON tools record_assignment_result, update_coagent, commit_transaction, inspect_self_development_bundle, record_self_development_verification; legacy capsule operations capsule_exec, capsule_list_dir, capsule_read_file, capsule_write_file; eval primitive capsule_go_eval in neither retirement list (counts verifiable via TestAssignedCoSuperBuilderIsExactClosedSet).
  Chartered as `docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` — the sole working entrypoint (ACTIVE.md).
  Revisit: when the carrier mission completes the retirement.
- **R8 — `actuator=tools` deletion deferred until every desk crosses (open).** The Engineering
  carrier keeps the `actuator=tools` registry branch alive but unused: no RLM proof may depend on
  it and no rollback path may target it. The owner (2026-09-11) set the deletion bar at the
  management and research desks also crossing to RLM, so the deletion does not happen in mission
  three. Source: `docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` item T5b.
  Revisit: when the last desk crosses; close it with the tool-profile tests proving the branch is
  gone.
- **R9 — deepseek/xiaomi provider policy heresy (owner ratification 2026-09-12).** The owner
  ratified the active provider set as opencode-go (with opencode-zen per carrier phase-1 setup)
  and chatgpt; the deepseek and xiaomi provider accounts are not configured and must not be
  silently substituted for. Compiled heresy being deleted in the carrier mission:
  `model_policy.go:32-46` constants (`defaultDeepSeekProvider`, `defaultXiaomiProvider`,
  `defaultConductorModel = "deepseek-v4-flash"`, `defaultSuperModel = "deepseek-v4-pro"`), the
  `:347-393` default-policy seeds and platform-fallback map, the `:630-651` deepseek↔xiaomi
  swap-table ladder (replaced by loud refusal — entitlement admits or refuses, never silently
  substitutes), `modelcatalog` entries, gateway env seeds, nix `DEFAULT_GATEWAY_*_MODELS`
  defaults, and the gateway model-id prefix heuristics. Roster model ids are unaffected:
  `deepseek-v4.1-flash` routes through the funded opencode-go provider. Full red ceremony applies
  (provider routing is a protected surface); rollback is git revert plus nix redeploy.
  Source: owner statements 2026-09-12 and `docs/reports/choir-model-policy-consensus-2026-09-12.md`.
  Revisit: carrier mission deletion completes it; any survivor becomes a named residue here.
- **R10 — Super substrate retirement (owner ratification 2026-09-12).** The owner states Super
  should be gone. Mission 2 (`choir-rlm-versioned-rename-2026-09-09`) renamed the vocabulary
  only: V1 `co-super` names decode through the frozen V1 map into `engineering`, while the
  machinery stays live — `internal/agentcore/super_controller.go` (2,809 lines, the persistent
  privileged drainer and CoSuper activation surfaces) and 1,331 `CoSuper`/`co_super` references.
  The carrier mission keeps what it chartered (assignment-fate saga, `update_coagent` parity,
  `actuator=tools` to R8); the full substrate retirement is stack-level. Source: owner statement
  2026-09-12, recorded in receipt engineering-carrier-completion-route-2026-09-12.
  Revisit: mission 6 (management as host-side activation) carries the activation rewrite;
  close the remainder at the post-11 residue pass with the profile tests proving the surface is
  gone.
- **R11 — `items.body` CAS externalization deferred to world-wire search
  redesign (owner direction 2026-09-19).** The platform-dolt normalization
  mission externalized `og_objects.body` (6.0G) to `platform-artifacts` CAS
  but left `items.body` (1.34G, ~780B mean, 5 rows >64KB on Store B) inline.
  The design premise that `items.body`/`raw_json`/`reader_snapshot` were fat
  payloads is falsified on the live store (`raw_json`=0 bytes,
  `reader_snapshot` is a TINYINT flag). Externalizing `items.body` breaks
  `cycle.Storage.SearchItems`' `lower(i.body) LIKE` recall — the search path
  behind `/internal/source-service/search` → the `source_search` agent tool
  (researcher role) and the objectgraph backfill. The search redesign
  (decoupling search from storage: FTS table, external index, or a
  CAS-hydrating search path) is world-wire product work, not storage
  normalization. Agentic-consensus panel 2026-09-19 (9/9 verdict A):
  `.agentic-consensus/agentic-consensus-20260919-010500/`. Source:
  `docs/goal-platform-dolt-normalization-2026-09-18.md` receipt
  `move3-cas-externalization` + amended acceptance. Revisit: when world-wire
  search is designed; the residual is bounded by a Store B growth SLO
  (monitor `items` table bytes + oldgen; reopen if growth threatens Store B
  capacity).

## Closed

- **R4 — Mission-1 correction superseding tuple.** Closed 2026-09-09 by settlement-gate item-3 repair (commit `965e26a7`): `CoSuperSupersedeTuple` required for attempt > 1, forbidden on attempt 1, validated in-store; slot conflict enforced on differing propositions without tuple.
- **R5 — Mission-1 narrow admission grammar.** Closed 2026-09-09 by settlement-gate item-4 repair (commit `18498447`): sequential turn execution for `capsule_go_eval` and `record_assignment_result`; Stage 1 pre-dispatch refusal (>= 2 terminals, > 1 eval, reversed order, forbidden companions) with none-run semantics; Stage 2 tray collision skip; toolloop decoupling for admitted shapes (a) and (b).

