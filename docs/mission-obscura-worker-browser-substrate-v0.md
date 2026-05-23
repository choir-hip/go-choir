# MissionGradient: Obscura Worker Browser Substrate v0

**Status:** checkpoint_incomplete
**Date:** 2026-05-22
**Related state:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md), [obscura-browser-in-vm-frontier-2026-05-13.md](obscura-browser-in-vm-frontier-2026-05-13.md), [backend-browser-vm-local-execution-blocker-2026-05-13.md](backend-browser-vm-local-execution-blocker-2026-05-13.md)

## One-Line Goal String

```text
/goal Run docs/mission-obscura-worker-browser-substrate-v0.md as a Codex-operated MissionGradient tangent mission: replace Chrome/Playwright-in-worker-VM evidence capture with a lighter Obscura browser substrate. Update and reconcile the Choir Obscura fork with upstream, preserving only the useful Choir parity patches; package and wire Obscura into the NixOS worker/candidate VM as the VM-local browser/extraction/control/evidence path through `CHOIR_OBSCURA_BIN`; remove Playwright browser bundles from worker VMs once Obscura proof is real; keep any remaining Playwright/Chrome use outside user/candidate VMs as a bounded verifier only. Prove the deployed path with staging identity, VM-local Obscura capabilities, browser/extraction smoke, Chiron-style human evidence capture, resource/closure comparison, rollback refs, residual risks, and a clear return point to docs/mission-human-proof-experiment-rerun-v0.md.
```

## Mission Frame

The human-proof experiment rerun exposed a browser-substrate mistake. Adding
Playwright browser binaries to every worker VM may unblock screenshots, but it
also turns each user/candidate VM into a much heavier computer. That is the
wrong default if Choir needs many persistent user computers, background
candidates, and worker agents.

The target direction is:

```text
VM-local browser/extraction/control substrate: Obscura
Heavy browser fidelity verifier: outside the user/candidate VM, bounded and explicit
Product truth: Dolt/product APIs plus Trace/VText/media evidence
```

Obscura should serve double duty:

- lightweight browser/extraction substrate for web acquisition;
- candidate/worker-local browser control and evidence capture where its
  fidelity is sufficient.

It must not be sold as Chrome until it actually proves Chrome-equivalent
behavior for the specific contract being claimed.

## Current Facts

- The deployed worker VM currently includes `pkgs.playwright-driver` and
  `PLAYWRIGHT_BROWSERS_PATH` through commit
  `6135a245cb4b1a15dd1a3d5b362320025ca0d321`.
- Choir runtime already has Obscura hooks:
  `CHOIR_OBSCURA_BIN` / `OBSCURA_BIN`, optional
  `CHOIR_OBSCURA_CDP_SCREENSHOTS`, and `/api/browser/*` capability reporting.
- Those hooks are not currently packaged/configured inside the worker VM.
- Previous Browser docs prove host-process Obscura text/html/link snapshots,
  opt-in CDP screenshots, persistent sessions, and bounded fill/click control.
- The known unresolved boundary is VM-local execution: do not expose `vm_id`,
  worker sandbox URLs, or vmctl handles through browser-public APIs.
- A local go-choir patch now packages a pinned Obscura fork revision and wires
  it into the sandbox VM while removing Playwright browser environment from the
  guest. This patch has Linux/Nix build proof on Node A, but it has not yet
  been pushed, deployed, or verified on staging.

## What Is Special In Our Obscura Fork

Local fork state:

```text
repo: /Users/wiz/obscura
branch: choir/playwright-parity-audit-2026-05-10
origin: https://github.com/yusefmosiah/obscura.git
upstream: https://github.com/h4ckf0r0day/obscura.git
fork main base: 85739d3
upstream main: 1950048
local branch delta: 2 commits ahead of old base, upstream is 36 commits ahead
```

The two Choir commits are:

- `df6eb63 Preserve Choir Playwright parity patch stack`
- `a91f43d Preserve Choir Obscura audit artifacts`

`a91f43d` is evidence only: docs, audit scripts, sanitized summaries, and patch
fragments.

`df6eb63` is the real special sauce. It touches 38 source files across
`obscura-browser`, `obscura-cdp`, `obscura-cli`, `obscura-dom`, `obscura-js`,
and `obscura-net`. The important capabilities are:

- external ES module execution for deployed Vite/Svelte apps;
- repeated module evaluation safety;
- runtime console events through `Runtime.consoleAPICalled`;
- CLI parity for global user-agent and `serve --obey-robots`;
- main-document Fetch interception and `Fetch.fulfillRequest`;
- `Schema.getDomains`, initialization compatibility, and reliable
  `Browser.close`;
- early preload execution and `Fetch.disable` cleanup;
- DOM insertion/replacement fixes and deterministic coordinate mouse input;
- private-network opt-in through `OBSCURA_ALLOW_PRIVATE_NETWORK=1`;
- WebAuthn initialization acknowledgement, not real passkey support;
- `Page.printToPDF`, `Page.captureScreenshot`, and screencast bridge through
  DOM serialization plus WeasyPrint/ImageMagick.

The last item is useful but must remain honestly labeled. It can produce
nonblank screenshots/video for smoke evidence. It is not native browser layout
fidelity.

## Upstream State

The fork's `main` is stale. Upstream has 36 newer commits, including important
CDP and security work:

- click-submit navigation parity;
- `Page.reload`;
- `data:` URL navigation;
- real `DOM.getBoxModel` and `DOM.getContentQuads`;
- `about:blank` navigation;
- `globalThis.self` and `_wrap` exposure;
- `file://` navigation gates;
- redirect target revalidation;
- runtime context validation;
- aarch64 Linux release artifact work;
- MCP and CLI improvements.

The latest public release is `v0.1.5`, but the source branch has newer relevant
fixes. Do not assume release binaries contain every needed fix.

## Merge Probe

A safe scratch worktree was created at:

```text
/Users/wiz/obscura-upstream-merge
branch: codex/choir-obscura-upstream-merge-2026-05-22
base: upstream/main @ 1950048
operation: cherry-pick df6eb63
```

The cherry-pick does not apply cleanly. It conflicts in 13 files:

```text
crates/obscura-browser/src/lib.rs
crates/obscura-browser/src/page.rs
crates/obscura-cdp/src/dispatch.rs
crates/obscura-cdp/src/domains/dom.rs
crates/obscura-cdp/src/domains/page.rs
crates/obscura-cdp/src/domains/runtime.rs
crates/obscura-cdp/src/domains/target.rs
crates/obscura-cdp/src/lib.rs
crates/obscura-cdp/src/server.rs
crates/obscura-cli/src/main.rs
crates/obscura-js/src/lib.rs
crates/obscura-js/src/ops.rs
crates/obscura-js/src/runtime.rs
```

This means the right move is not a blind merge. The patch stack should be
ported as slices, preserving upstream security/CDP improvements and only
reintroducing Choir-specific parity where still missing.

The scratch cherry-pick was aborted after recording the conflict list, so the
worktree is clean and can be reused for a deliberate slice-by-slice port.

### Current Reconciled Slice

The first reconciled slice is intentionally narrow:

```text
repo: /Users/wiz/obscura-upstream-merge
branch: codex/choir-obscura-upstream-merge-2026-05-22
base: upstream/main @ 1950048
commit: 348a651e287ad370546762e78fc2095a7d33dc93
change: external ES module entrypoints execute instead of only being resolved
```

This preserves the most immediately relevant Choir patch for deployed
Vite/Svelte surfaces while avoiding a blind merge of the stale 38-file patch
stack. Local Obscura tests passed for this slice:

```text
cargo test -p obscura-js load_module_fetches_and_evaluates_entry_source
cargo test -p obscura-js --lib
```

The remaining fork-special capabilities are not yet ported. They should be
reintroduced only when the worker-VM evidence path needs them and upstream does
not already provide the behavior.

## Local Nix Proof

Node A was used only as an x86_64 build lab. It was not made a production
target and no tracked files were edited there as source of truth.

Current local go-choir patch:

```text
obscura package revision: 348a651e287ad370546762e78fc2095a7d33dc93
obscura package output: /nix/store/gc6qvq656dm1wvxid97cpcxk69pfs9g6-obscura-0.1.0-choir-348a651
obscura closure: 507.9 MiB
guest image output: /nix/store/8fimb6vls8xj6ld9y1nyj0lyn5620wam-go-choir-guest-image
guest image closure: 3.4 GiB
guest image build time on warm Node A cache: 4m19.633s
```

Smoke command:

```text
obscura fetch https://example.com --dump text --timeout 10 --quiet
```

Result: returned the expected `Example Domain` text.

Closure comparison from Node A:

```text
playwright-core closure: 244.8 MiB
playwright-driver.browsers closure: 2.0 GiB
obscura closure: 507.9 MiB
```

Conclusion: Obscura is not tiny, but it avoids embedding the multi-browser
Chromium/Firefox/WebKit Playwright browser bundle into every user/candidate VM.
Heavy Chrome/Playwright should remain an external verifier unless a specific
contract proves it must be VM-local.

## Invariants

- No Chrome/Playwright browser bundle in user/candidate worker VMs in the
  target state.
- Obscura evidence must be labeled by fidelity level: extraction, DOM/CDP
  smoke, screenshot bridge, or true external verifier.
- Do not claim Obscura screenshots are pixel-equivalent to Chrome/WebKit unless
  a verifier proves that for the specific target.
- Browser-public APIs must not accept raw `vm_id`, `worker_id`,
  `worker_sandbox_url`, or vmctl handles.
- Candidate browser sessions must resolve through server-side lease authority.
- Private prompts, uploads, cookies, provider credentials, and DOM captures must
  not leak through browser evidence.
- Any remaining Playwright/Chrome use must be outside user/candidate VMs,
  bounded, and named as verifier-only.

## Homotopy

1. **Fork reconciliation**
   - Keep `/Users/wiz/obscura` preservation branch intact.
   - Port the Choir patch stack onto upstream main by slices, not as one
     monolith.
   - Prefer upstream implementations where they now exist.
   - Preserve audit scripts and update them to test the reconciled branch.

2. **Nix packaging**
   - Package Obscura from source or a pinned fork revision for x86_64 Linux.
   - Avoid depending on a local arm64 `/Users/wiz/obscura/target/release`
     binary.
   - Measure closure impact against `playwright-driver.browsers`.

3. **Worker VM wiring**
   - Add Obscura to the sandbox VM PATH.
   - Set `CHOIR_OBSCURA_BIN` explicitly.
   - Enable `CHOIR_OBSCURA_CDP_SCREENSHOTS` only after screenshot mode is
     verified.
   - Remove `playwright-driver`, `PLAYWRIGHT_BROWSERS_PATH`, and related
     Playwright VM environment once Obscura proof passes.

4. **Internal VM browser lease**
   - Bind browser sessions to candidate/worker lease identity server-side.
   - Keep worker sandbox URLs internal.
   - Add tests proving another owner cannot resolve or control a candidate
     browser.

5. **Staging proof**
   - Verify staging identity.
   - Launch a fresh worker/candidate VM.
   - Prove `obscura fetch`, `obscura serve`, `/api/browser/capabilities`, and
     a bounded screenshot/control path inside the VM.
   - Rerun a Chiron-style human-evidence prompt and require VText plus real
     screenshot/video or a precise Obscura fidelity blocker.

6. **Return to the paused experiment mission**
   - Resume `docs/mission-human-proof-experiment-rerun-v0.md`.
   - Rerun Chiron sequentially through Choir-in-Choir using the new browser
     substrate.

## Dense Feedback

Required evidence:

- fork reconciliation branch and conflict notes;
- Obscura audit result after upstream reconciliation;
- Nix build/closure comparison: Obscura vs Playwright browsers;
- VM-local `obscura fetch` and `obscura serve` smoke;
- `/api/browser/capabilities` from staging showing configured Obscura;
- screenshot/control proof, with fidelity label;
- Chiron rerun evidence showing VText narrative plus media;
- rollback refs for both go-choir and Obscura fork changes.

## Forbidden Shortcuts

- Do not keep Playwright in the VM and call the mission solved.
- Do not download arbitrary Obscura binaries into production VMs without a
  pinned, reviewable, reproducible path.
- Do not blindly cherry-pick the old Choir patch stack over upstream conflicts.
- Do not expose internal worker URLs or VM ids to product/browser APIs.
- Do not use fake screenshots, blank videos, DOM posters, or package receipts
  as human proof.
- Do not use Obscura's screenshot bridge as proof of pixel-accurate mobile UI.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: ported the narrow external-module Obscura slice onto upstream main, pushed Obscura branch `codex/choir-obscura-upstream-merge-2026-05-22`, packaged that commit in go-choir Nix, removed Playwright browser wiring from the sandbox VM, and proved the Obscura package plus full guest image build on Node A.
current artifact state: go-choir staging still serves `6135a` with Playwright browser wiring in worker VMs. The local worktree replaces that VM dependency with pinned Obscura, sets `CHOIR_OBSCURA_BIN` / `OBSCURA_BIN`, and updates worker/super prompt contracts to treat Chrome/Playwright as external verifier tooling.
what shipped: Obscura fork branch `348a651e287ad370546762e78fc2095a7d33dc93` was committed and pushed. No go-choir platform behavior for this tangent has shipped yet.
what was proven: blind full cherry-pick conflicts in 13 files; narrow Obscura runtime slice passes `cargo test -p obscura-js --lib`; Node A built `.#packages.x86_64-linux.obscura`; `obscura fetch https://example.com --dump text --timeout 10 --quiet` returned expected text; Node A built `.#guest-image --no-link` with Obscura in the sandbox VM graph; Obscura closure is 507.9 MiB versus `playwright-driver.browsers` at 2.0 GiB.
unproven or partial claims: the go-choir Obscura VM patch has not been committed, pushed, built by CI, deployed, or verified on staging. VM-local `/api/browser/capabilities`, CDP screenshot/control, candidate preview, and Chiron rerun proof are still missing. Most old Choir Obscura patch-stack features remain intentionally unported until needed.
belief-state changes: the heavy Playwright browser bundle is not necessary to keep in every worker VM merely to provide a VM-local browser/extraction primitive; Obscura can now be built reproducibly enough for the next platform proof, but its screenshot/control fidelity still needs product-path verification.
remaining error field: land the go-choir Obscura VM patch through CI/deploy, verify staging identity, launch a fresh worker/candidate VM, prove `obscura` and `CHOIR_OBSCURA_BIN` inside the guest, verify `/api/browser/capabilities` and candidate preview evidence, then rerun Chiron through Choir-in-Choir.
highest-impact remaining uncertainty: whether reconciled Obscura can produce enough owner-readable evidence for Chiron/Motion/Liquid without Chrome-level rendering.
next executable probe: run local focused tests/diff checks, commit and push the go-choir Obscura VM patch, monitor CI/deploy, then run staging VM-local Obscura capability and Chiron preview proof.
suggested resume goal string: use the one-line goal string in this file.
evidence artifact refs: `/Users/wiz/obscura`; `/Users/wiz/obscura-upstream-merge`; Obscura commit `348a651e287ad370546762e78fc2095a7d33dc93`; Node A `/tmp/go-choir-obscura-probe`; Obscura output `/nix/store/gc6qvq656dm1wvxid97cpcxk69pfs9g6-obscura-0.1.0-choir-348a651`; guest image output `/nix/store/8fimb6vls8xj6ld9y1nyj0lyn5620wam-go-choir-guest-image`; `docs/mission-human-proof-experiment-rerun-v0.md`.
rollback refs: revert the eventual go-choir Obscura VM patch if staging guest image or worker browser capability fails; revert `2995853a9ea80c23a6268035b5a87737e894d9eb` and `6135a245cb4b1a15dd1a3d5b362320025ca0d321` after Obscura replacement lands if Playwright-in-VM remains undesirable.
```

## Stopping Condition

Complete only when:

- Obscura fork is reconciled with upstream or a deliberate upstream/fork
  boundary is chosen;
- Obscura is packaged and configured in worker/candidate VMs;
- Playwright browser bundles are removed from those VMs;
- VM-local browser/extraction/screenshot/control contracts are verified on
  staging;
- human-proof experiment rerun can resume with Obscura as the browser substrate
  or with a precise fidelity blocker.

If incomplete, update this checkpoint and return to the paused human-proof
mission only with the remaining browser-substrate risk explicitly named.
