# An object kind's owner is a package, not a desk (2026-10-21)

Status: **problem record, from code reading at `6ce19af5a`.** The claims are
about what the code does; none depends on a ledger. Mutation class of this
document: **green**.

Item: SDC ARCH-04
([security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md)),
one writer per object kind. Station:
[choir-launch-fp4-selfdev-architecture-2026-10-21.md](../definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md).
Plans: doctrine C2 ("Texture is the canonical document and artifact
control-plane core; other appagents own their own typed artifact domains") and
standing question 5 (the single authority for a state).

## The claim

The object graph says who owns a kind only as a package: `KindRegistration.Owner`
is `OwnerVMStore` ("internal/store (computer)"), `OwnerCorpusd` or
`OwnerAutoradio` (`internal/objectgraph/registry.go:24-44`). The write gate,
`CheckWrite`, asks one question of a write: is the kind registered
(`registry.go:299`). It does not ask who is writing. So:

1. Texture's sole-writer rule lives only in the Texture turn's shape check
   (`internal/store/texture_turn.go:179-180`, `AuthorKind == AuthorAppAgent`,
   "caller authority proves Texture"). Any other store method can still
   `ogPut` a `choir.texture_revision`; nothing in the graph refuses it.
2. Every app desk to come (podcast, reader; doctrine I26) would need the same
   private rule, written again in its own store method. There is no place to say
   "this kind belongs to this desk" that a package install can write.
3. Doctrine says canonical document versions have **two** writer classes, the
   owner's edits (`AuthorUser`, immediate canonical-head CAS) and Texture's
   revisions (`AuthorAppAgent`), and no other desk writes document text. The
   registry cannot express "one desk, plus the owner's own edit path".

## What the code shows about writers today

Verified by reading the writing function for each Texture kind:

- `choir.texture_document`, `choir.texture_revision`, `choir.texture_decision`:
  `graph_store.go` (`ogPut`, `graph_store.go:2327, 2506, 2653`) and
  `lifecycle.go:1468, 1475`; the turn is `ApplyTextureTurnWithSourceGraph`
  (`texture_turn.go:315`).
- `choir.source_entity`, `choir.source_ref`: the Texture turn's source graph
  (`TextureSourceGraphWriteSet`, `texture_source_graph.go`).
- `choir.citation_check`: "written by the Texture turn that first lists the pair
  (ApplyTextureTurn), in the same transaction as the revision" (`support.go:21-24`).
- `choir.support_verdict`: "written by the runtime's Jev job as a system act"
  (`support.go:25-28`): runtime, not a desk.
- Every `choir.*` kind in the corpusd publication layer is written by
  `internal/platform` (`publication_graph.go`, `publication_checks.go`,
  `releases.go`).
- The remaining VM-store kinds (agents, runs, events, inbox and lifecycle
  bookkeeping, assignments, schedules, settings, labels) are written by
  `internal/store` on behalf of the desks. Their per-desk split is the desk
  manifest's "owned object kinds" field (SDC ARCH-02, another lane); this
  record does not guess it.

A single transaction mixes writers: a Texture turn commits its revision with
lifecycle bookkeeping (inbox records, wake outbox, events). A gate that required
one writer for a whole batch would refuse every Texture turn.

## Substrate versus symptom

Symptom of a missing field on the substrate that already exists. The write gate
(H-OG1) is the right place and already runs first in every durable store
mutation (`dolt_store.go:511, 524, 890, 977`; `platform/objectgraph_store.go:104,
154, 361, 611`). No replacement implementation exists unwired (searched
`internal/` for a writer or desk field on kinds: none). The fix is a column and
a check there, not a new layer.

## Failure modes of the fix, written before the tests

1. **A wrong classification refuses a legitimate write.** The table is read from
   code, and 60 call sites are undeclared. Mitigation: the gate has three modes,
   `off`, `observe` (default: count and log each mismatch once per kind and
   allow) and `enforce` (refuse). The default does not change production. The
   Texture turn declares its writer, and a store test runs a whole Texture turn
   under `enforce` with the batch's mixed kinds.
2. **The gate is only as strong as its declarations.** Undeclared writes pass in
   `observe`. The counter is the oracle: the follow-up converts the call sites
   the count names, then flips `enforce`. Recorded as a residual, not claimed
   as enforcement.
3. **A writer declared by the wrong code.** The declaration is a context value
   set by trusted in-process code at the method that holds the desk authority.
   It is never read from a cell, a model or a request body.
4. **A second writer registers a kind already owned.** `RegisterKindChecked`
   (the seam a package install uses) refuses a kind without exactly one known
   writer and refuses re-registration under a different writer.
5. **Two writer classes for documents.** `OwnerEdits` on the registration admits
   the owner's edit path for the document kinds only; a test shows the owner is
   refused on a kind without it.
6. **Tombstones and replay.** The gate skips them exactly as `CheckWrite` does
   (retiring a historic row, the event projector, vocabulary migrations).
7. **Mixed batches.** Kinds written by the runtime on behalf of any desk
   (`WriterRuntime`) pass under any declared desk; a desk-owned kind passes only
   under its desk (or the owner where `OwnerEdits`); a corpusd kind passes only
   under corpusd.

## Rollback

Revert the commit. The change adds a registry column, a context helper and a
mode flag; `observe` is the default, so a revert changes no behavior.

## Heresy delta

- discovered: Texture's rule is a private check in one store method; the graph
  cannot say which desk owns a kind; no package install seam refuses a second
  writer.
- introduced: none.
- repaired: none until the commit lands. After it, the column and the check
  exist; enforcement waits on declarations.
