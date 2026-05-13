# Prompt Product Path Worker Promotion Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Added deterministic runtime coverage for the product path:

```text
prompt bar -> conductor -> VText -> request_super_execution -> super -> request_worker_vm -> delegate_worker_vm -> local worker worktree -> export_patchset -> promotion queue
```

The test is `TestPromptBarToWorkerWorktreePromotionQueueDeterministic`.

It uses real runtime handlers/tools for the topology boundary:

- public `POST /api/prompt-bar`;
- VText-owned `request_super_execution`;
- super-only `request_worker_vm`;
- vmctl `request-worker`;
- super-only `delegate_worker_vm`;
- worker runtime internal run endpoints;
- worker `bash` and `export_patchset`;
- runtime promotion queue persistence.

The only synthetic part is the provider response sequence, which makes the proof deterministic. The test does not manually seed a promotion candidate and does not bypass the runtime route.

## Guarantees

- Prompt-bar routing opens a VText document before privileged execution.
- VText requests the persistent super; the browser does not request super directly.
- Super requests a worker VM handle through vmctl before delegation.
- Same-runtime worker delegation uses local worktree isolation.
- Worker tool execution writes to the isolated worktree, not the foreground repo.
- Worker export produces a patchset and queues exactly one promotion candidate.
- The queued promotion candidate records the foreground base SHA.
- Canonical foreground files are not mutated by worker execution.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestPromptBarToWorkerWorktreePromotionQueueDeterministic'
```

Result: passed.

## Boundary

This is a deterministic runtime/product-handler proof, not yet a live Playwright dogfood where Choir prompts itself through the visible desktop and Codex only observes. It proves the product-path topology and promotion queue bridge are wired enough for that next deformation.

It also remains a local worktree fallback proof, not Firecracker-grade isolation. A malicious shell command could still intentionally write outside the worktree until OS-level sandboxing is active.

## Next Deformation

The first visible Playwright pass is now recorded in `docs/live-playwright-worker-dogfood-proof-2026-05-13.md`.

Next, add idempotency and portfolio control so one product prompt does not create duplicate equivalent workers/candidates unless the objective explicitly asks for a candidate portfolio.
