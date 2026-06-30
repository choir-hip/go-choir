---
name: beads-missions
description: How to track Choir missions in beads (bd). Mission state lives in
  beads now (shadow mode): epic = mission, sub-issue = conjecture. Use when
  creating, advancing, settling, or querying missions instead of hand-editing
  docs/mission-graph.yaml.
---

# Beads for Choir Missions

Mission state is migrating to beads (`bd`). Store: `.beads/` (embedded Dolt,
prefix `choir`); `.beads/issues.jsonl` is the git-committed export. We are in
SHADOW MODE: `docs/mission-graph.yaml` is still authored, and
`go run ./cmd/mgimport` re-syncs YAML -> beads idempotently. Until the flip
(epic C6), edit the YAML for graph structure; use beads for live state/queries.

## The model
- **epic = mission**, **sub-issue (task, `--parent`) = conjecture**.
- A bead links to its paradoc/ledger in its description; it does NOT transcribe
  them (no double-ledger). Narrative stays in the paradoc; the move-log goes in
  bead notes; durable learning graduates to a doc.
- **Variant V** = count of open child conjectures under an epic. Never hand-typed.

## Start of session
    bd ready              # the frontier: unblocked, actionable missions
    bd ready --json       # same, for parsing
    bd show <id>          # full mission/conjecture state
    bd list               # active (open + in_progress); closed are hidden

## Create
    bd create "Title" -t epic  -p 1 -l mission,kind:spine -d "desc + paradoc/ledger paths" --silent
    bd create "C1: ..."  -t task --parent choir-XXX -p 1 -l conjecture -d "..." --silent
Link a bead to its graph node: `--external-ref mg:<mission-id>`.

## Dependencies (the DAG)
    bd dep add <issue> --blocked-by <blocker>   # 'issue depends on blocker'
Cycles are rejected automatically. `bd ready` only surfaces beads with all
blockers closed.

## Advance
    bd update <id> --claim                       # assign + in_progress
    bd update <id> --status in_progress
    bd update <id> --append-notes "PASS n / Move: probe|shift|construct|settle / Verdict: ... / dV: ... / Receipt: <sha|trace>"
    bd label add <id> <label>                    # NOTE arg order: <id> THEN <label>

## Status / verdict vocab (frozen - C4)
Mission status -> beads:
  planned->open . working->in_progress . open_handoff->in_progress + label `handoff`
  blocked->open + label `blocked` . settled->close reason "settled: ..."
  superseded->close reason "superseded: ..." + label `superseded`
Conjecture verdict = close-reason prefix: `supported|weakened|falsified|superseded`.
`discovered` is NOT a close - create a new sub-issue with a `discovered-from` edge
(this RAISES V; that's progress, not failure).
Labels: `kind:<spine|side|docs_truth|evidence|superseded>`, `mc:<green..black>`.

## Settle (the residue gate - DO NOT skip)
Before closing an epic:
  G1 descent: V=0 (all children closed) OR remaining children superseded/handed-off.
  G2 residue: the close must leave durable learning that RESOLVES - an
     assertion-register entry (A*/I*/E*), an architecture/doctrine doc edit, or
     (for supported) a merged commit AUTO-LINKED to an assertion. A ledger entry
     or "see the bead" does NOT count. Falsified/weakened MUST leave an evidence
     doc. Superseded carries its residue obligation forward to the successor.
    bd close <id> -r "supported: <strong definitive statement> | receipt: <sha>" --suggest-next
Only after the epic is closed AND its residue links resolve is its
`docs/mission-*.md` projection safe to delete.

## Re-sync from the YAML (shadow mode)
    go run ./cmd/mgimport     # idempotent; re-running yields delta 0
