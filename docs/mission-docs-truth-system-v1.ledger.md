# Docs Truth System v1 Ledger

## 2026-06-14 - paradoc opened

- Claim: v0 doccheck should not grow into a pile of warning regexes; the next
  useful object is a docs truth system with mission graph, assertion register
  wiring, focal docs spine, and code/docs heresy checking.
- Move: created `docs/mission-docs-truth-system-v1.md` as the successor
  paradoc to the broad docs-truth v0 handoff and the narrower heresy checker
  specs.
- Owner premise: making a new paradoc should update the mission graph.
- Expected ΔV: 0 against implementation obligations; this creates the source
  program for the next implementation pass.
- Actual ΔV: 0.

## 2026-06-14 - graph/register/code-surface checker implemented

- Claim tested: doccheck can become a docs truth system slice without turning
  baseline heresy counts into a blocking CI gate.
- Move: added `docs/mission-graph.yaml`, updated Parallax skill guidance in
  both repo and live skill copies, wired README/docs index to the focal truth
  spine, manifested the assertion ledger, and extended `cmd/doccheck` with
  mission graph validation, assertion-register validation, and typed code/docs
  heresy scan output.
- Evidence: `go test ./cmd/doccheck` passed; YAML parse checks passed for
  `docs/doc-authority-manifest.yaml` and `docs/mission-graph.yaml`;
  `.github/scripts/deploy-impact-classify-test` passed; docs/checker paths
  remain report-only, but the requested repo Parallax skill copy under
  `skills/` currently classifies as sandbox host-service impact;
  `scripts/doccheck` exited 0 with 202 docs, 803 warnings, 4622ms runtime.
- Report evidence: mission graph has 13 nodes and 13 dependency edges;
  assertion register has A1-A6, I1-I5, and E1-E4; code/docs scan covers 725
  files, 10 detector families, 53 detector terms, and 2413 findings split by
  typed context.
- Report-only boundary: no CI failure gate was added for detector findings.
  Existing warning count remains discovery-only.
- Expected ΔV: 8 by discharging graph seed, Parallax integration, focal docs
  spine, graph schema validation, structured detector projection, code/docs
  scan, assertion register validation, and report-only policy.
- Actual ΔV: 8. Remaining V=0; v1 settled. Successor work should review typed
  allow contexts and decide whether fail-on-introduced semantics are safe.

## 2026-06-14 - Mission Corpus Graph Expansion Note

- Claim tested: the settled docs truth system can absorb the mission-corpus
  indexing update without reopening v1 or turning historical MissionGradient
  reports into current doctrine.
- Move: expanded `docs/mission-graph.yaml` from a seed graph into a full
  mission-shaped-doc index and recorded a post-settlement factual-drift note in
  the v1 paradoc.
- Evidence: `go run ./cmd/doccheck` reports 71 mission graph nodes, 14
  dependency edges, zero ungraphed mission-shaped docs, and no R1/R5/R6/R7
  structural warnings.
- Expected ΔV: 0 for v1 settlement; this is factual drift repair and successor
  infrastructure.
- Actual ΔV: 0. v1 remains settled.
