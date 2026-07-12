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

## Post-Repair Independent Verification

The type-aware store-writer repair raised the baseline from 26 to 121 writers and correctly removed wrapper false positives, but independent micro-verification found one remaining current omission:

### S0-CONS-002 — `PatchRevisionMetadata` Wire writer omitted

**Status:** confirmed; blocking.

`internal/runtime/wire_platform_publish.go` directly calls the underlying `internal/store.Store.PatchRevisionMetadata` mutation. The writer classifier's mutation-prefix set omits `Patch`, so the current Wire mutation is absent from the baseline and a future `Patch*` store writer would not trigger drift. Existing regression coverage introduces only an `UpsertAppAdoption` writer and does not exercise a mutation verb outside the allowlist.

**Required repair:** classify `Patch` store mutations, add `PatchRevisionMetadata` to the Wire baseline with a Wire disposition, and add a focused regression proving a new `Patch*` underlying store mutation cannot bypass disposition.

### S0-CONS-003 — positive mutation-verb allowlist is the broken substrate

**Status:** confirmed; blocking; root-cause cluster.

After the `Patch` repair, independent verification found current underlying lifecycle mutations still omitted: `Store.ClaimCoSuperSlot`, `Store.ReleaseCoSuperSlotClaim`, and `Store.CancelAgentMutation`. The same positive verb-prefix gate drops `Claim`, `Release`, and `Cancel` before domain classification. S0-CONS-001, S0-CONS-002, and S0-CONS-003 are therefore one substrate bug, not three independent missing words.

**Structural assessment:** the dependency graph is `typed store call -> positive mutation verb allowlist -> domain noun classifier -> baseline`. Type resolution is now authoritative, but the next node throws away real mutations using an indefinitely incomplete vocabulary. Adding more verbs repeats the failure class. The substrate-level repair must remove the open-ended positive mutation-verb allowlist and derive mutability from an authoritative store-method classification that is exhaustive over every called `internal/store.Store` method. Unknown methods must fail closed for disposition, not disappear. Current lifecycle, Wire, and promotion calls must be covered by exact regressions, including Claim/Release/Cancel/Patch.

**Required repair:** replace incremental verb additions with exhaustive typed store-method classification; make every called store method either a declared read or a dispositioned writer, reject unknown methods, and regenerate the baseline.

### S0-CONS-004 — read-prefix fallback keeps the classifier fail-open

**Status:** confirmed; blocking; same substrate.

The substrate repair removed the positive writer-verb allowlist, but `storeCallDisposition` still declares any store method beginning with `Active`, `Count`, `Current`, `Find`, `Get`, `Has`, `Is`, `Latest`, `List`, `Load`, `Lookup`, `Read`, `Resolve`, or `Search` to be a read. An unknown mutator such as `GetAndDeleteState` or `LoadOrCreateRun` therefore bypasses the fail-closed branch. The `TransmogrifyState` regression proves only a name outside both allowlists.

**Required repair:** make the baseline itself classify every exact typed store-call identity as `read | lifecycle | wire | promotion`; the scanner must enumerate calls without inferring safety from prefixes. A new method/call is then an undispositioned added item and fails regardless of its name. Equivalent exact method-name authority is acceptable only if unknown names fail closed. Add read-prefixed mutator regressions.

### Final Substrate Repair Verification

The scanner now emits every exact typed production call resolving to `internal/store.Store` without inferring safety from its name. The baseline is the sole disposition authority: all 460 calls are explicitly `read`, `lifecycle`, `wire`, or `promotion`; a novel identity fails before it can be treated as safe. Regressions cover `GetAndDeleteState`, `LoadOrCreateRun`, `TransmogrifyState`, `PatchRevisionMetadata`, and a legitimate exact read that passes only after baseline disposition.

Independent verifier `S0RatchetVerifier` reran the full focused suite and baseline command at `9319eca8` and reported PASS with no blockers. Current Claim/Release/Cancel/Patch/promotion/Wire calls are present; all IDs resolve to underlying store methods rather than runtime wrappers.

## Post-Repair Panel

Run: `/tmp/choir-s0-post-repair-consensus-20260711`.

Six agents completed successfully: Codex, opencode, Cursor, OMP GPT-5.5, OMP Gemini 3.5, and OMP GLM 5.2. Devin produced no output and stalled beyond the runner deadline; the runner was terminated after preserving the six completed opinions. Cursor completed successfully with `status=ok`, so the permission prompts observed outside this run did not stall this panel. Per owner direction, one stalled panel member does not block adjudication when independent completed evidence remains.

Verdicts: Codex, Cursor, and OMP GPT-5.5 reported `BLOCKING`; OMP Gemini and OMP GLM reported `PASS`; opencode's completed output supplied investigation evidence but did not finish the requested verdict shape. The two demonstrated blocker classes below govern regardless of vote count.

### S0-POST-001 — interface-mediated store mutations are invisible

**Status:** confirmed lead; blocking pending repair.

Cursor identified live `runtime_persistence.go` calls through runtime-local interfaces such as `runSubmissionStore`. The called function resolves to an interface method rather than a method declared in `internal/store`, so the scanner emits no `store_calls` identity even when the concrete value is `*store.Store`. A new store mutation called only through the same interface shape can therefore bypass disposition.

**Required repair:** resolve store-backed interface calls through concrete type/data-flow evidence, or conservatively inventory interface method calls whose method name and signature match `internal/store.Store`; unknown interface-mediated store operations must fail closed. Add a runtime interface fixture.

### S0-POST-002 — Store method values bypass call inventory

**Status:** confirmed; blocking.

Codex and OMP GPT-5.5 independently found the same direct false pass. For `f := stateStore.SaveDesktopState; f(ctx, state)`, the selection resolves to the Store method but the later call resolves to a local variable, so direct `CallExpr` function resolution emits no identity. OMP GPT-5.5 reproduced this with a temporary focused test: a novel Store mutation invoked through a method value produced zero `store_calls` and the comparison passed.

**Required repair:** inventory Store method selections used as values and bind their eventual calls through SSA/data flow, or conservatively fail closed on every Store method selection until it is explicitly dispositioned. Add mutating and legitimate-read method-value regressions.

The panel also noted that three package-level `internal/store` helpers are over-inventoried as Store calls. This is conservative but should be corrected while resolving receiver identity: only Store methods, explicitly store-backed interface methods, and tracked Store method values belong.

### S0-POST-003 — interface matching lacks Store provenance

**Status:** confirmed; blocking repair correctness.

The interface repair matches any named runtime interface method by name/signature against `Store`, without proving that a `*store.Store` value can reach the receiver. An unrelated interface such as `reporter { GetRun() }`, implemented and used only by a fake runtime type, is therefore inventoried because Store also has `GetRun`. This creates false drift and contradicts the repair contract's unrelated-interface exclusion.

**Required repair:** interface-mediated inventory must require concrete Store provenance. At minimum, prove a Store value is passed/assigned to the interface receiver path; merely satisfying the interface or sharing a method signature is insufficient. Add same-name fake-interface and real Store-passed-interface regressions.

### Final Interface-Provenance Verification

The repair now seeds interface provenance only from concrete `*store.Store` arguments or assignments and propagates that provenance through runtime-local interface argument/assignment edges. Independent verification at `0a391d08` passed: fake-only same-signature interfaces are excluded; direct and transitive Store-backed interfaces are included; novel interface mutations fail until disposition; method values remain covered; package helpers and unrelated interfaces remain excluded. The baseline contains four provenance-backed `runSubmissionStore` identities and 457 concrete Store selections, 461 total.

Per owner instruction and observed panel behavior, subsequent panels exclude the stalled Devin member rather than allowing one zero-output process to halt the run. Cursor remains included because both completed panels recorded Cursor `ok`.

## Checkpoint Result

S0 remains `consensus_pending` only for the final post-repair panel. S0-POST-001 through S0-POST-003 are repaired according to focused and independent evidence; S1 remains waiting until the six-member non-stalled panel is adjudicated.
