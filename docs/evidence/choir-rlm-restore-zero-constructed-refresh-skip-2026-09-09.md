# Restore-Zero: constructed-computer deploy skip — 2026-09-09

Class: green (problem documentation; no runtime behavior change).
Authority: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md`.
Mutation class of the *named problem*: red (protected: Texture/canonical computer identity, vmctl refresh, constructed `ComputerVersion`, deploy routing). This file does not repair it.

This is not completion. Guest `/health` `ready` is not tail-only recovery proof.

## Problem

The snapshotting contract in `9341b5d1` is installed on the staging *host* and in the autoputer *package pointer*. It is not executing on the retained owner computer.

`computer-03335285269bdba4f94377e56879f9e6` is `snapshot_kind: constructed-computer-version`. CI `DEPLOY_ACTIVE_VM_REFRESH` therefore preserves it and records `active_computers.status=empty`. The guest Firecracker process was reattached, so it kept serving autoputer `80d43427`. PlanRecovery rebase/resume never ran. Prefix-read count for seq ≤ W is unobserved.

This is the same authority boundary named in `docs/evidence/g4-immutable-canary-deploy-refresh-blocker-2026-07-17.md`: a later global host/autoputer deployment must not rewrite a constructed `ComputerVersion` in place. Restore-Zero's session assumption that "CI refresh of the owner VM is the product path to a retained-store boot of `9341b5d1`" is false for this computer.

## Evidence (observed 2026-09-09, UTC)

### Source and deploy

- Source: `main@9341b5d18d5b962a4440671233efd402038a8b8a` (`red(restore-zero): rebase retained stores onto advertised watermark`).
- CI: [run 34323073581](https://github.com/choir-hip/go-choir/actions/runs/34323073581).
  - Attempt 1: `Deploy to Staging (Node B)` failed at 2026-09-09T07:29:41Z during NixOS closure copy (`error: reference '...-make-shell-wrapper-hook' of path '...-nodejs-install-executables' is not a valid path`). Incomplete receipt: `/var/lib/go-choir/deploy-failures/34323073581-1.json`. Guest not refreshed.
  - Attempt 2: same job succeeded; run `status=completed conclusion=success` at 2026-09-09T07:49:27Z.
- Deploy receipt `/var/lib/go-choir/deploy-receipt.json`:
  - `target_commit`: `9341b5d18d5b962a4440671233efd402038a8b8a`
  - `activated_at`: `2026-09-09T07:49:24Z`
  - `github.run_id`: `34323073581`, `run_attempt`: `2`
  - `artifacts.autoputer`: `{commit: 9341b5d1…, status: installed}`
  - `artifacts.active_computers`: `{commit: 9341b5d1…, status: empty}`
- Host autoputer package `build.json`: `commit 9341b5d18d5b962a4440671233efd402038a8b8a`, `built_at 20260909055454`.
- Proxy `https://choir.news/health` and local `:8082/health`: `status=ok`, `vmctl_status=ok`, `build.commit=9341b5d1…`, `deployed_at=2026-09-09T07:49:24Z`.

### Watermark and head (unchanged through this deploy)

- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Advertised W: `148431`, `base_ref=6099cf6998ac87c7e763cdbbb4e2ae051ae097815e49bf795dc5b008d61fc599`
- Canonical H: `148431`, `canonical_event_head=1eb9b93170bb09249381c032590f5bd1f869dc0cca84442b9a6a2e4c017907f4`
- Tail `H−W = 0`. A resume-at-head boot of the *new* binary would be legal (`tail=0`) if that binary actually booted this store.

### Owner computer after deploy

`GET http://127.0.0.1:8083/internal/vmctl/list` (X-Internal-Caller), ownership row:

| field | value |
| --- | --- |
| computer_id | `computer-03335285269bdba4f94377e56879f9e6` |
| vm_id | `candidate-fleet-e15cb89f25d963c220319b7b` |
| user_id | `5bd6de97-3b58-408c-bf89-c42c81b083de` |
| desktop_id | `primary` |
| kind | `interactive` |
| state | `active` |
| computer_url | `http://10.200.6.2:8085` |
| snapshot_kind | `constructed-computer-version` |
| warmness_class | `premium_always_on` |
| epoch | `893` |
| construction_version.code_ref | `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380` |
| construction_version.artifact_program_ref | `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f` |
| construction_disk_receipt_id | `disk-instantiation:sha256:af1ac24e778ce9abfdbb5ceb0c9f26f628d26fc86ab80c7763a11cdfc4c97e2e` |

Guest `GET http://10.200.6.2:8085/health`:

- `status=ready`, `runtime_health=ready`
- `build.commit=80d434279adbdd0a1243ec35feabe3bf11a00129`
- `build.deployed_at=2026-09-09T05:31:19Z` (pre-`9341b5d1` deploy)
- disk `used_bytes=16602869760`, `avail_bytes=16898887680` (~16.9 GiB free)

vmctl after NixOS switch (07:49:05Z):

```text
vmmanager: reattached VM candidate-fleet-e15cb89f25d963c220319b7b (host=http://10.200.6.2:8085 pid=641150 epoch=893)
```

Firecracker pid `641150` survived the vmctl unit stop; reattach kept the old guest binary. No `autoputer: projection recovery resume|rebased|required projection base refused` line exists for this computer after 07:40Z. No `/tmp/go-choir-vm-refresh-*.json` bodies were left (refresh loop did not run).

### Why CI did not refresh it

`.github/workflows/ci.yml` (deploy script, `DEPLOY_ACTIVE_VM_REFRESH=true`):

- Preserve: `select((.snapshot_kind // "") == "constructed-computer-version")`
- Refresh: `select((.snapshot_kind // "") != "constructed-computer-version")` and `held != true`

All three *active* ownerships on Node B at this observation are `constructed-computer-version`:

| computer | url | guest commit | note |
| --- | --- | --- | --- |
| `computer-03335285269bdba4f94377e56879f9e6` | `10.200.6.2:8085` | `80d43427` | owner; reattached |
| `computer-4c20ff4a21a021c4306d8c783be0037d` | `10.200.0.2:8085` | `c3314c59` | platform; reattached pid `1460048` |
| `computer-bb0f4fa583c0cde14334818d946e6378` | `10.200.11.2:8085` | `9341b5d1` | genesis boot, see below |

`active_computers.status=empty` is the honest receipt that G4 required, not a silent deploy miss.

### Non-proof: genesis guest on `9341b5d1`

`computer-bb0f4fa583c0cde14334818d946e6378` reported `build.commit=9341b5d1` at `deployed_at=2026-09-09T07:49:12Z`. vmctl leftover-process log listed Firecracker pids `1460048` and `641150` only; pid `646957` (`candidate-fleet-49ee3bd0…`) was not left over. At 07:49:14Z:

```text
autoputer: projection recovery genesis for computer-bb0f4fa583c0cde14334818d946e6378 (local=0 W=0 H=0 tail=0)
```

That is an empty-store genesis on a different computer. It does not prove retained-store `(W,H]` recovery for the owner. Disk used ~175 MiB vs owner ~16.6 GiB.

### Attempt-1 Nix copy error (separate, contained)

Attempt 1 failed before guest refresh. Attempt 2 completed the host OS switch. `/nix/var/nix/profiles/system` advanced; `/tmp/go-choir-nixos-result` → `37sbdf847xzc2wh8wpxrd36hgwdxsb8q-nixos-system-go-choir-b-26.11.20260701.ec84054`. That store-copy flake is not the Restore-Zero remaining error. Do not host-mount guest disks to "finish" snapshotting.

## Belief state

- `9341b5d1` is the staging host/proxy/autoputer-package identity. It is **not** the owner computer's effective guest identity.
- Advertised W=148431 at live H is a legal resume (`tail=0`) *if* the new boot path runs against this retained store.
- The owner computer will not pick that path up from platform deploy while it remains `constructed-computer-version` and its Firecracker process stays alive.
- In-guest rematerialize/restore on the still-running `80d43427` binary still has the nonempty-store skip; it cannot produce PlanRecovery proof.
- SSH `vmctl/refresh` would be host-shell surgery of a constructed computer, not the G4 product path, and is refused here.

## Remaining error

No owner-reachable product path has yet booted `9341b5d1`'s snapshotting contract against the retained owner store and shown prefix reads (seq ≤ W) = 0.

Candidate product paths (not chosen, not executed):

1. Owner-scoped rematerialize/restore on the live `80d43427` guest — will not run `PlanRecovery` from `9341b5d1`.
2. Owner-scoped `recover_current` / cold-recover if that path starts a new VM from the current autoputer package onto the retained disk without rewriting `ComputerVersion` identity. Unknown until the entrypoint is read against this computer's construction bindings.
3. A constructed-computer update that publishes a new `ComputerVersion` whose ArtifactProgramRef includes the snapshotting binary — a different mutation class than platform deploy refresh.
4. Weakening CI to refresh constructed computers — would reopen the G4 heresy (deploy mutates frozen CodeRef/ArtifactProgramRef join). Not a Restore-Zero default.

## What this file does not authorize

- SSH refresh, host-mount of `data.img`, in-place overwrite of live SQLite, or treating guest `/health` as acceptance.
- Marking Restore-Zero complete.
- Changing the G4 constructed-computer deploy skip in the same breath as this observation.

## Rollback

Docs-only. Revert this commit to restore prior belief text. Staging deploy `9341b5d1` and advertised W=148431 remain; they are not rolled back by this receipt.

## Heresy delta

- discovered: Restore-Zero retained-store boot cannot be proven via CI active-VM refresh on a `constructed-computer-version` owner computer; platform SHA and computer effective identity diverged on purpose.
- introduced: 0
- repaired: 0
