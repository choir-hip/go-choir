# RLM Versioned Rename Deployed Proof and Telemetry — 2026-09-11

Class: green (evidence only).
Authority: `docs/definitions/choir-rlm-versioned-rename-2026-09-09.md` (acceptance items 8 and 15).

## 1. Deployment and Environment Identity

- **Pushed Commits:**
  - `e3396329` red(cutover): base install marker normalization, safe head verify, and fixpoint heartbeat
  - `05b109a5` red(cutover): boot-stall repairs — fenced fast path, gate progress heartbeat, author_label leaf fix, unbounded blob download
  - `2366d7b8` red(cutover): stream OG migration scan — retain only migrated or ref-bearing rows
  - `ffdc2d85` red(cutover): unify multimodal verifier spelling; pin fail-closed default policy
  - `742fd4d3` red(cutover): channel-cast role canonicalize-or-refuse + OG write validator hook
  - `1e96e0b2` red(cutover): object-graph vocabulary boundary — in-place migration, write guard, fence coverage
  - `80dea87e` red(cutover): migration correctness — longest-first compound ID ordering, crash-atomic provenance, run_memory_entries carrier
  - `a14abf05` red(cutover): writer cutover — V2 canonical desks vocabulary, V1 decode seam, writer purity
  - `1f51aaf5` red(cutover): writer purity gate, decode-roots guard, and frozen V1 corpus manifest
  - `7cee94d5` red(cutover): vocabulary migration and serving fence wiring
  - `cb571960` red(cutover): computer-owned model-policy TOML V1 decode seam and V2 cutover
- **CI Status:**
  - Workflow: GitHub Actions CI run `34571343061`
  - Result: `completed success`
  - All CI gates passed: Plan CI Lanes, Docs Truth Check, Detect Staging Deploy Impact, Go Vet + Build, Vocabulary Gates (V1 writer-purity gate, decode-roots guard), Heresy Detector, Build Differential SBOM Candidate, all 6 `agentcore/textureowner` shards (race detector), all 6 `non-runtime` shards (race detector), isolated scale tests, Accept Differential SBOMs, Publish Rolling Flake, and Deploy to Staging (Node B).
- **Staging Verification:**
  - URL: `https://choir.news/health`
  - Response: `HTTP/2 200 OK`
  - Payload:
    ```json
    {
      "status": "ok",
      "service": "proxy",
      "upstream": "vmctl",
      "vmctl_routing": "enabled",
      "vmctl_status": "ok",
      "build": {
        "service": "proxy",
        "version": "0.1.0",
        "commit": "e3396329bb77e2c8835a530a2923b6b489e61f74",
        "deployed_at": "2026-09-11T07:14:19Z",
        "deployed_commit": "e3396329bb77e2c8835a530a2923b6b489e61f74"
      }
    }
    ```
- **Target Computer Identity (Staging Owner Computer):**
  - Computer: `computer-03335285269bdba4f94377e56879f9e6`
  - VM: `candidate-fleet-e15cb89f25d963c220319b7b`
  - Realization Epoch: 909
  - State: `active`
  - Service: `autoputer`
  - Runtime Health: `ready`
  - Effects Mode: OFF (pre-A checkpoint `99949fe2` remains untouched as self-development fence).
  - Guest Endpoint Health: `http://10.200.3.2:8085/health` returns `status: "ready"`, `runtime_health: "ready"`, `active_provider: "gateway"`, `researcher_count: 3`.

## 2. Versioned Rename & Cutover Acceptance Scenarios

| Scenario | Subsystem | Observed Result | Contract Satisfied |
|---|---|---|---|
| **R1: V1 Field Inventory** | `docs/evidence` | 12-class frozen inventory published (`choir-rlm-v1-inventory-corpus-2026-09-10.txt`). Every role carrier anchored to source file and line. | Rename scope bounded by evidence, not assumption. |
| **R2: Version-Selected Decode** | `computerevent`, `agentprofile` | V1 decode seam carries frozen V1 vocabulary; tape bytes preserved byte-identical under replay. | Historic tape never mutates; V1 rows decode deterministically. |
| **R3: Live Writer Cutover** | `agentprofile`, `capsule`, `promptstore` | All live writers emit only V2 desk vocabulary (`management`, `engineering`, `research`). Rejects legacy tokens. | New truth is written once, in one vocabulary. |
| **R4: Frozen Mapping Table** | `docs/evidence` | Mapping table frozen (`choir-rlm-v2-mapping-2026-09-10.md`). Forward and inverse maps defined for all role tokens. | Every equivalence written down and verified. |
| **R5: Unknown Live Refusal** | `agentprofile`, `modelpolicy` | Live requests with unknown or legacy role tokens (`super`, `co-super`) return typed error (`unknown role`). V2 tokens (`management`, `engineering`, `research`) resolve. | The unnamed can never execute or persist. |
| **R6: Live Alias Retirement** | `agentprofile`, `toolregistry` | General canonicalizers refuse legacy aliases on live path. Aliases survive only in frozen V1 decoder. | One vocabulary is live; old vocabulary is read-only history. |
| **R7: Engineering Desk End-to-End** | `agentprofile`, `modelpolicy` | Live resolver maps `engineering` to `deepseek/deepseek-v4-flash` via model policy TOML. | First RLM desk speaks new vocabulary without carrier rewrite. |
| **R8: Decoder Matrix on Staging** | `projectionbase`, `computerevent` | Decoder matrix published and exercised (`docs/evidence/choir-rlm-decoder-matrix-2026-09-10.md`). | Vocabulary seam holds on staging computer. |
| **R9: Adjacent Behavior Preservation** | `agentcore`, `store` | Focused contracts pass without regression: replay byte-identity, writer purity, activation refusal, grant attestation. | Adjacent behavior preserved across touched contracts. |
| **R10: Carrier Coverage (Object Graph)** | `store` | In-place migration of `og_objects` and `og_edges` with bounded fixpoint ID rewrite; serving fence covers all role-bearing fields; write guard active. | No unmigrated row holds live authority on serving carrier. |
| **R11: Historic Reader Compatibility** | `computerevent`, `store` | Historic events with V1 tokens remain verifiable and readable without mutating tape. | Pre-cutover records remain valid forever. |
| **R12: Write Root Closure** | `agentcore` | Channel-cast and management spawn validate role tokens against version-selected vocabulary before writing. | Every live writer emits one vocabulary. |
| **R13: Migration Correctness & Crash-Consistency** | `store` | Longest-first ID substitution; provenance report persisted before row mutation. Provenance report stamped with `FencedAt`. | Migration is crash-consistent and exactly invertible. |
| **R14: Verifier Role Spelling** | `agentprofile`, `modelpolicy` | Verifier role spellings unified across live vocabulary, policy keys, and fence acceptors; empty profile fails closed. | Resolved role selection matches configured policy. |
| **R15: Staging Proof & Record Closure** | `platform`, `vmctl` | Staging owner computer boots into ready state via `RecoveryResume` (local=148566 W=148431 H=148566 tail=0); fast-path cold reboot takes 29 seconds. | Deployed proof paid on physical staging computer. |

## 3. Live Staging Probes (2026-09-11)

1. **Proxy Health:**
   ```text
   GET https://choir.news/health
   HTTP/2 200 OK
   {"status":"ok","service":"proxy","upstream":"vmctl","vmctl_routing":"enabled","vmctl_status":"ok","build":{"commit":"e3396329bb77e2c8835a530a2923b6b489e61f74"}}
   ```
2. **Owner Computer VMCTL Resolution:**
   ```text
   GET http://127.0.0.1:8083/internal/vmctl/lookup?computer_id=computer-03335285269bdba4f94377e56879f9e6
   {"vm_id":"candidate-fleet-e15cb89f25d963c220319b7b","computer_id":"computer-03335285269bdba4f94377e56879f9e6","user_id":"5bd6de97-3b58-408c-bf89-c42c81b083de","desktop_id":"primary","state":"active","epoch":909}
   ```
3. **Guest Runtime Health:**
   ```text
   GET http://10.200.3.2:8085/health
   {"status":"ready","service":"autoputer","computer_id":"computer-03335285269bdba4f94377e56879f9e6","runtime_health":"ready","researcher_count":3,"active_provider":"gateway"}
   ```
4. **Model Policy V2 Role Resolution:**
   - `engineering`: returns `200 OK`, `provider: "deepseek"`, `model: "deepseek-v4-flash"`
   - `management`: returns `200 OK`, `provider: "chatgpt"`, `model: "gpt-5.6-luna"`
   - `research`: returns `200 OK`, `provider: "chatgpt"`, `model: "gpt-5.6-luna"`
   - `super` (legacy): returns `400 Bad Request`, `{"error":"unknown role"}`
   - `co-super` (legacy): returns `400 Bad Request`, `{"error":"unknown role"}`
5. **Fast-Path Reboot Timing:**
   - Microvm stop: `{"status":"stopped"}`
   - Microvm resolve (start): cold boot to active in **29.25 seconds** (skipping 12-minute initial scan via `FencedAt`).

## 4. Rollback References

- Retained disk rollback ref: `data.img.quarantine-1-40e7813a346e3d7a` on Node B (`/var/lib/go-choir/vm-state/candidate-fleet-e15cb89f25d963c220319b7b/`).
- Journal: `rec-1-40e7813a346e3d7a.journal`, canonical head `956ab4e4e4a473e2e976e56ea9d3c2195e86b74408f300c83840be64b6fdd21c`.
- Source rollback: `git revert` of cutover commit range; pre-cutover guest image preserved in Nix store history.
