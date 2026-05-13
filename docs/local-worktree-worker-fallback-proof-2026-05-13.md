# Local Worktree Worker Fallback Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Same-runtime worker delegation now fails closed unless local worktree isolation is enabled.

When `delegate_worker_vm` sees that `worker_sandbox_url` is the same as the current runtime URL:

- without `RUNTIME_LOCAL_WORKER_MODE=worktree`, delegation is refused;
- with `RUNTIME_LOCAL_WORKER_MODE=worktree`, the runtime creates a git worktree from the foreground repo HEAD;
- the worker run receives `tool_cwd` metadata pointing at that worktree;
- file, coding, git, and `export_patchset` tools use the run-specific `tool_cwd`;
- the worker prompt is prefixed with base SHA and export instructions;
- foreground cwd is not mutated by ordinary worker tool use.

`start-services.sh` now enables the local fallback:

```text
RUNTIME_SELF_URL=http://127.0.0.1:${SANDBOX_PORT}
RUNTIME_LOCAL_WORKER_MODE=worktree
RUNTIME_SUPER_FOREGROUND_MUTATION_MODE=worker_only
```

The foreground-super guard blocks direct `bash`, `write_file`, `edit_file`, and `export_patchset` when super is running without an isolated `tool_cwd`. Super can still inspect, request workers, and delegate. Worker profiles with isolated `tool_cwd` remain able to mutate their worktree and export patchsets.

## Why This Matters

This turns the previous local blocker into a bounded bootstrap path. On a Mac without Firecracker, `vmctl` still returns the same sandbox URL for foreground and worker runtimes, but mutable worker tool execution can now happen in a separate git worktree instead of the foreground repo directory.

This is not a substitute for microVM isolation. It is a local candidate-world approximation with the same artifact shape:

- worker id;
- branch/worktree identity;
- base SHA;
- worker head;
- exported patchset;
- queued promotion candidate.

## Verification

Commands run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestDelegateWorkerVMRefusesSameRuntimeWithoutIsolation|TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD|TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport|TestRuntimePromotionQueueDogfoodsLauncherUploadsThemesPatch'
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestForegroundSuperMutationGuardBlocksWritableTools|TestDelegateWorkerVMRefusesSameRuntimeWithoutIsolation|TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD|TestRuntimePromotionQueueDogfoodsLauncherUploadsThemesPatch'
```

Result: passed.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store ./internal/promotion
```

Result: passed.

```text
bash -n start-services.sh
```

Result: passed.

```text
./start-services.sh && sleep 3600
curl -sf http://127.0.0.1:8082/health | jq .
curl -sf http://127.0.0.1:8083/health | jq .
```

Result: services started successfully; proxy reported `vmctl_routing=enabled`; `vmctl` health returned `status=ok`.

```text
git diff --check
```

Result: passed.

## Boundary

The worktree fallback protects the normal working directory and promotion geometry, but it is not a security sandbox. A shell command can still intentionally write outside the worktree. Firecracker or another OS-level sandbox remains required before treating arbitrary autonomous mutable work as strongly isolated.

## Next Deformation

The first product-route deformation is now covered by `docs/prompt-product-path-worker-promotion-proof-2026-05-13.md`: prompt bar to VText, VText to super, super to vmctl worker request, local worktree worker delegation, export, and queued promotion candidate.

Next, run the same shape through the visible desktop with Playwright, then carry the queued candidate through Settings review and verifier-contract execution before any canonical mutation.
