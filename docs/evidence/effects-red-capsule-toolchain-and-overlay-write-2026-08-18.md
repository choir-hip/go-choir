# Effects capsule toolchain mount and overlay directory permissions — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `88652f064341e92684eda537add5dcdec0df1680` (deployed `2026-08-18T02:19:02Z`).

## Live observation

Following the deploy of `88652f06` (cancellation delivery fix), the epoch-295 refresh and operations POST succeeded: Super run `3a25f0b8` saw that prior attempts were cancelled and opened a fresh implementation CoSuper assignment `assignment-ba37afb1-2aa2-5569-b7de-127b68d5341f` in capsule `capsule-4f9013fd-882e-5e28-83d4-ddb593d12765`.

The CoSuper ran for ~5.5 minutes and completed with a blocker report:

```
1. capsule_exec failed for all commands with exec: "bash": executable file not found in $PATH
2. capsule_write_file returned permission denied for all paths attempted
```

## Root cause

Two substrate defects broke execution and writing inside the chrooted capsule namespace:

1. **Lower directory mode 0o555 blocked overlayfs writes:** `makeSubjectTreeReadOnly` in `internal/capsule/source_snapshot.go` stripped write bits from directories (`0o555`). When a process runs inside a user namespace mapped to host UID 65534, the Linux kernel's VFS permission check on the lower directory inode returns `-EACCES` ("permission denied"), preventing overlayfs from creating files or subdirectories in the upper layer. Lower directories must be `0o755` so the VFS permission check passes and overlayfs directs the write to `upperDir`.

2. **`/run` tmpfs masked NixOS system toolchain:** `prepareCapsuleRoot` in `internal/capsule/executor.go` mounted a tmpfs over `/run`, masking `/run/current-system/sw/bin` where NixOS links `environment.systemPackages` (`bash`, `go`, `git`, `make`, `gcc`, etc.). Furthermore, `/tmp` tmpfs was mounted with mode `0o755` owned by host root rather than world-writable sticky mode `1777`.

3. **Broker shell fallback:** `cmd/capsule-broker/main.go` invoked `bash` directly without falling back to `/run/current-system/sw/bin/bash` or `sh`.

## Repair

1. `internal/capsule/source_snapshot.go`: `makeSubjectTreeReadOnly` sets directory mode to `0o755` (files remain `0o444`/`0o555`).
2. `internal/capsule/executor.go`: `prepareCapsuleRoot` mounts `tmp`, `mnt`, and `run` with `mode=1777` and bind-mounts `/run/current-system` into `root/run/current-system` so system binaries are accessible inside the chroot. `unmountCapsuleRoot` cleans up `/run/current-system`.
3. `cmd/capsule-broker/main.go`: `handleExec` checks `/run/current-system/sw/bin/bash` and falls back to `sh` if `bash` is not found on PATH. Default PATH includes `/run/current-system/sw/bin`.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stays `executing`, no bundle. Constructed freeze `7122f279` unchanged. Mode `propose_only` gen 1. No mail. This is not a freeze.
