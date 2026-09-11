# Cutover boot stall: owner computer unbootable on staging — 2026-09-11

Class: red (problem documentation only; no runtime mutation in this receipt).
Authority: `docs/definitions/choir-rlm-versioned-rename-2026-09-09.md` (mission-2,
in flight). This is a discovered heresy against the deployed cutover, recorded
before any repair per problem-documentation-first.

## Observed

Staging owner computer `computer-03335285269bdba4f94377e56879f9e6`
(VM `candidate-fleet-e15cb89f25d963c220319b7b`) cannot boot as of the
`2366d7b8` deploy (`deployed_at 2026-09-11T00:40:20Z`). Proxy `/health` is ok
but every resolve stage errors at ~255–315s (vmctl `waitForGuestReady`
exhaustion). The guest is currently in an infinite crash loop.

Node-B journal (`journalctl -u go-choir-vmctl`):

```text
00:41:32 autoputer: projection recovery resume for computer-0333... (local=148566 W=148431 H=148566 tail=0)
00:41:32 autoputer: computer event authority reconstructed
00:41:32 autoputer: starting server on 0.0.0.0:8085
        <5 minutes of silence; /health serves 503 {"status":"replaying","sequence":0,"committed_sequence":0}>
00:46:33 vmmanager: firecracker process ... exited with error: signal: killed
00:46:33 vmctl: ... guest replay stalled at http://10.200.1.2:8085 (no sequence advance for 5m0s, seq=0)
00:46:33 vmmanager: marked VM candidate-fleet-e15cb89f25d963c220319b7b as failed
~02:04  bios-cold-recover rec-1-40e7813a346e3d7a: data.img quarantined
        (data.img.quarantine-1-40e7813a346e3d7a), fresh image staged
02:03:59 boot B on pre-quarantine disk: identical resume line, identical 5min
        silence, stall-killed 02:09:01
02:09:39+ boot C on fresh 1 GiB disk: required projection rebase refused:
        projection base: download blob: context deadline exceeded
        (Client.Timeout or context cancellation while reading body)
        -> log.Fatalf -> systemd restart every ~33s, still looping
```

## Defects (each independently load-bearing)

**D1 — post-reconstruct work is invisible to the stall detector.**
`runReplayPhase` (internal/autoputer/run.go:551) calls
`appender.Reconstruct` then `db.MigrateAndFenceServingVocabulary`
(vocab_migrate.go:737) then `reconcilePendingLifecycleReceipts` before
`gate.setPending(false)`. None of these advance `ReplaySequence`, so the
health gate serves a constant `seq=0` and vmctl's `ReplayStallTimeout`
(5m, vmmanager/manager.go:1953-1978) kills the VM mid-work. The migration
re-scans every inventoried table and the full `og_objects`/`og_edges` scan
(vocab_migrate_og.go:313-405) on **every** boot — there is no persisted
"already migrated" fast path — so this is not a one-time cost. Whether the
5min was slow-scan or deadlock is not yet distinguished; either way the
liveness contract is broken.

**D2 — stall-kill escalates to disk loss.**
The stall-kill marked the VM failed; `bios-cold-recover` then quarantined the
retained 15.5 GiB `data.img` and staged a fresh image. A slow boot became
permanent loss of the retained store. The quarantined disk survives at
`data.img.quarantine-1-40e7813a346e3d7a` (rollback ref).

**D3 — rebase cannot complete for real stores.**
`projectionbase.NewHTTPSource` sets `http.Client{Timeout: 30s}`
(source_http.go:36); `DownloadBlob` (source_http.go:133-153) streams the
whole base inside that single whole-request timeout. The advertised base for
this computer is ~15 GiB
(`/var/lib/go-choir/platform-artifacts/sha256/projection-base/`). No network
finishes that in 30s, so `RecoveryRebase`/`RecoveryInstall` can never succeed
for a store of this size — the recovery path is structurally broken, and the
guest crash-loops instead of refusing once.

## Belief state

- D1 is the primary defect: it fires on every boot of any computer whose
  migration scan exceeds 5 minutes, including post-migration boots (the
  verify pass re-scans unconditionally). Restoring the quarantined disk
  without fixing D1 reproduces the same stall-kill.
- The epoch-896 deployed cutover proof (ae62fc82) ran at `cb571960`, before
  `1e96e0b2`/`80dea87e`/`742fd4d3`/`ffdc2d85`/`2366d7b8` — the streaming OG
  scan and write-guard commits are the newest code on this path and the
  leading suspects for the >5min window.
- Boot-B's rebase demand is explained by the fresh disk (localSeq < W), not
  by store corruption; the quarantined disk's local head was 148566 = H.

## Next safe probes

1. Host-drive `RUNTIME_RECOVERY_REPLAY_ONLY=1` against a copy of the
   quarantined data.img: distinguishes slow-migration from deadlock and
   materializes a migrated store without the 5-minute window.
2. If slow: gate liveness must reflect migration progress (or migration must
   be skipped when a persisted report proves completion), and the stall
   detector must not kill during fenced post-replay work.
3. D3 needs a streaming download without whole-request timeout (per-read
   deadline or none), independent of D1.

## Probe measurements (2026-09-11, host-side on quarantined disk copy)

`go test` probe (`internal/store`, commit `2366d7b8`) ran each phase of
`MigrateAndFenceServingVocabulary` against a copy of the quarantined
14 GiB store on node-b:

| phase | time |
| --- | --- |
| store open | 6s |
| load report | 0s (report exists, `{"provenance":{},"counts":{}}` — store already V2) |
| plan SQL | 0s (0 writes, 0 provenance) |
| plan OG | **4m21s** (10378 objects + 8393 edges retained) |
| persist report | 0s |
| apply SQL / apply OG | 0s |
| verify fence | **FAILS**: `vocabfence: og:choir.lifecycle_command:body.stored_result.revision.author_label="appagent" not in v2 vocabulary` |

Conclusions:

- Slow, not deadlock: the guest was killed mid-scan at 5m01s; plan-og alone
  is 4m21s and the fence adds a second full `og_objects` scan, so the total
  exceeds the window on every boot even with zero migration work.
- **D4 — fence false positive.** `author_label` sits in `ogRoleLeafKeys`
  (vocab_migrate_og.go:623) but is a human-readable author label
  (`types/texture.go:113`: username or "appagent"), not desk vocabulary.
  `choir.lifecycle_command` objects embed texture revisions carrying it.
  Even if the scan finished inside the window, the guest would
  `log.Fatalf` on this refusal — the computer is unbootable two ways.
  The inventory missed this carrier class; the fence is behaving as
  specified against an incomplete leaf-key classification.
- The epoch-896 deployed proof ran `cb571960`, before the OG fence scan
  existed (`1e96e0b2`/`2366d7b8`); this path was never exercised on a real
  store until this boot.

## Rollback refs

- Retained disk: `data.img.quarantine-1-40e7813a346e3d7a` (node-b,
  vm-state/candidate-fleet-e15cb89f25d963c220319b7b/), journal
  `rec-1-40e7813a346e3d7a.journal`, canonical_head
  `956ab4e4e4a473e2e976e56ea9d3c2195e86b74408f300c83840be64b6fdd21c`.
- Code rollback: `git revert` of the cutover range; the pre-cutover guest
  image still exists in the runtime package history.

## Heresy delta

- discovered: D1 (silent post-replay work vs stall detector), D2
  (stall-kill -> disk quarantine escalation), D3 (30s whole-body timeout on
  base download).
- introduced: none in this receipt.
- repaired: none.
