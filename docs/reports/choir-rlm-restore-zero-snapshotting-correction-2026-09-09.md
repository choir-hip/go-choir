# Restore-Zero Was Not Snapshotting. Here Is What That Meant, and What Changed.

**Date:** 2026-09-09
**Subject:** Why `yusefnathanson@me.com` replayed a lifetime of events, why the earlier "complete" claim was false, how snapshotting actually landed, and why "Reload app" reappeared after the owner boot.
**Status:** Restore-Zero completed on a retained-store boot of `9341b5d1` with prefix reads = 0. The earlier W=1 / W=13 HTTP 200 claim remains demoted. The Reload-app card after that boot is a document-authority hole, not a second lifetime replay.

The earlier Restore-Zero completion PDF should still be read as a substrate-progress note that overclaimed. The invariant that finishes this mission is: **after a verified snapshot W exists, no recovery path may replay the prefix.** That invariant was not true on the live owner computer when this note first shipped. It is now encoded in boot, advertised at W = H, and observed on the owner computer: PlanRecovery **resumed** at `local=148431 W=148431 H=148431 tail=0` with no prefix page fetches.

## What you actually saw

The account did not boot. The browser got `{"error":"failed to resolve user autoputer"}`. Behind that JSON, the guest was not "stuck resolving." It was reconstructing the computer from a stale local head, tens of thousands of events behind the platform, while vmctl held a refresh lock and the proxy had nothing to paint except a 502.

That long reconstruct is the failure. The 502 is a symptom. Restore-Zero exists so that reconstruct never looks like a lifetime of tape.

## Why the huge tail happened

Choir already had snapshot machinery. Offline `choir-rebuild-base` can rebuild a ProjectionBase blob. The platform can advertise a watermark W. Rematerialize already knew how to install that blob into a sibling directory, replay only `(W, H]`, and swap. Boot did not use that path.

Boot asked one question: *is the store directory empty?* If yes, install a base. If no, skip and resume. The owner computer's disk was not empty. It had a local head around 20,000. The platform head was around 148,000. The advertised watermark was 1 (a 13-blob also existed and was never even the advertised W). Because the directory contained files, boot treated "resume" as recovery. Resume from 20,000 to 148,000 is a 128,000-event lifetime catch-up. The snapshots sat on the platform unused.

That skip lived in `materializeProjectionBaseIfNeeded`. Two tests treated it as correct behavior: a non-empty store was supposed to cause *zero* platform reads. Those tests encoded the defect.

So Restore-Zero, as first deployed, snapshotted **empty** computers. It did not keep W close to H. It did not rebase a retained computer onto W. Publication of W=1 / W=13 plus an HTTP 200 was never proof that recovery had become tail-only.

## What the second panel decided

The first consensus panel spent its energy on login 502s. That was the wrong question. A second panel, asked only about snapshotting, converged on a contract that matches the mission:

1. **Who writes W.** Host-side `choir-rebuild-base` remains the publisher. After a successful rebuild it must POST the watermark; at the time of the panel the CLI printed JSON and left the table at W=1 while a W=13 blob already existed. Guest reconstruct does not become the watermark writer. A periodic host rebuild is the cadence that keeps W near H. Publishing on every refresh as the *primary* writer was rejected: deploy SHA is not H, and it couples snapshot cost to boot.

2. **When a non-empty store must consume W.** Not "never" (the old skip). Not "only if the gap is large" as the rebase trigger. Always rebase when `local < W`. Resume only when the retained head already descends from W *and* the remaining tail fits a named bound. If advertised W is itself stale, **refuse** rather than replay 128k events. The bound is `MaxRecoveryTailEvents = 10,000`. The live shape at that moment (local ≈ 20k, W = 1 or 13, H ≈ 148k) had to fail closed.

3. **How to install W.** Reuse the rematerialize staged install: sibling directory, `InstallVerifiedBase` (empty dest), quarantine the original marker and texture workspace, flip, never overwrite live SQLite in place.

4. **The test that would have blocked the false complete.** Retained local=20,000, advertised W=148,000, frozen H=148,333. Recovery may apply 333 tail events and zero prefix events. The same fixture with W=13 must refuse with zero replay. Guest `/health` JSON is not that test.

## What I implemented

The decision is a pure function, `PlanRecovery`, so it can be tested without opening Dolt:

- empty + no chain → genesis (the only genesis)
- non-empty + no chain → refuse
- chain + missing or stale W → refuse
- empty + fresh W → install in place
- local < W + fresh W → rebase
- local ≥ W + tail inside 10,000 → resume

Boot calls that plan on every credentialed startup, including retained disks. The empty-store skip is deleted. A retained computer whose remaining work would exceed 10,000 events dies with `projection base refused` instead of disappearing into a 128k reconstruct. A retained computer that is behind a *fresh* W is rebased: the verified blob is unpacked beside the live store, the original is quarantined, the new marker and `.texture` workspace are flipped into place, then reconstruct starts at W.

`choir-rebuild-base --advertise` now POSTs the published blob through the existing watermark handler, which remains the sole writer of `computer_replay_watermarks`. Publishing a blob without advertising it is how W=13 existed while the table still said W=1.

Rematerialize uses the same tail bound: a restore whose `(W, H]` exceeds 10,000 refuses rather than becoming another lifetime replay.

Local tests that now exist, and that the previous completion could not have passed:

- PlanRecovery on the live staging numbers: W=1 and W=13 against H=148,333 refuse; local=20,000 with W=148,000 rebases a 333-event tail; local already past W resumes.
- A retained store installed at W=1, then rebased onto W=2, opens at sequence 2. The original realization is quarantined. Staging directories do not leak.
- Boot materialize against W=13 / H=148,333 on a real retained disk refuses and leaves the marker in place.
- Advertise POSTs computer, sequence, and blob digest to the watermark endpoint.

That contract landed as `9341b5d1` (`red(restore-zero): rebase retained stores onto advertised watermark`).

## What happened after the contract landed

Fail-closed recovery is not owner-visible boot. With W still at 1, a retained computer would have **refused** instead of replaying 128k events. The remaining work was publication, then a retained-store boot of that binary.

### 1. Host publication of W near H

A host-side near-head rebuild on Node B published blob `6099cf6998ac87c7e763cdbbb4e2ae051ae097815e49bf795dc5b008d61fc599` (15,884,999,168 bytes). Sidecar sequence **148431**, `canonical_head=1eb9b931…`, `vocabulary_version=v1`. The Node B rebuild binary was still `80d43427` and lacked `--advertise`, so the sole Dolt writer was POSTed directly:

```
POST http://127.0.0.1:8086/internal/computers/files/watermark
{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","watermark_sequence":148431,"base_ref":"6099cf69…"}
```

HTTP 200. `RecordReplayWatermark` only advances if the new sequence is greater, so 148431 replaced 1. GET and the Dolt table matched. That rebuild was allowed to read the prefix. It is publication, on the host, into an isolated scratch store. It is not recovery.

At that point W = H = 148431. A retained boot of the *new* binary could legally **resume** with `tail=0` if the store already sat at H. Rebase would run only if local were still behind W.

### 2. Host deploy did not boot the owner computer

CI [34323073581](https://github.com/choir-hip/go-choir/actions/runs/34323073581) installed `9341b5d1` on the staging host at 2026-09-09T07:49:24Z. `artifacts.autoputer.status=installed`. `artifacts.active_computers.status=empty`.

That empty refresh list is honest, not a miss. The owner computer is `snapshot_kind: constructed-computer-version`. G4 requires deploy to skip those realizations. After NixOS switch, vmctl reattached Firecracker pid 641150. The guest at `10.200.6.2:8085` kept serving `80d43427`, epoch 893. No `projection recovery resume|rebased` line appeared for this computer. Prefix-read count remained unobserved.

The product path that *does* refresh a named constructed computer, without rewriting ComputerVersion, is owner-scoped `POST /api/computers/{id}/lifecycle/refresh`. CI global refresh is not that path. SSH `curl` to `/internal/vmctl/refresh` was not issued. `recover_current` was not issued: that quarantines `data.img` and stages a blank 32 GiB successor.

### 3. Owner-scoped retained-store boot (the actual proof)

```
POST https://choir.news/api/computers/computer-03335285269bdba4f94377e56879f9e6/lifecycle/refresh
{"idempotency_key":"restore-zero-9341b5d1-owner-refresh-20260909"}
```

- Started 2026-09-09T08:16:45Z; HTTP 201 at 08:17:00Z.
- Epoch **893 → 894**. Guest moved `10.200.6.2` → `10.200.12.2`.
- Guest commit **`9341b5d1`**, `deployed_at=2026-09-09T08:16:51Z`.
- Persistent disk used ~15.5 GiB (retained, not a sparse blank).

vmctl journal for that Firecracker console:

```
2026/09/09 08:16:59 autoputer: projection recovery resume for computer-03335285269bdba4f94377e56879f9e6 (local=148431 W=148431 H=148431 tail=0)
2026/09/09 08:16:59 autoputer: computer event authority reconstructed
2026/09/09 08:17:00 vmctl: refreshed VM candidate-fleet-e15cb89f25d963c220319b7b … (new_epoch=894, deploy-image-refresh)
```

Interpretation against the Restore-Zero contract:

- Decision: **resume**, not rebase, not genesis, not refuse.
- `local = W = H = 148431`, `tail = 0`.
- Reconstruct completed immediately after the resume line.
- Prefix enumerated/fetched/reduced for seq ≤ W = 0. Applied = H−W = 0.
- No `ProjectionBase rebased`, `required projection base refused`, `projection recovery genesis`, or `after=0` lifetime replay for this computer.

That is prefix=0 retained-store recovery on the snapshotting binary. Guest `/health` `ready` is supporting context, not the acceptance line. Genesis guest `computer-bb0f4fa5` (`local=0 W=0 H=0`) is not this proof. Host/proxy SHA `9341b5d1` without this guest boot is not this proof.

Receipt: `docs/evidence/choir-rlm-restore-zero-retained-boot-2026-09-09.md`.

## Why "Reload app" came back after that boot

The computer restored. Desktop windows did not. That is a different invariant.

Authenticated `/` and `/assets/*` are the guest computer's SPA. The unsigned platform shell is a **different Vite build**. Yesterday's passkey fix (`53856b`) only handed the document off after login: `prewarmAuthenticatedComputer()` then `window.location.replace`. Cookie session restore and warm-boot window restore never ran that path. Desktop treats the route as stable when `computer_id` is unchanged; epoch 893→894 is ignored. Restored windows then `import()` whatever hashed chunks the already-loaded document baked in. Those requests are now authenticated, so the proxy sends them to the guest, which does not have the host hashes → 404 → **Could not open … Reload app**.

Observed after the retained boot, before the follow-on frontend:

| Surface | JS bundle |
| --- | --- |
| Host (`choir.news` unsigned) | `index-CXDFEMio.js`, then `index-vvJBmqzx.js` (`dc291240`) |
| Guest (`10.200.12.2`) | `index-Bi7uNT3r.js` (`9341b5d1`) |

Same CSS hash, different JS. Constructed-computer G4 is why the guest still serves `9341b5d1`. That split is expected. Runtime passivation and Super rewarm on boot are load-bearing; clearing desktop state or skipping those phases is the wrong fix.

The UI repair is one C15/I25 document replace when the executing `/assets/*` graph ≠ the live computer's `index.html`, after session restore on mount, after bootstrap becomes stable, and once on a dynamic-import 404. That landed as `dc291240` (`orange(frontend): hand off to computer surface after warm boot and session restore`). CI [34336474979](https://github.com/choir-hip/go-choir/actions/runs/34336474979) succeeded. Host frontend deployed at 2026-09-09T09:46:40Z with asset `index-vvJBmqzx.js`.

Until an already-open tab loads that host bundle, **click Reload app once the computer is `ready`**. A full reload with cookies loads the guest document and the windows come back. The next warm login from the host shell should do that replace itself.

## What is still not this mission

- Proxy 502 JSON (`failed to resolve user autoputer`) while resolve is locked remains a routing/UX defect. If we had "fixed" the 502 first, the owner would have watched a boot screen for the length of a lifetime replay. The snapshot contract is why that wait is now a resume at head, not a hundred thousand events.
- Periodic host rebuild so W stays within 10,000 of future H is follow-on (S1-B), not in `9341b5d1`.
- Do not weaken CI's skip of `snapshot_kind: constructed-computer-version`.
- Do not treat guest `/health` JSON, W=1/W=13 publication, host SHA `9341b5d1`, or genesis on a different computer as Restore-Zero proof.

## Short version

The mission is snapshotting so we never replay the whole log. We had snapshots that only attached to empty disks. The owner's disk was not empty, so boot ignored the snapshots and replayed history. The panel said: delete that skip, refuse a stale watermark, rebase a retained disk through the install path we already had, and make the publisher actually advertise W. That is `9341b5d1`.

Then we published W=148431 on the host, left the G4 constructed-computer skip intact, and used owner-scoped refresh — not CI global refresh, not SSH vmctl, not `recover_current`. The retained computer booted `9341b5d1` at epoch 894 and **resumed** at `local=W=H=148431 tail=0` with prefix reads = 0. That is completion.

The Reload-app card after that boot was the host document still executing while lazy imports went to the new guest. It is not a second lifetime replay. The host SPA now hands off when the asset graph differs. Click Reload app once on an already-open tab if the computer is already `ready`.
