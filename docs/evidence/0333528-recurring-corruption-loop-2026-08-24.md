# Incident: 0333528 recurring corruption/instability loop

Date: 2026-08-24 (verified live ~21:55Z)
Computer: `computer-03335285269bdba4f94377e56879f9e6` (owner yusefnathanson, owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`, VM `candidate-fleet-e15cb89f25d963c220319b7b`)
Class: **problem documentation (no fix in this commit — problem-documentation-first)**

## The problem

The retained computer 0333528 is in a **self-reinforcing recover → corrupt → recover loop**, not a one-off failure. The observed cycle, verified live 2026-08-24 ~21:35–21:55Z:

1. Guest is alive (autoputer-runtime pid 1032, kernel uptime ~4.6h) and is running the **effects/self-dev orchestration topology**: `autoputer: tool profiles enabled (conductor=2 super=24 … texture=6)` and `orchestration topology (super=1, researchers…)`. This Super/Conductor/candidate workload is the driver.
2. Tape is **pre-genesis** (no canonical genesis), so the runtime gate (`internal/agentcore/runtime.go:685-693`) refuses **every** run admission every ~3s: `runtime api: submit internal run: computer is pre-genesis: run admission refused (bootstrap-chain required; no canonical genesis on the tape)`.
3. vmctl marks the computer **unhealthy** (guest health check on `http://10.200.60.2:8085` fails) and attempts restart, incrementing `epoch` 771→772→773→**774** (at 21:35Z it was 771). Each reattach fails: `credential issue ... Post "http://127.0.0.1:8086/internal/comput...ials/issue": dial tcp 127.0.0.1:8086: connect: connection refused`, `guest health check failed for reattach VM`, `warm… failed to resume always-on desktop vm=e15cb89 user=5bd6de97…`.
4. Gateway symptom (user-visible): `proxy: failed to resolve computer surface for owner 5bd6de97… desktop primary: resolve autoputer canceled after transient vmctl error: vmctl client: resolve call failed: Post ".../internal/vmctl/resolve": context canceled` → Choir BIOS "Resolving active computer / Bootstrap probe 1/2/3 lost contact / Recovery request failed; continuing (bootstrap pending) / BOOTSTRAP REQUEST FAILED" — the app never loads.

## The verified mechanism (causal chain)

- **Driver**: the effects/self-dev mission (Super/Conductor/researchers) is the workload in the guest. It generates events; the tape/workspace grows (canonical head `132,539 / acc54c39…`; workspace 7–9 GiB; disk 11.6 GiB / 33.5 GiB). This growth is what makes replay slow and memory pressure real.
- **Original corruption trigger** (2026-08-23, recovery def): "Initial break: **capsule memory exhaustion + assignment supersession loop**," then boot timeouts because autoputer replays the full event chain before opening :8085. systemd-oomd is active in the guest. The OOM + mid-write kill → store corruption → `data.img` quarantined (the `data.img.quarantine-1/2-*` images, 2026-08-23 13:27, 19:00; 2026-08-24 16:58).
- **Boot replay O(history)**: at the 107k+ band the guest replay costs **3.2–6.5s/event** (0.2–0.3 ev/s) on the 4096 MiB guest (from `docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md`; the preflight restart-loop receipt: "the same guest replay I/O ceiling manifesting as a live boot loop"). This exceeds the boot/health window → the guest never reaches :8085-ready → vmctl unhealthy → kill/reboot → **restart loop**.
- **Credential TTL < boot**: credential TTL ~4 min < guest boot ~5 min (preflight PF-2 measurement receipt) → the credentials issue path (8086) fails → clean reattach/resume is impossible.
- **Latest trigger (pre-genesis)**: the 2026-08-24 21:13Z deploy refreshed `data.img` (epoch 715→768→771→774); the refreshed image's local tape has **no canonical genesis** → the run-admission gate refuses everything. The 2026-08-13 `bootstrap-chain` genesis (bound to the then-current guest release identity) does not survive the identity change + image refresh.
- **Substrate gaps (why unrecoverable/recurring)**: per `docs/designs/choir-durable-substrate-2026-08-23.md` §1 — files not in tape; mail host-side per-owner SQLite; DEK is an unrecoverable mode-0400 guest file; recovery is O(history) at boot. Plus: Dolt GC off (`RUNTIME_DOLT_GC_DISABLED=1`), no guest reclaim for large stores, `persistent_disk` warning (8 GiB default cap, 11.6 GiB used). These are exactly what the overhauls (Tracks K/F/M/Assurance) fix.

## The loop, stated

```
self-dev workload grows tape/workspace
  → OOM (capsule memory exhaustion) / mid-write corruption
  → data.img quarantined
  → boot replay O(history) exceeds health window  (credential TTL < boot too)
  → restart loop / down
  → recover (B14 replay-only / bootstrap-chain)
  → self-dev resumes
  → repeat; plus deploy image-refresh re-triggers pre-genesis
```

The immediate blocker is **pre-genesis** (no canonical genesis after the deploy's image refresh). But the *root cause* of recurrence is the substrate: the self-dev workload in a substrate with no durable file-CAS, no key escrow, unbounded replay, Dolt GC off/disk cap, and no guest-side OOM containment. bootstrap-chain or B14-rematerialize unblocks the *current* state but does **not** stop recurrences — the same loop re-runs.

## Belief state

- **Confirmed**: pre-genesis refusal (live), restart loop (epoch 771→774, unhealthy→reattach fails), credential issue 8086 refused, self-dev orchestration topology in guest, O(history) replay ceiling, substrate gaps, quarantine cascade (3 images), platform head 132,539/acc54c39 survived recovery.
- **Confirmed (documented)**: the 2026-08-13 bootstrap-chain and the 2026-08-24 B14 recovery both fixed the *then-current* broken state; neither is durable against the driver + deploy refresh.
- **Uncertain/unresolved**: whether the platform canonical head (132,539/acc54c39) is still the authoritative chain after the 21:13Z image refresh, and whether `bootstrap-chain` would replay-converge to it vs mint a fresh sequence-1 genesis (the open consensus question); which on-disk image (`data.img.pre-hostdrive-20260824` vs live `data.img` vs `data.img.quarantine-1-89c24…`) carries a matching local head.

## Remaining error / next research (no fix here)

- Read-only audit to resolve the split: (a) confirm the platform canonical head survives (132,539/acc54c39); (b) determine whether `bootstrap-chain` converges the refreshed tape to 132,539 or mints a new genesis; (c) identify which on-disk image carries the matching local head.
- Then decide the stable gate before overhauls: quiesce the self-dev workload (effects OFF, `running_runs=0`), establish a servable genesis/head, get disk headroom, and only then begin the K/F/M/Assurance overhauls (which run on a **test computer**, per their `acceptance` environment, not the retained 0333528).
- The overhauls definition (`choir-durable-substrate-overhauls-2026-08-23.md`) remains the durable fix; its `not_goals` exclude immediate 0333528 recovery, so the recovery/receipt path must land before Track K.

## References

- `docs/definitions/choir-durable-substrate-recovery-2026-08-23.md` (recovery, B14, root-cause "capsule memory exhaustion + assignment supersession loop")
- `docs/definitions/choir-durable-substrate-preflight-2026-08-24.md` (restart-loop receipt, PF-2 ceiling, PF-4 GC policy, credential-TTL findings)
- `docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md` (the durable fix; test-computer acceptance)
- `docs/designs/choir-durable-substrate-2026-08-23.md` §1 (the four substrate gaps)
- `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md` line 78 (2026-08-13 bootstrap-chain precedent)
- `docs/incident-vm-bootstrap-stale-route-2026-06-09.md` (analogous bootstrap-probe failure)
