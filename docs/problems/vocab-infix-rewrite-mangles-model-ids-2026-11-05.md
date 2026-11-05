# The vocabulary migration rewrites model ids (2026-11-05)

Status: fixed in the commit that adds this line (not yet deployed). Class: red (store
deposit upcast and replay fixpoint).

## Evidence

Node B gateway journal, 2026-11-05 18Z to 19Z: 27 inference requests from
guests named model `nvidia/nemotron-3.5-management-vl:free` (24 from the
paper, vm-1669020adcece51c2f5ff9b6df399ef5; 3 from vm-70070cf7…), refused
with `model ... is not in the catalog for provider openrouter`. The correct
id `nvidia/nemotron-3.5-super-vl:free` (modelcatalog DefaultPool, added
2026-11-05 in 6855e5eca) was requested 3 times. Source is correct.

Reproduced at origin/main: `ogLeafMigrate("nvidia/nemotron-3.5-super-vl:free")`
returns `nvidia/nemotron-3.5-management-vl:free`, changed.

## Mechanism

`migrateIDForward` (internal/store/vocab_migrate.go) rewrites the
hyphen-delimited desk infixes `-co-super-`, `-cosuper-`, `-researcher-`,
`-super-` with `strings.ReplaceAll` on the whole string. `ogLeafMigrate`
(vocab_migrate_og.go) applies it to every JSON string leaf of an object or
deposit; the only exemptions are leaves with whitespace (prose) and leaves
containing `://` (URLs), added after
problems/live-writes-refused-by-deposit-upcast-prose-leaves-2026-10-16.md.
A model id is neither, so it is treated as a constructed desk id. Callers:
the deposit upcast (vocab_upcast_deposit.go migrateDepositLeaves), the replay
fixpoint (vocab_replay_fence.go) and planOGMigration; IsServingVocabularyLeaf
uses the same rule, so a live writer is held to it.

Traced to the stored field (code trace; the deployed run records were not
read): the policy stamps the resolved model into run metadata
(`llm_model`, `model`, `llm_fallbacks`; runtime.go createRunWithMetadata).
`CreateRun` persists the record as a `choir.run` object whose body is the
RunRecord, so those metadata values are body leaves. On a store whose
replayed deposits were upcast (the paper's and any computer replayed from
scratch), the live finalize path also upcasts, so the stored leaves become
`nvidia/nemotron-3.5-management-vl:free` (and the same inside the
comma-joined `llm_fallbacks`). The actor runtime then loads the stored run
for `initial_dispatch` and channel reactivation (actorruntime/handler.go,
scopedRunForUpdate) and `executeWithToolLoop` takes the model from
`rec.Metadata` through `provideriface.ResolvedLLMConfigFromMetadata` and
`FallbackLLMConfigsFromMetadata`. `EnrichMetadata` returns early when
provider and model are already present, so the catalog check in
`modelpolicy.Assign` (which would remap an uncatalogued model) never sees
it. The 3 correct requests are activations that used the in-memory record
rather than the stored one (hypothesis, consistent with the trace).

## Effect

Any whitespace-free, non-URL stored string containing `-super-`,
`-co-super-`, `-cosuper-` or `-researcher-` is rewritten: model ids (any
"Super" model variant), file names, slugs. Today: one sixth of the paper's
pool (nemotron) is unreachable and its calls fail over. This is the second
instance of the class (the first was prose leaves, 2026-10-16): the rule
infers "this leaf is a desk id" from its shape, and the shape is not
specific enough.

## Fix direction

Restrict the infix rule to leaves that are constructed desk ids (the V1
shapes it was written for, run-super-assignment / work-super-assignment
style), not every leaf; repair the stored leaves already rewritten
(`-management-vl` is never a valid model id) or alias them. The replay
fixpoint must stay consistent: a deposit replayed from genesis and the live
store must agree under the narrowed rule.

## Failure modes considered (written before the fix)

- **Replay divergence.** A from-genesis replay and the live store must
  apply the same rule to the same deposit. The narrowed rule is one pure
  function used by every path (deposit upcast, end-of-replay plan, replay
  fixpoint, live-writer check), so the paths cannot disagree with each
  other. They can disagree with *history*: a store that upcast a model id
  under the old rule holds `-management-vl`, and a replay under the new
  rule yields `-super-vl`. That divergence is a correction (the tape holds
  the original bytes; the live row is the wrong one), not a new defect, and
  it is bounded to leaves that were never desk ids.
- **A real V1 desk id stops migrating.** Every V1 shape the infix rule was
  written for must still rewrite byte-identically, or a replay of a V1
  deposit leaves a V1 identity in a V2 store and the fence refuses it.
  Pinned by test: `run-super-assignment-1`, `work-super-assignment`,
  `run-cosuper-assignment-01`, `work-cosuper-1`, the V1 engineering
  command ids and digest refs (`co-super-report:…`, `co-super-grant:sha256:…`
  and siblings), `queue-super-report:…`, `persistent-super-recovery:v2:…`,
  `lifecycle-researcher-admission-recovery:v1:…`.
- **The fence refuses live writes.** Narrowing only grows the set of fixed
  points, so a leaf that is serving vocabulary today stays so; a model id,
  file name or slug becomes serving vocabulary. Nothing that was accepted
  is refused.
- **Already-mangled stored leaves stay mangled.** `-management-vl` is a
  fixed point of both rules, so no migration pass will undo it. The
  repair has to be at the read that chooses a model, or the mangled id
  keeps reaching the gateway from stored run records.

## Fix (2026-11-05)

- **The rule is narrowed to V1 constructed-id shapes.** `migrateIDForward`
  applies the infix rule only when `isV1InfixIDShape` holds: the leaf has
  no `/` and no `.`, and starts with one of the V1 constructed-id prefixes
  found in the code that minted them (`run-`, `work-`, `co-super-`,
  `queue-super-`, `persistent-super-`, `lifecycle-researcher-`; sources:
  the charter tie-break and the drill seeds, the V1 engineering command ids
  and digest refs, tools_engineering_assignment.go's queued producer
  update, the two recovery prefixes). Every such id rewrites to exactly the
  bytes it did before. The prefix rules (`super:` and siblings) and the
  exact-token map are unchanged. Because every path (deposit upcast,
  planOGMigration, replay fixpoint, identityFieldMigrated,
  IsServingVocabularyLeaf) goes through the same function, they agree.
- **Stored leaves are repaired at the read that chooses a model.**
  `provideriface.ResolvedLLMConfigFromMetadata` and
  `FallbackLLMConfigsFromMetadata` map a model the catalog does not list,
  for a provider it does list, back to the catalog id when inverting a
  desk infix (`-management-` to `-super-`, `-engineering-` to `-co-super-`
  or `-cosuper-`, `-research-` to `-researcher-`) lands on one. It is
  idempotent and touches nothing the catalog lists. Stored rows are not
  rewritten: a from-genesis replay already yields the correct bytes (the
  tape holds the original), and the run records are the only stored
  carrier that chooses a model.
- Tests: internal/store/vocab_infix_model_ids_test.go (model ids, paths,
  slugs unchanged and serving vocabulary; every V1 shape still migrates
  byte-identically; a replayed deposit holding model ids lands unchanged
  and the boot rescan agrees) and
  internal/provideriface/vocab_mangled_model_test.go.

## Residuals

- Live run records stored before the fix keep the mangled bytes; a
  replay-completeness probe on such a computer reports those rows as
  live/replay differences (replay is the correct side). They age out with
  the runs.
- Provenance readers that display `llm_model` from a stored run
  (management_tasks.go sub-call report, record_model.go) show the mangled
  id for runs stored before the fix. Display only.
- `persistent-super-recovery:v2:` and `queue-super-report:` are still
  minted live by current code and are not serving vocabulary under either
  rule: wherever one is stored as a whitespace-free leaf on an upcast store
  it is rewritten (not traced to a carrier). That predates this fix and is
  kept byte-identical here so replay does not change; it needs its own
  look.
- A slug or id that happens to start with `run-`/`work-` and carry a desk
  infix (no `/` or `.`) is still rewritten. No such leaf is known.
