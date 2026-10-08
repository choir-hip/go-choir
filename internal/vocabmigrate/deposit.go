package vocabmigrate

// Versioned projection-deposit upcasting (mission-2 residual: cutover-aware
// replay; owner-authorized recovery policy 2026-10-07).
//
// The canonical tape is immutable: it records what each writer wrote under the
// vocabulary active at write time. Replaying it byte-identically into a fresh
// projection deposits both V1-spelled (pre-cutover) and V2-spelled
// (post-cutover) rows for the same logical objects; the end-of-replay
// migration then has to rename stale V1 rows onto rows their own successors
// already hold — a convergence problem with no safe general resolution
// (2026-10-07 rebuild defects 3 and 4).
//
// DepositUpcastVersion names the frozen transformation that instead applies
// the vocabulary mapping to the VERIFIED projection-deposit view as it is
// deposited, in canonical event order. The original event, receipt, payload
// digest and reducer commitments are verified first and never rewritten; only
// the projection deposit is transformed. The transformation is deterministic
// (frozen tables and identity formulas only — no timestamps, no free-text
// rewriting, no wall-clock last-writer-wins) and idempotent: V2-spelled
// deposits pass through byte-identically.
//
// The store records which version transformed a store's deposits in the
// workspace sidecar `vocab-deposit-upcast.jsonl` and mirrors the version into
// MigrationReport.DepositUpcastVersion, so consumers of a published base can
// see the compatibility transformation that is already applied.
//
// Version history:
//
//	1 — initial. Desk tokens map through ForwardV1ToV2; desk-bearing IDs
//	    through the frozen prefix/infix rules; object identity re-derived
//	    from the verified per-kind identity formulas; embedded refs, edge
//	    endpoints and version/superseded columns rewrite through the deposit
//	    alias ledger; keys and frozen protocol values never move. The SQL role
//	    tables (agents, runs, channel_messages, inbox_deliveries, work_items,
//	    worker_updates) are not projection-deposited tables, so they stay
//	    owned by MigrateAndFenceServingVocabulary's one-time migration for
//	    retained V1 stores.
const DepositUpcastVersion = 1
