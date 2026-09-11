# Mission-2: RLM Versioned Rename — Completion Report

**Date:** 2026-09-11
**Definition:** `docs/definitions/choir-rlm-versioned-rename-2026-09-09.md`
**Status:** Settled with deployed proof on physical staging computer
**Duration:** ~48 hours (2026-09-09 → 2026-09-11)
**Mutation class:** red (protected surfaces: Texture canonical writes, Trace/evidence, checkpoint/route projection, auth/session renewal, vmctl, gateway/provider calls, run acceptance, deployment routing)

---

## 1. What was done

Renamed the live RLM vocabulary from V1 (`super`, `co-super`, `researcher`) to V2 (`management`, `engineering`, `research`) across every carrier — tape events, mailbox rows, lifecycle rows, assignment rows, grant rows, object-graph objects and edges, prompt roles, model-policy TOML, capsule vocabulary, broker admission, and all API surfaces — while preserving byte-identical replay of historic V1 records and fail-closed refusal of unknown tokens.

The rename was executed as a **single never-partially-deployed cutover**: live-row forward-migration and serving fence ran atomically, then `CurrentVocabularyVersion=v2` plus V2-only writers plus alias retirement plus fail-closed refusal landed in one commit (`a14abf05`), with in-scope client bundles (TextureEditor.svelte, CLI, capsule, API clients) updated in the same push.

## 2. How it was proved

Deployed proof on the physical staging computer (`computer-03335285269bdba4f94377e56879f9e6`, VM `candidate-fleet-e15cb89f25d963c220319b7b`, epoch 909):

- **V2 roles resolve:** `engineering` → `deepseek/deepseek-v4-flash`, `management` → `chatgpt/gpt-5.6-luna`, `research` → `chatgpt/gpt-5.6-luna` via `/api/model-policy/resolve` (200 OK).
- **V1 roles refuse:** `super`, `co-super` → 400 `{"error":"unknown role"}`.
- **Historic tape intact:** V1 replay byte-identical; frozen V1 decode verifies pre-cutover records without mutation.
- **Migration crash-consistent:** provenance report persisted before row mutation; `FencedAt` stamp enables 29-second fast-path reboot (vs 12-minute initial scan).
- **CI green:** all gates passed on `e3396329` (run `34571343061`).

## 3. Commit inventory

**77 commits** between `bcfa97b7` (draft) and `bddd4f00` (settle):

| Class | Count | Purpose |
|-------|-------|---------|
| `red` | 18 | Runtime cutover, migration, fence, decode roots, writer purity, OG boundary, boot-stall repairs |
| `yellow` | 3 | Inventory corpus freeze, gate script coverage, skills panel anchor |
| `green` | 56 | Definition drafts (10 rounds), reconciliation repins, evidence receipts, mission-3 prep, problem documentation |

### Red commits (runtime behavior)

| Commit | Change |
|--------|--------|
| `dc413b56` | Centralize Event/CASRequest/DurableEvent decode behind historic and admission roots |
| `c19e3028` | Widen projectionbase known vocabulary to v1+v2, never revert |
| `19dd5116` | `agentprofile.Canonical` returns typed unknown-profile tuple |
| `b2eccab3` | Frozen V1→V2 row migration core with proven inverses |
| `70057ab7` | Serving-fence verification core with refusal contracts |
| `219814ca` | SQL row migration with provenance plus scratch drill proving inverse |
| `00af1397` | Break test-only import cycle on seam pin test |
| `a14abf05` | **Single never-partially-deployed V2 writer cutover** with fail-closed refusal |
| `1f51aaf5` | Writer-purity gate replaces spent freeze gate; test assertions track V2 prose |
| `7cee94d5` | Wire live-row vocabulary migration and serving fence into production paths |
| `cb571960` | Decode V1 role sections in computer-owned model-policy TOML |
| `80dea87e` | Migration correctness — longest-first compound ID ordering, crash-atomic provenance, `run_memory_entries` carrier |
| `1e96e0b2` | Object-graph vocabulary boundary — in-place migration, write guard, fence coverage |
| `742fd4d3` | Channel-cast role canonicalize-or-refuse + OG write validator hook |
| `ffdc2d85` | Unify multimodal verifier spelling; pin fail-closed default policy |
| `2366d7b8` | Stream OG migration scan — retain only migrated or ref-bearing rows |
| `05b109a5` | Boot-stall repairs — fenced fast path, gate progress heartbeat, `author_label` leaf fix, unbounded blob download |
| `e3396329` | Base install marker normalization, safe head verify, and fixpoint heartbeat |

## 4. Problems found and fixed during execution

### 4.1 Post-cutover review reopened acceptance items

A 13-model convergent consensus panel (codex, claude/opus, cursor, opencode, omp-gpt56-sol/terra/luna, omp-gemini38, omp-cursor-grok46, omp-muse-spark, omp-nemotron-3-ultra, omp-glm53-flash) found that the initial deployed proof (`ae62fc82`, epoch 896) was **narrower than claimed**:

- Migration and serving fence did not cover the persistence carrier that actually serves (`og_objects`/`og_edges`).
- Historic V1 values were refused by live read paths (should be readable via frozen V1 decode).
- The mission was not recorded complete despite the receipt.

This triggered a repair wave (`80dea87e` → `2366d7b8`) that added the OG vocabulary boundary, write guard, fence coverage, and migration correctness fixes.

### 4.2 Boot stall on staging owner computer

After `2366d7b8` deployed, the owner computer entered an infinite crash loop: guest replay stalled at 5 minutes with no sequence advance, VM marked failed, cold recovery quarantined the disk, and fresh-disk boot refused rebase.

Root causes found and repaired (`05b109a5`, `e3396329`):

- **Silent post-replay migration:** the OG migration scan ran inside replay with no progress heartbeat, tripping the 5-minute stall detector.
- **`author_label` leaf fix:** fence refused `author_label=appagent` (D4) because the leaf-key classification was incomplete.
- **Unbounded blob download:** 30-second timeout on base-image download was too short for large blobs.
- **Base install marker normalization:** marker file path mismatch caused fresh-disk boot to refuse rebase.
- **Fixpoint heartbeat:** migration progress now emits heartbeats so the stall detector sees forward motion.

### 4.3 Disk space exhaustion (post-settlement)

The guest's 32 GiB `data.img` held an **18.6 GiB unflushed Dolt journal** (`state.texture/texture/.dolt/noms/vvvv...`). Live data was ~2 GiB. The journal grew ~1 GiB/hr and would have hit ENOSPC in ~12 hours.

Root cause: `MaybeRunDoltGC` had a 5 GiB size guard that counted **total disk used including the journal itself**. Once the journal pushed usage past 5 GiB, every GC was skipped, which let the journal grow more, which kept GC skipped. The emergency low-space path was unreachable behind the guard.

Fix (`a907f713`, orange(gc)):

- Emergency GC runs first, bypassing the size guard (ENOSPC is unrecoverable; OOM is retryable).
- Size guard now measures **live bytes** (used minus journal) — journal growth can't suppress the GC that reclaims it.
- New journal trigger: `RUNTIME_DOLT_GC_JOURNAL_GIB` (default 1 GiB) runs routine GC when the journal grows.
- `QuarantineDataImage` was called with `maxRetained=0` (pruning disabled) — now `VMCTL_RECOVERY_QUARANTINE_RETAINED` (default 2).
- Journal phases `swapped`/`booted`/`verified`/`route_published`/`done` are now prunable; pre-swap phases stay protected.
- `PruneRecoveryQuarantines` runs in the daily reclaim sweep.

Offline GC executed: 18.6 GiB journal → 2.1 GiB store; head seq 148655, index 148655, 305 log entries verified intact. Guest rebooted clean at 12.4% used.

## 5. Evidence artifacts

| Artifact | Path |
|----------|------|
| V1 inventory corpus (673 rows) | `docs/evidence/choir-rlm-v1-inventory-corpus-2026-09-10.txt` |
| V1→V2 mapping tables | `docs/evidence/choir-rlm-v2-mapping-2026-09-10.md` |
| Decoder matrix | `docs/evidence/choir-rlm-decoder-matrix-2026-09-10.md` |
| Post-cutover review | `docs/evidence/choir-rlm-rename-post-cutover-review-2026-09-10.md` |
| Boot stall problem record | `docs/evidence/choir-rlm-cutover-boot-stall-2026-09-11.md` |
| Deployed proof | `docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md` |

## 6. Rollback references

- **Source:** `git revert` of cutover commit range `dc413b56..e3396329`; pre-cutover guest image preserved in Nix store history.
- **Disk:** `data.img.quarantine-1-40e7813a346e3d7a` on Node B (`/var/lib/go-choir/vm-state/candidate-fleet-e15cb89f25d963c220319b7b/`).
- **Journal:** `rec-1-40e7813a346e3d7a.journal`, canonical head `956ab4e4e4a473e2e976e56ea9d3c2195e86b74408f300c83840be64b6fdd21c`.
- **Registry:** revert restores blocked-draft topology; predecessors remain completed non-entrypoints.

## 7. Heresy delta

- **Discovered:** 2 (post-cutover review found fence/migration gaps; boot stall found silent migration + stall detector interaction)
- **Introduced:** 0
- **Repaired:** 2 (OG boundary + write guard + fence coverage; boot-stall repairs + base install marker + fixpoint heartbeat)

## 8. Residual risks and next realism axis

- **Deploy time:** NixOS closure build on node-b takes ~13 minutes; nixos switch ~3 minutes. A binary cache or CI-built closure would cut this to <2 minutes.
- **Test contention:** `internal/store` tests are fsync-bound (each `Open` does many journal commits). `t.Parallel()` was tried and reverted — parallel engines thrash CPU without helping wall time. The tmpfs + shard increase in `e22b99d4` is the current mitigation.
- **Journal growth:** the 1 GiB journal trigger is a heuristic; if write volume spikes, the journal could still grow large between GC runs. Monitoring `journal_gib` in GC dispositions will show if the trigger needs tuning.
- **V1 decode coverage:** the frozen V1 decoder covers all twelve classes, but any new carrier added without inventory coverage will fail the writer-purity gate — this is intentional fail-closed behavior, not a bug.

## 9. What changed in the world

Before: `super`, `co-super`, `researcher` — confusing relative to desk roles, `researcher` implying singularity for a transparently scaling RLM.

After: `management`, `engineering`, `research` — desk names that match the product ontology. The old vocabulary is read-only history, preserved byte-identical on tape and decodable forever through the frozen V1 decoder. The new vocabulary is the only one that can execute or persist.
