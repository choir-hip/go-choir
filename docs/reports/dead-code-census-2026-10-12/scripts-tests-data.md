# Dead-code census: scripts, tests, specs, database schema, tracked artifacts

Census agent 3 of 5. Area: scripts/, Go tests, TLA+ specs and gates, database schema, tracked artifacts.
Read-only: nothing in the repo was edited, committed, or run against the network. HEAD = c30147a6 (2026-10-12).

## Method

- **Scripts.** Listed `git ls-files scripts` (83 files, 14,984 lines). For each file, `git grep -F <basename>` split into code/workflow/nix citers versus docs-only citers. Read the header of every non-trivial script. Checked the workflows (`.github/workflows/ci.yml`, `race.yml`, `staging-recovery-diagnostics.yml`) and `.github/scripts/*` for what actually runs.
- **Go tests.**
  - Unconditional `t.Skip`: awk scan of all 534 `_test.go` files (none found). Build tags: `git grep go:build`.
  - Compile-checked the never-run tag with `go test -c -tags comprehensive`.
  - `deadcode ./...` versus `deadcode -test ./...` (darwin build) to find functions reachable only from tests.
  - Cross-checked against another agent's linux-aware run (`deadcode/dcL-notest.txt` vs `dcL-test.txt` in this scratchpad). Linux-only code is hidden on darwin, so I trust the linux file where they differ.
  - Package reachability from `cmd/*` via `go list`.
- **Specs and gates.** Read `specs/`, `docs/heresy-detectors.md`; ran `scripts/check-heresies.sh` without `--report` (prints JSON, writes nothing). Ran the three CI vocabulary guards (all PASS). Ran `go run ./cmd/runtime-ratchet` (compare mode, read-only).
- **Schema.** Extracted every `CREATE TABLE` in non-test Go (137 tables). Counted `FROM/JOIN` readers and `INSERT/UPDATE/DELETE` writers in non-test and test Go, then hand-verified every zero.
- **Artifacts.** `stat` over all tracked files, then citer search for the biggest non-source ones.

Limits: column-level dead schema was not analysed. The `deadcode` lists are static reachability, so reflection or JSON-dispatch use would be missed. Playwright specs were checked only for CI wiring, not individually.

## Totals

| Section | Items | Approx. lines or bytes |
|---|---|---|
| 1. Scripts | 9 | about 12,800 lines of the 14,984 in scripts/ (plus 760 in frontend/) |
| 2. Tests | 8 | about 6,000 lines behind a never-set tag, about 12,000 lines of tests whose subject has no production caller, 1,871 lines of manual replay harness |
| 3. Specs, gates, detectors | 6 | 2,700 lines of runtime-ratchet Go, a 474 KB baseline, 551 lines of TLA+, a 78-line one-time workflow |
| 4. Schema | 2 | 24 tables never touched by Go SQL, 17 legacy runtime tables with no production writer |
| 5. Artifacts | 5 | a 32.8 MB binary, a tracked nix-store symlink, an orphaned Swift project |

Biggest single finding: `/capsule-broker` is a 32.8 MB static ELF binary committed to git root (item 5.1).

Key to "already scheduled": Cut 1 = `docs/definitions/choir-reeng-cut1-authority-2026-10-12.md`, Cut 2 = `docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md`. Cut 0 (`choir-reeng-cut0-eyes-2026-10-12.md`) is also mentioned where relevant; it adds `scripts/reeng_tracer.mjs` and `scripts/reeng_invariants.mjs`, which do not exist yet.

---

## 1. Scripts

### 1.1 Nineteen PDF report generators (3,059 lines)

- **Paths:** `scripts/generate_*.py`
  - Shared engine: `generate_outage_reports_pdf_2026_08_28.py` (205 lines), imported by 12 sibling scripts via `importlib`.
  - Standalone: `generate_24h_review_pdf.py` (377), `generate_consensus_report_pdf.py` (374), `generate_effects_report_pdf.py` (412), `generate_master_blueprint_pdf.py` (358), `generate_station_report_pdf.py` (356), `generate_yaegi_reorientation_report_pdf.py` (362).
  - Thin wrappers, each about 43 to 56 lines: `_mission_overview_`, `_optimal_harness_thesis_`, `_passivated_authority_review`, `_restore_zero_snapshotting_correction_`, `_rlm_*` (3), `_ui_*` (3), `_update_system_consensus_`, `_carrier_narrative_`.
- **What it does or did:** Each renders one named markdown report from `docs/reports/` or `docs/reviews/` into a reportlab PDF (`uv run --with reportlab python scripts/generate_X.py <out-dir>`). Added 2026-08-19 to 2026-09-13, one per report, during the August to September report cadence. The `reporter` skill (iCloud Choir Reports) now covers this need.
- **Why dead:**
  - No workflow, nix file, runbook, or other script runs them.
  - Zero docs cite them. The only code citer is `.github/scripts/deploy-impact-classify-test` asserting that `generate_carrier_narrative_pdf_2026_09_13.py` classifies as "ignored tooling".
  - The six standalone ones hard-code `tmp/pdfs/` and no source markdown path.
  - Every wrapper's source markdown still exists, so they run, but each has one fixed input and its job is done.
- **Citers:** the deploy-impact classifier glob `scripts/generate_*.py`, plus its test. Nothing else.
- **Already scheduled?** no
- **Proposal:** remove, after the owner confirms the PDFs have been read or archived. If PDF export is still wanted, keep only `generate_outage_reports_pdf_2026_08_28.py` as a generic engine and rename it.
- **Risk:** very low. Update `deploy-impact-classify-test` lines 197 and 207 if the carrier-narrative script goes.

### 1.2 Landed-mission acceptance probes (about 3,800 lines)

- **Paths:**
  - `scripts/s0b_disposable_keydriver.mjs` (111)
  - `scripts/s0m_*.mjs` (8 files, 95 to 160 lines each: ask, channel_mail_reject, disposable_keydriver, escalate, note, precommit_resolve, report, stranded_bound)
  - `scripts/s1a_refusal_matrix_probe.mjs` (204)
  - `scripts/s2_layered_update_probe.mjs` (303), `scripts/s2_station_acceptance.sh` (266), `scripts/s2_discharge_pending_transition.go` (342, `//go:build ignore`)
  - `scripts/smg_rlm_acceptance_probe.mjs` (212)
  - `scripts/m0_qa_probe.mjs` (157), `m3_vmctl_refresh_probe.mjs` (489), `m4_jev_roundtrip.mjs` (85), `m9a_platform_update_probe.mjs` (292)
  - `scripts/texture_revision_cadence_probe.mjs` (250), `scripts/conductor_jev_experiment.mjs` (71), `scripts/selfdev-turn-census.sh` (78)
- **What it does or did:** Each is a deployed-staging acceptance probe for one named mission station (S0m record-native cutovers, S1a refusal matrix, S2 layered update, SMG management-RLM, M0 to M9a milestones). They register a passkey on staging, drive the prompt bar or a product route, and write an evidence JSON. Added 2026-06-17 (m3, cadence) and 2026-08-21 (census), and 2026-09-26 to 2026-10-10 for the rest.
- **Why dead:**
  - Every station they prove has landed. The receipts live in `docs/evidence` and `docs/problems`.
  - Only docs-evidence prose cites them. No CI path runs them.
  - Several are anchored to things Cut 1 deletes. The S0m probes exercise `choir.Ask/Precommit/Reply/Note`, `open_researcher`, control packets, and `update_id` `:ask:`/`:report:` packet ids. Cut 1 deletes packets and `update_coagent` (cut1 lines 198, 233).
  - Two call `/api/trace/trajectories/*`, which Cut 0 records as "served by nothing" (`cut0-eyes` lines 74 to 79): `texture_revision_cadence_probe.mjs:87`, `m3_vmctl_refresh_probe.mjs:133`.
  - `s2_station_acceptance.sh` hard-codes an owner uuid, a computer id, and a rollback to commit `2f0e2cac`. `s2_discharge_pending_transition.go` is a manual repair for a runtime that predates a fix.
  - `m4_jev_roundtrip` and `conductor_jev_experiment` target the Jev judgment route. Cut 1 removes "the runtime Jev judge" (cut1 line 223).
- **Citers:** `.github/scripts/deploy-impact-classify` globs (`scripts/m3_*`, `scripts/*_probe.mjs`) and its test. `s0m_precommit_resolve_probe` references `s0m_ask_acceptance_probe` in a comment. Docs only otherwise (evidence and problem write-ups). Cut 0 names `texture_revision_cadence_probe` and `m3_vmctl_refresh_probe` only as consumers of the dead `/api/trace` route.
- **Already scheduled?** Cut 1 deletes the vocabulary these probe, but not the files. Cut 0 will add `scripts/reeng_tracer.mjs` and `scripts/reeng_invariants.mjs` as the replacement acceptance harness. Files themselves: no.
- **Proposal:** remove after Cut 0's tracer exists. Read `s0m_*`, `smg_*` and `m0_qa_probe.mjs` first: they contain the shortest recorded form of each desk handshake. `m0_qa_probe.mjs` is cited by the hillclimb docs as "reusable", and Cut 0's tracer can absorb its timing-baseline logic.
- **Risk:** low. Before removal, check that each station's receipt in `docs/evidence` records the probe commit, so the harness stays recoverable from git history.

### 1.3 Probes still tied to open gates (keep for now)

- **Paths:**
  - `scripts/m11_selfdev_episode_probe.mjs` (632) and `scripts/lib/selfdev_approval.mjs` (228)
  - `scripts/demo_selfdev_prompt_bar.mjs` (321)
  - `scripts/qa_test_computer.mjs` (93)
  - `scripts/sh_lose_the_disk_proof.mjs` (160)
  - `scripts/vmctl_receipt_probe.mjs`, `vmctl_no_escalation_probe.mjs`, `vmctl_wedge_recovery_probe.mjs` (468 total)
- **What it does:**
  - `m11` is the self-development episode probe for the Gate 2 acceptance chain (17 docs cite it, and `docs/mission-graph.yaml:1293` names it).
  - `qa_test_computer` mints a disposable owner and computer. It is cited in `skills/choir-cli/SKILL.md` and Cut 0 reuses it (`cut0-eyes:90`).
  - `sh_lose_the_disk_proof` is the Gate 1 exit test (passed 2026-10-09).
  - The three `vmctl_*` probes accept the vmctl Phase 0 review from 2026-10-10.
- **Why listed:** same shape as 1.2 (staging probes, no CI, docs-only citers), but their gates are still open or their output is reused.
- **Already scheduled?** no
- **Proposal:** keep. Revisit `m11`, `demo_selfdev`, and `lib/selfdev_approval` when Cut 1 re-binds self-development to records, because they assert work-item and packet shapes. Revisit `sh_*` and `vmctl_*` when their gates close.
- **Risk:** n/a (kept).

### 1.4 Node B storage report family (7 scripts, 2,927 lines)

- **Paths:** `scripts/node-b-storage-report` (1,294), `-storage-proof` (123), `-storage-snapshot-gates` (302), `-storage-verify-report` (61), `-storage-vm-gates` (441), `scripts/node-b-data-img-snapshot` (302), `scripts/node-b-nix-root-report` (404).
- **What it does or did:** Report-only operator tooling from the June 14 "Node B storage retention" mission. It surveys data images and VM state and Nix roots, then produces JSON gates for snapshots and VMs. Added 2026-06-14.
- **Why dead:**
  - Only citer is the deploy-impact classifier (an "ignored tooling" rule and its test) plus `docs/archive/mission-node-b-storage-retention-v0.md`. No workflow, nix file, or runbook runs them.
  - The 2026-10-09 disk survey (`docs/problems/node-b-disk-free-space-2026-10-09.md`) was done by hand, not with these.
  - Two evidence files recommend `node-b-storage-vm-gates` (`g5-post-cutover-reproducibility-2026-07-17.json`), so they were last relied on in July.
- **Citers:** `.github/scripts/deploy-impact-classify` line 131 and its test (lines 113 to 126); `docs/archive`.
- **Already scheduled?** no
- **Proposal:** owner question. Remove if Node B disk hygiene now runs through `docs/runbooks/offline-guest-gc.md` and `scripts/node-b-deploy-disk-preflight`. Otherwise keep only `node-b-storage-report`. The five-script gate chain (`proof`, `snapshot-gates`, `verify-report`, `vm-gates`, `data-img-snapshot`) is the clearest removal.
- **Risk:** low. Remove the matching classifier case and the test lines 113 to 126.

### 1.5 V1 desk-vocabulary inventory gate (4 scripts, 329 lines, plus evidence)

- **Paths:** `scripts/check-v1-inventory.sh` (79), `v1-inventory-corpus.py` (57), `v1-inventory-gate.py` (76), `v1-inventory-queries.py` (117). Data inputs: `docs/evidence/choir-rlm-v1-inventory-2026-09-10.json` (317 KB) and `choir-rlm-v1-inventory-corpus-2026-09-10.txt` (24 KB).
- **What it does or did:** A "standing CI gate" that froze every V1 desk-vocabulary occurrence (super, co-super, researcher, and so on) in a corpus manifest and failed on drift. Added 2026-09-10 (`ec359fae`).
- **Why dead:** Commit `1f51aaf5` the same day ("writer-purity gate replaces spent freeze gate") dropped it from CI. Today CI runs only `check-v1-writer-purity.sh` (which passes). Nothing invokes `check-v1-inventory.sh`. Its frozen manifest still lists files that have since moved.
- **Citers:** only `check-v1-inventory.sh` calling its three siblings. Docs mention it in evidence prose.
- **Already scheduled?** no
- **Proposal:** remove the four scripts. Keep the two evidence files (they are receipts). This is clearly retired, not a judgment call.
- **Risk:** very low.

### 1.6 Dolt authority-split one-off programs (3 Go mains, 508 lines)

- **Paths:** `scripts/dolt-dump-split/main.go` (220), `scripts/og-body-backfill/main.go` (123), `scripts/og-delta-sync/main.go` (165).
- **What it does or did:**
  - `dolt-dump-split` routes a `dolt dump` stream into Store A and Store B files.
  - `og-body-backfill` moves `og_objects` bodies into the filesystem CAS.
  - `og-delta-sync` copies rows written to the wrong store during the 2026-09-19 cutover window.
  - All three are built by `go vet ./...` and `go build` as `package main` directories.
- **Why dead:** Single-use migration tools for the platform-dolt storage normalisation of 2026-09-18 to 09-19. They are cited only by `docs/runbooks/platform-dolt-authority-split-2026-09-18.md` and `docs/archive/goal-platform-dolt-normalization-2026-09-18.md`. They hard-code the cutover fence timestamp `2026-09-19 02:45:00` and the table lists in `scripts/dolt-dump-split/main.go`. The Active view says Store B teardown was approved on 2026-10-09.
- **Citers:** the runbook and one archived goal. Nothing else (they are unreachable from every `cmd/*`).
- **Already scheduled?** no (Cut 2 keeps Store B behind corpusd but does not mention these).
- **Proposal:** remove `og-delta-sync` (fence-specific, finished). Keep `dolt-dump-split` and `og-body-backfill` only if the runbook is still the documented procedure for another split. Owner question: is the authority split repeatable, or done?
- **Risk:** low. They are unreachable from production; the runbook would need a "tools retired, see git history" note.

### 1.7 `scripts/lint/cts-receipt-lint.py` (56 lines), broken

- **What it did:** Mechanical receipt linter for the "continuous Texture supervision" Definition (2026-08-07).
- **Why dead:** It reads `docs/definitions/choir-continuous-texture-supervision-2026-08-07.md`, which no longer exists (moved to `docs/archive/`). It would crash on first read. No citers outside the archived Definition.
- **Already scheduled?** no
- **Proposal:** remove.
- **Risk:** none.

### 1.8 Loose diagnostic scripts in `frontend/` root (11 files, 760 lines)

(Overlaps the frontend census agent; listed here because they are scripts.)

- **Paths:** `frontend/check-network.mjs`, `check-runtime.mjs`, `check-runtime2.mjs`, `check-shell-content.mjs`, `check-task-status.mjs`, `final-screenshots.mjs`, `test-shell-access.mjs`, `transfer-cookies.mjs`, `webauthn-register.mjs`, `webauthn-register-capture.mjs`, `webauthn-register-round5.mjs`.
- **What it does or did:** April 2026 debugging scripts (all added 2026-04-11 or earlier). They read `test-results/legacy-gateway-e2e-round5/storage-state.json` and call old endpoints.
- **Why dead:** No package script, CI step, or doc runbook cites any of them. A 2026-08-18 architecture review already called them out as "orphaned diagnostic scripts with hardcoded stale endpoints". Three call `/api/trace/trajectories/...`, which nothing serves.
- **Already scheduled?** no
- **Proposal:** remove.
- **Risk:** none.

### 1.9 Smaller scripts, mostly keep

| Path | Verdict |
|---|---|
| `scripts/mail-acceptance-check`, `mail-gandi-plan-records`, `mail-gandi-rollback-records`, `mail-provider-readiness`, `mail-resend-setup` (1,232 lines), `nix/deploy-mail-creds.sh` (137) | Owner question. Operator tooling for Resend and Gandi mail setup. `maild` is a live service, but these are one-time provider setup scripts from May 2026, cited only by `docs/archive` and the classifier test. `deploy-mail-creds.sh` has no citer. Keep if mail provider setup may recur; otherwise remove. Risk: low. |
| `scripts/go-test-local` (11 lines) | Docs-only citers (all `docs/archive`). It runs the old non-agentcore plus sharded flow, which CI replaced. Remove or keep as a one-line dev helper (owner question). Risk: none. |
| `check-heresies.sh`, `check-decode-roots.sh`, `check-v1-writer-purity.sh`, `check-test-hang-guards.sh`, `doccheck`, `go-test-*` (non-local), `node-b-sync-service-pointers`, `node-b-deploy-disk-preflight`, `guest-signer-state-migrate/project` | Live. Cited by `ci.yml`, `race.yml`, or `nix/*.nix`. |
| `start-services.sh` (root) | Keep. Cited by README and `frontend/playwright.config.js`; every `go run ./cmd/*` target it launches still exists. |
| `load/k6/*` (3 files) and `load/README.md` | Keep or owner question. Not cited by any workflow, and no doc cites them outside the 2026-08-11 rename manifest. They target `/health` and `/` on choir.news, so they still work. |

---

## 2. Tests

### 2.1 The `comprehensive` build tag: 14 test files, 6,014 lines, never run

- **Paths:**
  - `internal/agentcore/{concurrent_workers,content_prompt,failure_isolation,live_ws,podcast,prompts,run_memory_integration,runtime,streaming}_test.go` (3,170 lines)
  - `internal/apihandler/costs_test.go` (268)
  - `internal/browsercontrol/handler_test.go` (802)
  - `internal/content/content_test.go` (810)
  - `internal/desktopstate/handler_test.go` (751)
  - `internal/mediastate/handler_test.go` (213)
  - Two more files carry `integration || comprehensive` or `integration` tags: `internal/browsercontrol/browser_live_test.go` (153) and `internal/provider/provider_integration_test.go` (310).
- **What it does or did:** An older split between "fast" and "comprehensive" tests. The tag was the pre-July way to keep the slow runtime suite out of the default run. Old mission docs ran `go test -tags comprehensive ./internal/runtime`.
- **Why dead:**
  - No workflow or script passes `-tags comprehensive`. CI shards run plain `go test`. `scripts/go-test-integration` uses only `-tags=integration` with a name regex.
  - `internal/runtime` no longer exists. The packages were renamed or split.
  - Rot is confirmed. `go test -c -tags comprehensive` fails to compile `internal/agentcore` (`content_prompt_test.go`: `handler.HandlePromptBar undefined`, `promptBarSubmitResponse undefined`, 12 errors) and `internal/desktopstate` (`handler_test.go:250: h.CloneState undefined`).
  - Positive evidence it is not all rot: `apihandler`, `content`, `mediastate` compile and pass under the tag (I ran them). The `agentcore` files other than `content_prompt_test.go` compile but I did not run them (heavy).
- **Citers:** none live. Only `docs/archive` run recipes.
- **Already scheduled?** no
- **Proposal:** two-step. First, decide per file whether it should run in CI: remove the tag from the ones that pass (apihandler, content, mediastate and likely most of agentcore) so they join the shards, and delete `content_prompt_test.go` and `desktopstate/handler_test.go` as compile-dead. Second, whatever still fails after that is genuinely dead.
- **Risk:** medium for step one (enabling tests may surface failures or slow the shards); none for deleting the two non-compiling files.

### 2.2 Packages with zero production importers (5 packages, 3,051 lines including tests)

Verified by `go list` reachability from every `cmd/*` (cmd/desktop is a separate module and imports none of them).

| Path | Lines (code + tests) | What it did | Origin |
|---|---|---|---|
| `internal/desktop/` | 541 | Wails desktop Base-sync client and Keychain key store (`BaseClient`, `FileKeyStore`). The matching Go File Provider bridge was already deleted ("seam-repair 2026-07-10"). | 2026-06-28, M5 |
| `internal/diskinstantiation/` | 872 | Audited ext4 disposable-realization contract and backend (`Ext4Backend`, receipts). Needs `mkfs.ext4` and `dumpe2fs` to even run its tests. | 2026-07-16 |
| `internal/recovery/` | 506 | Recovery capsule, restore-drill runner, integrity scrubber. A report (2026-08-27) calls it "implemented"; `docs/problems/s0-storage-lifecycle-gaps-2026-10-05.md` says it is integrity-only. | 2026-08-26 |
| `internal/trustedoutbox/` | 575 | Trusted outbox for the irreversible-email decision policy. Only caller is `internal/agentcore/effects_rehearsal_test.go`. | 2026-08-15 |
| `internal/base/testkit/` | 557 | Scenario runner for the Base three-tree reconciliation kernel. Only used by its own test. | 2026-06-28 |

- **Why dead:** no non-test importer anywhere (`git grep` of the import paths finds only the packages' own tests, plus `effects_rehearsal_test.go` for trustedoutbox).
- **Already scheduled?** no
- **Proposal:** remove together with their tests, after the owner confirms none is a designed-but-unwired capability they still want (`recovery` and `trustedoutbox` are the two to ask about; both describe plausible future features).
- **Risk:** low for `desktop`, `base/testkit`, `diskinstantiation`; medium for `recovery` and `trustedoutbox` (design intent).

### 2.3 Tests of code that only tests call (about 9,000 lines of tests)

Method: `deadcode` finds the function unreachable from `main` but reachable from `-test`. Confirmed by `git grep` for non-test callers. Where the linux-aware run disagreed with darwin, I used the linux one.

| Cluster (non-test lines, test lines) | What it is | Evidence |
|---|---|---|
| `internal/pii/` (845, 461) | PII redaction pipeline, regex and SLM redactors (2026-06-28, M21). | `NewPipeline`, `NewRegexRedactor`, `NewSLMRedactor` have no non-test caller. Only importer is `trace/redact.go`, whose `NewRedactingStore` and `NewPipelineFromConfig` also have no caller. |
| `internal/trace/redact.go` and `query.go` (410, 424) | Trace redaction wrapper and causal-chain HTTP handler. | No caller of `NewRedactingStore`, `NewHTTPHandler`, `NewQueries`. `cut0-eyes` lines 74 to 76 already notes "a causal-chain HTTP handler that nothing mounts". `NewSQLiteStore` is also test-only. |
| `internal/maild/spool.go` and `drain.go` (418, 215) | Host fsync'd MTA spool queue and async drain worker (2026-08-26). | `NewSpoolQueue`, `NewDrainWorker` have no caller outside the package. `cmd/maild` does not use them. The `spool_queue` SQLite table is created only by this code. |
| `internal/yaegikernel/actor_state.go`, `transclusion.go`, `sidecar.go`, `eval.go` (956, 221+) | Durable actor state manager, receipt collector, process-per-activation sidecar, Yaegi `Evaluator` (Phase 0 to 3, 2026-08-26). | `NewActorStateManager`, `NewReceiptCollector`, `NewSidecarRunner`, `ExecuteWorkerStdin` have no callers. `NewEvaluator` is called only by `sidecar.go`. Also `RunSessionLoop`/`RunSessionLoopWithDrain` (kept `…AndHooks` is live). |
| `internal/computerversion/` `tree_to_fs.go`, `state_generator.go`, `user_isomorphism.go`, `promotion_certificate.go`, `object_graph_snapshot.go`, `filesystem_projection.go`, `artifact_program_binding.go` (1,462 non-test, about 2,000 tests) | Substrate "computer version" proofs from July 2026: state generator, tree-to-filesystem materialiser, user-isomorphism checker, promotion certificate. | `TreeToFS`, `StateGenerator.Generate`, `GenerateFromEvents`, `UserIsomorphismChecker`, `PromotionCertificate.*`, `ObjectGraphSnapshot.*`, `NewJournalArtifactProgram` have no non-test caller. (`routeledger.PromotionCertificateRef` is a different symbol.) The rest of the package is live. |
| `internal/provider/bridge.go` (570) | `BridgeProvider`/`GatewayBridgeProvider` (April 2026 Bedrock and Z.AI bridge). | No caller of `NewBridgeProvider` or `NewGatewayBridgeProvider`. |
| `internal/gateway/client.go` (303) | `GatewayClient` (April 2026 host-side provider boundary). | `NewGatewayClient` has no caller. `search.NewGatewayClientFromEnv` is a different type. |
| `internal/base/journal` `MemJournal`, `base/blob` `Has/Root/CopyFrom`, `base/planner` `ApplyEvent` | Base kernel helpers. | Test-only symbols; the packages themselves are live via `cmd/baseharness`. |
| `internal/routeledger` `MemoryLedger`, `vmctl/route_authority.go` `newMemoryRouteAuthority` | In-memory ledger doubles. | Test doubles in non-test files (acceptable as test support, but not production). |
| `internal/vmctl/ownership.go` (17 methods: `ResolveOrAssign`, `GetOwnership`, `StopVM`, `HibernateVM`, `ResumeVM`, `RecoverVM`, `LogoutVM`, `CheckIdleVMs` and others) | `OwnershipRegistry` legacy lifecycle API. | Reachable only from `vmctl_test.go` and `ownership_test.go` in the linux-aware run. Needs a vmctl owner to confirm, since it is a 3,087-line red-class file. |
| `internal/vocabmigrate/migrate.go` (`MigrateAgentID`, `MigrateRunRecord`, `ProvenanceLog.*`, 11 funcs) | V1 to V2 vocabulary forward migration. | Test-only; the frozen-protocol and identity-seed helpers in the same package are live. |

- **Already scheduled?** Mostly no. Partial overlaps: Cut 1 deletes `tools_worker_update.go` coagent helpers (`coagentSourceFromURI`, `coagentClaim`, `coagentAction`), the `textureowner/coagent_injection.go` injectors, `toolloop.go` budget options (`WithInitialToolChoice`, `WithPassivatingToolSuccesses`, `WithRequiredWriteTools`), and `workitem/fingerprint.go`. `yaegikernel` appears in both cuts only for unrelated module changes.
- **Proposal:** remove each cluster with its tests, in code-census order. The pii, trace/redact, maild/spool, provider/bridge, gateway/client, and yaegikernel (actor_state, transclusion, sidecar, eval) clusters are the cleanest: each is self-contained, built in a one-day feature commit, and never wired. Owner question for `maild/spool` and `recovery` (see 2.2): they are plausible future infrastructure.
- **Risk:** low per cluster; run `go vet ./...` and the CI shards after each. Several of these tests are the only coverage of a design, so this is the "learn what it did, then delete" step.

### 2.4 RLM replay harness: 1,871 lines of Go plus fixtures

- **Paths:** `internal/agentcore/rlm_replay_test.go` (569), `internal/agentcore/rlm_replay_linux_test.go` (1,302), `docs/evidence/rlm-replay/goldens/*` (5 golden JSON plus manifest), `internal/agentcore/testdata/rlm_replay/**/.gitkeep` (10 empty files), `docs/evidence/rlm-replay/.gitkeep`.
- **What it does or did:** A capture-and-replay driver (P4, 2026-09-11) that replays recorded RLM cell operations against golden fixtures. It runs only on Node B against a capture state dir.
- **Why dead:**
  - `TestRLMReplayGoldens` skips unless `RLM_CAPTURE_STATE` and `CHOIR_CAPSULE_BROKER` are set. Neither is set in any workflow.
  - `docs/protocol-review-assign-and-engineering-routing-2026-10-11.md` lines 339 to 348 says once the legacy co-super branch goes the replay "cannot authenticate", and leans toward "retiring them: the files stay as evidence, and only the Linux driver goes".
  - One of the five goldens covers `update_coagent`, which Cut 1 deletes.
  - The ten `.gitkeep` directories under `internal/agentcore/testdata/rlm_replay/` are empty and read by nothing (the harness reads `docs/evidence/rlm-replay/`).
- **Citers:** the protocol review above; evidence write-ups.
- **Already scheduled?** Cut 1 (`update_coagent`, lines 233 and 4 mentions) obsoletes one golden but does not schedule the harness. The protocol review recommends it.
- **Proposal:** remove the two Go test files and the ten empty `.gitkeep` directories; keep the goldens as evidence.
- **Risk:** low. Weigh the loss of the only deployed replay oracle against its inability to authenticate after branch 3 goes.

### 2.5 Historical experiment tests that still run in CI (3 files, 562 lines, plus 1 tagged)

- **Paths:** `internal/computerversion/dolt_branch_experiment_test.go` (209), `dolt_branch_isolation_diag_test.go` (185), `driver_v2_smoke_test.go` (144), `internal/store/engine_concurrency_experiment_test.go` (168, tag `experiment`, never run).
- **What they do:** One-day embedded-Dolt experiments. The first two are self-described "historical 2026-07-07 experiment" and "diagnostic" whose conclusions were superseded by `dolt_branch_isolation_pinned_test.go`; they are "kept as source material for the D-PROMO evidence chain". The smoke test is a pre-flight driver check. The last is the W1 concurrency experiment.
- **Why dead:** the first two prove a conclusion that was retracted. They still run in the default shards. The `experiment`-tagged one has no runner.
- **Citers:** none outside themselves.
- **Already scheduled?** no
- **Proposal:** remove the first two. Keep the pinned-connection test (it holds the current finding). Delete or archive the `experiment`-tagged file once the W1 result is recorded in its Definition.
- **Risk:** very low.

### 2.6 Env-gated tests nobody turns on (about 1,300 lines)

None of the 70 `t.Skip` calls is unconditional (verified by scan). The following are conditional on env vars that no workflow, script, or nix file ever sets:

| Test | Env | Lines |
|---|---|---|
| `internal/vmmanager/selfdev_linux_harness_test.go` | `CHOIR_G1_LINUX_HARNESS=1` | 226 |
| `internal/capsule/executor_integration_linux_test.go`, `go_eval_integration_linux_test.go` | `CHOIR_CAPSULE_INTEGRATION=1` | 271 |
| `internal/browsercontrol/browser_live_test.go` | `GO_CHOIR_RUN_OBSCURA_CDP=1` plus the `integration` tag (but the CI integration regex only covers provider, agentcore, textureowner) | 153 |
| `internal/qdrant/ollama_embedder_test.go`, `routing_test.go` | `OLLAMA_TEST`, `QDRANT_TEST` | 644 |
| `internal/gateway/search_test.go` live-provider tests | `RUN_INTEGRATION_TESTS=1` plus API keys | n/a |

- **Why listed:** these are on-demand manual proofs, not CI dead code. Each documents how to run it.
- **Already scheduled?** no
- **Proposal:** keep (owner question only for the G1 harness, whose gate has passed).
- **Risk:** n/a.

### 2.7 Test fixtures nothing reads

- `internal/agentcore/testdata/rlm_replay/**/.gitkeep` (10 files) and `docs/evidence/rlm-replay/.gitkeep`: see 2.4.
- All other `testdata/` files are read: `internal/updater/testdata/*.nar` (3, read by `closure_test.go` and `state_compat_test.go`), `internal/sourcecontract/testdata/source_contract_matrix.json` (read by `matrix_test.go`).
- **Proposal:** remove the empty `.gitkeep` directories with 2.4. Risk: none.

### 2.8 Playwright specs are not part of CI (note, not a finding)

CI runs only `pnpm build` for the frontend (`ci.yml:565-568`). The 73 specs in `frontend/tests` (about 20,800 lines) run only by hand or against deployed staging. About 40 have no citer at all. The frontend census agent should judge them by feature. One concrete lead: `frontend/tests/universal-wire-app.spec.js` and `universal-wire-staging-acceptance.spec.js` test the app that Cut 1 and Cut 2 retire (`UniversalWireApp.svelte`, per cut2 line 130).

---

## 3. Specs, detector manifests, gates

### 3.1 `cmd/runtime-ratchet` plus `docs/runtime-dissolution-inventory.yaml`: a gate that fails at HEAD

- **Paths:** `cmd/runtime-ratchet/{main,inventory,compare}.go` and `inventory_test.go` (2,696 lines), `docs/runtime-dissolution-inventory.yaml` (5,996 lines, 474 KB).
- **What it does or did:** The S0 "runtime dissolution ratchet" (2026-07-11): a baseline inventory of every export, route, tool, store call, and citer in the then-`internal/runtime` package, compared against the live repo so the dissolution could only move forward.
- **Why dead:**
  - Not run by any workflow (`ci.yml` has no reference).
  - `go run ./cmd/runtime-ratchet` fails at HEAD with 2,849 mismatches (for example "baseline item `Tool:product_api_request` is missing from current inventory", "invalid disposition").
  - `internal/runtime` no longer exists. The dissolution finished, and the baseline schema is v6 from July.
  - It builds under `go build ./cmd/...`, so it also costs CI time.
- **Citers:** `cmd/doccheck/main.go:1662` (an exclusion path), `docs/reviews/rlm-implementation-consensus-2026-08-29.md` calling it an "active acceptance contract", and a G-audit study note.
- **Already scheduled?** no (neither cut mentions it)
- **Proposal:** remove the command and the YAML together. The review doc that calls it "active" is stale. Keep the YAML's git history as the dissolution record.
- **Risk:** low. Strike the `doccheck` exclusion for the YAML path in the same change.

### 3.2 TLA+ specs: `specs/` (551 lines)

- **Paths:** `specs/actor_protocol.tla/.cfg` (242 + 22), `specs/autoputer_lifecycle.tla/.cfg` (244 + 16), `specs/README.md` (27). The CI job `tla-model-check` (`ci.yml:171-193`) downloads `tla2tools.jar` from GitHub and model-checks every `*.tla`, but only when `specs/` changes.
- **What it does or did:**
  - `actor_protocol` models the actor runtime's durable update log and passivation.
  - `autoputer_lifecycle` models VM boot, health, recovery, and hibernation.
  - Both were written 2026-06-11 to 07-03 and mechanically renamed on 2026-08-11. `specs/README.md` already calls them "historical verification artifacts" that "do not define current Choir product authority".
- **Why dead:**
  - The README disclaims them.
  - `autoputer_lifecycle.tla` cites a Definition that no longer exists (`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`).
  - They model the passivation, packet, and durable-update-log delivery that Cut 1 replaces with cursor delivery (cut1 lines 190 to 216), so they will describe a mechanism that is gone.
  - The CI job spends a Java setup and a network download for artifacts that gate nothing.
- **Citers:** `docs/current-architecture.md:80` ("model-checked in `specs/` (TLC runs in CI)"), `.github/scripts/ci-impact-classify` (`specs/*` rule).
- **Already scheduled?** no (Cut 1 changes the modelled semantics but never lists the specs)
- **Proposal:** owner question. Either delete (history keeps them) with the CI job and the classifier rule, or rewrite one spec for the cursor-delivery model after Cut 1, which is the case where TLA+ would earn its keep. Do not leave the current two running in CI against a deleted design.
- **Risk:** low. Update `current-architecture.md:80`.
- **Local note:** `specs/states/` is gitignored but holds 43 GB of TLC state dumps on this machine (29 dated directories, July 3 onward). Reclaiming it is a local `rm -rf`, not a git change.

### 3.3 `.github/workflows/staging-recovery-diagnostics.yml` (78 lines)

- **What it does or did:** `workflow_dispatch`-only job named "One-time staging recovery diagnostics". It SSHes to Node B and dumps vmctl journal and the guest signer inode shape, tuned to the 2026-07-23 first-boot signer failure. Added in 7 commits on 2026-07-23.
- **Why dead:** self-described one-time, manually-triggered, and the incident it targets (vm-state `epoch >= 8197`, guest-core signer) is closed. Hard-codes `epoch >= 8197`.
- **Citers:** none.
- **Already scheduled?** no
- **Proposal:** remove.
- **Risk:** none.

### 3.4 Stale entries in `.github/scripts/deploy-impact-classify`

- **Paths:** the case arms for `cmd/shipper/*|internal/shipper/*` (line 281), `internal/promotion/*` (line 241, shared with `cmd/corpusd/*`), and the `scripts/node-b-storage-*` and `scripts/mail-*` globs (lines 131, 144).
- **Why dead:** `cmd/shipper`, `internal/shipper`, and `internal/promotion` do not exist. The script globs go away with 1.4 and 1.9.
- **Already scheduled?** no
- **Proposal:** remove the dead arms in the same change as whatever script removal precedes them. The test file needs matching edits.
- **Risk:** low; the classifier test (`deploy-impact-classify-test`) must be updated and run in CI.

### 3.5 Heresy detector manifest (`docs/heresy-detectors.md`)

- Only two rows are enforced (`H011/H012` and `I4`); both show zero hits, so they pass. The other 16 rows are report-only counts that gate nothing, and most hits are docs.
- `H023` (`pollInternalWorkerRun`, 5 docs hits, 0 code hits) and `H025` (`PostChildResult`, `PostChildError`, `WaitForChildResult`, 9 docs hits, 0 code hits) have reached their target in production code. They could be marked "repaired" and promoted to `enforce: zero`, which is the opposite of dead.
- The three H011/H012 patterns (`texturePromptNeedsSuperExecution`, `promptBarExplicitResearcherIntent`, `texturePromptExplicitlyRequestsResearcher`) match nothing anywhere; harmless.
- **Already scheduled?** no. **Proposal:** keep; promote `H023` and `H025`. **Risk:** none.

### 3.6 Vocabulary gate lists

`check-v1-writer-purity.sh`, `check-decode-roots.sh`, `check-test-hang-guards.sh` all PASS at HEAD and every path in their allowlists exists. The V1 inventory gate is covered in 1.5. Nothing to remove.

---

## 4. Database schema

I extracted all 137 `CREATE TABLE` statements and counted production readers and writers. Dolt DDL is `CREATE TABLE IF NOT EXISTS` in code, so removing a statement does not drop the table from existing databases. Any removal needs an explicit migration decision for staging (`docs/runbooks`).

### 4.1 Twenty-four Store B tables in `internal/platform/store.go` that no Go code reads or writes

- **Paths:** `internal/platform/store.go` lines 44 to 339 (`corpusSchemaDDL`, about 300 lines). Tables: `platform_subjects`, `publication_proposals`, `publications`, `publication_versions`, `public_routes`, `artifact_manifests`, `artifact_blobs`, `provenance_entities`, `provenance_activities`, `provenance_agents`, `provenance_edges`, `consent_records`, `review_records`, `retrieval_sources`, `retrieval_spans`, `retrieval_manifests`, `citation_edges`, `publication_source_entities`, `publication_transclusions`, `publication_policies`, `publication_version_proposals`, `proposal_delivery_records`, `verifier_attestations`, `rollback_refs`.
- **What it does or did:** A relational publication, provenance, consent, retrieval and citation model created by commit `866c7eef` "Add platform Dolt publication service" (2026-05-15). The Go publication service stores publications as `og_objects` and `og_edges` instead, so these tables were never populated by it.
- **Why dead:**
  - Zero `SELECT/FROM/JOIN/INSERT/UPDATE/DELETE` against any of the 24 tables in any non-test or test Go file (multi-line SQL included).
  - The only other mentions are table-name routing sets in `scripts/dolt-dump-split` and `og-delta-sync` (one-off tools, see 1.6) and two `"platform-dolt:publication_version_proposals/"` delivery-ref strings in `service.go:571,618` that are labels, not queries.
  - Cut 2 itself notes "the publication_transclusions table exists (internal/platform/store.go:260)" while designing publication on `og_objects` / Store B objects.
- **Citers:** the DDL only, plus the routing sets in the one-off scripts.
- **Already scheduled?** Cut 2 line 125 and line 413 acknowledge `internal/platform` publish and transclusion code will change, but neither lists these tables for removal. Verdict: no.
- **Proposal:** remove the DDL block. Before that, confirm on Node B that the live corpus database has these tables empty (`SELECT COUNT(*)` through the no-SSH read path or a one-off check). Leave the tables in place in the database (Dolt history); only the DDL goes.
- **Risk:** low if the tables are empty on staging; nothing in code reads them.

### 4.2 Seventeen legacy runtime tables in `internal/store/*.go` with no production writer

- **Paths and tables:**
  - `internal/store/store.go`: `agents`, `runs`, `channel_messages`, `inbox_deliveries`, `run_acceptances`, `run_continuations`, `trajectories`, `work_items`, `browser_sessions`, `worker_updates`, `coagent_mailboxes`, `desktop_state`, `computer_effective_state`.
  - `internal/store/texture.go`: `texture_documents`, `texture_controller_checkpoints`, `content_items`, `podcast_subscriptions`.
- **What it does or did:** The original relational runtime schema (April 2026 to June 2026): agents, runs, work items, mailboxes, desktop state. Live state moved to `og_objects` and the event tape.
- **Why dead:**
  - No production `INSERT/UPDATE/DELETE/REPLACE` against any of them. The only writers are tests (`store_test.go`, `migration_test.go`, `residue_import_test.go`, `vocab_drill_test.go`).
  - Remaining production readers are legacy-only: `vocab_migrate.go` (`SELECT DISTINCT role FROM …`), `residue_import.go:378` (`FROM runs`), and one-off reads in `trajectory.go:52`, `run_acceptance.go:130`, `store.go:2190`, `:3367`, `:1062`.
  - `agentcore/replay_eligibility.go` already classifies most of them `empty_until_supported`, which means the repo itself expects them to be empty.
  - Eight have no reader at all: `coagent_mailboxes`, `computer_effective_state`, `texture_documents`, `texture_controller_checkpoints`, `content_items`, `podcast_subscriptions`, `browser_sessions`, `run_continuations`.
  - `co_super_slots` is not in this list (it has live writers at `store.go:1752-1817`; the 'Cut 1' work-state removal may retire it).
- **Citers:** the DDL and indexes, `replay_eligibility.go`, vocab migration, and `residue_import.go`.
- **Already scheduled?** Partly. Cut 1 deletes "work items, packets, delivery claims" (cut1 line 198) but these concern objectgraph `choir.work_item` objects. It does not name the relational `work_items`, `worker_updates`, `inbox_deliveries`, or `run_continuations` tables. Verdict: no for the tables; yes for most of their eventual readers.
- **Proposal:** remove the DDL and the legacy readers together, after deciding what `vocab_migrate` and `residue_import` still guarantee for old stores. `replay_eligibility.go` entries must change in the same commit (the manifest version 3 is a hash-checked contract; bump it). This is a red-class change (store schema epoch, `storeschema.Version` is 1).
- **Risk:** medium. Removing a table from `replayAirworthinessEntries` changes the replay eligibility manifest, and existing guest stores still carry these tables.

---

## 5. Tracked artifacts

### 5.1 `/capsule-broker`, a 32.8 MB static ELF in the repo root

- **Paths:** `capsule-broker` (32,826,572 bytes; x86-64 Linux, not stripped, Go BuildID).
- **What it does or did:** A built copy of `cmd/capsule-broker`. It was added by commit `19e2c256` (2026-09-27, "red(runtime): delete actuator=tools end to end"), almost certainly by accident (`.gitignore` lists `/choir`, `/choir-builder`, `/choir-updater`, `/sourcecycled`, `/maild`, but not `/capsule-broker`). It is the single largest tracked file by an order of magnitude.
- **Why dead:** nothing consumes the root copy. Nix builds `cmd/capsule-broker` from source in `flake.nix`; docs and tests that mention the binary refer to `/tmp/capsule-broker` or a build output (`CHOIR_CAPSULE_BROKER=/tmp/capsule-broker`). The checked-in copy is stale by definition once the source changes.
- **Citers:** none for the file. Text matches are other paths that mention the name.
- **Already scheduled?** no
- **Proposal:** remove from the tree and add `/capsule-broker` to `.gitignore`. The 32 MB stays in history unless the owner wants a history rewrite (owner question: destructive, so not recommended).
- **Risk:** very low. Check `git log -p` once for any script that deliberately copies it; none found.

### 5.2 `result`, a tracked symlink into `/nix/store`

- **Path:** `result` (symlink to `/nix/store/srzb724q…-prefetch-npm-deps-0.1.0`, committed 2026-04-11).
- **What it is:** an accidentally committed `nix build` output link. It is dangling on every machine except the one that made it.
- **Why dead:** no citer; `.gitignore` does not list `result`, which is why it was added.
- **Already scheduled?** no. **Proposal:** remove and add `result` to `.gitignore`. **Risk:** none.

### 5.3 `macos/` Swift File Provider project (7 files)

- **Paths:** `macos/ChoirFileProvider/*.swift`, `Info.plist`, `ChoirFileProvider.entitlements`, `macos/ChoirFileProvider.xcodeproj/project.pbxproj`, `macos/README.md`.
- **What it does or did:** A macOS Finder File Provider extension that projected Base-synced files. Its own README now opens: "This Swift extension is orphaned and must not be packaged or registered until the PC-5 Base kernel acceptance matrix passes". The Go bridge it talked to (`internal/desktop/fileprovider`) was deleted 2026-07-10, and `internal/desktop` has no importer (see 2.2).
- **Citers:** none outside the 2026-08-11 rename manifest.
- **Already scheduled?** no
- **Proposal:** remove together with `internal/desktop/` and the Base sync client, unless the owner still plans the Finder integration (owner question; Base kernel work in `internal/base` and `cmd/base*` is the other half of that decision).
- **Risk:** low.

### 5.4 Other large tracked files (all kept)

Checked the next 35 by size. All are `docs/` history or live assets:

- `docs/evidence/*` JSON (carrier roster receipt 3.2 MB, g4/g5 July receipts, m11 reruns): each is cited by other evidence or docs.
- `docs/deck/*.png` (5.4 MB): cited by the deck HTML files.
- `docs/runtime-dissolution-inventory.yaml` (474 KB): see 3.1.
- `cmd/desktop/build/darwin/*` icons: Wails build inputs.
- `internal/sources/registry.json` (144 KB): `//go:embed` target.

Per instructions, no deletion is proposed for docs, evidence, or archive.

### 5.5 Small tracked config

- `docker-compose.qdrant.yml` (183 bytes): no code or doc runbook cites it; `nix/node-b.nix` defines the production qdrant service, and `internal/qdrant` is live. Owner question: keep as a local-dev convenience or remove.
- `load/` (k6): see 1.9.
- `start-services.sh`: keep.

### 5.6 Gitignored but present locally (not tracked, FYI)

`specs/states/` (43 GB), root `auth.db`, `auth.log`, `gateway.log`, `proxy.log`, `vmctl.log`, built binaries `choir`, `choir-builder`, `choir-updater`, and `test-results/`. All correctly ignored; they are local disk clutter only.

---

## Suggested removal order (lowest risk first)

1. Accidental artifacts: 5.1, 5.2.
2. Spent or broken tooling: 1.5, 1.7, 1.8, 3.3, 3.1, 2.7.
3. One-off mission probes after Cut 0 lands its tracer: 1.1, 1.2, 1.6, 1.4.
4. Self-contained unwired code with tests: 2.2, 2.3 clusters, 2.5, 2.4.
5. Needs a decision: 2.1 (enable or delete), 3.2 (specs), 4.1 and 4.2 (schema), 5.3 (macos), 1.9 (mail scripts).
