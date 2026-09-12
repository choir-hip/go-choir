# RLM Replay Deployed Proof — 2026-09-12

Class: green (evidence only).
Authority: `docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`
(P4-replay: one deployed proof per operation class, with the local harness
run per operation; P4-review r4 ACCEPT).

## 1. Deployment and Environment Identity

- **Deployed commit:** `d6b4fab172220160b5b093116719d31195258ed1`
  (P4-review r4 accept polish and slice advance)
- **CI run:** `34695293124` — `completed success` (all lanes incl. all
  agentcore/textureowner race shards, Deploy to Staging)
- **Staging identity:** `https://choir.news/health` serves
  `deployed_commit: d6b4fab1`, deployed `2026-09-12T13:22:18Z`, effects OFF
- **Proof host:** Node B, worktree `/root/deployed-proof` at exactly the
  deployed SHA (pristine; `git status` clean except the driver delta below)
- **Broker:** `/tmp/capsule-broker-deployed`, built with plain
  `go build ./cmd/capsule-broker` from the pristine deployed tree
  (`_test.go` files cannot affect the broker build)
- **Driver delta vs deployed SHA:** test-only
  (`internal/agentcore/rlm_replay_linux_test.go` — per-run capsule-name
  shortening for the 107-char unix socket limit; no product, broker,
  golden, or receipt-shape change)
- **Goldens:** committed at the deployed SHA (build_sha `8f987d7f` with
  qualified provenance note); manifest coherence asserted by the driver
- **Capture state:** fresh copy `/root/rlm-replay-deployed` of
  `/root/rlm-replay-state-v3` (capture 2026-09-12T03:08Z)

## 2. Command

```sh
RLM_CAPTURE_STATE=/root/rlm-replay-deployed \
CHOIR_CAPSULE_BROKER=/tmp/capsule-broker-deployed \
CHOIR_ACTUATOR=rlm \
go test ./internal/agentcore -run TestRLMReplayGoldens -v -count=1 -timeout 600s
```

## 3. Result

`--- PASS: TestRLMReplayGoldens (1.64s)` on 2026-09-12T13:31:24Z:

```text
replayed 5 goldens through the in-cell carrier: canonical equality +
zero-effect census + conflict/new-identity legs passed
```

Per-operation rows exercised (each: canonical equality on declared P0
fields, zero-effect census, pre-effect conflict on reused identity with
changed input, new-identity leg):

1. `commit_transaction` → `choir.Freeze` (8 compared fields; `state`
   excluded as freeze-time constant with recorded residue)
2. `inspect_self_development_bundle` → `choir.InspectBundle` (14 fields;
   integrity leg: whole-tree copy + one corrupted runtime file rejects
   with `frozen runtime file digest mismatch`)
3. `record_self_development_verification` → `choir.Verify` (4 fields;
   fail-on-recorded-pass refused)
4. `update_coagent` → `choir.Message` (8 fields; fresh identity mints a
   distinct update row, golden row untouched)
5. `record_assignment_result` → `choir.Complete` (12 fields; `replay`
   excluded as capture-unrepresentable marker with recorded residue;
   terminal assignment replays recorded fate, no second report)

Prior local verification on the same driver: green 3× consecutively on one
shared snapshot plus polish green (Node B, 2026-09-12).

## 4. Residues Carried (not blockers)

- Row-5 `state` and row-9 `replay` narrow the compared P0 set by named
  exclusion, enforced P0-relative by the driver guard
  (`replay-row5-state-excluded-2026-09-12`, `replay-p0-replay-marker-gap-2026-09-12`)
- Fresh-Freeze independence unproven; leg scoped to Start identity
  allocation (`replay-fresh-freeze-deferred-2026-09-12`)
- P4-review r4 dissents recorded: sol recapture preference and
  replay-marker product surfacing (majority: restamp-with-qualifier,
  exclusion+residue)
