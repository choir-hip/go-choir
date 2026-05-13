# Local vmctl Product Path Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

The local developer launcher now starts `vmctl` in host-process fallback mode as part of the normal stack:

- gateway starts first;
- `vmctl` starts on `VMCTL_PORT` with `VMCTL_SANDBOX_URL_BASE` pointing at the local sandbox;
- sandbox receives `RUNTIME_VMCTL_URL`, so super tools can call `request_worker_vm`;
- proxy receives `PROXY_VMCTL_URL`, so authenticated product traffic resolves through VM ownership before reaching the sandbox.

This does not prove Firecracker isolation. It proves the normal local dogfood path now contains the same control-plane edge used by production-like candidate worlds.

## Observed Transition

Direct worker allocation without a parent desktop correctly fails:

```text
POST /internal/vmctl/request-worker
-> 400 no parent desktop VM found for user dogfood-user desktop primary
```

After resolving the owner primary desktop:

```text
POST /internal/vmctl/resolve
-> vm_id=vm-2db52191d6e34e329549c24c7816947a
-> sandbox_url=http://127.0.0.1:8085
-> state=active
```

The same owner can request a typed worker handle:

```text
POST /internal/vmctl/request-worker
-> kind=worker
-> worker_id=worker-9c2f5a7b6e528745
-> vm_id=vm-1994b8820c4ae6797c2abc08ed1466b7
-> sandbox_url=http://127.0.0.1:8085
-> state=active
```

This precondition is useful product-path shape: authenticated browser traffic through the proxy resolves the primary desktop before app/super work tries to allocate a worker.

## Verification

Commands run against the patched launcher stack:

```text
./start-services.sh && sleep 3600
```

Result: services started successfully, including `vmctl`.

```text
curl -sf http://127.0.0.1:8083/health | jq .
```

Result: `status=ok`, `service=vmctl`.

```text
curl -sf http://127.0.0.1:8082/health | jq .
```

Result: `vmctl_routing=enabled`, `vmctl_url=http://127.0.0.1:8083`, upstream sandbox healthy.

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `40 passed`.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/vmctl ./internal/proxy
```

Result: passed.

## Boundary

This proves local control-plane reachability and proxy/sandbox wiring. It is not yet a full Choir-in-Choir proof because the patch was still made by Codex, and no live product prompt caused super to allocate a worker, delegate vsuper work, export a patchset, verify, promote, compact, and continue.

It also does not prove local mutation isolation. On this Mac, `vmctl` is using host-process fallback and the worker handle points at the same sandbox URL as the foreground runtime. The follow-up local worktree fallback mitigates ordinary repo mutation risk for bounded dogfood, but Firecracker remains the strong isolation target.

The next error is no longer "local stack lacks vmctl." The next error is "the product path has not yet initiated and observed the full super -> worker -> export -> promotion loop."

## Next Deformation

Use the prompt bar or a VText-owned `request_super_execution` path to ask Choir for one narrow self-development change. The successful low-resolution loop should:

- resolve the owner desktop through product traffic;
- let super request a worker handle;
- delegate one bounded vsuper/co-super run;
- export a patchset;
- queue the promotion candidate;
- render the candidate in Settings;
- verify with a named contract;
- promote only after explicit approval;
- write run memory/continuation for the next objective.
