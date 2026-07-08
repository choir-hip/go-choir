# Overnight VText, Super Console, Zot, And Source-Mounted Repair Report

**Date:** 2026-05-31  
**Window covered:** approximately 02:00 to 08:35 America/New_York  
**Primary staging target:** `https://choir.news`  
**Primary repo:** `go-choir`  
**Purpose:** give the owner a single review document for the overnight work on
VText stability, Trace removal, Super Console, real Zot, gateway routing, source
mounts, candidate repair, and platform promotion.

Doctrine note (2026-06-13): Trace and Terminal terms in this report are kept as
historical evidence of the cutover. Current doctrine is Trace as evidence and
Super Console/zot as the repair surface.

## 0. Executive Summary

The night started with the VText version-advancement regression and a deeper
product concern: the system kept almost working, then regressing. The strategic
decision was not to add another hidden prompt classifier or workflow state
machine. The work moved in the opposite direction:

1. **Invert the VText state machine.** Conductor routes. VText writes canonical
   versions. Co-agents send durable updates/evidence. VText wakes and writes the
   next version. The old conductor-authored first draft and required-tool
   choreography were removed or collapsed.
2. **Hard-cut visual Trace from the product.** Trace app was unshipped as a
   desktop app. Machine-readable evidence, logs, APIs, and run artifacts remain.
3. **Replace raw Terminal with singleton Super Console.** Super Console is the
   repair surface inside a user computer. It is not the main driver of Choir.
4. **Make Super Console run real Zot.** The placeholder/stub loop was replaced
   with upstream Zot packaged into the sandbox/Node B Nix deployment path.
5. **Route Zot through Choir gateway.** Zot now talks through an
   OpenAI-compatible gateway route using the sandbox gateway token and defaults
   to `gpt-5.5` with medium reasoning.
6. **Make Zot useful, not just present.** The gateway was fixed so Zot receives
   complete streamed tool-call arguments. Then source mounts and source lineage
   were added so Zot can inspect, patch, test, build, restart, verify, and write
   evidence inside the computer.
7. **Promote universal fixes correctly.** Local Zot/candidate evidence did not
   automatically become a platform fix. Codex lifted reviewed fixes into the
   platform repo, ran tests, pushed to `main`, monitored CI/deploy, verified
   staging identity, and recorded rollback/residual risks.

Final platform state at the end of this window:

```text
Deployed behavior commit: 8e5d9aaa629537cd39680c53d2fcdba8caaa93c5
Docs evidence commit:     ea7ed744f... docs: record dev shell promotion evidence
Staging health:           ok
Proxy commit:             8e5d9aaa629537cd39680c53d2fcdba8caaa93c5
Sandbox upstream commit:  8e5d9aaa629537cd39680c53d2fcdba8caaa93c5
```

The main residual risk is now narrower: a Zot process that restarts the sandbox
runtime can lose gateway auth/session continuity and may need a follow-up Zot
invocation with a fresh token to finish evidence export. The dev-shell/source
mount/platform promotion objective was completed, but restart-continuation is a
real next repair-loop problem.

## 1. Highest-Priority Review Guide

Read in this order.

### P0: Product And Architecture Decisions

1. **VText should be simple.** The target path is:

   ```text
   user/app prompt
     -> conductor routes/opens VText
     -> VText writes v1
     -> VText sends durable co-agent messages when help is needed
     -> workers/super reply with durable evidence
     -> VText wakes and writes v2/v3
   ```

2. **Trace app is gone as a human debugging route.** Logs and evidence remain,
   but humans should not scan a visual Trace UI as the normal debugging loop.
3. **Super Console is repair mode.** It is singleton per user computer, backed
   by out-of-process Zot, and should not become multiple coding agents or the
   main automation surface.
4. **Zot is not part of the MAS.** It can read, edit, run commands, rebuild,
   restart, and verify inside the computer, but it is not a scheduler peer,
   appagent, VText writer, or co-agent.
5. **Local repair is not platform promotion.** A local Zot patch can be personal,
   reusable, or universal. Universal fixes must still go through platform review,
   CI, staging deploy, staging health identity, and deployed acceptance.

### P1: Behavior Proven On Staging

1. VText product path advanced versions on staging after simplification.
2. Trace and Terminal desktop app routes were removed from product-facing UI.
3. `/api/terminal/ws` now returns `410`, pointing to Super Console replacement.
4. Super Console opened a real singleton repair session.
5. Real Zot was packaged and then routed through the gateway.
6. Zot successfully read computer-local source lineage through gateway tool
   calls after the streaming compatibility fix.
7. Source workspace mounts were materialized in the user computer:

   ```text
   /var/lib/go-choir/files/Source/platform
   /var/lib/go-choir/files/Source/user
   /var/lib/go-choir/files/Source/candidate
   /var/lib/go-choir/files/Build
   /var/lib/go-choir/files/.choir/source-lineage.json
   ```

8. Zot completed a user-computer repair-loop proof for the dev-shell runtime
   link gap: inspect, test, build, restart, verify, write evidence.

### P2: Remaining Risks

1. **Zot restart-continuation:** when Zot restarts the runtime it is talking
   through, the first Zot process can lose gateway auth. We worked around that
   with a second Zot invocation using a fresh token. This should become its own
   documented mission or fix.
2. **Super Console UX polish:** the console became real, but the product-level
   repair UX is still early. It needs better error surfacing, session selection,
   and restart-resume behavior.
3. **Personal/reusable promotion is designed, not fully productized.** The
   universal platform path was exercised. Personal active-computer promotion and
   reusable AppChangePackage adoption remain future work.
4. **Unified logs are still mostly a substrate direction.** Evidence files,
   source lineage, Zot sessions, and existing APIs are usable, but the clean
   low-overhead unified-log directory shape is not finished as a polished system.

## 2. Commit Timeline

The following commits landed during the covered window, in chronological order.

| Time ET | Commit | Summary |
| --- | --- | --- |
| 02:42 | `22f8a8f` | `docs: record vtext simplification mission` |
| 02:52 | `1735aa1` | `docs: record vtext staging repro checkpoint` |
| 03:19 | `9519606` | `unship trace and simplify vtext flow` |
| 03:21 | `f1f63bc` | `fix sandbox zot session id test` |
| 03:28 | `84c8c4f` | `include zot in sandbox deploy package` |
| 03:45 | `9e9bf46` | `docs: record vtext stability mission acceptance` |
| 05:43 | `ecd19fd` | `docs: scope source-mounted super console repair` |
| 06:28 | `c396124` | `docs: record super console zot cutover problem` |
| 06:28 | `24e0525` | `super-console: run real zot` |
| 06:41 | `27b1c47` | `docs: record zot gateway credential gap` |
| 06:45 | `7376717` | `super-console: route zot through gateway` |
| 06:49 | `cfa7c2a` | `docs: record chatgpt instruction requirement` |
| 06:51 | `d673f9b` | `provider: default chatgpt instructions` |
| 07:00 | `6230499` | `docs: record tetramark backdrop regression` |
| 07:06 | `0a4eb0f` | `frontend: soften desk sheet backdrop` |
| 07:10 | `262efdf` | `docs: record source workspace bootstrap gap` |
| 07:14 | `9a725bc` | `sandbox: bootstrap source workspace for zot` |
| 07:17 | `76d6c3f` | `docs: record zot tool streaming gap` |
| 07:20 | `97b2f9d` | `gateway: buffer zot streaming tool calls` |
| 07:23 | `a8e3ea4` | `docs: record zot gateway promotion proof` |
| 07:25 | `23df128` | `docs: record worker source workspace gap` |
| 07:31 | `59bfd26` | `vmctl: project source workspace identity into guests` |
| 07:40 | `f1abd7c` | `docs: record deploy refresh identity gap` |
| 07:43 | `2ef575b` | `vmctl: rehydrate guest identity during VM refresh` |
| 07:48 | `6f944c3` | `docs: record boot contract refresh gap` |
| 07:49 | `cb5f29b` | `ci: full refresh VMs for boot contract changes` |
| 07:59 | `7127cc3` | `docs: record VM boot identity promotion` |
| 08:01 | `8e6a06d` | `docs: record empty source checkout gap` |
| 08:04 | `8a1e36c` | `sandbox: materialize source checkouts for super console` |
| 08:15 | `471b6aa` | `docs: record zot source repair evidence` |
| 08:16 | `f41ee18` | `sandbox: summarize source checkout state` |
| 08:20 | `4425b34` | `docs: record zot source fix promotion` |
| 08:21 | `a972e75` | `docs: record node b dev shell link gap` |
| 08:21 | `8e5d9aa` | `nix: expose icu runtime libs in dev shell` |
| 08:33 | `ea7ed74` | `docs: record dev shell promotion evidence` |

Overall diff from the first mission doc commit through the final evidence doc:

```text
73 files changed
5136 insertions
4110 deletions
```

The large deletion is intentional: `frontend/src/lib/TraceApp.svelte` was
deleted, and Trace-facing frontend tests/routes were removed or redirected.

## 3. Phase 1: VText Regression Review And Simplification

### 3.1 Problem

VText version advancement had regressed repeatedly. The owner described it as
the "nth regression" and wanted a month-plus review rather than a narrow fix.
The review found a recurring pattern: the system kept accumulating hidden
control surfaces around VText.

Examples of the old failure-prone machinery:

- conductor-authored first appagent versions;
- prompt classification as runtime control flow;
- `requires_worker_grounding` style flags;
- `next_required_tool` choreography;
- pending mutation gates and worker wake coupling;
- stale-head safety without a clean retry path;
- frontend dirty-head behavior that could hide or defer version advancement.

### 3.2 Decision

The repair direction was **state-machine inversion plus via negativa**:

- delete or collapse workflow/control scaffolding;
- make conductor route instead of authoring canonical VText content;
- let VText be the only canonical document/version writer;
- treat worker/super output as durable evidence, not direct canonical text;
- make VText wake from durable co-agent updates and write the next version.

This is the product-level core of the work. The later Super Console/Zot work
exists to make this system debuggable when it breaks, not to replace VText.

### 3.3 Implementation

Primary behavior commit:

```text
9519606 unship trace and simplify vtext flow
```

Important changes:

- Conductor no longer writes the first appagent VText version.
- VText writes the appagent first revision through the normal VText edit path.
- Prompt/tool scaffolding was reduced.
- VText tests were updated around the new first-writer contract.
- Trace app was deleted from frontend code and routes.
- Terminal app was replaced in the desktop registry by Super Console.
- Super Console/Zot session plumbing was introduced.

Representative files changed:

```text
internal/runtime/vtext.go
internal/runtime/tools_vtext.go
internal/runtime/runtime.go
internal/runtime/prompt_defaults/vtext.md
internal/runtime/vtext_prompt_unit_test.go
internal/runtime/vtext_test.go
frontend/src/lib/Desktop.svelte
frontend/src/lib/SuperConsoleApp.svelte
frontend/src/lib/apps/registry.ts
frontend/src/lib/TraceApp.svelte
internal/sandbox/terminal.go
internal/zot/session.go
```

### 3.4 Proof

The run checkpoint recorded deployed proof at commit:

```text
84c8c4f005db913cf47f5bc66e1bf55c10bfb224
```

Deployed VText proof:

```text
Prompt: "Write and run one tiny shell command that prints the SHA256 of the word choir..."
Submission: a63c8c8e-b229-4f88-934f-6ab2a357b382
Document: 1f74f922-106f-44da-a118-2528f56d48a2
User revision: 2b682fd9-a262-449e-a667-2cbb405c2b93
VText appagent revision: dc134b56-f118-44b3-b7fa-6c6acf1332f4
Later VText revision: d5a51a53-7d41-48b4-9886-d13b3f9d2d95
Command evidence: echo -n choir | sha256sum
Output: 1be0686a785a469ecfeba5a30f06d591c4e1f2135e0f5559a51e6cd4173f5327  -
```

Important interpretation: staging evidence supported the simple path:

```text
route -> VText v1 -> durable co-agent evidence -> VText v2
```

The old state-machine/control-flow surface was removed rather than patched with
more prompt taxonomy.

## 4. Phase 2: Trace Unshipping And Super Console Replacement

### 4.1 Product Decision

The Trace app should not be a human debugging surface. Logs and evidence remain,
but the visual Trace desktop app was hard-cut. This was deliberately not
"demote Trace" and not "keep emergency Trace." The product path should move
away from humans scanning Trace and toward Zot processing logs/evidence.

### 4.2 Terminal Decision

Raw Terminal should not be the user-facing app. The replacement is singleton
Super Console:

```text
User computer breaks
  -> owner/expert opens Super Console
  -> Zot runs out-of-process from MAS
  -> Zot reads logs/source/process state
  -> Zot diagnoses and patches
  -> Zot rebuilds/restarts/verifies
  -> Zot writes ordinary markdown/text evidence
  -> VText can open those artifacts
```

### 4.3 Implementation

`TraceApp.svelte` was deleted. Terminal-facing routing/tests were changed to
Super Console expectations.

Deployed Super Console proof at the VText mission checkpoint:

```text
No Trace desktop icon
No Terminal desktop icon
/api/terminal/ws returns 410 "terminal app has been replaced by Super Console"
One Super Console window opens
Zot session artifacts persisted under .choir/zot/sessions
```

Initial Super Console was not yet real upstream Zot. It could persist sessions
and run command escapes, but it still used an in-tree placeholder for normal
natural-language prompts. That became the next documented problem.

## 5. Phase 3: Real Zot Cutover

### 5.1 Problem

User screenshots showed Super Console repeating the same canned response:

```text
diagnosis report: /mnt/persistent/files/.choir/zot/sessions/zot-2/diagnosis.md
```

This was correctly called out as not Zot. If it is a stub, it is not Zot. The
product needed actual upstream Zot running as a separate process.

A second issue was readability: dark red text on black under London Salmon made
Super Console hard to inspect.

### 5.2 Build Feasibility

Upstream Zot:

```text
Repo: https://github.com/patriceckhart/zot
Version: v0.2.6
Commit: 917da8c414e183118e68034e0e8c6f6b746f0132
```

Node B build probe:

```text
nix build .#zot
Built zot 0.2.6
Host NixOS closure built in 122 seconds
Ordinary guest image built in 35 seconds
Playwright guest image built in 44 seconds
Host and guest sandbox configs set CHOIR_ZOT_PATH to the real Zot binary
```

### 5.3 Implementation

Primary commits:

```text
c396124 docs: record super console zot cutover problem
24e0525 super-console: run real zot
```

Important changes:

- Packaged Zot in `flake.nix`.
- Added Zot binary into Node B/sandbox deployment wiring.
- Updated Super Console frontend and backend launch path.
- Kept the in-tree fallback only as degraded fallback, not the normal path.
- Set `ZOT_HOME` inside the user computer persistent filesystem.

### 5.4 Resulting Gap

After deploying real Zot, Super Console no longer showed the placeholder loop.
It showed real upstream Zot, but Zot tried to run its own provider login flow.
That was still wrong for Choir. A repair console inside a user computer should
not ask the owner to manage third-party LLM credentials. This led directly to
gateway routing.

## 6. Phase 4: Gateway Routing For Zot

### 6.1 Credential Boundary

Zot should use Choir's gateway, not raw provider credentials. The gateway owns
provider secrets and the sandbox receives a runtime gateway token. The intended
shape:

```text
Zot process in user computer
  -> OpenAI-compatible localhost gateway route
  -> gateway authenticates sandbox token
  -> gateway calls platform-owned provider adapter
```

Zot defaults:

```text
provider: openai-compatible
base-url: http://127.0.0.1:8084/provider/openai/v1
model: gpt-5.5
reasoning: medium
token source: sandbox runtime gateway token
```

### 6.2 Implementation

Primary commits:

```text
27b1c47 docs: record zot gateway credential gap
7376717 super-console: route zot through gateway
cfa7c2a docs: record chatgpt instruction requirement
d673f9b provider: default chatgpt instructions
```

Important changes:

- Added OpenAI Chat Completions-compatible gateway route.
- Routed Zot launch defaults through that route.
- Preserved provider secrets outside Zot/browser/session state.
- Added compatibility tests around OpenAI chat completions.
- Fixed ChatGPT provider behavior when instructions were empty by supplying a
  harmless default instruction.

### 6.3 Resulting Gap

The gateway authenticated and generated model turns, but Zot tool calls arrived
with empty arguments:

```text
{"name":"bash","args":{}}
{"name":"read","args":{}}
```

Root cause belief: the gateway streamed tool-call name frames and argument delta
frames separately; Zot executed the tool call as soon as it saw the name frame.
For Zot, the gateway needed to buffer and emit complete streamed tool calls.

## 7. Phase 5: Zot Tool-Call Streaming Compatibility

### 7.1 Implementation

Primary commits:

```text
76d6c3f docs: record zot tool streaming gap
97b2f9d gateway: buffer zot streaming tool calls
a8e3ea4 docs: record zot gateway promotion proof
```

Important changes:

- Buffered OpenAI-compatible streaming tool calls for Zot.
- Added gateway tests proving streamed tool calls are complete.
- Preserved ordinary streaming behavior where safe.

### 7.2 Promotion Evidence

Platform fix:

```text
Commit: 97b2f9d2efbd8af1d665d97d20b521335a9aa065
CI run: 26711182329, success
Deploy job: 78721672005, success
FlakeHub run: 26711182331, success
Staging deployed_at: 2026-05-31T11:21:51Z
```

Deployed Zot proof:

```text
Zot emitted a read tool call with complete arguments:
{"limit":20000,"offset":0,"path":"/var/lib/go-choir/files/.choir/source-lineage.json"}

Zot successfully read the file and answered:
sandbox-m1
/var/lib/go-choir/files/Source/platform
```

This proved real Zot could use the promoted Node B gateway and receive complete
tool-call arguments.

## 8. Phase 6: Tetramark Backdrop Regression

### 8.1 Problem

User report:

```text
When tapping the Tetramark, the screen above the sheet goes black.
It should not do that.
```

Codex reproduced the issue on deployed `https://choir.news` with a mobile
viewport. Evidence:

```json
{
  "deployedTheme": "futuristic-noir",
  "backdropBackground": "rgb(0, 0, 0)",
  "backdropZIndex": "9998",
  "backdropRect": {
    "x": 0,
    "y": 0,
    "width": 390,
    "height": 844
  },
  "surfaceMedia": "#000000"
}
```

Root-cause belief: `DeskSheet.svelte` used
`background: var(--choir-surface-media)` for a full-viewport backdrop, while the
theme token resolved to black. That token is appropriate for media surfaces,
not for a menu/sheet backdrop.

### 8.2 Implementation

Primary commits:

```text
6230499 docs: record tetramark backdrop regression
0a4eb0f frontend: soften desk sheet backdrop
```

Important files:

```text
frontend/src/lib/DeskSheet.svelte
frontend/tests/desktop-shell-core.spec.js
```

This was a small universal UI fix. It was not the final repair-loop proof, but
it was an early concrete bug in the Super Console/source-mount mission context.

## 9. Phase 7: Source-Mounted Computer Model

### 9.1 Problem

Real Zot was running and gateway-backed, but it still needed a stable source and
build workspace inside each computer. Without this, Zot could inspect logs and
run tools, but it could not reliably patch/rebuild/restart the affected computer
without ad hoc clone commands.

The required product object is a source-mounted computer:

```text
/Source/platform
/Source/user
/Source/candidate
/Build
/.choir/source-lineage.json
```

### 9.2 Mission Scope

Primary doc:

```text
ecd19fd docs: scope source-mounted super console repair
docs/archive/mission-super-console-source-mount-promotion-v0.md
```

The mission distinguished:

- **personal fix:** stays in one user's computer;
- **reusable Change/AppChangePackage:** packaged for optional adoption;
- **universal platform fix:** lifted into platform repo, CI, staging deploy,
  deployed acceptance;
- **diagnosis only:** no safe patch yet.

Critical invariant: local Zot evidence may justify a platform fix, but does not
automatically merge or deploy one.

### 9.3 Bootstrap Implementation

Primary commits:

```text
262efdf docs: record source workspace bootstrap gap
9a725bc sandbox: bootstrap source workspace for zot
```

Important changes:

- Sandbox startup creates source roots and build root.
- Source-lineage projection is written under `.choir/source-lineage.json`.
- Super Console/Zot receives environment pointing to those roots.
- Tests cover source workspace bootstrap and terminal integration.

Representative files:

```text
cmd/sandbox/main.go
internal/sandbox/source_workspace.go
internal/sandbox/source_workspace_test.go
internal/sandbox/terminal.go
internal/sandbox/terminal_test.go
```

### 9.4 Worker/Candidate Identity Gap

After source roots existed, the mission found that worker/candidate VMs did not
receive enough identity at boot. Firecracker boot args had `vm_id`, gateway
settings, and network details, but not product computer kind, owner, desktop,
worker, or candidate identity. That meant guests could not reliably know whether
they were active, candidate, or worker computers.

Primary commits:

```text
23df128 docs: record worker source workspace gap
59bfd26 vmctl: project source workspace identity into guests
```

Important changes:

- Projected computer kind, owner, desktop, worker/candidate identity into guest
  boot/config paths.
- Updated worker repo bootstrap instructions toward `Source/candidate`.
- Added tests around vmctl ownership and source workspace identity.

Representative files:

```text
cmd/vmctl/main.go
internal/runtime/tools_vmctl.go
internal/sandbox/source_workspace.go
internal/vmctl/ownership.go
internal/vmmanager/manager.go
nix/sandbox-vm.nix
```

## 10. Phase 8: VM Refresh And Boot-Contract Deployment

### 10.1 Problem

The identity projection code landed, but live Node B VM configs still lacked
the new boot args. The deploy had restarted services and hot-refreshed the
sandbox, but it did not full-refresh active Firecracker VMs. Kernel args and
Firecracker config are written at VM boot, so a hot refresh cannot prove or
apply boot-contract changes.

### 10.2 Fix Sequence

Primary commits:

```text
f1abd7c docs: record deploy refresh identity gap
2ef575b vmctl: rehydrate guest identity during VM refresh
6f944c3 docs: record boot contract refresh gap
cb5f29b ci: full refresh VMs for boot contract changes
7127cc3 docs: record VM boot identity promotion
```

Important changes:

- Rehydrated ownership identity into refreshed/recovered VM manager configs.
- Added a deploy impact class for active VM full refresh.
- Made vmctl/vmmanager/boot-contract changes trigger a real active VM refresh,
  not just sandbox hot refresh.

### 10.3 Proof

Promotion evidence for active VM boot identity:

```text
Code commit: cb5f29bc5db52986d30c6f616762836b3a29c25a
CI run: 26711779635, success
Forced staging deploy run: 26711815828, success
Deploy job: 78723354454, success
Staging commit: cb5f29bc5db52986d30c6f616762836b3a29c25a
Node B fc-config files containing choir.computer_kind: 31
```

Sample refreshed boot args:

```text
vm_id=vm-0595550332680fdf7d4cf1a59810c4c5
epoch=5
choir.computer_kind=active
choir.owner_id=89f8895f-d429-4965-a502-78bce2fc6ae6
choir.desktop_id=primary
```

Interpretation: boot-contract changes now have an explicit deploy path that can
actually rewrite Firecracker boot args.

## 11. Phase 9: Materializing Source Checkouts

### 11.1 Problem

After source roots and identity existed, live Node B source directories were
still empty. A path convention is not enough. Zot needs actual source checkouts.

Primary checkpoint:

```text
8e6a06d docs: record empty source checkout gap
```

### 11.2 Implementation

Primary commit:

```text
8a1e36c sandbox: materialize source checkouts for super console
```

Important behavior:

- Bootstrap now materializes platform baseline checkout.
- Bootstrap materializes writable candidate checkout.
- It preserves dirty candidate state instead of deleting local edits.
- It records checkout status in source-lineage state.
- It continues with precise status if Git/network/checkouts fail.

Representative files:

```text
internal/sandbox/source_workspace.go
internal/sandbox/source_workspace_test.go
internal/sandbox/terminal.go
```

### 11.3 Zot Candidate Repair Evidence

Primary checkpoint:

```text
471b6aa docs: record zot source repair evidence
```

Evidence:

- Real Zot started from active computer path on Node B.
- It inspected source lineage and the source workspace.
- It created a candidate source change to summarize checkout state.
- It wrote repair session evidence under:

  ```text
  /var/lib/go-choir/files/.choir/zot/sessions/zot-source-checkout-20260531
  ```

The candidate-local patch was not automatically accepted as a platform fix.
Codex then reviewed/lifted it into platform code.

### 11.4 Platform Lift

Primary commits:

```text
f41ee18 sandbox: summarize source checkout state
4425b34 docs: record zot source fix promotion
```

Behavior added:

- Source lineage now reports checkout status such as:

  ```text
  platform_checkout_status
  candidate_checkout_status
  dirty_state_summary
  ```

Promotion evidence:

```text
CI run: 26712362968, success
Deploy job: 78724820417, success
FlakeHub run: 26712362976, success
Staging commit: f41ee189f810ecc7fb3efbedbfeb83229e742f23
```

Product-path Super Console proof after deploy:

```json
{
  "platform_base_commit": "f41ee189f810ecc7fb3efbedbfeb83229e742f23",
  "current_runtime_build_ref": "f41ee189f810ecc7fb3efbedbfeb83229e742f23",
  "current_frontend_build_ref": "f41ee189f810ecc7fb3efbedbfeb83229e742f23",
  "dirty_state_summary": "dirty_preserved",
  "platform_checkout_status": "ok_platform_at_base",
  "candidate_checkout_status": "dirty_preserved"
}
```

The `dirty_preserved` status was expected because the candidate checkout still
contained Zot-authored repair evidence.

## 12. Phase 10: Node B Dev-Shell Runtime-Link Fix

### 12.1 Problem

Zot could inspect and patch source, but when it tried to run focused Go tests
inside the Node B user-computer source checkout, the transient Go test binary
failed:

```text
error while loading shared libraries: libicui18n.so.76: cannot open shared object file
FAIL github.com/yusefmosiah/go-choir/internal/sandbox 0.000s
```

Dev-shell inspection showed:

```text
LD_LIBRARY_PATH=
CGO_ENABLED=1
CGO_CFLAGS=-I/nix/store/...-icu4c-76.1-dev/include
CGO_LDFLAGS=-L/nix/store/...-icu4c-76.1/lib -licui18n -licuuc
```

Root cause: the Nix dev shell exposed ICU headers/linker flags but not the ICU
runtime library path needed by transient Go test binaries.

### 12.2 Documentation Before Fix

Problem checkpoint:

```text
a972e75 docs: record node b dev shell link gap
```

This followed the repo invariant: document the new platform behavior problem
before fixing it.

### 12.3 Implementation

Primary fix:

```text
8e5d9aa nix: expose icu runtime libs in dev shell
```

Change:

```nix
export LD_LIBRARY_PATH="${devPkgs.icu}/lib''${LD_LIBRARY_PATH:+:''${LD_LIBRARY_PATH}}"
```

This was added to the repo dev shell, rather than normalizing ad hoc
`LD_LIBRARY_PATH` commands in Zot prompts.

### 12.4 Local Verification

Before push:

```text
nix develop -c sh -lc 'test -n "$LD_LIBRARY_PATH" && printf "LD_LIBRARY_PATH=%s\n" "$LD_LIBRARY_PATH" && go env CGO_ENABLED CGO_CFLAGS CGO_LDFLAGS'
LD_LIBRARY_PATH=/nix/store/...-icu4c-76.1/lib
CGO_ENABLED=1
CGO_CFLAGS=-I/nix/store/...-icu4c-76.1-dev/include
CGO_LDFLAGS=-L/nix/store/...-icu4c-76.1/lib -licui18n -licuuc

nix develop -c go test ./internal/sandbox -run 'TestBootstrapSourceWorkspace(MaterializesPlatformAndCandidateCheckouts|PreservesDirtyCandidateCheckout)|TestSourceCheckoutDirtyStateSummary' -count=1
ok github.com/yusefmosiah/go-choir/internal/sandbox
```

### 12.5 Platform Promotion

Promotion evidence:

```text
Commit: 8e5d9aaa629537cd39680c53d2fcdba8caaa93c5
CI run: 26712476770, success
FlakeHub run: 26712476768, success
Staging health: ok
Staging deployed_at: 2026-05-31T12:23:22Z
```

Staging health:

```json
{
  "status": "ok",
  "commit": "8e5d9aaa629537cd39680c53d2fcdba8caaa93c5",
  "upstream": "8e5d9aaa629537cd39680c53d2fcdba8caaa93c5",
  "deployed_at": "2026-05-31T12:23:22Z"
}
```

Node B verification after deploy:

```text
cd /var/lib/go-choir/files/Source/platform
nix develop -c sh -lc 'printf "LD_LIBRARY_PATH=%s\n" "$LD_LIBRARY_PATH"; go env CGO_ENABLED CGO_CFLAGS CGO_LDFLAGS; go test ./internal/sandbox -run "TestBootstrapSourceWorkspace(MaterializesPlatformAndCandidateCheckouts|PreservesDirtyCandidateCheckout)|TestSourceCheckoutDirtyStateSummary" -count=1'
LD_LIBRARY_PATH=/nix/store/...-icu4c-76.1/lib
ok github.com/yusefmosiah/go-choir/internal/sandbox 0.105s
```

## 13. Final Zot Repair-Loop Proof

### 13.1 Evidence Directory

Final Zot repair-loop evidence:

```text
/var/lib/go-choir/files/.choir/zot/sessions/zot-devshell-rebuild-restart-20260531
```

Files:

```text
commands.jsonl
nix-env.txt
test-output.txt
build-output.txt
restart-output.txt
source-lineage.pretty.json
health.json
diagnosis.md
rollback.md
classification.md
```

### 13.2 What Zot Proved

Zot proved the Node B dev-shell runtime-link fix from inside the
source-mounted user computer:

```text
nix-env.txt:
LD_LIBRARY_PATH=/nix/store/...-icu4c-76.1/lib

test-output.txt:
ok github.com/yusefmosiah/go-choir/internal/sandbox 0.004s

build-output.txt:
empty successful output for go build ./cmd/sandbox

restart-output.txt:
go-choir-sandbox.service active after restart

health.json:
platform/source-lineage/runtime/frontend refs at 8e5d9aaa...
```

The final `health.json` recorded:

```json
{
  "platform_head": "8e5d9aaa629537cd39680c53d2fcdba8caaa93c5",
  "expected_fix_commit": "8e5d9aaa629537cd39680c53d2fcdba8caaa93c5",
  "head_matches_expected_fix_commit": true,
  "service": {
    "name": "go-choir-sandbox.service",
    "active_state": "active",
    "enabled_state": "enabled"
  },
  "secrets_included": false
}
```

`classification.md` classified the fix as:

```text
universal platform fix, not personal-only
```

`rollback.md` identified:

```text
Revert 8e5d9aaa629537cd39680c53d2fcdba8caaa93c5
or restore previous runtime build f41ee189f810ecc7fb3efbedbfeb83229e742f23
```

### 13.3 Important Caveat

The first Zot process lost gateway auth after it restarted
`go-choir-sandbox.service`. A second Zot invocation with a fresh token completed
the evidence export.

Interpretation:

- The dev-shell/source-mounted repair objective was achieved.
- Restart-continuation is real debt.
- Future Super Console/Zot work should support clean resume/survival across a
  runtime restart that Zot itself initiates.

## 14. Current Worktree State

At the end of the mission, unrelated staged work remained staged and untouched:

```text
A  cmd/sourcecycled/main.go
A  configs/sources.json
A  internal/cycle/cycle.go
A  internal/cycle/storage.go
A  internal/cycle/synthesize.go
A  internal/sources/gdelt.go
A  internal/sources/rss.go
A  internal/sources/telegram.go
A  internal/sources/types.go
```

Those files were not part of the VText/Super Console/Zot mission and were
preserved.

This report file is a new documentation artifact and should be reviewed
separately from that staged work.

## 15. What Is Actually Done

### Done: VText Simplification

- Conductor no longer owns first appagent document drafts.
- VText writes canonical appagent versions.
- Durable co-agent evidence drives later versions.
- Staging proof showed VText version advancement on a product path.

### Done: Trace Product Cutover

- Visual Trace app is removed from product desktop/app registry.
- No emergency visual Trace route is preserved.
- Machine-readable trace/log/evidence substrate remains.

### Done: Terminal Replacement

- Raw Terminal is no longer the user-facing desktop app.
- Super Console is the replacement repair surface.
- `/api/terminal/ws` returns a replacement response.

### Done: Real Zot

- Upstream Zot is packaged and deployed.
- Super Console launches real Zot in the user computer path.
- Zot session artifacts persist under `.choir/zot/sessions`.

### Done: Gateway-Backed Zot

- Zot uses Choir gateway via OpenAI-compatible route.
- Provider secrets stay host/platform-owned.
- Gateway supplies safe default instructions for ChatGPT backend.
- Gateway buffers tool calls so Zot receives complete tool arguments.

### Done: Source-Mounted Repair Substrate

- Source roots exist.
- Source lineage is projected locally.
- Platform and candidate checkouts are materialized.
- Dirty candidate state is preserved and reported.
- Active/candidate/worker identity is projected into guest boot config.

### Done: Platform Promotion Path

- Zot-authored candidate evidence was not auto-promoted.
- Universal fixes were lifted, tested, pushed, deployed, and verified.
- CI/FlakeHub/deploy/staging evidence is recorded.
- Rollback refs are recorded.

## 16. What Is Not Done Yet

### P0 Residual: Restart-Continuation

Zot should be able to initiate a runtime restart and then resume or reconnect
without losing the ability to finish its evidence bundle.

Current workaround:

```text
first Zot invocation performs test/build/restart
second Zot invocation uses fresh token and writes final evidence
```

Desired future behavior:

```text
single Super Console/Zot repair session survives or cleanly resumes after restart
```

### P1 Residual: Personal Promotion Product Path

The universal platform path was exercised. The personal and reusable paths are
still mostly design/invariant work:

- personal candidate -> active computer promotion;
- rollback TTL for local route switch;
- reusable Change/AppChangePackage packaging;
- adoption/rollback verifier evidence.

### P1 Residual: Unified Logs v0

The direction is clear:

```text
logs/runtime.jsonl
logs/frontend.jsonl
logs/agents.jsonl
logs/tools.jsonl
logs/builds.jsonl
logs/super-console/<session-id>.jsonl
```

But the full clean directory/indexing implementation is not finished. Current
evidence is spread across existing trace APIs, source-lineage files, Zot session
files, command logs, and mission docs.

### P2 Residual: Super Console UX

Super Console works as a repair path but is not polished:

- session status/history needs product treatment;
- failure messages need clearer explanation;
- theme/contrast needs continuing attention;
- command/tool streaming needs robust UI treatment;
- restart-resume needs first-class UX.

## 17. Review Questions For Owner

### Architecture

1. Does the VText simplification match the intended product ontology:
   conductor routes, VText writes, co-agents provide evidence?
2. Is the hard Trace cutover correct, including no emergency visual Trace route?
3. Is Super Console correctly bounded as singleton repair mode, not a new main
   scripting surface?
4. Is the local Zot authority boundary right: anything inside that computer, but
   no MAS peer/appagent/VText-writer status?

### Promotion

1. Is the distinction between personal, reusable, and universal fixes strong
   enough?
2. Should universal fixes always require Codex/platform-authorized lift, or can
   Zot eventually create a typed platform PR candidate directly?
3. What is the minimum acceptable rollback certificate for a personal computer
   repair?

### Debugging Workflow

1. Is the restart-continuation gap the right next P0?
2. Should Super Console keep a visible session history, or should VText be the
   primary readable archive of repair sessions?
3. What should be the first non-platform personal repair proof after this?

## 18. Bottom Line

The overnight work moved Choir from:

```text
VText regression + visual Trace spelunking + raw Terminal + placeholder Zot
```

to:

```text
simpler VText contract
+ no visual Trace app
+ singleton Super Console
+ real gateway-backed Zot
+ source-mounted computer
+ local test/build/restart/verify evidence
+ explicit platform promotion path
```

The most important strategic outcome is not any single line of code. It is that
debugging is now aimed at a faster user-computer inner loop without turning Zot
into the MAS or letting local repairs silently become platform changes.

The next high-leverage work is to make Zot/Super Console survive or resume
cleanly across the runtime restart it initiates.
