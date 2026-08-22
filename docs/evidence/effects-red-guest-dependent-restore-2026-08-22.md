# Staging Evidence: Tape Restore Is Guest-Dependent and Cannot Recover a Down Guest

- Date: 2026-08-22
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 361 `stopped` (previously `degraded`)
- Guest build: `f54eb7351dca` (staging proxy + guest, deployed 2026-08-22T19:32:04Z, CI 32579025541 green)
- Host: Node B, 181G free after reclaim (was 119G, below 120G threshold)
- Checkpoint fence: `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` (pre-A, untouched)

## Observation

Staging computer has been unrecoverable for ~18 hours (since 2026-08-21T15:54Z `last_active_at`). Browser (`choir.news` → `0333528`) shows `CHOIR BIOS / COMPUTER BOOT IS STILL PENDING` looping:

```
00s Powering user computer
00s Resolving active computer
15s Bootstrap probe 1 is still waiting; retrying
31s Bootstrap probe 2 is still waiting; retrying
31s Requesting computer recovery
33s Computer recovery requested
47s Bootstrap probe 3 is still waiting; retrying
```

`vmctl` resume was attempted 4+ times across subnets (`10.200.84.2 → 10.200.91.x → 10.200.99.x → 10.200.100.x → 10.200.102.x → 10.200.105.x`). Each cycle: kernel boots, `fc-config` regenerated correctly (`ip=10.200.106.2`, `gateway_url=http://10.200.106.1:8084`), `vm-*-tap` comes up, ping to guest succeeds (`0.09ms`), but `GET http://10.200.x.2:8085/health` returns `dial tcp 10.200.x.2:8085: connect: connection refused` for 3m then Firecracker is killed and marked `failed`:

```
2026/08/22 19:40:01 vmmanager: firecracker process for VM candidate-fleet-e15cb89 exited signal: killed
2026/08/22 19:40:01 vmctl: warmness policy failed to resume ... guest did not become healthy at http://10.200.x.2:8085 within 3m0s (last probe: Get "http://10.200.x.2:8085/health": dial tcp 10.200.x.2:8085: connect: connection refused)
2026/08/22 19:40:01 vmmanager: marked VM candidate-fleet-e15cb89 as failed
```

Product path `POST https://choir.news/api/computers/0333528/lifecycle/{rematerialize-from-tape,restore}` and `POST .../self-development/operations` both 502 `computer authority unavailable`. `GET /api/texture/documents` with `Authorization: Bearer` + `X-Choir-Computer` also 502 `failed to resolve user autoputer`.

Host disk and scheduling are healthy — CI is green, `f54eb735` deployed, 181G free, reclaim works, proxy/vmctl/corpusd `active`. The failure is lifecycle/tautology, not resources.

## Root cause

Tape restore is guest-authority code reachable only via guest authority:

1. `internal/agentcore/rematerialize.go:48-66,210,239` — `RematerializeFromTape` / `restoreComputer` require live `Runtime` (`rt.store`, `rt.eventAppender.ReconstructThroughTarget`, `verifyPinnedFrontend`, `workspaceReplaceMu`). They quarantine/swap `rematerialize-staging-*` / `rematerialize-quarantine-*` inside the guest workspace.

2. `internal/agentcore/api_self_development.go:104-125` — `handleSelfDevelopmentRoute` requires `X-Authenticated-Computer == computerID`. That header is set only after `resolveComputerURLForComputerTarget` succeeds.

3. `internal/proxy/computer_lifecycle.go:311-321` (cookie path) and `internal/proxy/api_key_computer_authority.go:182` (API-key path) refuse non-`active` realizations with `computer authority unavailable` before the header can be set.

4. Platform Dolt holds the tape at `/var/lib/go-choir/platform-dolt/platform/.dolt` (corpusd `EventSource`, `HeadCAS` per `computerevent/appender.go:108-112`) but has no host actor that can write a new realization without `:8085`. `autoputer/run.go:199-224` proves `ReconstructThroughTarget` replays platform → VM-local on every boot, but only when the guest is already running.

Therefore: `stopped/failed` guest → no `ComputerURL` → no `X-Authenticated-Computer` → no handler → no restore. Recovery (`Desktop.svelte` `wake_current_computer` → `vmctl` resume/refresh → preserve `data.img`) reuses the same `data.img` and waits on the same `:8085`, so it loops.

`data.img` (`/var/lib/go-choir/vm-state/<vm>/data.img`, virtio-blk `drive_id=data` in `vmmanager`) is a disposable projection, not the durable copy. Host-mounting ext4 and running host Dolt replay would be privileged, diverge on `ReducerVersion` vs host deploy SHA (`AGENTS.md: platform deploy SHA and computer effective identity can diverge on purpose`), and still need the guest's `CHOIR_PRIVACY_KEY_FILE` at `/mnt/persistent/choir-credentials/privacy-key` for private artifacts — tape alone is insufficient without key escrow.

## Required repair direction

Add a **host-orchestrated, corpusd-driven** cold path that does not require `:8085` or mounting `data.img` ext4:

- Extract deterministic `selfdevprotocol.RematerializeFromRequest` / `RestoreFromRequest` + `WitnessContentMatches` + `ReconstructThroughTarget` + `ComputerVersion`/pin verification into a shared `internal/computerrestore` engine callable by both guest and host. Single semantic appender stays in guest; host reconstruction is dry-run replay into a fresh store.

- Host orchestrator (`internal/vmctl/cold_recover.go`, internal endpoint `POST /internal/vmctl/computers/{id}/cold-recover`) acquires per-ComputerID durable lock + recovery lease (refuse appends/route changes for captured head), fences route, stops VM, quarantines `data.img` **as a file** (`→ data.img.quarantine-<epoch>-<ts>`, no mount), validates ownership + expected `canonical_head` / `routeGeneration`, and either:
  - `recover_current`: resolves current canonical head from corpusd itself, reconstructs current head to a fresh sibling `data.img` (or simply boots fresh and lets guest self-reconstruct on `autoputer/run.go:199`), verifies witness before route CAS. Takes no checkpoint param → structurally cannot rewind.
  - `authorized_checkpoint`: only after the Definition's E2 fence — requires canonical authorization binding `checkpointDigest + targetHead + policy/decision ref + expiry + operand scopes` (`AuthorityRef: platform-control:restore`), executor verifies quorum, appends `EventRestoreRequested` via corpusd lease, then reconstructs. 99949fe2 stays untouched until CoSuper→consensus→promotion→play→falsify completes.

- Proxy adds an owner-authenticated fallback branch for `POST /api/computers/{id}/lifecycle/{rematerialize-from-tape,restore}`: `active` → guest adapter, inactive-but-owner-authorized → host cold path with same scope checks. BIOS `wake_current_computer` does one `recover_current` after `:8085` refusal instead of looping resume. Internal only for the first slice; do not expose historical cold restore until authorization verification is implemented.

- Treat `recover_current` as operational forward recovery (already permitted by `AGENTS.md: product restore is a forward transaction`) and `99949fe2` restore as acceptance-fenced E2 proof (still OFF until promotion). Do not make World Wire, hypervisor migration, or effects ON part of this repair.

This problem is documented before code, per `AGENTS.md: Problem Documentation First` and `docs/memo-problem-documentation-first.md`. No code fix in this commit.

## Rollback / non-goals

Rollback: revert this doc commit; quarantine `data.img` files remain as evidence, not deleted. 99949fe2 untouched, event chain not rewound, effects stay OFF. Candidate Proof Definition `choir-scheduling-and-candidate-proof-2026-08-21` remains `working`.

Non-goals for this repair: arbitrary host ext4 surgery, platform-computer as sole recovery actor, new minimal initramfs, Cloud Hypervisor migration, parallel assignments, or early 99949fe2 rewind.
