# The vocabulary migration rewrites model ids (2026-11-05)

Status: open. Documented first; the fix follows. Class: red (store
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

The 3 correct requests most likely come from a fresh pool resolution; the
mangled ones from a model id read back from a stored, upcast leaf (not yet
traced to the object kind).

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
