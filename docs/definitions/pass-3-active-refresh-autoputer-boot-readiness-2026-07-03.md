# Definition: Pass 3 — Active Refresh / Autoputer Boot Readiness

**Status:** working  
**Date:** 2026-07-03  
**Governed by:** `definitions` skill, super definition `docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md`  
**Mutation class:** red for implementation; yellow for this definition document  
**Protected surfaces:** VM lifecycle, deploy routing, active computer route identity, guest runtime health, staging acceptance  
**Next execution:** investigate the refreshed active guest health failure before any rename, Nucleus, or promotion-encoding work.

---

## Harness Invocation Semantics

```text
/goal docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md
```

Read this child definition as the current Pass 3 authority under the suite super definition. Execute until the completion semantics below are satisfied with named evidence, or until a sharply evidenced blocker/supersession condition is met.

---

## Source Authority Order

1. Super definition: `docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md`
2. This Pass 3 child definition
3. Suite paradoc: `docs/mission-suite-autoputer-autopaper-spec-first-v0.md`
4. Ledger: `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md`
5. Product ontology: `docs/computer-ontology.md`
6. Repo contract: `AGENTS.md`
7. Product doctrine: `docs/agent-product-doctrine.md`
8. Prior child definition: `docs/definitions/pass-2-completion-definition-2026-07-03.md`
9. Codex review: `docs/reviews/promotion-gate-codex-review-2026-07-03.md`

When this file conflicts with the super definition, the super definition governs. When this file is silent, the suite paradoc governs.

---

## Active Definition Node

```yaml
id: pass-3-active-refresh-boot-readiness
kind: mission
status: testing
source: observed
term: active refreshed guest boot readiness
definition: A deployed ordinary guest image can refresh every active interactive computer, preserve its persistent data, boot through the VM manager, start the sandbox/autoputer runtime, and answer HTTP 200 on `/health` at the guest tap IP on port 8085 before the deploy refresh timeout.
non_definition:
  - Host services healthy is not active guest readiness.
  - A guest reaching a login prompt is not HTTP health readiness.
  - `go-choir-sandbox.service` starting in systemd is not enough if `/health` does not respond.
  - Treating active refresh as diagnostic in CI is not a product fix.
observables:
  - `Deploy to Staging (Node B)` logs for ordinary guest image deploys.
  - vmctl `/internal/vmctl/list` and `/health` state counts.
  - VM manager logs around `RefreshVMForDesktop`, `BootVM`, and `waitForGuestReady`.
  - Guest serial/systemd logs showing network, persistent mount, runtime install, runtime start, and `/health` binding.
  - Direct host probe `curl http://<guest-tap-ip>:8085/health` during readiness window.
execution_effect:
  - Mission C rename/Nucleus work may not claim boot readiness until this node settles.
  - Promotion encoding remains behind Codex reservations and must not use the current refresh failure as proof of the promotion path.
  - Deployment may continue to treat active refresh as diagnostic, but the suite cannot claim staging autoputer proof until active refresh is green.
settlement:
  rule: A behavior-changing commit deploys to staging, refreshes active interactive computers, and proves every refreshed guest answers `/health` with the deployed commit on port 8085; or the current failure is refuted/reframed with stronger evidence and the super definition is updated.
  settled_by: deployed evidence
  invalidation_triggers:
    - A later ordinary guest deploy again leaves an active interactive computer failed or unreachable on `/health`.
    - Host health passes but refreshed guest health fails or times out.
```

---

## Red-Class Ceremony For Implementation

```yaml
conjecture_delta:
  discovered:
    - C-C1/C-C2 now include a concrete deploy-time active refresh failure: active user computer `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` did not answer `/health` at `http://10.201.119.2:8085` within `3m0s`.
    - Universal wire platform computer recovery also failed in the same deploy window at `http://10.201.118.2:8085`.
  introduced: []
  repaired: []
protected_surfaces:
  - VM lifecycle refresh/recover/boot path in `internal/vmctl` and `internal/vmmanager`.
  - NixOS guest boot image in `nix/sandbox-vm.nix`.
  - Node B deploy active-refresh flow in `.github/workflows/ci.yml`.
  - Active computer route identity and rollback semantics.
admissible_evidence_class:
  - Deployed staging proof from GitHub Actions deploy job.
  - Host-side vmctl/proxy diagnostics during refresh.
  - Guest serial/systemd evidence from the failed boot window.
  - Focused Go tests for any lifecycle contract changed.
rollback_path:
  - Revert any code/config commit with `git revert`.
  - For deploy breakage, revert to the previous staging SHA and rerun deploy.
  - Preserve previous active computer route records; do not delete persistent data images as a first response.
heresy_delta:
  discovered:
    - A guest can reach systemd multi-user/login while still failing the product health contract on `:8085`.
    - At least one boot in the same window entered emergency mode with root locked, so boot readiness and HTTP readiness are distinct failure surfaces.
  introduced: []
  repaired: []
```

---

## Determined State

```yaml
determined_state:
  settled:
    - claim: Pass 2 is closed; PR #42 is merged and should not be re-merged.
      source: docs/definitions/pass-2-completion-definition-2026-07-03.md
      execution_effect: Start Pass 3 from active refresh boot readiness.
    - claim: The prior Nix source-filter failure was repaired by commit `02fa2ea6603b7f157c982e9da637ec714301c6bf`.
      source: CI run 28683693425
      execution_effect: Do not patch the old package-source symptom again.
    - claim: CI run `28684139979` is green for commit `8e694f4663412c1a33fc70e870f225f2510718f2`.
      source: GitHub Actions
      execution_effect: Main is not currently red from Pass 2.
    - claim: Deploy job `85072352680` failed active refresh after host services were deployed and healthy.
      source: GitHub Actions job logs
      execution_effect: The next work must target guest boot/readiness evidence, not host service health.
    - claim: During the failed deploy, `go-choir-sandbox.service` reached `Started` for one guest, but `/health` still did not answer before timeout.
      source: deploy diagnostics in artifact/job logs
      execution_effect: The root cause may be runtime bind/listen, network reachability, blocking startup after systemd service start, or health handler response, not merely systemd unit start.
    - claim: Another VM in the same deploy window entered emergency mode with root locked.
      source: deploy diagnostics in artifact/job logs
      execution_effect: Persistent disk/mount or boot dependency failure remains a live hypothesis.
    - claim: The first confirmed root cause is an evidence collapse in `internal/vmmanager`: `waitForGuestReady` only preserved a boolean guest-health result, so deploy logs could not distinguish HTTP non-200 health, response body, TCP timeout, or connect failure.
      source: observed code in `internal/vmmanager/manager.go`
      execution_effect: Add readiness diagnostics before choosing a product boot fix.
    - claim: The diagnostic patch was deployed to staging at commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`.
      source: CI run `28685279292`, deploy job `85076877932`, staging `/health`
      execution_effect: Do not re-land the diagnostic patch; use its deployed evidence surface for the next active-refresh probe.
    - claim: Deploy job `85076877932` did not exercise active interactive computer refresh because vmctl reported `active_vms: 0` and "No active interactive computers need refresh".
      source: GitHub Actions deploy log and staging `/health`
      execution_effect: Diagnostic sufficiency remains unproven for the active-refresh failure path; the next probe must create or observe an active computer before refresh.
    - claim: Staging proxy health reports deployed commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`, while `/health/ready` is degraded for runtime/dolt/ollama.
      source: `https://choir.news/health` and `https://choir.news/health/ready`
      execution_effect: Host proxy deploy identity is verified, but product runtime readiness is still not a Pass 3 completion proof.
    - claim: Product-path activation was attempted from the harness browser, but the session was signed out and the only available activation path required creating or using a passkey.
      source: headless browser observation of `https://choir.news` after opening Desk -> Sign in; no auth cookies or authenticated local/session storage were present.
      execution_effect: Do not create a production/staging user account or passkey as an implicit side effect of the dry run; record the external auth boundary and keep Pass 3 open.
  contested: []
  open:
    - node: root-cause-active-refresh-health
      missing: Confirm whether the active guest fails because the runtime never listens, listens on the wrong interface, blocks inside startup, returns non-200 health, loses network route, or stalls on persistent data.
    - node: current-node-b-state
      missing: Confirm whether Node B currently has failed ownership records or whether later deploys recovered them.
    - node: diagnostic-sufficiency
      missing: Focused tests now cover last-probe HTTP status/body preservation, and the patch is deployed, but diagnostic-sufficiency is not settled until a staging deploy exercises active interactive computer refresh with at least one active computer.
    - node: product-path-active-computer-access
      missing: An authenticated staging product session or explicit approval to create a disposable passkey-backed staging user, so the harness can create/observe an active interactive computer before an ordinary guest deploy.
```

---

## Root Cause Investigation Contract

No fix may land before at least one hypothesis is confirmed or falsified with evidence.

Current symptoms:

```text
Refresh failed for vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19 user 5bd6de97-3b58-408c-bf89-c42c81b083de desktop primary: 404
response: {"error":"failed to refresh VM ... wait for guest ready ... guest did not become healthy at http://10.201.119.2:8085 within 3m0s"}
```

Observed adjacent evidence:

```text
[ OK ] Started go-choir Sandbox Runtime (VM guest).
[ OK ] Reached target Multi-User System.
go-choir-sandbox login:
vmctl: universal wire platform computer ... guest did not become healthy at http://10.201.118.2:8085 within 3m0s
You are in emergency mode ... Cannot open access to console, the root account is locked.
proxy WS relay sandbox->client ... 10.201.117.2:8085: read: connection timed out
```

Hypotheses to test, in order:

1. **Runtime-listen hypothesis:** the sandbox process starts but never reaches `server.Start()` or never binds `0.0.0.0:8085`.
   - Fast falsifier: guest serial logs contain `sandbox: starting server on 0.0.0.0:8085` before timeout and direct host curl returns non-200.
   - Probe: add/inspect startup logs around runtime construction, route registration, `rt.Start`, and listener bind.

2. **Persistent-data hypothesis:** preserved `/mnt/persistent` state or runtime DB blocks startup/health for existing active computers, while fresh guests may work.
   - Fast falsifier: a fresh VM with empty data image also fails the same way.
   - Probe: compare active refresh, universal wire platform recovery, and any fresh test computer boot.

3. **Guest-network hypothesis:** the guest reaches multi-user but host-to-guest tap routing/firewall cannot reach `10.201.*.2:8085` after refresh.
   - Fast falsifier: host can connect to another guest endpoint on the same tap, or guest logs show accepted `/health` requests.
   - Probe: dump host route/iptables rules for the VM, guest assigned IP, and direct host curl timing.

4. **Health-response hypothesis:** `/health` is reachable but returns non-200 because runtime health is `failed`.
   - Fast falsifier: deploy log currently says timeout rather than HTTP 503; direct host curl with status/body decides it.
   - Probe: capture curl status and body during readiness instead of boolean `probeGuestHealth` only.

5. **Emergency-mode hypothesis:** one or more guests enter emergency mode due to a mount or dependency failure, preventing runtime health.
   - Fast falsifier: active target VM serial logs show clean multi-user with no emergency state and no failed units.
   - Probe: include per-VM serial identity and failed unit summary in deploy diagnostics.

---

## Implementation Boundaries

Allowed without further human approval:

- Add diagnostic logging or structured deploy evidence that does not change active route semantics.
- Add focused tests for vmctl/vmmanager refresh state transitions and readiness semantics.
- Fix a confirmed boot/readiness root cause if the patch is contained to VM lifecycle, guest Nix config, or deploy diagnostics and preserves route rollback.
- Update this definition, the super definition, paradoc, and ledger with observed evidence.

Requires human approval before mutation:

- Resetting or deleting active user persistent data images.
- Changing route identity semantics or promotion/rollback semantics.
- Weakening the staging proof requirement for suite completion.
- Encoding promotion certificate/approval behavior before Codex reservations are addressed.
- Introducing compatibility shims or dual runtime paths.

---

## Next Operators

1. `probe(current-node-b-state)` — if credentials/tools allow, collect current vmctl health/list state and confirm whether failed ownerships remain.
2. `probe(root-cause-active-refresh-health)` — inspect the code path from deploy refresh → vmctl handler → ownership registry → vmmanager refresh → guest Nix boot → runtime health.
3. `construct(diagnostic-sufficiency)` — if direct staging access is unavailable, add deploy diagnostics that capture the missing evidence on the next ordinary guest deploy.
4. `verify(diagnostic-sufficiency)` — run focused tests for any diagnostic helper or lifecycle change.
5. `construct(root-cause-fix)` — only after a hypothesis is confirmed.
6. `verify(staging-active-refresh)` — push, monitor CI/deploy, and require refreshed guest `/health` proof before settling C-C1/C-C2.
7. `settle(pass-3-active-refresh-boot-readiness)` — update super definition, paradoc, and ledger with the supported/refuted status.

---

## Completion Semantics

Pass 3 is COMPLETE when:

1. The active refresh root cause is identified with direct evidence.
2. A fix or explicit non-code remediation is landed on `main`.
3. Focused tests cover the changed lifecycle/readiness behavior where the repo can model it.
4. Main CI is green.
5. A behavior-changing deploy to staging refreshes active interactive computers and each refreshed guest answers `/health` on `:8085` with the deployed commit.
6. The suite ledger, paradoc, super definition, and this child definition are updated.

Pass 3 is BLOCKED when:

1. Current Node B or GitHub deploy evidence needed to distinguish the hypotheses is unreachable from available tools, and no diagnostic-only patch can safely improve the next deploy.
2. Fixing the confirmed root cause requires deleting/resetting active user data without human approval.
3. The root cause requires a product ontology or promotion/route authority change.

Pass 3 is IN PROGRESS when:

1. At least one boot-readiness hypothesis is being tested or instrumented.
2. Main remains green.
3. The next executable probe is recorded in this document.

---

## Forbidden Collapses

- Guest image built → active computer boots.
- systemd service started → HTTP health ready.
- Host health green → user computer healthy.
- Diagnostic CI waiver → product issue fixed.
- Emergency mode observed in one VM → every VM has the same root cause.
- Active refresh failed → promotion path is invalidated.
- Local `go build` green → Nix/Firecracker deploy path proven.

- Signed-out preview visible → product-path active computer exists.
- Passkey dialog visible → safe to create a staging user without explicit approval.

---

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: 0cf1ba4e31c4b8a932ac7b5438372267ac7b30c5 (super definition settled Pass 2 and pointed here)
  current_artifact_state:
    - Pass 2 child definition is settled complete.
    - Super definition says the next executable probe is active-refresh/autoputer boot readiness.
    - Deploy evidence exists from job 85072352680.
    - Pass 3 diagnostic patch landed in `internal/vmmanager/manager.go`: guest readiness timeout errors include the last `/health` probe status/body/error.
    - Pass 3 deploy diagnostic patch landed in `.github/workflows/ci.yml`: failure diagnostics include vmctl ownership snapshots and direct active sandbox health probes.
    - Commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a` deployed those diagnostics to staging; deploy job `85076877932` reported no active interactive computers needed refresh.
  what_was_proven:
    - Package source-filter bug is repaired.
    - Host services can deploy and report health while active guest refresh still fails in prior evidence.
    - Current evidence is sufficient to scope Pass 3 and confirm the first evidence-layer root cause; it is not sufficient to pick the product boot fix.
    - The deployed diagnostic patch did not regress host deploy health or CI.
  unproven_or_partial_claims:
    - Whether the active target VM reached `server.Start()`.
    - Whether preserved persistent state blocks runtime startup.
    - Whether host-to-guest tap networking blocks HTTP readiness.
    - Whether `/health` returns non-200 versus never accepting TCP.
    - Whether emergency mode is primary root cause or a second VM's separate failure.
    - Whether the new diagnostics capture the active-refresh failure path, because the first deploy after the patch had zero active interactive computers to refresh.
    - Whether an authenticated product session can create or wake an active interactive computer for the next deploy; the current harness browser is signed out and has no cookies/storage credentials.
  next_executable_probe: Create or observe an active interactive computer through the product path, then run or wait for an ordinary guest deploy that refreshes it and captures the new readiness diagnostics; if product-path activation is unavailable, record the missing credential/tool boundary instead of weakening Pass 3.
  suggested_goal_string: "/goal docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md"
  evidence_artifact_refs:
    - docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md Pass 8 through Pass 12
    - GitHub Actions deploy job 85072352680
    - CI run 28683693425
    - CI run 28684139979
    - CI run 28685279292 and deploy job 85076877932
    - Race Detector run 28685279281 attempt 2
    - browser product-path probe: `https://choir.news` opened signed out; Desk -> Sign in exposed passkey creation/login, but no account creation or login was performed.
    - staging `/health` showing deployed commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`
    - staging `/health/ready` showing degraded runtime/dolt/ollama
    - diagnostic patch files: `internal/vmmanager/manager.go`, `internal/vmmanager/manager_test.go`, `.github/workflows/ci.yml`
    - focused test: `go test ./internal/vmmanager -run TestWaitForGuestReady -count=1`
    - deploy-impact classifier test: `.github/scripts/deploy-impact-classify-test`
  rollback_refs:
    - main HEAD before Pass 3: 0cf1ba4e31c4b8a932ac7b5438372267ac7b30c5
    - package fix commit: 02fa2ea6603b7f157c982e9da637ec714301c6bf
    - active-refresh diagnostic gate commit: 8e694f4663412c1a33fc70e870f225f2510718f2
```
