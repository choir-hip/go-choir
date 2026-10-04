# S0b disposable computer terminal disposition

**Computer:** `computer-ac1808b4f04fc749c0781083ee747403`  
**Owner:** `69fcbf14-7fa7-442b-9e69-9c2666274b9e`  
**VM:** `vm-b59963240d2cd8da769b92cacb213802`  
**Observed terminal state:** 2026-10-04T07:11Z

## Terminal disposition

**Left running for the next session.** vmctl reports `active`; guest `/health` reports `ready`; `running_runs` and `running_processor_runs` are both `0`; guest `/api/runs` is empty. Self-development mode was explicitly returned to `off` (generation 2).

## Created / changed

- One M9a platform-update offer and its durable tape sequence (accepted, started, applied, checkpoint published) using `s0b-m9a-20261004-0707`.
- The M9a release remains active with marker `S0B_M9A_20261004_0707` and release digest `2e874fb951940e8156c4e50882ca96180763a51289555bbbab8433540f7739f1`.
- Product restore checkpoint `128b9d7d294e235b935816d27d15d75162b8deb4973d72c0cc99e5945abe63b6`; restore completed with `witness_matched: true` and `frontend_restaged: true`.
- The Go-effect keydriver omitted `computer:self_development:propose`; its operation was correctly refused. The attempted replacement key (the required propose/mode/runtime scopes) was also correctly refused because the attenuated key has neither `manage:keys` nor `admin`; no same-owner browser cookie survived the original disposable registration. No self-development operation, Go cell, capsule, or run was created.

## Left running

- The Firecracker VM and its host state directory under `/var/lib/go-choir/vm-state/vm-b59963240d2cd8da769b92cacb213802/`.
- The active M9a release and its checkpoint/quarantine artifacts on the disposable persistent disk.

## Teardown

This entire computer/VM and its persistent state are disposable. A future operator may stop and reclaim this one computer through its owner-scoped lifecycle path; do not reuse the M9a route result as a healthy platform-follow promotion because `docs/problems/s0-m9a-route-projection-owner-binding-2026-10-04.md` records the failed route-projection tail.
