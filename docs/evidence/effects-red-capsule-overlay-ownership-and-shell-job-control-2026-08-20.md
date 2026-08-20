# Staging Evidence: Capsule Overlay Ownership and Shell Job Control Blocker

- Date: 2026-08-20
- Mutation Class: Red
- Computer ID: `computer-03335285269bdba4f94377e56879f9e6`
- Affected Subsystem: `internal/capsule/executor.go` (`prepareCapsuleRoot`), `cmd/capsule-broker/main.go` (`handleExec`)

## Summary

Live execution of CoSuper implementation assignments on staging (e.g. `run:assignment-77df1b70-4203-5e8c-8f30-aa64afe54589` and `run:assignment-22c0ca42-2724-551f-bd46-8081870c3bac`) completed with structured blocker reports identifying two issues in the capsule runtime:

1. **Permission Denied / Read-Only Filesystem**: Host-side execution of `prepareCapsuleRoot` creates directories and identity files inside `upperDir` with Host UID 1000 ownership and mode 0755. Inside the capsule user namespace, the broker runs as Container UID 0 mapped to Host UID 65534 (`capsuleNamespaceHostID`), which has no write permissions on UID 1000-owned subdirectories in `upperDir`.
2. **Job Control getpgrp Error**: When `capsule_exec` executes `sh -c` inside the PID namespace without process group management, shells that link to bash attempt job control initialization and fail with `sh: initialize_job_control: getpgrp failed: Success`.

## Repair Strategy
1. In `prepareCapsuleRoot`, recursively chown the entire `upperDir` tree to `capsuleNamespaceHostID` after preparing root and identity files, ensuring Container UID 0 has full read/write permissions across all upperdir paths.
2. In `cmd/capsule-broker/main.go`, prefer `bash --noprofile --norc -c` and explicitly configure `SysProcAttr{Setpgid: false}` to prevent job control initialization failures.
