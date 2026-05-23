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

- Staging now serves commit `f452977af7532e99daf7850fd0b635915c3082e6`.
- Worker/candidate VM images now include the pinned Obscura package and no
  longer carry the Playwright browser bundle/environment as the VM-local
  browser substrate.
- Fresh deployed worker proof shows `/api/browser/capabilities` returning
  provider `obscura`, mode `backend`, substrate `obscura_cli_fetch`,
  configured/available `true`, status `ready`, and binary `obscura`.
- A fresh deployed worker browser session successfully navigated to
  `https://example.com`, returned text and link snapshots, and was then closed.
- The deployed `hibernate-worker` vmctl endpoint can now hibernate typed worker
  VMs without touching the parent desktop; the proof worker and parent were both
  left hibernated.
- Choir runtime has Obscura hooks:
  `CHOIR_OBSCURA_BIN` / `OBSCURA_BIN`, optional
  `CHOIR_OBSCURA_CDP_SCREENSHOTS`, and `/api/browser/*` capability reporting.
- Previous Browser docs prove host-process Obscura text/html/link snapshots,
  opt-in CDP screenshots, persistent sessions, and bounded fill/click control.
- The remaining Obscura-specific fidelity boundary is screenshot/control:
  VM-local text/html/link extraction is proven, while screenshots/video and
  behavior proof still require external Playwright/Chrome verifier evidence or
  a separately proven Obscura CDP screenshot/control contract.

## Capability Matrix

This is the current truth as of the staging proof on `f452977`.

| Capability | Current status | Evidence | Mission consequence |
| --- | --- | --- | --- |
| Public web text/html/link extraction | **Proven on staging worker VM** | `/api/browser/capabilities` reported provider `obscura`, substrate `obscura_cli_fetch`; browser session `b134f3a2-70e6-4199-af4d-fc70c77eeb8c` navigated to `https://example.com` and returned expected text plus one link. | Worker/candidate agents can use Obscura for lightweight acquisition and extraction without carrying Playwright browser bundles. |
| Arbitrary web scraping/extraction | **Partially proven** | The API supports URL-normalized backend snapshots: text, HTML, and up to 100 links. Only a simple public page has been exercised in deployed proof. | Safe to use for simple public extraction; not yet proof for adversarial sites, complex JS apps, paywalls, authenticated pages, or pages needing interaction. |
| Choir authenticated product UI | **Not proven** | `/api/browser/*` itself is authenticated, but `obscura fetch` does not inherit the user's browser cookies/session when it fetches a target URL. No deployed proof has shown Obscura logging into or controlling `draft.choir-ip.com` as a user. | Do not use Obscura as the acceptance path for authenticated Choir UI behavior yet. Use product APIs plus external authenticated Playwright/Chrome for owner-visible UI proof until a secure session-injection or service-token design exists. |
| Screenshots | **Code path exists, not deployed-proven** | Runtime has optional `CHOIR_OBSCURA_CDP_SCREENSHOTS` support and opt-in live tests for `Page.captureScreenshot`, but the fresh staging worker capability report did not advertise screenshot/CDP support. | Screenshots for human proof must still come from bounded external verifier tooling unless/until a deployed worker capability report shows `screenshot=true` and a staging screenshot proof passes. |
| Video | **Not implemented/proven** | No product API or Obscura runtime path records video. At most, future work could assemble frames from screenshots after the screenshot path is proven. | Playwright/Chrome video remains external verifier evidence. Do not claim Obscura video evidence. |
| Bounded actions (`fill`, `click`) | **Code path exists, not deployed-proven** | Runtime exposes `/api/browser/sessions/{id}/control` only when CDP screenshots are enabled; live tests cover fill/click behind `GO_CHOIR_RUN_OBSCURA_CDP=1`. Staging proof did not enable or verify this. | Not enough for “arbitrary actions.” Treat it as a future bounded-control slice with explicit selector/action limits and separate auth/security review. |
| General arbitrary browser automation | **Not supported** | The current API only models navigate, snapshot, optional screenshot, optional `fill`, and optional `click`. | Keep experiment acceptance on product APIs and external UI verifier. Do not ask Obscura to replace Playwright for full UI automation yet. |
| Web Lens iframe fallback | **Good next substrate candidate, not built** | Obscura can provide server-side text/html/link snapshots for pages that block iframes. It cannot yet provide an interactive, authenticated embedded browser replacement. | After the experiment mission, a good mission is “Web Lens Obscura snapshot fallback”: when iframe embedding fails, show an Obscura-backed readable snapshot/extracted links with clear fidelity label, not a fake iframe. |

The key design boundary is now clearer:

```text
Obscura in user/candidate VMs = lightweight acquisition and extraction.
External Playwright/Chrome = bounded human-media verifier for screenshots/video.
Future Obscura CDP slice = optional screenshot/fill/click substrate only after deployed proof.
```

This means the human-proof experiment rerun should use Obscura to reduce VM
weight and to let Choir agents inspect public web pages, but it should not wait
for Obscura to become full Playwright before rerunning Chiron. The Chiron
acceptance path should still require screenshots/video from external verifier
tooling until Obscura CDP screenshot/control is separately proven on staging.

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
last checkpoint: shipped the Obscura VM substrate at `ddc5069dd69e020fb0801941b4f80764559c5ec7`, then shipped `f452977af7532e99daf7850fd0b635915c3082e6` to add explicit internal typed-worker hibernation for proof cleanup. Fresh staging worker proof passed and both proof worker and parent were hibernated afterward.
current artifact state: staging proxy and upstream sandbox both report `f452977af7532e99daf7850fd0b635915c3082e6`. Worker/candidate VMs use pinned Obscura for backend text/html/link browser extraction through `CHOIR_OBSCURA_BIN` / `OBSCURA_BIN`; Chrome/Playwright remains external verifier tooling, not a VM-local browser dependency.
what shipped: Obscura fork branch `348a651e287ad370546762e78fc2095a7d33dc93`; go-choir commit `ddc5069dd69e020fb0801941b4f80764559c5ec7` (`Use Obscura in worker VMs`); go-choir commit `f452977af7532e99daf7850fd0b635915c3082e6` (`Add vmctl worker hibernate endpoint`). GitHub Actions run `26318375143` passed and deployed `ddc5069`; GitHub Actions run `26318837141` passed and deployed `f452977`.
what was proven: blind full Obscura cherry-pick conflicts in 13 files; narrow Obscura runtime slice passes `cargo test -p obscura-js --lib`; Node A built `.#packages.x86_64-linux.obscura`; `obscura fetch https://example.com --dump text --timeout 10 --quiet` returned expected text; Node A built `.#guest-image --no-link`; Obscura closure is 507.9 MiB versus `playwright-driver.browsers` at 2.0 GiB; staging `/health` reports proxy/upstream at `f452977`; a fresh deployed worker `worker-ed239185b8596177` on VM `vm-840aa642db68196a0e4359085cfb28a0` reported Obscura backend capabilities, created browser session `b134f3a2-70e6-4199-af4d-fc70c77eeb8c`, navigated to `https://example.com`, returned the expected `Example Domain` text snapshot and one link, closed the session, and then hibernated through `/internal/vmctl/hibernate-worker`.
unproven or partial claims: Obscura CDP screenshot/control is not yet proven in deployed worker VMs. Candidate preview, screenshots/video, and Chiron behavior proof still need owner-readable media evidence, likely through external Playwright/Chrome until Obscura CDP screenshot/control is separately proven. Most old Choir Obscura patch-stack features remain intentionally unported until needed.
belief-state changes: the VM-local browser substrate no longer needs the heavy Playwright browser bundle for extraction and simple navigation snapshots. The proof loop also needs explicit typed-worker cleanup; that is now present as internal vmctl hibernation rather than broad process killing.
remaining error field: return to `docs/mission-human-proof-experiment-rerun-v0.md` and rerun Chiron through Choir-in-Choir with Obscura as the VM-local extraction substrate and external Playwright/Chrome as bounded human-media verifier. If screenshot/control must become VM-local, define and prove a separate Obscura CDP screenshot/control slice rather than smuggling Chrome back into every VM.
highest-impact remaining uncertainty: whether the next Chiron rerun can produce an owner-pullable unlisted AppChangePackage with live VText narrative, screenshots/video, recipient build/adoption evidence, and rollback, without Codex hand-coding the feature.
next executable probe: run the Chiron rerun sequentially through the visible product path, requiring an unlisted owner-pullable package, causal VText dashboard updates, media evidence from external verifier tooling, recipient build/adoption proof, and rollback refs.
suggested resume goal string: use the one-line goal string in this file.
evidence artifact refs: `/Users/wiz/obscura`; `/Users/wiz/obscura-upstream-merge`; Obscura commit `348a651e287ad370546762e78fc2095a7d33dc93`; Node A `/tmp/go-choir-obscura-probe`; Obscura output `/nix/store/gc6qvq656dm1wvxid97cpcxk69pfs9g6-obscura-0.1.0-choir-348a651`; guest image output `/nix/store/8fimb6vls8xj6ld9y1nyj0lyn5620wam-go-choir-guest-image`; GitHub Actions runs `26318375143` and `26318837141`; staging `/health`; fresh proof user `obscura-proof-1779497627`; proof worker `worker-ed239185b8596177`; proof VM `vm-840aa642db68196a0e4359085cfb28a0`; proof browser session `b134f3a2-70e6-4199-af4d-fc70c77eeb8c`; `docs/mission-human-proof-experiment-rerun-v0.md`.
rollback refs: revert `ddc5069dd69e020fb0801941b4f80764559c5ec7` if Obscura VM substrate regresses worker browser extraction; revert `f452977af7532e99daf7850fd0b635915c3082e6` if the internal worker hibernation endpoint causes vmctl lifecycle regressions; revert `2995853a9ea80c23a6268035b5a87737e894d9eb` and `6135a245cb4b1a15dd1a3d5b362320025ca0d321` if Playwright-in-VM cleanup needs to be audited separately.
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
