# MissionGradient: Search Provider Plane v1

**Status:** checkpoint_incomplete — Gate A green; staging deploy proof pending  
**Date:** 2026-05-26  
**Approved design:** [design-search-provider-plane-v1.md](./design-search-provider-plane-v1.md)  
**Supersedes:** ad-hoc search fixes and prompt-side workarounds in `mission-research-runtime-evidence-cadence-v1.md` (gateway slice)  
**Purpose:** deploy and prove the durable gateway search control plane so researcher `web_search` never returns success-with-zero when eligible providers can answer, and VText cadence can stop compensating for broken retrieval.

---

## One-Line Goal String

```text
/goal Run docs/mission-search-provider-plane-v1.md as a Codex-operated MissionGradient mission: deploy and prove the Search Provider Plane v1 on staging (choir.news). Optimize the real gateway search artifact under its invariants by descending the loss landscape: parallel healthy-provider fan-out, durable provider health with exponential backoff, explicit search_outage when no yield, ops health/reset surfaces, and staging product-path proof that researcher web_search returns merged results (not empty success) under provider stress. Preserve topology—no prompt taxonomy, no VText cadence changes, no mail work. Stop only when acceptance criteria are met with named deploy evidence, or escalate invariant-level blockers.
```

---

## Cognitive Transform Review

### Current Uncertainty Or Obstacle

The approved design exists locally (`internal/gateway/searchplane/`, wired in `search_plane.go`) but **staging still runs the pre-plane gateway** (`GET /provider/v1/search/health` → 404). Node-b already has all five search API keys and **live search returns results** when called with a valid sandbox gateway token—so the loss is not missing keys, it is **missing deployed control-plane behavior** and **downstream compensation** (sequential calls, empty merges, researcher retry spirals).

### Selected Transforms

1. **Infrastructure-before-prompt** — Fix gateway contract first; do not tune researcher cadence until staging `web_search` obeys the plane invariants.
2. **Negative-proof** — Success is absence of forbidden states: no HTTP 200 + `results: []` when eligible providers exist; no HTTP to cooling providers.
3. **Activation boundary** — Separate “code merged” from “behavior live on staging”; `/health` and deploy SHA are the promotion gate.
4. **Stress as first-class axis** — Prove behavior when 2–3 of 5 providers are artificially cooled (strike escalation), not only when all are healthy.

### Route-Changing Insight

The mission optimizes **one artifact** (gateway search plane on staging), not “search feels better.” VText P1 stays blocked until this mission reaches `complete` or documents `blocked_incomplete` with a precise external blocker.

---

## Real Artifact

The artifact is the **deployed gateway search control plane** on Node B, consumed by sandbox VMs via:

```text
sandbox (researcher web_search)
  -> RUNTIME_GATEWAY_URL (http://127.0.0.1:8084)
  -> POST /provider/v1/search
  -> SearchClient.searchplane.Router
  -> parallel provider adapters (tavily, brave, parallel, exa, serper)
  -> SQLite provider health (CHOIR_SEARCH_HEALTH_PATH)
  -> merged results OR structured search_outage (503)
```

**Not in scope for this mission:** VText rewrite, persistent researcher pool, chyron server lines, maild, proxy HMAC (separate missions).

---

## Invariants

```text
I1. Researchers and VText never choose search providers or manage cooldowns.
I2. Eligible providers in cooling_down receive no HTTP until cooldown_until.
I3. Forbidden: HTTP 200 with results=[] when at least one eligible provider was called in the wave.
I4. Forbidden: success-with-zero when eligible providers exist but all return empty without classifying outage.
I5. Provider health survives gateway restart (SQLite under /var/lib/go-choir/gateway/).
I6. Staging is the acceptance environment (https://choir.news); local proof does not substitute.
I7. Problem-documentation-first: document deploy blockers before speculative fixes when staging evidence contradicts belief.
```

---

## Value Criterion (Loss Landscape)

Minimize:

```text
J = w_empty * 1[forbidden empty success]
  + w_outage * 1[search_outage when eligible existed]
  + w_latency * p95_fanout_seconds
  + w_retry * researcher_web_search_calls_per_checkpoint
  + w_strike * mean_strike_count_under_stress
```

**Uphill means:**

- `merged_count >= CHOIR_SEARCH_MIN_MERGED_RESULTS` (default 5) on healthy staging probes
- `waves <= 2`, `attempts` show parallel fan-out (multiple providers per wave, not sequential waterfall)
- `provider_health` visible in search response and `GET /provider/v1/search/health`
- After artificial cooldown of 3 providers, remaining providers still return merged results or explicit `search_outage`
- Health DB persists across `systemctl restart go-choir-gateway`

**Quality target:** `solid` (production-shaped, tested, observable, rollback-safe).

---

## Belief State (Initial)

```text
believed_current_artifact_state:
  - Local implementation complete on branch (searchplane package + handler routes + runtime projection fields).
  - Staging gateway binary predates search plane (health route 404).
  - Staging gateway has valid keys for all five providers (verified on node-b 2026-05-26).
  - Direct POST /provider/v1/search with sandbox token returns multi-provider results today.

evidence_for_belief:
  - docs/design-search-provider-plane-v1.md
  - docs/review-search-vtext-context-2026-05-26.md
  - node-b: curl /health lists search_providers; authenticated search returns hits.

main_uncertainties:
  - Whether CI/deploy will select gateway host-service rebuild on push.
  - Whether any provider keys are rate-limited in production traffic patterns.
  - Exact SQLite path permissions on staging (/var/lib/go-choir/gateway/).

highest_impact_uncertainty:
  Deployed SHA on choir.news matches merge commit and search plane routes are live.

next_observation_that_would_reduce_uncertainty:
  Push -> green CI -> staging deploy -> GET /provider/v1/search/health returns 200.
```

---

## Homotopy Parameters (Optimization Axes)

Increase realism in order. Do not skip a lower λ without evidence.

| λ | Realism added | Proof required |
|---|----------------|----------------|
| **λ0** | Unit + integration tests pass locally | `go test ./internal/gateway/searchplane/... ./internal/gateway/...` |
| **λ1** | Gateway binary on staging exposes health + search routes | `curl` health 200; health JSON has provider states |
| **λ2** | Parallel fan-out visible in attempts | Same-query attempts show 2+ providers, wall time ≈ max(provider), not sum |
| **λ3** | Durable health across restart | Cool provider, restart gateway, provider still cooling |
| **λ4** | Product-path web_search | Trace: no 8× empty web_search; merged_count > 0 on standard probes |
| **λ5** | Stress behavior | Reset 3 providers via ops API, search still yields or outages cleanly |

---

## Implementation Gradient

### Phase A — Land code (pre-deploy)

1. Ensure `internal/gateway/searchplane/` + `search_plane.go` + handler routes + runtime fields match design.
2. Unit tests: backoff doubling, parallel fanout, outage on all-cooling, no empty success.
3. Document any env defaults in mission evidence (do not commit secrets).

**Gate A:** `nix develop -c go test ./internal/gateway/searchplane/... ./internal/gateway/... -short` green.

### Phase B — Deploy to staging

Follow AGENTS.md landing loop:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
  -> verify staging commit identity -> run acceptance proofs below
```

**Gate B:** Staging health shows expected deployed commit matching merge SHA.

**Gate C:** `GET http://127.0.0.1:8084/provider/v1/search/health` (on node-b) returns provider health snapshot.

### Phase C — Staging acceptance probes

Run probes **after** Gate B. Record in evidence ledger.

| Probe ID | Action | Pass condition |
|----------|--------|----------------|
| **P-healthy** | `POST /provider/v1/search` query=`elan definition linguistics`, max_results=10 | `merged_count >= 5`, `len(providers) >= 1`, no `error: search_outage` |
| **P-broad** | Same with `recent advances in protein folding 2025` | `merged_count >= 5` |
| **P-parallel** | Inspect `attempts` for one query | ≥2 distinct providers called in same wave OR documented single-provider eligibility |
| **P-cooldown** | `POST .../health/reset` for `brave`; call search; verify `brave` skipped with `cooling_down` in attempts | No HTTP to brave while cooling |
| **P-restart** | Note strikes for one provider, restart `go-choir-gateway`, re-read health | `strike_count` preserved; `cooldown_until` still honored |
| **P-product** | Full prompt-bar → VText → researcher path on staging (Trace) | No sustained `web_search` with `results: 0`; v2+ appears when research warranted |

**Gate D:** All P-* probes pass on staging with cited trajectory/run ids.

### Phase D — Ops runbook

1. Document `POST /provider/v1/search/health/reset` after tier upgrade (in mission + short ops note in design doc cross-link).
2. Record default paths: `CHOIR_SEARCH_HEALTH_PATH`, `CHOIR_SEARCH_PROVIDERS_PER_QUERY=2`.

---

## Forbidden Shortcuts

```text
F1. Claiming mission complete from local tests only.
F2. Tuning researcher/VText prompts instead of fixing gateway empty-success.
F3. Returning HTTP 200 with empty results when eligible providers were available.
F4. Disabling providers in env instead of using health strikes/cooldowns for stress tests (except documented reset after test).
F5. Proving search via curl to third-party APIs bypassing gateway merge/outage logic.
F6. Expanding scope to maild, proxy HMAC, or VText P1 in the same mission.
F7. Weakening CI path filters to force a green build without host deploy.
F8. Editing tracked files directly on Node B as source of truth.
F9. Calling checkpoint_incomplete "done" or "mission achieved."
```

---

## Verification Commands (Reference)

Local:

```bash
nix develop -c go test ./internal/gateway/searchplane/... ./internal/gateway/... -short
```

Staging (node-b / choir.news):

```bash
# Gateway health includes search_providers
curl -fsS http://127.0.0.1:8084/health | jq .

# Provider health plane (after deploy)
curl -fsS http://127.0.0.1:8084/provider/v1/search/health | jq .

# Authenticated search (use sandbox token from /var/lib/go-choir/sandbox-gateway-token.env)
curl -fsS http://127.0.0.1:8084/provider/v1/search \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"query":"elan definition linguistics","max_results":10}' | jq .

# Reset provider after stress test
curl -fsS -X POST http://127.0.0.1:8084/provider/v1/search/health/reset \
  -H 'Content-Type: application/json' \
  -d '{"provider":"brave"}' | jq .
```

Product-path: use existing Playwright or Trace APIs per `AGENTS.md`; record trajectory_id, submission_id, tool results for P-product.

---

## Stopping Condition

Mission status may be set to **`complete`** only when **all** are true:

1. Merge commit is deployed to staging; health endpoint proves search plane is live.
2. Gates A–D passed with evidence ledger entries (probe ids, commands, pass/fail, ids).
3. At least one **P-product** trace shows researcher `web_search` with `merged_count > 0` OR explicit `search_outage` with full `provider_health` (no silent empty success loop).
4. Mission doc updated with **Run Checkpoint & Resumption State** (below).
5. `design-search-provider-plane-v1.md` acceptance criteria checkboxes updated to reflect deployed truth.

Otherwise: **`checkpoint_incomplete`** (resume with next executable probe) or **`blocked_incomplete`** (name external blocker: CI, DNS, provider outage, human approval).

---

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: 2026-05-26 — Gate A (local tests) green after ratelimit/search-plane test fix
current artifact state:
  - Code: searchplane + gateway routes committed locally (a1040f7); not yet on origin/main or staging.
  - Staging gateway binary still predates search plane (GET /provider/v1/search/health → 404 as of 2026-05-26).
  - Keys: present on staging gateway (node-b, prior session).
what shipped:
  - a1040f7 feat(gateway): deploy Search Provider Plane v1 (local only until push)
  - pending: test fix for SearchClient health store in rate-limit integration test
what was proven:
  - Gate A: nix develop -c go test ./internal/gateway/searchplane/... ./internal/gateway/... -short (pass 2026-05-26)
  - node-b direct POST /provider/v1/search with sandbox token returned hits (pre-plane binary, prior session)
unproven or partial claims:
  - origin/main contains search plane commits
  - CI green for gateway closure
  - Staging /provider/v1/search/health 200 with provider states
  - Parallel fan-out, durable health across restart, ops reset on staging
  - Product-path researcher web_search merged_count > 0 (P-product)
belief-state changes:
  - Gate A was blocked by TestSearchRateLimitDoesNotConsumeInferenceBudget using bare SearchClient without in-memory health store; fixed via testSearchClient().
remaining error field:
  - Deploy lag: code not pushed/deployed; downstream still compensates for missing control plane.
highest-impact remaining uncertainty:
  Deployed SHA on choir.news matches pushed commit and /provider/v1/search/health is live.
next executable probe:
  Push a1040f7 + test fix to origin/main → monitor CI → confirm staging deploy SHA → curl /provider/v1/search/health on node-b (Gate B).
suggested resume goal string:
  /goal Continue docs/mission-search-provider-plane-v1.md: push search plane to origin/main, complete landing loop (CI + staging deploy identity), then run Gates B–D staging probes (P-healthy through P-product). Record evidence in the mission ledger. Do not start VText or mail work. Set status complete only when acceptance criteria met.
evidence artifact refs:
  - docs/omp-session-resume-search-provider-plane-2026-05-26.md
  - Gate A command output (local, 2026-05-26)
rollback refs:
  - Revert deploy via git revert of search plane commit(s) + redeploy previous gateway closure if Gate D fails.
``````

---

## Evidence Ledger (Template)

For each nontrivial claim:

```text
claim:
evidence_source: command | trace | curl | manual observation
command_or_observation:
artifact_path:
result: pass | fail | partial
uncertainty_or_caveat:
supports_promotion: yes | no
```

---

## Relationship To Other Work

| Work | Relationship |
|------|----------------|
| [design-search-provider-plane-v1.md](./design-search-provider-plane-v1.md) | Canonical architecture (this mission implements and proves it) |
| [design-vtext-platform-v3.md](./design-vtext-platform-v3.md) | **Blocked until** this mission completes or explicitly defers with blocker |
| [mission-research-runtime-evidence-cadence-v1.md](./mission-research-runtime-evidence-cadence-v1.md) | Downstream consumer of gateway projections; do not duplicate gateway policy here |
| [mission-vtext-lineage-aware-runtime-cadence-v2.md](./mission-vtext-lineage-aware-runtime-cadence-v2.md) | Do not start VText cadence mission work until search plane is live |

---

## Resume Prompt (Copy-Paste)

For a fresh agent session:

```text
/goal Continue docs/mission-search-provider-plane-v1.md as a Codex-operated MissionGradient mission. Read Run Checkpoint & Resumption State and Evidence Ledger first. Gate A is green locally. Execute landing loop: push origin/main → monitor CI → verify staging deploy SHA → run Gates B–D (curl probes P-healthy..P-restart on node-b, then P-product via product path). Forbidden: prompt/VText fixes, bypassing gateway merge, claiming staging from local tests only. Update mission checkpoint and status (complete | checkpoint_incomplete | blocked_incomplete) with cited evidence before stopping.
```
