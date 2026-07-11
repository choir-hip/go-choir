# S0 Runtime Inventory And Ratchet Consensus

## Panel

Run: `/tmp/choir-s0-consensus-20260711`

| Agent | Status | Verdict |
|---|---|---|
| Codex CLI | ok | BLOCKING |
| Devin CLI | ok | PASS |
| Cursor Agent CLI | ok | BLOCKING |
| opencode CLI | ok | BLOCKING |
| OMP GPT-5.5 | ok | BLOCKING |
| OMP Gemini 3.5 | ok | PASS |
| OMP GLM 5.2 | ok | PASS |

All seven agents exited successfully. Cursor did not stall this panel: the runner used `--mode ask --trust --force --approve-mcps` with stdin detached, and `manifest.tsv` recorded `cursor ok 0`.

Exact prompt and raw outputs are retained in `/tmp/choir-s0-consensus-20260711/{prompt.md,manifest.tsv,*.out,*.cmd}` for this run. The reviewed range was `008a7b88cf200119c0f762cc51cfba6be3007445..93b67ee6`.

## Confirmed Blocking Finding

### S0-CONS-001 — lifecycle/Wire/promotion state-writer inventory is incomplete

**Status:** confirmed; blocking.

**Evidence:** Cursor and OMP GPT-5.5 independently identified the same detector gap, with opencode also identifying the promotion-writer omission. Local inspection confirmed current production mutations that the checked-in `state_writers` baseline does not name, including:

- `internal/runtime/app_promotion.go`: `UpsertComputerSourceLineage`, `UpsertAppChangePackage`, and repeated `UpsertAppAdoption` calls;
- `internal/runtime/api_app_promotion.go`: `UpsertAppChangePackage`;
- `internal/runtime/candidate_package_intake.go`: `UpsertAppChangePackage`, `UpsertAppAdoption`, `UpdateAppAdoptionIfCurrent`, and `UpsertComputerSourceLineage`.

`cmd/runtime-ratchet/inventory.go` currently requires both a narrow writer verb and a narrow object token in the selected method name. The live store mutations above use `Upsert` or promotion-state nouns not covered by that conjunction. A 26-row self-consistent baseline therefore produces a false pass while omitting protected promotion authority. Cursor additionally found that the baseline contains no `wire`-disposition writer despite live Wire publication mutation.

**Required repair:** inventory actual lifecycle, Wire, and promotion mutations through type-aware store/CAS/adapter call resolution rather than business-method-name regex alone; include the current missed store methods; add focused fixtures proving known promotion and Wire writers are present and that a new writer fails without disposition; regenerate the baseline and rerun independent verification plus consensus.

## Adjudicated Non-Blocking Findings

### Canonical-parent mismatch

Codex treated dispatch prompt parent `1a9a90b6` versus implementation baseline parent `f72a141e` as blocking. Rejected with evidence: the durable stage history records `1a9a90b6` as `dispatch_intent` parent and `f72a141e` as the subsequent canonical `dispatched` parent from which the isolated implementation started. These are distinct transaction stages, not conflicting claims. The implementation should still stop duplicating provenance constants when a later cutover makes that practical.

### Build-context coverage

Codex requested every possible GOOS/GOARCH/tag combination. Narrowed: Choir's current authoritative contexts are the Darwin development observer and Linux production/deployment observer, both scanned. Arbitrary Windows or undeclared product tags are not S0 production callers. The checker should make the supported context set explicit and fail when a declared production context cannot be evaluated; this is not a blocker for the current topology.

### Citer surfaces

opencode found root `README.md` and `.beads` references outside the configured citer surface. The suite's explicit citer contract names `docs/`, `specs/`, `skills/`, `AGENTS.md`, code comments, manifests, CI, and generated detector manifests. Root narrative and issue-ledger text are not silently accepted as implementation dependencies. Any manifest-shaped file remains required regardless of directory. This is a detector-boundary review item, not the confirmed S0 blocker.

### Heuristic and operational residuals

Non-blocking but retained for adjudication after repair:

- compatibility-marker identity still embeds line numbers;
- domain disposition is filename-heuristic and semantically reviewer-owned;
- route/tool discovery is syntax-shape based;
- the ratchet is not yet a dedicated CI workflow invocation, so S3 must invoke it as a mandatory checkpoint command;
- Git lookup failures can be reported as missing prior authority rather than preserving the underlying diagnostic;
- the caller graph's module prefix is repository-specific by design.

## Checkpoint Result

S0 remains `consensus_pending` / incomplete. The panel majority is not authority; the confirmed state-writer omission is independently reproduced and blocks adjudication. S1 must not start until S0-CONS-001 is repaired, independently reverified, and the post-repair panel is adjudicated.
