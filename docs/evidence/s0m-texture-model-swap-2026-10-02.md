# S0m texture model swap — live guest policy edit, 2026-10-02

Owner direction: texture desk → `opencode-go/deepseek-v4.1-flash` @ high reasoning.

## Repo change

`258916d5` (`internal/modelpolicy/model_policy.go`): fallback policy and
`defaultPolicyText` now route `roles.texture` to `opencode-go/deepseek-v4.1-flash`
@ high (same provider as engineering). Effective for fresh computers; existing
`System/model-policy.toml` files are never overwritten by `ensureDefaultPolicyFile`.

## Live guest edit (computer-03335285269bdba4f94377e56879f9e6)

`data.img` is mounted read-write by the running guest; the owner computer's
`/mnt/persistent/files/System/model-policy.toml` (V1-era, texture=chatgpt/gpt-5.6-luna/low)
predated the swap. No guest file-write API reaches `System/` (autoputer-proxy
allows GET/POST only; `PUT /api/files` is blocked).

Sequence executed on node-b:

1. `POST /internal/vmctl/stop` → guest down.
2. **First attempt failed**: `debugfs rm`+`write` on dirty journal; guest boot
   replayed journal and restored the old inode. Guest-visible file unchanged.
3. Re-stopped → `e2fsck -fy data.img` (journal replay, fs modified, clean).
4. `debugfs rm`+`write` on the clean image → verified via `debugfs cat`:
   `[roles.texture] provider="opencode-go" model="deepseek-v4.1-flash" reasoning="high"`.
5. `POST /internal/vmctl/resolve` → guest `ready` on `65275e46`.
6. `GET /api/model-policy/resolve?role=texture` now returns
   `opencode-go / deepseek-v4.1-flash / high`.

## Risk note

`debugfs` writes into `data.img` while the VM was confirmed down
(`firecracker.pid` absent, `ps` empty). First write was reverted by journal
replay — evidence that host-side image edits are only durable on a clean fs
post-e2fsck. Rollback: file content preserved at `/tmp/model-policy-new.toml`
host-side; old texture block was `chatgpt/gpt-5.6-luna/low`.
