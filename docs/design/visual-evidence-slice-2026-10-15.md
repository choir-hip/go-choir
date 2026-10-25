# Visual Evidence Slice: Screenshots From the Engineering Capsule

Date: 2026-10-15. Branch `feat/visual-evidence-slice`. Mutation class:
**orange** overall (capsule runtime, Freeze bundle, a precommit rule, a
read-only owner route, prompt framing), **red** for the guest image, the
vmctl boot input that carries a computer's class, and the credential-disk
grant (§9 carries the red ceremony).

Source: [Playwright in capsules](../reports/playwright-in-capsules-memory-2026-10-15.md)
(§4 placement (a), §5 evidence, §8 the minimal slice) and
[capsule jobs](capsule-long-exec-2026-10-15.md) (`choir.Start`, receipts,
the `SetJobAdmission` seam). Owner ask (2026-10-14): test computers make
images of their work for humans or agents to review. Vision input is on by
default for gpt-6-luna, deepseek-v4.1-flash and the Claude models, with the
computer's image store (`internal/imageblob`), `artifact:sha256:` refs and
`POST /api/model-policy/verify` landed (`9fa826855`).

This note is the design and the failure modes, written before the code
(CLAUDE.md "Testing"). It changes no runtime.

## 1. Shape

Five parts, each the smallest that closes its loop.

1. **Chromium in the guest image, launch granted per computer.** nixpkgs
   `chromium` (full Chromium in new-headless mode; nixpkgs has no headless
   shell) and a fontconfig file naming DejaVu go into the guest store,
   **on no PATH**. The right to launch it is a grant vmctl writes onto the
   computer's root-only credential disk at cold boot, the same carrier as
   the Claude delegate's grant: `visual-capture.json`. vmctl grants a
   computer whose id is on `CHOIR_VISUAL_CAPTURE_COMPUTERS`, or any
   computer of class test when the list holds `class:test`.
2. **A capsule capture step, run as a job.** A new guest tool,
   `choir-capture` (cmd/choir-capture over `internal/visualcapture`), is on
   the capsule PATH. After the frontend build the desk starts it with
   `choir.Start([]string{"choir-capture", "--route", "/"}, ...)`. It serves
   `/var/lib/artifact/release/frontend/` on the capsule's own loopback
   (SPA fallback; `/auth/session` answers signed-out, other `/auth/` 401 and `/api/` 503, at once), runs headless
   Chromium once per route at 1280×800 with reduced motion and no network,
   and writes each PNG, named by its sha256, plus `visual-evidence.json`
   into `/var/lib/artifact/evidence/visual/`. It is a job, so the 30 s exec
   limit does not apply.
3. **Evidence in the frozen bundle.** Freeze stages
   `var/lib/artifact/evidence/visual/` beside `release/` in the bundle
   directory (`incoming/<digest>/evidence/visual/`), checks every file's
   hash against the manifest and the manifest's frontend digest against
   the frozen release's `frontend/` files, and records
   `visual_evidence` in the bundle: the manifest digest, each capture's
   `artifact:sha256:` ref, route and size, and the receipt of the
   `choir-capture` job that made them. The bundle digest covers it. The
   Freeze result lists the captures. A new rule precommitment
   `visual_captured` (optional `routes`) settles at Freeze from that
   observation.
4. **Review.** Freeze puts each capture into the computer's image store,
   so any desk can pass the ref to a vision model. Management already has
   the seam: `choir.ProductAPI("POST", "/api/model-policy/verify", ...)`
   with `role: verifier_multimodal` and `image_ref` (the product API
   allowlist names this route for exactly this). On a computer whose grant
   is on, management's overlay gains a short section: before Verify of a
   candidate that cites captures, ask one vision call whether the screenshot
   shows the requested change, and cite the image ref with the verdict.
   Humans and drills read the captures through a new owner-authenticated
   read-only route,
   `GET /api/computers/{id}/self-development/operations/{op}/visual-evidence`
   (the manifest, and with `?ref=` the PNG bytes of a ref that bundle
   cites, never any other image).
5. **Memory.** When a capsule ends, after its cgroup is empty and before
   it is removed, the executor writes the capsule's `MemoryMax` to the
   cgroup's `memory.reclaim` and logs `memory.peak`, file and anon before
   and after. This returns the browser's page cache, which removal would
   otherwise reparent. `CHOIR_CAPSULE_TEARDOWN_RECLAIM=off` switches it off
   for the report's control run. The memory-lease hook
   (`vmctl.WithGuestMemoryLease`) is still only on `feat/memory-hotplug`
   (not an ancestor of `origin/main` at `133de45fa`), so this slice leaves
   the seam: `capsule.SetJobAdmission` is on main, and
   `visualcapture.IsCaptureJob(spec.Argv)` lets an admission hook size a
   browser lease differently from a build.

### Why the Chromium CLI and not Playwright

Playwright would need the nixpkgs `playwright-driver` (Node plus a driver
whose minor version must match its browser build) and a Node client in the
capsule. The capture needs one viewport PNG per route. Chromium's own
`--headless --screenshot` does that offline with one process tree per
shot, no driver, and no version pairing. It cannot wait for a named element,
so it uses `--virtual-time-budget` (Chromium advances virtual time until the
page is idle or the budget runs out) and `--force-prefers-reduced-motion`.
A ready-selector wait, filmstrips and video are the next step (a CDP client
in Go, which `internal/browsercontrol` already speaks for Obscura).

### The gate, end to end

```
vmctl (host)                       guest core                     capsule
CHOIR_VISUAL_CAPTURE_COMPUTERS ->  /run/choir-bootstrap/          broker env:
  ids | class:test                   visual-capture.json  ->        CHOIR_CAPTURE_BROWSER
cold boot writes the grant         executor reads at spawn          CHOIR_CAPTURE_FONTCONFIG
                                   (granted + binary present)     job env only (never a cell's)
                                                                  choir-capture refuses without it
```

vmctl decides `class:test` at cold boot from the computer's warmness class.
The test tier is refreshed from the auth DB by the idle sweep, so a computer
minted a moment ago may not be in it yet; on a miss vmctl reloads the tier
once before deciding (V2).

## 2. API as the desk sees it

```go
// after the frontend build cell (release tree holds frontend/)
id, err := choir.Start([]string{"choir-capture", "--route", "/", "--route", "/choir-drill/x.html"},
    map[string]any{"minutes": 5})
// woken when it ends; choir.Poll(id).Tail is its JSON summary
choir.Precommit(map[string]any{"question": "Freeze carries a screenshot of /",
    "distribution": map[string]float64{"yes": 0.8, "no": 0.2}, "resolver": "rule",
    "rule": map[string]any{"kind": "visual_captured", "routes": []string{"/"}}})
// Freeze result: "visual_captures": [{"artifact": "artifact:sha256:...", "route": "/"}]
```

Flags: `--route` (repeatable, default `/`, at most 6), `--width`/`--height`
(default 1280×800, at most 1920×1200), `--wait-ms` (virtual time budget,
default 5000, at most 20000). Output directory and served root are fixed.

## 3. Failure modes (written before the code)

Gate:

- **V1. An ungranted computer launches Chromium.** Guard: the binary is on
  no PATH; only a granted computer's capsule brokers carry
  `CHOIR_CAPTURE_BROWSER`, and only into the job environment (cells and
  direct exec build theirs from allowlists that omit it); `choir-capture`
  refuses with "visual capture is not granted on this computer". Residual
  (named, as for the claude binary): a cell that names the store path can
  exec Chromium directly, because the store is readable for the toolchain.
  The grant is a capability gate, not a security boundary; the capsule
  cgroup (memory, CPU, pids) bounds the cost either way.
- **V2. `class:test` is decided from a stale test tier.** A fresh test
  account is not yet in the tier the idle sweep refreshes. Guard: on a miss
  at boot, vmctl reloads the tier from the auth DB once. A failed read
  keeps the old set, so the decision fails closed (no grant).
- **V3. The grant outlives the policy.** The grant is written at cold boot;
  a snapshot resume keeps the old credential disk. Guard: none in the
  slice; to revoke, empty the list, deploy and cold-boot the computer (the
  same rule as the delegate). The grant carries `granted_at` and its basis.
- **V4. A capsule forges or reads the grant.** Guard: the credential disk
  is root-only at `/run/choir-bootstrap`, which the capsule's tmpfs `/run`
  masks; only guest core reads it.

Capture:

- **V5. Chromium hangs** (a page that never settles, GPU init). Guard: each
  shot has a hard 60 s deadline that SIGKILLs Chromium's process group; the
  job wall clock is above it.
- **V6. Chromium leaves orphans** (zygote, crashpad). Guard: its own
  process group, killed after each shot; the job supervisor's subreaper
  reaps whatever escaped (capsule jobs F2).
- **V7. The page reaches the network.** Guard: `--no-proxy-server`, proxy
  variables stripped from Chromium's environment, host resolution mapped to
  nothing except 127.0.0.1, background networking, sync and component
  updates off. The capsule's only egress, if any, is the allowlisted proxy.
- **V8. The screenshot shows a different build than the one frozen** (a
  capture before a rebuild). Guard: the manifest binds the sha256 of the
  served `frontend/` tree (sorted path and file digests). Freeze recomputes
  it from the staged release and refuses a mismatch: "visual evidence was
  captured from a different frontend build; capture again after the last
  build".
- **V9. A nondeterministic image** (the frontend animates on load). Guard:
  reduced motion and a virtual time budget. Residual: a page that animates
  regardless.
- **V10. A large capture exhausts the capsule.** Guard: viewport only, never
  full page; at most 1920×1200 and 6 routes; one Chromium at a time; the
  report measured 0.4 to 0.55 GiB per shot inside the 2 GiB capsule.
- **V11. The PNG is not the shot asked for** (truncated, wrong size, blank).
  Guard: the PNG is decoded and must be exactly the viewport size; a
  uniform image is marked `"blank": true` in the manifest, not refused.
- **V12. Text renders as boxes** (no fonts in the capsule). Guard: an
  explicit `FONTCONFIG_FILE` from the image naming DejaVu.
- **V13. Chromium cannot run under the capsule's seccomp, Landlock or mount
  layout.** The report measured seccomp only. Guard: the tool reports
  Chromium's exit and a stderr tail; the first staging run is the test
  (UNVERIFIED).
- **V14. The capsule has no inet** (a capsule without egress loads the
  workload filter that refuses AF_INET, so even loopback fails). Guard: the
  tool names the cause ("needs the capsule's loopback network: a capsule
  with the ecosystem network"). The engineering default is the ecosystem
  network, which a frontend build needs anyway for `npm ci`.
- **V15. Capture output lands in the source tree.** Guard: the output
  directory is fixed under `/var/lib/artifact/evidence/visual/`, outside
  `/workspace/platform`; the served root is the release tree.
- **V16. A route needs live data.** Guard: the backend answers at once as
  a signed-out one: `/auth/session` 200 `{"authenticated":false}` (what the
  auth service answers with no session cookie), other `/auth/` routes 401,
  `/api/` 503 JSON, so the page renders its signed-out state instead of
  waiting. (Until 2026-10-25 `/auth/` answered 503, which the app shell reads
  as a transient auth failure and stays on Loading; problem 6 in
  docs/evidence/selfdev-trace-reviews-2026-10-25.md.) Residual: screens that need data (mocks or a candidate binary in
  the capsule are the report's next axis).

Evidence:

- **V17. A cell hand-writes a PNG and a manifest.** Guard: Freeze records
  the receipt of the latest `choir-capture` job that ended done in this
  run; `visual_captured` settles yes only with that receipt. The rule never
  says the image is right, only that a capture of the frozen frontend
  exists (report §5.1: "a screenshot existing is not the screenshot being
  right").
- **V18. The new bundle field changes old bundles' digests.** Guard: an
  omitempty pointer, so a bundle without captures canonicalizes exactly as
  before; the in-cell inspection mirror gains the same field (its drift
  test pins it).
- **V19. Evidence bloats bundles and disk.** Guard: at most 6 captures,
  each at most the gateway's image limit (20 MiB), 24 MiB in all; it
  follows the bundle's retention; the image store copy is the only other.
- **V20. Evidence is installed or served as part of the release.** Guard:
  it lives beside `release/`, not under it; the materializer installs
  `RuntimeFiles` only.
- **V21. A partial manifest** (the job died mid-write, or a restart closed
  it). Guard: the tool writes the manifest last, by rename; Freeze refuses
  a manifest that names a file it cannot find or hash.
- **V22. The image store write fails.** Guard: Freeze does not fail on it
  (the bundle is the record); the result names the failure, and the
  route's `resolvable` flag and the drill catch it.
- **V23. `visual_captured` is settled from the desk's word.** Guard: it
  settles only from the Freeze observation the runtime built; a precommit
  after the candidate froze is refused (rule R2).

Review:

- **V24. A vision answer is treated as settlement.** Guard: the answer is
  management's verifier input, cited in its Verify; no rule reads it.
- **V25. The review loops or costs too much.** Guard: prompt-level only in
  this slice (one call per candidate, the captures the Freeze names).
  Residual: no runtime cap.
- **V26. The evidence route becomes a general image read.** Guard: owner
  authentication, and `?ref=` serves only a ref the operation's bundle
  cites; everything else is 404.

Memory:

- **V27. The browser's page cache outlives the capsule.** Guard: teardown
  reclaim (part 5), applied to every capsule because builds leave cache
  too; `EAGAIN` is not an error.
- **V28. Reclaim stalls teardown.** `destroy` holds the executor lock.
  Guard: one synchronous write of clean cache (milliseconds to a few
  hundred for 2 GiB), its duration logged. Residual: not bounded by a
  timer.
- **V29. No lease is held while Chromium runs.** Guard: none yet; the seam
  is named (part 5). The capsule's own budget holds the browser.

Restart:

- **V30. A restart mid-capture.** The capsule and the job end; the job is
  closed interrupted (capsule jobs F19); nothing resumes it, and a later
  Freeze sees no manifest or a stale one (V8, V21).

## 4. Tests

- `internal/visualcapture`: the manifest digest of a frontend tree; PNG
  checks (size, blank); flag bounds; the SPA server (fallback, 503 for
  `/api/`, no escape from the root); a full capture with a fake browser
  script (the argv it receives, the files and manifest it leaves, the
  refusal without a grant, a hang killed at the deadline).
- `internal/capsule`: grant reading; evidence staging and its refusals
  (hash mismatch, frontend mismatch, extra file, oversize, missing
  manifest); the broker job environment carries the capture keys only when
  granted.
- `internal/types`: `visual_captured` validation, settlement at Freeze
  (yes, no without a receipt, no on a missing route) and at the terminal.
- `internal/capsule/transaction` and `yaegikernel`: a bundle without
  captures keeps its digest; the inspection mirror reads the new field.
- `internal/vmmanager`: the grant for a listed id, for `class:test`, and
  none otherwise.
- `internal/runtimeprompts`: the overlays carry the capture and review
  sections only when granted.

## 5. The drill

`scripts/reeng_tracer.mjs --scenario engineering --visual`: the change is a
visible frontend page, `frontend/public/choir-drill/<token>.html`, whose
heading is the token. Engineering builds the frontend, runs
`choir-capture --route /choir-drill/<token>.html`, precommits
`visual_captured`, and freezes. The drill claims: Freeze's bundle cites a
capture of that route; the image ref resolves in the computer's image store
(the visual-evidence route says so and serves its PNG, kept in the trace
directory); the PNG is 1280×800. Observed, not claimed: a vision model's
reading of the heading through `POST /api/model-policy/verify`.

```
node scripts/reeng_tracer.mjs --label visual-<n> --scenario engineering --visual \
  --expect-sha <sha> --timeout-min 30
```

## 6. Residuals

- **R1.** Direct exec of the store path (V1).
- **R2.** No ready-selector wait, filmstrip or video (CDP client next).
- **R3.** Screens that need live data (V16).
- **R4.** No memory lease while Chromium runs (V29); `memory.peak` is
  logged at teardown, not yet a field of a capsule receipt.
- **R5.** Revocation needs a cold boot (V3).
- **R6.** The host RSS return is UNVERIFIED until the report's §8.2 probe
  runs on staging; this slice adds no RSS field to vmctl's VM view.
- **R7.** Review is prompt-taught; no runtime bound (V25).

## 7. Mutation class, protected surfaces, rollback

- Guest image (Chromium, fonts, `choir-capture`): **red** (guest image;
  invalidates snapshots on deploy, as every guest image change does).
  Rollback: drop the packages and redeploy.
- vmctl boot input (the warmness class reaches vmmanager) and the grant on
  the credential disk: **red** (VM lifecycle, credential disk). Rollback:
  empty `CHOIR_VISUAL_CAPTURE_COMPUTERS` and deploy; revert.
- Capsule teardown reclaim, the job environment, Freeze evidence staging,
  the bundle field, the rule, the route, the prompts: **orange**.
  Rollback: revert; no stored data migrates (the field is omitempty).

## 8. Red ceremony

- **Conjecture delta.** Visual evidence bound to a frozen candidate is a
  receipt the runtime can hold (existence, binding) and a model can judge
  (content); the two stay separate. A browser inside the capsule needs no
  new memory mechanism and no egress.
- **Protected surfaces.** Guest image; vmctl boot config; the credential
  disk; Freeze's evidence (tightened only: a bundle that cites captures
  must bind them, nothing previously refused is admitted).
- **Admissible evidence.** Local: unit tests and a local capture with a
  real Chromium (proves the tool, not the capsule). Staging: the drill in
  §5 on a minted test computer; the report's §8.2 probe for the memory
  return.
- **Heresy delta.** Discovered: a capsule without egress cannot use
  loopback inet at all (V14); the freeze bundle had no slot for evidence
  that is not a runtime file. Introduced: a store path a cell can exec
  without the grant (V1, named). Repaired: capsule page cache reparented
  at teardown (V27).

## 9. Resumed 2026-10-20

Owner direction (2026-10-20): "get Playwright working so that we can make
screenshots and videos of work inside capsules. So then users can approve
evidence they can see." The slice was paused on 2026-10-15 because the
local Chromium runs loaded the owner's Mac. This pass launched no browser
and built no guest image locally; it used Go tests and `nix eval`.
Mutation class of the pass: **orange** (capsule), **red** (guest image,
vmctl grant, node-b), **green** (this section).

### 9.1 What was rebased

Branch `feat/visual-evidence-resume-2026-10-20`, from `main` at
`73483585a`. The four commits of `feat/visual-evidence-slice` (base
`133de45fa`, about 580 commits behind) were cherry-picked in order: the
design (this file), `choir-capture`, the guest image plus vmctl grant, and
the unfinished capsule side. Two commits follow: the capsule fix and tests
(orange), and node-b's grant list set to empty (red).

Conflicts, all in the vmctl-to-vmmanager config path (`cmd/vmctl/main.go`,
`internal/vmctl/ownership.go`, `internal/vmmanager/manager.go`): adjacent
additions beside main's `OfflineMaintenance` hook (store compaction) and
`MemoryHotplug*` fields. Resolved by keeping both sides; no logic overlaps.
`nix/autoputer-vm.nix`, `flake.nix` and `nix/node-b.nix` merged cleanly
(the Claude delegate's grant files and the visual grant share
`createCredentialDisk`'s file map, merged in `bootVM`).

### 9.2 What changed in main that mattered

- **The capsule cgroup is an interface now** (`capsuleCgroup`), so the WIP's
  `CgroupManager.TeardownReclaim` did not compile. Teardown reclaim now
  goes through the `cgroupMemory` seam that job memory boosts added
  (`teardown_reclaim.go`), asks for the cgroup's whole charge instead of
  `MemoryMax`, and takes a fake in tests.
- **Memory hotplug and job leases landed** (`memory_mib` on `choir.Start`,
  `WithGuestMemoryLease`). V29 (no lease while Chromium runs) is closable
  without new code: teach the capture call
  `{"minutes": 5, "memory_mib": 768}`. Hotplug itself stays off by default
  (`VM_MEMORY_HOTPLUG`), so on staging today the browser still runs inside
  the capsule's 2 GiB.
- **Chromium moved** from 150 to 155.0.8059.79 in the pinned nixpkgs; its
  closure is 1,833,464,976 bytes (1.71 GiB) per `nix path-info -S` against
  cache.nixos.org. The delta against the whole guest closure is still
  UNVERIFIED (the report estimated 0.7 to 0.8 GiB on top of LibreOffice).
- The test tier moved into `OwnershipRegistry` (`warmnessClassForOwnership`);
  `bootWarmnessClass` (V2) was the only vmctl change needed.

### 9.3 Capsule side, corrected

- Teardown reclaim, tested over a fake and a temp cgroup directory
  (`internal/capsule/teardown_reclaim_test.go`): reclaims the whole charge;
  EAGAIN is `partial`, any other error is logged and never blocks
  teardown; `CHOIR_CAPSULE_TEARDOWN_RECLAIM=off` is the control run.
- Staging split so the refusals run without a capsule
  (`stageEvidenceFiles` with an injected opener; the Linux wrapper only
  does `Openat2`).
- The capture receipt now comes from the frozen capsule's own jobs (the WIP
  used every job of the run) and only from a `done` `choir-capture` job
  whose output carries the manifest's sha256 (`choir-capture` prints it,
  `visualcapture.Summary`). A manifest rewritten after the job gets no
  receipt, and `visual_captured` will not accept it.
- Freeze decodes every capture at the manifest's viewport and recomputes
  `blank`; it no longer trusts the manifest for either.
- Every staged file is scanned for secrets, and a refused directory writes
  nothing into the bundle area.

New failure modes found in this pass:

- **V31. A secret rides in a PNG or the manifest.** Guard: the release
  scan's rules (binary windows with a carry for captures, per line for the
  manifest). Test: a private key appended after the image data.
- **V32. A receipt from another capsule, or for another manifest.** Guard
  above. Test: the ten receipt cases in `visual_evidence_test.go`.
- **V33. Freeze believes the manifest's `blank` or size.** Guard: decode at
  Freeze. Test: lying `blank` both ways, a wrong-size and a non-PNG file.

### 9.4 Tests and checks (this pass)

Run on Darwin: `internal/visualcapture`, `internal/capsule` (new:
`visual_evidence_test.go`, `teardown_reclaim_test.go`,
`visual_capture_env_test.go`), `internal/capsule/transaction`,
`internal/vmmanager`, `internal/vmctl`, `cmd/vmctl`, `internal/autoputer`:
all pass. `GOOS=linux go vet` of `internal/capsule` and `cmd/capsule-broker`
(tests included) is clean; `cmd/capsule-broker/visual_capture_test.go` (V1,
the job manager is the only broker child that carries the capture keys) is
linux-only and was compile-checked, not run. `scripts/check-heresies.sh
--fail-on-regression` exits 0 (`/tmp/heresy-visual.json`). `nix eval` (no
build) shows: `go-choir-autoputer-vm` carries `CHOIR_CAPTURE_BROWSER_BIN`
(Chromium 155.0.8059.79), `CHOIR_CAPTURE_FONTCONFIG_FILE`, the credential
directory, and `choir-capture` among the system packages; `go-choir-b`'s
vmctl environment has `CHOIR_VISUAL_CAPTURE_COMPUTERS=` (empty: nothing is
granted). There is no dedicated nix check script for this.

### 9.5 Still design only (as of the first pass; items 1 to 3 and the route, listing and drill are built in section 10)

Not written: the `visual_evidence` bundle field and the in-cell inspection
mirror (V18), Freeze wiring in `agentcore` (image store write, the result's
`visual_captures`, V22), the `visual_captured` rule in `internal/types`
(validate, settle at Freeze, R2), the prompt sections (capture and `memory_mib`
for engineering, vision review for management), the read route (V26), and
`reeng_tracer.mjs --visual`. The capsule side is the last piece that had
no tests; the rest has none yet either.

### 9.6 Remaining path to a staging proof

1. Bundle field plus inspection mirror (omitempty pointer; drift test).
2. Freeze wiring: stage `Visual` into the bundle directory, write each
   capture to `internal/imageblob`, return `visual_captures`.
3. Rule `visual_captured` (optional `routes`), settled from the Freeze
   observation only; test yes, no without a receipt, no on a missing route.
4. Read route (owner-authenticated; `?ref=` only for refs the operation's
   bundle cites).
5. Prompts, including `memory_mib` on the capture job.
6. Tracer `--visual` drill (section 5).
7. Red deploy (9.7), then: set `CHOIR_VISUAL_CAPTURE_COMPUTERS` to one
   named test computer, deploy, cold-boot it, run the drill, and run the
   report's section 8.2 probe (host RSS of the Firecracker process returns
   to within 128 MiB of its pre-browser value within about 2 minutes), with
   a control run under `CHOIR_CAPSULE_TEARDOWN_RECLAIM=off`. V13 (Chromium
   under the capsule's seccomp, Landlock and mounts) is first tested here.
8. The owner-facing display (9.8).

### 9.7 Red ceremony for the guest-image change

- **Conjecture delta.** Unchanged from section 8, plus: a capture receipt is
  bound to its manifest by the job's own output, so existence and binding
  stay runtime facts and content stays a model or human judgment.
- **Protected surfaces.** Guest image (store closure +1 Chromium, fonts, one
  Go tool); vmctl boot input (`WarmnessClass`) and the credential disk's
  file set; node-b's vmctl environment; Freeze evidence (tightened only).
- **Admissible evidence.** Local: the tests above and the `nix eval`
  results. Staging: the drill and the 8.2 probe. A local browser run is not
  evidence for the capsule and was not made.
- **Rollback path.** Revert the red commits (097211c41 and c307acaab) and
  deploy; or leave the image and keep the list empty, which is the
  shipped state. A guest-image change invalidates snapshots at deploy as
  every guest-image change does, so each computer cold-boots once.
- **Heresy delta.** Discovered: the WIP counted every job of a run for the
  capture receipt, and trusted the manifest's `blank` and size. Introduced:
  the V1 residual (a cell can exec the store path without the grant); the
  guest store image grows by the Chromium closure. Repaired: both
  discovered items; capsule cache reparenting at teardown (V27), pending the
  staging probe.

### 9.8 Open owner decisions (recommendations)

- **Grant scope.** Start with one named test computer, then `class:test`.
  Never the default for a primary or the owner's computer until the 8.2
  probe shows memory returns.
- **Approval surface.** Texture. It already renders the commitment ledger
  into the owner-facing supervision surface, and Freeze's captures are
  evidence on that ledger; a second approval UI would fork the authority.
  The read route stays as the API and drill instrument, not a product view.
- **Video.** Not yet. Recording takes every CPU it is given and runs 2 to 3
  times slower than real time (report section 2). Ship viewport screenshots
  first, then a 3 to 5 frame filmstrip over CDP, and treat video as a
  separate one-at-a-time job class with its own lease.

## 10. Built 2026-10-20 (second pass), aligned with the Texture media design

Owner ruling (owner notes section 28): visual evidence is reviewed inside
Texture ("embedding this inside texture is the right approach"), and the
owner wants screenshots and videos. The Texture design
([texture-media-and-autoradio-design-2026-10-20.md](../reports/texture-media-and-autoradio-design-2026-10-20.md))
makes the asset route and the `capsule_evidence` embed the review surface.
This pass built the producer side of that contract and left the embed and the
owner display to its step T1. Sections 9.5 and 9.6 listed the design-only
items; items 1 to 3 below are those, built.

### 10.1 What was built

1. **Bundle field and Freeze.** `visual_evidence` on the capsule effect
   bundle (omitempty pointer: no captures, same digest as before; the content
   digest covers captures; they must answer to the frontend the bundle
   freezes). The verifier's in-cell inspection mirror reads it and re-hashes
   each staged capture. Freeze files every capture in the computer's asset
   store, returns `visual_captures` (artifact, route, blank, resolvable) and
   `visual_capture_receipt` to the cell, and does not fail on a store error
   (`visual_store_errors`, V22). The capture directory classifies as the
   artifact ledger, so Freeze does not reject the candidate for unknown paths.
2. **Rule `visual_captured`** (optional `routes`, at most 6). It settles from
   the runtime's Freeze observation only: a non-blank capture of each named
   route and the capture job's receipt (V17, V23). A replayed freeze observes
   nothing, so the rule stays open; at the terminal it is no (no candidate)
   or unresolved (a candidate froze, observation lost).
3. **Prompts, only on a granted computer.** Engineering: `choir-capture` as a
   job with `"memory_mib": 768` (closes V29: the job leases the browser's
   memory instead of competing with the build), the rule, and the bounds.
   Management: one vision call before Verify through
   `/api/model-policy/verify` with `role: verifier_multimodal` and the
   capture's `image_ref`, cited in the Verify and not a settlement (V24, V25).
4. **The asset route** `GET /api/assets/sha256/<hex>`: owner-authenticated,
   range requests, `Content-Type` from the stored kind, `nosniff`, a sandbox
   CSP, `private, max-age=31536000, immutable`, the digest as ETag, and a
   re-hash on every read. `internal/imageblob` gained the asset view: typed
   kinds (image served; audio and video are typed TODOs the store refuses),
   digest validated before the filesystem, tampered and unservable bytes
   refused. The proxy forwards `/api/*` as a reverse proxy, so ranges and
   validators pass through.
5. **The thin listing** `GET /api/computers/{id}/self-development/operations/{op}/visual-evidence`:
   which digests belong to the operation's frozen bundle, with the capture
   receipt and a per-capture `resolvable` flag; 404, not an empty list, for a
   candidate with no captures; the source-patch export's binding rules.
6. **The `--visual` drill** in `scripts/reeng_tracer.mjs` (`--scenario
   engineering --change static --visual`): the checks are pure predicates
   (`visualEvidenceChecks`, `node --test`), the PNG is kept in the trace
   directory, and the vision reading is observed, not claimed.

Tests (Darwin, all pass): `internal/types`, `internal/imageblob`,
`internal/capsule/...`, `internal/yaegikernel`, `internal/runtimeprompts`,
`internal/apihandler`, `internal/proxy`, and `internal/agentcore` run with
`-run 'Precommit|SelfDevelopment|Freeze|Asset|Visual|Rule|Overlay|Capsule|SystemPrompt|ToolProfile|Census|Product'`;
`node --test` on the four tracer test files (66 tests). Not run: the full
`internal/agentcore` suite; anything Linux-only (the capsule executor, the
broker tests); `freezeCapsuleEffectBundle` end to end (it needs the Linux
executor, so the Freeze wiring is tested as `recordFreezeVisual` and
`applyFreezeVisual`); the drill itself.

New failure modes found:

- **V34. The asset route reads any digest in the owner's store**, not only
  evidence. Accepted on the owner's own computer (owner-authenticated, one
  owner per store). It must not become the public route: T2's public asset
  route is separate and serves only pinned public digests.
- **V35. The asset store has no retention.** A capture outlives its bundle's
  retention, and nothing deletes it. Texture revisions that cite an asset
  must pin it, so a later store GC cannot orphan a published or past
  revision (T1 states the policy before any GC exists).
- **V36. The listing and the bytes disagree** (a store write failed, V22, or
  a blob was swapped). `resolvable` reports presence only; integrity is the
  asset route's re-hash, and the drill checks that served bytes hash to the
  listed digest.

### 10.2 Residuals that remain from section 6

R1 (direct exec of the store path), R2 (no ready-selector wait; filmstrip and
video are 10.4), R3 (screens that need data), R5 (revocation needs a cold
boot), R6 (host RSS return unverified until the staging probe). R4 is
narrowed: `memory.peak` is in the capsule teardown log line, and the job
lease is requested, but neither is a field of a capsule receipt yet. R7
(review is prompt-taught) stands, with the vision call bounded in the prompt
only.

### 10.3 What T1's `capsule_evidence` embed needs from this work

T1 is a red schema change and is not built here. These are the fields it
reads and the rules it keeps. The embed in the Texture design is
`capsule_evidence | event-artifact digest + operation id`.

| Embed field | Source | Notes |
| --- | --- | --- |
| `target.digest` | the listing's `digest` (64 lowercase hex) | One capture. The same sha256 as the `artifact:sha256:` ref in the bundle. Renders `<img src="/api/assets/sha256/<digest>">`; the owner's session cookie authorizes it. |
| `target.operation_id` | the listing's `operation_id` | Binds the embed to the candidate. |
| candidate identity | listing: `bundle_digest`, `frontend_sha256`, `capture_receipt` | `bundle_digest` is the operation's current digest (the draft digest until Verify finalizes it); `frontend_sha256` says which build; `capture_receipt` says which job vouches. Texture shows them on the card and does not store them in the revision (they re-resolve live). |
| viewport | listing: `width`, `height` | Lets the renderer reserve the box before the image loads. |
| `route` | listing: `route` | Shown with the caption and in the audio rendition. |
| blank flag | listing: `blank` | Renderers badge a blank capture "blank"; Texture should not present all-blank evidence as support for approving. |
| `resolvable` | listing: `resolvable` | False renders the "evidence not retained" card, not a broken image. |
| `caption` | written by Texture | The writer is Texture, never engineering. Default: "Screenshot of `<route>` from candidate `<operation_id>` (1280x800)". |
| `alt` | written by Texture | The capture has no description. Default: "Screenshot of `<route>`". A vision reading (the management review) may refine it, and is cited, not copied unchecked. |
| `privacy` | derived, always `private` | Publication must refuse a document that holds one (T2). |

Rules T1 must keep:

- **Membership check at write.** Texture's writer accepts a `capsule_evidence`
  embed only when the operation exists on this computer and its bundle's
  listing contains the digest, so a document cannot present an arbitrary
  stored image as evidence of a candidate. The renderer re-checks through the
  listing; a 410 (bundle not retained) renders the not-retained card while the
  asset route may still serve the pinned bytes (V35).
- **Pin on cite.** A revision that embeds a capture pins the asset (V35).
- **One kind of target.** The digest resolves through the asset route, not
  through the event artifact store. The Texture design's phrase
  "event-artifact digest" should read "asset digest" for this embed: the
  captures are image-store assets whose ref spelling happens to be
  `artifact:sha256:` (see 10.5).
- **The approval act stays an owner act.** The embed shows evidence beside the
  approve control; it carries no authority, and the decision route is
  unchanged.
- **A filmstrip is several digests.** Until the `selector` field carries
  frame ranges, a filmstrip is a short list of `capsule_evidence` embeds in
  capture order (10.4).

### 10.4 Filmstrip first, video as its own job class (design only)

The owner wants videos, so this is sequenced, not dropped. The report
measured recording at one full core (1.0 at a one-CPU cap, 1.9 at two), 2 to
3 times slower than real time, which is why it is a separate class.

1. **Screenshots** (built).
2. **Filmstrip, 3 to 5 frames.** Needs a CDP client in Go
   (`internal/browsercontrol` already speaks CDP to Obscura): load the page,
   wait on a ready selector, and capture at chosen virtual times. It reuses
   everything above: each frame is a PNG capture, named by its sha256, with
   `group` and `frame_index` added to the manifest and to
   `VisualEvidenceCapture` (omitempty, so old bundles keep their digests),
   still bounded by the 6-capture and 24 MiB limits. `visual_captured` keeps
   its meaning; a new rule is not needed. The Texture embed shows a strip, or
   a list of embeds in order.
3. **Animated image.** An APNG or GIF from the same frames, encoded in pure Go
   (no ffmpeg in the guest), is the first motion artifact: `<img>` plays it,
   the asset route serves it, and no new kind is needed (`image/apng`,
   `image/gif`; GIF is already a sniffed image type).
4. **Video.** A `choir-record` job class: one at a time per computer (the
   `SetJobAdmission` seam and `visualcapture.IsCaptureJob`-style detection
   already size jobs by class), a memory lease sized for recording
   (`memory_mib` at least 1536), a wall-clock cap, and an output cap per clip.
   Encoding needs a decision: ffmpeg in the guest image (measure the closure
   cost first; Chromium is already 1.71 GiB) or Chromium's own screencast
   frames encoded in Go. It adds `KindVideo` to the asset store, with the
   range route already in place (`http.ServeContent` answers ranges), a
   per-kind size cap, and a Texture `video` embed with `poster` (the first
   filmstrip frame). The same grant gates it; a computer can be granted
   screenshots and not video.

Failure modes to write before step 4: a recorder that never stops (wall clock
and process-group kill, as V5); two recordings at once (admission by class);
a clip larger than the cap (cap before write); a clip that plays different
content than the frozen build (bind the recording to the same frontend digest
as V8); the lease not returned (teardown reclaim, V27, and the lease release).

### 10.5 Where this conflicts with, or sharpens, the Texture design

- **"Event-artifact digest" is the wrong store** for `capsule_evidence` (see
  10.3). The target is an asset digest read through `/api/assets/sha256/`.
  The Texture design already says to generalize the image store into the media
  asset store, so this is a wording fix, not a design change.
- **The asset store is not yet the media store.** Only the kind typing and the
  route exist. The per-kind size cap, audio and video types, and PDF are T1 and
  T4. `Put` still enforces the gateway's 20 MiB image limit.
- **Per-operation versus owner-wide reads.** The listing is bound to a
  computer and an operation (it needs `X-Authenticated-Computer`); the asset
  route is owner-wide (V34). T1's resolver should go through the listing for
  evidence and not assume the asset route proves membership.
- **API-key scope.** `/api/assets/` is forwarded by the generic `/api/*`
  path, which checks the key's `read:runtime` scope, not the self-development
  read scope the listing uses. A key that can read the runtime can read any
  digest. Acceptable now (the owner's keys); T2's public route must not
  inherit it.
- **CSP.** The Texture design's `img-src 'self'` admits the asset route. The
  asset response carries its own `sandbox` CSP, which does not restrict its
  use as an `<img>`.
- **Order against S6.** Nothing here touches the schema, so T1 can run beside
  S6 as the Texture design says. What T1 should not start without is the
  producer side (this section) deployed on staging and the drill green.

### 10.6 Left before the red guest-image deploy

1. Run the Linux-only tests in CI (the executor staging path, the broker job
   environment, `internal/agentcore` in full) and fix what they find; none was
   run on Darwin.
2. `nix eval` the touched configurations again after rebase (done once for the
   guest and node-b on 2026-10-20; no build).
3. Choose and record the first grant: one named test computer in
   `CHOIR_VISUAL_CAPTURE_COMPUTERS`.
4. Read the red ceremony in 9.7 against the extra surfaces this pass touched:
   the bundle schema and digest (Freeze evidence), a new owner-readable
   route, and the proxy classifier. Rollback is a revert of the commits; the
   bundle field is omitempty and nothing migrates.
5. Deploy, cold-boot the granted computer, run
   `node scripts/reeng_tracer.mjs --label visual-<n> --scenario engineering
   --change static --visual --expect-sha <sha> --timeout-min 30`, then the
   section 8.2 probe with the `CHOIR_CAPSULE_TEARDOWN_RECLAIM=off` control.
   V13 (Chromium under the capsule's seccomp, Landlock and mounts) is first
   tested by that drill, and it is still the main unknown.
