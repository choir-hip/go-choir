# M0a — Research Desk Capability-Parity Checklist

Date: 2026-09-30
Mission: [`choir-signal-research-rlm-cutover-2026-09-29`](../definitions/choir-signal-research-rlm-cutover-2026-09-29.md)
Purpose: frozen acceptance baseline for the one-tool cutover. Every
model-facing tool deleted from the research registry must either (a) be
reachable as an in-cell `choir.*` verb, or (b) carry an explicit drop reason
recorded here. A tool deleted with neither is a silent capability regression
— the Goodhart the mission names.

Current research registry (`buildDeskCellRegistry` + `researchtools.Register`
+ `RegisterEvidenceTools`/`RegisterRunMemoryTools`): `desk_go_eval` plus the
tools below. Target registry: `{desk_go_eval}` only; capability moves to
`choir.*` verbs bound on the Go cell.

## Deletion set → verb mapping

| # | Deleted tool | choir.* verb / disposition | Notes |
|---|--------------|----------------------------|-------|
| 1 | `web_search` | `choir.WebSearch(query)` | phase 1 verb; egress-metered |
| 2 | `source_search` | `choir.SourceSearch(query)` | phase 1 verb; Source Service ledger |
| 3 | `fetch_url` | `choir.FetchURL(url)` | phase 1 verb; byte-metered egress |
| 4 | `import_document_content` | `choir.ImportDocument(url_or_path)` | phase 1 verb; ContentItem substrate |
| 5 | `import_url_content` | `choir.ImportURL(url)` | phase 1 verb; ContentItem substrate |
| 6 | `read_content_item` | `choir.ReadContentItem(content_id)` | phase 1 verb; untrusted source text |
| 7 | `list_content_item_selectors` | `choir.ListContentSelectors(content_id)` | phase 1 verb |
| 8 | `read_content_item_selector` | `choir.ReadContentSelector(content_id, selector)` | phase 1 verb |
| 9 | `search_wire_corpus` | `choir.SearchWireCorpus(query)` | phase 1 verb; wire corpus |
| 10 | `save_evidence` | `choir.SaveEvidence(item)` | verb; evidence ledger write |
| 11 | `read_evidence` | `choir.ReadEvidence(ref)` | verb; evidence ledger read |
| 12 | `list_evidence` | `choir.ListEvidence(filter)` | verb; evidence ledger list |
| 13 | `get_run_memory_entry` | `choir.RunMemoryEntry(id)` | verb; run-memory read |
| 14 | run-memory tools (list/search) | `choir.RunMemory(filter)` | verb; run-memory enumeration |

## Governance that must move, not vanish

- **EgressBudgetLedger** (64 calls / 32MiB per activation) meters tools
  1–9 today via `deps.chargeEgressCall` / `chargeEgressBytes`. On cutover
  the budget must charge at the **broker/host layer** on the new verbs —
  `Acceptance`: "a cell that exceeds the rehomed egress budget gets a
  budget error returned into the cell". A verb that bypasses the ledger
  is a governance regression.
- **Prompt overlays** `rlm_research_runtime.yaml` + `research.yaml`
  instruct the deleted cadence; rewritten to describe only the Go-module
  surface.

## Acceptance proof mapping

- Multi-search loop in one `desk_go_eval` cell (search→emit→search→emit)
  — `deployed proof`.
- This checklist verified: every row resolved verb-side or drop-reason —
  `static analysis + deployed proof`.
- Egress refusal returns a budget error into the cell — `deployed proof`.
- Post-deletion deployed tool schema = exactly `{desk_go_eval}`; active
  overlays reference only retained in-cell capabilities —
  `deployed proof + static analysis`.

## Open items before the deletion commit

- The 5 legacy capsule ops (`capsule_exec`, `capsule_read_file`,
  `capsule_write_file`, `capsule_list_dir`, `capsule_go_eval`) belong to the
  *engineering* carrier, not research — tracked under the carrier mission,
  not this checklist. Research uses `desk_go_eval`.
- `cancel_agent` is already dropped (typed) — out of scope.
