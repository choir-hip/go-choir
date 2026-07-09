You are one member of an independent agentic consensus panel.
Do not assume other agents agree with you.
Return concise, decision-useful output.

Task:
Phase A exit gate review for `docs/definitions/og-dolt-heresy-completion-2026-07-08.md`.

Review whether the claimed Phase A exit state is complete and sound. The deliverables are:

- W1 Detector manifest + CI discovery job: `docs/heresy-detectors.md` has H030/H031 rows, `scripts/check-heresies.sh` maps detector families to discovery patterns, and `.github/workflows/ci.yml` has a Heresy Detector Discovery job that runs the script and reports counts without failing.
- W2 Proxy/vmctl timeout hardening: `internal/proxy/config.go` `DefaultVmctlTimeout`, `internal/server/server.go` `ReadTimeout`/`WriteTimeout`, `nix/node-b.nix` and `start-services.sh` no longer override `PROXY_VMCTL_TIMEOUT` to 180s, and `docs/evidence/w2-timeout-staging-proof-2026-07-09.md` records a fast 504 for `/api/universal-wire/stories` under induced resolve failure.
- W3 Landing-loop evidence for seam commits e393eb5c / e5c1d38a: CI status, deployed identity, whether staging uses the lineage resolver or the fallback, and whether any flow has the promotion adapter configured.
- C1-C7 Doc truth corrections: `docs/current-architecture.md`, `docs/design-choir-headless-surface-v0.md`, `docs/choir-doctrine.md`, `docs/missions/substrate-hardening-v0.md`, `docs/missions/cross-substrate-proof-v0.md`, `docs/archive/mission-og-dolt-heresy-hard-cutover-v0.md`, `docs/doc-authority-manifest.yaml`, and `docs/README.md`.
- D-PROMO settlement: `internal/computerversion/dolt_branch_isolation_pinned_test.go` passes with `go test -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10`, confirming embedded Dolt branch isolation on a pinned connection.
- S1 Spec↔adapter reconciliation: `specs/promotion_protocol.tla` has a `Scope and conformance` header that records the spec as target-state, explains the storage-layer assumption for `BranchIsolation`, references the D-PROMO conformance test, and notes the current tag-only adapter does not implement branch isolation.
- P-TRIAGE: the `#### P-TRIAGE — past-mission open-edge triage table` in the definition document triages the ~25 ledger-sweep open edges into `absorbed:<phase>`, `retired`, or `external:<successor>` with a reason/pointer for each.

Also review the `Variant` snapshot and `Run Checkpoint` in the definition document for accuracy against the repo.

Output format:
1. Verdict: clear / conditional / reject for Phase A exit
2. Category-(a) phase-exit defects (must fix before Phase B)
3. Category-(b) new definition nodes (register, don't silently absorb)
4. Category-(c) out-of-scope noise (record and drop)
5. Confidence: high / medium / low
6. Specific repo evidence for each finding (file, command, observation)
