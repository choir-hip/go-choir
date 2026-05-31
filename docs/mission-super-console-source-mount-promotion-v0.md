# MissionGradient: Super Console Source Mount And Promotion v0

**Status:** draft
**Date:** 2026-05-31
**Related docs:** [computer-ontology.md](computer-ontology.md),
[mission-agentic-debugging-vtext-stability-v0.md](mission-agentic-debugging-vtext-stability-v0.md),
[mission-apps-and-changes-store-sweep-v0.md](mission-apps-and-changes-store-sweep-v0.md)

## One-Line Goal String

```text
/goal Run docs/mission-super-console-source-mount-promotion-v0.md as a Codex-operated MissionGradient mini mission: make every user, candidate, and worker computer mount the source/build workspace needed for zot/Super Console repair; prove zot can root-cause, patch, rebuild, restart, verify, and export evidence for one local bug fix inside a computer; then define the product path that classifies the local patch as personal, reusable Change/AppChangePackage, or universal platform fix. Preserve computer lineage, VText-as-artifact-surface, Super Console singleton repair mode, typed promotion, rollback evidence, and no automatic platform promotion from a local zot patch.
```

## Mission Frame

The VText/Super Console mission proved that zot can run out-of-process inside a
user computer and persist repair-session artifacts. It also exposed the next
gap: the computer does not yet reliably have the source workspace mounted where
zot can edit, rebuild, restart, and verify the running computer.

That breaks the intended repair loop:

```text
bug appears in one user computer
  -> owner opens Super Console
  -> zot inspects logs/source/runtime state
  -> zot patches the implicated source
  -> zot rebuilds/restarts this computer
  -> zot verifies locally
  -> zot writes diagnosis, patch, test output, rollback notes
  -> human/system classifies the patch for personal or broader promotion
```

The next mini mission makes source/build state a first-class mounted ledger
inside every computer, then proves the loop on one concrete bug. The current
theme-hydration bug is a good first payload: an authenticated computer with
London Salmon saved should not first paint Future Noir and only load the saved
theme after user input.

## Real Artifact

The artifact is:

```text
source-mounted computer
  -> source lineage record
  -> writable user/private source workspace
  -> readable platform baseline source snapshot
  -> local build/restart commands available to zot
  -> Super Console repair session
  -> diagnosis.md
  -> patch.diff
  -> test/build output
  -> rollback notes
  -> classification record: personal | Change/AppChangePackage | universal platform fix
  -> promotion/export path with verifier evidence
```

The artifact is not:

- a raw terminal replacing the product;
- many coding-agent consoles inside one computer;
- automatic merge from a user computer into `origin/main`;
- opaque VM snapshot promotion;
- a source checkout with no lineage, no rollback, and no verifier evidence;
- GitHub Actions as the inner repair loop.

## Invariants

- Every active, candidate, and worker computer should have an intentional source
  mount, not an accidental host path.
- Source/build state is a separate ledger from Dolt/app state, blob state,
  runtime caches, and route identity.
- Super Console remains singleton repair mode inside a computer.
- zot runs out-of-process from the runtime MAS and must not become a MAS peer,
  appagent, scheduler worker, or VText writer.
- VText remains the artifact-level surface. zot reports are ordinary markdown,
  text, patch, and log artifacts that VText can open.
- Local zot repair may mutate only that computer or its candidate fork until a
  typed promotion/export path is explicitly chosen.
- Universal fixes require platform review, CI, staging deploy, deployed
  acceptance, rollback refs, and compatibility with divergent computers.
- Personal fixes require a computer-level promotion certificate and rollback
  target, not a global deploy.
- Reusable inspiration must become a typed Change/AppChangePackage or source
  package, not a copied opaque disk.

## Source Mount Model

Every computer should expose stable source roots with distinct semantics:

```text
/Source/platform
  read-mostly snapshot of the platform baseline this computer is running

/Source/user
  writable private source/build workspace for this user's computer

/Source/candidate
  writable fork when the current computer is a candidate/review computer

/Build
  local build artifacts, caches, and restartable runtime/UI outputs

/.choir/source-lineage.json
  local projection of the durable source lineage record
```

Names may change during implementation, but the semantic split should not.
Source roots must be discoverable by zot without asking the user for a path.

Required lineage fields:

```text
computer_id
computer_kind: active | candidate | worker
owner_id
desktop_id
platform_base_commit
platform_source_mount
user_source_ref
user_source_mount
candidate_source_ref
candidate_source_mount
current_runtime_build_ref
current_frontend_build_ref
dirty_state_summary
last_verified_at
rollback_ref
```

This should connect to existing source-lineage/product APIs such as
`/api/computers/*/source-lineage`, but the user-facing object is still the
computer, not the API record.

## Repair Loop

The local loop should be fast enough to replace "open terminal, poke around,
push to CI, wait" for user-computer-level bugs:

```text
1. Open Super Console.
2. zot records session start and source-lineage snapshot.
3. zot reproduces or observes the bug through product-path commands/browser proof.
4. zot inspects unified logs, source, runtime state, and current build identity.
5. zot writes one causal diagnosis.
6. zot patches the smallest implicated source in the mounted workspace.
7. zot rebuilds the affected layer.
8. zot restarts or hot-swaps the affected runtime/UI inside this computer.
9. zot verifies through product path.
10. zot writes diagnosis.md, patch.diff, commands.jsonl, test-output.txt,
    rollback.md, and classification.md.
```

The theme-hydration proof should look like:

```text
saved owner theme = london-salmon
hard reload / fresh boot
first authenticated desktop paint already has data-theme-id="london-salmon"
no Future Noir flash before user pointer/key/focus input
theme remains London Salmon after input, reload, and session renewal
```

## Patch Classification

After local proof, classify the patch before promotion:

| Class | Meaning | Promotion Path |
| --- | --- | --- |
| Personal fix | Bespoke to one user's computer or preference/state | Candidate -> active computer promotion with rollback TTL |
| Reusable Change | Useful for other computers but not platform default | AppChangePackage/Change with Try -> verify -> install/rollback |
| Universal platform fix | Bug in the official platform/runtime/UI baseline | Lift patch to platform repo -> CI -> staging deploy -> deployed acceptance -> active computer refresh |
| Diagnosis only | No safe patch yet or root cause is external | Keep report, evidence, and next probe; do not promote |

Local zot evidence can justify a universal fix, but it does not itself merge or
deploy one. Codex or a platform-authorized agent lifts the patch into the
platform repo, preserving attribution to the zot evidence bundle.

## Dense Feedback

Minimum evidence for this mission:

- source-lineage record before repair;
- Super Console singleton session id;
- zot `session.jsonl`;
- mounted source roots visible to zot;
- bug reproduction artifact;
- patch diff generated inside the computer;
- build/restart command output;
- product-path verification artifact;
- rollback instructions or ref;
- classification record;
- if universal, platform commit, CI run, staging health identity, and deployed
  acceptance proof.

## Forbidden Shortcuts

- Do not solve the sample bug only from Codex without exercising Super Console
  and zot evidence.
- Do not mount only the platform repo on the host and call that "inside the
  computer."
- Do not hide source lineage in environment variables with no durable record.
- Do not use visual Trace as the debugging UI.
- Do not let Super Console become the normal driver of the computer.
- Do not auto-promote local zot patches to all users.
- Do not claim a universal fix without a platform deploy and deployed proof.
- Do not claim personal promotion without route rollback evidence.

## Rollback Policy

For local computer repair:

- keep the pre-repair source ref;
- keep the pre-repair runtime/frontend build ref;
- keep a `rollback.md` with exact revert/restart commands;
- if a route switch occurs, keep the previous active computer route target for a
  TTL;
- if verification fails, leave the candidate/local patch unpromoted and attach
  diagnosis.

For universal platform fixes:

- use normal git revert/forward-fix refs;
- monitor CI/deploy;
- verify staging build identity;
- refresh active computers only after deployed health is confirmed;
- record residual risks and compatibility with divergent source workspaces.

## Acceptance Criteria

1. Every new active/candidate/worker computer has source roots available at
   stable paths and reflected in source-lineage state.
2. Super Console can show zot the source-lineage snapshot.
3. zot can read and edit the mounted source for the computer it is repairing.
4. zot can run a focused rebuild and restart/hot-swap for at least one layer.
5. zot can verify the repaired behavior through product path.
6. zot emits `diagnosis.md`, `patch.diff`, `commands.jsonl`,
   `test-output.txt`, `rollback.md`, and `classification.md`.
7. The sample theme-hydration bug is fixed first in one computer through this
   loop, not only by direct Codex edits.
8. If classified universal, the patch is lifted to the platform repo and lands
   through CI, staging deploy, staging health identity, and deployed acceptance.
9. Docs explain the difference between personal source divergence, reusable
   Change/AppChangePackage, and universal platform fix.
10. No unrelated foreground user state is lost.

## Learning Side-Channel

Record the run state in this doc, and put per-repair artifacts in the user
computer filesystem under a durable zot session directory. If the sample theme
bug is used, add a short appendix to this doc with the exact source-lineage,
zot session id, local proof, and eventual promotion path.

## Initial Belief State

- The theme bug is likely caused by authenticated desktop first paint using
  `DEFAULT_THEME` before `/api/preferences/theme` hydrates the saved owner
  preference.
- A second theme/sheet bug is now confirmed on staging: tapping the Tetramark
  opens `DeskSheet`, whose full-viewport backdrop uses
  `--choir-surface-media`. The theme variable currently resolves to `#000000`,
  so the entire screen above the sheet turns black instead of preserving a
  themed desktop/sheet context.
- Super Console currently proves zot session persistence and command execution,
  but not full source-mounted local repair.
- Existing source-lineage and AppChangePackage/adoption concepts are close
  enough to reuse, but the default "every computer has source mounted" invariant
  is not yet implemented.
- The inner loop should be user-computer local; the outer loop for universal
  platform fixes still requires CI/deploy/staging acceptance.

## First Executable Probe

Use a staging or local user computer with London Salmon saved:

1. Open Super Console.
2. Have zot record source-lineage and mounted source roots.
3. Reproduce the Future Noir -> London Salmon switch requiring user input.
4. Patch the mounted source so saved theme applies before first authenticated
   desktop paint.
5. Rebuild/restart locally.
6. Verify no theme flash before input.
7. Export the patch/evidence bundle.
8. Classify as universal platform fix and lift it to the platform repo only
   after the one-computer proof exists.

## 2026-05-31 Tetramark Backdrop Regression Checkpoint

User report: when tapping the Tetramark, the screen above the sheet goes black.
It should not do that.

Codex reproduced the issue against deployed `https://choir.news` with a mobile
390x844 viewport by loading the desktop, tapping `[data-desk-menu-button]`, and
inspecting `[data-desk-sheet-backdrop]`.

Observed evidence:

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

Root-cause belief before patch: `frontend/src/lib/DeskSheet.svelte` renders a
fixed full-viewport backdrop with `background: var(--choir-surface-media)`.
`frontend/src/lib/theme.ts` currently hardcodes `--choir-surface-media` to
`#000000` for all themes, so a menu/sheet backdrop inherits the media-player
black surface instead of a sheet/backdrop token appropriate for the current
desktop theme. This is universal platform UI behavior, not a personal computer
preference.

Next repair constraint: Codex should keep acting as reproducer/verifier and run
real zot against the source to author the smallest UI patch, then lift the
result to the platform repo, CI, staging deploy, and deployed acceptance proof.

## 2026-05-31 Source Workspace Bootstrap Gap

Current code inspection shows the source-mount artifact is still mostly
implicit:

- the sandbox process receives `RUNTIME_WORKER_REPO_REMOTE`,
  `RUNTIME_WORKER_REPO_BASE_SHA`, `RUNTIME_PROMOTION_SOURCE_REPO`,
  `RUNTIME_SOURCE_LEDGER_REPO`, and `RUNTIME_PROMOTION_WORKSPACE_ROOT`;
- Super Console starts zot with `HOME`, `ZOT_HOME`, `ZOT_ROOT_DIR`, and gateway
  defaults, rooted at `SANDBOX_FILES_ROOT`;
- no sandbox startup path creates stable `Source/platform`, `Source/user`,
  `Source/candidate`, `Build`, or `.choir/source-lineage.json` roots in the
  computer filesystem;
- the product `/api/computers/*/source-lineage` record exists, but it does not
  project local mount paths or build workspace paths to zot;
- worker repo bootstrap instructions still tell agents to clone
  `go-choir-candidate` under the current directory, so worker source access is
  an instruction-level convention rather than a computer-level mounted ledger.

This means Zot can be launched and can persist sessions, but it cannot yet
discover a stable source/build workspace inside every active, candidate, and
worker computer. The next code checkpoint should add a small sandbox-owned
source workspace bootstrap that creates the directories and a durable local
lineage projection before Super Console starts. That is still not sufficient for
full personal promotion, but it is the first real substrate needed for the
repair loop.

## 2026-05-31 Zot Tool-Call Streaming Compatibility Gap

After gateway-backed Zot was installed, Codex tried to have real Zot patch the
Tetramark regression. The gateway/model path authenticated and produced model
turns, but every attempted tool call reached Zot with empty arguments:

```text
{"name":"bash","args":{}}
{"name":"read","args":{}}
```

Zot then failed with errors such as `command is required` and `path is
required`. That means Super Console can launch Zot, but Zot cannot yet perform
the repair loop's read/edit/command work through the gateway.

Root-cause hypothesis from code inspection: Zot uses OpenAI-compatible streaming
chat completions. The Choir gateway streams tool-call name frames and argument
delta frames separately. Zot appears to execute a tool call as soon as the
function name frame appears, before later argument deltas arrive. For this
client, the gateway should avoid partial streamed tool-call frames and emit a
complete tool call chunk when tools are present. This is a compatibility shim
for Zot's OpenAI-compatible path, not a new MAS role for Zot.

## 2026-05-31 Gateway Promotion Checkpoint

Platform fix promoted to `main`:

- commit: `97b2f9d2efbd8af1d665d97d20b521335a9aa065`
- title: `gateway: buffer zot streaming tool calls`
- CI: GitHub Actions run `26711182329`, success
- deploy: Node B staging deploy job `78721672005`, success
- FlakeHub publish: run `26711182331`, success
- staging health: `https://choir.news/health` reported proxy and sandbox
  deployed commit `97b2f9d2efbd8af1d665d97d20b521335a9aa065` at
  `2026-05-31T11:21:51Z`

Local verification before promotion:

```text
nix develop -c go test ./internal/gateway -run 'TestHandleOpenAIChatCompletions_(StreamingSSE|StreamingToolCallsAreComplete|RoutesThroughGateway)' -count=1
nix develop -c go test ./internal/gateway ./internal/sandbox ./cmd/gateway ./cmd/sandbox
```

Deployed Zot proof after promotion:

```text
ZOT_HOME=/var/lib/go-choir/files/.choir/zot zot \
  --provider openai \
  --model gpt-5.5 \
  --reasoning medium \
  --base-url http://127.0.0.1:8084/provider/openai/v1 \
  --api-key "$RUNTIME_GATEWAY_TOKEN" \
  --tools read \
  --max-steps 4 \
  --json "Use the read tool to read /var/lib/go-choir/files/.choir/source-lineage.json..."
```

Observed result: Zot emitted a `read` tool call whose arguments contained
`{"limit":20000,"offset":0,"path":"/var/lib/go-choir/files/.choir/source-lineage.json"}`,
successfully read the file, and answered:

```text
sandbox-m1
/var/lib/go-choir/files/Source/platform
```

Belief update: the gateway/Zot compatibility regression is fixed at platform
level. This proves real Zot can receive complete tool-call arguments through the
promoted Node B gateway and read the computer-local lineage file.

Remaining gap: this is not yet the full repair-loop proof. Zot has not yet
authored a source patch, rebuilt/restarted the user computer runtime/UI,
verified the product path, and exported a repair evidence bundle. The next
realism axis is to let Zot perform a contained source edit in the mounted source
workspace, while Codex acts as verifier and then classifies whether the resulting
change is personal, reusable, or universal platform work.

## 2026-05-31 Computer Kind And Worker Workspace Gap

The current source workspace bootstrap creates stable directories from the
sandbox process, and Super Console exports their paths to zot. That closes the
first discovery gap but not the full "every active/candidate/worker computer"
invariant.

Current inspection shows two remaining mismatches:

- Firecracker VM boot passes `vm_id`, gateway URL/token, vmctl URL, maild URL,
  and network settings through kernel cmdline, but it does not pass the product
  computer kind, owner id, desktop id, or worker id. A guest therefore has to
  infer kind from a random VM id, which is wrong for forked candidate desktops
  and worker VMs.
- Worker delegation prompts still instruct agents to create or use
  `go-choir-candidate` under the current directory and publish with
  `repo_path "go-choir-candidate"`. That preserves the old ad hoc checkout
  convention instead of making `Source/platform`, `Source/user`,
  `Source/candidate`, `Build`, and `.choir/source-lineage.json` the repair
  substrate.

Next code checkpoint: propagate computer kind/owner/desktop/worker identity
into guest env, teach source-lineage bootstrap to use it, and update worker repo
bootstrap instructions to clone/edit/publish from `Source/candidate`. This still
does not prove the full zot patch/build/restart loop, but it makes worker and
candidate computers line up with the source-mount artifact rather than a
parallel convention.

## 2026-05-31 Deploy-Time VM Refresh Identity Gap

Platform source-workspace identity propagation was lifted to `main` in commit
`59bfd260a245a2cf57b762d691e1f6f614e29bd9`, and staging health reported that
commit for both proxy and sandbox at `2026-05-31T11:32:31Z`. GitHub Actions run
`26711401341` and its Node B deploy job completed successfully.

However, live Node B VM config inspection after deploy still showed recent
`/var/lib/go-choir/vm-state/*/fc-config.json` boot args containing only product
runtime basics such as `vm_id` and `epoch`, with no
`choir.computer_kind`, `choir.owner_id`, `choir.desktop_id`,
`choir.worker_id`, or `choir.candidate_id` args. That means the platform code is
deployed, but refreshed/recovered running computers are not yet product-path
evidence for the new guest identity contract.

Current root-cause belief: deploy refresh is reusing persisted `VMConfig`
records through `RefreshVM`/`RecoverVM` paths. Those persisted records predate
the new identity fields, so `BootVM` correctly preserves and reboots an old
empty identity config. Fresh active/candidate/worker boots have code paths that
populate the fields, but deploy-time refresh must also rehydrate missing
identity from the ownership registry before asking the VM manager to rebuild
Firecracker config.

Next code checkpoint: teach the vmctl ownership refresh/recover path to merge
the current ownership identity into any refreshed VM manager config before boot.
The acceptance evidence should include a deployed Node B `fc-config.json` whose
boot args contain the expected `choir.computer_kind`, owner, desktop, and
worker/candidate identity fields after refresh, not just local tests.

## 2026-05-31 Hot Refresh Is Not Boot-Contract Refresh Gap

Commit `2ef575b3024227e1fa590f46da76f9adaec4c1b1` deployed the vmctl/vmmanager
identity rehydration fix to staging. CI and deploy succeeded, and
`https://choir.news/health` reported the same commit for proxy and sandbox.

Live Node B inspection still showed no `choir.computer_kind` boot args in
`/var/lib/go-choir/vm-state/*/fc-config.json`. Journal evidence explained why:
the deploy restarted vmctl and reattached existing Firecracker processes, then
used the sandbox hot-refresh path for active interactive computers. No
Firecracker config was rewritten after the new commit.

This is a deploy-impact classification bug, not evidence that the identity
rehydration code failed. Changes to vmctl, vmmanager, or VM boot-contract code
may not require rebuilding the guest image, but they do require a full active
computer refresh because kernel args and Firecracker config are only produced
on VM boot. Sandbox hot-refresh is sufficient for runtime package changes, but
it cannot prove or apply a boot-contract change.

Next code checkpoint: add an explicit active VM full-refresh deploy impact class
separate from ordinary guest image rebuild. Mark vmctl/vmmanager and
boot-contract-affecting vmctl changes with that class, and make manual forced
deploys able to exercise it. The acceptance proof remains a post-deploy
`fc-config.json` containing `choir.computer_kind` and the ownership identity
fields.
