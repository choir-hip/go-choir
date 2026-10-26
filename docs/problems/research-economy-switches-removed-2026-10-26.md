# Research economy switches removed (2026-10-26)

Mutation class: green for this doc; the deletion that follows is orange (guest
runtime behavior; no provider routing, no VM lifecycle). Rollback: git revert of
the deletion commit; both features were already off everywhere that matters.

## Owner ruling (2026-10-26, verbatim)

"we should definitely have this off. Over time, we can optimize the researcher
for when it should fan out with sub RLMs, as opposed to continue in a sequential
progress. But we actually are not bottlenecked on research token usage. Since we
instituted caching everywhere, we've been much more token efficient. So frankly,
I want to actually just remove that switch entirely."

"Yeah, research.searchcaps is a similarly bad pattern. And I guess, yeah, some
of these ideas were experiments that didn't work, so we shouldn't just turn
everything on."

## Decision

Delete both research economy experiments, not turn them off:

- `research.run_memory_elision` (E2), its env twin `CHOIR_RUN_MEMORY_ELISION`
  and the `_KEEP`/`_BATCH`/`_MIN_CHARS` knobs, with the rolling-elision code and
  the `run_memory_elision` run-progress event.
- `research.search_caps` (E3) and the env knobs `CHOIR_DESK_SEARCH_MAX_RESULTS`
  and `CHOIR_DESK_SEARCH_SNIPPET_CHARS`, with the snippet trimming code. A desk
  search asks for 40 results with whole snippets, as it does today with the
  switch off.

Why: the 2026-10-15 owner ruling ("Don't economize on search; maintain the
append-only pattern for 90%+ caching") stood against both; elision rewrites
already-cached history bytes; the simulated saving (-17% of all compute) was
measured before caching landed everywhere. Fan-out by sub-RLMs is the future
economy lever, not context trimming.

## Citers (deletion citers, standing question)

On origin/main at f238ed79a:

- `internal/agentcore`: `computer_settings.go` (two apply branches),
  `research_switches.go` + `_test.go` (deleted whole), `run_memory_elision.go` +
  `_test.go` (deleted whole), `run_memory.go` (the `elision` field, `withElision`,
  `elidedThrough`, the pressure estimate through the elision view),
  `desk_search.go` (caps arguments, `trimSearchSnippet`, defaults),
  `desk_search_trim_test.go`, `tools_desk.go:509` (passes the caps flag),
  `runtime.go` (two atomic flags, the boot call, the policy at the memory manager
  build).
- `internal/store/computer_setting.go`: two constants and two
  `KnownComputerSettings` entries.
- Searched and found no citer: `nix/`, `scripts/`, `frontend/`, `runtimeprompts`,
  any Settings UI list. The settings list is served from `KnownComputerSettings`.
- Docs that describe them live: `docs/evidence/switched-off-inventory-2026-10-26.md`
  (rows updated). Historical reports (`docs/reports/*`, ledgers, the 10-24
  inventory, the residual register's SW-01 row) are left as the record.

## Failure modes of the deletion, written before code

1. **Stored rows for a removed key.** b@c has both stored true; d@e and the paper
   have both stored false. The rows stay on the tape (the tape is append-only;
   they are not deleted). `ListComputerSettings` iterates `KnownComputerSettings`,
   so a stored row for a removed key is never read, never listed, never an error.
   GET `/api/computer-settings` must return 200 with exactly the known keys; the
   boot-time load must not read the removed keys at all. Guard: a store test
   writes a raw stored row for each removed key and lists.
2. **POST of a removed key** must answer the ordinary `unknown setting <key>` 400
   (the allowlist), not a 500 and not a stored row. A client still holding the old
   key (a stale script, the old inventory's flip list) is refused loudly.
3. **Env twins left set on a guest.** `CHOIR_RUN_MEMORY_ELISION=1` and
   `CHOIR_DESK_SEARCH_*` may still be exported on some guest image; after the
   deletion they must do nothing: no stub in the context, 40 results, whole
   snippets. Guard: tests set the env and assert the off behavior.
4. **Behavior drift on the off path.** The run memory context, the compaction
   pressure estimate and the search tool must be byte-identical to the switch-off
   path. Guard: the existing run memory and desk search tests, plus the env tests
   above.
5. **Census tests.** Tests that count or list known settings must not still name
   the removed keys; a test pins that the removed keys are not in
   `KnownComputerSettings`.
6. **Dead helpers left behind.** After the delete, `go vet` and `go build` must
   show nothing orphaned (the `entryID` field of the context item, env helpers).

## Heresy delta

discovered: none new. introduced: none. repaired: two switched-off experiments
that the 10-26 inventory carried as "leave off, owner ruling" are gone, so no
future flip list can offer them.
