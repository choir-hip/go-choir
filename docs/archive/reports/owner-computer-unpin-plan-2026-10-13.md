# Moving the owner's computer onto the current platform: plan

Date: 2026-10-13, about 15:00 UTC. A read-only plan; nothing in it has been
run. It follows
[`owner-computer-pinned-as-canary-2026-10-13`](../problems/owner-computer-pinned-as-canary-2026-10-13.md).
The owner approved "update it now": unpin the computer and give it a planned
restart onto the current image.

Mutation class: **red**. It touches VM lifecycle and deployment routing on the
owner's computer.

## Summary

- **One sanctioned call does the update: an owner-scoped refresh.**
  - Use `POST /api/computers/<id>/lifecycle/refresh` (the product path), or
    vmctl's `POST /internal/vmctl/refresh`.
  - It reboots the computer onto the current guest image (761d94c5) and keeps
    the data disk.
  - vmctl accepts it for a constructed computer. Only deploy's global sweep
    leaves constructed computers out (`internal/vmctl/handlers.go:680`).
- **No step can "unpin" the computer in today's code.**
  - The "canary" label is cosmetic. It is derived only for display, and no
    code reads it any more.
  - Deploys skip this computer for two other reasons:
    1. They restart no active computer unless a manual dispatch asks.
    2. Even then, they leave out every `constructed-computer-version`.
  - Both conditions still hold after the refresh.
- **The restart will be a crash boot, not a planned one.**
  - No host-side code writes a planned-restart marker.
  - Even with a marker, this binary rejects one from before the Cut 1
    work-state epoch (decision D7). So nothing resumes.
  - Owner ruling §13.1 ("nothing is preserved before Autopaper goes live")
    covers this. The owner should still hear it plainly, because they
    approved expecting work to resume once.
- **The served app (SPA) does follow the base image.**
  - After the refresh, the owner gets the 761d94c5 frontend
    (`assets/index-DDhnvWxb.js`) as well as the 761d94c5 runtime.
  - The old app-layer release stays in `current/`, but it is skipped at
    every boot.

## 1. Why deploys leave this computer alone (the root mechanism)

### Facts read on Node B (2026-10-13 15:00 UTC)

| Field | Value |
|---|---|
| ownership | computer-03335285…, VM candidate-fleet-e15cb89f…, user 5bd6de97-3b58-408c-bf89-c42c81b083de, desktop primary |
| state / epoch | active, 1189 |
| warmness | premium_always_on |
| snapshot_kind | constructed-computer-version |
| divergence_status | canary (derived: no explicit status is stored) |
| construction_version | code:sha256:499bee7b…, artifact-program:sha256:9d90c866… |
| route slot | `computer:5bd6de97-…:primary`, generation 1, receipt d9dcef02…, **committed 2026-07-17T18:24:12Z** (source_commit 7122f279) |
| guest /health | build 86770ae7 (built 20261010161634); self_development_marker `app-layer-39c0d991c9b5`; release_digest 841baff9…; running_runs 0; desk_pending_mutations 1 |
| served SPA | `assets/index-BZtu3UYT.js`, the 86770ae7 base package's frontend (`/nix/store/b1v7q4mq…-autoputer-0.1.0`) |
| current guest image | build_commit 761d94c5; autoputer `/nix/store/wvfsihnb…`; frontend `assets/index-DDhnvWxb.js`; manifest sha256 `55c3e5bb7598ff527839fd9fa4a75c619351cc09b9874cb5a52f33489becb8f2` |
| obligations | restart.kind crash_or_stop; 295 passivated runs (oldest 2026-09-03); 279 open work items (oldest 2026-08-16); no due actors or wakes |
| last boot | 2026-10-10 16:36, `stop` then `resume` with lifecycle caller `owner-approved-restart-after-firecracker-sigpipe-deadlock` (vmctl journal) |

### Three conditions keep the computer off the platform, each sufficient alone

1. **Deploys restart no active computer by default.**
   - `.github/workflows/ci.yml:852` sets `REFRESH_ACTIVE_COMPUTERS` only on a
     manual `workflow_dispatch` with `refresh_active_computers=true`.
   - Otherwise every active computer is listed as "Update pending, not
     restarted" (f6001aea, 2026-10-09).
2. **Even a dispatched refresh leaves out constructed computers.**
   - The selection in `ci.yml` (~1387–1407) drops
     `snapshot_kind == "constructed-computer-version"` and `held`. It logs
     "Preserving immutable constructed computer …" (627e7420, 2026-07-17).
   - `snapshot_kind` comes from the route ledger, not from the ownership
     record. `RouteAuthority.constructedOwnershipIdentity`
     (`internal/vmctl/route_authority.go:113-216`) returns `constructed=true`
     when the slot's approval and certificate evidence join the VM.
   - This computer's slot has existed since 2026-07-17. So it has been
     "constructed" since it was created, not since 2026-10-10 as the problem
     doc says.
3. **The idle sweep never stops it.** `premium_always_on` is a protected
   warmness class (`internal/vmctl/warmness_policy.go`, see
   [`idle-test-computers-keep-old-builds-root-cause-2026-10-11`](../problems/idle-test-computers-keep-old-builds-root-cause-2026-10-11.md)).
   So "update at the next idle moment" never comes.

### The "canary" label is not the gate

- `HandleList` sets `divergence_status = "canary"` for display only, when no
  explicit status is stored and the computer is constructed
  (`internal/vmctl/handlers.go:908-915`).
- The field is written only by the manual endpoint `POST /internal/vmctl/divergence`
  (`handlers.go:566`, `ownership.go:223`). Nothing in the repository calls it.
- The only code that read `divergence_status` was the platform app-layer push
  in CI, and a47122de (2026-10-09) deleted it.
- `HandleRefresh` refuses only an explicit `"divergent"` without `force`
  (`handlers.go:712`).

### Correcting the problem doc's dating

- The VM last booted at 2026-10-10 16:36, from a manual owner-approved
  stop/resume onto the 86770ae7 base. No deploy has restarted it since,
  because of conditions 1 and 3.
- **Unsure:** the vmctl journal shows `deploy-image-refresh` refreshes of this
  VM on 2026-10-09 (05:38, 08:49, 09:33, 10:00, 10:14, 12:25 UTC).
  - Condition 2 would have excluded it from a CI refresh.
  - The caller is not recorded, because lifecycle-caller logging came later.
  - These were probably operator refreshes during the S2 layering work, but
    that is not proven.

## 2. What could turn it back to "tracking"

| Lever | What it does | Does it unpin? |
|---|---|---|
| `POST /internal/vmctl/divergence` `{"computer_id":…,"divergence_status":"tracking","platform_base_ref":…}` | Persists an explicit status in vmctl's ownership file. `/list` then shows `tracking`. | **Label only.** `snapshot_kind` stays constructed, so the CI selection still leaves it out. No other reader exists. |
| `apply-platform-follow` route projection (`internal/vmctl/self_development_route.go:31-38`, `HandleApplyPlatformFollowRouteProjection`) | Promotes the route slot under the platform-follow evidence class. | **No.** The request must carry two platform-control signatures (corpusd signs them). The only producer is the guest's own `ApplyPlatformUpdate` tail (`internal/agentcore/platform_update.go`), driven by a signed offer from `POST /internal/computers/platform-updates/offer` (`internal/platform/platform_update.go:79`). An operator cannot mint the projection alone. A platform-follow slot also still reads `constructed=true` (`route_authority.go:215`). |
| Re-running the deleted app-layer push (offer then `/internal/runtime/platform-update` through `/internal/vmctl/autoputer-proxy/…`) | The guest applies a layered release. The guest writes a planned-restart marker (`platform_update.go:263`) and restarts its own runtime. | **Not recommended.** It was removed on purpose (a47122de). It failed 0/2 on this computer on 2026-10-09 ("offer binds a different realization", "base event head is stale", "entrypoint not materialized"). A same-commit layer cannot work after a base refresh ([`guest-runtime-fix-never-reaches-layered-computers-2026-10-09`](../problems/guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md)). |
| Removing the route slot | Would make the computer ordinary (`RouteAbsent`). | **Not sanctioned.** No endpoint exists. It would be a hand edit of the route/evidence ledger: red, irreversible, and it destroys provenance. |

Making the computer really follow the platform is a code change to CI's
selection, so it is a separate red mission. One option: let an explicit
`divergence_status == "tracking"` override the constructed exclusion. Even
then, deploys restart no active computer without the dispatch input. Until
that lands, every platform update for this computer needs its own explicit
refresh (the step below).

## 3. Planned restart: what exists and what does not

- **Where the marker is written:**
  - `agentcore.WritePlannedRestartMarker` (`internal/agentcore/planned_restart.go:49`)
    writes `<StorePath>.planned-restart` on the guest's persistent volume.
  - Its only callers are inside the guest: `Runtime.markPlannedRestart`
    (`planned_restart.go:169`). That runs from the platform-update apply
    (`platform_update.go:263`, reason `platform_update`) and the
    self-development apply and rollback (`self_development_materializer.go:315, 390`).
- **Where it is consumed:** once, at boot (`internal/autoputer/run.go:514` →
  `ConsumePlannedRestartMarker`, `planned_restart.go:102`).
  - With a marker, boot runs `passivate_interrupted_activations`.
  - Without one, boot runs `close_pre_boot_runs_after_crash` and
    `close_open_rounds_after_crash` (`internal/agentcore/runtime.go:750-754`).
  - A planned boot logs `actorruntime: boot follows a planned restart (<reason> <target>)`
    (`internal/actorruntime/handler.go:86`).
- **The host side has nothing.**
  - vmctl and vmmanager hold no marker code, and no guest endpoint writes one
    on request.
  - `vmmanager.RefreshVMWithConfig` SIGKILLs the Firecracker process with no
    guest quiesce, then boots (`internal/vmmanager/manager.go:1378-1401`).
  - The 2026-10-11 root-cause doc already named this gap: "needs a
    host-requested planned-restart marker, which does not exist yet".
- **D7 makes this transition a crash boot anyway.**
  - `CurrentWorkStateEpoch = 1` (`planned_restart.go:28`, aef01cef, Cut 1,
    2026-10-13). A marker of another epoch is treated as a crash
    (`planned_restart.go:126`). The 86770ae7 runtime predates the epoch.
  - The commit says: "nothing resumes into the records-only schema".
  - A host marker stamped with epoch 1 would get around D7's intent, so
    adding one first would be wrong for this transition.

**Consequence:** the refresh will close the pre-boot work, not resume it.

- **Runs:** every pending, running, passivated or blocked run is cancelled
  with fate `interrupted_by_restart`. Today that is 295 passivated runs and
  no running ones.
- **Rounds:** every open round gets a closure record.
- **Texture:** the one pending Texture agent mutation is marked stale.
- **Other boot phases:** source schedules and self-development operations
  get their crash closes.
- **Work items (unsure):** whether the 279 open work items are touched is not
  confirmed. The crash-close code reads only runs, rounds, schedules and
  operations.
- **Coverage:** owner ruling §13.1 in
  [`re-engineering-decisions-2026-10-12`](../re-engineering-decisions-2026-10-12.md)
  covers this outcome.

## 4. The app layer after the refresh (verified in source and console)

- The guest's `current/` is the app-layer release 39c0d991 (marker
  `app-layer-39c0d991c9b5`, digest 841baff9…). It was built for base
  `a402db05…`.
- **Runtime:**
  - The boot wrapper skips a layer whose `base_image_manifest_digest`
    differs from the booted base (`nix/autoputer-vm.nix:133-142`). It then
    execs the base runtime.
  - The console log shows this on every boot since 2026-10-09: "layered
    release built for base a402db05…, booted base …; serving base runtime".
  - On the new base (`55c3e5bb…`) it will skip again, so the runtime is 761d94c5.
- **SPA:**
  - A layered release dir has no `frontend/`
    ([`layered-release-spa-underivable-2026-10-09`](../problems/layered-release-spa-underivable-2026-10-09.md)).
  - So `ComputerSurface` falls back to `CHOIR_BASELINE_RELEASE_ROOT/frontend`
    (`internal/autoputer/computer_surface.go:69-96`). With the layer skipped,
    that is the base package.
  - This matches today: the served `index-BZtu3UYT.js` equals the 86770ae7
    base frontend. After the refresh it should be `index-DDhnvWxb.js`.
- **Expected leftovers, not failures:**
  - `/health` keeps reporting `self_development_marker app-layer-39c0d991c9b5`
    and `release_digest 841baff9…`, because `current/` is unchanged.
  - The stale layer stays pinned. Checkpoint frontend identity on this
    computer still binds to it (residual 1 of the layered-SPA doc).

## 5. Steps (not run)

Run the Node B commands as `ssh node-b`. The variables used below:

```
U=5bd6de97-3b58-408c-bf89-c42c81b083de
C=computer-03335285269bdba4f94377e56879f9e6
VM=candidate-fleet-e15cb89f25d963c220319b7b
```

### Step 0: document first (CLAUDE.md "Problem Documentation First")

Update the problem doc with the corrections in §1–§3 before acting:

- the computer has been constructed since 2026-07-17;
- the gate is `snapshot_kind` together with the deploy's no-restart default,
  not the "canary" label;
- the refresh is a crash boot by design (D7 and §13.1).

| | |
|---|---|
| Changes | Docs only. |
| Verify | The commit lands on `main`. The docs-only workflow runs. |
| Rollback | `git revert`. |

### Step 1: pre-flight (read-only)

1. **Ownership.**

   ```
   curl -fsS -H 'X-Internal-Caller: true' http://127.0.0.1:8083/internal/vmctl/list \
     | jq --arg c "$C" '.ownerships[]|select(.computer_id==$c)|{state,epoch,held,computer_url,snapshot_kind,divergence_status,platform_base_ref}'
   ```

   Expect `state active`, `held` absent or false, `epoch 1189`.
2. **Guest quiet.** `curl -fsS $URL/health | jq '{running_runs,desk_pending_mutations,selfdev_active_operations,build}'`.
   - Require `running_runs == 0`.
   - `desk_pending_mutations` is 1 today and will be marked stale. The
     owner should not be in the middle of a Texture edit.
3. **No deploy in flight.** `gh run list --workflow ci.yml --limit 3`.
   - A vmctl restart during a refresh is the dangerous case
     (`ci.yml` ~794 comment).
4. **Base-skip precondition.** The new base `sha256sum /var/lib/go-choir/guest/guest-image-manifest`
   is `55c3e5bb…`. It differs from the layer's base `a402db05…`, so the layer
   will be skipped.
5. **Memory.** `free -g`: 23 GiB available today. The VM takes 8 GiB, and the
   refresh kills before it boots.
6. **Tell the owner.** The computer will be unreachable for about 30–60 s
   (last boot: first healthy at 8.9 s, total 30.4 s). Pre-boot runs close
   as interrupted.

### Step 2 (optional): record the owner's intent in the label

```
curl -fsS -X POST -H 'X-Internal-Caller: true' -H 'Content-Type: application/json' \
  -d '{"computer_id":"'$C'","divergence_status":"tracking","platform_base_ref":"761d94c51b6c57040e375023f0b10b7fcf0c2346"}' \
  http://127.0.0.1:8083/internal/vmctl/divergence
```

| | |
|---|---|
| Changes | Persists `DivergenceStatus` and `PlatformBaseRef` in vmctl's ownership file. Behavior does not change (§2). |
| Verify | `/list` shows `divergence_status "tracking"`, `platform_base_ref 761d94c5…`, and still `snapshot_kind constructed-computer-version`. |
| Rollback | The same call with `"divergence_status":"canary"`. The endpoint refuses an empty status, and an explicit "canary" equals the derived one. |
| Risk | Misleading. Readers may assume deploys now update the computer. **Recommended: skip it**, or run it only together with the follow-up CI change that reads it. |

### Step 3: owner-scoped refresh onto the current image (the actual update)

**Preferred: the product path.** It writes a corpusd lifecycle receipt
(prepare, then complete) and is attributed as `proxy.lifecycle.refresh`.

```
choir computer refresh --computer $C --idempotency-key owner-unpin-refresh-761d94c5-2026-10-13
# = POST https://choir.news/api/computers/$C/lifecycle/refresh
#   body {"idempotency_key":"owner-unpin-refresh-761d94c5-2026-10-13"}
#   auth: the owner's session, or an API key with scope computer:lifecycle (or admin)
```

- The handler is `internal/proxy/computer_lifecycle.go:159`, which calls
  `vmctlClient.RefreshDesktopContext`.
- It succeeds only if vmctl then reports `active` with an epoch above 1189.
  It returns 201 with the receipt.
- The proxy's vmctl timeout is 30 m on Node B.

**Operator path on Node B (equivalent actuation, no corpusd receipt).** This
matches the CI mechanism and the 2026-10-10 precedent:

```
curl -sS -X POST -H 'X-Internal-Caller: true' \
  -H 'X-Choir-Lifecycle-Caller: owner-approved-unpin-refresh-2026-10-13' \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"'$U'","desktop_id":"primary"}' \
  http://127.0.0.1:8083/internal/vmctl/refresh
```

**What the refresh changes:**

- vmctl checks the route join (`requireComputerVersionRoute`, which passes
  for this joined constructed route).
- `RefreshVMForDesktop` (`internal/vmctl/ownership.go:2710`) then reserves
  epoch 1190 and a new realization id and credential envelope.
- vmmanager SIGKILLs the Firecracker process and boots with the image paths
  cleared to vmctl's current defaults (`refreshConfigForCurrentDeploy`,
  `manager.go:1483`), that is `/var/lib/go-choir/guest/*` = 761d94c5.
- `data.img` (the persistent disk) is kept. `computer_url` (the tap IP)
  changes.

**Verify:**

1. **vmctl journal.**

   ```
   sudo journalctl -u go-choir-vmctl --since -10min \
     | grep -E 'lifecycle request POST /internal/vmctl/refresh|refreshed VM '$VM
   ```

   Expect `new_epoch=1190, deploy-image-refresh`.
2. **`/list`.** `state active`, `epoch` above 1189, a new `computer_url`.
   `snapshot_kind` stays constructed.
3. **Boot timeline.**
   `curl -fsS -H 'X-Internal-Caller: true' "http://127.0.0.1:8083/internal/vmctl/boot-timeline?computer_id=$C" | jq '{boot_kind,epoch,outcome,first_healthy_offset_ms,store_disk_path}'`.
   Expect `boot_kind refresh`, `outcome healthy`.
4. **Guest identity.** `curl -fsS $NEW_URL/health | jq '{status,runtime_health,build,self_development_marker,release_digest,running_runs}'`.
   - Expect `build.commit 761d94c51b6c57040e375023f0b10b7fcf0c2346` and
     `built_at 20261013144331`.
   - The marker and release_digest stay unchanged (expected, §4).
5. **Served SPA.** `curl -fsS $NEW_URL/ | grep -o 'assets/index-[A-Za-z0-9_-]*\.js'`
   should give `assets/index-DDhnvWxb.js`.
6. **Boot kind in the guest console.**

   ```
   sudo grep -a -E 'layered release built for base|boot phase begin (close_pre_boot_runs_after_crash|passivate_interrupted_activations)|boot follows a planned restart|after a crash restart' \
     /var/lib/go-choir/vm-state/$VM/console.log | tail
   ```

   - Expect `booted base 55c3e5bb…; serving base runtime`, then
     `close_pre_boot_runs_after_crash`.
   - Expect **no** "boot follows a planned restart". No marker is written,
     so nothing is consumed (§3).
7. **Obligations.**

   ```
   curl -sS -H 'X-Internal-Caller: true' -H "X-Authenticated-User: $U" \
     "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/$U/api/runtime/obligations?desktop=primary" \
     | jq '{restart,runs,open_work_items}'
   ```

   Expect `restart.kind crash_or_stop` and passivated runs down from 295 to
   about 0.
8. **Owner's browser.** Reload `choir.news`; the deleted Universal Wire app
   should be gone.
   - **Unsure:** whether Autopaper content appears depends on this computer's
     own captures and sources, not only on the build.

**Rollback:** no sanctioned per-computer path back to 86770ae7 exists.

- vmctl always boots the current image pointer. Image overrides exist in
  `RefreshVMWithConfig` but are not exposed over HTTP.
- The 86770ae7 image is still at `/nix/store/mmdyl3h6…-go-choir-guest-image`
  until garbage collection, but booting one VM on it would be an
  unsanctioned host edit.
- The rollback is forward: fix on `main`, deploy, refresh again. The cut
  plan says the same ("forward-fix after Cut 1, tagged last-good commit per
  cut").

**If the guest fails to come up healthy:**

- vmctl marks the ownership `failed`.
- Retry the refresh once.
- If it boot-loops, `POST /internal/vmctl/hold` to fence it, then
  `POST /internal/vmctl/maintenance-serve` or cold-recover. Do not repeat
  refreshes blindly.

### Step 4: record evidence and follow-ups

- Append the verification output (epoch, build, SPA hash, console lines,
  obligations) to the problem doc.
- Open the follow-up red mission: decide whether a constructed computer is
  pinned by default (the problem doc's open decision). Pick one:
  1. CI's dispatch selection honors an explicit `tracking`;
  2. the owner's computer is moved to an ordinary route;
  3. a per-computer "update now" control is added.
- Separately: a host-requested planned-restart marker that respects D7 (the
  "update when idle" step from the 2026-10-11 doc).

## 6. Risks

- **Data.**
  - The refresh is a SIGKILL of a running Dolt guest, which equals a power
    loss.
  - Every deploy refresh has done the same, and Dolt journal recovery has
    held, but there is no quiesce. Do it with `running_runs == 0`.
  - The 761d94c5 runtime runs the forward store migrations of Cuts 0–2 on the
    owner's store. The old runtime may not read the store afterwards.
  - Both are accepted under §13.1 (no state snapshot before Autopaper goes
    live).
  - An optional owner-recovery checkpoint before the refresh
    (`choir computer checkpoint …`) gives an event-chain restore target. Its
    usefulness across the schema cut is **unsure**.
- **Open work.**
  - Crash close as in §3. No resume: there is no host marker, and D7
    forbids one here.
  - The owner approved expecting one resume. Tell them that this transition
    cannot provide it and why.
- **App layer.**
  - The skip depends on the release manifest keeping
    `base_image_manifest_digest` (`a402db05…`). The console proves it does.
  - If the skip ever failed, the 2026-10-09 layer would run on the new base
    (the 10-09 failure mode). Check step 3 verification 6.
  - The stale layer stays in `current/`; clearing it is S6 work.
- **Pin persists.**
  - After the refresh, deploys still leave this computer out (conditions
    1–3).
  - It will drift again with the next deploy unless someone refreshes it
    explicitly or the follow-up lands.
- **Owner visibility.** About 30–60 s of "recovering" or a blank screen. See
  [`owner-blank-screen-during-deploy-2026-10-13`](../problems/owner-blank-screen-during-deploy-2026-10-13.md).
- **Concurrency.** Do not overlap a CI deploy. vmctl restarts mid-refresh
  have killed busy guests before.

## Heresy delta

- **Discovered:**
  1. The problem doc dates the pin to 2026-10-10; the route slot shows the
     computer has been constructed since 2026-07-17.
  2. `divergence_status` has no reader since a47122de, so it is a dead
     control surface.
  3. No host-side planned-restart path exists. This was already known
     (2026-10-11 doc).
- **Introduced:** none.
- **Repaired:** none.
